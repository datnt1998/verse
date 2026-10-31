package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// PlatformStateStore hiện thực platforms.StateStore trên bảng platform_state:
// một dòng mỗi nền tảng, tạo lười khi ghi. Các ghi đọc-tính đơn giản đi trong
// một câu UPSERT; đặt chỗ có hạn mức cuộn mở transaction để kiểm giờ chờ,
// hạn mức và ghi lịch sử là một khối (xem queries/platforms.sql).
type PlatformStateStore struct {
	q  *gen.Queries
	db gen.DBTX
}

// NewPlatformStateStore dựng store trên một DBTX bất kỳ (pool hoặc tx).
func NewPlatformStateStore(db gen.DBTX) *PlatformStateStore {
	return &PlatformStateStore{q: gen.New(db), db: db}
}

var _ platforms.StateStore = (*PlatformStateStore)(nil)

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

// ReserveWeighted đặt chỗ gọi có trọng số: kiểm giờ chờ và hạn mức cuộn rồi
// ghi mốc gọi kế tiếp + lịch sử trong CÙNG một transaction (khoá hàng), nên
// hai process không thể cùng vượt hạn mức. Từ chối thì không ghi gì.
func (s *PlatformStateStore) ReserveWeighted(ctx context.Context, id domain.PlatformID, now time.Time, req platforms.SlotRequest) (platforms.SlotDecision, error) {
	txer, ok := s.db.(txBeginner)
	if !ok {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: store không mở được transaction", id)
	}
	tx, err := txer.Begin(ctx)
	if err != nil {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.EnsurePlatformState(ctx, id); err != nil {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	row, err := q.LockPlatformState(ctx, id)
	if err != nil {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	if row.PausedUntil.Valid && row.PausedUntil.Time.After(now) {
		until := row.PausedUntil.Time
		decision := platforms.SlotDecision{PausedUntil: &until, PauseReason: textPtr(row.PauseReason)}
		return decision, tx.Commit(ctx)
	}
	history := parseRequestHistory(row.RequestHistory)
	cost := req.Cost
	if cost < 1 {
		cost = 1
	}
	budget := platforms.BudgetOf(history, now, req.Windows, cost)
	if budget.AvailableAt.After(now) {
		availableAt := budget.AvailableAt
		return platforms.SlotDecision{BudgetAt: &availableAt}, tx.Commit(ctx)
	}
	startAt := now
	if row.NextRequestAt.Valid && row.NextRequestAt.Time.After(startAt) {
		startAt = row.NextRequestAt.Time
	}
	history = append(history, domain.PlatformRequest{At: startAt, Cost: cost})
	history = platforms.PruneHistory(history, now)
	next := startAt.Add(req.Spacing)
	if next.Before(startAt) {
		next = startAt
	}
	if _, err := q.UpdatePlatformRequestState(ctx, gen.UpdatePlatformRequestStateParams{
		Platform:       id,
		NextRequestAt:  ts(next),
		RequestHistory: encodeRequestHistory(history),
	}); err != nil {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return platforms.SlotDecision{}, fmt.Errorf("đặt chỗ %s: %w", id, err)
	}
	return platforms.SlotDecision{StartAt: startAt}, nil
}

// AcquireYtDlp giữ lease yt-dlp chéo process; false khi lease khác còn hạn.
func (s *PlatformStateStore) AcquireYtDlp(ctx context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error) {
	acquired, err := s.q.AcquirePlatformYtDlp(ctx, gen.AcquirePlatformYtDlpParams{
		Platform: id,
		Owner:    text(owner),
		Until:    ts(until),
		Now:      ts(now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("giữ lease yt-dlp %s: %w", id, err)
	}
	return acquired, nil
}

// ExtendYtDlp kéo dài lease yt-dlp của chính process khi slot đặt được lùi
// hơn dự kiến; false khi lease đã bị process khác lấy.
func (s *PlatformStateStore) ExtendYtDlp(ctx context.Context, id domain.PlatformID, owner string, until time.Time) (bool, error) {
	affected, err := s.q.ExtendPlatformYtDlpLease(ctx, gen.ExtendPlatformYtDlpLeaseParams{
		Platform: id, Owner: text(owner), LeaseUntil: ts(until),
	})
	if err != nil {
		return false, fmt.Errorf("gia hạn lease yt-dlp %s: %w", id, err)
	}
	return affected > 0, nil
}

// ReleaseYtDlp nhả lease yt-dlp chỉ khi vẫn thuộc owner.
func (s *PlatformStateStore) ReleaseYtDlp(ctx context.Context, id domain.PlatformID, owner string) error {
	if err := s.q.ReleasePlatformYtDlp(ctx, gen.ReleasePlatformYtDlpParams{Platform: id, YtdlpOwner: text(owner)}); err != nil {
		return fmt.Errorf("nhả lease yt-dlp %s: %w", id, err)
	}
	return nil
}

// SetNextTrickle ghi mốc sớm nhất được đọc trang danh sách kế tiếp.
func (s *PlatformStateStore) SetNextTrickle(ctx context.Context, id domain.PlatformID, at time.Time) error {
	if err := s.q.EnsurePlatformState(ctx, id); err != nil {
		return fmt.Errorf("đặt mốc đọc tiếp %s: %w", id, err)
	}
	if err := s.q.SetPlatformNextTrickle(ctx, gen.SetPlatformNextTrickleParams{Platform: id, NextTrickleAt: ts(at)}); err != nil {
		return fmt.Errorf("đặt mốc đọc tiếp %s: %w", id, err)
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
		RequestHistory:    parseRequestHistory(r.RequestHistory),
		NextTrickleAt:     r.NextTrickleAt.Time,
		YtdlpOwner:        textPtr(r.YtdlpOwner),
		YtdlpLeaseUntil:   timePtr(r.YtdlpLeaseUntil),
	}
}

// platformRequestJSON là hình lưu của một lần đặt chỗ trong request_history:
// mốc mili giây + trọng số (cùng hình với yt-factory để dữ liệu cũ đọc được).
type platformRequestJSON struct {
	At   int64 `json:"at"`
	Cost int   `json:"cost"`
}

func parseRequestHistory(raw []byte) []domain.PlatformRequest {
	if len(raw) == 0 {
		return nil
	}
	var items []platformRequestJSON
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]domain.PlatformRequest, 0, len(items))
	for _, it := range items {
		out = append(out, domain.PlatformRequest{At: time.UnixMilli(it.At), Cost: it.Cost})
	}
	return out
}

func encodeRequestHistory(items []domain.PlatformRequest) []byte {
	arr := make([]platformRequestJSON, 0, len(items))
	for _, it := range items {
		arr = append(arr, platformRequestJSON{At: it.At.UnixMilli(), Cost: it.Cost})
	}
	raw, err := json.Marshal(arr)
	if err != nil {
		return []byte("[]")
	}
	return raw
}
