package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
)

// SuggestStore giữ Đề xuất của trợ lý và đọc tín hiệu cho bộ sinh.
type SuggestStore struct{ db gen.DBTX }

var _ suggest.Store = (*SuggestStore)(nil)

func NewSuggestStore(db gen.DBTX) *SuggestStore { return &SuggestStore{db: db} }

const suggestColumns = `s.id::text, s.kind, s.agent, s.channel_id::text, c.name, s.object, s.title, s.reason, s.primary_action, s.value, s.state, s.cooldown_until, s.created_at, s.expires_at`
const suggestFrom = ` FROM suggestions s LEFT JOIN owner_channels c ON c.id = s.channel_id `

func scanSuggestion(row pgx.Row) (suggest.Suggestion, error) {
	var x suggest.Suggestion
	var reason, primary []byte
	if err := row.Scan(&x.ID, &x.Kind, &x.Agent, &x.ChannelID, &x.ChannelName, &x.Object, &x.Title, &reason, &primary, &x.Value, &x.State, &x.Cooldown, &x.CreatedAt, &x.ExpiresAt); err != nil {
		return x, err
	}
	var stored struct {
		Evidence string         `json:"evidence"`
		Reason   insight.Reason `json:"reason"`
	}
	if err := json.Unmarshal(reason, &stored); err != nil {
		return x, err
	}
	x.Evidence, x.Reason = stored.Evidence, stored.Reason
	if x.Reason.Facts == nil {
		x.Reason.Facts = []insight.Fact{}
	}
	return x, json.Unmarshal(primary, &x.Primary)
}

func (s *SuggestStore) Upsert(ctx context.Context, d suggest.Draft, now time.Time) (suggest.Suggestion, bool, error) {
	reason, err := json.Marshal(map[string]any{"evidence": d.Evidence, "reason": d.Reason})
	if err != nil {
		return suggest.Suggestion{}, false, err
	}
	primary, err := json.Marshal(d.Primary)
	if err != nil {
		return suggest.Suggestion{}, false, err
	}
	ttl := d.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	var channel any
	if d.ChannelID != "" {
		channel = d.ChannelID
	}
	// Hàng mới: open. Hàng cũ đang mở hoặc expired: làm mới nội dung (expired mở
	// lại). Hàng accepted/dismissed chỉ mở lại khi cooldown đã hết; còn trong
	// cooldown thì giữ nguyên và trả về state cũ.
	var id string
	err = s.db.QueryRow(ctx, `INSERT INTO suggestions(kind,agent,channel_id,object,title,reason,primary_action,value,state,created_at,expires_at)
VALUES($1,$2,$3::uuid,$4,$5,$6::jsonb,$7::jsonb,$8,'open',$9,$10)
ON CONFLICT (kind, COALESCE(channel_id,'00000000-0000-0000-0000-000000000000'::uuid), object) DO UPDATE SET
  agent=EXCLUDED.agent, title=EXCLUDED.title, reason=EXCLUDED.reason, primary_action=EXCLUDED.primary_action, value=EXCLUDED.value,
  state='open', created_at=CASE WHEN suggestions.state='open' THEN suggestions.created_at ELSE EXCLUDED.created_at END,
  expires_at=EXCLUDED.expires_at, cooldown_until=NULL, resolved_at=NULL
 WHERE suggestions.state IN ('open','expired') OR (suggestions.cooldown_until IS NOT NULL AND suggestions.cooldown_until <= $9)
RETURNING id::text`, d.Kind, d.Agent, channel, d.Object, d.Title, reason, primary, d.Value, now, now.Add(ttl)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Còn trong cooldown (hoặc đã nhận): giữ nguyên hàng cũ.
		err = s.db.QueryRow(ctx, `SELECT id::text FROM suggestions WHERE kind=$1 AND COALESCE(channel_id,'00000000-0000-0000-0000-000000000000'::uuid)=COALESCE($2::uuid,'00000000-0000-0000-0000-000000000000'::uuid) AND object=$3`, d.Kind, channel, d.Object).Scan(&id)
	}
	if err != nil {
		return suggest.Suggestion{}, false, err
	}
	x, ok, err := s.Get(ctx, id)
	return x, ok, err
}

func (s *SuggestStore) Get(ctx context.Context, id string) (suggest.Suggestion, bool, error) {
	x, err := scanSuggestion(s.db.QueryRow(ctx, `SELECT `+suggestColumns+suggestFrom+` WHERE s.id::text = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return x, false, nil
	}
	return x, err == nil, err
}

func (s *SuggestStore) Open(ctx context.Context, now time.Time) ([]suggest.Suggestion, error) {
	rows, err := s.db.Query(ctx, `SELECT `+suggestColumns+suggestFrom+` WHERE s.state='open' AND s.expires_at > $1 AND s.kind = ANY($2::text[]) ORDER BY s.value DESC, s.created_at DESC, s.id`, now, suggest.Kinds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []suggest.Suggestion{}
	for rows.Next() {
		x, err := scanSuggestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *SuggestStore) Resolve(ctx context.Context, id, state string, cooldownUntil *time.Time, now time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `UPDATE suggestions SET state=$2, cooldown_until=$3, resolved_at=$4 WHERE id::text=$1 AND state='open'`, id, state, cooldownUntil, now)
	return tag.RowsAffected() > 0, err
}

func (s *SuggestStore) ExpireStale(ctx context.Context, now time.Time) error {
	_, err := s.db.Exec(ctx, `UPDATE suggestions SET state='expired', resolved_at=$1 WHERE state='open' AND expires_at <= $1`, now)
	return err
}

func (s *SuggestStore) CloseOpenExcept(ctx context.Context, kind string, keepIDs []string, now time.Time) error {
	_, err := s.db.Exec(ctx, `UPDATE suggestions SET state='expired', resolved_at=$3 WHERE state='open' AND kind=$1 AND NOT (id::text = ANY($2::text[]))`, kind, keepIDs, now)
	return err
}

func (s *SuggestStore) TrimOpen(ctx context.Context, n int, now time.Time) error {
	_, err := s.db.Exec(ctx, `UPDATE suggestions SET state='expired', resolved_at=$2 WHERE id IN (
SELECT id FROM suggestions WHERE state='open' ORDER BY value DESC, created_at DESC, id OFFSET $1)`, n, now)
	return err
}

func (s *SuggestStore) ChannelNames(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.Query(ctx, `SELECT id::text, name FROM owner_channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// ---- Tín hiệu -------------------------------------------------------------

func (s *SuggestStore) HotVideos(ctx context.Context, now time.Time) ([]suggest.HotVideo, error) {
	rows, err := s.db.Query(ctx, `
WITH queued AS (
  SELECT q.id, q.channel_id, q.source_key, row_number() OVER (PARTITION BY q.channel_id ORDER BY (q.priority = 'pick') DESC, q.position, q.id) AS rank,
         count(*) OVER (PARTITION BY q.channel_id) AS len
  FROM clone_queue q WHERE q.state = 'queued' AND q.removed_at IS NULL
), cand AS (
  SELECT oc.id::text AS channel_id, oc.name AS channel_name, rc.title AS rival_title, v.id AS video_id, v.title, v.score,
         COALESCE(v.view_count, 0) AS views, COALESCE(v.published_at, v.first_seen_at) AS published_at,
         q.id::text AS queue_id, COALESCE(q.rank, 0) AS rank, COALESCE(q.len, 0) AS len,
         row_number() OVER (PARTITION BY oc.id ORDER BY v.score DESC, v.id) AS n
  FROM owner_channels oc
  JOIN owner_channel_sources s ON s.owner_channel_id = oc.id
  JOIN rival_channels rc ON rc.id = s.rival_id
  JOIN rival_videos v ON v.channel_id = rc.id
  LEFT JOIN queued q ON q.source_key = ('channel:' || oc.id::text || ':video:' || v.id)
  WHERE v.score >= $2 AND v.origin = 'discovery' AND v.first_seen_at > $1
    AND NOT EXISTS (SELECT 1 FROM clone_jobs j WHERE j.source_key = ('channel:' || oc.id::text || ':video:' || v.id) AND j.superseded_at IS NULL)
)
SELECT channel_id, channel_name, rival_title, video_id, title, score, views, published_at, queue_id, rank, len
FROM cand WHERE n <= 2 ORDER BY score DESC, video_id`, now.Add(-suggest.HotVideoWindow), suggest.HotVideoMinScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []suggest.HotVideo
	for rows.Next() {
		var h suggest.HotVideo
		var queueID *string
		if err := rows.Scan(&h.ChannelID, &h.ChannelName, &h.RivalTitle, &h.VideoID, &h.Title, &h.Score, &h.Views, &h.PublishedAt, &queueID, &h.QueueRank, &h.QueueLen); err != nil {
			return nil, err
		}
		if queueID != nil {
			h.Queued, h.QueueID = true, *queueID
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *SuggestStore) FailingTemplates(ctx context.Context, now time.Time) ([]suggest.FailingTemplate, error) {
	rows, err := s.db.Query(ctx, `
SELECT oc.id::text, oc.name, t.id::text, t.name,
  count(*) FILTER (WHERE EXISTS (SELECT 1 FROM clone_steps st WHERE st.job_id = j.id AND st.status = 'failed' AND st.error->>'code' = 'quality_gate'))::int AS failed,
  count(*)::int AS total, min(j.created_at)
FROM clone_jobs j
JOIN owner_channels oc ON oc.id = j.channel_id
JOIN templates t ON t.id = j.template_id
WHERE j.superseded_at IS NULL AND j.created_at > $1
  AND EXISTS (SELECT 1 FROM clone_steps st WHERE st.job_id = j.id AND st.step IN ('score','export') AND st.status IN ('succeeded','failed'))
GROUP BY oc.id, oc.name, t.id, t.name
HAVING count(*) FILTER (WHERE EXISTS (SELECT 1 FROM clone_steps st WHERE st.job_id = j.id AND st.status = 'failed' AND st.error->>'code' = 'quality_gate')) > 0
ORDER BY 5 DESC, t.id`, now.Add(-suggest.FailingWindow))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []suggest.FailingTemplate
	for rows.Next() {
		var f suggest.FailingTemplate
		if err := rows.Scan(&f.ChannelID, &f.ChannelName, &f.TemplateID, &f.Template, &f.Failed, &f.Total, &f.Since); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
