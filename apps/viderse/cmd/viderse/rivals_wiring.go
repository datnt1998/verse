package main

// Ráp mảng đối thủ (Khám phá): store đọc + service rivals cho GET/CLI, và
// gắn route /api/rivals* với mutations do luồng quét (scan.Service) cung cấp.
// serve.go gọi mountRivals sau khi dựng scan service; CLI gọi openRivalReads/
// openRivalMutations để mở pool theo cùng cách serve/migrate.

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

// rivalMutations là mặt ghi/quét mà web + CLI cần — hiện thực là
// *scan.Service (RunInline phục vụ `viderse rival scan` khi không có serve
// nào giữ lease quét YouTube).
type rivalMutations interface {
	web.RivalMutations
	RunInline(ctx context.Context, channelID string) (ran bool, err error)
}

// mountRivals dựng store/service đọc đối thủ và gắn route /api/rivals*;
// mutations đến từ scan service đã dựng sẵn trong serve. Lỗi 500 đi qua
// redactor của kho nhật ký chung như mảng nền tảng.
func mountRivals(g *web.Gateway, pool *pgxpool.Pool, logStore *logs.Store, mutations web.RivalMutations) {
	reads := rivals.New(postgres.NewRivalsStore(pool))
	web.MountRivalsDiag(g, reads, mutations, func(msg string) {
		logStore.Append("rivals", logs.LevelError, logStore.Redact(msg))
	})
}

// newRivalsUsage dựng bộ đếm kênh đối thủ cho /api/platforms (Usage của
// nền tảng YouTube). serve truyền vào platforms.Deps.Usage.
func newRivalsUsage(pool *pgxpool.Pool) *rivals.UsageCounter {
	return rivals.NewUsageCounter(postgres.NewRivalsStore(pool))
}

// openRivalPool mở pool app như serve/migrate (VIDERSE_DATABASE_URL).
func openRivalPool() (*pgxpool.Pool, error) {
	dsn, _ := bootstrap.DSNFromEnv()
	if dsn == "" {
		return nil, fmt.Errorf("thiếu VIDERSE_DATABASE_URL (vd postgres://viderse_app@127.0.0.1:5432/viderse_go)")
	}
	return postgres.OpenPool(context.Background(), dsn)
}

// openRivalReads mở pool và dựng service đọc đối thủ cho các lệnh `rival`
// chỉ-đọc (list, videos, posting-slots).
func openRivalReads() (*rivals.Service, func(), error) {
	pool, err := openRivalPool()
	if err != nil {
		return nil, nil, err
	}
	return rivals.New(postgres.NewRivalsStore(pool)), pool.Close, nil
}

// openRivalMutations mở pool và dựng scan service cho các lệnh `rival` ghi
// (add, remove, scan) qua đúng một đường ráp với serve — chỉ khác là không có
// hub SSE: activity vẫn ghi thật nên việc nền/lỗi/thông báo của CLI không
// lệch trạng thái so với server.
func openRivalMutations() (rivalMutations, *rivals.Service, func(), error) {
	pool, err := openRivalPool()
	if err != nil {
		return nil, nil, nil, err
	}
	activities := activity.New(postgres.NewActivityStore(pool), nil)
	svc, err := newScanService(pool, postgres.NewSettingsStore(pool), activities)
	if err != nil {
		pool.Close()
		return nil, nil, nil, err
	}
	reads := rivals.New(postgres.NewRivalsStore(pool))
	return svc, reads, func() { svc.Stop(); pool.Close() }, nil
}
