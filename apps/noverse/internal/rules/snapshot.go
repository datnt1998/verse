package rules

import (
	"fmt"
	"maps"
	"math"
	"strings"
)

// Snapshot 是本书归一化后的用户规则快照（meta/user_rules.json）。
//
// 它是运行时唯一事实源：开书/导入/刷新时由各来源归一化合并而成，之后 novel_context
// 注入与 commit_chapter 检查都只读这一份，不再反复读 rules 文件（避免漂移与双读者发散）。
//
// 注入给模型的只有 Structured + Preferences（见 Payload）；Version / Status / Sources /
// Uncertain 是运维与诊断元数据，不进 working_memory.user_rules。
type Snapshot struct {
	Version     int        `json:"version"`
	Status      Status     `json:"status"`
	Structured  Structured `json:"structured"`
	Preferences string     `json:"preferences"`
	Sources     []string   `json:"sources"`
	Uncertain   []string   `json:"uncertain"`
	// UserStructured ghi riêng ý định tường minh, không chứa system_defaults
	// hay giá trị gói; dùng để ghép gói sống mà không nhầm mặc định thành ghi đè.
	UserStructured *Structured `json:"user_structured,omitempty"`
	// FatigueSources lưu nguồn thắng theo từng từ, kể cả khi ngưỡng bằng mặc định.
	FatigueSources map[string]string `json:"fatigue_sources,omitempty"`
}

// Status 标记快照归一化是否完整成功。
type Status string

const (
	// StatusReady 所有来源都成功归一化。
	StatusReady Status = "ready"
	// StatusDegraded 至少一个来源归一化失败，已降级为 raw preferences（详见 Uncertain / 日志）。
	StatusDegraded Status = "degraded"
)

// SnapshotVersion 是当前快照 schema 版本，便于未来迁移。
// v2：chapter_words 退出 structured（字数是语义软约束，走 preferences）。
// v1 快照直接加载兼容：未知字段被反序列化忽略，下次叠加保存时自然收敛为 v2；
// 刻意不做"版本不符即重建"——那会丢掉 AddRuntimeRule 运行中追加的不可再生规则。
const SnapshotVersion = 2

// Candidate 是单个来源归一化后的候选结果。
//
// 来源按优先级低→高排列后交给 BuildSnapshot 确定性合并。LLM 只负责把单一来源的
// 自然语言变成候选 Structured/Preferences；优先级与字段覆盖由 BuildSnapshot（Go）裁定。
type Candidate struct {
	Source     string     `json:"source"`     // 可读来源标签，进入 Snapshot.Sources（如 system_defaults / startup_prompt / global:my.md）
	Structured Structured `json:"structured"` // 该来源候选结构化字段
	// FatigueSources phân biệt từ gói khai rõ với từ kế thừa từ system_defaults.
	FatigueSources map[string]string `json:"fatigue_sources,omitempty"`
	Preferences    string            `json:"preferences,omitempty"` // 该来源的自然语言偏好正文
	Uncertain      []string          `json:"uncertain,omitempty"`   // 该来源故意未提升到 structured 的项 + 原因（诊断）
	// UncertainNote 给用户看的一行“还没把握懂对”（Câu 26a）；空 = 模型已确定。
	UncertainNote string   `json:"uncertain_note,omitempty"`
	UncertainFlag bool     `json:"is_uncertain,omitempty"`
	Options       []string `json:"options,omitempty"`  // 仅在 Uncertain 非空时：0–3 条消歧后的改写规则（界面“cách hiểu”按钮），不进快照
	Degraded      bool     `json:"degraded,omitempty"` // 该来源归一化失败、已降级为 raw preferences
}

// Payload 返回注入 working_memory.user_rules 的形态：只暴露 structured + preferences。
// 即便都为空也返回稳定结构，避免 LLM 看到 user_rules=null 走异常分支。
func (s Snapshot) Payload() map[string]any {
	return map[string]any{
		"structured":  s.Structured,
		"preferences": s.Preferences,
	}
}

// BuildSnapshot 把按优先级（低→高）排好的候选确定性合并成快照。
//
// 合并规则（全部 Go 侧确定性，不交给 LLM）：
//   - genre：高优先级来源覆盖低优先级
//   - forbidden_chars / forbidden_phrases：取并集并去重（保留首次出现的顺序）。
//     一个来源新增一个禁用词不得抹掉低优先级来源（如 system_defaults 的内置套话）
//   - fatigue_words：按词叠加，同词时高优先级（近层）的阈值胜出
//   - preferences：不覆盖，按来源顺序拼接（高优先级在后），带来源标题
//   - Giá trị rỗng bị bỏ qua; ngưỡng fatigue bằng 0 là một giới hạn tường minh.
//   - 任一来源 Degraded → 快照 status=degraded
func BuildSnapshot(cands []Candidate) Snapshot {
	snap := Snapshot{
		Version: SnapshotVersion,
		Status:  StatusReady,
		Sources: make([]string, 0, len(cands)),
		// Luôn là mảng (không null) trong JSON gửi cho giao diện.
		Uncertain:      []string{},
		UserStructured: &Structured{},
		FatigueSources: make(map[string]string),
	}
	var prefs []string
	for _, c := range cands {
		structured := sanitizeStructured(c.Structured)
		snap.Structured = mergeStructured(snap.Structured, structured)
		for word := range structured.FatigueWords {
			snap.FatigueSources[word] = fatigueCandidateSource(c, word)
		}
		if c.Source != "system_defaults" {
			user := mergeStructured(*snap.UserStructured, structured)
			snap.UserStructured = &user
		}

		if p := strings.TrimSpace(c.Preferences); p != "" {
			if src := strings.TrimSpace(c.Source); src != "" {
				prefs = append(prefs, fmt.Sprintf("## [%s]\n\n%s", src, p))
			} else {
				prefs = append(prefs, p)
			}
		}
		if src := strings.TrimSpace(c.Source); src != "" {
			snap.Sources = append(snap.Sources, src)
		}
		snap.Uncertain = append(snap.Uncertain, c.Uncertain...)
		if c.Degraded {
			snap.Status = StatusDegraded
		}
	}
	snap.Preferences = strings.Join(prefs, "\n\n")
	return snap
}

// OverlaySnapshot 把一个高优先级候选叠加到已有快照上（候选胜出）。
//
// 用于运行中 Arbiter rules 动作：不重新归一化所有来源，只把新规则叠加进当前快照——
// structured 合并规则同 BuildSnapshot（禁用词取并集）、preferences 追加一段、
// sources/uncertain 累加、降级传播。
func OverlaySnapshot(base Snapshot, cand Candidate) Snapshot {
	out := base
	out.Version = SnapshotVersion
	structured := sanitizeStructured(cand.Structured)
	out.Structured = mergeStructured(out.Structured, structured)
	out.FatigueSources = make(map[string]string, len(base.FatigueSources)+len(cand.Structured.FatigueWords))
	maps.Copy(out.FatigueSources, base.FatigueSources)
	for word := range structured.FatigueWords {
		out.FatigueSources[word] = fatigueCandidateSource(cand, word)
	}
	user := explicitStructured(base)
	if cand.Source != "system_defaults" {
		user = mergeStructured(user, structured)
	}
	out.UserStructured = &user
	if p := strings.TrimSpace(cand.Preferences); p != "" {
		section := p
		if src := strings.TrimSpace(cand.Source); src != "" {
			section = fmt.Sprintf("## [%s]\n\n%s", src, p)
		}
		if strings.TrimSpace(out.Preferences) == "" {
			out.Preferences = section
		} else {
			out.Preferences = out.Preferences + "\n\n" + section
		}
	}
	if src := strings.TrimSpace(cand.Source); src != "" {
		out.Sources = append(append([]string{}, out.Sources...), src)
	}
	if len(cand.Uncertain) > 0 {
		out.Uncertain = append(append([]string{}, out.Uncertain...), cand.Uncertain...)
	}
	if cand.Degraded {
		out.Status = StatusDegraded
	}
	return out
}

// mergeStructured 把已净化的 src 叠到 dst 上：genre 近层覆盖，两个禁用清单取并集去重，
// fatigue_words 按词近层胜出。返回值不与 dst 共享底层数组/映射，调用方可放心持有旧快照。
func mergeStructured(dst, src Structured) Structured {
	out := dst
	if src.Genre != "" {
		out.Genre = src.Genre
	}
	out.ForbiddenChars = unionStrings(dst.ForbiddenChars, src.ForbiddenChars)
	out.ForbiddenPhrases = unionStrings(dst.ForbiddenPhrases, src.ForbiddenPhrases)
	out.FatigueWords = mergeFatigueWords(cloneFatigue(dst.FatigueWords), src.FatigueWords)
	measurementSource := src.Measurements
	if measurementSource == nil {
		measurementSource = dst.Measurements
	}
	if measurementSource != nil {
		measurements := *measurementSource
		measurements.EnglishWords = append([]string(nil), measurementSource.EnglishWords...)
		out.Measurements = &measurements
	}
	return out
}

// unionStrings 返回 a 后接 b 中不重复项的新切片（保持首次出现的顺序）；两者皆空返回 nil。
func unionStrings(a, b []string) []string {
	if len(a)+len(b) == 0 {
		return nil
	}
	out := make([]string, 0, len(a)+len(b))
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, s := range list {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// mergeFatigueWords 按词叠加疲劳词密度阈值，src 覆盖 dst 中的同词阈值（就近优先）。
// 让用户只需新增少量疲劳词，而不必重列内置基线。
func mergeFatigueWords(dst, src map[string]FatiguePer1000) map[string]FatiguePer1000 {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]FatiguePer1000, len(src))
	}
	maps.Copy(dst, src)
	return dst
}

func cloneFatigue(m map[string]FatiguePer1000) map[string]FatiguePer1000 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]FatiguePer1000, len(m))
	maps.Copy(out, m)
	return out
}

// SystemDefaults 是代码内置的机械基线（最低优先级来源），不走 LLM 归一化。
//
// 疲劳词阈值单位是 次/1.000 tiếng（FatiguePer1000），由旧版“N 次/章”按章长基准
// FatigueRefChapterTieng = 1.500 tiếng 换算（per_1000 = N/1.5，两位小数）：
// 像一/沉默了/没有说话/X息 这类后段节拍词来自 196 章长跑产物实证——传统 AI 套话
// 被前段表灭绝后，模型转而把它们用到章均 5-7 次，阈值放宽以容忍正常使用。
//
// fatigueOverrides 是 gói thể loại 的覆盖点（goi.json → fatigue_words，word→per_1000）：
// 非空时按词盖在内置表上；不传则行为与旧版一致。
func SystemDefaults(fatigueOverrides ...map[string]float64) Candidate {
	fatigue := map[string]FatiguePer1000{
		// 旧值（次/章）→ 新值（次/1.000 tiếng）：N÷1.5
		"không khỏi": 0.67 /*1*/, "bỗng nhiên": 0.67 /*1*/, "dường như": 1.33, /*2*/
		"chẳng biết từ đâu": 0.67 /*1*/, "tuy nhiên": 1.33 /*2*/, "một chút": 1.33, /*2*/
		"một vệt": 1.33 /*2*/, "một sợi": 1.33 /*2*/, "tựa như": 0.67, /*1*/
		"không kìm được": 0.67 /*1*/, "như một": 2 /*3*/, "im lặng": 1.33, /*2*/
		"không nói gì": 1.33 /*2*/, "một thoáng": 2 /*3*/, "chốc lát": 2, /*3*/
		"trong chớp mắt": 1.33, /*2*/
	}
	for _, ov := range fatigueOverrides {
		for w, p := range ov {
			if w = strings.TrimSpace(w); w != "" && p >= 0 && !math.IsNaN(p) && !math.IsInf(p, 0) {
				fatigue[w] = FatiguePer1000(p)
			}
		}
	}
	return Candidate{
		Source: "system_defaults",
		Structured: Structured{
			// 定长固定串的 AI 套句；checker 字面子串匹配，带变量的模式（不是X而是Y）归语义层。
			// 词表直接取自 assets/references/anti-ai-tone.md 的越南语条目，避免两套约定。
			ForbiddenPhrases: []string{"theo một nghĩa nào đó", "đáng chú ý là", "không rõ vì sao", "không nói thành lời"},
			FatigueWords:     fatigue,
		},
	}
}

// sanitizeStructured bỏ các trường rỗng, nhưng giữ ngưỡng fatigue bằng 0
// vì người dùng có thể siết một từ xuống không lần mỗi chương.
func sanitizeStructured(s Structured) Structured {
	out := Structured{}
	if g := strings.TrimSpace(s.Genre); g != "" {
		out.Genre = g
	}
	out.ForbiddenChars = nonEmptyStrings(s.ForbiddenChars)
	out.ForbiddenPhrases = nonEmptyStrings(s.ForbiddenPhrases)
	out.FatigueWords = sanitizeFatigueWords(s.FatigueWords)
	out.Measurements = s.Measurements
	return out
}

func nonEmptyStrings(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func sanitizeFatigueWords(m map[string]FatiguePer1000) map[string]FatiguePer1000 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]FatiguePer1000, len(m))
	for w, n := range m {
		if w = strings.TrimSpace(w); w == "" || n < 0 || math.IsNaN(float64(n)) || math.IsInf(float64(n), 0) {
			continue
		}
		out[w] = n
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
