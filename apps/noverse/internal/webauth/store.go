// Package webauth 管理 Web 界面（ainovel-cli serve）的多用户账号、登录会话与扫码配对。
// 数据存放在 ~/.ainovel/web-accounts.json（0600），跨进程用 flock 串行写入、原子替换；
// serve 进程按文件身份与修改时间缓存读取，CLI 修改账号后无需重启 serve 即生效。
package webauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gofrs/flock"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

const (
	SessionTTL     = 30 * 24 * time.Hour
	PairingTTL     = 5 * time.Minute
	MinPasswordLen = 8
	MaxNameLen     = 60
	touchInterval  = 10 * time.Minute
	fileVersion    = 4
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

var (
	ErrInvalidCredentials = errors.New("Sai tên đăng nhập hoặc mật khẩu.")
	ErrInvalidPairing     = errors.New("Mã QR không hợp lệ, đã dùng hoặc đã hết hạn.")
	ErrUnauthenticated    = errors.New("Chưa đăng nhập hoặc phiên đã hết hạn.")
	ErrNotFound           = errors.New("Không tìm thấy tài khoản hoặc thiết bị.")
	ErrExists             = errors.New("Tài khoản đã tồn tại.")
	ErrLastAdmin          = errors.New("Phải còn ít nhất một quản trị viên đang hoạt động.")
	ErrInvalidInput       = errors.New("Dữ liệu không hợp lệ")
	ErrForbidden          = errors.New("Bạn không có quyền thực hiện thao tác này.")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{2,32}$`)

// ValidUsername bounds unauthenticated limiter/audit targets to account identifiers.
func ValidUsername(username string) bool {
	return len(username) >= 2 && len(username) <= 32 && usernamePattern.MatchString(username)
}

// User 是持久化的账号；PasswordHash 只在本包内使用，对外一律用 View。
type User struct {
	Username           string                 `json:"username"`
	Name               string                 `json:"name"`
	Role               Role                   `json:"role"`
	Projects           map[string]ProjectRole `json:"projects"`
	Disabled           bool                   `json:"disabled"`
	PasswordHash       string                 `json:"passwordHash"`
	OnboardedAt        *time.Time             `json:"onboardedAt"`
	MustChangePassword bool                   `json:"mustChangePassword"`
	DisabledAt         *time.Time             `json:"disabledAt,omitempty"`
	DisabledBy         string                 `json:"disabledBy,omitempty"`
	DisabledReason     string                 `json:"disabledReason,omitempty"`
	PasswordChangedAt  *time.Time             `json:"passwordChangedAt,omitempty"`
	// TempPassword 是管理员签发的一次性临时密码（file v4）；不进入 View。
	// 登录优先用它；签发时永久哈希清空，ChangePassword 成功后二者交换。
	TempPassword *TempPassword `json:"tempPassword,omitempty"`
	// DeletedAt 是软删除墓碑；被删账号不再登录/出现，但其用户名永不复用
	// （与 PG citext unique across soft-deleted rows 一致，防接管）。
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
	// Recovery 是管理员的一次性恢复码（只存哈希），见 recovery.go；不进入 View。
	Recovery  *RecoveryCode `json:"recovery,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type UserView struct {
	Username           string                 `json:"username"`
	Name               string                 `json:"name"`
	Role               Role                   `json:"role"`
	Projects           map[string]ProjectRole `json:"projects"`
	Disabled           bool                   `json:"disabled"`
	OnboardedAt        *time.Time             `json:"onboardedAt"`
	MustChangePassword bool                   `json:"mustChangePassword"`
	DisabledAt         *time.Time             `json:"disabledAt,omitempty"`
	DisabledBy         string                 `json:"disabledBy,omitempty"`
	DisabledReason     string                 `json:"disabledReason,omitempty"`
	PasswordChangedAt  *time.Time             `json:"passwordChangedAt,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// UserListView 是 GET /api/users 使用的视图，在 UserView 基础上附带会话统计。
type UserListView struct {
	UserView
	LastSeenAt   *time.Time `json:"lastSeenAt"`
	SessionCount int        `json:"sessionCount"`
}

func (u User) View() UserView {
	projects := u.Projects
	if projects == nil {
		projects = map[string]ProjectRole{}
	}
	return UserView{Username: u.Username, Name: u.Name, Role: u.Role, Projects: projects, Disabled: u.Disabled, OnboardedAt: u.OnboardedAt, MustChangePassword: u.MustChangePassword, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, DisabledAt: u.DisabledAt, DisabledBy: u.DisabledBy, DisabledReason: u.DisabledReason, PasswordChangedAt: u.PasswordChangedAt}
}

type ProjectRole string

const (
	ProjectView   ProjectRole = "view"
	ProjectCollab ProjectRole = "collab"
	ProjectOwner  ProjectRole = "owner"
)

func (u User) ProjectRole(project string) ProjectRole {
	if u.Disabled || u.DeletedAt != nil {
		return ""
	}
	if u.Role == RoleAdmin {
		return ProjectOwner
	}
	return u.Projects[project]
}
func storyProject(id string) string {
	if p, _, ok := strings.Cut(id, "/"); ok {
		return p
	}
	return library.LegacyProjectID
}
func (u User) CanRead(id string) bool  { return u.CanReadProject(storyProject(id)) }
func (u User) CanWrite(id string) bool { return u.CanCollaborateProject(storyProject(id)) }
func (u User) CanReadProject(p string) bool {
	level := u.ProjectRole(p)
	return level == ProjectView || level == ProjectCollab || level == ProjectOwner
}
func (u User) CanCollaborateProject(p string) bool {
	level := u.ProjectRole(p)
	return level == ProjectCollab || level == ProjectOwner
}
func (u User) CanManageProject(p string) bool { return u.ProjectRole(p) == ProjectOwner }
func (u User) CanCreateStories() bool {
	if u.Disabled {
		return false
	}
	if u.Role == RoleAdmin {
		return true
	}
	for p := range u.Projects {
		if u.CanCollaborateProject(p) {
			return true
		}
	}
	return false
}

type session struct {
	ID         string    `json:"id"`
	SecretHash string    `json:"secretHash"`
	Username   string    `json:"username"`
	Device     string    `json:"device"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeen   time.Time `json:"lastSeen"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// Session 是会话的对外视图，不含密钥哈希。
type Session struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Device    string    `json:"device"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
	LastSeen  time.Time `json:"lastSeen"`
	ExpiresAt time.Time `json:"expiresAt"`
	Current   bool      `json:"current"`
}

func (s session) view(currentID string) Session {
	return Session{ID: s.ID, Username: s.Username, Device: s.Device, IP: s.IP, CreatedAt: s.CreatedAt, LastSeen: s.LastSeen, ExpiresAt: s.ExpiresAt, Current: s.ID == currentID}
}

type pairing struct {
	CodeHash  string    `json:"codeHash"`
	Username  string    `json:"username"`
	CreatedBy string    `json:"createdBy"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type data struct {
	Version           int          `json:"version"`
	Users             []User       `json:"users"`
	Sessions          []session    `json:"sessions"`
	Pairings          []pairing    `json:"pairings"`
	Audit             []AuditEntry `json:"audit"`
	legacyPermissions map[string]legacyPermission
}

func (d *data) user(username string) *User {
	for i := range d.Users {
		if d.Users[i].Username == username && d.Users[i].DeletedAt == nil {
			return &d.Users[i]
		}
	}
	return nil
}

// userAny 命中包括软删除墓碑在内的一切账号行（唯一性/防复用检查用）。
func (d *data) userAny(username string) *User {
	for i := range d.Users {
		if d.Users[i].Username == username {
			return &d.Users[i]
		}
	}
	return nil
}

// activeOwners 统计 project 未停用的 owner 数（admin 隐式 owner 不计入）。
func (d *data) activeOwners(project string) int {
	n := 0
	for _, u := range d.Users {
		if u.DeletedAt == nil && !u.Disabled && u.Role != RoleAdmin && u.Projects[project] == ProjectOwner {
			n++
		}
	}
	return n
}

func (d *data) dropSessions(keep func(session) bool) {
	d.Sessions = slices.DeleteFunc(d.Sessions, func(s session) bool { return !keep(s) })
}

func (d *data) enabledAdmins() int {
	n := 0
	for _, u := range d.Users {
		if u.Role == RoleAdmin && !u.Disabled && u.DeletedAt == nil {
			n++
		}
	}
	return n
}

// NewSession 是新建会话的结果；Cookie 是写进浏览器的完整凭据（id.secret），只出现这一次。
type NewSession struct {
	Cookie    string
	ExpiresAt time.Time
	User      UserView
}

type Store struct {
	path       string
	now        func() time.Time
	mu         sync.Mutex
	cache      *data
	stat       os.FileInfo
	projectIDs func() ([]string, error)
	// repo 非空时 Store 是门面，一切方法委派给外部 backend（postgres）。
	repo Repository
	// loginLimiter/recoveryLimiter được LoginLimiter()/RecoveryLimiter() khởi
	// tạo lười: DB buckets trên postgres, RAM trên file mode.
	loginLimiter    LoginLimiter
	recoveryLimiter RecoveryLimiter
}

func NewStore(path string) *Store { return &Store{path: path, now: time.Now} }

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return filepath.Join(home, ".ainovel", "web-accounts.json"), nil
}

func (s *Store) Path() string {
	if s.repo != nil {
		return "postgres (DATABASE_URL)"
	}
	return s.path
}

// snapshot uses the current file identity, never session-cached claims.
func (s *Store) snapshot() (*data, error) {
	s.mu.Lock()
	st, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.mu.Unlock()
		return &data{Version: fileVersion}, nil
	}
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if s.cache != nil && s.cache.Version == fileVersion && s.stat != nil && os.SameFile(s.stat, st) && s.stat.ModTime().Equal(st.ModTime()) && s.stat.Size() == st.Size() {
		d := s.cache
		s.mu.Unlock()
		return d, nil
	}
	d, err := s.read()
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if d.Version < fileVersion {
		s.mu.Unlock()
		if err := s.update(nil); err != nil {
			return nil, err
		}
		s.mu.Lock()
		d = s.cache
		s.mu.Unlock()
		return d, nil
	}
	s.cache, s.stat = d, st
	s.mu.Unlock()
	return d, nil
}

func (s *Store) read() (*data, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &data{Version: fileVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %w", s.path, err)
	}
	d := &data{}
	if err := json.Unmarshal(raw, d); err != nil {
		return nil, fmt.Errorf("file tài khoản %s bị hỏng: %w", s.path, err)
	}
	if d.Version > fileVersion {
		return nil, fmt.Errorf("file tài khoản %s có phiên bản %d mới hơn chương trình (%d)", s.path, d.Version, fileVersion)
	}
	return d, nil
}

// update 在进程内锁 + 文件锁下读最新数据、修改、清理过期项并原子写回。
func (s *Store) update(fn func(d *data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	lock := flock.New(s.path+".lock", flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("khoá file tài khoản: %w", err)
	}
	defer lock.Unlock()
	d, err := s.read()
	if err != nil {
		return err
	}
	complete := true
	if d.Version < fileVersion {
		complete, err = s.migrate(d)
		if err != nil {
			return err
		}
	}
	if !complete && fn == nil {
		s.cache = d
		s.stat = nil
		return nil
	}
	if fn != nil {
		if err := fn(d); err != nil {
			return err
		}
	}
	now := s.now()
	d.dropSessions(func(x session) bool { return x.ExpiresAt.After(now) })
	d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return !p.ExpiresAt.After(now) })
	d.Audit = slices.DeleteFunc(d.Audit, func(entry AuditEntry) bool { return entry.At.Before(now.AddDate(-1, 0, 0)) })
	if err := s.write(d); err != nil {
		return err
	}
	st, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	s.cache, s.stat = d, st
	return nil
}

func (s *Store) write(d *data) error {
	raw, err := marshalAccounts(d)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".web-accounts-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func normalizeProjects(projects map[string]ProjectRole) (map[string]ProjectRole, error) {
	out := make(map[string]ProjectRole, len(projects))
	for id, level := range projects {
		if !library.ValidProjectID(id) || (level != ProjectOwner && level != ProjectCollab && level != ProjectView) {
			return nil, fmt.Errorf("%w: quyền dự án không hợp lệ", ErrInvalidInput)
		}
		out[id] = level
	}
	return out, nil
}
func validRole(r Role) bool { return r == RoleAdmin || r == RoleMember }

func validDisplayName(name string) bool { return utf8.RuneCountInString(name) <= MaxNameLen }

func validatePassword(p string) error {
	if len([]rune(p)) < MinPasswordLen {
		return fmt.Errorf("%w: mật khẩu cần ít nhất %d ký tự", ErrInvalidInput, MinPasswordLen)
	}
	return nil
}

// HasUsers 报告是否已有任何账号（serve 首次启动时引导创建管理员）。
func (s *Store) HasUsers() (bool, error) {
	if s.repo != nil {
		return s.repo.HasUsers()
	}
	d, err := s.snapshot()
	if err != nil {
		return false, err
	}
	for _, u := range d.Users {
		if u.DeletedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

// Users 返回 GET /api/users 用的列表：账号视图附带有效会话数与最近活跃时间。
func (s *Store) Users() ([]UserListView, error) {
	if s.repo != nil {
		return s.repo.Users()
	}
	d, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	type stats struct {
		count int
		last  *time.Time
	}
	now := s.now()
	byUser := map[string]*stats{}
	for i := range d.Sessions {
		x := &d.Sessions[i]
		if !x.ExpiresAt.After(now) {
			continue
		}
		st := byUser[x.Username]
		if st == nil {
			st = &stats{}
			byUser[x.Username] = st
		}
		st.count++
		if st.last == nil || x.LastSeen.After(*st.last) {
			last := x.LastSeen
			st.last = &last
		}
	}
	out := make([]UserListView, 0, len(d.Users))
	for _, u := range d.Users {
		if u.DeletedAt != nil {
			continue
		}
		v := UserListView{UserView: u.View()}
		if st := byUser[u.Username]; st != nil {
			v.SessionCount, v.LastSeenAt = st.count, st.last
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b UserListView) int { return strings.Compare(a.Username, b.Username) })
	return out, nil
}

type NewUser struct {
	Username string                 `json:"username"`
	Name     string                 `json:"name"`
	Role     Role                   `json:"role"`
	Projects map[string]ProjectRole `json:"projects"`
	Password string                 `json:"password"`
	// MustChangePassword 为 true 时新用户首次登录必须改密；仅服务端设置（网管建号），不接受 JSON 输入。
	MustChangePassword bool        `json:"-"`
	Actor              *AuditActor `json:"-"`
}

func (s *Store) CreateUser(in NewUser) (UserView, error) {
	if s.repo != nil {
		return s.repo.CreateUser(in)
	}
	in.Username = strings.TrimSpace(in.Username)
	if !usernamePattern.MatchString(in.Username) {
		return UserView{}, fmt.Errorf("%w: tên đăng nhập chỉ gồm a-z, 0-9, . _ - (2–32 ký tự)", ErrInvalidInput)
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validDisplayName(in.Name) {
		return UserView{}, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", ErrInvalidInput, MaxNameLen)
	}
	if in.Role == "" {
		in.Role = RoleMember
	}
	if !validRole(in.Role) {
		return UserView{}, fmt.Errorf("%w: vai trò %q", ErrInvalidInput, in.Role)
	}
	projects, err := normalizeProjects(in.Projects)
	if err != nil {
		return UserView{}, err
	}
	if err := validatePassword(in.Password); err != nil {
		return UserView{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return UserView{}, err
	}
	var created User
	err = s.update(func(d *data) error {
		if d.userAny(in.Username) != nil {
			return ErrExists
		}
		for p, role := range projects {
			if role == ProjectOwner && d.activeOwners(p) > 0 {
				return ErrOwnerTaken
			}
		}
		now := s.now()
		created = User{Username: in.Username, Name: in.Name, Role: in.Role, Projects: projects, MustChangePassword: in.MustChangePassword, PasswordChangedAt: &now, CreatedAt: now, UpdatedAt: now}
		if in.MustChangePassword {
			createdBy := ""
			if in.Actor != nil {
				createdBy = in.Actor.Username
			}
			created.TempPassword = &TempPassword{CodeHash: hash, ExpiresAt: now.Add(TempPasswordTTL), CreatedBy: createdBy, CreatedAt: now}
		} else {
			created.PasswordHash = hash
		}
		d.Users = append(d.Users, created)
		s.addAudit(d, in.Actor, "acct", "Tạo tài khoản", in.Username, "", "", "")
		s.addAudit(d, in.Actor, "perm", "Đổi quyền hệ thống", in.Username, "", string(in.Role), "")
		s.addAudit(d, in.Actor, "perm", "Đổi quyền dự án", in.Username, "", projectSummary(projects), "")
		return nil
	})
	return created.View(), err
}

// CreateFirstAdmin 只在"还没有任何账号"时创建管理员，并在同一把写锁内建立会话（GAP-41）。
// 并发调用只有一个成功，其余返回 ErrExists；检查与写入同锁，跨进程也成立（flock）。
func (s *Store) CreateFirstAdmin(in NewUser, device, ip string) (NewSession, error) {
	if s.repo != nil {
		return s.repo.CreateFirstAdmin(in, device, ip)
	}
	in.Username = strings.TrimSpace(in.Username)
	if !usernamePattern.MatchString(in.Username) {
		return NewSession{}, fmt.Errorf("%w: tên đăng nhập chỉ gồm a-z, 0-9, . _ - (2–32 ký tự)", ErrInvalidInput)
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validDisplayName(in.Name) {
		return NewSession{}, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", ErrInvalidInput, MaxNameLen)
	}
	if err := validatePassword(in.Password); err != nil {
		return NewSession{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return NewSession{}, err
	}
	var out NewSession
	err = s.update(func(d *data) error {
		for _, u := range d.Users {
			if u.DeletedAt == nil {
				return ErrExists
			}
		}
		now := s.now()
		d.Users = append(d.Users, User{Username: in.Username, Name: in.Name, Role: RoleAdmin, Projects: map[string]ProjectRole{}, PasswordHash: hash, PasswordChangedAt: &now, CreatedAt: now, UpdatedAt: now})
		s.addAudit(d, nil, "acct", "Tạo tài khoản quản trị đầu tiên", in.Username, "", "", "")
		s.addAudit(d, nil, "perm", "Đổi quyền hệ thống", in.Username, "", string(RoleAdmin), "")
		s.addAudit(d, nil, "perm", "Đổi quyền dự án", in.Username, "", "{}", "")
		var err error
		out, err = s.newSession(d, in.Username, device, ip)
		return err
	})
	return out, err
}

// FirstUserCreatedAt 返回最早创建的账号时间，供 GAP-41 的 409 {at} 使用；
// 还没有账号时 ok 为 false。
func (s *Store) FirstUserCreatedAt() (at time.Time, ok bool, err error) {
	if s.repo != nil {
		return s.repo.FirstUserCreatedAt()
	}
	d, err := s.snapshot()
	if err != nil {
		return time.Time{}, false, err
	}
	for _, u := range d.Users {
		if u.DeletedAt == nil && (!ok || u.CreatedAt.Before(at)) {
			at, ok = u.CreatedAt, true
		}
	}
	return at, ok, nil
}

// UserPatch 中 nil 字段保持不变。
type UserPatch struct {
	Name     *string                 `json:"name"`
	Role     *Role                   `json:"role"`
	Projects *map[string]ProjectRole `json:"projects"`
	Disabled *bool                   `json:"disabled"`
	Password *string                 `json:"password"`
	// MustChangePassword 仅服务端设置（管理员重置他人密码时置 true），不接受 JSON 输入。
	MustChangePassword *bool       `json:"-"`
	Reason             string      `json:"reason,omitempty"`
	Actor              *AuditActor `json:"-"`
}

// UpdateUser 修改账号；改密码或停用会吊销该账号全部会话。
func (s *Store) UpdateUser(username string, patch UserPatch) (UserView, error) {
	if s.repo != nil {
		return s.repo.UpdateUser(username, patch)
	}
	var projects map[string]ProjectRole
	if patch.Projects != nil {
		var err error
		if projects, err = normalizeProjects(*patch.Projects); err != nil {
			return UserView{}, err
		}
	}
	if patch.Role != nil && !validRole(*patch.Role) {
		return UserView{}, fmt.Errorf("%w: vai trò %q", ErrInvalidInput, *patch.Role)
	}
	if patch.Name != nil && !validDisplayName(strings.TrimSpace(*patch.Name)) {
		return UserView{}, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", ErrInvalidInput, MaxNameLen)
	}
	hash := ""
	if patch.Password != nil {
		if err := validatePassword(*patch.Password); err != nil {
			return UserView{}, err
		}
		var err error
		if hash, err = hashPassword(*patch.Password); err != nil {
			return UserView{}, err
		}
	}
	var updated User
	err := s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		before := *u
		// Owner invariant (đúng một owner active mỗi project): trước khi áp
		// patch, kiểm mọi project bị ảnh hưởng không rơi về 0 hoặc vượt quá 1
		// owner active. Chuyển owner nguyên tử chỉ qua TransferProjectOwner.
		if patch.Projects != nil || patch.Disabled != nil {
			for _, p := range ownerCheckProjects(before.Projects, projects) {
				before_, after_ := d.activeOwners(p), ownerCountAfter(d, p, username, u, projects, patch)
				if after_ == 0 && before_ > 0 {
					return ErrLastOwner
				}
				if after_ > 1 {
					return ErrOwnerTaken
				}
			}
		}
		revoke := false
		if patch.Name != nil {
			u.Name = strings.TrimSpace(*patch.Name)
		}
		if patch.MustChangePassword != nil {
			u.MustChangePassword = *patch.MustChangePassword
		}
		if patch.Role != nil {
			u.Role = *patch.Role
		}
		if patch.Role != nil || patch.Projects != nil {
			delete(d.legacyPermissions, username)
		}
		if patch.Projects != nil {
			u.Projects = projects
		}
		if patch.Disabled != nil {
			revoke = revoke || (*patch.Disabled && !u.Disabled)
			u.Disabled = *patch.Disabled
			if *patch.Disabled {
				at := s.now()
				u.DisabledAt = &at
				u.DisabledReason = patch.Reason
				if patch.Actor != nil {
					u.DisabledBy = patch.Actor.Username
				}
			} else {
				u.DisabledAt = nil
				u.DisabledBy = ""
				u.DisabledReason = ""
			}
		}
		if hash != "" {
			at := s.now()
			if u.MustChangePassword {
				// 管理员重置/签发 → mật khẩu tạm một lần; mật khẩu cũ không còn dùng được.
				createdBy := ""
				if patch.Actor != nil {
					createdBy = patch.Actor.Username
				}
				u.TempPassword = &TempPassword{CodeHash: hash, ExpiresAt: at.Add(TempPasswordTTL), CreatedBy: createdBy, CreatedAt: at}
				u.PasswordHash = ""
			} else {
				u.PasswordHash = hash
				u.TempPassword = nil
			}
			u.PasswordChangedAt = &at
			revoke = true
		}
		// Last-admin guard fires only when THIS patch removes an active admin.
		if before.Role == RoleAdmin && !before.Disabled &&
			(u.Role != RoleAdmin || u.Disabled) && d.enabledAdmins() == 0 {
			return ErrLastAdmin
		}
		if patch.Role != nil && before.Role != u.Role {
			s.addAudit(d, patch.Actor, "perm", "Đổi quyền hệ thống", username, string(before.Role), string(u.Role), patch.Reason)
		}
		if patch.Projects != nil && !projectsEqual(before.Projects, u.Projects) {
			s.addAudit(d, patch.Actor, "perm", "Đổi quyền dự án", username, projectSummary(before.Projects), projectSummary(u.Projects), patch.Reason)
		}
		if patch.Disabled != nil && before.Disabled != u.Disabled {
			action := "Mở khoá tài khoản"
			if u.Disabled {
				action = "Khoá tài khoản"
			}
			s.addAudit(d, patch.Actor, "acct", action, username, "", "", patch.Reason)
		}
		if hash != "" {
			s.addAudit(d, patch.Actor, "sec", "Đặt lại mật khẩu", username, "", "", patch.Reason)
		}
		if patch.Name != nil && before.Name != u.Name {
			s.addAudit(d, patch.Actor, "acct", "Đổi tên tài khoản", username, before.Name, u.Name, patch.Reason)
		}
		u.UpdatedAt = s.now()
		if revoke {
			d.dropSessions(func(x session) bool { return x.Username != username })
			d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return p.Username == username })
		}
		updated = *u
		return nil
	})
	return updated.View(), err
}

// ownerCheckProjects hợp nhất các project trước/sau patch cần kiểm invariant.
func ownerCheckProjects(before, after map[string]ProjectRole) []string {
	set := map[string]bool{}
	for p := range before {
		set[p] = true
	}
	for p := range after {
		set[p] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

// ownerCountAfter đếm owner active còn lại của project sau khi áp patch lên u.
// Các user khác giữ nguyên; u nhận projects/disabled/role mới khi patch đặt.
func ownerCountAfter(d *data, project, username string, u *User, projects map[string]ProjectRole, patch UserPatch) int {
	n := 0
	for i := range d.Users {
		x := &d.Users[i]
		if x.DeletedAt != nil || x.Role == RoleAdmin {
			continue
		}
		disabled := x.Disabled
		role := x.Projects[project]
		if x.Username == username {
			if patch.Disabled != nil {
				disabled = *patch.Disabled
			}
			if patch.Projects != nil {
				role = projects[project]
			}
		}
		if !disabled && role == ProjectOwner {
			n++
		}
	}
	return n
}

func (s *Store) DeleteUser(username string, context ...AuditContext) error {
	if s.repo != nil {
		return s.repo.DeleteUser(username, context...)
	}
	return s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		// Không xoá owner cuối: phải transfer/delete project cùng giao dịch
		// (phần project đến ở phase 05); lỗi rõ ràng thay vì project mồ côi.
		for p, role := range u.Projects {
			if role == ProjectOwner && !u.Disabled && d.activeOwners(p) == 1 {
				return ErrLastOwner
			}
		}
		ctx := auditContext(context)
		s.addAudit(d, ctx.Actor, "acct", "Xoá tài khoản", username, "", "", ctx.Reason)
		at := s.now()
		u.DeletedAt = &at
		u.UpdatedAt = at
		if u.Role == RoleAdmin && !u.Disabled && d.enabledAdmins() == 0 {
			return ErrLastAdmin
		}
		d.dropSessions(func(x session) bool { return x.Username != username })
		d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return p.Username == username })
		return nil
	})
}

// MarkOnboarded 记录账号完成新手引导的时间；重复调用只保留首次时间。
func (s *Store) MarkOnboarded(username string) (UserView, error) {
	if s.repo != nil {
		return s.repo.MarkOnboarded(username)
	}
	var updated User
	err := s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		if u.OnboardedAt == nil {
			t := s.now()
			u.OnboardedAt = &t
			u.UpdatedAt = t
		}
		updated = *u
		return nil
	})
	return updated.View(), err
}

func (s *Store) newSession(d *data, username, device, ip string) (NewSession, error) {
	id, err := randomHex(8)
	if err != nil {
		return NewSession{}, err
	}
	secret, err := randomHex(32)
	if err != nil {
		return NewSession{}, err
	}
	now := s.now()
	x := session{ID: id, SecretHash: digest(secret), Username: username, Device: device, IP: ip, CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(SessionTTL)}
	d.Sessions = append(d.Sessions, x)
	return NewSession{Cookie: id + "." + secret, ExpiresAt: x.ExpiresAt, User: d.user(username).View()}, nil
}

// Login 校验密码并新建会话。用户不存在、停用、密码错误统一返回 ErrInvalidCredentials。
func (s *Store) Login(username, password, device, ip string) (NewSession, error) {
	if s.repo != nil {
		return s.repo.Login(username, password, device, ip)
	}
	d, err := s.snapshot()
	if err != nil {
		return NewSession{}, err
	}
	username = strings.TrimSpace(username)
	u := d.user(username)
	// 先校验（argon2 在写锁外）；user 不存在时跑 dummy hash 保持耗时一致，
	// 不泄露用户名是否存在。
	now := s.now()
	hash := dummyHash()
	tempOK := false
	if u != nil && u.TempPassword != nil {
		hash = u.TempPassword.CodeHash
		tempOK = u.TempPassword.usable(now)
	} else if u != nil {
		hash = u.PasswordHash
	}
	if !verifyPassword(password, hash) || u == nil || u.Disabled || (u.TempPassword != nil && !tempOK) {
		return NewSession{}, ErrInvalidCredentials
	}
	var out NewSession
	err = s.update(func(d *data) error {
		// 校验与写入之间账号可能被改/临时码被别的请求用掉，写锁内再确认一次。
		current := d.user(u.Username)
		if current == nil || current.Disabled {
			return ErrInvalidCredentials
		}
		if current.TempPassword != nil {
			if u.TempPassword == nil || !current.TempPassword.usable(s.now()) || current.TempPassword.CodeHash != u.TempPassword.CodeHash {
				return ErrInvalidCredentials
			}
			used := s.now()
			current.TempPassword.UsedAt = &used
		} else if current.PasswordHash == "" || current.PasswordHash != u.PasswordHash {
			return ErrInvalidCredentials
		}
		var err error
		out, err = s.newSession(d, u.Username, device, ip)
		return err
	})
	return out, err
}

// Pair 兑换一次性扫码配对码，成功后配对码立即作废。
func (s *Store) Pair(code, device, ip string) (NewSession, error) {
	if s.repo != nil {
		return s.repo.Pair(code, device, ip)
	}
	h := digest(strings.TrimSpace(code))
	var out NewSession
	err := s.update(func(d *data) error {
		now := s.now()
		i := slices.IndexFunc(d.Pairings, func(p pairing) bool {
			return subtle.ConstantTimeCompare([]byte(p.CodeHash), []byte(h)) == 1 && p.ExpiresAt.After(now)
		})
		if i < 0 {
			return ErrInvalidPairing
		}
		p := d.Pairings[i]
		d.Pairings = slices.Delete(d.Pairings, i, i+1)
		u := d.user(p.Username)
		if u == nil || u.Disabled {
			return ErrInvalidPairing
		}
		var err error
		out, err = s.newSession(d, u.Username, device, ip)
		return err
	})
	if errors.Is(err, ErrInvalidPairing) {
		// 配对失败时仍需持久化"作废"；update 遇错不写盘，这里单独删除已失效的码。
		_ = s.update(func(d *data) error {
			d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return p.CodeHash == h })
			return nil
		})
	}
	return out, err
}

// NewPairing 为 username 生成一次性配对码（PairingTTL 内有效）。
func (s *Store) NewPairing(username, createdBy string) (string, time.Time, error) {
	if s.repo != nil {
		return s.repo.NewPairing(username, createdBy)
	}
	code, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	var expires time.Time
	err = s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		if u.Disabled {
			return fmt.Errorf("%w: tài khoản đang bị khoá", ErrInvalidInput)
		}
		expires = s.now().Add(PairingTTL)
		d.Pairings = append(d.Pairings, pairing{CodeHash: digest(code), Username: username, CreatedBy: createdBy, ExpiresAt: expires})
		var actor *AuditActor
		if creator := d.user(createdBy); creator != nil {
			actor = &AuditActor{Username: creator.Username, Name: creator.Name}
		}
		s.addAudit(d, actor, "sec", "Tạo mã đăng nhập điện thoại", username, "", "", "")
		return nil
	})
	return code, expires, err
}

// Authenticate 校验 Cookie 并返回当前用户与会话；距上次记录超过 touchInterval 才回写 LastSeen。
func (s *Store) Authenticate(cookie string) (User, Session, error) {
	if s.repo != nil {
		return s.repo.Authenticate(cookie)
	}
	id, secret, ok := strings.Cut(cookie, ".")
	if !ok || id == "" || secret == "" {
		return User{}, Session{}, ErrUnauthenticated
	}
	d, err := s.snapshot()
	if err != nil {
		return User{}, Session{}, err
	}
	now := s.now()
	i := slices.IndexFunc(d.Sessions, func(x session) bool { return x.ID == id })
	if i < 0 {
		return User{}, Session{}, ErrUnauthenticated
	}
	x := d.Sessions[i]
	if subtle.ConstantTimeCompare([]byte(x.SecretHash), []byte(digest(secret))) != 1 || !x.ExpiresAt.After(now) {
		return User{}, Session{}, ErrUnauthenticated
	}
	u := d.user(x.Username)
	if u == nil || u.Disabled {
		return User{}, Session{}, ErrUnauthenticated
	}
	user := *u
	if now.Sub(x.LastSeen) > touchInterval {
		_ = s.update(func(d *data) error {
			for j := range d.Sessions {
				if d.Sessions[j].ID == id {
					d.Sessions[j].LastSeen = now
				}
			}
			return nil
		})
		x.LastSeen = now
	}
	return user, x.view(id), nil
}

// Sessions 列出某账号的有效会话；username 为空时列出全部。
func (s *Store) Sessions(username, currentID string) ([]Session, error) {
	if s.repo != nil {
		return s.repo.Sessions(username, currentID)
	}
	d, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := []Session{}
	for _, x := range d.Sessions {
		if (username == "" || x.Username == username) && x.ExpiresAt.After(now) {
			out = append(out, x.view(currentID))
		}
	}
	slices.SortFunc(out, func(a, b Session) int { return b.LastSeen.Compare(a.LastSeen) })
	return out, nil
}

// RevokeSession 删除会话；allowed 根据会话所属账号判断调用者是否有权吊销。
func (s *Store) RevokeSession(id string, allowed func(owner string) bool, context ...AuditContext) error {
	if s.repo != nil {
		return s.repo.RevokeSession(id, allowed, context...)
	}
	return s.update(func(d *data) error {
		i := slices.IndexFunc(d.Sessions, func(x session) bool { return x.ID == id })
		if i < 0 {
			return ErrNotFound
		}
		if !allowed(d.Sessions[i].Username) {
			return ErrForbidden
		}
		ctx := auditContext(context)
		s.addAudit(d, ctx.Actor, "sec", "Đăng xuất thiết bị", d.Sessions[i].Username, "", id, ctx.Reason)
		d.Sessions = slices.Delete(d.Sessions, i, i+1)
		return nil
	})
}

// ChangePassword 校验旧密码后修改，并吊销该账号除 keepSession 外的所有会话。
func (s *Store) ChangePassword(username, current, next, keepSession string) error {
	if s.repo != nil {
		return s.repo.ChangePassword(username, current, next, keepSession)
	}
	d, err := s.snapshot()
	if err != nil {
		return err
	}
	u := d.user(username)
	if u == nil {
		return ErrNotFound
	}
	// Phiên restricted (must_change) xác thực "mật khẩu hiện tại" bằng mật
	// khẩu tạm — kể cả khi mã đã đốt/hết hạn, miễn phiên còn hiệu lực.
	currentHash := u.PasswordHash
	if u.MustChangePassword && u.TempPassword != nil {
		currentHash = u.TempPassword.CodeHash
	}
	if currentHash == "" || !verifyPassword(current, currentHash) {
		return ErrForbidden
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	hash, err := hashPassword(next)
	if err != nil {
		return err
	}
	return s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		u.PasswordHash = hash
		u.TempPassword = nil
		at := s.now()
		u.PasswordChangedAt = &at
		// 自己成功改密后解除“必须改密”标记。
		u.MustChangePassword = false
		u.UpdatedAt = s.now()
		d.dropSessions(func(x session) bool { return x.Username != username || x.ID == keepSession })
		d.Pairings = slices.DeleteFunc(d.Pairings, func(p pairing) bool { return p.Username == username })
		s.addAudit(d, &AuditActor{Username: u.Username, Name: u.Name}, "sec", "Đổi mật khẩu", username, "", "", "")
		return nil
	})
}
