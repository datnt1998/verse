// Package userrules 是用户规则归一化的服务层：把各来源的自然语言规则经 LLM 结构化调用
// 归一化成候选结构化字段，再由 rules.BuildSnapshot 确定性合并成本书快照。
//
// 分层职责：
//   - rules 包：纯数据 + 确定性合并（Snapshot / Candidate / BuildSnapshot / SystemDefaults）
//   - 本包：LLM 归一化 + 编排 + 落盘（依赖 agentcore + store + rules）
//
// 归一化是增强路径，不是主创作的前置条件：任何来源失败都降级为 raw preferences，主创作必须继续。
package userrules

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// normalizeMaxTokens 单次归一化的输出上限（思考 token 与 JSON 输出共享这一预算）。
// 归一化 JSON 本身很小（通常 <1k），这里留大头是给"无法关闭思考的推理模型"的思考预算——
// 留窄了思考会挤占 JSON 导致截断、解析失败。max_tokens 是上限不是计费量，调大不增成本。
const normalizeMaxTokens = 8192

// normalizeContract 紧邻边界 DTO：全字段 required、fatigue_words 用对象数组
// （strict 模式禁止动态 key 的 map），两种模式共用同一 DTO 约定。
var normalizeContract = llmcontract.Contract{
	Name:        "userrules_normalize",
	Description: "Quy luật viết bằng ngôn ngữ tự nhiên của người dùng thành các trường có cấu trúc",
	Schema: schema.Object(
		schema.Property("structured", schema.Object(
			schema.Property("genre", schema.String("Thể loại; không có thì chuỗi rỗng")).Required(),
			schema.Property("forbidden_chars", schema.Array("Ký tự bị cấm xuất hiện", schema.String("Ký tự"))).Required(),
			schema.Property("forbidden_phrases", schema.Array("Cụm từ bị cấm (khớp nguyên văn)", schema.String("Cụm từ"))).Required(),
			schema.Property("fatigue_words", schema.Array("Từ lặp và mật độ tối đa", schema.Object(
				schema.Property("word", schema.String("Từ lặp")).Required(),
				schema.Property("per_1000", llmcontract.Nullable(schema.Number("Số lần tối đa trên mỗi 1.000 tiếng (số dương); ngưỡng chính, ưu tiên hơn max_per_chapter"))).Required(),
				schema.Property("max_per_chapter", llmcontract.Nullable(schema.Int("Số lần tối đa mỗi chương kiểu cũ (nguyên dương); chỉ điền khi người dùng nói rõ theo chương"))).Required(),
			))).Required(),
		)).Required(),
		schema.Property("preferences", schema.String("Sở thích văn phong/nhân vật/thẩm mỹ bằng ngôn ngữ tự nhiên; không có thì chuỗi rỗng")).Required(),
		schema.Property("uncertain", schema.Array("Mục cố ý không nâng lên structured kèm lý do", schema.String("Mục"))).Required(),
		schema.Property("is_uncertain", llmcontract.Nullable(schema.Bool("Noverse chưa chắc đã hiểu đúng quy tắc này; false khi đã hiểu chắc, null khi không có đánh giá"))).Required(),
		schema.Property("uncertain_note", llmcontract.Nullable(schema.String("Một câu ngắn cho người dùng biết Noverse chưa chắc điều gì ở quy tắc này; null khi đã hiểu chắc"))).Required(),
		schema.Property("options", schema.Array("0–3 cách hiểu: mỗi mục là một quy tắc đã viết lại rõ ràng; mảng rỗng khi uncertain rỗng", schema.String("Quy tắc viết lại"))).Required(),
	),
}

// Normalizer 把单个来源的自然语言规则归一化成 rules.Candidate。
type Normalizer struct {
	model llmtypes.ChatModel
}

// NewNormalizer 用一个 ChatModel 构造归一化器。归一化是一次性启动工具，
// 应传入能力较强的模型（如 ModelSet 的默认模型），不必跟随写作的弱模型。
//
// 归一化不覆盖 thinking：显式 off 本身也是只有部分模型支持的推理参数，
// 普通 chat 模型会拒绝它。沿用 provider/model 默认，由 normalizeMaxTokens
// 为不可关闭思考的模型预留输出预算。
func NewNormalizer(model llmtypes.ChatModel) *Normalizer {
	return &Normalizer{model: model}
}

// Normalize 归一化一个来源。失败返回 error（含真实原因），由调用方决定降级
// （normBatch.normalize 落 degraded 候选）——技术错误不再伪装成正常结果，
// 终止错误（鉴权/权限等）不重试。
func (n *Normalizer) Normalize(ctx context.Context, source, text string) (rules.Candidate, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return rules.Candidate{Source: source}, nil
	}
	if n == nil || n.model == nil {
		return rules.Candidate{}, fmt.Errorf("Model chuẩn hoá chưa được cấu hình")
	}

	out, err := llmcontract.Execute(ctx, n.model, llmcontract.Request[normalizerOutput]{
		Contract:     normalizeContract,
		SystemPrompt: normalizerSystemPrompt,
		Payload:      text,
		Options:      []llmtypes.CallOption{llmtypes.WithMaxTokens(normalizeMaxTokens)},
		Validate: func(out *normalizerOutput) error {
			_, err := out.toCandidate(source)
			return err
		},
		Agent: "rules",
		Hooks: llmcontract.Hooks{
			Resolved: func(res llmcontract.Resolution) {
				slog.Debug("Chọn giao thức chuẩn hoá luật", "module", "rules", "source", source,
					"contract", normalizeContract.Name, "structured_mode", res.Mode,
					"capability_source", res.Source, "provider", res.Provider, "model", res.Model,
					"schema_fingerprint", normalizeContract.Fingerprint())
			},
			Correction: func(ev llmcontract.Correction) {
				slog.Warn("Tự sửa đầu ra chuẩn hoá luật", "module", "rules", "source", source,
					"attempt", ev.Attempt, "layer", ev.Layer, "structured_mode", ev.Mode, "err", ev.Err)
			},
		},
	})
	if err != nil {
		return rules.Candidate{}, fmt.Errorf("Chuẩn hoá thất bại: %w", err)
	}
	return out.toCandidate(source)
}

// degraded 构造一个降级候选：归一化失败时把原文当作风格偏好，不提炼任何机械规则。
// uncertain 标注来源（便于回显"哪些来源未能解析"），但不含技术错误细节——技术错误只进日志。
func degraded(source, text string) rules.Candidate {
	return rules.Candidate{
		Source:      source,
		Preferences: text,
		Uncertain:   []string{source + ": chuẩn hoá thất bại, đã dùng nguyên văn làm sở thích văn phong (không rút ra luật cơ học)"},
		Degraded:    true,
	}
}

// normalizerOutput 是归一化器约定的边界 DTO（两种模式共用）：uncertain 固定
// 字符串数组，fatigue_words 固定对象数组——形态由契约钉死，不再多形态猜测。
type normalizerOutput struct {
	Structured    normalizerStructured `json:"structured"`
	Preferences   string               `json:"preferences"`
	Uncertain     []string             `json:"uncertain"`
	IsUncertain   *bool                `json:"is_uncertain"`
	UncertainNote *string              `json:"uncertain_note"`
	Options       []string             `json:"options"`
}

type normalizerStructured struct {
	Genre            string             `json:"genre"`
	ForbiddenChars   []string           `json:"forbidden_chars"`
	ForbiddenPhrases []string           `json:"forbidden_phrases"`
	FatigueWords     []fatigueWordEntry `json:"fatigue_words"`
}

type fatigueWordEntry struct {
	Word          string   `json:"word"`
	Per1000       *float64 `json:"per_1000"`
	MaxPerChapter *int     `json:"max_per_chapter"`
}

// toCandidate 校验边界 DTO 并转成领域候选：fatigue 条目须词非空且至少有一个正的
// 阈值——per_1000（lần/1.000 tiếng）优先，没有则拿 max_per_chapter 按章长基准
// rules.FatigueRefChapterTieng 换算（校验错误可反馈给模型修正）。
func (o normalizerOutput) toCandidate(source string) (rules.Candidate, error) {
	var fatigue map[string]rules.FatiguePer1000
	for _, e := range o.Structured.FatigueWords {
		word := strings.TrimSpace(e.Word)
		if word == "" {
			return rules.Candidate{}, fmt.Errorf("fatigue_words có mục từ rỗng")
		}
		var limit rules.FatiguePer1000
		switch {
		case e.Per1000 != nil && *e.Per1000 > 0:
			limit = rules.FatiguePer1000(*e.Per1000)
		case e.MaxPerChapter != nil && *e.MaxPerChapter > 0:
			limit = rules.FatiguePer1000(rules.MaxPerChapterToPer1000(float64(*e.MaxPerChapter)))
		case e.Per1000 != nil || e.MaxPerChapter != nil:
			return rules.Candidate{}, fmt.Errorf("fatigue_words[%q]: ngưỡng phải là số dương", word)
		default:
			return rules.Candidate{}, fmt.Errorf("fatigue_words[%q]: cần per_1000 hoặc max_per_chapter", word)
		}
		if fatigue == nil {
			fatigue = make(map[string]rules.FatiguePer1000, len(o.Structured.FatigueWords))
		}
		fatigue[word] = limit
	}
	uncertain := nonEmpty(o.Uncertain)
	note := ""
	if o.UncertainNote != nil {
		note = strings.TrimSpace(*o.UncertainNote)
	}
	// Ghi chú "chưa chắc" mà không kèm mục nào thì chính nó là lý do: quy tắc vẫn
	// được đánh dấu chưa chắc để giao diện hiện khung cam (Câu 26a).
	if note != "" && len(uncertain) == 0 {
		uncertain = []string{note}
	}
	return rules.Candidate{
		Source: source,
		Structured: rules.Structured{
			Genre:            strings.TrimSpace(o.Structured.Genre),
			ForbiddenChars:   nonEmpty(o.Structured.ForbiddenChars),
			ForbiddenPhrases: nonEmpty(o.Structured.ForbiddenPhrases),
			FatigueWords:     fatigue,
		},
		Preferences:   strings.TrimSpace(o.Preferences),
		Uncertain:     uncertain,
		UncertainFlag: o.IsUncertain != nil && *o.IsUncertain,
		UncertainNote: note,
		Options:       cleanOptions(o.Options, uncertain),
	}, nil
}

// maxOptions là số cách hiểu tối đa hiện cho người dùng (20-settings-rules §3.1).
const maxOptions = 3

// cleanOptions chỉ giữ options khi có uncertain (không có điều chưa chắc thì không có gì để chọn),
// bỏ mục rỗng/trùng và cắt còn tối đa maxOptions. Model nhả thừa là lỗi nhỏ nên cắt chứ không bắt sửa.
func cleanOptions(options, uncertain []string) []string {
	if len(uncertain) == 0 {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, o := range nonEmpty(options) {
		key := collapseSpace(o)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, o)
		if len(out) == maxOptions {
			break
		}
	}
	return out
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizerSystemPrompt 只描述归一化语义，输出结构由 normalizeContract 单点维护。
// 已用 10 条真实例子（含阈值发明陷阱）验证保守提升成立（10/10）。
const normalizerSystemPrompt = `Bạn là "Bộ chuẩn hoá luật" của hệ thống viết tiểu thuyết bằng AI. Bạn đọc luật viết dài hạn (ngôn ngữ tự nhiên) của người dùng từ MỘT nguồn, nâng những luật rõ ràng và kiểm tra được bằng máy lên structured, phần còn lại đưa vào preferences hoặc uncertain.

[Nâng thận trọng — quan trọng nhất]
- Chỉ ghi vào structured khi người dùng nói rõ ràng, không mơ hồ.
- forbidden_chars/forbidden_phrases là mức error: chỉ nâng những lệnh cấm rõ ràng kiểu "đừng để xuất hiện X / cấm dùng X / chớ viết X".
- fatigue_words: chỉ nâng khi có đồng thời "từ rõ ràng" và "ngưỡng số lần rõ ràng"; kiểu "dùng ít X / đừng lúc nào cũng dùng X" mà không có con số thì đưa vào preferences, tuyệt đối không tự bịa ngưỡng. Ngưỡng chính là per_1000 = số lần tối đa trên mỗi 1.000 tiếng; người dùng nói theo chương thì quy đổi với chương chuẩn 1.500 tiếng (per_1000 = số lần / 1,5) hoặc điền max_per_chapter kèm theo.
- Mọi mong muốn về số chữ/độ dài ("mỗi chương 3000 chữ", "ngắn lại một chút") đều đưa vào preferences: độ dài chương là vấn đề nhịp tự sự, do lúc sáng tác tự nắm, không kiểm tra bằng máy.
- Những gì không kiểm tra được bằng máy, không có ngưỡng rõ ràng, phụ thuộc ngữ cảnh, đều đưa vào preferences.
- Nguyên tắc: thà để lọt vào preferences còn hơn nâng sai (nâng sai sẽ báo lỗi sai ở mọi chương).

preferences giữ sở thích văn phong, nhân vật và thẩm mỹ bằng một đoạn ngôn ngữ tự nhiên dễ đọc.
uncertain ghi rõ những mục bạn cố ý không nâng lên structured và lý do.

[options — các cách hiểu để người dùng chọn]
- Chỉ điền khi uncertain không rỗng, và chỉ cho đúng điều mơ hồ đó (ví dụ "quá dài" không nói bao nhiêu). uncertain rỗng thì options là mảng rỗng.
- Tối đa 3 mục, mỗi mục là MỘT quy tắc đã viết lại bằng tiếng Việt, rõ ràng, tự đứng được (không cần câu gốc để hiểu), cùng giọng với người dùng.
- Mỗi mục phải là một cách hiểu khác nhau và hợp lý; không bịa con số mà người dùng không gợi ý — đưa ra các mức hợp lý hoặc cách diễn đạt tương đối.
- Ví dụ: "chương đừng quá dài" → ["Khoảng 3.000 chữ mỗi chương", "Ngắn hơn chương trước một chút"].`
