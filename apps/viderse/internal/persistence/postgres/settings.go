package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// SettingsStore đọc/ghi bảng settings qua gen.Queries. Tương đương
// settings-store của yt-factory: giá trị JSON, đọc sai kiểu trả về như chưa
// ghi (một dòng lỗi không làm gãy pipeline), ghi là upsert, xoá khoá không có
// là no-op.
type SettingsStore struct {
	q *gen.Queries
}

// NewSettingsStore dựng store trên một DBTX bất kỳ (pool hoặc transaction).
func NewSettingsStore(db gen.DBTX) *SettingsStore {
	return &SettingsStore{q: gen.New(db)}
}

// Get đọc giá trị JSON của key vào out (json.Unmarshal). Khoá thiếu hoặc giá
// trị sai kiểu với out trả found=false, err=nil.
func (s *SettingsStore) Get(ctx context.Context, key string, out any) (bool, error) {
	raw, err := s.q.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("đọc settings.%s: %w", key, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		// Giá trị sai kiểu đọc như chưa ghi — giữ hợp đồng của bản TS.
		return false, nil
	}
	return true, nil
}

// Set upsert giá trị JSON của key.
func (s *SettingsStore) Set(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("mã hoá settings.%s: %w", key, err)
	}
	if err := s.q.SetSetting(ctx, gen.SetSettingParams{Key: key, Value: raw}); err != nil {
		return fmt.Errorf("ghi settings.%s: %w", key, err)
	}
	return nil
}

// Clear xoá key; khoá không tồn tại là no-op.
func (s *SettingsStore) Clear(ctx context.Context, key string) error {
	if err := s.q.ClearSetting(ctx, key); err != nil {
		return fmt.Errorf("xoá settings.%s: %w", key, err)
	}
	return nil
}

// QueuePaused đọc cờ tạm dừng hàng chờ (domain.KeyQueuePaused); thiếu hoặc sai
// kiểu trả false.
func (s *SettingsStore) QueuePaused(ctx context.Context) (bool, error) {
	var paused bool
	found, err := s.Get(ctx, domain.KeyQueuePaused, &paused)
	if err != nil {
		return false, err
	}
	return found && paused, nil
}

// SetQueuePaused ghi cờ tạm dừng hàng chờ.
func (s *SettingsStore) SetQueuePaused(ctx context.Context, paused bool) error {
	return s.Set(ctx, domain.KeyQueuePaused, paused)
}
