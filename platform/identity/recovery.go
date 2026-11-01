package identity

import (
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"time"
)

// 管理员恢复码：16 个 [A-Z0-9]，显示成 XXXX-XXXX-XXXX-XXXX，只在生成时返回一次。
// 存储与密码同一套 argon2id 哈希；一次性：成功恢复后记 UsedAt，必须重新生成。
const (
	recoveryAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	recoveryLen      = 16
	recoveryGroup    = 4
)

var (
	ErrInvalidRecovery = errors.New("Mã khôi phục không đúng hoặc đã dùng")
	ErrRecoveryExists  = errors.New("Đã có mã khôi phục chưa dùng — hãy tạo mã mới")
)

// RecoveryCode 是持久化的恢复码记录；CodeHash 不对外暴露。
type RecoveryCode struct {
	CodeHash string `json:"codeHash"`
	// Mask 是给人看的遮罩（TRAX-••••-••••-41ZN），只在生成时算一次；
	// 旧文件没有这个字段 → 省略，界面回落到全点遮罩。
	Mask      string     `json:"mask,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

// RecoveryStatus 是 GET /api/me/recovery 的视图；从未生成过时 CreatedAt 为 nil。
type RecoveryStatus struct {
	CreatedAt *time.Time `json:"createdAt"`
	Used      bool       `json:"used"`
	// Mask 只在还有未使用的码时返回（Dialogs › recovery 的遮罩文案）。
	Mask string `json:"mask,omitempty"`
}

// newRecoveryCode 用 crypto/rand 拒绝采样生成恢复码（36 字符，丢弃 >=252 的字节避免取模偏差）。
func newRecoveryCode() (string, error) {
	const limit = 256 - 256%len(recoveryAlphabet)
	out := make([]byte, 0, recoveryLen)
	buf := make([]byte, 32)
	for len(out) < recoveryLen {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) < limit && len(out) < recoveryLen {
				out = append(out, recoveryAlphabet[int(b)%len(recoveryAlphabet)])
			}
		}
	}
	return string(out), nil
}

// formatRecoveryCode 把 16 个字符按 4 个一组用 "-" 连接。
func formatRecoveryCode(raw string) string {
	parts := make([]string, 0, recoveryLen/recoveryGroup)
	for i := 0; i < len(raw); i += recoveryGroup {
		parts = append(parts, raw[i:i+recoveryGroup])
	}
	return strings.Join(parts, "-")
}

// normalizeRecoveryCode 去掉空格与连字符并转大写；格式不对（长度、字符集）返回 false。
func normalizeRecoveryCode(code string) (string, bool) {
	code = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	if len(code) != recoveryLen {
		return "", false
	}
	for _, c := range code {
		if !strings.ContainsRune(recoveryAlphabet, c) {
			return "", false
		}
	}
	return code, true
}

// maskRecoveryCode 生成遮罩文案：首组与末组可见，中间两组隐藏（Dialogs › recovery）。
func maskRecoveryCode(raw string) string {
	return raw[:recoveryGroup] + "-••••-••••-" + raw[len(raw)-recoveryGroup:]
}

// IssueRecovery 为管理员生成恢复码并返回明文（仅此一次）。
// rotate=false：已有未使用的码时返回 ErrRecoveryExists（客户端应改用 rotate）；
// rotate=true：无条件换成新码，旧码立即作废。
func (s *Store) IssueRecovery(username string, rotate bool) (string, time.Time, error) {
	if s.repo != nil {
		return s.repo.IssueRecovery(username, rotate)
	}
	d, err := s.snapshot()
	if err != nil {
		return "", time.Time{}, err
	}
	u := d.user(username)
	if u == nil {
		return "", time.Time{}, ErrNotFound
	}
	if u.Role != RoleAdmin {
		return "", time.Time{}, ErrForbidden
	}
	if !rotate && u.Recovery != nil && u.Recovery.UsedAt == nil {
		return "", time.Time{}, ErrRecoveryExists
	}
	raw, err := newRecoveryCode()
	if err != nil {
		return "", time.Time{}, err
	}
	hash, err := hashPassword(raw)
	if err != nil {
		return "", time.Time{}, err
	}
	var created time.Time
	err = s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		if u.Role != RoleAdmin {
			return ErrForbidden
		}
		if !rotate && u.Recovery != nil && u.Recovery.UsedAt == nil {
			return ErrRecoveryExists
		}
		created = s.now()
		u.Recovery = &RecoveryCode{CodeHash: hash, Mask: maskRecoveryCode(raw), CreatedAt: created}
		u.UpdatedAt = created
		s.addAudit(d, &AuditActor{Username: u.Username, Name: u.Name}, "sec", "Tạo mã khôi phục", username, "", "", "")
		return nil
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return formatRecoveryCode(raw), created, nil
}

// RecoveryStatus 返回账号恢复码的状态；从未生成过则 CreatedAt 为 nil。
func (s *Store) RecoveryStatus(username string) (RecoveryStatus, error) {
	if s.repo != nil {
		return s.repo.RecoveryStatus(username)
	}
	d, err := s.snapshot()
	if err != nil {
		return RecoveryStatus{}, err
	}
	u := d.user(username)
	if u == nil {
		return RecoveryStatus{}, ErrNotFound
	}
	if u.Recovery == nil {
		return RecoveryStatus{}, nil
	}
	created := u.Recovery.CreatedAt
	status := RecoveryStatus{CreatedAt: &created, Used: u.Recovery.UsedAt != nil}
	if !status.Used {
		status.Mask = u.Recovery.Mask
	}
	return status, nil
}

// Recover 用一次性恢复码重设管理员密码：校验通过后烧掉恢复码、改密、吊销该账号全部会话与
// 待兑换的配对码，并新建一个会话。返回被吊销的会话 ID，供调用方广播 session.revoked。
// 用户不存在、非管理员、已停用、无码、码已用、码错误一律返回 ErrInvalidRecovery，
// 且无论哪种都完整执行一次 argon2 校验，耗时一致。
func (s *Store) Recover(username, code, newPassword, device, ip string) (NewSession, []string, error) {
	if s.repo != nil {
		return s.repo.Recover(username, code, newPassword, device, ip)
	}

	if err := validatePassword(newPassword); err != nil {
		return NewSession{}, nil, err
	}
	d, err := s.snapshot()
	if err != nil {
		return NewSession{}, nil, err
	}
	username = strings.TrimSpace(username)
	raw, formatOK := normalizeRecoveryCode(code)
	hash := dummyHash()
	u := d.user(username)
	eligible := recoverable(u)
	if eligible {
		hash = u.Recovery.CodeHash
	}
	match := verifyPassword(raw, hash)
	if !match || !eligible || !formatOK {
		return NewSession{}, nil, ErrInvalidRecovery
	}
	newHash, err := hashPassword(newPassword)
	if err != nil {
		return NewSession{}, nil, err
	}
	var out NewSession
	var revoked []string
	err = s.update(func(d *data) error {
		// 校验与写入之间码可能已被别的请求用掉或被重新生成，写锁内再确认一次。
		current := d.user(username)
		if !recoverable(current) || current.Recovery.CodeHash != u.Recovery.CodeHash {
			return ErrInvalidRecovery
		}
		now := s.now()
		current.Recovery.UsedAt = &now
		current.PasswordHash = newHash
		current.MustChangePassword = false
		current.UpdatedAt = now
		current.PasswordChangedAt = &now
		s.addAudit(d, nil, "sec", "Khôi phục mật khẩu", username, "", "", "")
		for _, x := range d.Sessions {
			if x.Username == username {
				revoked = append(revoked, x.ID)
			}
		}
		d.dropSessions(func(x session) bool { return x.Username != username })
		d.dropDeviceTokens(username)
		d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return p.Username == username })
		var err error
		out, err = s.newSession(d, username, device, ip)
		return err
	})
	if err != nil {
		return NewSession{}, nil, err
	}
	return out, revoked, nil
}

// recoverable 判断账号当前是否允许用恢复码恢复：存在、管理员、未停用、有未使用的码。
func recoverable(u *User) bool {
	return u != nil && u.Role == RoleAdmin && !u.Disabled && u.Recovery != nil && u.Recovery.UsedAt == nil
}
