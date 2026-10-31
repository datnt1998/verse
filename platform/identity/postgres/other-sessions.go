package postgres

import (
	"context"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (a *Accounts) RevokeOtherSessions(username, currentID string, audit identity.AuditContext) error {
	id, err := parseUUID(currentID)
	if err != nil {
		return identity.ErrUnauthenticated
	}
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		user, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrUnauthenticated
			}
			return sanitizeErr(err)
		}
		current, err := q.GetSessionByID(ctx, id)
		if err != nil {
			if noRows(err) {
				return identity.ErrUnauthenticated
			}
			return sanitizeErr(err)
		}
		if current.UserID != user.ID || current.RevokedAt.Valid || !current.ExpiresAt.Time.After(a.now()) {
			return identity.ErrUnauthenticated
		}
		app := identity.ActorApp(audit.Actor)
		ids, err := q.RevokeOtherUserSessions(ctx, gen.RevokeOtherUserSessionsParams{UserID: user.ID, RevokedAt: ts(a.now()), CurrentID: id, RevokedApp: pgtype.Text{String: string(app), Valid: app.Valid()}})
		if err != nil {
			return sanitizeErr(err)
		}
		if len(ids) == 0 {
			return nil
		}
		return a.addAudit(ctx, q, audit.Actor, "", "sec", "Đăng xuất các thiết bị khác", username, "", "", audit.Reason)
	})
}
