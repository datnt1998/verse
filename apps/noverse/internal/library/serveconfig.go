package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidRoot 表示 serve 的 thư viện根 không dùng được（không tồn tại, không phải
// thư mục, hoặc thiếu quyền đọc/ghi）。gateway 映射为 400。
var ErrInvalidRoot = errors.New("thư mục thư viện không hợp lệ")

// ServeConfig 对应 ~/.ainovel/serve.json：只在 web serve 里使用、不随书库走的设置。
// root：PUT /api/library 换 thư viện lúc chạy；lan：PUT /api/system/network 开关局域网监听。
type ServeConfig struct {
	Root string `json:"root,omitempty"`
	LAN  bool   `json:"lan"`
}

// ServeConfigPath 返回 ~/.ainovel/serve.json 的绝对路径。
func ServeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, ".ainovel", "serve.json"), nil
}

// LoadServeConfig 读取 serve.json；文件不存在返回零值（不报错）。
func LoadServeConfig() (ServeConfig, error) {
	var cfg ServeConfig
	path, err := ServeConfigPath()
	if err != nil {
		return cfg, err
	}
	found, err := readJSONFile(path, &cfg)
	if err != nil {
		return ServeConfig{}, fmt.Errorf("serve.json hỏng: %w", err)
	}
	if !found {
		return ServeConfig{}, nil
	}
	return cfg, nil
}

// LoadServeRoot 返回已保存的书库根；未配置返回 ""。
func LoadServeRoot() (string, error) {
	cfg, err := LoadServeConfig()
	if err != nil {
		return "", err
	}
	return cfg.Root, nil
}

// SaveServeRoot 把书库根写入 ~/.ainovel/serve.json（原子替换），保留其它字段。
func SaveServeRoot(root string) error {
	path, err := ServeConfigPath()
	if err != nil {
		return err
	}
	cfg, _ := LoadServeConfig()
	cfg.Root = root
	return writeJSONFile(path, cfg)
}

// SaveServeLAN 把局域网开关写入 ~/.ainovel/serve.json（原子替换），保留其它字段。
func SaveServeLAN(lan bool) error {
	path, err := ServeConfigPath()
	if err != nil {
		return err
	}
	cfg, _ := LoadServeConfig()
	cfg.LAN = lan
	return writeJSONFile(path, cfg)
}

// LegacyDefaultDirName là thư mục cũ ~/Truyen: chỉ còn được dùng tiếp nếu đã tồn tại,
// cài mới không còn tự tạo hay mặc định vào đó (09 §10 · GAP-34).
const LegacyDefaultDirName = "Truyen"

// ResolveServeRoot chọn thư viện lúc khởi động serve: cờ --root thắng, rồi serve.json,
// rồi ~/Truyen nếu đã có. Không có gì → "" (chưa chọn nơi lưu; bước Thư viện của web sẽ chọn).
// Không bao giờ dùng thư mục hiện tại.
func ResolveServeRoot(flagRoot string) (string, error) {
	pick := strings.TrimSpace(flagRoot)
	if pick == "" {
		if saved, err := LoadServeRoot(); err == nil && saved != "" {
			pick = saved
		}
	}
	if pick == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", nil
		}
		legacy := filepath.Join(home, LegacyDefaultDirName)
		if st, err := os.Stat(legacy); err != nil || !st.IsDir() {
			return "", nil
		}
		pick = legacy
	}
	abs, err := filepath.Abs(pick)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("thư mục gốc không hợp lệ: %s", abs)
	}
	return abs, nil
}

// CheckRoot 把 path 规范化为绝对路径并验证它可作书库根：
// 存在、是目录、可读、可写（用临时目录探测，不写入任何持久文件）。
func CheckRoot(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("%w: thiếu đường dẫn", ErrInvalidRoot)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRoot, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%w: không tìm thấy %s", ErrInvalidRoot, abs)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%w: %s không phải thư mục", ErrInvalidRoot, abs)
	}
	if _, err := os.ReadDir(abs); err != nil {
		return "", fmt.Errorf("%w: không đọc được %s", ErrInvalidRoot, abs)
	}
	probe, err := os.MkdirTemp(abs, ".ainovel-probe-")
	if err != nil {
		return "", fmt.Errorf("%w: không ghi được vào %s", ErrInvalidRoot, abs)
	}
	_ = os.RemoveAll(probe)
	return abs, nil
}
