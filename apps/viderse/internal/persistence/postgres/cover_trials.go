package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
)

// CoverTrialStore lưu lịch sử vẽ thử ảnh (migration 00037). Store dùng SQL
// thẳng thay vì sqlc vì hai bảng chỉ phục vụ một tính năng và cần chạy trong
// transaction của media.Publish (mediaDB đọc tx từ ctx).
type CoverTrialStore struct {
	pool *pgxpool.Pool
}

// NewCoverTrialStore dựng store trên pool.
func NewCoverTrialStore(pool *pgxpool.Pool) *CoverTrialStore {
	return &CoverTrialStore{pool: pool}
}

var _ library.TrialStore = (*CoverTrialStore)(nil)

const coverTrialColumns = `id::text, template_id::text, job_id::text, job_title, prompt_id,
	prompt_name, composition, hook_text, state, coalesce(media_path, ''),
	provider, model, width, height, cost_images, error_code, error_message,
	error_recovery, retry_at, created_at, finished_at`

func scanCoverTrial(row interface{ Scan(...any) error }) (library.CoverTrial, error) {
	var t library.CoverTrial
	var media, code, message, recovery *string
	var retryAt, finishedAt *time.Time
	err := row.Scan(
		&t.ID, &t.TemplateID, &t.JobID, &t.JobTitle, &t.PromptID,
		&t.PromptName, &t.Composition, &t.HookText, &t.State, &media,
		&t.Provider, &t.Model, &t.Width, &t.Height, &t.CostImages, &code, &message,
		&recovery, &retryAt, &t.CreatedAt, &finishedAt,
	)
	if err != nil {
		return t, err
	}
	if media != nil {
		t.MediaPath = *media
	}
	if code != nil || message != nil || recovery != nil || retryAt != nil {
		t.Error = &library.TrialError{}
		if code != nil {
			t.Error.Code = *code
		}
		if message != nil {
			t.Error.Message = *message
		}
		if recovery != nil {
			t.Error.Recovery = *recovery
		}
		t.Error.RetryAt = retryAt
	}
	t.FinishedAt = finishedAt
	return t, nil
}

// ListCoverTrials trả lịch sử mới nhất trước, tối đa limit hàng.
func (s *CoverTrialStore) ListCoverTrials(ctx context.Context, templateID string, limit int) ([]library.CoverTrial, error) {
	tid, err := uuidArg(templateID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 12
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT `+coverTrialColumns+` FROM cover_trials
		  WHERE template_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, tid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]library.CoverTrial, 0, limit)
	for rows.Next() {
		t, err := scanCoverTrial(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetCoverTrial đọc một lần vẽ thử theo template + id. Id hỏng không thể là
// một hàng có thật nên trả ErrNotFound (404), không phải lỗi nội bộ 500.
func (s *CoverTrialStore) GetCoverTrial(ctx context.Context, templateID, id string) (*library.CoverTrial, error) {
	tid, err := uuidArg(templateID)
	if err != nil {
		return nil, library.ErrNotFound
	}
	idArg, err := uuidArg(id)
	if err != nil {
		return nil, library.ErrNotFound
	}
	t, err := scanCoverTrial(mediaDB(ctx, s.pool).QueryRow(ctx,
		`SELECT `+coverTrialColumns+` FROM cover_trials WHERE template_id = $1 AND id = $2`, tid, idArg))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, library.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// InsertCoverTrial ghi một hàng mới (thường trong tx của media.Publish).
func (s *CoverTrialStore) InsertCoverTrial(ctx context.Context, t library.CoverTrial) error {
	tid, err := uuidArg(t.TemplateID)
	if err != nil {
		return err
	}
	id, err := uuidArg(t.ID)
	if err != nil {
		return err
	}
	job, err := uuidArg(t.JobID)
	if err != nil {
		return err
	}
	_, err = mediaDB(ctx, s.pool).Exec(ctx,
		`INSERT INTO cover_trials (id, template_id, job_id, job_title, prompt_id, prompt_name,
			composition, hook_text, state, media_path, provider, model, width, height,
			cost_images, error_code, error_message, error_recovery, retry_at, created_at, finished_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		id, tid, job, t.JobTitle, t.PromptID, t.PromptName, t.Composition, t.HookText,
		t.State, nullText(t.MediaPath), t.Provider, t.Model, t.Width, t.Height, t.CostImages,
		errorCode(t), errorMessage(t), errorRecovery(t), errorRetryAt(t), t.CreatedAt, t.FinishedAt)
	return err
}

// UpdateCoverTrial cập nhật trạng thái/kết quả của một hàng đã có.
func (s *CoverTrialStore) UpdateCoverTrial(ctx context.Context, t library.CoverTrial) error {
	id, err := uuidArg(t.ID)
	if err != nil {
		return err
	}
	tid, err := uuidArg(t.TemplateID)
	if err != nil {
		return err
	}
	tag, err := mediaDB(ctx, s.pool).Exec(ctx,
		`UPDATE cover_trials SET state=$3, media_path=$4, provider=$5, model=$6, width=$7,
			height=$8, cost_images=$9, error_code=$10, error_message=$11, error_recovery=$12,
			retry_at=$13, hook_text=$14, finished_at=$15
		 WHERE id=$1 AND template_id=$2`,
		id, tid, t.State, nullText(t.MediaPath), t.Provider, t.Model, t.Width, t.Height,
		t.CostImages, errorCode(t), errorMessage(t), errorRecovery(t), errorRetryAt(t),
		t.HookText, t.FinishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return library.ErrNotFound
	}
	return nil
}

// CoverTrialsBeyond trả các hàng cũ hơn keep hàng mới nhất (để dọn + nhả media).
func (s *CoverTrialStore) CoverTrialsBeyond(ctx context.Context, templateID string, keep int) ([]library.CoverTrial, error) {
	tid, err := uuidArg(templateID)
	if err != nil {
		return nil, err
	}
	if keep <= 0 {
		keep = 12
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT `+coverTrialColumns+` FROM cover_trials WHERE template_id = $1
		  ORDER BY created_at DESC, id DESC OFFSET $2`, tid, keep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []library.CoverTrial{}
	for rows.Next() {
		t, err := scanCoverTrial(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteCoverTrials xoá các hàng theo id (gọi trong tx nhả media).
func (s *CoverTrialStore) DeleteCoverTrials(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := mediaDB(ctx, s.pool).Exec(ctx, `DELETE FROM cover_trials WHERE id::text = ANY($1::text[])`, ids)
	return err
}

// CoverTrialMediaPaths trả mọi media_path của template (trước khi xoá template).
func (s *CoverTrialStore) CoverTrialMediaPaths(ctx context.Context, templateID string) ([]string, error) {
	tid, err := uuidArg(templateID)
	if err != nil {
		return nil, err
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT media_path FROM cover_trials WHERE template_id = $1 AND media_path <> ''`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}

// FailRunningCoverTrials đánh dấu các lần vẽ dở dang khi tiến trình khởi động
// lại. Không tự chạy lại: người dùng bấm Thử lại.
func (s *CoverTrialStore) FailRunningCoverTrials(ctx context.Context, at time.Time, message, recovery string) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE cover_trials SET state='failed', error_code='interrupted', error_message=$1,
			error_recovery=$2, finished_at=$3
		 WHERE state='running'`, message, recovery, at)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

const promptSampleColumns = `id::text, prompt_id, prompt_name, prompt_kind, coalesce(composition, ''),
	video_id::text, video_title, scene_id, hook_text, state, coalesce(media_path, ''),
	provider, model, width, height, cost_images, error_code, error_message,
	error_recovery, retry_at, created_at, finished_at`

func scanPromptSample(row interface{ Scan(...any) error }) (library.PromptSample, error) {
	var s library.PromptSample
	var media, code, message, recovery *string
	var retryAt, finishedAt *time.Time
	err := row.Scan(
		&s.ID, &s.PromptID, &s.PromptName, &s.PromptKind, &s.Composition,
		&s.VideoID, &s.VideoTitle, &s.SceneID, &s.HookText, &s.State, &media,
		&s.Provider, &s.Model, &s.Width, &s.Height, &s.CostImages, &code, &message,
		&recovery, &retryAt, &s.CreatedAt, &finishedAt,
	)
	if err != nil {
		return s, err
	}
	if media != nil {
		s.MediaPath = *media
	}
	if code != nil || message != nil || recovery != nil || retryAt != nil {
		s.Error = &library.TrialError{}
		if code != nil {
			s.Error.Code = *code
		}
		if message != nil {
			s.Error.Message = *message
		}
		if recovery != nil {
			s.Error.Recovery = *recovery
		}
		s.Error.RetryAt = retryAt
	}
	s.FinishedAt = finishedAt
	return s, nil
}

// ListPromptSamples trả ảnh mẫu mới nhất trước, tối đa limit hàng.
func (s *CoverTrialStore) ListPromptSamples(ctx context.Context, promptID string, limit int) ([]library.PromptSample, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT `+promptSampleColumns+` FROM image_prompt_samples
		  WHERE prompt_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, promptID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]library.PromptSample, 0, limit)
	for rows.Next() {
		sample, err := scanPromptSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, rows.Err()
}

// GetPromptSample đọc một ảnh mẫu theo prompt + id. Id hỏng trả ErrNotFound.
func (s *CoverTrialStore) GetPromptSample(ctx context.Context, promptID, id string) (*library.PromptSample, error) {
	idArg, err := uuidArg(id)
	if err != nil {
		return nil, library.ErrNotFound
	}
	sample, err := scanPromptSample(mediaDB(ctx, s.pool).QueryRow(ctx,
		`SELECT `+promptSampleColumns+` FROM image_prompt_samples WHERE prompt_id = $1 AND id = $2`,
		promptID, idArg))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, library.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sample, nil
}

// InsertPromptSample ghi một ảnh mẫu mới.
func (s *CoverTrialStore) InsertPromptSample(ctx context.Context, sample library.PromptSample) error {
	id, err := uuidArg(sample.ID)
	if err != nil {
		return err
	}
	video, err := uuidArg(sample.VideoID)
	if err != nil {
		return err
	}
	_, err = mediaDB(ctx, s.pool).Exec(ctx,
		`INSERT INTO image_prompt_samples (id, prompt_id, prompt_name, prompt_kind, composition,
			video_id, video_title, scene_id, hook_text, state, media_path, provider, model, width,
			height, cost_images, error_code, error_message, error_recovery, retry_at, created_at, finished_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		id, sample.PromptID, sample.PromptName, sample.PromptKind, nullText(sample.Composition),
		video, sample.VideoTitle, sample.SceneID, sample.HookText, sample.State,
		nullText(sample.MediaPath), sample.Provider, sample.Model, sample.Width, sample.Height,
		sample.CostImages, sampleErrorCode(sample), sampleErrorMessage(sample),
		sampleErrorRecovery(sample), sampleErrorRetryAt(sample), sample.CreatedAt, sample.FinishedAt)
	return err
}

// UpdatePromptSample cập nhật trạng thái/kết quả của một ảnh mẫu.
func (s *CoverTrialStore) UpdatePromptSample(ctx context.Context, sample library.PromptSample) error {
	id, err := uuidArg(sample.ID)
	if err != nil {
		return err
	}
	tag, err := mediaDB(ctx, s.pool).Exec(ctx,
		`UPDATE image_prompt_samples SET state=$3, media_path=$4, provider=$5, model=$6,
			width=$7, height=$8, cost_images=$9, error_code=$10, error_message=$11,
			error_recovery=$12, retry_at=$13, hook_text=$14, finished_at=$15
		 WHERE id=$1 AND prompt_id=$2`,
		id, sample.PromptID, sample.State, nullText(sample.MediaPath), sample.Provider,
		sample.Model, sample.Width, sample.Height, sample.CostImages, sampleErrorCode(sample),
		sampleErrorMessage(sample), sampleErrorRecovery(sample), sampleErrorRetryAt(sample),
		sample.HookText, sample.FinishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return library.ErrNotFound
	}
	return nil
}

// PromptSamplesBeyond trả các ảnh mẫu cũ hơn keep hàng mới nhất.
func (s *CoverTrialStore) PromptSamplesBeyond(ctx context.Context, promptID string, keep int) ([]library.PromptSample, error) {
	if keep <= 0 {
		keep = 12
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT `+promptSampleColumns+` FROM image_prompt_samples
		  WHERE prompt_id = $1 ORDER BY created_at DESC, id DESC OFFSET $2`, promptID, keep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []library.PromptSample{}
	for rows.Next() {
		sample, err := scanPromptSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, rows.Err()
}

// DeletePromptSamples xoá các ảnh mẫu theo id (gọi trong tx nhả media).
func (s *CoverTrialStore) DeletePromptSamples(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := mediaDB(ctx, s.pool).Exec(ctx, `DELETE FROM image_prompt_samples WHERE id::text = ANY($1::text[])`, ids)
	return err
}

// DeletePromptSamplesByPrompt xoá mọi hàng ảnh mẫu của prompt (gọi trong tx
// nhả media sau khi prompt đã xoá; bảng không có FK vì prompt có sẵn).
func (s *CoverTrialStore) DeletePromptSamplesByPrompt(ctx context.Context, promptID string) error {
	_, err := mediaDB(ctx, s.pool).Exec(ctx, `DELETE FROM image_prompt_samples WHERE prompt_id=$1`, promptID)
	return err
}

// PromptSampleMediaPaths trả mọi media_path của prompt (trước khi xoá prompt).
func (s *CoverTrialStore) PromptSampleMediaPaths(ctx context.Context, promptID string) ([]string, error) {
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT media_path FROM image_prompt_samples WHERE prompt_id = $1 AND media_path <> ''`, promptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}

// FailRunningPromptSamples đánh dấu các ảnh mẫu dở dang khi khởi động lại.
func (s *CoverTrialStore) FailRunningPromptSamples(ctx context.Context, at time.Time, message, recovery string) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE image_prompt_samples SET state='failed', error_code='interrupted',
			error_message=$1, error_recovery=$2, finished_at=$3
		 WHERE state='running'`, message, recovery, at)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ListPromptSampleIDs trả media id ảnh mẫu của nhiều prompt, mới nhất trước.
func (s *CoverTrialStore) ListPromptSampleIDs(ctx context.Context, promptIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(promptIDs))
	if len(promptIDs) == 0 {
		return out, nil
	}
	rows, err := mediaDB(ctx, s.pool).Query(ctx,
		`SELECT prompt_id, id::text FROM image_prompt_samples
		  WHERE prompt_id = ANY($1) AND state = 'ready'
		  ORDER BY prompt_id, created_at DESC, id DESC`, promptIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var promptID, id string
		if err := rows.Scan(&promptID, &id); err != nil {
			return nil, err
		}
		if len(out[promptID]) < 12 {
			out[promptID] = append(out[promptID], id)
		}
	}
	return out, rows.Err()
}

func nullText(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func errorCode(t library.CoverTrial) *string {
	if t.Error == nil {
		return nil
	}
	return nullText(t.Error.Code)
}

func errorMessage(t library.CoverTrial) *string {
	if t.Error == nil {
		return nil
	}
	return nullText(t.Error.Message)
}

func errorRecovery(t library.CoverTrial) *string {
	if t.Error == nil {
		return nil
	}
	return nullText(t.Error.Recovery)
}

func errorRetryAt(t library.CoverTrial) *time.Time {
	if t.Error == nil {
		return nil
	}
	return t.Error.RetryAt
}

func sampleErrorCode(s library.PromptSample) *string {
	if s.Error == nil {
		return nil
	}
	return nullText(s.Error.Code)
}

func sampleErrorMessage(s library.PromptSample) *string {
	if s.Error == nil {
		return nil
	}
	return nullText(s.Error.Message)
}

func sampleErrorRecovery(s library.PromptSample) *string {
	if s.Error == nil {
		return nil
	}
	return nullText(s.Error.Recovery)
}

func sampleErrorRetryAt(s library.PromptSample) *time.Time {
	if s.Error == nil {
		return nil
	}
	return s.Error.RetryAt
}
