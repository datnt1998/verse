package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/ttsworker"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// runWorkerCommand handles `ainovel-cli worker tts --api <tailnet-url>`: the
// Mac-side pull worker that synthesizes queued TTS jobs.
//
// The service token comes from the environment (TTS_WORKER_TOKEN) — never from
// argv, so it cannot leak through the process list or shell history. The
// worker makes outbound requests only; it opens no listener and never talks to
// Postgres.
func runWorkerCommand(args []string, out, errw io.Writer) int {
	if len(args) > 0 && args[0] == "token" {
		if err := runWorkerToken(args[1:], out); err != nil {
			fmt.Fprintln(errw, err)
			return 1
		}
		return 0
	}
	if len(args) == 0 || args[0] != "tts" {
		fmt.Fprintln(errw, "Dùng: ainovel-cli worker tts --api <địa-chỉ-api>\n      ainovel-cli worker token create|revoke|list")
		return 2
	}
	fs := flag.NewFlagSet("worker tts", flag.ContinueOnError)
	fs.SetOutput(errw)
	api := fs.String("api", os.Getenv("TTS_WORKER_API_URL"), "địa chỉ listener worker riêng (Tailscale), ví dụ http://100.x.y.z:8081")
	project := fs.String("project", "", "chỉ nhận việc của dự án này (không bắt buộc)")
	lease := fs.Duration("lease", 120*time.Second, "thời hạn thuê MONG MUỐN cho mỗi việc; server kẹp trong 30s..10m")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	token := os.Getenv("TTS_WORKER_TOKEN")
	if token == "" {
		fmt.Fprintln(errw, "Thiếu TTS_WORKER_TOKEN trong môi trường (không truyền token qua tham số).")
		return 2
	}
	engine, err := voice.NewEngine(voice.Shared().Config())
	if err != nil {
		fmt.Fprintf(errw, "Không khởi tạo được máy đọc: %v\n", err)
		return 1
	}
	defer engine.Close()

	worker, err := ttsworker.New(ttsworker.Config{
		APIURL:  *api,
		Token:   token,
		Engine:  engine,
		Project: *project,
		Lease:   *lease,
	})
	if err != nil {
		fmt.Fprintf(errw, "Cấu hình worker không hợp lệ: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(out, "Worker TTS đang chờ việc từ %s (máy đọc: %s).\n", *api, engine.Name())
	err = worker.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(errw, "Worker dừng vì lỗi: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "Worker TTS đã dừng.")
	return 0
}

// parseWorkerTokenCreate parses `worker token create [--cap tts] <tên>`.
//
// Go's flag package stops at the first positional argument, so `create mac-mini
// --cap tts` would silently turn the machine name into "mac-mini --cap tts".
// The flag must come first; a trailing flag is rejected. Only capabilities the
// schema allows (currently just `tts`) are accepted, so the CLI never advertises
// a capability that would fail an INSERT.
func parseWorkerTokenCreate(args []string) (string, []string, error) {
	fs := flag.NewFlagSet("worker token create", flag.ContinueOnError)
	capList := fs.String("cap", "tts", "capability, đặt TRƯỚC tên máy (schema hiện chỉ cho phép: tts)")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	for _, arg := range fs.Args() {
		if strings.HasPrefix(arg, "-") {
			return "", nil, fmt.Errorf("cờ phải đứng trước tên máy, ví dụ: worker token create --cap tts mac-mini (nhận %q)", arg)
		}
	}
	name := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if name == "" {
		return "", nil, fmt.Errorf("worker token create cần tên máy, ví dụ: worker token create --cap tts mac-mini")
	}
	caps := []string{}
	for _, c := range strings.Split(*capList, ",") {
		c = strings.ToLower(strings.TrimSpace(c))
		switch c {
		case "":
		case "tts":
			caps = append(caps, c)
		default:
			return "", nil, fmt.Errorf("capability %q chưa được hỗ trợ (schema chỉ cho phép: tts)", c)
		}
	}
	if len(caps) == 0 {
		return "", nil, fmt.Errorf("worker token create cần ít nhất một capability (chỉ có: tts)")
	}
	return name, caps, nil
}

// runWorkerToken quản lý thông tin xác thực của worker trong Postgres
// (chỉ có nghĩa khi STORE_DRIVER=postgres). `create` in plaintext một lần —
// DB chỉ giữ SHA-256; `revoke` thu hồi ngay; `list` không bao giờ in token.
func runWorkerToken(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("lệnh: worker token create [--cap tts] <tên> | worker token revoke <id|tên> | worker token list")
	}
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return fmt.Errorf("đọc cấu hình lưu trữ: %w", err)
	}
	if cfg.Driver != bootstrap.StoreDriverPostgres {
		return fmt.Errorf("worker token chỉ dùng với STORE_DRIVER=postgres")
	}
	ctx := context.Background()
	pool, err := bootstrap.OpenPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("mở pool dữ kiện: %w", err)
	}
	defer pool.Close()
	jobs := postgres.NewJobs(pool)
	switch args[0] {
	case "create":
		name, caps, err := parseWorkerTokenCreate(args[1:])
		if err != nil {
			return err
		}
		raw := make([]byte, 48)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		token := "wrt_" + base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256([]byte(token))
		w, err := jobs.RegisterWorker(ctx, name, hash[:], caps)
		if err != nil {
			return fmt.Errorf("tạo token: %w", err)
		}
		fmt.Fprintf(out, "Máy:  %s (%s)\n", w.Name, postgres.UUIDString(w.ID))
		fmt.Fprintf(out, "Token: %s\n", token)
		fmt.Fprintln(out, "Lưu ngay — token này chỉ hiện một lần và chỉ bản băm được lưu.")
		return nil
	case "revoke":
		if len(args) < 2 {
			return fmt.Errorf("worker token revoke cần id hoặc tên của máy")
		}
		id, err := postgres.ParseUUID(args[1])
		if err != nil {
			workers, lerr := jobs.ListWorkers(ctx)
			if lerr != nil {
				return lerr
			}
			for _, w := range workers {
				if w.Name == args[1] && w.RevokedAt == nil {
					id = w.ID
				}
			}
			if !id.Valid {
				return fmt.Errorf("không tìm thấy máy đang hoạt động tên %q", args[1])
			}
		}
		if err := jobs.RevokeWorker(ctx, id); err != nil {
			return fmt.Errorf("thu hồi: %w", err)
		}
		fmt.Fprintln(out, "Đã thu hồi.")
		return nil
	case "list":
		workers, err := jobs.ListWorkers(ctx)
		if err != nil {
			return err
		}
		for _, w := range workers {
			state := "active"
			if w.RevokedAt != nil {
				state = "revoked " + w.RevokedAt.UTC().Format("2006-01-02")
			}
			seen := "chưa kết nối"
			if w.LastSeenAt != nil {
				// UTC, có hậu tố Z: danh sách này là đầu ra vận hành, không
				// phụ thuộc múi giờ của máy đang chạy lệnh.
				seen = w.LastSeenAt.UTC().Format("2006-01-02 15:04Z")
			}
			fmt.Fprintf(out, "%s %-16s %-18s caps=%-20s last=%s\n", postgres.UUIDString(w.ID), w.Name, state, strings.Join(w.Capabilities, ","), seen)
		}
		return nil
	default:
		return fmt.Errorf("lệnh worker token không hợp lệ: %s", args[0])
	}
}
