package store

// mdLabels 是派生 Markdown 视图里的固定越南语标签。
//
// 这些 .md 会被 novel_context 读回模型上下文，因此标签语种不只是显示问题：
// 越南语正文里每章顶着「第 N 章」「核心事件」，等于持续暗示模型当前是中文语境，
// 实测导致正文夹带汉字（quan sát草木 / Bạo虐 / 顿号）。
type mdLabels struct {
	bookTitleFmt string
	synopsis     string
	charProfiles string
	charArc      string
	traits       string
	listSep      string
	openParen    string
	closeParen   string
	colon        string

	outline        string
	layeredOutline string
	volumeFmt      string
	arcFmt         string
	chapterFmt     string
	theme          string
	goal           string
	pendingArcFmt  string
	coreEvent      string
	hook           string
	scenes         string

	timeline      string
	foreshadow    string
	resolvedAtFmt string
	plantedAtFmt  string
	relationships string
	atChapterFmt  string
	worldRules    string
	rule          string
	boundary      string
}

// mdLabelsVI 用项目既定译名：tập / cung / chương / đại cương / điểm móc / phục bút。
var mdLabelsVI = mdLabels{
	bookTitleFmt: "%s", synopsis: "Giới thiệu", charProfiles: "Hồ sơ nhân vật", charArc: "Cung nhân vật",
	traits: "Đặc điểm", listSep: ", ", openParen: " (", closeParen: ")", colon: ": ",

	outline: "Đại cương", layeredOutline: "Đại cương phân tầng",
	volumeFmt: "Tập %d", arcFmt: "Cung %d", chapterFmt: "Chương %d",
	theme: "Chủ đề", goal: "Mục tiêu", pendingArcFmt: "*(chưa khai triển, ước %d chương)*",
	coreEvent: "Sự kiện chính", hook: "Điểm móc", scenes: "Cảnh",

	timeline: "Dòng thời gian", foreshadow: "Sổ phục bút",
	resolvedAtFmt: "đã thu ở chương %d", plantedAtFmt: "gieo ở chương %d, trạng thái: %s",
	relationships: "Quan hệ nhân vật", atChapterFmt: " (chương %d)",
	worldRules: "Luật thế giới", rule: "Luật", boundary: "Ranh giới",
}
