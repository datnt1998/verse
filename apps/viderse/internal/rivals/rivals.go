// Package rivals là tầng đọc của mảng đối thủ (Khám phá): danh sách kênh kèm
// đếm và trạng thái quét, trang video có lọc/sắp xếp/keyset, và mẫu giờ đăng
// đầu vào cho thuật toán khung đăng §12 (xem postingslots.go). Mọi ghi
// (thêm/xoá kênh, quét) thuộc package scan — package này chỉ đọc.
package rivals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// VideoSort là cách sắp xếp trang video của Khám phá.
type VideoSort = string

const (
	SortOutlier VideoSort = "outlier" // điểm nổi trội giảm, null cuối
	SortNewest  VideoSort = "newest"  // một kênh: position; mọi kênh: giờ đăng
	SortViews   VideoSort = "views"   // lượt xem giảm, null cuối
)

// videoSorts là tập sort hợp lệ của trang video.
var videoSorts = map[VideoSort]bool{SortOutlier: true, SortNewest: true, SortViews: true}

// withinDays là tập cửa sổ "Đăng trong N ngày" hợp lệ (0 = mọi thời gian).
var withinDays = map[int]bool{0: true, 7: true, 30: true, 90: true}

// Giới hạn trang video của Khám phá.
const (
	VideoPageDefault = 60
	VideoPageMax     = 200
)

// ErrRivalNotFound báo id kênh không phải kênh đối thủ đang theo dõi.
var ErrRivalNotFound = errors.New("không tìm thấy kênh đối thủ")

// ErrInvalidSort / ErrInvalidWithin / ErrInvalidZone / ErrInvalidLimit báo
// tham số lọc-sắp xếp sai — tầng web/CLI map sang lỗi invalid_input.
var (
	ErrInvalidSort   = errors.New("sắp xếp phải là outlier|newest|views")
	ErrInvalidWithin = errors.New("khoảng thời gian phải là 0|7|30|90 ngày")
	ErrInvalidZone   = errors.New("múi giờ phải là vn|ny")
	ErrInvalidLimit  = errors.New("limit phải là số nguyên từ 1 tới 200")
	ErrInvalidQuery  = errors.New("chuỗi tìm tối đa 200 ký tự")
)

// ErrInvalidCursor báo cursor của trang trước không đọc được hoặc bị sửa.
var ErrInvalidCursor = errors.New("cursor không hợp lệ")

// QueryMaxRunes là trần chuỗi tìm tiêu đề (ký tự, không phải byte) — dài hơn
// bị từ chối ở tầng đọc để HTTP và CLI cùng một luật, không âm thầm cắt.
const QueryMaxRunes = 200

// ListEntry là một dòng của GET /api/rivals: kênh + số đếm + trạng thái quét
// hiển thị trên sidebar.
type ListEntry struct {
	Channel          domain.RivalChannel
	VideoCount       int64 // số video đã đọc vào kho
	Outliers         int64 // số video có score >= OutlierMinScore
	NewOutliers      int64 // outliers gốc discovery, first_seen ≤ 7 ngày
	HistoryState     domain.HistoryState
	DiscoveryRunning bool
}

// ListResult là kết quả List: các kênh theo addedAt và tổng hợp toàn cục.
type ListResult struct {
	Rivals      []ListEntry
	TotalVideos int64
	NewOutliers int64 // tổng nổi trội mới trên mọi kênh
}

// VideoQuery là tham số đầu vào của một trang video (chưa chuẩn hoá).
type VideoQuery struct {
	Sort       VideoSort
	Long       bool   // chỉ video >= LongFormMinSec
	WithinDays int    // 0|7|30|90
	Query      string // chuỗi tìm trong tiêu đề
	Limit      int    // 1..VideoPageMax
	Cursor     string // cursor trang trước; rỗng = trang đầu
}

// VideoPage là một trang kết quả + tổng khớp bộ lọc + cursor trang kế
// (chuỗi opaque; rỗng = hết trang).
type VideoPage struct {
	Videos     []domain.RivalVideo
	Total      int64
	NextCursor string
}

// PageQuery là tham số đã chuẩn hoá mà store dùng: mẫu LIKE đã escape và
// cursor đã giải mã.
type PageQuery struct {
	ChannelID  string
	Sort       VideoSort
	Long       bool
	WithinDays int
	Pattern    string // '%…%' đã escape, rỗng = không lọc
	Now        time.Time
	Limit      int
	Cursor     *Cursor
}

// PostingSampleRow là một video trong mẫu giờ đăng: giờ đăng chính xác (nếu
// đã đọc được) và cờ đã đọc xong.
type PostingSampleRow struct {
	UploadedAt *time.Time // giờ đăng chính xác; nil = chưa có hoặc không có
	Checked    bool       // đã đọc trang video xong (kể cả kết quả không có giờ)
}

// Store là mặt lưu trữ Service cần — hiện thực là postgres.RivalsStore.
type Store interface {
	// ListRivals trả các kênh theo addedAt với đếm và trạng thái quét; now và
	// newSince cắt cửa sổ "mới" và độ tươi lease worker.
	ListRivals(ctx context.Context, now, newSince time.Time) ([]ListEntry, error)
	// RivalExists báo id có phải kênh đối thủ không.
	RivalExists(ctx context.Context, channelID string) (bool, error)
	// CountVideos đếm video khớp bộ lọc (ChannelID rỗng = mọi kênh).
	CountVideos(ctx context.Context, q PageQuery) (int64, error)
	// PageVideos trả tối đa q.Limit dòng của trang sau q.Cursor theo q.Sort.
	PageVideos(ctx context.Context, q PageQuery) ([]domain.RivalVideo, error)
	// PostingSample trả mẫu mới nhất của kênh cho thuật toán §12.
	PostingSample(ctx context.Context, channelID string, sample int) ([]PostingSampleRow, error)
	// UploadProbePausedUntil trả giờ tạm nghỉ đọc giờ đăng (nil = không nghỉ).
	UploadProbePausedUntil(ctx context.Context) (*time.Time, error)
	// CountRivalChannels đếm kênh đối thủ (phục vụ đếm usage của nền tảng).
	CountRivalChannels(ctx context.Context) (int64, error)
}

// Service là mặt đọc của mảng đối thủ: kiểm tham số, phân trang keyset và dựng
// đầu vào §12. An toàn cho goroutine (không giữ trạng thái).
type Service struct {
	store Store
	now   func() time.Time
}

// New dựng Service trên store.
func New(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// NewWithClock như New nhưng đồng hồ tiêm được cho test.
func NewWithClock(store Store, now func() time.Time) *Service {
	s := New(store)
	if now != nil {
		s.now = now
	}
	return s
}

// newOutliersWindow là cửa sổ "nổi trội mới" của sidebar.
const newOutliersWindow = 7 * 24 * time.Hour

// List trả danh sách đối thủ kèm đếm và tổng hợp cho sidebar.
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	now := s.now()
	entries, err := s.store.ListRivals(ctx, now, now.Add(-newOutliersWindow))
	if err != nil {
		return nil, fmt.Errorf("đọc danh sách đối thủ: %w", err)
	}
	out := &ListResult{Rivals: entries}
	for _, e := range entries {
		out.TotalVideos += e.VideoCount
		out.NewOutliers += e.NewOutliers
	}
	return out, nil
}

// Rival trả một dòng của danh sách (dùng cho phản hồi 201/409 của POST add).
func (s *Service) Rival(ctx context.Context, channelID string) (*ListEntry, error) {
	list, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list.Rivals {
		if list.Rivals[i].Channel.ID == channelID {
			return &list.Rivals[i], nil
		}
	}
	return nil, ErrRivalNotFound
}

// Videos trả một trang video của một kênh (hoặc mọi kênh khi channelID =
// domain.AllRivals / rỗng). Cursor rỗng = trang đầu; cursor lỗi →
// ErrInvalidCursor. Kênh lạ → ErrRivalNotFound.
func (s *Service) Videos(ctx context.Context, channelID string, in VideoQuery) (*VideoPage, error) {
	channel := channelID
	if channel == domain.AllRivals {
		channel = ""
	}
	if channel != "" {
		exists, err := s.store.RivalExists(ctx, channel)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrRivalNotFound
		}
	}
	if !videoSorts[in.Sort] {
		return nil, ErrInvalidSort
	}
	if !withinDays[in.WithinDays] {
		return nil, ErrInvalidWithin
	}
	if in.Limit <= 0 || in.Limit > VideoPageMax {
		return nil, ErrInvalidLimit
	}
	if len([]rune(in.Query)) > QueryMaxRunes {
		return nil, ErrInvalidQuery
	}
	// Store đọc limit+1 để biết còn trang sau: trang trả đủ limit dòng mà
	// kho hết hàng thì nextCursor phải rỗng chứ không lặp lại trang cuối.
	q := PageQuery{
		ChannelID:  channel,
		Sort:       in.Sort,
		Long:       in.Long,
		WithinDays: in.WithinDays,
		Pattern:    likePattern(in.Query),
		Now:        s.now(),
		Limit:      in.Limit + 1,
	}
	if in.Cursor != "" {
		cursor, err := decodeCursor(in.Cursor, cursorSig(q), in.Sort, channel != "")
		if err != nil {
			return nil, ErrInvalidCursor
		}
		q.Cursor = cursor
	}
	total, err := s.store.CountVideos(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("đếm video đối thủ: %w", err)
	}
	rows, err := s.store.PageVideos(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("đọc trang video đối thủ: %w", err)
	}
	page := &VideoPage{Total: total}
	if len(rows) > in.Limit {
		last := rows[in.Limit-1]
		page.Videos = rows[:in.Limit]
		next, err := cursorOf(q, last).encode(cursorSig(q))
		if err != nil {
			return nil, err
		}
		page.NextCursor = next
	} else {
		page.Videos = rows
	}
	return page, nil
}

// PostingSchedule dựng lịch đăng §12 của một kênh trong một múi giờ. Kênh lạ
// → ErrRivalNotFound; múi giờ lạ → ErrInvalidZone.
func (s *Service) PostingSchedule(ctx context.Context, channelID, zone string) (*domain.PostingSchedule, error) {
	exists, err := s.store.RivalExists(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRivalNotFound
	}
	loc, err := ZoneLocation(zone)
	if err != nil {
		return nil, ErrInvalidZone
	}
	rows, err := s.store.PostingSample(ctx, channelID, domain.PostingSample)
	if err != nil {
		return nil, fmt.Errorf("đọc mẫu giờ đăng của %s: %w", channelID, err)
	}
	paused, err := s.store.UploadProbePausedUntil(ctx)
	if err != nil {
		return nil, fmt.Errorf("đọc giờ nghỉ đọc giờ đăng: %w", err)
	}
	times := make([]time.Time, 0, len(rows))
	pending := 0
	for _, r := range rows {
		if r.UploadedAt != nil {
			times = append(times, *r.UploadedAt)
		}
		if !r.Checked {
			pending++
		}
	}
	sched := PostingSlots(channelID, zone, times, pending, loc)
	sched.PausedUntil = paused
	return sched, nil
}
