package host

import (
	"regexp"
	"strconv"
	"strings"
)

// legacyEventRules 是旧版（中文界面时期）写进 tui.log 的事件文本 → 当前越南语文案的兼容表。
// 只在读历史日志时使用（web Nhật ký 直接读 tui.log）；新事件在源头就是越南语。
// 顺序有意义：先匹配整句，再匹配片段，最后兜底替换全角标点。
// 右列必须与当前源头文案一致（observer_tools.go、host.go、engine.go、exp）。
var legacyEventRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	// ── 工具调用标签（displayToolName）──
	{regexp.MustCompile(`\(第(\d+)章·草稿\)`), "(chương $1 · bản nháp)"},
	{regexp.MustCompile(`\(第(\d+)章\)`), "(chương $1)"},
	{regexp.MustCompile(`\(第(\d+)章·`), "(chương $1·"},
	{regexp.MustCompile(`\(全局·`), "(toàn cục·"},
	{regexp.MustCompile(`\(全局\)`), "(toàn cục)"},
	{regexp.MustCompile(`\(本弧·`), "(cung này·"},
	{regexp.MustCompile(`\(本弧\)`), "(cung này)"},
	{regexp.MustCompile(` 错误: `), " lỗi: "},

	// ── 派发（dispatchSummary / dispatchDetail，任务文案来自 flow/router.go）──
	{regexp.MustCompile(`重写第 (\d+) 章`), "Viết lại chương $1"},
	{regexp.MustCompile(`写第 (\d+) 章`), "Viết chương $1"},
	{regexp.MustCompile(`审阅前 (\d+) 章：调用`), "Thẩm duyệt $1 chương đầu: gọi"},
	{regexp.MustCompile(`派发原因: `), "Lý do điều phối: "},
	{regexp.MustCompile(`完整任务:`), "Nhiệm vụ đầy đủ:"},
	{regexp.MustCompile(`PendingRewrites 队列剩余 (\d+) 章`), "hàng đợi PendingRewrites còn $1 chương"},
	{regexp.MustCompile(`续写下一章`), "viết tiếp chương sau"},
	{regexp.MustCompile(`全局审阅缺失`), "thiếu thẩm duyệt toàn cục"},

	// ── 重试 / 裁定 / 系统事件 ──
	{regexp.MustCompile(`重试 \((\d+)/(\d+)，(\S+?)后\)`), "Thử lại ($1/$2, sau $3)"},
	{regexp.MustCompile(`重试 \(`), "Thử lại ("},
	{regexp.MustCompile(`用户干预裁定`), "Phán định can thiệp"},
	{regexp.MustCompile(`启动裁定`), "Phán định khởi động"},
	{regexp.MustCompile(`失败裁定`), "Phán định thất bại"},
	{regexp.MustCompile(`^裁定[:：]\s*`), "Phán định: "},
	{regexp.MustCompile(`开始创作（规划师: (\S+?)——`), "Bắt đầu sáng tác (Kiến trúc sư lập kế hoạch truyện dài — "},
	{regexp.MustCompile(`已重开全书返工: 第 \[(.*?)\] 章入队`), "Đã mở lại toàn bộ để viết lại: chương [$1] vào hàng đợi"},
	{regexp.MustCompile(`StopGuard: (\S+) 未完成必要产物就试图结束，已拦截催促（连续第 (\d+) 次）`), "StopGuard: $1 định kết thúc khi chưa tạo đủ sản phẩm bắt buộc, đã chặn lệnh thúc giục (lần thứ $2 liên tiếp)"},
	{regexp.MustCompile(`模型已切换：`), "Đã đổi model: "},
	{regexp.MustCompile(`推理强度已切换：`), "Đã đổi mức suy luận: "},
	{regexp.MustCompile(`Provider 配置已保存：`), "Đã lưu cấu hình Provider: "},
	{regexp.MustCompile(`第 (\d+) 章正文和标题均未发生变化，未检测到重写改动`), "chính văn và tiêu đề chương $1 không thay đổi, không phát hiện chỉnh sửa viết lại"},

	// ── 导出（exp）──
	{regexp.MustCompile(`尚无已完成章节，无内容可导出`), "chưa có chương hoàn thành nào để xuất"},
	{regexp.MustCompile(`文件已存在：(.+?)（添加 --overwrite 覆盖）`), "tệp đã tồn tại: $1 (dùng --overwrite để ghi đè)"},
	{regexp.MustCompile(`写入失败：rename .+?\.tmp-\d+ (.+): file exists`), "ghi tệp thất bại: đã có thư mục hoặc tệp trùng tên $1"},
	{regexp.MustCompile(`写入失败：`), "ghi tệp thất bại: "},

	// ── 兜底片段 ──
	{regexp.MustCompile(`第\s*(\d+)\s*章`), "chương $1"},
	{regexp.MustCompile(`草稿`), "bản nháp"},
	{regexp.MustCompile(`（`), " ("},
	{regexp.MustCompile(`）`), ")"},
	{regexp.MustCompile(`：`), ": "},
	{regexp.MustCompile(`，`), ", "},
	{regexp.MustCompile(`《`), "«"},
	{regexp.MustCompile(`》`), "»"},
}

// legacyCompletion 是旧版完本摘要；字数要按 completionSummary 的 vi-VN 千分位重排，不能走纯替换表。
var legacyCompletion = regexp.MustCompile(`《(.+?)》创作完成: 共 (\d+) 章 (\d+) 字`)

// legacyHan 判断文本里是否还有汉字；没有就无需过兼容表。
var legacyHan = regexp.MustCompile(`[\x{4e00}-\x{9fff}（），：《》]`)

// LegacyEventText 把旧版日志中的中文事件文本转成当前越南语文案；不含汉字的文本原样返回。
// 模型生成的自由文本（裁定理由等）无法逐字翻译，只替换其中已知的固定片段。
func LegacyEventText(s string) string {
	if !legacyHan.MatchString(s) {
		return s
	}
	s = legacyCompletion.ReplaceAllStringFunc(s, func(m string) string {
		g := legacyCompletion.FindStringSubmatch(m)
		words, err := strconv.Atoi(g[3])
		if err != nil {
			return m
		}
		return "«" + g[1] + "» hoàn thành: " + g[2] + " chương, " + formatCount(words) + " chữ"
	})
	for _, rule := range legacyEventRules {
		s = rule.re.ReplaceAllString(s, rule.repl)
	}
	return s
}

// legacyUserPrefixes 是旧版给用户消息加的系统前缀；用户正文本身不翻译。
var legacyUserPrefixes = [][2]string{{"[继续] ", "[Tiếp tục] "}, {"[干预] ", "[Can thiệp] "}}

// LegacyUserText 只改写旧版用户消息的系统前缀（[继续]/[干预]），正文原样保留。
func LegacyUserText(s string) string {
	for _, p := range legacyUserPrefixes {
		if rest, ok := strings.CutPrefix(s, p[0]); ok {
			return p[1] + rest
		}
	}
	return s
}

// LegacyEvent 对事件的提示文案字段应用 LegacyEventText；工具参数/结果是原始 I/O，不改写。
func LegacyEvent(ev Event) Event {
	return mapEventMessages(ev, LegacyEventText)
}
