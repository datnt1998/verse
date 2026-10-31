// viderse là CLI của Viderse bản Go: serve giao diện web, migrate áp schema
// Postgres, login đăng nhập gói thuê bao model (chữ/ảnh đi qua OAuth Codex).
package main

import (
	"fmt"
	"os"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
)

const usage = `Cách dùng:
  viderse serve [-addr HOST:PORT]     Chạy web UI (mặc định 127.0.0.1:8789)
  viderse migrate up|status|version   Quản lý schema Postgres (VIDERSE_DATABASE_URL)
  viderse storage import|materialize|gc [--dry-run|--apply]   Bảo trì kho máy chủ
  viderse user grant <tên> --app noverse|viderse   Cấp quyền dùng một app
  viderse user revoke <tên> --app noverse|viderse  Thu hồi quyền một app
  viderse login [provider] [--device] Đăng nhập gói thuê bao (mặc định: openai-codex)
  viderse rival add|list|scan|videos|posting-slots|remove  Đối thủ YouTube (VIDERSE_DATABASE_URL)
  viderse channel list|show|create|update|delete|link|unlink|template|identity  Kênh của bạn
  viderse template list|show|create|duplicate|rename|set|delete  Template của Thư viện
  viderse overlay list|add|remove                Lớp phủ dùng chung của Thư viện
  viderse backdrop list|add|show|videos|refresh|remove  Video nền (kênh Twitch/Bilibili, video lẻ)
  viderse backdrop video add|list|remove         Video lẻ của Video nền
  viderse backdrop parse <link>                  Phân loại link nền tảng
  viderse clone run|batch|pause|resume|cancel|status|wait  Điều khiển pipeline qua máy chủ
  viderse logout [provider]           Xoá phiên đăng nhập
  viderse login --status              Xem trạng thái đăng nhập
  viderse version                     In phiên bản

  --device  Dùng mã thiết bị thay vì callback localhost (SSH/headless); xai-oauth luôn
            dùng mã thiết bị. anthropic-oauth --device: mở link ở máy bất kỳ rồi dán
            mã (code#state) hoặc URL chuyển hướng vào đây.

Biến môi trường:
  VIDERSE_DATABASE_URL           DSN Postgres của app (bắt buộc cho serve/migrate)
  VIDERSE_MIGRATION_DATABASE_URL DSN role migrate (mặc định dùng chung DSN app)
  VERSE_IDENTITY_DATABASE_URL    DSN kho tài khoản chung, role viderse_identity
                                 (bắt buộc cho serve và user grant|revoke)
  VIDERSE_ADDR                   Địa chỉ lắng nghe (ghi đè bởi -addr)
  VIDERSE_DAILY_PAID_IMAGE_LIMIT Trần ảnh trả phí/ngày/nguồn, 1–50 (mặc định 50)
  TRUSTED_PROXIES / PUBLIC_URL / SECURE_COOKIES   Ranh giới proxy (platform/webguard)
  VERSE_COOKIE_DOMAIN / VERSE_APP_ORIGINS         Cookie chung + allowlist origin
`

func main() {
	// Cài danh tính app cho core/auth trước khi bất kỳ lệnh nào mở kho
	// credential hay render lỗi — tường minh thay vì init() ngầm.
	bootstrap.ConfigureIdentity()

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	code := 0
	switch args[0] {
	case "storage":
		err = runStorage(args[1:])
	case "serve":
		err = runServe(args[1:])
	case "user":
		err = runUser(args[1:])
	case "migrate":
		err = runMigrate(args[1:])
	case "rival":
		code = runRivalCommand(args[1:], os.Stdout, os.Stderr)
	case "channel":
		code = runChannelCommand(args[1:], os.Stdout, os.Stderr)
	case "template":
		code = runTemplateCommand(args[1:], os.Stdout, os.Stderr)
	case "overlay":
		code = runOverlayCommand(args[1:], os.Stdout, os.Stderr)
	case "backdrop":
		code = runBackdropCommand(args[1:], os.Stdout, os.Stderr)
	case "clone":
		code = runCloneCommand(args[1:], os.Stdout, os.Stderr)
	case "login":
		code = runLoginCommand(args[1:], os.Stdout, os.Stderr)
	case "logout":
		code = runLogoutCommand(args[1:], os.Stdout, os.Stderr)
	case "version", "--version", "-v":
		fmt.Println("viderse dev")
		return
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "Lệnh không hợp lệ %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lỗi: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}
