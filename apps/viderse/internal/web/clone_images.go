package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

type CloneImageOperations interface {
	Read(context.Context, string) (cloneimages.View, error)
	Review(context.Context, string, cloneimages.ReviewInput) (cloneimages.ReviewResult, error)
	RedrawAll(context.Context, string, cloneimages.ResetInput) (cloneimages.GenerateResult, error)
	ApproveAll(context.Context, string, cloneimages.RevisionInput) (cloneimages.MutationResult, error)
	PlanOverrides(context.Context, string, cloneimages.OverrideInput) (cloneimages.OverridePlan, error)
	ApplyOverrides(context.Context, string, cloneimages.OverrideInput) (cloneimages.OverridePlan, error)
	UploadCover(context.Context, string, string, cloneimages.RevisionInput) (cloneimages.CoverResult, error)
	DisplayedCutout(context.Context, string, string) (string, error)
	RedrawVersions(context.Context, string, cloneimages.RedrawVersionsInput) (cloneimages.RedrawVersionsResult, error)
	UseVersion(context.Context, string, string, cloneimages.UseVersionInput) (cloneimages.UseVersionResult, error)
	CancelVersion(context.Context, string, string, int) (cloneimages.CancelVersionResult, error)
}
type CloneBackdropOperations interface {
	Read(context.Context, string) (*clonebackdrop.EvidenceView, error)
	Edit(context.Context, string, clonebackdrop.EditRequest) (clonebackdrop.Result, error)
}
type CloneOperations struct {
	Images    CloneImageOperations
	Backdrops CloneBackdropOperations
	Metadata  *clonemetadata.Service
	Edits     *jobedit.Service
	Bundles   *CloneBundles
	Bulk      *CloneBulk
}

// MountCloneOperations extends the existing authenticated clone dispatcher.
// It is independent of route-registration order relative to MountClone.
func MountCloneOperations(g *Gateway, ops CloneOperations) {
	g.cloneReviewRoute = func(w http.ResponseWriter, r *http.Request) bool { return ops.route(g, w, r) }
	g.Handle(http.MethodPatch, "/api/clone/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		if !ops.route(g, w, r) {
			cloneNotFound(w)
		}
	})
}
func (ops CloneOperations) route(g *Gateway, w http.ResponseWriter, r *http.Request) bool {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/clone/"), "/")
	if !strings.HasPrefix(r.URL.Path, "/api/clone/") {
		return false
	}
	if len(parts) == 1 && parts[0] == "bulk" && r.Method == http.MethodPost {
		ops.bulk(w, r)
		return true
	}
	if len(parts) == 1 && parts[0] == "download" && r.Method == http.MethodGet {
		if !g.RequireSameOrigin(w, r) {
			return true
		}
		ops.download(w, r)
		return true
	}
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "images", "backdrop", "cover", "thumbnail", "prompts", "metadata":
	default:
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	if !cloneSegment(parts[0]) {
		invalidClone(w, "Mã video không hợp lệ.")
		return true
	}
	if r.Method == http.MethodGet && !g.RequireSameOrigin(w, r) {
		return true
	}
	id := parts[0]
	switch parts[1] {
	case "metadata":
		ops.metadata(w, r, id, parts[2:])
	case "images":
		if !cloneAvailable(w, ops.Images != nil) {
			return true
		}
		ops.images(w, r, id, parts[2:])
	case "backdrop":
		if len(parts) != 2 {
			cloneNotFound(w)
			break
		}
		if !cloneAvailable(w, ops.Backdrops != nil) {
			return true
		}
		switch r.Method {
		case http.MethodGet:
			view, err := ops.Backdrops.Read(r.Context(), id)
			if err != nil {
				WriteCloneError(w, err)
				break
			}
			webapp.WriteJSON(w, 200, map[string]any{"backdrop": view})
		case http.MethodPost:
			var input clonebackdrop.EditRequest
			if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
				break
			}
			if !validBackdropEdit(input) {
				invalidClone(w, "Thao tác nền không hợp lệ.")
				break
			}
			out, err := ops.Backdrops.Edit(r.Context(), id, input)
			cloneOperationJSON(w, out, err)
		default:
			cloneNotFound(w)
		}
	case "cover":
		if len(parts) != 2 || r.Method != http.MethodPost {
			cloneNotFound(w)
			break
		}
		if cloneAvailable(w, ops.Images != nil) {
			ops.cover(w, r, id)
		}
	case "thumbnail":
		if len(parts) != 2 || r.Method != http.MethodGet {
			cloneNotFound(w)
			break
		}
		ops.thumbnail(w, r, id)
	case "prompts":
		if !cloneAvailable(w, ops.Images != nil) {
			break
		}
		var input cloneimages.OverrideInput
		if len(parts) == 3 && parts[2] == "plan" && r.Method == http.MethodPost {
			if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
				break
			}
			result, err := ops.Images.PlanOverrides(r.Context(), id, input)
			if err != nil {
				WriteCloneError(w, err)
				break
			}
			webapp.WriteJSON(w, 200, result)
		} else if len(parts) == 2 && r.Method == http.MethodPatch {
			if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
				break
			}
			result, err := ops.Images.ApplyOverrides(r.Context(), id, input)
			cloneOperationJSON(w, result, err)
		} else {
			cloneNotFound(w)
		}
	}
	return true
}
func cloneOperationJSON(w http.ResponseWriter, result any, err error) {
	cloneOperationJSONStatus(w, http.StatusAccepted, result, err)
}
func cloneOperationJSONStatus(w http.ResponseWriter, status int, result any, err error) {
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	webapp.WriteJSON(w, status, result)
}
func validBackdropEdit(input clonebackdrop.EditRequest) bool {
	if input.ItemID != "" && !cloneSegment(input.ItemID) {
		return false
	}
	if input.ChannelID != "" && !cloneSegment(input.ChannelID) || input.VideoID != "" && !cloneSegment(input.VideoID) {
		return false
	}
	switch input.Op {
	case "reroll":
		return input.ChannelID == "" && input.VideoID == "" && input.StartSec == nil && input.EndSec == nil
	case "remove":
		return input.ItemID != "" && input.ChannelID == "" && input.VideoID == "" && input.StartSec == nil && input.EndSec == nil
	case "swap":
		return input.ItemID != "" && input.ChannelID != "" && input.VideoID != ""
	case "add":
		return input.ItemID == "" && (input.ChannelID == "") == (input.VideoID == "")
	default:
		return false
	}
}
func (ops CloneOperations) images(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if len(parts) == 0 && r.Method == http.MethodGet {
		view, err := ops.Images.Read(r.Context(), id)
		if err != nil {
			WriteCloneError(w, err)
			return
		}
		webapp.WriteJSON(w, 200, view)
		return
	}
	if len(parts) == 0 && r.Method == http.MethodPost {
		var input struct {
			Op               string               `json:"op"`
			ExpectedRevision *int                 `json:"expectedRevision"`
			ConfirmCount     int                  `json:"confirmCount"`
			Kind             string               `json:"kind,omitempty"`
			Feedback         string               `json:"feedback,omitempty"`
			Actions          []cloneimages.Action `json:"actions,omitempty"`
		}
		if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
			return
		}
		switch input.Op {
		case "approveAll":
			// "Duyệt cả video & dựng" ghi ý định chủ duyệt bền vững; bước
			// duyệt chỉ mở cổng build qua PrepareCloneExport, không truyền
			// accept nên cổng Độ hoàn thiện vẫn giữ nguyên.
			out, err := ops.Images.ApproveAll(r.Context(), id, cloneimages.RevisionInput{ExpectedRevision: input.ExpectedRevision, ApproveExport: true})
			cloneOperationJSON(w, out, err)
		case "regenerateAll":
			out, err := ops.Images.RedrawAll(r.Context(), id, cloneimages.ResetInput{ExpectedRevision: input.ExpectedRevision, ConfirmCount: input.ConfirmCount, Kind: input.Kind, Feedback: input.Feedback})
			cloneOperationJSON(w, out, err)
		case "review":
			out, err := ops.Images.Review(r.Context(), id, cloneimages.ReviewInput{ExpectedRevision: input.ExpectedRevision, Actions: input.Actions})
			cloneOperationJSON(w, out, err)
		default:
			invalidClone(w, "Thao tác ảnh không hợp lệ.")
		}
		return
	}
	if len(parts) == 1 && parts[0] == "redraw" && r.Method == http.MethodPost {
		var input cloneimages.RedrawVersionsInput
		if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
			return
		}
		result, err := ops.Images.RedrawVersions(r.Context(), id, input)
		var tooMany *cloneimages.TooManyRefsError
		if errors.As(err, &tooMany) {
			webapp.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "too_many_refs", "limit": tooMany.Limit})
			return
		}
		cloneOperationJSONStatus(w, http.StatusAccepted, result, err)
		return
	}
	if len(parts) == 2 && cloneSegment(parts[0]) && parts[1] == "use" && r.Method == http.MethodPost {
		var input cloneimages.UseVersionInput
		if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
			return
		}
		if input.N < 1 {
			invalidClone(w, "Phiên bản ảnh không hợp lệ.")
			return
		}
		result, err := ops.Images.UseVersion(r.Context(), id, parts[0], input)
		cloneOperationJSONStatus(w, http.StatusOK, result, err)
		return
	}
	if len(parts) == 4 && cloneSegment(parts[0]) && parts[1] == "versions" && parts[3] == "cancel" && r.Method == http.MethodPost {
		n, err := strconv.Atoi(parts[2])
		if err != nil || n < 1 {
			invalidClone(w, "Phiên bản ảnh không hợp lệ.")
			return
		}
		result, err := ops.Images.CancelVersion(r.Context(), id, parts[0], n)
		cloneOperationJSONStatus(w, http.StatusOK, result, err)
		return
	}
	if len(parts) != 1 || !cloneSegment(parts[0]) {
		cloneNotFound(w)
		return
	}
	imageID := parts[0]
	if r.Method == http.MethodGet {
		if variant := r.URL.Query().Get("variant"); variant != "" {
			if variant != "cutout" {
				invalidClone(w, "Biến thể ảnh không hợp lệ.")
				return
			}
			version := 0
			if raw := r.URL.Query().Get("version"); raw != "" {
				var err error
				version, err = strconv.Atoi(raw)
				if err != nil || version < 1 {
					invalidClone(w, "Phiên bản ảnh không hợp lệ.")
					return
				}
			}
			var path string
			var err error
			generator, derived := ops.Images.(interface {
				Cutout(context.Context, string, string, int) (string, error)
			})
			if derived {
				path, err = generator.Cutout(r.Context(), id, imageID, version)
			} else if version == 0 {
				path, err = ops.Images.DisplayedCutout(r.Context(), id, imageID)
			} else {
				cloneNotFound(w)
				return
			}
			if err != nil {
				WriteCloneError(w, err)
				return
			}
			if !cloneAvailable(w, ops.Edits != nil && ops.Edits.Media != nil) {
				return
			}
			var handle *media.Handle
			if derived {
				handle, err = ops.Edits.Media.Acquire(r.Context(), path)
			} else {
				var snapshot jobedit.Snapshot
				snapshot, err = ops.Edits.Read(r.Context(), id, domain.CloneImages)
				if err == nil {
					handle, err = ops.Edits.Acquire(r.Context(), snapshot, path)
				}
			}
			if err != nil {
				WriteCloneError(w, err)
				return
			}
			defer handle.Release()
			file, err := os.Open(handle.Path)
			if err != nil {
				WriteCloneError(w, err)
				return
			}
			defer file.Close()
			stat, err := file.Stat()
			if err != nil {
				WriteCloneError(w, err)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			http.ServeContent(w, r, "subject.png", stat.ModTime(), file)
			return
		}
		view, err := ops.Images.Read(r.Context(), id)
		if err != nil {
			WriteCloneError(w, err)
			return
		}
		for _, item := range view.Items {
			if item.ID == imageID {
				var revision *int
				if view.Manifest != nil {
					revision = &view.Manifest.Revision
				}
				webapp.WriteJSON(w, 200, map[string]any{"item": item, "revision": revision, "approved": view.Approved})
				return
			}
		}
		cloneNotFound(w)
		return
	}
	if r.Method != http.MethodPost {
		cloneNotFound(w)
		return
	}
	var input struct {
		Op               string  `json:"op"`
		Version          *int    `json:"version,omitempty"`
		Prompt           *string `json:"prompt,omitempty"`
		ExpectedRevision *int    `json:"expectedRevision"`
	}
	if !webapp.DecodeJSONCode(w, r, &input, domain.CloneErrInvalidRequest) {
		return
	}
	if input.ExpectedRevision == nil || *input.ExpectedRevision < 1 {
		invalidClone(w, "Thiếu revision hiện tại của ảnh.")
		return
	}
	valid := false
	switch input.Op {
	case "approve":
		valid = input.Version != nil && *input.Version > 0 && input.Prompt == nil
	case "regenerate":
		valid = input.Version == nil
	case "editPrompt":
		valid = input.Version == nil && input.Prompt != nil && strings.TrimSpace(*input.Prompt) != ""
	case "skip", "unskip":
		valid = input.Version == nil && input.Prompt == nil
	}
	if !valid || input.Prompt != nil && utf8.RuneCountInString(*input.Prompt) > 8000 {
		invalidClone(w, "Thao tác ảnh không hợp lệ.")
		return
	}
	result, err := ops.Images.Review(r.Context(), id, cloneimages.ReviewInput{ExpectedRevision: input.ExpectedRevision, Actions: []cloneimages.Action{{ID: imageID, Op: input.Op, Version: input.Version, Prompt: input.Prompt}}})
	cloneOperationJSON(w, result, err)
}
func (ops CloneOperations) cover(w http.ResponseWriter, r *http.Request, id string) {
	if r.ContentLength > 20<<20 {
		webapp.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": domain.CloneErrInvalidRequest, "error": "Ảnh bìa tối đa 20 MB."})
		return
	}
	revision, err := strconv.Atoi(r.URL.Query().Get("expectedRevision"))
	if err != nil || revision < 1 {
		invalidClone(w, "Thiếu revision hiện tại của ảnh.")
		return
	}
	work, err := os.MkdirTemp("", "viderse-cover-upload-")
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	defer os.RemoveAll(work)
	file, err := os.OpenFile(filepath.Join(work, "cover.img"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	_, err = io.Copy(file, http.MaxBytesReader(w, r.Body, 20<<20))
	closeErr := file.Close()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			webapp.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"code": domain.CloneErrInvalidRequest, "error": "Ảnh bìa tối đa 20 MB."})
		} else {
			invalidClone(w, "Không đọc được ảnh bìa.")
		}
		return
	}
	if closeErr != nil {
		WriteCloneError(w, closeErr)
		return
	}
	result, err := ops.Images.UploadCover(r.Context(), id, file.Name(), cloneimages.RevisionInput{ExpectedRevision: &revision})
	cloneOperationJSON(w, result, err)
}
