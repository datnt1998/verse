package main

// Ráp luồng quét đối thủ (Trinh sát, Gói V5): youtube client + kho quét + service
// mặt API/CLI + worker nền một goroutine. serve gọi mountScan và truyền service
// cho mảng đối thủ (mutations); dừng worker trước khi đóng pool.
//
// Mọi request YouTube đi qua youtube.Pacer của client dùng chung — worker không
// tự gọi mạng ngoài client. Giờ chờ nền tảng đọc/ghi qua platformGate: đọc bằng
// đúng dữ liệu PlatformStateStore.Pause, ghi bằng platforms.Service.RecordBlocked
// ở serve (để UI Kết nối nhận SSE) hoặc cùng câu SQL đó ở CLI.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/datnt1998/verse/platform/events"
)

// platformGate thoả scan.PlatformGate trên dữ liệu nền tảng bền.
type platformGate struct {
	state   platforms.StateStore
	now     func() time.Time
	blocked func(ctx context.Context, id domain.PlatformID, retryAfter time.Duration) error
}

// Until trả giờ chờ còn hiệu lực của nền tảng (nil = không chờ).
func (g platformGate) Until(ctx context.Context, id domain.PlatformID) (*time.Time, error) {
	until, _, err := g.state.Pause(ctx, id, g.now())
	return until, err
}

// RecordBlocked ghi một lần bị chặn/429.
func (g platformGate) RecordBlocked(ctx context.Context, id domain.PlatformID, retryAfter time.Duration) error {
	return g.blocked(ctx, id, retryAfter)
}

// recordBlockedViaStore là đường ghi của CLI (không có hub/SSE): cùng câu
// reason với platforms.Service.RecordBlocked.
func recordBlockedViaStore(store platforms.StateStore, now func() time.Time) func(context.Context, domain.PlatformID, time.Duration) error {
	return func(ctx context.Context, id domain.PlatformID, retryAfter time.Duration) error {
		d, ok := platforms.Lookup(id)
		if !ok {
			return platforms.ErrPlatformUnknown
		}
		_, err := store.RecordBlocked(ctx, id, now(), d.Label+" đang giới hạn yêu cầu", retryAfter)
		return err
	}
}

// scanServiceDeps dựng phần phụ thuộc bền của luồng quét, dùng chung cho CLI và
// serve. Đây là điểm duy nhất biết youtube client, ScanStore và cổng nền tảng.
func scanServiceDeps(pool *pgxpool.Pool, settings *postgres.SettingsStore, activities *activity.Service) scan.Deps {
	now := time.Now
	state := postgres.NewPlatformStateStore(pool)
	return scan.Deps{
		Store:    postgres.NewScanStore(pool),
		YouTube:  youtube.NewClient(youtube.Options{Pacer: sharedYouTubePacer}),
		Settings: settings,
		Tasks:    activities,
		Platform: platformGate{state: state, now: now, blocked: recordBlockedViaStore(state, now)},
		Now:      now,
	}
}

// newScanService dựng scan.Service không-hub (CLI): notify tắt, Runtime do
// RunInline tự tạo worker ad-hoc bên trong. Đây là điểm vào cho `viderse rival`.
func newScanService(pool *pgxpool.Pool, settings *postgres.SettingsStore, activities *activity.Service) (*scan.Service, error) {
	return scan.New(scanServiceDeps(pool, settings, activities)), nil
}

// mountScan dựng service + worker của serve: SSE "rivals", báo nền tảng qua
// platforms.Service (SSE "platforms"), và đánh thức worker sau mỗi thay đổi từ
// API. Trả service (cho mảng đối thủ) và worker (để serve chạy/dừng nó).
func mountScan(g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore,
	hub *events.Hub, activities *activity.Service, platformsSvc *platforms.Service) (*scan.Service, *scan.Worker, error) {
	_ = g
	deps := scanServiceDeps(pool, settings, activities)
	if platformsSvc != nil {
		state := postgres.NewPlatformStateStore(pool)
		deps.Platform = platformGate{state: state, now: time.Now, blocked: platformsSvc.RecordBlocked}
	}
	svc := scan.New(deps)
	svc.SetNotifier(web.Notifier(hub, "rivals"))
	worker := scan.NewWorker(scan.WorkerDeps{
		Store:    deps.Store,
		YouTube:  deps.YouTube,
		Service:  svc,
		Platform: deps.Platform,
		Now:      time.Now,
	})
	svc.BindWorker(worker)
	return svc, worker, nil
}

// runScanWorker chạy worker nền theo vòng đời serve: ctx huỷ là dừng sạch
// (worker nhả lease, không ghi gì dở dang). Trả kênh đóng khi worker đã join.
func runScanWorker(ctx context.Context, worker *scan.Worker) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Run(ctx)
	}()
	return done
}
