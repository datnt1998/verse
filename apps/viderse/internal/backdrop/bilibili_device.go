package backdrop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// deviceCookiePattern là hình cookie buvid: không dấu ; khoảng trắng, ≤ 256.
var deviceCookiePattern = regexp.MustCompile(`^[^;\s]{1,256}$`)
var wbiKeyPattern = regexp.MustCompile(`(?i)^[a-f\d]{32}$`)

// deviceState là danh tính trình duyệt ẩn danh dùng chung cho danh mục, đăng
// nhập và yt-dlp, kèm khoá ký WBI đệm 24 giờ.
type deviceState struct {
	Buvid3       string `json:"buvid3"`
	Buvid4       string `json:"buvid4"`
	ImgKey       string `json:"imgKey,omitempty"`
	SubKey       string `json:"subKey,omitempty"`
	KeysExpireAt int64  `json:"keysExpireAt,omitempty"` // mili giây
}

// deviceStore giữ tệp danh tính thiết bị (0600) dưới thư mục cấu hình app.
type deviceStore struct {
	path string
	mu   sync.Mutex
}

// newDeviceStore đặt tệp danh tính dưới cfgDir.
func newDeviceStore(cfgDir string) *deviceStore {
	return &deviceStore{path: filepath.Join(cfgDir, "bilibili-device.json")}
}

// load đọc danh tính đã lưu, tạo mới (0600) khi chưa có. Tệp hỏng là lỗi thật
// — không lặng lẽ thay danh tính đang dùng.
func (d *deviceStore) load() (deviceState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	raw, err := os.ReadFile(d.path)
	if err == nil {
		var state deviceState
		if err := json.Unmarshal(raw, &state); err != nil {
			return deviceState{}, fmt.Errorf("đọc %s: %w", d.path, err)
		}
		if !deviceCookiePattern.MatchString(state.Buvid3) || !deviceCookiePattern.MatchString(state.Buvid4) {
			return deviceState{}, fmt.Errorf("đọc %s: danh tính thiết bị không hợp lệ", d.path)
		}
		return state, nil
	}
	if !os.IsNotExist(err) {
		return deviceState{}, fmt.Errorf("đọc %s: %w", d.path, err)
	}
	state := deviceState{Buvid3: newBuvid(), Buvid4: newBuvid()}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return deviceState{}, fmt.Errorf("tạo thư mục cấu hình: %w", err)
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return deviceState{}, err
	}
	file, err := os.OpenFile(d.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return d.loadLocked()
		}
		return deviceState{}, fmt.Errorf("tạo %s: %w", d.path, err)
	}
	if _, err := file.Write(payload); err != nil {
		file.Close()
		return deviceState{}, fmt.Errorf("ghi %s: %w", d.path, err)
	}
	if err := file.Close(); err != nil {
		return deviceState{}, err
	}
	return state, nil
}

// loadLocked đọc lại tệp khi một process khác vừa tạo trước (đã giữ mutex).
func (d *deviceStore) loadLocked() (deviceState, error) {
	raw, err := os.ReadFile(d.path)
	if err != nil {
		return deviceState{}, fmt.Errorf("đọc %s: %w", d.path, err)
	}
	var state deviceState
	if err := json.Unmarshal(raw, &state); err != nil {
		return deviceState{}, fmt.Errorf("đọc %s: %w", d.path, err)
	}
	return state, nil
}

// save ghi danh tính qua tệp tạm rồi đổi tên, quyền 0600.
func (d *deviceStore) save(state deviceState) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temp := fmt.Sprintf("%s.%s.tmp", d.path, hex.EncodeToString(randomBytes(6)))
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(temp, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, d.path); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

func newBuvid() string {
	return strings.ToUpper(hex.EncodeToString(randomBytes(16))) + "infoc"
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return b
}

// keysFresh đúng khi khoá đệm còn hạn và đúng hình.
func (s deviceState) keysFresh(now time.Time) bool {
	return wbiKeyPattern.MatchString(s.ImgKey) && wbiKeyPattern.MatchString(s.SubKey) && s.KeysExpireAt > now.UnixMilli()
}
