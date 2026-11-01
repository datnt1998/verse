package imagegen_test

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
)

// TestDisplayLabelUsesCardLabel: nguồn khoá API (id thẻ kiểu
// "openai_compat-xxxx") hiện tên thẻ Kết nối AI thay vì id trong các dòng
// hạn mức và meta — id nội bộ không bao giờ lên giao diện.
func TestDisplayLabelUsesCardLabel(t *testing.T) {
	p := imagegen.NewAPIProvider(imagegen.APIDeps{
		Name: "openai_compat-61ed93e9", Label: "Tương thích OpenAI",
		Style: imagegen.StyleOpenAIImages,
	})
	if got := imagegen.DisplayLabel(p); got != "Tương thích OpenAI" {
		t.Fatalf("DisplayLabel=%q", got)
	}
	// Không nhãn thẻ → rơi về nhãn loại nguồn (và với id lạ là chính id).
	anon := imagegen.NewAPIProvider(imagegen.APIDeps{Name: "openai_compat-61ed93e9", Style: imagegen.StyleOpenAIImages})
	if got := imagegen.DisplayLabel(anon); got != "openai_compat-61ed93e9" {
		t.Fatalf("DisplayLabel rỗng nhãn=%q", got)
	}
}
