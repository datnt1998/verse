// Ca test cho seam yt-dlp của Video nền: callback dòng nil không panic, cookie
// Bilibili được cấp qua tệp tạm đúng host, và listener NOTIFY đánh thức bộ
// lập lịch chéo process.
package main

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type fakeYtdlp struct {
	mu    sync.Mutex
	opts  *tools.RunYtDlpOptions
	calls int
}

func (f *fakeYtdlp) IsInstalled() bool { return true }

func (f *fakeYtdlp) Run(_ context.Context, _ []string, opts tools.RunYtDlpOptions) (string, error) {
	f.mu.Lock()
	f.calls++
	f.opts = &opts
	f.mu.Unlock()
	if opts.OnLine != nil {
		opts.OnLine("[download]  50% of 1MiB", tools.StreamStdout)
	}
	return `{"title":"x"}`, nil
}

type fakeJar struct {
	calls int
	host  string
}

func (f *fakeJar) WithBilibiliCookieJar(_ context.Context, rawURL string, fn func(jar string) error) error {
	f.calls++
	f.host = rawURL
	return fn("/tmp/fake-cookie.jar")
}

// TestYtdlpAdaptersAreNilSafeAndCookieAware: callback dòng nil không panic,
// dòng được chuyển khi có callback, và chỉ Bilibili nhận tệp cookie.
func TestYtdlpAdaptersAreNilSafeAndCookieAware(t *testing.T) {
	runner := &fakeYtdlp{}
	jar := &fakeJar{}
	run := makeBackdropYtdlpRun(runner, func() *platforms.Pacer { return nil }, jar)

	// Không có callback: adapter đăng ký nil với tools, không panic.
	if _, err := run(context.Background(), domain.PlatformTwitch, []string{"-J", "https://www.twitch.tv/x"}, 1, time.Second, nil); err != nil {
		t.Fatalf("twitch nil onLine: %v", err)
	}
	runner.mu.Lock()
	if runner.opts.OnLine != nil {
		t.Fatal("onLine nil phải truyền nil, không bọc closure gọi nil")
	}
	if runner.opts.CookieFile != "" || jar.calls != 0 {
		t.Fatalf("Twitch không dùng cookie: file=%q jarCalls=%d", runner.opts.CookieFile, jar.calls)
	}
	runner.mu.Unlock()

	var lines []string
	if _, err := run(context.Background(), domain.PlatformBilibili, []string{"-J", "https://www.bilibili.com/video/BV1xx411c7mD"}, 1, time.Second, func(line string) {
		lines = append(lines, line)
	}); err != nil {
		t.Fatalf("bilibili: %v", err)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.opts.OnLine == nil {
		t.Fatal("có callback thì phải chuyển xuống tools")
	}
	if jar.calls != 1 || jar.host != "https://www.bilibili.com/video/BV1xx411c7mD" {
		t.Fatalf("cookie jar = %+v", jar)
	}
	if runner.opts.CookieFile != "/tmp/fake-cookie.jar" {
		t.Fatalf("CookieFile = %q", runner.opts.CookieFile)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %v", lines)
	}
	_ = tools.StreamStdout
}

// TestBackdropWakeListenerReceivesNotify: LISTEN nhận NOTIFY từ process khác
// (một kết nối riêng), có gọi wake ngay sau khi đăng ký để bù NOTIFY trước đó.
func TestBackdropWakeListenerReceivesNotify(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("thiếu TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Huỷ ctx TRƯỚC khi Close: listener đang giữ một kết nối, Close sẽ chờ
	// tới khi nó nhả.
	defer pool.Close()
	defer cancel()

	wakes := make(chan struct{}, 8)
	notified := func() {
		select {
		case wakes <- struct{}{}:
		default:
		}
	}
	go runBackdropWakeListener(ctx, pool, notified, nil)

	// Lần wake đầu = đã LISTEN xong.
	select {
	case <-wakes:
	case <-time.After(5 * time.Second):
		t.Fatal("listener không báo sẵn sàng")
	}
	if _, err := pool.Exec(ctx, "SELECT pg_notify('backdrop_changed', 'twitch:a')"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-wakes:
	case <-time.After(5 * time.Second):
		t.Fatal("không nhận NOTIFY")
	}
}
