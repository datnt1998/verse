package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
)

// RivalsStore hiện thực rivals.Store trên rival_channels/rival_videos/
// scan_state/scan_jobs qua code sqlc — chỉ đọc. Ghi của luồng quét nằm ở
// ScanStore (queries/scan.sql).
type RivalsStore struct {
	q *gen.Queries
}

// NewRivalsStore dựng store trên một DBTX bất kỳ (pool hoặc transaction).
func NewRivalsStore(db gen.DBTX) *RivalsStore {
	return &RivalsStore{q: gen.New(db)}
}

var _ rivals.Store = (*RivalsStore)(nil)

// epochFloor là mốc "không có" của scan_schedule — epoch nghĩa là không nghỉ.
var epochFloor = time.Unix(0, 0).UTC()

// ListRivals đọc toàn bộ sidebar trong một truy vấn (xem queries/rivals.sql).
func (s *RivalsStore) ListRivals(ctx context.Context, now, newSince time.Time) ([]rivals.ListEntry, error) {
	rows, err := s.q.ListRivals(ctx, gen.ListRivalsParams{
		OutlierMin: domain.OutlierMinScore,
		NewSince:   ts(newSince),
		Now:        ts(now),
		Platform:   domain.PlatformYouTube,
	})
	if err != nil {
		return nil, err
	}
	out := make([]rivals.ListEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, rivals.ListEntry{
			Channel:          rivalChannelFromRow(r),
			VideoCount:       r.VideoCount,
			Outliers:         r.Outliers,
			NewOutliers:      r.NewOutliers,
			HistoryState:     domain.HistoryState(r.HistoryState),
			DiscoveryRunning: r.DiscoveryRunning,
		})
	}
	return out, nil
}

// RivalExists báo id có phải kênh đối thủ không.
func (s *RivalsStore) RivalExists(ctx context.Context, channelID string) (bool, error) {
	return s.q.RivalExists(ctx, channelID)
}

// CountVideos đếm video khớp bộ lọc.
func (s *RivalsStore) CountVideos(ctx context.Context, q rivals.PageQuery) (int64, error) {
	return s.q.CountRivalVideos(ctx, gen.CountRivalVideosParams{
		ChannelID:  channelArg(q.ChannelID),
		LongOnly:   q.Long,
		LongMinSec: domain.LongFormMinSec,
		WithinDays: int32(q.WithinDays),
		Now:        ts(q.Now),
		Q:          q.Pattern,
		MadeFor:    channelArg(q.MadeFor),
		NotMadeFor: channelArg(q.NotMadeFor),
		MadeAny:    q.MadeAny,
		NotMadeAny: q.NotMadeAny,
	})
}

// PageVideos trả một trang theo q.Sort; keyset của trang trước nằm trong
// q.Cursor (nil = trang đầu).
func (s *RivalsStore) PageVideos(ctx context.Context, q rivals.PageQuery) ([]domain.RivalVideo, error) {
	switch {
	case q.Sort == rivals.SortViews:
		rows, err := s.q.PageRivalVideosByViews(ctx, gen.PageRivalVideosByViewsParams{
			ChannelID:   channelArg(q.ChannelID),
			LongOnly:    q.Long,
			LongMinSec:  domain.LongFormMinSec,
			WithinDays:  int32(q.WithinDays),
			Now:         ts(q.Now),
			Q:           q.Pattern,
			MadeFor:     channelArg(q.MadeFor),
			NotMadeFor:  channelArg(q.NotMadeFor),
			MadeAny:     q.MadeAny,
			NotMadeAny:  q.NotMadeAny,
			CursorID:    cursorID(q.Cursor),
			CursorViews: cursorInt8(func(c *rivals.Cursor) *int64 { return c.Views }, q.Cursor),
			PageSize:    int64(q.Limit),
		})
		return videosFrom(rows, err, func(r gen.PageRivalVideosByViewsRow) pageRow { return pageRow(r) })
	case q.Sort == rivals.SortNewest && q.ChannelID == "":
		rows, err := s.q.PageAllRivalVideosByNewest(ctx, gen.PageAllRivalVideosByNewestParams{
			LongOnly:   q.Long,
			LongMinSec: domain.LongFormMinSec,
			WithinDays: int32(q.WithinDays),
			Now:        ts(q.Now),
			Q:          q.Pattern,
			MadeFor:    channelArg(q.MadeFor),
			NotMadeFor: channelArg(q.NotMadeFor),
			MadeAny:    q.MadeAny,
			NotMadeAny: q.NotMadeAny,
			CursorID:   cursorID(q.Cursor),
			CursorTs:   cursorTS(q.Cursor),
			PageSize:   int64(q.Limit),
		})
		return videosFrom(rows, err, func(r gen.PageAllRivalVideosByNewestRow) pageRow { return pageRow(r) })
	case q.Sort == rivals.SortNewest:
		rows, err := s.q.PageRivalVideosByPosition(ctx, gen.PageRivalVideosByPositionParams{
			ChannelID:  q.ChannelID,
			LongOnly:   q.Long,
			LongMinSec: domain.LongFormMinSec,
			WithinDays: int32(q.WithinDays),
			Now:        ts(q.Now),
			Q:          q.Pattern,
			MadeFor:    channelArg(q.MadeFor),
			NotMadeFor: channelArg(q.NotMadeFor),
			MadeAny:    q.MadeAny,
			NotMadeAny: q.NotMadeAny,
			CursorID:   cursorID(q.Cursor),
			CursorPos:  cursorInt8(func(c *rivals.Cursor) *int64 { return c.Pos }, q.Cursor),
			PageSize:   int64(q.Limit),
		})
		return videosFrom(rows, err, func(r gen.PageRivalVideosByPositionRow) pageRow { return pageRow(r) })
	default:
		rows, err := s.q.PageRivalVideosByOutlier(ctx, gen.PageRivalVideosByOutlierParams{
			ChannelID:   channelArg(q.ChannelID),
			LongOnly:    q.Long,
			LongMinSec:  domain.LongFormMinSec,
			WithinDays:  int32(q.WithinDays),
			Now:         ts(q.Now),
			Q:           q.Pattern,
			MadeFor:     channelArg(q.MadeFor),
			NotMadeFor:  channelArg(q.NotMadeFor),
			MadeAny:     q.MadeAny,
			NotMadeAny:  q.NotMadeAny,
			CursorID:    cursorID(q.Cursor),
			CursorScore: cursorFloat8(q.Cursor),
			PageSize:    int64(q.Limit),
		})
		return videosFrom(rows, err, func(r pageRow) pageRow { return r })
	}
}

// pageRow là hàng của bốn truy vấn trang. Chúng chọn cùng danh sách cột (bỏ
// title_fold chỉ dùng để tìm) nên sqlc sinh bốn kiểu cùng trường, đổi kiểu
// trực tiếp được.
type pageRow = gen.PageRivalVideosByOutlierRow

func videosFrom[R any](rows []R, err error, conv func(R) pageRow) ([]domain.RivalVideo, error) {
	if err != nil {
		return nil, err
	}
	out := make([]domain.RivalVideo, 0, len(rows))
	for _, r := range rows {
		out = append(out, rivalVideoFromRow(conv(r)))
	}
	return out, nil
}

// PostingSample đọc mẫu giờ đăng của một kênh.
func (s *RivalsStore) PostingSample(ctx context.Context, channelID string, sample int) ([]rivals.PostingSampleRow, error) {
	rows, err := s.q.ListPostingSample(ctx, gen.ListPostingSampleParams{
		ChannelID: channelID,
		Sample:    int64(sample),
	})
	if err != nil {
		return nil, err
	}
	out := make([]rivals.PostingSampleRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, rivals.PostingSampleRow{
			UploadedAt: timePtr(r.UploadedAt),
			Checked:    r.UploadCheckedAt.Valid,
		})
	}
	return out, nil
}

// UploadProbePausedUntil trả giờ tạm nghỉ đọc giờ đăng; epoch hoặc quá khứ
// nghĩa là không nghỉ (caller so với now khi cần — store giữ nguyên giá trị).
func (s *RivalsStore) UploadProbePausedUntil(ctx context.Context) (*time.Time, error) {
	until, err := s.q.UploadProbePausedUntil(ctx)
	if err != nil {
		return nil, err
	}
	if !until.Valid || until.Time.Equal(epochFloor) {
		return nil, nil
	}
	t := until.Time
	return &t, nil
}

// CountRivalChannels đếm kênh đối thủ đang theo dõi.
func (s *RivalsStore) CountRivalChannels(ctx context.Context) (int64, error) {
	return s.q.CountRivalChannels(ctx)
}

// OwnerChannelExists báo id có phải kênh của bạn không.
func (s *RivalsStore) OwnerChannelExists(ctx context.Context, channelID string) (bool, error) {
	return s.q.OwnerChannelExists(ctx, channelID)
}

// VideoUsage đọc lượt làm thật của các video trong một trang.
func (s *RivalsStore) VideoUsage(ctx context.Context, videoIDs []string) ([]domain.VideoUsage, error) {
	rows, err := s.q.ListVideoUsage(ctx, videoIDs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.VideoUsage, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.VideoUsage{
			VideoID:     r.VideoID,
			ChannelID:   uuidStr(r.ChannelID),
			ChannelName: r.ChannelName,
			Stage:       r.StageGroup,
			ActiveStep:  r.ActiveStep,
			JobID:       uuidStr(r.JobID),
			QueueItemID: uuidStr(r.QueueItemID),
			UpdatedAt:   r.UpdatedAt.Time,
		})
	}
	return out, nil
}

// AutoFetchBaselines đọc mốc bật Tự lấy của các cặp kênh–đối thủ đã liên kết.
func (s *RivalsStore) AutoFetchBaselines(ctx context.Context) ([]rivals.AutoFetchBaseline, error) {
	rows, err := s.q.ListAutoFetchBaselines(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]rivals.AutoFetchBaseline, 0, len(rows))
	for _, r := range rows {
		out = append(out, rivals.AutoFetchBaseline{
			ChannelID:   uuidStr(r.OwnerChannelID),
			ChannelName: r.Name,
			RivalID:     r.RivalID,
			AutoFetch:   r.AutoFetch,
			BaselineAt:  r.AutoBaselineAt.Time,
		})
	}
	return out, nil
}

// RivalVideoChannel trả kênh đối thủ của một video.
func (s *RivalsStore) RivalVideoChannel(ctx context.Context, videoID string) (string, bool, error) {
	channel, err := s.q.RivalVideoChannel(ctx, videoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return channel, true, nil
}

// SourceInfoArtifactPath trả đường dẫn download/source.info.json đã công bố.
func (s *RivalsStore) SourceInfoArtifactPath(ctx context.Context, videoID string) (string, bool, error) {
	path, err := s.q.SourceInfoArtifactPath(ctx, videoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

// ── chuyển kiểu ────────────────────────────────────────────────────────────

// channelArg gói channel id có-thể-rỗng của một truy vấn (rỗng = mọi kênh).
func channelArg(channelID string) pgtype.Text {
	if channelID == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: channelID, Valid: true}
}

func cursorID(c *rivals.Cursor) pgtype.Text {
	if c == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: c.ID, Valid: true}
}

func cursorFloat8(c *rivals.Cursor) pgtype.Float8 {
	if c == nil || c.Score == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *c.Score, Valid: true}
}

func cursorInt8(key func(*rivals.Cursor) *int64, c *rivals.Cursor) pgtype.Int8 {
	if c == nil {
		return pgtype.Int8{}
	}
	if v := key(c); v != nil {
		return pgtype.Int8{Int64: *v, Valid: true}
	}
	return pgtype.Int8{}
}

func cursorTS(c *rivals.Cursor) pgtype.Timestamptz {
	if c == nil || c.Published == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *c.Published, Valid: true}
}

func int4Ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func float8Ptr(v pgtype.Float8) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func boolPtr(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Bool
	return &b
}

func rivalChannelFromRow(r gen.ListRivalsRow) domain.RivalChannel {
	return domain.RivalChannel{
		ID:              r.ID,
		Handle:          r.Handle.String,
		Title:           r.Title,
		Description:     r.Description,
		AvatarURL:       r.AvatarUrl.String,
		URL:             r.Url,
		SubscriberCount: int8Ptr(r.SubscriberCount),
		MedianViews:     int8Ptr(r.MedianViews),
		MedianAt:        timePtr(r.MedianAt),
		AddedAt:         r.AddedAt.Time,
		ScannedAt:       timePtr(r.ScannedAt),
	}
}

func rivalVideoFromRow(r pageRow) domain.RivalVideo {
	v := domain.RivalVideo{
		ID:            r.ID,
		ChannelID:     r.ChannelID,
		Title:         r.Title,
		DurationSec:   int4Ptr(r.DurationSec),
		ViewCount:     int8Ptr(r.ViewCount),
		PublishedText: r.PublishedText.String,
		PublishedAt:   timePtr(r.PublishedAt),
		UploadedAt:    timePtr(r.UploadedAt),
		HasCaptions:   boolPtr(r.HasCaptions),
		Score:         float8Ptr(r.Score),
		Position:      r.Position,
		Origin:        r.Origin,
		FirstSeenAt:   r.FirstSeenAt.Time,
	}
	return v
}
