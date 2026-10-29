package main

import (
	"strings"
	"testing"
)

// TestParseWorkerTokenCreate pins the documented `worker token create [--cap
// tts] <name>` shape: flags first, only schema-allowed capabilities, and a
// trailing flag is an error instead of becoming part of the machine name.
func TestParseWorkerTokenCreate(t *testing.T) {
	t.Run("flag_before_name", func(t *testing.T) {
		name, caps, err := parseWorkerTokenCreate([]string{"--cap", "tts", "mac-mini"})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if name != "mac-mini" || len(caps) != 1 || caps[0] != "tts" {
			t.Fatalf("name=%q caps=%v", name, caps)
		}
	})

	t.Run("default_capability", func(t *testing.T) {
		name, caps, err := parseWorkerTokenCreate([]string{"mac-mini"})
		if err != nil || name != "mac-mini" || len(caps) != 1 || caps[0] != "tts" {
			t.Fatalf("name=%q caps=%v err=%v", name, caps, err)
		}
	})

	t.Run("trailing_flag_is_rejected", func(t *testing.T) {
		_, _, err := parseWorkerTokenCreate([]string{"mac-mini", "--cap", "tts"})
		if err == nil || !strings.Contains(err.Error(), "trước tên máy") {
			t.Fatalf("trailing flag must be rejected, got %v", err)
		}
	})

	t.Run("capabilities_are_schema_bound", func(t *testing.T) {
		for _, c := range []string{"audio_export", "text_export", "tts,audio_export"} {
			if _, _, err := parseWorkerTokenCreate([]string{"--cap", c, "mac-mini"}); err == nil {
				t.Fatalf("capability %q must be rejected (schema allows only tts)", c)
			}
		}
	})

	t.Run("name_is_required", func(t *testing.T) {
		if _, _, err := parseWorkerTokenCreate([]string{"--cap", "tts"}); err == nil {
			t.Fatal("missing machine name must fail")
		}
	})
}
