package diag

import (
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// 运行时检测阈值。
const (
	repeatCritical = 8 // 近端重复达到此次数升为 critical
	streamIdleWarn = 3 // stream_idle 累计告警阈值
)

// RuntimeRuleFunc 是运行时诊断规则的统一签名（对应创作侧的 RuleFunc）。
// 入参是脱敏聚合后的 RuntimeCapture，产出报告型 Finding——全部 AutoNone，
// 只诊断、不产 Action（观察者纪律，见 architecture.md §2.3）。
type RuntimeRuleFunc func(rc *RuntimeCapture) []Finding

var runtimeRules = []RuntimeRuleFunc{
	repeatedErrors,
	stuckStep,
	streamIdleStorm,
}

// runtimeFindings 跑全部运行时规则。
func runtimeFindings(rc *RuntimeCapture) []Finding {
	var out []Finding
	for _, rule := range runtimeRules {
		out = append(out, rule(rc)...)
	}
	return out
}

// Diagnose 是 /diag 的完整诊断入口：创作诊断 + 运行时信号 + 运行时检测，
// 返回合并后的 Report 与原始 RuntimeCapture（供导出复用，避免重复抓取）。
// 运行时 Finding 仅并入 Findings 供展示，不改 Actions——保持纯观察。
func Diagnose(s *store.Store) (Report, RuntimeCapture) {
	rep := Analyze(s)
	rc := CaptureRuntime(s)
	rep.Findings = append(rep.Findings, runtimeFindings(&rc)...)
	sortFindings(rep.Findings)
	return rep, rc
}

// repeatedErrors 只把"近端反复出现的错误 / 参数无效"判成 Finding。
// 不碰普通工具重复——subagent/novel_context/read_chapter 等在长跑里天然
// 高频，累计次数不是循环信号；真正的"反复而不推进"由 stuckStep 兜住。
func repeatedErrors(rc *RuntimeCapture) []Finding {
	var out []Finding
	for _, r := range rc.Repeats {
		var rule, title, sugg string
		switch {
		case strings.Contains(r.Sig, " · err: "):
			rule = "RepeatedToolError"
			title = "Trợ lý gặp cùng một lỗi nhiều lần"
			sugg = "Tạm dừng sáng tác, kiểm tra thông báo lỗi trong nhật ký rồi yêu cầu trợ lý xử lý nguyên nhân trước khi thử lại."
		case strings.Contains(r.Sig, "(args invalid)"):
			rule = "ArgsInvalidLoop"
			title = "Trợ lý liên tục gửi yêu cầu không hợp lệ"
			sugg = "Tạm dừng và yêu cầu trợ lý thực hiện lại công việc với yêu cầu rõ ràng hơn. Nếu vẫn lặp lỗi, hãy xuất chẩn đoán ẩn danh để gửi hỗ trợ."
		default:
			continue // 普通工具重复不产 Finding
		}
		sev := SevWarning
		if r.Count >= repeatCritical {
			sev = SevCritical
		}
		out = append(out, Finding{
			Rule:       rule,
			Category:   CatFlow,
			Severity:   sev,
			Confidence: ConfHigh,
			AutoLevel:  AutoNone,
			Target:     "runtime.flow",
			Title:      title,
			Evidence:   fmt.Sprintf("Cùng một lỗi đã xuất hiện %d lần trong các lượt xử lý gần đây.", r.Count),
			Suggestion: sugg,
			Technical:  r.Sig,
		})
	}
	return out
}

// stuckStep 检测 checkpoint 连续停在同一 step。
func stuckStep(rc *RuntimeCapture) []Finding {
	if rc.StuckStep == "" {
		return nil
	}
	sev := SevWarning
	if rc.StuckCount >= repeatCritical {
		sev = SevCritical
	}
	return []Finding{{
		Rule:       "StuckStep",
		Category:   CatFlow,
		Severity:   sev,
		Confidence: ConfHigh,
		AutoLevel:  AutoNone,
		Target:     "runtime.flow",
		Title:      "Công việc bị kẹt ở cùng một bước",
		Evidence:   fmt.Sprintf("Tiến độ đã ghi nhận cùng một bước %d lần liên tiếp mà chưa chuyển sang bước tiếp theo.", rc.StuckCount),
		Suggestion: "Mở nhật ký để xem công việc đang dở, tạm dừng rồi yêu cầu trợ lý xử lý bước đang kẹt trước khi tiếp tục.",
		Technical:  rc.StuckStep,
	}}
}

// streamIdleStorm 检测流式中断频发（#32）。
func streamIdleStorm(rc *RuntimeCapture) []Finding {
	n := rc.LogKinds["stream_idle"]
	if n < streamIdleWarn {
		return nil
	}
	return []Finding{{
		Rule:       "StreamIdleStorm",
		Category:   CatFlow,
		Severity:   SevWarning,
		Confidence: ConfHigh,
		AutoLevel:  AutoNone,
		Target:     "runtime.provider",
		Title:      "Phản hồi của trợ lý bị gián đoạn nhiều lần",
		Evidence:   fmt.Sprintf("Có %d lần phản hồi bị ngắt vì chờ quá lâu mà không nhận được nội dung mới.", n),
		Suggestion: "Kiểm tra kết nối mạng và dịch vụ mô hình trong cài đặt rồi thử lại. Nếu vẫn gián đoạn, hãy xuất chẩn đoán ẩn danh để gửi hỗ trợ.",
		Technical:  fmt.Sprintf("stream_idle=%d", n),
	}}
}
