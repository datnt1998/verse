package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// ImageReceiptStore là journal biên nhận ảnh trả phí của imagegen trên bảng
// image_receipts. Attempt được chốt tại DB (số attempt = MAX+1) nên hai lần
// Begin cùng key không thể san bằng nhau, và charged đếm được qua restart.
//
// Trần ngày được kiểm TRONG cùng transaction với việc ghi attempt, dưới một
// advisory lock theo provider: hai process không thể cùng đọc 49 rồi cùng ghi
// attempt thứ 50/51.
type ImageReceiptStore struct {
	db        gen.DBTX
	q         *gen.Queries
	lockPool  *pgxpool.Pool
	lockSlots chan struct{}
}

// NewImageReceiptStore dựng store trên một DBTX bất kỳ (pool hoặc transaction).
func NewImageReceiptStore(db gen.DBTX) *ImageReceiptStore {
	s := &ImageReceiptStore{db: db, q: gen.New(db)}
	if pool, ok := db.(*pgxpool.Pool); ok {
		s.lockPool = pool
		// Bound concurrent dedicated sessions without consuming query slots.
		s.lockSlots = make(chan struct{}, int(pool.Config().MaxConns))
	}
	return s
}

var _ imagegen.Journal = (*ImageReceiptStore)(nil)

// txBeginner là DBTX mở được transaction — pool hoặc chính pgx.Tx (savepoint).
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// LockKey giành quyền sở hữu logic của draw key qua process bằng session
// advisory lock của Postgres. Store dựng trên DBTX không phải pool (ví dụ
// transaction đang mở) thì không giữ được lock qua câu lệnh: trả no-op —
// unique index one-running vẫn chặn hai attempt chạy song song.
func (s *ImageReceiptStore) LockKey(ctx context.Context, key string) (func(), error) {
	if s.lockPool == nil {
		return func() {}, nil
	}
	select {
	case s.lockSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	conn, err := pgx.ConnectConfig(ctx, s.lockPool.Config().ConnConfig.Copy())
	if err != nil {
		<-s.lockSlots
		return nil, fmt.Errorf("giữ kết nối cho khoá %s: %w", key, err)
	}
	const lockSQL = `SELECT pg_advisory_lock(hashtextextended($1, 0))`
	if _, err := conn.Exec(ctx, lockSQL, "image-draw:"+key); err != nil {
		_ = conn.Close(context.Background())
		<-s.lockSlots
		return nil, fmt.Errorf("khoá draw key %s: %w", key, err)
	}
	return func() {
		// Mở khoá và trả kết nối; lỗi ở đây không còn gì để cứu.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, "image-draw:"+key)
		_ = conn.Close(unlockCtx)
		<-s.lockSlots
	}, nil
}

// Begin mở một attempt mới: khoá provider (nếu có trần) → kiểm trần → khoá key
// → ghi attempt, tất cả trong một transaction. Vượt trần → *CapReachedError;
// key đã có attempt running → imagegen.ErrAttemptConcluded.
func (s *ImageReceiptStore) Begin(ctx context.Context, a imagegen.Admission) (imagegen.JournalEntry, error) {
	txer, ok := s.db.(txBeginner)
	if !ok {
		return imagegen.JournalEntry{}, fmt.Errorf("journal ảnh cần DBTX mở được transaction")
	}
	tx, err := txer.Begin(ctx)
	if err != nil {
		return imagegen.JournalEntry{}, fmt.Errorf("mở transaction biên nhận ảnh: %w", err)
	}
	defer tx.Rollback(ctx)
	q := gen.New(tx)

	// Thứ tự khoá cố định provider → key để không có chu trình chờ.
	if a.PaidCap >= 0 {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
			"image-cap:"+a.Entry.Provider); err != nil {
			return imagegen.JournalEntry{}, fmt.Errorf("khoá trần ảnh %s: %w", a.Entry.Provider, err)
		}
		used, err := q.CountChargedImageReceipts(ctx, gen.CountChargedImageReceiptsParams{
			Provider:  a.Entry.Provider,
			StartedAt: pgtype.Timestamptz{Time: a.DayStart, Valid: true},
		})
		if err != nil {
			return imagegen.JournalEntry{}, fmt.Errorf("đếm ảnh trả phí của %s: %w", a.Entry.Provider, err)
		}
		if used >= int64(a.PaidCap) {
			return imagegen.JournalEntry{}, imagegen.ErrCapReached(a.Entry.Provider, used, a.PaidCap)
		}
	}
	// Không khoá key ở đây: caller đã giữ LockKey (session advisory lock) suốt
	// chuỗi settle→draw; khoá lại bằng transaction trên một kết nối khác sẽ
	// tự chặn chính mình. Hai Begin song song cùng key vẫn bị unique index
	// one-running chặn (23505 → ErrAttemptConcluded).

	startedAt := a.Entry.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	row, err := q.BeginImageReceipt(ctx, gen.BeginImageReceiptParams{
		DrawKey:    a.Entry.Key,
		Provider:   a.Entry.Provider,
		Model:      a.Entry.Model,
		OutputPath: a.Entry.Output,
		StartedAt:  pgtype.Timestamptz{Time: startedAt, Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return imagegen.JournalEntry{}, imagegen.ErrAttemptConcluded
		}
		return imagegen.JournalEntry{}, fmt.Errorf("ghi biên nhận ảnh %s: %w", a.Entry.Key, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return imagegen.JournalEntry{}, fmt.Errorf("chốt biên nhận ảnh %s: %w", a.Entry.Key, err)
	}
	return entryFromRow(row), nil
}

func (s *ImageReceiptStore) Latest(ctx context.Context, key string) (*imagegen.JournalEntry, error) {
	row, err := s.q.LatestImageReceipt(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc biên nhận ảnh %s: %w", key, err)
	}
	e := entryFromRow(row)
	return &e, nil
}

// Finish kết luận attempt đang chạy. Attempt đã được chủ khác kết luận (hoặc
// không còn running) → imagegen.ErrAttemptConcluded: không ghi đè quyết định.
func (s *ImageReceiptStore) Finish(ctx context.Context, key string, attempt int, state string, charged bool, receipt *imagegen.Receipt, errCode, errText string, finishedAt time.Time) error {
	var receiptJSON []byte
	if receipt != nil {
		data, err := json.Marshal(receipt)
		if err != nil {
			return fmt.Errorf("mã hoá receipt ảnh %s: %w", key, err)
		}
		receiptJSON = data
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	n, err := s.q.CompleteImageReceipt(ctx, gen.CompleteImageReceiptParams{
		DrawKey:    key,
		Attempt:    int32(attempt),
		State:      state,
		Charged:    charged,
		ErrorCode:  errCode,
		ErrorText:  errText,
		Receipt:    receiptJSON,
		FinishedAt: pgtype.Timestamptz{Time: finishedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("hoàn tất biên nhận ảnh %s: %w", key, err)
	}
	if n == 0 {
		return imagegen.ErrAttemptConcluded
	}
	return nil
}

func (s *ImageReceiptStore) UpdateScratch(ctx context.Context, key string, attempt int, scratch json.RawMessage) error {
	if err := s.q.ImageReceiptScratch(ctx, gen.ImageReceiptScratchParams{
		DrawKey: key,
		Attempt: int32(attempt),
		Scratch: scratch,
	}); err != nil {
		return fmt.Errorf("ghi scratch biên nhận ảnh %s: %w", key, err)
	}
	return nil
}

func (s *ImageReceiptStore) Charged(ctx context.Context, provider string, since time.Time) (int64, error) {
	n, err := s.q.CountChargedImageReceipts(ctx, gen.CountChargedImageReceiptsParams{
		Provider:  provider,
		StartedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("đếm ảnh trả phí của %s: %w", provider, err)
	}
	return n, nil
}

func (s *ImageReceiptStore) Abandon(ctx context.Context, key string) error {
	if err := s.q.AbandonImageReceipts(ctx, key); err != nil {
		return fmt.Errorf("bỏ biên nhận ảnh %s: %w", key, err)
	}
	return nil
}

// isUniqueViolation nhận diện SQLSTATE 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// entryFromRow đổi hàng gen sang JournalEntry của domain ảnh.
func entryFromRow(r gen.ImageReceipt) imagegen.JournalEntry {
	e := imagegen.JournalEntry{
		Key:       r.DrawKey,
		Attempt:   int(r.Attempt),
		Provider:  r.Provider,
		Model:     r.Model,
		Output:    r.OutputPath,
		State:     r.State,
		Charged:   r.Charged,
		ErrorCode: r.ErrorCode,
		ErrorText: r.ErrorText,
		Scratch:   json.RawMessage(r.Scratch),
		StartedAt: r.StartedAt.Time,
	}
	if r.FinishedAt.Valid {
		t := r.FinishedAt.Time
		e.FinishedAt = &t
	}
	if len(r.Receipt) > 0 {
		var rc imagegen.Receipt
		if json.Unmarshal(r.Receipt, &rc) == nil {
			e.Receipt = &rc
		}
	}
	return e
}
