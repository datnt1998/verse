package guard

import "strings"

// ArbiterRequest is the explicit worker-to-host handoff. It is not a completed
// writing task: the host must resolve it before dispatching the next worker.
const ArbiterRequest = "Cần Trọng tài:"

// ArbiterRequestText also catches legacy replies ending in an unanswered
// question. Quoted questions in chapter prose do not end in a bare question mark.
func ArbiterRequestText(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, ArbiterRequest) {
		return strings.TrimSpace(strings.TrimPrefix(text, ArbiterRequest))
	}
	if strings.HasSuffix(strings.TrimRight(text, "*_`"), "?") {
		return text
	}
	return ""
}

const AmbiguityPrompt = `

## Phân vân cần phán định
Không bao giờ kết thúc bằng câu hỏi gửi người dùng. Bạn không trò chuyện trực tiếp với họ trong thẻ bước. Nếu cần chọn giữa các hướng mà nhiệm vụ chưa cho phép tự quyết, hãy dùng kênh Trọng tài: trả lời cuối bắt đầu chính xác bằng "Cần Trọng tài:"; nêu chỗ phân vân, 2–3 phương án cụ thể và phương án khuyến nghị kèm lý do, bằng câu trần thuật. Đây là bàn giao chờ phán định, không phải báo đã hoàn thành. Không tự thực hiện phương án khi chưa được phân phối lại. Hệ thống sẽ hỏi người dùng bằng lựa chọn thật nếu họ bật Hỏi khi phân vân, hoặc để Trọng tài tự chọn và ghi nhật ký nếu họ tắt.
`
