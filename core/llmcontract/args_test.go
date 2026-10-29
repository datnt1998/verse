package llmcontract

import (
	"encoding/json"
	"testing"
)

// TestEncodeArgsFillsRequiredProperties pin hình dạng tham số strict: hợp đồng
// tool bắt mọi thuộc tính required phải có mặt, nên giá trị zero bị omitempty
// bỏ đi phải được điền lại — null khi schema cho phép null, ngược lại là giá
// trị rỗng đúng kiểu.
func TestEncodeArgsFillsRequiredProperties(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"chapter", "scope", "notes", "tags", "issues"},
		"properties": map[string]any{
			"chapter": map[string]any{"type": "integer"},
			"scope":   map[string]any{"type": "string"},
			"notes":   map[string]any{"type": []string{"string", "null"}},
			"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"issues": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"description", "quote"},
					"properties": map[string]any{
						"description": map[string]any{"type": "string"},
						"quote":       map[string]any{"type": []string{"string", "null"}},
					},
				},
			},
		},
	}
	value := struct {
		Chapter int      `json:"chapter"`
		Scope   string   `json:"scope,omitempty"`
		Notes   string   `json:"notes,omitempty"`
		Tags    []string `json:"tags,omitempty"`
		Issues  []struct {
			Description string `json:"description"`
			Quote       string `json:"quote,omitempty"`
		} `json:"issues,omitempty"`
	}{Chapter: 3, Issues: []struct {
		Description string `json:"description"`
		Quote       string `json:"quote,omitempty"`
	}{{Description: "thiếu móc"}}}

	args, err := EncodeArgs(schema, value)
	if err != nil {
		t.Fatalf("EncodeArgs: %v", err)
	}
	if err := ValidateJSON(schema, args); err != nil {
		t.Fatalf("tham số phải hợp lệ với schema strict: %v\n%s", err, args)
	}
	var decoded struct {
		Notes  *string  `json:"notes"`
		Tags   []string `json:"tags"`
		Issues []struct {
			Quote *string `json:"quote"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(args, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Notes != nil {
		t.Fatalf("notes phải là null, got %v", *decoded.Notes)
	}
	if decoded.Tags == nil || len(decoded.Tags) != 0 {
		t.Fatalf("tags phải là mảng rỗng, got %#v", decoded.Tags)
	}
	if len(decoded.Issues) != 1 || decoded.Issues[0].Quote != nil {
		t.Fatalf("quote phải là null, got %#v", decoded.Issues)
	}
}

// TestEncodeArgsRejectsRequiredWithoutValue: thuộc tính required không nullable
// và không suy ra được giá trị rỗng thì phải báo lỗi thay vì bịa dữ liệu.
func TestEncodeArgsRejectsUnrepresentableZero(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"nested"},
		"properties": map[string]any{
			"nested": map[string]any{"type": "object", "additionalProperties": true},
		},
	}
	if _, err := EncodeArgs(schema, struct{}{}); err != nil {
		t.Fatalf("object rỗng hợp lệ, got %v", err)
	}
	broken := map[string]any{
		"type":     "object",
		"required": []string{"weird"},
		"properties": map[string]any{
			"weird": map[string]any{"description": "không khai báo type"},
		},
	}
	_, err := EncodeArgs(broken, struct{}{})
	if err == nil {
		t.Fatal("thiếu required không null và không có kiểu rỗng phải là lỗi")
	}
}
