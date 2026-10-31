package web

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	buildversion "github.com/datnt1998/verse/apps/noverse/internal/version"
)

// RunWorker serves one book until the gateway closes stdin. File mode serves
// the story directory the process was started in; hosted mode (AINOVEL_STORY_SCOPE
// set) resolves the env-scoped store instead — there is no story directory.
func RunWorker(build buildversion.Info) error {
	var cfg bootstrap.Config
	var bundle assets.Bundle
	if strings.TrimSpace(os.Getenv(workerScopeEnv)) == "" {
		var err error
		cfg, err = bootstrap.LoadConfig()
		if err != nil {
			slog.Warn("Không đọc được cấu hình; mở truyện ở chế độ đọc", "module", "web_worker", "err", err)
			cfg = bootstrap.Config{}
		}
		cfg.FillDefaults()
		bundle = assets.Load(cfg.Style, assets.DefaultLoadOptions(cfg.OutputDir))
		if bundle.Err != nil {
			return bundle.Err
		}
	}
	rules.EnsureHomeRulesDir()
	h, closeStore, err := openWorkerHost(cfg, bundle, build)
	if err != nil {
		return err
	}
	defer closeStore()
	defer h.Close()
	// GAP-4：书库内启动自动导出（AINOVEL_LIBRARY_ROOT 由 gateway 注入；空=库外运行，no-op）。
	// hosted 没有书库目录也没有 export 工件存储（phase 07），这条只跑文件模式。
	if h.ModelReady() && os.Getenv(workerScopeEnv) == "" {
		if err := h.EnableAutoExport(os.Getenv(library.RootEnv)); err != nil {
			slog.Warn("tự động xuất không khả dụng", "module", "web_worker", "err", err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	shutdownSSE := make(chan struct{})
	server := &http.Server{Handler: newWorkerHandler(h, os.Getenv("AINOVEL_WORKER_TOKEN"), shutdownSSE)}
	stopped := make(chan struct{})
	go func() { defer close(stopped); _ = server.Serve(ln) }()
	mode := ""
	if !h.ModelReady() {
		mode = " readonly"
	}
	fmt.Printf("AINOVEL_WORKER_LISTEN %s%s\n", ln.Addr(), mode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	eof := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); close(eof) }()
	select {
	case <-ctx.Done():
	case <-eof:
	case <-stopped:
	}
	close(shutdownSSE)
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

// workerLeaseTTL is the renewable lease lifetime for a hosted engine run; the
// child renews at a third of it, so a 2-minute window absorbs long model waits
// without letting a dead worker hold the story for long.
const workerLeaseTTL = 2 * time.Minute

// openWorkerHost builds the Host for this worker process. File mode keeps the
// original chain (config chain → file store → flock → tui.log). Hosted mode —
// AINOVEL_STORY_SCOPE="project/story" + STORE_DRIVER=postgres env — opens the
// scoped store and claims the durable story lease itself, then injects both
// into the Host so no engine directory, lock file or log file is touched.
// The returned closer releases the lease and pool; call it after h.Close.
func openWorkerHost(cfg bootstrap.Config, bundle assets.Bundle, build buildversion.Info) (*host.Host, func(), error) {
	scope := strings.TrimSpace(os.Getenv(workerScopeEnv))
	if scope == "" {
		h, err := host.New(cfg, bundle, host.WithUnconfiguredReads(), host.WithFileLog("tui.log", false, slog.String("version", build.Version), slog.String("commit", build.Commit), slog.String("built", build.Date)))
		return h, func() {}, err
	}
	st, lease, pool, err := openWorkerStorage(context.Background())
	if err != nil {
		return nil, nil, err
	}
	closeStore := func() {
		if lease != nil {
			if err := lease.Close(); err != nil {
				slog.Warn("nhả quyền ghi truyện thất bại", "module", "web_worker", "err", err)
			}
		}
		pool.Close()
	}
	project, story, _ := strings.Cut(scope, "/") // openWorkerStorage validates scope
	if strings.HasPrefix(story, "draft/") {
		story = "" // Before promotion only global/project config and rules exist.
	}
	docs := postgres.NewConfigDocuments(pool)
	loadConfig := func() (bootstrap.Config, error) {
		return bootstrap.LoadHostedConfig(context.Background(), docs, project, story)
	}
	cfg, err = loadConfig()
	if err != nil {
		closeStore()
		return nil, nil, err
	}
	cfg.FillDefaults()
	bundle = assets.Load(cfg.Style, assets.DefaultLoadOptions(""))
	if bundle.Err != nil {
		closeStore()
		return nil, nil, bundle.Err
	}
	documentSources, err := hostedDocumentSources(context.Background(), pool, project, story)
	if err != nil {
		closeStore()
		return nil, nil, err
	}
	h, err := host.New(cfg, bundle, host.WithUnconfiguredReads(), host.WithStorage(host.Storage{Store: st, Lease: lease}), host.WithConfigLoader(loadConfig), documentSources)
	if err != nil {
		closeStore()
		return nil, nil, err
	}
	return h, closeStore, nil
}

// openWorkerStorage resolves the hosted worker's store from env: scope names
// the story, the DSN comes from StorageConfigFromEnv (same contract as serve).
// The lease is claimed here so a second worker fails closed with ErrLeaseLost
// before the Host starts — never a "wait and steal" path.
func openWorkerStorage(ctx context.Context) (*store.Store, *postgres.StoryLease, *postgres.Pool, error) {
	project, story, ok := strings.Cut(strings.TrimSpace(os.Getenv(workerScopeEnv)), "/")
	if !ok || project == "" || story == "" {
		return nil, nil, nil, fmt.Errorf("%s=%q không hợp lệ (project/story): %w", workerScopeEnv, os.Getenv(workerScopeEnv), errs.ErrConfig)
	}
	scfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cấu hình lưu trữ: %w", err)
	}
	if scfg.Driver != bootstrap.StoreDriverPostgres {
		return nil, nil, nil, fmt.Errorf("worker hosted cần STORE_DRIVER=postgres: %w", errs.ErrConfig)
	}
	pool, err := bootstrap.OpenPool(ctx, scfg)
	if err != nil {
		return nil, nil, nil, err
	}
	nar := postgres.NewNarrative(pool)
	// The worker is a service process: scope resolves without membership —
	// exclusivity is the lease, and the gateway only launches scopes the
	// requester was already authorized for.
	var resolved postgres.ScopeRef
	if id, draft := strings.CutPrefix(story, "draft/"); draft {
		resolved, err = nar.ResolveDraft(ctx, project, id, nil, true)
	} else {
		resolved, err = nar.Resolve(ctx, project, story, nil, false)
	}
	if err != nil {
		pool.Close()
		return nil, nil, nil, err
	}
	lease, err := postgres.NewStoryLease(ctx, nar, resolved, workerLeaseTTL)
	if err != nil {
		pool.Close()
		return nil, nil, nil, err
	}
	backend := postgres.NewBackend(ctx, pool, nar, resolved, postgres.WithLeaseFence(lease))
	st, err := store.NewFromBackend(backend)
	if err != nil {
		_ = lease.Close()
		pool.Close()
		return nil, nil, nil, err
	}
	if err := st.Init(); err != nil {
		_ = lease.Close()
		pool.Close()
		return nil, nil, nil, err
	}
	return st, lease, pool, nil
}
