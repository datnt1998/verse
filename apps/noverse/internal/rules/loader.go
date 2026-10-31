package rules

import (
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// LoadOptions 枚举 rules 文件来源目录，供 RawFileSources 扫描归一化。
//
// 目录不存在不算错误，扫描时静默跳过。
type LoadOptions struct {
	// HomeRulesDir locates the system lesson catalog; global .md files are ignored.
	HomeRulesDir string

	// ProjectRulesDir 是 <项目>/.ainovel/rules/ 目录（项目 = 书目录最近的含 project.json
	// 的祖先目录；镜像全局，同样扫描其下所有顶层 .md）。空表示跳过。
	ProjectRulesDir string

	// StoryRulesDir 是 <书目录>/.ainovel/rules/ 目录，优先级最高。空表示跳过。
	// 书目录即项目目录时应留空（避免同名文件被算成两层覆盖）。
	StoryRulesDir string
	// Sources holds scoped documents supplied by a non-filesystem backend.
	// Only project and story sources participate in the nearer-wins merge.
	Sources []RawSource
}

// ainovelDirName 是 ainovel 在 user / project 两级共用的 dotdir 名。
// 全局 ~/.ainovel/rules/ 与项目 ./.ainovel/rules/ 由此对称。
const ainovelDirName = ".ainovel"

// DefaultProjectRulesDir 拼出 ./.ainovel/rules/ 的绝对路径（基于给定项目目录）。
// 调用方传入项目根，避免在 loader 内部依赖 cwd；镜像 DefaultHomeRulesDir。
func DefaultProjectRulesDir(projectDir string) string {
	if projectDir == "" {
		return ""
	}
	return filepath.Join(projectDir, ainovelDirName, "rules")
}

// DefaultHomeRulesDir 拼出 ~/.ainovel/rules/ 目录的绝对路径。
// home 解析失败返回空串（调用方据此跳过该来源）。
func DefaultHomeRulesDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ainovelDirName, "rules")
}

// homeRulesReadme 是首次引导时写入 ~/.ainovel/rules/README.txt 的说明。
// 刻意用 .txt 后缀而非 .md——扫描只认 .md，这份说明不会被当成规则归一化。
const homeRulesReadme = `Quy tắc viết chỉ còn hai cấp: dự án và truyện.

Các tệp .md toàn cục cũ được chuyển vào từng dự án một lần khi mở thư viện.
Thư mục này không còn được nạp làm quy tắc.
Hãy đặt yêu cầu cần chuẩn hoá trong <dự án>/.ainovel/rules/*.md
hoặc <truyện>/.ainovel/rules/*.md; quy tắc truyện được ưu tiên.
`

// EnsureHomeRulesDir 尽力创建 ~/.ainovel/rules/ 目录并写入 README.txt 引导，
// 让用户发现这个全局偏好扩展点、知道怎么写。
// nice-to-have，非关键路径：home 解析失败或写入出错都静默吞掉，绝不阻断启动。
func EnsureHomeRulesDir() {
	if dir := DefaultHomeRulesDir(); dir != "" {
		_ = ensureRulesDirAt(dir)
	}
}

// ensureRulesDirAt 创建目录并把 README.txt 写成当前引导模板，是 EnsureHomeRulesDir 的可测内核。
// README.txt 是系统生成的引导文件（用户偏好写在 *.md，它不被扫描加载），每次都覆盖为
// 最新模板——不保留旧内容，也就不需要任何版本兼容逻辑。
func ensureRulesDirAt(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "README.txt"), []byte(homeRulesReadme), 0o644)
}

// DefaultOptions 根据当前工作目录构造常用 LoadOptions。
//
// 适合 Host 启动时调用一次，让用户规则服务复用同一份来源配置。
// 解析 cwd 失败时 ProjectRulesDir/StoryRulesDir 留空（扫描会跳过该来源）。
//
// Rule inheritance is project → story. The retired home directory is not read.
func DefaultOptions() LoadOptions {
	cwd, _ := os.Getwd()
	return OptionsForDir(cwd)
}

// OptionsForDir constructs project/story sources for workers and gateways.
func OptionsForDir(dir string) LoadOptions {
	var opts LoadOptions
	opts.HomeRulesDir = DefaultHomeRulesDir()
	if dir == "" {
		return opts
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	projectDir := library.FindProjectDir(abs)
	opts.ProjectRulesDir = DefaultProjectRulesDir(projectDir)
	// 书目录即项目目录时只有一层近层目录，避免同一目录扫描两次。
	if projectDir == "" || projectDir != abs {
		opts.StoryRulesDir = DefaultProjectRulesDir(abs)
	}
	return opts
}
