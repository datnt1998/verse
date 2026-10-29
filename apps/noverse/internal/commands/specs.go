// Package commands 是 TUI、web 与 host 共用的斜杠命令注册表：
// 命令声明（Spec/Param）与参数分词（G12）、语法校验（G13 的 "Cách dùng: …"）。
package commands

import "strings"

// Kind 描述一个参数的取值方式，与 GET /api/commands 的 kind 字段一致。
type Kind string

const (
	KindPath Kind = "path"
	KindEnum Kind = "enum"
	KindFlag Kind = "flag"
	KindNum  Kind = "num"
	KindText Kind = "text"
)

// Option 是枚举参数的一个候选值；Note 给 UI 做说明。
type Option struct {
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
}

// Param 描述一个命令参数。位置参数（path/enum/num/text）按声明顺序绑定；
// Prefix 非空表示 name=value 形式（如 from=、--story=）；Flag 非空表示开关。
type Param struct {
	Key      string   `json:"key"`
	Token    string   `json:"token"`
	Kind     Kind     `json:"kind"`
	Required bool     `json:"required"`
	Prefix   string   `json:"prefix,omitempty"`
	Flag     string   `json:"flag,omitempty"`
	Options  []Option `json:"options,omitempty"`
}

// Spec 是一个斜杠命令的声明。Name 不带斜杠，Usage 带斜杠。
// Only 为 ""、"welcome"（只在欢迎页）或 "running"（只在写作中）。
type Spec struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases"`
	Usage       string   `json:"usage"`
	Description string   `json:"-"`
	Group       string   `json:"group"`
	AutoExecute bool     `json:"autoExecute"`
	NeedsIdle   bool     `json:"needsIdle"`
	Only        string   `json:"only,omitempty"`
	Params      []Param  `json:"params"`
}

// 命令分组键（与 web i18n 的分组一一对应）。
const (
	GroupBegin = "begin"
	GroupWrite = "write"
	GroupStyle = "style"
	GroupEnd   = "end"
	GroupSys   = "sys"
)

// Only 的取值。
const (
	OnlyWelcome = "welcome"
	OnlyRunning = "running"
)

var roleOptions = []Option{
	{Value: "architect", Note: "Kiến trúc sư — lên dàn ý, quy hoạch tập"},
	{Value: "writer", Note: "Người viết — viết từng chương"},
	{Value: "editor", Note: "Biên tập viên — soát và chấm chương"},
}

var reviewOptions = []Option{
	{Value: "on", Note: "Dừng sau mỗi chương, chờ /next"},
	{Value: "off", Note: "Viết liền, không dừng"},
}

var storyOptions = []Option{
	{Value: "open", Note: "Truyện còn dang dở, sẽ viết tiếp"},
	{Value: "closed", Note: "Truyện đã có kết thúc"},
}

var exportFormatOptions = []Option{
	{Value: "txt"},
	{Value: "epub"},
	{Value: "narration", Note: "Bản đọc sạch cho TTS (không tiêu đề, không Markdown)"},
}

// registry 是唯一事实来源；顺序与设计稿 08 的侧栏一致。
var registry = []Spec{
	{
		Name:        "help",
		Usage:       "/help",
		Description: "Xem danh sách và hướng dẫn các lệnh",
		Group:       GroupSys,
		AutoExecute: true,
		Aliases:     []string{},
		Params:      []Param{},
	},
	{
		Name:        "model",
		Usage:       "/model [vai-trò]",
		Description: "Chuyển đổi Model và mức độ suy luận cho từng vai trò",
		Group:       GroupSys,
		AutoExecute: true,
		Aliases:     []string{},
		Params: []Param{{
			Key: "role", Token: "[vai-trò]", Kind: KindEnum, Options: roleOptions,
		}},
	},
	{
		Name:        "config",
		Usage:       "/config",
		Description: "Thêm hoặc chỉnh sửa Provider, Model và Cửa sổ ngữ cảnh",
		Group:       GroupSys,
		AutoExecute: true,
		Aliases:     []string{},
		Params:      []Param{},
	},
	{
		Name:        "diag",
		Usage:       "/diag",
		Description: "Bảng chẩn đoán sức khỏe và chất lượng truyện",
		Group:       GroupWrite,
		AutoExecute: true,
		Aliases:     []string{},
		Params:      []Param{},
	},
	{
		Name:        "review",
		Usage:       "/review on|off",
		Description: "Bật/tắt chế độ nghiệm thu từng chương",
		Group:       GroupWrite,
		Aliases:     []string{},
		Params: []Param{{
			Key: "mode", Token: "on|off", Kind: KindEnum, Required: true, Options: reviewOptions,
		}},
	},
	{
		Name:        "next",
		Usage:       "/next",
		Description: "Nghiệm thu và cho phép viết tiếp một chương mới",
		Group:       GroupWrite,
		AutoExecute: true,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params:      []Param{},
	},
	{
		Name:        "start",
		Usage:       "/start <đường-dẫn-tệp>",
		Description: "Tạo truyện mới từ tệp thiết lập hoặc đại cương có sẵn",
		Group:       GroupBegin,
		Only:        OnlyWelcome,
		Aliases:     []string{},
		Params: []Param{{
			Key: "path", Token: "<đường-dẫn-tệp>", Kind: KindPath, Required: true,
		}},
	},
	{
		Name:        "import",
		Usage:       "/import <đường-dẫn> [--yes] [--story=open|closed] [--continue] [--guide=<hướng-dẫn>]",
		Description: "Nhập tiểu thuyết từ bên ngoài theo ngữ nghĩa (không tham số để tiếp tục)",
		Group:       GroupBegin,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params: []Param{
			{Key: "path", Token: "<đường-dẫn>", Kind: KindPath},
			{Key: "yes", Token: "--yes", Kind: KindFlag, Flag: "--yes"},
			{Key: "story", Token: "--story=open|closed", Kind: KindEnum, Prefix: "--story=", Options: storyOptions},
			{Key: "cont", Token: "--continue", Kind: KindFlag, Flag: "--continue"},
			{Key: "guide", Token: "--guide=<hướng-dẫn>", Kind: KindText, Prefix: "--guide="},
		},
	},
	{
		Name:        "reopen",
		Usage:       "/reopen [hướng-viết-tiếp]",
		Description: "Mở lại cuốn sách đã hoàn thành để tiếp tục sáng tác",
		Group:       GroupEnd,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params: []Param{{
			Key: "dir", Token: "[hướng-viết-tiếp]", Kind: KindText,
		}},
	},
	{
		Name:        "cocreate",
		Aliases:     []string{"plan"},
		Usage:       "/cocreate",
		Description: "Tạm dừng viết để đồng sáng tác quy hoạch giai đoạn tiếp theo",
		Group:       GroupWrite,
		AutoExecute: true,
		Only:        OnlyRunning,
		Params:      []Param{},
	},
	{
		Name:        "simulate",
		Usage:       "/simulate",
		Description: "Thêm vài đoạn văn bạn thích vào Văn mẫu (Tuỳ chỉnh › Cách viết › Văn phong của bạn)…",
		Group:       GroupStyle,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params:      []Param{},
	},
	{
		Name:        "importsim",
		Usage:       "/importsim <tệp-profile.json>",
		Description: "Nhập hồ sơ mô phỏng văn phong có sẵn và gộp theo vân tay ngữ liệu",
		Group:       GroupStyle,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params: []Param{{
			Key: "path", Token: "<tệp-profile.json>", Kind: KindPath, Required: true,
		}},
	},
	{
		Name:        "sync",
		Usage:       "/sync [--check]",
		Description: "Kiểm tra hoặc tiếp nhận các chương đã sửa đổi thủ công từ bên ngoài",
		Group:       GroupWrite,
		AutoExecute: true,
		NeedsIdle:   true,
		Aliases:     []string{},
		Params: []Param{{
			Key: "check", Token: "[--check]", Kind: KindFlag, Flag: "--check",
		}},
	},
	{
		Name:        "export",
		Usage:       "/export [đường-dẫn|txt|epub|narration] [from=N] [to=M] [--overwrite|--keep] [doc-so]",
		Description: "Xuất các chương đã hoàn thành ra tệp TXT, EPUB hoặc narration",
		Group:       GroupEnd,
		AutoExecute: true,
		Aliases:     []string{},
		Params: []Param{
			{Key: "path", Token: "[đường-dẫn]", Kind: KindPath},
			{Key: "from", Token: "from=N", Kind: KindNum, Prefix: "from="},
			{Key: "to", Token: "to=M", Kind: KindNum, Prefix: "to="},
			{Key: "format", Token: "format=txt|epub|narration", Kind: KindEnum, Prefix: "format=", Options: exportFormatOptions},
			{Key: "ow", Token: "--overwrite", Kind: KindFlag, Flag: "--overwrite"},
			{Key: "keep", Token: "--keep", Kind: KindFlag, Flag: "--keep"},
			{Key: "docso", Token: "doc-so", Kind: KindFlag, Flag: "doc-so"},
		},
	},
}

// Specs 返回全部命令声明，按声明顺序。
func Specs() []Spec {
	out := make([]Spec, len(registry))
	copy(out, registry)
	return out
}

// Find 按名称或别名查找命令，忽略大小写与首尾空白。
func Find(name string) (Spec, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Spec{}, false
	}
	for _, spec := range registry {
		if spec.matches(name) {
			return spec, true
		}
	}
	return Spec{}, false
}

func (s Spec) matches(name string) bool {
	if strings.EqualFold(s.Name, name) {
		return true
	}
	for _, alias := range s.Aliases {
		if strings.EqualFold(alias, name) {
			return true
		}
	}
	return false
}

func hasOption(options []Option, value string) bool {
	for _, opt := range options {
		if opt.Value == value {
			return true
		}
	}
	return false
}
