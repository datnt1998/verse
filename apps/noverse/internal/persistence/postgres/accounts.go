package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/netip"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/webauth"
)

// Accounts là webauth.Repository trên Postgres: cùng ngữ nghĩa quan sát được
// với file backend — generic credential errors, mật khẩu tạm một lần + TTL,
// đúng một owner active mỗi project, tối thiểu một admin active, audit ghi
// trong cùng transaction với mutation. Định danh public vẫn là username;
// UUID chỉ là khóa nội bộ.
type Accounts struct {
	pool   *pgxpool.Pool
	pgpool *Pool
	now    func() time.Time
}

// NewAccounts builds the identity repository on the app-role pool. The pool
// is process-scoped; actors arrive as arguments — no session state here.
func NewAccounts(pool *Pool) *Accounts {
	return &Accounts{pool: pool.Pool(), pgpool: pool, now: time.Now}
}

// SetClock overrides the time source (integration tests only).
func (a *Accounts) SetClock(now func() time.Time) { a.now = now }

var _ webauth.Repository = (*Accounts)(nil)

// LoginLimiter trả limiter login/pairing dùng bảng login_rate_limits — chia
// sẻ giữa các process và sống sót restart (khác limiter RAM của file mode).
func (a *Accounts) LoginLimiter() webauth.LoginLimiter {
	return NewAuthLimiter(a.pgpool)
}

// RecoveryLimiter trả limiter cửa sổ trên cùng bảng cho recovery codes.
func (a *Accounts) RecoveryLimiter(maxFails int, window time.Duration) webauth.RecoveryLimiter {
	return NewWindowLimiter(a.pgpool, "recover", maxFails, window)
}

var _ webauth.LoginLimiterProvider = (*Accounts)(nil)

const (
	pgSessionTTL = 30 * 24 * time.Hour
	pgPairingTTL = 5 * time.Minute
	pgTouchGap   = 10 * time.Minute
)

// deterministicUUID derives a stable UUID for idempotent anchors (project by
// slug, device by username+label). This is a PINNED LEGACY RECIPE, not
// RFC 4122 UUIDv5: it takes the first 16 bytes of
// SHA-256("ainovel|v1|" + namespace + "|" + name) and stamps the version-5
// nibble/variant bits for compatibility with rows already written. Do not
// swap it for a standards-compliant v5 (SHA-1 namespace) implementation or
// change the salt: that would silently re-key existing projects/devices.
// Fresh ordinary entities use randomUUID (v4). A new standards-compliant v5
// mapping needs its own explicit mapping-version boundary.
func deterministicUUID(ns, name string) pgtype.UUID {
	sum := sha256.Sum256([]byte("ainovel|v1|" + ns + "|" + name))
	var id pgtype.UUID
	copy(id.Bytes[:], sum[:16])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x50
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}

func randomUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		return id, err
	}
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id, nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func tsPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

// pgTimestamptz wraps a zero-able time: zero → NULL (for blocked_until).
func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return ts(t)
}

func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// constraintErr maps Postgres unique violations to the repository contract's
// domain errors so callers never see raw SQLSTATEs.
func constraintErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch {
		case strings.Contains(pgErr.ConstraintName, "project_one_owner"):
			return webauth.ErrOwnerTaken
		case strings.Contains(pgErr.ConstraintName, "username") || strings.Contains(pgErr.ConstraintName, "users"):
			return webauth.ErrExists
		default:
			return fmt.Errorf("ràng buộc duy nhất %s: %w", pgErr.ConstraintName, errs.ErrStorageConflict)
		}
	}
	return sanitizeErr(err)
}
func digestBytes(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func inetArg(ip string) *netip.Addr {
	if ip == "" {
		return nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil
	}
	return &addr
}

func uuidText(id pgtype.UUID) string {
	b := id.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (a *Accounts) q(tx pgx.Tx) *gen.Queries { return gen.New(tx) }

func (a *Accounts) qp() *gen.Queries { return gen.New(a.pool) }

// --- row → domain mapping ------------------------------------------------

// userFromRow assembles webauth.User: base row + credential + temp password
// (open or last consumed-for-restricted-session) + memberships + latest
// recovery code.
func (a *Accounts) userFromRow(ctx context.Context, q *gen.Queries, row gen.User) (webauth.User, error) {
	u := webauth.User{
		Username:           row.Username,
		Name:               row.DisplayName,
		Role:               webauth.Role(row.GlobalRole),
		Disabled:           row.Status == "locked",
		OnboardedAt:        tsPtr(row.OnboardedAt),
		MustChangePassword: row.MustChangePassword,
		DisabledAt:         tsPtr(row.DisabledAt),
		DisabledReason:     row.DisabledReason,
		DeletedAt:          tsPtr(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		Projects:           map[string]webauth.ProjectRole{},
	}
	if row.DisabledBy.Valid {
		if by, err := q.GetUserByID(ctx, row.DisabledBy); err == nil {
			u.DisabledBy = by.Username
		}
	}
	if cred, err := q.GetCredential(ctx, row.ID); err == nil {
		u.PasswordHash = cred.PasswordHash.String
		u.PasswordChangedAt = tsPtr(cred.PasswordChangedAt)
	} else if !noRows(err) {
		return u, fmt.Errorf("đọc credential: %w", sanitizeErr(err))
	}
	if temp, err := q.GetLatestTempPassword(ctx, row.ID); err == nil && !temp.RevokedAt.Valid {
		u.TempPassword = &webauth.TempPassword{CodeHash: temp.PasswordHash, ExpiresAt: temp.ExpiresAt.Time, UsedAt: tsPtr(temp.UsedAt), CreatedAt: temp.CreatedAt.Time}
	} else if err != nil && !noRows(err) {
		return u, fmt.Errorf("đọc mật khẩu tạm: %w", sanitizeErr(err))
	}
	if rec, err := q.GetLatestRecoveryCode(ctx, row.ID); err == nil {
		u.Recovery = &webauth.RecoveryCode{CodeHash: rec.CodeHash, Mask: rec.Mask.String, CreatedAt: rec.CreatedAt.Time, UsedAt: tsPtr(rec.UsedAt)}
	} else if !noRows(err) {
		return u, fmt.Errorf("đọc mã khôi phục: %w", sanitizeErr(err))
	}
	members, err := q.ListMembershipsByUser(ctx, row.ID)
	if err != nil {
		return u, fmt.Errorf("đọc quyền dự án: %w", sanitizeErr(err))
	}
	for _, m := range members {
		u.Projects[m.Slug] = webauth.ProjectRole(m.Role)
	}
	return u, nil
}

func (a *Accounts) userByUsername(ctx context.Context, q *gen.Queries, username string) (webauth.User, gen.User, error) {
	row, err := q.GetUserByUsername(ctx, username)
	if err != nil {
		if noRows(err) {
			return webauth.User{}, gen.User{}, webauth.ErrNotFound
		}
		return webauth.User{}, gen.User{}, fmt.Errorf("đọc tài khoản: %w", sanitizeErr(err))
	}
	u, err := a.userFromRow(ctx, q, row)
	return u, row, err
}

// --- account lifecycle ----------------------------------------------------

func (a *Accounts) HasUsers() (bool, error) {
	n, err := a.qp().CountActiveUsers(context.Background())
	if err != nil {
		return false, sanitizeErr(err)
	}
	return n > 0, nil
}

func (a *Accounts) Users() ([]webauth.UserListView, error) {
	ctx := context.Background()
	q := a.qp()
	rows, err := q.ListActiveUsers(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	stats, err := q.SessionStatsByUser(ctx)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	byUser := map[[16]byte]gen.SessionStatsByUserRow{}
	for _, s := range stats {
		byUser[s.UserID.Bytes] = s
	}
	out := make([]webauth.UserListView, 0, len(rows))
	for _, row := range rows {
		u, err := a.userFromRow(ctx, q, row)
		if err != nil {
			return nil, err
		}
		v := webauth.UserListView{UserView: u.View()}
		if st, ok := byUser[row.ID.Bytes]; ok {
			v.SessionCount = int(st.SessionCount)
			if t, ok := st.LastSeenAt.(time.Time); ok {
				v.LastSeenAt = &t
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func validateNewUser(in *webauth.NewUser) (map[string]webauth.ProjectRole, error) {
	in.Username = strings.TrimSpace(in.Username)
	if !webauth.ValidUsername(in.Username) {
		return nil, fmt.Errorf("%w: tên đăng nhập chỉ gồm a-z, 0-9, . _ - (2–32 ký tự)", webauth.ErrInvalidInput)
	}
	in.Name = strings.TrimSpace(in.Name)
	if len([]rune(in.Name)) > webauth.MaxNameLen {
		return nil, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", webauth.ErrInvalidInput, webauth.MaxNameLen)
	}
	if in.Role == "" {
		in.Role = webauth.RoleMember
	}
	if in.Role != webauth.RoleAdmin && in.Role != webauth.RoleMember {
		return nil, fmt.Errorf("%w: vai trò %q", webauth.ErrInvalidInput, in.Role)
	}
	projects := map[string]webauth.ProjectRole{}
	for id, level := range in.Projects {
		if !library.ValidProjectID(id) || (level != webauth.ProjectOwner && level != webauth.ProjectCollab && level != webauth.ProjectView) {
			return nil, fmt.Errorf("%w: quyền dự án không hợp lệ", webauth.ErrInvalidInput)
		}
		projects[id] = level
	}
	return projects, nil
}

func validatePasswordInput(p string) error {
	if len([]rune(p)) < webauth.MinPasswordLen {
		return fmt.Errorf("%w: mật khẩu cần ít nhất %d ký tự", webauth.ErrInvalidInput, webauth.MinPasswordLen)
	}
	return nil
}

func (a *Accounts) CreateUser(in webauth.NewUser) (webauth.UserView, error) {
	projects, err := validateNewUser(&in)
	if err != nil {
		return webauth.UserView{}, err
	}
	if err := validatePasswordInput(in.Password); err != nil {
		return webauth.UserView{}, err
	}
	// Argon2 ở ngoài transaction: hash chậm không giữ lock/connection.
	hash, err := webauth.HashPasswordForBackend(in.Password)
	if err != nil {
		return webauth.UserView{}, err
	}
	var created gen.User
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		if _, err := q.GetUserByUsernameAny(ctx, in.Username); err == nil {
			return webauth.ErrExists
		} else if !noRows(err) {
			return sanitizeErr(err)
		}
		userID, err := randomUUID()
		if err != nil {
			return err
		}
		now := a.now()
		created, err = q.InsertUser(ctx, gen.InsertUserParams{
			ID:                 userID,
			Username:           in.Username,
			DisplayName:        in.Name,
			GlobalRole:         string(in.Role),
			MustChangePassword: in.MustChangePassword,
			CreatedAt:          ts(now),
		})
		if err != nil {
			return constraintErr(err)
		}
		if in.MustChangePassword {
			tempID, err := randomUUID()
			if err != nil {
				return err
			}
			var createdBy pgtype.UUID
			if in.Actor != nil {
				if actor, err := q.GetUserByUsername(ctx, in.Actor.Username); err == nil {
					createdBy = actor.ID
				}
			}
			if err := q.InsertTempPassword(ctx, gen.InsertTempPasswordParams{
				ID: tempID, UserID: userID, PasswordHash: hash,
				ExpiresAt: ts(now.Add(webauth.TempPasswordTTL)), CreatedBy: createdBy,
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		if err := q.InsertCredential(ctx, gen.InsertCredentialParams{
			UserID:            userID,
			PasswordHash:      pgtype.Text{String: hash, Valid: !in.MustChangePassword},
			PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		if err := a.replaceMemberships(ctx, q, created, map[string]webauth.ProjectRole{}, projects, in.Actor, ""); err != nil {
			return err
		}
		if err := a.addAudit(ctx, q, in.Actor, "", "acct", "Tạo tài khoản", in.Username, "", "", ""); err != nil {
			return err
		}
		if err := a.addAudit(ctx, q, in.Actor, "", "perm", "Đổi quyền hệ thống", in.Username, "", string(in.Role), ""); err != nil {
			return err
		}
		return a.addAudit(ctx, q, in.Actor, "", "perm", "Đổi quyền dự án", in.Username, "", projectSummary(projects), "")
	})
	if err != nil {
		return webauth.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), in.Username)
	if err != nil {
		return webauth.UserView{}, err
	}
	return u.View(), nil
}

func (a *Accounts) CreateFirstAdmin(in webauth.NewUser, device, ip string) (webauth.NewSession, error) {
	if _, err := validateNewUser(&in); err != nil {
		return webauth.NewSession{}, err
	}
	if err := validatePasswordInput(in.Password); err != nil {
		return webauth.NewSession{}, err
	}
	hash, err := webauth.HashPasswordForBackend(in.Password)
	if err != nil {
		return webauth.NewSession{}, err
	}
	var out webauth.NewSession
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		// Singleton lock serializes first-admin races across processes.
		if _, err := q.LockAuthInvariant(ctx); err != nil {
			return sanitizeErr(err)
		}
		n, err := q.CountActiveUsers(ctx)
		if err != nil {
			return sanitizeErr(err)
		}
		if n > 0 {
			return webauth.ErrExists
		}
		userID, err := randomUUID()
		if err != nil {
			return err
		}
		now := a.now()
		row, err := q.InsertUser(ctx, gen.InsertUserParams{
			ID: userID, Username: in.Username, DisplayName: in.Name,
			GlobalRole: string(webauth.RoleAdmin), CreatedAt: ts(now),
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if err := q.InsertCredential(ctx, gen.InsertCredentialParams{
			UserID: userID, PasswordHash: pgtype.Text{String: hash, Valid: true}, PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		if err := a.addAudit(ctx, q, nil, "", "acct", "Tạo tài khoản quản trị đầu tiên", in.Username, "", "", ""); err != nil {
			return err
		}
		if err := a.addAudit(ctx, q, nil, "", "perm", "Đổi quyền hệ thống", in.Username, "", string(webauth.RoleAdmin), ""); err != nil {
			return err
		}
		if err := a.addAudit(ctx, q, nil, "", "perm", "Đổi quyền dự án", in.Username, "", "{}", ""); err != nil {
			return err
		}
		out, err = a.newSession(ctx, q, tx, row, "password", pgtype.UUID{}, device, ip)
		return err
	})
	return out, err
}

func (a *Accounts) FirstUserCreatedAt() (time.Time, bool, error) {
	v, err := a.qp().FirstUserCreatedAt(context.Background())
	if err != nil {
		return time.Time{}, false, sanitizeErr(err)
	}
	t, ok := v.(time.Time)
	return t, ok, nil
}

func (a *Accounts) UpdateUser(username string, patch webauth.UserPatch) (webauth.UserView, error) {
	var projects map[string]webauth.ProjectRole
	if patch.Projects != nil {
		projects = map[string]webauth.ProjectRole{}
		for id, level := range *patch.Projects {
			if !library.ValidProjectID(id) || (level != webauth.ProjectOwner && level != webauth.ProjectCollab && level != webauth.ProjectView) {
				return webauth.UserView{}, fmt.Errorf("%w: quyền dự án không hợp lệ", webauth.ErrInvalidInput)
			}
			projects[id] = level
		}
	}
	if patch.Role != nil && *patch.Role != webauth.RoleAdmin && *patch.Role != webauth.RoleMember {
		return webauth.UserView{}, fmt.Errorf("%w: vai trò %q", webauth.ErrInvalidInput, *patch.Role)
	}
	if patch.Name != nil && len([]rune(strings.TrimSpace(*patch.Name))) > webauth.MaxNameLen {
		return webauth.UserView{}, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", webauth.ErrInvalidInput, webauth.MaxNameLen)
	}
	hash := ""
	if patch.Password != nil {
		if err := validatePasswordInput(*patch.Password); err != nil {
			return webauth.UserView{}, err
		}
		var err error
		// Argon2 ngoài transaction.
		if hash, err = webauth.HashPasswordForBackend(*patch.Password); err != nil {
			return webauth.UserView{}, err
		}
	}
	err := RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		// Lock the invariant row first: competing admin/user mutations
		// serialize so the last-admin and owner counts see each other.
		if _, err := q.LockAuthInvariant(ctx); err != nil {
			return sanitizeErr(err)
		}
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		before, err := a.userFromRow(ctx, q, row)
		if err != nil {
			return err
		}
		now := a.now()
		name := row.DisplayName
		role := row.GlobalRole
		status := row.Status
		mustChange := row.MustChangePassword
		disabledAt, disabledBy, disabledReason := row.DisabledAt, row.DisabledBy, row.DisabledReason
		revoke := false
		if patch.Name != nil {
			name = strings.TrimSpace(*patch.Name)
		}
		if patch.MustChangePassword != nil {
			mustChange = *patch.MustChangePassword
		}
		if patch.Role != nil {
			role = string(*patch.Role)
		}
		if patch.Disabled != nil {
			if *patch.Disabled && status == "active" {
				revoke = true
			}
			if *patch.Disabled {
				status = "locked"
				disabledAt = ts(now)
				disabledReason = patch.Reason
				disabledBy = pgtype.UUID{}
				if patch.Actor != nil {
					if actor, err := q.GetUserByUsername(ctx, patch.Actor.Username); err == nil {
						disabledBy = actor.ID
					}
				}
			} else {
				status = "active"
				disabledAt, disabledBy, disabledReason = pgtype.Timestamptz{}, pgtype.UUID{}, ""
			}
		}
		// Owner invariants across every affected project (before ∪ after).
		newProjects := before.Projects
		if patch.Projects != nil {
			newProjects = projects
		}
		if err := a.checkOwnerInvariants(ctx, q, row, status == "locked", before.Projects, newProjects); err != nil {
			return err
		}
		row, err = q.UpdateUserAfterPatch(ctx, gen.UpdateUserAfterPatchParams{
			ID:                 row.ID,
			DisplayName:        name,
			GlobalRole:         role,
			Status:             status,
			MustChangePassword: mustChange,
			DisabledAt:         disabledAt,
			DisabledBy:         disabledBy,
			DisabledReason:     disabledReason,
			UpdatedAt:          ts(now),
		})
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		if patch.Projects != nil {
			if err := a.replaceMemberships(ctx, q, row, before.Projects, projects, patch.Actor, patch.Reason); err != nil {
				return err
			}
		}
		if hash != "" {
			if mustChange {
				// 管理员重置/签发 → mật khẩu tạm một lần; mật khẩu cũ vô hiệu.
				if err := q.RevokeAllTempPasswords(ctx, gen.RevokeAllTempPasswordsParams{UserID: row.ID, RevokedAt: ts(now)}); err != nil {
					return sanitizeErr(err)
				}
				tempID, err := randomUUID()
				if err != nil {
					return err
				}
				var createdBy pgtype.UUID
				if patch.Actor != nil {
					if actor, err := q.GetUserByUsername(ctx, patch.Actor.Username); err == nil {
						createdBy = actor.ID
					}
				}
				if err := q.InsertTempPassword(ctx, gen.InsertTempPasswordParams{
					ID: tempID, UserID: row.ID, PasswordHash: hash,
					ExpiresAt: ts(now.Add(webauth.TempPasswordTTL)), CreatedBy: createdBy,
				}); err != nil {
					return sanitizeErr(err)
				}
				if err := q.SetCredentialHash(ctx, gen.SetCredentialHashParams{
					UserID: row.ID, PasswordHash: pgtype.Text{}, PasswordChangedAt: ts(now),
				}); err != nil {
					return sanitizeErr(err)
				}
			} else {
				if err := q.SetCredentialHash(ctx, gen.SetCredentialHashParams{
					UserID: row.ID, PasswordHash: pgtype.Text{String: hash, Valid: true}, PasswordChangedAt: ts(now),
				}); err != nil {
					return sanitizeErr(err)
				}
				if err := q.RevokeAllTempPasswords(ctx, gen.RevokeAllTempPasswordsParams{UserID: row.ID, RevokedAt: ts(now)}); err != nil {
					return sanitizeErr(err)
				}
			}
			revoke = true
		}
		// Last-admin guard only when this patch itself removes an active
		// admin — a member update in a zero-admin store must not trip it.
		if before.Role == webauth.RoleAdmin && !before.Disabled &&
			(role != string(webauth.RoleAdmin) || status == "locked") {
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return sanitizeErr(err)
			}
			if n == 0 {
				return webauth.ErrLastAdmin
			}
		}
		if patch.Role != nil && before.Role != webauth.Role(role) {
			if err := a.addAudit(ctx, q, patch.Actor, "", "perm", "Đổi quyền hệ thống", username, string(before.Role), role, patch.Reason); err != nil {
				return err
			}
		}
		if patch.Disabled != nil && before.Disabled != (status == "locked") {
			action := "Mở khoá tài khoản"
			if status == "locked" {
				action = "Khoá tài khoản"
			}
			if err := a.addAudit(ctx, q, patch.Actor, "", "acct", action, username, "", "", patch.Reason); err != nil {
				return err
			}
		}
		if hash != "" {
			if err := a.addAudit(ctx, q, patch.Actor, "", "sec", "Đặt lại mật khẩu", username, "", "", patch.Reason); err != nil {
				return err
			}
		}
		if patch.Name != nil && before.Name != name {
			if err := a.addAudit(ctx, q, patch.Actor, "", "acct", "Đổi tên tài khoản", username, before.Name, name, patch.Reason); err != nil {
				return err
			}
		}
		if revoke {
			if _, err := a.revokeUserAuth(ctx, q, tx, row.ID, pgtype.UUID{}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return webauth.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return webauth.UserView{}, err
	}
	return u.View(), nil
}

func (a *Accounts) DeleteUser(username string, auditCtxs ...webauth.AuditContext) error {
	ctx_ := auditContextOf(auditCtxs)
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		if _, err := q.LockAuthInvariant(ctx); err != nil {
			return sanitizeErr(err)
		}
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		u, err := a.userFromRow(ctx, q, row)
		if err != nil {
			return err
		}
		// Không xoá owner cuối: phải transfer/delete project trong cùng tx.
		for p, role := range u.Projects {
			if role == webauth.ProjectOwner && !u.Disabled {
				pid, err := projectID(ctx, q, p)
				if err != nil {
					return err
				}
				n, err := q.CountActiveOwners(ctx, pid)
				if err != nil {
					return sanitizeErr(err)
				}
				if n == 1 {
					return webauth.ErrLastOwner
				}
			}
		}
		now := a.now()
		if err := a.addAudit(ctx, q, ctx_.Actor, "", "acct", "Xoá tài khoản", username, "", "", ctx_.Reason); err != nil {
			return err
		}
		if err := q.SoftDeleteUser(ctx, gen.SoftDeleteUserParams{ID: row.ID, DeletedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		if u.Role == webauth.RoleAdmin && !u.Disabled {
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return sanitizeErr(err)
			}
			if n == 0 {
				return webauth.ErrLastAdmin
			}
		}
		_, err = a.revokeUserAuth(ctx, q, tx, row.ID, pgtype.UUID{})
		return err
	})
}

func auditContextOf(context []webauth.AuditContext) webauth.AuditContext {
	if len(context) > 0 {
		return context[0]
	}
	return webauth.AuditContext{}
}

// revokeUserAuth revokes every session except keep plus all pairings; the
// revoked session IDs come back for callers that broadcast them (Recover).
func (a *Accounts) revokeUserAuth(ctx context.Context, q *gen.Queries, tx pgx.Tx, userID pgtype.UUID, keep pgtype.UUID) ([]string, error) {
	revoked, err := revokeUserSessionsExcept(ctx, tx, userID, a.now(), keep)
	if err != nil {
		return nil, err
	}
	if err := q.RevokeUserPairings(ctx, gen.RevokeUserPairingsParams{UserID: userID, RevokedAt: ts(a.now())}); err != nil {
		return nil, sanitizeErr(err)
	}
	return revoked, nil
}

func (a *Accounts) MarkOnboarded(username string) (webauth.UserView, error) {
	err := RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		return q.MarkOnboarded(ctx, gen.MarkOnboardedParams{ID: row.ID, OnboardedAt: ts(a.now())})
	})
	if err != nil {
		return webauth.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return webauth.UserView{}, err
	}
	return u.View(), nil
}
