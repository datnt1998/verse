package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/apps/viderse/internal/toolsvc"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
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

	// ── Phân tích toàn bộ cấu hình trước mọi tác dụng phụ ────────────────
	// DSN, proxy và địa chỉ lỗi phải fail trước khi mở pool, chiếm cổng hay
	// tạo file nào; không fallback DSN identity sang DSN nghiệp vụ/file.
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
	identityDSN := bootstrap.IdentityDSNFromEnv()
	if identityDSN == "" {
		return fmt.Errorf("thiếu VERSE_IDENTITY_DATABASE_URL (DSN role viderse_identity tới kho tài khoản chung; xem apps/viderse/deploy/README.md)")
	}
	// Cấu hình trusted-proxy phân tích trước nhất: CIDR xấu hoặc Secure cookie
	// không có PUBLIC_URL https đều là lỗi khởi động, không lặng lẽ hạ cấp.
	proxyCfg, err := webguard.ConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình proxy: %w", err)
	}

	// Giữ cổng trước mọi tác dụng phụ còn lại: bản serve thứ hai (cổng đã
	// bận) phải thoát mà không mở pool hay đụng việc nền của bản đầu.
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	// Local SSO chia host-only cookie qua các cổng loopback; bind LAN/wildcard
	// sẽ trao cookie cho hàng xóm không tin cậy nên bị từ chối ngay.
	if proxyCfg.LocalSSO() && !ssoLoopbackOnly(listener.Addr().String()) {
		return fmt.Errorf("VERSE_APP_ORIGINS local yêu cầu bind loopback (VIDERSE_ADDR=%q bị từ chối)", *addr)
	}

	pool, err := postgres.OpenPool(context.Background(), appDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	accounts, closeAccounts, err := openAccounts(context.Background(), identityDSN)
	if err != nil {
		return err
	}
	defer closeAccounts()

	hub := web.NewEventHub()
	settings := postgres.NewSettingsStore(pool)
	statusSvc := status.New(settings, web.PublishStatus(hub))
	// Một service việc nền duy nhất cho cả app: storage và máy dùng chung.
	notifyActivity := web.Notifier(hub, "activity")
	activities := activity.New(postgres.NewActivityStore(pool), notifyActivity)

	gateway := web.NewGateway(accounts, hub, statusSvc)
	gateway.SetProxyConfig(proxyCfg)
	// Lưu trữ: watch ổ dự án và tự dọn chạy theo vòng đời của serve.
	bg, bgStop := context.WithCancel(context.Background())
	defer bgStop()
	storageSvc, serverCleanup, err := mountStorage(gateway, pool, settings, hub, statusSvc, activities, bg)
	if err != nil {
		return err
	}
	mediaManager, err := openMedia(bg, pool, storageSvc)
	if err != nil {
		return err
	}
	defer func() { _ = mediaManager.Close(context.Background()) }()
	// Tệp bền vững không thuộc tham chiếu nào (ví dụ lần công bố bị kill giữa
	// chừng) không được chặn khởi động: nói rõ cho chủ máy và vẫn phục vụ.
	if unreferenced := mediaManager.UnreferencedDurablePaths(); len(unreferenced) > 0 {
		if _, err := activities.Notify(bg, "needs_attention",
			"Cần xử lý: "+strconv.Itoa(len(unreferenced))+" tệp trong kho dữ liệu chưa thuộc tham chiếu nào ("+unreferencedSample(unreferenced)+"). Viderse vẫn chạy; mở Lưu trữ để kiểm tra.",
			"#/settings?section=storage", "Mở lưu trữ"); err != nil {
			slog.Warn("serve: ghi thông báo tệp chưa tham chiếu", "err", err)
		}
	}
	mountServerStorage(gateway, mediaManager, activities, hub, statusSvc, serverCleanup)
	// Máy: việc nền, công cụ, nhật ký và bước Chuẩn bị máy. Probe nền và
	// lượt cài đi theo vòng đời serve — dừng khi serve tắt.
	machineCtx, machineStop := context.WithCancel(context.Background())
	defer machineStop()
	cfgDir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return err
	}
	toolRegistry := tools.NewRegistry(cfgDir, nil)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	// Kho nhật ký dùng chung: mọi tiến trình con ghi vào đây, và Settings ›
	// Nhật ký/Báo lỗi đọc cùng nguồn.
	logStore := logs.New(filepath.Join(cfgDir, "logs"), home)
	logStore.SetTasks(activities)
	runner := toolsvc.NewLoggingRunner(&tools.ExecRunner{Registry: toolRegistry}, func(tool, level string) toolsvc.LineWriter {
		return logStore.Writer(tool, level)
	})
	toolDeps := tools.Deps{
		Runner:   runner,
		Paths:    &tools.FilePathStore{Dir: filepath.Join(cfgDir, "tools")},
		BaseDir:  cfgDir,
		Client:   &http.Client{Timeout: 15 * time.Minute},
		Registry: toolRegistry,
	}
	toolKit := tools.New(toolDeps)
	toolManaged := tools.NewManaged(toolDeps, nil, toolKit)
	// Video nền ráp TRƯỚC mảng nền tảng: route chính xác
	// POST /api/platforms/requests phải thắng tiền tố /api/platforms/.
	backdropRuntime := mountBackdrop(gateway, pool, cfgDir, hub, activities, storageSvc, logStore, toolKit.YtDlp, mediaManager.Media())
	defer backdropRuntime.Close() // dọn lần hai (idempotent) nếu thoát sớm trước khối shutdown
	platformSvc, err := mountPlatforms(gateway, pool, settings, hub, statusSvc, cfgDir, toolKit.YtDlp)
	if err != nil {
		return err
	}
	backdropRuntime.Bind(platformSvc)
	// Trinh sát: worker quét đối thủ chạy theo vòng đời serve (ctx riêng, dừng
	// trước khi pool đóng). Service của nó là mutations của mảng đối thủ.
	scanCtx, scanStop := context.WithCancel(context.Background())
	defer scanStop()
	scanSvc, scanWorker, err := mountScan(gateway, pool, settings, hub, activities, platformSvc)
	if err != nil {
		return err
	}
	activities.SetScanScheduleSource(activityScanSchedule(pool, scanSvc))
	scanWorkerDone := runScanWorker(scanCtx, scanWorker)
	mountRivals(gateway, pool, logStore, scanSvc)
	ai, err := wireAI(machineCtx, gateway, pool, settings, hub, statusSvc, cfgDir, toolKit, toolDeps.Runner)
	if err != nil {
		return err
	}
	channelSvc, err := mountChannels(gateway, pool, logStore, activities, ai.Executors, storageSvc, hub, mediaManager.Media())
	if err != nil {
		return err
	}
	machine, err := mountMachine(gateway, pool, settings, hub, statusSvc, machineCtx, cfgDir, logStore, toolDeps, toolKit, toolManaged, ai.Service, activities, notifyActivity, backdropRuntime.svc.PlatformRequests)
	if err != nil {
		return err
	}
	librarySvc := mountLibrary(gateway, libraryDeps{pool: pool, files: storageFiles{svc: storageSvc}, media: mediaManager.Media(), toolKit: toolKit, runner: toolDeps.Runner, ai: ai.Service, hub: hub, logStore: logStore}, newBackdropMix(pool))
	go seedLibraryBuiltins(bg, librarySvc, logStore)
	learnRT := newLearnRuntime(pool, settings, hub, ai)
	learnRT.Channels = channelSvc
	learnRT.BindCleanup(serverCleanup)
	learnRT.Mount(gateway)
	scanSvc.SetNotifier(learnRT.OnRivals(bg, web.Notifier(hub, "rivals")))
	go learnRT.Run(bg)
	seams := &assistantSeams{Learn: learnRT}
	cloneClose, err := mountClonePipeline(bg, gateway, pool, settings, hub, statusSvc, cfgDir, toolKit, toolDeps, ai, librarySvc, backdropRuntime.svc, mediaManager.Media(), scanSvc, fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port), seams)
	if err != nil {
		return err
	}
	defer cloneClose()
	defer mountAssistant(gateway, pool, ai, channelSvc, activities, seams)()
	bound, port, _ := net.SplitHostPort(listener.Addr().String())
	base := func(host string) string { return "http://" + net.JoinHostPort(host, port) }

	fmt.Printf("Tài khoản: kho chung (VERSE_IDENTITY_DATABASE_URL)\n")
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
	serverDone := stopHTTP(shutdown, server, hub)
	var first error
	// Ngừng nhận việc, huỷ worker, rồi chờ chúng (và tiến trình con của
	// installer) thoát trước khi đóng log/pool. Hub đã đóng subscriber ở
	// stopHTTP; worker publish thêm chỉ vào ring, không còn ai nghe.
	scanStop()
	if err := cloneClose(); err != nil && first == nil {
		first = err
	}
	// Worker join trước khi service dọn notifier: flush cuối cần kho còn sống.
	select {
	case <-scanWorkerDone:
	case <-shutdown.Done():
		if first == nil {
			first = shutdown.Err()
		}
	}
	scanSvc.Stop()
	machineStop()
	bgStop()
	// Huỷ và join bộ lập lịch + tải video trước khi storage/log/pool đóng.
	backdropRuntime.Close()
	// Run nhận diện đang chạy phải join trước khi pool đóng (chúng tự ghi
	// "interrupted" qua ctx riêng nếu còn giữ lease).
	if err := channelSvc.Close(shutdown); err != nil && first == nil {
		first = err
	}
	// Luồng đăng nhập/danh mục nền của Kết nối AI bám machineCtx đã huỷ ở
	// machineStop — join chúng trước khi đóng pool.
	ai.Service.Close()
	if err := machine.Shutdown(shutdown); err != nil && first == nil {
		first = err
	}
	if err := mediaManager.Close(shutdown); err != nil && first == nil {
		first = err
	}
	// Move của storage cũng phải join trước khi pool đóng (giữ journal
	// phục hồi nếu quá hạn).
	if err := storageSvc.Shutdown(shutdown); err != nil && first == nil {
		first = err
	}
	if err := <-serverDone; err != nil && first == nil {
		first = err
	}
	_ = logStore.Close()
	return first
}

// stopHTTP ngừng nhận request mới và kết thúc các luồng SSE đang mở. Luồng
// SSE chỉ dừng khi hub đóng subscriber của nó; nếu chờ Shutdown xong mới đóng
// hub thì một tab đang mở giữ Shutdown tới hết hạn và lệnh dừng trả lỗi.
// Kết quả Shutdown trả qua channel để caller join sau khi worker đã dừng.
func stopHTTP(ctx context.Context, server *http.Server, hub *events.Hub) <-chan error {
	done := make(chan error, 1)
	go func() { done <- server.Shutdown(ctx) }()
	hub.Close()
	return done
}

// openAccounts mở kho tài khoản dùng chung qua pool identity riêng (role
// viderse_identity, ProjectAccess=false: không đọc project/member Noverse).
// Pool có closer riêng, tách hẳn pool nghiệp vụ; DSN đến từ
// VERSE_IDENTITY_DATABASE_URL, không fallback sang DSN app hay file.
func openAccounts(ctx context.Context, identityDSN string) (*identity.Store, func(), error) {
	pool, err := identitypg.Open(ctx, identitypg.PoolConfig{DSN: identityDSN})
	if err != nil {
		return nil, nil, fmt.Errorf("mở kho tài khoản chung: %w", err)
	}
	store := identity.NewFromRepository(
		identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: false}),
		identity.Options{},
	)
	return store, pool.Close, nil
}

// ssoLoopbackOnly báo một địa chỉ lắng nghe đã resolve có nằm trong loopback
// không. Khi local SSO bật, cookie chung đi tới mọi cổng trên host, nên bind
// wildcard (0.0.0.0, [::]) hay LAN bị từ chối trực tiếp.
func ssoLoopbackOnly(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
