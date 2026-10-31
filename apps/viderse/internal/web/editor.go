package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// EditorQualityProvider adapts the scorer without coupling editor storage to
// render workers or pipeline repositories.
type EditorQualityProvider interface {
	Quality(context.Context, string) (editor.Object, error)
}

type editorHTTP struct {
	service *editor.Service
	quality EditorQualityProvider
}

func MountEditor(g *Gateway, svc *editor.Service, quality EditorQualityProvider) {
	h := editorHTTP{svc, quality}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		g.Handle(method, "/api/editor/", func(w http.ResponseWriter, r *http.Request, _ identity.User) { h.serve(w, r) })
	}
}
func editorJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeEditorError(w http.ResponseWriter, err error) {
	status, code, message := 500, "internal_error", "Không xử lý được yêu cầu editor"
	var e *editor.Error
	extra := editor.Object{}
	if errors.As(err, &e) {
		code, message = e.Code, e.Message
		for key, value := range e.Extra {
			extra[key] = value
		}
	} else if errors.Is(err, editor.ErrConflict) {
		code, message = "revision_conflict", "Project đã được sửa ở nơi khác"
	} else if errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		code, message = "not_found", "Không tìm thấy dữ liệu editor"
	}
	switch code {
	case "not_found":
		status = 404
	case "conflict", "revision_conflict":
		status = 409
	case "invalid_request", "invalid_command":
		status = 400
	case "forbidden_origin":
		status = 403
	case "payload_too_large":
		status = 413
	case "insufficient_storage":
		status = 507
	case "unsupported_format":
		status = 415
	case "probe_failed":
		status = 422
	case "locked":
		status = 503
	}
	extra["code"] = code
	extra["error"] = message
	editorJSON(w, status, extra)
}
func editorBody(w http.ResponseWriter, r *http.Request) (editor.Object, error) {
	const limit = 64 << 20
	if r.ContentLength > limit {
		return nil, &editor.Error{Code: "payload_too_large", Message: "JSON body too large"}
	}
	var body editor.Object
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if err := decoder.Decode(&body); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			return nil, &editor.Error{Code: "payload_too_large", Message: "JSON body too large"}
		}
		return nil, &editor.Error{Code: "invalid_request", Message: "Request body is not valid JSON"}
	}
	if body == nil {
		return nil, &editor.Error{Code: "invalid_request", Message: "Request body must be a JSON object"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, &editor.Error{Code: "invalid_request", Message: "Request body must contain exactly one JSON value"}
	}
	return body, nil
}
func editorRevision(r *http.Request) string {
	return strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), "\"")
}
func editorField(body editor.Object, key string) string { s, _ := body[key].(string); return s }
func editorDecode(body editor.Object, out any) error {
	b, err := json.Marshal(body)
	if err == nil {
		err = json.Unmarshal(b, out)
	}
	if err != nil {
		return &editor.Error{Code: "invalid_request", Message: "Invalid request fields"}
	}
	return nil
}
func editorStream(w http.ResponseWriter, r *http.Request, h *media.Handle, mime string) error {
	defer h.Release()
	file, err := os.Open(h.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
	return nil
}
func (h editorHTTP) serve(w http.ResponseWriter, r *http.Request) {
	if err := h.route(w, r); err != nil {
		writeEditorError(w, err)
	}
}
func (h editorHTTP) route(w http.ResponseWriter, r *http.Request) error {
	path := strings.TrimPrefix(r.URL.Path, "/api/editor/")
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			http.NotFound(w, r)
			return nil
		}
	}
	ctx := r.Context()
	s := h.service
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	invalid := func(message string) error { return &editor.Error{Code: "invalid_request", Message: message} }
	if len(parts) == 1 && parts[0] == "projects" {
		switch method {
		case http.MethodGet:
			projects, err := s.ListProjects(ctx)
			if err != nil {
				return err
			}
			editorJSON(w, 200, editor.Object{"projects": projects})
			return nil
		case http.MethodPost:
			body, err := editorBody(w, r)
			if err != nil {
				return err
			}
			var input editor.CreateProjectInput
			if err = editorDecode(body, &input); err != nil {
				return err
			}
			p, revision, err := s.CreateProject(ctx, input)
			if err != nil {
				return err
			}
			editorJSON(w, 201, editor.Object{"project": p, "revision": revision})
			return nil
		}
	}
	if parts[0] == "workspace" {
		if len(parts) == 1 && method == http.MethodGet {
			id, err := s.WorkspaceID(ctx)
			if err != nil {
				return err
			}
			editorJSON(w, 200, editor.Object{"workspaceId": id})
			return nil
		}
		if len(parts) == 2 && parts[1] == "effect-presets" {
			switch method {
			case http.MethodGet:
				data, revision, err := s.LoadEffectPresets(ctx)
				if err != nil {
					return err
				}
				w.Header().Set("ETag", strconv.Quote(revision))
				editorJSON(w, 200, data)
				return nil
			case http.MethodPut:
				body, err := editorBody(w, r)
				if err != nil {
					return err
				}
				revision, err := s.SaveEffectPresets(ctx, body, editorRevision(r))
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"revision": revision})
				return nil
			}
		}
	}
	if parts[0] == "projects" && len(parts) >= 2 {
		id := parts[1]
		if err := editor.ValidateID(id); err != nil {
			return err
		}
		if len(parts) == 2 {
			switch method {
			case http.MethodGet:
				p, revision, err := s.LoadProject(ctx, id)
				if err != nil {
					return err
				}
				w.Header().Set("ETag", strconv.Quote(revision))
				editorJSON(w, 200, p)
				return nil
			case http.MethodPut:
				body, err := editorBody(w, r)
				if err != nil {
					return err
				}
				revision, err := s.SaveProject(ctx, id, editor.Project(body), editorRevision(r))
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"revision": revision})
				return nil
			case http.MethodPatch:
				body, err := editorBody(w, r)
				if err != nil {
					return err
				}
				revision, err := s.RenameProject(ctx, id, editorField(body, "name"), editorRevision(r))
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"revision": revision})
				return nil
			case http.MethodDelete:
				if err := s.DeleteProject(ctx, id); err != nil {
					return err
				}
				w.WriteHeader(204)
				return nil
			}
		}
		if len(parts) == 3 && parts[2] == "duplicate" && method == http.MethodPost {
			body, err := editorBody(w, r)
			if err != nil {
				body = editor.Object{}
			}
			p, revision, err := s.DuplicateProject(ctx, id, editorField(body, "name"))
			if err != nil {
				return err
			}
			editorJSON(w, 201, editor.Object{"project": p, "revision": revision})
			return nil
		}
		if len(parts) == 3 && parts[2] == "commands" && method == http.MethodPost {
			body, err := editorBody(w, r)
			if err != nil {
				return err
			}
			commands, err := editor.ParseCommands(body["commands"])
			if err != nil {
				return err
			}
			var opts editor.ApplyCommandsOptions
			if err = editorDecode(body, &opts); err != nil {
				return err
			}
			if opts.BaseRevision == "" {
				opts.BaseRevision = editorField(body, "revision")
			}
			if opts.BaseRevision == "" {
				opts.BaseRevision = editorRevision(r)
			}
			if opts.BaseRevision == "" {
				return invalid("revision is required")
			}
			result, err := s.RunCommands(ctx, id, commands, opts)
			if err != nil {
				return err
			}
			editorJSON(w, 200, result)
			return nil
		}
		if len(parts) == 4 && parts[2] == "sidecars" {
			name := parts[3]
			switch method {
			case http.MethodGet:
				data, revision, err := s.LoadSidecar(ctx, id, name)
				if err != nil {
					return err
				}
				w.Header().Set("ETag", strconv.Quote(revision))
				editorJSON(w, 200, data)
				return nil
			case http.MethodPut:
				body, err := editorBody(w, r)
				if err != nil {
					return err
				}
				revision, err := s.SaveSidecar(ctx, id, name, body, editorRevision(r))
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"revision": revision})
				return nil
			}
		}
		if len(parts) == 4 && parts[2] == "media" {
			var records []editor.MediaRecord
			var err error
			switch method {
			case http.MethodPut:
				records, err = s.LinkMedia(ctx, id, parts[3])
			case http.MethodDelete:
				records, err = s.UnlinkMedia(ctx, id, parts[3])
			default:
				http.NotFound(w, r)
				return nil
			}
			if err != nil {
				return err
			}
			editorJSON(w, 200, editor.Object{"media": records})
			return nil
		}
		if len(parts) == 3 && parts[2] == "exports" {
			switch method {
			case http.MethodGet:
				records, err := s.ListExports(ctx, id)
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"exports": records})
				return nil
			case http.MethodPost:
				body, err := editorBody(w, r)
				if err != nil {
					return err
				}
				var opts editor.BeginExportOptions
				if err = editorDecode(body, &opts); err != nil {
					return err
				}
				exportID, err := s.BeginExport(ctx, id, opts)
				if err != nil {
					return err
				}
				editorJSON(w, 201, editor.Object{"exportId": exportID})
				return nil
			}
		}
		if len(parts) == 4 && parts[2] == "exports" {
			eid := parts[3]
			if err := editor.ValidateID(eid); err != nil {
				return err
			}
			switch method {
			case http.MethodGet:
				handle, record, err := s.AcquireExport(ctx, id, eid)
				if err != nil {
					return err
				}
				w.Header().Set("Content-Disposition", `attachment; filename="`+url.PathEscape(record.FileName)+`"`)
				w.Header().Set("Cache-Control", "private, no-cache")
				return editorStream(w, r, handle, record.MIME)
			case http.MethodPut:
				var offset *int64
				if raw, ok := r.URL.Query()["offset"]; ok {
					if len(raw) != 1 {
						return invalid("Invalid chunk offset")
					}
					value, err := strconv.ParseInt(raw[0], 10, 64)
					if err != nil || value < 0 {
						return invalid("Invalid chunk offset")
					}
					offset = &value
				}
				if err := s.UploadExport(ctx, id, eid, r.Body, offset); err != nil {
					return err
				}
				w.WriteHeader(204)
				return nil
			case http.MethodPost:
				if !r.URL.Query().Has("commit") {
					return invalid("POST requires ?commit to finalize the export")
				}
				body := editor.Object{}
				if r.ContentLength != 0 {
					var err error
					body, err = editorBody(w, r)
					if err != nil {
						return err
					}
				}
				record, err := s.CommitExport(ctx, id, eid, body)
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"export": record})
				return nil
			case http.MethodDelete:
				if err := s.DeleteExport(ctx, id, eid); err != nil {
					return err
				}
				w.WriteHeader(204)
				return nil
			}
		}
	}
	if parts[0] == "media" {
		if len(parts) == 1 {
			switch method {
			case http.MethodGet:
				records, err := s.ListMedia(ctx, r.URL.Query().Get("projectId"))
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"media": records})
				return nil
			case http.MethodPost:
				record, err := s.ImportMedia(ctx, editor.ImportMediaOptions{ProjectID: r.URL.Query().Get("projectId"), FileName: r.URL.Query().Get("fileName"), DeclaredMIME: r.Header.Get("Content-Type"), Body: r.Body})
				if err != nil {
					return err
				}
				editorJSON(w, 201, editor.Object{"media": record})
				return nil
			}
		}
		if len(parts) == 2 && parts[1] == "link" && method == http.MethodPost {
			body, err := editorBody(w, r)
			if err != nil {
				return err
			}
			result, err := s.ImportMediaFromLink(ctx, body)
			if err != nil {
				return err
			}
			status := 201
			if reused, _ := result["reused"].(bool); reused {
				status = 200
			}
			editorJSON(w, status, result)
			return nil
		}
		if len(parts) >= 2 {
			id := parts[1]
			if err := editor.ValidateID(id); err != nil {
				return err
			}
			if len(parts) == 2 {
				switch method {
				case http.MethodGet:
					record, err := s.GetMediaMetadata(ctx, id)
					if err != nil {
						return err
					}
					handle, err := s.AcquireMedia(ctx, id)
					if err != nil {
						return err
					}
					w.Header().Set("ETag", strconv.Quote(record.ContentHash))
					if r.URL.Query().Get("v") == record.ContentHash {
						w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
					} else {
						w.Header().Set("Cache-Control", "private, no-cache")
					}
					return editorStream(w, r, handle, record.MIME)
				case http.MethodPut:
					record, err := s.ReplaceMedia(ctx, id, editor.ImportMediaOptions{FileName: r.URL.Query().Get("fileName"), DeclaredMIME: r.Header.Get("Content-Type"), Body: r.Body})
					if err != nil {
						return err
					}
					editorJSON(w, 200, editor.Object{"media": record})
					return nil
				case http.MethodDelete:
					if err := s.DeleteMedia(ctx, id); err != nil {
						return err
					}
					w.WriteHeader(204)
					return nil
				}
			}
			if len(parts) == 3 && parts[2] == "metadata" {
				switch method {
				case http.MethodGet:
					record, err := s.GetMediaMetadata(ctx, id)
					if err != nil {
						return err
					}
					editorJSON(w, 200, record)
					return nil
				case http.MethodPatch:
					body, err := editorBody(w, r)
					if err != nil {
						return err
					}
					record, err := s.UpdateMediaMetadata(ctx, id, body)
					if err != nil {
						return err
					}
					editorJSON(w, 200, editor.Object{"media": record})
					return nil
				}
			}
			if len(parts) == 3 && parts[2] == "background" && method == http.MethodPost {
				var overlay *bool
				if r.ContentLength != 0 {
					body, err := editorBody(w, r)
					if err != nil {
						return err
					}
					if value, exists := body["overlay"]; exists {
						flag, ok := value.(bool)
						if !ok {
							return invalid("overlay must be a boolean")
						}
						overlay = &flag
					}
				}
				record, err := s.DetectMediaBackground(ctx, id, r.URL.Query().Get("refresh") == "1", overlay)
				if err != nil {
					return err
				}
				editorJSON(w, 200, editor.Object{"media": record})
				return nil
			}
			if len(parts) == 3 && parts[2] == "waveform" && method == http.MethodPost {
				result, err := s.MediaWaveform(ctx, id)
				if err != nil {
					return err
				}
				editorJSON(w, 200, result)
				return nil
			}
		}
	}
	if parts[0] == "overlays" {
		if len(parts) == 1 && method == http.MethodGet {
			records, err := s.ListOverlays(ctx)
			if err != nil {
				return err
			}
			editorJSON(w, 200, editor.Object{"media": records})
			return nil
		}
		if len(parts) == 2 && method == http.MethodDelete {
			records, err := s.RemoveFromOverlayLibrary(ctx, parts[1])
			if err != nil {
				return err
			}
			editorJSON(w, 200, editor.Object{"media": records})
			return nil
		}
	}
	// V13 endpoints are intentionally outside /projects, as published by design.
	if len(parts) == 2 && parts[1] == "quality" && method == http.MethodGet {
		if err := editor.ValidateID(parts[0]); err != nil {
			return err
		}
		if h.quality == nil {
			return fmt.Errorf("quality provider unavailable")
		}
		result, err := h.quality.Quality(ctx, parts[0])
		if err != nil {
			return err
		}
		editorJSON(w, 200, result)
		return nil
	}
	if len(parts) == 3 && parts[1] == "captions" && parts[2] == "align" && method == http.MethodPost {
		body, err := editorBody(w, r)
		if err != nil {
			return err
		}
		var input editor.AlignCaptionsInput
		if err = editorDecode(body, &input); err != nil {
			return err
		}
		if input.Revision == "" {
			input.Revision = editorRevision(r)
		}
		result, err := s.AlignCaptions(ctx, parts[0], input)
		if err != nil {
			return err
		}
		editorJSON(w, 200, result)
		return nil
	}
	http.NotFound(w, r)
	return nil
}
