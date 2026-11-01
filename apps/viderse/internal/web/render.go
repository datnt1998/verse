package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/platform/webapp"
)

// ServeHTTP has one deliberately narrow non-session path. A render credential
// never falls through to ordinary editor/account APIs and never creates a user.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.scopedRender != nil && (strings.HasPrefix(r.URL.Path, "/api/render/") || strings.HasPrefix(r.URL.Path, "/api/measure/")) {
		if !g.RequireSameOrigin(w, r) {
			return
		}
		g.scopedRender.ServeHTTP(w, r)
		return
	}
	g.Server.ServeHTTP(w, r)
}

type renderHTTP struct {
	service *render.Service
	ctx     context.Context
}

func MountRender(g *Gateway, s *render.Service, ctx context.Context) {
	h := renderHTTP{s, ctx}
	g.scopedRender = h
	g.renderCloneRoute = h.cloneRoute
}
func renderHTTPError(w http.ResponseWriter, err error) {
	var req *render.RequestError
	var value render.RequestError
	var clone *domain.CloneError
	if errors.As(err, &value) {
		req = &value
	}
	switch {
	case req != nil || errors.As(err, &req):
		status := 400
		if req.Code == "not_found" {
			status = 404
		}
		if req.Code == "conflict" {
			status = 409
		}
		if req.Code == "unauthorized" {
			status = 401
		}
		webapp.WriteJSON(w, status, editor.Object{"error": req.Message, "code": req.Code})
	case errors.As(err, &clone):
		WriteCloneError(w, clone)
	default:
		writeEditorError(w, err)
	}
}
func (h renderHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := h.route(w, r); err != nil {
		renderHTTPError(w, err)
	}
}
func (h renderHTTP) route(w http.ResponseWriter, r *http.Request) error {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
	if len(parts) < 2 {
		return &render.RequestError{Code: "not_found", Message: "Không tìm thấy yêu cầu"}
	}
	kind, id := parts[0], parts[1]
	if kind != "render" && kind != "measure" {
		return &render.RequestError{Code: "unauthorized", Message: "Token không được dùng cho đường dẫn này"}
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	pid, err := h.service.Store.CheckToken(id, token, kind)
	if err != nil {
		webapp.Error(w, 401, "Token xuất hoặc đo không hợp lệ hoặc đã hết hạn")
		return nil
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return &render.RequestError{Code: "invalid_request", Message: "Đường dẫn không hợp lệ"}
		}
	}
	ctx := r.Context()
	action := ""
	if len(parts) > 2 {
		action = parts[2]
	}
	jobID := ""
	if kind == "render" {
		request, e := h.service.Store.GetExport(id)
		if e != nil {
			return e
		}
		jobID = request.JobID
	} else {
		request, e := h.service.Store.GetMeasurement(id)
		if e != nil {
			return e
		}
		jobID = request.JobID
	}
	if action == "media" && len(parts) == 4 && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		p, _, e := h.service.Editor.LoadProject(ctx, pid)
		if e != nil {
			return e
		}
		records, e := h.projectMedia(ctx, pid, p)
		if e != nil {
			return e
		}
		var record *editor.MediaRecord
		for i := range records {
			if records[i].ID == parts[3] {
				record = &records[i]
				break
			}
		}
		if record == nil {
			return &render.RequestError{Code: "unauthorized", Message: "Token không được đọc media ngoài project"}
		}
		file, e := h.service.Editor.AcquireMedia(ctx, parts[3])
		if e != nil {
			return e
		}
		return editorStream(w, r, file, record.MIME)
	}
	if r.Method == http.MethodGet && (action == "" || action == "runtime") {
		payload, e := h.snapshot(ctx, kind, id, pid, jobID, token)
		if e != nil {
			return e
		}
		webapp.WriteJSON(w, 200, payload)
		return nil
	}
	if action == "chunks" && kind == "measure" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		q := r.URL.Query()
		seq, e := strconv.ParseInt(q.Get("seq"), 10, 64)
		if e != nil {
			return &render.RequestError{Code: "invalid_request", Message: "seq không hợp lệ"}
		}
		rate, e := strconv.Atoi(q.Get("sampleRate"))
		if e != nil {
			return e
		}
		channels, e := strconv.Atoi(q.Get("channels"))
		if e != nil {
			return e
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
		if e != nil {
			return e
		}
		ack, e := h.service.Store.AppendPCM(id, r.Header.Get("X-Render-Fence"), seq, body, rate, channels)
		if e != nil {
			return e
		}
		webapp.WriteJSON(w, 200, ack)
		return nil
	}
	if action == "exports" && kind == "render" {
		return h.exportStorage(w, r, parts, id, pid)
	}
	if r.Method != http.MethodPost {
		return &render.RequestError{Code: "invalid_request", Message: "Phương thức không hợp lệ"}
	}
	body, err := editorBody(w, r)
	if err != nil {
		return err
	}
	fence := editorField(body, "fence")
	if kind == "render" {
		var result render.ExportRequest
		switch action {
		case "claim":
			result, err = h.service.Store.ClaimExport(id, editorField(body, "nonce"))
		case "heartbeat":
			result, err = h.service.Store.HeartbeatExport(id, fence)
		case "progress":
			var p render.ExportProgress
			raw, _ := json.Marshal(body["progress"])
			if err = json.Unmarshal(raw, &p); err == nil {
				result, err = h.service.Store.ProgressExport(id, fence, p)
			}
		case "bind":
			result, err = h.service.Store.BindExport(id, fence, editorField(body, "exportId"))
		case "complete":
			current, e := h.service.Store.GetExport(id)
			if e != nil {
				return e
			}
			if current.ExportID == "" || current.ExportID != editorField(body, "exportId") {
				return &render.RequestError{Code: "conflict", Message: "Export không thuộc yêu cầu"}
			}
			file, _, e := h.service.Editor.AcquireExport(ctx, pid, current.ExportID)
			if e != nil {
				return e
			}
			file.Release()
			result, err = h.service.Store.CompleteExport(id, fence, current.ExportID)
		case "fail":
			var failure render.RequestError
			if e := editorDecode(asObject(body["error"]), &failure); e != nil {
				return e
			}
			result, err = h.service.Store.FailExport(id, fence, failure)
		default:
			return &render.RequestError{Code: "not_found", Message: "Không tìm thấy thao tác xuất"}
		}
		if err != nil {
			return err
		}
		webapp.WriteJSON(w, 200, editor.Object{"request": publicRender(result)})
		return nil
	}
	var result render.MeasurementRequest
	switch action {
	case "claim":
		binding, e := h.service.CurrentBinding(ctx, jobID)
		if e != nil {
			return e
		}
		result, err = h.service.Store.ClaimMeasurement(id, editorField(body, "nonce"), editorField(body, "ownerId"), binding)
	case "heartbeat", "progress":
		result, err = h.service.Store.HeartbeatMeasurement(id, fence)
	case "complete":
		var score any
		result, score, err = h.service.CompleteMeasurement(ctx, id, fence)
		if err != nil {
			return err
		}
		webapp.WriteJSON(w, 200, editor.Object{"request": publicMeasure(result), "score": score})
		return nil
	case "fail":
		var failure render.RequestError
		if e := editorDecode(asObject(body["error"]), &failure); e != nil {
			return e
		}
		result, err = h.service.Store.FailMeasurement(id, fence, failure)
	default:
		return &render.RequestError{Code: "not_found", Message: "Không tìm thấy thao tác đo"}
	}
	if err != nil {
		return err
	}
	webapp.WriteJSON(w, 200, editor.Object{"request": publicMeasure(result)})
	return nil
}
func asObject(value any) editor.Object { m, _ := value.(map[string]any); return m }
func publicRender(r render.ExportRequest) editor.Object {
	raw, _ := json.Marshal(r)
	var out editor.Object
	_ = json.Unmarshal(raw, &out)
	delete(out, "token")
	delete(out, "chrome")
	return out
}
func publicMeasure(r render.MeasurementRequest) editor.Object {
	raw, _ := json.Marshal(r)
	var out editor.Object
	_ = json.Unmarshal(raw, &out)
	delete(out, "token")
	delete(out, "chrome")
	return out
}
func (h renderHTTP) snapshot(ctx context.Context, kind, id, pid, jobID, token string) (editor.Object, error) {
	p, revision, err := h.service.Editor.LoadProject(ctx, pid)
	if err != nil {
		return nil, err
	}
	// Scope is the project document itself: every mediaId it references plus
	// its mediaItems entries. Build-imported IDs are `media-<hex>`, which the
	// GC's UUID-shaped token scan cannot see, so ProjectMediaIDs is not usable
	// here (it would deny this project's own media — observed 401/404).
	mediaRecords, err := h.projectMedia(ctx, pid, p)
	if err != nil {
		return nil, err
	}
	records := []editor.Object{}
	urls := map[string]string{}
	for _, record := range mediaRecords {
		mid := record.ID
		raw, _ := json.Marshal(record)
		var m editor.Object
		_ = json.Unmarshal(raw, &m)
		u := "/api/" + kind + "/" + id + "/media/" + url.PathEscape(mid) + "?token=" + url.QueryEscape(token)
		m["src"] = u
		records = append(records, m)
		urls[mid] = u
	}

	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if mid, ok := v["mediaId"].(string); ok {
				if u := urls[mid]; u != "" {
					if _, ok := v["src"]; ok {
						v["src"] = u
					}
					if _, ok := v["audioSrc"]; ok {
						v["audioSrc"] = u
					}
				}
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(p.Timeline())
	workspace, err := h.service.Editor.WorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	out := editor.Object{"project": p, "revision": revision, "media": records, "runtime": editor.Object{"workspaceId": workspace, "ready": true, "capabilities": editor.Object{"audioDecode": true, "gpuRequired": kind == "render"}}, "clone": nil}
	if kind == "render" {
		r, e := h.service.Store.GetExport(id)
		if e != nil {
			return nil, e
		}
		out["request"] = publicRender(r)
	} else {
		r, e := h.service.Store.GetMeasurement(id)
		if e != nil {
			return nil, e
		}
		out["request"] = publicMeasure(r)
	}
	if jobID != "" {
		job, _, _, _, e := h.service.Inputs(ctx, jobID)
		if e != nil {
			return nil, e
		}
		gate, e := h.service.Gate(ctx, jobID)
		if e != nil {
			return nil, e
		}
		blockers := []any{}
		if gate.Score != nil {
			for _, b := range gate.Score.Blockers {
				blockers = append(blockers, b)
			}
		}
		out["clone"] = editor.Object{"jobId": jobID, "generation": job.Generation, "score": gate.Score, "gate": editor.Object{"allowed": gate.Allowed, "blockers": blockers}, "latestRequest": editor.Object{"id": id, "status": asObject(out["request"])["status"]}}
	}
	return out, nil
}

func (h renderHTTP) projectMedia(ctx context.Context, pid string, p editor.Project) ([]editor.MediaRecord, error) {
	records, err := h.service.Editor.ListMedia(ctx, pid)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(records))
	for _, record := range records {
		seen[record.ID] = true
	}
	for _, id := range scopedMediaIDs(p) {
		if seen[id] {
			continue
		}
		record, err := h.service.Editor.GetMediaMetadata(ctx, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
		seen[id] = true
	}
	return records, nil
}

func scopedMediaIDs(p editor.Project) []string {
	set := map[string]bool{}
	if items, ok := p["mediaItems"].([]any); ok {
		for _, v := range items {
			if m, ok := v.(map[string]any); ok {
				if id, ok := m["id"].(string); ok && editor.ValidateID(id) == nil {
					set[id] = true
				}
			}
		}
	}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if id, ok := v["mediaId"].(string); ok && editor.ValidateID(id) == nil {
				set[id] = true
			}
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(p.Timeline())
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (h renderHTTP) exportStorage(w http.ResponseWriter, r *http.Request, parts []string, id, pid string) error {
	ctx := r.Context()
	request, err := h.service.Store.GetExport(id)
	if err != nil {
		return err
	}
	fence := r.Header.Get("X-Render-Fence")
	var body editor.Object
	if r.Method == http.MethodPost {
		body, err = editorBody(w, r)
		if err != nil {
			return err
		}
		fence = editorField(body, "fence")
	}
	if request.Fence == "" || fence != request.Fence || request.Terminal() {
		return &render.RequestError{Code: "conflict", Message: "Fence xuất không khớp"}
	}
	if len(parts) == 3 && r.Method == http.MethodPost {
		var opts editor.BeginExportOptions
		if err = editorDecode(asObject(body["options"]), &opts); err != nil {
			return err
		}
		opts.RequestID = id
		eid, e := h.service.Editor.BeginExport(ctx, pid, opts)
		if e != nil {
			return e
		}
		if _, e = h.service.Store.BindExport(id, fence, eid); e != nil {
			return e
		}
		webapp.WriteJSON(w, 201, editor.Object{"exportId": eid})
		return nil
	}
	if len(parts) < 4 || request.ExportID == "" || parts[3] != request.ExportID {
		return &render.RequestError{Code: "unauthorized", Message: "Token không được sửa export khác"}
	}
	eid := parts[3]
	if len(parts) == 4 && r.Method == http.MethodPut {
		var offset *int64
		if value := r.URL.Query().Get("offset"); value != "" {
			n, e := strconv.ParseInt(value, 10, 64)
			if e != nil || n < 0 {
				return &render.RequestError{Code: "invalid_request", Message: "Offset không hợp lệ"}
			}
			offset = &n
		}
		if err = h.service.Editor.UploadExport(ctx, pid, eid, http.MaxBytesReader(w, r.Body, 64<<20), offset); err != nil {
			return err
		}
		webapp.WriteJSON(w, 200, editor.Object{"ok": true})
		return nil
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		if err = h.service.Editor.DeleteExport(ctx, pid, eid); err != nil {
			return err
		}
		webapp.WriteJSON(w, 200, editor.Object{"ok": true})
		return nil
	}
	if len(parts) == 5 && parts[4] == "commit" && r.Method == http.MethodPost {
		record, e := h.service.Editor.CommitExport(ctx, pid, eid, asObject(body["metadata"]))
		if e != nil {
			return e
		}
		webapp.WriteJSON(w, 200, editor.Object{"export": record})
		return nil
	}
	return &render.RequestError{Code: "not_found", Message: "Không tìm thấy thao tác export"}
}
func (h renderHTTP) cloneRoute(w http.ResponseWriter, r *http.Request) bool {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/clone/"), "/")
	if len(parts) < 2 {
		return false
	}
	jobID, action := parts[0], parts[1]
	if action != "measure" && action != "export" && action != "measurements" && action != "exports" && action != "quality" {
		return false
	}
	if !cloneSegment(jobID) {
		invalidClone(w, "Mã video không hợp lệ")
		return true
	}
	if !h.clone(w, r, jobID, action, parts) {
		cloneNotFound(w)
	}
	return true
}
func (h renderHTTP) clone(w http.ResponseWriter, r *http.Request, jobID, action string, parts []string) bool {
	ctx := r.Context()
	s := h.service
	if action == "quality" && len(parts) == 2 && r.Method == http.MethodPost {
		body, err := editorBody(w, r)
		if err != nil {
			renderHTTPError(w, err)
			return true
		}
		accept, ok := body["accept"].(bool)
		if !ok {
			webapp.Error(w, 400, "accept phải là boolean")
			return true
		}
		job, e := s.Jobs.GetJob(ctx, jobID)
		if e != nil && !errors.Is(e, pipeline.ErrNotFound) {
			renderHTTPError(w, e)
			return true
		}
		if job == nil {
			WriteCloneError(w, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy video này."})
			return true
		}
		if e = s.Accept(jobID, accept); e != nil {
			renderHTTPError(w, e)
			return true
		}
		webapp.WriteJSON(w, 200, editor.Object{"accepted": accept})
		return true
	}
	if (action == "measure" || action == "export") && len(parts) == 2 && r.Method == http.MethodPost {
		body, err := editorBody(w, r)
		if err != nil {
			renderHTTPError(w, err)
			return true
		}
		if action == "measure" {
			_, _, pid, _, e := s.Inputs(ctx, jobID)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			binding, e := s.CurrentBinding(ctx, jobID)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			request, e := s.Store.CreateMeasurement(jobID, pid, binding)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			go func() { _, _ = s.Supervisor.LaunchMeasurement(h.ctx, request) }()
			webapp.WriteJSON(w, 202, editor.Object{"request": publicMeasure(request), "requestId": request.ID, "stateUrl": "/api/clone/" + jobID + "/measurements/" + request.ID})
			return true
		}
		if raw, ok := body["accept"]; ok {
			if _, valid := raw.(bool); !valid {
				webapp.Error(w, 400, "accept phải là boolean")
				return true
			}
		}
		accept, _ := body["accept"].(bool)
		request, e := s.BeginCloneExport(ctx, jobID, accept)
		if e != nil {
			renderHTTPError(w, e)
			return true
		}
		webapp.WriteJSON(w, 202, editor.Object{"request": publicRender(request), "requestId": request.ID, "stateUrl": "/api/clone/" + jobID + "/exports/" + request.ID})
		return true
	}
	if len(parts) == 3 && (r.Method == http.MethodGet || r.Method == http.MethodDelete) {
		id := parts[2]
		if action == "measurements" {
			request, e := s.Store.GetMeasurement(id)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			if request.JobID != jobID {
				cloneNotFound(w)
				return true
			}
			if r.Method == http.MethodDelete {
				request, e = s.Store.CancelMeasurement(id, request.Nonce)
				if e != nil {
					renderHTTPError(w, e)
					return true
				}
			}
			state, e := s.State(jobID)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			gate, e := s.Gate(ctx, jobID)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			webapp.WriteJSON(w, 200, editor.Object{"request": publicMeasure(request), "score": state.Score, "gate": gate})
			return true
		}
		if action == "exports" {
			request, e := s.Store.GetExport(id)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			if request.JobID != jobID {
				cloneNotFound(w)
				return true
			}
			if r.Method == http.MethodDelete {
				request, e = s.Store.CancelExport(id, request.Nonce)
				if e != nil {
					renderHTTPError(w, e)
					return true
				}
			}
			request, e = s.ExportState(ctx, request)
			if e != nil {
				renderHTTPError(w, e)
				return true
			}
			payload := editor.Object{"request": publicRender(request)}
			if request.Status == "done" {
				file, record, e := s.Editor.AcquireExport(ctx, request.ProjectID, request.ExportID)
				if e != nil {
					renderHTTPError(w, e)
					return true
				}
				file.Release()
				payload["export"] = record
			}
			webapp.WriteJSON(w, 200, payload)
			return true
		}
	}
	return false
}
