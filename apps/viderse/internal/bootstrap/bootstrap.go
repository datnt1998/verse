// Package bootstrap wires app identity and model configuration for viderse:
// where credentials live, which config file drives roles, and how a
// llm.ChatModel is built for the writer role.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	llmtypes "github.com/datnt1998/verse/core/llm"

	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

const configDirName = ".viderse"

// core/auth 是跨应用共享包：credential 存储路径、OAuth 回调页品牌、错误里的
// 登录命令、Claude Code wire version 的环境变量都来自 auth.Options。本二进制
// 的身份必须在任何包打开 credential 库之前装好，因此放在 init()：main 运行
// 前 Go 保证本 init 已执行。
func init() {
	auth.Configure(auth.Options{
		AppName:              "Viderse",
		Purpose:              "để viết kịch bản video",
		LoginCommand:         "viderse login",
		StoreDir:             configDirName,
		ClaudeCodeVersionEnv: "VIDERSE_CLAUDE_CODE_VERSION",
	})
}

// DefaultConfigDir trả thư mục dữ liệu của app: ~/.viderse.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// DefaultConfigPath trả đường dẫn ~/.viderse/config.json.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Config là cấu hình model của Viderse (~/.viderse/config.json), cùng schema
// provider/roles với core/llmconfig + core/roles. Role "writer" mặc định
// gpt-6-luna qua openai-codex khi file chưa có hoặc thiếu cấu hình.
type Config struct {
	Provider  string                              `json:"provider,omitempty"`
	ModelName string                              `json:"model,omitempty"`
	Providers map[string]llmconfig.ProviderConfig `json:"providers,omitempty"`
	Roles     map[string]roles.RoleConfig         `json:"roles,omitempty"`
	// ReasoningEffort là mức suy luận mặc định (off/low/medium/high/xhigh/max);
	// role có thể ghi đè qua roles.<role>.reasoning_effort.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

const (
	// WriterRole là role duy nhất của khung hiện tại.
	WriterRole = "writer"

	defaultProvider = "openai-codex"
	defaultModel    = "gpt-6-luna"
)

// LoadConfig đọc ~/.viderse/config.json; file chưa tồn tại trả cấu hình rỗng
// (mặc định áp lúc resolve). Lỗi parse là lỗi cứng — không lặng lẽ dùng mặc
// định trên file hỏng.
func LoadConfig() (Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("đọc %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s không phải JSON hợp lệ: %w: %w", path, llmconfig.ErrConfig, err)
	}
	return cfg, nil
}

// ResolveRole trả provider/model hiệu lực của một role: role ghi đè → provider
// mặc định của app (openai-codex/gpt-6-luna khi chưa cấu hình). explicit báo
// role có cấu hình riêng không.
func (c Config) ResolveRole(role string) (provider, model string, explicit bool) {
	defProvider, defModel := c.Provider, c.ModelName
	if defProvider == "" {
		defProvider = defaultProvider
	}
	if defModel == "" {
		defModel = defaultModel
	}
	return roles.Resolve(c.Roles, role, defProvider, defModel)
}

// ResolveReasoningEffort trả mức suy luận hiệu lực của role (chuỗi thô, có thể
// rỗng = không ghi đè).
func (c Config) ResolveReasoningEffort(role string) string {
	return roles.ResolveReasoningEffort(c.Roles, role, c.ReasoningEffort)
}

// ModelForRole dựng ChatModel cho một role từ cấu hình đã load: resolve
// provider/model → tra providers.<provider> → core/llmconfig.NewModel.
// Provider thiếu trên file là lỗi cấu hình, không lặng fallback.
func (c Config) ModelForRole(role string) (llmtypes.ChatModel, string, string, error) {
	provider, model, _ := c.ResolveRole(role)
	pc, ok := c.Providers[provider]
	if !ok {
		pc = llmconfig.ProviderConfig{}
	}
	m, err := llmconfig.NewModel(provider, model, pc, nil)
	if err != nil {
		return nil, provider, model, fmt.Errorf("model của vai %s: %w", role, err)
	}
	return m, provider, model, nil
}

// DSNFromEnv trả chuỗi kết nối Postgres của app (VIDERSE_DATABASE_URL) và của
// role migrate (VIDERSE_MIGRATION_DATABASE_URL, trống → dùng chung DSN app).
func DSNFromEnv() (appDSN, migrateDSN string) {
	appDSN = strings.TrimSpace(os.Getenv("VIDERSE_DATABASE_URL"))
	migrateDSN = strings.TrimSpace(os.Getenv("VIDERSE_MIGRATION_DATABASE_URL"))
	if migrateDSN == "" {
		migrateDSN = appDSN
	}
	return appDSN, migrateDSN
}
