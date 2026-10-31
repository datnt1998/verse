package postgres

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/jackc/pgx/v5"
)

// PrepareCloneExport is the explicit phase-05 review approval, not ResumeJob:
// generic resume intentionally leaves an awaiting_review gate closed.
func (s *CloneStore) PrepareCloneExport(ctx context.Context, jobID string) error {
	id, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.GetCloneJobForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if row.LeaseExpiresAt.Valid && row.LeaseExpiresAt.Time.After(time.Now()) {
			return pipeline.ErrConflict
		}
		job := jobFromGen(row)
		if job.SupersededAt != nil {
			return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Job đã được thay thế"}
		}
		tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx)
		if !ok {
			return pipeline.ErrFence
		}
		// The current build evidence must exist before any export can be approved.
		var buildStatus string
		var code *string
		if err = tx.QueryRow(ctx, `SELECT status,error->>'code' FROM clone_steps WHERE job_id=$1 AND step='build' FOR UPDATE`, id).Scan(&buildStatus, &code); err != nil {
			return err
		}
		if buildStatus != "succeeded" && (buildStatus != "waiting" || code == nil || *code != "awaiting_review") {
			return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có bản dựng chờ duyệt để xuất"}
		}
		if buildStatus == "succeeded" && job.Status == domain.ClonePending {
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE clone_steps SET status='succeeded',error=NULL,finished_at=$2,updated_at=$2 WHERE job_id=$1 AND step='build' AND status='waiting' AND error->>'code'='awaiting_review'`, id, time.Now()); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE clone_steps SET status='pending',error=NULL,output_manifest=NULL,finished_at=NULL,updated_at=$2 WHERE job_id=$1 AND step IN ('score','export')`, id, time.Now()); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE clone_jobs SET cancel_requested_at=NULL,paused_at=NULL WHERE id=$1`, id); err != nil {
			return err
		}
		if err = q.SetJobStatus(ctx, gen.SetJobStatusParams{ID: id, Status: "pending", Column3: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(ctx, id, nil)
	})
}
