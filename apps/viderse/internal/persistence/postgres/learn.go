package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/learn"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// LearnStore giữ quy tắc đã học và các lần sửa/góp ý dùng để học.
type LearnStore struct {
	db       gen.DBTX
	settings *SettingsStore
}

var _ learn.Store = (*LearnStore)(nil)

func NewLearnStore(db gen.DBTX) *LearnStore {
	return &LearnStore{db: db, settings: NewSettingsStore(db)}
}

func (s *LearnStore) Get(ctx context.Context, key string, out any) (bool, error) {
	return s.settings.Get(ctx, key, out)
}
func (s *LearnStore) Set(ctx context.Context, key string, value any) error {
	return s.settings.Set(ctx, key, value)
}

const ruleColumns = `id::text, channel_id::text, image_prompt_id, agent, text, COALESCE(pattern_key,''), source, source_label, learned_from, enabled, created_at`

func scanRule(row pgx.Row) (learn.Rule, error) {
	var r learn.Rule
	var raw []byte
	if err := row.Scan(&r.ID, &r.ChannelID, &r.ImagePromptID, &r.Agent, &r.Text, &r.PatternKey, &r.Source, &r.SourceLabel, &raw, &r.Enabled, &r.CreatedAt); err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw, &r.LearnedFrom); err != nil {
		return r, err
	}
	if r.LearnedFrom.Examples == nil {
		r.LearnedFrom.Examples = []learn.Example{}
	}
	return r, nil
}

func (s *LearnStore) ListRules(ctx context.Context, f learn.RuleFilter) ([]learn.Rule, error) {
	q := `SELECT ` + ruleColumns + ` FROM learned_rules WHERE ($1 = '' OR channel_id::text = $1) AND ($2 = '' OR image_prompt_id = $2) AND (NOT $3 OR enabled) ORDER BY created_at, id`
	rows, err := s.db.Query(ctx, q, f.ChannelID, f.ImagePromptID, f.OnlyEnabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []learn.Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *LearnStore) GetRule(ctx context.Context, id string) (learn.Rule, bool, error) {
	r, err := scanRule(s.db.QueryRow(ctx, `SELECT `+ruleColumns+` FROM learned_rules WHERE id::text = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

func (s *LearnStore) InsertRule(ctx context.Context, r learn.Rule) (learn.Rule, bool, error) {
	raw, err := json.Marshal(r.LearnedFrom)
	if err != nil {
		return r, false, err
	}
	out, err := scanRule(s.db.QueryRow(ctx, `INSERT INTO learned_rules(channel_id,image_prompt_id,agent,text,pattern_key,source,source_label,learned_from,enabled,created_at)
VALUES($1::uuid,$2,$3,$4,NULLIF($5,''),$6,$7,$8::jsonb,$9,$10)
ON CONFLICT DO NOTHING
RETURNING `+ruleColumns, r.ChannelID, r.ImagePromptID, r.Agent, r.Text, r.PatternKey, r.Source, r.SourceLabel, raw, r.Enabled, r.CreatedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		filter := learn.RuleFilter{}
		if r.ChannelID != nil {
			filter.ChannelID = *r.ChannelID
		} else {
			filter.ImagePromptID = *r.ImagePromptID
		}
		list, lerr := s.ListRules(ctx, filter)
		if lerr != nil {
			return r, false, lerr
		}
		for _, e := range list {
			if strings.EqualFold(e.Text, r.Text) {
				return e, false, nil
			}
		}
		return r, false, err
	}
	return out, err == nil, err
}

func (s *LearnStore) UpdateRule(ctx context.Context, id string, enabled *bool, text *string) (learn.Rule, bool, error) {
	r, err := scanRule(s.db.QueryRow(ctx, `UPDATE learned_rules SET enabled = COALESCE($2, enabled), text = COALESCE($3, text), updated_at = now() WHERE id::text = $1 RETURNING `+ruleColumns, id, enabled, text))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

func (s *LearnStore) DeleteRule(ctx context.Context, id string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM learned_rules WHERE id::text = $1`, id)
	return tag.RowsAffected() > 0, err
}

func (s *LearnStore) ChannelExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM owner_channels WHERE id::text = $1)`, id).Scan(&ok)
	return ok, err
}

func (s *LearnStore) ReplaceEdits(ctx context.Context, jobID, channelID string, rows []learn.EditRow) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM learn_edits WHERE job_id::text = $1`, jobID); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := s.db.Exec(ctx, `INSERT INTO learn_edits(job_id,channel_id,part,key,before_text,after_text,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			r.JobID, r.ChannelID, r.Part, r.Key, r.Before, r.After, r.At); err != nil {
			return err
		}
	}
	return nil
}

func (s *LearnStore) EditStats(ctx context.Context, minJobs int) ([]learn.EditStat, error) {
	rows, err := s.db.Query(ctx, `SELECT e.channel_id::text, e.key, count(DISTINCT e.job_id), min(e.created_at), max(e.created_at),
 COALESCE((SELECT jsonb_agg(x) FROM (SELECT jsonb_build_object('before',e2.before_text,'after',e2.after_text,'jobId',e2.job_id::text) x FROM learn_edits e2 WHERE e2.channel_id=e.channel_id AND e2.key=e.key ORDER BY e2.created_at DESC, e2.job_id LIMIT 5) t),'[]'::jsonb),
 COALESCE((SELECT jsonb_agg(a) FROM (SELECT e3.after_text a FROM learn_edits e3 WHERE e3.channel_id=e.channel_id AND e3.key=e.key ORDER BY e3.created_at DESC LIMIT 50) t),'[]'::jsonb)
FROM learn_edits e GROUP BY e.channel_id, e.key HAVING count(DISTINCT e.job_id) >= $1 ORDER BY max(e.created_at) DESC`, minJobs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learn.EditStat
	for rows.Next() {
		var st learn.EditStat
		var ex, afters []byte
		if err := rows.Scan(&st.ChannelID, &st.Key, &st.Jobs, &st.First, &st.Last, &ex, &afters); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(ex, &st.Examples); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(afters, &st.Afters); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *LearnStore) OtherEdits(ctx context.Context, channelID string, since time.Time, limit int) ([]learn.EditRow, error) {
	rows, err := s.db.Query(ctx, `SELECT job_id::text, channel_id::text, part, key, before_text, after_text, created_at FROM learn_edits WHERE channel_id::text = $1 AND key = $2 AND created_at >= $3 ORDER BY created_at DESC, job_id LIMIT $4`, channelID, learn.KeyOther, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learn.EditRow
	for rows.Next() {
		var r learn.EditRow
		if err := rows.Scan(&r.JobID, &r.ChannelID, &r.Part, &r.Key, &r.Before, &r.After, &r.At); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *LearnStore) AddFeedback(ctx context.Context, f learn.FeedbackRow) error {
	_, err := s.db.Exec(ctx, `INSERT INTO learn_feedback(job_id,channel_id,image_prompt_id,text,norm,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6)`, f.JobID, f.ChannelID, f.ImagePromptID, f.Text, f.Norm, f.At)
	return err
}

func (s *LearnStore) FeedbackStats(ctx context.Context, minCount int) ([]learn.FeedbackStat, error) {
	rows, err := s.db.Query(ctx, `SELECT f.image_prompt_id, (array_agg(f.norm ORDER BY f.created_at DESC))[1], count(*), min(f.created_at), max(f.created_at),
 COALESCE(jsonb_agg(jsonb_build_object('before','','after',f.text,'jobId',f.job_id::text) ORDER BY f.created_at DESC),'[]'::jsonb)
FROM learn_feedback f GROUP BY f.image_prompt_id, lower(f.norm) HAVING count(*) >= $1 ORDER BY max(f.created_at) DESC`, minCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learn.FeedbackStat
	for rows.Next() {
		var st learn.FeedbackStat
		var ex []byte
		if err := rows.Scan(&st.ImagePromptID, &st.Norm, &st.Count, &st.First, &st.Last, &ex); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(ex, &st.Examples); err != nil {
			return nil, err
		}
		if len(st.Examples) > 5 {
			st.Examples = st.Examples[:5]
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *LearnStore) FeedbackNorms(ctx context.Context, imagePromptID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT norm FROM learn_feedback WHERE image_prompt_id = $1 GROUP BY norm ORDER BY max(created_at) DESC LIMIT 20`, imagePromptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
