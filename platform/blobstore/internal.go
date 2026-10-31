package blobstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// writeSum fans writes out to the sink and a running SHA-256. Digest() is the
// canonical hash of everything written so far.
type writeSum struct {
	w io.Writer
	h hash.Hash
}

func newWriteSum(w io.Writer) *writeSum { return &writeSum{w: w, h: sha256.New()} }

func (s *writeSum) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	s.h.Write(p[:n])
	return n, err
}

// Digest returns the lowercase-hex digest of bytes written so far.
func (s *writeSum) Digest() string { return hex.EncodeToString(s.h.Sum(nil)) }

// digestReader hashes everything read from r until EOF.
func digestReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// errTooLargeReader bounds a stream: at max+1 bytes it surfaces ErrTooLarge
// instead of EOF so callers never commit a truncated object.
type errTooLargeReader struct {
	r     io.Reader
	left  int64
	total int64
}

func (b *errTooLargeReader) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, ErrTooLarge
	}
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	b.total += int64(n)
	if b.total > 0 && b.left < 0 {
		return n, ErrTooLarge
	}
	return n, err
}

// capReader returns r wrapped so reads beyond max bytes fail with ErrTooLarge.
func capReader(r io.Reader, max int64) io.Reader {
	return &errTooLargeReader{r: r, left: max}
}

var filenameBad = regexp.MustCompile(`[^\p{L}\p{N}._()\- ]+`)

// safeFilename renders a Content-Disposition safe name: path components
// stripped, dangerous characters collapsed, RFC 5987 fallback for non-ASCII.
func safeFilename(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\\`); i >= 0 {
		name = name[i+1:]
	}
	name = filenameBad.ReplaceAllString(name, "_")
	name = strings.Trim(name, " .")
	if name == "" {
		name = "download"
	}
	return truncateFilename(name, 128)
}

// truncateFilename caps name at limit bytes on a UTF-8 boundary so filename*
// is never malformed. The final extension is preserved whenever it fits
// beside at least one stem rune.
func truncateFilename(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		if e := name[i:]; utf8.ValidString(e) && len(e) <= limit-1 {
			stem, ext = name[:i], e
		}
	}
	stem = cutUTF8(stem, limit-len(ext))
	if stem == "" {
		// No room for even one stem rune: the extension is not worth keeping.
		stem, ext = cutUTF8("download", limit), ""
	}
	return stem + ext
}

// cutUTF8 returns the longest prefix of s that fits in limit bytes without
// splitting a rune.
func cutUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	i := limit
	for i > 0 && !utf8.ValidString(s[:i]) {
		i--
	}
	return s[:i]
}

// ContentDisposition builds an attachment/inline header value that is safe for
// both ASCII and UTF-8 names.
func ContentDisposition(kind, name string) string {
	name = safeFilename(name)
	ascii := true
	for _, r := range name {
		if r > 127 {
			ascii = false
			break
		}
	}
	if ascii {
		return fmt.Sprintf(`%s; filename="%s"`, kind, name)
	}
	// ASCII fallback plus RFC 5987 UTF-8 name.
	fallback := filenameBad.ReplaceAllString(
		strings.Map(func(r rune) rune {
			if r > 127 {
				return '_'
			}
			return r
		}, name), "_")
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, kind, fallback, pctEncode(name))
}

func pctEncode(s string) string {
	const hexchars = "0123456789ABCDEF"
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == '_' || c == '~' || c == '(' || c == ')' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexchars[c>>4])
		b.WriteByte(hexchars[c&0xf])
	}
	return b.String()
}
