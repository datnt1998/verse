package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// PlatformStateStore hiện thực platforms.StateStore trên bảng platform_state:
// một dòng mỗi nền tảng, tạo lười khi ghi. Mọi ghi đọc-tính đều nguyên tử
// trong một câu UPSERT (xem queries/platforms.sql).
type PlatformStateStore struct {
	q *gen.Queries
}

// NewPlatformStateStore dựng store trên một DBTX bất kỳ (pool hoặc tx).
func NewPlatformStateStore(db gen.DBTX) *PlatformStateStore {
	return &PlatformStateStore{q: gen.New(db)}
}

// All đọc trạng thái của mọi nền tảng đã có dòng (một truy vấn).
func (s *PlatformStateStore) All(ctx context.Context) ([]domain.PlatformState, error) {
	rows, err := s.q.ListPlatformStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc platform_state: %w", err)
	}
	out := make([]domain.PlatformState, 0, len(rows))
	for _, r := range rows {
		out = append(out, platformStateFromRow(r))
	}
	return out, nil
}

// One đọc trạng thái một nền tảng; found=false khi chưa có dòng.
func (s *PlatformStateStore) One(ctx context.Context, id domain.PlatformID) (domain.PlatformState, bool, error) {
	r, err := s.q.GetPlatformState(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PlatformState{Platform: id}, false, nil
	}
	if err != nil {
		return domain.PlatformState{}, false, fmt.Errorf("đọc platform_state.%s: %w", id, err)
	}
	return platformStateFromRow(r), true, nil
}

// Pause đọc giờ chờ còn hiệu lực. Đọc là một SELECT (luồng quét gọi mỗi nhịp
// heartbeat khi rảnh); chỉ khi giờ chờ đã hết hạn mới ghi một lần để dọn.
func (s *PlatformStateStore) Pause(ctx context.Context, id domain.PlatformID, now time.Time) (until *time.Time, reason *string, err error) {
	r, err := s.q.GetPlatformState(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("đọc giờ chờ %s: %w", id, err)
	}
	until = timePtr(r.PausedUntil)
	if until == nil {
		return nil, nil, nil
	}
	if !until.After(now) {
		if err := s.q.ClearExpiredPlatformPause(ctx, gen.ClearExpiredPlatformPauseParams{Platform: id, Now: ts(now)}); err != nil {
			return nil, nil, fmt.Errorf("dọn giờ chờ %s: %w", id, err)
		}
		return nil, nil, nil
	}
	return until, textPtr(r.PauseReason), nil
}

// RecordBlocked ghi một lần bị chặn: tăng đếm và đặt giờ chờ tăng dần theo
// câu UPSERT (15·2^(n-1) phút trần 120; retryAfter chỉ kéo dài).
func (s *PlatformStateStore) RecordBlocked(ctx context.Context, id domain.PlatformID, now time.Time, reason string, retryAfter time.Duration) (domain.PlatformState, error) {
	r, err := s.q.RecordPlatformBlocked(ctx, gen.RecordPlatformBlockedParams{
		Platform:     id,
		Now:          ts(now),
		Reason:       text(reason),
		RetryAfterMs: float64(retryAfter.Milliseconds()),
	})
	if err != nil {
		return domain.PlatformState{}, fmt.Errorf("ghi giờ chờ %s: %w", id, err)
	}
	return domain.PlatformState{
		Platform:     id,
		PausedUntil:  timePtr(r.PausedUntil),
		PauseReason:  textPtr(r.PauseReason),
		FailureCount: int(r.FailureCount),
	}, nil
}

// ClearPause xoá giờ chờ và đếm chặn của một nền tảng.
func (s *PlatformStateStore) ClearPause(ctx context.Context, id domain.PlatformID) error {
	if err := s.q.ClearPlatformPause(ctx, id); err != nil {
		return fmt.Errorf("xoá giờ chờ %s: %w", id, err)
	}
	return nil
}

// RecordError ghi lỗi nền tảng gần nhất cần người xử lý.
func (s *PlatformStateStore) RecordError(ctx context.Context, id domain.PlatformID, code string, at time.Time, jobID *string) error {
	if err := s.q.RecordPlatformError(ctx, gen.RecordPlatformErrorParams{
		Platform:       id,
		LastErrorCode:  code,
		LastErrorAt:    ts(at),
		LastErrorJobID: optText(jobID),
	}); err != nil {
		return fmt.Errorf("ghi lỗi %s: %w", id, err)
	}
	return nil
}

// ClearError xoá lỗi gần nhất của một nền tảng.
func (s *PlatformStateStore) ClearError(ctx context.Context, id domain.PlatformID) error {
	if err := s.q.ClearPlatformError(ctx, id); err != nil {
		return fmt.Errorf("xoá lỗi %s: %w", id, err)
	}
	return nil
}

// ReserveSlot đặt chỗ gọi kế tiếp và trả thời điểm được phép bắt đầu.
func (s *PlatformStateStore) ReserveSlot(ctx context.Context, id domain.PlatformID, now time.Time, spacing time.Duration) (time.Time, error) {
	startAt, err := s.q.ReservePlatformSlot(ctx, gen.ReservePlatformSlotParams{
		Platform:  id,
		Now:       ts(now),
		SpacingMs: float64(spacing.Milliseconds()),
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	return startAt.Time, nil
}

// AcquireScanner giữ lease quét; trả acquired=false khi scanner khác còn hạn.
func (s *PlatformStateStore) AcquireScanner(ctx context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error) {
	acquired, err := s.q.AcquirePlatformScanner(ctx, gen.AcquirePlatformScannerParams{
		Platform: id,
		Owner:    text(owner),
		Until:    ts(until),
		Now:      ts(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("giữ lease quét %s: %w", id, err)
	}
	return acquired, nil
}

// ReleaseScanner nhả lease chỉ khi vẫn thuộc owner.
func (s *PlatformStateStore) ReleaseScanner(ctx context.Context, id domain.PlatformID, owner string) error {
	if err := s.q.ReleasePlatformScanner(ctx, gen.ReleasePlatformScannerParams{Platform: id, ScannerOwner: text(owner)}); err != nil {
		return fmt.Errorf("nhả lease quét %s: %w", id, err)
	}
	return nil
}

// ── chuyển kiểu ────────────────────────────────────────────────────────────

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func text(s string) pgtype.Text         { return pgtype.Text{String: s, Valid: true} }
func optText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func platformStateFromRow(r gen.PlatformState) domain.PlatformState {
	return domain.PlatformState{
		Platform:          r.Platform,
		NextRequestAt:     r.NextRequestAt.Time,
		PausedUntil:       timePtr(r.PausedUntil),
		PauseReason:       textPtr(r.PauseReason),
		FailureCount:      int(r.FailureCount),
		ScannerOwner:      textPtr(r.ScannerOwner),
		ScannerLeaseUntil: timePtr(r.ScannerLeaseUntil),
		LastErrorCode:     r.LastErrorCode,
		LastErrorAt:       timePtr(r.LastErrorAt),
		LastErrorJobID:    textPtr(r.LastErrorJobID),
	}
}
