package main

// CLI `viderse backdrop ...` (chuyển từ cli/backdrop.ts): đọc/sửa thư viện
// Video nền mà không cần serve. Mọi lệnh in JSON một dòng ở stdout, lỗi in
// JSON ra stderr với exit code 1.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

const backdropUsage = `Cách dùng:
  viderse backdrop list                          Liệt kê kênh nền + số video mỗi loại
  viderse backdrop add <link>                    Thêm kênh Twitch hoặc trang Bilibili
  viderse backdrop show <id>                     Chi tiết một kênh (usedBy, hạn mức Bilibili)
  viderse backdrop videos <id> [--kind k] [--limit n] [--cursor c]
                                                 Danh mục video của kênh (mới nhất trước)
  viderse backdrop refresh <id>                  Đọc lại danh sách ngay
  viderse backdrop remove <id>                   Xoá kênh (409 khi template còn dùng)
  viderse backdrop video add <link> [--from s] [--to s]
                                                 Thêm video lẻ + tải đoạn dùng
  viderse backdrop video list                    Liệt kê video lẻ
  viderse backdrop video remove <id>             Xoá video lẻ (409 khi template còn dùng)
  viderse backdrop parse <link>                  Phân loại một link (channel|video|unsupported|invalid)

Biến môi trường: VIDERSE_DATABASE_URL (như serve).
`

// runBackdropCommand xử lý `viderse backdrop ...`; trả về exit code.
func runBackdropCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, backdropUsage)
		return 0
	}
	if len(args) == 0 {
		return backdropUsageError(stderr, "thiếu lệnh con (list|add|show|videos|refresh|remove|video|parse)")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	svc, closeDeps, err := openBackdropCLI(ctx)
	if err != nil {
		return backdropErrOut(stderr, err)
	}
	defer closeDeps()

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		views, err := svc.Channels(ctx)
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		out := make([]any, 0, len(views))
		for _, view := range views {
			out = append(out, web.BackdropChannelToWire(view))
		}
		writeBackdropJSON(stdout, map[string]any{"channels": out})
		return 0

	case "add":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		view, created, err := svc.AddChannel(ctx, args[1])
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, map[string]any{"created": created, "channel": web.BackdropChannelToWire(view)})
		return 0

	case "show":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		view, err := svc.Channel(ctx, args[1])
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, web.BackdropChannelToWire(view))
		return 0

	case "videos":
		if len(args) < 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		flags := flag.NewFlagSet("backdrop videos", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		kind := flags.String("kind", "", "loại video (clip|highlight|vod|video)")
		limit := flags.Int("limit", 50, "số video mỗi trang")
		cursor := flags.String("cursor", "", "con trỏ trang kế tiếp")
		if err := flags.Parse(args[2:]); err != nil {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		options := backdrop.ChannelVideosOptions{Limit: *limit, Cursor: *cursor}
		if *kind != "" {
			options.Kinds = []domain.PlatformKind{*kind}
		}
		page, err := svc.ChannelVideos(ctx, args[1], options)
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, web.BackdropVideoPageToWire(page))
		return 0

	case "refresh":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		view, err := svc.RefreshChannel(ctx, args[1], true)
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, web.BackdropChannelToWire(view))
		return 0

	case "remove":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		if err := svc.RemoveChannel(ctx, args[1]); err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, map[string]any{"removed": args[1]})
		return 0

	case "video":
		return runBackdropVideoCommand(ctx, svc, args[1:], stdout, stderr)

	case "parse":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		source, err := svc.ParseSource(ctx, args[1])
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, web.ParsedSourceToWire(source))
		return 0
	}
	return backdropUsageError(stderr, "sai cú pháp lệnh")
}

// runBackdropVideoCommand xử lý nhóm `viderse backdrop video ...`.
func runBackdropVideoCommand(ctx context.Context, svc *backdrop.Service, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return backdropUsageError(stderr, "thiếu lệnh con (add|list|remove)")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		views, err := svc.Videos(ctx)
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		out := make([]any, 0, len(views))
		for _, view := range views {
			out = append(out, web.BackdropVideoToWire(view))
		}
		writeBackdropJSON(stdout, map[string]any{"videos": out})
		return 0

	case "add":
		flags := flag.NewFlagSet("backdrop video add", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		from := flags.Float64("from", 0, "giây bắt đầu")
		to := flags.Float64("to", 0, "giây kết thúc")
		if len(args) < 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		if err := flags.Parse(args[2:]); err != nil {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		var segment *domain.BackdropSegment
		if *from > 0 || *to > 0 {
			segment = &domain.BackdropSegment{FromSec: *from, ToSec: *to}
		}
		view, err := svc.AddVideo(ctx, args[1], segment)
		if err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, web.BackdropVideoToWire(view))
		return 0

	case "remove":
		if len(args) != 2 {
			return backdropUsageError(stderr, "sai cú pháp lệnh")
		}
		if err := svc.RemoveVideo(ctx, args[1]); err != nil {
			return backdropErrOut(stderr, err)
		}
		writeBackdropJSON(stdout, map[string]any{"removed": args[1]})
		return 0
	}
	return backdropUsageError(stderr, "sai cú pháp lệnh")
}

// openBackdropCLI mở pool và dựng service Video nền cho CLI: nhịp và hạn mức
// đi qua platforms.Service thật, việc nền chạy đồng bộ (lệnh trả khi xong).
func openBackdropCLI(ctx context.Context) (*backdrop.Service, func(), error) {
	pool, err := openRivalPool()
	if err != nil {
		return nil, nil, err
	}
	closeAll := func() { pool.Close() }
	root, err := cliProjectRoot(ctx, pool)
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	cfgDir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	store := postgres.NewBackdropStore(pool)
	settings := postgres.NewSettingsStore(pool)
	acc := accounts.New(accounts.Deps{Dir: cfgDir, Settings: settings})
	platformSvc := platforms.New(platforms.Deps{
		Store:  postgres.NewPlatformStateStore(pool),
		Logins: acc,
		Now:    time.Now,
	})
	pacer := platformSvc.NewPacer(nil, nil)
	ytdlp := tools.New(tools.Deps{
		Runner:  tools.Runner(&tools.ExecRunner{}),
		Paths:   &tools.FilePathStore{Dir: filepath.Join(cfgDir, "tools")},
		BaseDir: cfgDir,
	}).YtDlp
	runYtDlp := makeBackdropYtdlpRun(ytdlp, func() *platforms.Pacer { return pacer }, acc)
	svc := backdrop.New(backdrop.Deps{
		Store:     store,
		Platforms: platformSvc,
		Pacer:     pacer,
		Request: func(callCtx context.Context, resetBackoff bool, fn func(context.Context) error) error {
			return pacer.Request(callCtx, domain.PlatformBilibili, resetBackoff, fn)
		},
		Doer:        &http.Client{Timeout: 30 * time.Second},
		RunYtDlp:    runYtDlp,
		CfgDir:      cfgDir,
		Cookie:      acc.CookieHeader,
		Now:         time.Now,
		Random:      rand.Float64,
		StorageRoot: func() string { return root },
		ResolvePath: func(rel string) (string, error) { return filepath.Join(root, rel), nil },
		Spawn:       func(spawnCtx context.Context, fn func(context.Context)) { fn(spawnCtx) },
		Background:  func() context.Context { return ctx },
	})
	return svc, closeAll, nil
}

// ── đầu ra ─────────────────────────────────────────────────────────────────

func writeBackdropJSON(w io.Writer, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintf(w, "{\"error\":%q}\n", err.Error())
		return
	}
	fmt.Fprintf(w, "%s\n", raw)
}

func backdropErrOut(w io.Writer, err error) int {
	body := map[string]any{"error": err.Error()}
	var apiErr *backdrop.Error
	if errors.As(err, &apiErr) {
		body["code"] = apiErr.Code
		if apiErr.Recovery != "" {
			body["recovery"] = apiErr.Recovery
		}
		if apiErr.RetryAt != nil {
			body["retryAt"] = apiErr.RetryAt.UTC().Format(time.RFC3339)
		}
	} else {
		var coded interface{ ErrorCode() string }
		if errors.As(err, &coded) {
			body["code"] = coded.ErrorCode()
		}
		if until, ok := err.(interface{ RetryAt() time.Time }); ok && !until.RetryAt().IsZero() {
			body["retryAt"] = until.RetryAt().UTC().Format(time.RFC3339)
		}
	}
	writeBackdropJSON(w, body)
	return 1
}

func backdropUsageError(w io.Writer, message string) int {
	writeBackdropJSON(w, map[string]any{"error": message})
	return 2
}
