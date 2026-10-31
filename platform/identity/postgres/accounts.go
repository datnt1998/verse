package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/postgres/gen"
)

// Accounts is identity.Repository on Postgres: the same observable semantics
// as the file backend — generic credential errors, one-time temporary
// passwords with a TTL, exactly one active owner per project, at least one
// active admin, audit in the same transaction as the mutation. The public
// identifier stays the username; the UUID is an internal key.
//
// The instance is process-scoped and immutable: options are fixed at
// construction, actors arrive as arguments. It never carries a current user,
// app or grant between requests.
type Accounts struct {
	pool    *pgxpool.Pool
	now     func() time.Time
	opts    Options
	project func(string) bool
}

// NewAccounts builds the identity repository over the caller-owned pool.
func NewAccounts(pool *pgxpool.Pool, opts Options) *Accounts {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	valid := opts.ValidProjectID
	if valid == nil {
		valid = func(string) bool { return true }
	}
	return &Accounts{pool: pool, now: now, opts: opts, project: valid}
}

var _ identity.Repository = (*Accounts)(nil)

// LoginLimiter returns the login/pairing limiter backed by login_rate_limits —
// shared between processes and surviving restarts (unlike the RAM limiter of
// file mode). Both apps use the same buckets.
func (a *Accounts) LoginLimiter() identity.LoginLimiter {
	return NewAuthLimiter(a.pool)
}

// RecoveryLimiter returns the sliding-window limiter on the same table.
func (a *Accounts) RecoveryLimiter(maxFails int, window time.Duration) identity.RecoveryLimiter {
	return NewWindowLimiter(a.pool, "recover", maxFails, window)
}

var _ identity.LoginLimiterProvider = (*Accounts)(nil)

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
// Fresh ordinary entities use randomUUID (v4).
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

// constraintErr maps Postgres unique violations to identity sentinels so
// callers never see raw SQLSTATEs and the HTTP layer keeps its 409 mapping.
func constraintErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch {
		case strings.Contains(pgErr.ConstraintName, "project_one_owner"):
			return identity.ErrOwnerTaken
		case strings.Contains(pgErr.ConstraintName, "username") || strings.Contains(pgErr.ConstraintName, "users"):
			return identity.ErrExists
		default:
			return fmt.Errorf("ràng buộc duy nhất %s: %w", pgErr.ConstraintName, identity.ErrConflict)
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

// requireProjectAccess rejects the Noverse project surface before any SQL when
// the backend was built for a project-less app. Denial is typed, never a fake
// success and never a weakened invariant.
func (a *Accounts) requireProjectAccess() error {
	if !a.opts.ProjectAccess {
		return fmt.Errorf("%w: thao tác dự án không có trên backend này", identity.ErrUnsupported)
	}
	return nil
}

// --- row → domain mapping ------------------------------------------------

// userFromRow assembles identity.User: base row + credential + temp password
// (open or last consumed-for-restricted-session) + active app grants + latest
// recovery code. Project memberships are queried only when the backend serves
// project access; the Viderse role must never touch project tables.
func (a *Accounts) userFromRow(ctx context.Context, q *gen.Queries, row gen.User) (identity.User, error) {
	u := identity.User{
		Username:           row.Username,
		Name:               row.DisplayName,
		Role:               identity.Role(row.GlobalRole),
		Disabled:           row.Status == "locked",
		OnboardedAt:        tsPtr(row.OnboardedAt),
		MustChangePassword: row.MustChangePassword,
		DisabledAt:         tsPtr(row.DisabledAt),
		DisabledReason:     row.DisabledReason,
		DeletedAt:          tsPtr(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		Projects:           map[string]identity.ProjectRole{},
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
		u.TempPassword = &identity.TempPassword{CodeHash: temp.PasswordHash, ExpiresAt: temp.ExpiresAt.Time, UsedAt: tsPtr(temp.UsedAt), CreatedAt: temp.CreatedAt.Time}
	} else if err != nil && !noRows(err) {
		return u, fmt.Errorf("đọc mật khẩu tạm: %w", sanitizeErr(err))
	}
	if rec, err := q.GetLatestRecoveryCode(ctx, row.ID); err == nil {
		u.Recovery = &identity.RecoveryCode{CodeHash: rec.CodeHash, Mask: rec.Mask.String, CreatedAt: rec.CreatedAt.Time, UsedAt: tsPtr(rec.UsedAt)}
	} else if !noRows(err) {
		return u, fmt.Errorf("đọc mã khôi phục: %w", sanitizeErr(err))
	}
	grants, err := q.ListAppGrants(ctx, row.ID)
	if err != nil {
		return u, fmt.Errorf("đọc quyền ứng dụng: %w", sanitizeErr(err))
	}
	explicit := make(map[identity.App]identity.Role, len(grants))
	for _, g := range grants {
		explicit[identity.App(g.App)] = identity.Role(g.Role)
	}
	u.Grants = identity.EffectiveGrants(u.Role, explicit)
	if a.opts.ProjectAccess {
		members, err := q.ListMembershipsByUser(ctx, row.ID)
		if err != nil {
			return u, fmt.Errorf("đọc quyền dự án: %w", sanitizeErr(err))
		}
		for _, m := range members {
			u.Projects[m.Slug] = identity.ProjectRole(m.Role)
		}
	}
	return u, nil
}

func (a *Accounts) userByUsername(ctx context.Context, q *gen.Queries, username string) (identity.User, gen.User, error) {
	row, err := q.GetUserByUsername(ctx, username)
	if err != nil {
		if noRows(err) {
			return identity.User{}, gen.User{}, identity.ErrNotFound
		}
		return identity.User{}, gen.User{}, fmt.Errorf("đọc tài khoản: %w", sanitizeErr(err))
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

func (a *Accounts) Users() ([]identity.UserListView, error) {
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
	out := make([]identity.UserListView, 0, len(rows))
	for _, row := range rows {
		u, err := a.userFromRow(ctx, q, row)
		if err != nil {
			return nil, err
		}
		v := identity.UserListView{UserView: u.View()}
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

// validateNewUser normalizes and validates account input. Project grants are
// rejected outright when this backend has no project surface.
func (a *Accounts) validateNewUser(in *identity.NewUser) (map[string]identity.ProjectRole, error) {
	in.Username = strings.TrimSpace(in.Username)
	if !identity.ValidUsername(in.Username) {
		return nil, fmt.Errorf("%w: tên đăng nhập chỉ gồm a-z, 0-9, . _ - (2–32 ký tự)", identity.ErrInvalidInput)
	}
	in.Name = strings.TrimSpace(in.Name)
	if len([]rune(in.Name)) > identity.MaxNameLen {
		return nil, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", identity.ErrInvalidInput, identity.MaxNameLen)
	}
	if in.Role == "" {
		in.Role = identity.RoleMember
	}
	if in.Role != identity.RoleAdmin && in.Role != identity.RoleMember {
		return nil, fmt.Errorf("%w: vai trò %q", identity.ErrInvalidInput, in.Role)
	}
	if len(in.Projects) > 0 && !a.opts.ProjectAccess {
		return nil, fmt.Errorf("%w: tài khoản backend này không nhận quyền dự án", identity.ErrUnsupported)
	}
	projects := map[string]identity.ProjectRole{}
	for id, level := range in.Projects {
		if !a.project(id) || (level != identity.ProjectOwner && level != identity.ProjectCollab && level != identity.ProjectView) {
			return nil, fmt.Errorf("%w: quyền dự án không hợp lệ", identity.ErrInvalidInput)
		}
		projects[id] = level
	}
	return projects, nil
}

// requestedApps resolves the explicit app grants of a new account: the
// server-side choice wins; with none, the Noverse backend keeps the historical
// Noverse access while a project-less backend grants nothing.
func (a *Accounts) requestedApps(in identity.NewUser) ([]identity.App, error) {
	apps := in.Apps
	if len(apps) == 0 && a.opts.ProjectAccess {
		apps = []identity.App{identity.AppNoverse}
	}
	seen := make(map[identity.App]bool, len(apps))
	out := make([]identity.App, 0, len(apps))
	for _, app := range apps {
		if !app.Valid() {
			return nil, identity.ErrUnknownApp
		}
		if seen[app] {
			continue
		}
		seen[app] = true
		out = append(out, app)
	}
	return out, nil
}

func validatePasswordInput(p string) error {
	if len([]rune(p)) < identity.MinPasswordLen {
		return fmt.Errorf("%w: mật khẩu cần ít nhất %d ký tự", identity.ErrInvalidInput, identity.MinPasswordLen)
	}
	return nil
}

func (a *Accounts) CreateUser(in identity.NewUser) (identity.UserView, error) {
	projects, err := a.validateNewUser(&in)
	if err != nil {
		return identity.UserView{}, err
	}
	apps, err := a.requestedApps(in)
	if err != nil {
		return identity.UserView{}, err
	}
	if err := validatePasswordInput(in.Password); err != nil {
		return identity.UserView{}, err
	}
	// Argon2 outside the transaction: the slow hash must not hold a connection.
	hash, err := identity.HashPasswordForBackend(in.Password)
	if err != nil {
		return identity.UserView{}, err
	}
	var created gen.User
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		if _, err := q.GetUserByUsernameAny(ctx, in.Username); err == nil {
			return identity.ErrExists
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
		actorID := pgtype.UUID{}
		if in.Actor != nil && strings.TrimSpace(in.Actor.Username) != "" {
			if actor, err := q.GetUserByUsername(ctx, in.Actor.Username); err == nil {
				actorID = actor.ID
			}
		}
		if in.MustChangePassword {
			tempID, err := randomUUID()
			if err != nil {
				return err
			}
			if err := q.InsertTempPassword(ctx, gen.InsertTempPasswordParams{
				ID: tempID, UserID: userID, PasswordHash: hash,
				ExpiresAt: ts(now.Add(identity.TempPasswordTTL)), CreatedBy: actorID,
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
		if a.opts.ProjectAccess {
			if err := a.replaceMemberships(ctx, q, created, map[string]identity.ProjectRole{}, projects, in.Actor, ""); err != nil {
				return err
			}
		}
		for _, app := range apps {
			if err := q.InsertAppGrant(ctx, gen.InsertAppGrantParams{
				UserID: userID, App: string(app), Role: string(identity.RoleMember),
				GrantedBy: actorID, GrantedAt: ts(now),
			}); err != nil {
				return sanitizeErr(err)
			}
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
		return identity.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), in.Username)
	if err != nil {
		return identity.UserView{}, err
	}
	return u.View(), nil
}

func (a *Accounts) CreateFirstAdmin(in identity.NewUser, device, ip string) (identity.NewSession, error) {
	if _, err := a.validateNewUser(&in); err != nil {
		return identity.NewSession{}, err
	}
	apps, err := a.requestedApps(in)
	if err != nil {
		return identity.NewSession{}, err
	}
	if err := validatePasswordInput(in.Password); err != nil {
		return identity.NewSession{}, err
	}
	hash, err := identity.HashPasswordForBackend(in.Password)
	if err != nil {
		return identity.NewSession{}, err
	}
	var out identity.NewSession
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
			return identity.ErrExists
		}
		userID, err := randomUUID()
		if err != nil {
			return err
		}
		now := a.now()
		row, err := q.InsertUser(ctx, gen.InsertUserParams{
			ID: userID, Username: in.Username, DisplayName: in.Name,
			GlobalRole: string(identity.RoleAdmin), CreatedAt: ts(now),
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if err := q.InsertCredential(ctx, gen.InsertCredentialParams{
			UserID: userID, PasswordHash: pgtype.Text{String: hash, Valid: true}, PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		for _, app := range apps {
			if err := q.InsertAppGrant(ctx, gen.InsertAppGrantParams{
				UserID: userID, App: string(app), Role: string(identity.RoleMember),
				GrantedAt: ts(now),
			}); err != nil {
				return sanitizeErr(err)
			}
		}
		if err := a.addAudit(ctx, q, nil, "", "acct", "Tạo tài khoản quản trị đầu tiên", in.Username, "", "", ""); err != nil {
			return err
		}
		if err := a.addAudit(ctx, q, nil, "", "perm", "Đổi quyền hệ thống", in.Username, "", string(identity.RoleAdmin), ""); err != nil {
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

func (a *Accounts) UpdateUser(username string, patch identity.UserPatch) (identity.UserView, error) {
	if !a.opts.ProjectAccess && (patch.Role != nil || patch.Projects != nil || patch.Disabled != nil) {
		return identity.UserView{}, fmt.Errorf("%w: thao tác quyền hệ thống/dự án không có trên backend này", identity.ErrUnsupported)
	}
	var projects map[string]identity.ProjectRole
	if patch.Projects != nil {
		projects = map[string]identity.ProjectRole{}
		for id, level := range *patch.Projects {
			if !a.project(id) || (level != identity.ProjectOwner && level != identity.ProjectCollab && level != identity.ProjectView) {
				return identity.UserView{}, fmt.Errorf("%w: quyền dự án không hợp lệ", identity.ErrInvalidInput)
			}
			projects[id] = level
		}
	}
	if patch.Role != nil && *patch.Role != identity.RoleAdmin && *patch.Role != identity.RoleMember {
		return identity.UserView{}, fmt.Errorf("%w: vai trò %q", identity.ErrInvalidInput, *patch.Role)
	}
	if patch.Name != nil && len([]rune(strings.TrimSpace(*patch.Name))) > identity.MaxNameLen {
		return identity.UserView{}, fmt.Errorf("%w: tên hiển thị tối đa %d ký tự", identity.ErrInvalidInput, identity.MaxNameLen)
	}
	hash := ""
	if patch.Password != nil {
		if err := validatePasswordInput(*patch.Password); err != nil {
			return identity.UserView{}, err
		}
		var err error
		// Argon2 outside the transaction.
		if hash, err = identity.HashPasswordForBackend(*patch.Password); err != nil {
			return identity.UserView{}, err
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
				return identity.ErrNotFound
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
		if a.opts.ProjectAccess {
			if err := a.checkOwnerInvariants(ctx, q, row, status == "locked", before.Projects, newProjects); err != nil {
				return err
			}
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
				return identity.ErrNotFound
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
				// Admin reset/issue → one-time temporary password; the old
				// password stops working.
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
					ExpiresAt: ts(now.Add(identity.TempPasswordTTL)), CreatedBy: createdBy,
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
		if before.Role == identity.RoleAdmin && !before.Disabled &&
			(role != string(identity.RoleAdmin) || status == "locked") {
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return sanitizeErr(err)
			}
			if n == 0 {
				return identity.ErrLastAdmin
			}
		}
		if patch.Role != nil && before.Role != identity.Role(role) {
			// A privilege change invalidates every existing session: the next
			// login issues a new session id with the new effective grants.
			revoke = true
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
			reason := "admin_changed"
			if status == "locked" {
				reason = "locked"
			}
			if _, err := a.revokeUserAuth(ctx, q, tx, row.ID, reason, identity.ActorApp(patch.Actor)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return identity.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return identity.UserView{}, err
	}
	return u.View(), nil
}

func (a *Accounts) DeleteUser(username string, auditCtxs ...identity.AuditContext) error {
	if err := a.requireProjectAccess(); err != nil {
		return err
	}
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
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		u, err := a.userFromRow(ctx, q, row)
		if err != nil {
			return err
		}
		// Do not delete the last owner: transfer/delete the project first.
		for p, role := range u.Projects {
			if role == identity.ProjectOwner && !u.Disabled {
				pid, err := projectID(ctx, q, p)
				if err != nil {
					return err
				}
				n, err := q.CountActiveOwners(ctx, pid)
				if err != nil {
					return sanitizeErr(err)
				}
				if n == 1 {
					return identity.ErrLastOwner
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
		if u.Role == identity.RoleAdmin && !u.Disabled {
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return sanitizeErr(err)
			}
			if n == 0 {
				return identity.ErrLastAdmin
			}
		}
		_, err = a.revokeUserAuth(ctx, q, tx, row.ID, "admin_changed", identity.ActorApp(ctx_.Actor))
		return err
	})
}

func auditContextOf(context []identity.AuditContext) identity.AuditContext {
	if len(context) > 0 {
		return context[0]
	}
	return identity.AuditContext{}
}

// revokeUserAuth revokes every session (current one included) and all pairings
// of the account; the revoked session IDs come back for callers that broadcast
// them (Recover).
func (a *Accounts) revokeUserAuth(ctx context.Context, q *gen.Queries, tx pgx.Tx, userID pgtype.UUID, reason string, app identity.App) ([]string, error) {
	revoked, err := revokeAllUserSessions(ctx, tx, userID, a.now(), reason, app)
	if err != nil {
		return nil, err
	}
	if err := q.RevokeUserPairings(ctx, gen.RevokeUserPairingsParams{UserID: userID, RevokedAt: ts(a.now())}); err != nil {
		return nil, sanitizeErr(err)
	}
	return revoked, nil
}

func (a *Accounts) MarkOnboarded(username string) (identity.UserView, error) {
	err := RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		return q.MarkOnboarded(ctx, gen.MarkOnboardedParams{ID: row.ID, OnboardedAt: ts(a.now())})
	})
	if err != nil {
		return identity.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return identity.UserView{}, err
	}
	return u.View(), nil
}
