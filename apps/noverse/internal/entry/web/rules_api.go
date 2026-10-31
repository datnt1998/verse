package web

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/userrules"
	"github.com/datnt1998/verse/platform/identity"
)

// Các endpoint quy tắc chạy thẳng trong gateway (như audio/export): chúng chỉ đọc/ghi tệp của
// truyện và gọi model chuẩn hoá, không cần engine worker đang sống.

const (
	understandTimeout = 2 * time.Minute
	applyTimeout      = 5 * time.Minute
	understandMaxText = 2000
	violationsDefault = 3
	violationsMax     = 50
	rulesRecheckKind  = "rules.recheck"
)

var errRulesNoModel = errors.New("rules: no model configured")

// rulesModel dựng model chuẩn hoá từ cấu hình đã chọn (cùng một cấu hình với viết truyện).
// chainDir == "" nghĩa là chỉ lớp toàn cục. Thiếu provider/model → errRulesNoModel.
func rulesModel(chainDir string) (llmtypes.ChatModel, error) {
	var cfg bootstrap.Config
	var err error
	if chainDir == "" {
		cfg, err = bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
	} else {
		cfg, err = bootstrap.LoadConfigAt(chainDir)
	}
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", errRulesNoModel, err)
	}
	return rulesModelForConfig(cfg)
}

func rulesModelForConfig(cfg bootstrap.Config) (llmtypes.ChatModel, error) {
	if strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, errRulesNoModel
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", errRulesNoModel, err)
	}
	return models.Default, nil
}

func writeRulesNoModel(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]any{"error": needsModelMessage, "needsConfig": true})
}

// POST /api/rules/understand {text, scope?, project?, story?} → {structured, preferences, uncertain, chips, status}.
// Không lưu gì. scope/project/story chỉ chọn chuỗi cấu hình model (mặc định: toàn cục).
func (g *Gateway) serveRulesUnderstand(w http.ResponseWriter, r *http.Request, user identity.User) {
	var req struct {
		Text    string `json:"text"`
		Scope   string `json:"scope,omitempty"`
		Project string `json:"project,omitempty"`
		Story   string `json:"story,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		jsonError(w, http.StatusBadRequest, "Hãy nhập quy tắc cần kiểm tra")
		return
	}
	if len([]rune(text)) > understandMaxText {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("Quy tắc quá dài (tối đa %d ký tự)", understandMaxText))
		return
	}
	t, err := g.resolveConfigScope(req.Scope, req.Project, req.Story)
	if err != nil {
		scopeError(w, err)
		return
	}
	if err := checkConfigWrite(user, t); err != nil {
		scopeError(w, err)
		return
	}
	model, err := g.rulesModelTarget(r.Context(), t)
	if err != nil {
		writeRulesNoModel(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), understandTimeout)
	defer cancel()
	u, err := userrules.NewNormalizer(model).Understand(ctx, text)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			jsonError(w, http.StatusGatewayTimeout, "Mô hình phản hồi quá lâu, hãy thử lại")
		case r.Context().Err() != nil:
			// Khách đã ngắt kết nối: không còn ai nhận phản hồi.
		default:
			jsonError(w, http.StatusInternalServerError, "Không phân tích được quy tắc")
		}
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// GET /api/rules/effective?scope=global|project&project= → {snapshot, items}: như S/rules/effective
// nhưng không có truyện (không runtime, không timing, không cấp truyện). Dòng chưa chuẩn hoá được
// (chưa có model, lỗi model) hạ cấp thành lời dặn nguyên văn — GET này không bao giờ 409.
func (g *Gateway) serveRulesEffectiveScoped(w http.ResponseWriter, r *http.Request, user identity.User) {
	q := r.URL.Query()
	scope := q.Get("scope")
	if scope == "" {
		scope = "global"
	}
	if scope != "global" && scope != "project" {
		jsonError(w, http.StatusBadRequest, "scope phải là global hoặc project")
		return
	}
	t, err := g.resolveConfigScope(scope, q.Get("project"), "")
	if err != nil {
		scopeError(w, err)
		return
	}
	if scope == "project" && !user.CanReadProject(t.project) {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	opts := rules.LoadOptions{HomeRulesDir: rulesHomeDir()}
	if t.hosted {
		repo := g.hostedTextDocuments()
		if repo == nil {
			scopeError(w, library.ErrStorageUnsupported)
			return
		}
		opts, err = repo.RuleOptions(r.Context(), textActor(user), t.project, "")
		if err != nil {
			scopeError(w, err)
			return
		}
	} else if scope == "project" {
		opts.ProjectRulesDir = rulesDirIn(t.dir)
	}
	cachePath := ""
	if home := rulesHomeDir(); home != "" {
		cachePath = filepath.Join(filepath.Dir(home), "rules-cache.json")
	}
	var model llmtypes.ChatModel
	if m, err := g.rulesModelTarget(r.Context(), t); err == nil {
		model = m
	}
	ctx, cancel := context.WithTimeout(r.Context(), applyTimeout)
	defer cancel()
	eff, err := userrules.NewFileRules(model, opts, cachePath).Effective(ctx)
	if err != nil {
		if r.Context().Err() == nil {
			jsonError(w, http.StatusInternalServerError, "Không đọc được quy tắc: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, eff)
}

// rulesService dựng dịch vụ quy tắc cho một truyện. model có thể nil (không có cấu hình):
// khi đó mọi dòng chưa có trong bộ nhớ đệm sẽ hạ cấp thành lời dặn nguyên văn.
func (g *Gateway) rulesService(st library.Story, sr *host.StoryReader, model llmtypes.ChatModel) (*userrules.Service, error) {
	if st.Dir == "" {
		style := ""
		if s := sr.Store(); s != nil {
			if meta, err := s.RunMeta.Load(); err == nil && meta != nil {
				style = meta.Style
			}
		}
		bundle := assets.Load(style, assets.LoadOptions{})
		if bundle.Err != nil {
			return nil, bundle.Err
		}
		pack, err := rules.PackDefaults(bundle.Pack)
		if err != nil {
			return nil, err
		}
		repo := g.hostedTextDocuments()
		if repo == nil {
			return nil, library.ErrStorageUnsupported
		}
		opts, err := repo.RuleOptions(context.Background(), nil, st.Project, st.Story)
		if err != nil {
			return nil, err
		}
		return userrules.NewService(sr.Store(), model, opts, pack), nil
	}
	cfg, err := bootstrap.LoadConfigAt(st.Dir)
	if err != nil {
		return nil, err
	}
	bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(sr.EngineDir()))
	if bundle.Err != nil {
		return nil, bundle.Err
	}
	pack, err := rules.PackDefaults(bundle.Pack)
	if err != nil {
		return nil, err
	}
	return userrules.NewService(sr.Store(), model, rules.OptionsForDir(st.Dir), pack), nil
}

// serveStoryRules xử lý S/rules/*: effective · apply · runtime/:id[/keep] · violations。
// 授权已在 serveStories 的 storyAccessAllowed 完成（GET 需可读、其余需可写）。
func (g *Gateway) serveStoryRules(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader, tail string) {
	segs := strings.Split(tail, "/")
	switch {
	case tail == "effective" && r.Method == http.MethodGet:
		g.serveRulesEffective(w, r, st, sr)
	case tail == "violations" && r.Method == http.MethodGet:
		serveRulesViolations(w, r, sr)
	case tail == "apply" && r.Method == http.MethodPost:
		g.serveRulesApply(w, r, st, sr)
	case len(segs) == 3 && segs[0] == "runtime" && segs[2] == "keep" && r.Method == http.MethodPost:
		g.serveRulesRuntimeKeep(w, st, sr, segs[1])
	case len(segs) == 2 && segs[0] == "runtime" && r.Method == http.MethodDelete:
		g.serveRulesRuntimeDelete(w, st, sr, segs[1])
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// storyModelOrNil: model của truyện, nil khi chưa cấu hình.
func (g *Gateway) storyModelOrNil(st library.Story) llmtypes.ChatModel {
	m, err := g.rulesModelTarget(context.Background(), configTarget{hosted: st.Dir == "", project: st.Project, scope: "story", story: st, chainTo: st.Dir})
	if err != nil {
		return nil
	}
	return m
}

func (g *Gateway) serveRulesEffective(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	ctx, cancel := context.WithTimeout(r.Context(), applyTimeout)
	defer cancel()
	svc, err := g.rulesService(st, sr, g.storyModelOrNil(st))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được gói quy tắc: "+err.Error())
		return
	}
	eff, err := svc.Effective(ctx)
	if err != nil {
		if r.Context().Err() == nil {
			jsonError(w, http.StatusInternalServerError, "Không đọc được quy tắc: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, eff)
}

func serveRulesViolations(w http.ResponseWriter, r *http.Request, sr *host.StoryReader) {
	last := violationsDefault
	if raw := r.URL.Query().Get("last"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > violationsMax {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("last phải là số từ 1 đến %d", violationsMax))
			return
		}
		last = n
	}
	chapters := sr.Store().World.RecentRuleViolations(last)
	out := make([]ruleViolationChapter, 0, len(chapters))
	for _, c := range chapters {
		vs := c.Violations
		if vs == nil {
			vs = []rules.Violation{}
		}
		out = append(out, ruleViolationChapter{Chapter: c.Chapter, At: c.At, Violations: vs})
	}
	writeJSON(w, http.StatusOK, ruleViolationsResponse{Chapters: out})
}

func (g *Gateway) serveRulesApply(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	var req struct {
		Recheck bool `json:"recheck,omitempty"`
	}
	if body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)); err != nil {
		jsonError(w, http.StatusBadRequest, "JSON không hợp lệ")
		return
	} else if len(strings.TrimSpace(string(body))) > 0 {
		d := json.NewDecoder(bytes.NewReader(body))
		d.DisallowUnknownFields()
		if err := d.Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, jsonBodyMessage(err))
			return
		}
	}
	model := g.storyModelOrNil(st)
	if model == nil {
		writeRulesNoModel(w)
		return
	}
	startPrompt := ""
	if meta, err := sr.Store().RunMeta.Load(); err == nil && meta != nil {
		startPrompt = meta.StartPrompt
	}
	ctx, cancel := context.WithTimeout(r.Context(), applyTimeout)
	defer cancel()
	svc, err := g.rulesService(st, sr, model)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được gói quy tắc: "+err.Error())
		return
	}
	res, err := svc.Apply(ctx, startPrompt)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			jsonError(w, http.StatusGatewayTimeout, "Mô hình phản hồi quá lâu, chưa áp dụng gì cả")
		case r.Context().Err() != nil:
		default:
			jsonError(w, http.StatusInternalServerError, "Không áp dụng được quy tắc: "+err.Error())
		}
		return
	}
	g.storyChanged(st.Project, st.Story, "rules")
	resp := map[string]any{"snapshot": res.Snapshot, "effectiveFrom": res.EffectiveFrom}
	if res.PinnedChapter > 0 {
		resp["pinnedChapter"] = res.PinnedChapter
	}
	if req.Recheck {
		resp["jobId"] = g.startRulesRecheck(st, svc, res.Snapshot)
	}
	writeJSON(w, http.StatusOK, resp)
}

// startRulesRecheck mở job rules.recheck chạy nền: kiểm tra máy mọi chương đã lưu theo snapshot
// vừa áp dụng và ghi vi phạm. Không bao giờ sửa chương. Trả về id job.
func (g *Gateway) startRulesRecheck(st library.Story, svc *userrules.Service, snap *rules.Snapshot) string {
	hub := g.jobsHub()
	hub.mu.Lock()
	hub.seq++
	id := fmt.Sprintf("%s/%s/%d", st.ID, rulesRecheckKind, uint64(time.Now().UnixNano())+hub.seq)
	if hub.controls == nil {
		hub.controls = map[string]func(context.Context, string) error{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	hub.controls[id] = func(_ context.Context, op string) error {
		if op != "cancel" {
			return errJobUnsupported
		}
		cancel()
		return nil
	}
	hub.mu.Unlock()
	job := Job{ID: id, Kind: rulesRecheckKind, Title: "Kiểm tra lại quy tắc", Status: JobRunning,
		Href: storyRouteBase(st.Project, st.Story) + "?tab=rules", Project: st.Project, Story: st.Story, StartedAt: time.Now()}
	hub.apply(JobChange{Job: job})
	go func() {
		defer cancel()
		res, err := svc.Recheck(ctx, snap, func(done, total, chapter int) {
			p := float64(done) / float64(max(total, 1))
			job.Progress = &p
			job.Unit = &JobUnit{Done: done, Total: total, Label: "Chương"}
			job.Step = fmt.Sprintf("Chương %d", chapter)
			hub.apply(JobChange{Job: job})
		})
		g.storyChanged(st.Project, st.Story, "rules")
		now := time.Now()
		job.FinishedAt = &now
		switch {
		case errors.Is(err, context.Canceled):
			hub.apply(JobChange{Job: job, Removed: true})
		case err != nil:
			job.Status, job.Error = JobFailed, err.Error()
			hub.apply(JobChange{Job: job})
		default:
			p := 1.0
			job.Status, job.Progress = JobDoneS, &p
			job.Result = &JobResult{
				Title:   fmt.Sprintf("Đã kiểm tra lại %d chương", res.Checked),
				Sub:     fmt.Sprintf("%d chương vi phạm · %d lỗi", res.WithViolations, res.Violations),
				Actions: []JobAction{{Label: "Xem quy tắc", Kind: "open", Href: storyRouteBase(st.Project, st.Story) + "?tab=rules", Primary: true}},
				Data:    res,
			}
			hub.apply(JobChange{Job: job})
		}
	}()
	return id
}

func (g *Gateway) serveRulesRuntimeKeep(w http.ResponseWriter, st library.Story, sr *host.StoryReader, rawID string) {
	id, err := url.PathUnescape(rawID)
	if err != nil || id == "" {
		jsonError(w, http.StatusBadRequest, "id không hợp lệ")
		return
	}
	svc, err := g.rulesService(st, sr, nil)
	if err != nil {
		writeRuntimeRuleError(w, err)
		return
	}
	rule, err := svc.KeepRuntime(id)
	if err != nil {
		writeRuntimeRuleError(w, err)
		return
	}
	g.storyChanged(st.Project, st.Story, "rules")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": rule.Text, "file": rules.ManagedRulesFile})
}

func (g *Gateway) serveRulesRuntimeDelete(w http.ResponseWriter, st library.Story, sr *host.StoryReader, rawID string) {
	id, err := url.PathUnescape(rawID)
	if err != nil || id == "" {
		jsonError(w, http.StatusBadRequest, "id không hợp lệ")
		return
	}
	svc, err := g.rulesService(st, sr, nil)
	if err != nil {
		writeRuntimeRuleError(w, err)
		return
	}
	if err := svc.DropRuntime(id); err != nil {
		writeRuntimeRuleError(w, err)
		return
	}
	g.storyChanged(st.Project, st.Story, "rules")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeRuntimeRuleError(w http.ResponseWriter, err error) {
	if errors.Is(err, userrules.ErrRuntimeRuleNotFound) {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	jsonError(w, http.StatusInternalServerError, err.Error())
}
