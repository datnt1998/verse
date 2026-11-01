package main

// Ráp mảng Thư viện: store postgres, service (template + prompt ảnh + lớp
// phủ), đường giá ảnh từ aiconn, đường ffmpeg/ffprobe từ tools, thư mục dự án
// từ storage và SSE từ hub. serve gọi mountLibrary; CLI gọi openLibrary với
// cùng cách dựng nhưng không có hub (không có ai nghe).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/aiconn"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
)

// libraryRunner là adapter tools.Runner → library.Runner (stdout của một lần
// chạy, có trần thời gian).
type libraryRunner struct{ runner tools.Runner }

// Run chạy một binary và trả stdout; lỗi giữ nguyên (caller quyết định 400).
func (r libraryRunner) Run(ctx context.Context, file string, args []string) (string, error) {
	result, err := r.runner.Run(ctx, file, args, tools.Spec{Timeout: 60 * time.Second, Label: "library"})
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// libraryBins là adapter tools.Ffmpeg → library.Binaries, có cache ngắn để
// mỗi lần thêm lớp phủ không probe lại binary (Resolve chạy thử ffmpeg).
type libraryBins struct {
	ffmpeg *tools.Ffmpeg

	mu  sync.Mutex
	at  time.Time
	ffm string
	ffp string
	ok  bool
}

// Tools trả đường ffmpeg/ffprobe dùng được; ok=false khi máy chưa có.
func (b *libraryBins) Tools(ctx context.Context) (string, string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.at.IsZero() && time.Since(b.at) < time.Minute {
		return b.ffm, b.ffp, b.ok
	}
	b.at = time.Now()
	b.ffm, b.ffp, b.ok = "", "", false
	if b.ffmpeg != nil {
		if resolved := b.ffmpeg.Resolve(ctx); resolved != nil && resolved.Ffprobe != "" {
			b.ffm, b.ffp, b.ok = resolved.Ffmpeg, resolved.Ffprobe, true
		}
	}
	return b.ffm, b.ffp, b.ok
}

// libraryPrice là adapter aiconn → library.PriceOf: giá một ảnh của model vẽ
// chính của Hoạ sĩ; ok=false khi model chưa khai giá.
type libraryPrice struct{ svc *aiconn.Service }

// PerImageUSD đọc Agents rồi RefInfo của Artist.Draw.
func (p libraryPrice) PerImageUSD(ctx context.Context) (float64, bool, error) {
	if p.svc == nil {
		return 0, false, nil
	}
	agents, err := p.svc.Agents(ctx)
	if err != nil {
		return 0, false, err
	}
	if agents.Artist.Draw.Provider == "" || agents.Artist.Draw.Model == "" {
		return 0, false, nil
	}
	_, model, _, err := p.svc.RefInfo(ctx, agents.Artist.Draw)
	if err != nil {
		return 0, false, err
	}
	if model.PerImage > 0 {
		return model.PerImage, true, nil
	}
	return 0, false, nil
}

// libraryDeps gom phụ thuộc dựng service cho serve và CLI.
type libraryDeps struct {
	pool     *pgxpool.Pool
	files    library.Files
	media    *media.Service
	toolKit  *tools.Tools
	runner   tools.Runner
	ai       *aiconn.Service
	hub      *events.Hub
	logStore *logs.Store
}

// newLibraryService dựng service từ phụ thuộc đã có.
func newLibraryService(deps libraryDeps) *library.Service {
	var notify func(topic, ref string)
	if deps.hub != nil {
		templates := web.Notifier(deps.hub, library.TopicTemplate)
		prompts := web.Notifier(deps.hub, library.TopicPrompt)
		overlays := web.Notifier(deps.hub, library.TopicOverlay)
		notify = func(topic, ref string) {
			switch topic {
			case library.TopicTemplate:
				templates(ref)
			case library.TopicPrompt:
				prompts(ref)
			case library.TopicOverlay:
				overlays(ref)
			}
		}
	}
	d := library.Deps{Store: postgres.NewLibraryStore(deps.pool), Files: deps.files, Media: deps.media}
	d.Notify = notify
	if deps.runner != nil {
		d.Runner = libraryRunner{runner: deps.runner}
	}
	if deps.toolKit != nil && deps.toolKit.Ffmpeg != nil {
		d.Bins = &libraryBins{ffmpeg: deps.toolKit.Ffmpeg}
	}
	d.Price = libraryPrice{svc: deps.ai}
	return library.New(d)
}

// mountLibrary dựng service Thư viện trong serve, gắn route /api/templates*,
// /api/image-prompts* và /api/overlays*, rồi ghi gói hiệu ứng có sẵn ở nền
// theo vòng đời serve. mix là route nền của mảng Video nền (cùng tiền tố
// /api/templates/).
func mountLibrary(g *web.Gateway, deps libraryDeps, mix web.TemplateBackdropMix) *library.Service {
	svc := newLibraryService(deps)
	web.MountLibraryDiag(g, svc, mix, func(msg string) {
		if deps.logStore != nil {
			deps.logStore.Append("library", logs.LevelError, deps.logStore.Redact(msg))
		}
	})
	return svc
}

// seedLibraryBuiltins ghi 12 hiệu ứng có sẵn ở nền; lỗi ghi nhật ký chứ
// không làm sập serve (thư viện vẫn dùng được, chỉ thiếu hiệu ứng).
func seedLibraryBuiltins(ctx context.Context, svc *library.Service, logStore *logs.Store) {
	added, err := svc.SeedBuiltins(ctx)
	if err != nil && ctx.Err() == nil {
		if logStore != nil {
			logStore.Append("library", logs.LevelError, logStore.Redact("hiệu ứng có sẵn: "+err.Error()))
		}
		return
	}
	if added > 0 && logStore != nil {
		logStore.Append("library", logs.LevelInfo, fmt.Sprintf("đã thêm %d hiệu ứng có sẵn vào Thư viện › Lớp phủ", added))
	}
}

// ── CLI ────────────────────────────────────────────────────────────────────

// cliProjectRoot đọc thư mục dự án đang chọn, mặc định <config>/projects.
func cliProjectRoot(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	cfgDir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	location := filepath.Join(cfgDir, "projects")
	if found, err := postgres.NewSettingsStore(pool).Get(ctx, domain.KeyStorageLocation, &location); err != nil {
		return "", err
	} else if !found || location == "" {
		location = filepath.Join(cfgDir, "projects")
	}
	if !filepath.IsAbs(location) {
		location = filepath.Join(home, location)
	}
	return location, nil
}

// openLibrary mở pool + công cụ + thư mục dự án cho `viderse template` và
// `viderse overlay`; hàm đóng trả về giải phóng pool.
func openLibrary(ctx context.Context) (*library.Service, func(), error) {
	pool, err := openRivalPool()
	if err != nil {
		return nil, nil, err
	}
	root, err := cliProjectRoot(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	manager, err := openCLIMedia(ctx, pool, func() string { return root })
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	cfgDir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		_ = manager.Close(context.Background())
		pool.Close()
		return nil, nil, err
	}
	runner := tools.Runner(&tools.ExecRunner{})
	toolKit := tools.New(tools.Deps{
		Runner:  runner,
		Paths:   &tools.FilePathStore{Dir: filepath.Join(cfgDir, "tools")},
		BaseDir: cfgDir,
	})
	svc := newLibraryService(libraryDeps{
		pool:    pool,
		files:   cliStorage{root: root},
		media:   manager.Media(),
		toolKit: toolKit,
		runner:  runner,
	})
	// Nhập template cũ cần mặt media của editor (ImportFileOnce) — serve gắn mặt
	// này sau khi dựng service Editor, CLI gắn ngay vì dựng được cả hai tại chỗ.
	svc.BindMediaImporter(newEditorService(pool, manager.Media(), cfgDir, toolKit, tools.Deps{Runner: runner}))
	return svc, func() {
		_ = manager.Close(context.Background())
		pool.Close()
	}, nil
}

// storageFiles đổi *storage.Service sang mặt Files của service Thư viện.
type storageFiles struct{ svc *storage.Service }

func (f storageFiles) Root() string { return f.svc.Root() }

func (f storageFiles) Resolve(rel string) (string, error) { return f.svc.Resolve(rel) }
