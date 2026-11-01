package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/headless"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/startup"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/tui"
	"github.com/datnt1998/verse/apps/noverse/internal/entry/web"
	"github.com/datnt1998/verse/apps/noverse/internal/eval"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	buildversion "github.com/datnt1998/verse/apps/noverse/internal/version"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const cliUsage = `Cách dùng:
  ainovel-cli                              Mở giao diện viết truyện trong terminal
  ainovel-cli serve [--root DIR] [--addr HOST:PORT]
                                           Mở giao diện web
  ainovel-cli serve user add|list|passwd|rm  Quản lý tài khoản web
  ainovel-cli login [provider] [--device]    Đăng nhập gói thuê bao AI
  ainovel-cli login --status                Xem trạng thái đăng nhập
  ainovel-cli logout [provider]             Xoá phiên đăng nhập

Quản trị lưu trữ:
  ainovel-cli storage migrate up|status|version
                                           Cập nhật hoặc xem trạng thái cơ sở dữ liệu
  ainovel-cli storage copy --from-config A --to-config B --manifest M
                                           Sao chép và đối soát dữ liệu nhị phân
  ainovel-cli storage import --root R --accounts A --source-id U --owner-map S=U --manifest M
                                           Nhập thư viện tệp vào cơ sở dữ liệu
  ainovel-cli storage reconcile --manifest M [--verify-blobs] [--json]
                                           Đối soát thư viện đã nhập
  ainovel-cli storage export-files --destination DIR [--revoke-sessions]
                                           Xuất cơ sở dữ liệu về thư viện tệp
  ainovel-cli storage gc [--dry-run] [--json]
                                           Dọn dữ liệu lưu trữ không còn dùng
  ainovel-cli storage purge-audit [--days N] Xoá nhật ký quản trị cũ

Máy đọc chạy nền:
  ainovel-cli worker tts --api URL [--project SLUG]
                                           Nhận việc đọc truyện (cần TTS_WORKER_TOKEN)
  ainovel-cli worker token create [--cap tts] NAME
                                           Cấp mã truy cập cho máy đọc
  ainovel-cli worker token list             Liệt kê máy đọc
  ainovel-cli worker token revoke ID_OR_NAME
                                           Thu hồi mã truy cập

Lệnh khác:
  ainovel-cli version | --version | -v       Xem phiên bản
  ainovel-cli update [VERSION]              Cập nhật chương trình
  ainovel-cli voice install|status|voices|say
                                           Cài đặt, kiểm tra hoặc dùng giọng đọc
  ainovel-cli eval --cases PATH             Chạy bộ đánh giá
  ainovel-cli --headless [--prompt TEXT | --prompt-file PATH]
                                           Chạy không có giao diện (PATH là - để đọc stdin)
  ainovel-cli --help | -h | help            Xem hướng dẫn này
`

// headlessMode ghi nhận chế độ khởi động headless.
var headlessMode bool

func main() {
	// 记录构建身份：store/revision 写 ChapterRecord 时用 version.Build() 盖戳。
	buildversion.SetBuild(buildversion.Info{Version: version, Commit: commit, Date: date})
	if len(os.Args) > 1 && os.Args[1] == "eval" {
		os.Exit(eval.Command(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "login" {
		os.Exit(runLoginCommand(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "logout" {
		os.Exit(runLogoutCommand(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "voice" {
		os.Exit(runVoiceCommand(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "storage" {
		if err := runStorage(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Lỗi storage: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		os.Exit(runWorkerCommand(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Lỗi serve: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "__web-worker" {
		if err := web.RunWorker(versionInfo()); err != nil {
			fmt.Fprintf(os.Stderr, "Worker không khởi động được: %v\n", err)
			os.Exit(1)
		}
		return
	}

	opts, args, err := parseCLIOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lỗi tham số: %v\n", err)
		os.Exit(2)
	}
	if opts.Help {
		fmt.Fprint(os.Stdout, cliUsage)
		return
	}
	if opts.Version {
		buildversion.Print(os.Stdout, versionInfo())
		return
	}
	if opts.Update {
		if err := runSelfUpdate(opts.UpdateVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Cập nhật thất bại: %v\n", err)
			os.Exit(1)
		}
		return
	}
	headlessMode = opts.Headless

	// Khởi tạo lần đầu nếu chưa có cấu hình
	if bootstrap.NeedsSetup() {
		if opts.Headless {
			die("Lỗi: Chế độ headless không hỗ trợ thiết lập lần đầu, vui lòng chạy chế độ TUI trước để hoàn tất cấu hình.")
		}
		setupCfg, err := bootstrap.RunSetup()
		if err != nil {
			die("Thiết lập thất bại: %v", err)
		}
		runWithConfig(setupCfg, opts, args)
		return
	}

	// Đọc cấu hình
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		die("Lỗi cấu hình: %v", err)
	}

	runWithConfig(cfg, opts, args)
}

// die xử lý thoát khi có lỗi nghiêm trọng: in ra stderr, lưu log và tạm dừng nếu ở terminal tương tác.
func die(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	if path := bootstrap.WriteStartupError(msg); path != "" {
		fmt.Fprintf(os.Stderr, " (Chi tiết lỗi đã được ghi vào: %s)\n", path)
	}
	if !headlessMode && stdinIsTerminal() {
		fmt.Fprint(os.Stderr, "\nNhấn Enter để thoát...")
		fmt.Fscanln(os.Stdin)
	}
	os.Exit(1)
}

func stdinIsTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd())
}

func runWithConfig(cfg bootstrap.Config, opts cliOptions, args []string) {
	rules.EnsureHomeRulesDir()

	if len(args) > 0 {
		die("Lỗi: Không hỗ trợ truyền trực tiếp yêu cầu truyện qua dòng lệnh, vui lòng nhập vào ô chat trong giao diện TUI sau khi khởi động.")
	}

	cfg.FillDefaults()
	bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(cfg.OutputDir))
	if bundle.Err != nil {
		die("Lỗi: %v", bundle.Err)
	}
	if opts.Headless {
		prompt, err := loadPrompt(opts)
		if err != nil {
			die("Lỗi: %v", err)
		}
		if err := headless.Run(cfg, bundle, headless.Options{Prompt: prompt}); err != nil {
			die("Lỗi: %v", err)
		}
		return
	}
	if opts.Prompt != "" || opts.PromptFile != "" {
		die("Lỗi: Tham số --prompt / --prompt-file chỉ có thể sử dụng trong chế độ --headless.")
	}
	if err := tui.Run(cfg, bundle, versionInfo()); err != nil {
		die("Lỗi: %v", err)
	}
}

type cliOptions struct {
	Help          bool
	Headless      bool
	Prompt        string
	PromptFile    string
	Version       bool
	Update        bool
	UpdateVersion string
}

func parseCLIOptions(argv []string) (cliOptions, []string, error) {
	var opts cliOptions
	var args []string
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--help", "-h", "help":
			opts.Help = true
		case "--version", "-v":
			opts.Version = true
		case "version":
			if i+1 < len(argv) {
				return opts, nil, fmt.Errorf("lệnh version không nhận thêm tham số")
			}
			opts.Version = true
		case "update":
			if opts.Update {
				return opts, nil, fmt.Errorf("lệnh update chỉ có thể chỉ định một lần")
			}
			opts.Update = true
			if i+1 < len(argv) {
				if strings.HasPrefix(argv[i+1], "-") {
					return opts, nil, fmt.Errorf("lệnh update chỉ nhận một tham số phiên bản tùy chọn")
				}
				opts.UpdateVersion = argv[i+1]
				i++
			}
			if i+1 < len(argv) {
				return opts, nil, fmt.Errorf("lệnh update chỉ nhận một tham số phiên bản tùy chọn")
			}
		case "--headless":
			opts.Headless = true
		case "--prompt":
			if i+1 >= len(argv) {
				return opts, nil, fmt.Errorf("--prompt thiếu giá trị")
			}
			opts.Prompt = argv[i+1]
			i++
		case "--prompt-file":
			if i+1 >= len(argv) {
				return opts, nil, fmt.Errorf("--prompt-file thiếu giá trị")
			}
			opts.PromptFile = argv[i+1]
			i++
		default:
			if strings.HasPrefix(argv[i], "-") {
				return opts, nil, fmt.Errorf("không nhận diện cờ %q; dùng --help để xem hướng dẫn", argv[i])
			}
			args = append(args, argv[i])
		}
	}
	if opts.Prompt != "" && opts.PromptFile != "" {
		return opts, nil, fmt.Errorf("--prompt và --prompt-file không thể dùng đồng thời")
	}
	if opts.Version && (opts.Update || opts.Headless || opts.Prompt != "" || opts.PromptFile != "" || len(args) > 0) {
		return opts, nil, fmt.Errorf("version không thể dùng chung với các tham số khởi động khác")
	}
	if opts.Update && (opts.Headless || opts.Prompt != "" || opts.PromptFile != "" || len(args) > 0) {
		return opts, nil, fmt.Errorf("update không thể dùng chung với các tham số khởi động khác")
	}
	return opts, args, nil
}

func versionInfo() buildversion.Info {
	return buildversion.Resolve(buildversion.Info{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
}

func runSelfUpdate(target string) error {
	info := versionInfo()
	result, err := buildversion.Update(context.Background(), buildversion.UpdateOptions{
		Repo:           buildversion.ReleaseRepo,
		BinaryName:     "ainovel-cli",
		TargetVersion:  target,
		CurrentVersion: info.Version,
	})
	if err != nil {
		return err
	}
	if !result.Updated {
		fmt.Printf("Noverse hiện đã là phiên bản mới nhất (%s)\n", result.Version)
		return nil
	}
	fmt.Printf("Noverse đã được cập nhật lên phiên bản: %s\n", result.Version)
	fmt.Printf("Vị trí cài đặt: %s\n", result.Path)
	return nil
}

func loadPrompt(opts cliOptions) (string, error) {
	return loadPromptFrom(opts, os.Stdin)
}

func loadPromptFrom(opts cliOptions, stdin io.Reader) (string, error) {
	if opts.PromptFile == "" {
		return strings.TrimSpace(opts.Prompt), nil
	}

	if opts.PromptFile == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("đọc prompt thất bại: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return startup.LoadPromptFile(opts.PromptFile)
}
