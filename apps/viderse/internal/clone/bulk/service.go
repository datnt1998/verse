// Package bulk applies a bounded owner action without hiding per-video errors.
package bulk

import (
	"context"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/bundle"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type Jobs interface {
	Get(context.Context, string) (*domain.CloneJobView, error)
	Resume(context.Context, string) (*domain.CloneJob, error)
	Cancel(context.Context, string) (*domain.CloneJob, error)
	Delete(context.Context, string) error
}
type Images interface {
	ApproveAll(context.Context, string, cloneimages.RevisionInput) (cloneimages.MutationResult, error)
}
type Store interface {
	jobedit.Store
	MarkPosted(context.Context, string, bool) error
	RecordExportRemoved(context.Context, jobedit.Snapshot) error
}
type Exports interface {
	DeleteExport(context.Context, string, string) error
}
type Service struct {
	Jobs           Jobs
	Images         Images
	Store          Store
	Exports        Exports
	Publish        func(string, any)
	Approvals      ApprovalStore
	ApproveProject func(context.Context, string) error
}
type Input struct {
	Action            string         `json:"action"`
	JobIDs            []string       `json:"jobIds"`
	ExpectedRevisions map[string]int `json:"expectedRevisions,omitempty"`
}
type Row struct {
	JobID string `json:"jobId"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
type Result struct {
	Results []Row `json:"results"`
}

func invalid(message string) error {
	return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: message}
}
func unavailable() error {
	return &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Thao tác video chưa sẵn sàng"}
}
func (s *Service) Run(ctx context.Context, input Input) (Result, error) {
	if len(input.JobIDs) < 1 || len(input.JobIDs) > 100 {
		return Result{}, invalid("Chọn từ 1 đến 100 video")
	}
	switch input.Action {
	case "retry", "cancel", "approve", "delete", "posted", "unposted", "deleteExport":
	default:
		return Result{}, invalid("Thao tác video không hợp lệ")
	}
	for _, id := range input.JobIDs {
		if id == "" || len(id) > 64 || strings.ContainsAny(id, "/\\% \r\n\t") {
			return Result{}, invalid("Mã video không hợp lệ")
		}
	}
	result := Result{Results: make([]Row, len(input.JobIDs))}
	for index, id := range input.JobIDs {
		err := s.run(ctx, id, input)
		row := Row{JobID: id, OK: err == nil}
		if err != nil {
			row.Error = err.Error()
		}
		result.Results[index] = row
	}
	return result, nil
}
func (s *Service) run(ctx context.Context, id string, input Input) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.Jobs == nil {
		return unavailable()
	}
	switch input.Action {
	case "retry":
		_, err := s.Jobs.Resume(ctx, id)
		return err
	case "cancel":
		_, err := s.Jobs.Cancel(ctx, id)
		return err
	case "delete":
		return s.Jobs.Delete(ctx, id)
	case "approve":
		if s.Images == nil {
			return unavailable()
		}
		var revision *int
		if value, ok := input.ExpectedRevisions[id]; ok {
			revision = &value
		}
		_, err := s.Images.ApproveAll(ctx, id, cloneimages.RevisionInput{ExpectedRevision: revision, ApproveExport: true})
		return err
	case "posted", "unposted":
		if s.Store == nil {
			return unavailable()
		}
		view, err := s.Jobs.Get(ctx, id)
		if err != nil {
			return err
		}
		if view == nil {
			return &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy video"}
		}
		if input.Action == "posted" && view.Status != domain.CloneSucceeded {
			return invalid("Chỉ đánh dấu đã đăng cho video đã xuất xong")
		}
		if input.Action == "unposted" && view.PostedAt == nil {
			return invalid("Video chưa được đánh dấu đã đăng")
		}
		err = s.Store.MarkPosted(ctx, id, input.Action == "posted")
		if err == nil && s.Publish != nil {
			s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": id})
		}
		return err
	case "deleteExport":
		if s.Store == nil || s.Exports == nil {
			return unavailable()
		}
		snapshot, err := s.Store.Read(ctx, id, domain.CloneExport)
		if err != nil {
			return err
		}
		if snapshot.Job.PostedAt == nil {
			return invalid("Chỉ xoá file xuất của video đã đánh dấu đã đăng")
		}
		projectID, exportID, err := bundle.ExportIDs(snapshot)
		if err != nil {
			return err
		}
		if err = s.Exports.DeleteExport(ctx, projectID, exportID); err != nil {
			return err
		}
		if err = s.Store.RecordExportRemoved(ctx, snapshot); err != nil {
			return err
		}
		if s.Publish != nil {
			s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": id})
		}
		return nil
	}
	return invalid("Thao tác video không hợp lệ")
}
