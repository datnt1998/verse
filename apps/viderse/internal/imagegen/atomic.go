package imagegen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// writeAtomicExclusive ghi bytes vào target nguyên tử (temp trong cùng thư
// mục + link) và KHÔNG ghi đè: target đã tồn tại → lỗi EEXIST, tương đương
// flag "wx"/COPYFILE_EXCL của bản cũ.
func writeAtomicExclusive(target string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// link() thất bại nguyên tử khi target đã có — đúng nghĩa "wx".
	if err := os.Link(tmpName, target); err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("output đã tồn tại: %w", fs.ErrExist)
		}
		return err
	}
	os.Remove(tmpName)
	ok = true
	return nil
}

// copyAtomicExclusive đọc src và ghi vào target nguyên tử không ghi đè.
func copyAtomicExclusive(src, target string, perm fs.FileMode) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(MaxImageBytes)+1))
	if err != nil {
		return err
	}
	return writeAtomicExclusive(target, data, perm)
}

// writeJSONAtomic ghi JSON của v vào file nguyên tử (temp + rename; cho phép
// ghi đè — dùng cho receipt sidecar/scratch).
func writeJSONAtomic(file string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, file); err != nil {
		return err
	}
	ok = true
	return nil
}

// sha256File trả hex SHA-256 của tệp.
func sha256File(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
