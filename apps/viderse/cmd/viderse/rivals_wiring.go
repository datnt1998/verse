package main

// Ráp mảng đối thủ (Khám phá): store đọc + service rivals cho GET/CLI, và
// gắn route /api/rivals* với mutations do luồng quét (scan.Service) cung cấp.
// serve.go gọi mountRivals sau khi dựng scan service; CLI gọi openRivalReads/
// openRivalMutations để mở pool theo cùng cách serve/migrate.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/blobstore"
)

// rivalMutations là mặt ghi/quét mà web + CLI cần — hiện thực là
// *scan.Service (RunInline phục vụ `viderse rival scan` khi không có serve
// nào giữ lease quét YouTube).
type rivalMutations interface {
	web.RivalMutations
	RunInline(ctx context.Context, channelID string) (ran bool, err error)
}

// rivalPreviewWiring là nguồn đọc chương thật cho xem trước Khám phá: artifact
// đã công bố trong kho media + một lượt dò yt-dlp có giới hạn khi chưa có.
// Mọi trường đều có thể nil (CLI/test) — service trả lỗi cấu hình thật.
type rivalPreviewWiring struct {
	Media      *media.Service
	YtDlp      ytdlpRunner
	CookieArgs func(context.Context) ([]string, error)
	Pacer      *platforms.Pacer
}

// mountRivals dựng store/service đọc đối thủ và gắn route /api/rivals*;
// mutations đến từ scan service đã dựng sẵn trong serve. Lỗi 500 đi qua
// redactor của kho nhật ký chung như mảng nền tảng.
func mountRivals(g *web.Gateway, pool *pgxpool.Pool, logStore *logs.Store, mutations web.RivalMutations, preview rivalPreviewWiring) {
	reads := rivals.NewWithPreview(postgres.NewRivalsStore(pool), rivals.PreviewDeps{
		OpenArtifact: rivalArtifactOpener(preview.Media),
		ProbeInfo:    rivalInfoProbe(preview.YtDlp, preview.Pacer, preview.CookieArgs),
	})
	web.MountRivalsDiag(g, reads, mutations, func(msg string) {
		logStore.Append("rivals", logs.LevelError, logStore.Redact(msg))
	})
}

// rivalArtifactOpener mở artifact theo đường dẫn logic qua kho media và tự
// nhả handle khi đóng. Tệp đã công bố nhưng không còn trong kho được báo là
// rivals.ErrArtifactAbsent để xem trước dò metadata một lần; lỗi đọc khác giữ
// nguyên là lỗi thật.
func rivalArtifactOpener(svc *media.Service) func(context.Context, string) (io.ReadCloser, error) {
	if svc == nil {
		return nil
	}
	return func(ctx context.Context, path string) (io.ReadCloser, error) {
		handle, err := svc.Acquire(ctx, path)
		if err != nil {
			if blobstore.IsNotFound(err) || errors.Is(err, os.ErrNotExist) {
				return nil, rivals.ErrArtifactAbsent
			}
			return nil, err
		}
		file, err := os.Open(handle.Path)
		if err != nil {
			handle.Release()
			if errors.Is(err, os.ErrNotExist) {
				return nil, rivals.ErrArtifactAbsent
			}
			return nil, err
		}
		return &releasedArtifact{File: file, release: handle.Release}, nil
	}
}

type releasedArtifact struct {
	*os.File
	release func()
}

func (f *releasedArtifact) Close() error {
	err := f.File.Close()
	f.release()
	return err
}

// rivalInfoProbe dò metadata nguồn bằng yt-dlp ở chế độ chỉ đọc thông tin,
// kèm cookie của hồ sơ trình duyệt đã chọn và nhịp nền tảng dùng chung. Thời
// hạn 25 giây + ctx của request: đóng tab là huỷ lệnh.
func rivalInfoProbe(tool ytdlpRunner, pacer *platforms.Pacer, cookies func(context.Context) ([]string, error)) func(context.Context, string) ([]byte, error) {
	if tool == nil {
		return nil
	}
	return func(ctx context.Context, videoID string) ([]byte, error) {
		args := []string{
			"-J", "--skip-download", "--no-playlist",
			"--retries", "0", "--extractor-retries", "0", "--socket-timeout", "20",
		}
		if cookies != nil {
			extra, err := cookies(ctx)
			if err != nil {
				return nil, err
			}
			args = append(args, extra...)
		}
		args = append(args, "https://www.youtube.com/watch?v="+videoID)
		var raw []byte
		run := func(callCtx context.Context) error {
			out, err := tool.Run(callCtx, args, tools.RunYtDlpOptions{Timeout: 25 * time.Second})
			raw = []byte(out)
			return err
		}
		var err error
		if pacer != nil {
			err = pacer.RunYtDlp(ctx, domain.PlatformYouTube, 4, 25*time.Second, run)
		} else {
			err = run(ctx)
		}
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
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

// openRivalAccounts dựng nguồn cookie yt-dlp dùng chung cho xem trước. Chưa
// chọn hồ sơ trình duyệt thì CookieArgs trả rỗng và yt-dlp chạy ẩn danh.
func openRivalAccounts(cfgDir string, settings *postgres.SettingsStore) *accounts.Service {
	return accounts.New(accounts.Deps{Dir: cfgDir, Settings: settings})
}
