package performance

import (
	"context"
	"errors"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

// SettingKeyDataAPI là khoá trong bảng settings giữ khoá Data API đã niêm phong.
const SettingKeyDataAPI = "youtube.dataApiKey"

// Settings là kho khoá/giá trị (postgres.SettingsStore thoả mãn).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
	Clear(ctx context.Context, key string) error
}

// Sealer niêm phong bí mật (secretbox.Box thoả mãn).
type Sealer interface {
	Seal(plaintext []byte) ([]byte, error)
	Reveal(sealed []byte) ([]byte, error)
}

var (
	// ErrKeyRejected: Google không nhận khoá.
	ErrKeyRejected = errors.New("Google không nhận khoá này. Kiểm tra lại khoá hoặc bật YouTube Data API v3 cho khoá.")
	// ErrKeyUnchecked: không kiểm tra được khoá (mất mạng, Google lỗi) nên chưa lưu.
	ErrKeyUnchecked = errors.New("Chưa kiểm tra được khoá lúc này. Thử lại khi có mạng.")
	// ErrKeyFormat: khoá trống hoặc có ký tự lạ.
	ErrKeyFormat = errors.New("Khoá không hợp lệ")
)

// checkVideoID là video công khai luôn tồn tại, dùng để thử khoá (1 đơn vị hạn mức).
const checkVideoID = "jNQXAC9IVRw"

// KeyStore giữ khoá Data API tuỳ chọn, niêm phong như khoá nhà cung cấp AI.
type KeyStore struct {
	Settings Settings
	Box      Sealer
	Data     *DataAPIReader
}

type storedKey struct {
	Enc  []byte `json:"enc"`
	Hint string `json:"hint"`
}

// Key trả khoá đã mở ("" = chưa cấu hình). Thoả KeyProvider.
func (k *KeyStore) Key(ctx context.Context) (string, error) {
	var v storedKey
	found, err := k.Settings.Get(ctx, SettingKeyDataAPI, &v)
	if err != nil || !found || len(v.Enc) == 0 {
		return "", err
	}
	plain, err := k.Box.Reveal(v.Enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Status cho biết đã có khoá chưa và gợi ý "••••abcd".
func (k *KeyStore) Status(ctx context.Context) (configured bool, hint string, err error) {
	var v storedKey
	found, err := k.Settings.Get(ctx, SettingKeyDataAPI, &v)
	if err != nil || !found || len(v.Enc) == 0 {
		return false, "", err
	}
	return true, v.Hint, nil
}

// Set kiểm tra khoá với Google rồi lưu (niêm phong). Khoá không vào log hay lỗi.
func (k *KeyStore) Set(ctx context.Context, key string) (hint string, err error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 || strings.ContainsAny(key, " \t\r\n\"'&?#/") {
		return "", ErrKeyFormat
	}
	if k.Data != nil {
		_, e := k.Data.ReadWithKey(ctx, key, checkVideoID)
		var rejected *KeyRejectedError
		switch {
		case e == nil, errors.Is(e, ErrVideoUnavailable):
		case errors.As(e, &rejected):
			return "", ErrKeyRejected
		default:
			return "", ErrKeyUnchecked
		}
	}
	enc, err := k.Box.Seal([]byte(key))
	if err != nil {
		return "", err
	}
	hint = secretbox.Hint(key)
	if err = k.Settings.Set(ctx, SettingKeyDataAPI, storedKey{Enc: enc, Hint: hint}); err != nil {
		return "", err
	}
	return hint, nil
}

// Clear xoá khoá; số liệu quay về đọc trang công khai.
func (k *KeyStore) Clear(ctx context.Context) error {
	return k.Settings.Clear(ctx, SettingKeyDataAPI)
}
