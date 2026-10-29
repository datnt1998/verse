package comments

import (
	"fmt"
	"time"
)

// Repository là lớp lưu trữ góp ý cho một truyện đã định phạm vi (backend file
// theo thư mục, backend postgres theo project+story). Store giữ toàn bộ
// nghiệp vụ (validate, vòng đời, neo mốc) còn repository chỉ trả lời: đọc/ghi
// danh sách góp ý của một chương trong một giao dịch, đọc chính văn để giải
// mốc neo, liệt kê chương có góp ý và hàng đợi sự kiện phát SSE.
//
// Nghĩa của hai entry point giống hệt file backend cũ:
//   - Transact: nạp danh sách, migrate/định hình + làm mới mốc neo trên chính
//     văn hiện tại (đọc lười khi có góp ý đủ điều kiện neo), chạy fn rồi ghi
//     lại khi fn báo changed hoặc migrate đã đổi dữ liệu.
//   - TransactAt: như Transact nhưng dùng đoạn văn do đường ghi chương cung
//     cấp; paras rỗng nghĩa là bỏ qua bước làm mới mốc neo (vd. MarkFixed).
type Repository interface {
	// Transact khoá danh sách góp ý của chương, migrate trên chính văn hiện
	// tại và ghi lại nếu changed/migrated. fn phải trả danh sách mới, cờ
	// changed và lỗi; lỗi huỷ toàn bộ giao dịch.
	Transact(chapter int, fn func(list []Comment) ([]Comment, bool, error)) error
	// TransactAt giống Transact nhưng migrate KHÔNG giải mốc neo: đường ghi
	// chương tự chạy applyWriteResolution trên đoạn văn nó truyền vào fn.
	// paras được giữ trong chữ ký để ghi chú ngữ cảnh, repository không dùng.
	TransactAt(chapter int, paras []string, fn func(list []Comment) ([]Comment, bool, error)) error
	ChapterText(chapter int) (string, error)
	// Chapters liệt kê các chương đang có góp ý (tăng dần), dùng cho
	// CountsByChapter và ResolveAllFixed.
	Chapters() ([]int, error)
	// ReadRaw đọc danh sách thô của một chương (đã chuẩn hoá status tối
	// thiểu) mà không giải mốc neo — cho đếm nhanh; chương không có góp ý trả
	// nil, nil.
	ReadRaw(chapter int) ([]Comment, error)
	// WithEvents chạy fn trên hàng đợi sự kiện đang chờ và lưu kết quả trong
	// cùng một giao dịch (AppendEvent/DrainEvents).
	WithEvents(fn func([]ReanchorEvent) []ReanchorEvent) error
}

// migrateFile chuẩn hoá một record thô (file hoặc row) thành Comment hiện
// hành: gán status mặc định, cờ whole và làm mới mốc neo khi có đoạn văn.
// Trả (danh sách, có thay đổi cần ghi lại).
func migrateFile(list []commentFile, paras []string, now time.Time) ([]Comment, bool) {
	out := make([]Comment, len(list))
	changed := false
	for i, raw := range list {
		c := raw.Comment
		if c.Chips == nil {
			c.Chips = []string{}
		}
		if raw.AppliedAt != nil {
			c.SentAt = raw.AppliedAt
			changed = true
		}
		if c.Status == "" {
			changed = true
			if c.SentAt != nil {
				c.Status = StatusSent
			} else {
				c.Status = StatusPending
			}
		}
		if c.Paragraph == nil && c.Quote == "" && !c.Whole {
			c.Whole = true
			changed = true
		}
		out[i] = c
	}
	_ = now
	// Làm mới mốc neo trên chính văn hiện tại khi đọc được; trạng thái
	// sent/fixed/done không đổi ở đường đọc (vòng đời thuộc về đường ghi).
	if len(paras) > 0 {
		for i := range out {
			if !out[i].anchorEligible() {
				continue
			}
			switch out[i].Status {
			case StatusPending, StatusLost, StatusUnsure:
				before := out[i]
				applyReadResolution(&out[i], paras)
				if !commentsEqual(before, out[i]) {
					changed = true
				}
			case StatusSent:
				before := out[i]
				applyAnchorTracking(&out[i], paras)
				if !commentsEqual(before, out[i]) {
					changed = true
				}
			}
		}
	}
	return out, changed
}

// MigrateStored giống migrateFile nhưng nhận []Comment đã decode (postgres
// rows); dùng chung logic neo với đường file — backend nào cũng phải trả kết
// quả migrate qua đây, không tự viết lại.

// AnchorEligible reports whether a comment can participate in anchor
// resolution — backend repositories need it for lazy text loading.
func AnchorEligible(c Comment) bool { return c.anchorEligible() }

// NormalizedParagraphs tách chính văn thành đoạn đã chuẩn hoá — tương đương
// anchor.go normalizedParagraphs, export cho backend postgres đọc text.
func NormalizedParagraphs(markdown string) []string { return normalizedParagraphs(markdown) }
func MigrateStored(list []Comment, paras []string, now time.Time) ([]Comment, bool) {
	raw := make([]commentFile, len(list))
	for i, c := range list {
		raw[i] = commentFile{Comment: c}
	}
	return migrateFile(raw, paras, now)
}

// storedStatus tính status đếm nhanh từ record thô (CountsByChapter).
func storedStatus(c Comment) string {
	if c.Status != "" {
		return c.Status
	}
	if c.SentAt != nil {
		return StatusSent
	}
	return StatusPending
}

// validateChapter kiểm tra số chương trước mọi transaction.
func validateChapter(chapter int) error {
	if chapter < 1 {
		return fmt.Errorf("%w: số chương phải lớn hơn 0", ErrInvalid)
	}
	return nil
}
