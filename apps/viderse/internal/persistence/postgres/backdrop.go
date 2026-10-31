package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// BackdropStore là kho của thư viện Video nền trên backdrop_channels,
// backdrop_channel_videos, backdrop_videos; đọc recipe của templates và
// platform_requests để dựng usedBy và ghi yêu cầu hỗ trợ.
type BackdropStore struct {
	q  *gen.Queries
	db DBTXer
}

// NewBackdropStore dựng store trên pool/connection (cần Begin được).
func NewBackdropStore(db DBTXer) *BackdropStore {
	return &BackdropStore{q: gen.New(db), db: db}
}

var _ backdrop.Store = (*BackdropStore)(nil)

// WithTx mở transaction và chạy fn trên store gắn transaction.
func (s *BackdropStore) WithTx(ctx context.Context, fn func(store backdrop.Store) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(&BackdropStore{q: s.q.WithTx(tx), db: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *BackdropStore) Channel(ctx context.Context, id string) (domain.BackdropChannel, bool, error) {
	row, err := s.q.GetBackdropChannel(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return domain.BackdropChannel{}, false, nil
		}
		return domain.BackdropChannel{}, false, err
	}
	return backdropChannelFromRow(row), true, nil
}

func (s *BackdropStore) Channels(ctx context.Context) ([]domain.BackdropChannel, error) {
	rows, err := s.q.ListBackdropChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BackdropChannel, 0, len(rows))
	for _, row := range rows {
		out = append(out, backdropChannelFromRow(row))
	}
	return out, nil
}

func (s *BackdropStore) InsertChannel(ctx context.Context, channel domain.BackdropChannel) (bool, error) {
	created, err := s.q.InsertBackdropChannel(ctx, gen.InsertBackdropChannelParams{
		ID:            channel.ID,
		Platform:      string(channel.Platform),
		Handle:        channel.Handle,
		Url:           channel.URL,
		Title:         optText(channel.Title),
		AvatarUrl:     optText(channel.AvatarURL),
		AddedAt:       ts(channel.AddedAt),
		ListedAt:      optTime(channel.ListedAt),
		ListErrorCode: optText(nilIfEmpty(channel.ListErrorCode)),
		ListError:     optText(nilIfEmpty(channel.ListError)),
		ScanNextPage:  optInt4(channel.ScanNextPage),
		ScanTotal:     int32(channel.ScanTotal),
		ScanStopKnown: channel.ScanStopKnown,
	})
	if err != nil {
		return false, err
	}
	return created > 0, nil
}

func (s *BackdropStore) UpdateChannelList(ctx context.Context, id string, now time.Time, errCode, errMessage string) error {
	if errCode == "" {
		return s.q.MarkBackdropChannelListed(ctx, gen.MarkBackdropChannelListedParams{ID: id, ListedAt: ts(now)})
	}
	return s.q.SetBackdropChannelListError(ctx, gen.SetBackdropChannelListErrorParams{ID: id, Code: text(errCode), Message: text(errMessage)})
}

func (s *BackdropStore) UpdateChannelScan(ctx context.Context, id string, nextPage *int, stopKnown bool, total int, title *string) error {
	return s.q.UpdateBackdropChannelScan(ctx, gen.UpdateBackdropChannelScanParams{
		NextPage: optInt4(nextPage), StopKnown: stopKnown, Total: int32(total), Title: optText(title), ID: id,
	})
}

func (s *BackdropStore) UpdateChannelTitle(ctx context.Context, id string, title string) error {
	return s.q.UpdateBackdropChannelTitle(ctx, gen.UpdateBackdropChannelTitleParams{Title: text(title), ID: id})
}

func (s *BackdropStore) DeleteChannel(ctx context.Context, id string) (bool, error) {
	removed, err := s.q.DeleteBackdropChannel(ctx, id)
	if err != nil {
		return false, err
	}
	return removed > 0, nil
}

func (s *BackdropStore) ChannelVideos(ctx context.Context, channelID string) ([]domain.BackdropChannelVideo, error) {
	rows, err := s.q.ListBackdropChannelVideos(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BackdropChannelVideo, 0, len(rows))
	for _, row := range rows {
		out = append(out, channelVideoFromRow(row))
	}
	return out, nil
}

func (s *BackdropStore) ChannelVideo(ctx context.Context, channelID, videoID string) (domain.BackdropChannelVideo, bool, error) {
	row, err := s.q.GetBackdropChannelVideo(ctx, gen.GetBackdropChannelVideoParams{ChannelID: channelID, VideoID: videoID})
	if err != nil {
		if isNoRows(err) {
			return domain.BackdropChannelVideo{}, false, nil
		}
		return domain.BackdropChannelVideo{}, false, err
	}
	return channelVideoFromRow(row), true, nil
}

// listingVideoJSON là hình jsonb_to_recordset của một dòng danh mục.
type listingVideoJSON struct {
	VideoID      string   `json:"video_id"`
	Kind         string   `json:"kind"`
	URL          string   `json:"url"`
	Title        *string  `json:"title"`
	DurationSec  *float64 `json:"duration_sec"`
	ThumbnailURL *string  `json:"thumbnail_url"`
	PublishedAt  *string  `json:"published_at"`
	Position     int      `json:"position"`
}

func (s *BackdropStore) UpsertVideos(ctx context.Context, channelID string, videos []domain.BackdropChannelVideo) error {
	if len(videos) == 0 {
		return nil
	}
	rows := make([]listingVideoJSON, 0, len(videos))
	for _, video := range videos {
		title := video.Title
		row := listingVideoJSON{
			VideoID: video.VideoID, Kind: video.Kind, URL: video.URL,
			Title: &title, DurationSec: video.DurationSec, ThumbnailURL: video.ThumbnailURL,
			Position: video.Position,
		}
		if video.PublishedAt != nil {
			value := video.PublishedAt.UTC().Format(time.RFC3339Nano)
			row.PublishedAt = &value
		}
		rows = append(rows, row)
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return s.q.UpsertBackdropChannelVideos(ctx, gen.UpsertBackdropChannelVideosParams{
		ChannelID: channelID,
		Videos:    payload,
		ListedAt:  ts(videos[0].ListedAt),
	})
}

func (s *BackdropStore) ExpireMissingVods(ctx context.Context, channelID string, keep []string, now time.Time) error {
	return s.q.ExpireMissingBackdropVods(ctx, gen.ExpireMissingBackdropVodsParams{ChannelID: channelID, Keep: keep, Now: ts(now)})
}

func (s *BackdropStore) MarkChannelVideoUnavailable(ctx context.Context, channelID, videoID, reason string, now time.Time) error {
	return s.q.MarkBackdropChannelVideoUnavailable(ctx, gen.MarkBackdropChannelVideoUnavailableParams{
		ChannelID: channelID, VideoID: videoID, Reason: text(reason), Now: ts(now),
	})
}

func (s *BackdropStore) UpdateChannelVideoProbe(ctx context.Context, channelID, videoID string, probe backdrop.ProbeUpdate) error {
	return s.q.UpdateBackdropChannelVideoProbe(ctx, gen.UpdateBackdropChannelVideoProbeParams{
		ChannelID: channelID, VideoID: videoID,
		Title: optText(probe.Title), DurationSec: optFloat4(probe.DurationSec),
		ThumbnailUrl: optText(probe.ThumbnailURL), PublishedAt: optTime(probe.PublishedAt),
		ProbedAt: ts(probe.ProbedAt), ProbeInfo: probe.ProbeInfo, ClearUnavailable: probe.ClearUnavailable,
	})
}

func (s *BackdropStore) UsableCounts(ctx context.Context, channelIDs []string) (map[string]map[domain.PlatformKind]int, error) {
	out := make(map[string]map[domain.PlatformKind]int, len(channelIDs))
	if len(channelIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.UsableBackdropCounts(ctx, channelIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		byKind, ok := out[row.ChannelID]
		if !ok {
			byKind = map[domain.PlatformKind]int{}
			out[row.ChannelID] = byKind
		}
		byKind[row.Kind] = int(row.Total)
	}
	return out, nil
}

func (s *BackdropStore) PageChannelVideos(ctx context.Context, query backdrop.PageQuery) ([]domain.BackdropChannelVideo, error) {
	kinds := make([]string, 0, len(query.Kinds))
	for _, kind := range query.Kinds {
		kinds = append(kinds, kind)
	}
	params := gen.PageBackdropChannelVideosParams{
		ChannelID:  query.ChannelID,
		Kinds:      kinds,
		LimitCount: int32(query.Limit),
	}
	if query.Cursor != nil {
		params.HasCursor = true
		params.CursorVideo = query.Cursor.VideoID
		params.CursorPublished = optTime(query.Cursor.PublishedAt)
	}
	rows, err := s.q.PageBackdropChannelVideos(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BackdropChannelVideo, 0, len(rows))
	for _, row := range rows {
		out = append(out, channelVideoFromRow(row))
	}
	return out, nil
}

func (s *BackdropStore) CountChannelVideos(ctx context.Context, channelID string, kinds []string) (int, error) {
	total, err := s.q.CountBackdropChannelVideos(ctx, gen.CountBackdropChannelVideosParams{ChannelID: channelID, Kinds: nonNilStrings(kinds)})
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

func (s *BackdropStore) CountUnavailableVideos(ctx context.Context, channelID string, kinds []string) (int, error) {
	total, err := s.q.CountUnavailableBackdropChannelVideos(ctx, gen.CountUnavailableBackdropChannelVideosParams{ChannelID: channelID, Kinds: nonNilStrings(kinds)})
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

// nonNilStrings đổi nil thành mảng rỗng: truy vấn lọc so cardinality(kinds)=0
// nên tham số NULL sẽ lọc sạch mọi dòng.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (s *BackdropStore) Videos(ctx context.Context) ([]domain.BackdropVideo, error) {
	rows, err := s.q.ListBackdropVideos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.BackdropVideo, 0, len(rows))
	for _, row := range rows {
		out = append(out, backdropVideoFromRow(row))
	}
	return out, nil
}

func (s *BackdropStore) Video(ctx context.Context, id string) (domain.BackdropVideo, bool, error) {
	row, err := s.q.GetBackdropVideo(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return domain.BackdropVideo{}, false, nil
		}
		return domain.BackdropVideo{}, false, err
	}
	return backdropVideoFromRow(row), true, nil
}

func (s *BackdropStore) InsertVideo(ctx context.Context, video domain.BackdropVideo) (bool, error) {
	created, err := s.q.InsertBackdropVideo(ctx, gen.InsertBackdropVideoParams{
		ID: video.ID, Platform: string(video.Platform), Url: video.URL, Title: video.Title,
		DurationSec:    optFloat4(video.DurationSec),
		SegmentFromSec: float32(video.Segment.FromSec),
		SegmentToSec:   float32(video.Segment.ToSec),
		FilePath:       optText(nilIfEmpty(video.FilePath)),
		FileHeight:     optInt4(intPtrOrNil(video.FileHeight)),
		FileBytes:      optInt8(video.FileBytes),
		State:          video.State,
		Progress:       optFloat4(video.Progress),
		ErrorCode:      text(video.ErrorCode),
		Error:          text(video.Error),
		AddedAt:        ts(video.AddedAt),
		UpdatedAt:      ts(video.UpdatedAt),
	})
	if err != nil {
		return false, err
	}
	return created > 0, nil
}

func (s *BackdropStore) UpdateVideo(ctx context.Context, id string, patch backdrop.VideoPatch) error {
	params := gen.UpdateBackdropVideoParams{
		ID:            id,
		ClearProgress: patch.ClearProgress,
		Progress:      optFloat4(patch.Progress),
		FilePath:      optText(patch.FilePath),
		FileHeight:    optInt4(patch.FileHeight),
		FileBytes:     optInt8Ptr(patch.FileBytes),
		ErrorCode:     optText(patch.ErrorCode),
		Error:         optText(patch.Error),
		Title:         optText(patch.Title),
		DurationSec:   optFloat4(patch.DurationSec),
		UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if patch.State != nil {
		params.State = text(string(*patch.State))
	}
	if patch.Segment != nil {
		params.SegmentFromSec = pgtype.Float4{Float32: float32(patch.Segment.FromSec), Valid: true}
		params.SegmentToSec = pgtype.Float4{Float32: float32(patch.Segment.ToSec), Valid: true}
	}
	return s.q.UpdateBackdropVideo(ctx, params)
}

func (s *BackdropStore) DeleteVideo(ctx context.Context, id string) (bool, error) {
	removed, err := s.q.DeleteBackdropVideo(ctx, id)
	if err != nil {
		return false, err
	}
	return removed > 0, nil
}

// ClaimDownload giành lượt tải cho một video lẻ.
func (s *BackdropStore) ClaimDownload(ctx context.Context, id, owner string, leaseUntil, now time.Time, force bool) (int64, bool, error) {
	generation, err := s.q.ClaimBackdropVideoDownload(ctx, gen.ClaimBackdropVideoDownloadParams{
		ID: id, Owner: text(owner), LeaseUntil: ts(leaseUntil), Now: ts(now), Force: force,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("giành lượt tải %s: %w", id, err)
	}
	return generation, true, nil
}

// TouchDownload ghi tiến độ và gia hạn lease của lượt tải.
func (s *BackdropStore) TouchDownload(ctx context.Context, id, owner string, generation int64, progress *float64, leaseUntil, now time.Time) (bool, error) {
	affected, err := s.q.TouchBackdropVideoDownload(ctx, gen.TouchBackdropVideoDownloadParams{
		ID: id, Owner: text(owner), Generation: generation, Progress: float4Of(progress), LeaseUntil: ts(leaseUntil), Now: ts(now),
	})
	if err != nil {
		return false, fmt.Errorf("ghi tiến độ %s: %w", id, err)
	}
	return affected > 0, nil
}

// FinishDownload ghi trạng thái ready khi còn đúng chủ lượt tải.
func (s *BackdropStore) FinishDownload(ctx context.Context, id, owner string, generation int64, filePath string, height int, size int64, now time.Time) (bool, error) {
	affected, err := mediaQueries(ctx, s.q).FinishBackdropVideoDownload(ctx, gen.FinishBackdropVideoDownloadParams{
		ID: id, Owner: text(owner), Generation: generation,
		FilePath: text(filePath), FileHeight: int4Scan(height), FileBytes: int8Scan(size), Now: ts(now),
	})
	if err != nil {
		return false, fmt.Errorf("chốt tải %s: %w", id, err)
	}
	return affected > 0, nil
}

// FailDownload ghi trạng thái failed khi còn đúng chủ lượt tải.
func (s *BackdropStore) FailDownload(ctx context.Context, id, owner string, generation int64, code, message string, now time.Time) (bool, error) {
	affected, err := s.q.FailBackdropVideoDownload(ctx, gen.FailBackdropVideoDownloadParams{
		ID: id, Owner: text(owner), Generation: generation,
		ErrorCode: text(code), Error: text(message), Now: ts(now),
	})
	if err != nil {
		return false, fmt.Errorf("ghi lỗi tải %s: %w", id, err)
	}
	return affected > 0, nil
}

// ClearStaleDownloads đánh dấu các lượt tải mồ côi sau khi process chết.
func (s *BackdropStore) ClearStaleDownloads(ctx context.Context, now time.Time) (int, error) {
	affected, err := s.q.ClearStaleBackdropVideoDownloads(ctx, ts(now))
	if err != nil {
		return 0, fmt.Errorf("dọn lượt tải mồ côi: %w", err)
	}
	return int(affected), nil
}

// LockResource giữ khoá transaction theo tài nguyên (advisory lock).
func (s *BackdropStore) LockResource(ctx context.Context, key string) error {
	if err := s.q.LockBackdropResource(ctx, key); err != nil {
		return fmt.Errorf("giữ khoá %s: %w", key, err)
	}
	return nil
}

// NotifyChange phát NOTIFY để process khác đánh thức bộ lập lịch.
func (s *BackdropStore) NotifyChange(ctx context.Context, key string) error {
	if err := s.q.NotifyBackdropChange(ctx, key); err != nil {
		return fmt.Errorf("phát thay đổi %s: %w", key, err)
	}
	return nil
}

func int4Scan(v int) pgtype.Int4   { return pgtype.Int4{Int32: int32(v), Valid: true} }
func int8Scan(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }

func float4Of(v *float64) pgtype.Float4 {
	if v == nil {
		return pgtype.Float4{}
	}
	return pgtype.Float4{Float32: float32(*v), Valid: true}
}

func (s *BackdropStore) Templates(ctx context.Context) ([]backdrop.TemplateRecipeRow, error) {
	rows, err := s.q.ListTemplatesForBackdrop(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backdrop.TemplateRecipeRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, backdrop.TemplateRecipeRow{ID: row.ID, Name: row.Name, Recipe: []byte(row.Recipe)})
	}
	return out, nil
}

func (s *BackdropStore) RecordPlatformRequest(ctx context.Context, host, label string, now time.Time) (backdrop.PlatformRequestRecord, error) {
	row, err := s.q.RecordPlatformRequest(ctx, gen.RecordPlatformRequestParams{Host: host, Label: label, Now: ts(now)})
	if err != nil {
		return backdrop.PlatformRequestRecord{}, err
	}
	return platformRequestFromRow(row), nil
}

func (s *BackdropStore) PlatformRequests(ctx context.Context) ([]backdrop.PlatformRequestRecord, error) {
	rows, err := s.q.ListPlatformRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backdrop.PlatformRequestRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, platformRequestFromRow(row))
	}
	return out, nil
}

// ── chuyển kiểu ────────────────────────────────────────────────────────────

func backdropChannelFromRow(row gen.BackdropChannel) domain.BackdropChannel {
	channel := domain.BackdropChannel{
		ID:            row.ID,
		Platform:      domain.PlatformID(row.Platform),
		Handle:        row.Handle,
		URL:           row.Url,
		Title:         textPtr(row.Title),
		AvatarURL:     textPtr(row.AvatarUrl),
		AddedAt:       row.AddedAt.Time,
		ListedAt:      timePtr(row.ListedAt),
		ScanTotal:     int(row.ScanTotal),
		ScanStopKnown: row.ScanStopKnown,
	}
	channel.ListErrorCode = valueOfText(row.ListErrorCode)
	channel.ListError = valueOfText(row.ListError)
	if row.ScanNextPage.Valid {
		next := int(row.ScanNextPage.Int32)
		channel.ScanNextPage = &next
	}
	return channel
}

func channelVideoFromRow(row gen.BackdropChannelVideo) domain.BackdropChannelVideo {
	video := domain.BackdropChannelVideo{
		ChannelID:     row.ChannelID,
		VideoID:       row.VideoID,
		Kind:          row.Kind,
		URL:           row.Url,
		Title:         row.Title,
		DurationSec:   floatPtr4(row.DurationSec),
		ThumbnailURL:  textPtr(row.ThumbnailUrl),
		PublishedAt:   timePtr(row.PublishedAt),
		Position:      int(row.Position),
		ListedAt:      row.ListedAt.Time,
		ProbedAt:      timePtr(row.ProbedAt),
		ProbeInfo:     row.ProbeInfo,
		UnavailableAt: timePtr(row.UnavailableAt),
	}
	video.UnavailableReason = valueOfText(row.UnavailableReason)
	return video
}

func backdropVideoFromRow(row gen.BackdropVideo) domain.BackdropVideo {
	return domain.BackdropVideo{
		ID:          row.ID,
		Platform:    domain.PlatformID(row.Platform),
		URL:         row.Url,
		Title:       row.Title,
		DurationSec: floatPtr4(row.DurationSec),
		Segment: domain.BackdropSegment{
			FromSec: float64(row.SegmentFromSec),
			ToSec:   float64(row.SegmentToSec),
		},
		FilePath:   valueOfText(row.FilePath),
		FileHeight: int(valueOfInt4(row.FileHeight)),
		FileBytes:  valueOfInt8(row.FileBytes),
		State:      domain.BackdropVideoState(row.State),
		Progress:   floatPtr4(row.Progress),
		ErrorCode:  valueOfText(row.ErrorCode),
		Error:      valueOfText(row.Error),
		AddedAt:    row.AddedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,

		DownloadGeneration: row.DownloadGeneration,
		DownloadOwner:      textPtr(row.DownloadOwner),
		DownloadLeaseUntil: timePtr(row.DownloadLeaseUntil),
	}
}

func platformRequestFromRow(row gen.PlatformRequest) backdrop.PlatformRequestRecord {
	return backdrop.PlatformRequestRecord{
		Host: row.Host, Label: row.Label, Count: int(row.Count),
		FirstAt: row.FirstAt.Time, LastAt: row.LastAt.Time,
	}
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return ts(*value)
}

func optFloat4(value *float64) pgtype.Float4 {
	if value == nil {
		return pgtype.Float4{}
	}
	return pgtype.Float4{Float32: float32(*value), Valid: true}
}

func floatPtr4(value pgtype.Float4) *float64 {
	if !value.Valid {
		return nil
	}
	out := float64(value.Float32)
	return &out
}

func optInt4(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func valueOfInt4(value pgtype.Int4) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int32)
}

func optInt8(value int64) pgtype.Int8 {
	if value == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func optInt8Ptr(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func valueOfInt8(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func valueOfText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func intPtrOrNil(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}
