package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/commands"
	"github.com/datnt1998/noverse/internal/diag"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/entry/startup"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/host/imp"
)

// handleLive 处理契约 §5 新增的 worker 端点（共创、长任务、模型/推理、诊断）。
// 返回 true 表示已接管该路径；由主 switch 的末尾 case 调用。
func (s *workerServer) handleLive(w http.ResponseWriter, r *http.Request, p string) bool {
	switch {
	case s.workerJobs(w, r, p):
	case r.Method == "GET" && strings.HasPrefix(p, "sync/diff/"):
		s.getRevisionDiff(w, r, p)
	case r.Method == "POST" && p == "sync/fix":
		s.postSyncFix(w, r)
	case r.Method == "GET" && p == "reopen":
		s.getReopenChecks(w)
	case r.Method == "GET" && p == "simulate/profile":
		s.getSimulationProfile(w, r)
	case r.Method == "GET" && p == "export/file":
		s.downloadExport(w, r)
	case (r.Method == "POST" || r.Method == "GET") && p == "diag/export":
		s.exportDiagnosis(w)
	case r.Method == "GET" && p == "diag/last":
		s.getLastDiagnosis(w)
	case r.Method == "POST" && strings.HasPrefix(p, "diag/actions/"):
		s.sendDiagnosisAction(w, r, strings.TrimPrefix(p, "diag/actions/"))
	case r.Method == "GET" && p == "cocreate":
		s.getCoCreate(w)
	case r.Method == "POST" && p == "cocreate/message":
		s.postCoCreateMessage(w, r)
	case r.Method == "POST" && p == "cocreate/commit":
		s.postCoCreateCommit(w, r)
	case r.Method == "POST" && p == "cocreate/cancel":
		s.postCoCreateCancel(w, r)
	case r.Method == "POST" && p == "import":
		s.postImport(w, r)
	case r.Method == "POST" && p == "import/preview":
		s.postImportPreview(w, r)
	case r.Method == "POST" && p == "import/confirm":
		s.postImportConfirm(w, r)
	case r.Method == "GET" && p == "import":
		s.getImport(w)
	case r.Method == "POST" && p == "simulate":
		s.postSimulate(w, r)
	case r.Method == "POST" && p == "importsim":
		s.postImportSim(w, r)
	case r.Method == "POST" && p == "sync":
		s.postSync(w, r)
	case r.Method == "GET" && p == "diag":
		writeJSON(w, 200, s.diagnosis())
	case r.Method == "POST" && p == "start-file":
		s.postStartFile(w, r)
	case r.Method == "GET" && p == "models":
		s.getModels(w)
	case r.Method == "POST" && p == "models/context-reload":
		if err := s.h.RefreshModelContexts(); err != nil {
			jsonError(w, 500, err.Error())
			return true
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	case r.Method == "POST" && p == "models":
		s.postModels(w, r)
	case r.Method == "POST" && p == "thinking":
		s.postThinking(w, r)
	case r.Method == "POST" && p == "command":
		s.postCommand(w, r)
	default:
		return false
	}
	return true
}

// ── 路径沙盒 ──
// {path} 相对路径依次相对故事目录（worker cwd）与项目目录解析，绝对路径必须落在
// 书库根内（AINOVEL_LIBRARY_ROOT）；无书库根（旧布局）时相对路径不得越出故事目录。

func (s *workerServer) sandbox() string {
	if s.root != "" {
		return s.root
	}
	return s.storyDir
}

func within(path, boundary string) bool {
	if boundary == "" {
		return true
	}
	path, err := canonicalPath(path)
	if err != nil {
		return false
	}
	boundary, err = canonicalPath(boundary)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(boundary, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

// canonicalPath resolves existing ancestors too, so a new export under /tmp is
// compared correctly with a cwd reported as /private/tmp on macOS.
func canonicalPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

// resolveStoryPath 解析用户给定路径并强制沙盒；mustExist 要求最终文件存在。
func (s *workerServer) resolveStoryPath(raw string, mustExist bool) (string, error) {
	boundary := s.sandbox()
	if filepath.IsAbs(raw) {
		p := filepath.Clean(raw)
		if !within(p, boundary) {
			return "", fmt.Errorf("Đường dẫn phải nằm trong thư viện: %s", raw)
		}
		if mustExist {
			if _, err := os.Stat(p); err != nil {
				return "", fmt.Errorf("Không tìm thấy tệp: %s", raw)
			}
		}
		return p, nil
	}
	bases := []string{s.storyDir}
	if s.projectDir != "" && s.projectDir != s.storyDir && within(s.projectDir, boundary) {
		bases = append(bases, s.projectDir)
	}
	for _, base := range bases {
		p := filepath.Join(base, raw)
		if !within(p, boundary) {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if mustExist {
		return "", fmt.Errorf("Không tìm thấy tệp: %s", raw)
	}
	for _, base := range bases {
		p := filepath.Join(base, raw)
		if within(p, boundary) {
			return p, nil
		}
	}
	return "", fmt.Errorf("Đường dẫn phải nằm trong thư viện: %s", raw)
}

// jobPreCheck 契约 409「đang chạy」：引擎运行中或共创进行中不启动长任务。
// 任务槽冲突由 jobs.begin 判重。
func (s *workerServer) jobPreCheck(w http.ResponseWriter) bool {
	if s.h.Snapshot().IsRunning {
		jsonError(w, 409, "Đang chạy, không thể bắt đầu tác vụ khác")
		return false
	}
	return true
}

// ── Nhập tiểu thuyết ──

func (s *workerServer) postImport(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Path          string `json:"path"`
		StoryStatus   string `json:"storyStatus"`
		ContinueAfter *bool  `json:"continueAfter"`
		Guide         string `json:"guide"`
		AutoConfirm   bool   `json:"autoConfirm"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if v.StoryStatus != "" && v.StoryStatus != "open" && v.StoryStatus != "closed" {
		jsonError(w, 400, "storyStatus chỉ nhận open hoặc closed")
		return
	}
	opts := imp.Options{
		AutoConfirm:      v.AutoConfirm,
		StoryResolution:  v.StoryStatus,
		ContinueAfterSet: v.ContinueAfter != nil,
		Guidance:         v.Guide,
	}
	if v.ContinueAfter != nil {
		opts.ContinueAfter = *v.ContinueAfter
	}
	if !s.jobPreCheck(w) {
		return
	}
	if strings.TrimSpace(v.Path) != "" {
		path, err := s.resolveStoryPath(strings.TrimSpace(v.Path), true)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		opts.SourcePath = path
	}
	id, err := s.launchImport(opts)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": id})
}

// launchImport 占用导入任务槽并启动导入（G13 命令与 POST /import 共用）。
func (s *workerServer) launchImport(opts imp.Options) (string, error) {
	id, ok := s.jobs.begin("import")
	if !ok {
		return "", fmt.Errorf("Tác vụ nhập truyện đang chạy")
	}
	s.jobs.remember(id, func() (string, error) { return s.launchImport(opts) })
	s.emitJob(jobEvent{Job: id, Kind: "import", Message: "Đang nhập truyện", Level: "info"})
	ch, err := s.h.ImportFrom(s.jobs.context(id), opts)
	if err != nil {
		s.finishJob(id, "import", jobEvent{Error: err.Error(), Level: "error"})
		return "", err
	}
	go s.pumpImport(id, ch)
	return id, nil
}

func (s *workerServer) postImportConfirm(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Accept        bool   `json:"accept"`
		StoryStatus   string `json:"storyStatus"`
		ContinueAfter *bool  `json:"continueAfter"`
		Guide         string `json:"guide"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if v.StoryStatus != "" && v.StoryStatus != "open" && v.StoryStatus != "closed" {
		jsonError(w, 400, "storyStatus chỉ nhận open hoặc closed")
		return
	}
	if !v.Accept {
		// 契约未定义"拒绝"语义：客户端放弃本次导入继续，等待工作区保留。
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	if !s.jobPreCheck(w) {
		return
	}
	if v.Guide != "" {
		if h, ok := s.h.(interface {
			ImportState() (imp.PreviewState, error)
		}); ok {
			state, err := h.ImportState()
			if err != nil {
				jsonError(w, 409, err.Error())
				return
			}
			if strings.TrimSpace(v.Guide) != state.Options.Guide {
				jsonError(w, 409, "Hãy chia lại và xem trước hướng dẫn mới trước khi xác nhận")
				return
			}
		}
	}
	id, ok := s.jobs.begin("import")
	if !ok {
		jsonError(w, 409, "Tác vụ nhập truyện đang chạy")
		return
	}
	s.emitJob(jobEvent{Job: id, Kind: "import", Stage: "confirm", Level: "info", Message: "Đã xác nhận nhập truyện"})
	opts := imp.Options{AcceptSegmentation: true, StoryResolution: v.StoryStatus, ContinueAfterSet: v.ContinueAfter != nil}
	if v.ContinueAfter != nil {
		opts.ContinueAfter = *v.ContinueAfter
	}
	ch, err := s.h.ImportFrom(s.jobs.context(id), opts)
	if err != nil {
		s.finishJob(id, "import", jobEvent{Error: err.Error(), Level: "error"})
		jsonError(w, 409, err.Error())
		return
	}
	go s.pumpImport(id, ch)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// postImportPreview stages Architect segmentation for the confirmation screen.
// Guided previews and the subsequent import share the same durable artifact.
func (s *workerServer) postImportPreview(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Path  string `json:"path"`
		Guide string `json:"guide"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	var path string
	var err error
	if strings.TrimSpace(v.Path) != "" {
		path, err = s.resolveStoryPath(strings.TrimSpace(v.Path), true)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
	}
	var preview imp.Preview
	if h, ok := s.h.(interface {
		ImportPreview(context.Context, imp.Options) (imp.Preview, error)
	}); ok {
		if !s.jobPreCheck(w) {
			return
		}
		preview, err = h.ImportPreview(r.Context(), imp.Options{SourcePath: path, Guidance: v.Guide})
	} else {
		jsonError(w, 409, "Kiến trúc sư chưa sẵn sàng để chia lại chương")
		return
	}
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, preview)
}

func (s *workerServer) getImport(w http.ResponseWriter) {
	out := map[string]any{"resumeHint": s.h.ImportResumeHint(), "job": nil}
	out["schemaVersion"] = 1
	if h, ok := s.h.(interface {
		ImportState() (imp.PreviewState, error)
	}); ok {
		state, err := h.ImportState()
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		out["preview"], out["path"], out["options"] = state.Preview, state.Path, state.Options
	}
	if ev, ok := s.jobs.latest("import"); ok {
		out["job"] = ev
	}
	writeJSON(w, 200, out)
}

// ── Mô phỏng văn phong ──

func (s *workerServer) postSimulate(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Dir string `json:"dir"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if !s.jobPreCheck(w) {
		return
	}
	dir := strings.TrimSpace(v.Dir)
	if dir != "" {
		resolved, err := s.resolveStoryPath(dir, false)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		dir = resolved
	}
	id, err := s.launchSimulate(dir)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": id})
}

// launchSimulate 占用仿写任务槽并启动；返回任务 id。
func (s *workerServer) launchSimulate(dir string) (string, error) {
	id, ok := s.jobs.begin("simulate")
	if !ok {
		return "", fmt.Errorf("Tác vụ mô phỏng đang chạy")
	}
	s.jobs.remember(id, func() (string, error) { return s.launchSimulate(dir) })
	s.emitJob(jobEvent{Job: id, Kind: "simulate", Message: "Đang học văn phong", Level: "info"})
	ch, err := s.h.SimulateSource(s.jobs.context(id), dir)
	if err != nil {
		s.finishJob(id, "simulate", jobEvent{Error: err.Error(), Level: "error"})
		return "", err
	}
	go s.pumpSim(id, "simulate", ch)
	return id, nil
}

func (s *workerServer) postImportSim(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if !s.jobPreCheck(w) {
		return
	}
	if strings.TrimSpace(v.Path) == "" {
		jsonError(w, 400, "Thiếu đường dẫn tệp")
		return
	}
	path, err := s.resolveStoryPath(strings.TrimSpace(v.Path), true)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	id, err := s.launchImportSim(path)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": id})
}

// launchImportSim 占用导入画像任务槽并启动；返回任务 id。
func (s *workerServer) launchImportSim(path string) (string, error) {
	id, ok := s.jobs.begin("importsim")
	if !ok {
		return "", fmt.Errorf("Tác vụ nhập hồ sơ đang chạy")
	}
	s.jobs.remember(id, func() (string, error) { return s.launchImportSim(path) })
	s.emitJob(jobEvent{Job: id, Kind: "importsim", Message: "Đang nhập hồ sơ văn phong", Level: "info"})
	ch, err := s.h.ImportSimulationProfile(s.jobs.context(id), path)
	if err != nil {
		s.finishJob(id, "importsim", jobEvent{Error: err.Error(), Level: "error"})
		return "", err
	}
	go s.pumpSim(id, "importsim", ch)
	return id, nil
}

// ── Đồng bộ tu chỉnh ngoài ──

func (s *workerServer) postSync(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Check bool `json:"check"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if v.Check {
		changes, err := s.checkSyncJob()
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		if changes == nil {
			changes = []host.RevisionChange{}
		}
		writeJSON(w, 200, map[string]any{"changed": changes})
		return
	}
	if !s.jobPreCheck(w) {
		return
	}
	id, err := s.launchSync()
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": id})
}

// launchSync 占用同步任务槽并启动；返回任务 id。
func (s *workerServer) launchSync() (string, error) {
	id, ok := s.jobs.begin("sync")
	if !ok {
		return "", fmt.Errorf("Tác vụ đồng bộ đang chạy")
	}
	s.jobs.remember(id, s.launchSync)
	s.emitJob(jobEvent{Job: id, Kind: "sync", Stage: "sync", Level: "info", Message: "Đang đồng bộ tu chỉnh"})
	go s.runSyncJob(id)
	return id, nil
}

// ── /start <tệp> ──

func (s *workerServer) postStartFile(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Path) == "" {
		jsonError(w, 400, "Thiếu đường dẫn tệp")
		return
	}
	snap := s.h.Snapshot()
	if !(snap.Phase == "" && snap.BookTitle == "") {
		jsonError(w, 409, "Truyện đã bắt đầu, không thể dùng /start")
		return
	}
	path, err := s.resolveStoryPath(strings.TrimSpace(v.Path), true)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	raw, err := startup.LoadPromptFile(path)
	if err != nil {
		jsonError(w, 400, fmt.Sprintf("Không đọc được tệp thiết lập: %s", v.Path))
		return
	}
	prompt, err := startup.PrepareQuick(raw)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	pending, err := s.dispatchAction("Đang chuẩn bị bắt đầu sáng tác", func() error { return s.startPrompt(prompt) })
	writeDispatched(w, pending, err, map[string]any{"ok": true, "action": "start"})
}

// startPrompt runs the model-backed rule snapshot and start decision.
func (s *workerServer) startPrompt(prompt string) error {
	if err := s.h.PrepareUserRules(prompt); err != nil {
		return err
	}
	return s.h.StartPrepared(prompt)
}

// ── Model & suy luận ──

func (s *workerServer) getModels(w http.ResponseWriter) {
	roles := make([]map[string]any, 0, 4)
	for _, role := range []string{"default", "architect", "writer", "editor"} {
		provider, model, _ := s.h.CurrentModelSelection(role)
		opts := s.h.AvailableThinkingLevels(role)
		if opts == nil {
			opts = []string{}
		}
		roles = append(roles, map[string]any{
			"role": role, "provider": provider, "model": model,
			"thinking": s.h.CurrentThinking(role), "thinkingOptions": opts,
		})
	}
	writeJSON(w, 200, map[string]any{"roles": roles, "providers": modelCatalogJSON(s.h.ModelCatalog())})
}

func (s *workerServer) postModels(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Role     string `json:"role"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if !host.ValidWebRole(v.Role) {
		jsonError(w, 400, "Vai trò không hợp lệ: "+v.Role)
		return
	}
	if err := s.h.SwitchModel(v.Role, v.Provider, v.Model); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *workerServer) postThinking(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Role  string `json:"role"`
		Level string `json:"level"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	if !host.ValidWebRole(v.Role) {
		jsonError(w, 400, "Vai trò không hợp lệ: "+v.Role)
		return
	}
	if err := s.h.SetRoleThinking(v.Role, v.Level); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Chẩn đoán ──

var diagSectionTitles = map[diag.Category]string{
	diag.CatFlow:     "Luồng viết",
	diag.CatQuality:  "Chất lượng",
	diag.CatPlanning: "Quy hoạch",
	diag.CatContext:  "Ngữ cảnh",
}

func diagLevel(sev diag.Severity) string {
	switch sev {
	case diag.SevCritical:
		return "err"
	case diag.SevWarning:
		return "warn"
	default:
		return "info"
	}
}

// diagResponse 把 diag.Report 投影为契约 §5 的 sections 结构：
// overview（概览指标）→ 各分类 findings → actions（建议动作）。
func diagResponse(rep diag.Report) map[string]any {
	type item = map[string]any
	overview := []item{
		{"label": "Chương", "value": fmt.Sprintf("%d/%d", rep.Stats.CompletedChapters, rep.Stats.TotalChapters), "level": "info"},
		{"label": "Số chữ", "value": fmt.Sprintf("%d", rep.Stats.TotalWords), "level": "info"},
		{"label": "Giai đoạn", "value": rep.Stats.Phase, "level": "info"},
	}
	if rep.Stats.ReviewCount > 0 {
		v := fmt.Sprintf("%d lần", rep.Stats.ReviewCount)
		if rep.Stats.AvgReviewScore > 0 {
			v += fmt.Sprintf(" · điểm TB %.1f", rep.Stats.AvgReviewScore)
		}
		overview = append(overview, item{"label": "Thẩm duyệt", "value": v, "level": "info"})
	}
	if rep.Stats.RewriteCount > 0 {
		overview = append(overview, item{"label": "Viết lại", "value": fmt.Sprintf("%d lần", rep.Stats.RewriteCount), "level": "warn"})
	}
	if rep.Stats.ForeshadowOpen > 0 || rep.Stats.ForeshadowStale > 0 {
		lvl := "info"
		if rep.Stats.ForeshadowStale > 0 {
			lvl = "warn"
		}
		overview = append(overview, item{"label": "Phục bút", "value": fmt.Sprintf("đang mở %d · đình trệ %d", rep.Stats.ForeshadowOpen, rep.Stats.ForeshadowStale), "level": lvl})
	}
	if rep.Stats.PlanningTier != "" {
		overview = append(overview, item{"label": "Quy hoạch", "value": rep.Stats.PlanningTier, "level": "info"})
	}
	if len(rep.Findings) == 0 {
		overview = append(overview, item{"label": "Kết quả", "value": "Không phát hiện vấn đề nào", "level": "ok"})
	}
	sections := []map[string]any{
		{"id": "overview", "title": "Tổng quan", "items": overview},
	}
	byCat := map[diag.Category][]item{}
	order := []diag.Category{}
	for _, f := range rep.Findings {
		if _, seen := byCat[f.Category]; !seen {
			order = append(order, f.Category)
		}
		detail := f.Evidence
		if f.Suggestion != "" {
			if detail != "" {
				detail += "\n"
			}
			detail += "→ " + f.Suggestion
		}
		byCat[f.Category] = append(byCat[f.Category], item{
			"label": f.Title, "value": string(f.Severity), "level": diagLevel(f.Severity), "detail": detail,
		})
	}
	for _, cat := range order {
		title := diagSectionTitles[cat]
		if title == "" {
			title = string(cat)
		}
		sections = append(sections, map[string]any{"id": string(cat), "title": title, "items": byCat[cat]})
	}
	if len(rep.Actions) > 0 {
		items := make([]item, 0, len(rep.Actions))
		for _, a := range rep.Actions {
			items = append(items, item{"label": a.Summary, "value": string(a.Kind), "level": "info", "detail": a.Message})
		}
		sections = append(sections, map[string]any{"id": "actions", "title": "Hành động đề xuất", "items": items})
	}
	return map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339), "sections": sections}
}

// ── Lệnh từ ô chat web (G13) ──

// postCommand 解析并执行一行斜杠命令：语法错误 400（"Cách dùng: …"），
// needsIdle 与 only 违反 409，动作失败 409，成功返回 {ok, message?}。
func (s *workerServer) postCommand(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Line string `json:"line"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	inv, err := commands.Parse(v.Line)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	snap := s.snapshot()
	if inv.Spec.NeedsIdle && snap.IsRunning {
		jsonError(w, 409, "Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /"+inv.Name)
		return
	}
	welcome := snap.Phase == "" && snap.BookTitle == ""
	if inv.Spec.Only == commands.OnlyWelcome && !welcome {
		jsonError(w, 409, "Lệnh chỉ có thể dùng ở màn hình chào mừng: /"+inv.Name)
		return
	}
	if inv.Spec.Only == commands.OnlyRunning && welcome {
		jsonError(w, 409, "Lệnh chỉ khả dụng khi đang viết truyện: /"+inv.Name)
		return
	}
	if inv.Name == "start" || inv.Name == "reopen" {
		// Both reach Arbiter/model calls; command feedback continues via events.
		results := make(chan string, 1)
		pending, err := s.dispatchAction("Đang xử lý lệnh /"+inv.Name, func() error {
			message, _, err := s.executeCommand(context.Background(), inv)
			results <- message
			return err
		})
		message := "Đang xử lý lệnh /" + inv.Name
		select {
		case message = <-results:
		default:
		}
		writeDispatched(w, pending, err, map[string]any{"ok": true, "message": message, "result": nil})
		return
	}
	message, result, err := s.executeCommand(r.Context(), inv)
	if err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": message, "result": result})
}

// dispatchCommand 把已解析的命令映射到既有 Host 动作。help/model/config/cocreate
// 是界面动作，服务端只回执提示，由前端负责打开对应面板。
func (s *workerServer) dispatchCommand(ctx context.Context, inv commands.Invocation) (string, error) {
	switch inv.Name {
	case "help":
		return "Mở hướng dẫn lệnh trên giao diện", nil
	case "model":
		if role := inv.Values["role"]; role != "" {
			return fmt.Sprintf("Mở bảng chọn model cho vai trò %s", host.RoleLabel(role)), nil
		}
		return "Mở bảng chọn model cho cả ba trợ lý", nil
	case "config":
		return "Mở Cài đặt › Nhà cung cấp AI", nil
	case "cocreate":
		return "Mở phòng đồng sáng tác", nil
	case "diag":
		rep := s.h.Diagnose()
		return fmt.Sprintf("Chẩn đoán: %d phát hiện, %d hành động đề xuất", len(rep.Findings), len(rep.Actions)), nil
	case "review":
		mode := domain.ChapterAdvanceMode("auto")
		if inv.Values["mode"] == "on" {
			mode = domain.ChapterAdvanceReview
		}
		if err := s.h.SetAdvanceMode(mode); err != nil {
			return "", err
		}
		if mode == domain.ChapterAdvanceReview {
			return "Đã bật duyệt từng chương", nil
		}
		return "Đã tắt duyệt từng chương", nil
	case "next":
		if err := s.h.AdvanceOneChapter(); err != nil {
			return "", err
		}
		return "Đã cho phép viết chương tiếp theo", nil
	case "reopen":
		if err := s.h.Reopen(inv.Values["dir"]); err != nil {
			return "", err
		}
		label, err := s.h.Resume()
		if err != nil {
			return "", err
		}
		if label == "" {
			return "Đã mở lại truyện để viết tiếp", nil
		}
		return "Đã mở lại và tiếp tục viết: " + label, nil
	case "start":
		path, err := s.resolveStoryPath(inv.Values["path"], true)
		if err != nil {
			return "", err
		}
		if err := s.startFromFile(path); err != nil {
			return "", err
		}
		return "Đã bắt đầu truyện mới từ " + filepath.Base(path), nil
	case "import":
		opts := imp.Options{
			AutoConfirm:     inv.Flags["yes"],
			ContinueAfter:   inv.Flags["cont"],
			StoryResolution: inv.Values["story"],
			Guidance:        inv.Values["guide"],
		}
		if p := strings.TrimSpace(inv.Values["path"]); p != "" {
			path, err := s.resolveStoryPath(p, true)
			if err != nil {
				return "", err
			}
			opts.SourcePath = path
		}
		if _, err := s.launchImport(opts); err != nil {
			return "", err
		}
		if opts.SourcePath == "" {
			return "Đang tiếp tục lần nhập truyện dở", nil
		}
		return "Đang nhập truyện từ " + filepath.Base(opts.SourcePath), nil
	case "simulate":
		if _, err := s.launchSimulate(""); err != nil {
			return "", err
		}
		return "Đang học văn phong từ văn mẫu của bạn", nil
	case "importsim":
		path, err := s.resolveStoryPath(inv.Values["path"], true)
		if err != nil {
			return "", err
		}
		if _, err := s.launchImportSim(path); err != nil {
			return "", err
		}
		return "Đang nhập hồ sơ văn phong " + filepath.Base(path), nil
	case "sync":
		if inv.Flags["check"] {
			changes, err := s.h.CheckRevisionChanges()
			if err != nil {
				return "", err
			}
			if len(changes) == 0 {
				return "Không phát hiện chương nào bị chỉnh sửa từ bên ngoài", nil
			}
			parts := make([]string, 0, len(changes))
			for _, ch := range changes {
				parts = append(parts, fmt.Sprintf("%d", ch.Chapter))
			}
			return "Các chương có chỉnh sửa: " + strings.Join(parts, ", "), nil
		}
		if _, err := s.launchSync(); err != nil {
			return "", err
		}
		return "Đang phân tích và tiếp nhận các chương tu chỉnh", nil
	default:
		return "", fmt.Errorf("Lệnh chưa hỗ trợ trên web: /%s", inv.Name)
	}
}

// startFromFile 用一份设置/大纲文件启动新书（/start 的共享动作）。
func (s *workerServer) startFromFile(path string) error {
	raw, err := startup.LoadPromptFile(path)
	if err != nil {
		return fmt.Errorf("Không đọc được tệp thiết lập: %s", filepath.Base(path))
	}
	prompt, err := startup.PrepareQuick(raw)
	if err != nil {
		return err
	}
	return s.startPrompt(prompt)
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
