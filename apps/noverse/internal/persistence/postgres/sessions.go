package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/webauth"
)

// --- sessions: inet/nullable-uuid parameters live in queries/accounts.sql ---

// insertSession stores ONLY the SHA-256 of the cookie secret — plaintext
// tokens never touch the database (session-hijack hardening).
func insertSession(ctx context.Context, tx pgx.Tx, a *Accounts, row gen.User, deviceID, sessionID pgtype.UUID, tokenHash []byte, kind string, tempID pgtype.UUID, ip, ua string, expiresAt time.Time) error {
	err := gen.New(tx).InsertSession(ctx, gen.InsertSessionParams{
		ID:                  sessionID,
		UserID:              row.ID,
		DeviceID:            deviceID,
		TokenHash:           tokenHash,
		CredentialKind:      kind,
		TemporaryPasswordID: tempID,
		LastIp:              inetArg(ip),
		UserAgent:           ua,
		LastSeenAt:          ts(a.now()),
		ExpiresAt:           ts(expiresAt),
	})
	if err != nil {
		return fmt.Errorf("ghi phiên: %w", sanitizeErr(err))
	}
	return nil
}

// upsertDevice idempotently anchors a device (deterministic id by
// username+label) and bumps last_seen/last_ip.
func upsertDevice(ctx context.Context, tx pgx.Tx, a *Accounts, userID pgtype.UUID, name, ip string) (pgtype.UUID, error) {
	id := deterministicUUID("device", fmt.Sprintf("%x|%s", userID.Bytes, name))
	row, err := gen.New(tx).UpsertDevice(ctx, gen.UpsertDeviceParams{
		ID:         id,
		UserID:     userID,
		Name:       name,
		LastIp:     inetArg(ip),
		LastSeenAt: ts(a.now()),
	})
	if err != nil {
		return id, sanitizeErr(err)
	}
	return row.ID, nil
}

// listSessions fetches active sessions; username "" lists every active
// session (admin view). Returns session + owner username + device label.
type pgSessionRow struct {
	Session  gen.Session
	Username string
	Device   string
}

func listSessions(ctx context.Context, db gen.DBTX, userID pgtype.UUID) ([]pgSessionRow, error) {
	rows, err := gen.New(db).ListSessions(ctx, userID)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]pgSessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, pgSessionRow{
			Session: gen.Session{
				ID: r.ID, UserID: r.UserID, DeviceID: r.DeviceID,
				TokenHash: r.TokenHash, CredentialKind: r.CredentialKind,
				TemporaryPasswordID: r.TemporaryPasswordID, LastIp: r.LastIp,
				UserAgent: r.UserAgent, LastSeenAt: r.LastSeenAt,
				ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
			},
			Username: r.Username, Device: r.DeviceName,
		})
	}
	return out, nil
}

// revokeUserSessionsExcept revokes every live session of a user except keep
// and returns the revoked session IDs (Recover reports them for broadcast).
func revokeUserSessionsExcept(ctx context.Context, tx pgx.Tx, userID pgtype.UUID, now time.Time, keep pgtype.UUID) ([]string, error) {
	ids, err := gen.New(tx).RevokeUserSessionsExcept(ctx, gen.RevokeUserSessionsExceptParams{
		UserID: userID, RevokedAt: ts(now), KeepID: keep,
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	revoked := make([]string, 0, len(ids))
	for _, id := range ids {
		revoked = append(revoked, uuidText(id))
	}
	return revoked, nil
}

// newSession creates device + session inside the caller's tx. Returned cookie
// is id.secret — the only copy of the secret; storage keeps its SHA-256.
func (a *Accounts) newSession(ctx context.Context, q *gen.Queries, tx pgx.Tx, row gen.User, kind string, tempID pgtype.UUID, device, ip string) (webauth.NewSession, error) {
	u, err := a.userFromRow(ctx, q, row)
	if err != nil {
		return webauth.NewSession{}, err
	}
	sessionID, err := randomUUID()
	if err != nil {
		return webauth.NewSession{}, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return webauth.NewSession{}, err
	}
	secretHex := hexString(secret)
	deviceID, err := upsertDevice(ctx, tx, a, row.ID, device, ip)
	if err != nil {
		return webauth.NewSession{}, fmt.Errorf("ghi thiết bị: %w", sanitizeErr(err))
	}
	expiresAt := a.now().Add(pgSessionTTL)
	if err := insertSession(ctx, tx, a, row, deviceID, sessionID, digestBytes(secretHex), kind, tempID, ip, device, expiresAt); err != nil {
		return webauth.NewSession{}, err
	}
	return webauth.NewSession{
		Cookie:    uuidText(sessionID) + "." + secretHex,
		ExpiresAt: expiresAt,
		User:      u.View(),
	}, nil
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2], out[i*2+1] = digits[v>>4], digits[v&0xf]
	}
	return string(out)
}

// randomHex returns n crypto-random bytes as lowercase hex (pairing codes).
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hexString(b), nil
}

// --- login / pairing / authenticate ----------------------------------------

func (a *Accounts) Login(username, password, device, ip string) (webauth.NewSession, error) {
	ctx := context.Background()
	username = strings.TrimSpace(username)
	// Load + verify OUTSIDE the tx (argon2 must not hold locks); unknown users
	// still pay the full argon2 cost via a dummy hash — no enumeration.
	row, err := a.qp().GetUserByUsername(ctx, username)
	var u webauth.User
	if err == nil {
		u, err = a.userFromRow(ctx, a.qp(), row)
	}
	if err != nil && !noRows(err) {
		return webauth.NewSession{}, fmt.Errorf("đọc tài khoản: %w", sanitizeErr(err))
	}
	found := err == nil
	now := a.now()
	hash := webauth.DummyHashForBackend()
	tempOpen := false
	if found && u.TempPassword != nil {
		hash = u.TempPassword.CodeHash
		tempOpen = u.TempPassword.UsedAt == nil && u.TempPassword.ExpiresAt.After(now)
	} else if found {
		hash = u.PasswordHash
	}
	if !webauth.VerifyPasswordForBackend(password, hash) || !found || u.Disabled || (u.TempPassword != nil && !tempOpen) {
		return webauth.NewSession{}, webauth.ErrInvalidCredentials
	}
	var out webauth.NewSession
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		q := a.q(tx)
		// Re-check inside the tx: verify ran before the write lock; the
		// account or temp credential may have changed in between.
		cur, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrInvalidCredentials
			}
			return sanitizeErr(err)
		}
		if cur.Status == "locked" {
			return webauth.ErrInvalidCredentials
		}
		tempID := pgtype.UUID{}
		kind := "password"
		if temp, err := q.GetOpenTempPasswordForUpdate(ctx, cur.ID); err == nil {
			if u.TempPassword == nil || temp.PasswordHash != u.TempPassword.CodeHash ||
				!temp.ExpiresAt.Time.After(a.now()) {
				return webauth.ErrInvalidCredentials
			}
			if err := q.ConsumeTempPassword(ctx, gen.ConsumeTempPasswordParams{ID: temp.ID, UsedAt: ts(a.now())}); err != nil {
				return sanitizeErr(err)
			}
			tempID, kind = temp.ID, "temporary"
		} else if noRows(err) {
			if u.TempPassword != nil {
				// Temp was consumed/revoked concurrently.
				return webauth.ErrInvalidCredentials
			}
			cred, err := q.GetCredential(ctx, cur.ID)
			if err != nil {
				if noRows(err) {
					return webauth.ErrInvalidCredentials
				}
				return sanitizeErr(err)
			}
			if !cred.PasswordHash.Valid || cred.PasswordHash.String != u.PasswordHash {
				return webauth.ErrInvalidCredentials
			}
		} else {
			return sanitizeErr(err)
		}
		out, err = a.newSession(ctx, q, tx, cur, kind, tempID, device, ip)
		return err
	})
	return out, err
}

func (a *Accounts) Pair(code, device, ip string) (webauth.NewSession, error) {
	ctx := context.Background()
	h := digestBytes(webauth.DigestForBackend(strings.TrimSpace(code)))
	var out webauth.NewSession
	err := RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		q := a.q(tx)
		p, err := q.GetPairingByTokenHash(ctx, h)
		if err != nil {
			if noRows(err) {
				return webauth.ErrInvalidPairing
			}
			return sanitizeErr(err)
		}
		row, err := q.GetUserByID(ctx, p.UserID)
		if err != nil {
			if noRows(err) {
				return webauth.ErrInvalidPairing
			}
			return sanitizeErr(err)
		}
		if row.Status == "locked" || p.UsedAt.Valid || p.RevokedAt.Valid || !p.ExpiresAt.Time.After(a.now()) {
			return webauth.ErrInvalidPairing
		}
		if err := q.ConsumePairing(ctx, gen.ConsumePairingParams{ID: p.ID, UsedAt: ts(a.now())}); err != nil {
			return sanitizeErr(err)
		}
		out, err = a.newSession(ctx, q, tx, row, "pairing", pgtype.UUID{}, device, ip)
		return err
	})
	return out, err
}

func (a *Accounts) NewPairing(username, createdBy string) (string, time.Time, error) {
	code, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	var expires time.Time
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserByUsername(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		var creator pgtype.UUID
		if by, err := q.GetUserByUsername(ctx, createdBy); err == nil {
			creator = by.ID
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		expires = a.now().Add(pgPairingTTL)
		return q.InsertPairing(ctx, gen.InsertPairingParams{
			ID: id, UserID: row.ID, TokenHash: digestBytes(webauth.DigestForBackend(code)),
			CreatedBy: creator, ExpiresAt: ts(expires),
		})
	})
	return code, expires, err
}

func (a *Accounts) Authenticate(cookie string) (webauth.User, webauth.Session, error) {
	id, secret, ok := strings.Cut(cookie, ".")
	if !ok || id == "" || secret == "" {
		return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
	}
	sessionID, err := parseUUID(id)
	if err != nil {
		return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
	}
	ctx := context.Background()
	q := a.qp()
	sess, err := q.GetSessionByID(ctx, sessionID)
	if err != nil {
		if noRows(err) {
			return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
		}
		return webauth.User{}, webauth.Session{}, sanitizeErr(err)
	}
	row, err := q.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if noRows(err) {
			return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
		}
		return webauth.User{}, webauth.Session{}, sanitizeErr(err)
	}
	now := a.now()
	if row.Status == "locked" || !sess.ExpiresAt.Time.After(now) || sess.RevokedAt.Valid {
		return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
	}
	if !equalBytes(sess.TokenHash, digestBytes(secret)) {
		return webauth.User{}, webauth.Session{}, webauth.ErrUnauthenticated
	}
	if now.Sub(sess.LastSeenAt.Time) >= pgTouchGap {
		// last_seen touch is opportunistic: a failed write must NOT be
		// reported as a newer last-seen value. The session still authenticates.
		if err := RunInTx(ctx, a.pool, func(tx pgx.Tx) error {
			return a.q(tx).TouchSessionLastSeen(ctx, gen.TouchSessionLastSeenParams{ID: sess.ID, LastSeenAt: ts(now)})
		}); err == nil {
			sess.LastSeenAt = ts(now)
		}
	}
	u, err := a.userFromRow(ctx, q, row)
	if err != nil {
		return webauth.User{}, webauth.Session{}, err
	}
	deviceLabel := sess.UserAgent
	return u, sessionView(sess, deviceLabel, id), nil
}

func sessionView(sess gen.Session, deviceLabel, currentID string) webauth.Session {
	return webauth.Session{
		ID:        uuidText(sess.ID),
		Device:    deviceLabel,
		CreatedAt: sess.CreatedAt.Time.UTC(),
		LastSeen:  sess.LastSeenAt.Time.UTC(),
		ExpiresAt: sess.ExpiresAt.Time.UTC(),
		Current:   uuidText(sess.ID) == currentID,
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	compact := strings.ReplaceAll(strings.ToLower(s), "-", "")
	if len(compact) != 32 {
		return id, fmt.Errorf("uuid ngắn")
	}
	b := make([]byte, 16)
	for i := 0; i < 16; i++ {
		hi, err1 := hexDigit(compact[i*2])
		lo, err2 := hexDigit(compact[i*2+1])
		if err1 != nil || err2 != nil {
			return id, fmt.Errorf("uuid không hợp lệ")
		}
		b[i] = hi<<4 | lo
	}
	copy(id.Bytes[:], b)
	id.Valid = true
	return id, nil
}

func hexDigit(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	}
	return 0, fmt.Errorf("hex")
}

// --- session management -----------------------------------------------------

func (a *Accounts) Sessions(username, currentID string) ([]webauth.Session, error) {
	ctx := context.Background()
	var userID pgtype.UUID
	if strings.TrimSpace(username) != "" {
		row, err := a.qp().GetUserByUsername(ctx, strings.TrimSpace(username))
		if err != nil {
			if noRows(err) {
				return nil, webauth.ErrNotFound
			}
			return nil, sanitizeErr(err)
		}
		userID = row.ID
	}
	rows, err := listSessions(ctx, a.pool, userID)
	if err != nil {
		return nil, err
	}
	out := make([]webauth.Session, 0, len(rows))
	for _, r := range rows {
		out = append(out, sessionView(r.Session, r.Device, currentID))
	}
	return out, nil
}

func (a *Accounts) RevokeSession(id string, allowed func(owner string) bool, auditCtxs ...webauth.AuditContext) error {
	ctx_ := auditContextOf(auditCtxs)
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		sessionID, err := parseUUID(strings.ToLower(strings.TrimSpace(id)))
		if err != nil {
			return webauth.ErrNotFound
		}
		sess, err := q.GetSessionByID(ctx, sessionID)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		row, err := q.GetUserByID(ctx, sess.UserID)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		if !allowed(row.Username) {
			return webauth.ErrForbidden
		}
		if sess.RevokedAt.Valid {
			return nil // no-op: don't add audit (file parity)
		}
		if err := q.RevokeSession(ctx, gen.RevokeSessionParams{ID: sess.ID, RevokedAt: ts(a.now())}); err != nil {
			return sanitizeErr(err)
		}
		return a.addAudit(ctx, q, ctx_.Actor, "", "sec", "Thu hồi phiên", row.Username, "", "", ctx_.Reason)
	})
}

func (a *Accounts) RevokeAll(username, keep string, auditCtx webauth.AuditContext) error {
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		var keepID pgtype.UUID
		if keep != "" {
			if parsed, err := parseUUID(strings.ToLower(keep)); err == nil {
				keepID = parsed
			}
		}
		if _, err := a.revokeUserAuth(ctx, q, tx, row.ID, keepID); err != nil {
			return err
		}
		return a.addAudit(ctx, q, auditCtx.Actor, "", "sec", "Thu hồi mọi phiên", username, "", "", auditCtx.Reason)
	})
}
