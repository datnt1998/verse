package sim

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// GAP-35 / Invariant #7: thông điệp của /simulate hiện lên UI (job, Activity,
// modal TUI) nên không được nhắc tên thư mục nguồn hay đường dẫn.
func TestUserMessagesNeverMentionSourceFolder(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "simulate")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := store.NewStore(filepath.Join(dir, "output", "novel"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}

	// Thư mục rỗng: lỗi phải là câu tiếng Việt thuần, không lộ tên thư mục.
	events, err := Run(context.Background(), Deps{Store: st, LLM: &scriptedLLM{}, Prompts: Prompts{Source: "s", Merge: "m"}}, Options{SourceDir: sourceDir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var last Event
	for ev := range events {
		assertPlainMessage(t, ev.Message)
		if ev.Err != nil {
			assertPlainMessage(t, ev.Err.Error())
		}
		last = ev
	}
	if last.Stage != StageError || last.Err == nil {
		t.Fatalf("empty corpus: stage=%s err=%v", last.Stage, last.Err)
	}
	if !strings.Contains(last.Message, "Chưa có văn mẫu") {
		t.Fatalf("empty corpus message = %q", last.Message)
	}

	// Thư mục chưa tồn tại: cùng quy tắc (đây là lỗi người dùng gặp khi chưa thêm mẫu).
	if _, err := scanSources(filepath.Join(dir, "chưa-có")); err == nil {
		t.Fatal("missing corpus must error")
	} else {
		assertPlainMessage(t, err.Error())
	}
}

// Tên văn mẫu hiện trên thanh tiến độ chỉ là tên bài, không kèm thư mục con.
func TestSourceDisplayNameHidesFolders(t *testing.T) {
	for rel, want := range map[string]string{
		"a.txt":                 "a",
		"nested/b.md":           "b",
		"nested/sâu/c.markdown": "c",
	} {
		if got := sourceDisplayName(rel); got != want {
			t.Fatalf("sourceDisplayName(%q) = %q, want %q", rel, got, want)
		}
	}
}

func assertPlainMessage(t *testing.T, msg string) {
	t.Helper()
	lower := strings.ToLower(msg)
	for _, banned := range []string{"simulate", ".txt", ".md", "/", "\\"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("user-facing message leaks %q: %q", banned, msg)
		}
	}
}
