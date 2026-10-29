package webauth

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// GAP-41：首次运行（还没有任何账号）时的设置码。
// 码 8 个字符，去掉容易看错的 0 O 1 I L，显示成 XXXX-XXXX；明文只出现在 serve 的启动输出里，
// 内存里只留加盐哈希。设置码是"能进这台机器"的凭据，所以校验按常数时间比较、失败次数全局计数。
const (
	SetupCodeTTL      = 30 * time.Minute
	SetupMaxAttempts  = 5
	SetupLockDuration = 10 * time.Minute
	setupAlphabet     = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	setupLen          = 8
	setupGroup        = 4
)

var (
	ErrSetupWrong   = errors.New("Mã thiết lập chưa đúng")
	ErrSetupLocked  = fmt.Errorf("Đã thử sai %d lần, trang thiết lập tạm khoá", SetupMaxAttempts)
	ErrSetupExpired = errors.New("Mã thiết lập đã hết hạn — khởi động lại Noverse để có mã mới")
	ErrSetupToken   = errors.New("Mã thiết lập đã hết hạn — nhập mã mới để tiếp tục")
	// ErrSetupFormat: mã sai định dạng (không đủ 8 ký tự hợp lệ sau khi chuẩn hoá).
	// Câu 1: lỗi này không tính vào 5 lần thử.
	ErrSetupFormat = errors.New("Mã thiết lập phải gồm 8 ký tự, dạng XXXX-XXXX")
)

// SetupStatus 是 GET /api/setup 需要的状态；Expired 由码的到期时间算出。
type SetupStatus struct {
	ExpiresAt    time.Time
	Expired      bool
	AttemptsLeft int
	LockedUntil  time.Time
}

// SetupVerify 是一次 verify 的结果。出错时 AttemptsLeft / LockedUntil 仍有值，供响应体使用。
type SetupVerify struct {
	Token        string
	ExpiresAt    time.Time
	AttemptsLeft int
	LockedUntil  time.Time
}

// SetupState 是首次设置的内存状态（不落盘）。字段都只在锁内读写。
type SetupState struct {
	mu   sync.Mutex
	now  func() time.Time
	salt string

	codeHash    string
	expiresAt   time.Time
	attempts    int
	lockedUntil time.Time

	tokenHash      string
	tokenExpiresAt time.Time
}

// NewSetupState 生成一个新设置码并只保留其哈希；返回的明文只用于启动时打印一次。
func NewSetupState(now func() time.Time) (*SetupState, string, error) {
	if now == nil {
		now = time.Now
	}
	raw, err := newSetupCode()
	if err != nil {
		return nil, "", err
	}
	salt, err := randomHex(16)
	if err != nil {
		return nil, "", err
	}
	s := &SetupState{now: now, salt: salt, expiresAt: now().Add(SetupCodeTTL)}
	s.codeHash = s.hash(raw)
	return s, formatSetupCode(raw), nil
}

// newSetupCode 用 crypto/rand 拒绝采样生成 8 个字符（丢弃 >=248 的字节，避免取模偏差）。
func newSetupCode() (string, error) {
	const limit = 256 - 256%len(setupAlphabet)
	out := make([]byte, 0, setupLen)
	buf := make([]byte, 32)
	for len(out) < setupLen {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) < limit && len(out) < setupLen {
				out = append(out, setupAlphabet[int(b)%len(setupAlphabet)])
			}
		}
	}
	return string(out), nil
}

// formatSetupCode 把 8 个字符按 4 个一组用 "-" 连接，方便人读与抄写。
func formatSetupCode(raw string) string {
	parts := make([]string, 0, setupLen/setupGroup)
	for i := 0; i < len(raw); i += setupGroup {
		parts = append(parts, raw[i:i+setupGroup])
	}
	return strings.Join(parts, "-")
}

// normalizeSetupCode 去掉空格与连字符并转大写；长度或字符集不对返回 false。
func normalizeSetupCode(code string) (string, bool) {
	code = strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\t", "").Replace(strings.TrimSpace(code)))
	if len(code) != setupLen {
		return "", false
	}
	for _, c := range code {
		if !strings.ContainsRune(setupAlphabet, c) {
			return "", false
		}
	}
	return code, true
}

// hash 是加盐 SHA-256：设置码只有 40 位熵，盐让内存里的哈希不可直接穷举。
func (s *SetupState) hash(raw string) string { return digest(s.salt + raw) }

// unlockIfDue 在锁定期结束后清零失败计数（"锁结束或成功即重置"）。
func (s *SetupState) unlockIfDue(now time.Time) {
	if !s.lockedUntil.IsZero() && !s.lockedUntil.After(now) {
		s.lockedUntil = time.Time{}
		s.attempts = 0
	}
}

// Status 返回当前状态；已过期时 Expired 为 true。
func (s *SetupState) Status() SetupStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.unlockIfDue(now)
	st := SetupStatus{ExpiresAt: s.expiresAt, LockedUntil: s.lockedUntil, AttemptsLeft: SetupMaxAttempts - s.attempts}
	if st.AttemptsLeft < 0 {
		st.AttemptsLeft = 0
	}
	st.Expired = s.codeHash == "" || !now.Before(s.expiresAt)
	return st
}

// Verify 校验设置码；成功时签发一次性 setupToken（寿命与码的剩余时间相同）。
// 错误分支：ErrSetupExpired / ErrSetupLocked / ErrSetupWrong。
func (s *SetupState) Verify(code string) (SetupVerify, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.unlockIfDue(now)
	if s.codeHash == "" || !now.Before(s.expiresAt) {
		return SetupVerify{}, ErrSetupExpired
	}
	if s.lockedUntil.After(now) {
		return SetupVerify{AttemptsLeft: 0, LockedUntil: s.lockedUntil}, ErrSetupLocked
	}
	raw, ok := normalizeSetupCode(code)
	if !ok {
		// Câu 1: mã sai định dạng chưa phải một lần thử — không trừ lượt, không khoá.
		return SetupVerify{AttemptsLeft: SetupMaxAttempts - s.attempts}, ErrSetupFormat
	}
	if subtle.ConstantTimeCompare([]byte(s.hash(raw)), []byte(s.codeHash)) != 1 {
		s.attempts++
		if s.attempts >= SetupMaxAttempts {
			s.lockedUntil = now.Add(SetupLockDuration)
			return SetupVerify{AttemptsLeft: 0, LockedUntil: s.lockedUntil}, ErrSetupLocked
		}
		return SetupVerify{AttemptsLeft: SetupMaxAttempts - s.attempts}, ErrSetupWrong
	}
	s.attempts = 0
	token, err := randomHex(32)
	if err != nil {
		return SetupVerify{}, err
	}
	s.tokenHash = digest(token)
	s.tokenExpiresAt = s.expiresAt
	return SetupVerify{Token: token, ExpiresAt: s.expiresAt, AttemptsLeft: SetupMaxAttempts}, nil
}

// CheckToken 校验 setupToken 是否有效（未用掉、未过期）。只校验不消费：
// 创建失败时表单还能重试，只有创建成功才作废（Invalidate）。
func (s *SetupState) CheckToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.tokenHash == "" || token == "" || !now.Before(s.tokenExpiresAt) ||
		subtle.ConstantTimeCompare([]byte(digest(token)), []byte(s.tokenHash)) != 1 {
		return ErrSetupToken
	}
	return nil
}

// Invalidate 在管理员创建成功后清掉码、token 与计数：设置码与 token 都是一次性的。
func (s *SetupState) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codeHash, s.tokenHash = "", ""
	s.expiresAt, s.tokenExpiresAt = time.Time{}, time.Time{}
	s.attempts, s.lockedUntil = 0, time.Time{}
}

// ValidateSetupAdmin 校验首次设置表单，返回出错字段（username|name|password）与越南语文案。
func ValidateSetupAdmin(name, username, password string) (field, msg string) {
	if !usernamePattern.MatchString(strings.TrimSpace(username)) {
		return "username", "Tên đăng nhập chỉ gồm a-z, 0-9, . _ - (2–32 ký tự)"
	}
	if !validDisplayName(strings.TrimSpace(name)) {
		return "name", fmt.Sprintf("Tên hiển thị tối đa %d ký tự", MaxNameLen)
	}
	if err := validatePassword(password); err != nil {
		return "password", fmt.Sprintf("Mật khẩu cần ít nhất %d ký tự", MinPasswordLen)
	}
	return "", ""
}
