package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/postgres/gen"
)

var _ identity.AppGranter = (*Accounts)(nil)

// AppGrants returns the explicit grant rows of an account (global-admin
// inheritance is not materialized; the user view carries the effective map).
func (a *Accounts) AppGrants(username string) (map[identity.App]identity.Role, error) {
	ctx := context.Background()
	row, err := a.qp().GetUserByUsername(ctx, username)
	if err != nil {
		if noRows(err) {
			return nil, identity.ErrNotFound
		}
		return nil, sanitizeErr(err)
	}
	rows, err := a.qp().ListAppGrants(ctx, row.ID)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make(map[identity.App]identity.Role, len(rows))
	for _, g := range rows {
		out[identity.App(g.App)] = identity.Role(g.Role)
	}
	return out, nil
}

// SetAppGrant grants app membership on behalf of actor. The actor must be a
// global admin; the check runs inside the mutation transaction, so a
// concurrent demotion cannot slip a grant through.
func (a *Accounts) SetAppGrant(username string, app identity.App, role identity.Role, actor *identity.AuditActor, reason string) error {
	return a.mutateAppGrant(username, app, role, actor, reason, false, true)
}

// RevokeAppGrant removes an explicit grant row on behalf of a global admin.
func (a *Accounts) RevokeAppGrant(username string, app identity.App, actor *identity.AuditActor, reason string) error {
	return a.mutateAppGrant(username, app, "", actor, reason, false, false)
}

// OperatorSetAppGrant is the trusted local-operator path (CLI): it does not
// impersonate a user, so audit rows carry actor_kind=system and granted_by
// stays NULL.
func (a *Accounts) OperatorSetAppGrant(username string, app identity.App, role identity.Role) error {
	return a.mutateAppGrant(username, app, role, nil, "", true, true)
}

// OperatorRevokeAppGrant is the trusted local-operator removal path (CLI).
func (a *Accounts) OperatorRevokeAppGrant(username string, app identity.App) error {
	return a.mutateAppGrant(username, app, "", nil, "", true, false)
}

// mutateAppGrant applies one grant change in a single transaction with the
// same lock order as account mutations (invariant row, then the target user):
//   - operator=false: the actor user row is resolved and must be an active
//     global admin; nil actor is forbidden.
//   - operator=true: trusted server operator, no actor user.
//
// The grant row decides idempotency: an unchanged row is a no-op with no audit
// and no session revocation. A real change writes the audit entry and revokes
// EVERY session of the target, so the next login carries fresh grants.
func (a *Accounts) mutateAppGrant(username string, app identity.App, role identity.Role, actor *identity.AuditActor, reason string, operator, grant bool) error {
	if !app.Valid() {
		return identity.ErrUnknownApp
	}
	if grant && role != identity.RoleMember {
		return fmt.Errorf("%w: chỉ cấp quyền member; admin ứng dụng chỉ đến từ vai trò hệ thống", identity.ErrInvalidInput)
	}
	username = strings.TrimSpace(username)
	if !identity.ValidUsername(username) {
		return fmt.Errorf("%w: tên đăng nhập không hợp lệ", identity.ErrInvalidInput)
	}
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		if _, err := q.LockAuthInvariant(ctx); err != nil {
			return sanitizeErr(err)
		}
		var actorID pgtype.UUID
		var auditActor *identity.AuditActor
		if operator {
			auditActor = nil // system actor, granted_by NULL
		} else {
			if actor == nil || strings.TrimSpace(actor.Username) == "" {
				return identity.ErrForbidden
			}
			actorRow, err := q.GetUserByUsername(ctx, actor.Username)
			if err != nil {
				if noRows(err) {
					return identity.ErrForbidden
				}
				return sanitizeErr(err)
			}
			if actorRow.GlobalRole != string(identity.RoleAdmin) || actorRow.Status != "active" {
				return identity.ErrForbidden
			}
			actorID = actorRow.ID
			auditActor = actor
		}
		target, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return identity.ErrNotFound
			}
			return sanitizeErr(err)
		}
		now := a.now()
		existing, err := q.GetAppGrant(ctx, gen.GetAppGrantParams{UserID: target.ID, App: string(app)})
		hasRow := err == nil
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		action := "Cấp quyền ứng dụng"
		from, to := "", string(app)+":"+string(role)
		if !grant {
			action = "Thu hồi quyền ứng dụng"
			from, to = string(app)+":"+string(existing.Role), ""
		}
		switch {
		case grant && hasRow && existing.Role == string(role):
			return nil // no change: no audit, no revocation
		case grant && hasRow:
			if err := q.UpdateAppGrant(ctx, gen.UpdateAppGrantParams{
				UserID: target.ID, App: string(app), Role: string(role),
				GrantedBy: actorID, GrantedAt: ts(now),
			}); err != nil {
				return sanitizeErr(err)
			}
		case grant:
			if err := q.InsertAppGrant(ctx, gen.InsertAppGrantParams{
				UserID: target.ID, App: string(app), Role: string(role),
				GrantedBy: actorID, GrantedAt: ts(now),
			}); err != nil {
				return constraintErr(err)
			}
		case !hasRow:
			return nil // nothing to revoke: no audit, no revocation
		default:
			if err := q.DeleteAppGrant(ctx, gen.DeleteAppGrantParams{UserID: target.ID, App: string(app)}); err != nil {
				return sanitizeErr(err)
			}
		}
		if err := a.addAudit(ctx, q, auditActor, "", "perm", action, username, from, to, reason); err != nil {
			return err
		}
		if _, err := a.revokeUserAuth(ctx, q, tx, target.ID); err != nil {
			return err
		}
		return nil
	})
}
