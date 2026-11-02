package rivals

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeStore là rivals.Store giả cho test service: trả dữ liệu cài sẵn và ghi
// lại PageQuery đã chuẩn hoá.
type fakeStore struct {
	entries   []ListEntry
	exists    map[string]bool
	videos    []domain.RivalVideo
	total     int64
	sample    []PostingSampleRow
	paused    *time.Time
	count     int64
	err       error
	lastCount PageQuery
	lastPage  PageQuery
	lastChan  string

	owners       map[string]bool
	usage        []domain.VideoUsage
	baselines    []AutoPickBaseline
	videoChan    map[string]string
	infoPaths    map[string]string
	lastUsageIDs []string
}

func (f *fakeStore) ListRivals(context.Context, time.Time, time.Time) ([]ListEntry, error) {
	return f.entries, f.err
}

func (f *fakeStore) RivalExists(_ context.Context, id string) (bool, error) {
	return f.exists[id], f.err
}

func (f *fakeStore) CountVideos(_ context.Context, q PageQuery) (int64, error) {
	f.lastCount = q
	return f.total, f.err
}

func (f *fakeStore) PageVideos(_ context.Context, q PageQuery) ([]domain.RivalVideo, error) {
	f.lastPage = q
	return f.videos, f.err
}

func (f *fakeStore) PostingSample(_ context.Context, id string, _ int) ([]PostingSampleRow, error) {
	f.lastChan = id
	return f.sample, f.err
}

func (f *fakeStore) UploadProbePausedUntil(context.Context) (*time.Time, error) {
	return f.paused, f.err
}

func (f *fakeStore) CountRivalChannels(context.Context) (int64, error) {
	return f.count, f.err
}

func (f *fakeStore) OwnerChannelExists(_ context.Context, id string) (bool, error) {
	return f.owners[id], f.err
}

func (f *fakeStore) VideoUsage(_ context.Context, ids []string) ([]domain.VideoUsage, error) {
	f.lastUsageIDs = ids
	return f.usage, f.err
}

func (f *fakeStore) AutoPickBaselines(context.Context) ([]AutoPickBaseline, error) {
	return f.baselines, f.err
}

func (f *fakeStore) RivalVideoChannel(_ context.Context, id string) (string, bool, error) {
	channel, ok := f.videoChan[id]
	return channel, ok, f.err
}

func (f *fakeStore) SourceInfoArtifactPath(_ context.Context, id string) (string, bool, error) {
	path, ok := f.infoPaths[id]
	return path, ok, f.err
}

func TestListAggregatesTotals(t *testing.T) {
	store := &fakeStore{entries: []ListEntry{
		{Channel: domain.RivalChannel{ID: "UCa"}, VideoCount: 10, Outliers: 2, NewOutliers: 1, HistoryState: "idle"},
		{Channel: domain.RivalChannel{ID: "UCb"}, VideoCount: 5, Outliers: 0, NewOutliers: 3, HistoryState: "running"},
	}}
	got, err := New(store).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalVideos != 15 || got.NewOutliers != 4 || len(got.Rivals) != 2 {
		t.Fatalf("list = %+v", got)
	}
}

func TestVideosValidatesQuery(t *testing.T) {
	store := &fakeStore{exists: map[string]bool{"UCa": true}}
	svc := New(store)
	ctx := context.Background()
	base := VideoQuery{Sort: SortOutlier, WithinDays: 0, Limit: 10}

	if _, err := svc.Videos(ctx, "UCa", base); err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if store.lastPage.Pattern != "" || store.lastCount.ChannelID != "UCa" {
		t.Fatalf("PageQuery = %+v", store.lastPage)
	}
	bad := base
	bad.Sort = "magic"
	if _, err := svc.Videos(ctx, "UCa", bad); !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("sort = %v", err)
	}
	bad = base
	bad.WithinDays = 14
	if _, err := svc.Videos(ctx, "UCa", bad); !errors.Is(err, ErrInvalidWithin) {
		t.Fatalf("within = %v", err)
	}
	bad = base
	bad.Limit = 201
	if _, err := svc.Videos(ctx, "UCa", bad); !errors.Is(err, ErrInvalidLimit) {
		t.Fatalf("limit = %v", err)
	}
	bad = base
	bad.Cursor = "không-phải-cursor"
	if _, err := svc.Videos(ctx, "UCa", bad); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor = %v", err)
	}
	// Chuỗi tìm quá 200 ký tự bị từ ở tầng đọc (HTTP + CLI cùng luật).
	bad = base
	bad.Query = strings.Repeat("ế", QueryMaxRunes+1)
	if _, err := svc.Videos(ctx, "UCa", bad); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("q dài = %v", err)
	}
	// Đúng 200 ký tự (600 byte UTF-8) vẫn hợp lệ — đếm theo rune.
	bad.Query = strings.Repeat("ế", QueryMaxRunes)
	if _, err := svc.Videos(ctx, "UCa", bad); err != nil {
		t.Fatalf("q đúng trần: %v", err)
	}
	if _, err := svc.Videos(ctx, "UCx-lạ", base); !errors.Is(err, ErrRivalNotFound) {
		t.Fatalf("kênh lạ = %v", err)
	}
}

func TestVideosAllScopeSkipsExistence(t *testing.T) {
	store := &fakeStore{exists: map[string]bool{}, total: 42}
	svc := New(store)
	q := VideoQuery{Sort: SortNewest, Limit: 10}
	page, err := svc.Videos(context.Background(), domain.AllRivals, q)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 42 || store.lastPage.ChannelID != "" {
		t.Fatalf("all = %+v", store.lastPage)
	}
}

func TestCursorRoundTripAndTamper(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	score := 4.2
	video := domain.RivalVideo{ID: "vidid000001", ChannelID: "UCa", Score: &score, Position: 7}
	more := domain.RivalVideo{ID: "vidid000002", ChannelID: "UCa", Score: &score, Position: 8}
	// Fake trả limit+1 dòng để mô phỏng trang đầy.
	store := &fakeStore{exists: map[string]bool{"UCa": true}, videos: []domain.RivalVideo{video, more}, total: 30}
	svc := NewWithClock(store, func() time.Time { return now })
	q := VideoQuery{Sort: SortOutlier, Limit: 1}
	page, err := svc.Videos(context.Background(), "UCa", q)
	if err != nil {
		t.Fatal(err)
	}
	if page.NextCursor == "" {
		t.Fatal("trang đầy phải có nextCursor")
	}
	raw := page.NextCursor
	// Cursor hợp lệ giải được và đi tiếp đúng trang.
	store.videos = nil
	page2, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortOutlier, Limit: 1, Cursor: raw})
	if err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	if page2.NextCursor != "" {
		t.Fatal("trang thiếu không được có nextCursor")
	}
	if store.lastPage.Cursor == nil || store.lastPage.Cursor.Score == nil || *store.lastPage.Cursor.Score != score || store.lastPage.Cursor.ID != video.ID {
		t.Fatalf("cursor tới store = %+v", store.lastPage.Cursor)
	}
	// Sửa một byte → invalid.
	tampered := raw[:len(raw)-4] + "XXXX"
	if _, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortOutlier, Limit: 1, Cursor: tampered}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor sửa = %v", err)
	}
	// Cursor của truy vấn khác (đổi sort) → invalid.
	if _, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortViews, Limit: 1, Cursor: raw}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor khác sort = %v", err)
	}
}

func TestCursorNullTailRoundTrip(t *testing.T) {
	// Trang kết thúc ở dòng score NULL: cursor phải ghi "đã vào đuôi NULL",
	// không phải "không có cursor" — nếu không sẽ lặp lại trang đầu.
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	video := domain.RivalVideo{ID: "vidid000099", ChannelID: "UCa", Position: 9}
	more := domain.RivalVideo{ID: "vidid000100", ChannelID: "UCa", Position: 10}
	store := &fakeStore{exists: map[string]bool{"UCa": true}, videos: []domain.RivalVideo{video, more}, total: 30}
	svc := NewWithClock(store, func() time.Time { return now })
	page, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortOutlier, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.NextCursor == "" {
		t.Fatalf("cursor đuôi NULL rỗng")
	}
	raw := page.NextCursor
	store.videos = nil
	if _, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortOutlier, Limit: 1, Cursor: raw}); err != nil {
		t.Fatalf("giải cursor đuôi NULL: %v", err)
	}
	if store.lastPage.Cursor == nil || store.lastPage.Cursor.Score != nil || store.lastPage.Cursor.ID != video.ID {
		t.Fatalf("cursor đuôi NULL tới store = %+v", store.lastPage.Cursor)
	}
}

func TestPostingScheduleCountsPending(t *testing.T) {
	ts := time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC)
	paused := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	store := &fakeStore{
		exists: map[string]bool{"UCa": true},
		sample: []PostingSampleRow{
			{UploadedAt: &ts, Checked: true},
			{UploadedAt: nil, Checked: false}, // chưa đọc giờ
			{UploadedAt: nil, Checked: true},  // đọc xong nhưng không có giờ
		},
		paused: &paused,
	}
	sched, err := New(store).PostingSchedule(context.Background(), "UCa", domain.ZoneVN)
	if err != nil {
		t.Fatal(err)
	}
	if sched.Total != 1 || sched.Pending != 1 || sched.PausedUntil == nil || !sched.PausedUntil.Equal(paused) {
		t.Fatalf("schedule = %+v", sched)
	}
	if _, err := New(store).PostingSchedule(context.Background(), "UCx-lạ", domain.ZoneVN); !errors.Is(err, ErrRivalNotFound) {
		t.Fatalf("kênh lạ = %v", err)
	}
	if _, err := New(store).PostingSchedule(context.Background(), "UCa", "berlin"); !errors.Is(err, ErrInvalidZone) {
		t.Fatalf("zone lạ = %v", err)
	}
}

func TestUsageCounterReportsYouTubeRivals(t *testing.T) {
	store := &fakeStore{count: 3}
	usage, err := NewUsageCounter(store).Usage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	yt := usage[domain.PlatformYouTube]
	if yt.Rivals == nil || *yt.Rivals != 3 || yt.Channels != 0 || yt.Videos != 0 {
		t.Fatalf("usage = %+v", usage)
	}
}

// rawCursor dựng cursor từ JSON thô để thử các hình dạng sai.
func rawCursor(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// TestCursorStrictValidation khoá kiểm hình dạng cursor: chỉ base64url(JSON
// đúng phiên bản, đúng sort được yêu cầu, đúng khoá theo sort, id video 11
// ký tự, số nguyên/finite, đúng dấu vân tay) mới qua — còn lại invalid_cursor.
// Mỗi ca đi kèm sort/kênh của REQUEST khớp payload để hàng nào cũng tới đúng
// nhánh nó kiểm (không chết sớm ở dấu vân tay).
func TestCursorStrictValidation(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exists: map[string]bool{"UCa": true}, videos: []domain.RivalVideo{
		{ID: "vidid000001", ChannelID: "UCa", Position: 7},
	}, total: 10}
	svc := NewWithClock(store, func() time.Time { return now })

	sig := func(channel string, sort VideoSort) string {
		return cursorSig(PageQuery{ChannelID: channel, Sort: sort})
	}
	outSig := sig("UCa", SortOutlier)
	viewSig := sig("UCa", SortViews)
	newSig := sig("UCa", SortNewest)
	allSig := sig("", SortNewest)
	valid := `{"v":2,"s":"outlier","sc":1.5,"id":"vidid000001","g":` + strconv.Quote(outSig) + `}`

	cases := []struct {
		name    string
		channel string    // kênh của request ("" = all)
		sort    VideoSort // sort của request — phải khớp payload
		cursor  string
		ok      bool
	}{
		{"hợp lệ", "UCa", SortOutlier, rawCursor(valid), true},
		{"hợp lệ đuôi NULL", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), true},
		{"phiên bản lạ", "UCa", SortOutlier, rawCursor(`{"v":1,"s":"outlier","id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), false},
		{"dấu vân tay khác", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","id":"vidid000001","g":"khac"}`), false},
		{"id ngắn", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","id":"abc","g":` + strconv.Quote(outSig) + `}`), false},
		{"id ký tự lạ", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","id":"vidid0000!1","g":` + strconv.Quote(outSig) + `}`), false},
		{"sort lạ", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"hot","id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), false},
		{"sort payload lệch request", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"views","vw":5,"id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), false},
		{"sai khoá cho sort", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","vw":5,"id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), false},
		{"score vô hạn", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","sc":1e999,"id":"vidid000001","g":` + strconv.Quote(outSig) + `}`), false},
		{"views lẻ", "UCa", SortViews, rawCursor(`{"v":2,"s":"views","vw":1.5,"id":"vidid000001","g":` + strconv.Quote(viewSig) + `}`), false},
		{"views âm", "UCa", SortViews, rawCursor(`{"v":2,"s":"views","vw":-2,"id":"vidid000001","g":` + strconv.Quote(viewSig) + `}`), false},
		{"position âm (discovery)", "UCa", SortNewest, rawCursor(`{"v":2,"s":"newest","po":-9999,"id":"vidid000001","g":` + strconv.Quote(newSig) + `}`), true},
		{"position lẻ", "UCa", SortNewest, rawCursor(`{"v":2,"s":"newest","po":1.5,"id":"vidid000001","g":` + strconv.Quote(newSig) + `}`), false},
		{"newest một kênh thiếu position", "UCa", SortNewest, rawCursor(`{"v":2,"s":"newest","id":"vidid000001","g":` + strconv.Quote(newSig) + `}`), false},
		{"newest gộp kênh có position", "", SortNewest, rawCursor(`{"v":2,"s":"newest","po":3,"id":"vidid000001","g":` + strconv.Quote(allSig) + `}`), false},
		{"thừa trường", "UCa", SortOutlier, rawCursor(`{"v":2,"s":"outlier","id":"vidid000001","la":"x","g":` + strconv.Quote(outSig) + `}`), false},
		{"đuôi dấu đóng", "UCa", SortOutlier, rawCursor(valid + `}`), false},
		{"đuôi dấu đóng vuông", "UCa", SortOutlier, rawCursor(valid + `]`), false},
		{"đuôi JSON thứ hai", "UCa", SortOutlier, rawCursor(valid + ` {"x":1}`), false},
		{"không phải base64", "UCa", SortOutlier, "!!!khong-base64!!!", false},
		{"quá dài", "UCa", SortOutlier, strings.Repeat("A", cursorMaxLen+1), false},
		{"có khoảng trắng", "UCa", SortOutlier, rawCursor(valid) + " ", false},
	}
	for _, c := range cases {
		id := c.channel
		if id == "" {
			id = domain.AllRivals
		}
		page, err := svc.Videos(context.Background(), id, VideoQuery{
			Sort: c.sort, Limit: 1, Cursor: c.cursor,
		})
		if c.ok {
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if page == nil {
				t.Fatalf("%s: không có trang", c.name)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%s: err = %v", c.name, err)
		}
	}
}

// TestCursorReuseAcrossFilters báo lỗi khi cursor của bộ lọc này dùng cho bộ
// lọc/khác phạm vi khác — kể cả cùng sort nhưng đổi long/within/q/kênh.
func TestCursorReuseAcrossFilters(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{exists: map[string]bool{"UCa": true, "UCb": true}, videos: []domain.RivalVideo{
		{ID: "vidid000001", ChannelID: "UCa"}, {ID: "vidid000002", ChannelID: "UCa"},
	}}
	svc := NewWithClock(store, func() time.Time { return now })
	page, err := svc.Videos(context.Background(), "UCa", VideoQuery{Sort: SortOutlier, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cursor := page.NextCursor
	for _, other := range []VideoQuery{
		{Sort: SortOutlier, Limit: 1, Long: true},
		{Sort: SortOutlier, Limit: 1, WithinDays: 7},
		{Sort: SortOutlier, Limit: 1, Query: "đêm"},
		{Sort: SortViews, Limit: 1},
		{Sort: SortNewest, Limit: 1},
	} {
		other.Cursor = cursor
		if _, err := svc.Videos(context.Background(), "UCa", other); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%+v: err = %v", other, err)
		}
	}
	// Cùng bộ lọc nhưng kênh khác cũng bị từ (dấu vân tay gồm kênh).
	if _, err := svc.Videos(context.Background(), "UCb", VideoQuery{Sort: SortOutlier, Limit: 1, Cursor: cursor}); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("kênh khác: %v", err)
	}
}

// TestIlikePatternEscapes khoá escape % _ \ để tìm là tìm chuỗi con đúng nghĩa.
func TestIlikePatternEscapes(t *testing.T) {
	cases := map[string]string{
		"":     "",
		"  ":   "",
		"đêm":  "%đêm%",
		"100%": `%100\%%`,
		"a_b":  `%a\_b%`,
		`c\d`:  `%c\\d%`,
		`%_\`:  `%\%\_\\%`,
	}
	for in, want := range cases {
		if got := likePattern(in); got != want {
			t.Fatalf("%q → %q muốn %q", in, got, want)
		}
	}
}
