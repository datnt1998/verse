// Ca test chuyển từ yt-factory src/server/youtube/channel-pages.test.ts:
// transport fixture ghi lại thứ tự request, phân loại bằng URL/body y như bản TS
// (sw.js_data → bootstrap; resolve_url → resolve; body có "continuation" →
// continuation; body có "params" → tab; còn lại → home).
package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testChannel = "UCHnyfMqiRRG1u-2MsSQLbXA"

func fixtureText(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("đọc fixture %s: %v", name, err)
	}
	return string(raw)
}

func fixtureJSON(t *testing.T, name string) jmap {
	t.Helper()
	var m jmap
	if err := json.Unmarshal([]byte(fixtureText(t, name)), &m); err != nil {
		t.Fatalf("fixture %s không phải JSON: %v", name, err)
	}
	return m
}

// dig đi theo các bước string (khoá object) / int (chỉ mục mảng); fixture thiếu
// hình dạng mong đợi thì fail test ngay.
func dig(t *testing.T, root any, steps ...any) any {
	t.Helper()
	at := root
	for _, step := range steps {
		switch s := step.(type) {
		case int:
			a, ok := at.([]any)
			if !ok || s >= len(a) {
				t.Fatalf("fixture không có bước %v", step)
			}
			at = a[s]
		case string:
			m, ok := at.(map[string]any)
			if !ok {
				t.Fatalf("fixture không có bước %v", step)
			}
			at = m[s]
		}
		if at == nil {
			t.Fatalf("fixture không có bước %v", step)
		}
	}
	return at
}

func jobs(t *testing.T, root any, steps ...any) jmap {
	t.Helper()
	return jobj(dig(t, root, steps...))
}

func jarrs(t *testing.T, root any, steps ...any) []any {
	t.Helper()
	return jarr(dig(t, root, steps...))
}

func videosTab(t *testing.T, page jmap) jmap {
	t.Helper()
	for _, tab := range jarrs(t, page, "contents", "twoColumnBrowseResultsRenderer", "tabs") {
		tr := jobj(jobj(tab)["tabRenderer"])
		if sel, _ := tr["selected"].(bool); sel {
			return tr
		}
	}
	t.Fatal("fixture không có tab được chọn")
	return nil
}

func grid(t *testing.T, page jmap) jmap {
	t.Helper()
	return jobs(t, videosTab(t, page), "content", "richGridRenderer")
}

func tabItems(t *testing.T, page jmap) []any {
	return jarrs(t, grid(t, page), "contents")
}

func lockup(t *testing.T, item any) jmap {
	t.Helper()
	return jobs(t, item, "richItemRenderer", "content", "lockupViewModel")
}

func lockupMeta(t *testing.T, item any) jmap {
	t.Helper()
	return jobs(t, lockup(t, item), "metadata", "lockupMetadataViewModel")
}

func selectChip(t *testing.T, page jmap, label string) {
	t.Helper()
	for _, chip := range jarrs(t, grid(t, page), "header", "chipBarViewModel", "chips") {
		view := jobs(t, chip, "chipViewModel")
		view["selected"] = jstr(view["text"]) == label
	}
}

// -------------------------------- transport ---------------------------------

type reqKind string

const (
	kindSW           reqKind = "sw"
	kindHome         reqKind = "home"
	kindTab          reqKind = "tab"
	kindContinuation reqKind = "continuation"
	kindResolve      reqKind = "resolve"
	kindPlayer       reqKind = "player"
)

type tracedReq struct {
	kind reqKind
	path string
	body jmap
	ctx  context.Context
}

type handlerFunc func(req tracedReq) (*http.Response, error)

type fixtureTransport struct {
	handlers map[reqKind]handlerFunc
	trace    []tracedReq
}

func okResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func textResponse(status int, body string, headers map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func (ft *fixtureTransport) defaults(t *testing.T) map[reqKind]handlerFunc {
	return map[reqKind]handlerFunc{
		kindSW:   func(tracedReq) (*http.Response, error) { return okResponse(fixtureText(t, "sw-js-data.txt")), nil },
		kindHome: func(tracedReq) (*http.Response, error) { return okResponse(fixtureText(t, "channel-home.json")), nil },
		kindTab:  func(tracedReq) (*http.Response, error) { return okResponse(fixtureText(t, "videos-tab.json")), nil },
		kindContinuation: func(req tracedReq) (*http.Response, error) {
			switch jstr(req.body["continuation"]) {
			case "FIXTURE_PAGE_2":
				return okResponse(fixtureText(t, "videos-continuation.json")), nil
			case "FIXTURE_CHIP_LATEST":
				return okResponse(fixtureText(t, "videos-latest-filter.json")), nil
			}
			return textResponse(http.StatusBadRequest, "{}", nil), nil
		},
		kindResolve: func(tracedReq) (*http.Response, error) {
			return okResponse(`{"endpoint":{"browseEndpoint":{"browseId":"` + testChannel + `"}}}`), nil
		},
		kindPlayer: func(tracedReq) (*http.Response, error) { return okResponse(fixtureText(t, "player.json")), nil },
	}
}

// install ghi trace mới và đặt handler (mặc định + overrides); trả ft để đọc trace.
func (ft *fixtureTransport) install(t *testing.T, overrides map[reqKind]handlerFunc) *fixtureTransport {
	t.Helper()
	ft.handlers = ft.defaults(t)
	for k, h := range overrides {
		ft.handlers[k] = h
	}
	ft.trace = nil
	return ft
}

func (ft *fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body jmap
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
	}
	var kind reqKind
	switch {
	case strings.Contains(req.URL.Path, "sw.js_data"):
		kind = kindSW
	case strings.Contains(req.URL.Path, "resolve_url"):
		kind = kindResolve
	case strings.HasSuffix(req.URL.Path, "/youtubei/v1/player"):
		kind = kindPlayer
	case body != nil && body["continuation"] != nil:
		kind = kindContinuation
	case body != nil && body["params"] != nil:
		kind = kindTab
	default:
		kind = kindHome
	}
	tr := tracedReq{kind: kind, path: req.URL.Path, body: body, ctx: req.Context()}
	ft.trace = append(ft.trace, tr)
	h := ft.handlers[kind]
	if h == nil {
		return nil, errors.New("test attempted an unhandled request: " + string(kind))
	}
	return h(tr)
}

func (ft *fixtureTransport) kinds() []reqKind {
	var out []reqKind
	for _, r := range ft.trace {
		out = append(out, r.kind)
	}
	return out
}

// newFixtureClient dựng client trên transport fixture, bộ điều tốc tắt nhịp
// (tương đương governor null của bản TS).
func newFixtureClient(ft *fixtureTransport) *Client {
	return NewClient(Options{
		HTTPClient: &http.Client{Transport: ft},
		Pacer:      &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond},
	})
}

func kindStrings(kinds []reqKind) []string {
	var out []string
	for _, k := range kinds {
		out = append(out, string(k))
	}
	return out
}

func assertKinds(t *testing.T, ft *fixtureTransport, want ...reqKind) {
	t.Helper()
	got := kindStrings(ft.kinds())
	wantStrs := make([]string, len(want))
	for i, k := range want {
		wantStrs[i] = string(k)
	}
	if len(got) != len(wantStrs) {
		t.Fatalf("thứ tự request = %v, muốn %v", got, wantStrs)
	}
	for i := range got {
		if got[i] != wantStrs[i] {
			t.Fatalf("thứ tự request = %v, muốn %v", got, wantStrs)
		}
	}
}

func fetchError(t *testing.T, err error) *FetchError {
	t.Helper()
	if err == nil {
		t.Fatal("mong đợi lỗi, nhận nil")
	}
	var fe *FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("mong đợi *FetchError, nhận %T: %v", err, err)
	}
	return fe
}

func videoIDs(videos []ScannedVideo) []string {
	var out []string
	for _, v := range videos {
		out = append(out, v.ID)
	}
	return out
}

// ------------------------------- trang đầu ---------------------------------

func TestFirstPageLatestOrderRequests(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	// Không request lọc (Latest đã chọn), không request config, không request per-video.
	assertKinds(t, ft, kindSW, kindHome, kindTab)

	wantIDs := []string{"NiCgcUTL9_Q", "ONAuUyml0yI", "JsBZOcqZerk", "J1WoNuemKOg"}
	got := videoIDs(page.Videos)
	for i, id := range wantIDs {
		if i >= len(got) || got[i] != id {
			t.Fatalf("video ids = %v, muốn %v", got, wantIDs)
		}
	}
	newest := page.Videos[0]
	if newest.Title != "The Technology that Made the Internet Possible" {
		t.Fatalf("title = %q", newest.Title)
	}
	if newest.ThumbnailURL != "https://i.ytimg.com/vi/NiCgcUTL9_Q/mqdefault.jpg" {
		t.Fatalf("thumbnail = %q", newest.ThumbnailURL)
	}
	if newest.DurationSec == nil || *newest.DurationSec != 34*60+32 {
		t.Fatalf("durationSec = %v", newest.DurationSec)
	}
	if newest.ViewCount == nil || *newest.ViewCount != 2_000_000 {
		t.Fatalf("viewCount = %v", newest.ViewCount)
	}
	if newest.PublishedText == nil || *newest.PublishedText != "14h ago" {
		t.Fatalf("publishedText = %v", newest.PublishedText)
	}
	if newest.PublishedAt == nil {
		t.Fatal("publishedAt nil")
	}
	second := page.Videos[1]
	if second.DurationSec == nil || *second.DurationSec != 308 {
		t.Fatalf("second durationSec = %v", second.DurationSec)
	}
	if second.PublishedText == nil || *second.PublishedText != "12d ago" {
		t.Fatalf("second publishedText = %v", second.PublishedText)
	}

	if page.Profile == nil {
		t.Fatal("profile nil")
	}
	if page.Profile.ID != testChannel {
		t.Fatalf("profile.id = %q", page.Profile.ID)
	}
	if page.Profile.Handle == nil || *page.Profile.Handle != "@veritasium" {
		t.Fatalf("profile.handle = %v", page.Profile.Handle)
	}
	if page.Profile.Title != "Veritasium" {
		t.Fatalf("profile.title = %q", page.Profile.Title)
	}
	if page.Profile.SubscriberCount == nil || *page.Profile.SubscriberCount != 21_400_000 {
		t.Fatalf("profile.subscriberCount = %v", page.Profile.SubscriberCount)
	}
	if page.Profile.VideoCount == nil || *page.Profile.VideoCount != 540 {
		t.Fatalf("profile.videoCount = %v", page.Profile.VideoCount)
	}

	if page.NextCursor == nil {
		t.Fatal("nextCursor nil")
	}
	cur := page.NextCursor
	if cur.ChannelID != testChannel || cur.Tab != "videos" || cur.Order != "latest" {
		t.Fatalf("cursor = %+v", cur)
	}
	if cur.Continuation.Token != "FIXTURE_PAGE_2" || cur.Continuation.Request != "CONTINUATION_REQUEST_TYPE_BROWSE" {
		t.Fatalf("continuation = %+v", cur.Continuation)
	}
	if cur.Session.ClientName != "WEB" || cur.Session.ClientVersion != "2.20261012.01.00" {
		t.Fatalf("session = %+v", cur.Session)
	}
}

func TestFirstPageCursorSurvivesPersistence(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	stored, err := SerializeChannelCursor(*page.NextCursor)
	if err != nil {
		t.Fatalf("SerializeChannelCursor: %v", err)
	}
	if len(stored) > MaxCursorBytes {
		t.Fatalf("cursor %d bytes > %d", len(stored), MaxCursorBytes)
	}
	// Chỉ dữ liệu thuần: JSON hoá lại phải y nguyên.
	var a, b any
	if err := json.Unmarshal([]byte(stored), &a); err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(page.NextCursor)
	if err := json.Unmarshal(raw2, &b); err != nil {
		t.Fatal(err)
	}
	if !deepEqualJSON(a, b) {
		t.Fatal("cursor không round-trip dạng JSON thuần")
	}
	restored, err := ParseChannelCursor(stored)
	if err != nil {
		t.Fatalf("ParseChannelCursor: %v", err)
	}
	if restored != *page.NextCursor {
		t.Fatalf("cursor khôi phục = %+v", restored)
	}
}

func TestHandleResolvesOnceUCIDDoesNot(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	if _, err := c.FirstChannelPage(context.Background(), "@veritasium"); err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	assertKinds(t, ft, kindSW, kindResolve, kindHome, kindTab)
	if got := jstr(ft.trace[1].body["url"]); got != "https://www.youtube.com/@veritasium" {
		t.Fatalf("resolve url = %q", got)
	}

	// UC id không tốn request phân giải, kể cả bootstrap.
	ft2 := (&fixtureTransport{}).install(t, nil)
	id, err := newFixtureClient(ft2).ResolveChannelRef(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("ResolveChannelRef: %v", err)
	}
	if id != testChannel {
		t.Fatalf("id = %q", id)
	}
	assertKinds(t, ft2)
}

func TestResolveChannelRefUnknownHandleNotFound(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindResolve: func(tracedReq) (*http.Response, error) {
			return textResponse(http.StatusNotFound, "{}", nil), nil
		},
	})
	_, err := newFixtureClient(ft).ResolveChannelRef(context.Background(), "@does-not-exist")
	fe := fetchError(t, err)
	if fe.Kind != KindNotFound {
		t.Fatalf("kind = %q", fe.Kind)
	}
}

func TestChannelWithoutVideosTabYieldsEmptyFirstPage(t *testing.T) {
	home := fixtureJSON(t, "channel-home.json")
	tabs := jarrs(t, home, "contents", "twoColumnBrowseResultsRenderer", "tabs")
	kept := tabs[:0]
	for _, tab := range tabs {
		tr := jobj(jobj(tab)["tabRenderer"])
		if tr == nil || jstr(tr["title"]) != "Videos" {
			kept = append(kept, tab)
		}
	}
	home["contents"] = jmap{"twoColumnBrowseResultsRenderer": jmap{"tabs": kept}}
	raw, _ := json.Marshal(home)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	assertKinds(t, ft, kindSW, kindHome)
	if len(page.Videos) != 0 || page.NextCursor != nil {
		t.Fatalf("page = %+v", page)
	}
	if page.Profile == nil || page.Profile.ID != testChannel {
		t.Fatalf("profile = %+v", page.Profile)
	}
}

func TestVideosTabWithoutContinuationIsEndOfTab(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	grid(t, tab)["contents"] = tabItems(t, tab)[:4]
	raw, _ := json.Marshal(tab)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	if len(page.Videos) != 4 || page.NextCursor != nil {
		t.Fatalf("videos=%d nextCursor=%v", len(page.Videos), page.NextCursor)
	}
}

// ------------------------------ bộ lọc Latest -------------------------------

func TestLatestFilterRequestedOnceWhenOtherSortSelected(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	selectChip(t, tab, "Popular")
	// Thân theo thứ tự Popular không được rò vào kết quả.
	cont := fixtureJSON(t, "videos-continuation.json")
	grid(t, tab)["contents"] = jarrs(t, cont, "onResponseReceivedActions", 0, "appendContinuationItemsAction", "continuationItems")
	raw, _ := json.Marshal(tab)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	assertKinds(t, ft, kindSW, kindHome, kindTab, kindContinuation)
	if got := jstr(ft.trace[3].body["continuation"]); got != "FIXTURE_CHIP_LATEST" {
		t.Fatalf("filter token = %q", got)
	}
	wantIDs := []string{"NiCgcUTL9_Q", "ONAuUyml0yI", "JsBZOcqZerk", "J1WoNuemKOg"}
	for i, id := range videoIDs(page.Videos) {
		if id != wantIDs[i] {
			t.Fatalf("video ids = %v", videoIDs(page.Videos))
		}
	}
	if page.NextCursor == nil || page.NextCursor.Continuation.Token != "FIXTURE_PAGE_2" {
		t.Fatalf("nextCursor = %+v", page.NextCursor)
	}
}

func TestUnconfirmedLatestOrderFails(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	selectChip(t, tab, "Popular")
	stale := fixtureJSON(t, "videos-latest-filter.json")
	header := jobs(t, stale, "onResponseReceivedActions", 0, "reloadContinuationItemsCommand")
	for _, chip := range jarrs(t, header, "continuationItems", 0, "chipBarViewModel", "chips") {
		view := jobs(t, chip, "chipViewModel")
		view["selected"] = jstr(view["text"]) == "Popular"
	}
	tabRaw, _ := json.Marshal(tab)
	staleRaw, _ := json.Marshal(stale)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab:          func(tracedReq) (*http.Response, error) { return okResponse(string(tabRaw)), nil },
		kindContinuation: func(tracedReq) (*http.Response, error) { return okResponse(string(staleRaw)), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindUnexpectedPage {
		t.Fatalf("kind = %q", fe.Kind)
	}
	if !strings.Contains(fe.Message, "Latest") {
		t.Fatalf("message = %q", fe.Message)
	}
}

func TestFilterBarWithoutLatestChipFails(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	for _, chip := range jarrs(t, grid(t, tab), "header", "chipBarViewModel", "chips") {
		view := jobs(t, chip, "chipViewModel")
		if jstr(view["text"]) == "Latest" {
			view["text"] = "Recently uploaded"
		}
	}
	raw, _ := json.Marshal(tab)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindUnexpectedPage {
		t.Fatalf("kind = %q", fe.Kind)
	}
	assertKinds(t, ft, kindSW, kindHome, kindTab)
}

// ---------------------- renderer nullable và trùng lặp -----------------------

func TestNullableRenderersAndDuplicatesDropped(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	rows := tabItems(t, tab)

	// Hàng 1: không có metadata view/published.
	delete(jobs(t, lockupMeta(t, rows[1])), "metadata")
	// Hàng 2: không có overlay badge thời lượng.
	delete(jobs(t, lockup(t, rows[2]), "contentImage", "thumbnailViewModel"), "overlays")
	// Hàng 3: title rỗng.
	lockupMeta(t, rows[3])["title"] = jmap{"content": ""}
	// Lặp hàng 0 và một hàng id không phải video id.
	repeated := deepCopy(rows[0])
	broken := deepCopy(rows[0])
	lockup(t, broken)["contentId"] = "not-a-video-id"

	grid(t, tab)["contents"] = []any{rows[0], rows[1], repeated, rows[2], broken, rows[3], rows[4]}
	raw, _ := json.Marshal(tab)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	page, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	wantIDs := []string{"NiCgcUTL9_Q", "ONAuUyml0yI", "JsBZOcqZerk", "J1WoNuemKOg"}
	got := videoIDs(page.Videos)
	if len(got) != len(wantIDs) {
		t.Fatalf("video ids = %v", got)
	}
	for i := range wantIDs {
		if got[i] != wantIDs[i] {
			t.Fatalf("video ids = %v", got)
		}
	}
	noMeta := page.Videos[1]
	if noMeta.ViewCount != nil || noMeta.PublishedText != nil || noMeta.PublishedAt != nil {
		t.Fatalf("noMeta = %+v", noMeta)
	}
	if noMeta.DurationSec == nil || *noMeta.DurationSec != 308 {
		t.Fatalf("noMeta.durationSec = %v", noMeta.DurationSec)
	}
	noDuration := page.Videos[2]
	if noDuration.DurationSec != nil {
		t.Fatalf("noDuration.durationSec = %v", noDuration.DurationSec)
	}
	if noDuration.ViewCount == nil || *noDuration.ViewCount != 18_000_000 {
		t.Fatalf("noDuration.viewCount = %v", noDuration.ViewCount)
	}
	noTitle := page.Videos[3]
	if noTitle.Title != "" {
		t.Fatalf("noTitle.title = %q", noTitle.Title)
	}
	if noTitle.DurationSec == nil || *noTitle.DurationSec != 1365 {
		t.Fatalf("noTitle.durationSec = %v", noTitle.DurationSec)
	}
	if noTitle.ViewCount == nil || *noTitle.ViewCount != 2_700_000 {
		t.Fatalf("noTitle.viewCount = %v", noTitle.ViewCount)
	}
	if page.NextCursor == nil || page.NextCursor.Continuation.Token != "FIXTURE_PAGE_2" {
		t.Fatalf("nextCursor = %+v", page.NextCursor)
	}
}

// deepEqualJSON so hai giá trị JSON đã decode (số là float64) bằng giá trị sâu.
func deepEqualJSON(a, b any) bool {
	switch ta := a.(type) {
	case map[string]any:
		tb, ok := b.(map[string]any)
		if !ok || len(ta) != len(tb) {
			return false
		}
		for k, v := range ta {
			if !deepEqualJSON(v, tb[k]) {
				return false
			}
		}
		return true
	case []any:
		tb, ok := b.([]any)
		if !ok || len(ta) != len(tb) {
			return false
		}
		for i := range ta {
			if !deepEqualJSON(ta[i], tb[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func deepCopy(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func TestUnvalidatableContinuationIsErrorNotEndOfTab(t *testing.T) {
	tab := fixtureJSON(t, "videos-tab.json")
	items := tabItems(t, tab)
	last := items[len(items)-1]
	cmd := jobs(t, last, "continuationItemRenderer", "continuationEndpoint", "continuationCommand")
	cmd["token"] = "has spaces and \n newline"
	raw, _ := json.Marshal(tab)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindUnexpectedPage {
		t.Fatalf("kind = %q", fe.Kind)
	}
	if strings.Contains(fe.Message, "newline") {
		t.Fatalf("message lộ token: %q", fe.Message)
	}
}

// ------------------------------ trang kế ------------------------------------

func TestNextPageReplaysTokenOnIssuingSession(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	second, err := c.NextChannelPage(context.Background(), *first.NextCursor)
	if err != nil {
		t.Fatalf("NextChannelPage: %v", err)
	}
	// Cùng process: session trang đầu được tái dùng (một bootstrap sw.js_data).
	assertKinds(t, ft, kindSW, kindHome, kindTab, kindContinuation)
	if got := jstr(ft.trace[3].body["continuation"]); got != "FIXTURE_PAGE_2" {
		t.Fatalf("continuation = %q", got)
	}
	wantIDs := []string{"XX7PdJIGiCw", "CYlon2tvywA", "onr80iOoEXs"}
	for i, id := range videoIDs(second.Videos) {
		if id != wantIDs[i] {
			t.Fatalf("video ids = %v", videoIDs(second.Videos))
		}
	}
	if second.Profile != nil {
		t.Fatalf("profile = %+v", second.Profile)
	}
	if second.NextCursor == nil ||
		second.NextCursor.Continuation.Token != "FIXTURE_PAGE_3" ||
		second.NextCursor.Continuation.Request != "CONTINUATION_REQUEST_TYPE_BROWSE" {
		t.Fatalf("nextCursor = %+v", second.NextCursor)
	}
	if second.NextCursor.Session != first.NextCursor.Session {
		t.Fatalf("session = %+v", second.NextCursor.Session)
	}
	if second.Videos[0].PublishedText == nil || *second.Videos[0].PublishedText != "11mo ago" {
		t.Fatalf("publishedText = %v", second.Videos[0].PublishedText)
	}
}

func TestResumedCursorCarriesOwnVisitorData(t *testing.T) {
	visitorData := EncodeVisitorData("OtherVisit1", 1_760_000_100)
	cursor := ChannelCursor{
		Version:      1,
		ChannelID:    testChannel,
		Tab:          "videos",
		Order:        "latest",
		Continuation: Continuation{Token: "FIXTURE_PAGE_2", Request: "CONTINUATION_REQUEST_TYPE_BROWSE"},
		Session:      SessionContext{ClientName: "WEB", ClientVersion: "2.20261012.01.00", VisitorData: visitorData},
	}
	ft := (&fixtureTransport{}).install(t, nil)
	// Process mới → client mới, session bootstrap lại cho visitor data của cursor.
	page, err := newFixtureClient(ft).NextChannelPage(context.Background(), cursor)
	if err != nil {
		t.Fatalf("NextChannelPage: %v", err)
	}
	assertKinds(t, ft, kindSW, kindContinuation)
	got := jstr(jget(ft.trace[1].body, "context", "client", "visitorData"))
	if got != visitorData {
		t.Fatalf("visitorData = %q, muốn %q", got, visitorData)
	}
	if len(page.Videos) != 3 {
		t.Fatalf("videos = %d", len(page.Videos))
	}
	if page.NextCursor == nil || page.NextCursor.Session.VisitorData != visitorData {
		t.Fatalf("nextCursor session = %+v", page.NextCursor)
	}
}

func TestStaleClientVersionCursorRejectedBeforeRequest(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	stale := *first.NextCursor
	stale.Session.ClientVersion = "1.19990101.00.00"
	ft.trace = nil

	_, err = c.NextChannelPage(context.Background(), stale)
	fe := fetchError(t, err)
	if fe.Kind != KindCursorInvalid {
		t.Fatalf("kind = %q", fe.Kind)
	}
	assertKinds(t, ft)
	if strings.Contains(fe.Message, first.NextCursor.Session.VisitorData) && first.NextCursor.Session.VisitorData != "" {
		t.Fatalf("message lộ visitor data: %q", fe.Message)
	}
}

func TestStructurallyInvalidCursorRejectedWithoutRequest(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	bad := ChannelCursor{Version: 2, ChannelID: "nope"}
	_, err := newFixtureClient(ft).NextChannelPage(context.Background(), bad)
	fe := fetchError(t, err)
	if fe.Kind != KindCursorInvalid {
		t.Fatalf("kind = %q", fe.Kind)
	}
	assertKinds(t, ft)
}

func TestExpiredContinuationIsCursorInvalid(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) {
			return textResponse(http.StatusBadRequest, `{"error":{"code":400,"message":"Request contains an invalid argument."}}`, nil), nil
		},
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	rejected := fetchError(t, err)
	if rejected.Kind != KindCursorInvalid || rejected.Status != 400 {
		t.Fatalf("rejected = %+v", rejected)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) { return okResponse("{}"), nil },
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	empty := fetchError(t, err)
	if empty.Kind != KindCursorInvalid {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestLastContinuationPageEndsWalk(t *testing.T) {
	last := fixtureJSON(t, "videos-continuation.json")
	list := jarrs(t, last, "onResponseReceivedActions", 0, "appendContinuationItemsAction", "continuationItems")
	jobs(t, last, "onResponseReceivedActions", 0, "appendContinuationItemsAction")["continuationItems"] = list[:3]
	raw, _ := json.Marshal(last)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	cursor, err := ParseChannelCursor(`{"version":1,"channelId":"` + testChannel + `","tab":"videos","order":"latest","continuation":{"token":"FIXTURE_PAGE_2","request":"CONTINUATION_REQUEST_TYPE_BROWSE"},"session":{"clientName":"WEB","clientVersion":"2.20261012.01.00","visitorData":null}}`)
	if err != nil {
		t.Fatalf("ParseChannelCursor: %v", err)
	}
	page, err := newFixtureClient(ft).NextChannelPage(context.Background(), cursor)
	if err != nil {
		t.Fatalf("NextChannelPage: %v", err)
	}
	if len(page.Videos) != 3 || page.NextCursor != nil {
		t.Fatalf("videos=%d nextCursor=%v", len(page.Videos), page.NextCursor)
	}
}

// ---------------------------- phân loại lỗi ---------------------------------

func TestHTTP429CarriesRetryAfterMillis(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) {
			return textResponse(429, "slow down", map[string]string{"Retry-After": "120"}), nil
		},
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindRateLimited || fe.Status != 429 {
		t.Fatalf("fe = %+v", fe)
	}
	if fe.RetryDelay == nil || *fe.RetryDelay != 120*time.Second {
		t.Fatalf("retryDelay = %v", fe.RetryDelay)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return textResponse(429, "slow down", nil), nil },
	})
	_, err = newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	bare := fetchError(t, err)
	if bare.Kind != KindRateLimited || bare.RetryDelay != nil {
		t.Fatalf("bare = %+v", bare)
	}
}

func TestRateLimitDuringBootstrapAlsoClassified(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindSW: func(tracedReq) (*http.Response, error) {
			return textResponse(429, "", map[string]string{"Retry-After": "7"}), nil
		},
	})
	c := newFixtureClient(ft)
	_, err := c.FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindRateLimited || fe.RetryDelay == nil || *fe.RetryDelay != 7*time.Second {
		t.Fatalf("fe = %+v", fe)
	}
	// Bootstrap lỗi không được cache: lần gọi sau bootstrap lại. Phạt 429 thuộc
	// pacer của client cũ nên dùng client/pacer mới (tương đương bản TS tắt governor).
	ft.install(t, nil)
	c2 := newFixtureClient(ft)
	page, err := c2.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage sau bootstrap lỗi: %v", err)
	}
	if len(page.Videos) != 4 {
		t.Fatalf("videos = %d", len(page.Videos))
	}
}

func Test5xxNetworkAndTruncatedBodiesAreTransient(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return textResponse(503, "", nil), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindTransient || fe.Status != 503 {
		t.Fatalf("fe = %+v", fe)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) {
			return nil, &urlError{err: errors.New("connection reset")}
		},
	})
	_, err = newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if kind, _ := KindOf(fetchError(t, err)); kind != KindTransient {
		t.Fatalf("kind = %q", kind)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return okResponse(`{"contents":{"twoColumn`), nil },
	})
	_, err = newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if kind, _ := KindOf(fetchError(t, err)); kind != KindTransient {
		t.Fatalf("kind = %q", kind)
	}
}

type urlError struct{ err error }

func (e *urlError) Error() string   { return e.err.Error() }
func (e *urlError) Timeout() bool   { return false }
func (e *urlError) Temporary() bool { return true }
func (e *urlError) Unwrap() error   { return e.err }

// http.Client.Do bọc lỗi transport trong *url.Error — nhưng RoundTripper trả
// trực tiếp vào Do được wrap; khai báo type để isTransportError nhận diện qua
// chuỗi unwrap. urlError trên đây giữ tương đương errno-cause của bản TS.

func TestLoginAndBotChallengesAreBlocked(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) {
			return textResponse(429, "<html>Our systems have detected unusual traffic. Sign in to confirm you're not a bot</html>", nil), nil
		},
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindBlocked || fe.RetryDelay != nil {
		t.Fatalf("fe = %+v", fe)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return textResponse(403, "", nil), nil },
	})
	_, err = newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if kind, _ := KindOf(fetchError(t, err)); kind != KindBlocked {
		t.Fatalf("kind = %q", kind)
	}
}

func TestUnavailableChannelIsNotFound(t *testing.T) {
	gone := fixtureJSON(t, "channel-home.json")
	// Header giữ trang parse được; alert là dấu hiệu kênh không khả dụng.
	delete(gone, "metadata")
	delete(gone, "contents")
	gone["alerts"] = []any{jmap{"alertRenderer": jmap{"type": "ERROR", "text": jmap{"simpleText": "This channel does not exist."}}}}
	raw, _ := json.Marshal(gone)
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return okResponse(string(raw)), nil },
	})
	_, err := newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindNotFound {
		t.Fatalf("kind = %q", fe.Kind)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindHome: func(tracedReq) (*http.Response, error) { return textResponse(404, "{}", nil), nil },
	})
	_, err = newFixtureClient(ft).FirstChannelPage(context.Background(), testChannel)
	if kind, _ := KindOf(fetchError(t, err)); kind != KindNotFound {
		t.Fatalf("kind = %q", kind)
	}
}

func TestDiagnosticsNeverLeakTokensOrBodies(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	secrets := []string{"FIXTURE_PAGE_2", "private-body-marker"}
	if v := first.NextCursor.Session.VisitorData; v != "" {
		secrets = append(secrets, v)
	}

	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) {
			return textResponse(400, "private-body-marker FIXTURE_PAGE_2", nil), nil
		},
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	rejected := fetchError(t, err)

	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) {
			return textResponse(502, "private-body-marker", nil), nil
		},
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	unavailable := fetchError(t, err)

	ft.install(t, map[reqKind]handlerFunc{
		kindContinuation: func(tracedReq) (*http.Response, error) {
			return okResponse(`{"onResponseReceivedActions":[]}`), nil
		},
	})
	_, err = c.NextChannelPage(context.Background(), *first.NextCursor)
	odd := fetchError(t, err)

	for _, fe := range []*FetchError{rejected, unavailable, odd} {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(fe.Message, secret) {
				t.Fatalf("message lộ %q: %q", secret, fe.Message)
			}
		}
	}
}

func TestParseRetryAfterReadsDeltaAndHTTPDates(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	check := func(value string, want *time.Duration) {
		t.Helper()
		got := ParseRetryAfter(value, now)
		if (got == nil) != (want == nil) || (got != nil && want != nil && *got != *want) {
			t.Fatalf("ParseRetryAfter(%q) = %v, muốn %v", value, got, want)
		}
	}
	d := func(sec int) *time.Duration { v := time.Duration(sec) * time.Second; return &v }
	check("90", d(90))
	check(" 0 ", d(0))
	check("Tue, 13 Oct 2026 10:02:00 GMT", d(120))
	check("Tue, 13 Oct 2026 09:00:00 GMT", d(0))
	check("soon", nil)
	check("", nil)
	check("99999999999", d(31_536_000))
}

// ------------------------- huỷ và bộ điều tốc chung --------------------------

func TestAbortedContextSendsNothing(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newFixtureClient(ft).FirstChannelPage(ctx, testChannel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	assertKinds(t, ft)
}

func TestAbortInFlightSurfacesAbortNotClassifiedFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindHome: func(req tracedReq) (*http.Response, error) {
			cancel()
			<-req.ctx.Done()
			return nil, req.ctx.Err()
		},
	})
	_, err := newFixtureClient(ft).FirstChannelPage(ctx, testChannel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	var fe *FetchError
	if errors.As(err, &fe) {
		t.Fatalf("lỗi huỷ không được thành FetchError: %v", fe)
	}
}

func TestEveryRequestGoesThroughThePacer(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	var order []string
	pacer := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond}
	// Bọc pacer bằng cách đếm request qua transport: trace đã ghi thứ tự —
	// tương đương assert start/done xen kẽ của bản TS.
	c := NewClient(Options{HTTPClient: &http.Client{Transport: ft}, Pacer: pacer})
	ctx := context.Background()
	first, err := c.FirstChannelPage(ctx, "@veritasium")
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	order = append(order, "first-done")
	if _, err := c.NextChannelPage(ctx, *first.NextCursor); err != nil {
		t.Fatalf("NextChannelPage: %v", err)
	}
	order = append(order, "next-done")
	assertKinds(t, ft, kindSW, kindResolve, kindHome, kindTab, kindContinuation)
	// Request xếp hàng nghiêm ngặt: trace không chồng lấn (mỗi request xong
	// trước khi cái sau bắt đầu — kiểm gián tiếp qua thứ tự trace duy nhất).
	if len(order) != 2 {
		t.Fatalf("order = %v", order)
	}
}

func TestPacerRefusalBeforeRequestPropagatesUnchanged(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	retryAt := time.Now().Add(time.Hour)
	pacer := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond, MaxWait: time.Millisecond}
	// Ép lịch chung quá xa ngân sách chờ → DeferredError thay vì chờ.
	pacer.penalize(retryAt)
	c := NewClient(Options{HTTPClient: &http.Client{Transport: ft}, Pacer: pacer})
	_, err := c.FirstChannelPage(context.Background(), testChannel)
	var de *DeferredError
	if !errors.As(err, &de) {
		t.Fatalf("mong đợi *DeferredError, nhận %v", err)
	}
	if !de.RetryAt.Equal(retryAt) {
		t.Fatalf("retryAt = %v, muốn %v", de.RetryAt, retryAt)
	}
	assertKinds(t, ft)

	// Bootstrap bị từ chối không được giữ làm session.
	pacer2 := &Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond}
	c2 := NewClient(Options{HTTPClient: &http.Client{Transport: ft}, Pacer: pacer2})
	page, err := c2.FirstChannelPage(context.Background(), testChannel)
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	if len(page.Videos) != 4 {
		t.Fatalf("videos = %d", len(page.Videos))
	}
}

func TestPacer429StillClassifiedFromResponse(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindTab: func(tracedReq) (*http.Response, error) {
			return textResponse(429, "", map[string]string{"Retry-After": "45"}), nil
		},
	})
	c := newFixtureClient(ft)
	_, err := c.FirstChannelPage(context.Background(), testChannel)
	fe := fetchError(t, err)
	if fe.Kind != KindRateLimited || fe.RetryDelay == nil || *fe.RetryDelay != 45*time.Second {
		t.Fatalf("fe = %+v", fe)
	}
}

// ------------------------- tuần tự cursor / remainder ------------------------

func validCursor() ChannelCursor {
	return ChannelCursor{
		Version:      1,
		ChannelID:    testChannel,
		Tab:          "videos",
		Order:        "latest",
		Continuation: Continuation{Token: "FIXTURE_PAGE_2", Request: "CONTINUATION_REQUEST_TYPE_BROWSE"},
		Session:      SessionContext{ClientName: "WEB", ClientVersion: "2.20261012.01.00", VisitorData: "CgtGaXh0dXJl%3D"},
	}
}

func TestCursorValidationRejectsMutations(t *testing.T) {
	raw, err := json.Marshal(validCursor())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseChannelCursor(string(raw)); err != nil {
		t.Fatalf("cursor hợp lệ bị từ chối: %v", err)
	}

	mutations := map[string]func(c jmap){
		"other version":  func(c jmap) { c["version"] = 2.0 },
		"bad channel id": func(c jmap) { c["channelId"] = "UCshort" },
		"other tab":      func(c jmap) { c["tab"] = "shorts" },
		"other order":    func(c jmap) { c["order"] = "popular" },
		"extra key":      func(c jmap) { c["extra"] = 1 },
		"extra continuation key": func(c jmap) {
			jobj(c["continuation"])["command"] = jmap{}
		},
		"other request type": func(c jmap) {
			jobj(c["continuation"])["request"] = "CONTINUATION_REQUEST_TYPE_WATCH_NEXT"
		},
		"empty token": func(c jmap) { jobj(c["continuation"])["token"] = "" },
		"token with whitespace": func(c jmap) {
			jobj(c["continuation"])["token"] = "a b"
		},
		"oversized token": func(c jmap) {
			jobj(c["continuation"])["token"] = strings.Repeat("A", 20_000)
		},
		"bad client name": func(c jmap) { jobj(c["session"])["clientName"] = "web; drop" },
		"oversized visitor data": func(c jmap) {
			jobj(c["session"])["visitorData"] = strings.Repeat("A", 2000)
		},
	}
	for name, mutate := range mutations {
		var c jmap
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		mutate(c)
		mutated, _ := json.Marshal(c)
		if _, err := ParseChannelCursor(string(mutated)); err == nil {
			t.Fatalf("%s: cursor bị chấp nhận", name)
		} else {
			fe := fetchError(t, err)
			if fe.Kind != KindCursorInvalid {
				t.Fatalf("%s: kind = %q", name, fe.Kind)
			}
		}
	}

	if _, err := ParseChannelCursor("{not json"); err == nil {
		t.Fatal("JSON hỏng được chấp nhận")
	}
	var padded jmap
	_ = json.Unmarshal(raw, &padded)
	padded["padding"] = strings.Repeat("x", MaxCursorBytes)
	paddedRaw, _ := json.Marshal(padded)
	if _, err := ParseChannelCursor(string(paddedRaw)); err == nil {
		t.Fatal("cursor quá lớn được chấp nhận")
	}
	bad := validCursor()
	bad.ChannelID = "bad"
	if _, err := SerializeChannelCursor(bad); err == nil {
		t.Fatal("serialize cursor xấu không lỗi")
	}
}

func testVideo(id string, over func(*ScannedVideo)) ScannedVideo {
	dur := int64(61)
	views := int64(1200)
	pubText := "2d ago"
	pubAt := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	v := ScannedVideo{
		ID:            id,
		Title:         "Video " + id,
		ThumbnailURL:  "https://i.ytimg.com/vi/" + id + "/mqdefault.jpg",
		DurationSec:   &dur,
		ViewCount:     &views,
		PublishedText: &pubText,
		PublishedAt:   &pubAt,
	}
	if over != nil {
		over(&v)
	}
	return v
}

func TestPageRemainderRoundTripsWithTimesAndNulls(t *testing.T) {
	videos := []ScannedVideo{
		testVideo("aaaaaaaaaaa", nil),
		testVideo("bbbbbbbbbbb", func(v *ScannedVideo) {
			v.DurationSec, v.ViewCount, v.PublishedText, v.PublishedAt = nil, nil, nil, nil
		}),
	}
	stored, err := SerializeScannedVideos(videos)
	if err != nil {
		t.Fatalf("SerializeScannedVideos: %v", err)
	}
	restored, err := ParseScannedVideos(stored)
	if err != nil {
		t.Fatalf("ParseScannedVideos: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("restored = %d", len(restored))
	}
	if restored[0].PublishedAt == nil || !restored[0].PublishedAt.Equal(*videos[0].PublishedAt) {
		t.Fatalf("publishedAt = %v", restored[0].PublishedAt)
	}
	if restored[1].PublishedAt != nil || restored[1].DurationSec != nil {
		t.Fatalf("restored[1] = %+v", restored[1])
	}
	if restored[0].ID != videos[0].ID || restored[0].Title != videos[0].Title {
		t.Fatalf("restored[0] = %+v", restored[0])
	}
}

func TestPageRemainderLimitsEnforced(t *testing.T) {
	tooMany := make([]ScannedVideo, MaxRemainderVideos+1)
	for i := range tooMany {
		id := "v" + strings.Repeat("0", 9) + string(rune('a'+i%26))
		tooMany[i] = testVideo(id[:11], nil)
	}
	if _, err := SerializeScannedVideos(tooMany); err == nil {
		t.Fatal("quá nhiều video được serialize")
	}
	raw, _ := json.Marshal(tooMany)
	if _, err := ParseScannedVideos(string(raw)); err == nil {
		t.Fatal("quá nhiều video được parse")
	}
	if _, err := ParseScannedVideos("[{"); err == nil {
		t.Fatal("JSON hỏng được parse")
	}
	extra := testVideo("aaaaaaaaaaa", nil)
	var m jmap
	rawV, _ := json.Marshal(extra)
	_ = json.Unmarshal(rawV, &m)
	m["extra"] = true
	rawExtra, _ := json.Marshal([]any{m})
	if _, err := ParseScannedVideos(string(rawExtra)); err == nil {
		t.Fatal("khoá lạ được chấp nhận")
	}
	bad := testVideo("not-an-id", nil)
	rawBad, _ := json.Marshal([]ScannedVideo{bad})
	if _, err := ParseScannedVideos(string(rawBad)); err == nil {
		t.Fatal("id xấu được chấp nhận")
	}
	long := testVideo("aaaaaaaaaaa", func(v *ScannedVideo) { v.Title = strings.Repeat("x", 10_000) })
	rawLong, _ := json.Marshal([]ScannedVideo{long})
	if _, err := ParseScannedVideos(string(rawLong)); err == nil {
		t.Fatal("title quá dài được chấp nhận")
	}
}

// Endpoint phải ghép thành /youtubei/v1/<path>: URL dán thiếu dấu "/" giữa
// version và tên endpoint từng làm YouTube trả HTTP 400 trên trang thật.
func TestRequestPathsAreInnerTubeAPIPaths(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	c := newFixtureClient(ft)
	first, err := c.FirstChannelPage(context.Background(), "@veritasium")
	if err != nil {
		t.Fatalf("FirstChannelPage: %v", err)
	}
	if _, err := c.NextChannelPage(context.Background(), *first.NextCursor); err != nil {
		t.Fatalf("NextChannelPage: %v", err)
	}
	want := map[reqKind]string{
		kindResolve:      "/youtubei/v1/navigation/resolve_url",
		kindHome:         "/youtubei/v1/browse",
		kindTab:          "/youtubei/v1/browse",
		kindContinuation: "/youtubei/v1/browse",
	}
	seen := map[reqKind]bool{}
	for _, req := range ft.trace {
		if req.kind == kindSW {
			if req.path != "/sw.js_data" {
				t.Fatalf("bootstrap path = %q", req.path)
			}
			continue
		}
		if req.path != want[req.kind] {
			t.Fatalf("%s path = %q, muốn %q", req.kind, req.path, want[req.kind])
		}
		seen[req.kind] = true
	}
	for kind := range want {
		if !seen[kind] {
			t.Fatalf("không có request %s", kind)
		}
	}
}
