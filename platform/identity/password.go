package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// argon2id 参数：64 MiB、3 轮、2 线程，单次约百毫秒；登录是低频操作，宁慢勿弱。
const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
)

// hashSlots 限制同时进行的 argon2 计算数，避免并发登录请求把内存打满。
var hashSlots = make(chan struct{}, 2)

// dummyHash 用于不存在的用户，保证"用户不存在"与"密码错误"耗时一致，不泄露用户名是否存在。
// 懒计算：包被 CLI 引入时不应在启动阶段付出一次 argon2 成本。
var dummyHash = sync.OnceValue(func() string {
	hash, err := hashPassword("ainovel-dummy-password")
	if err != nil {
		panic(err)
	}
	return hash
})

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hashSlots <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-hashSlots
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// verifyPassword 按哈希里记录的参数重新计算，参数调整后旧哈希仍可校验。
func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil || memory == 0 || iterations == 0 || threads == 0 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	hashSlots <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	<-hashSlots
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---- Backend helpers cho repository ngoài (postgres) ----
// Cùng một thuật toán/tham số argon2id — không viết biến thể thứ hai; backend
// khác gọi các hàm này để giữ hash tương thích byte-per-byte với file mode.

// HashPasswordForBackend băm mật khẩu/mã khôi phục bằng argon2id chuẩn.
func HashPasswordForBackend(password string) (string, error) { return hashPassword(password) }

// VerifyPasswordForBackend kiểm chuỗi argon2id đã lưu, giữ hashSlots bound.
func VerifyPasswordForBackend(password, encoded string) bool {
	return verifyPassword(password, encoded)
}

// DummyHashForBackend trả hash giả cho đường "user không tồn tại" (đồng giờ).
func DummyHashForBackend() string { return dummyHash() }

// DigestForBackend là SHA-256 hex dùng cho pairing/session secret derivation.
func DigestForBackend(secret string) string { return digest(secret) }

// NewRecoveryCodeForBackend sinh mã khôi phục 16 ký tự (crypto/rand).
func NewRecoveryCodeForBackend() (string, error) { return newRecoveryCode() }

// FormatRecoveryCodeForBackend nhóm mã thành XXXX-XXXX-XXXX-XXXX để trả một lần.
func FormatRecoveryCodeForBackend(raw string) string { return formatRecoveryCode(raw) }

// MaskRecoveryCodeForBackend tạo mask XXXX-••••-••••-XXXX cho RecoveryStatus.
func MaskRecoveryCodeForBackend(raw string) string { return maskRecoveryCode(raw) }

// NormalizeRecoveryCodeForBackend chuẩn hoá input trước khi verify.
func NormalizeRecoveryCodeForBackend(code string) (string, bool) {
	return normalizeRecoveryCode(code)
}
