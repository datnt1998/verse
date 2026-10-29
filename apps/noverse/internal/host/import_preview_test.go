package host

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/host/imp"
	"github.com/datnt1998/noverse/internal/llmretry"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

func TestImportPreviewRequiresConfiguredModel(t *testing.T) {
	readHost := newReadHost(bootstrap.Config{OutputDir: t.TempDir()}, nil)
	for _, h := range []*Host{nil, readHost, {models: &bootstrap.ModelSet{}}} {
		_, err := h.ImportPreview(context.Background(), imp.Options{Guidance: "tách ở dấu sao"})
		if err == nil || !strings.Contains(err.Error(), "Chưa có cấu hình mô hình") {
			t.Fatalf("model-free preview must return Vietnamese configuration error: %v", err)
		}
	}
}

type previewFailureModel struct {
	plainTrackedTestModel
	calls    int
	deadline time.Time
	err      error
}

// retryableNetworkError mirrors agentcore's provider adapter for an unreachable endpoint.
type retryableNetworkError struct{}

func (retryableNetworkError) Error() string {
	return "dial tcp 127.0.0.1:9: connect: connection refused"
}
func (retryableNetworkError) Retryable() bool      { return true }
func (retryableNetworkError) Is(target error) bool { return target == llmtypes.ErrProviderNetwork }

func (m *previewFailureModel) Generate(ctx context.Context, _ []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.calls++
	m.deadline, _ = ctx.Deadline()
	if m.err == nil {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, m.err
}

func newPreviewFailureHost(t *testing.T, model llmtypes.ChatModel) (*Host, string) {
	t.Helper()
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(st.Dir(), "nguon.txt")
	if err := os.WriteFile(path, []byte("Chương 1\nNội dung truyện.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gate := llmretry.NewGate()
	models := &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "preview", model, nil)}
	models.SetRetryGate(gate)
	return &Host{models: models, engine: &engine{}, store: st, styleStats: tools.NewStyleStatsIndex(st), usage: &UsageTracker{}, retryGate: gate}, path
}

func TestImportPreviewBoundsModelContext(t *testing.T) {
	want := errors.New("lỗi yêu cầu cụ thể")
	model := &previewFailureModel{err: want}
	h, path := newPreviewFailureHost(t, model)
	before := time.Now()
	_, err := h.ImportPreview(context.Background(), imp.Options{SourcePath: path})
	after := time.Now()
	if !errors.Is(err, want) {
		t.Fatalf("preview lost request error: %v", err)
	}
	if model.deadline.Before(before.Add(ImportPreviewTimeout)) || model.deadline.After(after.Add(ImportPreviewTimeout)) {
		t.Fatalf("preview model deadline=%v, want start+%v", model.deadline, ImportPreviewTimeout)
	}
	if h.exclusive != "" || h.exclusiveCancel != nil {
		t.Fatal("failed preview retained exclusive slot")
	}
}

func TestImportPreviewProviderFailureDoesNotRetry(t *testing.T) {
	model := &previewFailureModel{err: retryableNetworkError{}}
	h, path := newPreviewFailureHost(t, model)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := h.ImportPreview(ctx, imp.Options{SourcePath: path, Guidance: "giữ nguyên chương"})
	if !errors.Is(err, llmtypes.ErrProviderNetwork) || !strings.HasPrefix(err.Error(), "Không kết nối được nhà cung cấp AI") {
		t.Fatalf("provider failure must fail fast with original cause, got %v", err)
	}
	if model.calls != 1 || h.retryGate.Waiting() != 0 {
		t.Fatalf("preview retried: calls=%d waiting=%d", model.calls, h.retryGate.Waiting())
	}
	state, stateErr := h.ImportState()
	if stateErr != nil || state.Path != "nguon.txt" || state.Options.Guide != "giữ nguyên chương" || state.Preview != nil {
		t.Fatalf("failed preview must retain source/guide but no fabricated preview: %+v, %v", state, stateErr)
	}
	if _, err := os.Stat(filepath.Join(h.store.Dir(), "meta", "import", "source.txt")); err != nil {
		t.Fatalf("failed preview lost source snapshot: %v", err)
	}
}

func TestImportPreviewDeadlineCancelsProviderRequest(t *testing.T) {
	model := &previewFailureModel{}
	h, path := newPreviewFailureHost(t, model)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := h.ImportPreview(ctx, imp.Options{SourcePath: path})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "Không kết nối được nhà cung cấp AI") {
		t.Fatalf("deadline must cancel the request with Vietnamese provider error, got %v", err)
	}
	if model.calls != 1 || h.exclusive != "" {
		t.Fatalf("deadline left preview running: calls=%d exclusive=%q", model.calls, h.exclusive)
	}
}
