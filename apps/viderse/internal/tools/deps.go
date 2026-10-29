// Phụ thuộc dùng chung cho các bộ quản lý công cụ và thứ tự chọn binary.
package tools

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PathStore lưu đường dẫn thực thi do người dùng chọn (Cài đặt › Công cụ);
// dưới mỗi key là một tệp JSON {"path": "..."}. Bản lưu qua bảng settings
// nằm ở gói settings phía trên — gói này chỉ cần Get/Set.
type PathStore interface {
	// Get trả đường dẫn đã lưu dưới key; false khi vắng hoặc bản ghi hỏng.
	Get(key string) (string, bool)
	// Set ghi đường dẫn; path rỗng xoá bản ghi.
	Set(key, path string) error
}

// FilePathStore là PathStore lưu mỗi key một tệp JSON dưới dir (dùng được
// ngay khi gói settings chưa có; quyền thư mục 0700 vì đường dẫn có thể lộ
// bố cục máy).
type FilePathStore struct{ Dir string }

func (s *FilePathStore) file(key string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, key)
	return filepath.Join(s.Dir, clean+".json")
}

func (s *FilePathStore) Get(key string) (string, bool) {
	data, err := os.ReadFile(s.file(key))
	if err != nil {
		return "", false
	}
	var rec struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &rec); err != nil || rec.Path == "" {
		return "", false
	}
	return rec.Path, true
}

func (s *FilePathStore) Set(key, path string) error {
	file := s.file(key)
	if path == "" {
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// Deps là phụ thuộc tiêm vào của mọi bộ quản lý công cụ: không toàn cục,
// test cấp Runner/PathStore giả và thư mục tạm.
type Deps struct {
	// Runner chạy mọi lời gọi binary; bắt buộc.
	Runner Runner
	// Paths là kho đường dẫn đã cấu hình; nil → không có cấu hình người dùng.
	Paths PathStore
	// BaseDir là thư mục dữ liệu của app (~/.viderse): bin/, cache/, python/…
	BaseDir string
	// Client cho mọi HTTP; nil → http.DefaultClient.
	Client *http.Client
	// Home ghi đè $HOME (test); rỗng → os.UserHomeDir().
	Home string
	// PathEnv ghi đè PATH khi tìm binary hệ thống. Khác rỗng thì chỉ PATH đó
	// được dùng (test cô lập bằng thư mục tạm rỗng); rỗng → PATH thật cộng
	// các chỗ cài chuẩn (/opt/homebrew, /usr/local…).
	PathEnv string
	// SystemDirs, khi khác nil, thay danh sách chỗ cài chuẩn theo nền tảng
	// (nil → danh sách thật). Test cô lập bằng []string{}.
	SystemDirs []string
	// Registry, khi khác nil, ghi mọi nhóm process spawn qua đường không đi
	// ExecRunner (Chrome CDP headless) để lần chạy sau dọn nhóm mồ côi.
	Registry *Registry
	// CDPSpawn là seam test: đổi cách Chrome headless được Start (mặc định
	// cmd.Start). Trả pgid của nhóm con (pid leader).
	CDPSpawn func(cmd *exec.Cmd) (pgid int, err error)
}

func (d Deps) home() string {
	if d.Home != "" {
		return d.Home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (d Deps) pathEnv() string {
	if d.PathEnv != "" {
		return d.PathEnv
	}
	return os.Getenv("PATH")
}

// standardDirs trả chỗ cài chuẩn của nền tảng; SystemDirs khác nil thì thay
// hẳn (test cô lập). Không bao giờ trả thư mục rỗng.
func (d Deps) standardDirs() []string {
	if d.SystemDirs != nil {
		return d.SystemDirs
	}
	switch runtime.GOOS {
	case "darwin":
		return []string{"/opt/homebrew/bin", "/usr/local/bin"}
	case "linux":
		return []string{"/usr/bin", "/usr/local/bin"}
	}
	return nil
}

// configuredPath đọc đường dẫn người dùng chọn cho một công cụ; "" khi không có.
func (d Deps) configuredPath(key string) string {
	if d.Paths == nil {
		return ""
	}
	p, _ := d.Paths.Get(key)
	return p
}

// SetConfiguredPath ghi/xoá đường dẫn người dùng chọn.
func (d Deps) SetConfiguredPath(key, path string) error {
	if d.Paths == nil {
		return nil
	}
	return d.Paths.Set(key, path)
}

// ToolSource đánh dấu bản binary được chọn đến từ đâu.
type ToolSource string

const (
	// SourceConfigured là đường dẫn người dùng chọn trong Cài đặt.
	SourceConfigured ToolSource = "configured"
	// SourceSystem là binary phát hiện trên máy (PATH + chỗ cài chuẩn).
	SourceSystem ToolSource = "system"
	// SourceManaged là bản app tự cài dưới <data>/bin.
	SourceManaged ToolSource = "managed"
)

// candidate là một đường dẫn ứng viên cùng nguồn của nó.
type candidate struct {
	path   string
	source ToolSource
}

// resolutionOrder xếp các ứng viên theo thứ tự chọn của app: đường dẫn đã
// cấu hình → binary hệ thống → bản app tự quản. Ứng viên rỗng bị bỏ.
func resolutionOrder(configured string, system []string, managed string) []candidate {
	out := make([]candidate, 0, len(system)+2)
	if configured != "" {
		out = append(out, candidate{configured, SourceConfigured})
	}
	for _, p := range system {
		if p != "" {
			out = append(out, candidate{p, SourceSystem})
		}
	}
	if managed != "" {
		out = append(out, candidate{managed, SourceManaged})
	}
	return out
}

// Tools là bó các bộ quản lý công cụ; bootstrap dựng một lần qua New.
type Tools struct {
	Ffmpeg *Ffmpeg
	Uv     *Uv
	YtDlp  *YtDlp
	Codex  *Codex
	Chrome *Chrome
}

// New dựng mọi bộ quản lý trên cùng một bộ phụ thuộc.
func New(d Deps) *Tools {
	return &Tools{
		Ffmpeg: &Ffmpeg{d: d},
		Uv:     &Uv{d: d},
		YtDlp: &YtDlp{d: d, verified: NewFileVerificationCache[string](
			func(v string) bool { return v != "" }, 0)},
		Codex: &Codex{d: d, verifiedIsolation: NewFileVerificationCache[CodexIsolation](
			func(is CodexIsolation) bool { return is.State == IsolationVerified }, 0)},
		Chrome: &Chrome{d: d,
			verifiedProbe: NewFileVerificationCache[chromeProbe](
				func(p chromeProbe) bool { return p.Caps != nil && chromeCapsOk(*p.Caps) }, 0),
			verifiedExport: NewFileVerificationCache[exportProbe](
				func(p exportProbe) bool { return p.GPU && p.Encoder }, 0),
		},
	}
}
