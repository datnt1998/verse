package secretbox

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSealRevealRoundTrip(t *testing.T) {
	box := New(t.TempDir())
	secret := "sk-test-AbCdEfGh1234"
	sealed, err := box.Seal([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(secret)) {
		t.Fatal("ciphertext chứa plaintext")
	}
	got, err := box.Reveal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != secret {
		t.Fatalf("reveal = %q", got)
	}
	// Hai lần seal cho ciphertext khác nhau (nonce ngẫu nhiên).
	sealed2, _ := box.Seal([]byte(secret))
	if bytes.Equal(sealed, sealed2) {
		t.Fatal("hai ciphertext trùng nhau — nonce không ngẫu nhiên")
	}
}

func TestKeyFileCreatedWith0600(t *testing.T) {
	dir := t.TempDir()
	box := New(dir)
	if _, err := box.Seal([]byte("x")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("quyền khoá = %o", info.Mode().Perm())
	}
}

func TestWrongKeyFileIsExplicitError(t *testing.T) {
	dir := t.TempDir()
	box := New(dir)
	sealed, err := box.Seal([]byte("sk-secret"))
	if err != nil {
		t.Fatal(err)
	}
	// Thay khoá máy bằng một khoá khác.
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), bytes.Repeat([]byte{7}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := box.Reveal(sealed)
	if err == nil {
		t.Fatalf("sai khoá vẫn mở được: %q", got)
	}
	if !errors.Is(err, ErrWrongKey) {
		t.Fatalf("lỗi phải là ErrWrongKey: %v", err)
	}
	if got != nil {
		t.Fatal("sai khoá trả dữ liệu — phải lỗi rõ ràng")
	}
}

func TestCorruptKeyFileIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), []byte("ngắn"), 0o600); err != nil {
		t.Fatal(err)
	}
	box := New(dir)
	if _, err := box.Seal([]byte("x")); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("khoá hỏng phải lỗi ErrWrongKey: %v", err)
	}
}

func TestCorruptCiphertextIsError(t *testing.T) {
	box := New(t.TempDir())
	sealed, err := box.Seal([]byte("dữ liệu"))
	if err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 0xff
	if _, err := box.Reveal(sealed); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("ciphertext sửa đổi phải lỗi: %v", err)
	}
	if _, err := box.Reveal([]byte("quá ngắn")); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("ciphertext ngắn phải lỗi: %v", err)
	}
}

func TestHint(t *testing.T) {
	if got := Hint("sk-live-abc7c2e"); got != "••••7c2e" {
		t.Fatalf("hint = %q", got)
	}
	if got := Hint("abc"); got != "••••" {
		t.Fatalf("hint ngắn = %q", got)
	}
}

// Tạo khoá lần đầu là DUY NHẤT MỘT LẦN kể cả khi nhiều goroutine (và nhiều
// process) cùng khởi động lạnh: tệp khoá đã có không bao giờ bị ghi đè, nếu
// không mọi ciphertext cũ mất khả năng mở.
func TestConcurrentFirstUseKeepsOneKey(t *testing.T) {
	dir := t.TempDir()
	type result struct {
		sealed []byte
		err    error
	}
	boxes := make([]*Box, 8)
	for i := range boxes {
		boxes[i] = New(dir)
	}
	out := make(chan result, len(boxes))
	var wg sync.WaitGroup
	for _, b := range boxes {
		wg.Add(1)
		go func(b *Box) {
			defer wg.Done()
			sealed, err := b.Seal([]byte("sk-shared"))
			out <- result{sealed, err}
		}(b)
	}
	wg.Wait()
	close(out)

	first := (*result)(nil)
	for r := range out {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if first == nil {
			first = &r
			continue
		}
		got, err := New(dir).Reveal(r.sealed)
		if err != nil || string(got) != "sk-shared" {
			t.Fatalf("khoá bị ghi đè giữa chừng: %v %q", err, got)
		}
	}
	if _, err := New(dir).Reveal(first.sealed); err != nil {
		t.Fatalf("ciphertext của lần tạo đầu không mở được: %v", err)
	}
}
