package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

var _ jobedit.Store = (*CloneStore)(nil)

// Read takes one coherent owner-edit snapshot. The original step checkpoint is
// retained even when the service later hydrates the published manifest.
func (s *CloneStore) Read(ctx context.Context, id string, step domain.CloneStep) (jobedit.Snapshot, error) {
	var snapshot jobedit.Snapshot
	u, err := cloneUUIDOf(id)
	if err != nil || !u.Valid || !domain.IsCloneStep(string(step)) {
		return snapshot, ErrNotFound
	}
	err = s.inTx(ctx, func(txctx context.Context) error {
		row, err := s.q(txctx).GetCloneJobForUpdate(txctx, u)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		snapshot.Job = jobFromGen(row)
		snapshot.Steps, err = s.ListSteps(txctx, id)
		if err != nil {
			return err
		}
		found := false
		for _, record := range snapshot.Steps {
			if record.Step == step {
				snapshot.Step = record
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}
		if step == domain.CloneImages && len(snapshot.Step.Checkpoint) != 0 {
			var manifest images.Manifest
			if err := json.Unmarshal(snapshot.Step.Checkpoint, &manifest); err != nil {
				return err
			}
			tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
			count, err := overlayImageIntents(txctx, tx, u, &manifest)
			if err != nil {
				return err
			}
			override, err := overlayImageOverride(txctx, tx, u, &manifest, false)
			if err != nil {
				return err
			}
			if count > 0 || override {
				snapshot.Manifest, err = json.Marshal(manifest)
				if err != nil {
					return err
				}
			}
		}
		snapshot.Artifacts, err = s.ListArtifacts(txctx, id)
		return err
	})
	return snapshot, err
}
func relativeCloneArtifact(path string) string {
	parts := strings.SplitN(path, "/", 4)
	if len(parts) == 4 && parts[0] == "clone" {
		return parts[3]
	}
	return path
}

// Commit serializes against worker admission (job lock) and active attempts
// (step locks). The checkpoint CAS, publication metadata and invalidation all
// use the media publication transaction supplied through ctx when present.
func (s *CloneStore) Commit(ctx context.Context, before jobedit.Snapshot, change jobedit.Change) error {
	u, err := cloneUUIDOf(before.Job.ID)
	if err != nil || !u.Valid {
		return ErrNotFound
	}
	if !json.Valid(change.Checkpoint) || !domain.IsCloneStep(string(before.Step.Step)) || change.InvalidateFrom != "" && !domain.IsCloneStep(string(change.InvalidateFrom)) {
		return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Thay đổi video không hợp lệ"}
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		row, err := q.GetCloneJobForUpdate(txctx, u)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		job := jobFromGen(row)
		if job.SupersededAt != nil || job.Generation != before.Job.Generation {
			return jobedit.Conflict("Video đã được thay thế bởi lượt làm mới")
		}
		if job.CancelRequestedAt != nil || job.Status == domain.CloneCancelled {
			return jobedit.Conflict("Video đã bị huỷ")
		}
		tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
		rows, err := tx.Query(txctx, `SELECT step FROM clone_steps WHERE job_id=$1 ORDER BY step FOR UPDATE`, u)
		if err != nil {
			return err
		}
		for rows.Next() {
			var step string
			if err := rows.Scan(&step); err != nil {
				rows.Close()
				return err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		records, err := s.ListSteps(txctx, before.Job.ID)
		if err != nil {
			return err
		}
		current := jobedit.Snapshot{Job: job, Steps: records}
		found := false
		for _, record := range records {
			if record.Step == before.Step.Step {
				current.Step = record
				found = true
				break
			}
		}
		if !found {
			return ErrNotFound
		}
		if (change.ImageReview || change.ImageOverride) && before.Step.Step == domain.CloneImages && current.Step.Status == domain.CloneRunning {
			if job.Fence != before.Job.Fence || current.Step.Fence != before.Step.Fence || current.Step.Attempt != before.Step.Attempt || !bytes.Equal(current.Step.Checkpoint, before.Step.Checkpoint) {
				return jobedit.Conflict("Nội dung đã thay đổi — tải lại rồi thử lại")
			}
			for _, dependent := range domain.DependentsOf(domain.CloneImages) {
				for _, record := range records {
					if record.Step == dependent && record.Status == domain.CloneRunning {
						return jobedit.Conflict("Bước phụ thuộc đang chạy — chờ xong rồi thử lại")
					}
				}
			}
			var manifest, expected images.Manifest
			if err := json.Unmarshal(current.Step.Checkpoint, &manifest); err != nil {
				return err
			}
			if _, err := overlayImageIntents(txctx, tx, u, &manifest); err != nil {
				return err
			}
			if _, err := overlayImageOverride(txctx, tx, u, &manifest, false); err != nil {
				return err
			}
			if err := json.Unmarshal(before.Manifest, &expected); err != nil {
				return err
			}
			if manifest.Revision != expected.Revision {
				return jobedit.Conflict("Danh sách ảnh đã thay đổi; tải lại trước khi sửa")
			}
			if change.ImageOverride {
				if err := enqueueImageOverride(txctx, tx, u, expected.Revision, change.Checkpoint); err != nil {
					return err
				}
				if _, err := tx.Exec(txctx, `UPDATE clone_steps SET checkpoint=checkpoint-'reviewOrigin' WHERE job_id=$1 AND step='images'`, u); err != nil {
					return err
				}
			} else if err := enqueueImageIntents(txctx, tx, u, before.Manifest, change.Checkpoint, change.ImageIDs); err != nil {
				return err
			}
			var requested images.Manifest
			if err := json.Unmarshal(change.Checkpoint, &requested); err != nil {
				return err
			}
			if requested.ReviewOrigin && !manifest.ReviewOrigin {
				if _, err := tx.Exec(txctx, `UPDATE clone_steps SET checkpoint=jsonb_set(checkpoint,'{reviewOrigin}','true'::jsonb) WHERE job_id=$1 AND step='images'`, u); err != nil {
					return err
				}
			}
			names := make([]string, 0)
			for _, step := range domain.DependentsOf(domain.CloneImages) {
				names = append(names, string(step))
			}
			if _, err := q.InvalidateStepRange(txctx, gen.InvalidateStepRangeParams{JobID: u, Column2: names, Column3: nowTz()}); err != nil {
				return err
			}
			if _, err := tx.Exec(txctx, `DELETE FROM clone_bulk_review_intents WHERE job_id=$1`, u); err != nil {
				return err
			}
			active := domain.CloneImages
			return s.refreshDerived(txctx, u, &active)
		}
		leaseActive := row.LeaseExpiresAt.Valid && row.LeaseExpiresAt.Time.After(time.Now())
		if (leaseActive && before.Step.Step != domain.CloneMetadata) || jobedit.Busy(current, before.Step.Step) || jobedit.Busy(current, change.InvalidateFrom) {
			return jobedit.Conflict("Video đang xử lý — chờ xong rồi thử lại")
		}
		if (before.Step.Step != domain.CloneMetadata && job.Fence != before.Job.Fence) || current.Step.Fence != before.Step.Fence || current.Step.Attempt != before.Step.Attempt || current.Step.Status != before.Step.Status || !bytes.Equal(current.Step.Checkpoint, before.Step.Checkpoint) {
			return jobedit.Conflict("Nội dung đã thay đổi — tải lại rồi thử lại")
		}
		output := domain.CloneStepOutput{Artifacts: []domain.CloneArtifact{}, Data: map[string]any{}}
		if current.Step.Output != nil {
			output = *current.Step.Output
			output.Artifacts = append([]domain.CloneArtifact(nil), output.Artifacts...)
			output.Data = make(map[string]any, len(current.Step.Output.Data))
			for key, value := range current.Step.Output.Data {
				output.Data[key] = value
			}
		}
		for _, artifact := range change.Artifacts {
			if !strings.HasPrefix(artifact.Path, "clone/"+before.Job.ID+"/") || artifact.Step != before.Step.Step {
				return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Tệp không thuộc video này"}
			}
			if err := s.insertArtifact(txctx, u, before.Step.Step, artifact); err != nil {
				return err
			}
			relative := relativeCloneArtifact(artifact.Path)
			retained := output.Artifacts[:0]
			for _, old := range output.Artifacts {
				if relativeCloneArtifact(old.Path) != relative {
					retained = append(retained, old)
				}
			}
			output.Artifacts = append(retained, artifact)
		}
		if before.Step.Step == domain.CloneBackdrop {
			var manifest any
			if err := json.Unmarshal(change.Checkpoint, &manifest); err != nil {
				return err
			}
			if wrapper, ok := manifest.(map[string]any); ok {
				if nested, exists := wrapper["manifest"]; exists {
					manifest = nested
				}
			}
			output.Data["manifest"] = manifest
		} else {
			output.Data["manifest"] = string(before.Step.Step) + "/manifest.json"
		}
		if before.Step.Step == domain.CloneImages {
			oldManifest := before.Manifest
			if len(oldManifest) == 0 {
				oldManifest = before.Step.Checkpoint
			}
			if err := recordImageApproval(txctx, tx, u, oldManifest, change.Checkpoint); err != nil {
				return err
			}
			// The images service validates the full approval gate before
			// CommitOwnerApproval; the checkpoint written here is that exact
			// manifest under the step CAS, so storage only needs its revision.
			var manifest struct {
				Revision int `json:"revision"`
			}
			if err := json.Unmarshal(change.Checkpoint, &manifest); err != nil {
				return err
			}
			output.Data["revision"] = manifest.Revision
			if change.ApproveExport {
				if _, err := tx.Exec(txctx, `INSERT INTO clone_bulk_review_intents(job_id,image_revision) VALUES($1,$2) ON CONFLICT(job_id) DO UPDATE SET image_revision=EXCLUDED.image_revision,created_at=now()`, u, manifest.Revision); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(txctx, `DELETE FROM clone_bulk_review_intents WHERE job_id=$1`, u); err != nil {
					return err
				}
			}
		} else if before.Step.Step != domain.CloneMetadata {
			if _, err := tx.Exec(txctx, `DELETE FROM clone_bulk_review_intents WHERE job_id=$1`, u); err != nil {
				return err
			}
		}
		raw, err := json.Marshal(output)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(txctx, `UPDATE clone_steps SET checkpoint=$3::jsonb,output_manifest=$4::jsonb,updated_at=now() WHERE job_id=$1 AND step=$2`, u, string(before.Step.Step), change.Checkpoint, raw); err != nil {
			return err
		}
		if before.Step.Step == domain.CloneMetadata {
			if err := s.saveCloneMetadataState(txctx, u, change.Checkpoint); err != nil {
				return err
			}
			if before.Step.Status == domain.CloneFailed || before.Step.Status == domain.CloneWaiting {
				if _, err := tx.Exec(txctx, `UPDATE clone_steps SET status='succeeded',error=NULL,finished_at=now() WHERE job_id=$1 AND step=$2`, u, string(before.Step.Step)); err != nil {
					return err
				}
			}
		}
		// Metadata has no downstream consumers in the pipeline. When a
		// different step owns the lease, preserve its running status and active
		// step; the checkpoint CAS still fences concurrent metadata edits.
		if before.Step.Step == domain.CloneMetadata && leaseActive {
			return s.refreshDerived(txctx, u, job.ActiveStep)
		}
		if change.InvalidateFrom == "" {
			return s.refreshDerived(txctx, u, nil)
		}
		invalidated := []domain.CloneStep{change.InvalidateFrom}
		invalidated = append(invalidated, domain.DependentsOf(change.InvalidateFrom)...)
		names := make([]string, 0, len(invalidated))
		for _, step := range invalidated {
			if step != before.Step.Step || change.Rerun {
				names = append(names, string(step))
			}
		}
		if _, err := q.InvalidateStepRange(txctx, gen.InvalidateStepRangeParams{JobID: u, Column2: names, Column3: nowTz()}); err != nil {
			return err
		}
		if change.Rerun {
			// The retained fingerprint must not make a manual redraw look reusable.
			if _, err := tx.Exec(txctx, `UPDATE clone_steps SET status='pending',input_fingerprint='',error=NULL,finished_at=NULL WHERE job_id=$1 AND step=$2`, u, string(before.Step.Step)); err != nil {
				return err
			}
		}
		if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "pending", Column3: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

// RecordExportRemoved retains the publishing history after the editor unbinds
// the committed file. A newer generation/export must never receive the marker.
func (s *CloneStore) RecordExportRemoved(ctx context.Context, before jobedit.Snapshot) error {
	u, err := cloneUUIDOf(before.Job.ID)
	if err != nil || !u.Valid {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		row, err := s.q(txctx).GetCloneJobForUpdate(txctx, u)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		job := jobFromGen(row)
		if job.SupersededAt != nil || job.Generation != before.Job.Generation || job.Status != domain.CloneSucceeded || job.PostedAt == nil {
			return jobedit.Conflict("Trạng thái video đã thay đổi")
		}
		current, err := s.GetStep(txctx, before.Job.ID, domain.CloneExport)
		if err != nil {
			return err
		}
		if current.Status != domain.CloneSucceeded || current.Output == nil || before.Step.Output == nil || current.Output.Data["exportId"] != before.Step.Output.Data["exportId"] {
			return jobedit.Conflict("Bản xuất của video đã thay đổi")
		}
		current.Output.Data["removedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
		raw, err := json.Marshal(current.Output)
		if err != nil {
			return err
		}
		tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
		if _, err := tx.Exec(txctx, `UPDATE clone_steps SET output_manifest=$2::jsonb,updated_at=now() WHERE job_id=$1 AND step='export'`, u, raw); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}
