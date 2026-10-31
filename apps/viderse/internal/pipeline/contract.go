// Package pipeline owns fenced clone execution and its shared step context.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

var ErrFence = errors.New("lượt chạy đã được thay thế")
var ErrPaused = errors.New("đã tạm dừng")
var ErrNotFound = errors.New("không tìm thấy video")
var ErrConflict = errors.New("video đang được xử lý")

type StepHandler interface {
	Run(context.Context, *StepContext) (StepResult, error)
}
type HandlerFunc func(context.Context, *StepContext) (StepResult, error)

func (f HandlerFunc) Run(ctx context.Context, s *StepContext) (StepResult, error) { return f(ctx, s) }

type Registry map[domain.CloneStep]StepHandler

// ProcessResult là kết quả một lần chạy process con (đuôi stdout/stderr có trần).
type ProcessResult = tools.Result

// ProcessRunner chạy process con trong nhóm riêng: huỷ ctx giết cả nhóm.
// Runner ghi pgid lên clone_steps.process_group khi spec.OnStart được đặt.
// Steps dùng trực tiếp cho argv thô, hoặc truyền làm tools.Runner khi dựng
// tools.YtDlp/tools.Ffmpeg để giữ phân loại lỗi + OnLine.
type ProcessRunner func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error)

func (r ProcessRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return r(ctx, file, args, spec)
}

type StepResult struct {
	Artifacts []media.Artifact
	// Step-specific structured manifests have intentionally heterogeneous values.
	Data    map[string]any
	Waiting *domain.CloneError
}
type Media interface {
	Acquire(context.Context, string) (*media.Handle, error)
	Publish(context.Context, []media.Artifact, func(context.Context) error) error
}
type StepContext struct {
	Job  domain.CloneJob
	Step domain.CloneStepRecord
	// StagingDir is stable across retries of a generation; paid output paths must not change.
	StagingDir      string
	Upstream        map[domain.CloneStep]domain.CloneStepOutput
	Checkpoint      json.RawMessage
	Media           Media
	ReportProgress  func(context.Context, domain.CloneProgress) error
	WriteCheckpoint func(context.Context, json.RawMessage) error
	ProcessRunner   ProcessRunner
	ImageSlot       func(context.Context, func(context.Context) error) error
	IsPaused        func() bool
}

func (s *StepContext) Progress(ctx context.Context, p domain.CloneProgress) error {
	if p.UnitsDone < 0 || (p.UnitsTotal != nil && (*p.UnitsTotal < 0 || p.UnitsDone > *p.UnitsTotal)) {
		return errors.New("tiến độ không hợp lệ")
	}
	return s.ReportProgress(ctx, p)
}

// SaveCheckpoint accepts any JSON-serializable step-owned manifest at this boundary.
func (s *StepContext) SaveCheckpoint(ctx context.Context, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = s.WriteCheckpoint(ctx, b); err == nil {
		s.Checkpoint = b
	}
	return err
}
func (s *StepContext) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	return s.Media.Acquire(ctx, path)
}
func (s *StepContext) Publish(ctx context.Context, a []media.Artifact, commit func(context.Context) error) error {
	return s.Media.Publish(ctx, a, commit)
}

// RunProcess chạy một process con theo ctx của step: huỷ step giết cả nhóm
// process. Khi spec.OnStart được đặt, pgid của nhóm được ghi lên
// clone_steps.process_group để recovery dọn nhóm mồ côi.
func (s *StepContext) RunProcess(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return s.ProcessRunner(ctx, file, args, spec)
}
func (s *StepContext) WithImageSlot(ctx context.Context, run func(context.Context) error) error {
	return s.ImageSlot(ctx, run)
}
func (s *StepContext) PauseRequested() bool { return s.IsPaused != nil && s.IsPaused() }

// ClaimedStep là step vừa được claim bởi worker kèm fence của attempt và
// output của các step upstream đã xong.
type ClaimedStep struct {
	Step     domain.CloneStepRecord
	Fence    int64
	Upstream map[domain.CloneStep]domain.CloneStepOutput
}

// QueueDrain là điểm gọi của pipeline vào hàng chờ (internal/pipeline/queue):
// RequestDrain xếp lịch một vòng drain (debounce bên trong queue), gọi sau khi
// job đỗ/settle và khi mục mới vào hàng. Impl do serve wire.
type QueueDrain interface {
	RequestDrain()
}

// JobRunner là điểm gọi của hàng chờ vào worker: Start tạo/ghi nhận job cho
// một mục (đã qua kiểm của drain) rồi báo worker có việc mới — job chạy ngay
// nếu còn lane. Impl do serve wire.
type JobRunner interface {
	Start(ctx context.Context, item domain.CloneQueueItem) (*domain.CloneJob, error)
	Wake()
}

// Scheduler là vòng điều phối của hàng chờ trong serve: drain tick (safety
// net ~60s) + discovery tick (~15 phút) chạy trong process giữ advisory lock.
// Queue implement; serve khởi động/dừng cùng worker.
type Scheduler interface {
	Start(ctx context.Context) error
	Stop() error
}
