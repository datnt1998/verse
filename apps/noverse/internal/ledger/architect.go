package ledger

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
)

type Proposal struct {
	Values  Values `json:"values"`
	Changed []int  `json:"changed"`
	Brief   string `json:"brief"`
}

var axisContract = llmcontract.Contract{Name: "architect_ledger", Description: "Kiến trúc sư đọc hoặc đổi mười trục biến số của yêu cầu truyện", Schema: schema.Object(
	schema.Property("values", schema.Array("Đúng mười giá trị theo thứ tự mười trục; chưa rõ thì tự chọn chi tiết sáng tác phù hợp yêu cầu", schema.String("Giá trị trục bằng tiếng Việt"))).Required(),
	schema.Property("brief", schema.String("Yêu cầu đầy đủ sau khi đổi trục; khi chỉ đọc trục giữ nguyên yêu cầu đầu vào")).Required(),
	schema.Property("changed", schema.Array("Số trục đã đổi từ 1 đến 10; mảng rỗng khi chỉ đọc", schema.Int("Số trục"))).Required(),
)}

type modelOutput struct {
	Values  []string `json:"values"`
	Brief   string   `json:"brief"`
	Changed []int    `json:"changed"`
}

func Analyze(ctx context.Context, model llmtypes.ChatModel, brief string, prior []Story, suggest bool) (Proposal, error) {
	if model == nil {
		return Proposal{}, fmt.Errorf("Chưa cấu hình Kiến trúc sư")
	}
	if len(prior) > 10 {
		prior = prior[:10]
	}
	payload, _ := json.Marshal(struct {
		Brief   string  `json:"brief"`
		Axes    []Axis  `json:"axes"`
		Prior   []Story `json:"prior"`
		Suggest bool    `json:"suggest"`
	}{brief, Axes, prior, suggest})
	out, err := llmcontract.Execute(ctx, model, llmcontract.Request[modelOutput]{Contract: axisContract, SystemPrompt: "Bạn là Kiến trúc sư. Đọc mười trục từ yêu cầu. suggest=false: giữ nguyên brief và changed=[]. suggest=true: thay ít chi tiết nhất để không trùng tổ hợp 1+3+4+6, khác truyện liền trước ít nhất 4 trục, tránh dùng nghề/phản diện/sân khấu quá 2 lần trong 10 truyện và tránh cùng mở đầu 3 truyện liền. Viết lại yêu cầu nhất quán; không thay dữ liệu truyện cũ. Chỉ dùng tiếng Việt.", Payload: string(payload), Options: []llmtypes.CallOption{llmtypes.WithMaxTokens(8192)}, Agent: "architect", Validate: func(o *modelOutput) error {
		if len(o.Values) != 10 {
			return fmt.Errorf("Cần đúng 10 trục")
		}
		if strings.TrimSpace(o.Brief) == "" {
			return fmt.Errorf("Yêu cầu không được rỗng")
		}
		var v Values
		copy(v[:], o.Values)
		if err := Validate(v); err != nil {
			return err
		}
		for _, n := range o.Changed {
			if n < 1 || n > 10 {
				return fmt.Errorf("Số trục phải từ 1 đến 10")
			}
		}
		if suggest && CheckValues(v, prior).Blocking() {
			return fmt.Errorf("Đề xuất vẫn còn trùng trục bắt sửa")
		}
		return nil
	}})
	if err != nil {
		return Proposal{}, err
	}
	p := Proposal{Brief: out.Brief, Changed: out.Changed}
	copy(p.Values[:], out.Values)
	if p.Changed == nil {
		p.Changed = []int{}
	}
	return p, nil
}
