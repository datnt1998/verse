package main

import (
	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newAutonomyService dựng Mức tự chủ của kênh. Reviewer được gắn khi dịch vụ
// ảnh dựng xong (mountCloneOwnerOperations); trợ lý và Học dùng cùng instance.
func newAutonomyService(pool *pgxpool.Pool, publish func(string, any), diag func(string)) *autonomy.Service {
	return &autonomy.Service{Store: postgres.NewAutonomyStore(pool), Publish: publish, Diag: diag}
}
