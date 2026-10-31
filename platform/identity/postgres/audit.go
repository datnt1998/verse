package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/postgres/gen"
)

// addAudit appends one audit entry inside the caller's transaction — the log
// is part of the same atomic write as the mutation it records.
//
// Account/grant entries use the project-less INSERT, which never returns
// columns: the least-privilege identity role holds INSERT on that column list
// only. Project-scoped entries (Noverse membership operations) keep project_id.
func (a *Accounts) addAudit(ctx context.Context, q *gen.Queries, actor *identity.AuditActor, project, typ, action, target, from, to, reason string) error {
	id, err := randomUUID()
	if err != nil {
		return err
	}
	var actorID pgtype.UUID
	kind := "system"
	if actor != nil && strings.TrimSpace(actor.Username) != "" {
		if row, err := q.GetUserByUsername(ctx, actor.Username); err == nil {
			actorID, kind = row.ID, "user"
		} else if !noRows(err) {
			return sanitizeErr(err)
		}
	}
	details := map[string]string{}
	if from != "" {
		details["from"] = from
	}
	if to != "" {
		details["to"] = to
	}
	if strings.TrimSpace(reason) != "" {
		details["reason"] = strings.TrimSpace(reason)
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if project != "" {
		p, err := q.GetProjectBySlug(ctx, project)
		if err == nil {
			err = q.InsertProjectAudit(ctx, gen.InsertProjectAuditParams{
				ID:          id,
				ProjectID:   p.ID,
				ActorUserID: actorID,
				ActorKind:   kind,
				Action:      action,
				TargetKind:  typ,
				TargetID:    target,
				Details:     payload,
				OccurredAt:  ts(a.now()),
			})
			return sanitizeErr(err)
		}
		if !noRows(err) {
			return sanitizeErr(err)
		}
		// A missing project anchor degrades to an account entry (NULL project)
		// instead of inventing a project row.
	}
	err = q.InsertAccountAudit(ctx, gen.InsertAccountAuditParams{
		ID:          id,
		ActorUserID: actorID,
		ActorKind:   kind,
		Action:      action,
		TargetKind:  typ,
		TargetID:    target,
		Details:     payload,
		OccurredAt:  ts(a.now()),
	})
	return sanitizeErr(err)
}

// Audit reads entries newest-first with the same filter/cursor semantics as
// the file backend: cursor is the zero-padded entry id, Next is the id of
// the last returned item when a full page was emitted.
func (a *Accounts) Audit(query identity.AuditQuery) (identity.AuditPage, error) {
	if err := a.requireProjectAccess(); err != nil {
		return identity.AuditPage{}, err
	}
	if query.Type != "" && query.Type != "acct" && query.Type != "perm" && query.Type != "sec" {
		return identity.AuditPage{}, fmt.Errorf("%w: loại nhật ký không hợp lệ", identity.ErrInvalidInput)
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	ctx := context.Background()
	rows, err := listAuditRows(ctx, a.pool, query.Type, strings.TrimSpace(query.Actor), seqBefore(query.Before), query.Limit+1)
	if err != nil {
		return identity.AuditPage{}, err
	}
	page := identity.AuditPage{Items: []identity.AuditEntry{}}
	for i, r := range rows {
		if i == query.Limit {
			page.Next = page.Items[len(page.Items)-1].ID
			break
		}
		var details map[string]string
		_ = json.Unmarshal(r.Details, &details)
		entry := identity.AuditEntry{
			ID:      seqID(r.Sequence),
			At:      r.OccurredAt.Time,
			Type:    r.TargetKind,
			Action:  r.Action,
			Target:  r.TargetID,
			Project: r.ProjectSlug.String,
			From:    details["from"],
			To:      details["to"],
			Reason:  details["reason"],
		}
		if r.ActorUsername != "" {
			entry.Actor = &identity.AuditActor{Username: r.ActorUsername, Name: r.ActorName}
		}
		page.Items = append(page.Items, entry)
	}
	return page, nil
}

type auditRow struct {
	Sequence      int64
	ActorKind     string
	Action        string
	TargetKind    string
	TargetID      string
	Details       []byte
	OccurredAt    pgtype.Timestamptz
	ProjectSlug   pgtype.Text
	ActorUsername string
	ActorName     string
}

// listAuditRows filters server-side; Before is the entry-id cursor.
func listAuditRows(ctx context.Context, db gen.DBTX, typ, actor string, beforeSeq int64, limit int) ([]auditRow, error) {
	rows, err := gen.New(db).ListAudit(ctx, gen.ListAuditParams{
		Column1: typ, Column2: actor, Column3: beforeSeq, Limit: int32(limit),
	})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := make([]auditRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, auditRow{
			Sequence: r.Sequence, ActorKind: r.ActorKind, Action: r.Action,
			TargetKind: r.TargetKind, TargetID: r.TargetID, Details: r.Details,
			OccurredAt: r.OccurredAt, ProjectSlug: r.ProjectSlug,
			ActorUsername: r.ActorUsername, ActorName: r.ActorName,
		})
	}
	return out, nil
}

// PurgeAuditBefore previews or deletes audit history under the maintenance
// principal. A dry-run performs only SELECTs; real retention commits at most
// 1000 rows per transaction and returns already committed progress on failure.
func (a *Accounts) PurgeAuditBefore(ctx context.Context, cutoff time.Time, dryRun bool) (int64, error) {
	if err := a.requireProjectAccess(); err != nil {
		return 0, err
	}
	var allowed bool
	if err := a.pool.QueryRow(ctx, `SELECT noverse_is_maintenance()`).Scan(&allowed); err != nil {
		return 0, sanitizeErr(err)
	}
	if !allowed {
		return 0, fmt.Errorf("storage purge-audit yêu cầu vai trò bảo trì noverse_migrate qua MIGRATION_DATABASE_URL")
	}
	if dryRun {
		n, err := gen.New(a.pool).CountAuditBefore(ctx, ts(cutoff))
		return n, sanitizeErr(err)
	}
	var total int64
	for {
		var n int64
		err := RunInTx(ctx, a.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SET LOCAL noverse.allow_immutable_write = 'on'`); err != nil {
				return sanitizeErr(err)
			}
			var err error
			n, err = a.q(tx).DeleteAuditRetentionBatch(ctx, gen.DeleteAuditRetentionBatchParams{Cutoff: ts(cutoff), BatchSize: 1000})
			return sanitizeErr(err)
		})
		if err != nil {
			return total, err
		}
		total += n
		if n < 1000 {
			return total, nil
		}
	}
}

// seqBefore parses "audit-00000042" (or bare digits) back to a sequence.
func seqBefore(id string) int64 {
	s := strings.TrimPrefix(strings.TrimSpace(id), "audit-")
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

func seqID(seq int64) string {
	return fmt.Sprintf("audit-%08d", seq)
}

func (a *Accounts) RecordSecurityFailure(username string) error {
	username = strings.TrimSpace(username)
	if !identity.ValidUsername(username) {
		return nil
	}
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		if _, err := q.GetUserByUsername(ctx, username); err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		return a.addAudit(ctx, q, nil, "", "sec", "Đăng nhập sai nhiều lần", username, "", "", "")
	})
}
