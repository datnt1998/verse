// Package secretbox niêm phong bí mật nhỏ của app (khoá API của nhà cung cấp
// AI) bằng AES-256-GCM với một khoá máy ngẫu nhiên 32 byte.
//
// Khoá máy sống trong <dir>/secret.key, quyền 0600, được tạo lần đầu dùng.
// Cơ sở dữ liệu chỉ giữ ciphertext (nonce ‖ sealed); API chỉ trả gợi ý
// "••••abcd". Sai khoá hoặc ciphertext hỏng là lỗi tường minh — không bao
// giờ trả rỗng lặng lẽ.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"syscall"
)

// ErrWrongKey báo ciphertext không mở được bằng khoá máy hiện tại — khoá file
// bị thay/mất hoặc dữ liệu được ghi bởi máy khác.
var ErrWrongKey = errors.New("khoá máy không giải mã được ciphertext")

// Box niêm phong và mở bí mật bằng khoá máy của app.
type Box struct {
	path string
}

// New tạo Box với khoá nằm tại <dir>/secret.key.
func New(dir string) *Box {
	return &Box{path: filepath.Join(dir, "secret.key")}
}

// Path trả đường dẫn tệp khoá (chẩn đoán, không in nội dung).
func (b *Box) Path() string { return b.path }

// Readiness is safe diagnostic metadata; it never exposes key bytes.
type Readiness struct {
	OK    bool   `json:"ok"`
	Path  string `json:"path"`
	Issue string `json:"issue,omitempty"`
}

type keyError struct {
	issue string
	err   error
}

func (e *keyError) Error() string { return e.err.Error() }
func (e *keyError) Unwrap() error { return e.err }

// Readiness uses the same validation and first-create lock as Seal/Reveal.
// Existing invalid keys are not chmodded, replaced or regenerated.
func (b *Box) Readiness() Readiness {
	_, err := b.key()
	out := Readiness{OK: err == nil, Path: b.path}
	if err != nil {
		out.Issue = "readonly_dir"
		var typed *keyError
		if errors.As(err, &typed) {
			out.Issue = typed.issue
		}
	}
	return out
}

func readKey(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, &keyError{"bad_permissions", fmt.Errorf("đọc khoá máy: %w", err)}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, &keyError{"bad_permissions", err}
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !ok || owner.Uid != uint32(os.Geteuid()) {
		return nil, &keyError{"bad_permissions", fmt.Errorf("tệp khoá máy phải thuộc người chạy và có quyền 0600")}
	}
	data, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil {
		return nil, &keyError{"bad_permissions", err}
	}
	return checkKey(path, data)
}

// key đọc khoá máy; khi chưa có thì tạo DUY NHẤT MỘT LẦN dưới khoá tệp —
// đọc lại trong khoá rồi mới sinh + ghi tệp tạm riêng + rename. Khoá đã có
// (kể cả do tiến trình khác vừa tạo trong lúc ta chờ khoá) không bao giờ bị
// ghi đè: ghi đè làm mọi ciphertext cũ không mở được nữa. Tệp hư hỏng
// (kích thước sai) là lỗi — tạo lại ngầm cũng mất dữ liệu theo cách âm thầm.
func (b *Box) key() ([]byte, error) {
	data, err := readKey(b.path)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("đọc khoá máy: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(b.path), 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục khoá: %w", err)
	}
	// Khoá tệp tuần tự hoá lần tạo đầu giữa các goroutine VÀ giữa serve với
	// CLI (flock tự nhả khi process chết). Khoá giữ rất ngắn — chỉ đọc/ghi
	// một tệp 32 byte.
	fl := flock.New(b.path + ".lock")
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("khoá tạo khoá máy: %w", err)
	}
	defer func() { _ = fl.Unlock() }()
	data, err = readKey(b.path)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("đọc khoá máy: %w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("sinh khoá máy: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(b.path), ".secret.key-*")
	if err != nil {
		return nil, fmt.Errorf("ghi khoá máy: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(key); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return nil, fmt.Errorf("ghi khoá máy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("ghi khoá máy: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("ghi khoá máy: %w", err)
	}
	if err := os.Rename(tmpName, b.path); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("ghi khoá máy: %w", err)
	}
	return key, nil
}

// checkKey bắt buộc khoá đúng 32 byte; tệp hỏng là lỗi tường minh.
func checkKey(path string, data []byte) ([]byte, error) {
	if len(data) != 32 {
		return nil, &keyError{"corrupt", fmt.Errorf("tệp khoá %s hỏng (độ dài %d ≠ 32): %w", path, len(data), ErrWrongKey)}
	}
	return data, nil
}

// Seal mã hoá plaintext; trả nonce ‖ ciphertext (nonce 12 byte ngẫu nhiên).
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	key, err := b.key()
	if err != nil {
		return nil, err
	}
	gcm, err := gcmOf(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("sinh nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Reveal mở ciphertext do Seal tạo. Sai khoá/hỏng dữ liệu → ErrWrongKey.
func (b *Box) Reveal(sealed []byte) ([]byte, error) {
	key, err := readKey(b.path)
	if err != nil {
		return nil, err
	}
	gcm, err := gcmOf(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("ciphertext quá ngắn (%d byte): %w", len(sealed), ErrWrongKey)
	}
	nonce, body := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("giải mã thất bại: %w", errors.Join(err, ErrWrongKey))
	}
	return plain, nil
}

// Hint trả gợi ý che khoá "••••<4 ký tự cuối>" cho API; khoá ngắn hơn 4 ký
// tự được che hoàn toàn.
func Hint(secret string) string {
	if len(secret) <= 4 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}

func gcmOf(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("khoá máy không hợp lệ: %w", err)
	}
	return cipher.NewGCM(block)
}
