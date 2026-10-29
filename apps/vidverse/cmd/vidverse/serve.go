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

	"github.com/datnt1998/verse/apps/vidverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/vidverse/internal/web"
	"github.com/datnt1998/verse/platform/identity"
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
		*addr = strings.TrimSpace(os.Getenv("VIDVERSE_ADDR"))
	}
	if *addr == "" {
		*addr = "127.0.0.1:8788"
	}

	accounts, err := openAccounts()
	if err != nil {
		return err
	}
	// Cấu hình trusted-proxy phân tích trước khi nghe cổng: CIDR xấu hoặc Secure
	// cookie không có PUBLIC_URL https đều là lỗi khởi động, không lặng lẽ hạ cấp.
	proxyCfg, err := webguard.ConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình proxy: %w", err)
	}
	server := web.New(accounts)
	server.SetProxyConfig(proxyCfg)

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
	// Chưa có tài khoản → in mã thiết lập. Mã chỉ xuất hiện một lần này (SetupCodeOnce xoá ngay sau).
	if code := server.SetupCodeOnce(); code != "" {
		fmt.Printf("Mở trình duyệt để thiết lập: %s\n", base("127.0.0.1"))
		fmt.Printf("Mã thiết lập: %s (hết hạn sau %d phút)\n", code, int(identity.SetupCodeTTL.Minutes()))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 10 * time.Second}
	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(listener) }()
	select {
	case <-ctx.Done():
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}

// openAccounts mở kho identity file-backend ở ~/.vidverse/web-accounts.json.
func openAccounts() (*identity.Store, error) {
	dir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	path, err := identity.DefaultPath(dir)
	if err != nil {
		return nil, err
	}
	// Vidverse chưa chia project: mọi project ID đều chấp nhận, không có legacy.
	return identity.NewStore(path, identity.Options{}), nil
}
