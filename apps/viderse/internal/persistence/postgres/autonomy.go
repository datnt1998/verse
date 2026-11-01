package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AutonomyStore giữ Mức tự chủ của kênh và việc máy tự làm theo mức đó.
type AutonomyStore struct {
	pool  *pgxpool.Pool
	clone *CloneStore
}

// NewAutonomyStore dựng store trên pool.
func NewAutonomyStore(pool *pgxpool.Pool) *AutonomyStore {
	return &AutonomyStore{pool: pool, clone: NewCloneStore(pool)}
}

var _ autonomy.Store = (*AutonomyStore)(nil)

func autonomyJSON(a domain.Autonomy) []byte {
	if a.Pick == "" {
		a = domain.DefaultAutonomy()
	}
	raw, _ := json.Marshal(a.Normalize())
	return raw
}

func orPriority(p string) string {
	if p == "" {
		return domain.PrioritySteady
	}
	return p
}

func (s *AutonomyStore) ChannelAutonomy(ctx context.Context, channelID string) (domain.Autonomy, string, bool, error) {
	id, err := uuidArg(channelID)
	if err != nil {
		return domain.Autonomy{}, "", false, nil
	}
	var raw []byte
	var name string
	err = s.pool.QueryRow(ctx, `SELECT autonomy, name FROM owner_channels WHERE id=$1`, id).Scan(&raw, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Autonomy{}, "", false, nil
	}
	if err != nil {
		return domain.Autonomy{}, "", false, err
	}
	return domain.ParseAutonomy(raw), name, true, nil
}

func (s *AutonomyStore) SetChannelAutonomy(ctx context.Context, channelID string, next domain.Autonomy) (domain.Autonomy, bool, error) {
	id, err := uuidArg(channelID)
	if err != nil {
		return domain.Autonomy{}, false, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Autonomy{}, false, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT autonomy FROM owner_channels WHERE id=$1 FOR UPDATE`, id).Scan(&raw); errors.Is(err, pgx.ErrNoRows) {
		return domain.Autonomy{}, false, nil
	} else if err != nil {
		return domain.Autonomy{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE owner_channels SET autonomy=$2::jsonb, updated_at=clock_timestamp() WHERE id=$1`, id, autonomyJSON(next)); err != nil {
		return domain.Autonomy{}, false, err
	}
	return domain.ParseAutonomy(raw), true, tx.Commit(ctx)
}

func (s *AutonomyStore) JobAutonomy(ctx context.Context, jobID string) (domain.Autonomy, bool, error) {
	id, err := cloneUUIDOf(jobID)
	if err != nil || !id.Valid {
		return domain.Autonomy{}, false, nil
	}
	var raw []byte
	err = s.pool.QueryRow(ctx, `SELECT c.autonomy FROM clone_jobs j JOIN owner_channels c ON c.id=j.channel_id WHERE j.id=$1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Autonomy{}, false, nil
	}
	if err != nil {
		return domain.Autonomy{}, false, err
	}
	return domain.ParseAutonomy(raw), true, nil
}

const pendingReviewsSQL = `
SELECT j.id::text, j.channel_id::text, COALESCE(j.source_snapshot->>'title',''), COALESCE(r.rev,0), c.autonomy,
       COALESCE((j.auto_review->>'tries')::int,0)
FROM clone_jobs j
JOIN owner_channels c ON c.id=j.channel_id
JOIN clone_steps i ON i.job_id=j.id AND i.step='images'
CROSS JOIN LATERAL (SELECT NULLIF(i.checkpoint->>'revision','')::int AS rev) r
WHERE j.superseded_at IS NULL AND j.cancel_requested_at IS NULL AND j.paused_at IS NULL
  AND (j.lease_expires_at IS NULL OR j.lease_expires_at <= now())
  AND c.autonomy->'approve'->>'mode' IN ('score','all')
  AND EXISTS (SELECT 1 FROM clone_steps w WHERE w.job_id=j.id AND w.step IN ('images','build') AND w.status='waiting' AND w.error->>'code'=$2)
  AND (j.auto_review IS NULL
       OR (j.auto_review->>'revision')::int IS DISTINCT FROM r.rev
       OR (j.auto_review->>'result'='error' AND COALESCE((j.auto_review->>'tries')::int,0) < $3))
ORDER BY j.updated_at
LIMIT $1`

func (s *AutonomyStore) PendingReviews(ctx context.Context, limit int) ([]autonomy.PendingReview, error) {
	rows, err := s.pool.Query(ctx, pendingReviewsSQL, limit, domain.CloneErrAwaitingReview, autonomy.MaxReviewTries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []autonomy.PendingReview{}
	for rows.Next() {
		var p autonomy.PendingReview
		var raw []byte
		if err := rows.Scan(&p.JobID, &p.ChannelID, &p.Title, &p.ImageRevision, &raw, &p.Tries); err != nil {
			return nil, err
		}
		p.Autonomy = domain.ParseAutonomy(raw)
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordReview ghi kết quả xét điểm; riêng khi đã tự duyệt thì ghi cùng một
// giao dịch Hành động tự làm và thông báo "Tự duyệt theo điểm" (Xem + Reason).
func (s *AutonomyStore) RecordReview(ctx context.Context, o autonomy.ReviewOutcome) error {
	id, err := cloneUUIDOf(o.JobID)
	if err != nil || !id.Valid {
		return ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	mark, _ := json.Marshal(map[string]any{"revision": o.Revision, "result": o.Result, "tries": o.Tries, "summary": o.Summary, "min": o.Autonomy.Approve.Min, "at": time.Now().UTC().Format(time.RFC3339)})
	if _, err := tx.Exec(ctx, `UPDATE clone_jobs SET auto_review=$2::jsonb WHERE id=$1`, id, mark); err != nil {
		return err
	}
	if o.Result == autonomy.ResultApproved {
		if err := recordAutoApprove(ctx, tx, id, o); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func approveReason(o autonomy.ReviewOutcome) insight.Reason {
	if o.Autonomy.Approve.Mode == domain.ApproveAll {
		return insight.Reason{Agent: domain.AgentQA, Text: "Kênh đặt Tự duyệt hết nên ảnh được duyệt mà không xét điểm. Máy vẫn không tự đăng.", Facts: []insight.Fact{
			{Label: "Cách duyệt", Value: "Tự duyệt hết", Source: "Mức tự chủ của kênh"},
			{Label: "Số ảnh", Value: fmt.Sprintf("%d", o.Verdict.Total), Source: "Bước Ảnh"},
		}}
	}
	v := o.Verdict
	return insight.Reason{Agent: domain.AgentQA, Text: fmt.Sprintf("Cả %d ảnh đều từ %d điểm trở lên nên Kiểm định tự duyệt ảnh theo Mức tự chủ của kênh. Máy vẫn không tự đăng.", v.Total, v.Min), Facts: []insight.Fact{
		{Label: "Ngưỡng điểm", Value: fmt.Sprintf("≥ %d", v.Min), Source: "Mức tự chủ của kênh"},
		{Label: "Ảnh đạt", Value: fmt.Sprintf("%d/%d", v.Passed, v.Total), Source: "Kiểm định"},
		{Label: "Điểm thấp nhất", Value: fmt.Sprintf("%d", v.Lowest), Source: "Kiểm định"},
	}}
}

func recordAutoApprove(ctx context.Context, tx pgx.Tx, jobID pgtype.UUID, o autonomy.ReviewOutcome) error {
	reason := approveReason(o)
	raw, err := json.Marshal(reason)
	if err != nil {
		return err
	}
	text := "Tự duyệt theo điểm"
	if o.Autonomy.Approve.Mode == domain.ApproveAll {
		text = "Tự duyệt hết ảnh"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO clone_auto_actions(job_id,kind,text,reason) VALUES($1,'auto_approve',$2,$3::jsonb)`, jobID, text, raw); err != nil {
		return err
	}
	prefs := activity.DefaultNotificationSettings()
	if _, err := NewSettingsStore(tx).Get(ctx, "notifications.preferences", &prefs); err != nil {
		return fmt.Errorf("đọc cài đặt thông báo: %w", err)
	}
	window := activity.NotifyMergeWindow
	if !prefs.Merge {
		window = 0
	}
	spec := activity.NotificationSpec{
		Kind: "action_result", Text: text + ": " + o.Title, Href: "#/?video=" + o.JobID, ActionLabel: "Xem",
		Audience: activity.AudienceAll, Object: &domain.ObjectRef{Type: "video", ID: o.JobID, Label: o.Title},
	}
	notice, err := upsertScopedNotificationTx(ctx, tx, gen.New(tx), spec, window)
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(activity.NotificationDetail{Reason: &reason})
	_, err = tx.Exec(ctx, `UPDATE notifications SET detail=$2::jsonb WHERE id=$1`, notice.ID, detail)
	return err
}

var autoFixLabels = map[string]string{
	domain.AutoFixSourceDownload: "tải video nguồn",
	domain.AutoFixImage:          "vẽ ảnh",
	domain.AutoFixRender:         "dựng video",
	domain.AutoFixQuality:        "đo lại Độ hoàn thiện",
}

// BeginAutoFix chạy trong một giao dịch với refreshDerived của CloneStore: step
// đang chạy về pending ngay, không qua `failed`, nên không có thông báo "Cần xử
// lý" cho lỗi máy tự sửa được.
func (s *AutonomyStore) BeginAutoFix(ctx context.Context, jobID string, step domain.CloneStep, code string, now time.Time) (bool, error) {
	id, err := cloneUUIDOf(jobID)
	if err != nil || !id.Valid {
		return false, ErrNotFound
	}
	retry := false
	err = s.clone.inTx(ctx, func(txctx context.Context) error {
		tx := txctx.Value(mediaTxKey{}).(pgx.Tx)
		var generation int
		var raw []byte
		if err := tx.QueryRow(txctx, `SELECT generation, auto_fix FROM clone_jobs WHERE id=$1 FOR UPDATE`, id).Scan(&generation, &raw); err != nil {
			return err
		}
		var existing domain.AutoFix
		if len(raw) > 0 && json.Unmarshal(raw, &existing) == nil && existing.At != "" && existing.Generation == generation {
			// Lần thử lại duy nhất đã dùng: lỗi này ở lại cho chủ kênh, kèm giờ đã thử.
			_, err := tx.Exec(txctx, `UPDATE clone_jobs SET auto_fix=jsonb_set(auto_fix,'{result}','"failed"') WHERE id=$1`, id)
			return err
		}
		tag, err := tx.Exec(txctx, `UPDATE clone_steps SET status='pending', error=NULL, finished_at=NULL, updated_at=now() WHERE job_id=$1 AND step=$2 AND status='running'`, id, string(step))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		fix := domain.AutoFix{At: now.Format(time.RFC3339), Code: code, Result: domain.AutoFixRetrying, Step: string(step), Generation: generation}
		fixJSON, _ := json.Marshal(fix)
		if _, err := tx.Exec(txctx, `UPDATE clone_jobs SET auto_fix=$2::jsonb WHERE id=$1`, id, fixJSON); err != nil {
			return err
		}
		kind, text := "auto_retry", "Tự thử lại "+autoFixLabels[code]
		if code == domain.AutoFixQuality {
			kind = "auto_fix"
		}
		reason, _ := json.Marshal(insight.Reason{Agent: "system", Text: "Theo Mức tự chủ của kênh, máy thử lại đúng một lần trước khi báo bạn.", Facts: []insight.Fact{
			{Label: "Bước", Value: string(step), Source: "Tiến trình"},
			{Label: "Lỗi", Value: code, Source: "Tiến trình"},
			{Label: "Lúc", Value: now.Format("15:04"), Source: "Tiến trình"},
		}})
		if _, err := tx.Exec(txctx, `INSERT INTO clone_auto_actions(job_id,kind,text,reason) VALUES($1,$2,$3,$4::jsonb)`, id, kind, text, reason); err != nil {
			return err
		}
		retry = true
		return s.clone.refreshDerived(txctx, id, nil)
	})
	return retry, err
}

func (s *AutonomyStore) SettleAutoFix(ctx context.Context, jobID string, step domain.CloneStep) error {
	id, err := cloneUUIDOf(jobID)
	if err != nil || !id.Valid {
		return nil
	}
	_, err = s.pool.Exec(ctx, `UPDATE clone_jobs SET auto_fix=jsonb_set(auto_fix,'{result}','"recovered"') WHERE id=$1 AND auto_fix->>'result'='retrying' AND auto_fix->>'step'=$2`, id, string(step))
	return err
}

// RaiseCandidates: kênh Duyệt ảnh đang "Hỏi" mà ≥ minStreak video duyệt gần
// nhất đều không vẽ lại ảnh nào và mọi ảnh đạt ngưỡng điểm.
func (s *AutonomyStore) RaiseCandidates(ctx context.Context, minStreak int) ([]autonomy.RaiseSignal, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, name, autonomy FROM owner_channels WHERE autonomy->'approve'->>'mode'='ask' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	type channel struct {
		id, name string
		a        domain.Autonomy
	}
	var channels []channel
	for rows.Next() {
		var c channel
		var raw []byte
		if err := rows.Scan(&c.id, &c.name, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		c.a = domain.ParseAutonomy(raw)
		channels = append(channels, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []autonomy.RaiseSignal{}
	for _, c := range channels {
		streak, err := s.cleanStreak(ctx, c.id, c.a.Approve.Min, minStreak)
		if err != nil {
			return nil, err
		}
		if streak >= minStreak {
			out = append(out, autonomy.RaiseSignal{ChannelID: c.id, ChannelName: c.name, Streak: streak, CurrentMode: c.a.Approve.Mode, Min: c.a.Approve.Min})
		}
	}
	return out, nil
}

func (s *AutonomyStore) cleanStreak(ctx context.Context, channelID string, min, limit int) (int, error) {
	id, err := uuidArg(channelID)
	if err != nil {
		return 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT i.checkpoint FROM clone_jobs j JOIN clone_steps i ON i.job_id=j.id AND i.step='images' WHERE j.channel_id=$1 AND j.superseded_at IS NULL AND j.image_approved_at IS NOT NULL ORDER BY j.image_approved_at DESC LIMIT $2`, id, limit+5)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	streak := 0
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return 0, err
		}
		var m images.Manifest
		if len(raw) == 0 || json.Unmarshal(raw, &m) != nil || !cleanApproval(m, min) {
			break
		}
		streak++
	}
	return streak, rows.Err()
}

// cleanApproval: không ảnh nào vẽ lại (mỗi mục đúng một version) và mọi ảnh
// đang dùng đạt ngưỡng điểm.
func cleanApproval(m images.Manifest, min int) bool {
	counted := 0
	for _, it := range m.Items {
		if it.Status == "skipped" {
			continue
		}
		v := images.Displayed(it)
		if len(it.Versions) != 1 || v == nil || v.Score == nil || v.Score.Value < min {
			return false
		}
		counted++
	}
	return counted > 0
}

// autoApprovalPending báo video vừa tới Chờ duyệt sẽ được máy tự duyệt ngay:
// kênh đặt Duyệt theo điểm (mọi ảnh đã đủ và đạt ngưỡng) hoặc Tự duyệt hết.
func autoApprovalPending(ctx context.Context, tx pgx.Tx, jobID string) (bool, error) {
	var autonomyRaw, checkpoint []byte
	err := tx.QueryRow(ctx, `SELECT c.autonomy, i.checkpoint FROM clone_jobs j JOIN owner_channels c ON c.id=j.channel_id JOIN clone_steps i ON i.job_id=j.id AND i.step='images' WHERE j.id=$1`, jobID).Scan(&autonomyRaw, &checkpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a := domain.ParseAutonomy(autonomyRaw)
	if a.Approve.Mode == domain.ApproveAsk {
		return false, nil
	}
	var m images.Manifest
	if len(checkpoint) == 0 || json.Unmarshal(checkpoint, &m) != nil || !images.ImagesReady(m) {
		return false, nil
	}
	if a.Approve.Mode == domain.ApproveAll {
		return true, nil
	}
	return autonomy.Evaluate(a.Approve.Min, manifestScores(m)).AllPass, nil
}

// manifestScores lấy điểm của ảnh đang hiển thị của từng mục trong manifest.
func manifestScores(m images.Manifest) []autonomy.ItemScore {
	out := make([]autonomy.ItemScore, 0, len(m.Items))
	scene := 0
	for _, i := range m.Items {
		if i.Kind == "scene" {
			scene++
		}
		item := autonomy.ItemScore{ID: i.ID, Kind: i.Kind, Label: autonomy.ItemLabel(i.Kind, scene), Skipped: i.Status == "skipped"}
		if v := images.Displayed(i); v != nil {
			item.Score = v.Score
		}
		out = append(out, item)
	}
	return out
}
