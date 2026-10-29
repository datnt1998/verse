package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
	"github.com/datnt1998/verse/core/llmconfig"
	"github.com/datnt1998/verse/core/roles"
)

const configDirName = ".ainovel"

// DefaultConfigPath 返回全局配置文件路径 ~/.ainovel/config.json。
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName, "config.json")
}

// DefaultConfigDir 返回 ~/.ainovel 目录路径；取不到家目录时返回空字符串。
// 仅用于读/写不强制存在的文件（如模型缓存），不会自动创建目录。
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, configDirName)
}

// configDir 返回 ~/.ainovel 目录路径，不存在时创建。
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục người dùng: %w", err)
	}
	dir := filepath.Join(home, configDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục cấu hình: %w", err)
	}
	return dir, nil
}

// projectConfigPath 返回项目/故事级配置文件的相对路径 ./.ainovel/config.json。
// 近层 dotdir 镜像全局 ~/.ainovel/，复用同一个 configDirName；相对目录解析。
func projectConfigPath() string {
	return filepath.Join(configDirName, "config.json")
}

// EffectiveConfigPath 返回 TUI 改动（/config、/model）应写回的配置文件：
// 沿 故事目录(cwd) → 最近祖先项目目录 → 全局 的继承链取**最近的已存在**文件——
// 与读取时近层覆盖远层的方向一致，保证"改当前生效的那份"、改完立刻生效。
// 仅编辑已存在的近层配置，不会凭空创建（创建项目覆盖是用户主动放文件的动作）。
func EffectiveConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return DefaultConfigPath()
	}
	if p := filepath.Join(cwd, projectConfigPath()); fileExists(p) {
		return p
	}
	if projectDir := library.FindProjectDir(cwd); projectDir != "" {
		if p := filepath.Join(projectDir, projectConfigPath()); fileExists(p) {
			return p
		}
	}
	return DefaultConfigPath()
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// ConfigLevel 记录继承链上的一层：来源文件路径与该文件自身的配置。
// Scope 取 "global" | "project" | "story"；Exists=false 表示该层没有配置文件。
type ConfigLevel struct {
	Scope  string
	Path   string
	Exists bool
	Config Config
}

// LoadConfig 按优先级加载并合并配置（GAP-10 继承链）：
//  1. ~/.ainovel/config.json（全局）
//  2. <项目>/.ainovel/config.json（cwd 最近的含 project.json 的祖先目录）
//  3. <故事>/.ainovel/config.json（cwd 自身）
func LoadConfig() (Config, error) { return LoadConfigAt(".") }

// LoadConfigAt 以 dir 为故事目录合并配置链，不切换进程工作目录。
// dir 是书目录（TUI）或 worker 的 cwd；项目层由 library.FindProjectDir 向上定位。
func LoadConfigAt(dir string) (Config, error) {
	_, cfg, err := LoadConfigChain(dir)
	return cfg, err
}

// LoadConfigChain 返回继承链各层与合并结果。链上的层固定为 global → project
// → story；project/story 目录相同（书目录即项目目录）时只保留 project 一层。
func LoadConfigChain(dir string) ([]ConfigLevel, Config, error) {
	storyAbs, err := filepath.Abs(dir)
	if err != nil {
		storyAbs = dir
	}
	projectDir := library.FindProjectDir(storyAbs)

	var levels []ConfigLevel
	var cfg Config

	// 1. 全局配置。它是最低优先级基底，坏文件降级为告警而非阻断——可被近层覆盖；
	//    硬失败会把"坏全局 + 有效近层配置"的用户挡在门外。
	if p := DefaultConfigPath(); p != "" {
		lvl := ConfigLevel{Scope: "global", Path: p}
		global, found, gerr := loadOptionalJSON(p)
		switch {
		case gerr != nil:
			slog.Warn("phân tích cấu hình toàn cục thất bại, đã bỏ qua (có thể bị cấp dự án/truyện ghi đè)", "module", "config", "path", p, "err", gerr)
		case found:
			lvl.Exists, lvl.Config = true, global
			cfg = global
		}
		levels = append(levels, lvl)
	}

	// 2/3. 项目层与故事层。坏文件 fail loud：用户主动放的配置，静默吞掉会让
	//    "配了不生效"无从排查（issue #37）。同一路径只入链一次。
	add := func(scope, dir string) error {
		if dir == "" {
			return nil
		}
		path := filepath.Join(dir, projectConfigPath())
		for _, l := range levels {
			if l.Path == path {
				return nil
			}
		}
		lvl := ConfigLevel{Scope: scope, Path: path}
		c, found, lerr := loadOptionalJSON(path)
		if lerr != nil {
			return fmt.Errorf("phân tích cấu hình cấp %s %s thất bại (kiểm tra cú pháp JSON): %w", scopeLabel(scope), path, lerr)
		}
		if found {
			lvl.Exists, lvl.Config = true, c
			cfg = MergeConfig(cfg, c)
		}
		levels = append(levels, lvl)
		return nil
	}
	if projectDir != "" {
		if err := add("project", projectDir); err != nil {
			return levels, cfg, err
		}
	}
	// 故事层总在 cwd 本身；目录即项目目录时路径相同，add 去重只留 project 层。
	if err := add("story", storyAbs); err != nil {
		return levels, cfg, err
	}
	return levels, cfg, nil
}

func scopeLabel(scope string) string {
	switch scope {
	case "project":
		return "dự án"
	case "story":
		return "truyện"
	}
	return scope
}

// loadOptionalJSON 读取一个可选的配置文件：
//   - 文件不存在 → (zero, false, nil)，由调用方决定用默认/上层值
//   - 文件存在但解析失败 → 返回错误（不再静默吞掉——否则用户的配置"配了不生效"
//     却无从排查，正是 issue #37 的根因）
func loadOptionalJSON(path string) (Config, bool, error) {
	cfg, err := loadJSONFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	return cfg, true, nil
}

// LoadConfigFile 读取单个 JSON 配置文件，支持 // 行注释。
// 不做任何合并，仅返回该文件自身的配置。文件不存在时返回错误。
func LoadConfigFile(path string) (Config, error) {
	return loadJSONFile(path)
}

// loadJSONFile 读取 JSON 配置文件，支持 // 行注释。
// 文件不存在时返回错误（由调用方决定是否忽略）。
func loadJSONFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cleaned := stripJSONComments(data)
	var cfg Config
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return Config{}, fmt.Errorf("tệp %s không hợp lệ: %s", path, utils.DescribeJSONError(err))
	}
	warnLegacyLanguage(cleaned)
	return cfg, nil
}

// warnLegacyLanguage 对旧配置里的 language 键给一次进程级提示：zh 不再支持，
// 统一按 tiếng Việt 处理；键本身静默忽略，不再写回。
func warnLegacyLanguage(data []byte) {
	var legacy struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(legacy.Language)) {
	case "zh", "cn", "chinese":
		warnLegacyLanguageOnce.Do(func() {
			slog.Warn("ngôn ngữ zh không còn được hỗ trợ, dùng tiếng Việt", "module", "bootstrap")
		})
	}
}

var warnLegacyLanguageOnce sync.Once

// ReadConfigJSON 读取单个配置文件为原始 JSON 对象（支持 // 注释），不做合并、
// 不解码为 Config。网关用它计算每个键的继承来源，并在 PUT 时回插保密的 api_key。
// 文件不存在返回 (nil, nil)；解析失败返回错误。
func ReadConfigJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(stripJSONComments(data), &m); err != nil {
		return nil, fmt.Errorf("tệp %s không hợp lệ: %s", path, utils.DescribeJSONError(err))
	}
	return m, nil
}

// MergeConfig 将 overlay 合并到 base 上。非零值字段覆盖，map 按 key 合并。
// 网关也用它把"项目/故事层不含的键"补回全局值后返回给前端。
func MergeConfig(base, overlay Config) Config {
	if overlay.Provider != "" {
		base.Provider = overlay.Provider
	}
	if overlay.ModelName != "" {
		base.ModelName = overlay.ModelName
	}
	if overlay.ReasoningEffort != "" {
		base.ReasoningEffort = overlay.ReasoningEffort
	}
	if overlay.Style != "" {
		base.Style = overlay.Style
	}
	if overlay.Language != "" {
		base.Language = overlay.Language
	}
	if overlay.ContextWindow > 0 {
		base.ContextWindow = overlay.ContextWindow
	}
	if overlay.AdvanceMode != "" {
		base.AdvanceMode = overlay.AdvanceMode
	}
	// ask_user: con trỏ để lớp gần tắt được giá trị lớp ngoài (khác "không khai báo").
	if overlay.AskUser != nil {
		value := *overlay.AskUser
		base.AskUser = &value
	}

	// Providers: overlay 的 key 覆盖 base 同名 key
	if len(overlay.Providers) > 0 {
		if base.Providers == nil {
			base.Providers = make(map[string]llmconfig.ProviderConfig)
		}
		for k, v := range overlay.Providers {
			existing := base.Providers[k]
			if v.Type != "" {
				existing.Type = v.Type
			}
			if v.API != "" {
				existing.API = v.API
			}
			if v.APIKey != "" {
				existing.APIKey = v.APIKey
			}
			if v.BaseURL != "" {
				existing.BaseURL = v.BaseURL
			}
			if len(v.Models) > 0 {
				existing.Models = append([]llmconfig.ModelConfig(nil), v.Models...)
			}
			if len(v.ExtraBody) > 0 {
				existing.ExtraBody = cloneMap(v.ExtraBody)
			}
			if len(v.Extra) > 0 {
				existing.Extra = cloneMap(v.Extra)
			}
			base.Providers[k] = existing
		}
	}

	// Roles: overlay 的 key 覆盖 base 同名 key
	if len(overlay.Roles) > 0 {
		if base.Roles == nil {
			base.Roles = make(map[string]roles.RoleConfig)
		}
		for k, v := range overlay.Roles {
			existing := base.Roles[k]
			if v.Provider != "" {
				existing.Provider = v.Provider
			}
			if v.Model != "" {
				existing.Model = v.Model
			}
			if len(v.Fallbacks) > 0 {
				existing.Fallbacks = append([]roles.ModelRef(nil), v.Fallbacks...)
			}
			if v.ReasoningEffort != "" {
				existing.ReasoningEffort = v.ReasoningEffort
			}
			base.Roles[k] = existing
		}
	}

	// Budget / Notify：整块覆盖（项目级预算/告警是独立政策声明，不与全局逐字段拼接）
	if overlay.Budget != (BudgetConfig{}) {
		base.Budget = overlay.Budget
	}
	if overlay.Notify.Enabled != nil || overlay.Notify.Command != "" || len(overlay.Notify.Events) > 0 {
		base.Notify = overlay.Notify
	}

	// TTS: ghi đè từng trường — lớp gần chỉ cần đổi một khoá (vd quality) mà vẫn
	// thừa hưởng engine/device của lớp ngoài.
	if overlay.TTS != (TTSConfig{}) {
		if overlay.TTS.Engine != "" {
			base.TTS.Engine = overlay.TTS.Engine
		}
		if overlay.TTS.Model != "" {
			base.TTS.Model = overlay.TTS.Model
		}
		if overlay.TTS.Device != "" {
			base.TTS.Device = overlay.TTS.Device
		}
		if overlay.TTS.Quality != "" {
			base.TTS.Quality = overlay.TTS.Quality
		}
		if overlay.TTS.Endpoint != "" {
			base.TTS.Endpoint = overlay.TTS.Endpoint
		}
		if overlay.TTS.APIKey != "" {
			base.TTS.APIKey = overlay.TTS.APIKey
		}
	}

	return base
}

func cloneMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// CloneConfig 深拷贝配置中会在运行时修改的 map/slice，避免候选配置污染当前配置。
func CloneConfig(cfg Config) Config {
	clone := cfg
	clone.Providers = make(map[string]llmconfig.ProviderConfig, len(cfg.Providers))
	for name, pc := range cfg.Providers {
		pc.Models = append([]llmconfig.ModelConfig(nil), pc.Models...)
		pc.Extra = cloneMap(pc.Extra)
		pc.ExtraBody = cloneMap(pc.ExtraBody)
		clone.Providers[name] = pc
	}
	clone.Roles = make(map[string]roles.RoleConfig, len(cfg.Roles))
	for role, rc := range cfg.Roles {
		rc.Fallbacks = append([]roles.ModelRef(nil), rc.Fallbacks...)
		clone.Roles[role] = rc
	}
	clone.Notify.Events = append([]string(nil), cfg.Notify.Events...)
	return clone
}

// SaveProviderConfig 补丁式更新目标配置层里单个 provider 的凭证与模型库。
// 只动 providers 段，绝不触碰顶层 provider/model 选择——“当前用哪个”归 /model。
// 目标不存在时创建最小配置；目标损坏时拒绝覆盖。
func SaveProviderConfig(path string, provider string, pc llmconfig.ProviderConfig) error {
	target, found, err := loadOptionalJSON(path)
	if err != nil {
		return err
	}
	if !found {
		target = Config{}
	}
	if target.Providers == nil {
		target.Providers = make(map[string]llmconfig.ProviderConfig)
	}
	target.Providers[provider] = pc
	return SaveConfig(path, target)
}

// stripJSONComments 去除 JSON 中的 // 行注释，跟踪引号状态避免误删字符串内容。
func stripJSONComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		b := data[i]

		if escaped {
			out = append(out, b)
			escaped = false
			continue
		}

		if inString {
			out = append(out, b)
			if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}

		// 不在字符串内
		if b == '"' {
			inString = true
			out = append(out, b)
			continue
		}

		// 检测 // 注释
		if b == '/' && i+1 < len(data) && data[i+1] == '/' {
			// 跳到行尾
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
			continue
		}

		out = append(out, b)
	}

	return out
}

// WriteStartupError 把启动期致命错误追加写入 ~/.ainovel/last-error.log，并返回
// 该文件路径（best-effort，失败时返回空字符串）。双击启动时控制台窗口会随进程
// 退出立即关闭、错误一闪而过，落盘是这类用户事后追溯的唯一途径。
func WriteStartupError(msg string) string {
	dir := DefaultConfigDir()
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(dir, "last-error.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg); err != nil {
		return ""
	}
	return path
}

// SaveConfig 将配置写入指定路径（JSON 格式，缩进美化）。
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}
