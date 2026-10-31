package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
	"github.com/datnt1998/verse/platform/identity"
)

func (g *Gateway) hostedTextDocuments() *postgres.TextDocuments {
	if r, ok := g.libRepo.(interface {
		TextDocuments() *postgres.TextDocuments
	}); ok {
		return r.TextDocuments()
	}
	return nil
}
func textActor(user identity.User) *postgres.Actor {
	return &postgres.Actor{Username: user.Username, GlobalRole: string(user.Role)}
}
func ruleStory(t configTarget) string {
	if t.scope == "story" {
		return t.story.Story
	}
	return ""
}
func ruleVersion(d postgres.TextDocument) string { return fmt.Sprintf("pgd-%d", d.Version) }
func (g *Gateway) hostedRuleFiles(ctx context.Context, user identity.User, t configTarget) ([]postgres.TextDocument, error) {
	repo := g.hostedTextDocuments()
	if repo == nil {
		return nil, errs.ErrStorageUnsupported
	}
	return repo.List(ctx, textActor(user), t.project, ruleStory(t), "rules")
}
func (g *Gateway) hostedRulesChain(ctx context.Context, user identity.User, t configTarget) ([]rulesChainDoc, error) {
	chain := []rulesChainDoc{}
	scopes := []string{"project"}
	if t.scope == "story" {
		scopes = append(scopes, "story")
	}
	for _, scope := range scopes {
		layer := t
		layer.scope = scope
		rows, err := g.hostedRuleFiles(ctx, user, layer)
		if err != nil {
			return nil, err
		}
		doc := rulesChainDoc{Scope: scope, Dir: "", Exists: len(rows) > 0, Files: []ruleFileInfo{}}
		for _, row := range rows {
			if !validRuleName(row.Name) || strings.HasPrefix(row.Name, ".") {
				continue
			}
			ts := row.UpdatedAt.UTC().Format(time.RFC3339)
			doc.Files = append(doc.Files, ruleFileInfo{Name: row.Name, Size: int64(len(row.Text)), UpdatedAt: &ts})
		}
		chain = append(chain, doc)
	}
	return chain, nil
}
func (g *Gateway) hostedRuleFile(w http.ResponseWriter, r *http.Request, user identity.User, t configTarget, name, text string, remove bool) {
	rows, err := g.hostedRuleFiles(r.Context(), user, t)
	if err != nil {
		scopeError(w, err)
		return
	}
	var current postgres.TextDocument
	current.Name = name
	for _, row := range rows {
		if row.Name == name {
			current = row
			break
		}
	}
	resource := func() (any, error) {
		return map[string]any{"name": name, "text": current.Text, "version": ruleVersion(current)}, nil
	}
	if r.Method == http.MethodGet {
		if current.Version == 0 {
			jsonError(w, 404, "không tìm thấy")
			return
		}
		if !checkIfMatchV(w, r, ruleVersion(current), resource) {
			return
		}
		value, _ := resource()
		writeJSON(w, 200, value)
		return
	}
	want := strings.TrimSpace(r.Header.Get("If-Match"))
	if token, err := strconv.Unquote(want); err == nil {
		want = token
	}
	row, err := g.hostedTextDocuments().Mutate(r.Context(), textActor(user), t.project, ruleStory(t), "rules", name, text, want, remove, 0)
	if errors.Is(err, errs.ErrStorageConflict) {
		rows, readErr := g.hostedRuleFiles(r.Context(), user, t)
		if readErr != nil {
			scopeError(w, readErr)
			return
		}
		current = postgres.TextDocument{Name: name}
		for _, d := range rows {
			if d.Name == name {
				current = d
				break
			}
		}
		writeVersionToken(w, ruleVersion(current))
		value, _ := resource()
		writeJSON(w, 409, map[string]any{"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": ruleVersion(current), "current": value})
		return
	}
	if err != nil {
		scopeError(w, err)
		return
	}
	if remove {
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	token := writeVersionToken(w, ruleVersion(row))
	writeJSON(w, 200, map[string]any{"name": name, "size": len(row.Text), "updatedAt": row.UpdatedAt.UTC().Format(time.RFC3339), "version": token})
}
func (g *Gateway) serveHostedStyleSamples(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, tail string) {
	repo := g.hostedTextDocuments()
	if repo == nil {
		scopeError(w, errs.ErrStorageUnsupported)
		return
	}
	rows, err := repo.List(r.Context(), textActor(user), st.Project, st.Story, "style_sample")
	if err != nil {
		scopeError(w, err)
		return
	}
	sample := func(d postgres.TextDocument) StyleSample {
		return StyleSample{ID: d.Name, Title: styleSampleTitle(d.Name), Chars: domain.WordCount(utils.DecodeText([]byte(d.Text))), AddedAt: d.UpdatedAt.Format(time.RFC3339)}
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		samples := []StyleSample{}
		total := StyleSampleTotals{Count: len(rows)}
		for _, d := range rows {
			s := sample(d)
			samples = append(samples, s)
			total.Chars += s.Chars
		}
		writeJSON(w, 200, map[string]any{"samples": samples, "total": total})
	case tail == "" && r.Method == http.MethodPost:
		if len(rows) >= maxStyleSamples {
			jsonError(w, 409, fmt.Sprintf("Đã có %d văn mẫu — xoá bớt trước khi thêm.", maxStyleSamples))
			return
		}
		title, text, ok := readStyleSampleBody(w, r)
		if !ok {
			return
		}
		if strings.TrimSpace(text) == "" {
			jsonError(w, 400, "Cần dán nội dung văn mẫu")
			return
		}
		row, err := repo.Mutate(r.Context(), textActor(user), st.Project, st.Story, "style_sample", styleSampleBase(title)+".txt", utils.DecodeText([]byte(text)), "", false, maxStyleSamples)
		if errors.Is(err, errs.ErrStorageConflict) {
			jsonError(w, 409, fmt.Sprintf("Đã có %d văn mẫu — xoá bớt trước khi thêm.", maxStyleSamples))
			return
		}
		if err != nil {
			scopeError(w, err)
			return
		}
		g.storyChanged(st.Project, st.Story, "style")
		writeJSON(w, 201, sample(row))
	case tail != "" && r.Method == http.MethodDelete:
		if _, err := styleSamplePath("simulate", tail); err != nil {
			jsonError(w, 404, "không tìm thấy văn mẫu")
			return
		}
		_, err := repo.Mutate(r.Context(), textActor(user), st.Project, st.Story, "style_sample", tail, "", "", true, 0)
		if err != nil {
			scopeError(w, err)
			return
		}
		g.storyChanged(st.Project, st.Story, "style")
		w.WriteHeader(204)
	default:
		jsonError(w, 404, "không tìm thấy")
	}
}
