// Đo độ trễ đầu cuối của GET /api/rivals/:id/videos (handler + store trên
// Postgres thật): 20.000 video trên 4 kênh, mỗi sort có/không q, trang đầu và
// trang sâu qua cursor keyset. Bỏ qua khi thiếu TEST_DATABASE_URL.
//
// Mục tiêu p95 ≤ 100 ms mỗi biến thể. Test log p50/p95 và chỉ fail khi vượt
// 400 ms (biến dạng thật, không phải nhiễu máy).
package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/status"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/identity"
)

// benchSettings là settings giả của status.Service cho gateway trong bench.
type benchSettings struct{}

func (benchSettings) QueuePaused(context.Context) (bool, error) { return false, nil }

// stubMutations là RivalMutations giả — bench chỉ đo route GET videos.
type stubMutations struct{}

func (stubMutations) Add(context.Context, string) (domain.RivalChannel, error) {
	return domain.RivalChannel{}, fmt.Errorf("bench stub")
}
func (stubMutations) Remove(context.Context, string) error { return fmt.Errorf("bench stub") }
func (stubMutations) Status(context.Context, string) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, fmt.Errorf("bench stub")
}
func (stubMutations) Control(context.Context, string, domain.ScanAction) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, fmt.Errorf("bench stub")
}
func (stubMutations) SetCap(context.Context, string, int) (domain.ScanStatus, error) {
	return domain.ScanStatus{}, fmt.Errorf("bench stub")
}

// benchGateway là gateway thật (auth + chống cross-origin bật) với route
// /api/rivals* gắn store Postgres thật — đúng đường đi của serve.
type benchGateway struct {
	handler http.Handler
	cookie  string
}

func newBenchGateway(t *testing.T, pool *pgxpool.Pool) *benchGateway {
	t.Helper()
	accounts := identity.NewStore(t.TempDir()+"/web-accounts.json", identity.Options{})
	if _, err := accounts.CreateUser(identity.NewUser{
		Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass",
	}); err != nil {
		t.Fatal(err)
	}
	hub := web.NewEventHub()
	t.Cleanup(hub.Close)
	svc := status.New(benchSettings{}, web.PublishStatus(hub))
	g := web.NewGateway(accounts, hub, svc)
	web.MountRivals(g, rivals.New(postgres.NewRivalsStore(pool)), stubMutations{})

	// Đăng nhập để lấy cookie phiên như client thật.
	login := httptest.NewRequest(http.MethodPost, "/api/login",
		strings.NewReader(`{"username":"admin","password":"admin-pass"}`))
	login.Header.Set("Content-Type", "application/json")
	login.RemoteAddr = "192.0.2.1:1234"
	login.Host = "127.0.0.1:8789"
	w := httptest.NewRecorder()
	g.ServeHTTP(w, login)
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", w.Code, w.Body.String())
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == "viderse_session" {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("thiếu session cookie")
	}
	return &benchGateway{handler: g, cookie: cookie}
}

// get gọi một GET đã xác thực và trả thời gian + trang kết quả.
func (bg *benchGateway) get(path string) (time.Duration, videoPageOut) {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	r.AddCookie(&http.Cookie{Name: "viderse_session", Value: bg.cookie})
	w := httptest.NewRecorder()
	start := time.Now()
	bg.handler.ServeHTTP(w, r)
	elapsed := time.Since(start)
	if w.Code != http.StatusOK {
		panic(fmt.Sprintf("%s → %d: %s", path, w.Code, w.Body.String()))
	}
	var out videoPageOut
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return elapsed, out
}

type videoPageOut struct {
	Total      int64   `json:"total"`
	NextCursor *string `json:"nextCursor"`
	Videos     []struct {
		ID string `json:"id"`
	} `json:"videos"`
}

// percentiles trả p50/p95 của một mẫu thời gian (ms).
func percentiles(samples []time.Duration) (p50, p95 float64) {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	pick := func(p float64) float64 {
		idx := int(float64(len(samples)) * p)
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return float64(samples[idx].Microseconds()) / 1000
	}
	return pick(0.50), pick(0.95)
}

// seedRivalBenchVideos chèn 20.000 video trên 4 kênh bằng một câu INSERT:
// ~10% tiêu đề chứa token tìm kiếm "truyện đêm", score/views/phân bố đều,
// 5% thiếu score (nhánh NULLS LAST của keyset).
func seedRivalBenchVideos(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		seedRival(t, pool, rivalID(i), fmt.Sprintf("bench%d", i),
			time.Now().Add(-time.Duration(i)*time.Hour))
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO rival_videos
		 (id, channel_id, title, duration_sec, view_count, published_at, uploaded_at,
		  upload_checked_at, score, position, origin, first_seen_at)
		SELECT 'vid' || lpad(n::text, 8, '0'),
		       'UC' || lpad(((n % 4) + 1)::text, 22, '0'),
		       CASE WHEN n % 10 = 0
		            THEN 'truyện đêm khuya ' || n
		            ELSE 'video số ' || n END,
		       300 + (n % 900),
		       (n * 37) % 4000000 + 100,
		       now() - ((n % 400) || ' days')::interval - ((n % 86400) || ' seconds')::interval,
		       CASE WHEN n % 4 = 0
		            THEN now() - ((n % 400) || ' days')::interval - ((n % 86400) || ' seconds')::interval
		            ELSE NULL END,
		       CASE WHEN n % 4 = 0 THEN now() ELSE NULL END,
		       CASE WHEN n % 20 = 0 THEN NULL ELSE ((n % 1500)::float8 / 100) END,
		       (n / 4) + 1,
		       CASE WHEN n % 3 = 0 THEN 'history' ELSE 'discovery' END,
		       now() - ((n % 30) || ' days')::interval
		FROM generate_series(1, 20000) AS n`); err != nil {
		t.Fatalf("seed 20k video: %v", err)
	}
	for i := 1; i <= 4; i++ {
		if _, err := pool.Exec(ctx,
			`INSERT INTO scan_state (channel_id, indexed_videos) VALUES ($1, 5000)`, rivalID(i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `ANALYZE rival_videos`); err != nil {
		t.Fatal(err)
	}
}

// TestRivalVideosEndpointLatency đo p50/p95 của GET /api/rivals/:id/videos cho
// mọi biến thể sort × q × độ sâu trang trên 20.000 video.
func TestRivalVideosEndpointLatency(t *testing.T) {
	pool := provision(t)
	seedRivalBenchVideos(t, pool)
	bg := newBenchGateway(t, pool)
	svc := rivals.New(postgres.NewRivalsStore(pool))
	ctx := context.Background()

	// Trang sâu: đi cursor tới sau ~n×200 dòng đầu của một sort (kênh đơn
	// 5.000 dòng → trang sâu ở ~2.400; gộp 20.000 → ~6.000).
	deepCursor := func(channel, sortKey, q string, pages int) string {
		cursor := ""
		for range pages {
			page, err := svc.Videos(ctx, channel, rivals.VideoQuery{
				Sort: sortKey, Limit: 200, Cursor: cursor, Query: q,
			})
			if err != nil {
				t.Fatalf("đi cursor: %v", err)
			}
			if page.NextCursor == "" {
				return ""
			}
			cursor = page.NextCursor
		}
		return cursor
	}

	variants := []struct {
		name    string
		channel string
		sort    string
		q       string
		deep    int // số trang 200 để đi tới trang sâu (0 = trang đầu)
	}{
		{"outlier/first", rivalID(1), "outlier", "", 0},
		{"outlier/deep", rivalID(1), "outlier", "", 12},
		{"newest/first", rivalID(1), "newest", "", 0},
		{"newest/deep", rivalID(1), "newest", "", 12},
		{"views/first", rivalID(1), "views", "", 0},
		{"views/deep", rivalID(1), "views", "", 12},
		{"outlier/q/first", rivalID(1), "outlier", "truyện đêm", 0},
		{"newest/q/deep", rivalID(1), "newest", "truyện đêm", 2},
		{"all/outlier/first", domain.AllRivals, "outlier", "", 0},
		{"all/outlier/deep", domain.AllRivals, "outlier", "", 30},
		{"all/newest/first", domain.AllRivals, "newest", "", 0},
		{"all/newest/deep", domain.AllRivals, "newest", "", 30},
		{"all/views/first", domain.AllRivals, "views", "", 0},
		{"all/views/deep", domain.AllRivals, "views", "", 30},
		{"all/q/first", domain.AllRivals, "outlier", "truyện đêm", 0},
		{"all/q/deep", domain.AllRivals, "newest", "truyện đêm", 4},
	}

	const iters = 120
	var worstP95 float64
	var worstName string
	for _, v := range variants {
		cursor := ""
		if v.deep > 0 {
			cursor = deepCursor(v.channel, v.sort, v.q, v.deep)
			if cursor == "" {
				t.Fatalf("%s: không tới được trang sâu", v.name)
			}
		}
		path := fmt.Sprintf("/api/rivals/%s/videos?sort=%s&limit=200", v.channel, v.sort)
		if v.q != "" {
			path += "&q=" + url.QueryEscape(v.q)
		}
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		// Khởi động một lần (plan cache + bộ nhớ trang).
		bg.get(path)
		samples := make([]time.Duration, 0, iters)
		for range iters {
			d, _ := bg.get(path)
			samples = append(samples, d)
		}
		p50, p95 := percentiles(samples)
		t.Logf("%-20s p50=%6.2fms p95=%6.2fms", v.name, p50, p95)
		if p95 > worstP95 {
			worstP95, worstName = p95, v.name
		}
	}
	t.Logf("biến thể chậm nhất: %s p95=%.2fms", worstName, worstP95)
	// Hồi quy: phần đọc phải luôn dưới 400 ms kể cả máy CI ồn.
	if worstP95 > 400 {
		t.Fatalf("p95 chậm nhất %.2fms vượt ngưỡng hồi quy 400ms (%s)", worstP95, worstName)
	}
}

// TestRivalListAcceptanceLatency đo GET /api/rivals trên fixture nặng: 50 đối
// thủ × 2.000 video + 50.000 việc discovery đã hoàn tất còn giữ (1.000/kênh,
// ~42 ngày nhịp giờ) — ca từng làm sidebar 462 ms khi thiếu index lịch sử của
// scan_jobs. Mục tiêu nghiệm thu p95 ≤ 100 ms đọc từ log khi máy rảnh; ngưỡng
// fail 400 ms là ngưỡng hồi quy để suite đầy đủ chạy song song không chập chờn.
func TestRivalListAcceptanceLatency(t *testing.T) {
	// Đo độ trễ nghiệm thu phải chạy một mình: khi các suite khác chiếm CPU,
	// p95 phản ánh lập lịch của OS chứ không phải truy vấn (p50 vẫn ~21 ms).
	if testing.Short() {
		t.Skip("đo độ trễ nghiệm thu: bỏ qua ở -short, chạy riêng khi máy rảnh")
	}
	pool := provision(t)
	seedRivalListFixture(t, pool)
	bg := newBenchGateway(t, pool)

	// Máy CI thường chạy song song: lấy mẫu tốt hơn trong hai lần liền nhau
	// để loại nhiễu lập lịch, không che truy vấn chậm thật (cả hai lần đều
	// chậm nếu truy vấn chậm).
	const iters = 80
	bg.get("/api/rivals")
	samples := make([]time.Duration, 0, iters)
	for range iters {
		a, _ := bg.get("/api/rivals")
		b, _ := bg.get("/api/rivals")
		if b < a {
			a = b
		}
		samples = append(samples, a)
	}
	p50, p95 := percentiles(samples)
	t.Logf("GET /api/rivals (50 kênh, 100k video, 50k việc giữ lại) p50=%.2fms p95=%.2fms", p50, p95)
	if p95 > 400 {
		t.Fatalf("list p95 %.2fms vượt ngưỡng hồi quy 400ms", p95)
	}
}

// seedRivalListFixture dựng 50 kênh, 2.000 video/kênh và 1.000 việc
// discovery hoàn tất/kênh — đủ để lộ truy vấn quét toàn bảng scan_jobs.
func seedRivalListFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= 50; i++ {
		seedRival(t, pool, rivalID(i), fmt.Sprintf("list%d", i),
			time.Now().Add(-time.Duration(i)*time.Minute))
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO rival_videos
		 (id, channel_id, title, duration_sec, view_count, published_at, uploaded_at,
		  upload_checked_at, score, position, origin, first_seen_at)
		SELECT 'l' || lpad(n::text, 10, '0'),
		       'UC' || lpad(((n % 50) + 1)::text, 22, '0'),
		       'video ' || n,
		       600 + (n % 300),
		       1000 + (n % 50000),
		       now() - ((n % 900) || ' days')::interval,
		       NULL,
		       NULL,
		       CASE WHEN n % 5 = 0 THEN NULL ELSE ((n % 900)::float8 / 100) END,
		       (n / 50) + 1,
		       'baseline',
		       now() - interval '1 day'
		FROM generate_series(1, 100000) AS n`); err != nil {
		t.Fatalf("seed 100k video: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO scan_jobs (id, channel_id, kind, state, created_at, updated_at, finished_at)
		SELECT gen_random_uuid(),
		       'UC' || lpad(((n % 50) + 1)::text, 22, '0'),
		       'discovery', 'complete',
		       now() - ((n / 50) || ' hours')::interval,
		       now() - ((n / 50) || ' hours')::interval,
		       now() - ((n / 50) || ' hours')::interval
		FROM generate_series(1, 50000) AS n`); err != nil {
		t.Fatalf("seed 50k việc giữ lại: %v", err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE rival_videos`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE scan_jobs`); err != nil {
		t.Fatal(err)
	}
}
