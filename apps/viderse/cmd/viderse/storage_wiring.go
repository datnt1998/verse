package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

// mountStorage ráp mảng Cài đặt › Lưu trữ: vị trí thư mục dự án, kiểm tra,
// chuyển nền, theo dõi ổ và dọn dẹp. ctx là vòng đời của serve — watch và tự
// dọn chạy tới khi ctx kết thúc.
func mountStorage(g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, statusSvc *status.Service, activities *activity.Service, ctx context.Context) (*storage.Service, error) {
	dir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	notifyStorage := web.Notifier(hub, "storage")

	svc, err := storage.New(ctx, storage.Deps{
		Home:       home,
		ConfigDir:  dir,
		DefaultDir: filepath.Join(dir, "projects"),
		Settings:   settings,
		Volumes:    storage.CachedVolumes(storage.DefaultVolumes(home)),
		Tasks:      activities,
		Notify:     activities,
		Opener:     finderOpener{},
		Emit:       notifyStorage,
		Bump:       func() { statusSvc.Bump() },
	})
	if err != nil {
		return nil, err
	}

	clean := retention.New(retention.Deps{
		Roots:     svc,
		ConfigDir: dir,
		TmpDir:    os.TempDir(),
		Reaper:    registryReaper{reg: tools.NewRegistry(dir, nil)},
		Receipts:  postgres.NewStorageStore(pool),
		Tasks:     activities,
		Settings:  settings,
		Changed:   svc.Changed,
	})

	web.MountStorage(g, svc, clean)
	statusSvc.RegisterSource(storageAttention{svc: svc})

	go func() {
		if err := svc.Start(ctx); err != nil && ctx.Err() == nil {
			slog.Error("storage: vòng theo dõi ổ dừng", "err", err)
		}
	}()
	go func() {
		if err := clean.Start(ctx); err != nil && ctx.Err() == nil {
			slog.Error("retention: vòng dọn dẹp dừng", "err", err)
		}
	}()
	return svc, nil
}

// storageAttention đổi mục cần xử lý của Lưu trữ sang hình của status.Service
// — gói nghiệp vụ không import tầng status.
type storageAttention struct{ svc *storage.Service }

func (a storageAttention) Report(ctx context.Context) (status.Report, error) {
	items := a.svc.AttentionItems(ctx)
	report := status.Report{}
	for _, it := range items {
		report.Attention = append(report.Attention, status.Attention{Kind: it.Kind, Text: it.Text, Href: it.Href})
	}
	return report, nil
}

// registryReaper kết thúc nhóm process mồ côi qua sổ process-groups của tools.
type registryReaper struct{ reg *tools.Registry }

func (r registryReaper) ReapOrphaned(ctx context.Context, dryRun bool) ([]string, error) {
	groups, err := r.reg.ReapOrphaned(ctx, dryRun, nil, tools.TerminateOptions{})
	labels := make([]string, 0, len(groups))
	for _, g := range groups {
		labels = append(labels, g.Label)
	}
	return labels, err
}

// finderOpener mở thư mục bằng trình quản lý tệp của máy chạy server.
type finderOpener struct{}

func (finderOpener) Open(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch {
	case fileExists("/usr/bin/open"):
		cmd = exec.CommandContext(ctx, "open", path)
	case fileExists("/usr/bin/xdg-open"):
		cmd = exec.CommandContext(ctx, "xdg-open", path)
	default:
		return &storage.Error{Code: storage.ErrNotSupported, Message: "máy chủ không có trình mở thư mục"}
	}
	return cmd.Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
