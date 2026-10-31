package postgres_test

// Test hành vi kho quét đối thủ trên Postgres thật: các ca của
// scan-repository.test.ts (bản TS) cùng những ca ranh giới của hợp đồng
// Viderse — thứ tự khoá parent/state, fence trong từng giao dịch, khối khoá
// discovery, bookmark của lần đọc đầu, mẫu thăm dò giờ đăng và chi phí head
// check. Các ca chen ngang dùng hai kết nối thật với cổng chặn tường minh
// (gateTx) để thứ tự sự kiện là xác định, không dựa vào thời gian.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/datnt1998/verse/platform/pgtest"
)

// ── tiện ích ────────────────────────────────────────────────────────────────

// cid dựng channel id hợp lệ (UC + 22 ký tự) từ một nhãn ngắn.
func cid(tag string) string {
	id := "UC" + tag
	for len(id) < 24 {
		id += "0"
	}
	return id[:24]
}

func scanStore(t *testing.T) (*postgres.ScanStore, *pgxpool.Pool) {
	t.Helper()
	pool := provision(t)
	return postgres.NewScanStore(pool), pool
}

func video(id string, views int64) youtube.ScannedVideo {
	v := padID(id)
	return youtube.ScannedVideo{
		ID:           v,
		Title:        "video " + id,
		ViewCount:    new(views),
		ThumbnailURL: "https://i.ytimg.com/vi/" + v + "/mqdefault.jpg",
	}
}

// padID dựng video id hợp lệ (11 ký tự) từ một nhãn ngắn.
func padID(tag string) string {
	id := tag
	for len(id) < 11 {
		id += "0"
	}
	return id[:11]
}

// leaseOf lấy fence đang giữ của kênh.
func leaseOf(t *testing.T, s *postgres.ScanStore, channelID, owner string) scan.Lease {
	t.Helper()
	lease, err := s.AcquireChannelLease(context.Background(), channelID, owner, time.Minute)
	if err != nil {
		t.Fatalf("acquire lease: %v", err)
	}
	if lease == nil {
		t.Fatal("kênh đang bận, không lấy được fence")
	}
	return *lease
}

func baseProfile(id string) *youtube.ChannelProfile {
	handle := "@" + strings.ToLower(id)
	desc := "kênh thử"
	return &youtube.ChannelProfile{
		ID: id, Handle: &handle, Title: "Kênh " + id, Description: &desc,
		URL: "https://www.youtube.com/channel/" + id, SubscriberCount: new(int64(1234)),
	}
}

// addChannel ghi baseline đầu tiên (Add): hồ sơ + 3 video trong một tx.
func addChannel(t *testing.T, s *postgres.ScanStore, id string) (scan.Lease, scan.CommitPageResult) {
	t.Helper()
	next := testCursor(id)
	return addChannelTagged(t, s, id, "v", &next)
}

// addChannelTagged như addChannel với tiền tố id video riêng (id video là khoá
// toàn cục: hai kênh không thể cùng id) và cursor kế tiếp tuỳ chọn.
func addChannelTagged(t *testing.T, s *postgres.ScanStore, id, prefix string, next *youtube.ChannelCursor) (scan.Lease, scan.CommitPageResult) {
	t.Helper()
	ctx := context.Background()
	lease := scan.Lease{ChannelID: id, Token: uuidLike(1), Owner: "add", Until: time.Now().Add(time.Minute)}
	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos: []youtube.ScannedVideo{
			video(padID(prefix+"3"), 300), video(padID(prefix+"2"), 200), video(padID(prefix+"1"), 100),
		},
		NextCursor: next,
		Origin:     domain.OriginBaseline,
		Lease:      lease,
		Profile:    baseProfile(id),
		Now:        time.Now(),
	})
	if err != nil {
		t.Fatalf("commit baseline: %v", err)
	}
	return lease, res
}

// uuidLike sinh uuid ổn định cho test (không cần ngẫu nhiên).
func uuidLike(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func testCursor(channelID string) youtube.ChannelCursor {
	return youtube.ChannelCursor{
		Version:      1,
		ChannelID:    channelID,
		Tab:          "videos",
		Order:        "latest",
		Continuation: youtube.Continuation{Token: "tok", Request: "CONTINUATION_REQUEST_TYPE_BROWSE"},
		Session:      youtube.SessionContext{ClientName: "WEB", ClientVersion: "2.20260101.00.00"},
	}
}

func startHistory(t *testing.T, s *postgres.ScanStore, id string) *scan.Job {
	t.Helper()
	job, err := s.StartBackfill(context.Background(), id)
	if err != nil {
		t.Fatalf("start backfill: %v", err)
	}
	if job == nil {
		return nil
	}
	return job
}

func claim(t *testing.T, s *postgres.ScanStore, job *scan.Job, lease scan.Lease) *scan.Job {
	t.Helper()
	if job == nil {
		t.Fatal("không có việc để claim")
	}
	claimed, err := s.ClaimJob(context.Background(), job.ID, lease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	return claimed
}

// storedVideo là một hàng rival_videos đọc thẳng để so giá trị đã lưu.
type storedVideo struct {
	title      string
	position   int64
	origin     string
	views      *int64
	score      *float64
	firstSeen  time.Time
	updatedAt  time.Time
	uploadedAt *time.Time
	checkedAt  *time.Time
	captions   *bool
}

func readVideo(t *testing.T, pool *pgxpool.Pool, id string) storedVideo {
	t.Helper()
	var v storedVideo
	err := pool.QueryRow(context.Background(), `
		SELECT title, position, origin, view_count, score, first_seen_at, updated_at,
		       uploaded_at, upload_checked_at, has_captions
		FROM rival_videos WHERE id = $1`, id).
		Scan(&v.title, &v.position, &v.origin, &v.views, &v.score, &v.firstSeen, &v.updatedAt,
			&v.uploadedAt, &v.checkedAt, &v.captions)
	if err != nil {
		t.Fatalf("đọc video %s: %v", id, err)
	}
	return v
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("đếm: %v", err)
	}
	return n
}

// ── cổng chặn trong giao dịch (thứ tự sự kiện xác định) ───────────────────

// gate chặn đúng một lần câu SQL khớp chuỗi, ngay trong giao dịch của store —
// nhờ vậy test dựng được cảnh "đang giữ khoá, chưa ghi" mà không cần ngủ.
type gate struct {
	match   string
	reached chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func newGate(match string) *gate {
	return &gate{match: match, reached: make(chan struct{}), resume: make(chan struct{})}
}

func (g *gate) before(ctx context.Context, sql string) error {
	if g == nil || !strings.Contains(sql, g.match) {
		return nil
	}
	fired := false
	g.once.Do(func() {
		fired = true
		close(g.reached)
	})
	if !fired {
		return nil
	}
	select {
	case <-g.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// gatedDB bọc pool để mọi giao dịch đi qua một gate.
type gatedDB struct {
	postgres.DBTXer
	g *gate
}

func (d gatedDB) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := d.DBTXer.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return gatedTx{Tx: tx, g: d.g}, nil
}

type gatedTx struct {
	pgx.Tx
	g *gate
}

func (t gatedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := t.g.before(ctx, sql); err != nil {
		return nil, err
	}
	return t.Tx.Query(ctx, sql, args...)
}

func (t gatedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := t.g.before(ctx, sql); err != nil {
		return pgconn.CommandTag{}, err
	}
	return t.Tx.Exec(ctx, sql, args...)
}

// QueryRow cũng phải đi qua cổng: câu :one của sqlc dùng QueryRow (chạy khi
// Scan), nếu bỏ qua thì cảnh chen ngang không dựng được.
func (t gatedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := t.g.before(ctx, sql); err != nil {
		return failedRow{err}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

type failedRow struct{ err error }

func (r failedRow) Scan(...any) error { return r.err }

func gatedStore(pool *pgxpool.Pool, g *gate) *postgres.ScanStore {
	return postgres.NewScanStore(gatedDB{DBTXer: pool, g: g})
}

// ensureBlocked khẳng định một goroutine còn đang chờ (chưa trả về).
func ensureBlocked[T any](t *testing.T, done <-chan T, d time.Duration) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("thao tác phải bị chặn bởi khoá hàng, nhưng đã trả về")
	case <-time.After(d):
	}
}

// ── baseline ────────────────────────────────────────────────────────────────

func TestScanBaselineStoresProfileOrderAndContinuation(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("baseline")
	next := testCursor(id)
	remainder := []youtube.ScannedVideo{video(padID("vr"), 7)}
	lease := scan.Lease{ChannelID: id, Token: uuidLike(11), Owner: "add", Until: time.Now().Add(time.Minute)}
	now := time.Now().Truncate(time.Microsecond)
	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID:     id,
		Videos:        []youtube.ScannedVideo{video(padID("v3"), 300), video(padID("v2"), 200), video(padID("v1"), 100)},
		NextCursor:    &next,
		Remainder:     remainder,
		Origin:        domain.OriginBaseline,
		Lease:         lease,
		Profile:       baseProfile(id),
		NewestVideoID: new(padID("v3")),
		Now:           now,
	})
	if err != nil {
		t.Fatalf("commit baseline: %v", err)
	}
	if res.Added != 3 || res.Scanned != 3 || res.Overlap {
		t.Fatalf("added=%d scanned=%d overlap=%v", res.Added, res.Scanned, res.Overlap)
	}

	ch, err := s.Channel(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Title != "Kênh "+id || ch.Handle != "@"+strings.ToLower(id) || ch.Description != "kênh thử" {
		t.Fatalf("hồ sơ không được ghi đúng: %+v", ch)
	}
	if ch.URL != "https://www.youtube.com/channel/"+id || ch.SubscriberCount == nil || *ch.SubscriberCount != 1234 {
		t.Fatalf("hồ sơ thiếu url/subscriber: %+v", ch)
	}

	st, err := s.EnsureState(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st.IndexedVideos != 3 {
		t.Fatalf("indexed_videos = %d, muốn 3", st.IndexedVideos)
	}
	if len(st.HeadAnchors) != 3 || st.HeadAnchors[0] != padID("v3") || st.HeadAnchors[2] != padID("v1") {
		t.Fatalf("anchors phải theo thứ tự trang: %v", st.HeadAnchors)
	}
	if st.HeadNewestID != padID("v3") || st.HeadCheckedAt == nil {
		t.Fatalf("mốc head của trang đầu sai: newest=%q checked=%v", st.HeadNewestID, st.HeadCheckedAt)
	}

	// Bookmark của lần đọc đầu phải được lưu cùng trang — việc lịch sử đọc tiếp
	// đúng chỗ dừng, không quay lại đầu tab.
	wantCursor, err := youtube.SerializeChannelCursor(next)
	if err != nil {
		t.Fatal(err)
	}
	wantRemainder, err := youtube.SerializeScannedVideos(remainder)
	if err != nil {
		t.Fatal(err)
	}
	if st.HistoryCursor != wantCursor || st.HistoryRemainder != wantRemainder {
		t.Fatalf("bookmark chưa lưu: cursor=%q remainder=%q", st.HistoryCursor, st.HistoryRemainder)
	}

	// Vị trí/origin/first_seen của hàng đầu trang.
	v3 := readVideo(t, pool, padID("v3"))
	if v3.position != 0 || v3.origin != "baseline" || !v3.firstSeen.Equal(now) {
		t.Fatalf("hàng baseline sai vị trí/origin/first_seen: %+v", v3)
	}
	if v3.views == nil || *v3.views != 300 {
		t.Fatalf("lượt xem không được lưu: %+v", v3.views)
	}
}

func TestScanHistoryStartTakesOverBaselineContinuation(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("handoff")
	next := testCursor(id)
	addLease, _ := addChannelTagged(t, s, id, "v", &next)
	if err := s.ReleaseChannelLease(context.Background(), addLease); err != nil {
		t.Fatal(err)
	}

	job := startHistory(t, s, id)
	if job == nil {
		t.Fatal("phải có việc lịch sử")
	}
	wantCursor, _ := youtube.SerializeChannelCursor(next)
	if job.Cursor != wantCursor {
		t.Fatalf("việc lịch sử phải đọc tiếp cursor của lần đọc đầu: %q", job.Cursor)
	}
	st, err := s.EnsureState(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st.HistoryCursor != "" || st.HistoryRemainder != "" {
		t.Fatalf("bookmark phải chuyển sang việc rồi xoá khỏi state: %+v", st)
	}

	// Một trang discovery không được đụng tới bookmark lịch sử.
	lease := leaseOf(t, s, id, "worker")
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	djob, err := s.NextRunnableJob(ctx, nil)
	if err != nil || djob == nil || djob.Kind != domain.ScanDiscovery {
		t.Fatalf("thiếu việc discovery: %+v %v", djob, err)
	}
	claim(t, s, djob, lease)
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos:    []youtube.ScannedVideo{video(padID("dnew"), 5)},
		Origin:    domain.OriginDiscovery,
		Lease:     lease,
		JobID:     djob.ID,
		Now:       time.Now(),
	}); err != nil {
		t.Fatalf("commit discovery: %v", err)
	}
	var cursor *string
	if err := pool.QueryRow(ctx, `SELECT history_cursor FROM scan_state WHERE channel_id = $1`, id).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if cursor != nil {
		t.Fatalf("trang discovery ghi đè bookmark lịch sử: %q", *cursor)
	}
}

// TestScanRejectedBaselineRollsBackProfile: trang baseline hỏng (continuation
// không hợp lệ) phải cuốn theo cả hồ sơ vừa tạo trong cùng giao dịch.
func TestScanRejectedBaselineRollsBackProfile(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("badbase")
	bad := youtube.ChannelCursor{Version: 9}
	_, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID:  id,
		Videos:     []youtube.ScannedVideo{video(padID("bad1"), 10)},
		NextCursor: &bad,
		Origin:     domain.OriginBaseline,
		Lease:      scan.Lease{ChannelID: id, Token: uuidLike(31), Owner: "add", Until: time.Now().Add(time.Minute)},
		Profile:    baseProfile(id),
		Now:        time.Now(),
	})
	if err == nil {
		t.Fatal("continuation hỏng phải làm trang thất bại")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_channels WHERE id = $1`, id); n != 0 {
		t.Fatalf("hồ sơ của trang hỏng còn sót: %d", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE channel_id = $1`, id); n != 0 {
		t.Fatalf("video của trang hỏng còn sót: %d", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM scan_state WHERE channel_id = $1`, id); n != 0 {
		t.Fatalf("trạng thái của trang hỏng còn sót: %d", n)
	}
	// Kênh vẫn thêm được sau đó với một trang hợp lệ.
	lease, res := addChannel(t, s, id)
	if res.Added != 3 {
		t.Fatalf("added=%d", res.Added)
	}
	_ = lease
}

// ── video: upsert giữ danh tính, null không xoá ────────────────────────────

func TestScanPageKeepsIdentityAndKnownValues(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("immutable")
	lease, _ := addChannel(t, s, id)
	before := readVideo(t, pool, padID("v1"))
	newTitle := "v1 đổi tên"
	later := time.Now().Add(2 * time.Second)

	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos: []youtube.ScannedVideo{
			{ID: padID("v1"), Title: newTitle, ViewCount: nil}, // null không xoá view đã biết
			video(padID("v0"), 50),
		},
		Origin: domain.OriginHistory,
		Lease:  lease,
		Now:    later,
	})
	if err != nil {
		t.Fatalf("commit history: %v", err)
	}
	if res.Added != 1 || res.Scanned != 2 {
		t.Fatalf("added=%d scanned=%d, muốn 1/2", res.Added, res.Scanned)
	}
	after := readVideo(t, pool, padID("v1"))
	if after.title != newTitle {
		t.Fatalf("tiêu đề mới phải được ghi: %q", after.title)
	}
	if after.views == nil || *after.views != before.views2() {
		t.Fatalf("null của trang mới xoá lượt xem đã biết: %v", after.views)
	}
	if after.position != before.position || after.origin != before.origin {
		t.Fatalf("vị trí/origin phải bất biến: %+v → %+v", before, after)
	}
	if !after.firstSeen.Equal(before.firstSeen) {
		t.Fatalf("first_seen_at phải bất biến: %v → %v", before.firstSeen, after.firstSeen)
	}
	if !after.updatedAt.After(before.updatedAt) {
		t.Fatalf("updated_at phải mới hơn: %v → %v", before.updatedAt, after.updatedAt)
	}
	// Hàng mới nối sau cùng, score theo mức thường hiện tại (chưa có → null).
	v0 := readVideo(t, pool, padID("v0"))
	if v0.position != 3 || v0.origin != "history" {
		t.Fatalf("hàng mới sai vị trí/origin: %+v", v0)
	}
	if v0.score != nil {
		t.Fatalf("chưa có mức thường thì score phải null: %v", *v0.score)
	}
}

func (v storedVideo) views2() int64 {
	if v.views == nil {
		return -1
	}
	return *v.views
}

func TestScanForeignChannelIDNeverMoves(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	a := cid("ownera")
	b := cid("ownerb")
	leaseA, _ := addChannel(t, s, a)
	addChannelTagged(t, s, b, "b", nil)
	bBefore := readVideo(t, pool, padID("b3"))

	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: a,
		Videos: []youtube.ScannedVideo{
			{ID: padID("b3"), Title: "bị đè", ViewCount: new(int64(999))},
			video(padID("vA"), 1),
		},
		Origin: domain.OriginHistory,
		Lease:  leaseA,
		Now:    time.Now(),
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	bAfter := readVideo(t, pool, padID("b3"))
	if bAfter.title != bBefore.title || bAfter.position != bBefore.position ||
		bAfter.origin != bBefore.origin || *bAfter.views != *bBefore.views {
		t.Fatalf("hàng của kênh khác bị đụng: %+v → %+v", bBefore, bAfter)
	}
	stB, _ := s.EnsureState(ctx, b)
	if stB.IndexedVideos != 3 {
		t.Fatalf("kênh B bị đụng: indexed=%d", stB.IndexedVideos)
	}
}

// ── fence ───────────────────────────────────────────────────────────────────

// TestScanFenceMatrixRejectsWrongAndExpiredTokens kiểm từng đường ghi fenced:
// token sai hay fence hết hạn đều không được ghi gì.
func TestScanFenceMatrixRejectsWrongAndExpiredTokens(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("matrix")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	wrong := lease
	wrong.Token = uuidLike(4242)
	expired := lease
	expired.Until = time.Now().Add(-time.Second)

	expire := func() {
		if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() - interval '1 second' WHERE channel_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	restore := func() {
		if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() + interval '1 minute' WHERE channel_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name string
		call func(l scan.Lease) error
	}{
		{"backoff", func(l scan.Lease) error {
			return s.SetJobBackoff(ctx, job.ID, l, scan.Backoff{NextAttemptAt: time.Now().Add(time.Minute), Attempts: 1, ErrorCode: "transient"})
		}},
		{"error", func(l scan.Lease) error {
			return s.SetJobError(ctx, job.ID, l, scan.JobError{ErrorCode: "blocked"})
		}},
		{"capped", func(l scan.Lease) error { return s.SetJobCapped(ctx, job.ID, l) }},
		{"paused", func(l scan.Lease) error {
			return s.SetJobPaused(ctx, job.ID, l, scan.JobError{ErrorCode: "cursor_invalid"})
		}},
		{"complete", func(l scan.Lease) error { return s.CompleteJob(ctx, job.ID, l) }},
		{"restart", func(l scan.Lease) error { return s.RestartJobCursor(ctx, job.ID, l, errors.New("cursor hỏng")) }},
		{"claim", func(l scan.Lease) error {
			_, err := s.ClaimJob(ctx, job.ID, l)
			return err
		}},
		{"page", func(l scan.Lease) error {
			_, err := s.CommitPage(ctx, scan.CommitPageInput{
				ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("leak"), 1)},
				Origin: domain.OriginHistory, Lease: l, JobID: job.ID, Now: time.Now(),
			})
			return err
		}},
		{"head", func(l scan.Lease) error {
			_, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: l, JobID: job.ID, Now: time.Now()})
			return err
		}},
		{"upload", func(l scan.Lease) error {
			return s.RecordUploadTime(ctx, l, padID("v1"), new(time.Now()), true, time.Now())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+"_wrong_token", func(t *testing.T) {
			if err := tc.call(wrong); scan.StoreCodeOf(err) != scan.StoreLeaseLost {
				t.Fatalf("muốn lease_lost, được %v", err)
			}
			if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("leak")); n != 0 {
				t.Fatal("trang của fence sai đã ghi hàng")
			}
		})
		t.Run(tc.name+"_expired", func(t *testing.T) {
			expire()
			defer restore()
			if err := tc.call(expired); scan.StoreCodeOf(err) != scan.StoreLeaseLost {
				t.Fatalf("muốn lease_lost, được %v", err)
			}
		})
	}
	got, err := s.Job(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.ScanRunning || got.Attempts != 0 || got.ErrorCode != "" {
		t.Fatalf("việc bị đổi bởi lượt ghi sai fence: %+v", got)
	}
	v1 := readVideo(t, pool, padID("v1"))
	if v1.uploadedAt != nil || v1.checkedAt != nil {
		t.Fatal("lượt đọc giờ đăng của fence sai đã ghi")
	}
}

// TestScanFenceHeldDuringTransitionBlocksTakeover dựng đúng cảnh của lỗi
// TOCTOU cho từng đường chuyển trạng thái: A đã đọc fence hợp lệ và đang trong
// giao dịch ghi; B không thể chiếm fence cho tới khi giao dịch của A kết thúc.
func TestScanFenceHeldDuringTransitionBlocksTakeover(t *testing.T) {
	type transition struct {
		name  string
		match string
		// prepare dựng việc ở trạng thái mà bước ghi cần, trả việc + fence.
		prepare func(t *testing.T, s *postgres.ScanStore, id string) (*scan.Job, scan.Lease)
		call    func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, lease scan.Lease) error
	}
	claimed := func(t *testing.T, s *postgres.ScanStore, id string) (*scan.Job, scan.Lease) {
		lease, _ := addChannel(t, s, id)
		job := startHistory(t, s, id)
		claim(t, s, job, lease)
		return job, lease
	}
	cases := []transition{
		{"error", "SET state = 'error'", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.SetJobError(ctx, job.ID, l, scan.JobError{ErrorCode: "blocked"})
		}},
		{"backoff", "SET state = 'backoff'", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.SetJobBackoff(ctx, job.ID, l, scan.Backoff{NextAttemptAt: time.Now().Add(time.Minute), Attempts: 1, ErrorCode: "transient"})
		}},
		{"capped", "SET state = 'capped'", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.SetJobCapped(ctx, job.ID, l)
		}},
		{"paused", "SET state = 'paused'", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.SetJobPaused(ctx, job.ID, l, scan.JobError{ErrorCode: "cursor_invalid"})
		}},
		{"complete", "SET state = 'complete'", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.CompleteJob(ctx, job.ID, l)
		}},
		{"restart", "SET cursor = NULL", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.RestartJobCursor(ctx, job.ID, l, errors.New("cursor hỏng"))
		}},
		{"claim", "SET state = 'running'", func(t *testing.T, s *postgres.ScanStore, id string) (*scan.Job, scan.Lease) {
			lease, _ := addChannel(t, s, id)
			return startHistory(t, s, id), lease
		}, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			_, err := s.ClaimJob(ctx, job.ID, l)
			return err
		}},
		{"page", "FROM rival_videos WHERE id = ANY", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			_, err := s.CommitPage(ctx, scan.CommitPageInput{
				ChannelID: job.ChannelID, Videos: []youtube.ScannedVideo{video(padID("tp"), 10)},
				Origin: domain.OriginHistory, Lease: l, JobID: job.ID, Now: time.Now(),
			})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := provision(t)
			ctx := context.Background()
			id := cid("take_" + tc.name)
			base := postgres.NewScanStore(pool)
			job, lease := tc.prepare(t, base, id)

			g := newGate(tc.match)
			slow := gatedStore(pool, g)
			done := make(chan error, 1)
			go func() { done <- tc.call(ctx, slow, job, lease) }()
			select {
			case <-g.reached:
			case err := <-done:
				t.Fatalf("A kết thúc trước bước ghi (gate không chạm): %v", err)
			}

			// B (kết nối khác) muốn chiếm fence: phải chờ khoá hàng trạng thái.
			blockedCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
			defer cancel()
			gotLease, err := base.AcquireChannelLease(blockedCtx, id, "worker-b", time.Minute)
			if err == nil {
				t.Fatalf("chiếm được fence khi giao dịch khác đang giữ khoá: %+v", gotLease)
			}
			if scan.StoreCodeOf(err) != "" {
				t.Fatalf("lỗi phải là lỗi kết nối/huỷ ctx, không phải lỗi nghiệp vụ: %v", err)
			}
			// Đổi hạn lease cũng phải chờ khoá hàng trạng thái.
			shortCtx, cancel2 := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel2()
			if _, err := pool.Exec(shortCtx, `UPDATE scan_state SET lease_until = now() - interval '1 second' WHERE channel_id = $1`, id); err == nil {
				t.Fatal("UPDATE trạng thái lọt qua khi giao dịch khác đang giữ khoá")
			}

			close(g.resume)
			if err := <-done; err != nil {
				t.Fatalf("A phải ghi được (fence còn hiệu lực): %v", err)
			}

			// Sau khi A xong: fence cũ hết hạn, B chiếm thật; token cũ không ghi
			// được nữa.
			if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() - interval '1 second' WHERE channel_id = $1`, id); err != nil {
				t.Fatal(err)
			}
			if leaseB, err := base.AcquireChannelLease(ctx, id, "worker-b", time.Minute); err != nil || leaseB == nil {
				t.Fatalf("B phải chiếm được fence sau khi A xong: %v", err)
			}
			if err := base.RestartJobCursor(ctx, job.ID, lease, errors.New("cũ")); scan.StoreCodeOf(err) != scan.StoreLeaseLost {
				t.Fatalf("token cũ ghi được sau khi bị chiếm: %v", err)
			}
		})
	}
}

// TestScanTwoRealLeasesOnlyOnePageLands: hai fence đều do kho cấp thật (một
// cái hết hạn rồi được cấp lại); chỉ trang của fence đang hiệu lực được ghi.
func TestScanTwoRealLeasesOnlyOnePageLands(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("twoleases")
	addLease, _ := addChannel(t, s, id)
	if err := s.ReleaseChannelLease(ctx, addLease); err != nil {
		t.Fatal(err)
	}

	old, err := s.AcquireChannelLease(ctx, id, "worker-cũ", 300*time.Millisecond)
	if err != nil || old == nil {
		t.Fatalf("fence đầu: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	fresh, err := s.AcquireChannelLease(ctx, id, "worker-mới", time.Minute)
	if err != nil || fresh == nil {
		t.Fatalf("fence sau khi hết hạn: %v", err)
	}
	job := startHistory(t, s, id)

	// Hai lượt claim thật, song song: fence cũ phải thua.
	start := make(chan struct{})
	var wg sync.WaitGroup
	var claimedOld *scan.Job
	var errOld error
	var claimedFresh *scan.Job
	var errFresh error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		claimedOld, errOld = s.ClaimJob(ctx, job.ID, *old)
	}()
	go func() {
		defer wg.Done()
		<-start
		claimedFresh, errFresh = s.ClaimJob(ctx, job.ID, *fresh)
	}()
	close(start)
	wg.Wait()
	if scan.StoreCodeOf(errOld) != scan.StoreLeaseLost || claimedOld != nil {
		t.Fatalf("fence hết hạn vẫn claim được: %+v %v", claimedOld, errOld)
	}
	if errFresh != nil || claimedFresh == nil {
		t.Fatalf("fence mới phải claim được: %v", errFresh)
	}

	page := func(l scan.Lease, tag string) error {
		_, err := s.CommitPage(ctx, scan.CommitPageInput{
			ChannelID: id, Videos: []youtube.ScannedVideo{video(padID(tag), 10)},
			Origin: domain.OriginHistory, Lease: l, JobID: job.ID, Now: time.Now(),
		})
		return err
	}
	if err := page(*old, "oldpage"); scan.StoreCodeOf(err) != scan.StoreLeaseLost {
		t.Fatalf("trang của fence cũ phải bị từ chối: %v", err)
	}
	if err := page(*fresh, "newpage"); err != nil {
		t.Fatalf("trang của fence mới: %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("oldpage")); n != 0 {
		t.Fatal("trang của fence cũ đã ghi")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("newpage")); n != 1 {
		t.Fatal("trang của fence mới không được ghi")
	}
}

// ── gỡ kênh chèn ngang trang ───────────────────────────────────────────────

// TestScanDeleteDuringPageCommitSerializes: gỡ kênh và ghi trang khoá cùng thứ
// tự parent → state → job → video nên không deadlock; bên tới sau thấy kết quả
// của bên trước.
func TestScanDeleteDuringPageCommitSerializes(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	id := cid("delpage")
	base := postgres.NewScanStore(pool)
	lease, _ := addChannel(t, base, id)
	job := startHistory(t, base, id)
	claim(t, base, job, lease)

	// A đang ghi trang (đã giữ khoá parent + state), B gỡ kênh.
	g := newGate("FROM rival_videos WHERE id = ANY")
	slow := gatedStore(pool, g)
	pageErr := make(chan error, 1)
	go func() {
		_, err := slow.CommitPage(ctx, scan.CommitPageInput{
			ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("p1"), 10)},
			Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
		})
		pageErr <- err
	}()
	<-g.reached

	deleteErr := make(chan error, 1)
	go func() { deleteErr <- base.DeleteChannel(ctx, id) }()
	ensureBlocked(t, deleteErr, 150*time.Millisecond)
	close(g.resume)
	if err := <-pageErr; err != nil {
		t.Fatalf("trang phải ghi xong trước khi kênh bị gỡ: %v", err)
	}
	if err := <-deleteErr; err != nil {
		t.Fatalf("gỡ kênh sau trang: %v", err)
	}
	if _, err := base.Channel(ctx, id); scan.StoreCodeOf(err) != scan.StoreChannelMissing {
		t.Fatalf("kênh phải bị gỡ: %v", err)
	}
}

// TestScanPageAfterRemovedChannelReportsStopped: nếu kênh bị gỡ trước, trang
// đang chờ trả not-found sạch (không deadlock, không ghi gì).
func TestScanPageAfterRemovedChannelReportsStopped(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	id := cid("delwin")
	base := postgres.NewScanStore(pool)
	lease, _ := addChannel(t, base, id)
	job := startHistory(t, base, id)
	claim(t, base, job, lease)

	g := newGate("DELETE FROM rival_channels")
	slow := gatedStore(pool, g)
	deleteErr := make(chan error, 1)
	go func() { deleteErr <- slow.DeleteChannel(ctx, id) }()
	<-g.reached

	pageCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var page error
	pageDone := make(chan struct{})
	go func() {
		_, page = base.CommitPage(pageCtx, scan.CommitPageInput{
			ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("late"), 10)},
			Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
		})
		close(pageDone)
	}()
	ensureBlocked(t, pageDone, 150*time.Millisecond)
	close(g.resume)
	if err := <-deleteErr; err != nil {
		t.Fatalf("gỡ kênh: %v", err)
	}
	<-pageDone
	if scan.StoreCodeOf(page) != scan.StoreChannelMissing {
		t.Fatalf("trang của kênh đã gỡ phải là channel_not_found, được %v", page)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("late")); n != 0 {
		t.Fatal("trang sau khi gỡ vẫn ghi hàng")
	}
}

// ── thêm trùng ──────────────────────────────────────────────────────────────

// TestScanDuplicateBaselineRaceYieldsExists: hai lượt thêm cùng kênh song
// song — một thắng, một nhận channel_exists (để service trả 409 exists), dữ
// liệu của kênh chỉ đến từ lượt thắng.
func TestScanDuplicateBaselineRaceYieldsExists(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("dupadd")

	commit := func(token int, tag string) error {
		_, err := s.CommitPage(ctx, scan.CommitPageInput{
			ChannelID: id,
			Videos:    []youtube.ScannedVideo{video(padID(tag), 10)},
			Origin:    domain.OriginBaseline,
			Lease:     scan.Lease{ChannelID: id, Token: uuidLike(token), Owner: fmt.Sprintf("add-%d", token), Until: time.Now().Add(time.Minute)},
			Profile:   baseProfile(id),
			Now:       time.Now(),
		})
		return err
	}
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range 2 {
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = commit(100+i, fmt.Sprintf("dup%d", i))
		}(i)
	}
	close(start)
	wg.Wait()

	okCount, existsCount := 0, 0
	for _, err := range errs {
		switch scan.StoreCodeOf(err) {
		case "":
			if err != nil {
				t.Fatalf("lỗi không xác định: %v", err)
			}
			okCount++
		case scan.StoreChannelExists:
			existsCount++
		default:
			t.Fatalf("muốn channel_exists, được %v", err)
		}
	}
	if okCount != 1 || existsCount != 1 {
		t.Fatalf("ok=%d exists=%d, muốn 1/1", okCount, existsCount)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE channel_id = $1`, id); n != 1 {
		t.Fatalf("kênh phải chỉ có dữ liệu của lượt thắng: %d hàng", n)
	}
	st, err := s.EnsureState(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st.IndexedVideos != 1 {
		t.Fatalf("indexed=%d, muốn 1", st.IndexedVideos)
	}
}

// ── discovery: khối khoá cố định ───────────────────────────────────────────

func TestScanDiscoveryPagesKeepOrderAndPositions(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("discpages")
	lease, _ := addChannel(t, s, id)

	created, err := s.StartDiscovery(ctx, id, false)
	if err != nil || !created {
		t.Fatalf("start discovery: created=%v err=%v", created, err)
	}
	again, err := s.StartDiscovery(ctx, id, false)
	if err != nil || again {
		t.Fatalf("lần hai phải không tạo: created=%v err=%v", again, err)
	}
	job, err := s.NextRunnableJob(ctx, nil)
	if err != nil || job == nil || job.Kind != domain.ScanDiscovery {
		t.Fatalf("runnable phải là discovery: %+v", job)
	}
	if job.OrderBase == nil || *job.OrderBase != -int64(scan.DiscoveryBlock) {
		t.Fatalf("khối khoá phải được khoá trước một lần: %+v", job.OrderBase)
	}
	base := *job.OrderBase
	claim(t, s, job, lease)

	page1 := []youtube.ScannedVideo{video(padID("v4"), 400), video(padID("v3"), 300)}
	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: page1, NextCursor: cursorPtr(testCursor(id)),
		Origin: domain.OriginDiscovery, Lease: lease, JobID: job.ID,
		NewestVideoID: new(padID("v4")), Now: time.Now(),
	})
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	if !res.Overlap || res.Added != 1 || res.NextOrdinal != 2 {
		t.Fatalf("trang 1: overlap=%v added=%d ordinal=%d", res.Overlap, res.Added, res.NextOrdinal)
	}

	// Trang 2 trong cùng run: không đánh số lại, không đụng khoá duy nhất.
	res2, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID:  id,
		Videos:     []youtube.ScannedVideo{video(padID("v6"), 600), video(padID("v5"), 500)},
		NextCursor: cursorPtr(testCursor(id)),
		Origin:     domain.OriginDiscovery, Lease: lease, JobID: job.ID,
		NewestVideoID: new(padID("v6")), Now: time.Now(),
	})
	if err != nil {
		t.Fatalf("trang 2 (không được tái đánh số): %v", err)
	}
	if res2.Overlap || res2.Added != 2 || res2.NextOrdinal != 4 {
		t.Fatalf("trang 2: overlap=%v added=%d ordinal=%d", res2.Overlap, res2.Added, res2.NextOrdinal)
	}
	if got := readVideo(t, pool, padID("v4")); got.position != base {
		t.Fatalf("hàng trang 1 đổi vị trí: %d, muốn %d", got.position, base)
	}
	if got := readVideo(t, pool, padID("v3")); got.position != 0 || got.origin != "baseline" {
		t.Fatalf("hàng đã biết phải giữ vị trí/origin cũ: %+v", got)
	}
	if got := readVideo(t, pool, padID("v6")); got.position != base+2 || got.origin != "discovery" {
		t.Fatalf("hàng trang 2 sai vị trí: %d, muốn %d", got.position, base+2)
	}
	if got := readVideo(t, pool, padID("v5")); got.position != base+3 {
		t.Fatalf("hàng trang 2 sai vị trí: %d", got.position)
	}
	mid, _ := s.Job(ctx, job.ID)
	if mid.OrderBase == nil || *mid.OrderBase != base || mid.NextOrdinal != 4 {
		t.Fatalf("run phải giữ nguyên khối: base=%v ordinal=%d", mid.OrderBase, mid.NextOrdinal)
	}
}

func cursorPtr(c youtube.ChannelCursor) *youtube.ChannelCursor { return &c }

// TestScanDiscoveryOrdinalBound: khối 10.000 khoá là trần cứng của một run.
func TestScanDiscoveryOrdinalBound(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("discbound")
	lease, _ := addChannel(t, s, id)
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	job, _ := s.NextRunnableJob(ctx, nil)
	claim(t, s, job, lease)
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET next_ordinal = 9995 WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	var page []youtube.ScannedVideo
	for i := range 5 {
		page = append(page, video(fmt.Sprintf("edge%07d", i), 1))
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: page, Origin: domain.OriginDiscovery, Lease: lease, JobID: job.ID, Now: time.Now(),
	}); err != nil {
		t.Fatalf("trang cuối khối phải ghi được: %v", err)
	}
	_, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: []youtube.ScannedVideo{video("overflow001", 1)},
		Origin: domain.OriginDiscovery, Lease: lease, JobID: job.ID, Now: time.Now(),
	})
	if scan.StoreCodeOf(err) != scan.StoreKeyRange {
		t.Fatalf("muốn key_range khi vượt 10.000, được %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, "overflow001"); n != 0 {
		t.Fatal("trang vượt trần vẫn ghi hàng")
	}
}

// TestScanDiscoveryRebaseOnlyWhenNearSafeBound: dồn thứ tự đúng một lần lúc
// mở run khi khối sắp chạm miền an toàn (không bao giờ giữa các trang).
func TestScanDiscoveryRebaseOnlyWhenNearSafeBound(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("discsafe")
	addChannel(t, s, id)
	// Đẩy vị trí sát miền an toàn: min - 10.000 < -KeyLimit.
	near := -scan.KeyLimit + 5
	if _, err := pool.Exec(ctx, `
		UPDATE rival_videos SET position = $2 + (position - (SELECT min(position) FROM rival_videos WHERE channel_id = $1))
		WHERE channel_id = $1`, id, near); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	job, _ := s.NextRunnableJob(ctx, nil)
	if job.OrderBase == nil || *job.OrderBase != -int64(scan.DiscoveryBlock) {
		t.Fatalf("khối mới phải bắt đầu ngay dưới 0 sau khi dồn: %v", job.OrderBase)
	}
	var min int64
	if err := pool.QueryRow(ctx, `SELECT min(position) FROM rival_videos WHERE channel_id = $1`, id).Scan(&min); err != nil {
		t.Fatal(err)
	}
	if min != 0 {
		t.Fatalf("thứ tự phải được dồn về 0..n-1, min=%d", min)
	}
}

// TestScanStartDiscoveryReplacesStoppedRun: việc đã huỷ không chặn lượt kiểm
// tra sau; việc tạm dừng/lỗi chỉ được mở lại khi người dùng yêu cầu.
func TestScanStartDiscoveryReplacesStoppedRun(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("discstop")
	lease, _ := addChannel(t, s, id)

	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	first, _ := s.NextRunnableJob(ctx, nil)

	// Huỷ: kênh vẫn đến hạn nên lịch phải xếp được lượt mới.
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'cancelled' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET head_checked_at = now() - interval '3 hours' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueChannels(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0] != id {
		t.Fatalf("kênh đến hạn phải được chọn lại sau khi huỷ: %v", due)
	}
	created, err := s.StartDiscovery(ctx, id, false)
	if err != nil || !created {
		t.Fatalf("lượt kiểm tra sau khi huỷ: created=%v err=%v", created, err)
	}
	now, _ := s.NextRunnableJob(ctx, nil)
	if now == nil || now.ID == first.ID {
		t.Fatalf("phải có run mới runnable: %+v", now)
	}

	// Tạm dừng: lịch tự động không đụng, người dùng thì mở lại.
	if err := s.ControlBackfill(ctx, id, domain.ScanActionPause); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'paused' WHERE id = $1`, now.ID); err != nil {
		t.Fatal(err)
	}
	_ = lease
	due, err = s.DueChannels(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("việc discovery tạm dừng không được coi là chỗ trống cho lịch: %v", due)
	}
	if created, err = s.StartDiscovery(ctx, id, false); err != nil || created {
		t.Fatalf("lịch tự động không được mở lại việc tạm dừng: created=%v err=%v", created, err)
	}
	if created, err = s.StartDiscovery(ctx, id, true); err != nil || !created {
		t.Fatalf("người dùng yêu cầu phải có lượt mới: created=%v err=%v", created, err)
	}
	old, _ := s.Job(ctx, now.ID)
	if old.State != domain.ScanCancelled {
		t.Fatalf("lượt tạm dừng phải bị bỏ hẳn: %s", old.State)
	}
	restarted, err := s.NextRunnableJobForChannel(ctx, id)
	if err != nil || restarted == nil || restarted.ID == now.ID || restarted.NextOrdinal != 0 {
		t.Fatalf("phải có run mới runnable từ ordinal 0: %+v %v", restarted, err)
	}
}

// ── head check ──────────────────────────────────────────────────────────────

func TestScanHeadCheckKeepsRunningRequirementAndExactMedian(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("median")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	// Kênh đã có 3 hàng baseline [100,200,300]; thêm [10,20,30,40,50,1000] và
	// một hàng chưa có lượt xem (rơi khỏi mẫu trung vị nhưng vẫn trong 150 hàng
	// mới nhất). 9 giá trị đã biết xếp lại: trung vị = 50.
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos: []youtube.ScannedVideo{
			video("m0100000000", 10), video("m0200000000", 20), video("m0300000000", 30),
			video("m0400000000", 40), video("m0500000000", 50), video("m1000000000", 1000),
			{ID: "mnull000000", Title: "chưa có lượt xem", ThumbnailURL: "https://i.ytimg.com/vi/mnull000000/mqdefault.jpg"},
		},
		Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	res, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: lease, Now: time.Now()})
	if err != nil {
		t.Fatalf("head check: %v", err)
	}
	if res.MedianViews == nil || *res.MedianViews != 50 {
		t.Fatalf("trung vị = %v, muốn 50", res.MedianViews)
	}
	ch, _ := s.Channel(ctx, id)
	if ch.MedianViews == nil || *ch.MedianViews != 50 {
		t.Fatalf("mức thường chưa ghi: %v", ch.MedianViews)
	}
	if got := readVideo(t, pool, "m0500000000"); got.score == nil || *got.score != 1 {
		t.Fatalf("score = %v, muốn 50/50", got.score)
	}
	if got := readVideo(t, pool, "mnull000000"); got.score != nil {
		t.Fatalf("hàng không có lượt xem phải có score null: %v", *got.score)
	}

	// Việc phải còn running mới được kết thúc cùng head check.
	if err := s.ReleaseChannelLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	discoveryLease := leaseOf(t, s, id, "worker")
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	djob, _ := s.NextRunnableJob(ctx, nil)
	claim(t, s, djob, discoveryLease)
	if err := s.SetJobPaused(ctx, djob.ID, discoveryLease, scan.JobError{ErrorCode: "cursor_invalid"}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Channel(ctx, id)
	_, err = s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{
		CheckedAt: time.Now().Add(time.Minute), Lease: discoveryLease, JobID: djob.ID, Now: time.Now(),
	})
	if scan.StoreCodeOf(err) != scan.StoreJobStopped {
		t.Fatalf("head check của việc đã dừng phải là job_stopped, được %v", err)
	}
	after, _ := s.Channel(ctx, id)
	if !after.ScannedAt.Equal(*before.ScannedAt) {
		t.Fatal("head check hỏng vẫn ghi mốc quét")
	}
}

// TestScanHeadCheckWritesOnlyChangedScores: mức thường không đổi → không hàng
// điểm nào bị ghi lại; mức đổi → chỉ hàng có điểm khác bị ghi.
func TestScanHeadCheckWritesOnlyChangedScores(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("scorewrite")
	lease, _ := addChannel(t, s, id)
	var page []youtube.ScannedVideo
	for i := 1; i <= 12; i++ {
		page = append(page, video(fmt.Sprintf("s%010d", i), int64(i*100)))
	}
	for i := 1; i <= 3; i++ {
		v := fmt.Sprintf("n%010d", i)
		page = append(page, youtube.ScannedVideo{
			ID: v, Title: "chưa có lượt xem", ThumbnailURL: "https://i.ytimg.com/vi/" + v + "/mqdefault.jpg",
		})
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: page, Origin: domain.OriginHistory, Lease: lease, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: lease, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	before := xmins(t, pool, id)
	if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: lease, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if n := changedRows(before, xmins(t, pool, id)); n != 0 {
		t.Fatalf("mức thường không đổi mà ghi lại %d hàng điểm", n)
	}

	// Đổi vài lượt xem làm mức thường đổi hẳn: chỉ hàng lệch điểm bị ghi.
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET view_count = view_count * 3 WHERE channel_id = $1 AND position % 3 = 0`, id); err != nil {
		t.Fatal(err)
	}
	var medianBefore *float64
	var v *float64
	if err := pool.QueryRow(ctx, `SELECT median_views::float8 FROM rival_channels WHERE id = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	medianBefore = v
	// Mức thường mới tính trước, rồi mới đếm số hàng đang lệch điểm trên dữ
	// liệu cũ (sau head check thì hàng nào cũng đã khớp).
	var newMedian *int64
	{
		rows, err := pool.Query(ctx, `
			SELECT view_count FROM rival_videos WHERE channel_id = $1
			ORDER BY position, id LIMIT 150`, id)
		if err != nil {
			t.Fatal(err)
		}
		var values []*float64
		for rows.Next() {
			var v *int64
			if err := rows.Scan(&v); err != nil {
				t.Fatal(err)
			}
			if v != nil {
				f := float64(*v)
				values = append(values, &f)
			}
		}
		rows.Close()
		if m := youtube.Median(values); m != nil {
			newMedian = new(int64(*m + 0.5))
		}
	}
	if newMedian == nil || (medianBefore != nil && int64(*medianBefore) == *newMedian) {
		t.Fatalf("mức thường phải đổi sau khi nhân lượt xem: %v", newMedian)
	}
	before = xmins(t, pool, id)
	stale := countRows(t, pool, `
		SELECT count(*) FROM rival_videos
		WHERE channel_id = $1
		  AND score IS DISTINCT FROM CASE WHEN $2::bigint > 0 AND view_count IS NOT NULL
		      THEN view_count::double precision / $2::double precision ELSE NULL END`,
		id, *newMedian)
	if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: lease, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	var medianAfter int64
	if err := pool.QueryRow(ctx, `SELECT median_views FROM rival_channels WHERE id = $1`, id).Scan(&medianAfter); err != nil {
		t.Fatal(err)
	}
	if medianAfter != *newMedian {
		t.Fatalf("mức thường ghi %d, muốn %d", medianAfter, *newMedian)
	}
	want := stale
	got := changedRows(before, xmins(t, pool, id))
	if got != want {
		t.Fatalf("ghi lại %d hàng, số hàng lệch điểm là %d", got, want)
	}
	if got == 0 || got >= len(before) {
		t.Fatalf("số hàng bị ghi lại không phân biệt được: %d/%d", got, len(before))
	}
}

// TestScanHeadCheckCost10000 đo chi phí head check trên kênh 10.000 hàng: mức
// thường không đổi phải rẻ hơn hẳn kiểu ghi lại cả kênh (số liệu cho báo cáo,
// không đặt ngưỡng cứng vì máy khác nhau).
func TestScanHeadCheckCost10000(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("headcost")
	lease, _ := addChannel(t, s, id)
	if _, err := pool.Exec(ctx, `
		INSERT INTO rival_videos (id, channel_id, title, view_count, position, origin)
		SELECT 'c' || lpad(i::text, 10, '0'), $1, 'hàng ' || i, (i % 97) * 1000, 100 + i, 'history'
		FROM generate_series(0, 9999) i`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET indexed_videos = 10003 WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	head := func() time.Duration {
		start := time.Now()
		if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: lease, Now: time.Now()}); err != nil {
			t.Fatalf("head check: %v", err)
		}
		return time.Since(start)
	}
	first := head()
	beforeXmins := xmins(t, pool, id)

	// Đối chứng: đúng câu UPDATE cả kênh của đường cũ (không lọc hàng lệch).
	var median int64
	if err := pool.QueryRow(ctx, `SELECT median_views FROM rival_channels WHERE id = $1`, id).Scan(&median); err != nil {
		t.Fatal(err)
	}
	legacyStart := time.Now()
	if _, err := pool.Exec(ctx, `
		UPDATE rival_videos
		SET score = CASE WHEN $2::bigint > 0 AND view_count IS NOT NULL
		                 THEN view_count::double precision / $2::double precision ELSE NULL END
		WHERE channel_id = $1`, id, median); err != nil {
		t.Fatal(err)
	}
	legacy := time.Since(legacyStart)
	afterXmins := xmins(t, pool, id)
	unchanged := changedRows(beforeXmins, afterXmins)

	second := head()
	if n := changedRows(afterXmins, xmins(t, pool, id)); n != 0 {
		t.Fatalf("head check mức không đổi ghi lại %d hàng", n)
	}
	t.Logf("10.000 hàng: head check đầu=%v, head check mức không đổi=%v, UPDATE cả kênh kiểu cũ=%v, hàng kiểu cũ ghi lại=%d",
		first, second, legacy, unchanged)
}

// xmins chụp xmin của mọi hàng video (đổi xmin = hàng bị ghi lại).
func xmins(t *testing.T, pool *pgxpool.Pool, channelID string) map[string]string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT id, xmin::text FROM rival_videos WHERE channel_id = $1`, channelID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, xmin string
		if err := rows.Scan(&id, &xmin); err != nil {
			t.Fatal(err)
		}
		out[id] = xmin
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func changedRows(before, after map[string]string) int {
	n := 0
	for id, xmin := range after {
		if before[id] != xmin {
			n++
		}
	}
	return n
}

// TestScanHistoryEndStartIsNoop: tab đã hết thì không tạo việc.
func TestScanHistoryEndStartIsNoop(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("ended")
	addChannel(t, s, id)
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET history_end = true WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	job, err := s.StartBackfill(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if job != nil {
		t.Fatalf("tab đã hết không được tạo việc: %+v", job)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM scan_jobs WHERE channel_id = $1`, id); n != 0 {
		t.Fatalf("không được chèn việc nào: %d", n)
	}
}

// TestScanStateInitializesFromExistingRows: trạng thái tạo lười đọc từ dữ liệu
// đã có (đếm, anchors, video mới nhất) — không phải từ trang vừa ghi.
func TestScanStateInitializesFromExistingRows(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("initstate")
	if _, err := pool.Exec(ctx, `
		INSERT INTO rival_channels (id, title, url) VALUES ($1, 'Kênh cũ', 'https://x')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO rival_videos (id, channel_id, title, view_count, position, origin) VALUES
		('i0000000001', $1, 'a', 10, 2, 'history'),
		('i0000000002', $1, 'b', 20, 0, 'baseline'),
		('i0000000003', $1, 'c', 30, 1, 'baseline')`, id); err != nil {
		t.Fatal(err)
	}
	st, err := s.EnsureState(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st.IndexedVideos != 3 {
		t.Fatalf("indexed=%d, muốn 3", st.IndexedVideos)
	}
	if len(st.HeadAnchors) != 3 || st.HeadAnchors[0] != "i0000000002" || st.HeadAnchors[1] != "i0000000003" {
		t.Fatalf("anchors phải theo thứ tự vị trí: %v", st.HeadAnchors)
	}
	if st.HeadNewestID != "i0000000002" {
		t.Fatalf("head_newest = %q", st.HeadNewestID)
	}
	if st.HistoryEnd || st.HistoryCap != domain.HistoryCapDefault {
		t.Fatalf("mặc định trạng thái sai: %+v", st)
	}
}

// ── việc: claim, huỷ, restart cursor ───────────────────────────────────────

func TestScanStoppedJobRejectsWholePage(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("stopped")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	if err := s.ControlBackfill(ctx, id, domain.ScanActionCancel); err != nil {
		t.Fatal(err)
	}
	_, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos:    []youtube.ScannedVideo{video(padID("cancelpage"), 1)},
		Origin:    domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
	})
	if scan.StoreCodeOf(err) != scan.StoreJobStopped {
		t.Fatalf("muốn job_stopped, được %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("cancelpage")); n != 0 {
		t.Fatal("trang của việc đã huỷ vẫn ghi")
	}
	st, _ := s.EnsureState(ctx, id)
	if st.IndexedVideos != 3 {
		t.Fatalf("trạng thái bị đổi bởi trang bị từ chối: %d", st.IndexedVideos)
	}
}

func TestScanClaimRequiresRunnableJob(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("claimonly")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	if err := s.ControlBackfill(ctx, id, domain.ScanActionPause); err != nil {
		t.Fatal(err)
	}
	got, err := s.ClaimJob(ctx, job.ID, lease)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("việc đã tạm dừng không được claim: %+v", got)
	}
	// Hàng ảo không thuộc kênh của fence.
	if _, err := s.ClaimJob(ctx, uuidLike(99), lease); scan.StoreCodeOf(err) != scan.StoreJobStopped {
		t.Fatalf("việc không tồn tại phải là job_stopped, được %v", err)
	}
}

func TestScanCursorRestartClearsContinuation(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("restartcur")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	cur := testCursor(id)
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("r1"), 1)},
		NextCursor: &cur, Remainder: []youtube.ScannedVideo{video(padID("r2"), 2)},
		Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RestartJobCursor(ctx, job.ID, lease, errors.New("continuation hỏng")); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Job(ctx, job.ID)
	if got.Cursor != "" || got.Remainder != "" {
		t.Fatalf("cursor/remainder phải bị xoá: %q %q", got.Cursor, got.Remainder)
	}
	if got.ReplayTarget != padID("r1") || got.Restarts != 1 {
		t.Fatalf("replay_target/restarts sai: %q %d", got.ReplayTarget, got.Restarts)
	}
	if got.Pages != 1 || got.Scanned != 1 || got.Added != 1 {
		t.Fatalf("tiến độ phải giữ nguyên: pages=%d scanned=%d added=%d", got.Pages, got.Scanned, got.Added)
	}
	// Trang phát lại gặp id đích → tắt replay_target.
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("r1"), 1)},
		Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Job(ctx, job.ID)
	if got.ReplayTarget != "" {
		t.Fatalf("replay_target phải tắt khi thấy lại id đích: %q", got.ReplayTarget)
	}
}

func TestScanPageAndRemainderLimits(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("pagelimit")
	lease, _ := addChannel(t, s, id)
	var tooMany []youtube.ScannedVideo
	for i := 0; i <= scan.PageMax; i++ {
		tooMany = append(tooMany, video(fmt.Sprintf("lim%08d", i), 1))
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: tooMany, Origin: domain.OriginHistory, Lease: lease, Now: time.Now(),
	}); scan.StoreCodeOf(err) != scan.StoreLimit {
		t.Fatalf("trang vượt trần phải là limit, được %v", err)
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Remainder: tooMany, Origin: domain.OriginHistory, Lease: lease, Now: time.Now(),
	}); scan.StoreCodeOf(err) != scan.StoreLimit {
		t.Fatalf("remainder vượt trần phải là limit, được %v", err)
	}
}

// TestScanRunnableSelectionCoversStoppedAndTargetChannel.
func TestScanRunnableSelectionCoversStoppedAndTargetChannel(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	h := cid("runnableh")
	d := cid("runnabled")
	leaseH, _ := addChannel(t, s, h)
	addChannel(t, s, d)
	job := startHistory(t, s, h)
	claim(t, s, job, leaseH)
	if err := s.SetJobBackoff(ctx, job.ID, leaseH, scan.Backoff{
		NextAttemptAt: time.Now().Add(time.Hour), Attempts: 1, ErrorCode: "transient",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartDiscovery(ctx, d, false); err != nil {
		t.Fatal(err)
	}
	next, err := s.NextRunnableJob(ctx, nil)
	if err != nil || next == nil || next.Kind != domain.ScanDiscovery {
		t.Fatalf("muốn discovery trước, được %+v", next)
	}
	// Lọc theo kênh: kênh d đứng trước trong hàng đợi nhưng hỏi kênh h vẫn ra
	// việc discovery của chính h sau khi mở.
	if got, err := s.NextRunnableJobForChannel(ctx, d); err != nil || got == nil || got.ChannelID != d {
		t.Fatalf("lọc theo kênh sai: %+v %v", got, err)
	}
	if got, err := s.NextRunnableJobForChannel(ctx, h); err != nil || got != nil {
		t.Fatalf("kênh h chỉ có backoff chưa tới hạn: %+v %v", got, err)
	}
	if _, err := s.StartDiscovery(ctx, h, false); err != nil {
		t.Fatal(err)
	}
	if got, err := s.NextRunnableJobForChannel(ctx, h); err != nil || got == nil || got.ChannelID != h {
		t.Fatalf("kênh h phải có việc chạy được của chính nó: %+v %v", got, err)
	}
	// Bỏ qua hết kênh có việc → nil.
	if got, err := s.NextRunnableJob(ctx, map[string]bool{h: true, d: true}); err != nil || got != nil {
		t.Fatalf("skip hết phải trả nil: %+v %v", got, err)
	}
	// Việc đã tạm dừng không runnable.
	if err := s.ReleaseChannelLease(ctx, leaseH); err != nil {
		t.Fatal(err)
	}
	workerLease := leaseOf(t, s, h, "worker-h")
	got, err := s.NextRunnableJobForChannel(ctx, h)
	if err != nil || got == nil {
		t.Fatalf("việc discovery của kênh h: %+v %v", got, err)
	}
	claim(t, s, got, workerLease)
	if err := s.SetJobPaused(ctx, got.ID, workerLease, scan.JobError{ErrorCode: "cursor_invalid"}); err != nil {
		t.Fatal(err)
	}
	if r, err := s.NextRunnableJobForChannel(ctx, h); err != nil || r != nil {
		t.Fatalf("việc tạm dừng không được chọn: %+v %v", r, err)
	}
}

// ── lịch: một lần đọc cho vòng rảnh ───────────────────────────────────────

// TestScanWakeupSeparatesActionableFromFuture: một lần đọc phải nói được "có
// việc làm ngay" (kể cả việc do process khác tạo) tách khỏi "hạn kế tiếp".
func TestScanWakeupSeparatesActionableFromFuture(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("wakeup")
	lease, _ := addChannel(t, s, id)
	// Mẫu thăm dò giờ đăng đã đọc hết: chỉ còn lịch quét/việc quét trong phép thử.
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET upload_checked_at = now() WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	wakeup := func() scan.Wakeup {
		t.Helper()
		w, err := s.NextWakeup(ctx, 1, time.Now())
		if err != nil {
			t.Fatalf("next wakeup: %v", err)
		}
		return w
	}

	if w := wakeup(); w.Actionable || w.Next != nil {
		t.Fatalf("kênh vừa quét, không có việc: %+v", w)
	}

	// Việc mới xếp hàng (như process khác vừa tạo) trên kênh không bị fence →
	// có việc ngay.
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseChannelLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); !w.Actionable {
		t.Fatalf("việc queued phải là actionable: %+v", w)
	}
	job, _ := s.NextRunnableJob(ctx, nil)
	lease = leaseOf(t, s, id, "worker")
	claim(t, s, job, lease)

	// Backoff ở tương lai: chưa làm được gì, nhưng có hạn để ngủ tới.
	due := time.Now().Add(30 * time.Minute).Truncate(time.Millisecond)
	if err := s.SetJobBackoff(ctx, job.ID, lease, scan.Backoff{NextAttemptAt: due, Attempts: 1, ErrorCode: "transient"}); err != nil {
		t.Fatal(err)
	}
	// Tắt hạn lease để chỉ còn một mốc.
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = NULL WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	w := wakeup()
	if w.Actionable || w.Next == nil || w.Next.Sub(due).Abs() > time.Second {
		t.Fatalf("backoff tương lai: %+v (muốn next=%v)", w, due)
	}

	// Backoff đã tới hạn → có việc ngay.
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET next_attempt_at = now() - interval '1 second' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); !w.Actionable {
		t.Fatalf("backoff quá hạn phải actionable: %+v", w)
	}

	// Dòng running còn lại của chủ cũ mà fence kênh vẫn còn hạn: chưa claim
	// được, hạn thức là lúc fence hết.
	until := time.Now().Add(3 * time.Minute).Truncate(time.Millisecond)
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'running' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = $2 WHERE channel_id = $1`, id, until); err != nil {
		t.Fatal(err)
	}
	w = wakeup()
	if w.Actionable {
		t.Fatalf("việc running bị fence còn hạn không phải việc sẵn sàng: %+v", w)
	}
	if w.Next == nil || w.Next.Sub(until).Abs() > time.Second {
		t.Fatalf("hạn phải là lúc fence hết: %+v (muốn %v)", w.Next, until)
	}
	// Fence hết hạn thì dòng running claim được ngay.
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = NULL WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); !w.Actionable {
		t.Fatalf("fence trống thì việc running phải actionable: %+v", w)
	}

	// Việc xong, rescan đã quá hạn → có việc ngay.
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'complete' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE scan_state SET head_checked_at = now() - interval '3 hours', lease_until = NULL
		WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); !w.Actionable {
		t.Fatalf("kênh quá hạn rescan phải actionable: %+v", w)
	}

	// Rescan còn ở tương lai → chỉ có hạn ngủ.
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET head_checked_at = now() - interval '30 minutes' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	w = wakeup()
	if w.Actionable || w.Next == nil || w.Next.After(time.Now().Add(31*time.Minute)) {
		t.Fatalf("rescan tương lai: %+v", w)
	}

	// Run discovery còn sống thì kênh không tính vào hạn rescan (không spin).
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET head_checked_at = now() - interval '3 hours' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'paused' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); w.Actionable || w.Next != nil {
		t.Fatalf("việc discovery tạm dừng không được tạo hạn: %+v", w)
	}

	// Hết nghỉ thăm dò giờ đăng: chưa tới hạn thì ngủ tới mốc, tới hạn thì làm.
	if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET state = 'complete' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET head_checked_at = now() WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET upload_checked_at = NULL WHERE channel_id = $1 AND position = 0`, id); err != nil {
		t.Fatal(err)
	}
	pause := time.Now().Add(10 * time.Minute).Truncate(time.Millisecond)
	if err := s.PauseUploads(ctx, pause); err != nil {
		t.Fatal(err)
	}
	w = wakeup()
	if w.Actionable || w.Next == nil || w.Next.Sub(pause).Abs() > time.Second {
		t.Fatalf("nghỉ thăm dò tương lai: %+v (muốn %v)", w, pause)
	}
	// PauseUploads chỉ kéo dài mốc, nên hạ mốc về quá khứ bằng SQL.
	if _, err := pool.Exec(ctx, `UPDATE scan_schedule SET upload_times_next_at = now() - interval '1 minute' WHERE id`); err != nil {
		t.Fatal(err)
	}
	if w := wakeup(); !w.Actionable {
		t.Fatalf("hết nghỉ thăm dò phải actionable: %+v", w)
	}
}

// ── thăm dò giờ đăng ───────────────────────────────────────────────────────

func TestScanUploadCandidateIsBoundedToNewestSample(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("uploads")
	lease, _ := addChannel(t, s, id)
	var page []youtube.ScannedVideo
	for i := range 38 {
		page = append(page, video(fmt.Sprintf("u%010d", i), int64(i+1)))
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: page, Origin: domain.OriginHistory, Lease: lease, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_schedule SET upload_times_next_at = 'epoch' WHERE id`); err != nil {
		t.Fatal(err)
	}
	// Đánh dấu 30 hàng mới nhất đã đọc (3 baseline + 38 trang xếp sau).
	if _, err := pool.Exec(ctx, `
		UPDATE rival_videos SET upload_checked_at = now()
		WHERE channel_id = $1 AND position < 30`, id); err != nil {
		t.Fatal(err)
	}
	cand, err := s.NextUploadCandidate(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if cand != nil {
		t.Fatalf("mẫu 30 hàng đã đọc hết, không được tràn xuống lịch sử: %+v", cand)
	}
	// Bỏ đánh dấu một hàng trong mẫu → nó là ứng viên (kênh không bị fence).
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET upload_checked_at = NULL WHERE channel_id = $1 AND position = 7`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = NULL WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	cand, err = s.NextUploadCandidate(ctx, time.Now())
	if err != nil || cand == nil {
		t.Fatalf("hàng trong mẫu chưa đọc phải là ứng viên: %+v %v", cand, err)
	}
	if cand.VideoID != "u0000000004" {
		t.Fatalf("ứng viên phải là hàng ở vị trí 7: %+v", cand)
	}
}

func TestScanUploadTimeWriteIsFencedToChannel(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	a := cid("uplocka")
	b := cid("uplockb")
	leaseA, _ := addChannel(t, s, a)
	if err := s.ReleaseChannelLease(ctx, leaseA); err != nil {
		t.Fatal(err)
	}
	addChannelTagged(t, s, b, "b", nil) // hàng b3, b2, b1 của kênh B
	if _, err := pool.Exec(ctx, `UPDATE scan_schedule SET upload_times_next_at = 'epoch' WHERE id`); err != nil {
		t.Fatal(err)
	}
	lease := leaseOf(t, s, a, "probe")
	at := time.Now().Truncate(time.Second)
	if err := s.RecordUploadTime(ctx, lease, padID("v1"), &at, true, time.Now()); err != nil {
		t.Fatalf("ghi giờ đăng: %v", err)
	}
	got := readVideo(t, pool, padID("v1"))
	if got.uploadedAt == nil || !got.uploadedAt.Equal(at) || got.captions == nil || !*got.captions {
		t.Fatalf("giờ đăng/phụ đề chưa ghi: %+v", got)
	}

	// Video của kênh khác: không được ghi dù fence hợp lệ.
	before := readVideo(t, pool, padID("b1"))
	if err := s.RecordUploadTime(ctx, lease, padID("b1"), &at, false, time.Now()); err != nil {
		t.Fatalf("ghi chéo kênh: %v", err)
	}
	after := readVideo(t, pool, padID("b1"))
	if after.uploadedAt != nil || after.checkedAt != nil {
		t.Fatal("ghi giờ đăng của kênh khác đã lọt")
	}
	_ = before

	// Fence cũ: không ghi.
	stale := leaseA
	stale.Token = uuidLike(777)
	if err := s.RecordUploadTime(ctx, stale, padID("v2"), &at, false, time.Now()); scan.StoreCodeOf(err) != scan.StoreLeaseLost {
		t.Fatalf("muốn lease_lost, được %v", err)
	}
	if v2 := readVideo(t, pool, padID("v2")); v2.checkedAt != nil {
		t.Fatal("fence cũ vẫn ghi được")
	}
}

// ── lease worker ───────────────────────────────────────────────────────────

// TestScanWorkerLeaseReturnsStoredOwner: lease phải mang owner do process giữ
// (không phải tên nền tảng), nếu không nhịp gia hạn đầu tiên đã mất lease.
func TestScanWorkerLeaseReturnsStoredOwner(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	owner := "scan-thử-nghiệm"
	lease, err := s.AcquireWorkerLease(ctx, owner, time.Minute)
	if err != nil || lease == nil {
		t.Fatalf("acquire: %v %v", lease, err)
	}
	if lease.Owner != owner {
		t.Fatalf("owner trả về %q, muốn %q", lease.Owner, owner)
	}
	renewed, err := s.RenewWorkerLease(ctx, *lease, time.Minute)
	if err != nil || renewed == nil {
		t.Fatalf("gia hạn bằng owner vừa cấp phải thành công: %v %v", renewed, err)
	}
	other := scan.WorkerLease{Owner: "scan-khác", Until: time.Now().Add(time.Minute)}
	if got, err := s.RenewWorkerLease(ctx, other, time.Minute); err != nil || got != nil {
		t.Fatalf("owner khác không được gia hạn: %v %v", got, err)
	}
	busy, err := s.AcquireWorkerLease(ctx, "scan-chen", time.Minute)
	if err != nil || busy != nil {
		t.Fatalf("lease đang giữ phải trả nil: %+v %v", busy, err)
	}
	if err := s.ReleaseWorkerLease(ctx, *lease); err != nil {
		t.Fatal(err)
	}
	if again, err := s.AcquireWorkerLease(ctx, "scan-sau", time.Minute); err != nil || again == nil || again.Owner != "scan-sau" {
		t.Fatalf("sau khi nhả phải cấp được: %+v %v", again, err)
	}
}

// ── giữ nguyên các ca cũ ───────────────────────────────────────────────────

func TestScanCrashRecoveryContinuesFromCursorWithoutDuplicates(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("crashrec")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	tok := testCursor(id)
	cursor, err := youtube.SerializeChannelCursor(tok)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID:  id,
		Videos:     []youtube.ScannedVideo{video(padID("old2"), 20), video(padID("old1"), 10)},
		NextCursor: &tok,
		Origin:     domain.OriginHistory,
		Lease:      lease,
		JobID:      job.ID,
		Now:        time.Now(),
	}); err != nil {
		t.Fatalf("commit trang 1: %v", err)
	}

	// Worker chết: fence kênh hết hạn → worker mới giữ lại được.
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() - interval '1 second' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	newLease := leaseOf(t, s, id, "worker-2")
	restored, err := s.Job(ctx, job.ID)
	if err != nil || restored == nil {
		t.Fatalf("việc phải còn sống sau crash: %v", err)
	}
	if restored.Cursor != cursor {
		t.Fatalf("cursor đã lưu sai: %q", restored.Cursor)
	}
	again := claim(t, s, restored, newLease)
	if again == nil || again.State != domain.ScanRunning {
		t.Fatal("worker mới phải claim lại được việc bỏ dở")
	}
	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id,
		Videos:    []youtube.ScannedVideo{video(padID("old2"), 20), video(padID("old3"), 5)},
		Origin:    domain.OriginHistory,
		Lease:     newLease,
		JobID:     job.ID,
		Now:       time.Now(),
	})
	if err != nil {
		t.Fatalf("commit trang 2: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("chỉ old3 là mới, added=%d", res.Added)
	}
	st, _ := s.EnsureState(ctx, id)
	if st.IndexedVideos != 6 {
		t.Fatalf("indexed=%d, muốn 6 (3 baseline + 2 + 1)", st.IndexedVideos)
	}
}

func TestScanHistoryCapCapsAndResumeMapsRetry(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("historycap")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	var many []youtube.ScannedVideo
	for i := 0; i < 147; i++ {
		many = append(many, video(fmt.Sprintf("h%09d", i), int64(i+10)))
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: many, Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHistoryCap(ctx, id, 150); err != nil {
		t.Fatal(err)
	}
	if err := s.SetJobCapped(ctx, job.ID, lease); err != nil {
		t.Fatalf("capped: %v", err)
	}
	got, _ := s.Job(ctx, job.ID)
	if got.State != domain.ScanCapped {
		t.Fatalf("state=%s, muốn capped", got.State)
	}
	if err := s.ControlBackfill(ctx, id, domain.ScanActionResume); scan.StoreCodeOf(err) != scan.StoreAlreadyCapped {
		t.Fatalf("resume khi đủ cap phải already_capped, được %v", err)
	}
	if err := s.SetHistoryCap(ctx, id, 200); err != nil {
		t.Fatal(err)
	}
	if err := s.ControlBackfill(ctx, id, domain.ScanActionResume); err != nil {
		t.Fatalf("resume: %v", err)
	}
	got, _ = s.Job(ctx, job.ID)
	if got.State != domain.ScanQueued {
		t.Fatalf("state=%s, muốn queued sau khi nâng cap", got.State)
	}
}

func TestScanBackoffWritesAreFencedAndSanitized(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("backoff")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	raw := fmt.Errorf("lỗi https://secret.example/x?token=%s", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	if err := s.SetJobBackoff(ctx, job.ID, lease, scan.Backoff{
		NextAttemptAt: time.Now().Add(time.Second), Attempts: 1, ErrorCode: "transient", Err: raw,
	}); err != nil {
		t.Fatalf("backoff: %v", err)
	}
	got, _ := s.Job(ctx, job.ID)
	if got.State != domain.ScanBackoff || got.Attempts != 1 {
		t.Fatalf("state=%s attempts=%d", got.State, got.Attempts)
	}
	if got.LastError == "" || got.LastError == raw.Error() {
		t.Fatalf("last_error phải được làm sạch: %q", got.LastError)
	}
	if err := s.ControlBackfill(ctx, id, domain.ScanActionPause); err != nil {
		t.Fatal(err)
	}
	if err := s.SetJobError(ctx, job.ID, lease, scan.JobError{ErrorCode: "blocked"}); scan.StoreCodeOf(err) != scan.StoreJobStopped {
		t.Fatalf("muốn job_stopped, được %v", err)
	}
}

func TestScanDiscoveryOverlapCompletesHeadCheck(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("discovery")
	lease, _ := addChannel(t, s, id)

	created, err := s.StartDiscovery(ctx, id, false)
	if err != nil || !created {
		t.Fatalf("start discovery: created=%v err=%v", created, err)
	}
	job, err := s.NextRunnableJob(ctx, nil)
	if err != nil || job == nil || job.Kind != domain.ScanDiscovery {
		t.Fatalf("runnable phải là discovery: %+v", job)
	}
	claimed := claim(t, s, job, lease)
	res, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID:     id,
		Videos:        []youtube.ScannedVideo{video(padID("v4"), 400), video(padID("v3"), 300)},
		Origin:        domain.OriginDiscovery,
		Lease:         lease,
		JobID:         claimed.ID,
		NewestVideoID: new(padID("v4")),
		Now:           time.Now(),
	})
	if err != nil {
		t.Fatalf("commit discovery: %v", err)
	}
	if !res.Overlap {
		t.Fatal("trang chứa anchor v3 phải báo overlap")
	}
	if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{
		CheckedAt: time.Now(), NewestID: new(padID("v4")), Lease: lease, JobID: claimed.ID, Now: time.Now(),
	}); err != nil {
		t.Fatalf("head check: %v", err)
	}
	got, _ := s.Job(ctx, claimed.ID)
	if got.State != domain.ScanComplete {
		t.Fatalf("discovery phải complete, được %s", got.State)
	}
	st, _ := s.EnsureState(ctx, id)
	if !st.DiscoveryCaughtUp || st.HeadNewestID != padID("v4") {
		t.Fatalf("head mốc sai: caught_up=%v newest=%q", st.DiscoveryCaughtUp, st.HeadNewestID)
	}
	ch, _ := s.Channel(ctx, id)
	if ch.MedianViews == nil {
		t.Fatal("head check phải ghi mức thường")
	}
}

func TestScanDeleteCascadesVideosStateJobsAndTasks(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("cascade")
	addChannel(t, s, id)
	startHistory(t, s, id)
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO background_tasks (id, kind, title, state, object_ref)
		VALUES (gen_random_uuid(), 'channel_scan', 'Quét lịch sử', 'running',
		        jsonb_build_object('Type', 'rival', 'ID', $1::text, 'Label', 'x'))`, id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteChannel(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Channel(ctx, id); scan.StoreCodeOf(err) != scan.StoreChannelMissing {
		t.Fatalf("muốn channel_not_found, được %v", err)
	}
	if _, err := s.EnsureState(ctx, id); scan.StoreCodeOf(err) != scan.StoreChannelMissing {
		t.Fatalf("state phải bị xoá theo, được %v", err)
	}
	for _, sql := range []string{
		`SELECT count(*) FROM rival_videos WHERE channel_id = $1`,
		`SELECT count(*) FROM scan_state WHERE channel_id = $1`,
		`SELECT count(*) FROM scan_jobs WHERE channel_id = $1`,
	} {
		if n := countRows(t, pool, sql, id); n != 0 {
			t.Fatalf("còn %d hàng sót sau khi gỡ (%s)", n, sql)
		}
	}
	if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 0 {
		t.Fatalf("dòng Việc nền còn sót: %d", n)
	}
	if err := s.DeleteChannel(ctx, id); scan.StoreCodeOf(err) != scan.StoreChannelMissing {
		t.Fatalf("gỡ lần hai phải là channel_not_found, được %v", err)
	}
}

// ── fence: hạn phải được xét bằng mốc đọc sau khoá ───────────────────────

// shortLease nhả fence hiện tại rồi lấy fence hạn ngắn (để hết hạn đúng lúc
// một giao dịch khác đang giữ khoá).
func shortLease(t *testing.T, s *postgres.ScanStore, id string, ttl time.Duration) scan.Lease {
	t.Helper()
	ctx := context.Background()
	if st, err := s.EnsureState(ctx, id); err == nil && st.LeaseToken != "" {
		if err := s.ReleaseChannelLease(ctx, scan.Lease{ChannelID: id, Token: st.LeaseToken}); err != nil {
			t.Fatalf("nhả fence cũ: %v", err)
		}
	}
	lease, err := s.AcquireChannelLease(ctx, id, "fence-ngắn", ttl)
	if err != nil || lease == nil {
		t.Fatalf("lấy fence hạn ngắn: %+v %v", lease, err)
	}
	return *lease
}

// holdLock giữ khoá hàng của câu sql trên một kết nối riêng, trả hàm nhả.
func holdLock(t *testing.T, pool *pgxpool.Pool, sql, arg string) func() {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("mở giao dịch giữ khoá: %v", err)
	}
	if _, err := tx.Exec(ctx, sql, arg); err != nil {
		t.Fatalf("giữ khoá: %v", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if err := tx.Commit(ctx); err != nil {
				t.Fatalf("nhả khoá: %v", err)
			}
		})
	}
}

// TestScanLeaseExpiryDuringLockWait: fence hết hạn trong lúc thao tác đang chờ
// khoá parent (hoặc khoá trạng thái) phải bị từ chối — hạn được xét bằng mốc
// đọc sau khi đã giữ khoá, không phải mốc lúc gọi.
func TestScanLeaseExpiryDuringLockWait(t *testing.T) {
	type path struct {
		name    string
		prepare func(t *testing.T, s *postgres.ScanStore, id string, ttl time.Duration) (*scan.Job, scan.Lease)
		call    func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, lease scan.Lease) error
		check   func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job)
	}
	claimed := func(t *testing.T, s *postgres.ScanStore, id string, ttl time.Duration) (*scan.Job, scan.Lease) {
		job := startHistory(t, s, id)
		lease := shortLease(t, s, id, ttl)
		return claim(t, s, job, lease), lease
	}
	paths := []path{
		{"transition", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.SetJobError(ctx, job.ID, l, scan.JobError{ErrorCode: "blocked"})
		}, func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job) {
			if got, _ := s.Job(context.Background(), job.ID); got.State != domain.ScanRunning {
				t.Fatalf("việc bị đổi bởi fence đã hết hạn: %s", got.State)
			}
		}},
		{"page", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			_, err := s.CommitPage(ctx, scan.CommitPageInput{
				ChannelID: job.ChannelID, Videos: []youtube.ScannedVideo{video(padID("exp_page"), 10)},
				Origin: domain.OriginHistory, Lease: l, JobID: job.ID, Now: time.Now(),
			})
			return err
		}, func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job) {
			if n := countRows(t, pool, `SELECT count(*) FROM rival_videos WHERE id = $1`, padID("exp_page")); n != 0 {
				t.Fatal("trang của fence đã hết hạn vẫn ghi")
			}
		}},
		{"head", claimed, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			_, err := s.CompleteHeadCheck(ctx, job.ChannelID, scan.HeadCheckInput{CheckedAt: time.Now(), Lease: l, Now: time.Now()})
			return err
		}, func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job) {
			if ch, _ := s.Channel(context.Background(), job.ChannelID); ch.MedianViews != nil {
				t.Fatalf("head check của fence hết hạn vẫn ghi mức thường: %v", *ch.MedianViews)
			}
		}},
		{"upload", func(t *testing.T, s *postgres.ScanStore, id string, ttl time.Duration) (*scan.Job, scan.Lease) {
			return nil, shortLease(t, s, id, ttl)
		}, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			return s.RecordUploadTime(ctx, l, padID("v1"), new(time.Now()), true, time.Now())
		}, func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job) {
			if got := readVideo(t, pool, padID("v1")); got.uploadedAt != nil || got.checkedAt != nil {
				t.Fatal("kết quả thăm dò của fence hết hạn vẫn ghi")
			}
		}},
		{"claim", func(t *testing.T, s *postgres.ScanStore, id string, ttl time.Duration) (*scan.Job, scan.Lease) {
			lease := shortLease(t, s, id, ttl)
			return startHistory(t, s, id), lease
		}, func(ctx context.Context, s *postgres.ScanStore, job *scan.Job, l scan.Lease) error {
			_, err := s.ClaimJob(ctx, job.ID, l)
			return err
		}, func(t *testing.T, s *postgres.ScanStore, pool *pgxpool.Pool, job *scan.Job) {
			if got, _ := s.Job(context.Background(), job.ID); got.State != domain.ScanQueued {
				t.Fatalf("việc bị claim bởi fence đã hết hạn: %s", got.State)
			}
		}},
	}
	waits := []struct {
		name string
		sql  string
	}{
		{"parent", `SELECT 1 FROM rival_channels WHERE id = $1 FOR UPDATE`},
		{"state", `UPDATE scan_state SET updated_at = updated_at WHERE channel_id = $1`},
	}
	for _, p := range paths {
		for _, w := range waits {
			t.Run(p.name+"_"+w.name, func(t *testing.T) {
				pool := provision(t)
				ctx := context.Background()
				id := cid("exp_" + p.name)
				base := postgres.NewScanStore(pool)
				addChannel(t, base, id)
				job, lease := p.prepare(t, base, id, 250*time.Millisecond)

				release := holdLock(t, pool, w.sql, id)
				defer release()

				done := make(chan error, 1)
				go func() {
					done <- p.call(ctx, base, job, lease)
				}()
				// Thao tác đang chờ khoá; fence hết hạn trong lúc đó.
				time.Sleep(600 * time.Millisecond)
				select {
				case err := <-done:
					t.Fatalf("thao tác không chờ khoá mà đã xong: %v", err)
				default:
				}
				// Giữ khoá lâu hơn hạn fence một chút rồi mới nhả.
				time.Sleep(100 * time.Millisecond)
				release()

				select {
				case err := <-done:
					if scan.StoreCodeOf(err) != scan.StoreLeaseLost {
						t.Fatalf("fence hết hạn trong lúc chờ khoá phải là lease_lost, được %v", err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("thao tác không trả về sau khi khoá được nhả")
				}
				p.check(t, base, pool, job)
			})
		}
	}
}

// TestScanStoppedDiscoveryRetryStartsFreshRun: lượt kiểm tra lại sau
// một run đã dừng phải là run mới — khối khoá mới, anchors hiện tại, ordinal
// 0, không cursor — chứ không tiếp tục run đã chạm trần 10.000.
func TestScanStoppedDiscoveryRetryStartsFreshRun(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("gapfresh")
	lease, _ := addChannel(t, s, id)
	if _, err := s.StartDiscovery(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	stale, _ := s.NextRunnableJob(ctx, nil)
	claim(t, s, stale, lease)
	if _, err := pool.Exec(ctx, `
		UPDATE scan_jobs SET state = 'error', error_code = 'gap_incomplete',
		       next_ordinal = 10000, newest_id = 'zzzzzzzzzzz'
		WHERE id = $1`, stale.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() - interval '1 minute' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}

	created, err := s.StartDiscovery(ctx, id, true)
	if err != nil || !created {
		t.Fatalf("kiểm tra lại phải tạo run mới: created=%v err=%v", created, err)
	}
	old, err := s.Job(ctx, stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if old.State != domain.ScanCancelled {
		t.Fatalf("lượt cũ phải bị bỏ hẳn: %s", old.State)
	}
	fresh, err := s.NextRunnableJob(ctx, nil)
	if err != nil || fresh == nil || fresh.Kind != domain.ScanDiscovery {
		t.Fatalf("phải có run discovery mới runnable: %+v %v", fresh, err)
	}
	if fresh.ID == stale.ID {
		t.Fatal("vẫn là run cũ")
	}
	if fresh.NextOrdinal != 0 {
		t.Fatalf("run mới phải bắt đầu ở ordinal 0, được %d", fresh.NextOrdinal)
	}
	if fresh.Cursor != "" || fresh.Remainder != "" {
		t.Fatalf("run mới không được mang continuation: %q %q", fresh.Cursor, fresh.Remainder)
	}
	if fresh.OrderBase == nil || *fresh.OrderBase != -int64(scan.DiscoveryBlock) {
		t.Fatalf("run mới phải khoá khối mới: %+v", fresh.OrderBase)
	}
	if len(fresh.Anchors) == 0 {
		t.Fatal("run mới phải có anchors hiện tại")
	}
	if fresh.NewestID != "" || fresh.Pages != 0 || fresh.Scanned != 0 || fresh.Added != 0 {
		t.Fatalf("run mới không được mang tiến độ cũ: %+v", fresh)
	}
	// Trang đầu của run mới ghi được (khối mới chưa dùng khoá nào).
	freshLease := leaseOf(t, s, id, "worker-mới")
	claim(t, s, fresh, freshLease)
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: id, Videos: []youtube.ScannedVideo{video(padID("after_gap"), 5)},
		Origin: domain.OriginDiscovery, Lease: freshLease, JobID: fresh.ID,
		NewestVideoID: new(padID("after_gap")), Now: time.Now(),
	}); err != nil {
		t.Fatalf("trang đầu của run mới: %v", err)
	}
}

// ── dòng Việc nền: hình JSON thật, tạo/nối nguyên tử, giữ tiến độ ─────────

// scanTaskSpec là hình dòng Việc nền của một việc quét.
func scanTaskSpec(channelID string) domain.TaskSpec {
	return domain.TaskSpec{
		Kind:  domain.TaskChannelScan,
		Title: "Quét lịch sử",
		Object: domain.ObjectRef{
			Type:  "rival",
			ID:    channelID,
			Label: "Kênh đối thủ",
		},
	}
}

// TestScanRemoveClearsRealActivityRows: dòng Việc nền do activity thật ghi
// (object_ref theo domain.ObjectRef) phải bị gỡ cùng kênh.
func TestScanRemoveClearsRealActivityRows(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("realtask")
	addChannel(t, s, id)
	tasks := activity.New(postgres.NewActivityStore(pool), func(string) {})
	if _, err := tasks.Start(ctx, scanTaskSpec(id)); err != nil {
		t.Fatalf("tạo dòng Việc nền thật: %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 1 {
		t.Fatalf("dòng Việc nền chưa được ghi theo hình thật: %d", n)
	}
	if err := s.DeleteChannel(ctx, id); err != nil {
		t.Fatalf("gỡ kênh: %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 0 {
		t.Fatalf("dòng Việc nền còn sót sau khi gỡ kênh: %d", n)
	}
}

// TestScanTaskCreateLinkIsAtomic: gỡ kênh không thể chen giữa tạo và nối dòng
// — hoặc dòng chưa từng được tạo, hoặc gỡ cuốn luôn nó theo.
func TestScanTaskCreateLinkIsAtomic(t *testing.T) {
	t.Run("remove_trước", func(t *testing.T) {
		pool := provision(t)
		ctx := context.Background()
		id := cid("atomic_del")
		base := postgres.NewScanStore(pool)
		addChannel(t, base, id)
		job := startHistory(t, base, id)

		g := newGate("DELETE FROM rival_channels")
		slow := gatedStore(pool, g)
		removeErr := make(chan error, 1)
		go func() { removeErr <- slow.DeleteChannel(ctx, id) }()
		<-g.reached

		syncErr := make(chan error, 1)
		var taskID string
		go func() {
			var err error
			res, err := base.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
			taskID = res.TaskID
			syncErr <- err
		}()
		ensureBlocked(t, syncErr, 150*time.Millisecond)
		close(g.resume)
		if err := <-removeErr; err != nil {
			t.Fatalf("gỡ kênh: %v", err)
		}
		if err := <-syncErr; scan.StoreCodeOf(err) != scan.StoreChannelMissing {
			t.Fatalf("nối việc vào kênh đã gỡ phải là channel_not_found, được %v", err)
		}
		if taskID != "" {
			t.Fatalf("không được trả id của dòng mồ côi: %q", taskID)
		}
		if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 0 {
			t.Fatalf("bỏ lại %d dòng Việc nền mồ côi", n)
		}
	})

	t.Run("tạo_trước", func(t *testing.T) {
		s, pool := scanStore(t)
		ctx := context.Background()
		id := cid("atomic_add")
		addChannel(t, s, id)
		job := startHistory(t, s, id)
		synced, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
		taskID := synced.TaskID
		if err != nil || taskID == "" {
			t.Fatalf("tạo+nối: %q %v", taskID, err)
		}
		linked, _ := s.Job(ctx, job.ID)
		if linked.TaskID != taskID {
			t.Fatalf("việc chưa được nối: %q ≠ %q", linked.TaskID, taskID)
		}
		if err := s.DeleteChannel(ctx, id); err != nil {
			t.Fatal(err)
		}
		if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE id = $1`, taskID); n != 0 {
			t.Fatalf("dòng Việc nền sống sót sau khi gỡ kênh: %d", n)
		}
		// Gọi lại trên kênh đã gỡ là lỗi thật, và không được tạo dòng mới.
		if _, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id)); scan.StoreCodeOf(err) != scan.StoreChannelMissing {
			t.Fatalf("muốn channel_not_found, được %v", err)
		}
		if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 0 {
			t.Fatalf("lần gọi sau khi gỡ tạo lại %d dòng", n)
		}
	})
}

// TestScanTaskAdoptionKeepsProgress: nhận lại dòng Việc nền không được xoá
// tiến độ đang hiện (khác retry sau failed), và tiến độ của các trang sau vẫn
// đến từ đường cập nhật thường.
func TestScanTaskAdoptionKeepsProgress(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("adoptprogress")
	addChannel(t, s, id)
	job := startHistory(t, s, id)
	tasks := activity.New(postgres.NewActivityStore(pool), func(string) {})

	created, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
	first := created.TaskID
	if err != nil || first == "" {
		t.Fatalf("tạo dòng: %q %v", first, err)
	}
	if err := tasks.Update(ctx, first, domain.TaskUpdate{
		Progress: &domain.TaskProgress{Done: 3, Total: 150},
	}); err != nil {
		t.Fatalf("ghi tiến độ thật: %v", err)
	}

	// Nhận chủ lại (khôi phục sau crash): cùng dòng, tiến độ còn nguyên.
	adoptedSync, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
	adopted := adoptedSync.TaskID
	if err != nil {
		t.Fatalf("nhận lại: %v", err)
	}
	if adopted != first {
		t.Fatalf("nhận lại phải dùng đúng dòng cũ: %q ≠ %q", adopted, first)
	}
	locked, _ := s.Job(ctx, job.ID)
	view, _ := scan.TaskPresentation(*locked)
	progress, state := storedTask(t, pool, first)
	if progress == nil || progress.Done != 3 || progress.Total != 150 {
		t.Fatalf("nhận lại xoá mất tiến độ: %+v", progress)
	}
	if state != view.State {
		t.Fatalf("trạng thái hiển thị phải theo việc đang khoá: %s (muốn %s)", state, view.State)
	}

	// Các trang sau: tiến độ đi qua đường cập nhật thường, rồi nhận lại lần nữa
	// vẫn giữ mốc mới nhất.
	for done := int64(4); done <= 6; done++ {
		if err := tasks.Update(ctx, first, domain.TaskUpdate{
			Progress: &domain.TaskProgress{Done: done, Total: 150},
		}); err != nil {
			t.Fatalf("tiến độ trang %d: %v", done, err)
		}
	}
	if _, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id)); err != nil {
		t.Fatalf("nhận lại sau nhiều trang: %v", err)
	}
	progress, _ = storedTask(t, pool, first)
	if progress == nil || progress.Done != 6 {
		t.Fatalf("tiến độ của trang cuối bị xoá: %+v", progress)
	}
	// Spec mang tiến độ mới thì bên gọi là chủ con số đó.
	spec := scanTaskSpec(id)
	spec.Progress = &domain.TaskProgress{Done: 7, Total: 150}
	if _, err := s.SyncScanTask(ctx, job.ID, id, spec); err != nil {
		t.Fatalf("nhận lại kèm tiến độ: %v", err)
	}
	progress, _ = storedTask(t, pool, first)
	if progress == nil || progress.Done != 7 {
		t.Fatalf("tiến độ của spec phải được ghi: %+v", progress)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, id); n != 1 {
		t.Fatalf("nhận lại không được tạo dòng thứ hai: %d", n)
	}
}

// storedTask đọc tiến độ + trạng thái của một dòng Việc nền thật.
func storedTask(t *testing.T, park *pgxpool.Pool, id string) (*domain.TaskProgress, domain.TaskState) {
	t.Helper()
	list, err := postgres.NewActivityStore(park).ListTasks(context.Background())
	if err != nil {
		t.Fatalf("đọc việc nền: %v", err)
	}
	for _, task := range list {
		if task.ID == id {
			return task.Progress, task.State
		}
	}
	t.Fatalf("không thấy dòng việc %s", id)
	return nil, ""
}

// storedPresentation đọc câu mô tả và nút bấm đã lưu của một dòng việc.
func storedPresentation(t *testing.T, park *pgxpool.Pool, id string) (string, domain.TaskAction) {
	t.Helper()
	list, err := postgres.NewActivityStore(park).ListTasks(context.Background())
	if err != nil {
		t.Fatalf("đọc việc nền: %v", err)
	}
	for _, task := range list {
		if task.ID == id {
			return task.Phase, task.Action
		}
	}
	t.Fatalf("không thấy dòng việc %s", id)
	return "", domain.TaskAction{}
}

// TestScanCommitLatency đo p95 thời gian commit một trang (20 video) trên
// Postgres thật — số liệu cho báo cáo, không đặt ngưỡng cứng.
func TestScanCommitLatency(t *testing.T) {
	s, _ := scanStore(t)
	ctx := context.Background()
	id := cid("latency")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	const n = 40
	samples := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		var page []youtube.ScannedVideo
		for j := 0; j < 20; j++ {
			page = append(page, video(fmt.Sprintf("l%06d%04d", i, j), int64(i*20+j)))
		}
		start := time.Now()
		if _, err := s.CommitPage(ctx, scan.CommitPageInput{
			ChannelID: id, Videos: page, Origin: domain.OriginHistory, Lease: lease, JobID: job.ID, Now: time.Now(),
		}); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[len(samples)*95/100]
	t.Logf("commit 20 video: p50=%v p95=%v max=%v", samples[len(samples)/2], p95, samples[len(samples)-1])
}

// ── retention của bảng việc ─────────────────────────────────────────────────

// Mỗi giờ một lượt kiểm tra video mới để lại một dòng complete, thay một lượt
// hỏng để lại một dòng cancelled: nếu không dọn thì bảng việc lớn mãi. Một lần
// chèn lượt mới phải dọn các lượt đã dừng của chính kênh/loại đó, chừa đúng
// dòng mới nhất mà danh sách đối thủ đọc, và không đụng kênh khác.
func TestScanFinishedJobsArePruned(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("retention")
	other := cid("retention_other")
	base, _ := addChannel(t, s, id)
	if err := s.ReleaseChannelLease(ctx, base); err != nil {
		t.Fatal(err)
	}
	otherBase, _ := addChannelTagged(t, s, other, "w", nil)
	if err := s.ReleaseChannelLease(ctx, otherBase); err != nil {
		t.Fatal(err)
	}

	// Kênh khác: vài lượt đã dừng sẵn, phải còn nguyên sau mọi lần dọn.
	if _, err := pool.Exec(ctx, `INSERT INTO scan_jobs (id, channel_id, kind, state, finished_at)
		SELECT gen_random_uuid(), $1, 'discovery', 'complete', now() FROM generate_series(1, 4)`, other); err != nil {
		t.Fatal(err)
	}

	// N lượt discovery hỏng rồi bị thay.
	for i := 0; i < 5; i++ {
		if _, err := s.StartDiscovery(ctx, id, false); err != nil {
			t.Fatalf("mở lượt %d: %v", i, err)
		}
		w := leaseOf(t, s, id, "worker")
		job, err := s.NextRunnableJobForChannel(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		claim(t, s, job, w)
		if err := s.SetJobPaused(ctx, job.ID, w, scan.JobError{ErrorCode: "cursor_invalid"}); err != nil {
			t.Fatal(err)
		}
		if err := s.ReleaseChannelLease(ctx, w); err != nil {
			t.Fatal(err)
		}
		if _, err := s.StartDiscovery(ctx, id, true); err != nil {
			t.Fatalf("thay lượt %d: %v", i, err)
		}
	}

	// N lượt kiểm tra video mới theo giờ, mỗi lượt kết thúc một dòng discovery.
	var lastFinished string
	for i := 0; i < 5; i++ {
		if _, err := s.StartDiscovery(ctx, id, false); err != nil {
			t.Fatalf("mở lượt giờ %d: %v", i, err)
		}
		w := leaseOf(t, s, id, "worker")
		job, err := s.NextRunnableJobForChannel(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		claim(t, s, job, w)
		if _, err := s.CompleteHeadCheck(ctx, id, scan.HeadCheckInput{
			CheckedAt: time.Now().Add(time.Duration(i) * time.Minute),
			Lease:     w, JobID: job.ID, Now: time.Now(),
		}); err != nil {
			t.Fatalf("head check %d: %v", i, err)
		}
		lastFinished = job.ID
		if err := s.ReleaseChannelLease(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	// N lượt sử ký lịch sử chạy xong.
	for i := 0; i < 5; i++ {
		job := startHistory(t, s, id)
		if job == nil {
			t.Fatalf("lượt lịch sử %d không mở được", i)
		}
		w := leaseOf(t, s, id, "worker")
		claim(t, s, job, w)
		if err := s.CompleteJob(ctx, job.ID, w); err != nil {
			t.Fatalf("xong lịch sử %d: %v", i, err)
		}
		if err := s.ReleaseChannelLease(ctx, w); err != nil {
			t.Fatal(err)
		}
		lastFinished = job.ID
	}

	count := func(channel, kind, states string) int {
		t.Helper()
		return countRows(t, pool, `SELECT count(*) FROM scan_jobs
			WHERE channel_id = $1 AND kind = $2 AND state IN (`+states+`)`, channel, kind)
	}
	if n := count(id, "discovery", `'complete','cancelled'`); n > 1 {
		t.Fatalf("discovery để lại %d dòng đã dừng, muốn tối đa 1", n)
	}
	if n := count(id, "history", `'complete','cancelled'`); n > 1 {
		t.Fatalf("history để lại %d dòng đã dừng, muốn tối đa 1", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM scan_jobs WHERE channel_id = $1`, id); n > 2 {
		t.Fatalf("kênh giữ %d dòng việc, muốn tối đa 2 (một còn sống, một đã dừng)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM scan_jobs WHERE channel_id = $1 AND id = $2`, id, lastFinished); n != 1 {
		t.Fatalf("dòng đã dừng mới nhất phải được giữ lại")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM scan_jobs WHERE channel_id = $1`, other); n != 4 {
		t.Fatalf("kênh khác bị dọn mất dòng: còn %d, muốn 4", n)
	}
}

// ── lịch rảnh: việc đọc giờ đăng và fence của người khác ────────────────────

// Video chưa đọc trong mẫu 30 là việc sẵn sàng ngay cả khi chưa từng có dòng
// tạm nghỉ nào (kênh vừa thêm, máy chủ đang rảnh từ trước).
func TestScanWakeupSeesUnpausedUploadWork(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("wakeupload")
	lease, _ := addChannel(t, s, id)
	if err := s.ReleaseChannelLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM scan_schedule`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE scan_state SET head_checked_at = now(), lease_until = NULL
		WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	cand, err := s.NextUploadCandidate(ctx, time.Now())
	if err != nil || cand == nil {
		t.Fatalf("video chưa đọc phải là ứng viên: %+v %v", cand, err)
	}
	w, err := s.NextWakeup(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !w.Actionable {
		t.Fatalf("ứng viên chưa đọc không bị chặn phải là việc sẵn sàng: %+v", w)
	}
}

// Nghỉ đọc giờ đăng đã hết nhưng kênh đang bị fence của process khác: không
// phải việc sẵn sàng, chỉ là hạn ngủ tới lúc fence hết. Kênh khác rảnh vẫn
// phải làm được ngay.
func TestScanWakeupKeepsFencedWorkOutOfActionable(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	fenced := cid("fencedup")
	lease, _ := addChannel(t, s, fenced)
	if err := s.ReleaseChannelLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO scan_schedule (id, upload_times_next_at)
		VALUES (true, now() - interval '1 minute')
		ON CONFLICT (id) DO UPDATE SET upload_times_next_at = excluded.upload_times_next_at`); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(2 * time.Minute).Truncate(time.Millisecond)
	if _, err := pool.Exec(ctx, `
		UPDATE scan_state SET head_checked_at = now(), lease_until = $2
		WHERE channel_id = $1`, fenced, until); err != nil {
		t.Fatal(err)
	}
	if cand, err := s.NextUploadCandidate(ctx, time.Now()); err != nil || cand != nil {
		t.Fatalf("kênh bị fence không được chọn: %+v %v", cand, err)
	}
	w, err := s.NextWakeup(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w.Actionable {
		t.Fatalf("nghỉ đã hết một mình không làm việc bị fence thành sẵn sàng: %+v", w)
	}
	if w.Next == nil || w.Next.Sub(until).Abs() > time.Second {
		t.Fatalf("hạn ngủ phải là lúc fence hết: %+v (muốn %v)", w.Next, until)
	}

	// Kênh khác không bị fence, cùng có video chưa đọc → việc sẵn sàng.
	// Id video là khoá toàn cục: kênh thứ hai phải mang tiền tố riêng.
	free := cid("freeup")
	freeLease, _ := addChannelTagged(t, s, free, "w", nil)
	if err := s.ReleaseChannelLease(ctx, freeLease); err != nil {
		t.Fatal(err)
	}
	w, err = s.NextWakeup(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !w.Actionable {
		t.Fatalf("kênh rảnh phải cho việc sẵn sàng dù kênh kia bị fence: %+v", w)
	}
}

// ── dòng Việc nền đi theo trạng thái việc đang khoá ──────────────────────────

func TestScanTaskSyncFollowsLockedJobState(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()

	// Việc vừa bị tạm dừng, nơi gọi còn ảnh chụp "đang chạy".
	paused := cid("tasksyncpause")
	lease, _ := addChannel(t, s, paused)
	job := startHistory(t, s, paused)
	claim(t, s, job, lease)
	if err := s.ControlBackfill(ctx, paused, domain.ScanActionPause); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Job(ctx, job.ID); got.State != domain.ScanPaused {
		t.Fatalf("việc phải tạm dừng trước: %s", got.State)
	}
	synced, err := s.SyncScanTask(ctx, job.ID, paused, scanTaskSpec(paused))
	taskID := synced.TaskID
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, state := storedTask(t, pool, taskID); state != domain.TaskPaused {
		t.Fatalf("ảnh chụp cũ không được dựng lại dòng đang chạy: %s", state)
	}

	// Việc đã chạm trần: không tạo dòng nào, cũng không có dòng đang chạy.
	capped := cid("tasksynccap")
	cappedLease, _ := addChannel(t, s, capped) // baseline 3 video
	cappedJob := startHistory(t, s, capped)
	claim(t, s, cappedJob, cappedLease)
	var many []youtube.ScannedVideo
	for i := 0; i < 147; i++ {
		many = append(many, video(fmt.Sprintf("c%09d", i), int64(i+10)))
	}
	if _, err := s.CommitPage(ctx, scan.CommitPageInput{
		ChannelID: capped, Videos: many, Origin: domain.OriginHistory,
		Lease: cappedLease, JobID: cappedJob.ID, Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHistoryCap(ctx, capped, 150); err != nil {
		t.Fatal(err)
	}
	if err := s.SetJobCapped(ctx, cappedJob.ID, cappedLease); err != nil {
		t.Fatalf("chạm trần: %v", err)
	}
	if got, _ := s.Job(ctx, cappedJob.ID); got.State != domain.ScanCapped {
		t.Fatalf("việc phải chạm trần trước: %s", got.State)
	}
	if _, err := s.SyncScanTask(ctx, cappedJob.ID, capped, scanTaskSpec(capped)); scan.StoreCodeOf(err) != scan.StoreJobStopped {
		t.Fatalf("việc chạm trần không được tạo dòng Việc nền, được %v", err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM background_tasks WHERE object_ref->>'ID' = $1`, capped); n != 0 {
		t.Fatalf("việc chạm trần để lại %d dòng", n)
	}
}

// Nhận lại dòng việc không mang tiến độ thì kho không ghi cột tiến độ: mười
// lần nhận lại ở cùng một thời điểm không sinh lần ghi tiến độ nào.
func TestScanTaskAdoptionWritesNoProgressByItself(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("progressbudget")
	addChannel(t, s, id)
	job := startHistory(t, s, id)

	// Vai app không có quyền DDL: dựng bộ đếm bằng kết nối bảo trì, hàm
	// SECURITY DEFINER để nó tự ghi bộ đếm dưới quyền chủ schema.
	schema := pgx.Identifier{pgtest.SchemaOf(pool.Config().ConnString())}.Sanitize()
	admin, err := pgx.Connect(ctx, pgtest.RequireURL(t))
	if err != nil {
		t.Fatalf("kết nối bảo trì: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	if _, err := admin.Exec(ctx, fmt.Sprintf(`
		SET search_path TO %s;
		CREATE TABLE progress_writes (n integer NOT NULL DEFAULT 0);
		INSERT INTO progress_writes DEFAULT VALUES;
		CREATE FUNCTION count_progress_write() RETURNS trigger
		LANGUAGE plpgsql SECURITY DEFINER AS $$
		BEGIN
			IF NEW.progress IS DISTINCT FROM OLD.progress THEN
				UPDATE progress_writes SET n = n + 1;
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER progress_write_count BEFORE UPDATE ON background_tasks
		FOR EACH ROW EXECUTE FUNCTION count_progress_write();`, schema)); err != nil {
		t.Fatalf("dựng bộ đếm: %v", err)
	}
	writes := func() int {
		t.Helper()
		var n int
		if err := admin.QueryRow(ctx, `SELECT n FROM progress_writes`).Scan(&n); err != nil {
			t.Fatalf("đọc bộ đếm: %v", err)
		}
		return n
	}

	spec := scanTaskSpec(id)
	spec.Progress = &domain.TaskProgress{Done: 3, Total: 150}
	synced, err := s.SyncScanTask(ctx, job.ID, id, spec)
	taskID := synced.TaskID
	if err != nil {
		t.Fatal(err)
	}
	if writes() != 0 {
		t.Fatalf("dòng mới được khởi tạo một lần, không phải một lần ghi cập nhật: %d", writes())
	}
	for i := 0; i < 10; i++ {
		if _, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id)); err != nil {
			t.Fatalf("nhận lại %d: %v", i, err)
		}
	}
	if n := writes(); n != 0 {
		t.Fatalf("nhận lại không mang tiến độ mà vẫn ghi %d lần", n)
	}
	progress, _ := storedTask(t, pool, taskID)
	if progress == nil || progress.Done != 3 {
		t.Fatalf("nhận lại làm mất tiến độ đang hiện: %+v", progress)
	}

	spec = scanTaskSpec(id)
	spec.Progress = &domain.TaskProgress{Done: 7, Total: 150}
	if _, err := s.SyncScanTask(ctx, job.ID, id, spec); err != nil {
		t.Fatal(err)
	}
	if n := writes(); n != 1 {
		t.Fatalf("spec mang tiến độ phải ghi đúng một lần: %d", n)
	}
	if progress, _ := storedTask(t, pool, taskID); progress == nil || progress.Done != 7 {
		t.Fatalf("tiến độ của spec chưa được ghi: %+v", progress)
	}
}

// Dòng running còn lại của chủ cũ chỉ claim được khi fence kênh trống hoặc hết
// hạn: fence còn hạn của người khác thì chỉ cho hạn thức, không phải việc sẵn
// sàng — nếu không, worker mới quay vòng chọn lại đúng dòng đó tới lúc fence hết.
func TestScanWakeupGatesRecoveredRunningJobOnChannelFence(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("recoverrun")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	// Không còn việc nào khác: mẫu thăm dò giờ đăng đọc hết, rescan còn xa.
	if _, err := pool.Exec(ctx, `UPDATE rival_videos SET upload_checked_at = now() WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET head_checked_at = now() WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Job(ctx, job.ID); got.State != domain.ScanRunning {
		t.Fatalf("việc phải đang chạy: %s", got.State)
	}

	// Chủ cũ còn hạn: chưa claim được, hạn thức đúng lúc fence hết.
	until := time.Now().Add(2 * time.Minute).Truncate(time.Millisecond)
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = $2 WHERE channel_id = $1`, id, until); err != nil {
		t.Fatal(err)
	}
	w, err := s.NextWakeup(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w.Actionable {
		t.Fatalf("dòng running sau fence còn hạn không phải việc sẵn sàng: %+v", w)
	}
	if w.Next == nil || w.Next.Sub(until).Abs() > time.Second {
		t.Fatalf("hạn thức phải là lúc fence hết: %+v (muốn %v)", w.Next, until)
	}

	// Fence hết hạn (chủ cũ chết): claim được ngay.
	if _, err := pool.Exec(ctx, `UPDATE scan_state SET lease_until = now() - interval '1 second' WHERE channel_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	w, err = s.NextWakeup(ctx, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !w.Actionable {
		t.Fatalf("fence hết hạn thì dòng running phải claim được: %+v", w)
	}
}

// Nội dung dòng Việc nền (trạng thái, câu mô tả, nút bấm) lấy từ việc đang bị
// khoá, không lấy từ ảnh chụp của nơi gọi: việc vừa bị tạm dừng mà một trang
// cũ ghi tiến độ thì dòng phải nói "đã tạm dừng" kèm nút tiếp tục, không được
// quay lại "đang chạy" kèm nút huỷ.
func TestScanTaskSyncUsesLockedJobPresentation(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("presentation")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)
	running, _ := s.Job(ctx, job.ID)
	stale := scanTaskSpec(id) // ảnh chụp trước khi tạm dừng
	if running.State == domain.ScanRunning {
		view, live := scan.TaskPresentation(*running)
		if !live {
			t.Fatal("việc đang chạy phải còn hiển thị")
		}
		stale.Phase, stale.Action = view.Phase, view.Action
	}

	if err := s.ControlBackfill(ctx, id, domain.ScanActionPause); err != nil {
		t.Fatal(err)
	}
	locked, _ := s.Job(ctx, job.ID)
	if locked.State != domain.ScanPaused {
		t.Fatalf("việc phải tạm dừng trước: %s", locked.State)
	}
	view, live := scan.TaskPresentation(*locked)
	if !live {
		t.Fatal("việc tạm dừng vẫn còn hiển thị")
	}

	// Tiến độ của trang cũ đi kèm ảnh chụp cũ.
	stale.Progress = &domain.TaskProgress{Done: 5, Total: 150}
	synced, err := s.SyncScanTask(ctx, job.ID, id, stale)
	taskID := synced.TaskID
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	progress, state := storedTask(t, pool, taskID)
	if state != view.State || state != domain.TaskPaused {
		t.Fatalf("trạng thái phải theo việc đang khoá: %s (muốn %s)", state, view.State)
	}
	phase, action := storedPresentation(t, pool, taskID)
	if phase != view.Phase {
		t.Fatalf("câu mô tả phải theo việc đang khoá: %q (muốn %q)", phase, view.Phase)
	}
	if action.Op != view.Action.Op || action.Href != view.Action.Href {
		t.Fatalf("nút bấm phải theo việc đang khoá: %+v (muốn %+v)", action, view.Action)
	}
	if action.Op != domain.TaskOpResume {
		t.Fatalf("việc tạm dừng phải có nút tiếp tục: %+v", action)
	}
	// Tiến độ vẫn được ghi theo đúng ngân sách (spec mang tiến độ thì ghi).
	if progress == nil || progress.Done != 5 {
		t.Fatalf("tiến độ của spec phải được ghi: %+v", progress)
	}
}

// Tập kênh bận phải lọc trước LIMIT: 50 việc của kênh bận không được che mất
// việc của kênh rảnh thứ 51 (nếu không worker quay lại đúng tiền tố đó mãi).
func TestScanRunnableSkipAppliesBeforeLimit(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()

	const n = 51
	base := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	skip := map[string]bool{}
	var last string
	for i := 0; i < n; i++ {
		id := cid(fmt.Sprintf("fair%02d", i))
		next := testCursor(id)
		lease, _ := addChannelTagged(t, s, id, fmt.Sprintf("f%02d", i), &next)
		job := startHistory(t, s, id)
		if job == nil {
			t.Fatalf("kênh %d không có việc", i)
		}
		if err := s.ReleaseChannelLease(ctx, lease); err != nil {
			t.Fatal(err)
		}
		// Thứ tự chọn là updated_at tăng dần: đặt cố định để phép thử tất định.
		if _, err := pool.Exec(ctx, `UPDATE scan_jobs SET updated_at = $2 WHERE id = $1`,
			job.ID, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
		if i < n-1 {
			skip[id] = true
		}
		last = id
	}

	first, err := s.NextRunnableJob(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.ChannelID != cid("fair00") {
		t.Fatalf("không bỏ kênh nào thì chọn việc cũ nhất: %+v", first)
	}

	got, err := s.NextRunnableJob(ctx, skip)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ChannelID != last {
		t.Fatalf("phải chọn được kênh rảnh thứ %d sau 50 kênh bận: %+v", n, got)
	}

	// Bận hết thì không còn việc nào.
	skip[last] = true
	if got, err := s.NextRunnableJob(ctx, skip); err != nil || got != nil {
		t.Fatalf("bận hết phải trả rỗng: %+v %v", got, err)
	}
}

// Một lần hỏng thật chỉ báo một lần: các lần làm mới dòng việc đã failed không
// báo lại, ảnh chụp lỗi cũ sau khi việc đã chạy lại cũng không, và lần hỏng
// thứ hai mới báo tiếp. Thông báo + dòng lỗi ghi cùng giao dịch với dòng việc.
func TestScanFailureNoticeRecordsOncePerRealFailure(t *testing.T) {
	s, pool := scanStore(t)
	ctx := context.Background()
	id := cid("noticeonce")
	lease, _ := addChannel(t, s, id)
	job := startHistory(t, s, id)
	claim(t, s, job, lease)

	// Dòng việc đang chạy trước, để lần failed là một lần chuyển thật.
	if _, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id)); err != nil {
		t.Fatal(err)
	}
	notices := func() (rows, total int) {
		t.Helper()
		if err := pool.QueryRow(ctx,
			`SELECT count(*), coalesce(sum(count), 0) FROM notifications WHERE kind = 'needs_attention'`).
			Scan(&rows, &total); err != nil {
			t.Fatalf("đọc thông báo: %v", err)
		}
		return rows, total
	}
	events := func() int {
		t.Helper()
		return countRows(t, pool, `SELECT count(*) FROM error_events`)
	}

	if rows, total := notices(); rows != 0 || total != 0 {
		t.Fatalf("chưa hỏng thì chưa có thông báo: %d/%d", rows, total)
	}

	if err := s.SetJobError(ctx, job.ID, lease, scan.JobError{ErrorCode: "blocked", Err: errors.New("kênh chặn")}); err != nil {
		t.Fatalf("đưa việc về error: %v", err)
	}
	failed, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
	if err != nil {
		t.Fatalf("sync lần hỏng: %v", err)
	}
	if failed.NotificationID == "" {
		t.Fatal("lần hỏng đầu phải ghi thông báo")
	}
	if failed.View.State != domain.TaskFailed {
		t.Fatalf("dòng việc phải failed: %s", failed.View.State)
	}
	if rows, total := notices(); rows != 1 || total != 1 {
		t.Fatalf("một lần hỏng phải một thông báo: %d dòng, count %d", rows, total)
	}
	if n := events(); n != 1 {
		t.Fatalf("một lần hỏng phải một dòng lỗi: %d", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM error_events WHERE object_ref->>'ID' = $1`, id); n != 1 {
		t.Fatalf("dòng lỗi phải trỏ đúng đối tượng: %d", n)
	}

	// Hai lần làm mới dòng đã failed: không báo lại.
	for i := 0; i < 2; i++ {
		again, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
		if err != nil {
			t.Fatalf("làm mới %d: %v", i, err)
		}
		if again.NotificationID != "" {
			t.Fatalf("làm mới không được báo lại: %s", again.NotificationID)
		}
	}
	if rows, total := notices(); rows != 1 || total != 1 {
		t.Fatalf("làm mới không được tăng thông báo: %d dòng, count %d", rows, total)
	}
	if n := events(); n != 1 {
		t.Fatalf("làm mới không được thêm dòng lỗi: %d", n)
	}

	// Việc chạy lại rồi mới tới một ảnh chụp lỗi cũ: không báo.
	if err := s.ControlBackfill(ctx, id, domain.ScanActionResume); err != nil {
		t.Fatalf("chạy lại: %v", err)
	}
	stale, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
	if err != nil {
		t.Fatalf("sync sau khi chạy lại: %v", err)
	}
	if stale.NotificationID != "" || stale.View.State == domain.TaskFailed {
		t.Fatalf("việc đã chạy lại không được báo lỗi: %+v", stale)
	}
	if rows, total := notices(); rows != 1 || total != 1 {
		t.Fatalf("ảnh chụp lỗi cũ không được tăng thông báo: %d dòng, count %d", rows, total)
	}

	// Lần hỏng thật thứ hai: báo tiếp.
	if err := s.ReleaseChannelLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	againLease := leaseOf(t, s, id, "worker")
	claimed, err := s.ClaimJob(ctx, job.ID, againLease)
	if err != nil || claimed == nil {
		t.Fatalf("claim lại: %+v %v", claimed, err)
	}
	if err := s.SetJobError(ctx, job.ID, againLease, scan.JobError{ErrorCode: "blocked", Err: errors.New("kênh chặn")}); err != nil {
		t.Fatalf("hỏng lần hai: %v", err)
	}
	second, err := s.SyncScanTask(ctx, job.ID, id, scanTaskSpec(id))
	if err != nil {
		t.Fatalf("sync lần hỏng thứ hai: %v", err)
	}
	if second.NotificationID == "" {
		t.Fatal("lần hỏng thứ hai phải báo")
	}
	if _, total := notices(); total != 2 {
		t.Fatalf("hai lần hỏng phải hai thông báo, được %d", total)
	}
	if n := events(); n != 2 {
		t.Fatalf("hai lần hỏng phải hai dòng lỗi: %d", n)
	}
}
