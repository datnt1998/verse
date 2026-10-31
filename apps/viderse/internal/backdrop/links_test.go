package backdrop_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// fakeDoer trả phản hồi theo URL: dùng cho luật một lần đọc b23.tv.
type fakeDoer struct {
	status   int
	location string
	err      error
	calls    int
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	header := http.Header{}
	if f.location != "" {
		header.Set("Location", f.location)
	}
	status := f.status
	if status == 0 {
		status = http.StatusFound
	}
	return &http.Response{StatusCode: status, Header: header, Body: http.NoBody}, nil
}

func TestParseChannelLink(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		input  string
		wantID string
		reject bool
	}{
		{"twitch bare", "twitch.tv/SomeStreamer", "twitch:somestreamer", false},
		{"twitch www", "https://www.twitch.tv/somestreamer", "twitch:somestreamer", false},
		{"twitch videos page", "https://twitch.tv/SomeStreamer/videos/all", "twitch:somestreamer", false},
		{"twitch clips page", "https://twitch.tv/SomeStreamer/clips?filter=clips", "twitch:somestreamer", false},
		{"twitch vod link is a video", "https://www.twitch.tv/somestreamer/videos/1234567", "", true},
		{"twitch clip path is a video", "https://www.twitch.tv/somestreamer/clip/SomeSlug", "", true},
		{"clips host is a video", "https://clips.twitch.tv/SomeSlug", "", true},
		{"twitch live", "https://www.twitch.tv/videos/123", "", true},
		{"twitch reserved", "https://www.twitch.tv/directory", "", true},
		{"space", "https://space.bilibili.com/123456", "bilibili:123456", false},
		{"space video tab", "https://space.bilibili.com/123456/video?tid=0", "bilibili:123456", false},
		{"space upload", "https://space.bilibili.com/123456/upload/video", "bilibili:123456", false},
		{"space with query", "https://space.bilibili.com/123456?from=search", "bilibili:123456", false},
		{"space missing mid", "https://space.bilibili.com/", "", true},
		{"bilibili video", "https://www.bilibili.com/video/BV1xx411c7mD", "", true},
		{"youtube", "https://www.youtube.com/@somebody", "", true},
		{"invalid text", "not a url at all", "", true},
		{"empty", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link, err := backdrop.ParseChannelLink(ctx, tc.input, nil)
			if tc.reject {
				var apiErr *backdrop.Error
				if !errors.As(err, &apiErr) || apiErr.Code != backdrop.CodeInvalidInput {
					t.Fatalf("muốn invalid_input, được %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("link hợp lệ bị từ chối: %v", err)
			}
			if link.ID() != tc.wantID {
				t.Fatalf("id = %q, muốn %q", link.ID(), tc.wantID)
			}
		})
	}
}

func TestParseChannelLinkRecoveryCopy(t *testing.T) {
	ctx := context.Background()
	_, err := backdrop.ParseChannelLink(ctx, "https://www.twitch.tv/somestreamer", nil)
	if err != nil {
		t.Fatalf("kênh hợp lệ: %v", err)
	}
	_, err = backdrop.ParseChannelLink(ctx, "https://www.twitch.tv/somestreamer/clip/SomeSlug", nil)
	var apiErr *backdrop.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("muốn lỗi nghiệp vụ, được %v", err)
	}
	if apiErr.Message != "Dán link kênh Twitch hoặc trang cá nhân Bilibili" {
		t.Fatalf("message = %q", apiErr.Message)
	}
	if !strings.Contains(apiErr.Recovery, "video lẻ") {
		t.Fatalf("recovery cho link video = %q", apiErr.Recovery)
	}
}

func TestParseChannelLinkB23SingleRedirect(t *testing.T) {
	ctx := context.Background()

	space := &fakeDoer{location: "https://space.bilibili.com/98765?share_source=copy"}
	link, err := backdrop.ParseChannelLink(ctx, "https://b23.tv/AbCdEf", space)
	if err != nil {
		t.Fatalf("b23 trỏ tới trang cá nhân: %v", err)
	}
	if link.ID() != "bilibili:98765" {
		t.Fatalf("id = %q", link.ID())
	}
	if space.calls != 1 {
		t.Fatalf("calls = %d, muốn 1 lần đọc chuyển hướng", space.calls)
	}

	video := &fakeDoer{location: "https://www.bilibili.com/video/BV1xx411c7mD"}
	_, err = backdrop.ParseChannelLink(ctx, "https://b23.tv/AbCdEf", video)
	var apiErr *backdrop.Error
	if !errors.As(err, &apiErr) || !strings.Contains(apiErr.Recovery, "video lẻ") {
		t.Fatalf("b23 trỏ tới video: %v", err)
	}

	other := &fakeDoer{location: "https://example.com/foo"}
	_, err = backdrop.ParseChannelLink(ctx, "https://b23.tv/AbCdEf", other)
	if !errors.As(err, &apiErr) || apiErr.Recovery != "Ví dụ: https://www.twitch.tv/<tên kênh> hoặc https://space.bilibili.com/<số>" {
		t.Fatalf("b23 trỏ nơi khác: %v", err)
	}

	broken := &fakeDoer{err: errors.New("mạng hỏng")}
	_, err = backdrop.ParseChannelLink(ctx, "https://b23.tv/AbCdEf", broken)
	if !errors.As(err, &apiErr) || apiErr.Message != "Không mở được link rút gọn b23.tv" {
		t.Fatalf("b23 lỗi mạng: %v", err)
	}
}

func TestChannelOfIDRoundTrip(t *testing.T) {
	if link, ok := backdrop.ChannelOfID("twitch:somestreamer"); !ok || link.URL != "https://www.twitch.tv/somestreamer" {
		t.Fatalf("twitch: %+v %v", link, ok)
	}
	if link, ok := backdrop.ChannelOfID("bilibili:123456"); !ok || link.URL != "https://space.bilibili.com/123456" {
		t.Fatalf("bilibili: %+v %v", link, ok)
	}
	for _, id := range []string{"twitch:Videos", "twitch:", "youtube:abc", "twitch:a:b", "", "bilibili:not-a-mid"} {
		if _, ok := backdrop.ChannelOfID(id); ok {
			t.Fatalf("%q phải bị từ chối", id)
		}
	}
}

func TestParseAssetLink(t *testing.T) {
	cases := []struct {
		input  string
		wantID string
		ok     bool
	}{
		{"https://www.twitch.tv/somechannel/videos/2233445", "twitch:v2233445", true},
		{"https://www.twitch.tv/videos/2233445", "twitch:v2233445", true},
		{"https://twitch.tv/videos/99887766?filter=all", "twitch:v99887766", true},
		{"https://www.twitch.tv/somechannel/clip/AbcDefGhi", "twitch:AbcDefGhi", true},
		{"https://clips.twitch.tv/AbcDefGhi?tt_content=url", "twitch:AbcDefGhi", true},
		{"https://www.bilibili.com/video/BV1xx411c7mD/?spm=x", "bilibili:BV1xx411c7mD", true},
		{"https://www.bilibili.com/video/av170001", "bilibili:av170001", true},
		{"https://b23.tv/AbCdEf", "bilibili:AbCdEf", true},
		{"https://www.twitch.tv/somechannel", "", false},
		{"https://space.bilibili.com/123456", "", false},
		{"https://www.bilibili.com/video/BVbad", "", false},
		{"https://www.youtube.com/watch?v=abc", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		link, ok := backdrop.ParseAssetLink(tc.input)
		if ok != tc.ok {
			t.Fatalf("%q: ok = %v, muốn %v", tc.input, ok, tc.ok)
		}
		if ok && link.ID() != tc.wantID {
			t.Fatalf("%q: id = %q, muốn %q", tc.input, link.ID(), tc.wantID)
		}
	}
}

func TestParseSourceKinds(t *testing.T) {
	ctx := context.Background()

	source, err := backdrop.ParseSource(ctx, "https://www.douyin.com/video/123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if source.Kind != domain.SourceUnsupported || source.Label != "Douyin" || source.Host != "douyin.com" {
		t.Fatalf("douyin = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://www.tiktok.com/@x/video/1", nil)
	if source.Kind != domain.SourceUnsupported || source.Label != "TikTok" {
		t.Fatalf("tiktok = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://www.kuaishou.com/short-video/1", nil)
	if source.Kind != domain.SourceUnsupported || source.Label != "Kuaishou" {
		t.Fatalf("kuaishou = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://www.xiaohongshu.com/explore/1", nil)
	if source.Kind != domain.SourceUnsupported || source.Label != "Xiaohongshu" {
		t.Fatalf("xiaohongshu = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://www.example.com/watch/1", nil)
	if source.Kind != domain.SourceInvalid || source.Recovery == "" {
		t.Fatalf("host lạ = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://space.bilibili.com/123456", nil)
	if source.Kind != domain.SourceChannel || source.Platform != domain.PlatformBilibili || source.ID != "bilibili:123456" {
		t.Fatalf("kênh = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://www.bilibili.com/video/BV1xx411c7mD", nil)
	if source.Kind != domain.SourceVideo || source.ID != "bilibili:BV1xx411c7mD" || source.Title != nil || source.DurationSec != nil {
		t.Fatalf("video = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "https://b23.tv/AbCdEf", &fakeDoer{location: "https://www.bilibili.com/video/BV1xx411c7mD?p=1"})
	if source.Kind != domain.SourceVideo || source.ID != "bilibili:BV1xx411c7mD" {
		t.Fatalf("b23 video = %+v", source)
	}

	source, _ = backdrop.ParseSource(ctx, "   ", nil)
	if source.Kind != domain.SourceInvalid || source.Message == "" {
		t.Fatalf("rỗng = %+v", source)
	}
}

// countingBody ghi lại có ai đọc body của phản hồi b23 hay không.
type countingBody struct{ reads atomic.Int64 }

func (b *countingBody) Read([]byte) (int, error) {
	b.reads.Add(1)
	return 0, io.EOF
}
func (b *countingBody) Close() error { return nil }

// redirectDoer trả 302 tới đích; ghi lại request và body; không bao giờ được
// gọi cho địa chỉ đích.
type redirectDoer struct {
	mu       sync.Mutex
	requests []string
	deadline time.Duration
	body     *countingBody
}

func (d *redirectDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, req.URL.String())
	if deadline, ok := req.Context().Deadline(); ok {
		d.deadline = time.Until(deadline)
	}
	header := http.Header{}
	header.Set("Location", "https://space.bilibili.com/123456")
	return &http.Response{StatusCode: http.StatusFound, Header: header, Body: d.body}, nil
}

// TestResolveB23OneRequestNoRedirectNoBody: đọc đúng một lần chuyển hướng
// b23.tv, không theo redirect, không đọc body, deadline 10 giây.
func TestResolveB23OneRequestNoRedirectNoBody(t *testing.T) {
	doer := &redirectDoer{body: &countingBody{}}
	link, err := backdrop.ParseChannelLink(context.Background(), "https://b23.tv/AbCdEf", doer)
	if err != nil || link.ID() != "bilibili:123456" {
		t.Fatalf("link = %+v err=%v", link, err)
	}
	doer.mu.Lock()
	defer doer.mu.Unlock()
	if len(doer.requests) != 1 || !strings.HasPrefix(doer.requests[0], "https://b23.tv/") {
		t.Fatalf("requests = %v, muốn đúng 1 lần tới b23.tv và không theo redirect", doer.requests)
	}
	if doer.body.reads.Load() != 0 {
		t.Fatalf("không được đọc body, reads = %d", doer.body.reads.Load())
	}
	if doer.deadline <= 0 || doer.deadline > 11*time.Second {
		t.Fatalf("deadline = %v, muốn ~10 giây", doer.deadline)
	}
}

// TestResolveB23RejectsHostVariants: chỉ host chính xác b23.tv (không www/m,
// không cổng, không userinfo) mới được coi là link rút gọn.
func TestResolveB23RejectsHostVariants(t *testing.T) {
	doer := &redirectDoer{body: &countingBody{}}
	for _, input := range []string{
		"https://www.b23.tv/AbCdEf",
		"https://b23.tv.evil.test/AbCdEf",
		"https://user@b23.tv/AbCdEf",
	} {
		if _, err := backdrop.ParseChannelLink(context.Background(), input, doer); err == nil {
			t.Fatalf("%q phải bị từ chối", input)
		}
	}
	if len(doer.requests) != 0 {
		t.Fatalf("không được gọi mạng cho host sai: %v", doer.requests)
	}
}

// b23Transport định tuyến host b23.tv tới httptest server; mọi host khác lỗi
// ngay — tức redirect nếu bị theo sẽ lỗi nổi, không bao giờ chạm bilibili.com.
type b23Transport struct{ server string }

func (t b23Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() != "b23.tv" {
		return nil, fmt.Errorf("redirect bị theo tới %q", req.URL.String())
	}
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http" // chỉ test: đưa tới server HTTP thay vì b23.tv thật
	cloned.URL.Host = t.server
	return http.DefaultTransport.RoundTrip(cloned)
}

// TestB23LocationOneRequestNoFollowNoPause: đúng một request tới b23.tv, không
// theo redirect (không request thứ hai tới đích), chỉ đọc Location, và đường
// parse không đi qua pacer — thất bại không được ghi giờ chờ nền tảng.
func TestB23LocationOneRequestNoFollowNoPause(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q", r.Method)
		}
		w.Header().Set("Location", "https://space.bilibili.com/123456")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	host := server.Listener.Addr().String()

	// Client "production": transport gốc, có CheckRedirect mặc định — đây là
	// dạng doer đã từng làm bug (Do bên trong theo redirect).
	base := &http.Client{Timeout: 5 * time.Second, Transport: b23Transport{server: host}}
	doer := backdrop.NewBilibiliDoer(base, func(ctx context.Context, reset bool, fn func(context.Context) error) error {
		t.Fatal("parse path không được gọi pacer")
		return nil
	}, time.Now)

	link, err := backdrop.ParseChannelLink(context.Background(), "https://b23.tv/AbCdEf", doer)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if link.ID() != "bilibili:123456" {
		t.Fatalf("id = %q", link.ID())
	}
	if hits.Load() != 1 {
		t.Fatalf("requests tới b23.tv = %d, muốn đúng 1 (không follow, không retry)", hits.Load())
	}
}

// TestB23Location412ReturnsErrorWithoutPause: một lần 412 duy nhất của link
// rút gọn trả lỗi "giới hạn" cho UI nhưng không ghi vào trạng thái chặn của
// nền tảng (vì không đi qua pacer).
func TestB23Location412ReturnsErrorWithoutPause(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer server.Close()
	host := server.Listener.Addr().String()

	pacerCalls := atomic.Int64{}
	base := &http.Client{Timeout: 5 * time.Second}
	doer := backdrop.NewBilibiliDoer(base, func(ctx context.Context, reset bool, fn func(context.Context) error) error {
		pacerCalls.Add(1)
		return fn(ctx)
	}, time.Now)

	// Định tuyến b23.tv tới server; host khác lỗi nổi → không thể "vô tình" gọi
	// bilibili.com.
	tr := b23Transport{server: host}
	base.Transport = tr

	_, err := backdrop.ParseChannelLink(context.Background(), "https://b23.tv/AbCdEf", doer)
	if err == nil {
		t.Fatal("412 phải trả lỗi")
	}
	var rl *backdrop.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("muốn RateLimitError, được %T %v", err, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("requests = %d, muốn đúng 1", hits.Load())
	}
	if pacerCalls.Load() != 0 {
		t.Fatalf("pacer bị gọi %d lần từ đường parse — pause sẽ bị ghi sai", pacerCalls.Load())
	}
}
