package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/postgres/gen"
)

// --- change password --------------------------------------------------------

// ChangePassword verifies the current password, stores the new hash and
// revokes EVERY session of the account — including the calling one. Both apps
// must sign in again with the new credential.
func (a *Accounts) ChangePassword(username, current, next string, auditCtxs ...identity.AuditContext) error {
	ctx := context.Background()
	u, _, err := a.userByUsername(ctx, a.qp(), username)
	if err != nil {
		return err
	}
	// A restricted session (must_change) authenticates "current password" with
	// the temporary password — even after it was consumed/expired, as long as
	// the session itself is still valid.
	currentHash := u.PasswordHash
	if u.MustChangePassword && u.TempPassword != nil {
		currentHash = u.TempPassword.CodeHash
	}
	if currentHash == "" || !identity.VerifyPasswordForBackend(current, currentHash) {
		return identity.ErrForbidden
	}
	if err := validatePasswordInput(next); err != nil {
		return err
	}
	hash, err := identity.HashPasswordForBackend(next)
	if err != nil {
		return err
	}
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		now := a.now()
		// Storage-conflict parity: if the credential changed between the
		// verify read and this tx, refuse instead of double-hashing.
		cred, err := q.GetCredential(ctx, row.ID)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		if cred.PasswordHash.String != u.PasswordHash {
			return fmt.Errorf("%w: tài khoản vừa được thay đổi, hãy đăng nhập lại", identity.ErrConflict)
		}
		if err := q.SetCredentialHash(ctx, gen.SetCredentialHashParams{
			UserID: row.ID, PasswordHash: pgtype.Text{String: hash, Valid: true}, PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.RevokeAllTempPasswords(ctx, gen.RevokeAllTempPasswordsParams{UserID: row.ID, RevokedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		// Clear must_change with the column-minimal statement: the
		// least-privilege identity role may UPDATE only these columns.
		if err := q.ClearMustChangePassword(ctx, gen.ClearMustChangePasswordParams{ID: row.ID, UpdatedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		if _, err := a.revokeUserAuth(ctx, q, tx, row.ID, "password_changed", identity.ActorApp(auditContextOf(auditCtxs).Actor)); err != nil {
			return err
		}
		return a.addAudit(ctx, q, &identity.AuditActor{Username: u.Username, Name: u.Name}, "", "sec", "Đổi mật khẩu", username, "", "", "")
	})
}

// --- recovery codes -----------------------------------------------------------

func (a *Accounts) IssueRecovery(username string, rotate bool) (string, time.Time, error) {
	raw, err := identity.NewRecoveryCodeForBackend()
	if err != nil {
		return "", time.Time{}, err
	}
	hash, err := identity.HashPasswordForBackend(raw)
	if err != nil {
		return "", time.Time{}, err
	}
	mask := identity.MaskRecoveryCodeForBackend(raw)
	var createdAt time.Time
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		// Recovery codes are admin-only (file parity: ErrForbidden for
		// members) — check role before any code bookkeeping.
		if row.GlobalRole != string(identity.RoleAdmin) {
			return identity.ErrForbidden
		}
		rec, recErr := q.GetLatestRecoveryCodeForUpdate(ctx, row.ID)
		if recErr != nil && !noRows(recErr) {
			return sanitizeErr(recErr)
		}
		open := recErr == nil && !rec.UsedAt.Valid && !rec.RevokedAt.Valid
		if open && !rotate {
			return identity.ErrRecoveryExists
		}
		if open {
			if err := q.RevokeRecoveryCode(ctx, gen.RevokeRecoveryCodeParams{ID: rec.ID, RevokedAt: ts(a.now())}); err != nil {
				return sanitizeErr(err)
			}
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		createdAt = a.now()
		return q.InsertRecoveryCode(ctx, gen.InsertRecoveryCodeParams{
			ID: id, UserID: row.ID, CodeHash: hash, Mask: pgtype.Text{String: mask, Valid: true},
		})
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return identity.FormatRecoveryCodeForBackend(raw), createdAt, nil
}

func (a *Accounts) RecoveryStatus(username string) (identity.RecoveryStatus, error) {
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return identity.RecoveryStatus{}, err
	}
	var st identity.RecoveryStatus
	if u.Recovery != nil {
		st = identity.RecoveryStatus{CreatedAt: &u.Recovery.CreatedAt, Used: u.Recovery.UsedAt != nil, Mask: maskIfOpen(u.Recovery)}
	}
	return st, nil
}

// maskIfOpen keeps the mask only while an unused code exists (file parity).
func maskIfOpen(r *identity.RecoveryCode) string {
	if r.UsedAt == nil {
		return r.Mask
	}
	return ""
}

func (a *Accounts) Recover(username, code, newPassword, device, ip string) (identity.NewSession, []string, error) {
	if err := validatePasswordInput(newPassword); err != nil {
		return identity.NewSession{}, nil, err
	}
	ctx := context.Background()
	row, err := a.qp().GetUserByUsername(ctx, username)
	var u identity.User
	if err == nil {
		u, err = a.userFromRow(ctx, a.qp(), row)
	}
	if err != nil && !noRows(err) {
		return identity.NewSession{}, nil, fmt.Errorf("đọc tài khoản: %w", sanitizeErr(err))
	}
	found := err == nil
	normalized, validShape := identity.NormalizeRecoveryCodeForBackend(code)
	hash := identity.DummyHashForBackend()
	if found && u.Recovery != nil && u.Recovery.UsedAt == nil {
		hash = u.Recovery.CodeHash
	}
	if !validShape || !identity.VerifyPasswordForBackend(normalized, hash) || !found || u.Disabled || u.Recovery == nil || u.Recovery.UsedAt != nil {
		return identity.NewSession{}, nil, identity.ErrInvalidRecovery
	}
	newHash, err := identity.HashPasswordForBackend(newPassword)
	if err != nil {
		return identity.NewSession{}, nil, err
	}
	var out identity.NewSession
	var revoked []string
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		cur, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrInvalidRecovery
			}
			return sanitizeErr(err)
		}
		if cur.Status == "locked" {
			return identity.ErrInvalidRecovery
		}
		rec, err := q.GetLatestRecoveryCodeForUpdate(ctx, cur.ID)
		if err != nil {
			if noRows(err) {
				return identity.ErrInvalidRecovery
			}
			return sanitizeErr(err)
		}
		if rec.UsedAt.Valid || rec.RevokedAt.Valid || rec.CodeHash != hash {
			return identity.ErrInvalidRecovery
		}
		now := a.now()
		if err := q.ConsumeRecoveryCode(ctx, gen.ConsumeRecoveryCodeParams{ID: rec.ID, UsedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.SetCredentialHash(ctx, gen.SetCredentialHashParams{
			UserID: cur.ID, PasswordHash: pgtype.Text{String: newHash, Valid: true}, PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.RevokeAllTempPasswords(ctx, gen.RevokeAllTempPasswordsParams{UserID: cur.ID, RevokedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.ClearMustChangePassword(ctx, gen.ClearMustChangePasswordParams{ID: cur.ID, UpdatedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		var revokeErr error
		revoked, revokeErr = a.revokeUserAuth(ctx, q, tx, cur.ID, "password_changed", "")
		if revokeErr != nil {
			return revokeErr
		}
		out, err = a.newSession(ctx, q, tx, cur, "recovery", pgtype.UUID{}, device, ip)
		return err
	})
	if err != nil {
		return identity.NewSession{}, nil, err
	}
	return out, revoked, nil
}
