package chapterfacts

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/agent/schema"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

func TestQuotePropertyIsNullableRequired(t *testing.T) {
	obj := schema.Object(Properties(false)...)
	props, _ := obj["properties"].(map[string]any)
	// 直接核对三个容器的 item schema：quote 是 ["string","null"] 且列入 required。
	for _, key := range []string{"timeline_events", "foreshadow_updates", "state_changes"} {
		arr, ok := props[key].(map[string]any)
		if !ok {
			t.Fatalf("%s missing: %#v", key, props)
		}
		items, _ := arr["items"].(map[string]any)
		itemProps, _ := items["properties"].(map[string]any)
		q, ok := itemProps["quote"].(map[string]any)
		if !ok {
			t.Fatalf("%s item missing quote property: %#v", key, itemProps)
		}
		if got := fmt.Sprint(q["type"]); got != "[string null]" {
			t.Fatalf("%s.quote type = %v, want nullable string", key, q["type"])
		}
		required, _ := items["required"].([]string)
		if !slices.Contains(required, "quote") {
			t.Fatalf("%s.quote must be required (strict mode): %v", key, required)
		}
	}
}

// abandon 是合法的第四种动作，due_by / reason 都是可空且必须出现的字段；
// Validate 与 ApplyForeshadow 一样要求 abandon 带 reason。
func TestForeshadowAbandonSchemaAndValidation(t *testing.T) {
	obj := schema.Object(Properties(false)...)
	props, _ := obj["properties"].(map[string]any)
	arr, _ := props["foreshadow_updates"].(map[string]any)
	items, _ := arr["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)

	action, _ := itemProps["action"].(map[string]any)
	values, _ := action["enum"].([]string)
	if !slices.Contains(values, "abandon") {
		t.Fatalf("action enum must include abandon: %v", values)
	}
	dueBy, _ := itemProps["due_by"].(map[string]any)
	if got := fmt.Sprint(dueBy["type"]); got != "[integer null]" {
		t.Fatalf("due_by type = %v, want nullable integer", dueBy["type"])
	}
	reason, _ := itemProps["reason"].(map[string]any)
	if got := fmt.Sprint(reason["type"]); got != "[string null]" {
		t.Fatalf("reason type = %v, want nullable string", reason["type"])
	}
	required, _ := items["required"].([]string)
	for _, key := range []string{"due_by", "reason"} {
		if !slices.Contains(required, key) {
			t.Fatalf("%s must be required in strict mode: %v", key, required)
		}
	}

	facts := func(reason string) domain.ChapterFacts {
		return domain.ChapterFacts{
			Title: "Chương 3", Summary: "x", KeyEvents: []string{"e"},
			ForeshadowUpdates: []domain.ForeshadowUpdate{
				{ID: "fs", Action: "abandon", Reason: reason},
			},
		}
	}
	if err := Validate(facts("tuyến phụ bị cắt")); err != nil {
		t.Fatalf("abandon có reason phải hợp lệ: %v", err)
	}
	if err := Validate(facts("")); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("abandon thiếu reason phải bị từ chối: %v", err)
	}
}

// ValidateQuotes 只校验非空 quote：空/null 不拦；非空必须能定位到正文。
func TestValidateQuotes(t *testing.T) {
	text := "Lâm Uyển mở bức thư dưới ánh đèn dầu."

	facts := func(quote string) domain.ChapterFacts {
		return domain.ChapterFacts{
			Title: "Chương 1", Summary: "x", KeyEvents: []string{"e"},
			ForeshadowUpdates: []domain.ForeshadowUpdate{
				{ID: "fs", Action: "plant", Description: "d", Quote: quote},
			},
		}
	}

	if err := ValidateQuotes(text, facts("")); err != nil {
		t.Fatalf("empty quote must pass: %v", err)
	}
	if err := ValidateQuotes(text, facts("mở bức thư dưới ánh đèn dầu")); err != nil {
		t.Fatalf("located quote must pass: %v", err)
	}
	if err := ValidateQuotes(text, facts("câu không tồn tại")); err == nil ||
		!strings.Contains(err.Error(), "foreshadow_updates[0]") {
		t.Fatalf("missing quote must name the item: %v", err)
	}
	if err := ValidateQuotes(text, facts("ngắn")); err == nil ||
		!strings.Contains(err.Error(), "ít nhất") {
		t.Fatalf("short quote must be rejected: %v", err)
	}
}
