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
	// HomeRulesDir 是 ~/.ainovel/rules/ 目录；扫描其下所有顶层 .md（文件名字典序合并）。空表示跳过。
	HomeRulesDir string

	// ProjectRulesDir 是 <项目>/.ainovel/rules/ 目录（项目 = 书目录最近的含 project.json
	// 的祖先目录；镜像全局，同样扫描其下所有顶层 .md）。空表示跳过。
	ProjectRulesDir string

	// StoryRulesDir 是 <书目录>/.ainovel/rules/ 目录，优先级最高。空表示跳过。
	// 书目录即项目目录时应留空（避免同名文件被算成两层覆盖）。
	StoryRulesDir string
	// Sources holds scoped documents supplied by a non-filesystem backend.
	// They follow the home layer and participate in the same nearer-wins merge.
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
const homeRulesReadme = `Đây là nơi đặt sở thích viết toàn cục, áp dụng cho mọi truyện.

Tạo một tệp .md (ví dụ my-style.md), viết yêu cầu bằng lời lẽ bình thường —
không cần định dạng nào, không cần YAML:

    # Nhân vật
    - Đừng viết nhân chính Lâm Trần thành thánh mẫu, lạnh ngoài nóng trong là được
    # Văn phong
    - Dùng cảm giác cơ thể (đốt ngón tay trắng bệch) thay cho nhãn cảm xúc (căng thẳng)
    - Đối thoại đừng quá sách vở, mỗi chương khoảng 3000 chữ
    - Đừng để lọt những câu sáo kiểu "theo một nghĩa nào đó"

Viết xong không cần bận tâm định dạng: hệ thống dùng model chuẩn hoá các yêu cầu ngôn ngữ
tự nhiên này thành ràng buộc có cấu trúc (phạm vi số chữ, từ cấm, ngưỡng lặp từ…),
khi viết tự động tuân theo, khi nộp chương tự động tự kiểm.

Nhiều tệp .md được gộp theo thứ tự từ điển tên tệp; tệp ẩn bắt đầu bằng dấu chấm và
tệp không phải .md đều bị bỏ qua (nên README.txt này không bị coi là luật).

Các câu sáo AI thường gặp và ngưỡng lặp từ cơ học đã có sẵn, dùng được ngay,
không viết gì cũng không sao.

Thứ tự ưu tiên khi nạp (cao → thấp): ./.ainovel/rules/*.md (truyện này) > ~/.ainovel/rules/*.md (thư mục này) > mặc định tích hợp
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
// 继承链（GAP-10）：~/.ainovel/rules → <项目>/.ainovel/rules → <书目录>/.ainovel/rules，
// 同名文件更近的层覆盖。项目 = cwd 最近的含 project.json 的祖先目录。
//
// 路径语义：近层目录绑定 **当前工作目录（cwd）** 而非 outputDir。
// 用户 cd 到不同目录启动写不同的书，./.ainovel/rules/ 自然跟着 cwd 走；如需跨书共享，
// 放项目层或 ~/.ainovel/rules/ 全局目录即可（其下所有 .md 都会被加载）。
func DefaultOptions() LoadOptions {
	cwd, _ := os.Getwd()
	return OptionsForDir(cwd)
}

// OptionsForDir 以 dir 为书目录构造三级继承来源，供 cwd 之外的场景（worker、测试、网关）。
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
