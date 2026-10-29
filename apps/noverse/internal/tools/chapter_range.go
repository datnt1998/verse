package tools

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// plannedChapters trả tổng số chương theo đại cương; 0 khi chưa biết (chưa lập
// đại cương) — lúc đó không kiểm tra trần.
//
// Lấy max(progress.total_chapters, số chương có trong đại cương) để không bao
// giờ từ chối một chương ĐÃ có trong đại cương chỉ vì progress bị cũ (đại cương
// được mở rộng sau khi progress.total_chapters được ghi).
func plannedChapters(st *store.Store) int {
	if st == nil {
		return 0
	}
	total := 0
	if progress, err := st.Progress.Load(); err == nil && progress != nil {
		total = progress.TotalChapters
	}
	if outlined := outlinedChapters(st); outlined > total {
		total = outlined
	}
	return total
}

// outlinedChapters đếm số chương có trong đại cương (phân tầng hoặc phẳng).
func outlinedChapters(st *store.Store) int {
	if volumes, err := st.Outline.LoadLayeredOutline(); err == nil && len(volumes) > 0 {
		return len(domain.FlattenOutline(volumes))
	}
	if outline, err := st.Outline.LoadOutline(); err == nil {
		return len(outline)
	}
	return 0
}

// writingChapter trả chương sắp viết (đã xong + 1); 0 khi không đọc được tiến độ.
func writingChapter(st *store.Store) int {
	if st == nil {
		return 0
	}
	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		return 0
	}
	return progress.NextChapter()
}

// checkChapterParam từ chối số chương ngoài khoảng hợp lệ bằng thông điệp tiếng
// Việt nêu rõ khoảng, để model tự sửa tham số ngay lượt sau thay vì đoán số
// khác (production 2026-10-26: kiến trúc sư gọi 0, 1, 25, -1, 9, 100 rồi
// 2147483647 trong 10 lượt liền, không lượt nào có tiến triển).
//
// chapter == 0 được phép: đó là cách gọi "không truyền chapter" (nhánh kiến
// trúc sư đọc trạng thái tiến độ + nền tảng). Số âm là lỗi tham số.
//
// Trần là progress.total_chapters — với truyện đã lập đại cương phân tầng, con
// số này vẫn là tổng theo kế hoạch (64), không phải số chương đã có đại cương
// chi tiết (8), nên yêu cầu ngữ cảnh cho chương 25 vẫn hợp lệ.
func checkChapterParam(st *store.Store, chapter int) error {
	if chapter == 0 {
		return nil
	}
	total := plannedChapters(st)
	if chapter < 0 {
		return fmt.Errorf("chapter=%d không hợp lệ: chapter là số chương tính từ 1; bỏ hẳn chapter để đọc trạng thái tiến độ", chapter)
	}
	if total > 0 && chapter > total {
		return fmt.Errorf("chapter=%d ngoài khoảng hợp lệ 1..%d (đại cương %d chương, đang viết chương %d): chọn chapter trong 1..%d, hoặc bỏ chapter để đọc trạng thái tiến độ",
			chapter, total, total, writingChapter(st), total)
	}
	return nil
}
