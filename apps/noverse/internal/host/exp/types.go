// Package exp 实现已完成章节的导出能力。
//
// 与 imp/ 对称：纯本地 IO，不依赖 LLM，不改 store 状态。导出可以与
// Engine 并发运行（只读 Progress + 章节终稿），属于横向能力。
//
// 当前支持 TXT、EPUB 与 narration（TTS 干净文本，Master v3 §15.5）。
package exp

import "github.com/datnt1998/noverse/internal/store"

// Format 标识导出格式。
type Format string

const (
	// FormatTXT 纯文本输出。
	FormatTXT Format = "txt"
	// FormatEPUB 标准 EPUB 3 容器（zip + xhtml）。
	FormatEPUB Format = "epub"
	// FormatNarration 朗读用干净文本：无书名/章标题/Markdown，
	// 章间两个空行、段间一个空行，UTF-8 + LF + 末尾换行（§15.5）。
	// 无法由后缀推断，只能显式指定；落盘仍是 .txt。
	FormatNarration Format = "narration"
)

// FileSuffix 返回该格式默认文件名的后缀（含分隔符）。
// narration 仍写 .txt，但用 "-narration" 中缀避免与普通 TXT 同名。
func (f Format) FileSuffix() string {
	switch f {
	case FormatEPUB:
		return ".epub"
	case FormatNarration:
		return "-narration.txt"
	default:
		return ".txt"
	}
}

// FileExt 返回该格式落盘的纯扩展名（导出 pattern 的 {ext} 变量用）。
func (f Format) FileExt() string {
	if f == FormatEPUB {
		return "epub"
	}
	return "txt"
}

// Options 控制导出行为。zero-value 等价于"导出全本到默认路径，文件存在时报错"。
//
// 版式：《书名》 → 卷分隔 → 章节正文。两类内部数据不进导出：premise（创作蓝图，
// 含目标读者 / 核心消费点 / 写作禁区等后台元信息，给作者与引擎看，不是读者的序）；
// 弧分隔（读者视角下弧是过细的内部结构）。书名与卷分隔始终保留。
type Options struct {
	// Format 空字符串时由 OutPath 后缀推断（.txt → TXT，.epub → EPUB）；
	// OutPath 也为空时回退 FormatTXT。SDK 调用方可显式指定以跳过推断。
	Format Format

	// OutPath 输出文件路径；空表示 {novelDir}/{BookMetadata.Title}.{ext}。
	// 开头的 "~" 展开为用户主目录；指向已存在目录或以 "/" 结尾时，
	// 文件写入该目录，文件名同样取 {BookMetadata.Title}.{ext}。
	OutPath string

	// From / To 章节范围，闭区间。0 表示从第 1 章 / 到最后一章。
	// 范围内未完成的章节会被跳过并写入 Result.Skipped，不视为错误。
	From, To int

	// OnExists 输出文件已存在时的策略："keep" 在扩展名前追加 -{YYYYMMDD-HHmmss}
	// 后保留两个文件；"replace" 覆盖；空（默认）报错。Overwrite 为旧路径的显式
	// 覆盖开关，置位时优先于 OnExists。
	OnExists string

	// Overwrite 文件存在时是否覆盖；默认拒绝。
	Overwrite bool

	// ReadNumbers 仅对 FormatNarration 生效：用 audio 的确定性规范化把数字、
	// 日期、金额、符号等读成越南语字（§15.5 "bản narration không chứa chữ số"）。
	ReadNumbers bool

	// DryRun 只渲染不写盘、不检查冲突：Result.Path/Bytes/Chapters/Skipped 是“若现在
	// 导出”的预估（web 的同名文件比较卡片用）。
	DryRun bool

	// Progress 每读完一章的标题后回调（done 从 1 起，total 为将写入的章数，
	// chapter 为章号，title 可能为空）；nil 不回调。在 Run 的调用 goroutine 中执行。
	Progress func(done, total, chapter int, title string)
}

// ExistsError 表示默认策略下目标文件已存在（未写入任何东西）。
type ExistsError struct {
	// Path 为已存在的目标文件路径。
	Path string
}

func (e *ExistsError) Error() string {
	return "tệp đã tồn tại: " + e.Path + " (dùng --overwrite để ghi đè)"
}

// Deps 是 Run 所需依赖。仅 store；导出无需 LLM、prompt、bundle。
type Deps struct {
	Store *store.Store
}

// Result 是一次成功导出的产物摘要。
type Result struct {
	// Path 实际写入的文件路径（绝对或调用方传入的相对）。
	Path string
	// Chapters 实际写入的章节数。
	Chapters int
	// Bytes 文件字节数（UTF-8）。
	Bytes int
	// Skipped 落在请求范围内但未完成的章节号。
	Skipped []int
}
