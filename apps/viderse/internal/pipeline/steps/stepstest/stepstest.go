// Package stepstest dựng đồ giả dùng chung cho test của các step handler:
// pipeline.Media fake (path → file trên disk), StepContext tối thiểu và các
// seam inject phổ biến. Chỉ nên import từ *_test.go.
package stepstest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

// FakeMedia là pipeline.Media trên filesystem: Publish sao chép Artifact.File
// vào <root>/<Path>, Acquire trả Handle đọc file đó. Acquire của path chưa
// publish → os.ErrNotFound.
type FakeMedia struct {
	Root string
	mu   sync.Mutex
	// Commits ghi nhận commit cb được gọi trong lần Publish (để test chứng
	// minh publish+checkpoint chạy cùng lúc).
	Commits int
	// FailPublish nếu đặt, Publish trả lỗi này (để test crash giữa publish).
	FailPublish error
	// Published liệt kê path đã publish theo thứ tự.
	Published []string
}

// Publish ghi file artifact vào root rồi gọi commit (cùng một lần gọi — đủ
// để test thứ tự, không cần tx thật).
func (m *FakeMedia) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailPublish != nil {
		return m.FailPublish
	}
	for _, a := range artifacts {
		dst := filepath.Join(m.Root, filepath.FromSlash(a.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(a.File)
		if err != nil {
			return err
		}
		tmp := dst + ".part"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		m.Published = append(m.Published, a.Path)
	}
	sort.Strings(m.Published)
	if commit != nil {
		m.Commits++
		return commit(ctx)
	}
	return nil
}

// Acquire trả handle đọc file đã publish.
func (m *FakeMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := filepath.Join(m.Root, filepath.FromSlash(path))
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	return &media.Handle{Path: p, Ref: media.Ref{Path: path}}, nil
}

// Put ghi một artifact đầu vào như thể upstream đã publish (cho test
// dựng đầu vào).
func (m *FakeMedia) Put(path string, data []byte) {
	dst := filepath.Join(m.Root, filepath.FromSlash(path))
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	_ = os.WriteFile(dst, data, 0o644)
	m.Published = append(m.Published, path)
}

// Ctx dựng StepContext tối thiểu. Điền thêm field khi test cần
// (Checkpoint, Upstream, ImageSlot, ProcessRunner…).
func Ctx(t TestingT, step domain.CloneStep, job domain.CloneJob) (*pipeline.StepContext, *FakeMedia) {
	t.Helper()
	staging := t.TempDir()
	m := &FakeMedia{Root: t.TempDir()}
	s := &pipeline.StepContext{
		Job:            job,
		Step:           domain.CloneStepRecord{JobID: job.ID, Step: step, Status: domain.CloneRunning, Attempt: 1},
		StagingDir:     staging,
		Upstream:       map[domain.CloneStep]domain.CloneStepOutput{},
		Media:          m,
		ReportProgress: func(context.Context, domain.CloneProgress) error { return nil },
		ImageSlot:      func(ctx context.Context, run func(context.Context) error) error { return run(ctx) },
		IsPaused:       func() bool { return false },
	}
	s.WriteCheckpoint = func(_ context.Context, b json.RawMessage) error {
		s.Checkpoint = append(json.RawMessage(nil), b...)
		return nil
	}
	return s, m
}

// Job trả một CloneJob tối thiểu cho test.
func Job(id string) domain.CloneJob {
	return domain.CloneJob{ID: id, ChannelID: "ch-test", Status: domain.CloneRunning, Generation: 1}
}

// CheckpointBytes trả checkpoint hiện tại của ctx.
func CheckpointBytes(s *pipeline.StepContext) []byte {
	return s.Checkpoint
}

// TestingT là phần testing.T mà helper cần.
type TestingT interface {
	Helper()
	TempDir() string
}
