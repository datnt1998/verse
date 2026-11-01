package postgres

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/platform/identity"
)

// device_tokens stores the SHA-256 of a known-device token — the plaintext
// never reaches the database, exactly like session secrets. A token is not a
// session: it only tells the login limiter that this browser already proved the
// account password. Revocation is revoked_at so the row stays for audit.
const (
	saveDeviceTokenSQL = `
INSERT INTO device_tokens (id, user_id, token_hash, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (token_hash) DO NOTHING`

	// capDeviceTokensSQL keeps only the newest maxDeviceTokens live tokens of
	// the account: scripted logins that drop cookies must not grow the table.
	capDeviceTokensSQL = `
UPDATE device_tokens SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
  AND id NOT IN (
      SELECT id FROM device_tokens
      WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
      ORDER BY created_at DESC
      LIMIT 10)`

	deviceTokenValidSQL = `
SELECT EXISTS (
    SELECT 1 FROM device_tokens t
    JOIN users u ON u.id = t.user_id
    WHERE u.username = $1 AND u.deleted_at IS NULL
      AND t.token_hash = $2 AND t.revoked_at IS NULL AND t.expires_at > $3)`

	revokeDeviceTokensSQL = `
UPDATE device_tokens SET revoked_at = $2, updated_at = $2, version = version + 1
WHERE user_id = $1 AND revoked_at IS NULL`
)

// hexDigest decodes the hex SHA-256 digest the identity package hands over.
func hexDigest(hash string) ([]byte, error) {
	raw, err := hex.DecodeString(hash)
	if err != nil || len(raw) != 32 {
		return nil, identity.ErrInvalidInput
	}
	return raw, nil
}

// SaveDeviceToken stores the digest of a device token for the account.
func (a *Accounts) SaveDeviceToken(username, tokenHash string, expiresAt time.Time) error {
	digest, err := hexDigest(tokenHash)
	if err != nil {
		return err
	}
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		row, err := a.q(tx).GetUserByUsername(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		now := a.now()
		if _, err := tx.Exec(ctx, saveDeviceTokenSQL, id, row.ID, digest, ts(expiresAt), ts(now)); err != nil {
			return sanitizeErr(err)
		}
		if _, err := tx.Exec(ctx, capDeviceTokensSQL, row.ID, ts(now)); err != nil {
			return sanitizeErr(err)
		}
		return nil
	})
}

// DeviceTokenValid reports whether the digest is a live device token of the
// account. An unknown account answers false: the caller must not learn whether
// it exists from a limiter hint.
func (a *Accounts) DeviceTokenValid(username, tokenHash string) (bool, error) {
	digest, err := hexDigest(tokenHash)
	if err != nil {
		return false, nil
	}
	var ok bool
	err = a.pool.QueryRow(context.Background(), deviceTokenValidSQL, username, digest, ts(a.now())).Scan(&ok)
	if err != nil {
		return false, sanitizeErr(err)
	}
	return ok, nil
}

// RevokeDeviceTokens forgets every known device of the account.
func (a *Accounts) RevokeDeviceTokens(username string) error {
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		row, err := a.q(tx).GetUserByUsername(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		return a.revokeUserDeviceTokens(ctx, tx, row.ID, a.now())
	})
}

// revokeUserDeviceTokens revokes every live device token of a user inside the
// caller's transaction — the device half of revokeUserAuth.
func (a *Accounts) revokeUserDeviceTokens(ctx context.Context, tx pgx.Tx, userID pgtype.UUID, now time.Time) error {
	if _, err := tx.Exec(ctx, revokeDeviceTokensSQL, userID, ts(now)); err != nil {
		return sanitizeErr(err)
	}
	return nil
}
