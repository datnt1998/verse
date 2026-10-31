package backdrop_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// fakePlatforms là PlatformState giả: giờ chờ, hạn mức và mốc đọc dần đặt tay.
type fakePlatforms struct {
	mu       sync.Mutex
	state    domain.PlatformState
	budget   platforms.BudgetReport
	trickles []time.Time
}

func (f *fakePlatforms) State(context.Context, domain.PlatformID) (domain.PlatformState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, true, nil
}

func (f *fakePlatforms) Budget(context.Context, domain.PlatformID) (platforms.BudgetReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.budget, nil
}

func (f *fakePlatforms) SetNextTrickle(_ context.Context, _ domain.PlatformID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trickles = append(f.trickles, at)
	f.state.NextTrickleAt = at
	return nil
}

// current đọc bản trạng thái mới nhất (an toàn khi scheduler chạy song song).
func (f *fakePlatforms) current() domain.PlatformState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// setTrickle hạ mốc đọc dần, mô phỏng người dùng bấm Làm mới / hết chặn.
func (f *fakePlatforms) setTrickle(at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.NextTrickleAt = at
}

// fakeRunner giả lập yt-dlp: `-J` trả metadata, lệnh tải ghi tệp + tiến độ.
type fakeRunner struct {
	probeJSON   string
	probes      int
	downloads   int
	downloadErr error
	progress    []string
	listings    map[string][]map[string]any
}

func (f *fakeRunner) run(_ context.Context, platform domain.PlatformID, args []string, _ int, _ time.Duration, onLine func(string)) (string, error) {
	for _, arg := range args {
		if arg == "-J" {
			f.probes++
			return f.probeJSON, nil
		}
	}
	f.downloads++
	if f.downloadErr != nil {
		return "", f.downloadErr
	}
	target := ""
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) {
			target = args[i+1]
		}
	}
	file := strings.ReplaceAll(target, "%(ext)s", "mp4")
	if file != "" {
		if err := os.WriteFile(file, []byte(strings.Repeat("v", 1024)), 0o644); err != nil {
			return "", err
		}
	}
	for _, percent := range f.progress {
		if onLine != nil {
			onLine("[download] " + percent + "% of 10MiB")
		}
	}
	return "", nil
}

func (f *fakeRunner) listing(_ context.Context, _ domain.PlatformID, args []string, _ int, _ time.Duration, _ func(string)) (string, error) {
	url := args[len(args)-1]
	key := "vod"
	switch {
	case strings.Contains(url, "/clips"):
		key = "clip"
	case strings.Contains(url, "filter=highlights"):
		key = "highlight"
	}
	raw, err := json.Marshal(map[string]any{"entries": f.listings[key]})
	return string(raw), err
}

// harness gói service cùng store + đồng hồ giả.
type harness struct {
	store    *memStore
	svc      *backdrop.Service
	platform *fakePlatforms
	runner   *fakeRunner
	now      time.Time
	root     string
	probeOn  func(ctx context.Context, mid string, pn int) (backdrop.BilibiliPage, error)
	wake     func()

	notesMu sync.Mutex
	notes   []string
}

// notified trả các ref đã phát qua Notify (SSE "backdrop").
func (h *harness) notified() []string {
	h.notesMu.Lock()
	defer h.notesMu.Unlock()
	return append([]string(nil), h.notes...)
}

func newHarness(t *testing.T, runnerBackend func(h *harness) backdrop.RunYtDlpFunc) *harness {
	t.Helper()
	h := &harness{
		store:    newMemStore(),
		platform: &fakePlatforms{budget: platforms.BudgetReport{AvailableAt: time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)}},
		now:      time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC),
		root:     t.TempDir(),
	}
	h.runner = &fakeRunner{probeJSON: `{"title":"Clip","duration":120}`, progress: []string{"10", "20", "30", "100"}}
	var run backdrop.RunYtDlpFunc
	if runnerBackend != nil {
		run = runnerBackend(h)
	}
	if run == nil {
		run = h.runner.run
	}
	h.svc = backdrop.New(backdrop.Deps{
		Store:       h.store,
		Platforms:   h.platform,
		RunYtDlp:    run,
		CfgDir:      t.TempDir(),
		Doer:        nil,
		Now:         func() time.Time { return h.now },
		Random:      func() float64 { return 0.5 },
		Spawn:       func(ctx context.Context, fn func(context.Context)) { fn(ctx) },
		Background:  func() context.Context { return context.Background() },
		StorageRoot: func() string { return h.root },
		ResolvePath: func(rel string) (string, error) { return filepath.Join(h.root, rel), nil },
		BilibiliPage: func(ctx context.Context, mid string, pn int) (backdrop.BilibiliPage, error) {
			if h.probeOn == nil {
				return backdrop.BilibiliPage{}, errors.New("không có trang giả")
			}
			return h.probeOn(ctx, mid, pn)
		},
		Notify: func(ref string) {
			h.notesMu.Lock()
			defer h.notesMu.Unlock()
			h.notes = append(h.notes, ref)
		},
	})
	return h
}

func vodJSON(id string, duration float64) map[string]any {
	return map[string]any{"id": id, "url": "https://www.twitch.tv/videos/" + id[1:], "title": "VOD " + id, "duration": duration}
}

func clipJSON(slug string) map[string]any {
	return map[string]any{"id": "1", "url": "https://www.twitch.tv/somechannel/clip/" + slug, "title": "Clip", "duration": 30.0, "timestamp": 1700000000}
}

func TestAddTwitchChannelIngestsAndExpiresMissingVods(t *testing.T) {
	h := newHarness(t, func(h *harness) backdrop.RunYtDlpFunc {
		h.runner.listings = map[string][]map[string]any{
			"vod":       {vodJSON("v1", 3600), vodJSON("v2", 1800)},
			"highlight": {vodJSON("v3", 600)},
			"clip":      {clipJSON("AbcDef")},
		}
		return h.runner.listing
	})
	ctx := context.Background()
	view, created, err := h.svc.AddChannel(ctx, "https://www.twitch.tv/somechannel")
	if err != nil || !created {
		t.Fatalf("add: %v created=%v", err, created)
	}
	if view.Channel.ID != "twitch:somechannel" || view.Channel.Title == nil || *view.Channel.Title != "somechannel" {
		t.Fatalf("channel = %+v", view.Channel)
	}
	if view.Counts[domain.PlatformKindVod] != 2 || view.Counts[domain.PlatformKindHighlight] != 1 || view.Counts[domain.PlatformKindClip] != 1 {
		t.Fatalf("counts = %v", view.Counts)
	}
	if view.Channel.ListedAt == nil {
		t.Fatal("lần đọc thành công phải đặt listedAt")
	}

	// Danh sách trọn không còn v1: v1 hết hạn, v2 còn.
	h.runner.listings["vod"] = []map[string]any{vodJSON("v2", 1800)}
	refreshed, err := h.svc.RefreshChannel(ctx, "twitch:somechannel", true)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.Counts[domain.PlatformKindVod] != 1 {
		t.Fatalf("counts sau khi mất VOD = %v", refreshed.Counts)
	}
	rows := h.store.videoRows("twitch:somechannel")
	if rows["v1"].UnavailableReason != domain.BackdropExpired || rows["v1"].UnavailableAt == nil {
		t.Fatalf("v1 = %+v, muốn expired", rows["v1"])
	}
	if rows["AbcDef"].UnavailableAt != nil {
		t.Fatal("clip không bao giờ hết hạn")
	}

	// Video quay lại thì tự dùng được lần nữa.
	h.runner.listings["vod"] = []map[string]any{vodJSON("v1", 3600), vodJSON("v2", 1800)}
	refreshed, err = h.svc.RefreshChannel(ctx, "twitch:somechannel", true)
	if err != nil {
		t.Fatalf("refresh 2: %v", err)
	}
	if refreshed.Counts[domain.PlatformKindVod] != 2 {
		t.Fatalf("counts sau khi video trở lại = %v", refreshed.Counts)
	}
}

func TestChannelVideosKeysetPagination(t *testing.T) {
	h := newHarness(t, nil)
	base := h.now
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:somechannel", Platform: domain.PlatformTwitch, Handle: "somechannel", URL: "https://www.twitch.tv/somechannel", AddedAt: base})
	for i := range 30 {
		at := base.Add(-time.Duration(i) * time.Hour)
		id := "v" + strings.Repeat("0", 1) + itoaTest(100+i)
		h.store.seedVideos("twitch:somechannel", domain.BackdropChannelVideo{
			VideoID: id, Kind: domain.PlatformKindVod, URL: "https://example.test/" + id,
			Title: "video " + itoaTest(i), PublishedAt: &at, Position: i, ListedAt: base,
		})
	}
	h.store.seedVideos("twitch:somechannel", domain.BackdropChannelVideo{
		VideoID: "v000", Kind: domain.PlatformKindVod, URL: "https://example.test/v000",
		Title: "không ngày", Position: 99, ListedAt: base,
	})

	ctx := context.Background()
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page, err := h.svc.ChannelVideos(ctx, "twitch:somechannel", backdrop.ChannelVideosOptions{Limit: 7, Cursor: cursor})
		if err != nil {
			t.Fatalf("trang %d: %v", pages+1, err)
		}
		pages++
		if page.Total != 31 {
			t.Fatalf("total = %d, muốn 31", page.Total)
		}
		for _, item := range page.Items {
			if seen[item.Video.VideoID] {
				t.Fatalf("mã %q lặp giữa các trang", item.Video.VideoID)
			}
			seen[item.Video.VideoID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 10 {
			t.Fatal("phân trang không kết thúc")
		}
	}
	if len(seen) != 31 {
		t.Fatalf("thấy %d video, muốn 31", len(seen))
	}
	// Video không ngày nằm cuối theo NULLS LAST.
	last, err := h.svc.ChannelVideos(ctx, "twitch:somechannel", backdrop.ChannelVideosOptions{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if last.Items[len(last.Items)-1].Video.VideoID != "v000" {
		t.Fatalf("mục cuối = %q, muốn v000", last.Items[len(last.Items)-1].Video.VideoID)
	}

	// Con trỏ của bộ lọc khác là lỗi, không phải trang lạ.
	vodPage, err := h.svc.ChannelVideos(ctx, "twitch:somechannel", backdrop.ChannelVideosOptions{Limit: 7, Kinds: []domain.PlatformKind{domain.PlatformKindVod}})
	if err != nil || vodPage.NextCursor == "" {
		t.Fatalf("trang lọc vod: %v cursor=%q", err, vodPage.NextCursor)
	}
	if _, err := h.svc.ChannelVideos(ctx, "twitch:somechannel", backdrop.ChannelVideosOptions{Limit: 7, Cursor: vodPage.NextCursor, Kinds: []domain.PlatformKind{domain.PlatformKindClip}}); err == nil {
		t.Fatal("con trỏ lệch bộ lọc phải bị từ chối")
	}
	// Loại không thuộc nền tảng.
	if _, err := h.svc.ChannelVideos(ctx, "twitch:somechannel", backdrop.ChannelVideosOptions{Kinds: []domain.PlatformKind{domain.PlatformKindVideo}}); err == nil {
		t.Fatal("loại video của Bilibili không dùng được cho Twitch")
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestChannelViewsReportUsedByAndChosenBy(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:nightdrivefm", Platform: domain.PlatformTwitch, Handle: "nightdrivefm", URL: "https://www.twitch.tv/nightdrivefm", Title: new("Night Drive FM"), AddedAt: h.now})
	at := h.now.Add(-time.Hour)
	h.store.seedVideos("twitch:nightdrivefm",
		domain.BackdropChannelVideo{VideoID: "v1", Kind: domain.PlatformKindVod, URL: "u1", Title: "một", PublishedAt: &at, ListedAt: h.now, Position: 0},
		domain.BackdropChannelVideo{VideoID: "v2", Kind: domain.PlatformKindVod, URL: "u2", Title: "hai", PublishedAt: &at, ListedAt: h.now, Position: 1},
	)
	recipe := `{"backdrop":{"channels":[{"channelId":"twitch:nightdrivefm","kinds":["vod"],"pinned":["v1"]}],"clipSec":60,"fixedVideoIds":[]},"appearance":{"speed":1}}`
	h.store.seedTemplate("t1", "Lofi 2 giờ", recipe)

	view, err := h.svc.Channel(ctx, "twitch:nightdrivefm")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.UsedBy) != 1 || view.UsedBy[0].Mode != "pick" || view.UsedBy[0].ChosenCount != 1 || view.UsedBy[0].Name != "Lofi 2 giờ" {
		t.Fatalf("usedBy = %+v", view.UsedBy)
	}

	page, err := h.svc.ChannelVideos(ctx, "twitch:nightdrivefm", backdrop.ChannelVideosOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chosen := map[string]int{}
	for _, item := range page.Items {
		chosen[item.Video.VideoID] = len(item.ChosenBy)
	}
	if chosen["v1"] != 1 || chosen["v2"] != 0 {
		t.Fatalf("chosenBy = %v", chosen)
	}

	if err := h.svc.RemoveChannel(ctx, "twitch:nightdrivefm"); err == nil {
		t.Fatal("kênh đang dùng phải bị chặn xoá")
	} else {
		var apiErr *backdrop.Error
		if !errors.As(err, &apiErr) || apiErr.Code != backdrop.CodeInUse || !strings.Contains(apiErr.Message, "Lofi 2 giờ") {
			t.Fatalf("err = %v", err)
		}
		if apiErr.Recovery == "" {
			t.Fatal("lỗi in_use phải kèm gợi ý phục hồi")
		}
	}
}

func TestRemoveChannelWhenUnused(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:x", Platform: domain.PlatformTwitch, Handle: "x", URL: "https://www.twitch.tv/x", AddedAt: h.now})
	if err := h.svc.RemoveChannel(ctx, "twitch:x"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := h.store.videoRows("twitch:x")["v1"]; ok {
		t.Fatal("danh mục của kênh phải bị xoá theo")
	}
	if err := h.svc.RemoveChannel(ctx, "twitch:x"); err == nil {
		t.Fatal("xoá kênh không tồn tại phải trả not_found")
	}
}

func TestAddVideoProbesDownloadsAndThrottlesProgress(t *testing.T) {
	h := newHarness(t, nil)
	h.runner.probeJSON = `{"title":"Đêm Sài Gòn","duration":120,"thumbnail":"https://i.test/a.jpg"}`
	ctx := context.Background()
	view, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/somechannel/clip/AbcDef", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if h.runner.probes != 1 || h.runner.downloads != 1 {
		t.Fatalf("probes=%d downloads=%d, muốn 1/1", h.runner.probes, h.runner.downloads)
	}
	video := view.Video
	if video.ID != "twitch:AbcDef" || video.State != domain.BackdropReady {
		t.Fatalf("video = %+v", video)
	}
	if video.Segment.FromSec != 0 || video.Segment.ToSec != 120 {
		t.Fatalf("đoạn mặc định = %+v, muốn trọn 120 giây", video.Segment)
	}
	if video.FilePath != "library/backdrops/twitch-AbcDef.mp4" || video.FileBytes != 1024 || video.FileHeight != 480 {
		t.Fatalf("file = %+v", video)
	}
	if _, err := os.Stat(filepath.Join(h.root, video.FilePath)); err != nil {
		t.Fatalf("tệp phải tồn tại: %v", err)
	}
	// Đồng hồ đứng yên: chỉ mốc đầu qua cửa sổ, các mốc sau (kể cả 100%) bị
	// gộp — không có ngoại lệ cho 100%.
	if writes := h.store.progressWriteCount(); writes != 1 {
		t.Fatalf("ghi tiến độ = %d, muốn 1 (throttle 4 lần/giây, gộp cả 100%%)", writes)
	}
	if view.Video.Progress != nil {
		t.Fatalf("xong thì progress = nil, được %v", *view.Video.Progress)
	}

	// Thêm lại cùng video: trả dòng cũ, không dò lại.
	before := h.runner.probes
	again, err := h.svc.AddVideo(ctx, "https://clips.twitch.tv/AbcDef", nil)
	if err != nil {
		t.Fatalf("add lại: %v", err)
	}
	if again.Video.ID != video.ID || h.runner.probes != before {
		t.Fatalf("thêm lại phải trả dòng cũ (probes=%d)", h.runner.probes)
	}
	if h.runner.downloads != 1 {
		t.Fatalf("thêm lại video đã xong không được tải lại (downloads=%d)", h.runner.downloads)
	}
}

func TestAddVideoSegmentValidation(t *testing.T) {
	h := newHarness(t, nil)
	h.runner.probeJSON = `{"title":"x","duration":120}`
	ctx := context.Background()

	if _, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", &domain.BackdropSegment{FromSec: 0, ToSec: 700}); err == nil {
		t.Fatal("đoạn > 600 giây phải bị từ chối")
	}
	if _, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", &domain.BackdropSegment{FromSec: 0, ToSec: 200}); err == nil {
		t.Fatal("đoạn vượt độ dài video phải bị từ chối")
	}
	if _, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", &domain.BackdropSegment{FromSec: 30, ToSec: 30}); err == nil {
		t.Fatal("đoạn rỗng phải bị từ chối")
	}
	view, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", &domain.BackdropSegment{FromSec: 10, ToSec: 70})
	if err != nil {
		t.Fatalf("đoạn hợp lệ: %v", err)
	}
	if view.Video.Segment.FromSec != 10 || view.Video.Segment.ToSec != 70 {
		t.Fatalf("segment = %+v", view.Video.Segment)
	}
	if _, err := h.svc.AddVideo(ctx, "https://space.bilibili.com/123456", nil); err == nil {
		t.Fatal("link kênh không phải video lẻ")
	}
}

func TestUpdateVideoSegmentRedownloadsAndDeleteGuard(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.runner.probeJSON = `{"title":"x","duration":300}`
	view, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	downloads := h.runner.downloads
	updated, err := h.svc.UpdateVideoSegment(ctx, "twitch:Abc", domain.BackdropSegment{FromSec: 100, ToSec: 160})
	if err != nil {
		t.Fatalf("đổi đoạn: %v", err)
	}
	if h.runner.downloads != downloads+1 || updated.Video.State != domain.BackdropReady {
		t.Fatalf("downloads=%d state=%s", h.runner.downloads, updated.Video.State)
	}
	if _, err := h.svc.UpdateVideoSegment(ctx, "twitch:Abc", domain.BackdropSegment{FromSec: 0, ToSec: 601}); err == nil {
		t.Fatal("đoạn > 600 phải bị từ chối")
	}
	if view.Video.ID != "twitch:Abc" {
		t.Fatalf("id = %q", view.Video.ID)
	}

	h.store.seedTemplate("t1", "Có clip cố định", `{"backdrop":{"channels":[],"clipSec":60,"fixedVideoIds":["twitch:Abc"]}}`)
	if err := h.svc.RemoveVideo(ctx, "twitch:Abc"); err == nil {
		t.Fatal("video đang ghim phải bị chặn xoá")
	} else {
		var apiErr *backdrop.Error
		if !errors.As(err, &apiErr) || apiErr.Code != backdrop.CodeInUse {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestDownloadFailureMarksFailed(t *testing.T) {
	run := func(ctx context.Context, platform domain.PlatformID, args []string, cost int, timeout time.Duration, onLine func(string)) (string, error) {
		for _, arg := range args {
			if arg == "-J" {
				return `{"title":"x","duration":60}`, nil
			}
		}
		return "", &backdrop.RateLimitError{Message: "Bilibili đang giới hạn yêu cầu"}
	}
	h := newHarness(t, func(*harness) backdrop.RunYtDlpFunc { return run })
	view, err := h.svc.AddVideo(context.Background(), "https://www.twitch.tv/x/clip/Abc", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if view.Video.State != domain.BackdropFailed || view.Video.ErrorCode != backdrop.CodeRateLimited {
		t.Fatalf("video = %+v", view.Video)
	}
	if view.Video.Progress != nil {
		t.Fatalf("thất bại thì progress = nil")
	}
}

func TestMixAcceptanceExample(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	base := h.now
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:v1", Platform: domain.PlatformTwitch, Title: "fixed 1", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 85}, State: domain.BackdropReady, AddedAt: base})
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:v2", Platform: domain.PlatformTwitch, Title: "fixed 2", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 85}, State: domain.BackdropReady, AddedAt: base})
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:slowroads", Platform: domain.PlatformTwitch, Handle: "slowroads", URL: "https://www.twitch.tv/slowroads", AddedAt: base})
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:nightdrivefm", Platform: domain.PlatformTwitch, Handle: "nightdrivefm", URL: "https://www.twitch.tv/nightdrivefm", AddedAt: base})
	for i := range 50 {
		at := base.Add(-time.Duration(i) * time.Hour)
		h.store.seedVideos("twitch:slowroads", domain.BackdropChannelVideo{
			VideoID: "s" + itoaTest(i), Kind: domain.PlatformKindVod, URL: "s", Title: "s",
			PublishedAt: &at, DurationSec: new(600.0), ListedAt: base, Position: i,
		})
	}
	for i := range 4 {
		h.store.seedVideos("twitch:nightdrivefm", domain.BackdropChannelVideo{
			VideoID: "n" + itoaTest(i), Kind: domain.PlatformKindVod, URL: "n", Title: "n",
			DurationSec: new(300.0), ListedAt: base, Position: i,
		})
	}
	recipe := `{"backdrop":{"channels":[
		{"channelId":"twitch:slowroads","kinds":["vod"],"pinned":[]},
		{"channelId":"twitch:nightdrivefm","kinds":["vod"],"pinned":["n0","n1","n2","n3"]}
	],"clipSec":60,"fixedVideoIds":["twitch:v1","twitch:v2"]},"appearance":{"speed":1}}`
	h.store.seedTemplate("t1", "Mix", recipe)

	report, err := backdrop.NewMixReader(h.store).Mix(ctx, "t1", 42*60)
	if err != nil {
		t.Fatalf("mix: %v", err)
	}
	if report.FixedSec != 170 {
		t.Fatalf("fixedSec = %v, muốn 170", report.FixedSec)
	}
	if report.Need != 40 || report.Capped {
		t.Fatalf("need=%d capped=%v, muốn 40/false", report.Need, report.Capped)
	}
	if report.LoopSec != 0 {
		t.Fatalf("loopSec = %v, muốn 0", report.LoopSec)
	}
	clips := map[string]int{}
	fresh := map[string]int{}
	for _, channel := range report.PerChannel {
		clips[channel.ChannelID] = channel.Clips
		fresh[channel.ChannelID] = channel.FreshVideos
	}
	if clips["twitch:slowroads"] != 36 || clips["twitch:nightdrivefm"] != 4 {
		t.Fatalf("clips = %v, muốn 36 + 4", clips)
	}
	if fresh["twitch:nightdrivefm"] != 4 {
		t.Fatalf("fresh = %v", fresh)
	}
}

func TestMixCappedAndMissingTemplate(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:a", Platform: domain.PlatformTwitch, Handle: "a", URL: "u", AddedAt: h.now})
	for i := range 60 {
		h.store.seedVideos("twitch:a", domain.BackdropChannelVideo{
			VideoID: "v" + itoaTest(i), Kind: domain.PlatformKindVod, URL: "u", Title: "t",
			DurationSec: new(600.0), ListedAt: h.now, Position: i,
		})
	}
	h.store.seedTemplate("t1", "Dài", `{"backdrop":{"channels":[{"channelId":"twitch:a","kinds":["vod"],"pinned":[]}],"clipSec":60}}`)
	report, err := backdrop.NewMixReader(h.store).Mix(ctx, "t1", 60*60)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Capped || report.Need != backdrop.MaxBackdropPicks {
		t.Fatalf("report = %+v, muốn capped với 40 clip", report)
	}
	if report.LoopSec <= 0 {
		t.Fatalf("thiếu giây phải báo loopSec > 0, được %v", report.LoopSec)
	}
	if _, err := backdrop.NewMixReader(h.store).Mix(ctx, "nope", 60); err == nil {
		t.Fatal("template không tồn tại phải lỗi")
	}
}

func TestTrickleAdvancesPagesThenStops(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	pages := map[int]backdrop.BilibiliPage{
		1: {Total: 5, PageSize: 2, Videos: biliVideos("a", "b"), Title: new("Kênh")},
		2: {Total: 5, PageSize: 2, Videos: biliVideos("c", "d")},
		3: {Total: 5, PageSize: 2, Videos: biliVideos("e")},
	}
	var requested []int
	h.probeOn = func(_ context.Context, _ string, pn int) (backdrop.BilibiliPage, error) {
		requested = append(requested, pn)
		return pages[pn], nil
	}
	h.store.seedChannel(domain.BackdropChannel{
		ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123",
		AddedAt: h.now, ScanNextPage: new(1),
	})
	for range 3 {
		if h.platform.state.NextTrickleAt.After(h.now) {
			h.now = h.platform.state.NextTrickleAt
		}
		served, err := h.svc.Trickle(ctx)
		if err != nil {
			t.Fatalf("trickle: %v", err)
		}
		if !served {
			ch, _, _ := h.store.Channel(ctx, "bilibili:123")
			t.Fatalf("trang phải được đọc khi đã tới mốc: %+v", ch)
		}
	}
	if len(requested) != 3 || requested[0] != 1 || requested[2] != 3 {
		t.Fatalf("requested = %v, muốn 1,2,3", requested)
	}
	channel, _, _ := h.store.Channel(ctx, "bilibili:123")
	if channel.ScanNextPage != nil {
		t.Fatalf("hết trang thì scanNextPage = nil, được %d", *channel.ScanNextPage)
	}
	if channel.Title == nil || *channel.Title != "Kênh" {
		t.Fatalf("title = %v", channel.Title)
	}
	if _, err := h.svc.Trickle(ctx); err != nil {
		t.Fatal(err)
	}
	if len(requested) != 3 {
		t.Fatalf("không còn trang thì không gọi thêm, requested = %v", requested)
	}
	if len(h.platform.trickles) != 3 {
		t.Fatalf("trickles = %d, muốn 3 (mỗi trang một mốc)", len(h.platform.trickles))
	}
}

func biliVideos(ids ...string) []backdrop.ListedVideo {
	out := make([]backdrop.ListedVideo, 0, len(ids))
	for _, id := range ids {
		out = append(out, backdrop.ListedVideo{VideoID: id, Kind: domain.PlatformKindVideo, URL: "https://www.bilibili.com/video/" + id, Title: new(id)})
	}
	return out
}

func TestTrickleStopsOnRiskControlAndSchedulesNext(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.probeOn = func(context.Context, string, int) (backdrop.BilibiliPage, error) {
		return backdrop.BilibiliPage{}, &backdrop.RateLimitError{Message: "Bilibili đang giới hạn yêu cầu", Retry: 20 * time.Minute}
	}
	h.store.seedChannel(domain.BackdropChannel{
		ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123",
		AddedAt: h.now, ScanNextPage: new(1),
	})
	served, err := h.svc.Trickle(ctx)
	if err != nil {
		t.Fatalf("risk-control không được nổi lên: %v", err)
	}
	if served {
		t.Fatal("trang bị chặn không tính là vừa đọc")
	}
	if len(h.platform.trickles) != 1 {
		t.Fatalf("trickles = %d, muốn 1 (hẹn lượt sau)", len(h.platform.trickles))
	}
	channel, _, _ := h.store.Channel(ctx, "bilibili:123")
	if channel.ListErrorCode != backdrop.CodeRateLimited || channel.ListError == "" {
		t.Fatalf("phải ghi lỗi đọc được vào kênh: %+v", channel)
	}
	if channel.ScanNextPage == nil {
		t.Fatal("bị chặn thì con trỏ còn nguyên")
	}
}

func TestNextDueHonorsTricklePauseAndBudget(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedChannel(domain.BackdropChannel{
		ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123",
		AddedAt: h.now, ScanNextPage: new(2),
	})
	later := h.now.Add(30 * time.Minute)
	h.platform.state.NextTrickleAt = later
	h.platform.budget.AvailableAt = h.now

	served, err := h.svc.Trickle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if served {
		t.Fatal("chưa tới mốc trickle thì không đọc")
	}
	if len(h.platform.trickles) != 0 {
		t.Fatal("không đọc thì không đặt mốc mới")
	}

	// Nghỉ bị chặn tới 45 phút: không đọc dù trickle đã tới.
	h.platform.state.NextTrickleAt = h.now
	h.platform.state.PausedUntil = ptrTime(h.now.Add(45 * time.Minute))
	served, err = h.svc.Trickle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if served {
		t.Fatal("đang chờ bị chặn thì không đọc")
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestRecordPlatformRequestIsIdempotentPerHost(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	first, err := h.svc.RecordPlatformRequest(ctx, "https://www.douyin.com/video/1")
	if err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	second, err := h.svc.RecordPlatformRequest(ctx, "douyin.com")
	if err != nil {
		t.Fatalf("lần 2: %v", err)
	}
	if first.Host != "douyin.com" || first.Label != "Douyin" {
		t.Fatalf("record = %+v", first)
	}
	if second.Count != 2 {
		t.Fatalf("count = %d, muốn 2 (một dòng, hai lần bấm)", second.Count)
	}
	rows, err := h.svc.PlatformRequests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Host != "douyin.com" {
		t.Fatalf("rows = %+v", rows)
	}
	h.now = h.now.Add(time.Minute)
	third, _ := h.svc.RecordPlatformRequest(ctx, "douyin.com")
	if !third.LastAt.After(second.LastAt) {
		t.Fatalf("lastAt phải cập nhật: %v → %v", second.LastAt, third.LastAt)
	}
	if _, err := h.svc.RecordPlatformRequest(ctx, "vimeo.com"); err != nil {
		t.Fatalf("vimeo là nền tảng chưa hỗ trợ hợp lệ: %v", err)
	}
	if _, err := h.svc.RecordPlatformRequest(ctx, "twitch.tv"); err == nil {
		t.Fatal("twitch đã hỗ trợ, không phải yêu cầu hợp lệ")
	}
}

func TestUsageCountsChannelsAndVideos(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:a", Platform: domain.PlatformTwitch, Handle: "a", URL: "u", AddedAt: h.now})
	h.store.seedChannel(domain.BackdropChannel{ID: "bilibili:1", Platform: domain.PlatformBilibili, Handle: "1", URL: "u", AddedAt: h.now})
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:v9", Platform: domain.PlatformTwitch, Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropReady})
	usage, err := h.svc.Usage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if usage[domain.PlatformTwitch].Channels != 1 || usage[domain.PlatformTwitch].Videos != 1 {
		t.Fatalf("twitch = %+v", usage[domain.PlatformTwitch])
	}
	if usage[domain.PlatformBilibili].Channels != 1 || usage[domain.PlatformBilibili].Videos != 0 {
		t.Fatalf("bilibili = %+v", usage[domain.PlatformBilibili])
	}
}

func TestAddBilibiliChannelQueuesWhenBlocked(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.probeOn = func(context.Context, string, int) (backdrop.BilibiliPage, error) {
		return backdrop.BilibiliPage{}, &backdrop.RateLimitError{Message: "Bilibili đang giới hạn yêu cầu"}
	}
	view, created, err := h.svc.AddChannel(ctx, "https://space.bilibili.com/123")
	if err != nil {
		t.Fatalf("bị chặn phải xếp hàng chứ không lỗi: %v", err)
	}
	if !created || view.Channel.ScanNextPage == nil || *view.Channel.ScanNextPage != 1 {
		t.Fatalf("view = %+v", view.Channel)
	}
	if view.Channel.ListErrorCode != backdrop.CodeRateLimited {
		t.Fatalf("listError = %+v", view.Channel.ListErrorCode)
	}
}

// sortVideoIDs chỉ để đọc kết quả dễ hơn khi lỗi.
func sortVideoIDs(items []backdrop.VideoItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Video.VideoID)
	}
	sort.Strings(ids)
	return ids
}

// TestRunSchedulerSleepsUntilDueThenWakes đo nhịp rảnh: bộ lập lịch ngủ tới
// mốc trang kế (không hỏi vòng), đọc đúng một trang khi tới mốc, rồi ngủ tiếp;
// Wake đánh thức ngay khi mốc bị hạ (Làm mới, hết chặn, đăng nhập mới).
func TestRunSchedulerSleepsUntilDueThenWakes(t *testing.T) {
	h := newHarness(t, nil)
	h.probeOn = func(_ context.Context, _ string, pn int) (backdrop.BilibiliPage, error) {
		ids := []string{"a", "b"}
		if pn == 2 {
			ids = []string{"c", "d"}
		}
		return backdrop.BilibiliPage{Total: 5, PageSize: 2, Videos: biliVideos(ids...)}, nil
	}
	h.store.seedChannel(domain.BackdropChannel{
		ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123",
		AddedAt: time.Now(), ScanNextPage: new(1),
	})
	h.platform.setTrickle(time.Now().Add(120 * time.Millisecond))

	var scans atomic.Int64
	svc := backdrop.New(backdrop.Deps{
		Store:       h.store,
		Platforms:   h.platform,
		RunYtDlp:    h.runner.run,
		CfgDir:      t.TempDir(),
		Spawn:       func(ctx context.Context, fn func(context.Context)) { fn(ctx) },
		Background:  func() context.Context { return context.Background() },
		StorageRoot: func() string { return h.root },
		ResolvePath: func(rel string) (string, error) { return filepath.Join(h.root, rel), nil },
		BilibiliPage: func(ctx context.Context, mid string, pn int) (backdrop.BilibiliPage, error) {
			scans.Add(1)
			return h.probeOn(ctx, mid, pn)
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.RunScheduler(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Chưa tới mốc: không được đọc, không được đốt CPU hỏi vòng.
	time.Sleep(40 * time.Millisecond)
	if got := scans.Load(); got != 0 {
		t.Fatalf("chưa tới mốc mà đã đọc %d trang", got)
	}
	// Tới mốc: đúng một trang.
	deadline := time.Now().Add(time.Second)
	for scans.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := scans.Load(); got != 1 {
		t.Fatalf("tới mốc đọc %d trang, muốn 1", got)
	}
	// Sau khi đọc, mốc mới ở tương lai: im lặng, không đọc lại trang vừa đọc.
	time.Sleep(120 * time.Millisecond)
	if got := scans.Load(); got != 1 {
		t.Fatalf("qua mốc rồi vẫn đọc thêm %d trang", got-1)
	}

	// Hạ mốc + Wake (Làm mới/hết chặn): đọc ngay, không chờ hết giờ ngủ.
	h.platform.setTrickle(time.Now())
	svc.Wake()
	deadline = time.Now().Add(time.Second)
	for scans.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := scans.Load(); got != 2 {
		t.Fatalf("Wake không đọc ngay: %d trang", got)
	}
}

// TestNotifyFiresOnCatalogueAndDownload kiểm SSE "backdrop": đọc danh mục, tiến
// độ tải (đã gộp 4 lần/giây ở web.Notifier) và lúc xong đều phát ref tương ứng.
func TestNotifyFiresOnCatalogueAndDownload(t *testing.T) {
	h := newHarness(t, func(h *harness) backdrop.RunYtDlpFunc {
		h.runner.listings = map[string][]map[string]any{
			"vod":       {vodJSON("v1", 3600)},
			"highlight": {},
			"clip":      {},
		}
		return func(ctx context.Context, platform domain.PlatformID, args []string, cost int, timeout time.Duration, onLine func(string)) (string, error) {
			for _, arg := range args {
				if arg == "--flat-playlist" {
					return h.runner.listing(ctx, platform, args, cost, timeout, onLine)
				}
			}
			return h.runner.run(ctx, platform, args, cost, timeout, onLine)
		}
	})
	ctx := context.Background()
	if _, _, err := h.svc.AddChannel(ctx, "https://www.twitch.tv/somechannel"); err != nil {
		t.Fatal(err)
	}
	if notes := h.notified(); len(notes) == 0 || notes[len(notes)-1] != "twitch:somechannel" {
		t.Fatalf("thêm kênh phải phát SSE ref kênh: %v", notes)
	}

	before := len(h.notified())
	view, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", nil)
	if err != nil || view.Video.State != domain.BackdropReady {
		t.Fatalf("thêm video: %+v %v", view.Video, err)
	}
	notes := h.notified()[before:]
	if len(notes) < 2 {
		t.Fatalf("tải video phải phát ít nhất lúc bắt đầu và lúc xong: %v", notes)
	}
	for _, ref := range notes {
		if ref != "twitch:Abc" {
			t.Fatalf("ref lạ: %v", notes)
		}
	}

	// Gỡ video cũng phát để danh sách tự cập nhật.
	if err := h.svc.RemoveVideo(ctx, "twitch:Abc"); err != nil {
		t.Fatal(err)
	}
	notes = h.notified()
	if notes[len(notes)-1] != "twitch:Abc" {
		t.Fatalf("gỡ video phải phát ref: %v", notes)
	}
}

// TestProgressWriterDedupesRepeatedHundred: yt-dlp lặp "100%" nhiều dòng vẫn
// chỉ một lần ghi, và mốc lặp cùng giá trị bị bỏ.
func TestProgressWriterDedupesRepeatedHundred(t *testing.T) {
	h := newHarness(t, func(h *harness) backdrop.RunYtDlpFunc {
		h.runner.progress = []string{"1", "1", "100", "100", "100", "100"}
		return h.runner.run
	})
	if _, err := h.svc.AddVideo(context.Background(), "https://www.twitch.tv/x/clip/Abc", nil); err != nil {
		t.Fatal(err)
	}
	if writes := h.store.progressWriteCount(); writes > 4 {
		t.Fatalf("ghi tiến độ = %d, vượt trần 4 lần/giây", writes)
	}
}

// TestRefreshReadsNewestPageKeepsBackfill: Làm mới đọc trang MỚI NHẤT nhưng
// không đụng con trỏ đọc dần lịch sử (trang 5 vẫn còn).
func TestRefreshReadsNewestPageKeepsBackfill(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	var pages []int
	h.probeOn = func(_ context.Context, _ string, pn int) (backdrop.BilibiliPage, error) {
		pages = append(pages, pn)
		return backdrop.BilibiliPage{Total: 100, PageSize: 30, Videos: biliVideos("new")}, nil
	}
	h.store.seedChannel(domain.BackdropChannel{
		ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123",
		AddedAt: h.now, ScanNextPage: new(5),
	})
	if _, err := h.svc.RefreshChannel(ctx, "bilibili:123", true); err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0] != 1 {
		t.Fatalf("trang đã đọc = %v, muốn trang 1 (mới nhất)", pages)
	}
	channel, _, _ := h.store.Channel(ctx, "bilibili:123")
	if channel.ScanNextPage == nil || *channel.ScanNextPage != 5 {
		t.Fatalf("con trỏ backfill = %v, muốn giữ 5", channel.ScanNextPage)
	}
}

// TestStaleDownloadIsReconciledOnStart: lượt tải mồ côi (lease hết) bị đánh
// dấu failed khi bộ lập lịch khởi động, để thêm lại được.
func TestStaleDownloadIsReconciledOnStart(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:old", Platform: domain.PlatformTwitch, URL: "u", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropDownloading})
	if _, claimed, err := h.store.ClaimDownload(ctx, "twitch:old", "process-đã-chết", h.now.Add(-time.Minute), h.now.Add(-time.Hour), false); err != nil || !claimed {
		t.Fatalf("giành lease giả: claimed=%v err=%v", claimed, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	h.svc.RunScheduler(cancelled)
	row, _, _ := h.store.Video(ctx, "twitch:old")
	if row.State != domain.BackdropFailed || row.ErrorCode != "interrupted" {
		t.Fatalf("video = %+v, muốn failed/interrupted", row)
	}
	if h.store.staleClears != 1 {
		t.Fatalf("số lần dọn = %d", h.store.staleClears)
	}
}

// TestAddVideoReclaimsStaleDownload: dòng "đang tải" có lease hết được thêm
// lại (giành quyền) và tải xong bình thường.
func TestAddVideoReclaimsStaleDownload(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "https://www.twitch.tv/x/clip/Abc", Title: "cũ", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropDownloading})
	if _, claimed, _ := h.store.ClaimDownload(ctx, "twitch:Abc", "dead", h.now.Add(-time.Minute), h.now.Add(-time.Hour), false); !claimed {
		t.Fatal("giành lease giả")
	}
	view, err := h.svc.AddVideo(ctx, "https://www.twitch.tv/x/clip/Abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if view.Video.State != domain.BackdropReady {
		t.Fatalf("state = %s, muốn ready", view.Video.State)
	}
}

// TestDownloadGuardsIgnoreStaleGeneration: lượt tải cũ không ghi đè lượt mới.
func TestDownloadGuardsIgnoreStaleGeneration(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "u", Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropFailed})
	gen1, claimed, _ := h.store.ClaimDownload(ctx, "twitch:Abc", "owner-a", h.now.Add(time.Minute), h.now, false)
	if !claimed {
		t.Fatal("giành lượt 1")
	}
	gen2, claimed, _ := h.store.ClaimDownload(ctx, "twitch:Abc", "owner-b", h.now.Add(time.Minute), h.now, true)
	if !claimed || gen2 <= gen1 {
		t.Fatalf("giành lượt 2: gen=%d claimed=%v", gen2, claimed)
	}
	if applied, _ := h.store.FinishDownload(ctx, "twitch:Abc", "owner-a", gen1, "x.mp4", 480, 10, h.now); applied {
		t.Fatal("lượt cũ không được chốt ready")
	}
	if applied, _ := h.store.TouchDownload(ctx, "twitch:Abc", "owner-a", gen1, new(0.5), h.now.Add(time.Minute), h.now); applied {
		t.Fatal("lượt cũ không được ghi tiến độ")
	}
	if applied, _ := h.store.FinishDownload(ctx, "twitch:Abc", "owner-b", gen2, "x.mp4", 480, 10, h.now); !applied {
		t.Fatal("lượt mới phải chốt được")
	}
}

// TestMixUsesRealCutsAndEligibleFresh: freshVideos theo đúng bộ lọc (pins,
// kinds, sàn clip), fixed chỉ tính video ready, loopSec theo độ dài cut thật.
func TestMixUsesRealCutsAndEligibleFresh(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:f1", Platform: domain.PlatformTwitch, Segment: domain.BackdropSegment{FromSec: 0, ToSec: 100}, State: domain.BackdropReady})
	h.store.seedFixed(domain.BackdropVideo{ID: "twitch:f2", Platform: domain.PlatformTwitch, Segment: domain.BackdropSegment{FromSec: 0, ToSec: 60}, State: domain.BackdropDownloading})
	h.store.seedChannel(domain.BackdropChannel{ID: "twitch:a", Platform: domain.PlatformTwitch, Handle: "a", URL: "u", AddedAt: h.now})
	h.store.seedVideos("twitch:a",
		domain.BackdropChannelVideo{VideoID: "a", Kind: domain.PlatformKindVod, URL: "u", Title: "a", DurationSec: new(600.0), ListedAt: h.now, Position: 0},
		domain.BackdropChannelVideo{VideoID: "b", Kind: domain.PlatformKindVod, URL: "u", Title: "b", DurationSec: new(600.0), ListedAt: h.now, Position: 1},
		domain.BackdropChannelVideo{VideoID: "c", Kind: domain.PlatformKindVod, URL: "u", Title: "c", DurationSec: new(600.0), ListedAt: h.now, Position: 2},
		domain.BackdropChannelVideo{VideoID: "short", Kind: domain.PlatformKindVod, URL: "u", Title: "s", DurationSec: new(5.0), ListedAt: h.now, Position: 3},
		domain.BackdropChannelVideo{VideoID: "clip", Kind: domain.PlatformKindClip, URL: "u", Title: "l", DurationSec: new(600.0), ListedAt: h.now, Position: 4},
	)
	// 2 pins, clipSec 60, speed 2 → need 1 clip; played 30 → thiếu 470.
	recipe := `{"backdrop":{"channels":[{"channelId":"twitch:a","kinds":["vod"],"pinned":["a","b"]}],"clipSec":60,"clipCount":1,"fixedVideoIds":["twitch:f1","twitch:f2"]},"appearance":{"speed":2}}`
	h.store.seedTemplate("t1", "M", recipe)
	report, err := backdrop.NewMixReader(h.store).Mix(ctx, "t1", 600)
	if err != nil {
		t.Fatal(err)
	}
	if report.FixedSec != 50 { // chỉ f1 ready (100/2), f2 đang tải bị bỏ
		t.Fatalf("fixedSec = %v, muốn 50", report.FixedSec)
	}
	if len(report.PerChannel) != 1 || report.PerChannel[0].FreshVideos != 2 {
		t.Fatalf("fresh = %+v, muốn 2 (chỉ pins a,b đúng loại, bỏ short/clip)", report.PerChannel)
	}
	if report.Need != 1 || report.PerChannel[0].Clips != 1 {
		t.Fatalf("need=%d clips=%d", report.Need, report.PerChannel[0].Clips)
	}
	// Loop theo cut thật: target 550 − 30 = 520.
	if report.LoopSec < 519 || report.LoopSec > 521 {
		t.Fatalf("loopSec = %v, muốn ~520", report.LoopSec)
	}
}
