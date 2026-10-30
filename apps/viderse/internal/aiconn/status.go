package aiconn

import (
	"context"
	"fmt"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// AttentionItem là một mục cần xử lý do aiconn báo lên chấm bánh răng —
// composition root map sang status.Attention (kind thuộc domain.Attention*).
type AttentionItem struct {
	Kind domain.AttentionKind
	Text string
	Href string
}

// ConnectHref là đích mở mục Kết nối AI trong Cài đặt.
const ConnectHref = "#/settings?section=connect"

// AttentionReport tính các mục cần xử lý của kết nối AI: đăng nhập gói hết
// hạn/bị thu hồi (login_expired) và trần ảnh ngày đã chạm (quota_low khi
// còn dưới 20% hoặc hết — biết được duy nhất qua biên nhận trả phí).
// Thuần local và giới hạn: chỉ đọc các thẻ đăng nhập gói (2 dòng), không
// kéo danh mục hay provider khoá API.
func (s *Service) AttentionReport(ctx context.Context) ([]AttentionItem, error) {
	views, err := s.Subscribed(ctx)
	if err != nil {
		return nil, err
	}
	var out []AttentionItem
	for _, v := range views {
		switch v.Status {
		case domain.StatusLoggedOut:
			out = append(out, AttentionItem{
				Kind: domain.AttentionLoginExpired,
				Text: "Chưa đăng nhập " + v.Label,
				Href: ConnectHref,
			})
		case domain.StatusAuthFailed:
			out = append(out, AttentionItem{
				Kind: domain.AttentionLoginExpired,
				Text: "Đăng nhập " + v.Label + " đã hết hạn",
				Href: ConnectHref,
			})
		}
		if v.Quota != nil && v.Quota.Limit > 0 && v.Quota.Used >= v.Quota.Limit {
			out = append(out, AttentionItem{
				Kind: domain.AttentionQuotaLow,
				Text: fmt.Sprintf("%s đã dùng hết %d ảnh trả phí hôm nay", v.Label, v.Quota.Limit),
				Href: ConnectHref,
			})
		}
	}
	return out, nil
}
