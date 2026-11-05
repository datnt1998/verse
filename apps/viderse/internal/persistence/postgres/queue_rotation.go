package postgres

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DefaultTemplate trả template mặc định hiện tại của kênh (nil khi kênh không
// có/không tồn tại). Chỉ là gợi ý cho lập kế hoạch và restart; admission chốt
// lại dưới khoá hàng kênh nên kết quả này có thể cũ.
func (s *QueueStore) DefaultTemplate(ctx context.Context, channelID string) (*domain.ChannelTemplate, error) {
	row, err := s.q.DefaultTemplate(ctx, queueUUID(channelID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.ChannelTemplate{ID: uuidString(row.ID), Name: row.Name}, nil
}

// TemplateInfo trả template theo id toàn Thư viện (nil khi không tồn tại) —
// dùng cho override của một lần tạo (không còn giới hạn "thuộc danh sách kênh").
func (s *QueueStore) TemplateInfo(ctx context.Context, templateID string) (*domain.ChannelTemplate, error) {
	names, err := s.TemplateNames(ctx, []string{templateID})
	if err != nil {
		return nil, err
	}
	name, ok := names[templateID]
	if !ok {
		return nil, nil
	}
	return &domain.ChannelTemplate{ID: templateID, Name: name}, nil
}

// DefaultsForChannels trả template mặc định của nhiều kênh trong một truy vấn
// (bỏ kênh không có). Dùng để hiển thị tên mặc định của mục hàng chờ và job.
func (s *QueueStore) DefaultsForChannels(ctx context.Context, ids []string) (map[string]domain.ChannelTemplate, error) {
	if len(ids) == 0 {
		return map[string]domain.ChannelTemplate{}, nil
	}
	args := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		args = append(args, queueUUID(id))
	}
	rows, err := s.q.ListChannelDefaults(ctx, args)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.ChannelTemplate, len(rows))
	for _, r := range rows {
		out[uuidString(r.OwnerChannelID)] = domain.ChannelTemplate{ID: uuidString(r.TemplateID), Name: r.Name}
	}
	return out, nil
}

// TemplateNames trả tên template theo id (tên hiển thị cho job đang chạy và
// override của hàng chờ).
func (s *QueueStore) TemplateNames(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	args := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		args = append(args, queueUUID(id))
	}
	rows, err := s.q.TemplateNames(ctx, args)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[uuidString(r.ID)] = r.Name
	}
	return out, nil
}
