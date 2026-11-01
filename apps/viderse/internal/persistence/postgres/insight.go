package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"time"
)

// ReadRecap reads current waiting jobs and channel counters from the same
// durable clone state that drives the workbench.
func (s *ActivityStore) ReadRecap(ctx context.Context, since, now time.Time) (insight.Recap, error) {
	r := insight.Recap{From: since, To: now, NeedsYou: []insight.Need{}, Auto: []insight.AutoAction{}, Suggestions: []any{}, Channels: []insight.Channel{}}
	rows, err := s.db.Query(ctx, `SELECT id::text, COALESCE(source_snapshot->>'title',''), stage_group FROM clone_jobs WHERE superseded_at IS NULL AND stage_group IN ('review','attention') ORDER BY created_at,id`)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var n insight.Need
		if err = rows.Scan(&n.JobID, &n.Label, &n.State); err != nil {
			return r, err
		}
		r.NeedsYou = append(r.NeedsYou, n)
	}
	if err = rows.Err(); err != nil {
		return r, err
	}
	rows.Close()
	rows, err = s.db.Query(ctx, `SELECT c.id::text,c.pace_per_day, count(j.id) FILTER (WHERE j.stage_group IN ('ready','posted') AND j.updated_at >= date_trunc('day',$1::timestamptz)), count(j.id) FILTER (WHERE j.stage_group IN ('review','attention')), count(j.id) FILTER (WHERE j.stage_group='ready') FROM owner_channels c LEFT JOIN clone_jobs j ON j.channel_id=c.id AND j.superseded_at IS NULL GROUP BY c.id,c.pace_per_day ORDER BY c.id`, now)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var c insight.Channel
		if err = rows.Scan(&c.ChannelID, &c.Cap, &c.Done, &c.Waiting, &c.Ready); err != nil {
			return r, err
		}
		r.Channels = append(r.Channels, c)
	}
	if err = rows.Err(); err != nil {
		return r, err
	}
	rows.Close()
	err = s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM clone_queue WHERE state='queued'), EXISTS(SELECT 1 FROM clone_jobs WHERE superseded_at IS NULL AND updated_at > $1 AND updated_at <= $2) OR EXISTS(SELECT 1 FROM clone_queue WHERE created_at > $1 AND created_at <= $2)`, since, now).Scan(&r.Today.Queued, &r.HasNew)
	if err != nil {
		return r, err
	}
	var picked int
	var pickedIDs []string
	if err = s.db.QueryRow(ctx, `SELECT count(*), COALESCE(array_agg(id::text) FILTER (WHERE state='queued' AND removed_at IS NULL AND job_id IS NULL), '{}') FROM clone_queue WHERE origin='auto' AND created_at > $1 AND created_at <= $2`, since, now).Scan(&picked, &pickedIDs); err != nil {
		return r, err
	}
	if picked > 0 {
		action := insight.AutoAction{Kind: "auto_pick", Text: fmt.Sprintf("Tự lấy %d video mới", picked), Count: picked, Href: "#/queue"}
		if len(pickedIDs) > 0 {
			action.Undo = &insight.Undo{Op: "remove_queue", Params: map[string]any{"ids": pickedIDs}}
		}
		r.Auto = append(r.Auto, action)
	}
	rows, err = s.db.Query(ctx, `SELECT kind,text,reason,job_id::text FROM clone_auto_actions WHERE created_at > $1 AND created_at <= $2 ORDER BY created_at DESC,id DESC`, since, now)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var action insight.AutoAction
		var raw []byte
		var jobID string
		if err = rows.Scan(&action.Kind, &action.Text, &raw, &jobID); err != nil {
			rows.Close()
			return r, err
		}
		var reason insight.Reason
		if err = json.Unmarshal(raw, &reason); err != nil {
			rows.Close()
			return r, err
		}
		action.Count = 1
		action.Href = "#/?video=" + jobID
		action.Reason = &reason
		r.Auto = append(r.Auto, action)
		r.HasNew = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return r, err
	}
	rows.Close()
	var remaining, total *int
	if err = s.db.QueryRow(ctx, `SELECT sum(GREATEST(s.progress_total-s.progress_done,0))::int, sum(s.progress_total)::int FROM clone_steps s JOIN clone_jobs j ON j.id=s.job_id WHERE s.step='images' AND s.progress_total IS NOT NULL AND j.superseded_at IS NULL AND j.created_at >= date_trunc('day',$1::timestamptz)`, now).Scan(&remaining, &total); err != nil {
		return r, err
	}
	r.Today.ImagesLeft, r.Today.ImagesTotal = remaining, total
	r.Summarize()
	return r, nil
}
