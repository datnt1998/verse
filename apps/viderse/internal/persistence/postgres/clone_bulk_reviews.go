package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/jackc/pgx/v5"
)

func (s *CloneStore) ListBulkReviewIntents(ctx context.Context, limit int) ([]bulk.ReviewIntent, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT i.job_id::text,i.image_revision FROM clone_bulk_review_intents i JOIN clone_jobs j ON j.id=i.job_id JOIN clone_steps b ON b.job_id=j.id AND b.step='build' WHERE j.superseded_at IS NOT NULL OR j.cancel_requested_at IS NOT NULL OR (j.paused_at IS NULL AND (j.lease_expires_at IS NULL OR j.lease_expires_at<=now()) AND (b.status='succeeded' OR b.status='waiting' AND b.error->>'code'='awaiting_review')) ORDER BY i.created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []bulk.ReviewIntent{}
	for rows.Next() {
		var intent bulk.ReviewIntent
		if err := rows.Scan(&intent.JobID, &intent.ImageRevision); err != nil {
			return nil, err
		}
		out = append(out, intent)
	}
	return out, rows.Err()
}
func (s *CloneStore) ConsumeBulkReviewIntent(ctx context.Context, intent bulk.ReviewIntent, approve func(context.Context, string) error) error {
	id, err := cloneUUIDOf(intent.JobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		row, err := s.q(txctx).GetCloneJobForUpdate(txctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
		var revision int
		if err = tx.QueryRow(txctx, `SELECT image_revision FROM clone_bulk_review_intents WHERE job_id=$1 FOR UPDATE`, id).Scan(&revision); errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if revision != intent.ImageRevision {
			return nil
		}
		clear := func() error {
			_, err := tx.Exec(txctx, `DELETE FROM clone_bulk_review_intents WHERE job_id=$1`, id)
			return err
		}
		job := jobFromGen(row)
		if job.SupersededAt != nil || job.CancelRequestedAt != nil || job.Status == domain.CloneCancelled {
			return clear()
		}
		if row.LeaseExpiresAt.Valid && row.LeaseExpiresAt.Time.After(time.Now()) {
			return nil
		}
		if job.PausedAt != nil {
			return nil
		}
		imageStep, err := s.GetStep(txctx, intent.JobID, domain.CloneImages)
		if err != nil {
			return err
		}
		var value struct {
			Revision int `json:"revision"`
		}
		if len(imageStep.Checkpoint) > 0 {
			if err = json.Unmarshal(imageStep.Checkpoint, &value); err != nil {
				return err
			}
		} else if imageStep.Output != nil {
			raw, err := json.Marshal(imageStep.Output.Data)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(raw, &value); err != nil {
				return err
			}
		}
		if value.Revision != revision {
			return clear()
		}
		// PrepareCloneExport joins this transaction and retains its exact review,
		// live-lease and downstream-score gates. There is no accept override here.
		if err := approve(txctx, intent.JobID); err != nil {
			return err
		}
		return clear()
	})
}

// ApproveDraft stages an owner approval for an image manifest that is already
// committed. It never invalidates or re-renders the published draft:
//   - build parked waiting/awaiting_review: record the durable intent and run
//     the existing PrepareCloneExport CAS in the same transaction;
//   - build pending/running: record the intent; the dispatcher opens the gate
//     once the draft parks;
//   - build succeeded: no-op (the gate is already open or downstream running).
//
// The image revision is CAS-checked so a concurrent redraw cannot be approved
// accidentally. Quality/score gates are not touched (no accept flag).
func (s *CloneStore) ApproveDraft(ctx context.Context, jobID string, imageRevision int) error {
	id, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		row, err := s.q(txctx).GetCloneJobForUpdate(txctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		job := jobFromGen(row)
		if job.SupersededAt != nil {
			return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Job đã được thay thế"}
		}
		if job.CancelRequestedAt != nil || job.Status == domain.CloneCancelled {
			return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Job đã được huỷ"}
		}
		tx, ok := txctx.Value(mediaTxKey{}).(pgx.Tx)
		if !ok {
			return pipeline.ErrFence
		}
		var revision int
		if err = tx.QueryRow(txctx, `SELECT COALESCE((checkpoint->>'revision')::int, NULLIF(output_manifest->'data'->>'revision','')::int, 0) FROM clone_steps WHERE job_id=$1 AND step='images' FOR UPDATE`, id).Scan(&revision); err != nil {
			return err
		}
		if revision != imageRevision {
			return &domain.CloneError{Code: domain.CloneErrStale, Message: "Ảnh đã thay đổi — tải lại rồi duyệt"}
		}
		var buildStatus string
		var code *string
		if err = tx.QueryRow(txctx, `SELECT status,error->>'code' FROM clone_steps WHERE job_id=$1 AND step='build' FOR UPDATE`, id).Scan(&buildStatus, &code); err != nil {
			return err
		}
		switch {
		case buildStatus == "succeeded":
			return nil
		case buildStatus == "pending" || buildStatus == "running":
			_, err = tx.Exec(txctx, `INSERT INTO clone_bulk_review_intents(job_id,image_revision) VALUES($1,$2) ON CONFLICT(job_id) DO UPDATE SET image_revision=EXCLUDED.image_revision,created_at=now()`, id, revision)
			return err
		case buildStatus == "waiting" && code != nil && *code == "awaiting_review":
			if _, err = tx.Exec(txctx, `INSERT INTO clone_bulk_review_intents(job_id,image_revision) VALUES($1,$2) ON CONFLICT(job_id) DO UPDATE SET image_revision=EXCLUDED.image_revision,created_at=now()`, id, revision); err != nil {
				return err
			}
			if row.LeaseExpiresAt.Valid && row.LeaseExpiresAt.Time.After(time.Now()) {
				return nil
			}
			if err = s.PrepareCloneExport(txctx, jobID); err != nil {
				return err
			}
			// The guarded transition ran in this transaction, so the intent is
			// consumed here: leaving it would let a later dispatcher reset
			// score/export a second time.
			_, err = tx.Exec(txctx, `DELETE FROM clone_bulk_review_intents WHERE job_id=$1`, id)
			return err
		default:
			return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có bản dựng chờ duyệt để xuất"}
		}
	})
}
