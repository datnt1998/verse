package performance

import "context"

// SettingKeyDataAPI là khoá trong bảng settings giữ khoá Data API đã niêm phong.
const SettingKeyDataAPI = "youtube.dataApiKey"

// Settings là kho khoá/giá trị (postgres.SettingsStore thoả mãn).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
}

// Sealer mở bí mật đã niêm phong (secretbox.Box thoả mãn).
type Sealer interface {
	Reveal(sealed []byte) ([]byte, error)
}

// KeyStore đọc khoá Data API đã cấu hình trước đây, niêm phong như khoá nhà
// cung cấp AI. Không còn đường UI/API ghi khoá mới.
type KeyStore struct {
	Settings Settings
	Box      Sealer
}

type storedKey struct {
	Enc []byte `json:"enc"`
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
