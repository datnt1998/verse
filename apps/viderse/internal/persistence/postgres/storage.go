package postgres

import (
	"context"
	"encoding/json"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
)

// StorageStore là phần DB mà mảng Lưu trữ cần ngoài settings: đường dẫn scratch
// của các lần vẽ ảnh còn chạy, để bộ dọn không xoá phiên phục hồi.
//
// Không có bảng riêng cho vị trí/bản cũ/dọn dẹp — chúng là settings JSON, nên
// đổi thư mục và khởi động lại không cần migrate gì.
type StorageStore struct {
	db gen.DBTX
	q  *gen.Queries
}

// NewStorageStore dựng store trên một DBTX bất kỳ (pool hoặc transaction).
func NewStorageStore(db gen.DBTX) *StorageStore {
	return &StorageStore{db: db, q: gen.New(db)}
}

var _ retention.Receipts = (*StorageStore)(nil)

// ScratchDirs trả thư mục làm việc của mọi attempt ảnh đang 'running'. Scratch
// hỏng/không có trường work thì bỏ qua — không suy đoán đường dẫn.
func (s *StorageStore) ScratchDirs(ctx context.Context) ([]string, error) {
	rows, err := s.q.UnfinishedImageReceiptScratch(ctx)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, raw := range rows {
		var scratch struct {
			Work string `json:"work"`
			Home string `json:"home"`
		}
		if err := json.Unmarshal(raw, &scratch); err != nil {
			continue
		}
		for _, dir := range []string{scratch.Work, scratch.Home} {
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs, nil
}
