package guard

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/datnt1998/noverse/internal/agent"
)

// repeatFailureLimit là số lần một lời gọi y nguyên (cùng tool, cùng đúng
// arguments) được phép lỗi trước khi bị chặn.
const repeatFailureLimit = 2

// repeatFailureMaxKeys chặn map phình vô hạn trong một phiên rất dài.
const repeatFailureMaxKeys = 256

// NewRepeatFailureMiddleware chặn vòng lặp "gọi lại y nguyên lời gọi vừa lỗi".
//
// Vì sao cần: production 2026-10-26, kiến trúc sư mở rộng cung gọi
// novel_context 10 lượt liên tiếp — {chapter:0} bốn lần với đúng một lỗi ngân
// sách, rồi 1/25/-1/9/100 rồi 2147483647 — mỗi lượt là một request LLM đầy đủ
// mà không có tiến triển nào (tui.log 22:24–22:31).
//
// Cách đếm: theo (tên tool + arguments nguyên văn). Lời gọi thành công xoá bộ
// đếm của chính nó, nên đọc lại cùng một chương sau khi sửa vẫn chạy bình
// thường. Sau repeatFailureLimit lần lỗi, đúng cặp (tool, args) đó bị chặn tới
// hết phiên process: mọi cách gọi khác (bỏ chapter, chương khác, source khác)
// vẫn chạy, nên agent luôn còn đường tiến — đó là chủ ý, vì lời gọi đã lỗi y
// nguyên hai lần thì lần thứ ba gần như chắc chắn lỗi tiếp (production: cùng
// một lỗi ngân sách, bốn lần).
//
// Middleware trả lỗi nghĩa là tool KHÔNG chạy; agentcore biến nó thành kết quả
// is_error để model đọc lý do và đổi tham số.
func NewRepeatFailureMiddleware() agentcore.ToolMiddleware {
	var mu sync.Mutex
	failures := make(map[string]int)
	return func(ctx context.Context, call llmtypes.ToolCall, next agentcore.ToolExecuteFunc) (json.RawMessage, error) {
		key := call.Name + "\x00" + string(call.Args)

		mu.Lock()
		failed := failures[key]
		if len(failures) > repeatFailureMaxKeys {
			// Phiên quá dài: bỏ bộ đếm cũ thay vì phình bộ nhớ. Điều kiện chặn
			// vẫn đạt lại sau repeatFailureLimit lần lỗi mới.
			clear(failures)
			failed = 0
		}
		mu.Unlock()

		if failed >= repeatFailureLimit {
			return nil, fmt.Errorf("lời gọi %s với đúng tham số này đã lỗi %d lần liền và lặp lại sẽ không thành công. Hãy đổi tham số (ví dụ bỏ chapter, chọn chương khác, đổi source) hoặc dùng thông tin đã có để làm bước tiếp theo", call.Name, failed)
		}

		out, err := next(ctx, call.Args)

		mu.Lock()
		if err != nil {
			failures[key] = failures[key] + 1
		} else {
			delete(failures, key)
		}
		mu.Unlock()
		return out, err
	}
}
