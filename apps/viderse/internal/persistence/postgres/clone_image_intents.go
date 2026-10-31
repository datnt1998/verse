package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// overlayImageIntents projects accepted owner changes over the worker checkpoint.
// The worker remains the only writer of the checkpoint while its lease is live.
func overlayImageIntents(ctx context.Context, tx pgx.Tx, id any, m *images.Manifest) (int, error) {
	rows, err := tx.Query(ctx, `SELECT image_id,item FROM clone_image_review_intents WHERE job_id=$1 ORDER BY created_at,image_id`, id)
	if err != nil { return 0, err }
	defer rows.Close()
	count := 0
	for rows.Next() {
		var imageID string
		var raw []byte
		if err := rows.Scan(&imageID, &raw); err != nil { return 0, err }
		var item images.Item
		if err := json.Unmarshal(raw, &item); err != nil { return 0, err }
		found := false
		for i := range m.Items {
			if m.Items[i].ID == imageID {
				m.Items[i] = item
				found = true
				break
			}
		}
		if !found { return 0, errors.New("image intent refers to missing image") }
		if item.Status == "pending" {
			selected := false
			for _, id := range m.QueueOnly { if id == imageID { selected = true; break } }
			if !selected { m.QueueOnly = append(m.QueueOnly, imageID) }
		}
		count++
	}
	if err := rows.Err(); err != nil { return 0, err }
	m.Revision += count
	return count, nil
}

func enqueueImageIntents(ctx context.Context, tx pgx.Tx, id any, before, after json.RawMessage, imageIDs []string) error {
	var old, next images.Manifest
	if err := json.Unmarshal(before, &old); err != nil { return err }
	if err := json.Unmarshal(after, &next); err != nil { return err }
	if len(old.Items) != len(next.Items) { return jobedit.Conflict("Danh sách ảnh đã thay đổi; tải lại trước khi sửa") }
	for _, imageID := range imageIDs {
		found := false
		for i := range next.Items {
			if next.Items[i].ID != imageID { continue }
			found = true
			raw, err := json.Marshal(next.Items[i]); if err != nil { return err }
			result, err := tx.Exec(ctx, `INSERT INTO clone_image_review_intents(job_id,image_id,item,expected_revision) VALUES($1,$2,$3::jsonb,$4) ON CONFLICT DO NOTHING`, id, imageID, raw, old.Revision)
			if err != nil { return err }
			if result.RowsAffected() != 1 { return jobedit.Conflict("Ảnh đang chờ xử lý") }
			break
		}
		if !found { return jobedit.Conflict("Danh sách ảnh đã thay đổi; tải lại trước khi sửa") }
	}
	return nil
}

// A review-origin marker is the only field an owner may add to a live
// checkpoint. Preserve it when the worker saves an older in-memory snapshot.
func preserveReviewOrigin(previous, next json.RawMessage) (json.RawMessage, error) {
	var old, current images.Manifest
	if err := json.Unmarshal(previous, &old); err != nil || !old.ReviewOrigin {
		return next, nil
	}
	if err := json.Unmarshal(next, &current); err != nil {
		return nil, err
	}
	if current.ReviewOrigin || images.ImagesApproved(current) {
		return next, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(next, &fields); err != nil {
		return nil, err
	}
	fields["reviewOrigin"] = json.RawMessage("true")
	return json.Marshal(fields)
}

// DrainImageIntents atomically merges accepted edits into the fenced worker
// checkpoint. Deletion and checkpoint persistence commit together.
func (s *CloneStore) DrainImageIntents(ctx context.Context, jobID string, fence int64, attempt int, checkpoint json.RawMessage) (json.RawMessage, error) {
	u, err := cloneUUIDOf(jobID); if err != nil { return nil, ErrNotFound }
	result := checkpoint
	err = s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if _, err := q.GetCloneJobForUpdate(txctx, u); err != nil { return err }
		tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
		var previous json.RawMessage
		if err := tx.QueryRow(txctx, `SELECT checkpoint FROM clone_steps WHERE job_id=$1 AND step='images' FOR UPDATE`, u).Scan(&previous); err != nil { return err }
		checkpoint, err = preserveReviewOrigin(previous, checkpoint); if err != nil { return err }
		var m images.Manifest
		if err := json.Unmarshal(checkpoint, &m); err != nil { return err }
		count, err := overlayImageIntents(txctx, tx, u, &m); if err != nil { return err }
		override, err := overlayImageOverride(txctx, tx, u, &m, true); if err != nil { return err }
		if count == 0 && !override { return nil }
		result, err = json.Marshal(m); if err != nil { return err }
		if err = s.WriteStepCheckpoint(txctx, jobID, domain.CloneImages, fence, attempt, result); err != nil { return err }
		if count > 0 {
			if _, err = tx.Exec(txctx, `DELETE FROM clone_image_review_intents WHERE job_id=$1`, u); err != nil { return err }
		}
		if override {
			_, err = tx.Exec(txctx, `DELETE FROM clone_image_override_intents WHERE job_id=$1`, u)
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) { return nil, pipeline.ErrFence }
	return result, err
}
