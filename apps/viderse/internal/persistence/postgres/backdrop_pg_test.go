// Ca test lưu trữ Video nền trên Postgres thật: gộp danh mục khi đọc lại (giữ
// published_at cũ khi lần sau không có), hết hạn VOD khi danh sách trọn thiếu
// video (clip thì không), keyset trang danh mục (NULLS LAST + con trỏ), cập
// nhật tải video lẻ (tiến độ, xong, lỗi), và yêu cầu nền tảng gộp theo host.
//
// Chạy trên schema riêng provision bằng pgtest với cặp role của Viderse rồi
// áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func seedBackdropChannel(t *testing.T, store *postgres.BackdropStore, id string, platform domain.PlatformID, handle string, now time.Time) {
	t.Helper()
	created, err := store.InsertChannel(context.Background(), domain.BackdropChannel{
		ID: id, Platform: platform, Handle: handle,
		URL: "https://example.test/" + handle, AddedAt: now,
	})
	if err != nil || !created {
		t.Fatalf("tạo kênh %s: created=%v err=%v", id, created, err)
	}
}

func TestBackdropCatalogueUpsertKeepsPublishedAndExpiresVods(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBackdropChannel(t, store, "twitch:chan", domain.PlatformTwitch, "chan", now)

	published := now.Add(-48 * time.Hour)
	duration := 1892.0
	thumb := "https://i.test/old.jpg"
	first := []domain.BackdropChannelVideo{
		{ChannelID: "twitch:chan", VideoID: "v1", Kind: domain.PlatformKindVod, URL: "u1", Title: "một", DurationSec: &duration, PublishedAt: &published, ThumbnailURL: &thumb, ListedAt: now, Position: 0},
		{ChannelID: "twitch:chan", VideoID: "v2", Kind: domain.PlatformKindVod, URL: "u2", Title: "hai", DurationSec: &duration, PublishedAt: &published, ListedAt: now, Position: 1},
		{ChannelID: "twitch:chan", VideoID: "c1", Kind: domain.PlatformKindClip, URL: "u3", Title: "clip", ListedAt: now, Position: 2},
	}
	if err := store.UpsertVideos(ctx, "twitch:chan", first); err != nil {
		t.Fatalf("đọc lần 1: %v", err)
	}

	// Lần đọc sau: v1 đổi tiêu đề và mất published, v2 biến mất khỏi danh sách
	// (VOD hết hạn), c1 vẫn còn; thêm v3.
	newThumb := "https://i.test/new.jpg"
	second := []domain.BackdropChannelVideo{
		{ChannelID: "twitch:chan", VideoID: "v1", Kind: domain.PlatformKindVod, URL: "u1", Title: "một (mới)", DurationSec: &duration, ThumbnailURL: &newThumb, ListedAt: now.Add(time.Minute), Position: 0},
		{ChannelID: "twitch:chan", VideoID: "c1", Kind: domain.PlatformKindClip, URL: "u3", Title: "clip", ListedAt: now.Add(time.Minute), Position: 1},
		{ChannelID: "twitch:chan", VideoID: "v3", Kind: domain.PlatformKindVod, URL: "u4", Title: "ba", DurationSec: &duration, ListedAt: now.Add(time.Minute), Position: 2},
	}
	if err := store.UpsertVideos(ctx, "twitch:chan", second); err != nil {
		t.Fatalf("đọc lần 2: %v", err)
	}
	if err := store.ExpireMissingVods(ctx, "twitch:chan", []string{"v1", "v3"}, now.Add(time.Minute)); err != nil {
		t.Fatalf("hết hạn: %v", err)
	}

	rows, err := store.ChannelVideos(ctx, "twitch:chan")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.BackdropChannelVideo{}
	for _, row := range rows {
		byID[row.VideoID] = row
	}
	v1 := byID["v1"]
	if v1.Title != "một (mới)" || v1.ThumbnailURL == nil || *v1.ThumbnailURL != newThumb {
		t.Fatalf("v1 = %+v", v1)
	}
	if v1.PublishedAt == nil || !v1.PublishedAt.Equal(published) {
		t.Fatalf("published_at cũ phải giữ khi lần sau thiếu: %v", v1.PublishedAt)
	}
	if v1.UnavailableAt != nil {
		t.Fatalf("v1 còn trong danh sách mà bị đánh hết hạn: %+v", v1)
	}
	if byID["v2"].UnavailableAt == nil || byID["v2"].UnavailableReason != domain.BackdropExpired {
		t.Fatalf("v2 phải expired: %+v", byID["v2"])
	}
	if byID["c1"].UnavailableAt != nil {
		t.Fatalf("clip không bao giờ hết hạn: %+v", byID["c1"])
	}

	// Video hết hạn quay lại: lần đọc kế tiếp tự dùng được lại (không cần probe).
	back := []domain.BackdropChannelVideo{
		{ChannelID: "twitch:chan", VideoID: "v2", Kind: domain.PlatformKindVod, URL: "u2", Title: "hai", DurationSec: &duration, PublishedAt: &published, ListedAt: now.Add(2 * time.Minute), Position: 0},
	}
	if err := store.UpsertVideos(ctx, "twitch:chan", back); err != nil {
		t.Fatal(err)
	}
	row, found, err := store.ChannelVideo(ctx, "twitch:chan", "v2")
	if err != nil || !found {
		t.Fatalf("v2: found=%v err=%v", found, err)
	}
	if row.UnavailableAt != nil || row.UnavailableReason != "" {
		t.Fatalf("v2 quay lại phải dùng được: %+v", row)
	}

	// Probe Bilibili ghi metadata + xoá hạn khi video trở lại.
	title := "tên thật"
	probeDur := 240.0
	probeAt := now.Add(3 * time.Minute)
	if err := store.UpdateChannelVideoProbe(ctx, "twitch:chan", "v3", backdrop.ProbeUpdate{
		Title: &title, DurationSec: &probeDur, ProbedAt: probeAt, ProbeInfo: []byte(`{"id":"v3"}`), ClearUnavailable: true,
	}); err != nil {
		t.Fatalf("probe: %v", err)
	}
	row, _, _ = store.ChannelVideo(ctx, "twitch:chan", "v3")
	if row.Title != title || row.DurationSec == nil || *row.DurationSec != probeDur || row.ProbedAt == nil || !row.ProbedAt.Equal(probeAt) {
		t.Fatalf("probe row = %+v", row)
	}
	if string(row.ProbeInfo) != `{"id": "v3"}` && string(row.ProbeInfo) != `{"id":"v3"}` {
		t.Fatalf("probeInfo = %q", row.ProbeInfo)
	}

	// Xoá kênh: danh mục đi theo.
	if deleted, err := store.DeleteChannel(ctx, "twitch:chan"); err != nil || !deleted {
		t.Fatalf("xoá kênh: deleted=%v err=%v", deleted, err)
	}
	if rows, err := store.ChannelVideos(ctx, "twitch:chan"); err != nil || len(rows) != 0 {
		t.Fatalf("danh mục còn lại %d dòng: %v", len(rows), err)
	}
}

func TestBackdropChannelVideosKeysetNullsLast(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBackdropChannel(t, store, "bilibili:42", domain.PlatformBilibili, "42", now)

	var videos []domain.BackdropChannelVideo
	for i := range 23 {
		published := now.Add(-time.Duration(i) * time.Hour)
		videos = append(videos, domain.BackdropChannelVideo{
			ChannelID: "bilibili:42", VideoID: videoID(i + 100), Kind: domain.PlatformKindVideo,
			URL: "u", Title: "v", PublishedAt: &published, ListedAt: now, Position: i,
		})
	}
	videos = append(videos, domain.BackdropChannelVideo{
		ChannelID: "bilibili:42", VideoID: videoID(1), Kind: domain.PlatformKindVideo,
		URL: "u", Title: "không ngày", ListedAt: now, Position: 99,
	})
	if err := store.UpsertVideos(ctx, "bilibili:42", videos); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	var cursor *backdrop.ChannelVideoCursor
	pages := 0
	for {
		rows, err := store.PageChannelVideos(ctx, backdrop.PageQuery{ChannelID: "bilibili:42", Cursor: cursor, Limit: 7})
		if err != nil {
			t.Fatalf("trang %d: %v", pages+1, err)
		}
		pages++
		for _, row := range rows {
			if seen[row.VideoID] {
				t.Fatalf("video %q lặp giữa các trang", row.VideoID)
			}
			seen[row.VideoID] = true
		}
		if len(rows) < 7 {
			break
		}
		last := rows[len(rows)-1]
		cursor = &backdrop.ChannelVideoCursor{ChannelID: "bilibili:42", PublishedAt: last.PublishedAt, VideoID: last.VideoID}
		if pages > 6 {
			t.Fatal("phân trang không kết thúc")
		}
	}
	if len(seen) != 24 {
		t.Fatalf("đọc %d video, muốn 24", len(seen))
	}
	rows, err := store.PageChannelVideos(ctx, backdrop.PageQuery{ChannelID: "bilibili:42", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if rows[len(rows)-1].VideoID != videoID(1) {
		t.Fatalf("mục cuối = %q, muốn video không ngày", rows[len(rows)-1].VideoID)
	}
	total, err := store.CountChannelVideos(ctx, "bilibili:42", nil)
	if err != nil || total != 24 {
		t.Fatalf("đếm = %d err=%v", total, err)
	}
	if err := store.MarkChannelVideoUnavailable(ctx, "bilibili:42", videoID(105), domain.BackdropUnavailable, now); err != nil {
		t.Fatal(err)
	}
	unavailable, err := store.CountUnavailableVideos(ctx, "bilibili:42", nil)
	if err != nil || unavailable != 1 {
		t.Fatalf("đếm unavailable = %d err=%v", unavailable, err)
	}
}

func TestBackdropVideoStateUpdates(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	duration := 300.0
	segment := domain.BackdropSegment{FromSec: 30, ToSec: 90}
	created, err := store.InsertVideo(ctx, domain.BackdropVideo{
		ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "https://www.twitch.tv/x/clip/Abc",
		Title: "Clip", DurationSec: &duration, Segment: segment, State: domain.BackdropDownloading,
		AddedAt: now, UpdatedAt: now,
	})
	if err != nil || !created {
		t.Fatalf("thêm: created=%v err=%v", created, err)
	}
	// Cùng id: trả dòng cũ, không tạo lại (không đổi dữ liệu đã có).
	if created, err := store.InsertVideo(ctx, domain.BackdropVideo{
		ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "u", Title: "khác",
		Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropReady,
		AddedAt: now, UpdatedAt: now,
	}); err != nil || created {
		t.Fatalf("thêm trùng: created=%v err=%v", created, err)
	}
	if row, _, _ := store.Video(ctx, "twitch:Abc"); row.Title != "Clip" {
		t.Fatalf("thêm trùng không được ghi đè: %+v", row)
	}

	progress := 0.4
	if err := store.UpdateVideo(ctx, "twitch:Abc", backdrop.VideoPatch{Progress: &progress}); err != nil {
		t.Fatal(err)
	}
	row, found, _ := store.Video(ctx, "twitch:Abc")
	if !found || row.Progress == nil || math.Abs(*row.Progress-progress) > 1e-4 {
		t.Fatalf("tiến độ = %+v", row.Progress)
	}

	path := "library/backdrops/twitch-Abc.mp4"
	height := 480
	size := int64(2048)
	ready := domain.BackdropReady
	if err := store.UpdateVideo(ctx, "twitch:Abc", backdrop.VideoPatch{
		State: &ready, ClearProgress: true, FilePath: &path, FileHeight: &height, FileBytes: &size,
	}); err != nil {
		t.Fatal(err)
	}
	row, _, _ = store.Video(ctx, "twitch:Abc")
	if row.State != domain.BackdropReady || row.Progress != nil {
		t.Fatalf("xong: state=%s progress=%v", row.State, row.Progress)
	}
	if row.FilePath != path || row.FileHeight != height || row.FileBytes != size {
		t.Fatalf("tệp = %+v", row)
	}

	// Đổi đoạn dùng: giữ nguyên tệp cho tới khi tải lại xong.
	newSegment := domain.BackdropSegment{FromSec: 60, ToSec: 120}
	if err := store.UpdateVideo(ctx, "twitch:Abc", backdrop.VideoPatch{Segment: &newSegment}); err != nil {
		t.Fatal(err)
	}
	row, _, _ = store.Video(ctx, "twitch:Abc")
	if row.Segment != newSegment || row.FilePath != path {
		t.Fatalf("đoạn mới = %+v", row)
	}

	code := backdrop.CodeRateLimited
	message := "Bilibili đang giới hạn yêu cầu"
	if err := store.UpdateVideo(ctx, "twitch:Abc", backdrop.VideoPatch{ErrorCode: &code, Error: &message, ClearProgress: true}); err != nil {
		t.Fatal(err)
	}
	row, _, _ = store.Video(ctx, "twitch:Abc")
	if row.ErrorCode != code || row.Error != message {
		t.Fatalf("lỗi = %+v", row)
	}

	list, err := store.Videos(ctx)
	if err != nil || len(list) != 1 || list[0].ID != "twitch:Abc" {
		t.Fatalf("danh sách = %+v err=%v", list, err)
	}
	if deleted, err := store.DeleteVideo(ctx, "twitch:Abc"); err != nil || !deleted {
		t.Fatalf("xoá: deleted=%v err=%v", deleted, err)
	}
	if _, found, _ := store.Video(ctx, "twitch:Abc"); found {
		t.Fatal("xoá rồi vẫn còn")
	}
}

func TestBackdropPlatformRequestUpsert(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	first := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)

	record, err := store.RecordPlatformRequest(ctx, "douyin.com", "Douyin", first)
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	if record.Host != "douyin.com" || record.Label != "Douyin" || record.Count != 1 || !record.FirstAt.Equal(first) {
		t.Fatalf("record = %+v", record)
	}
	later := first.Add(10 * time.Minute)
	record, err = store.RecordPlatformRequest(ctx, "douyin.com", "Douyin", later)
	if err != nil {
		t.Fatal(err)
	}
	if record.Count != 2 || !record.FirstAt.Equal(first) || !record.LastAt.Equal(later) {
		t.Fatalf("lần 2 = %+v, muốn count 2, first giữ nguyên, last mới", record)
	}
	if _, err := store.RecordPlatformRequest(ctx, "tiktok.com", "TikTok", later); err != nil {
		t.Fatal(err)
	}
	rows, err := store.PlatformRequests(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v err=%v", rows, err)
	}
	if rows[0].Host != "douyin.com" {
		t.Fatalf("thứ tự = %+v", rows)
	}
}

// TestBackdropDownloadClaimsGuardAndReconcile: claim theo lease, generation cũ
// không ghi đè lượt mới, và lượt mồ côi bị dọn khi lease hết.
func TestBackdropDownloadClaimsGuardAndReconcile(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	if created, err := store.InsertVideo(ctx, domain.BackdropVideo{
		ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "u", Title: "x",
		Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropDownloading,
		AddedAt: now, UpdatedAt: now,
	}); err != nil || !created {
		t.Fatalf("chèn: created=%v err=%v", created, err)
	}

	gen1, claimed, err := store.ClaimDownload(ctx, "twitch:Abc", "owner-a", now.Add(time.Minute), now, false)
	if err != nil || !claimed {
		t.Fatalf("giành lượt 1: claimed=%v err=%v", claimed, err)
	}
	if _, claimed, _ := store.ClaimDownload(ctx, "twitch:Abc", "owner-b", now.Add(time.Minute), now, false); claimed {
		t.Fatal("lease còn hạn thì process khác không giành được")
	}
	gen2, claimed, err := store.ClaimDownload(ctx, "twitch:Abc", "owner-b", now.Add(time.Minute), now, true)
	if err != nil || !claimed || gen2 <= gen1 {
		t.Fatalf("ép giành lượt 2: gen=%d claimed=%v err=%v", gen2, claimed, err)
	}
	if applied, _ := store.TouchDownload(ctx, "twitch:Abc", "owner-a", gen1, new(0.5), now.Add(time.Minute), now); applied {
		t.Fatal("lượt cũ không được ghi tiến độ")
	}
	if applied, _ := store.FinishDownload(ctx, "twitch:Abc", "owner-a", gen1, "x.mp4", 480, 10, now); applied {
		t.Fatal("lượt cũ không được chốt")
	}
	if applied, _ := store.TouchDownload(ctx, "twitch:Abc", "owner-b", gen2, new(0.75), now.Add(time.Minute), now); !applied {
		t.Fatal("lượt mới phải ghi được tiến độ")
	}
	row, _, _ := store.Video(ctx, "twitch:Abc")
	if row.Progress == nil || math.Abs(*row.Progress-0.75) > 1e-4 {
		t.Fatalf("progress = %v", row.Progress)
	}

	// Lease đã hết mốc: dọn về failed/interrupted để thêm lại được.
	if _, claimed, _ := store.ClaimDownload(ctx, "twitch:Abc", "owner-dead", now.Add(-time.Minute), now, true); !claimed {
		t.Fatal("giành lượt mồ côi")
	}
	cleared, err := store.ClearStaleDownloads(ctx, now)
	if err != nil || cleared != 1 {
		t.Fatalf("dọn = %d err=%v", cleared, err)
	}
	row, _, _ = store.Video(ctx, "twitch:Abc")
	if row.State != domain.BackdropFailed || row.ErrorCode != "interrupted" || row.Progress != nil {
		t.Fatalf("sau dọn = %+v", row)
	}
}

// TestBackdropDeleteLockSerializesTemplateSave: xoá kênh giữ khoá transaction
// theo kênh; lượt "lưu template" lấy cùng khoá sẽ chờ, rồi thấy kênh đã mất
// nên phải bỏ ghi — không còn khe giữa kiểm tồn tại và xoá.
func TestBackdropDeleteLockSerializesTemplateSave(t *testing.T) {
	pool := provision(t)
	store := postgres.NewBackdropStore(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBackdropChannel(t, store, "twitch:locked", domain.PlatformTwitch, "locked", now)

	locked := make(chan struct{})
	release := make(chan struct{})
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- store.WithTx(ctx, func(tx backdrop.Store) error {
			if err := tx.LockResource(ctx, backdrop.ChannelLockKey("twitch:locked")); err != nil {
				return err
			}
			close(locked)
			<-release
			if _, err := tx.DeleteChannel(ctx, "twitch:locked"); err != nil {
				return err
			}
			return nil
		})
	}()
	<-locked

	type saveResult struct {
		found bool
		at    time.Time
	}
	saved := make(chan saveResult, 1)
	go func() {
		_ = store.WithTx(context.Background(), func(tx backdrop.Store) error {
			if err := tx.LockResource(ctx, backdrop.ChannelLockKey("twitch:locked")); err != nil {
				return err
			}
			_, found, err := tx.Channel(ctx, "twitch:locked")
			saved <- saveResult{found: found, at: time.Now()}
			return err
		})
	}()

	select {
	case result := <-saved:
		t.Fatalf("lượt lưu chen được khi xoá chưa commit: %+v", result)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-deleteDone; err != nil {
		t.Fatalf("xoá: %v", err)
	}
	select {
	case result := <-saved:
		if result.found {
			t.Fatal("lượt lưu phải thấy kênh đã bị xoá")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lượt lưu không được đánh thức sau commit")
	}
}
