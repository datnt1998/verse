// Package pipeline owns fenced clone execution and its shared step context.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
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

type ProcessSpec struct {
	Executable string
	Args       []string
	Dir        string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
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
	ProcessRunner   func(context.Context, ProcessSpec) error
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
func (s *StepContext) RunProcess(ctx context.Context, p ProcessSpec) error {
	return s.ProcessRunner(ctx, p)
}
func (s *StepContext) WithImageSlot(ctx context.Context, run func(context.Context) error) error {
	return s.ImageSlot(ctx, run)
}
func (s *StepContext) PauseRequested() bool { return s.IsPaused != nil && s.IsPaused() }
