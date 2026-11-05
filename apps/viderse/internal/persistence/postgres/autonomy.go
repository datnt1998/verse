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

// AutonomyStore giữ việc máy tự làm sau V38: tự duyệt ảnh khi kênh bật
// Tự duyệt và mọi ảnh đạt ngưỡng cố định 85, và tự sửa lỗi đúng một lần.
type AutonomyStore struct {
	pool  *pgxpool.Pool
	clone *CloneStore
}

// NewAutonomyStore dựng store trên pool.
func NewAutonomyStore(pool *pgxpool.Pool) *AutonomyStore {
	return &AutonomyStore{pool: pool, clone: NewCloneStore(pool)}
}

var _ autonomy.Store = (*AutonomyStore)(nil)

// JobAutoApprove trả cờ Tự duyệt của kênh chứa job, và job có tồn tại không.
// Dùng cho tóm tắt điểm ảnh (ngưỡng cố định 85).
func (s *AutonomyStore) JobAutoApprove(ctx context.Context, jobID string) (bool, bool, error) {
	id, err := cloneUUIDOf(jobID)
	if err != nil || !id.Valid {
		return false, false, nil
	}
	var on bool
	err = s.pool.QueryRow(ctx, `SELECT c.auto_approve FROM clone_jobs j JOIN owner_channels c ON c.id=j.channel_id WHERE j.id=$1`, id).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return on, true, nil
}

// ChannelAutoApprove đọc cờ Tự duyệt hiện tại của kênh (false khi không có
// kênh). run.go đọc lại ngay trước khi duyệt để toggle tắt không bị bỏ qua.
func (s *AutonomyStore) ChannelAutoApprove(ctx context.Context, channelID string) (bool, error) {
	id, err := uuidArg(channelID)
	if err != nil {
		return false, nil
	}
	var on bool
	err = s.pool.QueryRow(ctx, `SELECT auto_approve FROM owner_channels WHERE id=$1`, id).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return on, err
}

const pendingReviewsSQL = `
SELECT j.id::text, j.channel_id::text, COALESCE(j.source_snapshot->>'title',''), COALESCE(r.rev,0), c.auto_approve,
       COALESCE((j.auto_review->>'tries')::int,0)
FROM clone_jobs j
JOIN owner_channels c ON c.id=j.channel_id
JOIN clone_steps i ON i.job_id=j.id AND i.step='images'
CROSS JOIN LATERAL (SELECT NULLIF(i.checkpoint->>'revision','')::int AS rev) r
WHERE j.superseded_at IS NULL AND j.cancel_requested_at IS NULL AND j.paused_at IS NULL
  AND (j.lease_expires_at IS NULL OR j.lease_expires_at <= now())
  AND c.auto_approve
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
		if err := rows.Scan(&p.JobID, &p.ChannelID, &p.Title, &p.ImageRevision, &p.AutoApprove, &p.Tries); err != nil {
			return nil, err
		}
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
	mark, _ := json.Marshal(map[string]any{"revision": o.Revision, "result": o.Result, "tries": o.Tries, "summary": o.Summary, "min": domain.AutoApproveMin, "at": time.Now().UTC().Format(time.RFC3339)})
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
	v := o.Verdict
	return insight.Reason{Agent: domain.AgentQA, Text: fmt.Sprintf("Cả %d ảnh đều từ %d điểm trở lên nên Kiểm định tự duyệt ảnh theo thiết lập Tự duyệt của kênh. Máy vẫn không tự đăng.", v.Total, v.Min), Facts: []insight.Fact{
		{Label: "Ngưỡng điểm", Value: fmt.Sprintf("≥ %d", v.Min), Source: "Thiết lập Tự duyệt của kênh"},
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
	const text = "Tự duyệt theo điểm"
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
		reason, _ := json.Marshal(insight.Reason{Agent: "system", Text: "Theo chính sách tự sửa, máy thử lại đúng một lần trước khi báo bạn.", Facts: []insight.Fact{
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

// autoApprovalPending báo video vừa tới Chờ duyệt sẽ được máy tự duyệt ngay:
// kênh bật Tự duyệt và mọi ảnh đã đủ, đạt ngưỡng cố định 85.
func autoApprovalPending(ctx context.Context, tx pgx.Tx, jobID string) (bool, error) {
	var autoApprove bool
	var checkpoint []byte
	err := tx.QueryRow(ctx, `SELECT c.auto_approve, i.checkpoint FROM clone_jobs j JOIN owner_channels c ON c.id=j.channel_id JOIN clone_steps i ON i.job_id=j.id AND i.step='images' WHERE j.id=$1`, jobID).Scan(&autoApprove, &checkpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !autoApprove {
		return false, nil
	}
	var m images.Manifest
	if len(checkpoint) == 0 || json.Unmarshal(checkpoint, &m) != nil || !images.ImagesReady(m) {
		return false, nil
	}
	// Mọi ảnh không bỏ qua phải đạt ngưỡng cố định 85.
	return autonomy.Evaluate(domain.AutoApproveMin, manifestScores(m)).AllPass, nil
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
