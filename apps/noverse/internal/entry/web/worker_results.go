package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/datnt1998/noverse/internal/commands"
	"github.com/datnt1998/noverse/internal/diag"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/host/exp"
)

type exportConflictError struct{ payload map[string]any }

func (e *exportConflictError) Error() string {
	return "Tệp đã tồn tại; chọn ghi đè hoặc giữ cả hai bản"
}

func exportConflict(path string, incoming *exp.Result, root string) map[string]any {
	display := publicExportPath(root, path)
	existing := map[string]any{"path": display, "size": 0, "bytes": 0, "modTime": nil, "modifiedAt": nil, "chapters": nil}
	if info, err := os.Stat(path); err == nil {
		existing["size"], existing["bytes"] = info.Size(), info.Size()
		existing["modTime"], existing["modifiedAt"] = info.ModTime(), info.ModTime()
	}
	if chapters, ok := exp.CountChapters(path); ok {
		existing["chapters"] = chapters
	}
	next := map[string]any{"path": display, "chapters": incoming.Chapters, "size": incoming.Bytes, "bytesEstimate": incoming.Bytes, "skipped": nonNilInts(incoming.Skipped)}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	return map[string]any{"path": display, "format": format, "existing": existing, "incoming": next}
}

func publicExportPath(root, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path))
	}
	resolved, err := canonicalPath(path)
	boundary, boundaryErr := canonicalPath(root)
	if err == nil && boundaryErr == nil {
		if relative, err := filepath.Rel(boundary, resolved); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(relative)
		}
	}
	return filepath.Base(path)
}

func publicExportError(err error, root string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	var fileError *os.PathError
	if errors.As(err, &fileError) {
		message = strings.ReplaceAll(message, fileError.Path, publicExportPath(root, fileError.Path))
	}
	roots := []string{root}
	if resolved, err := canonicalPath(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	for _, prefix := range roots {
		if prefix != "" {
			message = strings.ReplaceAll(message, strings.TrimRight(prefix, string(filepath.Separator))+string(filepath.Separator), "")
		}
	}
	return errors.New(message)
}

func writeCommandError(w http.ResponseWriter, err error) {
	var conflict *exportConflictError
	if errors.As(err, &conflict) {
		body := map[string]any{"error": err.Error(), "existing": conflict.payload["existing"], "incoming": conflict.payload["incoming"], "result": map[string]any{"kind": "exportExists", "exists": conflict.payload}}
		writeJSON(w, http.StatusConflict, body)
		return
	}
	jsonError(w, http.StatusConflict, err.Error())
}

func nonNilInts(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func (s *workerServer) exportOptions(inv commands.Invocation) (exp.Options, error) {
	opts := exp.Options{OnExists: "fail"}
	if inv.Flags["ow"] {
		opts.OnExists = "replace"
	}
	if inv.Flags["keep"] {
		opts.OnExists = "keep"
	}
	if inv.Flags["docso"] {
		opts.ReadNumbers = true
	}
	// 第一个位置参数是格式名（txt / epub / narration）时按格式处理，否则当路径。
	// 也可显式写 format=narration（路径位置不受影响）。
	path := strings.TrimSpace(inv.Values["path"])
	switch exp.Format(path) {
	case exp.FormatTXT, exp.FormatEPUB, exp.FormatNarration:
		opts.Format, path = exp.Format(path), ""
	}
	if p := path; p != "" {
		resolved, err := s.resolveStoryPath(p, false)
		if err != nil {
			return opts, err
		}
		opts.OutPath = resolved
	}
	if f := strings.TrimSpace(inv.Values["format"]); f != "" {
		opts.Format = exp.Format(f)
	}
	opts.From, _ = strconv.Atoi(inv.Values["from"])
	opts.To, _ = strconv.Atoi(inv.Values["to"])
	return opts, nil
}

func (s *workerServer) launchExport(opts exp.Options) (string, string, error) {
	preview := opts
	preview.DryRun = true
	preview.Progress = nil
	incoming, err := s.h.Export(context.Background(), preview)
	if err != nil {
		return "", "", publicExportError(err, s.storyDir)
	}
	path := incoming.Path
	mode := opts.OnExists
	if opts.Overwrite {
		mode = "replace"
	}
	if mode == "" || mode == "fail" {
		if _, err := os.Stat(path); err == nil {
			return "", path, &exportConflictError{payload: exportConflict(path, incoming, s.storyDir)}
		}
	}
	opts.OutPath = path
	id, ok := s.jobs.begin("export")
	if !ok {
		return "", path, fmt.Errorf("Tác vụ xuất truyện đang chạy")
	}
	s.jobs.remember(id, func() (string, error) { next, _, err := s.launchExport(opts); return next, err })
	s.emitJob(jobEvent{Job: id, Kind: "export", Stage: "render", Level: "info", Message: "Đang xuất truyện"})
	opts.Progress = func(done, total, chapter int, title string) {
		s.emitJob(jobEvent{Job: id, Kind: "export", Stage: "render", Level: "info", Current: done, Total: total, Message: fmt.Sprintf("Chương %d · %s", chapter, title)})
	}
	go func() {
		res, err := s.h.Export(s.jobs.context(id), opts)
		last := jobEvent{Job: id, Kind: "export", Stage: "done", Level: "success", Message: "Đã xuất truyện"}
		if err != nil {
			last.Level = "error"
			last.Error = publicExportError(err, s.storyDir).Error()
			last.Message = last.Error
		} else {
			rel := res.Path
			if s.storyDir != "" {
				if p, err := filepath.Rel(s.storyDir, res.Path); err == nil {
					rel = filepath.ToSlash(p)
				}
			}
			last.Export = map[string]any{"path": rel, "name": filepath.Base(res.Path), "title": s.h.Snapshot().BookTitle, "format": strings.TrimPrefix(filepath.Ext(res.Path), "."), "bytes": res.Bytes, "chapters": res.Chapters, "skipped": nonNilInts(res.Skipped)}
		}
		s.finishJob(id, "export", last)
	}()
	rel := path
	if s.storyDir != "" {
		if p, err := filepath.Rel(s.storyDir, path); err == nil {
			rel = filepath.ToSlash(p)
		}
	}
	return id, rel, nil
}

func (s *workerServer) checkSyncJob() ([]host.RevisionChange, error) {
	id, ok := s.jobs.begin("sync")
	if !ok {
		return nil, fmt.Errorf("Tác vụ đồng bộ đang chạy")
	}
	s.emitJob(jobEvent{Job: id, Kind: "sync", Stage: "check", Level: "info", Message: "Đang kiểm tra tu chỉnh"})
	changes, err := s.h.CheckRevisionChanges()
	if changes == nil {
		changes = []host.RevisionChange{}
	}
	last := jobEvent{Job: id, Kind: "sync", Stage: "check", Level: "success", Message: fmt.Sprintf("Phát hiện %d chương có tu chỉnh", len(changes)), Sync: map[string]any{"changed": changes}}
	if err != nil {
		last.Error = err.Error()
		last.Level = "error"
	}
	s.finishJob(id, "sync", last)
	return changes, err
}

func (s *workerServer) executeCommand(ctx context.Context, inv commands.Invocation) (string, any, error) {
	switch inv.Name {
	case "export":
		opts, err := s.exportOptions(inv)
		if err != nil {
			return "", nil, err
		}
		id, path, err := s.launchExport(opts)
		return "Đang xuất truyện", map[string]any{"kind": "job", "job": id, "jobKind": "export", "path": path}, err
	case "sync":
		if inv.Flags["check"] {
			changes, err := s.checkSyncJob()
			return "Đã kiểm tra tu chỉnh", map[string]any{"kind": "syncCheck", "changed": changes}, err
		}
	case "diag":
		return "Mở bảng chẩn đoán", map[string]any{"kind": "panel", "panel": "diag"}, nil
	case "model":
		return "Mở bảng chọn model", map[string]any{"kind": "panel", "panel": "model", "role": inv.Values["role"]}, nil
	}
	message, err := s.dispatchCommand(ctx, inv)
	if err != nil {
		return "", nil, err
	}
	switch inv.Name {
	case "sync", "simulate", "importsim", "import":
		if ev, ok := s.jobs.latest(inv.Name); ok {
			return message, map[string]any{"kind": "job", "job": ev.Job, "jobKind": inv.Name}, nil
		}
	}
	return message, nil, nil
}

func (s *workerServer) diagnosis() map[string]any {
	id, ok := s.jobs.begin("diag")
	if ok {
		s.emitJob(jobEvent{Job: id, Kind: "diag", Stage: "scan", Level: "info", Message: "Đang chẩn đoán truyện"})
	}
	view := s.h.DiagnoseView()
	rep := view.Report
	out := diagResponse(rep)
	st := rep.Stats
	out["stats"] = map[string]any{"completedChapters": st.CompletedChapters, "totalChapters": st.TotalChapters, "totalWords": st.TotalWords, "avgWordsPerChapter": st.AvgWordsPerCh, "phase": st.Phase, "flow": st.Flow, "planningTier": st.PlanningTier, "reviewCount": st.ReviewCount, "rewriteCount": st.RewriteCount, "avgReviewScore": st.AvgReviewScore, "foreshadowOpen": st.ForeshadowOpen, "foreshadowStale": st.ForeshadowStale, "startedAt": view.StartedAt, "currentVolume": view.CurrentVolume, "totalVolumes": view.TotalVolumes}
	findings := []map[string]any{}
	actions := []map[string]any{}
	for _, f := range rep.Findings {
		findings = append(findings, map[string]any{"fingerprint": diag.FindingFingerprint(f), "rule": f.Rule, "category": f.Category, "severity": f.Severity, "confidence": f.Confidence, "autoLevel": f.AutoLevel, "target": f.Target, "title": f.Title, "evidence": f.Evidence, "suggestion": f.Suggestion, "technical": f.Technical})
	}
	s.diagMu.Lock()
	for _, a := range rep.Actions {
		actions = append(actions, map[string]any{"fingerprint": a.Fingerprint, "sourceRule": a.SourceRule, "kind": a.Kind, "severity": a.Severity, "summary": a.Summary, "message": a.Message, "sent": s.diagSent[a.Fingerprint]})
	}
	s.diagMu.Unlock()
	out["findings"], out["actions"] = findings, actions
	s.diagMu.Lock()
	s.diagLast = out
	s.diagMu.Unlock()
	if ok {
		s.finishJob(id, "diag", jobEvent{Stage: "done", Level: "success", Message: fmt.Sprintf("%d phát hiện · %d hành động đề xuất", len(findings), len(actions))})
	}
	return out
}

func (s *workerServer) sendDiagnosisAction(w http.ResponseWriter, r *http.Request, fingerprint string) {
	report := s.h.Diagnose()
	var message string
	for _, a := range report.Actions {
		if a.Fingerprint == fingerprint {
			message = a.Message
			if a.Kind == diag.ActionEnqueueFollowUp {
				break
			}
		}
	}
	if message == "" {
		for _, f := range report.Findings {
			if diag.FindingFingerprint(f) == fingerprint {
				message = f.Suggestion
				break
			}
		}
	}
	if message == "" {
		jsonError(w, 404, "Không tìm thấy hành động chẩn đoán")
		return
	}
	action := "continue"
	if s.h.Snapshot().IsRunning {
		action = "steer"
	}
	// diagMu guards only the sent set; the Arbiter call runs outside it.
	s.diagMu.Lock()
	if s.diagSent[fingerprint] {
		s.diagMu.Unlock()
		writeJSON(w, 200, map[string]any{"ok": true, "action": action})
		return
	}
	if s.diagSent == nil {
		s.diagSent = map[string]bool{}
	}
	s.diagSent[fingerprint] = true
	s.diagMu.Unlock()
	unmark := func() {
		s.diagMu.Lock()
		delete(s.diagSent, fingerprint)
		s.diagMu.Unlock()
	}
	pending, err := s.dispatchAction("Đang gửi hành động chẩn đoán", func() error {
		var err error
		if action == "steer" {
			err = s.h.Steer(message, r.Header.Get("X-Ainovel-Username"))
		} else {
			err = s.h.Continue(message, r.Header.Get("X-Ainovel-Username"))
		}
		if err != nil {
			unmark()
		}
		return err
	})
	if err != nil {
		unmark()
	}
	writeDispatched(w, pending, err, map[string]any{"ok": true, "action": action})
}

func (s *workerServer) exportDiagnosis(w http.ResponseWriter) {
	view := s.h.DiagnoseView()
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"chan-doan.md\"")
	_, _ = w.Write(view.Export)
}

func (s *workerServer) getSimulationProfile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("download") == "1" {
		path, err := s.h.SimulationProfileFile()
		if err != nil {
			jsonError(w, 404, "Chưa có hồ sơ văn phong")
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\"ho-so-van-phong.json\"")
		http.ServeFile(w, r, path)
		return
	}
	view, err := s.h.SimulationProfileView()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if view == nil {
		jsonError(w, 404, "Chưa có hồ sơ văn phong")
		return
	}
	writeJSON(w, 200, view)
}

func (s *workerServer) getLastDiagnosis(w http.ResponseWriter) {
	s.diagMu.Lock()
	if s.diagLast == nil {
		s.diagMu.Unlock()
		jsonError(w, 404, "Chưa có báo cáo chẩn đoán")
		return
	}
	last := make(map[string]any, len(s.diagLast))
	for key, value := range s.diagLast {
		last[key] = value
	}
	actions := []map[string]any{}
	for _, cached := range s.diagLast["actions"].([]map[string]any) {
		action := make(map[string]any, len(cached))
		for key, value := range cached {
			action[key] = value
		}
		fingerprint, _ := action["fingerprint"].(string)
		action["sent"] = s.diagSent[fingerprint]
		actions = append(actions, action)
	}
	last["actions"] = actions
	s.diagMu.Unlock()
	writeJSON(w, 200, last)
}

// Keep downloads rooted at the story; the gateway authorizes the story prefix.
func exportDownloadHref(path string) string { return "export/file?path=" + url.QueryEscape(path) }

func (s *workerServer) downloadExport(w http.ResponseWriter, r *http.Request) {
	path, err := safeExportPath(s.storyDir, r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, 400, "Đường dẫn không hợp lệ")
		return
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !within(resolved, s.storyDir) {
		jsonError(w, 404, "Không tìm thấy tệp")
		return
	}
	if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		jsonError(w, 404, "Không tìm thấy tệp")
		return
	}
	w.Header().Set("Content-Disposition", contentDisposition(filepath.Base(path)))
	http.ServeFile(w, r, resolved)
}
