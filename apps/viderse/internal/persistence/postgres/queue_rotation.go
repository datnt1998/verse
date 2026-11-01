package postgres

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/performance"
)

// RotationTemplates là danh sách luân phiên template của kênh: Templates cộng
// vòng thứ hai cho template được chạy nhiều hơn (bấm tay hoặc do ưu tiên Lượt
// xem). Con trỏ owner.TemplateCursor chạy trên danh sách này.
func (s *QueueStore) RotationTemplates(ctx context.Context, owner domain.Channel) ([]domain.ChannelTemplate, error) {
	templates, err := s.Templates(ctx, owner.ID)
	if err != nil || len(templates) < 2 {
		return templates, err
	}
	id := queueUUID(owner.ID)
	rows, err := s.q.ListTemplateWeights(ctx, id)
	if err != nil {
		return nil, err
	}
	manual := map[string]int{}
	for _, r := range rows {
		manual[uuidString(r.TemplateID)] = int(r.Weight)
	}
	var views map[string][]int64
	if owner.Priority == domain.PriorityViews {
		stats, err := s.q.ListChannelTemplateViews7d(ctx, id)
		if err != nil {
			return nil, err
		}
		views = map[string][]int64{}
		for _, r := range stats {
			key := uuidString(r.TemplateID)
			views[key] = append(views[key], r.Views)
		}
	}
	return performance.RotationSequence(templates, performance.RotationWeights(manual, owner.Priority, views)), nil
}

// RivalVideoScores trả điểm nổi trội của các video đối thủ (chỉ video có điểm).
func (s *QueueStore) RivalVideoScores(ctx context.Context, ids []string) (map[string]float64, error) {
	rows, err := s.q.ListRivalVideoScores(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Score.Valid {
			out[r.ID] = r.Score.Float64
		}
	}
	return out, nil
}
