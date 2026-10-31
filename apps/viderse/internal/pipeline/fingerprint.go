package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// InputFingerprint là chữ ký xác định của đầu vào một step: output được dùng
// lại chỉ khi fingerprint khớp. Định dạng "sha256:<hex>" như bản cũ.
func InputFingerprint(input any) string {
	sum := sha256.Sum256([]byte(StableJSON(input)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// StableJSON marshal JSON với khoá sắp alphabet — Go's encoding/json sắp khoá
// map ở mọi cấp tự động, nên fingerprint không phụ thuộc thứ tự field.
func StableJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
