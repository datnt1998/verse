package main

// Ráp mảng Video nền: store, service, route HTTP, bộ lập lịch đọc dần Bilibili
// và adapter route /api/templates/{id}/backdrop/mix (Thư viện sở hữu tiền tố,
// chuyển tiếp sang đây). serve gọi mountBackdrop TRƯỚC mountPlatforms để route
// chính xác POST /api/platforms/requests thắng tiền tố /api/platforms/; Bind
// sau mountPlatforms để đặt nhịp/hạn mức/lease cho service đã dựng.

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/webapp"
)

// newBackdropMix dựng bộ đọc "Nền của một video" cho route mix của template.
// Thư viện gọi qua web.TemplateBackdropMix; pool nil → nil (route trả 404).
func newBackdropMix(pool *pgxpool.Pool) web.TemplateBackdropMix {
	if pool == nil {
		return nil
	}
	return &backdropMix{reader: backdrop.NewMixReader(postgres.NewBackdropStore(pool))}
}

type backdropMix struct {
	reader *backdrop.MixReader
}

// ServeMix trả GET /api/templates/{id}/backdrop/mix?lengthSec=N.
func (m *backdropMix) ServeMix(w http.ResponseWriter, r *http.Request, templateID string) {
	lengthSec := 0
	if raw := r.URL.Query().Get("lengthSec"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": "Độ dài video không hợp lệ", "code": backdrop.CodeInvalidInput,
			})
			return
		}
		lengthSec = value
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	report, err := m.reader.Mix(ctx, templateID, lengthSec)
	if err != nil {
		web.WriteBackdropError(w, err, nil)
		return
	}
	web.WriteBackdropMix(w, r, report)
}

// backdropRuntime giữ vòng đời của mảng Video nền: service, bộ lập lịch và
// các việc nền tải video. Close huỷ ctx rồi join trước khi pool đóng.
type backdropRuntime struct {
	svc   *backdrop.Service
	pool  *pgxpool.Pool
	ctx   context.Context
	stop  context.CancelFunc
	pacer atomic.Pointer[platforms.Pacer]
	wg    sync.WaitGroup
	once  sync.Once
}

// mountBackdrop dựng store/service, gắn route và đăng ký hook đánh thức bộ
// lập lịch khi mảng nền tảng phát sự kiện (đăng nhập mới, gỡ chặn).
func mountBackdrop(
	g *web.Gateway,
	pool *pgxpool.Pool,
	cfgDir string,
	hub *events.Hub,
	activities *activity.Service,
	storageSvc *storage.Service,
	logStore *logs.Store,
	ytdlp ytdlpRunner,
) *backdropRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	r := &backdropRuntime{pool: pool, ctx: ctx, stop: cancel}

	acc := accounts.New(accounts.Deps{Dir: cfgDir, Notify: func(string) {}})
	run := makeBackdropYtdlpRun(ytdlp, func() *platforms.Pacer { return r.pacer.Load() }, acc)
	r.svc = backdrop.New(backdrop.Deps{
		Store:    postgres.NewBackdropStore(pool),
		RunYtDlp: run,
		CfgDir:   cfgDir,
		// Bilibili coi mọi lời gọi là một trình duyệt; timeout 30 giây cho
		// trang danh mục thường (pacing đã giới hạn nhịp).
		Doer:   &http.Client{Timeout: 30 * time.Second},
		Cookie: acc.CookieHeader,
		Now:    time.Now,
		Random: rand.Float64,
		Tasks:  activities,
		Spawn: func(spawnCtx context.Context, fn func(context.Context)) {
			r.wg.Add(1)
			go func() { defer r.wg.Done(); fn(spawnCtx) }()
		},
		Background:  func() context.Context { return ctx },
		StorageRoot: storageSvc.Root,
		ResolvePath: storageSvc.Resolve,
		Notify:      web.Notifier(hub, "backdrop"),
	})
	web.MountBackdropDiag(g, r.svc, func(msg string) {
		logStore.Append("backdrop", logs.LevelError, logStore.Redact(msg))
	})
	// Đăng nhập mới / gỡ chặn phát sự kiện "platforms": bộ lập lịch tính lại
	// mốc kế tiếp ngay thay vì ngủ tới mốc cũ.
	hub.OnPublish(func(ev events.Event) {
		if ev.Type == "platforms" {
			r.svc.Wake()
		}
	})
	return r
}

// Bind gắn nhịp, hạn mức, lease và cookie sau khi mảng nền tảng đã dựng, rồi
// chạy bộ lập lịch. Bộ đếm nguồn của /api/platforms cộng thêm số kênh nền và
// video lẻ.
func (r *backdropRuntime) Bind(platformSvc *platforms.Service) {
	pacer := platformSvc.NewPacer(nil, nil)
	r.pacer.Store(pacer)
	r.svc.Bind(backdrop.BindDeps{
		Platforms: platformSvc,
		Pacer:     pacer,
		Request: func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error {
			return pacer.Request(ctx, domain.PlatformBilibili, resetBackoff, fn)
		},
	})
	platformSvc.SetUsage(platforms.ComposeUsage(newRivalsUsage(r.pool), r.svc))
	// Process khác (CLI) thêm/gỡ kênh hoặc đặt mốc đọc dần sẽ NOTIFY; nghe để
	// bộ lập lịch tính lại mốc ngay, không phải hỏi vòng DB.
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		runBackdropWakeListener(r.ctx, r.pool, r.svc.Wake, nil)
	}()
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.svc.RunScheduler(r.ctx)
	}()
}

// runBackdropWakeListener giữ một kết nối LISTEN backdrop_changed và gọi wake
// mỗi lần có NOTIFY. Gọi wake ngay sau khi LISTEN để bù thông báo phát trước
// lúc đăng ký; mất kết nối thì thử lại sau một giây (ctx huỷ thì dừng).
func runBackdropWakeListener(ctx context.Context, pool *pgxpool.Pool, wake func(), logf func(string)) {
	for {
		if ctx.Err() != nil {
			return
		}
		conn, err := pool.Acquire(ctx)
		if err != nil {
			if !sleepBrief(ctx) {
				return
			}
			continue
		}
		if _, err := conn.Exec(ctx, "LISTEN backdrop_changed"); err != nil {
			conn.Release()
			if logf != nil {
				logf(err.Error())
			}
			if !sleepBrief(ctx) {
				return
			}
			continue
		}
		wake()
		for {
			if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
				break
			}
			wake()
		}
		conn.Release()
		if ctx.Err() != nil {
			return
		}
		if logf != nil {
			logf("mất kết nối lắng nghe thay đổi Video nền; thử lại")
		}
		if !sleepBrief(ctx) {
			return
		}
	}
}

func sleepBrief(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(time.Second):
		return true
	}
}

// Close huỷ ctx và join bộ lập lịch + việc nền tải video, có trần chờ để
// shutdown không treo nếu một tác vụ bám tài nguyên ngoài.
func (r *backdropRuntime) Close() {
	r.once.Do(func() {
		r.stop()
		done := make(chan struct{})
		go func() {
			defer close(done)
			r.wg.Wait()
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	})
}

// cookieJarProvider cấp tệp cookie Netscape tạm cho một lần chạy yt-dlp.
type cookieJarProvider interface {
	WithBilibiliCookieJar(ctx context.Context, rawURL string, fn func(jar string) error) error
}

// ytdlpLineSink đổi callback dòng của mảng sang callback của tools; nil ở phía
// gọi thì không đăng ký callback (tools không gọi nil), tránh panic trong
// goroutine đọc pipe khi probe/list không cần tiến độ.
func ytdlpLineSink(onLine func(string)) func(line string, _ tools.Stream) {
	if onLine == nil {
		return nil
	}
	return func(line string, _ tools.Stream) { onLine(line) }
}

// ytdlpInvoke chạy một lệnh yt-dlp, kèm tệp cookie Bilibili khi URL thuộc
// Bilibili (tệp tạm 0600, tự xoá sau khi lệnh xong).
func ytdlpInvoke(ctx context.Context, ytdlp ytdlpRunner, cookies cookieJarProvider, platform domain.PlatformID, args []string, timeout time.Duration, onLine func(string)) (string, error) {
	run := func(jar string) (string, error) {
		return ytdlp.Run(ctx, args, tools.RunYtDlpOptions{
			Timeout:    timeout,
			OnLine:     ytdlpLineSink(onLine),
			CookieFile: jar,
		})
	}
	if platform != domain.PlatformBilibili || cookies == nil {
		return run("")
	}
	rawURL := ""
	for i := len(args) - 1; i >= 0; i-- {
		if strings.HasPrefix(args[i], "http://") || strings.HasPrefix(args[i], "https://") {
			rawURL = args[i]
			break
		}
	}
	var stdout string
	err := cookies.WithBilibiliCookieJar(ctx, rawURL, func(jar string) error {
		out, err := run(jar)
		stdout = out
		return err
	})
	return stdout, err
}

// makeBackdropYtdlpRun ráp nhịp, callback dòng và cookie thành RunYtDlp của
// mảng Video nền; pacer đọc muộn vì mảng nền tảng dựng sau.
func makeBackdropYtdlpRun(ytdlp ytdlpRunner, pacer func() *platforms.Pacer, cookies cookieJarProvider) backdrop.RunYtDlpFunc {
	return func(callCtx context.Context, platform domain.PlatformID, args []string, cost int, timeout time.Duration, onLine func(string)) (string, error) {
		var stdout string
		invoke := func(callCtx context.Context) error {
			out, err := ytdlpInvoke(callCtx, ytdlp, cookies, platform, args, timeout, onLine)
			stdout = out
			return err
		}
		if p := pacer(); p != nil {
			return stdout, p.RunYtDlp(callCtx, platform, cost, timeout, invoke)
		}
		err := invoke(callCtx)
		return stdout, err
	}
}
