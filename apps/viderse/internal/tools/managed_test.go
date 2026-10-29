// Test chuyển từ src/server/tools/managed-tools.test.ts của yt-factory.
package tools

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestManagedToolIdsIsExactlyTheToolList(t *testing.T) {
	want := []string{"ffmpeg", "uv", "transcription", "codex", "chrome"}
	if len(managedToolIDs) != len(want) {
		t.Fatalf("managedToolIDs = %v", managedToolIDs)
	}
	for i, id := range managedToolIDs {
		if string(id) != want[i] {
			t.Fatalf("managedToolIDs = %v", managedToolIDs)
		}
	}
	if !IsManagedToolID("ffmpeg") || IsManagedToolID("ffmpeg ") || IsManagedToolID("") {
		t.Fatal("IsManagedToolID sai")
	}
}

func TestManagedInstallRejectsUnknownTool(t *testing.T) {
	base := t.TempDir()
	tools := New(Deps{Runner: &ExecRunner{}, BaseDir: base})
	m := NewManaged(Deps{Runner: &ExecRunner{}, BaseDir: base}, nil, tools)
	if _, err := m.Install(context.Background(), ManagedToolID("curl")); err == nil {
		t.Fatal("không từ chối công cụ lạ")
	}
	if err := m.Cancel("curl"); err == nil {
		t.Fatal("không từ chối huỷ công cụ lạ")
	}
	if _, err := m.State("curl"); err == nil {
		t.Fatal("không từ chối state công cụ lạ")
	}
	if err := m.SetPath("curl", "/usr/bin/curl"); err == nil {
		t.Fatal("không từ chối đường dẫn công cụ lạ")
	}
}

func TestManagedSetPathRejectsToolWithoutSettingKey(t *testing.T) {
	base := t.TempDir()
	tools := New(Deps{Runner: &ExecRunner{}, BaseDir: base})
	m := NewManaged(Deps{Runner: &ExecRunner{}, BaseDir: base}, nil, tools)
	if err := m.SetPath(ManagedChrome, "/usr/bin/chromium"); err == nil {
		t.Fatal("chrome không có key cấu hình vẫn nhận đường dẫn")
	}
}

func TestManagedLifecycleInstallRunsRegisteredBody(t *testing.T) {
	base := t.TempDir()
	deps := Deps{Runner: &ExecRunner{}, BaseDir: base}
	tools := New(deps)
	m := NewManaged(deps, nil, tools)
	ran := false
	m.RegisterLifecycle(ManagedTranscription, func(ctx context.Context, lc *LifecycleContext) error {
		ran = true
		return nil
	})
	res, err := m.Install(context.Background(), ManagedTranscription)
	if err != nil {
		t.Fatal(err)
	}
	if !ran || res.ToolID != "transcription" {
		t.Fatalf("res = %+v ran = %v", res, ran)
	}
}

func TestManagedLifecycleInstallPropagatesFailure(t *testing.T) {
	base := t.TempDir()
	deps := Deps{Runner: &ExecRunner{}, BaseDir: base}
	tools := New(deps)
	m := NewManaged(deps, nil, tools)
	m.RegisterLifecycle(ManagedTranscription, func(ctx context.Context, lc *LifecycleContext) error {
		return errors.New("venv dựng hỏng")
	})
	if _, err := m.Install(context.Background(), ManagedTranscription); err == nil {
		t.Fatal("lỗi lifecycle không lan ra")
	}
	st, err := m.State(ManagedTranscription)
	if err != nil {
		t.Fatal(err)
	}
	if st == nil || st.Phase != PhaseFailed {
		t.Fatalf("state = %+v", st)
	}
}

// Hai caller Install cùng lúc chia đúng một lượt lifecycle: thân run chỉ chạy
// một lần, cả hai nhận cùng kết quả, và install-state.json đóng ở done.
func TestManagedLifecycleConcurrentCallersShareRun(t *testing.T) {
	base := t.TempDir()
	deps := Deps{Runner: &ExecRunner{}, BaseDir: base}
	tools := New(deps)
	m := NewManaged(deps, nil, tools)
	var runs int32
	release := make(chan struct{})
	m.RegisterLifecycle(ManagedTranscription, func(ctx context.Context, lc *LifecycleContext) error {
		atomic.AddInt32(&runs, 1)
		<-release
		return nil
	})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			_, errs[k] = m.Install(context.Background(), ManagedTranscription)
		}(i)
	}
	close(release)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("runs = %d, muốn 1", got)
	}
	st, err := m.State(ManagedTranscription)
	if err != nil {
		t.Fatal(err)
	}
	if st == nil || st.Phase != PhaseDone {
		t.Fatalf("state = %+v", st)
	}
}
