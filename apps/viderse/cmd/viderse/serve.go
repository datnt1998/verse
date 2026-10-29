package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/assets"
	"github.com/datnt1998/verse/apps/viderse/internal/agent"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/models"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/webguard"
)

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	addr := flags.String("addr", "", "Địa chỉ lắng nghe")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("tham số không hợp lệ: %v", flags.Args())
	}

	if *addr == "" {
		*addr = strings.TrimSpace(os.Getenv("VIDERSE_ADDR"))
	}
	if *addr == "" {
		*addr = "127.0.0.1:8789"
	}
	appDSN, _ := bootstrap.DSNFromEnv()
	if appDSN == "" {
		return fmt.Errorf("thiếu VIDERSE_DATABASE_URL (vd postgres://viderse_app@127.0.0.1:5432/viderse_go)")
	}

	accounts, err := openAccounts()
	if err != nil {
		return err
	}

	// Cấu hình trusted-proxy phân tích trước nhất: CIDR xấu hoặc Secure cookie
	// không có PUBLIC_URL https đều là lỗi khởi động, không lặng lẽ hạ cấp.
	proxyCfg, err := webguard.ConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình proxy: %w", err)
	}

	pool, err := agent.OpenPool(context.Background(), appDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	queries := gen.New(pool)

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		return err
	}
	// Giá theo token lấy từ OpenRouter (cache 24h trong ~/.viderse) như Noverse;
	// thiếu bảng giá thì agent_runs.cost_usd để trống.
	if dir, err := bootstrap.DefaultConfigDir(); err == nil {
		models.StartPricingRefresh(models.DefaultRegistry(), dir)
	}
	printWriterLoginHint(cfg)
	runner := agent.NewRunner(queries,
		agent.ModelFactory(cfg.ModelForRole),
		assets.WriterSystemPrompt,
		cfg.ResolveReasoningEffort)

	gateway := web.NewGateway(accounts, queries, runner)
	gateway.SetProxyConfig(proxyCfg)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	bound, port, _ := net.SplitHostPort(listener.Addr().String())
	base := func(host string) string { return "http://" + net.JoinHostPort(host, port) }

	fmt.Printf("Tài khoản: %s\n", accounts.Path())
	if ip := net.ParseIP(bound); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		fmt.Printf("Mở trên máy này: %s\n", base("127.0.0.1"))
	} else {
		fmt.Printf("Địa chỉ: %s\n", base(bound))
	}
	// Chưa có tài khoản → in địa chỉ mở và mã thiết lập. Mã chỉ xuất hiện một lần này (SetupCodeOnce xoá ngay sau).
	if code := gateway.SetupCodeOnce(); code != "" {
		fmt.Printf("Mở trình duyệt để thiết lập: %s\n", base("127.0.0.1"))
		fmt.Printf("Mã thiết lập: %s (hết hạn sau %d phút)\n", code, int(identity.SetupCodeTTL.Minutes()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runner.Start(ctx)

	server := &http.Server{Handler: gateway}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownResult := make(chan error, 1)
	go func() { shutdownResult <- server.Shutdown(shutdown) }()
	runner.Wait()
	return <-shutdownResult
}

// openAccounts mở kho identity file-backend ở ~/.viderse/web-accounts.json.
func openAccounts() (*identity.Store, error) {
	dir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	path, err := identity.DefaultPath(dir)
	if err != nil {
		return nil, err
	}
	// Viderse không chia project: mọi project ID đều chấp nhận, không có legacy.
	return identity.NewStore(path, identity.Options{}), nil
}

// printWriterLoginHint báo ngay lúc khởi động khi writer dùng provider OAuth
// chưa đăng nhập: run nào cũng sẽ fail ở lượt gọi model đầu tiên.
func printWriterLoginHint(cfg bootstrap.Config) {
	provider, model, _ := cfg.ResolveRole(bootstrap.WriterRole)
	if _, oauth := auth.Lookup(provider); !oauth {
		return
	}
	store, err := auth.DefaultStore()
	if err != nil {
		fmt.Printf("Không mở được kho đăng nhập: %v\n", err)
		return
	}
	if _, ok, err := store.Get(provider); err != nil {
		fmt.Printf("Không đọc được kho đăng nhập: %v\n", err)
	} else if !ok {
		fmt.Printf("Writer dùng %s/%s nhưng chưa đăng nhập: chạy `viderse login %s`.\n", provider, model, provider)
	}
}

func runMigrate(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("cần đúng một lệnh: up | status | version")
	}
	action := flags.Arg(0)
	switch action {
	case "up", "status", "version":
	default:
		return fmt.Errorf("lệnh không hợp lệ %q (up|status|version)", action)
	}
	_, migrateDSN := bootstrap.DSNFromEnv()
	if migrateDSN == "" {
		return fmt.Errorf("thiếu VIDERSE_MIGRATION_DATABASE_URL (hoặc VIDERSE_DATABASE_URL)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := pgmigrate.OpenDB(migrateDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	switch action {
	case "up":
		res, err := pgmigrate.Up(ctx, db, migrations.FS)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			fmt.Println("Không có migration nào chờ — schema đã mới nhất.")
			return nil
		}
		for _, r := range res {
			fmt.Printf("Áp dụng %s (%.0fms)\n", r.Source.Path, float64(r.Duration.Microseconds())/1000)
		}
		return nil
	case "status":
		st, err := pgmigrate.Status(ctx, db, migrations.FS)
		if err != nil {
			return err
		}
		for _, s := range st {
			state := "đã áp dụng"
			if !s.Applied {
				state = "chờ"
			}
			fmt.Printf("%05d %s %s\n", s.Version, state, s.Source)
		}
		return nil
	case "version":
		v, err := pgmigrate.Version(ctx, db, migrations.FS)
		if err != nil {
			return err
		}
		fmt.Printf("Phiên bản schema: %d\n", v)
		return nil
	}
	return nil
}
