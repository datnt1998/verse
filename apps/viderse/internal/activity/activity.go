// Package activity là bộ đếm việc nền của Viderse (hợp đồng dữ liệu §11):
// các mảng nghiệp vụ mở một việc nền qua Start/Update/Finish trên các kiểu
// domain.Task*, và ghi thông báo bằng Notify theo luật gộp 10 giây. Store là
// interface phía người dùng — hiện thực Postgres ở internal/persistence/postgres.
//
// Bảng chỉ giữ việc chưa kết thúc: Finish xoá dòng. Mỗi thay đổi phát một sự
// kiện qua notify(ref) để web vô hiệu cache /api/activity. Khởi động app gọi
// Reconcile: dòng còn active của process đã chết trở thành failed kèm hành
// động Thử lại, thay vì treo "đang chạy" mãi.
package activity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Task là một việc nền chưa kết thúc (dòng background_tasks).
type Task struct {
	ID        string
	Kind      domain.TaskKind
	Agent     string
	Title     string
	Object    domain.ObjectRef
	Phase     string
	Progress  *domain.TaskProgress
	ETASec    *int
	State     domain.TaskState
	Action    domain.TaskAction
	OwnerPID  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Notification là một thông báo (dòng notifications).
type Notification struct {
	ID          string
	Kind        string
	Count       int
	Text        string
	Href        string
	ActionLabel string
	CreatedAt   time.Time
	ReadAt      *time.Time
}

// Snapshot là hình dữ liệu của GET /api/activity.
type Snapshot struct {
	Tasks     []Task         // mọi việc chưa kết thúc (wire: "running")
	VideoJobs int            // việc kind video đang state=running
	Queued    int            // việc kind video đang state=waiting (tới khi có hàng chờ thật)
	Unread    []Notification // thông báo chưa đọc
}

// Store là phần lưu trữ service cần. Mọi thao tác nguyên tử theo dòng; luật
// "một đối tượng một việc" và "gộp 10 giây" nằm trong tx của store.
type Store interface {
	// FindTaskByObject trả việc của một đối tượng (kind + object_ref), ErrNotFound khi vắng.
	FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*Task, error)
	InsertTask(ctx context.Context, spec domain.TaskSpec) (*Task, error)
	// ResetTask viết lại toàn bộ nội dung của một việc (retry sau failed).
	ResetTask(ctx context.Context, id string, spec domain.TaskSpec) error
	UpdateTask(ctx context.Context, id string, u domain.TaskUpdate) error
	DeleteTask(ctx context.Context, id string) error
	ListTasks(ctx context.Context) ([]Task, error)
	// CountTasks đếm theo (kind, state); kind rỗng khớp mọi loại.
	CountTasks(ctx context.Context, kind domain.TaskKind, state domain.TaskState) (int64, error)
	// FailTasksByIDs chuyển các việc còn active của process đã chết thành
	// failed với hành động Thử lại; trả các dòng đã đụng.
	FailTasksByIDs(ctx context.Context, ids []string) ([]Task, error)
	// InsertErrorEvent ghi một lỗi bền cho mục "Lỗi gần đây".
	InsertErrorEvent(ctx context.Context, e ErrorEvent) error
	// ListErrorEvents đọc lỗi từ since (mới nhất trước), tối đa limit dòng.
	ListErrorEvents(ctx context.Context, since time.Time, limit int) ([]ErrorEvent, error)
	// CountErrorEvents đếm lỗi từ since — không phụ thuộc trang hiển thị.
	CountErrorEvents(ctx context.Context, since time.Time) (int, error)
	// UpsertNotification ghi thông báo: cùng kind và lần mới nhất trẻ hơn
	// window thì tăng count/cập nhật nội dung của dòng đó, ngược lại chèn dòng
	// mới. Nguyên tử giữa các writer đồng thời.
	UpsertNotification(ctx context.Context, kind, text, href, actionLabel string, window time.Duration) (*Notification, error)
	ListUnreadNotifications(ctx context.Context, limit int) ([]Notification, error)
	// MarkRead đánh dấu đã đọc; ids rỗng nghĩa là tất cả.
	MarkRead(ctx context.Context, ids []string) error
}

// ErrNotFound là lỗi "không có dòng" của store (FindTaskByObject).
var ErrNotFound = errors.New("không tìm thấy")

// NotifyMergeWindow là cửa sổ gộp thông báo của §11.
const NotifyMergeWindow = 10 * time.Second

// Service quản lý việc nền và thông báo. Tất cả phương thức an toàn cho
// goroutine; notify được gọi ngoài mọi khoá và không được chặn.
type Service struct {
	store     Store
	notify    func(ref string)
	startLock sync.Mutex // chống hai Start cùng đối tượng đua nhau trong process
}

// New dựng service. notify nhận ref (id việc vừa đổi, id thông báo, hoặc "");
// nil = không phát sự kiện (test).
func New(store Store, notify func(ref string)) *Service {
	return &Service{store: store, notify: notify}
}

func (s *Service) emit(ref string) {
	if s.notify != nil {
		s.notify(ref)
	}
}

// Start mở một việc nền mới và trả id của nó. Một đối tượng chỉ có một việc:
// Start lặp trên cùng (kind, object) kế thừa dòng cũ — việc failed được đặt
// lại thành running (retry), việc đang sống được viết lại nội dung.
func (s *Service) Start(ctx context.Context, spec domain.TaskSpec) (string, error) {
	if strings.TrimSpace(spec.Title) == "" {
		return "", fmt.Errorf("việc nền cần tiêu đề")
	}
	if spec.Kind == "" {
		return "", fmt.Errorf("việc nền cần kind")
	}
	s.startLock.Lock()
	defer s.startLock.Unlock()
	existing, err := s.store.FindTaskByObject(ctx, spec.Kind, spec.Object)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", err
	}
	var id string
	if existing == nil {
		created, err := s.store.InsertTask(ctx, spec)
		if err != nil {
			return "", err
		}
		id = created.ID
	} else {
		if err := s.store.ResetTask(ctx, existing.ID, spec); err != nil {
			return "", err
		}
		id = existing.ID
	}
	s.emit(id)
	return id, nil
}

// Update đổi phần đang thay đổi của một việc; trường nil giữ giá trị cũ.
func (s *Service) Update(ctx context.Context, id string, u domain.TaskUpdate) error {
	if id == "" {
		return fmt.Errorf("thiếu id việc nền")
	}
	if err := s.store.UpdateTask(ctx, id, u); err != nil {
		return err
	}
	if u.State == domain.TaskFailed {
		if t, err := s.findTask(ctx, id); err == nil {
			why := t.Phase
			label := ""
			if u.Action != nil {
				label = u.Action.Label
			}
			_ = s.store.InsertErrorEvent(ctx, ErrorEvent{
				Kind: "task_failed", What: t.Title, Why: why,
				Href: actionHref(t.Action), ActionLabel: label, Op: actionOp(t.Action),
				Object: &t.Object,
			})
		}
	}
	s.emit(id)
	return nil
}

// Finish kết thúc việc nền: xoá dòng (bảng chỉ giữ việc chưa kết thúc).
func (s *Service) Finish(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := s.store.DeleteTask(ctx, id); err != nil {
		return err
	}
	s.emit(id)
	return nil
}

// Notify ghi một thông báo cho người dùng: cùng kind trong NotifyMergeWindow
// gộp vào dòng mới nhất (count tăng), ngoài cửa sổ tạo dòng mới. Trả id thông
// báo (dòng mới hoặc dòng được gộp).
func (s *Service) Notify(ctx context.Context, kind, text, href, actionLabel string) (string, error) {
	n, err := s.store.UpsertNotification(ctx, kind, text, href, actionLabel, NotifyMergeWindow)
	if err != nil {
		return "", err
	}
	_ = s.store.InsertErrorEvent(ctx, ErrorEvent{Kind: kind, What: text, Href: href, ActionLabel: actionLabel})
	s.emit(n.ID)
	return n.ID, nil
}

// Reconcile chạy khi khởi động: việc còn active của process trước (đã chết)
// thành failed kèm nút Thử lại — caller của từng mảng tự khởi động lại khi
// người dùng bấm. Trả số việc đã đánh dấu.
func (s *Service) Reconcile(ctx context.Context) (int, error) {
	tasks, err := s.store.ListTasks(ctx)
	if err != nil {
		return 0, err
	}
	var dead []string
	for _, t := range tasks {
		if t.State != domain.TaskRunning && t.State != domain.TaskPaused && t.State != domain.TaskWaiting {
			continue
		}
		if t.OwnerPID == 0 || !processAlive(t.OwnerPID) {
			dead = append(dead, t.ID)
		}
	}
	if len(dead) == 0 {
		return 0, nil
	}
	orphans, err := s.store.FailTasksByIDs(ctx, dead)
	if err != nil {
		return 0, err
	}
	if len(orphans) > 0 {
		s.emit("")
	}
	return len(orphans), nil
}

// Snapshot render hình của GET /api/activity.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	tasks, err := s.store.ListTasks(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.Before(tasks[j].CreatedAt) })
	video, err := s.store.CountTasks(ctx, domain.TaskVideo, domain.TaskRunning)
	if err != nil {
		return Snapshot{}, err
	}
	queued, err := s.store.CountTasks(ctx, domain.TaskVideo, domain.TaskWaiting)
	if err != nil {
		return Snapshot{}, err
	}
	unread, err := s.store.ListUnreadNotifications(ctx, 50)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Tasks: tasks, VideoJobs: int(video), Queued: int(queued), Unread: unread}, nil
}

// MarkRead đánh dấu thông báo đã đọc; ids rỗng = tất cả.
func (s *Service) MarkRead(ctx context.Context, ids []string) error {
	if err := s.store.MarkRead(ctx, ids); err != nil {
		return err
	}
	s.emit("")
	return nil
}
