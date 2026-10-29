package host

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/store"
)

// 冷启动共创：从零澄清需求，产出整本书的创作指令。
const coCreateSystemPrompt = `Bạn là trợ lý đồng sáng tác truyện. Nhiệm vụ của bạn không phải trực tiếp bắt đầu viết truyện, mà là qua nhiều lượt đối thoại ngắn giúp người dùng làm rõ nhu cầu sáng tác, đồng thời liên tục chỉnh lý ra một chỉ thị sáng tác tiếng Việt có thể giao trực tiếp cho engine sáng tác.

Mỗi lượt trả lời phải xuất nghiêm ngặt theo định dạng XML sau, gồm năm thẻ xuất hiện lần lượt, mỗi thẻ đều phải có thẻ mở và thẻ đóng đúng:

<reply>
Trả lời tự nhiên bằng tiếng Việt cho người dùng xem: trước hết đáp lại đầu vào của người dùng, rồi đặt tối đa 1 đến 2 câu hỏi quan trọng nhất hiện tại. Nếu thông tin đã đủ để bắt đầu sáng tác, hãy bảo người dùng có thể nhấn Ctrl+S để bắt đầu.
</reply>

<draft>
Bản nháp chỉ thị sáng tác đầy đủ hiện tại, dùng Markdown: bắt đầu trực tiếp từ tiêu đề cấp hai, ví dụ "## Chủ đề", "## Yếu tố then chốt", "## Thông tin cần làm rõ"; liệt kê các ý chính bằng gạch đầu dòng. Mỗi lượt phải **cập nhật tích luỹ** trên các kết luận đã có, hấp thụ ý định mới nhất của người dùng; kể cả khi lượt này không có bổ sung cũng phải viết lại nguyên văn bản nháp đầy đủ — không được lược bớt, không được viết chỗ giữ chỗ kiểu "(giữ nguyên lượt trước)".
</draft>
` + coCreateProtocolTail

// 阶段共创：小说已写了一部分，规划"后续阶段"的走向。调用方需把当前故事状态摘要
// 追加到本 prompt 之后（"## 当前故事状态" 段），让模型在已写内容的基础上规划。
const stageCoCreateSystemPrompt = `Bạn là trợ lý "đồng sáng tác theo giai đoạn" cho truyện. Truyện này đã viết một phần (tiến độ xem ở mục "Trạng thái truyện hiện tại" bên dưới). Người dùng tạm dừng lại, muốn cùng bạn lên kế hoạch hướng đi của "giai đoạn tiếp theo", rồi mới tiếp tục sáng tác.

Nhiệm vụ của bạn không phải viết tiếp nội văn, mà là qua nhiều lượt đối thoại ngắn giúp người dùng nghĩ rõ đoạn tiếp theo (vài chương kế tiếp / cung tiếp theo / tập tiếp theo) sẽ đi về đâu, đồng thời liên tục chỉnh lý ra một "brief hướng tiếp theo" để engine sáng tác dựa vào đó đẩy tiến.

Luật bất biến: mọi đề xuất phải nhất quán với cốt truyện, nhân vật, phục bút đã xảy ra trong "Trạng thái truyện hiện tại", tuyệt đối không lật đổ hay bỏ qua nội dung đã viết; chỉ lập kế hoạch "đi tiếp thế nào", không thiết kế lại cả cuốn truyện.

Mỗi lượt trả lời phải xuất nghiêm ngặt theo định dạng XML sau, gồm năm thẻ xuất hiện lần lượt, mỗi thẻ đều phải có thẻ mở và thẻ đóng đúng:

<reply>
Trả lời tự nhiên bằng tiếng Việt cho người dùng xem: trước hết đáp lại đầu vào của người dùng, rồi đặt tối đa 1 đến 2 câu hỏi quan trọng nhất hiện tại. Nếu hướng tiếp theo đã đủ rõ ràng, hãy bảo người dùng có thể nhấn Ctrl+S để giao hướng đi cho engine sáng tác và tiếp tục sáng tác.
</reply>

<draft>
"Brief hướng tiếp theo" đầy đủ hiện tại, dùng Markdown: bắt đầu trực tiếp từ tiêu đề cấp hai, ví dụ "## Hướng đi tiếp theo", "## Bước ngoặt then chốt", "## Phục bút cần thu", "## Nhịp độ và độ dài"; liệt kê các ý chính bằng gạch đầu dòng. Mỗi lượt phải **cập nhật tích luỹ** trên các kết luận đã có, hấp thụ ý định mới nhất của người dùng; kể cả khi lượt này không có bổ sung cũng phải viết lại nguyên văn brief đầy đủ — không được lược bớt, không được viết chỗ giữ chỗ kiểu "(giữ nguyên lượt trước)".
</draft>
` + coCreateProtocolTail

// coCreateProtocolTail is shared by both modes: readiness, suggestions and proposal.
// 两模式只在开场语境与 <draft> 语义上不同，协议完全一致。
const coCreateProtocolTail = `
<ready>false</ready>

<suggestions>
1-3 câu "người dùng có thể muốn nói tiếp theo", mỗi dòng một câu bắt đầu bằng "- ". Đây là gợi ý dẫn dắt khi người dùng bị bế,
nhấn phím số để điền vào ô nhập, người dùng có thể chỉnh sửa rồi gửi.

Yêu cầu:
- Dùng giọng của người dùng, như lời người dùng nói với bạn, đừng viết thành câu hỏi ngược của trợ lý.
- Mỗi câu không quá 25 chữ, cấu trúc câu đa dạng, tránh nghe như một khuôn mẫu.
- Đưa ra khuynh hướng / lựa chọn / ý định bổ sung, đừng dùng một câu để viết sẵn toàn bộ thiết lập thay người dùng.
</suggestions>

<proposal>
<title>Tiêu đề hướng đi đã thống nhất</title>
<chapters>
<chapter><number>Số chương</number><title>Tên chương</title><summary>Một câu tóm tắt diễn biến đề xuất</summary></chapter>
</chapters>
<impacts><impact>Một tác động cụ thể lên dàn ý, chương đã viết hoặc phục bút</impact></impacts>
</proposal>

Yêu cầu đề xuất:
- Khi đủ dữ kiện để đề xuất, điền tiêu đề, các chương theo thứ tự số tăng dần và ít nhất một tác động. Số chương phải đúng với tiến độ thật; không sao chép số hay nội dung ví dụ.
- Mỗi tên chương, tóm tắt và tác động là một dòng tiếng Việt. Tác động chỉ nêu thay đổi thực sự có trong bản nháp, không bịa số chương giữ nguyên hay số phục bút.
- Khi người dùng góp ý, trả lại toàn bộ đề xuất đã chỉnh, đồng thời cập nhật <draft> với cùng các chương và tác động để engine thực hiện đúng hướng đã chốt.
- Chưa có đề xuất đủ rõ thì để <proposal></proposal>, không tự tạo chương giả. Dùng &amp; và &lt; khi cần ký tự & và < trong nội dung đề xuất.

Quy chuẩn đầu ra:
- Phải dùng năm thẻ XML: <reply> / <draft> / <ready> / <suggestions> / <proposal>, mỗi thẻ đều phải mở đóng đầy đủ.
- Tên thẻ chỉ được là chữ thường tiếng Anh, không đổi thành <REPLY> / <REWRITE> / <tra-loi> hay bất kỳ biến thể nào.
- Không thêm bất kỳ giải thích, suy nghĩ hay hàng rào mã nào bên ngoài các thẻ.
- Trong <draft> cho phép Markdown nhiều dòng, cứ xuống dòng trực tiếp, không cần escape gì.
- <ready> chỉ viết true hoặc false. Khi thông tin đã đủ thì điền true.
- Khi <ready>true</ready>, <suggestions> có thể để trống (giữ thẻ rỗng <suggestions></suggestions> là được).`

// CoCreateProgressKind 标识流式回调的内容类型。
const (
	CoCreateProgressThinking = "thinking"
	CoCreateProgressReply    = "reply"
)

// Five-tag XML protocol. Closing tags delimit streaming previews precisely.
const (
	tagReply       = "reply"
	tagDraft       = "draft"
	tagReady       = "ready"
	tagSuggestions = "suggestions"
	tagProposal    = "proposal"
)

// CoCreateProposal contains only structured content supplied by the model.
// Revision is assigned by the session, never trusted from model output.
type CoCreateProposal struct {
	SchemaVersion int                       `json:"schemaVersion" xml:"-"`
	Title         string                    `json:"title" xml:"title"`
	Revision      int                       `json:"revision" xml:"-"`
	Chapters      []CoCreateProposalChapter `json:"chapters" xml:"chapters>chapter"`
	Impacts       []string                  `json:"impacts" xml:"impacts>impact"`
}

type CoCreateProposalChapter struct {
	SchemaVersion int    `json:"schemaVersion" xml:"-"`
	Chapter       int    `json:"chapter" xml:"number"`
	Title         string `json:"title" xml:"title"`
	Summary       string `json:"summary" xml:"summary"`
}

func coCreateStream(ctx context.Context, models *bootstrap.ModelSet, sessions store.SessionStore, sysPrompt string, history []CoCreateMessage, onProgress func(kind, text string)) (reply CoCreateReply, err error) {
	if len(history) == 0 {
		return CoCreateReply{}, fmt.Errorf("Lịch sử đồng sáng tác trống")
	}

	model := models.ForRole("thinking")

	msgs := []llmtypes.Message{llmtypes.SystemMsg(sysPrompt)}
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "assistant":
			msgs = append(msgs, assistantMsg(content))
		default:
			msgs = append(msgs, llmtypes.UserMsg(content))
		}
	}

	var raw, thinking strings.Builder

	// 排查"model trả về phản hồi trống"等偶发问题需要看到模型实际返回什么。
	// 每轮全程落盘到 <output>/meta/sessions/cocreate.jsonl，与正式创作的 session 日志同位。
	start := time.Now()
	defer func() {
		if sessions == nil {
			return
		}
		if logErr := sessions.LogCoCreate(coCreateLogEntry{
			Time:         time.Now(),
			DurationMS:   time.Since(start).Milliseconds(),
			InputHistory: history,
			RawResponse:  raw.String(),
			RawLen:       len([]rune(raw.String())),
			Thinking:     thinking.String(),
			ParsedReply:  reply.Message,
			ParsedDraft:  reply.Prompt,
			ParsedReady:  reply.Ready,
			ParsedSugs:   reply.Suggestions,
			Error:        errString(err),
		}); logErr != nil {
			slog.Warn("Ghi log phiên đồng sáng tác xuống đĩa thất bại", "module", "cocreate", "err", logErr)
		}
	}()

	streamCh, err := model.GenerateStream(ctx, msgs, nil, llmtypes.WithMaxTokens(2048))
	if err != nil {
		return CoCreateReply{}, fmt.Errorf("Đồng sáng tác: gọi model thất bại: %w", err)
	}

	var streamed bool
	for ev := range streamCh {
		switch ev.Type {
		case llmtypes.StreamEventThinkingDelta:
			thinking.WriteString(ev.Delta)
			if onProgress != nil {
				onProgress(CoCreateProgressThinking, thinking.String())
			}
		case llmtypes.StreamEventTextDelta:
			streamed = true
			raw.WriteString(ev.Delta)
			if onProgress != nil {
				onProgress(CoCreateProgressReply, extractReplyPreview(raw.String()))
			}
		case llmtypes.StreamEventDone:
			if !streamed {
				raw.WriteString(ev.Message.TextContent())
			}
		case llmtypes.StreamEventError:
			if ev.Err != nil {
				return CoCreateReply{}, fmt.Errorf("Đồng sáng tác: gọi model thất bại: %w", ev.Err)
			}
			return CoCreateReply{}, fmt.Errorf("Đồng sáng tác: gọi model thất bại")
		}
	}

	// Some reasoning models place the complete answer in reasoning_content.
	// Parse that same real output when the final-answer channel is empty.
	rawText := raw.String()
	if strings.TrimSpace(rawText) == "" {
		if t := strings.TrimSpace(thinking.String()); t != "" {
			rawText = t
		}
	}
	reply, err = parseCoCreateResponse(rawText)
	return reply, err
}

// coCreateLogEntry 是写入 meta/sessions/cocreate.jsonl 的一行结构。
// 字段命名贴近 jsonl 直查习惯（snake_case），方便 jq 过滤。
type coCreateLogEntry struct {
	Time         time.Time         `json:"time"`
	DurationMS   int64             `json:"duration_ms"`
	InputHistory []CoCreateMessage `json:"input_history"`
	RawResponse  string            `json:"raw_response"`
	RawLen       int               `json:"raw_len"`
	Thinking     string            `json:"thinking,omitempty"`
	ParsedReply  string            `json:"parsed_reply"`
	ParsedDraft  string            `json:"parsed_draft"`
	ParsedReady  bool              `json:"parsed_ready"`
	ParsedSugs   []string          `json:"parsed_sugs,omitempty"`
	Error        string            `json:"error,omitempty"`
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func assistantMsg(text string) llmtypes.Message {
	return llmtypes.Message{
		Role:      llmtypes.RoleAssistant,
		Content:   []llmtypes.ContentBlock{llmtypes.TextBlock(text)},
		Timestamp: time.Now(),
	}
}

// parseCoCreateResponse 解析 XML 标签输出。模型若没遵守协议（直接说自然语言），
// 整段作为 reply 显示，draft 留空让 session 保留上一轮。
func parseCoCreateResponse(raw string) (CoCreateReply, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CoCreateReply{}, fmt.Errorf("Đồng sáng tác: model trả về phản hồi trống")
	}

	reply, draft, ready, suggestions := splitCoCreateMarkers(raw)
	if reply == "" {
		// 模型没遵守 XML 协议：整段作为 reply。
		return CoCreateReply{Message: raw, Prompt: "", Ready: false, Raw: raw}, nil
	}
	return CoCreateReply{
		Message:     reply,
		Prompt:      draft,
		Ready:       ready,
		Suggestions: suggestions,
		Proposal:    parseCoCreateProposal(extractTagContent(raw, tagProposal)),
		Raw:         raw,
	}, nil
}

func parseCoCreateProposal(content string) *CoCreateProposal {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var proposal CoCreateProposal
	if err := xml.Unmarshal([]byte("<proposal>"+content+"</proposal>"), &proposal); err != nil {
		return nil
	}
	proposal.Title = strings.Join(strings.Fields(proposal.Title), " ")
	if proposal.Title == "" || len(proposal.Chapters) == 0 || len(proposal.Impacts) == 0 {
		return nil
	}
	lastChapter := 0
	for i := range proposal.Chapters {
		chapter := &proposal.Chapters[i]
		chapter.Title = strings.Join(strings.Fields(chapter.Title), " ")
		chapter.Summary = strings.Join(strings.Fields(chapter.Summary), " ")
		if chapter.Chapter <= lastChapter || chapter.Title == "" || chapter.Summary == "" {
			return nil
		}
		chapter.SchemaVersion = 1
		lastChapter = chapter.Chapter
	}
	for i, impact := range proposal.Impacts {
		proposal.Impacts[i] = strings.Join(strings.Fields(impact), " ")
		if proposal.Impacts[i] == "" {
			return nil
		}
	}
	proposal.SchemaVersion = 1
	return &proposal
}

// splitCoCreateMarkers 按四个 XML 标签切分文本。
// 标签可能缺失（流式中段或模型遗漏），缺失部分对应字段为空 / false / nil。
// 缺失闭标签时，extractTagContent 会取到字符串末尾，仍尽力解析。
func splitCoCreateMarkers(s string) (reply, draft string, ready bool, suggestions []string) {
	reply = extractTagContent(s, tagReply)
	draft = extractTagContent(s, tagDraft)
	readyStr := strings.ToLower(extractTagContent(s, tagReady))
	ready = readyStr == "true" || readyStr == "yes"
	suggestions = parseSuggestions(extractTagContent(s, tagSuggestions))
	return
}

// extractTagContent 从 s 中抠出 <tag>...</tag> 之间的文本。
// 三种偶发故障场景兜底，避免直接走降级丢字段：
//  1. 有开无闭（流式中段）→ 切到下一个已知开标签前
//  2. 无开有闭（模型 typo，如 <suggestions> 写成 <uggestions>）→ 从最近一个已知
//     完整闭合标签的结束位置开始，到 </tag> 之前
//  3. reply 完全无开标签（模型直接以自然语言开篇，末尾贴 </reply>）→ 从开头到 </reply>
func extractTagContent(s, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	oIdx := strings.Index(s, open)
	if oIdx >= 0 {
		rest := s[oIdx+len(open):]
		if cIdx := strings.Index(rest, closeTag); cIdx >= 0 {
			return strings.TrimSpace(rest[:cIdx])
		}
		// 有开无闭 → 切到下一个已知开标签前
		for _, other := range []string{"<reply>", "<draft>", "<ready>", "<suggestions>", "<proposal>"} {
			if other == open {
				continue
			}
			if idx := strings.Index(rest, other); idx >= 0 {
				rest = rest[:idx]
			}
		}
		return strings.TrimSpace(rest)
	}

	// 无开有闭 → 从最近一个已知完整闭合标签的结束位置开始，到 </tag>。
	if cIdx := strings.Index(s, closeTag); cIdx >= 0 {
		prefix := s[:cIdx]
		start := 0
		for _, t := range []string{"</reply>", "</draft>", "</ready>", "</suggestions>", "</proposal>"} {
			if t == closeTag {
				continue
			}
			if i := strings.LastIndex(prefix, t); i >= 0 {
				if end := i + len(t); end > start {
					start = end
				}
			}
		}
		return strings.TrimSpace(prefix[start:])
	}
	return ""
}

// parseSuggestions 把 <suggestions> 段每行抠出来，去掉 "- " / "* " / "1. " 等列表前缀。
// 最多保留 3 条；空行、过短（<2 字）、整行像 XML 标签的（typo 开标签兜底残留，
// 例如 <uggestions>）忽略。
func parseSuggestions(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 整行像 XML 标签 → 跳过（防 typo 开标签污染）
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
			continue
		}
		// 剥列表前缀
		switch {
		case strings.HasPrefix(line, "- "):
			line = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "* "):
			line = strings.TrimSpace(line[2:])
		case isOrderedSuggestion(line):
			line = stripOrderedPrefix(line)
		}
		if len([]rune(line)) < 2 {
			continue
		}
		out = append(out, line)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

// isOrderedSuggestion 判断行首是否形如 "1. " / "12. "（数字+点+空格）。
func isOrderedSuggestion(line string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(line) && line[i] == '.' && line[i+1] == ' '
}

func stripOrderedPrefix(line string) string {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(line) {
		return line
	}
	return strings.TrimSpace(line[i+2:])
}

// extractReplyPreview 流式预览：raw 还在生长时给 UI 一段可显示的文本。
// 找到 <reply> 之后的内容，切到 </reply> 或下一个开标签 <draft> 之前。
// 模型半遵守（漏 <reply> 开标签）时，开头到 </reply> 或 <draft> 都算 reply。
func extractReplyPreview(raw string) string {
	trimmed := strings.TrimSpace(raw)
	open := "<" + tagReply + ">"
	closeTag := "</" + tagReply + ">"
	markers := []string{"<draft>", "<ready>", "<suggestions>", "<proposal>"}

	rest := trimmed
	if rIdx := strings.Index(trimmed, open); rIdx >= 0 {
		rest = trimmed[rIdx+len(open):]
	}
	if cIdx := strings.Index(rest, closeTag); cIdx >= 0 {
		return strings.TrimSpace(rest[:cIdx])
	}
	for _, marker := range markers {
		if idx := strings.Index(rest, marker); idx >= 0 {
			rest = rest[:idx]
		}
	}
	return strings.TrimSpace(rest)
}
