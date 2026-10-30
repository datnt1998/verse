// Package setup là điều phối bước Chuẩn bị máy (W1): sáu hàng — năm lượt cài
// công cụ tự chạy theo thứ tự, hàng cuối là đăng nhập ChatGPT chờ người
// dùng. Một hàng lỗi không chặn các hàng khác (W1 Lỗi). Trạng thái đọc qua
// interface Tools của phía dùng (internal/toolsvc thoả); tiến độ đi qua việc
// nền + sự kiện tools nên màn này chỉ đọc lại.
package setup

import (
	"context"
	"fmt"
	"sync"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// Tools là phần của toolsvc mà orchestrator cần — khai ở phía dùng.
type Tools interface {
	// StepChecks đánh giá các bước (fresh=true probe lại thật).
	StepChecks(ctx context.Context, fresh bool) (map[domain.SetupStepID]domain.SetupCheck, error)
	// Ensure cài một bước và trả lỗi của lượt (join lượt đang chạy).
	Ensure(ctx context.Context, id domain.SetupStepID) error
	// LiveProgress là tiến độ trực tiếp của lượt đang chạy (nil = không có).
	LiveProgress(id domain.SetupStepID) *domain.InstallProgress
	// RequiredBytes là dung lượng trống tối thiểu để cài bước (0 = không đo
	// được) — chỉ cho kiểm tra đĩa.
	RequiredBytes(id domain.SetupStepID) int64
	// ApproxBytes là dung lượng cài đặt xấp xỉ của bước, để phần giới thiệu
	// “khoảng X GB” có số thật.
	ApproxBytes(id domain.SetupStepID) int64
}

// installStepIDs là năm bước tự cài theo thứ tự của W1.
var installStepIDs = []domain.SetupStepID{
	domain.SetupStepYtDlp, domain.SetupStepFfmpeg, domain.SetupStepTranscription,
	domain.SetupStepChrome, domain.SetupStepCodex,
}

// StepState là trạng thái một hàng trên màn Chuẩn bị máy.
type StepState = string

const (
	StepDone      StepState = "done"
	StepPending   StepState = "pending"
	StepRunning   StepState = "running"
	StepFailed    StepState = "failed"
	StepNeedsUser StepState = "needs_user"
)

// Step là một hàng của màn Chuẩn bị máy.
type Step struct {
	ID       domain.SetupStepID
	State    StepState
	Progress *domain.InstallProgress
	Error    string
	// SizeBytes là dung lượng bước chiếm khi cài xong (0 = không đo được).
	SizeBytes int64
	// Provided = công cụ có sẵn trên máy nên không phải tải (Chrome hệ thống).
	Provided bool
}

// Disk là dung lượng trống/cần của ổ chứa thư mục dữ liệu.
type Disk struct {
	FreeBytes int64
	// RequiredBytes là chỗ trống tối thiểu cho các bước còn thiếu (kiểm tra,
	// không hiển thị kích thước).
	RequiredBytes int64
	Enough        bool
	// ApproxBytes là tổng cài đặt xấp xỉ của các bước còn thiếu — cho câu
	// “khoảng X GB” trong phần giới thiệu.
	ApproxBytes int64
}

// OverallState là trạng thái tổng của màn: running | login | failed | done.
type OverallState = string

const (
	OverallRunning OverallState = "running"
	OverallLogin   OverallState = "login"
	OverallFailed  OverallState = "failed"
	OverallIdle    OverallState = "idle"
	OverallDone    OverallState = "done"
)

// Status là hình của GET /api/setup.
type Status struct {
	State    OverallState
	Complete bool
	Running  bool
	Steps    []Step
	Disk     Disk
}

// run là một lượt cài: bước hiện tại, lỗi theo bước, và kênh kết thúc.
type run struct {
	current domain.SetupStepID
	errors  map[domain.SetupStepID]string
	done    chan struct{}
	only    domain.SetupStepID // chỉ chạy một bước (retry đơn); rỗng = hết
}

// Service điều phối bước Chuẩn bị máy. Một lượt cài một lúc trong process;
// Start gọi lại gộp vào lượt đang chạy.
type Service struct {
	tools   Tools
	baseDir string
	notify  func(ref string)
	baseCtx context.Context // vòng đời serve; lượt cài nền dừng khi serve tắt

	mu         sync.Mutex
	started    bool // đã có lượt chạy trong process này (phân biệt idle)
	run        *run
	lastErrors map[domain.SetupStepID]string
	wg         sync.WaitGroup // lượt cài nền đang chạy
}

// New dựng service. ctx là vòng đời serve — lượt cài nền dừng khi serve tắt;
// baseDir là thư mục dữ liệu để đo dung lượng; notify nhận ref "" khi trạng
// thái setup đổi (nil = không phát).
func New(ctx context.Context, t Tools, baseDir string, notify func(ref string)) *Service {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Service{tools: t, baseDir: baseDir, notify: notify, baseCtx: ctx,
		lastErrors: map[domain.SetupStepID]string{}}
}

func (s *Service) emit() {
	if s.notify != nil {
		s.notify("")
	}
}

// Status render hình hiện tại. fresh=true probe lại công cụ thật.
func (s *Service) Status(ctx context.Context, fresh bool) (Status, error) {
	checks, err := s.tools.StepChecks(ctx, fresh)
	if err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	r := s.run
	started := s.started
	errs := s.lastErrors
	if r != nil {
		errs = r.errors
	}
	// Sao chép map lỗi dưới khoá — runSteps ghi lỗi đồng thời.
	copied := make(map[domain.SetupStepID]string, len(errs))
	for k, v := range errs {
		copied[k] = v
	}
	current := domain.SetupStepID("")
	if r != nil {
		current = r.current
	}
	s.mu.Unlock()

	steps := make([]Step, 0, len(domain.SetupStepIDs))
	for _, id := range domain.SetupStepIDs {
		check := checks[id]
		st := Step{ID: id, SizeBytes: check.SizeBytes, Provided: check.Provided}
		switch {
		case current == id:
			st.State = StepRunning
			st.Progress = s.tools.LiveProgress(id)
		case check.Need == domain.SetupNeedReady:
			st.State = StepDone
		default:
			st.Error = copied[id]
			if st.Error == "" {
				st.Error = check.Error
			}
			switch {
			case st.Error != "":
				st.State = StepFailed
			case check.Need == domain.SetupNeedLogin:
				st.State = StepNeedsUser
			default:
				st.State = StepPending
			}
		}
		steps = append(steps, st)
	}

	var required, approx int64
	for _, id := range installStepIDs {
		if checks[id].Need != domain.SetupNeedReady && checks[id].Need != "" {
			required += s.tools.RequiredBytes(id)
			approx += s.tools.ApproxBytes(id)
		}
	}
	free := int64(-1)
	if s.baseDir != "" {
		free = tools.FreeDiskBytes(s.baseDir)
	}
	complete := true
	hasRunning, hasFailed, hasLogin := false, false, false
	for _, st := range steps {
		switch st.State {
		case StepDone:
		case StepRunning:
			hasRunning = true
			complete = false
		case StepFailed:
			hasFailed = true
			complete = false
		case StepNeedsUser:
			hasLogin = true
			complete = false
		default:
			complete = false
		}
	}
	overall := OverallDone
	switch {
	case !started && r == nil && !hasFailed && !complete:
		// Máy sạch, chưa từng chạy: màn bấm Bắt đầu (POST) — GET không tự chạy.
		overall = OverallIdle
	case complete:
		overall = OverallDone
	case hasRunning || r != nil:
		overall = OverallRunning
	case hasFailed:
		overall = OverallFailed
	case hasLogin:
		overall = OverallLogin
	}
	return Status{
		State:    overall,
		Complete: complete,
		Running:  r != nil,
		Steps:    steps,
		Disk: Disk{FreeBytes: free, RequiredBytes: required, Enough: free < 0 || free >= required,
			ApproxBytes: approx},
	}, nil
}

// Shutdown chờ mọi lượt cài nền kết thúc (tối đa theo ctx) để tiến trình con
// được dọn trước khi tầng lưu trữ đóng.
func (s *Service) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start chạy lượt cài nền cho mọi bước thiếu; gọi lại gộp vào lượt đang
// chạy. Trả hình mới nhất (không chờ lượt xong).
func (s *Service) Start(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return s.Status(ctx, false)
	}
	s.started = true
	s.mu.Unlock()
	s.kick("")
	return s.Status(ctx, false)
}

// Retry chạy lại đúng một bước (Thử lại trên hàng lỗi); join lượt đang chạy
// nếu có. Trả hình mới nhất.
func (s *Service) Retry(ctx context.Context, id domain.SetupStepID) (Status, error) {
	if !isInstallStep(id) {
		return Status{}, fmt.Errorf("bước %q không phải cài đặt", id)
	}
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	s.kick(id)
	return s.Status(ctx, false)
}

// kick khởi một lượt cài nền nếu chưa có (only rỗng = mọi bước thiếu).
func (s *Service) kick(only domain.SetupStepID) {
	s.mu.Lock()
	if s.run != nil {
		s.mu.Unlock()
		return
	}
	r := &run{errors: map[domain.SetupStepID]string{}, done: make(chan struct{}), only: only}
	s.run = r
	s.wg.Add(1)
	s.mu.Unlock()
	s.emit()
	go func() {
		defer s.wg.Done()
		s.runSteps(r)
	}()
}

// runSteps cài lần lượt các bước thiếu; lỗi một bước không chặn bước khác —
// đúng hành vi của bản cũ.
func (s *Service) runSteps(r *run) {
	defer func() {
		s.mu.Lock()
		s.lastErrors = r.errors
		s.run = nil
		s.mu.Unlock()
		close(r.done)
		s.emit()
	}()
	targets := installStepIDs
	if r.only != "" {
		targets = []domain.SetupStepID{r.only}
	}
	for _, id := range targets {
		checks, err := s.tools.StepChecks(s.baseCtx, false)
		if err != nil {
			s.setError(r, id, err.Error())
			continue
		}
		if checks[id].Need == domain.SetupNeedReady {
			continue
		}
		s.setCurrent(r, id)
		err = s.tools.Ensure(s.baseCtx, id)
		s.setCurrent(r, "")
		if err != nil {
			s.setError(r, id, err.Error())
			s.emit()
			continue
		}
		after, aerr := s.tools.StepChecks(s.baseCtx, true)
		if aerr == nil && after[id].Need != domain.SetupNeedReady {
			s.setError(r, id, firstErr(after[id].Error, "đã cài nhưng công cụ vẫn chưa đạt kiểm tra"))
			s.emit()
		}
	}
}

func (s *Service) setCurrent(r *run, id domain.SetupStepID) {
	s.mu.Lock()
	r.current = id
	s.mu.Unlock()
	s.emit()
}

// setError ghi lỗi của một bước dưới khoá (Status đọc map này).
func (s *Service) setError(r *run, id domain.SetupStepID, msg string) {
	s.mu.Lock()
	r.errors[id] = msg
	s.mu.Unlock()
}

func firstErr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func isInstallStep(id domain.SetupStepID) bool {
	for _, s := range installStepIDs {
		if s == id {
			return true
		}
	}
	return false
}
