// Package library 描述 Web 书库的目录约定：书库根（serve --root）→ 项目（含 project.json）→ 故事（项目下 dirs.stories 的子目录）。
// 这里只放纯数据与文件约定，不依赖 host / engine；gateway、host（自动导出）、bootstrap（配置继承）共同使用。
package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	ConfigVersion   = 1
	ProjectFileName = "project.json"
	StoryFileName   = "story.json"
	// LegacyProjectID 是把书库根下旧式书目录（直接含 output/）聚合成的匿名项目。
	LegacyProjectID = "_"
	// RootEnv 由 gateway 传给 worker，worker 据此读取书库配置（自动导出等）。
	RootEnv = "AINOVEL_LIBRARY_ROOT"
)

type Dirs struct {
	Ideas   string `json:"ideas"`
	Assets  string `json:"assets"`
	Stories string `json:"stories"`
	Exports string `json:"exports"`
	// Audio là thư mục âm thanh trong từng truyện (không phải cấp dự án).
	Audio string `json:"audio"`
}

type AutoExport struct {
	OnVolume  bool     `json:"onVolume"`
	OnBook    bool     `json:"onBook"`
	OnChapter bool     `json:"onChapter"`
	Formats   []string `json:"formats"`
	Pattern   string   `json:"pattern"`
	OnExists  string   `json:"onExists"`
}

// Config 对应 <root>/.ainovel/library.json。
type Config struct {
	Version    int        `json:"version"`
	Dirs       Dirs       `json:"dirs"`
	SlugWords  int        `json:"slugWords"`
	AutoExport AutoExport `json:"autoExport"`
}

func DefaultConfig() Config {
	return Config{
		Version:   ConfigVersion,
		Dirs:      Dirs{Ideas: "y-tuong", Assets: "tai-san", Stories: "kich_ban", Exports: "xuat-ban", Audio: "audio"},
		SlugWords: 5,
		AutoExport: AutoExport{
			OnVolume: true, OnBook: true, Formats: []string{"txt", "epub"},
			Pattern: "tap-{tap}/{truyen}.tap-{tap}.{ext}", OnExists: "replace",
		},
	}
}

// ProjectFile 对应 <project>/project.json。旧文件的 language 键已废弃：
// 读取时静默忽略，写入不再产出。
type ProjectFile struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Notes       string     `json:"notes"`
	CreatedAt   *time.Time `json:"createdAt"`
	// UniverseName là tên vũ trụ chung tuỳ chọn (GAP-44, câu 3). Trống thì tầng
	// API dùng tên dự án thay thế; tệp giữ nguyên trống.
	UniverseName string `json:"universeName,omitempty"`
}

// StoryFile 对应 <story>/story.json。Status: idea | writing | paused | done | published。
// Title 是显示名；TitleRenamed=false 时网关把它与 meta/book.json 的书名自动对齐
// （不能反写：book.json 才是作品信息的唯一事实源）。创建不填 name 时 Title 为空，
// 界面显示 "Truyện chưa đặt tên"（tempName）。
// ShortTitle 是 Gói AF 的 {Tên ngắn}（≤32 ký tự）：下载文件名/封面文字用；
// 缺省时后端从 Title 按词边界截断，不回写。
type StoryFile struct {
	Title      string `json:"title"`
	ShortTitle string `json:"shortTitle,omitempty"`
	// TitleRenamed đánh dấu người dùng đã đổi "Tên truyện" qua PUT /meta:
	// đồng bộ tự động theo meta/book.json chỉ khi cờ này false. Bản ghi cũ
	// không có khoá → false, tức tiêu đề lấy từ yêu cầu lúc tạo được coi là tạm.
	TitleRenamed bool       `json:"titleRenamed,omitempty"`
	Status       string     `json:"status"`
	Idea         string     `json:"idea"`
	Tags         []string   `json:"tags"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    *time.Time `json:"createdAt"`
}

// Story 是已解析的故事位置；ID = "<project>/<story>"。
type Story struct {
	Project    string
	Story      string
	ID         string
	Dir        string // 故事目录绝对路径（worker cwd）
	ProjectDir string // 项目目录绝对路径；匿名项目为书库根
	Root       string // 书库根绝对路径
	// UpdatedAt is the row timestamp on hosted backends (nil for file mode,
	// where the directory mtime is the source and callers stat it).
	UpdatedAt *time.Time
}

func ConfigPath(root string) string { return filepath.Join(root, ".ainovel", "library.json") }

// LoadConfig 读取书库配置；文件不存在返回默认值，缺省字段用默认值补齐。
func LoadConfig(root string) (Config, error) {
	cfg := DefaultConfig()
	raw, err := os.ReadFile(ConfigPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("library.json hỏng: %w", err)
	}
	def := DefaultConfig()
	if cfg.Dirs.Ideas == "" {
		cfg.Dirs.Ideas = def.Dirs.Ideas
	}
	if cfg.Dirs.Assets == "" {
		cfg.Dirs.Assets = def.Dirs.Assets
	}
	if cfg.Dirs.Stories == "" {
		cfg.Dirs.Stories = def.Dirs.Stories
	}
	if cfg.Dirs.Exports == "" {
		cfg.Dirs.Exports = def.Dirs.Exports
	}
	if cfg.Dirs.Audio == "" {
		cfg.Dirs.Audio = def.Dirs.Audio
	}
	if cfg.SlugWords <= 0 {
		cfg.SlugWords = def.SlugWords
	}
	if cfg.AutoExport.Pattern == "" {
		cfg.AutoExport.Pattern = def.AutoExport.Pattern
	}
	if cfg.AutoExport.OnExists == "" {
		cfg.AutoExport.OnExists = def.AutoExport.OnExists
	}
	if cfg.AutoExport.Formats == nil {
		cfg.AutoExport.Formats = def.AutoExport.Formats
	}
	cfg.Version = ConfigVersion
	return cfg, nil
}

var dirNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

var reservedNames = map[string]bool{"output": true, ".ainovel": true, ProjectFileName: true, StoryFileName: true}

// ValidDirName 校验新建的项目 / 故事 / dirs.* 目录名。
func ValidDirName(name string) bool {
	return len(name) <= 100 && dirNamePattern.MatchString(name) && !reservedNames[name]
}

// Slugify 去掉越南语声调，转小写，取前 words 个词用 "-" 连接；结果为空返回 "truyen-moi"。
func Slugify(text string, words int) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(text) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r == 'đ' || r == 'Đ':
			b.WriteRune('d')
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	parts := strings.Fields(b.String())
	if words > 0 && len(parts) > words {
		parts = parts[:words]
	}
	if len(parts) == 0 {
		return "truyen-moi"
	}
	return strings.Join(parts, "-")
}

// FindProjectDir 从 start 向上查找最近的含 project.json 的目录（不越过用户主目录与文件系统根）。找不到返回 ""。
func FindProjectDir(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	for {
		if st, err := os.Stat(filepath.Join(dir, ProjectFileName)); err == nil && !st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || dir == home {
			return ""
		}
		dir = parent
	}
}

// SiblingStories chỉ lấy các truyện khác trong cùng dự án của thư mục engine.
// Không có dự án thì không có nguồn đối chiếu; lỗi đọc không bị coi là danh sách rỗng.
func SiblingStories(engineDir string) ([]StoryDir, error) {
	project := FindProjectDir(engineDir)
	if project == "" {
		return nil, nil
	}
	lib, err := Open(filepath.Dir(project))
	if err != nil {
		return nil, err
	}
	stories, err := lib.Stories(filepath.Base(project))
	if err != nil {
		return nil, err
	}
	current, err := filepath.Abs(engineDir)
	if err != nil {
		return nil, err
	}
	var others []StoryDir
	for _, story := range stories {
		rel, err := filepath.Rel(story.Dir, current)
		if err != nil {
			return nil, err
		}
		if rel != "." && (rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			others = append(others, story)
		}
	}
	return others, nil
}
