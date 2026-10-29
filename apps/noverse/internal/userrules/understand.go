package userrules

import (
	"context"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

// Understanding là câu trả lời "Noverse hiểu quy tắc này là gì" cho một dòng nhập — chưa lưu gì.
type Understanding struct {
	Structured  rules.Structured `json:"structured"`
	Preferences string           `json:"preferences"`
	Uncertain   []string         `json:"uncertain"`
	// UncertainNote là một câu cho người dùng biết Noverse chưa chắc điều gì; "" khi đã hiểu chắc.
	UncertainNote string `json:"uncertainNote"`
	// Options là 0–3 cách hiểu (mỗi mục là quy tắc đã viết lại rõ ràng) khi Uncertain không rỗng; luôn là mảng.
	Options []string     `json:"options"`
	Chips   []rules.Chip `json:"chips"`
	// Status = degraded khi chuẩn hoá lỗi: quy tắc vẫn dùng được, chỉ là lời dặn nguyên văn.
	Status rules.Status `json:"status"`
}

// Understand chuẩn hoá thử một quy tắc để hiện chip trước khi lưu. Lỗi chuẩn hoá không phải lỗi
// của lời gọi: hạ cấp thành lời dặn nguyên văn (Status=degraded) — giống đường ghi thật.
// Chỉ trả error khi ctx đã hết hạn/bị huỷ, vì khi đó không còn ai chờ kết quả hạ cấp.
func (n *Normalizer) Understand(ctx context.Context, text string) (Understanding, error) {
	text = strings.TrimSpace(text)
	cand, err := n.Normalize(ctx, "understand", text)
	if cerr := ctx.Err(); cerr != nil {
		return Understanding{}, cerr
	}
	if err != nil {
		cand = degraded("understand", text)
	}
	u := Understanding{
		Structured:  cand.Structured,
		Preferences: cand.Preferences,
		Uncertain:   cand.Uncertain,
		Options:     optionsOrEmpty(cand),
		Chips:       rules.Chips(cand),
		Status:      rules.StatusReady,
	}
	if u.Uncertain == nil {
		u.Uncertain = []string{}
	}
	u.UncertainNote = strings.TrimSpace(cand.UncertainNote)
	if u.UncertainNote == "" && len(u.Uncertain) > 0 {
		u.UncertainNote = u.Uncertain[0]
	}
	if cand.Degraded {
		u.Status = rules.StatusDegraded
	}
	return u, nil
}
