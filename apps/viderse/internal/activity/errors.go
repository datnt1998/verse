package activity

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ErrorsWindow là cửa sổ của mục "Lỗi gần đây" (7 ngày qua).
const ErrorsWindow = 7 * 24 * time.Hour

// errorsLimit là số mục tối đa của danh sách lỗi gần đây.
const errorsLimit = 20

// ErrorItem là một mục của "Lỗi gần đây": việc nền thất bại hoặc thông báo
// cần xử lý trong ErrorsWindow. Kind giữ nguyên loại (task/notification) để
// tầng web chọn nhãn và liên kết; Status là câu ngắn người dùng.
type ErrorItem struct {
	ID      string
	At      time.Time
	What    string
	Why     string
	Status  string
	Href    string
	Link    string
	Object  *domain.ObjectRef
	Action  domain.TaskAction
	Kind    string // task_failed | needs_attention | action_result
	Retried bool
}

// ErrorEvent là một lỗi đã ghi bền (error_events) — lịch sử 7 ngày không mất
// khi người dùng đọc thông báo hay thử lại việc nền.
type ErrorEvent struct {
	ID          string
	At          time.Time
	Kind        string
	What        string
	Why         string
	Href        string
	ActionLabel string
	Op          string
	Object      *domain.ObjectRef
	Audience    Audience
}

// findTask đọc một việc theo id (cho lịch sử lỗi khi việc chuyển failed).
func (s *Service) findTask(ctx context.Context, id string) (Task, error) {
	tasks, err := s.store.ListTasks(ctx)
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, ErrNotFound
}

// actionHref lấy liên kết của hành động (rỗng khi không có).
func actionHref(a domain.TaskAction) string { return a.Href }

// Errors trả các lỗi gần đây để web render mục Báo lỗi: lịch sử bền 7 ngày
// (việc thất bại và thông báo cần xử lý), không phụ thuộc trạng thái đã đọc.
func (s *Service) Errors(ctx context.Context) ([]ErrorItem, error) {
	since := time.Now().Add(-ErrorsWindow)
	events, err := s.store.ListErrorEvents(ctx, since, errorsLimit)
	if err != nil {
		return nil, err
	}
	items := make([]ErrorItem, 0, len(events))
	for _, ev := range events {
		status, tone := "Đã xử lý", ""
		if ev.Kind == "task_failed" || ev.Kind == "needs_attention" {
			status = "Cần xử lý"
		}
		link := ev.ActionLabel
		if link == "" {
			link = "Mở"
		}
		items = append(items, ErrorItem{
			ID: ev.ID, At: ev.At, What: ev.What, Why: ev.Why,
			Status: status, Href: ev.Href, Link: link, Kind: ev.Kind,
			Action: domain.TaskAction{Label: ev.ActionLabel, Href: ev.Href, Op: ev.Op}, Object: ev.Object,
		})
		_ = tone
	}
	return items, nil
}

// ErrorsCount đếm số lỗi trong cửa sổ (nút tiêu đề "7 ngày qua · N lỗi").
func (s *Service) ErrorsCount(ctx context.Context) (int, error) {
	return s.store.CountErrorEvents(ctx, time.Now().Add(-ErrorsWindow))
}

// actionOp giữ op của hành động (vd retry) để màn Báo lỗi phát lại đúng thao tác.
func actionOp(a domain.TaskAction) string { return a.Op }
