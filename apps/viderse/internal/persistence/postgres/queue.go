package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
)

// QueueStore là tầng lưu trữ của hàng chờ clone (§3) trên gen.Queries —
// mirror của queue.ts + repository.ts (phần queue/auto-pick) của yt-factory.
// Mọi hàm nhận gen.DBTX qua constructor nên gọi được cả trong transaction
// lẫn trên pool (service tự mở tx khi cần atomic).
type QueueStore struct {
	q *gen.Queries
}

// NewQueueStore dựng store trên một DBTX bất kỳ (pool hoặc transaction).
func NewQueueStore(db gen.DBTX) *QueueStore {
	return &QueueStore{q: gen.New(db)}
}

var _ queue.Store = (*QueueStore)(nil)

// With binds the store to a transaction or dedicated advisory-lock connection.
func (s *QueueStore) With(db queue.DBTX) queue.Store {
	return NewQueueStore(db)
}

// TryAdvisoryLock chiếm khoá session-scoped của key trên chính connection
// đang giữ (caller phải giữ *pgxpool.Conn để khoá sống qua các câu lệnh sau).
func (s *QueueStore) TryAdvisoryLock(ctx context.Context, key int64) (bool, error) {
	acquired, err := s.q.TryAdvisoryLock(ctx, key)
	if err != nil {
		return false, fmt.Errorf("khoá advisory %d: %w", key, err)
	}
	return acquired, nil
}

// UnlockAdvisory nhả khoá session của key.
func (s *QueueStore) UnlockAdvisory(ctx context.Context, key int64) error {
	if err := s.q.UnlockAdvisory(ctx, key); err != nil {
		return fmt.Errorf("nhả advisory %d: %w", key, err)
	}
	return nil
}

// InsertQueueItem thêm một mục mới vào hàng.
func (s *QueueStore) InsertQueueItem(ctx context.Context, item domain.CloneQueueItem) error {
	snapshot, err := json.Marshal(item.Source)
	if err != nil {
		return fmt.Errorf("mã hoá snapshot nguồn: %w", err)
	}
	return s.q.InsertQueueItem(ctx, gen.InsertQueueItemParams{
		ID:             queueUUID(item.ID),
		ChannelID:      queueUUID(item.ChannelID),
		SourceVideoID:  item.SourceVideoID,
		SourceKey:      item.SourceKey,
		SourceSnapshot: snapshot,
		Priority:       string(item.Priority),
		Origin:         string(item.Origin),
		Position:       item.Position,
		NoImages:       item.NoImages,
		TemplateID:     optUUID(item.TemplateID),
		CreatedAt:      pgtype.Timestamptz{Time: item.CreatedAt, Valid: true},
	})
}

// GetQueueItem đọc một mục theo id; nil khi không có.
func (s *QueueStore) GetQueueItem(ctx context.Context, id string) (*domain.CloneQueueItem, error) {
	row, err := s.q.GetQueueItem(ctx, queueUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("đọc mục hàng chờ %s: %w", id, err)
	}
	item, err := queueItemOf(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListQueuedItems trả mọi mục đang chờ (trần 2000 như bản TS).
func (s *QueueStore) ListQueuedItems(ctx context.Context) ([]domain.CloneQueueItem, error) {
	rows, err := s.q.ListQueuedItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc hàng chờ: %w", err)
	}
	return queueItemsOf(rows)
}

// ListLaneItems trả mục đang chờ của một kênh, xếp theo position.
func (s *QueueStore) ListLaneItems(ctx context.Context, channelID string) ([]domain.CloneQueueItem, error) {
	rows, err := s.q.ListLaneItems(ctx, queueUUID(channelID))
	if err != nil {
		return nil, fmt.Errorf("đọc lane %s: %w", channelID, err)
	}
	return queueItemsOf(rows)
}

// UpdateQueuePosition ghi vị trí phân số mới của một mục.
func (s *QueueStore) UpdateQueuePosition(ctx context.Context, id string, position float64) error {
	if err := s.q.UpdateQueuePosition(ctx, gen.UpdateQueuePositionParams{ID: queueUUID(id), Position: position}); err != nil {
		return fmt.Errorf("dời mục %s: %w", id, err)
	}
	return nil
}

// RemoveQueueItem giữ nguyên mục chưa chạy để có thể Hoàn tác trong 24 giờ.
func (s *QueueStore) RemoveQueueItem(ctx context.Context, id string, at time.Time) (bool, error) {
	n, err := s.q.RemoveQueueItem(ctx, gen.RemoveQueueItemParams{
		ID: queueUUID(id), RemovedAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("bỏ mục hàng chờ: %w", err)
	}
	return n > 0, nil
}

// RestoreQueueItem khôi phục chính dòng cũ; nguồn đã được xếp/chạy là xung đột.
func (s *QueueStore) RestoreQueueItem(ctx context.Context, id string) (bool, error) {
	n, err := s.q.RestoreQueueItem(ctx, queueUUID(id))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return false, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video đã có trong hàng chờ"}
	}
	if err != nil {
		return false, fmt.Errorf("khôi phục mục hàng chờ: %w", err)
	}
	return n > 0, nil
}

// PruneQueueHistory giữ lịch sử 30 ngày, mục đã bỏ 24 giờ kể từ lúc bỏ.
func (s *QueueStore) PruneQueueHistory(ctx context.Context, now time.Time) error {
	if err := s.q.PruneQueueHistory(ctx, gen.PruneQueueHistoryParams{
		HistoryCutoff: pgtype.Timestamptz{Time: now.Add(-30 * 24 * time.Hour), Valid: true},
		RemovedCutoff: pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
	}); err != nil {
		return fmt.Errorf("dọn lịch sử hàng chờ: %w", err)
	}
	return nil
}

// LockOwnerChannel khoá dòng kênh trong transaction (SELECT FOR UPDATE) để
// hai lần xếp hàng cùng kênh không chèn chồng position. Lỗi khi kênh không có.
func (s *QueueStore) LockOwnerChannel(ctx context.Context, channelID string) error {
	_, err := s.q.LockQueueOwner(ctx, queueUUID(channelID))
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Kênh không tồn tại"}
	}
	return err
}

// ActiveSourceKeys trả khoá của mọi job chưa bị thay thế.
func (s *QueueStore) ActiveSourceKeys(ctx context.Context) ([]string, error) {
	return s.q.ActiveSourceKeys(ctx)
}

// QueuedSourceKeys trả khoá của mọi mục đang chờ.
func (s *QueueStore) QueuedSourceKeys(ctx context.Context) ([]string, error) {
	return s.q.QueuedSourceKeys(ctx)
}

// CountTotalQueued đếm mọi mục đang chờ.
func (s *QueueStore) CountTotalQueued(ctx context.Context) (int64, error) {
	return s.q.CountTotalQueued(ctx)
}

// LaneLastStarts trả giờ bắt đầu và job mới nhất của từng kênh đã chạy.
func (s *QueueStore) LaneLastStarts(ctx context.Context) (map[string]queue.LaneLastStart, error) {
	rows, err := s.q.LaneLastStarts(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc lượt kênh: %w", err)
	}
	out := make(map[string]queue.LaneLastStart, len(rows))
	for _, row := range rows {
		out[uuidString(row.ChannelID)] = queue.LaneLastStart{
			LastStart:    tsPtr(row.LastStart),
			LastSequence: tsPtr(row.LastSequence),
		}
	}
	return out, nil
}

// PaceUsedToday đếm mục batch đã bắt đầu từ mốc midnight truyền vào (giờ máy).
func (s *QueueStore) PaceUsedToday(ctx context.Context, midnight time.Time) (map[string]int64, error) {
	rows, err := s.q.PaceUsedToday(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("đếm nhịp hôm nay: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[uuidString(row.ChannelID)] = row.Used
	}
	return out, nil
}

// CountActiveJobs đếm job đang giữ luồng.
func (s *QueueStore) CountActiveJobs(ctx context.Context) (int, error) {
	n, err := s.q.CountActiveJobs(ctx)
	return int(n), err
}

// ListActiveJobs trả job đang giữ luồng theo thứ tự tạo.
func (s *QueueStore) ListActiveJobs(ctx context.Context) ([]domain.CloneJob, error) {
	rows, err := s.q.ListActiveJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc job đang chạy: %w", err)
	}
	jobs := make([]domain.CloneJob, len(rows))
	for i, row := range rows {
		jobs[i] = cloneJobOf(row)
	}
	return jobs, nil
}

// ListJobSteps trả các bước của một job.
func (s *QueueStore) ListJobSteps(ctx context.Context, jobID string) ([]domain.CloneStepRecord, error) {
	rows, err := s.q.ListJobSteps(ctx, queueUUID(jobID))
	if err != nil {
		return nil, fmt.Errorf("đọc bước job %s: %w", jobID, err)
	}
	steps := make([]domain.CloneStepRecord, len(rows))
	for i, row := range rows {
		steps[i] = cloneStepOf(row)
	}
	return steps, nil
}

// MarkQueueItemStarted ghi nhận mục đã thành job; false khi mục đã đổi tay.
func (s *QueueStore) MarkQueueItemStarted(ctx context.Context, id string, jobID *string, at time.Time) (bool, error) {
	n, err := s.q.MarkQueueItemStarted(ctx, gen.MarkQueueItemStartedParams{
		ID:        queueUUID(id),
		JobID:     optUUID(jobID),
		StartedAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("ghi mục bắt đầu %s: %w", id, err)
	}
	return n > 0, nil
}

// MarkQueueItemSkipped đỗ một mục kèm lý do.
func (s *QueueStore) MarkQueueItemSkipped(ctx context.Context, id, reason string) error {
	if err := s.q.MarkQueueItemSkipped(ctx, gen.MarkQueueItemSkippedParams{ID: queueUUID(id), Reason: pgtype.Text{String: reason, Valid: true}}); err != nil {
		return fmt.Errorf("đỗ mục %s: %w", id, err)
	}
	return nil
}

// ── ETA basis ──────────────────────────────────────────────────────────────

// ListRecentStepDurations trả tối đa 400 lần bước thành công gần nhất.
func (s *QueueStore) ListRecentStepDurations(ctx context.Context) ([]queue.StepDuration, error) {
	rows, err := s.q.ListRecentStepDurations(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc thời lượng bước: %w", err)
	}
	out := make([]queue.StepDuration, 0, len(rows))
	for _, row := range rows {
		var snap struct {
			DurationSec *float64 `json:"durationSec"`
		}
		_ = json.Unmarshal(row.SourceSnapshot, &snap)
		var sceneCount int
		var manifest struct {
			Data struct {
				SceneCount *int `json:"sceneCount"`
			} `json:"data"`
		}
		if len(row.OutputManifest) > 0 && json.Unmarshal(row.OutputManifest, &manifest) == nil && manifest.Data.SceneCount != nil {
			sceneCount = *manifest.Data.SceneCount
		}
		var seconds float64
		if snap.DurationSec != nil {
			seconds = *snap.DurationSec
		}
		out = append(out, queue.StepDuration{
			Step:          row.Step,
			Elapsed:       time.Duration(row.ElapsedSeconds * float64(time.Second)),
			SourceSeconds: seconds,
			SceneCount:    sceneCount,
			FinishedAt:    row.FinishedAt.Time,
		})
	}
	return out, nil
}

// ── Kế hoạch / trùng / đối thủ ─────────────────────────────────────────────

// FindJobsBySourceKeys trả job đang hoạt động theo khoá nguồn (mới nhất cuối).
func (s *QueueStore) FindJobsBySourceKeys(ctx context.Context, keys []string) ([]domain.CloneJob, error) {
	rows, err := s.q.FindJobsBySourceKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("tìm job theo khoá: %w", err)
	}
	jobs := make([]domain.CloneJob, len(rows))
	for i, row := range rows {
		jobs[i] = cloneJobOf(row)
	}
	return jobs, nil
}

// FindQueueRowsBySourceKeys trả mục queued/started theo khoá nguồn.
func (s *QueueStore) FindQueueRowsBySourceKeys(ctx context.Context, keys []string) ([]domain.CloneQueueItem, error) {
	rows, err := s.q.FindQueueRowsBySourceKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("tìm mục theo khoá: %w", err)
	}
	return queueItemsOf(rows)
}

// ListVideosByIDs trả video đối thủ kèm tên kênh + mức trung vị (cho điểm
// nổi trội của plan).
func (s *QueueStore) ListVideosByIDs(ctx context.Context, ids []string) (map[string]queue.RivalVideoView, error) {
	rows, err := s.q.ListVideosByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("đọc video đối thủ: %w", err)
	}
	out := make(map[string]queue.RivalVideoView, len(rows))
	for _, v := range rows {
		out[v.ID] = queue.RivalVideoView{
			ID:          v.ID,
			ChannelID:   v.ChannelID,
			Title:       v.Title,
			DurationSec: int32Ptr(v.DurationSec),
			ViewCount:   int64Ptr(v.ViewCount),
			PublishedAt: tsPtr(v.PublishedAt),
			HasCaptions: boolPtr(v.HasCaptions),
			Position:    v.Position,
			Origin:      v.Origin,
			FirstSeenAt: v.FirstSeenAt.Time,
		}
	}
	return out, nil
}

// ChannelInfoOf trả (title, medianViews) của các kênh đối thủ.
func (s *QueueStore) ChannelInfoOf(ctx context.Context, ids []string) (map[string]struct {
	Title       string
	MedianViews *int64
}, error,
) {
	rows, err := s.q.ListChannelsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("đọc kênh đối thủ: %w", err)
	}
	out := make(map[string]struct {
		Title       string
		MedianViews *int64
	}, len(rows))
	for _, c := range rows {
		out[c.ID] = struct {
			Title       string
			MedianViews *int64
		}{Title: c.Title, MedianViews: int64Ptr(c.MedianViews)}
	}
	return out, nil
}

// ── Tự lấy (auto-pick) ─────────────────────────────────────────────────────

// ListSourcesWithChannels trả đối thủ của một kênh theo thứ tự liên kết.
func (s *QueueStore) ListSourcesWithChannels(ctx context.Context, channelID string) ([]queue.ChannelSourceRow, error) {
	rows, err := s.q.ListSourcesWithChannels(ctx, queueUUID(channelID))
	if err != nil {
		return nil, fmt.Errorf("đọc nguồn kênh %s: %w", channelID, err)
	}
	out := make([]queue.ChannelSourceRow, len(rows))
	for i, row := range rows {
		out[i] = queue.ChannelSourceRow{
			RivalID:        row.RivalID,
			LinkedAt:       row.LinkedAt.Time,
			AutoBaselineAt: tsPtr(row.AutoBaselineAt),
			RivalScannedAt: tsPtr(row.ScannedAt),
		}
	}
	return out, nil
}

// SetSourceBaseline ghi mốc Tự lấy của một liên kết.
func (s *QueueStore) SetSourceBaseline(ctx context.Context, channelID, rivalID string, at time.Time) error {
	return s.q.SetSourceBaseline(ctx, gen.SetSourceBaselineParams{
		OwnerChannelID: queueUUID(channelID), RivalID: rivalID,
		AutoBaselineAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
}

// LastAutoQueuedChannel trả kênh đối thủ của mục auto mới nhất ("" khi chưa có).
func (s *QueueStore) LastAutoQueuedChannel(ctx context.Context, channelID string) (string, error) {
	id, err := s.q.LastAutoQueuedChannel(ctx, queueUUID(channelID))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// ListEligibleVideos trả id video ứng viên Tự lấy của một đối thủ.
func (s *QueueStore) ListEligibleVideos(ctx context.Context, rivalID string, baseline time.Time, ownerID string) ([]string, error) {
	return s.q.ListEligibleVideos(ctx, gen.ListEligibleVideosParams{
		ChannelID:   rivalID,
		FirstSeenAt: pgtype.Timestamptz{Time: baseline, Valid: true},
		Column3:     ownerID,
	})
}

// ── Đọc cho GET /api/queue + counts ────────────────────────────────────────

// ListDoneTodayJobs trả job ready/posted/review cập nhật sau midnight.
func (s *QueueStore) ListDoneTodayJobs(ctx context.Context, midnight time.Time) ([]queue.DoneTodayJob, error) {
	rows, err := s.q.ListDoneTodayJobs(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("đọc job xong hôm nay: %w", err)
	}
	out := make([]queue.DoneTodayJob, len(rows))
	for i, row := range rows {
		var snap struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal(row.SourceSnapshot, &snap)
		out[i] = queue.DoneTodayJob{
			JobID:      uuidString(row.ID),
			ChannelID:  uuidString(row.ChannelID),
			Title:      snap.Title,
			StageGroup: row.StageGroup,
			UpdatedAt:  row.UpdatedAt.Time,
		}
	}
	return out, nil
}

// ListJobStageCounts gom số job theo (kênh, stage_group).
func (s *QueueStore) ListJobStageCounts(ctx context.Context) (map[string]map[string]int64, error) {
	rows, err := s.q.ListJobStageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("đếm job theo nhóm: %w", err)
	}
	out := map[string]map[string]int64{}
	for _, row := range rows {
		owner := uuidString(row.ChannelID)
		if out[owner] == nil {
			out[owner] = map[string]int64{}
		}
		out[owner][row.StageGroup] += row.N
	}
	return out, nil
}

// ListQueueCountsByChannel đếm mục đang chờ theo kênh.
func (s *QueueStore) ListQueueCountsByChannel(ctx context.Context) (map[string]int64, error) {
	rows, err := s.q.ListQueueCountsByChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("đếm hàng theo kênh: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[uuidString(row.ChannelID)] = row.N
	}
	return out, nil
}

// ListDoneTodayCounts đếm job ready/posted hôm nay theo kênh.
func (s *QueueStore) ListDoneTodayCounts(ctx context.Context, midnight time.Time) (map[string]int64, error) {
	rows, err := s.q.ListDoneTodayCounts(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("đếm xong hôm nay: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[uuidString(row.ChannelID)] = row.N
	}
	return out, nil
}

// CountChargedImagesToday đếm ảnh trả phí đã tính từ midnight theo provider.
func (s *QueueStore) CountChargedImagesToday(ctx context.Context, midnight time.Time) (map[string]int64, error) {
	rows, err := s.q.CountChargedImagesToday(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("đếm ảnh trả phí: %w", err)
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Provider] = row.Used
	}
	return out, nil
}

// ── ánh xạ gen → domain ────────────────────────────────────────────────────

func queueUUID(id string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}
	}
	return u
}

func optUUID(id *string) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return queueUUID(*id)
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func int32Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func int64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func queueItemsOf(rows []gen.CloneQueue) ([]domain.CloneQueueItem, error) {
	items := make([]domain.CloneQueueItem, len(rows))
	for i, row := range rows {
		item, err := queueItemOf(row)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func queueItemOf(row gen.CloneQueue) (domain.CloneQueueItem, error) {
	var snap domain.CloneSourceSnapshot
	if len(row.SourceSnapshot) > 0 {
		if err := json.Unmarshal(row.SourceSnapshot, &snap); err != nil {
			return domain.CloneQueueItem{}, fmt.Errorf("snapshot nguồn mục %s: %w", row.SourceVideoID, err)
		}
	}
	var jobID *string
	if row.JobID.Valid {
		id := uuidString(row.JobID)
		jobID = &id
	}
	var reason *string
	if row.Reason.Valid {
		reason = &row.Reason.String
	}
	return domain.CloneQueueItem{
		ID:            uuidString(row.ID),
		ChannelID:     uuidString(row.ChannelID),
		SourceVideoID: row.SourceVideoID,
		SourceKey:     row.SourceKey,
		Source:        snap,
		Priority:      domain.ClonePriority(row.Priority),
		Origin:        domain.CloneOrigin(row.Origin),
		Position:      row.Position,
		State:         domain.CloneQueueState(row.State),
		NoImages:      row.NoImages,
		TemplateID:    queueUUIDPtr(row.TemplateID),
		JobID:         jobID,
		Reason:        reason,
		CreatedAt:     row.CreatedAt.Time,
		StartedAt:     tsPtr(row.StartedAt),
		RemovedAt:     tsPtr(row.RemovedAt),
	}, nil
}

func cloneJobOf(row gen.CloneJob) domain.CloneJob {
	var snap domain.CloneSourceSnapshot
	if len(row.SourceSnapshot) > 0 {
		_ = json.Unmarshal(row.SourceSnapshot, &snap)
	}
	var active *domain.CloneStep
	if row.ActiveStep.Valid {
		step := domain.CloneStep(row.ActiveStep.String)
		active = &step
	}
	return domain.CloneJob{
		ID:                uuidString(row.ID),
		ChannelID:         uuidString(row.ChannelID),
		SourceKey:         row.SourceKey,
		Source:            snap,
		TemplateID:        uuidString(row.TemplateID),
		TemplateSnapshot:  row.TemplateSnapshot,
		ModelRefs:         row.ModelRefs,
		NoImages:          row.NoImages,
		Generation:        int(row.Generation),
		SupersededAt:      tsPtr(row.SupersededAt),
		PausedAt:          tsPtr(row.PausedAt),
		CancelRequestedAt: tsPtr(row.CancelRequestedAt),
		Priority:          domain.ClonePriority(row.Priority),
		Origin:            domain.CloneOrigin(row.Origin),
		Status:            domain.CloneStatus(row.Status),
		StageGroup:        row.StageGroup,
		ActiveStep:        active,
		Fence:             row.Fence,
		Version:           row.Version,
		PostedAt:          tsPtr(row.PostedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func cloneStepOf(row gen.CloneStep) domain.CloneStepRecord {
	var progress domain.CloneProgress
	progress.UnitsDone = row.ProgressDone
	if row.ProgressTotal.Valid {
		progress.UnitsTotal = &row.ProgressTotal.Int64
	}
	progress.Phase = row.ProgressPhase
	var output *domain.CloneStepOutput
	if len(row.OutputManifest) > 0 {
		var out domain.CloneStepOutput
		if json.Unmarshal(row.OutputManifest, &out) == nil {
			output = &out
		}
	}
	var cerr *domain.CloneError
	if len(row.Error) > 0 {
		var e domain.CloneError
		if json.Unmarshal(row.Error, &e) == nil {
			cerr = &e
		}
	}
	var pg *int
	if row.ProcessGroup.Valid {
		v := int(row.ProcessGroup.Int32)
		pg = &v
	}
	return domain.CloneStepRecord{
		JobID:        uuidString(row.JobID),
		Step:         domain.CloneStep(row.Step),
		Status:       domain.CloneStatus(row.Status),
		Attempt:      int(row.Attempt),
		Progress:     progress,
		Checkpoint:   row.Checkpoint,
		Output:       output,
		Error:        cerr,
		Fence:        row.Fence,
		ProcessGroup: pg,
		StartedAt:    tsPtr(row.StartedAt),
		FinishedAt:   tsPtr(row.FinishedAt),
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func (s *QueueStore) Owners(ctx context.Context) ([]domain.Channel, error) {
	rows, err := s.q.ListOwnerChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Channel, len(rows))
	for i, row := range rows {
		out[i] = ownerChannelFromRow(row)
	}
	return out, nil
}

func (s *QueueStore) Owner(ctx context.Context, id string) (*domain.Channel, error) {
	row, err := s.q.GetOwnerChannel(ctx, queueUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v := ownerChannelFromRow(row)
	return &v, nil
}

func (s *QueueStore) Templates(ctx context.Context, channelID string) ([]domain.ChannelTemplate, error) {
	rows, err := s.q.ListChannelTemplates(ctx, []pgtype.UUID{queueUUID(channelID)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChannelTemplate, len(rows))
	for i, row := range rows {
		out[i] = domain.ChannelTemplate{ID: uuidString(row.TemplateID), Name: row.Name, Position: int(row.Position)}
	}
	return out, nil
}

func (s *QueueStore) TemplateRecipe(ctx context.Context, id string) (json.RawMessage, error) {
	row, err := s.q.GetTemplate(ctx, queueUUID(id))
	return row.Recipe, err
}

func queueUUIDPtr(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	value := uuidString(id)
	return &value
}

func (s *QueueStore) PromotePick(ctx context.Context, id string, position float64) error {
	return s.q.PromoteQueuePick(ctx, gen.PromoteQueuePickParams{ID: queueUUID(id), Position: position})
}
