// Ca test trạng thái nền tảng trên Postgres thật, chuyển từ
// src/server/backdrop/pacing.test.ts và bilibili-login.test.ts của yt-factory:
// chờ tăng dần 15/30/60/120/120 phút, dọn trường chờ hết hạn, Retry-After kéo
// dài, đặt chỗ gọi theo khoảng cách, lease quét một scanner, lỗi gần nhất.
//
// Chạy trên schema riêng provision bằng pgtest với cặp role của Viderse rồi
// áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func TestPlatformStateEscalatesAndClears(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewPlatformStateStore(provision(t))
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)

	for _, minutes := range []int{15, 30, 60, 120, 120} {
		st, err := store.RecordBlocked(ctx, domain.PlatformBilibili, now, "Bilibili đang giới hạn yêu cầu", 0)
		if err != nil {
			t.Fatalf("record blocked: %v", err)
		}
		want := now.Add(time.Duration(minutes) * time.Minute)
		if st.PausedUntil == nil || !st.PausedUntil.Equal(want) {
			t.Fatalf("until = %v, muốn %v", st.PausedUntil, want)
		}
		until, reason, err := store.Pause(ctx, domain.PlatformBilibili, now)
		if err != nil || until == nil || !until.Equal(want) || reason == nil {
			t.Fatalf("pause = %v %v %v", until, reason, err)
		}
		// Hết giờ chờ đọc lại thì trường chờ được dọn nhưng đếm vẫn còn.
		until, reason, err = store.Pause(ctx, domain.PlatformBilibili, want)
		if err != nil || until != nil || reason != nil {
			t.Fatalf("sau hạn pause = %v %v", until, reason)
		}
		cur, found, err := store.One(ctx, domain.PlatformBilibili)
		if err != nil || !found {
			t.Fatalf("one: %v %v", found, err)
		}
		if cur.FailureCount == 0 {
			t.Fatal("đếm chặn không được xoá khi hết giờ chờ")
		}
	}

	// Đăng nhập mới: xoá cả giờ chờ lẫn đếm để lần chặn sau bắt đầu 15 phút.
	if err := store.ClearPause(ctx, domain.PlatformBilibili); err != nil {
		t.Fatal(err)
	}
	cur, _, _ := store.One(ctx, domain.PlatformBilibili)
	if cur.PausedUntil != nil || cur.PauseReason != nil || cur.FailureCount != 0 {
		t.Fatalf("sau clear = %+v", cur)
	}
	st, err := store.RecordBlocked(ctx, domain.PlatformBilibili, now, "lại bị chặn", 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.PausedUntil == nil || !st.PausedUntil.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("lần chặn mới = %v", st.PausedUntil)
	}
}

func TestPlatformRetryAfterOnlyExtends(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewPlatformStateStore(provision(t))
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)

	st, err := store.RecordBlocked(ctx, domain.PlatformBilibili, now, "chặn", 45*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if st.PausedUntil == nil || !st.PausedUntil.Equal(now.Add(45*time.Minute)) {
		t.Fatalf("until = %v, muốn +45 phút", st.PausedUntil)
	}
	// Retry-After ngắn hơn công thức không được rút ngắn lần chặn thứ hai (30 phút).
	st, err = store.RecordBlocked(ctx, domain.PlatformBilibili, now, "chặn", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if st.PausedUntil == nil || !st.PausedUntil.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("until = %v, muốn +30 phút", st.PausedUntil)
	}
}

func TestPlatformSlotReservesSpacing(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewPlatformStateStore(provision(t))
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	spacing := 5500 * time.Millisecond

	start, err := store.ReserveSlot(ctx, domain.PlatformBilibili, now, spacing)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(now) {
		t.Fatalf("start = %v, muốn %v", start, now)
	}
	start, err = store.ReserveSlot(ctx, domain.PlatformBilibili, now, spacing)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(now.Add(spacing)) {
		t.Fatalf("start thứ hai = %v, muốn %v", start, now.Add(spacing))
	}
	// Twitch không có khoảng cách: start = max(now, next cũ), next không đổi.
	start, err = store.ReserveSlot(ctx, domain.PlatformTwitch, now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(now) {
		t.Fatalf("twitch start = %v", start)
	}
}

func TestPlatformScannerLeaseIsExclusive(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewPlatformStateStore(provision(t))
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)

	ok, err := store.AcquireScanner(ctx, domain.PlatformBilibili, "owner-1", now, now.Add(30*time.Minute))
	if err != nil || !ok {
		t.Fatalf("acquire đầu = %v %v", ok, err)
	}
	ok, err = store.AcquireScanner(ctx, domain.PlatformBilibili, "owner-2", now.Add(time.Minute), now.Add(31*time.Minute))
	if err != nil || ok {
		t.Fatalf("scanner thứ hai phải bị từ chối, được %v %v", ok, err)
	}
	if err := store.ReleaseScanner(ctx, domain.PlatformBilibili, "owner-2"); err != nil {
		t.Fatal(err)
	}
	// Nhả sai owner không được lấy mất lease.
	ok, err = store.AcquireScanner(ctx, domain.PlatformBilibili, "owner-2", now.Add(2*time.Minute), now.Add(32*time.Minute))
	if err != nil || ok {
		t.Fatalf("lease phải còn của owner-1, được %v %v", ok, err)
	}
	// Lease hết hạn thì scanner khác giữ được.
	later := now.Add(31 * time.Minute)
	ok, err = store.AcquireScanner(ctx, domain.PlatformBilibili, "owner-3", later, later.Add(30*time.Minute))
	if err != nil || !ok {
		t.Fatalf("lease hết hạn phải lấy được, được %v %v", ok, err)
	}
	if err := store.ReleaseScanner(ctx, domain.PlatformBilibili, "owner-3"); err != nil {
		t.Fatal(err)
	}
	cur, _, _ := store.One(ctx, domain.PlatformBilibili)
	if cur.ScannerOwner != nil || cur.ScannerLeaseUntil != nil {
		t.Fatalf("sau release = %+v", cur)
	}
}

func TestPlatformErrorRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewPlatformStateStore(provision(t))
	at := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	job := "job-9"

	if err := store.RecordError(ctx, domain.PlatformYouTube, "cookies_failed", at, &job); err != nil {
		t.Fatal(err)
	}
	cur, found, err := store.One(ctx, domain.PlatformYouTube)
	if err != nil || !found {
		t.Fatalf("one: %v %v", found, err)
	}
	if cur.LastErrorCode != "cookies_failed" || cur.LastErrorAt == nil || !cur.LastErrorAt.Equal(at) {
		t.Fatalf("error = %+v", cur)
	}
	if cur.LastErrorJobID == nil || *cur.LastErrorJobID != "job-9" {
		t.Fatalf("jobId = %v", cur.LastErrorJobID)
	}
	all, err := store.All(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("all = %d %v", len(all), err)
	}
	if err := store.ClearError(ctx, domain.PlatformYouTube); err != nil {
		t.Fatal(err)
	}
	cur, _, _ = store.One(ctx, domain.PlatformYouTube)
	if cur.LastErrorCode != "" || cur.LastErrorAt != nil || cur.LastErrorJobID != nil {
		t.Fatalf("sau clear = %+v", cur)
	}
}
