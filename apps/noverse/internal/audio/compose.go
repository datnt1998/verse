package audio

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// AI 把一章正文拆成"朗读段 + 谁在说"。这是唯一需要模型的地方：
// 规范化、标志位、兜底全部在 Go 里做，模型只做它擅长的切分与归属判断。

// ComposeInput 是一次"soạn bản đọc"的输入。
type ComposeInput struct {
	Volume     int
	Chapter    int
	Title      string
	Text       string
	Characters []ComposeSpeaker
	Mode       string // cast | single
	Normalize  NormalizeOptions
	Dict       []Pronunciation
}

// ComposeSpeaker 是给模型看的说话人清单。
type ComposeSpeaker struct {
	ID      string
	Name    string
	Role    string
	Aliases []string
}

// ComposeResult 是一次 soạn 的结果。
type ComposeResult struct {
	Segments []Segment
	Model    string
}

// Composer 抽象"AI soạn bản đọc"，测试可注入假实现。
type Composer interface {
	Compose(ctx context.Context, in ComposeInput) (ComposeResult, error)
}

// ChatComposer 用故事当前配置的模型（gateway 用 bootstrap.NewModelSet 解析三级配置）。
type ChatComposer struct {
	Model llmtypes.ChatModel
	ID    string // "provider/model"，进缓存键
}

// composeSystemPrompt 是输出协议。要求纯 JSON（不要 markdown 围栏）：
// 解析失败时仍有兜底（剥围栏 + 截取第一个 { 到最后一个 }）。
const composeSystemPrompt = `Bạn là biên tập viên bản đọc (audio script) cho một tiểu thuyết tiếng Việt.
Nhiệm vụ: chia một chương thành các ĐOẠN ĐỌC liên tiếp, gán người nói cho từng đoạn.

Quy tắc bắt buộc:
1. Mỗi đoạn là một câu hoặc một cụm câu liền mạch, tối đa 35 từ. Câu dài hơn phải cắt ở dấu chấm, dấu chấm phẩy hoặc dấu phẩy.
2. Lời thoại trong ngoặc kép (“…”) là một đoạn riêng, speaker là người nói. Phần dẫn truyện ngoài ngoặc kép có speaker "dan".
3. Chỉ dùng speaker id trong danh sách người nói được cung cấp; nếu không chắc chắn, chọn người khả dĩ nhất và ghi thêm "guess" (tối đa 3 ứng viên kèm xác suất 0–1, tổng xấp xỉ 1).
4. "tone" là sắc thái ngắn bằng tiếng Việt khi cần (Bình thản, Mỉa mai nhẹ, Ngọt, Lạnh, Giễu cợt, Thì thầm…). Để trống nếu bình thường.
5. "pauseMs" là khoảng ngừng sau đoạn: 0–200 bình thường, 300–600 khi chuyển cảnh/ý, 600–900 trước lời thoại quan trọng.
6. "text" giữ nguyên văn, chỉ bỏ phần không đọc được (ghi chú trong ngoặc vuông, ký hiệu trang trí). Không viết lại, không tóm tắt, không thêm chữ.
7. Không bỏ sót câu nào; giữ đúng thứ tự trong chương.

Chỉ trả về JSON đúng một đối tượng, không kèm giải thích, không markdown:
{"segments":[{"speaker":"dan","text":"…","tone":"","pauseMs":0,"guess":[{"who":"id","p":0.62}]}]}`

// Compose 调用模型并做确定性后处理。
func (c *ChatComposer) Compose(ctx context.Context, in ComposeInput) (ComposeResult, error) {
	if c == nil || c.Model == nil {
		return ComposeResult{}, errors.New("chưa cấu hình mô hình AI cho truyện này")
	}
	if strings.TrimSpace(in.Text) == "" {
		return ComposeResult{}, errors.New("chương chưa có nội dung để soạn bản đọc")
	}
	// 剧本导出的章节正文里可能残留场景标题（CẢNH 7 – MONTAGE…）、地点行
	// （PHÒNG DỰ ÁN AI.）、说话人标签（TRIỆU: …）与转场行（CẮT ĐEN.）。这些
	// 都是排版标注：正文改成能朗读的句子，转场换成长停顿，标签换成台词归属。
	plan := PlanReadingText(in.Text)
	if len(plan.Changed) > 0 {
		if strings.TrimSpace(plan.Text) == "" {
			return ComposeResult{}, errors.New("chương chỉ có tiêu đề cảnh kiểu kịch bản, không còn nội dung để đọc")
		}
		in.Text = plan.Text
	}
	// Bản đọc chạy nền nhưng không được chờ nhà cung cấp vô hạn.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	user := buildComposeUserPrompt(in)
	resp, err := c.Model.Generate(ctx,
		[]llmtypes.Message{llmtypes.SystemMsg(composeSystemPrompt), llmtypes.UserMsg(user)},
		nil, llmtypes.WithMaxTokens(16000))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ComposeResult{}, err
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return ComposeResult{}, fmt.Errorf("Nhà cung cấp AI phản hồi quá lâu. Kiểm tra kết nối ở Cài đặt › Nhà cung cấp rồi thử soạn lại: %w", err)
		}
		return ComposeResult{}, fmt.Errorf("Không kết nối được nhà cung cấp AI. Kiểm tra dịch vụ đang chạy và Cài đặt › Nhà cung cấp rồi thử soạn lại: %w", err)
	}
	raw := messageText(resp)
	parsed, err := parseComposeResponse(raw)
	if err != nil {
		return ComposeResult{}, fmt.Errorf("mô hình trả về bản đọc không đọc được: %w", err)
	}
	segs, err := finalizeSegments(parsed, in)
	if err != nil {
		return ComposeResult{}, err
	}
	segs = applyReadingPlan(segs, plan, in)
	return ComposeResult{Segments: segs, Model: c.ID}, nil
}

// buildComposeUserPrompt 拼用户消息：人物表 + 章节正文。
func buildComposeUserPrompt(in ComposeInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Truyện: chương %d", in.Chapter)
	if strings.TrimSpace(in.Title) != "" {
		fmt.Fprintf(&b, " — %s", in.Title)
	}
	b.WriteString("\n\nNgười nói được phép dùng:\n")
	b.WriteString("- dan = Dẫn truyện (mọi câu ngoài lời thoại)\n")
	for _, sp := range in.Characters {
		if sp.ID == NarratorID {
			continue
		}
		fmt.Fprintf(&b, "- %s = %s", sp.ID, sp.Name)
		if sp.Role != "" {
			fmt.Fprintf(&b, " (%s)", sp.Role)
		}
		if len(sp.Aliases) > 0 {
			fmt.Fprintf(&b, " — còn gọi: %s", strings.Join(sp.Aliases, ", "))
		}
		b.WriteByte('\n')
	}
	b.WriteString("\nChế độ giọng: ")
	if in.Mode == ModeSingle {
		b.WriteString("một giọng đọc tất cả — vẫn gán speaker chính xác để giữ sắc thái.\n")
	} else {
		b.WriteString("phân vai — mỗi nhân vật một giọng.\n")
	}
	b.WriteString("\nNội dung chương:\n")
	b.WriteString(in.Text)
	return b.String()
}

// messageText 取模型回复的正文（拼接所有 text block）。
func messageText(resp *llmtypes.LLMResponse) string {
	if resp == nil {
		return ""
	}
	var b strings.Builder
	for _, block := range resp.Message.Content {
		if block.Text != "" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// composeResponse 是模型的 JSON 形状。
type composeResponse struct {
	Segments []struct {
		Speaker string        `json:"speaker"`
		Text    string        `json:"text"`
		Tone    string        `json:"tone"`
		PauseMs int           `json:"pauseMs"`
		Guess   []SpeakerPick `json:"guess"`
		Source  string        `json:"source"`
	} `json:"segments"`
}

// parseComposeResponse 容忍 markdown 围栏与前后废话，只取 JSON 对象。
func parseComposeResponse(raw string) (composeResponse, error) {
	var out composeResponse
	text := strings.TrimSpace(raw)
	if i := strings.Index(text, "```"); i >= 0 {
		text = text[i:]
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		if j := strings.Index(text, "```"); j >= 0 {
			text = text[:j]
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return out, errors.New("không thấy JSON trong phản hồi")
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &out); err != nil {
		return out, err
	}
	if len(out.Segments) == 0 {
		return out, errors.New("bản đọc không có đoạn nào")
	}
	return out, nil
}

// finalizeSegments 做确定性后处理：编号、说话人解析、规范化、标志位。
func finalizeSegments(parsed composeResponse, in ComposeInput) ([]Segment, error) {
	byName := map[string]ComposeSpeaker{}
	byID := map[string]ComposeSpeaker{}
	for _, sp := range in.Characters {
		byID[strings.ToLower(sp.ID)] = sp
		byName[strings.ToLower(sp.Name)] = sp
		for _, a := range sp.Aliases {
			if a = strings.TrimSpace(a); a != "" {
				byName[strings.ToLower(a)] = sp
			}
		}
	}
	byID[NarratorID] = ComposeSpeaker{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator}

	segs := make([]Segment, 0, len(parsed.Segments))
	for _, raw := range parsed.Segments {
		src := strings.TrimSpace(stripMarkdown(raw.Text))
		if src == "" {
			continue
		}
		text, changes := Normalize(src, in.Normalize, in.Dict)
		if strings.TrimSpace(text) == "" {
			continue
		}
		seg := Segment{
			ID:         fmt.Sprintf("s%04d", len(segs)+1),
			Text:       text,
			Source:     src,
			Tone:       strings.TrimSpace(raw.Tone),
			PauseMs:    clampPause(raw.PauseMs),
			Normalized: changes,
		}
		if len(changes) == 0 {
			seg.Normalized = nil
		}
		// 说话人：先按 id，再按名字/别名；都不认识就保留原名并标 unknown。
		key := strings.ToLower(strings.TrimSpace(raw.Speaker))
		sp, ok := byID[key]
		if !ok {
			sp, ok = byName[key]
		}
		if ok {
			seg.Speaker = sp.ID
		} else if key == "" {
			seg.Speaker = NarratorID
		} else {
			seg.Speaker = strings.TrimSpace(raw.Speaker)
			seg.Flags = append(seg.Flags, FlagUnknownSpeaker)
			seg.SpeakerGuess = normalizeGuesses(raw.Guess)
		}
		if countWords(src) > LongSentenceWords {
			seg.Flags = append(seg.Flags, FlagLongSentence)
		}
		if words := englishWords(text); len(words) > 0 {
			seg.Flags = append(seg.Flags, FlagEnglish)
		}
		seg.Flags = sortFlags(seg.Flags)
		segs = append(segs, seg)
	}
	segs = foldUnspeakable(segs)
	if len(segs) == 0 {
		return nil, errors.New("bản đọc rỗng sau khi chuẩn hoá")
	}
	for i := range segs {
		segs[i].ID = fmt.Sprintf("s%04d", i+1)
	}
	return segs, nil
}

// clampPause 把模型给的停顿限制在 0–1500ms。
func clampPause(ms int) int {
	if ms < 0 {
		return 0
	}
	if ms > 1500 {
		return 1500
	}
	return ms
}

// normalizeGuesses 清洗"可能是谁"：丢掉空项、概率夹到 0–1、按概率降序、最多 3 条。
func normalizeGuesses(in []SpeakerPick) []SpeakerPick {
	out := make([]SpeakerPick, 0, len(in))
	for _, g := range in {
		who := strings.TrimSpace(g.Who)
		if who == "" {
			continue
		}
		p := g.P
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		out = append(out, SpeakerPick{Who: who, P: p})
	}
	if len(out) == 0 {
		return nil
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].P > out[j-1].P; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// mdEmphasis 匹配章节 Markdown 里的强调标记（*Tên phim*、**đậm**、_nghiêng_）。
// 强调只是排版：留着会进字幕、也可能被读出来。
var mdEmphasis = regexp.MustCompile(`(\*{1,3}|_{1,3})([^*_\n]+?)(\*{1,3}|_{1,3})`)

// stripMarkdown 去掉强调标记，保留文字。
func stripMarkdown(s string) string {
	return mdEmphasis.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdEmphasis.FindStringSubmatch(m)
		if sub[1] != sub[3] {
			return m
		}
		return sub[2]
	})
}
