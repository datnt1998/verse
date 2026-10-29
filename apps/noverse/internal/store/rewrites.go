package store

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// Câu 9 §H: log chương thực sự bước vào viết lại, meta/rewrites.jsonl, chỉ thêm.
// Mỗi dòng {chapter, at, cause}. Nguồn duy nhất để Quality biết một chương đã
// từng được viết lại (status "rewritten"); không suy từ bằng chứng thẩm duyệt và
// không dựng lại lịch sử trước bản ghi này.
const (
	rewritesFile = "meta/rewrites.jsonl"

	// RewriteCauseReview: Editor chấm ra Bắt sửa nên chương vào hàng đợi.
	RewriteCauseReview = "review"
	// RewriteCauseUser: người dùng tự yêu cầu (góp ý đã duyệt, sửa xưng hô/quan hệ,
	// mở lại sách để viết lại).
	RewriteCauseUser = "user"
)

// enterRewritesUnlocked ghi một dòng cho mỗi chương mới vào hàng đợi (có trong
// next nhưng chưa có trong prev). Gọi trong write lock của progress.
// prev là hàng đợi trước khi đổi; nhờ vậy chương đã nằm trong hàng đợi không bị
// ghi lặp, còn chương quay lại hàng đợi sau khi đã viết xong thì có dòng mới.
func (s *fileProgressStore) enterRewritesUnlocked(prev, next []int, cause string) error {
	var lines []byte
	at := ""
	for _, ch := range next {
		if ch <= 0 || slices.Contains(prev, ch) {
			continue
		}
		if cause != RewriteCauseReview && cause != RewriteCauseUser {
			return fmt.Errorf("nguyên nhân viết lại không hợp lệ: %q", cause)
		}
		if at == "" {
			at = time.Now().Format(time.RFC3339)
		}
		data, err := json.Marshal(domain.RewriteEntry{Chapter: ch, At: at, Cause: cause})
		if err != nil {
			return err
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	if len(lines) == 0 {
		return nil
	}
	return s.io.AppendLineUnlocked(rewritesFile, lines)
}

// Rewrites trả toàn bộ log viết lại theo thứ tự ghi.
func (s *fileProgressStore) Rewrites() ([]domain.RewriteEntry, error) {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	data, err := s.io.ReadFileUnlocked(rewritesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]domain.RewriteEntry, 0, strings.Count(string(data), "\n"))
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Dòng cuối có thể dở dang nếu tiến trình chết giữa lúc ghi; bỏ qua dòng
		// không đọc được thay vì làm hỏng cả endpoint đọc.
		var entry domain.RewriteEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil || entry.Chapter <= 0 {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

// RewritesByChapter gom log theo chương, giữ thứ tự ghi trong từng chương.
func (s *fileProgressStore) RewritesByChapter() (map[int][]domain.RewriteEntry, error) {
	entries, err := s.Rewrites()
	if err != nil {
		return nil, err
	}
	out := make(map[int][]domain.RewriteEntry, len(entries))
	for _, entry := range entries {
		out[entry.Chapter] = append(out[entry.Chapter], entry)
	}
	return out, nil
}
