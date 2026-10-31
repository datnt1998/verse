// Package postgres là tầng lưu trữ của Viderse: mở pool và hiện thực các store
// interface mà gói nghiệp vụ khai báo, trên code sqlc sinh từ queries/. Gói
// nghiệp vụ không cầm *pgxpool.Pool — chỉ bootstrap đưa pool vào đây.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPool mở pgxpool từ DSN (dùng chung cho serve/test) và ping để lỗi
// kết nối/DSN sai bùng ngay lúc khởi động thay vì lần query đầu.
// Long-lived session locks and LISTEN use separate connections, never pool
// slots: their work must still be able to query even when every holder is busy.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("mở pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("kết nối postgres: %w", err)
	}
	return pool, nil
}
