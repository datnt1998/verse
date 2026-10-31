package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// writeTransitionDetail runs while both the notification merge advisory lock
// and the job transition transaction are held.
func writeTransitionDetail(ctx context.Context, tx pgx.Tx, notificationID pgtype.UUID, job domain.CloneJob, stage string) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT detail FROM notifications WHERE id=$1 FOR UPDATE`, notificationID).Scan(&raw); err != nil {
		return err
	}
	var detail activity.NotificationDetail
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &detail); err != nil {
			return err
		}
	}
	switch stage {
	case domain.StageReview:
		var name string
		if err := tx.QueryRow(ctx, `SELECT name FROM owner_channels WHERE id=$1`, job.ChannelID).Scan(&name); err != nil {
			return err
		}
		found := false
		for i := range detail.Channels {
			if detail.Channels[i].ChannelID == job.ChannelID {
				detail.Channels[i].Count++
				found = true
				break
			}
		}
		if !found {
			detail.Channels = append(detail.Channels, activity.NotificationChannelDetail{ChannelID: job.ChannelID, Name: name, Count: 1})
		}
	case domain.StageAttention:
		var cause, recovery string
		err := tx.QueryRow(ctx, `SELECT error->>'message', COALESCE(error->>'recovery','') FROM clone_steps WHERE job_id=$1 AND status IN ('failed','waiting') AND COALESCE(error->>'message','')<>'' ORDER BY updated_at DESC LIMIT 1`, job.ID).Scan(&cause, &recovery)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		detail.Cause = cause
		detail.Recovery = nil
		lower := strings.ToLower(recovery)
		if strings.Contains(lower, "đăng nhập") && (strings.Contains(lower, "kết nối ai") || strings.Contains(lower, "tạo ảnh minh hoạ")) {
			detail.Recovery = &activity.NotificationAction{Label: "Đăng nhập lại", Href: "#/settings?section=connect"}
		}
	case domain.StageReady:
		mp4 := &activity.NotificationMP4Detail{}
		// Export durationMs measures the render job, not the video. The build
		// output is the actual project timeline used by the exporter.
		err := tx.QueryRow(ctx, `SELECT CASE WHEN jsonb_typeof(output_manifest->'data'->'durationSec')='number' THEN (output_manifest->'data'->>'durationSec')::double precision END FROM clone_steps WHERE job_id=$1 AND step='build'`, job.ID).Scan(&mp4.DurationSec)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT CASE WHEN jsonb_typeof(output_manifest->'data'->'size')='number' THEN (output_manifest->'data'->>'size')::bigint END FROM clone_steps WHERE job_id=$1 AND step='export'`, job.ID).Scan(&mp4.Bytes)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if mp4.DurationSec != nil || mp4.Bytes != nil {
			detail.MP4 = mp4
		}
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE notifications SET detail=$2::jsonb WHERE id=$1`, notificationID, encoded)
	return err
}
