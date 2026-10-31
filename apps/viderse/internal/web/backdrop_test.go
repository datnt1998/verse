package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// fakeBackdrop trả dữ liệu đặt tay và ghi lại tham số đã nhận.
type fakeBackdrop struct {
	parsed       domain.ParsedSource
	parseInput   string
	channels     []backdrop.ChannelView
	channelView  backdrop.ChannelView
	created      bool
	addedInput   string
	videoPage    backdrop.VideoPage
	videosOpts   backdrop.ChannelVideosOptions
	videoViews   []backdrop.VideoView
	videoView    backdrop.VideoView
	addedURL     string
	addedSegment *domain.BackdropSegment
	patchedSeg   domain.BackdropSegment
	removedID    string
	filePath     string
	record       backdrop.PlatformRequestRecord
	requests     []backdrop.PlatformRequestRecord
	err          error
}

func (f *fakeBackdrop) ParseSource(_ context.Context, input string) (domain.ParsedSource, error) {
	f.parseInput = input
	return f.parsed, f.err
}

func (f *fakeBackdrop) Channels(context.Context) ([]backdrop.ChannelView, error) {
	return f.channels, f.err
}

func (f *fakeBackdrop) Channel(context.Context, string) (backdrop.ChannelView, error) {
	return f.channelView, f.err
}

func (f *fakeBackdrop) AddChannel(_ context.Context, input string) (backdrop.ChannelView, bool, error) {
	f.addedInput = input
	return f.channelView, f.created, f.err
}

func (f *fakeBackdrop) RefreshChannel(context.Context, string, bool) (backdrop.ChannelView, error) {
	return f.channelView, f.err
}

func (f *fakeBackdrop) RemoveChannel(_ context.Context, id string) error {
	f.removedID = id
	return f.err
}

func (f *fakeBackdrop) ChannelVideos(_ context.Context, _ string, opts backdrop.ChannelVideosOptions) (backdrop.VideoPage, error) {
	f.videosOpts = opts
	return f.videoPage, f.err
}

func (f *fakeBackdrop) Videos(context.Context) ([]backdrop.VideoView, error) {
	return f.videoViews, f.err
}

func (f *fakeBackdrop) Video(context.Context, string) (backdrop.VideoView, error) {
	return f.videoView, f.err
}

func (f *fakeBackdrop) AddVideo(_ context.Context, url string, segment *domain.BackdropSegment) (backdrop.VideoView, error) {
	f.addedURL = url
	f.addedSegment = segment
	return f.videoView, f.err
}

func (f *fakeBackdrop) UpdateVideoSegment(_ context.Context, _ string, segment domain.BackdropSegment) (backdrop.VideoView, error) {
	f.patchedSeg = segment
	return f.videoView, f.err
}

func (f *fakeBackdrop) RemoveVideo(_ context.Context, id string) error {
	f.removedID = id
	return f.err
}

func (f *fakeBackdrop) VideoFile(context.Context, string) (string, error) {
	return f.filePath, f.err
}

func (f *fakeBackdrop) RecordPlatformRequest(_ context.Context, host string) (backdrop.PlatformRequestRecord, error) {
	f.addedURL = host
	return f.record, f.err
}

func (f *fakeBackdrop) PlatformRequests(context.Context) ([]backdrop.PlatformRequestRecord, error) {
	return f.requests, f.err
}

func decodeItems(t *testing.T, w *httptest.ResponseRecorder) []any {
	t.Helper()
	var out []any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body không phải JSON array: %q", w.Body.String())
	}
	return out
}

func backdropFixture(t *testing.T, svc *fakeBackdrop) (*testGateway, string) {
	t.Helper()
	tg := newTestGateway(t)
	MountBackdrop(tg.g, svc)
	return tg, tg.sessionCookie()
}

func TestParseSourceWire(t *testing.T) {
	title := "Đêm Sài Gòn"
	duration := 2892.0
	svc := &fakeBackdrop{parsed: domain.ParsedSource{
		Kind: domain.SourceChannel, Platform: domain.PlatformTwitch,
		ID: "twitch:nightdrivefm", Handle: "nightdrivefm", URL: "https://www.twitch.tv/nightdrivefm",
	}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/sources/parse", cookie, `{"input":"https://twitch.tv/nightdrivefm"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["kind"] != "channel" || body["platform"] != "twitch" || body["id"] != "twitch:nightdrivefm" || body["handle"] != "nightdrivefm" {
		t.Fatalf("body = %v", body)
	}
	if svc.parseInput != "https://twitch.tv/nightdrivefm" {
		t.Fatalf("input = %q", svc.parseInput)
	}

	svc.parsed = domain.ParsedSource{Kind: domain.SourceVideo, Platform: domain.PlatformBilibili, ID: "bilibili:BV1xx411c7mD", URL: "https://www.bilibili.com/video/BV1xx411c7mD", Title: &title, DurationSec: &duration}
	w = tg.do(http.MethodPost, "/api/sources/parse", cookie, `{"input":"https://b23.tv/abc"}`)
	body = decode(t, w)
	if body["kind"] != "video" || body["title"] != title || body["durationSec"] != duration {
		t.Fatalf("video body = %v", body)
	}

	svc.parsed = domain.ParsedSource{Kind: domain.SourceUnsupported, Host: "douyin.com", Label: "Douyin"}
	w = tg.do(http.MethodPost, "/api/sources/parse", cookie, `{"input":"https://www.douyin.com/video/1"}`)
	body = decode(t, w)
	if body["kind"] != "unsupported" || body["host"] != "douyin.com" || body["label"] != "Douyin" {
		t.Fatalf("unsupported body = %v", body)
	}

	svc.parsed = domain.ParsedSource{Kind: domain.SourceInvalid, Message: "Link không hợp lệ", Recovery: "Dán link video"}
	w = tg.do(http.MethodPost, "/api/sources/parse", cookie, `{"input":"x"}`)
	body = decode(t, w)
	if body["kind"] != "invalid" || body["recovery"] != "Dán link video" {
		t.Fatalf("invalid body = %v", body)
	}

	// Input rỗng vẫn trả 200 với kind invalid để UI hiện gợi ý phục hồi.
	svc.parsed = domain.ParsedSource{Kind: domain.SourceInvalid, Message: "Link không hợp lệ", Recovery: "Dán link video"}
	w = tg.do(http.MethodPost, "/api/sources/parse", cookie, `{"input":""}`)
	if w.Code != http.StatusOK || decode(t, w)["kind"] != "invalid" || svc.parseInput != "" {
		t.Fatalf("status %d body=%s input=%q", w.Code, w.Body.String(), svc.parseInput)
	}
}

func TestBackdropChannelListWire(t *testing.T) {
	added := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	listed := added.Add(time.Minute)
	title := "Night Drive FM"
	avatar := "channels/twitch-nightdrivefm/avatar.png"
	paused := added.Add(45 * time.Minute)
	reason := "quota"
	svc := &fakeBackdrop{channels: []backdrop.ChannelView{{
		Channel: domain.BackdropChannel{
			ID: "twitch:nightdrivefm", Platform: domain.PlatformTwitch, Handle: "nightdrivefm",
			URL: "https://www.twitch.tv/nightdrivefm", Title: &title, AvatarURL: &avatar,
			AddedAt: added, ListedAt: &listed, ListErrorCode: "rate_limited", ListError: "Bilibili đang giới hạn",
		},
		Counts:      map[domain.PlatformKind]int{domain.PlatformKindVod: 12, domain.PlatformKindClip: 3},
		PausedUntil: &paused, PauseReason: &reason,
		UsedBy: []backdrop.TemplateUsage{{TemplateID: "t-1", Name: "Lofi 2 giờ", Mode: "random", Kinds: []domain.PlatformKind{domain.PlatformKindVod}, ChosenCount: 2}},
	}}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodGet, "/api/backdrop/channels", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	list := decodeItems(t, w)
	if len(list) != 1 {
		t.Fatalf("body = %v", list)
	}
	channel := list[0].(map[string]any)
	if channel["id"] != "twitch:nightdrivefm" || channel["platform"] != "twitch" || channel["title"] != title || channel["avatarUrl"] != avatar {
		t.Fatalf("channel = %v", channel)
	}
	if channel["listedAt"] != "2026-10-29T08:01:00Z" || channel["pausedUntil"] != "2026-10-29T08:45:00Z" || channel["pauseReason"] != "quota" {
		t.Fatalf("channel times = %v", channel)
	}
	listError := channel["listError"].(map[string]any)
	if listError["code"] != "rate_limited" {
		t.Fatalf("listError = %v", listError)
	}
	counts := channel["counts"].(map[string]any)
	if counts["vod"] != float64(12) || counts["clip"] != float64(3) {
		t.Fatalf("counts = %v", counts)
	}
	usedBy := channel["usedBy"].([]any)[0].(map[string]any)
	if usedBy["templateId"] != "t-1" || usedBy["mode"] != "random" || usedBy["chosenCount"] != float64(2) {
		t.Fatalf("usedBy = %v", usedBy)
	}
	if channel["bilibili"] != nil {
		t.Fatalf("Twitch không có khối bilibili: %v", channel["bilibili"])
	}
}

func TestBackdropChannelAddAndRefresh(t *testing.T) {
	svc := &fakeBackdrop{created: true, channelView: backdrop.ChannelView{Channel: domain.BackdropChannel{ID: "bilibili:123", Platform: domain.PlatformBilibili, Handle: "123", URL: "https://space.bilibili.com/123", AddedAt: time.Now()}}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/backdrop/channels", cookie, `{"url":"https://space.bilibili.com/123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.addedInput != "https://space.bilibili.com/123" {
		t.Fatalf("input = %q", svc.addedInput)
	}
	if decode(t, w)["id"] != "bilibili:123" {
		t.Fatal("body sai")
	}

	svc.created = false
	w = tg.do(http.MethodPost, "/api/backdrop/channels", cookie, `{"input":"https://space.bilibili.com/123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("kênh đã có phải 200, được %d", w.Code)
	}

	w = tg.do(http.MethodPost, "/api/backdrop/channels/bilibili:123/refresh", cookie, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("refresh status %d: %s", w.Code, w.Body.String())
	}
	w = tg.do(http.MethodDelete, "/api/backdrop/channels/bilibili:123", cookie, "")
	if w.Code != http.StatusNoContent || svc.removedID != "bilibili:123" {
		t.Fatalf("delete status %d id=%q", w.Code, svc.removedID)
	}
}

func TestBackdropChannelVideosWire(t *testing.T) {
	published := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	unavailableAt := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	duration := 2892.0
	title := "Đêm Sài Gòn"
	thumb := "https://i.test/a.jpg"
	svc := &fakeBackdrop{videoPage: backdrop.VideoPage{
		Items: []backdrop.VideoItem{{
			Video: domain.BackdropChannelVideo{
				VideoID: "v1", Kind: domain.PlatformKindVod, URL: "https://www.twitch.tv/videos/1", Title: title,
				PublishedAt: &published, DurationSec: &duration, ThumbnailURL: &thumb,
			},
			ChosenBy: []backdrop.TemplateRef{{TemplateID: "t-1", Name: "Lofi 2 giờ"}},
		}, {
			Video: domain.BackdropChannelVideo{
				VideoID: "v0", Kind: domain.PlatformKindVod, URL: "u0", Title: "mất",
				UnavailableAt: &unavailableAt, UnavailableReason: domain.BackdropExpired,
			},
		}},
		Counts: map[domain.PlatformKind]int{domain.PlatformKindVod: 2}, Total: 2, Unavailable: 1,
		NextCursor: "eyJ2IjoiNTEifQ",
	}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodGet, "/api/backdrop/channels/twitch:nightdrivefm/videos?kind=vod,clip&sort=oldest&limit=48&cursor=abc", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.videosOpts.Sort != "oldest" || svc.videosOpts.Limit != 48 || svc.videosOpts.Cursor != "abc" {
		t.Fatalf("opts = %+v", svc.videosOpts)
	}
	if len(svc.videosOpts.Kinds) != 2 || svc.videosOpts.Kinds[1] != domain.PlatformKindClip {
		t.Fatalf("kinds = %v", svc.videosOpts.Kinds)
	}
	body := decode(t, w)
	if body["total"] != float64(2) || body["unavailable"] != float64(1) || body["nextCursor"] != "eyJ2IjoiNTEifQ" {
		t.Fatalf("page = %v", body)
	}
	items := body["items"].([]any)
	first := items[0].(map[string]any)
	if first["videoId"] != "v1" || first["title"] != title || first["publishedAt"] != "2026-10-01T00:00:00Z" || first["durationSec"] != duration {
		t.Fatalf("item = %v", first)
	}
	chosenBy := first["chosenBy"].([]any)[0].(map[string]any)
	if chosenBy["templateId"] != "t-1" || chosenBy["name"] != "Lofi 2 giờ" {
		t.Fatalf("chosenBy = %v", chosenBy)
	}
	second := items[1].(map[string]any)
	unavailable := second["unavailable"].(map[string]any)
	if unavailable["reason"] != "expired" || unavailable["at"] != "2026-10-20T00:00:00Z" {
		t.Fatalf("unavailable = %v", unavailable)
	}
}

func TestBackdropVideoAddPatchDeleteAndFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(file, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	segment := domain.BackdropSegment{FromSec: 10, ToSec: 70}
	svc := &fakeBackdrop{filePath: file, videoView: backdrop.VideoView{Video: domain.BackdropVideo{
		ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "https://www.twitch.tv/x/clip/Abc",
		Title: "Clip", Segment: segment, State: domain.BackdropReady,
		FilePath: "library/backdrops/twitch-Abc.mp4", FileHeight: 480, FileBytes: 10,
		AddedAt: time.Now(), UpdatedAt: time.Now(),
	}}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/backdrop/videos", cookie, `{"url":"https://www.twitch.tv/x/clip/Abc","segment":{"fromSec":10,"toSec":70}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.addedURL != "https://www.twitch.tv/x/clip/Abc" || svc.addedSegment == nil || svc.addedSegment.ToSec != 70 {
		t.Fatalf("add = %q %+v", svc.addedURL, svc.addedSegment)
	}
	body := decode(t, w)
	if body["state"] != "ready" || body["segment"].(map[string]any)["toSec"] != float64(70) {
		t.Fatalf("body = %v", body)
	}
	fileWire := body["file"].(map[string]any)
	if fileWire["height"] != float64(480) || fileWire["bytes"] != float64(10) || fileWire["path"] != "library/backdrops/twitch-Abc.mp4" {
		t.Fatalf("file = %v", fileWire)
	}

	w = tg.do(http.MethodPatch, "/api/backdrop/videos/twitch:Abc", cookie, `{"segment":{"fromSec":20,"toSec":80}}`)
	if w.Code != http.StatusOK || svc.patchedSeg.FromSec != 20 || svc.patchedSeg.ToSec != 80 {
		t.Fatalf("patch status %d seg=%+v", w.Code, svc.patchedSeg)
	}
	w = tg.do(http.MethodPatch, "/api/backdrop/videos/twitch:Abc", cookie, `{"segment":{}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("thiếu đoạn phải 400, được %d", w.Code)
	}

	w = tg.do(http.MethodGet, "/api/backdrop/videos/twitch:Abc/file", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("file status %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("content-type = %q", ct)
	}
	if w.Body.Len() != 10 {
		t.Fatalf("bytes = %d", w.Body.Len())
	}
	if !strings.Contains(w.Header().Get("Accept-Ranges"), "bytes") {
		t.Fatalf("Accept-Ranges = %q", w.Header().Get("Accept-Ranges"))
	}

	w = tg.do(http.MethodDelete, "/api/backdrop/videos/twitch:Abc", cookie, "")
	if w.Code != http.StatusNoContent || svc.removedID != "twitch:Abc" {
		t.Fatalf("delete status %d id=%q", w.Code, svc.removedID)
	}
}

func TestBackdropInUseConflictCarriesRecovery(t *testing.T) {
	svc := &fakeBackdrop{err: backdrop.ErrorfRecovery(backdrop.CodeInUse, "Kênh đang được 2 mẫu dùng", "Gỡ kênh khỏi mẫu trước")}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodDelete, "/api/backdrop/channels/twitch:a", cookie, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["code"] != "in_use" || !strings.Contains(body["error"].(string), "2 mẫu") {
		t.Fatalf("body = %v", body)
	}
	if body["recovery"] != "Gỡ kênh khỏi mẫu trước" {
		t.Fatalf("recovery = %v", body["recovery"])
	}
}

func TestPlatformRequestWire(t *testing.T) {
	first := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	last := first.Add(5 * time.Minute)
	svc := &fakeBackdrop{record: backdrop.PlatformRequestRecord{Host: "douyin.com", Label: "Douyin", Count: 3, FirstAt: first, LastAt: last}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/platforms/requests", cookie, `{"host":"douyin.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["host"] != "douyin.com" || body["label"] != "Douyin" || body["count"] != float64(3) || body["lastAt"] != "2026-10-29T08:05:00Z" {
		t.Fatalf("body = %v", body)
	}

	svc.err = backdrop.ErrorfRecovery(backdrop.CodeUnsupportedPlatform, "Nền tảng này đã hỗ trợ", "")
	w = tg.do(http.MethodPost, "/api/platforms/requests", cookie, `{"host":"twitch.tv"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestBackdropVideoListWire(t *testing.T) {
	svc := &fakeBackdrop{videoViews: []backdrop.VideoView{{Video: domain.BackdropVideo{
		ID: "twitch:Abc", Platform: domain.PlatformTwitch, URL: "u", Title: "Clip",
		Segment: domain.BackdropSegment{FromSec: 0, ToSec: 120}, State: domain.BackdropDownloading,
		Progress: new(0.25), AddedAt: time.Now(), UpdatedAt: time.Now(),
	}, UsedBy: []backdrop.TemplateRef{{TemplateID: "t-1", Name: "Mẫu"}}}}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodGet, "/api/backdrop/videos", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	list := decodeItems(t, w)
	video := list[0].(map[string]any)
	if video["id"] != "twitch:Abc" || video["state"] != "downloading" || video["progress"] != 0.25 {
		t.Fatalf("video = %v", video)
	}
	if _, ok := video["file"]; ok {
		t.Fatalf("chưa có tệp thì không có khối file: %v", video)
	}
	usedBy := video["usedBy"].([]any)[0].(map[string]any)
	if usedBy["name"] != "Mẫu" {
		t.Fatalf("usedBy = %v", usedBy)
	}
}

// codedError mang mã lỗi như lỗi tools/platforms, không phải *backdrop.Error.
type codedError struct{ code string }

func (e codedError) Error() string     { return "lỗi nền tảng: " + e.code }
func (e codedError) ErrorCode() string { return e.code }

// TestBackdropCodedErrorsMapToWire: mã lỗi nền tảng đi đúng HTTP, kèm retryAt
// khi lỗi có mốc thử lại.
func TestBackdropCodedErrorsMapToWire(t *testing.T) {
	until := time.Date(2026, 10, 29, 9, 15, 0, 0, time.UTC)
	svc := &fakeBackdrop{err: &platforms.PausedError{Platform: domain.PlatformBilibili, Label: "Bilibili", Until: until, Reason: "quota"}}
	tg, cookie := backdropFixture(t, svc)
	w := tg.do(http.MethodPost, "/api/backdrop/channels/bilibili:1/refresh", cookie, "{}")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["code"] != "rate_limited" || body["retryAt"] != "2026-10-29T09:15:00Z" {
		t.Fatalf("body = %v", body)
	}

	svc.err = codedError{code: backdrop.CodeLoginRequired}
	w = tg.do(http.MethodPost, "/api/backdrop/videos", cookie, `{"url":"https://www.bilibili.com/video/BV1xx411c7mD"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("login_required phải 401, được %d: %s", w.Code, w.Body.String())
	}
	if decode(t, w)["code"] != backdrop.CodeLoginRequired {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// TestBackdropAddVideoPartialSegmentRejected: object segment thiếu một đầu là
// 400, không lặng lẽ dùng mặc định.
func TestBackdropAddVideoPartialSegmentRejected(t *testing.T) {
	svc := &fakeBackdrop{}
	tg, cookie := backdropFixture(t, svc)
	for _, body := range []string{
		`{"url":"https://www.twitch.tv/x/clip/Abc","segment":{"fromSec":10}}`,
		`{"url":"https://www.twitch.tv/x/clip/Abc","segment":{"toSec":70}}`,
		`{"url":"https://www.twitch.tv/x/clip/Abc","segment":{}}`,
	} {
		w := tg.do(http.MethodPost, "/api/backdrop/videos", cookie, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status %d: %s", body, w.Code, w.Body.String())
		}
		if decode(t, w)["code"] != backdrop.CodeInvalidInput {
			t.Fatalf("body %s: %s", body, w.Body.String())
		}
	}
	if svc.addedURL != "" {
		t.Fatalf("không được gọi service, đã gọi %q", svc.addedURL)
	}
}
