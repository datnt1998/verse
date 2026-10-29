package assets

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// writerGoldenPath là đường dẫn tới baseline byte-perfect của prompt writer.
const writerGoldenPath = "testdata/writer-golden.md"

// TestWriterGolden_DefaultStyle khoá toàn bộ prompt writer của style `default`
// bằng cách so từng byte với assets/testdata/writer-golden.md.
// Chỉ sinh lại baseline khi UPDATE_WRITER_GOLDEN=1 (theo cách làm của ratchet màu).
func TestWriterGolden_DefaultStyle(t *testing.T) {
	// LoadOptions trống: không có thư mục ghi đè nào, ~/.ainovel không lọt vào.
	bundle := Load("default", LoadOptions{})
	got := BuildWriterPrompt(bundle.Prompts.Writer, bundle.Voice, bundle.Styles["default"])

	if os.Getenv("UPDATE_WRITER_GOLDEN") == "1" {
		writeFile(t, writerGoldenPath, got)
		return
	}

	want, err := os.ReadFile(writerGoldenPath)
	if err != nil {
		t.Fatalf("Thiếu baseline %s (chạy UPDATE_WRITER_GOLDEN=1 để sinh): %v", writerGoldenPath, err)
	}
	if string(want) != got {
		t.Fatalf("Prompt writer lệch baseline %s (chỉ cập nhật qua UPDATE_WRITER_GOLDEN=1):\n%s",
			writerGoldenPath, firstDiff(string(want), got))
	}
}

// firstDiff mô tả điểm lệch đầu tiên giữa hai chuỗi theo dòng.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	n := len(wl)
	if len(gl) < n {
		n = len(gl)
	}
	for i := range n {
		if wl[i] != gl[i] {
			return fmt.Sprintf("dòng %d:\n  muốn: %q\n  được: %q", i+1, wl[i], gl[i])
		}
	}
	return fmt.Sprintf("số dòng lệch nhau: muốn %d, được %d", len(wl), len(gl))
}
