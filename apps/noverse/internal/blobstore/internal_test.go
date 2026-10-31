package blobstore

import (
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

// filenameStar extracts and percent-decodes the RFC 5987 filename* value a
// browser would hand to decodeURIComponent.
func filenameStar(t *testing.T, header string) string {
	t.Helper()
	const mark = `filename*=UTF-8''`
	i := strings.Index(header, mark)
	if i < 0 {
		t.Fatalf("missing filename*: %q", header)
	}
	raw := header[i+len(mark):]
	if j := strings.IndexByte(raw, ';'); j >= 0 {
		raw = raw[:j]
	}
	dec, err := url.PathUnescape(raw)
	if err != nil {
		t.Fatalf("filename* unescape: %v", err)
	}
	return dec
}

// TestContentDispositionUTF8Boundary: the 128-byte cap must never split a
// multibyte rune — the value is advertised as UTF-8 and the frontend calls
// decodeURIComponent on it. The extension must survive truncation.
func TestContentDispositionUTF8Boundary(t *testing.T) {
	// 43 copies of the 3-byte letter 'ế' = 129 bytes: byte 128 falls inside
	// the last rune on both sides of the cap when an extension is kept.
	long := strings.Repeat("ế", 43)
	for _, tc := range []struct{ name, wantExt string }{
		{long, ""},
		{long + ".wav", ".wav"},
		{long + ".epub", ".epub"},
	} {
		header := ContentDisposition("attachment", tc.name)
		decoded := filenameStar(t, header)
		if !utf8.ValidString(decoded) {
			t.Errorf("%q → filename* is not valid UTF-8: %q", header, decoded)
		}
		if len(decoded) > 128 {
			t.Errorf("filename = %d bytes, want ≤128", len(decoded))
		}
		if tc.wantExt != "" && !strings.HasSuffix(decoded, tc.wantExt) {
			t.Errorf("extension lost under truncation: %q", decoded)
		}
	}
	// Exactly 128 bytes with the last rune intact must pass through unchanged.
	exact := strings.Repeat("ế", 42) + "é"
	if got := filenameStar(t, ContentDisposition("attachment", exact)); got != exact {
		t.Errorf("boundary name changed: %q", got)
	}
}
