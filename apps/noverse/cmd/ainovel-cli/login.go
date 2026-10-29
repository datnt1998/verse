package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/anthropicoauth"
	"github.com/datnt1998/noverse/internal/auth"
	"github.com/datnt1998/noverse/internal/codex"
	"github.com/datnt1998/noverse/internal/xai"
)

const loginUsage = `Cách dùng:
  ainovel-cli login [provider] [--device]   Đăng nhập gói thuê bao (mặc định: openai-codex; hoặc xai-oauth cho SuperGrok / X Premium+, anthropic-oauth cho Claude Pro/Max)
  ainovel-cli logout [provider]             Xoá phiên đăng nhập
  ainovel-cli login --status                Xem trạng thái đăng nhập

  --device  Dùng mã thiết bị thay vì callback localhost (SSH/headless); xai-oauth luôn dùng mã thiết bị.
            anthropic-oauth --device: mở link ở máy bất kỳ rồi dán mã (code#state) hoặc URL chuyển hướng vào đây
`

// runLoginCommand xử lý `ainovel-cli login ...`; trả về exit code.
func runLoginCommand(args []string, stdout, stderr io.Writer) int {
	var (
		providerID = auth.OpenAICodexID
		opts       auth.LoginOptions
		status     bool
		named      bool
	)
	for _, arg := range args {
		switch arg {
		case "--device":
			opts.Device = true
		case "--status":
			status = true
		case "-h", "--help":
			fmt.Fprint(stdout, loginUsage)
			return 0
		default:
			if strings.HasPrefix(arg, "-") || named {
				fmt.Fprintf(stderr, "Tham số không hợp lệ: %s\n\n%s", arg, loginUsage)
				return 2
			}
			providerID, named = arg, true
		}
	}

	store, err := auth.DefaultStore()
	if err != nil {
		fmt.Fprintf(stderr, "Lỗi: %v\n", err)
		return 1
	}
	if status {
		return printLoginStatus(store, stdout, stderr)
	}
	provider, ok := auth.Lookup(providerID)
	if !ok {
		fmt.Fprintf(stderr, "Provider %q không hỗ trợ đăng nhập. Hỗ trợ: %s\n", providerID, supportedLoginProviders())
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	fmt.Fprintf(stdout, "Đăng nhập %s…\n", provider.Name())
	if auth.PasteCode(provider.ID()) {
		opts.Manual = readPastedCodes(ctx, os.Stdin, stdout)
	}
	cred, err := auth.Login(ctx, store, provider.ID(), auth.ConsoleController(stdout), opts)
	if err != nil {
		fmt.Fprintf(stderr, "Đăng nhập thất bại: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Đã đăng nhập %s: %s\nCredential lưu tại %s\n", provider.ID(), cred.Label(), store.Path())
	if provider.ID() == auth.OpenAICodexID || provider.ID() == auth.XAIOAuthID || provider.ID() == auth.AnthropicOAuthID {
		printOAuthModels(ctx, store, provider.ID(), stdout)
		fmt.Fprintf(stdout, "Dùng trong config: \"provider\": %q, \"providers\": {%q: {}}, \"model\": \"<model ở trên>\"\n", provider.ID(), provider.ID())
	}
	return 0
}

// runLogoutCommand xử lý `ainovel-cli logout [provider]`.
func runLogoutCommand(args []string, stdout, stderr io.Writer) int {
	providerID := auth.OpenAICodexID
	switch {
	case len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")):
		fmt.Fprint(stderr, loginUsage)
		return 2
	case len(args) == 1:
		providerID = args[0]
	}
	store, err := auth.DefaultStore()
	if err != nil {
		fmt.Fprintf(stderr, "Lỗi: %v\n", err)
		return 1
	}
	existed, err := store.Delete(providerID)
	if err != nil {
		fmt.Fprintf(stderr, "Đăng xuất thất bại: %v\n", err)
		return 1
	}
	if !existed {
		fmt.Fprintf(stdout, "%s chưa đăng nhập.\n", providerID)
		return 0
	}
	fmt.Fprintf(stdout, "Đã đăng xuất %s.\n", providerID)
	return 0
}

func printLoginStatus(store *auth.Store, stdout, stderr io.Writer) int {
	for _, provider := range auth.Providers() {
		cred, ok, err := store.Get(provider.ID())
		if err != nil {
			fmt.Fprintf(stderr, "Lỗi: %v\n", err)
			return 1
		}
		if !ok {
			fmt.Fprintf(stdout, "%-14s chưa đăng nhập\n", provider.ID())
			continue
		}
		state := "token còn hạn đến " + cred.ExpiresAt().Local().Format("2006-01-02 15:04")
		if !time.Now().Before(cred.ExpiresAt()) {
			state = "token hết hạn (sẽ tự làm mới khi dùng)"
		}
		fmt.Fprintf(stdout, "%-14s %s — %s\n", provider.ID(), cred.Label(), state)
		if provider.ID() == auth.OpenAICodexID || provider.ID() == auth.XAIOAuthID || provider.ID() == auth.AnthropicOAuthID {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			printOAuthModels(ctx, store, provider.ID(), stdout)
			cancel()
		}
	}
	return 0
}

// printOAuthModels in danh sách model tài khoản được dùng; lỗi chỉ in cảnh báo.
func printOAuthModels(ctx context.Context, store *auth.Store, id string, w io.Writer) {
	manager := auth.NewManager(store)
	var lines []string
	var err error
	switch id {
	case xai.ProviderName:
		var models []xai.Model
		models, err = xai.ListModels(ctx, "", xai.ManagerTokens(manager))
		for _, m := range models {
			lines = append(lines, "    "+m.Slug)
		}
	case anthropicoauth.ProviderName:
		var models []anthropicoauth.Model
		models, err = anthropicoauth.ListModels(ctx, "", anthropicoauth.ManagerTokens(manager))
		for _, m := range models {
			lines = append(lines, "    "+m.Slug)
		}
	default:
		var models []codex.Model
		models, err = codex.ListModels(ctx, "", codex.ManagerTokens(manager))
		for _, m := range models {
			line := "    " + m.Slug
			if m.ContextWindow > 0 {
				line += fmt.Sprintf("  (ngữ cảnh %dK)", m.ContextWindow/1000)
			}
			lines = append(lines, line)
		}
	}
	if err != nil {
		fmt.Fprintf(w, "  (không lấy được danh sách model: %v)\n", err)
		return
	}
	fmt.Fprintln(w, "  Model khả dụng:")
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

func supportedLoginProviders() string {
	ids := make([]string, 0)
	for _, p := range auth.Providers() {
		ids = append(ids, p.ID())
	}
	return strings.Join(ids, ", ")
}

// readPastedCodes đọc từng dòng stdin làm mã dán tay cho luồng đăng nhập hỗ trợ dán mã
// (URL chuyển hướng hoặc code#state). Mã bị từ chối thì báo lý do và chờ dòng kế tiếp.
// Stdin đóng/EOF thì dừng đọc — luồng callback localhost vẫn chạy bình thường.
func readPastedCodes(ctx context.Context, in io.Reader, out io.Writer) <-chan auth.ManualSubmission {
	ch := make(chan auth.ManualSubmission)
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			result := make(chan error, 1)
			select {
			case ch <- auth.ManualSubmission{Input: line, Result: result}:
			case <-ctx.Done():
				return
			}
			select {
			case err := <-result:
				if err != nil {
					fmt.Fprintf(out, "Mã không dùng được: %v — dán lại mã khác:\n", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}
