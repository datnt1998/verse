package diag

import (
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// HookTypeChain 检查 Master §7：同类章末钩子不能连续超过两章。
func HookTypeChain(snap *Snapshot) []Finding {
	var findings []Finding
	last, run := "", 0
	for i, entry := range snap.Outline {
		hook := strings.ToLower(strings.TrimSpace(entry.HookType))
		if hook == "" {
			last, run = "", 0
			continue
		}
		if hook == last {
			run++
		} else {
			last, run = hook, 1
		}
		if run == 3 {
			finding := retentionFinding("HookTypeChain", "Kiểu móc lặp quá hai chương liên tiếp (§7)", fmt.Sprintf("Chương %d–%d dùng cùng một kiểu móc cuối chương.", i-1, i+1), "Yêu cầu Kiến trúc sư đổi kiểu móc cuối chương trong đại cương, thay vì chỉ đổi câu chữ.")
			finding.Technical = "hook_type=" + hook
			findings = append(findings, finding)
		}
	}
	return findings
}

// retentionLength 不把尚未展开的长篇大纲冒充全书总长。
func retentionLength(snap *Snapshot) int {
	for _, volume := range snap.Volumes {
		for _, arc := range volume.Arcs {
			if !arc.IsExpanded() {
				return 0
			}
		}
	}
	return len(snap.Outline)
}

// MidpointTwistPosition 章节终点的序位/章节总数近似时长，区间含两端。
func MidpointTwistPosition(snap *Snapshot) []Finding {
	n := retentionLength(snap)
	if n == 0 {
		return nil
	}
	present := false
	for i, entry := range snap.Outline {
		present = present || entry.MidpointTwist || entry.HookType != "" || entry.Promise != "" || entry.OpenQuestions != nil
		if entry.MidpointTwist && (i+1)*100 >= 45*n && (i+1)*100 <= 60*n {
			return nil
		}
	}
	if !present {
		return nil
	}
	finding := retentionFinding("MidpointTwistPosition", "Thiếu cú lật ở giữa truyện (§8)", fmt.Sprintf("Đại cương có %d chương nhưng chưa có cú lật trong vùng 45–60%% của truyện.", n), "Yêu cầu Kiến trúc sư bố trí một cú lật trong vùng 45–60% đại cương để thay đổi thế cục ở giữa truyện.")
	finding.ChapterFrom, finding.ChapterTo = (45*n+99)/100, 60*n/100
	return []Finding{finding}
}

// PromiseScenePosition 只认 da_tra: + 场景描述；取首次兑现标题/开场承诺的章节。
func PromiseScenePosition(snap *Snapshot) []Finding {
	n := retentionLength(snap)
	if n == 0 {
		return nil
	}
	present, first := false, 0
	for i, entry := range snap.Outline {
		present = present || strings.TrimSpace(entry.Promise) != ""
		if domain.PromiseFulfilled(entry.Promise) && first == 0 {
			first = i + 1
		}
	}
	if !present || (first > 0 && first*100 >= 35*n && first*100 <= 60*n) {
		return nil
	}
	evidence := fmt.Sprintf("Đại cương có %d chương nhưng chưa đánh dấu cảnh thực hiện lời hứa ở tiêu đề hoặc mở đầu.", n)
	if first > 0 {
		evidence = fmt.Sprintf("Cảnh thực hiện lời hứa đầu tiên nằm ở chương %d/%d, ngoài vùng 35–60%% của truyện.", first, n)
	}
	return []Finding{retentionFinding("PromiseScenePosition", "Cảnh thực hiện lời hứa chưa ở đúng vị trí (§3)", evidence, "Yêu cầu Kiến trúc sư đặt cảnh thực hiện lời hứa ở tiêu đề hoặc mở đầu vào vùng 35–60% đại cương, rồi mở ra một lời hứa lớn hơn sau cảnh này.")}
}

func retentionFinding(rule, title, evidence, suggestion string) Finding {
	return Finding{
		Rule: rule, Category: CatPlanning, Severity: SevWarning, Confidence: ConfHigh,
		AutoLevel: AutoNone, Target: "prompt.architect", Title: title,
		Evidence: evidence, Suggestion: suggestion,
	}
}

// RetentionMap là bản đồ giữ chân đọc thẳng từ đại cương cho màn D5
// (GET …/retention): vị trí cảnh lời hứa và cú lật, chương thứ ba của mỗi chuỗi
// móc lặp, và các chương có số câu hỏi treo ngoài 2–4. Hàm chỉ đọc đại cương,
// không ghi gì; các RuleFunc ở trên giữ nguyên hành vi cũ (Finding như trước).
type RetentionMap struct {
	// Total là mẫu số của mọi vị trí (1-based): số chương đã mở hết cung khi
	// đại cương phân tầng đã mở rộng, ngược lại là số dòng đại cương hiện có.
	Total int
	// PromiseChapter / TwistChapter là chương 1-based đầu tiên có cảnh lời hứa
	// (da_tra:) / cú lật; 0 nghĩa là chưa đánh dấu.
	PromiseChapter int
	TwistChapter   int
	HookChains     []HookChain
	// FewQuestions / ManyQuestions là các chương (trừ chương cuối) có dưới 2 /
	// trên 4 câu hỏi đang treo. Rỗng khi đại cương chưa khai open_questions.
	FewQuestions  []int
	ManyQuestions []int
	// PromiseDeclared / TwistDeclared / HooksDeclared / QuestionsDeclared cho
	// biết trường tương ứng có xuất hiện ở bất kỳ chương nào (đại cương cũ: false).
	PromiseDeclared   bool
	TwistDeclared     bool
	HooksDeclared     bool
	QuestionsDeclared bool
}

// HookChain là một chuỗi ≥3 chương liền nhau cùng kiểu móc cuối chương (§7).
type HookChain struct {
	Type     string // kiểu móc đã chuẩn hoá (TrimSpace + lowercase) như HookTypeChain
	Chapters []int  // ba chương đầu của chuỗi; chương thứ ba là chương bị báo
}

// BuildRetentionMap tính bản đồ giữ chân từ đại cương của snapshot.
func BuildRetentionMap(snap *Snapshot) RetentionMap {
	if snap == nil {
		return RetentionMap{}
	}
	n := retentionLength(snap)
	if n == 0 {
		n = len(snap.Outline)
	}
	out := RetentionMap{Total: n}
	last, run := "", 0
	for i, entry := range snap.Outline {
		chapter := i + 1
		out.PromiseDeclared = out.PromiseDeclared || strings.TrimSpace(entry.Promise) != ""
		out.TwistDeclared = out.TwistDeclared || entry.MidpointTwist
		out.HooksDeclared = out.HooksDeclared || strings.TrimSpace(entry.HookType) != ""
		out.QuestionsDeclared = out.QuestionsDeclared || entry.OpenQuestions != nil
		if out.PromiseChapter == 0 && domain.PromiseFulfilled(entry.Promise) {
			out.PromiseChapter = chapter
		}
		if out.TwistChapter == 0 && entry.MidpointTwist {
			out.TwistChapter = chapter
		}
		hook := strings.ToLower(strings.TrimSpace(entry.HookType))
		if hook == "" {
			last, run = "", 0
		} else if hook == last {
			run++
		} else {
			last, run = hook, 1
		}
		if run == 3 {
			out.HookChains = append(out.HookChains, HookChain{Type: hook, Chapters: []int{chapter - 2, chapter - 1, chapter}})
		}
		if entry.OpenQuestions != nil && chapter < n {
			switch count := len(entry.OpenQuestions); {
			case count < 2:
				out.FewQuestions = append(out.FewQuestions, chapter)
			case count > 4:
				out.ManyQuestions = append(out.ManyQuestions, chapter)
			}
		}
	}
	return out
}
