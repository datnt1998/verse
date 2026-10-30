package aiconn

// Body lỗi của provider không bao giờ được lọt nguyên văn (có thể chứa khoá);
// bộ lọc phải LUÔN dừng kể cả khi chuỗi khớp lồng nhau.

import (
	"strings"
	"testing"
)

func TestSanitizeTerminatesOnNestedMatches(t *testing.T) {
	// Bản cũ giữ "Bearer "/"sk-" trong kết quả nên khớp lại chính nó → lặp vô hạn.
	input := strings.Repeat("Bearer sk-secret-value ", 20000)
	done := make(chan string, 1)
	go func() { done <- sanitize([]byte(input)) }()
	out := <-done
	if strings.Contains(out, "sk-secret-value") {
		t.Fatalf("chuỗi bí mật còn sót: %q", out[:40])
	}
	if len(out) > 400 {
		t.Fatalf("thiếu trần độ dài: %d", len(out))
	}
}

func TestSanitizeRedactsKeysAndHeaders(t *testing.T) {
	cases := []string{
		`{"error":"invalid key sk-live-abc123456"}`,
		`Authorization: Bearer eyJhbGciOi.payload.sig`,
		`x-api-key: abcd1234efgh`,
		`{"api_key":"abcdef123456"}`,
	}
	for _, in := range cases {
		out := sanitize([]byte(in))
		if !strings.Contains(out, "***") {
			t.Fatalf("không che %q → %q", in, out)
		}
		for _, secret := range []string{"sk-live", "eyJhbGciOi", "abcd1234efgh", "abcdef123456"} {
			if strings.Contains(out, secret) {
				t.Fatalf("lọt %q trong %q", secret, out)
			}
		}
	}
}
