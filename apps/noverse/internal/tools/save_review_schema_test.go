package tools

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/core/llmcontract"
)

var updateSchemaGolden = flag.Bool("update", false, "ghi lại golden testdata")

// TestSaveReviewSchemaGolden pin nguyên văn schema strict của save_review — hợp
// đồng JSON duy nhất giữa LLM và bản ghi thẩm duyệt. Hình dạng của
// domain.ReviewEntry có thể đổi mà không được kéo schema đổi theo, nên schema
// thủ công có golden riêng để mọi thay đổi hợp đồng đều phải cố ý.
func TestSaveReviewSchemaGolden(t *testing.T) {
	got, err := json.MarshalIndent(NewSaveReviewTool(nil, nil).Schema(), "", "  ")
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "save_review_schema.json")
	if *updateSchemaGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("schema save_review khác %s; nếu thay đổi là chủ đích, chạy `go test ./internal/tools -run TestSaveReviewSchemaGolden -update` rồi rà lại hợp đồng LLM\ngot:\n%s", path, got)
	}
}

// TestSaveReviewArgsSatisfyStrictSchema pin chiều ngược lại: fixture dựng từ
// domain.ReviewEntry phải là tham số hợp lệ của chính schema strict đó, kể cả
// các thuộc tính required mà bản ghi lưu trữ bỏ trống (null).
func TestSaveReviewArgsSatisfyStrictSchema(t *testing.T) {
	tool := NewSaveReviewTool(nil, nil)
	args := reviewArgs(t, domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "accept", Summary: "Đã đọc chương",
		Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 80, Comment: "Đã đọc chương"}},
		Issues:     []domain.ConsistencyIssue{{Type: "hook", Severity: domain.SeverityWarning, Description: "Móc cuối chương nhạt", Evidence: "trích nguyên văn"}},
	})
	if err := llmcontract.ValidateJSON(tool.Schema(), args); err != nil {
		t.Fatalf("tham số fixture phải hợp lệ với schema strict: %v\n%s", err, args)
	}
}
