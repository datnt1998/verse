package postgres

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

// CloneStore là kho của pipeline clone: mọi ghi đi trong transaction, mọi tx
// ghi xong đều refresh stage_group/active_step/version của job (read model
// Bàn dựng, cùng tx — không trigger như bản SQLite). Khi ctx mang một
// pgx.Tx (ví dụ bên trong media.Publication), store ghi trên tx đó nên
// publish artifact + fenced step commit là một bước nguyên tử.
type CloneStore struct {
	pool *pgxpool.Pool
}

func NewCloneStore(pool *pgxpool.Pool) *CloneStore { return &CloneStore{pool: pool} }

// ErrNotFound: job/step không tồn tại.
var ErrNotFound = pipeline.ErrNotFound

// ErrFenceLost: attempt không còn giữ fence — ghi bị từ chối.
var ErrFenceLost = pipeline.ErrFence

// q trả Queries gắn tx từ ctx khi có (media.Publication tx), không thì pool.
func (s *CloneStore) q(ctx context.Context) *gen.Queries {
	if tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return gen.New(tx)
	}
	return gen.New(s.pool)
}

func (s *CloneStore) txCtx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, mediaTxKey{}, tx)
}

// inTx chạy fn trong một pgx transaction: nếu ctx đã mang tx thì chạy thẳng,
// không thì mở tx mới và commit khi fn nil.
func (s *CloneStore) inTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(mediaTxKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.txCtx(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

/* ------------------------------ converters ------------------------------ */

func cloneUUIDOf(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, nil
	}
	err := u.Scan(s)
	return u, err
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}

func tstz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func nowTz() pgtype.Timestamptz { return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true} }

func tptr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}

func pstr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func i8ptr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func iptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func jobFromGen(r gen.CloneJob) domain.CloneJob {
	var src domain.CloneSourceSnapshot
	_ = json.Unmarshal(r.SourceSnapshot, &src)
	var active *domain.CloneStep
	if r.ActiveStep.Valid {
		s := domain.CloneStep(r.ActiveStep.String)
		active = &s
	}
	var autoFix *domain.AutoFix
	if len(r.AutoFix) > 0 {
		var f domain.AutoFix
		if json.Unmarshal(r.AutoFix, &f) == nil && f.At != "" {
			autoFix = &f
		}
	}
	return domain.CloneJob{
		ID:                uuidStr(r.ID),
		ChannelID:         uuidStr(r.ChannelID),
		SourceKey:         r.SourceKey,
		Source:            src,
		TemplateID:        uuidStr(r.TemplateID),
		TemplateSnapshot:  r.TemplateSnapshot,
		ModelRefs:         r.ModelRefs,
		NoImages:          r.NoImages,
		Generation:        int(r.Generation),
		SupersededAt:      tptr(r.SupersededAt),
		PausedAt:          tptr(r.PausedAt),
		CancelRequestedAt: tptr(r.CancelRequestedAt),
		Priority:          domain.ClonePriority(r.Priority),
		Origin:            domain.CloneOrigin(r.Origin),
		Status:            domain.CloneStatus(r.Status),
		StageGroup:        r.StageGroup,
		ActiveStep:        active,
		Fence:             r.Fence,
		Version:           r.Version,
		PostedAt:          tptr(r.PostedAt),
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
		AutoFix:           autoFix,
	}
}

func stepFromGen(r gen.CloneStep) domain.CloneStepRecord {
	var out *domain.CloneStepOutput
	if len(r.OutputManifest) > 0 {
		var o domain.CloneStepOutput
		if json.Unmarshal(r.OutputManifest, &o) == nil {
			out = &o
		}
	}
	var cerr *domain.CloneError
	if len(r.Error) > 0 {
		var e domain.CloneError
		if json.Unmarshal(r.Error, &e) == nil && e.Code != "" {
			cerr = &e
		}
	}
	var pg *int
	if r.ProcessGroup.Valid {
		v := int(r.ProcessGroup.Int32)
		pg = &v
	}
	return domain.CloneStepRecord{
		JobID:   uuidStr(r.JobID),
		Step:    domain.CloneStep(r.Step),
		Status:  domain.CloneStatus(r.Status),
		Attempt: int(r.Attempt),
		Progress: domain.CloneProgress{
			UnitsDone:  r.ProgressDone,
			UnitsTotal: iptr(r.ProgressTotal),
			Phase:      r.ProgressPhase,
		},
		Checkpoint:   r.Checkpoint,
		Output:       out,
		Error:        cerr,
		Fence:        r.Fence,
		ProcessGroup: pg,
		StartedAt:    tptr(r.StartedAt),
		FinishedAt:   tptr(r.FinishedAt),
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

func artifactFromGen(r gen.CloneArtifact) domain.CloneArtifact {
	return domain.CloneArtifact{
		ID: r.ID, Path: r.Path, SHA256: r.Sha256, Bytes: r.Bytes,
		Step: domain.CloneStep(r.Step), ContentType: r.ContentType, Filename: r.Filename,
	}
}

/* ------------------------------ reads ------------------------------ */

func (s *CloneStore) GetJob(ctx context.Context, jobID string) (*domain.CloneJob, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	r, err := s.q(ctx).GetCloneJob(ctx, u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j := jobFromGen(r)
	return &j, nil
}

func (s *CloneStore) GetJobBySourceKey(ctx context.Context, sourceKey string) (*domain.CloneJob, error) {
	r, err := s.q(ctx).GetCloneJobBySourceKey(ctx, sourceKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j := jobFromGen(r)
	return &j, nil
}

func (s *CloneStore) ListSteps(ctx context.Context, jobID string) ([]domain.CloneStepRecord, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := s.q(ctx).ListCloneSteps(ctx, u)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneStepRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, stepFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) GetStep(ctx context.Context, jobID string, step domain.CloneStep) (*domain.CloneStepRecord, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	r, err := s.q(ctx).GetCloneStep(ctx, gen.GetCloneStepParams{JobID: u, Step: string(step)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rec := stepFromGen(r)
	return &rec, nil
}

func (s *CloneStore) ListBoardJobs(ctx context.Context, channelID *string, limit int) ([]domain.CloneJob, error) {
	var ch pgtype.UUID
	if channelID != nil && *channelID != "" && *channelID != "all" {
		u, err := cloneUUIDOf(*channelID)
		if err != nil {
			return nil, ErrNotFound
		}
		ch = u
	}
	rows, err := s.q(ctx).ListBoardJobs(ctx, gen.ListBoardJobsParams{Column1: ch, Column2: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneJob, 0, len(rows))
	for _, r := range rows {
		out = append(out, jobFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) ListClaimableJobs(ctx context.Context, limit int) ([]domain.CloneJob, error) {
	rows, err := s.q(ctx).ListClaimableCloneJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneJob, 0, len(rows))
	for _, r := range rows {
		out = append(out, jobFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) ListInterruptedJobs(ctx context.Context, limit int) ([]domain.CloneJob, error) {
	rows, err := s.q(ctx).ListInterruptedCloneJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneJob, 0, len(rows))
	for _, r := range rows {
		out = append(out, jobFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) ListFinishedJobs(ctx context.Context, limit int) ([]domain.CloneJob, error) {
	rows, err := s.q(ctx).ListFinishedCloneJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneJob, 0, len(rows))
	for _, r := range rows {
		out = append(out, jobFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) ListArtifacts(ctx context.Context, jobID string) ([]domain.CloneArtifact, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := s.q(ctx).ListCloneArtifacts(ctx, u)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CloneArtifact, 0, len(rows))
	for _, r := range rows {
		out = append(out, artifactFromGen(r))
	}
	return out, nil
}

func (s *CloneStore) GetArtifact(ctx context.Context, jobID, artifactID string) (*domain.CloneArtifact, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	r, err := s.q(ctx).GetCloneArtifact(ctx, gen.GetCloneArtifactParams{JobID: u, ID: artifactID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a := artifactFromGen(r)
	return &a, nil
}

func (s *CloneStore) CountBoardGroups(ctx context.Context) (map[[2]string]int64, error) {
	rows, err := s.q(ctx).CountBoardGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := map[[2]string]int64{}
	for _, r := range rows {
		out[[2]string{uuidStr(r.ChannelID), r.StageGroup}] = r.N
	}
	return out, nil
}

func (s *CloneStore) CountBoardGroupsToday(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := s.q(ctx).CountBoardGroupsToday(ctx, tstz(&since))
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[uuidStr(r.ChannelID)] = r.N
	}
	return out, nil
}

func (s *CloneStore) CountChannelJobsToday(ctx context.Context, channelID string, since time.Time) (int64, error) {
	u, err := cloneUUIDOf(channelID)
	if err != nil {
		return 0, ErrNotFound
	}
	return s.q(ctx).CountChannelJobsToday(ctx, gen.CountChannelJobsTodayParams{ChannelID: u, Column2: tstz(&since)})
}

/* ------------------------------ job lifecycle ------------------------------ */

type StartJobInput = pipeline.StartJobInput
type StartJobResult = pipeline.StartJobResult

// StartJob trả job active của sourceKey hoặc tạo mới một generation với mọi
// step pending. Chế độ Replaces supersede generation cũ trong cùng tx.
func (s *CloneStore) StartJob(ctx context.Context, in StartJobInput) (*StartJobResult, error) {
	res := &StartJobResult{}
	err := s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		now := nowTz()
		if in.QueueItemID != "" {
			// Queue edits and admission take the channel lock before touching
			// the item. A stale scheduler snapshot must not create/adopt a job.
			channelID, err := cloneUUIDOf(in.ChannelID)
			if err != nil {
				return err
			}
			if _, err := q.LockQueueOwner(txctx, channelID); err != nil {
				return err
			}
			queueID, err := cloneUUIDOf(in.QueueItemID)
			if err != nil {
				return err
			}
			item, err := q.LockQueueItemForAdmission(txctx, queueID)
			if errors.Is(err, pgx.ErrNoRows) {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ không còn tồn tại"}
			}
			if err != nil {
				return err
			}
			if item.State != "queued" || item.RemovedAt.Valid {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ đã bị bỏ hoặc đã chạy"}
			}
			if in.QueueOverrideRevision != 0 && item.OverrideRevision != in.QueueOverrideRevision {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ đã thay đổi, vui lòng tải lại"}
			}
			if item.RestartOf.Valid {
				in.Replaces = uuidStr(item.RestartOf)
			}
		}
		if in.Replaces != "" {
			id, err := cloneUUIDOf(in.Replaces)
			if err != nil {
				return pipeline.ErrNotFound
			}
			old, err := q.GetCloneJobForUpdate(txctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return pipeline.ErrNotFound
			}
			if err != nil {
				return err
			}
			if old.SupersededAt.Valid || old.SourceKey != in.SourceKey || old.Status == "pending" || old.Status == "running" || (old.LeaseExpiresAt.Valid && old.LeaseExpiresAt.Time.After(time.Now())) {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video vẫn đang làm hoặc đã có bản làm lại"}
			}
		}
		existing, err := q.GetCloneJobBySourceKey(txctx, in.SourceKey)
		var haveExisting bool
		switch {
		case err == nil:
			haveExisting = true
		case errors.Is(err, pgx.ErrNoRows):
			haveExisting = false
		default:
			return err
		}
		if haveExisting && in.Replaces == "" {
			// Reuse: job active còn đó — trả nó, mục hàng chờ (nếu có) adopt vào.
			if in.QueueItemID != "" {
				qu, _ := cloneUUIDOf(in.QueueItemID)
				jb, _ := cloneUUIDOf(uuidStr(existing.ID))
				n, err := q.MarkQueueItemStarted(txctx, gen.MarkQueueItemStartedParams{ID: qu, JobID: jb, StartedAt: now})
				if err != nil {
					return err
				}
				if n == 0 {
					return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ đã được xử lý"}
				}
			}
			j := jobFromGen(existing)
			res.Job, res.Created = &j, false
			return nil
		}
		if in.Replaces != "" && (!haveExisting || in.Replaces != uuidStr(existing.ID)) {
			return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video đã có bản làm lại"}
		}
		gen_ := int32(1)
		if haveExisting {
			live := existing.LeaseExpiresAt.Valid && existing.LeaseExpiresAt.Time.After(time.Now())
			if live || existing.Status == "running" || existing.Status == "pending" {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video vẫn đang làm"}
			}
			gen_ = existing.Generation + 1
			if err := q.SupersedeCloneJob(txctx, gen.SupersedeCloneJobParams{ID: existing.ID, Column2: now}); err != nil {
				return err
			}
			if err := q.CancelOpenSteps(txctx, gen.CancelOpenStepsParams{JobID: existing.ID, Column2: now}); err != nil {
				return err
			}
		}
		id, _ := cloneUUIDOf(newUUID())
		ch, _ := cloneUUIDOf(in.ChannelID)
		tpl, _ := cloneUUIDOf(in.TemplateID)
		srcJSON, _ := json.Marshal(in.Source)
		row, err := q.InsertCloneJob(txctx, gen.InsertCloneJobParams{
			ID: id, ChannelID: ch, SourceKey: in.SourceKey, SourceSnapshot: srcJSON,
			TemplateID: tpl, TemplateSnapshot: in.TemplateSnapshot, ModelRefs: in.ModelRefs,
			NoImages: in.NoImages, Generation: gen_,
			Priority: string(orDef(string(in.Priority), "pick")),
			Origin:   string(orDef(string(in.Origin), "manual")),
			Column12: now,
		})
		if err != nil {
			return err
		}
		for _, step := range domain.CloneSteps {
			if err := q.InsertCloneStep(txctx, gen.InsertCloneStepParams{
				JobID: row.ID, Step: string(step), Column3: now}); err != nil {
				return err
			}
		}
		if len(in.ImageCheckpoint) > 0 {
			if err := q.SetInitialImageOverrides(txctx, gen.SetInitialImageOverridesParams{JobID: row.ID, Column2: in.ImageCheckpoint}); err != nil {
				return err
			}
		}
		if in.QueueItemID != "" {
			qu, _ := cloneUUIDOf(in.QueueItemID)
			n, err := q.MarkQueueItemStarted(txctx, gen.MarkQueueItemStartedParams{ID: qu, JobID: row.ID, StartedAt: now})
			if err != nil {
				return err
			}
			if n == 0 {
				return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ đã được xử lý"}
			}
		}
		if ch.Valid {
			tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
			if _, err := tx.Exec(txctx, "UPDATE owner_channels SET template_cursor=template_cursor+1 WHERE id=$1", ch); err != nil {
				return err
			}
		}
		if err := s.refreshDerived(txctx, row.ID, nil); err != nil {
			return err
		}
		j := jobFromGen(row)
		res.Job, res.Created = &j, true
		return nil
	})
	return res, err
}

func orDef(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

/* ------------------------------ derived read-model ------------------------------ */

// refreshDerived cập nhật stage_group + active_step + version trong cùng tx:
// read model của Bàn dựng không bao giờ đi lệch với write.
func (s *CloneStore) refreshDerived(ctx context.Context, jobID pgtype.UUID, activeStep *domain.CloneStep) error {
	q := s.q(ctx)
	row, err := q.GetCloneJobForUpdate(ctx, jobID)
	if err != nil {
		return err
	}
	job := jobFromGen(row)
	steps, err := s.ListSteps(ctx, uuidStr(jobID))
	if err != nil {
		return err
	}
	group := domain.StageGroupOf(job, steps)
	if err := recordStageNotification(ctx, job, group); err != nil {
		return err
	}
	return q.SetJobDerived(ctx, gen.SetJobDerivedParams{
		ID: jobID, Column2: nowTz(), StageGroup: group,
		ActiveStep: pstr((*string)(activeStep)),
	})
}

/* ------------------------------ lease ------------------------------ */

func (s *CloneStore) ClaimJobLease(ctx context.Context, jobID, owner string, ttl time.Duration) (*domain.CloneJob, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	exp := time.Now().UTC().Add(ttl)
	r, err := s.q(ctx).ClaimJobLease(ctx, gen.ClaimJobLeaseParams{
		ID: u, LeaseOwner: pstr(&owner), Column3: tstz(&exp), Column4: nowTz()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // lease giữ bởi người khác hoặc job đã thay thế
	}
	if err != nil {
		return nil, err
	}
	j := jobFromGen(r)
	return &j, nil
}

func (s *CloneStore) HeartbeatJobLease(ctx context.Context, jobID, owner string, ttl time.Duration) (bool, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return false, ErrNotFound
	}
	exp := time.Now().UTC().Add(ttl)
	n, err := s.q(ctx).HeartbeatJobLease(ctx, gen.HeartbeatJobLeaseParams{
		ID: u, LeaseOwner: pstr(&owner), Column3: tstz(&exp), Column4: nowTz()})
	return n == 1, err
}

func (s *CloneStore) ReleaseJobLease(ctx context.Context, jobID, owner string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.q(ctx).ReleaseJobLease(ctx, gen.ReleaseJobLeaseParams{
		ID: u, LeaseOwner: pstr(&owner), Column3: nowTz()})
}

/* ------------------------------ step claim & fenced writes ------------------------------ */

// ClaimedStep (định nghĩa ở pipeline) = step vừa claim + fence + upstream.
type ClaimedStep = pipeline.ClaimedStep

// ClaimNextStep: một transaction — kiểm lease là của worker, job không
// huỷ/supersede/paused, rồi CAS step đầu tiên trong chuỗi chưa succeeded:
// pending/stale luôn, waiting khi retryAt đã qua (awaiting_review bị loại
// tự nhiên vì không retryAt).
func (s *CloneStore) ClaimNextStep(ctx context.Context, jobID, workerID string) (*ClaimedStep, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	var out *ClaimedStep
	err = s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		row, err := q.GetCloneJobForUpdate(txctx, u)
		if err != nil {
			return err
		}
		job := jobFromGen(row)
		if job.CancelRequestedAt != nil || job.SupersededAt != nil || job.PausedAt != nil {
			return nil
		}
		if !row.LeaseOwner.Valid || row.LeaseOwner.String != workerID {
			return nil
		}
		steps, err := q.ListCloneSteps(txctx, u)
		if err != nil {
			return err
		}
		// Upstream từ các step đã xong.
		sort.Slice(steps, func(i, j int) bool {
			return domain.IndexStep(domain.CloneStep(steps[i].Step)) < domain.IndexStep(domain.CloneStep(steps[j].Step))
		})
		upstream := map[domain.CloneStep]domain.CloneStepOutput{}
		var candidate *gen.CloneStep
		for i := range steps {
			rec := &steps[i]
			if rec.Status == "succeeded" {
				var o domain.CloneStepOutput
				if json.Unmarshal(rec.OutputManifest, &o) == nil {
					upstream[domain.CloneStep(rec.Step)] = o
				}
				continue
			}
			candidate = rec
			break
		}
		if candidate == nil {
			return nil // chainDone hoặc không còn gì claim
		}
		fenceRow, err := q.BumpJobFence(txctx, u)
		if err != nil {
			return err
		}
		claimed, err := q.ClaimCloneStep(txctx, gen.ClaimCloneStepParams{
			JobID: u, Step: candidate.Step, Fence: fenceRow, Column4: nowTz()})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // điều kiện claim không còn đúng (retryAt chưa qua…)
			}
			return err
		}
		if candidate.Status == "waiting" {
			if err := recordStepAutoRetry(txctx, u, candidate.Step); err != nil {
				return err
			}
		}
		stepName := domain.CloneStep(candidate.Step)
		if err := s.refreshDerived(txctx, u, &stepName); err != nil {
			return err
		}
		_ = q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "running", Column3: nowTz()})
		out = &ClaimedStep{Step: stepFromGen(claimed), Fence: fenceRow, Upstream: upstream}
		return nil
	})
	return out, err
}

// requireRows: execrows = 0 → fence mất.
func fenceGuard(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrFenceLost
	}
	return nil
}

func (s *CloneStore) WriteStepProgress(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, p domain.CloneProgress) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	n, err := s.q(ctx).WriteStepProgress(ctx, gen.WriteStepProgressParams{
		JobID: u, Step: string(step), Column3: fence,
		ProgressDone: p.UnitsDone, ProgressTotal: i8ptr(p.UnitsTotal),
		ProgressPhase: p.Phase, Column7: nowTz(), Column8: int32(attempt)})
	return fenceGuard(n, err)
}

func (s *CloneStore) WriteStepCheckpoint(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, checkpoint json.RawMessage) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	if step == domain.CloneImages {
		return s.inTx(ctx, func(txctx context.Context) error {
			q := s.q(txctx)
			if _, err := q.GetCloneJobForUpdate(txctx, u); err != nil {
				return err
			}
			tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
			var previous json.RawMessage
			if err := tx.QueryRow(txctx, `SELECT checkpoint FROM clone_steps WHERE job_id=$1 AND step='images' FOR UPDATE`, u).Scan(&previous); err != nil {
				return err
			}
			var pendingOverride bool
			if err := tx.QueryRow(txctx, `SELECT EXISTS (SELECT 1 FROM clone_image_override_intents WHERE job_id=$1)`, u).Scan(&pendingOverride); err != nil {
				return err
			}
			if pendingOverride && startsImageFlight(previous, checkpoint) {
				return pipeline.ErrImageOverridePending
			}
			if pendingOverride {
				checkpoint, err = clearReviewOrigin(checkpoint)
			} else {
				checkpoint, err = preserveReviewOrigin(previous, checkpoint)
			}
			if err != nil {
				return err
			}
			n, err := q.WriteStepCheckpoint(txctx, gen.WriteStepCheckpointParams{
				JobID: u, Step: string(step), Column3: fence,
				Column4: checkpoint, Column5: nowTz(), Column6: int32(attempt)})
			if err := fenceGuard(n, err); err != nil {
				return err
			}
			if err := recordImageAutoActions(txctx, u, previous, checkpoint); err != nil {
				return err
			}
			return recordImageApproval(txctx, tx, u, previous, checkpoint)
		})
	}
	n, err := s.q(ctx).WriteStepCheckpoint(ctx, gen.WriteStepCheckpointParams{
		JobID: u, Step: string(step), Column3: fence,
		Column4: checkpoint, Column5: nowTz(), Column6: int32(attempt)})
	return fenceGuard(n, err)
}

func (s *CloneStore) WriteStepProcessGroup(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, pgid int) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	n, err := s.q(ctx).WriteStepProcessGroup(ctx, gen.WriteStepProcessGroupParams{
		JobID: u, Step: string(step), Column3: fence,
		ProcessGroup: pgtype.Int4{Int32: int32(pgid), Valid: true}, Column5: nowTz(), Column6: int32(attempt)})
	return fenceGuard(n, err)
}

func (s *CloneStore) WriteStepFingerprint(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, fingerprint string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	n, err := s.q(ctx).WriteStepFingerprint(ctx, gen.WriteStepFingerprintParams{
		JobID: u, Step: string(step), Column3: fence,
		InputFingerprint: fingerprint, Column5: nowTz(), Column6: int32(attempt)})
	return fenceGuard(n, err)
}

// ApplyStepSuccess: ghi output manifest + artifacts + step→succeeded, cùng tx.
// Được gọi cả từ commit bình thường lẫn bên trong media.Publish commit.
func (s *CloneStore) ApplyStepSuccess(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, output domain.CloneStepOutput) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	manifest, _ := json.Marshal(output)
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		var pending bool
		var checkpoint json.RawMessage
		if step == domain.CloneImages {
			if _, err := q.GetCloneJobForUpdate(txctx, u); err != nil {
				return err
			}
			tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
			if err := tx.QueryRow(txctx, `SELECT checkpoint, EXISTS (SELECT 1 FROM clone_image_review_intents WHERE job_id=$1 UNION ALL SELECT 1 FROM clone_image_override_intents WHERE job_id=$1) FROM clone_steps WHERE job_id=$1 AND step='images' FOR UPDATE`, u).Scan(&checkpoint, &pending); err != nil {
				return err
			}
		}
		for _, a := range output.Artifacts {
			if _, err := q.InsertCloneArtifact(txctx, gen.InsertCloneArtifactParams{
				ID: a.ID, JobID: u, Step: string(step), Path: a.Path,
				Sha256: a.SHA256, Bytes: a.Bytes, ContentType: a.ContentType,
				Filename: a.Filename}); err != nil {
				return err
			}
		}
		n, err := q.ApplyStepSuccess(txctx, gen.ApplyStepSuccessParams{
			JobID: u, Step: string(step), Column3: fence,
			Column4: manifest, Column5: nowTz(), Column6: int32(attempt)})
		if err := fenceGuard(n, err); err != nil {
			return err
		}
		if pending {
			tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
			if _, err := tx.Exec(txctx, `UPDATE clone_steps SET status='pending',checkpoint=$2::jsonb,input_fingerprint='',finished_at=NULL WHERE job_id=$1 AND step='images'`, u, checkpoint); err != nil {
				return err
			}
			if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "pending", Column3: nowTz()}); err != nil {
				return err
			}
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

// ApplyStepWaiting: publish được phần việc (nếu có) rồi đỗ step — artifact
// insert + output manifest + error + status waiting, cùng fenced tx.
func (s *CloneStore) ApplyStepWaiting(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, output *domain.CloneStepOutput, cerr *domain.CloneError) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	var manifest []byte
	if output != nil {
		manifest, _ = json.Marshal(output)
	}
	errJSON, _ := json.Marshal(cerr)
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if output != nil {
			for _, artifact := range output.Artifacts {
				if err := s.insertArtifact(txctx, u, step, artifact); err != nil {
					return err
				}
			}
		}
		n, err := q.ApplyStepWaiting(txctx, gen.ApplyStepWaitingParams{
			JobID: u, Step: string(step), Column3: fence,
			Column4: manifest, Column5: nowTz(), Column6: int32(attempt),
			Column7: errJSON})
		if err := fenceGuard(n, err); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) insertArtifact(ctx context.Context, jobID pgtype.UUID, step domain.CloneStep, a domain.CloneArtifact) error {
	_, err := s.q(ctx).InsertCloneArtifact(ctx, gen.InsertCloneArtifactParams{
		ID: a.ID, JobID: jobID, Step: string(step), Path: a.Path,
		Sha256: a.SHA256, Bytes: a.Bytes, ContentType: a.ContentType, Filename: a.Filename})
	return err
}

// ReportStepOutcome: failed (không retry được) hoặc waiting (retryable).
func (s *CloneStore) ReportStepFailure(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int, cerr *domain.CloneError) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	status := "failed"
	if cerr != nil && domain.CloneErrRetryable(cerr.Code) {
		status = "waiting"
	}
	errJSON, _ := json.Marshal(cerr)
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		n, err := q.ReportStepFailure(txctx, gen.ReportStepFailureParams{
			JobID: u, Step: string(step), Column3: fence, Status: status,
			Column5: nowTz(), Column6: int32(attempt), Column7: errJSON})
		if err := fenceGuard(n, err); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) ReportStepCancelled(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		n, err := q.ReportStepCancelled(txctx, gen.ReportStepCancelledParams{
			JobID: u, Step: string(step), Column3: fence,
			Column4: nowTz(), Column5: int32(attempt)})
		if err := fenceGuard(n, err); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

// ParkStepPaused: step về pending giữ checkpoint — điểm vào của Tạm dừng.
func (s *CloneStore) ParkStepPaused(ctx context.Context, jobID string, step domain.CloneStep,
	fence int64, attempt int) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		n, err := q.ParkStepPaused(txctx, gen.ParkStepPausedParams{
			JobID: u, Step: string(step), Column3: fence,
			Column4: nowTz(), Column5: int32(attempt)})
		if err := fenceGuard(n, err); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

/* ------------------------------ job state ops ------------------------------ */

func (s *CloneStore) SetJobStatus(ctx context.Context, jobID string, status domain.CloneStatus) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{
			ID: u, Status: string(status), Column3: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) SetPaused(ctx context.Context, jobID string, paused bool) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	var at *time.Time
	if paused {
		now := time.Now().UTC()
		at = &now
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if err := q.SetJobPaused(txctx, gen.SetJobPausedParams{ID: u, Column2: tstz(at)}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) RequestCancel(ctx context.Context, jobID string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if err := q.RequestJobCancel(txctx, gen.RequestJobCancelParams{ID: u, Column2: nowTz()}); err != nil {
			return err
		}
		// Pending work không bao giờ chạy được nữa.
		if err := q.CancelOpenSteps(txctx, gen.CancelOpenStepsParams{JobID: u, Column2: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

// ResumeJob: failed/waiting(non-gate) steps → pending; job → pending; nếu job
// cancelled (chưa supersede, lease chết) thì step cancelled mở lại pending và
// cancel request gỡ bỏ.
func (s *CloneStore) ResumeJob(ctx context.Context, jobID string) (*domain.CloneJob, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	var out *domain.CloneJob
	err = s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		row, err := q.GetCloneJobForUpdate(txctx, u)
		if err != nil {
			return err
		}
		job := jobFromGen(row)
		leaseLive := row.LeaseExpiresAt.Valid && row.LeaseExpiresAt.Time.After(time.Now())
		if job.Status == domain.CloneCancelled && job.SupersededAt == nil && !leaseLive {
			if _, err := q.ResumeCancelledSteps(txctx, gen.ResumeCancelledStepsParams{JobID: u, Column2: nowTz()}); err != nil {
				return err
			}
			// Gỡ cancel request: cột không có param riêng nên dùng SQL trực tiếp.
			if tx, ok := txctx.Value(mediaTxKey{}).(pgx.Tx); ok {
				if _, err := tx.Exec(txctx,
					`UPDATE clone_jobs SET cancel_requested_at = NULL WHERE id = $1`, u); err != nil {
					return err
				}
			}
		}
		if job.Status == domain.CloneFailed || job.Status == domain.CloneWaiting ||
			job.Status == domain.ClonePending || job.Status == domain.CloneCancelled || (job.Status == domain.CloneRunning && !leaseLive) {
			if _, err := q.ResumeSteps(txctx, gen.ResumeStepsParams{JobID: u, Column2: nowTz()}); err != nil {
				return err
			}
			if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "pending", Column3: nowTz()}); err != nil {
				return err
			}
		}
		// Tạm dừng: chạy tiếp gỡ paused_at.
		if job.PausedAt != nil {
			if err := q.SetJobPaused(txctx, gen.SetJobPausedParams{ID: u, Column2: pgtype.Timestamptz{}}); err != nil {
				return err
			}
		}
		if err := s.refreshDerived(txctx, u, nil); err != nil {
			return err
		}
		j, err := s.GetJob(txctx, jobID)
		if err != nil {
			return err
		}
		out = j
		return nil
	})
	return out, err
}

// InvalidateSteps: đánh stale/pending step + dependents (sửa job).
func (s *CloneStore) InvalidateSteps(ctx context.Context, jobID string, steps []domain.CloneStep) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	names := make([]string, 0, len(steps))
	for _, st := range steps {
		names = append(names, string(st))
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if _, err := q.InvalidateStepRange(txctx, gen.InvalidateStepRangeParams{
			JobID: u, Column2: names, Column3: nowTz()}); err != nil {
			return err
		}
		if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "pending", Column3: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) MarkStepStale(ctx context.Context, jobID string, step domain.CloneStep, fingerprint string) (bool, error) {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return false, ErrNotFound
	}
	var marked bool
	err = s.inTx(ctx, func(txctx context.Context) error {
		n, err := s.q(txctx).MarkStepStale(txctx, gen.MarkStepStaleParams{
			JobID: u, Step: string(step), InputFingerprint: fingerprint, Column4: nowTz()})
		if err != nil {
			return err
		}
		marked = n == 1
		if marked {
			return s.refreshDerived(txctx, u, nil)
		}
		return nil
	})
	return marked, err
}

func (s *CloneStore) RequeueParkedStep(ctx context.Context, jobID string, step domain.CloneStep) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if _, err := q.RequeueParkedStep(txctx, gen.RequeueParkedStepParams{
			JobID: u, Step: string(step), Column3: nowTz()}); err != nil {
			return err
		}
		if err := q.SetJobStatus(txctx, gen.SetJobStatusParams{ID: u, Status: "pending", Column3: nowTz()}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) MarkPosted(ctx context.Context, jobID string, posted bool) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	var at *time.Time
	if posted {
		now := time.Now().UTC()
		at = &now
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		q := s.q(txctx)
		if err := q.MarkJobPosted(txctx, gen.MarkJobPostedParams{ID: u, Column2: tstz(at)}); err != nil {
			return err
		}
		return s.refreshDerived(txctx, u, nil)
	})
}

func (s *CloneStore) DeleteJob(ctx context.Context, jobID string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(tx context.Context) error {
		if err := s.deleteCutouts(tx, jobID); err != nil {
			return err
		}
		return s.q(tx).DeleteCloneJob(tx, u)
	})
}

func (s *CloneStore) RemoveArtifacts(ctx context.Context, jobID string, ids []string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		return s.q(txctx).DeleteCloneArtifacts(txctx, gen.DeleteCloneArtifactsParams{JobID: u, Column2: ids})
	})
}

func (s *CloneStore) RemoveArtifactsByPath(ctx context.Context, jobID string, paths []string) error {
	u, err := cloneUUIDOf(jobID)
	if err != nil {
		return ErrNotFound
	}
	return s.inTx(ctx, func(txctx context.Context) error {
		return s.q(txctx).DeleteCloneArtifactsByPath(txctx, gen.DeleteCloneArtifactsByPathParams{JobID: u, Column2: paths})
	})
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
