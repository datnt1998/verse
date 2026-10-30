// Package retention biết mọi thứ tạm Viderse để lại trên đĩa và khi nào nó
// hết dùng — bản Go của `src/server/retention.ts` và `editor/media-gc.ts`
// (yt-factory 2c201e6), áp lên layout thư mục dự án của Viderse.
//
// Nguyên tắc:
//   - Dọn tạm (lúc mở + mỗi 6 giờ) chỉ đụng thứ chắc chắn bỏ: thư mục tạm của
//     owner đã chết, bản ghi yêu cầu xuất đã xong quá hạn, bản xuất chưa chốt
//     quá 24 giờ, media chờ dọn của lần xoá trước.
//   - Dọn tay (Cài đặt › Lưu trữ) chạy thêm media không còn ai giữ, và luôn
//     xem trước trước khi xoá.
//   - Video đang chạy, video chờ duyệt và mọi thứ template/editor đang giữ
//     không bao giờ bị dọn.
package retention

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// errNotExist là tiện ích nội bộ.
func notExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// readJSON đọc một tệp JSON vào out; lỗi hoặc tệp hỏng trả false.
func readJSON(path string, out any) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, out) == nil
}

// writeJSONAtomic ghi JSON qua tệp tạm + rename (không bao giờ để tệp cụt).
func writeJSONAtomic(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// uuidToken là mẫu id media nằm trong JSON dự án (một chiều: dự án → media).
var uuidToken = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// tokensIn đọc mọi UUID trong một tệp văn bản (chữ thường). Lỗi đọc trả ra —
// một tệp tham chiếu không đọc được phải chặn dọn (fail closed), không được
// coi như "không có tham chiếu".
func tokensIn(path string, into map[string]bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, token := range uuidToken.FindAll(raw, -1) {
		into[strings.ToLower(string(token))] = true
	}
	return nil
}

// portableID là tên thư mục an toàn (giống assertPortableId của bản cũ).
var portableID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// isPortableID báo một tên dùng được làm id thư mục.
func isPortableID(name string) bool { return portableID.MatchString(name) }

// randID trả một mã hex ngẫu nhiên (preview id, khoá tạm).
func randID(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "0"
	}
	return hex.EncodeToString(buf)
}

// keyedLocks khoá theo tên trong một process: dự án và media cùng tên không
// bao giờ bị hai lần dọn chạm cùng lúc. App một process nên đây là đủ; ý định
// giữ nguyên như lock thư mục của bản cũ (chỉ khác chỗ ghi).
type keyedLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newKeyedLocks() *keyedLocks { return &keyedLocks{locks: map[string]*sync.Mutex{}} }

func (k *keyedLocks) lock(key string) func() {
	k.mu.Lock()
	m, ok := k.locks[key]
	if !ok {
		m = &sync.Mutex{}
		k.locks[key] = m
	}
	k.mu.Unlock()
	m.Lock()
	return m.Unlock
}
