package main

// Ráp mảng nền tảng nguồn (V23/V24): registry + trạng thái bền + đăng nhập
// chỉ-nằm-trên-máy, rồi gắn route HTTP và nguồn chấm Cài đặt. Chỉ cmd được
// phép biết cả ba gói — packages còn lại chỉ thấy interface nhỏ của mình.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
)

// mountPlatforms dựng accounts + platforms trên cùng pool/settings và bộ công
// cụ dùng chung của serve (cùng runner/registry/quyền cài đặt — không tự tạo
// tools.New thứ hai), gắn /api/platforms* và đăng ký nguồn cần xử lý.
func mountPlatforms(g *web.Gateway, pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, statusSvc *status.Service, cfgDir string, ytdlp ytdlpRunner) error {
	dir := cfgDir
	notifyPlatforms := web.Notifier(hub, "platforms")
	// Mọi thay đổi nền tảng cũng làm mới snapshot đầu trang: chấm Cài đặt
	// đổi trong cùng một nhịp SSE, không phải chờ lượt poll dự phòng.
	touch := func(ref string) {
		notifyPlatforms(ref)
		statusSvc.Bump()
	}
	acc := accounts.New(accounts.Deps{
		Dir:      dir,
		Settings: settings,
		Probe:    ytDlpProbe{tool: ytdlp},
		Notify:   notifyPlatforms,
	})
	svc := platforms.New(platforms.Deps{
		Store:  postgres.NewPlatformStateStore(pool),
		Logins: acc,
		Now:    time.Now,
		Notify: touch,
	})
	// Lỗi 500: nguyên nhân đi qua redactor chung của logs.Store trước khi chạm
	// đĩa — đường dẫn home, DSN, token không bao giờ vào nhật ký hay JSON.
	home, _ := os.UserHomeDir()
	logStore := logs.New(filepath.Join(dir, "logs"), home)
	web.MountPlatformsDiag(g, svc, func(msg string) {
		logStore.Append("platforms", logs.LevelError, msg)
	})
	statusSvc.RegisterSource(platformAttention{svc: svc})
	return nil
}

// platformAttention chuyển mục cần xử lý của mảng nền tảng sang hình mà
// status.Service cộng vào snapshot — adapter ở chỗ ráp để gói nghiệp vụ
// không import tầng status.
type platformAttention struct {
	svc *platforms.Service
}

func (p platformAttention) Report(ctx context.Context) (status.Report, error) {
	items, err := p.svc.Attention(ctx)
	if err != nil {
		return status.Report{}, err
	}
	report := status.Report{Attention: make([]status.Attention, 0, len(items))}
	for _, it := range items {
		report.Attention = append(report.Attention, status.Attention{Kind: it.Kind, Text: it.Text, Href: it.Href})
	}
	return report, nil
}

// ytdlpRunner là phần tối thiểu của *tools.YtDlp mà probe cần — khai ở cmd để
// probe dùng đúng toolkit dùng chung được truyền vào (L2), và để test mô
// phỏng không cần binary thật.
type ytdlpRunner interface {
	IsInstalled() bool
	Run(ctx context.Context, args []string, opts tools.RunYtDlpOptions) (string, error)
}

// ytDlpProbe kiểm tra một hồ sơ trình duyệt bằng yt-dlp ở chế độ chỉ đọc
// thông tin (không tải media). Đây là seam duy nhất chạm binary của mảng tài
// khoản; accounts chỉ thấy interface YouTubeProbe.
type ytDlpProbe struct {
	tool ytdlpRunner
}

// probeURL là video công khai ổn định của YouTube, chỉ để xác nhận yt-dlp đọc
// được cookie của hồ sơ — không tải nội dung.
const probeURL = "https://www.youtube.com/watch?v=jNQXAC9IVRw"

func (p ytDlpProbe) Check(ctx context.Context, browser, profile string) error {
	if p.tool == nil || !p.tool.IsInstalled() {
		return errors.New("yt-dlp chưa cài")
	}
	_, err := p.tool.Run(ctx, []string{
		"--simulate", "--skip-download", "--no-playlist", "--no-warnings",
		"--cookies-from-browser", browser + ":" + profile, probeURL,
	}, tools.RunYtDlpOptions{Timeout: 90 * time.Second})
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() == "unavailable" {
		// Video mẫu không xem được không nói gì về cookie: yt-dlp đã đọc được
		// hồ sơ và YouTube đã trả lời.
		return nil
	}
	return err
}
