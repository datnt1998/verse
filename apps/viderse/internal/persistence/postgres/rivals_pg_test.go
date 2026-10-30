// Ca test đọc đối thủ trên Postgres thật: danh sách kèm đếm/trạng thái quét,
// trang video keyset cho ba sort + lọc + con trỏ NULL tail, mẫu giờ đăng và
// đếm usage.
//
// Chạy trên schema riêng provision bằng pgtest với cặp role của Viderse rồi
// áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// rivalID dựng id kênh hợp lệ (^UC[A-Za-z0-9_-]{22}$) cho test.
func rivalID(n int) string {
	return fmt.Sprintf("UC%022d", n)
}

// videoID dựng id video hợp lệ (11 ký tự) cho test.
func videoID(n int) string {
	return fmt.Sprintf("vid%08d", n)
}

func seedRival(t *testing.T, pool *pgxpool.Pool, id, title string, addedAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO rival_channels (id, handle, title, url, added_at)
		 VALUES ($1, $2, $3, 'https://youtube.com/'||$2, $4)`,
		id, "@h"+title, title, addedAt); err != nil {
		t.Fatalf("seed kênh %s: %v", id, err)
	}
}

func seedVideo(t *testing.T, pool *pgxpool.Pool, id, channelID string, pos int64, score *float64, views *int64, dur *int, publishedAt *time.Time, uploadedAt *time.Time, origin string, firstSeen time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO rival_videos
		 (id, channel_id, title, duration_sec, view_count, published_at, uploaded_at,
		  upload_checked_at, score, position, origin, first_seen_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7,
		         CASE WHEN $7::timestamptz IS NOT NULL OR $8 THEN now() ELSE NULL END,
		         $9, $10, $11, $12)`,
		id, channelID, "Video "+id, dur, views, publishedAt, uploadedAt,
		uploadedAt == nil && publishedAt == nil, // cờ đã đọc khi cả hai nil
		score, pos, origin, firstSeen); err != nil {
		t.Fatalf("seed video %s: %v", id, err)
	}
}

func TestRivalListCountsAndStates(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now.Add(-48*time.Hour))
	seedRival(t, pool, rivalID(2), "beta", now.Add(-24*time.Hour))

	score := 4.5
	views := int64(9000)
	dur := 600
	pub := now.Add(-10 * 24 * time.Hour)
	old := now.Add(-20 * 24 * time.Hour)
	// Kênh 1: một outlier discovery mới (≤ 7 ngày), một outlier discovery cũ,
	// một outlier history mới (không tính mới), một video thường.
	seedVideo(t, pool, videoID(1), rivalID(1), 1, &score, &views, &dur, &pub, nil, domain.OriginDiscovery, now.Add(-time.Hour))
	seedVideo(t, pool, videoID(2), rivalID(1), 2, &score, &views, &dur, &pub, nil, domain.OriginDiscovery, old)
	seedVideo(t, pool, videoID(3), rivalID(1), 3, &score, &views, &dur, &pub, nil, domain.OriginHistory, now.Add(-time.Hour))
	seedVideo(t, pool, videoID(4), rivalID(1), 4, nil, nil, &dur, &pub, nil, domain.OriginBaseline, old)
	// Kênh 2: một video.
	seedVideo(t, pool, videoID(5), rivalID(2), 1, nil, &views, &dur, &pub, nil, domain.OriginBaseline, old)

	// Trạng thái quét kênh 1: history job đang backoff.
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_state (channel_id, indexed_videos) VALUES ($1, 4)`, rivalID(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_jobs (id, channel_id, kind, state, updated_at)
		 VALUES (gen_random_uuid(), $1, 'history', 'backoff', now()),
		        (gen_random_uuid(), $1, 'discovery', 'running', now())`, rivalID(1)); err != nil {
		t.Fatal(err)
	}
	// Kênh 2: đã đọc hết tab, không có việc → complete.
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_state (channel_id, indexed_videos, history_end) VALUES ($1, 1, true)`, rivalID(2)); err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListRivals(ctx, now, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
	a, b := entries[0], entries[1]
	if a.Channel.ID != rivalID(1) || b.Channel.ID != rivalID(2) {
		t.Fatalf("thứ tự addedAt = %s %s", a.Channel.ID, b.Channel.ID)
	}
	if a.VideoCount != 4 || a.Outliers != 3 || a.NewOutliers != 1 {
		t.Fatalf("kênh 1 = %+v", a)
	}
	if a.HistoryState != "backoff" || !a.DiscoveryRunning {
		t.Fatalf("kênh 1 scan = %+v", a)
	}
	if b.VideoCount != 1 || b.Outliers != 0 || b.NewOutliers != 0 {
		t.Fatalf("kênh 2 = %+v", b)
	}
	if b.HistoryState != "complete" || b.DiscoveryRunning {
		t.Fatalf("kênh 2 scan = %+v", b)
	}

	// Worker chết (lease hết hạn): job running của kênh 1 đọc là queued.
	if _, err := pool.Exec(ctx,
		`INSERT INTO platform_state (platform, scanner_owner, scanner_lease_until)
		 VALUES ('youtube', 'w1', now() - interval '1 minute')
		 ON CONFLICT (platform) DO UPDATE SET scanner_owner = 'w1',
		     scanner_lease_until = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE scan_jobs SET state = 'running' WHERE channel_id = $1 AND kind = 'history'`, rivalID(1)); err != nil {
		t.Fatal(err)
	}
	entries, err = store.ListRivals(ctx, now, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].HistoryState != "queued" {
		t.Fatalf("running không worker phải đọc queued, được %q", entries[0].HistoryState)
	}
}

func TestRivalVideoPagesKeyset(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	svc := rivals.New(store)
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now.Add(-48*time.Hour))
	// 6 video: score phân tán, có NULL score và NULL views.
	scores := []float64{9, 5, 5, 2, 0}
	for i, sc := range scores {
		v := sc
		views := int64((i + 1) * 1000)
		dur := 600
		seedVideo(t, pool, videoID(10+i), rivalID(1), int64(i+1), &v, &views, &dur, nil, nil, domain.OriginBaseline, now.Add(-time.Hour))
	}
	// videoID(15): score NULL, views NULL.
	seedVideo(t, pool, videoID(15), rivalID(1), 6, nil, nil, nil, nil, nil, domain.OriginBaseline, now.Add(-time.Hour))
	if _, err := pool.Exec(ctx, `INSERT INTO scan_state (channel_id) VALUES ($1)`, rivalID(1)); err != nil {
		t.Fatal(err)
	}

	// Sort outlier: 9, 5, 5 (thứ tự id), 2, 0 rồi NULL.
	page1, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortOutlier, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Videos) != 3 || page1.Total != 6 || page1.NextCursor == "" {
		t.Fatalf("trang 1 = %d videos, total %d", len(page1.Videos), page1.Total)
	}
	if page1.Videos[0].ID != videoID(10) || page1.Videos[1].ID != videoID(11) || page1.Videos[2].ID != videoID(12) {
		t.Fatalf("thứ tự outlier = %v %v %v", page1.Videos[0].ID, page1.Videos[1].ID, page1.Videos[2].ID)
	}
	page2, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortOutlier, Limit: 3, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Videos) != 3 || page2.Videos[0].ID != videoID(13) || page2.Videos[1].ID != videoID(14) || page2.Videos[2].ID != videoID(15) {
		t.Fatalf("trang 2 = %v", page2.Videos)
	}
	if page2.NextCursor != "" {
		t.Fatal("hết trang phải nextCursor rỗng")
	}

	// Sort views: 5000…1000 rồi NULL.
	views, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortViews, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(views.Videos) != 6 || views.Videos[0].ID != videoID(14) || views.Videos[5].ID != videoID(15) {
		t.Fatalf("thứ tự views = %v", views.Videos)
	}

	// Sort newest của một kênh: position ASC.
	newest, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if newest.Videos[0].ID != videoID(10) || newest.Videos[1].ID != videoID(11) {
		t.Fatalf("thứ tự newest = %v", newest.Videos)
	}
	deep, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 2, Cursor: newest.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if deep.Videos[0].ID != videoID(12) {
		t.Fatalf("trang sâu newest = %v", deep.Videos)
	}

	// Cursor sửa → 400-tier error.
	if _, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortOutlier, Limit: 2, Cursor: "abc.def"}); err == nil {
		t.Fatal("cursor xấu phải lỗi")
	}
}

func TestRivalVideoFilters(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	svc := rivals.New(store)
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now)
	longDur, shortDur := 600, 120
	pub := now.Add(-10 * 24 * time.Hour)
	old := now.Add(-40 * 24 * time.Hour)
	up := now.Add(-2 * 24 * time.Hour)
	// v1: dài + uploaded 2 ngày trước; v2: ngắn + published 10 ngày; v3: dài + 40 ngày.
	seedVideo(t, pool, videoID(1), rivalID(1), 1, nil, nil, &longDur, &pub, &up, domain.OriginBaseline, now)
	seedVideo(t, pool, videoID(2), rivalID(1), 2, nil, nil, &shortDur, &pub, nil, domain.OriginBaseline, now)
	seedVideo(t, pool, videoID(3), rivalID(1), 3, nil, nil, &longDur, &old, nil, domain.OriginBaseline, now)
	if _, err := pool.Exec(ctx, `INSERT INTO scan_state (channel_id) VALUES ($1)`, rivalID(1)); err != nil {
		t.Fatal(err)
	}

	long, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortNewest, Long: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(long.Videos) != 2 || long.Total != 2 {
		t.Fatalf("long = %d", len(long.Videos))
	}
	within, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortNewest, WithinDays: 7, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Chỉ v1 đủ (uploaded 2 ngày); v2 ước lượng 10 ngày, v3 40 ngày đều quá.
	if len(within.Videos) != 1 || within.Videos[0].ID != videoID(1) {
		t.Fatalf("within7 = %v", within.Videos)
	}
	within30, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{Sort: rivals.SortNewest, WithinDays: 30, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(within30.Videos) != 2 {
		t.Fatalf("within30 = %d", len(within30.Videos))
	}
}

func TestPostingSampleAndUsage(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	svc := rivals.New(store)
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now)
	seedRival(t, pool, rivalID(2), "beta", now)
	exact := now.Add(-24 * time.Hour)
	weekBefore := exact.Add(-7 * 24 * time.Hour)
	// Hai video có giờ đăng chính xác, một video chưa đọc.
	seedVideo(t, pool, videoID(1), rivalID(1), 1, nil, nil, nil, nil, &exact, domain.OriginBaseline, now)
	seedVideo(t, pool, videoID(2), rivalID(1), 2, nil, nil, nil, nil, &weekBefore, domain.OriginBaseline, now)
	seedVideo(t, pool, videoID(3), rivalID(1), 3, nil, nil, nil, nil, nil, domain.OriginBaseline, now)
	// upload_checked_at của v3 vẫn NULL vì cả uploaded/published nil mà cờ
	// seed để false — sửa tay để test pending.
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET upload_checked_at = NULL WHERE id = $1`, videoID(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_schedule (id, upload_times_next_at) VALUES (true, $1)`,
		now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	sched, err := svc.PostingSchedule(ctx, rivalID(1), domain.ZoneVN)
	if err != nil {
		t.Fatal(err)
	}
	if sched.Total != 2 || sched.Pending != 1 || sched.PausedUntil == nil {
		t.Fatalf("schedule = %+v", sched)
	}

	count, err := store.CountRivalChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d", count)
	}
	usage, err := rivals.NewUsageCounter(store).Usage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if usage[domain.PlatformYouTube].Rivals == nil || *usage[domain.PlatformYouTube].Rivals != 2 {
		t.Fatalf("usage = %+v", usage)
	}
}

// TestRivalVideoPagesSixWayKeyset duyệt cả sáu tổ hợp sort × phạm vi bằng
// cursor trang 3 dòng trên dữ liệu có tie và NHIỀU dòng đuôi NULL: mỗi dòng
// khớp bộ lọc phải xuất hiện đúng một lần, không lặp/không sót.
func TestRivalVideoPagesSixWayKeyset(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	svc := rivals.New(store)
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now.Add(-2*time.Hour))
	seedRival(t, pool, rivalID(2), "beta", now.Add(-time.Hour))
	if _, err := pool.Exec(ctx, `INSERT INTO scan_state (channel_id) VALUES ($1) ON CONFLICT DO NOTHING`, rivalID(1)); err != nil {
		t.Fatal(err)
	}

	// Mỗi kênh 8 video: score trùng (4,4,4), NULL score ×3 (ba dòng đuôi
	// NULL), views trùng (100,100), NULL views ×2, published trùng.
	dur := 600
	pos := int64(0)
	add := func(ch string, score *float64, views *int64, pub *time.Time) string {
		pos++
		id := videoID(int(pos))
		seedVideo(t, pool, id, ch, pos, score, views, &dur, pub, nil, domain.OriginBaseline, now)
		return id
	}
	f := func(v float64) *float64 { return &v }
	i64 := func(v int64) *int64 { return &v }
	day := now.Add(-24 * time.Hour)
	day2 := now.Add(-48 * time.Hour)

	wantByChannel := map[string][]string{}
	for _, ch := range []string{rivalID(1), rivalID(2)} {
		wantByChannel[ch] = append(wantByChannel[ch],
			add(ch, f(4), i64(100), &day),
			add(ch, f(4), i64(200), &day2),
			add(ch, f(4), nil, nil), // tie score, NULL views
			add(ch, f(1), i64(100), &day),
			add(ch, nil, i64(50), nil), // đuôi NULL score
			add(ch, nil, nil, nil),     // đuôi NULL score+views
			add(ch, nil, nil, &day),    // đuôi NULL score, có published
			add(ch, f(0), nil, nil),
		)
	}

	for _, sortKey := range []string{rivals.SortOutlier, rivals.SortViews, rivals.SortNewest} {
		for _, ch := range []string{rivalID(1), domain.AllRivals} {
			var got []string
			cursor := ""
			for pages := 0; ; pages++ {
				page, err := svc.Videos(ctx, ch, rivals.VideoQuery{Sort: sortKey, Limit: 3, Cursor: cursor})
				if err != nil {
					t.Fatalf("%s/%s: %v", sortKey, ch, err)
				}
				for _, v := range page.Videos {
					got = append(got, v.ID)
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
				if pages > 20 {
					t.Fatalf("%s/%s: cursor không kết thúc", sortKey, ch)
				}
			}
			want := wantByChannel[ch]
			if ch == domain.AllRivals {
				want = append(append([]string{}, wantByChannel[rivalID(1)]...), wantByChannel[rivalID(2)]...)
			}
			// Mỗi dòng khớp đúng một lần.
			seen := map[string]int{}
			for _, id := range got {
				seen[id]++
			}
			for _, id := range want {
				if seen[id] != 1 {
					t.Fatalf("%s/%s: %s xuất hiện %d lần", sortKey, ch, id, seen[id])
				}
			}
			if len(got) != len(want) {
				t.Fatalf("%s/%s: %d dòng, muốn %d", sortKey, ch, len(got), len(want))
			}
		}
	}
}

// TestRivalVideoSearchEscapesLiteralWildcards: %, _ và \ trong q là ký tự
// thường, không phải wildcard SQL.
func TestRivalVideoSearchEscapesLiteralWildcards(t *testing.T) {
	pool := provision(t)
	svc := rivals.New(postgres.NewRivalsStore(pool))
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now)
	if _, err := pool.Exec(ctx, `INSERT INTO scan_state (channel_id) VALUES ($1)`, rivalID(1)); err != nil {
		t.Fatal(err)
	}
	dur := 600
	for i, title := range []string{"giảm 100% hiệu suất", "giảm 1000 hiệu suất", "a_b", "axb", `c\d`, "cxd"} {
		seedVideo(t, pool, videoID(i+1), rivalID(1), int64(i+1), nil, nil, &dur, &now, nil, domain.OriginBaseline, now)
		if _, err := pool.Exec(ctx, `UPDATE rival_videos SET title = $1 WHERE id = $2`, title, videoID(i+1)); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		q    string
		want int
	}{
		{"100%", 1}, // chỉ video phần trăm, không khớp "1000"
		{"a_b", 1},  // không khớp "axb"
		{`c\d`, 1},  // không khớp "cxd"
	} {
		page, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{
			Sort: rivals.SortNewest, Limit: 10, Query: tc.q,
		})
		if err != nil {
			t.Fatalf("%q: %v", tc.q, err)
		}
		if len(page.Videos) != tc.want {
			t.Fatalf("%q khớp %d video, muốn %d", tc.q, len(page.Videos), tc.want)
		}
	}
}

// TestRivalVideoSearchIgnoresCaseAndAccents: tìm tiêu đề không phân biệt hoa
// thường và dấu ở cả hai phía (đ ↔ d), cộng dồn với lọc đang bật, và tổng
// đếm khớp đúng số video tìm thấy.
func TestRivalVideoSearchIgnoresCaseAndAccents(t *testing.T) {
	pool := provision(t)
	svc := rivals.New(postgres.NewRivalsStore(pool))
	ctx := context.Background()
	now := time.Now()

	seedRival(t, pool, rivalID(1), "alpha", now)
	if _, err := pool.Exec(ctx, `INSERT INTO scan_state (channel_id) VALUES ($1)`, rivalID(1)); err != nil {
		t.Fatal(err)
	}
	long, short := 600, 60
	for i, v := range []struct {
		title string
		dur   *int
	}{
		{"Ngôi Nhà Ma Ở Đà Lạt", &long},
		{"THE FLOOR BELOW", &long},
		{"Café at the end of the floor", &short},
		{"Đường về nhà", &long},
	} {
		seedVideo(t, pool, videoID(i+1), rivalID(1), int64(i+1), nil, nil, v.dur, &now, nil, domain.OriginBaseline, now)
		if _, err := pool.Exec(ctx, `UPDATE rival_videos SET title = $1 WHERE id = $2`, v.title, videoID(i+1)); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		q    string
		long bool
		want []string
	}{
		{"ngoi nha", false, []string{videoID(1)}},
		{"DA LAT", false, []string{videoID(1)}},
		{"Ngôi Nhà", false, []string{videoID(1)}},
		{"floor", false, []string{videoID(2), videoID(3)}},
		{"floor", true, []string{videoID(2)}}, // lọc dài bỏ video ngắn
		{"CAFE", false, []string{videoID(3)}},
		{"duong ve", false, []string{videoID(4)}},
		{"đường", false, []string{videoID(4)}},
		{"basement", false, nil},
	} {
		page, err := svc.Videos(ctx, rivalID(1), rivals.VideoQuery{
			Sort: rivals.SortNewest, Limit: 10, Query: tc.q, Long: tc.long,
		})
		if err != nil {
			t.Fatalf("%q: %v", tc.q, err)
		}
		var got []string
		for _, v := range page.Videos {
			got = append(got, v.ID)
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("%q long=%v → %v, muốn %v", tc.q, tc.long, got, tc.want)
		}
		if page.Total != int64(len(tc.want)) {
			t.Fatalf("%q long=%v: total %d, muốn %d", tc.q, tc.long, page.Total, len(tc.want))
		}
	}
}

// TestRivalNewestPagesFromNegativeDiscoveryPositions là hồi quy hành động
// ghi-quét → đọc nhiều trang: video discovery do scan-commit thật ghi mang
// position ÂM (khối min - DiscoveryBlock), nên cursor newest của server phải
// đi qua được nhiều trang — trước đây decoder từ chối pos < 0.
func TestRivalNewestPagesFromNegativeDiscoveryPositions(t *testing.T) {
	s, pool := scanStore(t)
	store := postgres.NewRivalsStore(pool)
	svc := rivals.New(store)
	ctx := context.Background()
	id := cid("negpos")

	// Đường ghi thật: baseline rồi một run discovery — vị trí âm theo thiết kế.
	// addChannel giữ fence của kênh; dùng lại lease đó cho trang discovery.
	lease, _ := addChannel(t, s, id)
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatalf("start discovery: %v", err)
	}
	job, err := s.NextRunnableJob(ctx, nil)
	if err != nil || job == nil {
		t.Fatalf("không có việc discovery: %v %v", job, err)
	}
	if job.OrderBase == nil || *job.OrderBase != -int64(scan.DiscoveryBlock) {
		t.Fatalf("order base = %v, muốn -DiscoveryBlock", job.OrderBase)
	}
	claim(t, s, job, lease)
	newest := padID("ng0")
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos: []youtube.ScannedVideo{
			video(padID("ng1"), 10), video(padID("ng2"), 20),
			video(padID("ng3"), 30), video(padID("ng4"), 40),
		},
		NextCursor:    cursorPtr(testCursor(id)),
		Origin:        domain.OriginDiscovery,
		Lease:         lease,
		JobID:         job.ID,
		NewestVideoID: &newest,
		Now:           time.Now(),
	}); err != nil {
		t.Fatalf("commit discovery: %v", err)
	}
	// Xác nhận trên DB: các hàng discovery thật sự âm.
	var minPos int64
	if err := pool.QueryRow(ctx,
		`SELECT min(position) FROM rival_videos WHERE channel_id = $1`, id).Scan(&minPos); err != nil {
		t.Fatal(err)
	}
	if minPos >= 0 {
		t.Fatalf("min position = %d, kỳ vọng âm", minPos)
	}

	// Trang 1: hai hàng discovery âm đầu tiên (position ASC).
	p1, err := svc.Videos(ctx, id, rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 2})
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	if len(p1.Videos) != 2 || p1.Videos[0].ID != padID("ng1") || p1.Videos[1].ID != padID("ng2") {
		t.Fatalf("trang 1 = %v", p1.Videos)
	}
	if p1.NextCursor == "" {
		t.Fatal("trang 1 phải có nextCursor")
	}
	// Trang 2 qua cursor pos âm — trước đây ở đây trả ErrInvalidCursor.
	p2, err := svc.Videos(ctx, id, rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 2, Cursor: p1.NextCursor})
	if err != nil {
		t.Fatalf("trang 2 qua cursor pos âm: %v", err)
	}
	if len(p2.Videos) != 2 || p2.Videos[0].ID != padID("ng3") || p2.Videos[1].ID != padID("ng4") {
		t.Fatalf("trang 2 = %v", p2.Videos)
	}
	// Trang 3: qua miền âm → dương, tới hàng baseline đầu.
	p3, err := svc.Videos(ctx, id, rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 2, Cursor: p2.NextCursor})
	if err != nil {
		t.Fatalf("trang 3: %v", err)
	}
	if len(p3.Videos) != 2 || p3.Videos[0].Position < 0 {
		t.Fatalf("trang 3 = %v", p3.Videos)
	}
}

// TestRivalListPicksLatestJobIgnoresFinishedRun: "việc còn sống" phải khớp
// liveJobTx của kho quét (NOT IN complete/cancelled). Một run đã huỷ cũ hơn
// không được thắng run mới đã xong — nếu không sidebar hiện "đã huỷ" cho kênh
// vừa quét xong. Ngược lại, một việc còn sống cũ hơn vẫn thắng run đã xong.
func TestRivalListPicksLatestJobIgnoresFinishedRun(t *testing.T) {
	pool := provision(t)
	store := postgres.NewRivalsStore(pool)
	ctx := context.Background()
	now := time.Now()

	// Kênh 1: cancelled cũ + complete mới hơn → phải đọc "complete".
	seedRival(t, pool, rivalID(1), "cu", now.Add(-48*time.Hour))
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_jobs (id, channel_id, kind, state, updated_at)
		 VALUES (gen_random_uuid(), $1, 'history', 'cancelled', now() - interval '2 hours'),
		        (gen_random_uuid(), $1, 'history', 'complete', now() - interval '1 hour')`,
		rivalID(1)); err != nil {
		t.Fatal(err)
	}

	// Kênh 2: queued cũ hơn complete mới hơn → việc còn sống phải thắng.
	seedRival(t, pool, rivalID(2), "moi", now.Add(-24*time.Hour))
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_jobs (id, channel_id, kind, state, updated_at)
		 VALUES (gen_random_uuid(), $1, 'history', 'queued', now() - interval '2 hours'),
		        (gen_random_uuid(), $1, 'history', 'complete', now() - interval '1 hour')`,
		rivalID(2)); err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListRivals(ctx, now, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
	if got := entries[0].HistoryState; got != "complete" {
		t.Fatalf("kênh có cancelled cũ + complete mới: history = %q, muốn complete", got)
	}
	if got := entries[1].HistoryState; got != "queued" {
		t.Fatalf("kênh có queued cũ + complete mới: history = %q, muốn queued", got)
	}
}
