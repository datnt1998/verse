package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/platform/identity"
)

type mutationResponse struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *mutationResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *mutationResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *mutationResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	if w.body.Len() < 1<<20 {
		n := min(len(data), (1<<20)-w.body.Len())
		_, _ = w.body.Write(data[:n])
	}
	return w.ResponseWriter.Write(data)
}
func (w *mutationResponse) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (g *Gateway) storyChanged(project, story string, parts ...string) {
	g.events().Publish(Event{Type: "story.changed", Project: project, Story: story, Data: map[string]any{"project": project, "story": story, "parts": parts}})
}
func (g *Gateway) projectChanged(project string) {
	g.events().Publish(Event{Type: "project.changed", Project: project, Data: map[string]any{"project": project}})
}
func (g *Gateway) configChanged(scope, project, story string) {
	g.restartUnconfiguredWorkers(scope, project, story)
	data := map[string]any{"scope": scope}
	if project != "" {
		data["project"] = project
	}
	if story != "" {
		data["story"] = story
	}
	g.events().Publish(Event{Type: "config.changed", Project: project, Story: story, Data: data})
}
func (g *Gateway) accountChanged(username string) {
	g.events().Publish(Event{Type: "account.changed", User: username, AdminsToo: true, Data: map[string]any{"username": username}})
	users, err := g.accounts.Users()
	if err != nil {
		return
	}
	for _, view := range users {
		if view.Username == username {
			g.events().RefreshUser(g.accounts.UserFromView(view.UserView))
			return
		}
	}
	g.events().DropUser(username)
}

func (g *Gateway) publishMutation(r *http.Request, user identity.User, current identity.Session, data []byte) {
	path := strings.Trim(r.URL.Path, "/")
	segs := strings.Split(path, "/")
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	switch {
	case path == "api/library":
		g.events().Publish(Event{Type: "library.changed", Data: map[string]any{}})
	case path == "api/config" || path == "api/config/oauth/logout" || strings.HasPrefix(path, "api/rules/"):
		q := r.URL.Query()
		scope := q.Get("scope")
		p, s := q.Get("project"), q.Get("story")
		if res, ok := r.Context().Value(mutationScopeKey{}).(mutationScope); ok {
			scope, p, s = res.scope, res.project, res.story
		}
		if scope == "" {
			scope = "global"
		}
		if strings.HasPrefix(path, "api/rules/") {
			g.events().Publish(Event{Type: "config.changed", Project: p, Story: s, Data: map[string]any{"section": "rules", "scope": scope, "project": p, "story": s}})
		} else {
			g.configChanged(scope, p, s)
		}
	case path == "api/me" || strings.HasPrefix(path, "api/me/"):
		g.accountChanged(user.Username)
	case path == "api/logout":
		g.revokeEvent(user.Username, current.ID)
		g.accountChanged(user.Username)
	case len(segs) >= 3 && segs[1] == "sessions":
		// The owner was resolved before revocation while the session still exists.
		if owner, ok := r.Context().Value(revokedOwnerKey{}).(string); ok {
			g.revokeEvent(owner, segs[2])
			g.accountChanged(owner)
		}
	case len(segs) >= 2 && segs[1] == "users":
		name, _ := result["username"].(string)
		if len(segs) > 2 {
			name = segs[2]
		}
		if name != "" {
			g.accountChanged(name)
		}
	case len(segs) >= 2 && segs[1] == "projects":
		if len(segs) == 2 || segs[2] == "adopt" {
			g.events().Publish(Event{Type: "library.changed", Data: map[string]any{}})
			if project, ok := result["id"].(string); ok {
				g.projectChanged(project)
			}
			return
		}
		p := segs[2]
		if len(segs) == 4 && segs[3] == "stories" {
			if story, ok := result["story"].(string); ok {
				g.storyChanged(p, story, "meta", "snapshot", "chapters", "outline")
			}
		}
		if len(segs) > 4 && segs[3] == "stories" {
			s := segs[4]
			parts := []string{"snapshot"}
			if len(segs) > 5 {
				switch segs[5] {
				case "meta", "rename":
					parts = append(parts, "meta")
				case "exports", "export":
					parts = append(parts, "exports")
				case "audio":
					parts = append(parts, "audio")
				case "reviews", "review", "comments":
					parts = append(parts, "reviews")
				case "outline":
					parts = append(parts, "outline", "chapters")
				case "sync", "import", "importsim", "next":
					parts = append(parts, "chapters", "outline", "reviews", "meta")
				}
			}
			g.storyChanged(p, s, parts...)
			if len(segs) > 5 && segs[5] == "rename" {
				if renamed, ok := result["story"].(string); ok && renamed != s {
					g.storyChanged(p, renamed, "meta", "snapshot")
				}
			}
		}
		g.projectChanged(p)
		g.events().Publish(Event{Type: "library.changed", Data: map[string]any{}})
	}
	if previous, ok := r.Context().Value(previousSessionsKey{}).([]identity.Session); ok {
		remaining, _ := g.accounts.Sessions("", current.ID)
		active := make(map[string]bool, len(remaining))
		for _, session := range remaining {
			active[session.ID] = true
		}
		for _, session := range previous {
			if !active[session.ID] {
				g.revokeEvent(session.Username, session.ID)
			}
		}
	}
}

func (g *Gateway) revokeEvent(username, session string) {
	g.events().Publish(Event{Type: "session.revoked", User: username, Data: map[string]any{"username": username, "session": session}})
}

type mutationScopeKey struct{}
type revokedOwnerKey struct{}
type previousSessionsKey struct{}
type mutationScope struct{ scope, project, story string }

// Worker events use stable tool identifiers, not translated summaries. Tool
// completion and job completion arrive only after their persistent writes.
func (g *Gateway) publishWorkerChange(key, kind string, data []byte) {
	g.notifyWorkerEvent(key, kind, data)
	p, s, ok := strings.Cut(key, "/")
	if !ok || strings.HasPrefix(key, "draft:") {
		return
	}
	if kind == "job" || kind == "job.done" {
		var job struct {
			Kind   string `json:"kind"`
			Done   bool   `json:"done"`
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &job) != nil || (kind == "job" && !job.Done) || (kind == "job.done" && job.Status != "done") {
			return
		}
		switch job.Kind {
		case "export":
			g.storyChanged(p, s, "exports", "snapshot")
		case "sync", "import", "importsim":
			g.storyChanged(p, s, "snapshot", "chapters", "outline", "reviews", "meta", "comments")
			// §AI: nhận bản sửa tay / nhập truyện là ghi chương — phát sự kiện
			// gắn lại góp ý mà việc nền đã xếp.
			g.publishCommentReanchor(key)
		}
		return
	}
	if kind != "event" {
		return
	}
	var ev host.Event
	if json.Unmarshal(data, &ev) != nil || ev.Failed {
		return
	}
	if ev.Category == "TOOL" && !ev.FinishedAt.IsZero() {
		tool := strings.Fields(ev.Summary)
		if len(tool) == 0 {
			return
		}
		names := strings.FieldsFunc(tool[0], func(r rune) bool { return r == '(' || r == '[' })
		if len(names) == 0 {
			return
		}
		name := names[0]
		switch name {
		case "commit_chapter", "draft_chapter", "edit_chapter":
			g.storyChanged(p, s, "snapshot", "chapters", "reviews", "comments")
			g.publishCommentReanchor(key)
		case "plan_chapter":
			g.storyChanged(p, s, "snapshot", "chapters", "reviews")
		case "save_review", "review_chapter", "submit_review", "save_review_result":
			g.storyChanged(p, s, "snapshot", "reviews", "chapters")
		case "save_outline", "save_foundation", "save_layered_outline", "append_volume", "revise_outline", "revise_layered_outline", "resolve_outline_feedback":
			g.storyChanged(p, s, "snapshot", "outline", "chapters")
		}
	}
	if ev.Category == "SYSTEM" && ev.Level == "success" && strings.HasPrefix(ev.Summary, "Đã xuất ") {
		g.storyChanged(p, s, "exports")
	}
}

// publishCommentReanchor lấy các sự kiện gắn lại góp ý mà đường ghi chương
// (tool, sync, import) đã xếp vào .ainovel/gop-y/_events.json rồi phát SSE
// comments.reanchored cho từng chương — cho dải báo/toast trên FE.
func (g *Gateway) publishCommentReanchor(key string) {
	st, err := g.resolveStoryID(key)
	if err != nil {
		return
	}
	// Drain the reanchor journal through the backend comment surface: file
	// mode reads <story>/.ainovel/gop-y/_events.json; hosted drains the
	// comment_events story_state row via the internal scoped store — never a
	// directory fallback on st.Dir == "".
	cs := comments.New(st.Dir)
	if g.factStore != nil {
		engine, err := g.openFactStoreInternal(st)
		if err != nil {
			return
		}
		cs = engine.Comments()
	}
	if cs == nil {
		return
	}
	events, err := cs.DrainEvents()
	if err != nil {
		return
	}
	for _, ev := range events {
		g.events().Publish(Event{
			Type: "comments.reanchored", Project: st.Project, Story: st.Story,
			Data: map[string]any{
				"chapter": ev.Chapter, "fixed": ev.Fixed, "unfixed": ev.Unfixed, "need": ev.Need,
			},
		})
	}
}
