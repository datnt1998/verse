package diag

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PlanActions 根据高置信 Finding 生成可执行动作。
// 只有 Confidence==high && AutoLevel==safe 的 Finding 才会产出 Action。
func PlanActions(findings []Finding) []Action {
	var actions []Action
	seen := make(map[string]struct{})

	for _, f := range findings {
		if f.Confidence != ConfHigh || f.AutoLevel != AutoSafe {
			continue
		}
		if _, ok := seen[f.Rule]; ok {
			continue
		}
		seen[f.Rule] = struct{}{}

		actions = append(actions, planRule(f)...)
	}
	return actions
}

func planRule(f Finding) []Action {
	key := FindingFingerprint(f)

	switch f.Rule {
	case "PhaseFlowMismatch":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEmitNotice, Severity: f.Severity, Summary: f.Title, Message: f.Title, Fingerprint: key},
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Rà lại tiến độ sáng tác", Message: f.Evidence + " Hãy rà lại giai đoạn sáng tác và công việc còn dở trước khi chạy tiếp.", Fingerprint: key},
		}
	case "OutlineExhausted":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Mở rộng đại cương để viết tiếp", Message: "Số chương đã hoàn thành đã chạm giới hạn đại cương. Hãy ưu tiên nhờ Kiến trúc sư mở rộng cung tiếp theo hoặc thêm tập mới, rồi mới viết tiếp.", Fingerprint: key},
		}
	case "OrphanedSteer":
		return []Action{
			{SourceRule: f.Rule, Kind: ActionEnqueueFollowUp, Severity: f.Severity, Summary: "Xử lý chỉ thị chuyển hướng đang chờ", Message: "Có chỉ thị chuyển hướng của bạn chưa được xử lý. Hãy áp dụng chỉ thị này trước rồi tiếp tục công việc hiện tại.", Fingerprint: key},
		}
	default:
		return nil
	}
}

// FindingFingerprint 是 Finding 的稳定指纹：同规则、同作用面、同标题与证据 → 同指纹。
// 12 位十六进制，可直接放进 URL（web POST …/diag/actions/:fingerprint）。
func FindingFingerprint(f Finding) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s", f.Rule, f.Target, f.Title, f.Evidence, f.Technical)))
	return hex.EncodeToString(sum[:6])
}
