// Package rules 实现用户偏好的输入层（Policy）：把各来源的写作规则归一化、合并成
// 本书快照（见 snapshot.go），运行时由 novel_context 注入、commit_chapter 机械检查。
//
// Rule 是第四类事实，跟 Progress / Checkpoint / Artifact 并列，但性质相反：
// 前三类是系统输出，Rule 是用户意图的持久化输入。
//
// 设计约束（不可妥协）：
//   - 工具只返事实，不返指令（Violation 是事实，由 editor 决定是否触发重写）
//   - 不引入新的 verdict 路径（复用 PendingRewrites）
//   - 不引入严格度字段（severity 由规则类型固定映射，editor 自主语义裁定）
//   - 不动 Flow Router（rule 不参与路由）
package rules

import (
	"encoding/json"
	"fmt"
	"math"
)

// SourceKind 标记规则文件来源，仅用于生成来源标签（如 global:my-style.md）。
type SourceKind int

const (
	// SourceGlobal — 用户全局偏好（~/.ainovel/rules/ 目录下所有 .md，按文件名字典序合并），跨书复用。
	SourceGlobal SourceKind = iota
	// SourceProject — 项目规则（<项目>/.ainovel/rules/，项目 = 书目录最近的含 project.json 的祖先目录）。
	SourceProject
	// SourceStory — 本书规则（<书目录>/.ainovel/rules/ 目录下所有 .md，按文件名字典序合并），优先级最高。
	SourceStory
)

// String 返回来源的可读名称，用于来源标签前缀。
func (k SourceKind) String() string {
	switch k {
	case SourceGlobal:
		return "global"
	case SourceProject:
		return "project"
	case SourceStory:
		return "story"
	default:
		return "unknown"
	}
}

// FatigueRefChapterTieng là độ dài chương chuẩn (1.500 tiếng) dùng quy đổi giữa
// ngưỡng cũ "N lần mỗi chương" (max_per_chapter) và ngưỡng mới "lần/1.000 tiếng"
// (per_1000): per_1000 = max_per_chapter / 1,5.
const FatigueRefChapterTieng = 1500

// MaxPerChapterToPer1000 quy đổi ngưỡng "N lần mỗi chương" (chương chuẩn
// FatigueRefChapterTieng tiếng) sang lần/1.000 tiếng, làm tròn 2 số lẻ.
func MaxPerChapterToPer1000(maxPerChapter float64) float64 {
	return math.Round(maxPerChapter*1000/FatigueRefChapterTieng*100) / 100
}

// FatiguePer1000 là ngưỡng lặp của một từ dễ lạm dụng: số lần cho phép trên mỗi
// 1.000 tiếng của chương (đo theo domain.WordCount — 1 từ trắng cách = 1 tiếng).
//
// Tương thích ngược trên dây chuyền (snapshot, norm cache, API): đọc được cả ba
// hình dạng —
//   - số trần {"một chút": 2}        → max_per_chapter kiểu cũ, quy đổi theo
//     FatigueRefChapterTieng (số lẻ trần hiểu là per_1000: ngưỡng mỗi chương luôn nguyên);
//   - object {"per_1000": 1.33, "max_per_chapter": 2} → per_1000 thắng khi cả hai có;
//     chỉ có max_per_chapter thì quy đổi như số trần.
//
// Ghi luôn ra object đủ hai khóa: per_1000 là sự thật, max_per_chapter là giá trị
// tương đương ở chương chuẩn 1.500 tiếng để khách cũ (nhãn "N lần/chương") vẫn hiển thị được.
type FatiguePer1000 float64

// Per1000 trả ngưỡng lần/1.000 tiếng.
func (f FatiguePer1000) Per1000() float64 { return float64(f) }

// MaxPerChapter trả ngưỡng tương đương ở chương chuẩn 1.500 tiếng: số lần nguyên
// lớn nhất không vi phạm (floor, tối thiểu 1 — checker luôn cho phép ít nhất 1 lần).
func (f FatiguePer1000) MaxPerChapter() int {
	n := int(float64(f) * FatigueRefChapterTieng / 1000)
	if n < 1 {
		return 1
	}
	return n
}

// MarshalJSON ghi dạng object đủ hai khóa (per_1000 + max_per_chapter suy ra).
func (f FatiguePer1000) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Per1000       float64 `json:"per_1000"`
		MaxPerChapter int     `json:"max_per_chapter"`
	}{Per1000: float64(f), MaxPerChapter: f.MaxPerChapter()})
}

// UnmarshalJSON đọc cả số trần (legacy max_per_chapter, hoặc per_1000 nếu lẻ) lẫn
// object hai khóa; per_1000 thắng khi cả hai cùng có mặt và dương.
func (f *FatiguePer1000) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case nil:
		*f = 0
	case float64:
		if x == math.Trunc(x) {
			*f = FatiguePer1000(MaxPerChapterToPer1000(x))
		} else {
			*f = FatiguePer1000(x)
		}
	case map[string]any:
		if p, ok := x["per_1000"].(float64); ok && p > 0 {
			*f = FatiguePer1000(p)
			return nil
		}
		if m, ok := x["max_per_chapter"].(float64); ok && m > 0 {
			*f = FatiguePer1000(MaxPerChapterToPer1000(m))
			return nil
		}
		*f = 0
	default:
		return fmt.Errorf("fatigue_words: giá trị phải là số hoặc object, got %T", v)
	}
	return nil
}

// Structured 装载机械可检的结构化规则字段（归一化各来源后的候选/合并结果）。
// 章节字数刻意不在此列：多长算一章是叙事完整性问题，属语义裁量（writer/editor），
// 数字化成机械硬线会诱导模型为跨线注水——字数意愿走 preferences 自然语言通道。
type Structured struct {
	Genre            string   `json:"genre,omitempty"`
	ForbiddenChars   []string `json:"forbidden_chars,omitempty"`
	ForbiddenPhrases []string `json:"forbidden_phrases,omitempty"`
	// fatigue_words 的值是“次/1.000 tiếng”（FatiguePer1000），JSON 读写兼容
	// 旧的 max_per_chapter 形态（见 FatiguePer1000 注释）。
	FatigueWords map[string]FatiguePer1000 `json:"fatigue_words,omitempty"`
	// Measurements được ghép với gói lúc kiểm tra; nil giữ nguyên hành vi cũ.
	Measurements *Measurements `json:"do_do,omitempty"`
}

// IsEmpty 用于判定是否完全没有结构化规则；checker 可据此跳过。
func (s Structured) IsEmpty() bool {
	return s.Genre == "" &&
		len(s.ForbiddenChars) == 0 &&
		len(s.ForbiddenPhrases) == 0 &&
		len(s.FatigueWords) == 0 && s.Measurements == nil
}

// Severity 标记 Violation 的严重等级。
// 固定映射（用户不可配置）：
//
//	forbidden_chars 出现             -> Error
//	forbidden_phrases 出现           -> Error
//	fatigue_words 超密度阈值           -> Warning
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Violation 是 checker 的输出：本章违反了某条机械规则的事实陈述。
//
// 注意：commit_chapter 把 violations 透传到返回 JSON，不阻断 commit；
// editor 在审阅时把这些事实映射到现有七维（aesthetic/pacing/character/consistency），
// 由 LLM 自主决定是否升级 verdict 触发 polish/rewrite。
type Violation struct {
	Rule     string   `json:"rule"`               // forbidden_chars / forbidden_phrases / fatigue_words
	Target   string   `json:"target,omitempty"`   // 具体违规对象（哪个词/字符）
	Limit    any      `json:"limit,omitempty"`    // 阈值；fatigue_words=本章允许次数(int) / forbidden_*=空
	Per1000  float64  `json:"per_1000,omitempty"` // fatigue_words: 密度阈值 lần/1.000 tiếng
	Actual   any      `json:"actual"`             // 实际值：出现次数
	Severity Severity `json:"severity"`           // error / warning
	Message  string   `json:"message,omitempty"`  // 越南语事实陈述（fatigue_words 记录密度）
}
