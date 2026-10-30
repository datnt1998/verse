// Nguồn: yt-factory src/server/retention.ts (dọn thư mục tạm phải bỏ qua
// scratch của lần vẽ ảnh còn chạy) — đọc scratch từ biên nhận thật.
//
// Chạy trên Postgres thật: provision schema riêng bằng pgtest rồi áp migration
// nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

// storageAdmission là một lượt vẽ ảnh không giới hạn trần ngày.
func storageAdmission(key string) imagegen.Admission {
	return imagegen.Admission{
		Entry:   imagegen.JournalEntry{Key: key, Provider: "codex", Model: "m", Output: "/tmp/a.png"},
		PaidCap: -1,
	}
}

func timeNow() time.Time { return time.Now() }

func TestStorageStoreScratchDirsOnlyRunning(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)

	if _, err := store.Begin(ctx, storageAdmission("draw-running")); err != nil {
		t.Fatalf("begin running: %v", err)
	}
	if err := store.UpdateScratch(ctx, "draw-running", 1,
		[]byte(`{"work":"/tmp/viderse-image-call-1","home":"/tmp/codex-home-1"}`)); err != nil {
		t.Fatalf("scratch: %v", err)
	}
	if _, err := store.Begin(ctx, storageAdmission("draw-done")); err != nil {
		t.Fatalf("begin done: %v", err)
	}
	if err := store.UpdateScratch(ctx, "draw-done", 1,
		[]byte(`{"work":"/tmp/viderse-image-call-2"}`)); err != nil {
		t.Fatalf("scratch done: %v", err)
	}
	if err := store.Finish(ctx, "draw-done", 1, "ok", true, nil, "", "", timeNow()); err != nil {
		t.Fatalf("finish: %v", err)
	}

	dirs, err := postgres.NewStorageStore(pool).ScratchDirs(ctx)
	if err != nil {
		t.Fatalf("ScratchDirs: %v", err)
	}
	got := map[string]bool{}
	for _, dir := range dirs {
		got[dir] = true
	}
	if !got["/tmp/viderse-image-call-1"] || !got["/tmp/codex-home-1"] {
		t.Fatalf("thiếu scratch của lần chạy: %v", dirs)
	}
	if got["/tmp/viderse-image-call-2"] {
		t.Fatalf("lần đã xong không được bảo vệ: %v", dirs)
	}
}
