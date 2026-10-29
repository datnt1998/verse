package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// --- change password --------------------------------------------------------

func (a *Accounts) ChangePassword(username, current, next, keepSession string) error {
	ctx := context.Background()
	u, _, err := a.userByUsername(ctx, a.qp(), username)
	if err != nil {
		return err
	}
	// Phiên restricted (must_change) xác thực "mật khẩu hiện tại" bằng mật
	// khẩu tạm — kể cả khi mã đã đốt/hết hạn, miễn phiên còn hiệu lực.
	currentHash := u.PasswordHash
	if u.MustChangePassword && u.TempPassword != nil {
		currentHash = u.TempPassword.CodeHash
	}
	if currentHash == "" || !webauth.VerifyPasswordForBackend(current, currentHash) {
		return webauth.ErrForbidden
	}
	if err := validatePasswordInput(next); err != nil {
		return err
	}
	hash, err := webauth.HashPasswordForBackend(next)
	if err != nil {
		return err
	}
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
		now := a.now()
		// Storage-conflict parity: if the credential changed between the
		// verify read and this tx, refuse instead of double-hashing.
		cred, err := q.GetCredential(ctx, row.ID)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		if cred.PasswordHash.String != u.PasswordHash {
			return fmt.Errorf("%w: tài khoản vừa được thay đổi, hãy đăng nhập lại", errConflict())
		}
		if err := q.SetCredentialHash(ctx, gen.SetCredentialHashParams{
			UserID: row.ID, PasswordHash: pgtype.Text{String: hash, Valid: true}, PasswordChangedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.RevokeAllTempPasswords(ctx, gen.RevokeAllTempPasswordsParams{UserID: row.ID, RevokedAt: ts(now)}); err != nil {
			return sanitizeErr(err)
		}
		// Clear must_change: the user chose their own permanent password.
		if _, err := q.UpdateUserAfterPatch(ctx, gen.UpdateUserAfterPatchParams{
			ID: row.ID, DisplayName: row.DisplayName, GlobalRole: row.GlobalRole,
			Status: row.Status, MustChangePassword: false, DisabledAt: row.DisabledAt,
			DisabledBy: row.DisabledBy, DisabledReason: row.DisabledReason, UpdatedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		var keepID pgtype.UUID
		if keepSession != "" {
			if parsed, err := parseUUID(strings.ToLower(keepSession)); err == nil {
				keepID = parsed
			}
		}
		if _, err := a.revokeUserAuth(ctx, q, tx, row.ID, keepID); err != nil {
			return err
		}
		return a.addAudit(ctx, q, &webauth.AuditActor{Username: u.Username, Name: u.Name}, "", "sec", "Đổi mật khẩu", username, "", "", "")
	})
}

func errConflict() error { return errs.ErrStorageConflict }

// --- recovery codes -----------------------------------------------------------

func (a *Accounts) IssueRecovery(username string, rotate bool) (string, time.Time, error) {
	raw, err := webauth.NewRecoveryCodeForBackend()
	if err != nil {
		return "", time.Time{}, err
	}
	hash, err := webauth.HashPasswordForBackend(raw)
	if err != nil {
		return "", time.Time{}, err
	}
	mask := webauth.MaskRecoveryCodeForBackend(raw)
	var createdAt time.Time
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		// Recovery codes are admin-only (file parity: ErrForbidden for
		// members) — check role before any code bookkeeping.
		if row.GlobalRole != string(webauth.RoleAdmin) {
			return webauth.ErrForbidden
		}
		rec, recErr := q.GetLatestRecoveryCodeForUpdate(ctx, row.ID)
		if recErr != nil && !noRows(recErr) {
			return sanitizeErr(recErr)
		}
		open := recErr == nil && !rec.UsedAt.Valid && !rec.RevokedAt.Valid
		if open && !rotate {
			return webauth.ErrRecoveryExists
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
	return webauth.FormatRecoveryCodeForBackend(raw), createdAt, nil
}

func (a *Accounts) RecoveryStatus(username string) (webauth.RecoveryStatus, error) {
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return webauth.RecoveryStatus{}, err
	}
	var st webauth.RecoveryStatus
	if u.Recovery != nil {
		st = webauth.RecoveryStatus{CreatedAt: &u.Recovery.CreatedAt, Used: u.Recovery.UsedAt != nil, Mask: maskIfOpen(u.Recovery)}
	}
	return st, nil
}

// maskIfOpen keeps the mask only while an unused code exists (file parity).
func maskIfOpen(r *webauth.RecoveryCode) string {
	if r.UsedAt == nil {
		return r.Mask
	}
	return ""
}

func (a *Accounts) Recover(username, code, newPassword, device, ip string) (webauth.NewSession, []string, error) {
	if err := validatePasswordInput(newPassword); err != nil {
		return webauth.NewSession{}, nil, err
	}
	ctx := context.Background()
	row, err := a.qp().GetUserByUsername(ctx, username)
	var u webauth.User
	if err == nil {
		u, err = a.userFromRow(ctx, a.qp(), row)
	}
	if err != nil && !noRows(err) {
		return webauth.NewSession{}, nil, fmt.Errorf("đọc tài khoản: %w", sanitizeErr(err))
	}
	found := err == nil
	normalized, validShape := webauth.NormalizeRecoveryCodeForBackend(code)
	hash := webauth.DummyHashForBackend()
	if found && u.Recovery != nil && u.Recovery.UsedAt == nil {
		hash = u.Recovery.CodeHash
	}
	if !validShape || !webauth.VerifyPasswordForBackend(normalized, hash) || !found || u.Disabled || u.Recovery == nil || u.Recovery.UsedAt != nil {
		return webauth.NewSession{}, nil, webauth.ErrInvalidRecovery
	}
	newHash, err := webauth.HashPasswordForBackend(newPassword)
	if err != nil {
		return webauth.NewSession{}, nil, err
	}
	var out webauth.NewSession
	var revoked []string
	err = RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		cur, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrInvalidRecovery
			}
			return sanitizeErr(err)
		}
		if cur.Status == "locked" {
			return webauth.ErrInvalidRecovery
		}
		rec, err := q.GetLatestRecoveryCodeForUpdate(ctx, cur.ID)
		if err != nil {
			if noRows(err) {
				return webauth.ErrInvalidRecovery
			}
			return sanitizeErr(err)
		}
		if rec.UsedAt.Valid || rec.RevokedAt.Valid || rec.CodeHash != hash {
			return webauth.ErrInvalidRecovery
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
		if _, err := q.UpdateUserAfterPatch(ctx, gen.UpdateUserAfterPatchParams{
			ID: cur.ID, DisplayName: cur.DisplayName, GlobalRole: cur.GlobalRole,
			Status: cur.Status, MustChangePassword: false, DisabledAt: cur.DisabledAt,
			DisabledBy: cur.DisabledBy, DisabledReason: cur.DisabledReason, UpdatedAt: ts(now),
		}); err != nil {
			return sanitizeErr(err)
		}
		var revokeErr error
		revoked, revokeErr = a.revokeUserAuth(ctx, q, tx, cur.ID, pgtype.UUID{})
		if revokeErr != nil {
			return revokeErr
		}
		out, err = a.newSession(ctx, q, tx, cur, "recovery", pgtype.UUID{}, device, ip)
		return err
	})
	if err != nil {
		return webauth.NewSession{}, nil, err
	}
	return out, revoked, nil
}
