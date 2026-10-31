package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/platform/blobhttp"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// CloneService is the job-facing pipeline boundary; scheduling stays in the queue.
type CloneService interface {
	Get(context.Context, string) (*domain.CloneJobView, error)
	List(context.Context, *string, int) (domain.CloneJobList, error)
	Counts(context.Context) (domain.CloneCounts, error)
	Run(context.Context, string) (*domain.CloneJob, error)
	Pause(context.Context, string) (*domain.CloneJob, error)
	Resume(context.Context, string) (*domain.CloneJob, error)
	Cancel(context.Context, string) (*domain.CloneJob, error)
	Delete(context.Context, string) error
	Step(context.Context, string, domain.CloneStep) (*domain.CloneStepRecord, error)
	Artifact(context.Context, string, string) (*domain.CloneArtifact, error)
}

// CloneEnqueuer admits new videos through the shared queue, never the worker.
type CloneEnqueuer interface {
	Enqueue(context.Context, domain.CloneEnqueueRequest) (domain.CloneEnqueueResult, error)
}

type cloneMedia interface {
	Resolve(context.Context, string) (blobstore.BlobStore, media.Descriptor, media.Ref, func(), error)
	Protect(context.Context, media.Ref, time.Time) error
}

// MountClone registers authenticated clone routes. MountQueue must run first:
// its exact /api/clone/plan route precedes the job-action prefix route.
func MountClone(g *Gateway, svc CloneService, enqueuer CloneEnqueuer, files *media.Service) {
	g.Clone = svc
	var source cloneMedia
	if files != nil {
		source = files
	}
	g.registerCloneRoutes(enqueuer, source)
}

func (g *Gateway) registerCloneRoutes(enqueuer CloneEnqueuer, files cloneMedia) {
	g.Handle(http.MethodGet, "/api/clone", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Cache-Control", "no-store")
		listClone(w, r, g.Clone)
	})
	g.Handle(http.MethodPost, "/api/clone", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Cache-Control", "no-store")
		enqueueClone(w, r, enqueuer)
	})
	g.Handle(http.MethodGet, "/api/clone/counts", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Cache-Control", "no-store")
		if !cloneAvailable(w, g.Clone != nil) {
			return
		}
		counts, err := g.Clone.Counts(r.Context())
		if err != nil {
			WriteCloneError(w, err)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, counts)
	})
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		g.Handle(method, "/api/clone/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			w.Header().Set("Cache-Control", "no-store")
			g.routeClone(w, r, files)
		})
	}
}

func cloneAvailable(w http.ResponseWriter, available bool) bool {
	if !available {
		webapp.WriteJSON(w, http.StatusServiceUnavailable, cloneErrorWire{
			Error: "Dịch vụ làm video tạm thời chưa sẵn sàng.", Code: domain.CloneErrToolMissing,
		})
	}
	return available
}

func cloneSegment(value string) bool {
	return len(value) <= 128 && !strings.ContainsAny(value, "/% \t\r\n") && media.ValidatePath(value) == nil
}

func invalidClone(w http.ResponseWriter, message string) {
	WriteCloneError(w, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: message})
}

func listClone(w http.ResponseWriter, r *http.Request, svc CloneService) {
	query := r.URL.Query()
	for key, values := range query {
		if (key != "channel" && key != "channelId" && key != "limit") || len(values) != 1 {
			invalidClone(w, "Bộ lọc video không hợp lệ.")
			return
		}
	}
	channel := query.Get("channelId")
	if other, ok := query["channel"]; ok {
		if _, both := query["channelId"]; both && channel != other[0] {
			invalidClone(w, "Chỉ chọn một kênh để lọc video.")
			return
		}
		channel = other[0]
	}
	var channelID *string
	if channel != "" && channel != "all" {
		if !cloneSegment(channel) {
			invalidClone(w, "Kênh không hợp lệ.")
			return
		}
		channelID = &channel
	}
	limit := 10000
	if values, ok := query["limit"]; ok {
		var err error
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			invalidClone(w, "Số video cần lấy phải từ 1 đến 100.")
			return
		}
	}
	if !cloneAvailable(w, svc != nil) {
		return
	}
	list, err := svc.List(r.Context(), channelID, limit)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	list.BulkEligibility = domain.BulkEligibility(list.Jobs)
	if id := domain.PickDefaultJob(list.Jobs); id != "" {
		list.DefaultJobID = &id
	}
	if channelID == nil {
		visible := list.Jobs[:0]
		queued := 0
		for _, job := range list.Jobs {
			if job.StageGroup == domain.StagePosted {
				continue
			}
			if job.StageGroup == domain.StageQueued {
				queued++
				if queued > 3 {
					continue
				}
			}
			visible = append(visible, job)
		}
		list.Jobs = visible
	}
	webapp.WriteJSON(w, http.StatusOK, list)
}

func enqueueClone(w http.ResponseWriter, r *http.Request, svc CloneEnqueuer) {
	var request domain.CloneEnqueueRequest
	if !webapp.DecodeJSONCode(w, r, &request, domain.CloneErrInvalidRequest) {
		return
	}
	if !cloneSegment(request.ChannelID) || len(request.URLs) == 0 {
		invalidClone(w, "Chọn kênh và ít nhất một liên kết video.")
		return
	}
	if request.Priority != "" && request.Priority != domain.ClonePick && request.Priority != domain.CloneBatch {
		invalidClone(w, "Mức ưu tiên không hợp lệ.")
		return
	}
	for _, value := range request.URLs {
		if strings.TrimSpace(value) == "" || len(value) > 2048 {
			invalidClone(w, "Liên kết video không hợp lệ.")
			return
		}
	}
	if !cloneAvailable(w, svc != nil) {
		return
	}
	result, err := svc.Enqueue(r.Context(), request)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	webapp.WriteJSON(w, http.StatusCreated, result)
}

func (g *Gateway) routeClone(w http.ResponseWriter, r *http.Request, files cloneMedia) {
	if g.renderCloneRoute != nil && g.renderCloneRoute(w, r) {
		return
	}
	if g.cloneReviewRoute != nil && g.cloneReviewRoute(w, r) {
		return
	}
	if g.clonePostedRoute != nil && g.clonePostedRoute(w, r) {
		return
	}
	svc := g.Clone
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/clone/"), "/", 4)
	if !cloneSegment(parts[0]) {
		invalidClone(w, "Mã video không hợp lệ.")
		return
	}
	id := parts[0]
	if len(parts) == 3 && parts[1] == "artifacts" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if !g.RequireSameOrigin(w, r) {
			return
		}
		if !cloneSegment(parts[2]) {
			invalidClone(w, "Mã tệp không hợp lệ.")
			return
		}
		if cloneAvailable(w, svc != nil) {
			serveCloneArtifact(w, r, svc, files, id, parts[2])
		}
		return
	}
	if len(parts) == 3 && parts[1] == "steps" && r.Method == http.MethodGet {
		if !domain.IsCloneStep(parts[2]) {
			invalidClone(w, "Bước làm video không hợp lệ.")
			return
		}
		if !cloneAvailable(w, svc != nil) {
			return
		}
		step, err := svc.Step(r.Context(), id, domain.CloneStep(parts[2]))
		if err != nil {
			WriteCloneError(w, err)
			return
		}
		if step == nil || step.JobID != id || step.Step != domain.CloneStep(parts[2]) {
			cloneNotFound(w)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, struct {
			Step *domain.CloneStepRecord `json:"step"`
		}{Step: step})
		return
	}
	if len(parts) != 1 || r.Method == http.MethodHead {
		cloneNotFound(w)
		return
	}
	if !cloneAvailable(w, svc != nil) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		view, err := svc.Get(r.Context(), id)
		if err != nil {
			WriteCloneError(w, err)
			return
		}
		if view == nil || view.ID != id {
			cloneNotFound(w)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, view)
	case http.MethodPost:
		actionClone(w, r, svc, id)
	case http.MethodDelete:
		if err := svc.Delete(r.Context(), id); err != nil {
			WriteCloneError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func actionClone(w http.ResponseWriter, r *http.Request, svc CloneService, id string) {
	var request struct {
		Action string `json:"action"`
		DryRun bool   `json:"dryRun"`
	}
	if !webapp.DecodeJSONCode(w, r, &request, domain.CloneErrInvalidRequest) {
		return
	}
	if request.DryRun && request.Action != "restart" {
		invalidClone(w, "Chỉ xem trước được thao tác làm lại.")
		return
	}
	var job *domain.CloneJob
	var err error
	switch request.Action {
	case "run":
		job, err = svc.Run(r.Context(), id)
	case "pause":
		job, err = svc.Pause(r.Context(), id)
	case "resume":
		job, err = svc.Resume(r.Context(), id)
	case "cancel":
		job, err = svc.Cancel(r.Context(), id)
	case "restart":
		if request.DryRun {
			planner, ok := svc.(interface {
				RestartPlan(context.Context, string) (*domain.CloneRestartPlan, error)
			})
			if !ok {
				cloneAvailable(w, false)
				return
			}
			plan, err := planner.RestartPlan(r.Context(), id)
			if err != nil {
				WriteCloneError(w, err)
				return
			}
			webapp.WriteJSON(w, http.StatusOK, plan)
			return
		}
		restarter, ok := svc.(interface {
			Restart(context.Context, string) (*domain.CloneQueueItem, error)
		})
		if !ok {
			cloneAvailable(w, false)
			return
		}
		result, restartErr := restarter.Restart(r.Context(), id)
		if restartErr != nil {
			WriteCloneError(w, restartErr)
			return
		}
		webapp.WriteJSON(w, http.StatusAccepted, map[string]any{"restartedFrom": id, "queueItem": result})
		return
	default:
		invalidClone(w, "Thao tác làm video không hợp lệ.")
		return
	}
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	if job == nil || job.ID != id {
		cloneNotFound(w)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, job)
}

func cloneNotFound(w http.ResponseWriter) {
	WriteCloneError(w, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy video hoặc tệp."})
}

// serveCloneArtifact authorizes the manifest entry before resolving any media.
// Resolve is performed once: the identity checked here is exactly the binding
// streamed below, held under its media read lease throughout delivery.
func serveCloneArtifact(w http.ResponseWriter, r *http.Request, svc CloneService, files cloneMedia, id, artifactID string) {
	artifact, err := svc.Artifact(r.Context(), id, artifactID)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	if artifact == nil || artifact.ID != artifactID {
		cloneNotFound(w)
		return
	}
	if err := media.ValidatePath(artifact.Path); err != nil {
		invalidClone(w, "Đường dẫn tệp không hợp lệ.")
		return
	}
	key, err := media.ContentKey(artifact.SHA256)
	if err != nil || artifact.Bytes < 0 {
		invalidClone(w, "Thông tin nhận dạng tệp không hợp lệ.")
		return
	}
	prefix := "clone/" + id + "/" + artifact.SHA256 + "/"
	if !strings.HasPrefix(artifact.Path, prefix) {
		cloneNotFound(w)
		return
	}
	if !cloneAvailable(w, files != nil) {
		return
	}
	store, descriptor, ref, release, err := files.Resolve(r.Context(), artifact.Path)
	if err != nil {
		WriteCloneError(w, err)
		return
	}
	defer release()
	if ref.Path != artifact.Path || ref.SHA256 != artifact.SHA256 || ref.Bytes != artifact.Bytes || ref.Key != key {
		WriteCloneError(w, &domain.CloneError{Code: domain.CloneErrStale, Message: "Tệp đã thay đổi. Vui lòng tải lại thông tin video."})
		return
	}
	const ttl = 2 * time.Minute
	if descriptor.Config.Driver == "s3" && descriptor.Config.Delivery == "redirect" && r.Method == http.MethodGet {
		if err := files.Protect(r.Context(), ref, time.Now().Add(ttl+2*time.Minute)); err != nil {
			WriteCloneError(w, err)
			return
		}
	}
	kind := "inline"
	if r.URL.Query().Get("download") != "" {
		kind = "attachment"
	}
	err = blobhttp.Serve(w, r, store, blobstore.BlobInfo{
		Key: ref.Key, Size: ref.Bytes, SHA256: ref.SHA256,
		ContentType: ref.ContentType, ModTime: ref.VerifiedAt,
	}, blobhttp.Options{
		Delivery: descriptor.Config.Delivery, PresignTTL: ttl,
		CacheControl: "private, max-age=31536000, immutable", Disposition: kind, Filename: ref.Filename,
	})
	if err != nil {
		WriteCloneError(w, err)
	}
}

type cloneErrorWire struct {
	Error         string     `json:"error"`
	Code          string     `json:"code"`
	Recovery      *string    `json:"recovery"`
	RetryAt       *time.Time `json:"retryAt"`
	RetryAtSource string     `json:"retryAtSource,omitempty"`
}

// WriteCloneError preserves actionable pipeline errors without exposing internal
// filesystem, provider credential or database diagnostics to the browser.
func WriteCloneError(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store")
	var cloneErr *domain.CloneError
	status := http.StatusInternalServerError
	if errors.As(err, &cloneErr) {
		switch cloneErr.Code {
		case domain.CloneErrInvalidRequest:
			status = http.StatusBadRequest
		case domain.CloneErrNotFound:
			status = http.StatusNotFound
		case domain.CloneErrConflict, domain.CloneErrCancelled,
			domain.CloneErrStale, domain.CloneErrTemplateChanged, domain.CloneErrQualityGate, domain.CloneErrAwaitingReview:
			status = http.StatusConflict
		case domain.CloneErrQuota:
			status = http.StatusTooManyRequests
		case domain.CloneErrToolMissing, domain.CloneErrInterrupted:
			status = http.StatusServiceUnavailable
		case domain.CloneErrSourceUnavailable:
			status = http.StatusGone
		}
		if status != http.StatusInternalServerError {
			webapp.WriteJSON(w, status, cloneErrorWire{
				Error: cloneErr.Message, Code: cloneErr.Code, Recovery: cloneErr.Recovery,
				RetryAt: cloneErr.RetryAt, RetryAtSource: cloneErr.RetryAtSource,
			})
			return
		}
	}
	if errors.Is(err, pipeline.ErrNotFound) {
		webapp.WriteJSON(w, http.StatusNotFound, cloneErrorWire{Error: "Không tìm thấy video.", Code: domain.CloneErrNotFound})
		return
	}
	switch {
	case errors.Is(err, blobstore.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, blobstore.ErrInvalidRange):
		status = http.StatusRequestedRangeNotSatisfiable
	case errors.Is(err, blobstore.ErrInvalidKey):
		status = http.StatusBadRequest
	case errors.Is(err, media.ErrConflict), errors.Is(err, media.ErrBusy):
		status = http.StatusConflict
	}
	if status == http.StatusInternalServerError {
		slog.Error("clone request failed", "err", err)
		webapp.WriteJSON(w, status, cloneErrorWire{Error: "Lỗi nội bộ khi làm video.", Code: domain.CloneErrInternal})
		return
	}
	code, message := domain.CloneErrConflict, "Tệp đang được cập nhật. Vui lòng thử lại."
	switch status {
	case http.StatusNotFound:
		code, message = domain.CloneErrNotFound, "Không tìm thấy tệp."
	case http.StatusBadRequest, http.StatusRequestedRangeNotSatisfiable:
		code, message = domain.CloneErrInvalidRequest, "Yêu cầu đọc tệp không hợp lệ."
	}
	webapp.WriteJSON(w, status, cloneErrorWire{Error: message, Code: code})
}
