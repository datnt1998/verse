package storage

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Attention là một mục cần xử lý của Lưu trữ (kiểu riêng của gói, không import
// tầng status — chỗ ráp đổi sang status.Attention).
type Attention struct {
	Kind domain.AttentionKind
	Text string
	Href string
}

// AttentionItems trả các root dự án không truy cập được: đầu trang hiện chấm
// và mục Cài đặt › Lưu trữ có chấm cùng nhịp SSE.
func (s *Service) AttentionItems(context.Context) []Attention {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Attention, 0, len(s.missing))
	for root := range s.missing {
		items = append(items, Attention{
			Kind: domain.AttentionToolFailed,
			Text: "Không thấy " + s.missingLabel(root),
			Href: "#/settings?section=storage",
		})
	}
	return items
}
