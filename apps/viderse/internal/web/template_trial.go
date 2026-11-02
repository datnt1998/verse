package web

// HTTP của "Vẽ thử ảnh" (Template.dc.html › lib^cover; Library.dc.html ›
// view=editor ảnh mẫu). Mỗi lần vẽ là MỘT ảnh thật tốn một lượt hạn mức; client
// phải gửi confirm:true. Trạng thái chạy nền trả về để UI đóng trang vẫn xong;
// huỷ/thử lại chỉ khi người dùng bấm. Ảnh bền phục vụ qua media.Service.

import (
	"context"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/webapp"
)

// coverTrialWire là một hàng lịch sử + URL ảnh (rỗng khi chưa xong).
type coverTrialWire struct {
	library.CoverTrial
	ImageURL string `json:"imageUrl,omitempty"`
}

type promptSampleWire struct {
	library.PromptSample
	ImageURL string `json:"imageUrl,omitempty"`
}

type coverTrialStatusWire struct {
	Sources       []library.PreviewSource `json:"sources"`
	History       []coverTrialWire        `json:"history"`
	Task          *library.PreviewTask    `json:"task,omitempty"`
	Quota         *library.TrialQuota     `json:"quota,omitempty"`
	Cost          library.TrialCost       `json:"cost"`
	Prompt        *library.TrialPrompt    `json:"prompt,omitempty"`
	Ready         bool                    `json:"ready"`
	BlockedReason string                  `json:"blockedReason,omitempty"`
}

type promptSampleStatusWire struct {
	Sources       []library.PreviewSource `json:"sources"`
	History       []promptSampleWire      `json:"history"`
	Task          *library.PreviewTask    `json:"task,omitempty"`
	Quota         *library.TrialQuota     `json:"quota,omitempty"`
	Cost          library.TrialCost       `json:"cost"`
	Prompt        *library.TrialPrompt    `json:"prompt,omitempty"`
	Ready         bool                    `json:"ready"`
	BlockedReason string                  `json:"blockedReason,omitempty"`
}

func coverTrialStatusToWire(s *library.CoverTrialStatus, templateID string) coverTrialStatusWire {
	history := make([]coverTrialWire, 0, len(s.History))
	for _, row := range s.History {
		item := coverTrialWire{CoverTrial: row}
		if row.State == library.TrialReady {
			item.ImageURL = "/api/templates/" + templateID + "/cover-trials/" + row.ID + "/image"
		}
		history = append(history, item)
	}
	return coverTrialStatusWire{
		Sources: s.Sources, History: history, Task: s.Task, Quota: s.Quota, Cost: s.Cost,
		Prompt: s.Prompt, Ready: s.Ready, BlockedReason: s.Blocked,
	}
}

func promptSampleStatusToWire(s *library.PromptSampleStatus, promptID string) promptSampleStatusWire {
	history := make([]promptSampleWire, 0, len(s.History))
	for _, row := range s.History {
		item := promptSampleWire{PromptSample: row}
		if row.State == library.TrialReady {
			item.ImageURL = "/api/image-prompts/" + promptID + "/samples/" + row.ID + "/image"
		}
		history = append(history, item)
	}
	return promptSampleStatusWire{
		Sources: s.Sources, History: history, Task: s.Task, Quota: s.Quota, Cost: s.Cost,
		Prompt: s.Prompt, Ready: s.Ready, BlockedReason: s.Blocked,
	}
}

// routeTemplateTests xử lý /api/templates/{id}/cover-trials[/{trial}[/cancel|/image]].
// Trả false khi path không thuộc nhóm này để routeTemplates xử lý tiếp.
func routeTemplateTrials(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag, rest string) bool {
	segs := strings.Split(strings.Trim(rest, "/"), "/")
	if len(segs) < 2 || segs[1] != "cover-trials" {
		return false
	}
	templateID := segs[0]
	if templateID == "" || strings.Contains(templateID, "/") {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return true
	}
	ctx := r.Context()
	switch {
	case len(segs) == 2 && r.Method == http.MethodGet:
		status, err := svc.CoverTrialStatus(ctx, templateID)
		if err != nil {
			writeLibraryError(w, err, diag)
			return true
		}
		webapp.WriteJSON(w, http.StatusOK, coverTrialStatusToWire(status, templateID))
	case len(segs) == 2 && r.Method == http.MethodPost:
		var req library.CoverTrialRequest
		if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
			return true
		}
		status, err := svc.StartCoverTrial(ctx, templateID, req)
		if err != nil {
			writeLibraryError(w, err, diag)
			return true
		}
		webapp.WriteJSON(w, http.StatusAccepted, coverTrialStatusToWire(status, templateID))
	case len(segs) == 4 && segs[3] == "cancel" && r.Method == http.MethodPost:
		if err := svc.CancelCoverTrial(ctx, templateID, segs[2]); err != nil {
			writeLibraryError(w, err, diag)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case len(segs) == 4 && segs[3] == "image" && r.Method == http.MethodGet:
		serveTrialImage(w, r, svc, diag, func() (string, error) {
			return svc.CoverTrialImagePath(ctx, templateID, segs[2])
		})
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
	return true
}

// routePromptSamples xử lý /api/image-prompts/{id}/samples[/scenes|/{sample}[/image|/cancel]].
func routePromptSamples(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag, promptID, tail string) {
	ctx := r.Context()
	segs := strings.Split(strings.Trim(tail, "/"), "/")
	switch {
	case len(segs) == 1 && segs[0] == "samples" && r.Method == http.MethodGet:
		status, err := svc.PromptSampleStatus(ctx, promptID)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, promptSampleStatusToWire(status, promptID))
	case len(segs) == 1 && segs[0] == "samples" && r.Method == http.MethodPost:
		var req library.PromptSampleRequest
		if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
			return
		}
		status, err := svc.StartPromptSample(ctx, promptID, req)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusAccepted, promptSampleStatusToWire(status, promptID))
	case len(segs) == 2 && segs[0] == "samples" && segs[1] == "scenes" && r.Method == http.MethodGet:
		scenes, err := svc.PromptScenes(ctx, promptID, r.URL.Query().Get("videoId"))
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"items": scenes})
	case len(segs) == 3 && segs[0] == "samples" && segs[2] == "cancel" && r.Method == http.MethodPost:
		if err := svc.CancelPromptSample(ctx, promptID, segs[1]); err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case len(segs) == 3 && segs[0] == "samples" && segs[2] == "image" && r.Method == http.MethodGet:
		serveTrialImage(w, r, svc, diag, func() (string, error) {
			return svc.PromptSampleImagePath(ctx, promptID, segs[1])
		})
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveTrialImage phục vụ ảnh thử đã lưu bền qua media.Service.
func serveTrialImage(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag, path func() (string, error)) {
	mediaSvc, ok := svc.(interface{ Media() *media.Service })
	if !ok || mediaSvc.Media() == nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	pathValue, err := path()
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	if err := mediaSvc.Media().ServeHTTP(w, r, pathValue); err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

// promptWireFillSamples điền media id ảnh mẫu cho wire ImagePrompt (giữ hình
// JSON samples: []string của api/library.ts, tối đa 12, mới nhất trước).
func promptWireFillSamples(ctx context.Context, svc LibraryService, items []domain.ImagePrompt, usage map[string][]library.TemplateRef) []ImagePromptWire {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	samples := svc.PromptSampleIDs(ctx, ids)
	out := make([]ImagePromptWire, 0, len(items))
	for _, item := range items {
		wire := ImagePromptToWire(item, usage)
		if list := samples[item.ID]; len(list) > 0 {
			wire.Samples = list
		}
		out = append(out, wire)
	}
	return out
}
