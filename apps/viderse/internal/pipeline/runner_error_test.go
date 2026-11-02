package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode"
)

func TestAttemptErrorMessageKeepsCauseUnderVietnameseCopy(t *testing.T) {
	got := attemptErrorMessage(errors.New("exec: boom"))
	if !strings.Contains(got, "boom") {
		t.Fatalf("mất nguyên nhân chẩn đoán: %q", got)
	}
	if !hasVietnamese(got) {
		t.Fatalf("câu dẫn không phải tiếng Việt: %q", got)
	}
	if got == "exec: boom" {
		t.Fatal("lộ nguyên văn lỗi nội bộ làm câu hiện")
	}
}

func TestAttemptErrorMessageTranslatesDeadline(t *testing.T) {
	got := attemptErrorMessage(context.DeadlineExceeded)
	if strings.Contains(strings.ToLower(got), "deadline") || !hasVietnamese(got) {
		t.Fatalf("hết giờ vẫn hiện chữ nội bộ: %q", got)
	}
}

func hasVietnamese(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
