package main

import (
	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newAutonomyService dựng phần máy tự làm: tự duyệt ảnh (ngưỡng cố định 85)
// và tự sửa lỗi một lần. Reviewer được gắn khi dịch vụ ảnh dựng xong
// (mountCloneOwnerOperations).
func newAutonomyService(pool *pgxpool.Pool, publish func(string, any), diag func(string)) *autonomy.Service {
	return &autonomy.Service{Store: postgres.NewAutonomyStore(pool), Publish: publish, Diag: diag}
}
