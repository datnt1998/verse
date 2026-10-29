package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/web"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

func runServe(args []string) error {
	if len(args) > 0 && args[0] == "user" {
		return runServeUser(args[1:])
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", "", "Thư mục sách (mặc định: ~/.ainovel/serve.json, rồi ~/Truyen nếu đã có; không có thì chọn trên web)")
	addr := flags.String("addr", "", "Địa chỉ lắng nghe")
	lan := flags.Bool("lan", false, "Cho phép điện thoại trong mạng LAN")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("tham số không hợp lệ: %v", flags.Args())
	}
	// 14 §G2 + 09 §10: --root thắng, rồi serve.json, rồi ~/Truyen nếu đã có; còn lại là chưa chọn
	// nơi lưu truyện (absolute == ""), web sẽ hỏi ở bước hướng dẫn.
	absolute, err := library.ResolveServeRoot(*root)
	if err != nil {
		return err
	}
	// 存储环境严格校验：坏的 STORE_DRIVER/DATABASE_URL 在启动期失败，
	// 绝不静默回落 file。
	storageCfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình lưu trữ: %w", err)
	}
	// STORAGE_ENV là tiền tố môi trường của mọi khoá blob; giá trị lạ là lỗi
	// khởi động, không bao giờ đoán (trống = dev).
	keys, err := blobkey.FromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình STORAGE_ENV: %w", err)
	}
	// 未配置也允许启动：首次登录的引导流程会在网页上配置供应商与模型。
	if _, err := os.Stat(bootstrap.DefaultConfigPath()); err != nil {
		fmt.Println("Chưa có cấu hình mô hình: đăng nhập web để thiết lập ở bước hướng dẫn (hoặc chạy ainovel-cli login).")
	}
	accounts, closeAccounts, err := openAccounts(storageCfg)
	if err != nil {
		return err
	}
	defer closeAccounts()
	// GAP-41：还没有账号时不再在终端里问名字/密码；serve 启动后打印设置码，在网页上创建管理员。
	// 默认只监听本机；hosted（容器）通过 HTTP_ADDR=0.0.0.0:8080 指定 —— Traefik
	// 是唯一的公开入口，容器端口不直接发布。--lan 与 serve.json 的 lan 通过
	// Network 另开局域网监听，之后可在网页里开关。
	if *addr == "" {
		*addr = strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	}
	if *addr == "" {
		*addr = "127.0.0.1:8686"
	}
	// trusted-proxy 配置必须最先解析：坏的 CIDR 或 Secure cookie 无 HTTPS
	// PUBLIC_URL 都是启动错误，不是运行期静默降级。
	proxyCfg, err := web.ProxyConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình proxy: %w", err)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	gateway := web.NewGateway(absolute, accounts, versionInfo())
	gateway.SetProxyConfig(proxyCfg)
	// STORE_DRIVER=postgres: mọi truy cập dữ kiện truyện đi qua backend có
	// scope (resolve project/story + capability gate) với actor của request —
	// repository kiểm membership trong cùng tx. File dưới đĩa không
	// còn là nguồn đọc ngầm; lỗi mở kho được trả về caller, không fallback.
	if storageCfg.Driver == bootstrap.StoreDriverPostgres {
		ctx := context.Background()
		pool, err := bootstrap.OpenPool(ctx, storageCfg)
		if err != nil {
			return fmt.Errorf("mở pool dữ kiện truyện: %w", err)
		}
		defer pool.Close()
		gateway.SetLibraryRepository(postgres.NewLibraryRepository(pool))
		// Hosted engine workers get the same storage contract via env (never
		// argv); the child resolves its own scoped store and claims the lease.
		gateway.SetWorkerStorage(&storageCfg)
		gateway.SetFactStore(func(st library.Story, user *webauth.User) (*store.Store, error) {
			if user == nil {
				return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
			}
			actor := &postgres.Actor{Username: user.Username, GlobalRole: string(user.Role)}
			return bootstrap.OpenStoryStore(ctx, pool, st.Project, st.Story, actor)
		})
		nar := postgres.NewNarrative(pool)
		blobCfg, err := bootstrap.BlobConfigFromEnv()
		if err != nil {
			return fmt.Errorf("đọc cấu hình blob: %w", err)
		}
		blobs, err := bootstrap.OpenBlobStore(blobCfg)
		if err != nil {
			return fmt.Errorf("mở blob store: %w", err)
		}
		gateway.SetAudioStore(&web.PGAudioDeps{
			Jobs:  postgres.NewJobs(pool),
			Audio: postgres.NewAudio(pool),
			Cache: postgres.NewExportCache(pool),
			Nar:   nar,
			Blobs: blobs,
			Keys:  keys,
		})
		// /readyz：schema 版本 + 连接（CheckSchema 查询数据库）和 BlobStore
		// 可达性。TTS worker 不在探针里 —— 家里 Mac 掉线是队列退化，不是
		// 进程不健康（重启容器不会带来模型）。
		gateway.SetHealth(web.HealthDeps{
			SchemaVersion: func(ctx context.Context) (int64, error) {
				if err := pool.CheckSchema(ctx); err != nil {
					return 0, err
				}
				// Readiness never runs migrations; it only refuses traffic when
				// this binary's embedded migrations are not all applied.
				if err := pool.CheckMigrations(ctx); err != nil {
					return 0, err
				}
				return pool.SchemaVersion(ctx)
			},
			BlobReachable: func(ctx context.Context) error {
				return bootstrap.CheckBlobStore(ctx, blobs, blobCfg)
			},
		})
		// Worker API listener: a separate bind (WORKER_LISTEN_ADDR, e.g. the
		// Tailscale IP) so worker tokens never share the UI listener's ACLs.
		// Unset means the API stays off — no implicit exposure.
		if addr := strings.TrimSpace(os.Getenv("WORKER_LISTEN_ADDR")); addr != "" {
			if err := runWorkerListener(addr, pool, blobs, blobCfg, keys); err != nil {
				return fmt.Errorf("worker listener: %w", err)
			}
		}
		// Hosted ffmpeg export runner: claims audio_export jobs, materializes
		// inputs to scratch, reuses the export pipeline, publishes blobs +
		// export_cache. In-process on the VPS; if the process dies mid-job the
		// lease expiry hands the job back to the queue.
		if runner, err := audio.NewPGExportRunner(ctx, audio.PGDeps{
			Jobs: postgres.NewJobs(pool), Audio: postgres.NewAudio(pool),
			Cache: postgres.NewExportCache(pool), Nar: nar, Blobs: blobs, Keys: keys,
		}); err == nil {
			go func() {
				if err := runner.Run(ctx, 3*time.Second); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("export runner dừng vì lỗi", "module", "serve", "err", err)
				}
			}()
		} else {
			slog.Error("không khởi động được export runner", "module", "serve", "err", err)
		}
	}
	// 14 §G8：nháp đồng sáng tác quá 7 ngày bị dọn khi khởi động.
	if absolute != "" {
		if lib, err := library.Open(absolute); err == nil {
			if n, err := lib.CleanupDrafts(7 * 24 * time.Hour); err == nil && n > 0 {
				fmt.Printf("Đã dọn %d bản nháp đồng sáng tác cũ.\n", n)
			}
		}
	}
	server := &http.Server{Handler: gateway}
	bound, port, _ := net.SplitHostPort(listener.Addr().String())
	base := func(host string) string { return "http://" + net.JoinHostPort(host, port) }
	ip := net.ParseIP(bound)
	if absolute == "" {
		fmt.Printf("Chưa chọn nơi lưu truyện — chọn ở bước hướng dẫn trên web.\nTài khoản: %s\n", accounts.Path())
	} else {
		fmt.Printf("Thư mục sách: %s\nTài khoản: %s\n", absolute, accounts.Path())
	}
	if ip.IsLoopback() || ip.IsUnspecified() {
		fmt.Printf("Mở trên máy này: %s\n", base("127.0.0.1"))
	}
	var phoneURLs []string
	switch {
	case ip.IsLoopback():
		network := gateway.EnableNetwork(web.NetworkOptions{Port: port})
		saved, _ := library.LoadServeConfig()
		if *lan || saved.LAN {
			if err := network.Set(true); err != nil {
				fmt.Printf("Không mở được kết nối cho điện thoại: %v\n", err)
			}
		}
		phoneURLs = network.URLs()
	case ip.IsUnspecified():
		for _, address := range web.LANAddresses() {
			gateway.PublicURLs = append(gateway.PublicURLs, base(address))
		}
		phoneURLs = gateway.PublicURLs
	default:
		gateway.PublicURLs = []string{base(bound)}
		phoneURLs = gateway.PublicURLs
	}
	// GAP-41：还没有账号 → 打印打开地址与设置码。码只在这一处出现（SetupCodeOnce 之后清空）。
	if code := gateway.SetupCodeOnce(); code != "" {
		target := base("127.0.0.1")
		if !ip.IsLoopback() && len(phoneURLs) > 0 {
			target = phoneURLs[0]
		}
		fmt.Printf("Mở trình duyệt để thiết lập: %s\n", target)
		fmt.Printf("Mã thiết lập: %s (hết hạn sau %d phút)\n", code, int(webauth.SetupCodeTTL.Minutes()))
	}
	for _, u := range phoneURLs {
		fmt.Printf("Điện thoại / mạng khác: %s\n", u)
	}
	if len(phoneURLs) > 0 {
		fmt.Println("Đăng nhập điện thoại: mở trang trên máy tính → Tài khoản → \"Đăng nhập điện thoại bằng QR\".")
		fmt.Println("Cảnh báo: kết nối là HTTP không mã hoá; ngoài mạng nhà hãy truy cập qua Tailscale.")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
	case err = <-done:
		if err != nil && err != http.ErrServerClosed {
			gateway.Close()
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownResult := make(chan error, 1)
	go func() { shutdownResult <- server.Shutdown(shutdown) }()
	gateway.Close()
	_ = voice.CloseShared()
	return <-shutdownResult
}

// openAccounts mở kho tài khoản theo STORE_DRIVER — cờ này chọn trọn bộ
// backend sự kiện + tài khoản, không có selector thứ hai.
// file: web-accounts.json như cũ. postgres: pool app-role + postgres.Accounts
// qua webauth.NewFromRepository. Trả kèm closer cho pool (no-op ở file).
func openAccounts(cfg bootstrap.StorageConfig) (*webauth.Store, func(), error) {
	if cfg.Driver == bootstrap.StoreDriverPostgres {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pool, err := bootstrap.OpenPool(ctx, cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("mở postgres cho tài khoản: %w", err)
		}
		return webauth.NewFromRepository(postgres.NewAccounts(pool)), pool.Close, nil
	}
	path, err := webauth.DefaultPath()
	if err != nil {
		return nil, nil, err
	}
	return webauth.NewStore(path), func() {}, nil
}

// promptNewPassword 在终端无回显地读两次密码；非终端（脚本管道）读一行。
func promptNewPassword(reader *bufio.Reader) (string, error) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		line, err := reader.ReadString('\n')
		if err != nil && !(errors.Is(err, io.EOF) && line != "") {
			return "", fmt.Errorf("đọc mật khẩu từ stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	read := func(label string) (string, error) {
		fmt.Print(label)
		b, err := term.ReadPassword(os.Stdin.Fd())
		fmt.Println()
		return string(b), err
	}
	first, err := read(fmt.Sprintf("Mật khẩu (ít nhất %d ký tự): ", webauth.MinPasswordLen))
	if err != nil {
		return "", err
	}
	second, err := read("Nhập lại mật khẩu: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("hai lần nhập mật khẩu không khớp")
	}
	return first, nil
}

const serveUserUsage = `Quản lý tài khoản web:
  ainovel-cli serve user add <tên> [--role admin|member] [--name "Tên hiển thị"] [--projects "du-an=collab,du-an-khac=view"]
  ainovel-cli serve user list
  ainovel-cli serve user passwd <tên>
  ainovel-cli serve user rm <tên>
Vai trò: admin (quản trị hệ thống) · member (thành viên).
--projects: "<dự án>=owner", "<dự án>=collab", "<dự án>=view", phân tách bằng dấu phẩy. Thành viên mặc định chưa được vào dự án nào.`

func runServeUser(args []string) error {
	if len(args) == 0 {
		fmt.Println(serveUserUsage)
		return nil
	}
	storageCfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return fmt.Errorf("cấu hình lưu trữ: %w", err)
	}
	accounts, closeAccounts, err := openAccounts(storageCfg)
	if err != nil {
		return err
	}
	defer closeAccounts()
	command, rest := args[0], args[1:]
	needName := func() (string, error) {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return "", fmt.Errorf("thiếu tên đăng nhập\n%s", serveUserUsage)
		}
		return rest[0], nil
	}
	actor := &webauth.AuditActor{Username: "dòng lệnh", Name: "Dòng lệnh"}
	switch command {
	case "add":
		username, err := needName()
		if err != nil {
			return err
		}
		flags := flag.NewFlagSet("serve user add", flag.ContinueOnError)
		role := flags.String("role", "", "admin | member (mặc định: admin nếu là tài khoản đầu tiên, còn lại member)")
		name := flags.String("name", "", "Tên hiển thị")
		projects := flags.String("projects", "", "Quyền dự án: dự-án=owner|collab|view, phân tách bằng dấu phẩy")
		if err := flags.Parse(rest[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("tham số không hợp lệ: %v", flags.Args())
		}
		if *role == "" {
			has, err := accounts.HasUsers()
			if err != nil {
				return err
			}
			*role = string(webauth.RoleMember)
			if !has {
				*role = string(webauth.RoleAdmin)
			}
		}
		password, err := promptNewPassword(bufio.NewReader(os.Stdin))
		if err != nil {
			return err
		}
		permissions := map[string]webauth.ProjectRole{}
		if *projects != "" {
			for _, entry := range strings.Split(*projects, ",") {
				id, level, ok := strings.Cut(strings.TrimSpace(entry), "=")
				if !ok {
					return fmt.Errorf("Quyền dự án cần có dạng dự-án=owner|collab|view.")
				}
				permissions[id] = webauth.ProjectRole(level)
			}
		}
		user, err := accounts.CreateUser(webauth.NewUser{Username: username, Name: *name, Role: webauth.Role(*role), Projects: permissions, Password: password, Actor: actor})
		if err != nil {
			return err
		}
		fmt.Printf("Đã tạo %q (vai trò %s).\n", user.Username, user.Role)
	case "list":
		users, err := accounts.Users()
		if err != nil {
			return err
		}
		if len(users) == 0 {
			fmt.Println("Chưa có tài khoản nào.")
		}
		for _, u := range users {
			status := ""
			if u.Disabled {
				status = " (đã khoá)"
			}
			permissions := make([]string, 0, len(u.Projects))
			for id, level := range u.Projects {
				permissions = append(permissions, id+"="+string(level))
			}
			sort.Strings(permissions)
			summary := strings.Join(permissions, ",")
			if u.Role == webauth.RoleAdmin {
				summary = "Mọi dự án"
			}
			fmt.Printf("%-20s %-7s dự án: %-30s %s%s\n", u.Username, u.Role, summary, u.Name, status)
		}
	case "passwd":
		username, err := needName()
		if err != nil {
			return err
		}
		password, err := promptNewPassword(bufio.NewReader(os.Stdin))
		if err != nil {
			return err
		}
		if _, err := accounts.UpdateUser(username, webauth.UserPatch{Password: &password, Actor: actor}); err != nil {
			return err
		}
		fmt.Printf("Đã đổi mật khẩu %q; mọi thiết bị của tài khoản này đã bị đăng xuất.\n", username)
	case "rm":
		username, err := needName()
		if err != nil {
			return err
		}
		if err := accounts.DeleteUser(username, webauth.AuditContext{Actor: actor}); err != nil {
			return err
		}
		fmt.Printf("Đã xoá %q.\n", username)
	default:
		return fmt.Errorf("lệnh không hợp lệ: %s\n%s", command, serveUserUsage)
	}
	return nil
}

// runWorkerListener serves the private worker API on its own bind address
// (WORKER_LISTEN_ADDR — typically the server's Tailscale IP). Tokens are
// checked against the durable jobs table; hosted workers claim PG jobs only,
// so the file-dir claim path is a no-op here.
func runWorkerListener(addr string, pool *postgres.Pool, blobs blobstore.BlobStore, blobCfg bootstrap.BlobConfig, keys blobkey.Keys) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("không nghe được %s: %w", addr, err)
	}
	queue := &web.PGWorkerQueue{
		Jobs: postgres.NewJobs(pool), Audio: postgres.NewAudio(pool),
	}
	api := &web.WorkerAPI{
		Queue: queue, Blobs: blobs, Registry: &web.PGAudioBlobs{Audio: postgres.NewAudio(pool)},
		BlobDriver: string(blobCfg.Driver),
		Keys:       keys,
	}
	server := &http.Server{Handler: api.Handler()}
	fmt.Printf("Worker API: http://%s/worker/...\n", listener.Addr().String())
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "worker listener lỗi: %v\n", err)
		}
	}()
	return nil
}
