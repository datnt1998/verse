package bootstrap

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/auth/anthropicoauth"
	"github.com/datnt1998/verse/core/auth/codex"
	"github.com/datnt1998/verse/core/auth/xai"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

// exampleConfig là mẫu cấu hình có chú thích ghi vào ~/.ainovel/config.example.jsonc.
//
//go:embed config.example.jsonc
var exampleConfig string

// NeedsSetup kiểm tra xem có cần chạy trình hướng dẫn khởi tạo lần đầu hay không.
func NeedsSetup() bool {
	if p := DefaultConfigPath(); p != "" {
		if _, err := os.Stat(p); err == nil {
			return false
		}
	}
	if _, err := os.Stat(projectConfigPath()); err == nil {
		return false
	}
	return true
}

type setupProvider struct {
	name           string
	label          string
	baseURL        string // base_url điền sẵn
	needType       bool   // Proxy tùy chỉnh cần hỏi thêm type và base_url
	apiKeyOptional bool   // true nếu cho phép để trống API Key
	oauth          bool   // Gói thuê bao: đăng nhập OAuth thay cho API Key/Base URL
}

// ProviderPreset là mục cấu hình Provider dùng chung cho Setup và lệnh /config.
type ProviderPreset struct {
	Name           string
	Label          string
	BaseURL        string
	NeedType       bool
	APIKeyOptional bool
	OAuth          bool
}

var setupProviders = []setupProvider{
	{name: "ollama", label: "Ollama (Cục bộ / Offline - Miễn phí)", baseURL: "http://localhost:11434/v1", apiKeyOptional: true},
	{name: "openrouter", label: "OpenRouter (Claude, Gemini, DeepSeek, Qwen...)", baseURL: "https://openrouter.ai/api/v1"},
	{name: "opencode-go", label: "OpenCode Go (Kimi, DeepSeek, GLM, Qwen, MiniMax… - gói thuê bao, dùng API key)", baseURL: "https://opencode.ai/inference/go/v1"},
	{name: codex.ProviderName, label: "ChatGPT Plus/Pro (Codex Subscription - đăng nhập OAuth)", apiKeyOptional: true, oauth: true},
	{name: xai.ProviderName, label: "SuperGrok / X Premium+ (Gói thuê bao xAI - đăng nhập OAuth)", apiKeyOptional: true, oauth: true},
	{name: anthropicoauth.ProviderName, label: "Claude Pro/Max (Gói thuê bao Anthropic - đăng nhập OAuth)", apiKeyOptional: true, oauth: true},
	{name: "gemini", label: "Google Gemini", baseURL: ""},
	{name: "anthropic", label: "Anthropic Claude", baseURL: ""},
	{name: "deepseek", label: "DeepSeek", baseURL: "https://api.deepseek.com/v1"},
	{name: "openai", label: "OpenAI", baseURL: ""},
	{name: "qwen", label: "Alibaba Qwen (DashScope)", baseURL: ""},
	{name: "glm", label: "Zhipu GLM", baseURL: ""},
	{name: "grok", label: "xAI Grok", baseURL: ""},
	{name: "bedrock", label: "AWS Bedrock", apiKeyOptional: true},
	{name: "custom", label: "Custom Proxy (Proxy / API tùy chỉnh)", needType: true, apiKeyOptional: true},
}

// ProviderPresets trả về danh sách các thiết lập Provider mẫu.
func ProviderPresets() []ProviderPreset {
	out := make([]ProviderPreset, 0, len(setupProviders))
	for _, preset := range setupProviders {
		out = append(out, ProviderPreset{
			Name: preset.name, Label: preset.label, BaseURL: preset.baseURL,
			NeedType: preset.needType, APIKeyOptional: preset.apiKeyOptional, OAuth: preset.oauth,
		})
	}
	return out
}

// RunSetup chạy trình hướng dẫn thiết lập lần đầu và trả về cấu hình tạo được.
func RunSetup() (Config, error) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).
		Render("Chưa tìm thấy tệp cấu hình, bắt đầu khởi tạo thiết lập..."))
	fmt.Fprintf(os.Stderr, "  Đường dẫn tệp cấu hình: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(DefaultConfigPath()))
	fmt.Fprintf(os.Stderr, "  Sau khi hoàn tất, bạn có thể chỉnh sửa tệp này để tùy biến nâng cao.\n")
	fmt.Fprintln(os.Stderr)

	// Step 1: Chọn Nhà cung cấp AI (Provider)
	sp, err := runProviderSelect()
	if err != nil {
		return Config{}, err
	}

	providerName := sp.name
	var pc llmconfig.ProviderConfig
	printStepDone("Nhà cung cấp AI", sp.label)

	// Tùy biến proxy: hỏi thêm tên và giao thức API
	if sp.needType {
		providerName, err = runTextInput("Tên Provider", "my-proxy")
		if err != nil {
			return Config{}, err
		}
		providerType, err := runTypeSelect()
		if err != nil {
			return Config{}, err
		}
		pc.Type = providerType
	}

	if sp.oauth {
		// Step 2-3: Đăng nhập gói thuê bao thay cho API Key / Base URL
		label, err := ensureOAuthLogin(providerName)
		if err != nil {
			return Config{}, err
		}
		printStepDone("Đăng nhập", label)
	} else {
		// Step 2: Nhập API Key
		var apiKey string
		if sp.apiKeyOptional {
			apiKey, err = runOptionalTextInput("[2/4] API Key (Nhấn Enter để bỏ qua nếu dùng Ollama/Local)", "Để trống nếu không cần API Key")
		} else {
			apiKey, err = runTextInput("[2/4] API Key", "sk-xxx...")
		}
		if err != nil {
			return Config{}, err
		}
		pc.APIKey = apiKey
		if apiKey == "" {
			printStepDone("API Key", "Không sử dụng (Mặc định cho Ollama/Local)")
		} else {
			printStepDone("API Key", maskKey(apiKey))
		}

		// Step 3: Base URL (Nhấn Enter để dùng mặc định)
		baseDefault := sp.baseURL
		baseHint := "Để trống dùng địa chỉ mặc định"
		if baseDefault != "" {
			baseHint = baseDefault
		}
		baseURL, err := runTextInputWithDefault("[3/4] Base URL (Nhấn Enter để dùng địa chỉ mặc định, hoặc nhập địa chỉ proxy/Ollama)", baseHint, baseDefault)
		if err != nil {
			return Config{}, err
		}
		pc.BaseURL = baseURL
		if baseURL != "" {
			printStepDone("Base URL", baseURL)
		} else {
			printStepDone("Base URL", "Mặc định")
		}
	}

	// Step 4: Tên Model (bắt buộc)
	var modelCfg llmconfig.ModelConfig
	if sp.oauth {
		modelCfg, err = runOAuthModelSelect(providerName)
	} else {
		modelPlaceholder := "Ví dụ: qwen2.5:14b / ainovel-qwen / google/gemini-2.5-flash / claude-3-5-sonnet"
		if providerName == "ollama" {
			modelPlaceholder = "Ví dụ: qwen2.5:14b / ainovel-qwen / qwen3:14b"
		}
		modelCfg.Name, err = runTextInput("[4/4] Tên Model chính", modelPlaceholder)
	}
	if err != nil {
		return Config{}, err
	}
	modelName := modelCfg.Name
	printStepDone("Model", modelName)
	pc.Models = []llmconfig.ModelConfig{modelCfg}

	cfg := Config{
		Provider:  providerName,
		ModelName: modelName,
		Providers: map[string]llmconfig.ProviderConfig{providerName: pc},
		Roles:     map[string]roles.RoleConfig{},
		Style:     "default",
	}

	// Lưu cấu hình
	path := DefaultConfigPath()
	if err := SaveConfig(path, cfg); err != nil {
		return cfg, fmt.Errorf("lỗi lưu cấu hình: %w", err)
	}

	// Tạo file ví dụ mẫu
	saveExampleConfig()

	rulesDir := rules.DefaultHomeRulesDir()

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s Cấu hình đã được lưu tại: %s\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓"), path)
	fmt.Fprintf(os.Stderr, "  Provider mặc định: %s\n", providerName)
	fmt.Fprintf(os.Stderr, "  Model mặc định: %s\n", modelName)
	fmt.Fprintln(os.Stderr, "  Bạn có thể dùng lệnh /config hoặc /model trong TUI để thay đổi bất cứ lúc nào.")
	if rulesDir != "" {
		fmt.Fprintf(os.Stderr, "  Các quy tắc và phong cách viết cá nhân có thể đặt tại thư mục: %s\n", rulesDir)
	}
	fmt.Fprintln(os.Stderr)

	return cfg, nil
}

func saveExampleConfig() {
	dir, err := configDir()
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "config.example.jsonc"), []byte(exampleConfig), 0o644)
}

// ensureOAuthLogin dùng phiên đã đăng nhập nếu có, nếu không chạy luồng đăng nhập
// trình duyệt; trả về nhãn tài khoản.
func ensureOAuthLogin(id string) (string, error) {
	store, err := auth.DefaultStore()
	if err != nil {
		return "", err
	}
	if cred, ok, err := store.Get(id); err != nil {
		return "", err
	} else if ok {
		return cred.Label(), nil
	}
	switch id {
	case xai.ProviderName:
		fmt.Fprintln(os.Stderr, "  [2/4] Đăng nhập tài khoản SuperGrok / X Premium+ (mở link bên dưới trên bất kỳ thiết bị nào rồi nhập mã)")
	case anthropicoauth.ProviderName:
		fmt.Fprintln(os.Stderr, "  [2/4] Đăng nhập tài khoản Claude Pro/Max (trình duyệt sẽ mở; SSH/headless: Ctrl+C rồi chạy `ainovel-cli login anthropic-oauth --device`)")
	default:
		fmt.Fprintln(os.Stderr, "  [2/4] Đăng nhập tài khoản ChatGPT (trình duyệt sẽ mở; SSH/headless: Ctrl+C rồi chạy `ainovel-cli login --device`)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cred, err := auth.Login(ctx, store, id, auth.ConsoleController(os.Stderr), auth.LoginOptions{})
	if err != nil {
		return "", fmt.Errorf("đăng nhập %s: %w", id, err)
	}
	return cred.Label(), nil
}

// printStepDone in dòng xác nhận hoàn thành một bước.
func printStepDone(label, value string) {
	fmt.Fprintf(os.Stderr, "  %s %s: %s\n",
		lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓"),
		label,
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(value))
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// ---------- TUI Components ----------

func runProviderSelect() (setupProvider, error) {
	m := setupSelectModel{
		title: "[1/4] Chọn Nhà Cung Cấp AI (Provider)",
		items: setupProviders,
	}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return setupProvider{}, err
	}
	result := final.(setupSelectModel)
	if result.cancelled {
		return setupProvider{}, fmt.Errorf("đã hủy khởi tạo")
	}
	return result.items[result.cursor], nil
}

var apiTypeOptions = []setupProvider{
	{name: "openai", label: "Chuẩn OpenAI (Tương thích phần lớn các bên)"},
	{name: "anthropic", label: "Chuẩn Anthropic"},
	{name: "gemini", label: "Chuẩn Google Gemini"},
}

func runTypeSelect() (string, error) {
	m := setupSelectModel{
		title: "Loại giao thức API (API Protocol Type)",
		items: apiTypeOptions,
	}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result := final.(setupSelectModel)
	if result.cancelled {
		return "", fmt.Errorf("đã hủy khởi tạo")
	}
	return result.items[result.cursor].name, nil
}

func runTextInput(label, placeholder string) (string, error) {
	return runTextInputWithDefault(label, placeholder, "")
}

// oauthModel là một model mà tài khoản gói thuê bao được dùng.
type oauthModel struct {
	slug, display string
	contextWindow int
}

// listOAuthModels lấy danh sách model của tài khoản đã đăng nhập theo provider OAuth id.
func listOAuthModels(ctx context.Context, id string, manager *auth.Manager) ([]oauthModel, error) {
	var out []oauthModel
	switch id {
	case xai.ProviderName:
		models, err := xai.ListModels(ctx, "", xai.ManagerTokens(manager))
		for _, m := range models {
			out = append(out, oauthModel{slug: m.Slug, display: m.DisplayName})
		}
		return out, err
	case anthropicoauth.ProviderName:
		models, err := anthropicoauth.ListModels(ctx, "", anthropicoauth.ManagerTokens(manager))
		for _, m := range models {
			out = append(out, oauthModel{slug: m.Slug, display: m.DisplayName})
		}
		return out, err
	}
	models, err := codex.ListModels(ctx, "", codex.ManagerTokens(manager))
	for _, m := range models {
		out = append(out, oauthModel{slug: m.Slug, display: m.DisplayName, contextWindow: m.ContextWindow})
	}
	return out, err
}

// runOAuthModelSelect cho chọn trong danh sách model mà tài khoản gói thuê bao được
// dùng (kèm context window nếu backend công bố); không lấy được danh sách thì rơi về
// nhập tay.
func runOAuthModelSelect(id string) (llmconfig.ModelConfig, error) {
	manager, err := auth.DefaultManager()
	if err != nil {
		return llmconfig.ModelConfig{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	available, err := listOAuthModels(ctx, id, manager)
	if err != nil || len(available) == 0 {
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Không lấy được danh sách model (%v), vui lòng nhập tay.\n", err)
		}
		example := "Ví dụ: gpt-5.5 (xem `ainovel-cli login --status`)"
		switch id {
		case xai.ProviderName:
			example = "Ví dụ: " + xai.DefaultModel
		case anthropicoauth.ProviderName:
			example = "Ví dụ: " + anthropicoauth.DefaultModel
		}
		name, err := runTextInput("[4/4] Tên Model chính", example)
		return llmconfig.ModelConfig{Name: name}, err
	}
	items := make([]setupProvider, len(available))
	for i, m := range available {
		label := m.slug
		if m.display != "" && m.display != m.slug {
			label = m.display + " (" + m.slug + ")"
		}
		if m.contextWindow > 0 {
			label += fmt.Sprintf(" · ngữ cảnh %dK", m.contextWindow/1000)
		}
		items[i] = setupProvider{name: m.slug, label: label}
	}
	final, err := tea.NewProgram(setupSelectModel{title: "[4/4] Chọn Model chính", items: items}, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return llmconfig.ModelConfig{}, err
	}
	result := final.(setupSelectModel)
	if result.cancelled {
		return llmconfig.ModelConfig{}, fmt.Errorf("đã hủy khởi tạo")
	}
	chosen := available[result.cursor]
	return llmconfig.ModelConfig{Name: chosen.slug, ContextWindow: chosen.contextWindow}, nil
}

func runOptionalTextInput(label, placeholder string) (string, error) {
	m := setupInputModel{label: label, placeholder: placeholder, allowEmpty: true}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result := final.(setupInputModel)
	if result.cancelled {
		return "", fmt.Errorf("đã hủy khởi tạo")
	}
	return utils.CleanInputLine(result.value), nil
}

func runTextInputWithDefault(label, placeholder, defaultValue string) (string, error) {
	m := setupInputModel{label: label, placeholder: placeholder, defaultValue: defaultValue}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result := final.(setupInputModel)
	if result.cancelled {
		return "", fmt.Errorf("đã hủy khởi tạo")
	}
	if result.value == "" && result.defaultValue != "" {
		return result.defaultValue, nil
	}
	return utils.CleanInputLine(result.value), nil
}

// ---------- Select Component ----------

var (
	setupCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	setupDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	setupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	setupInputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

type setupSelectModel struct {
	title     string
	items     []setupProvider
	cursor    int
	cancelled bool
}

func (m setupSelectModel) Init() tea.Cmd { return nil }

func (m setupSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m setupSelectModel) View() string {
	var b strings.Builder
	b.WriteString(setupHeaderStyle.Render(m.title))
	b.WriteString("\n\n")
	for i, item := range m.items {
		cursor := "  "
		label := item.label
		if i == m.cursor {
			cursor = setupCursorStyle.Render("❯ ")
			label = setupCursorStyle.Render(label)
		}
		b.WriteString(cursor + label + "\n")
	}
	b.WriteString(setupDimStyle.Render("\n  ↑↓ Chọn  Enter Xác nhận  Esc Hủy"))
	return b.String()
}

// ---------- Input Component ----------

type setupInputModel struct {
	label        string
	placeholder  string
	defaultValue string
	allowEmpty   bool
	value        string
	cancelled    bool
}

func (m setupInputModel) Init() tea.Cmd { return nil }

func (m setupInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			if utils.CleanInputLine(m.value) != "" || m.defaultValue != "" || m.allowEmpty {
				return m, tea.Quit
			}
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "backspace":
			if len(m.value) > 0 {
				runes := []rune(m.value)
				m.value = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.value += utils.CleanInputRunes(msg.Runes)
			} else if msg.Type == tea.KeySpace {
				m.value += " "
			}
		}
	}
	return m, nil
}

func (m setupInputModel) View() string {
	var b strings.Builder
	b.WriteString(setupHeaderStyle.Render(m.label))
	b.WriteString("\n\n")
	b.WriteString(setupInputStyle.Render("❯ "))
	if m.value == "" {
		b.WriteString(setupCursorStyle.Render("▌"))
		b.WriteString(setupDimStyle.Render(m.placeholder))
	} else {
		b.WriteString(m.value)
		b.WriteString(setupCursorStyle.Render("▌"))
	}
	b.WriteString(setupDimStyle.Render("  (Enter Xác nhận, Esc Hủy)"))
	b.WriteString("\n")
	return b.String()
}
