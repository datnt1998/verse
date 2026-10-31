package editor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func TestMediaParseAssetLinkAcceptsOnlyProviderVideos(t *testing.T) {
	for _, c := range []struct{ input, platform string }{{"bilibili.com/video/BV1fspq6cEty?p=2&spm_id_from=x", "bilibili"}, {"http://www.twitch.tv/videos/2899740602", "twitch"}, {"https://m.bilibili.com/video/av170001", "bilibili"}, {"https://b23.tv/yZZOjOg", "bilibili"}, {"https://www.twitch.tv/shroud/clip/PreciousRelievedCodCorgiDerp-IqjaHMAL8JQQH78F", "twitch"}, {"https://clips.twitch.tv/PreciousRelievedCodCorgiDerp", "twitch"}} {
		link := ParseAssetLink(c.input)
		if link == nil || link.Platform != c.platform || !strings.HasPrefix(link.URL, "https://") {
			t.Fatalf("%s: %+v", c.input, link)
		}
	}
	for _, input := range []string{"https://space.bilibili.com/546195/video", "https://live.bilibili.com/123", "https://www.bilibili.com/video/BV1short", "https://www.twitch.tv/shroud", "https://www.twitch.tv/shroud/videos?filter=archives", "https://www.youtube.com/watch?v=MiUHjLxm3V0", "https://notbilibili.com/video/BV1fspq6cEty", "BV1fspq6cEty", "https://127.0.0.1/videos/1", "https://twitch.tv@127.0.0.1/videos/1", "https://twitch.tv:22/videos/1"} {
		if ParseAssetLink(input) != nil {
			t.Fatalf("unsafe/non-video link accepted: %s", input)
		}
	}
}
func TestMediaResolveCutBoundsSourceAndClips(t *testing.T) {
	for _, duration := range []*float64{nil, mediaNumber(600)} {
		cut, e := ResolveCut("", "  ", duration)
		if e != nil || cut != nil {
			t.Fatalf("whole: %v %v", cut, e)
		}
	}
	for _, c := range []struct {
		from, to string
		duration *float64
		want     LinkCut
	}{{"1:00", "1:30", mediaNumber(29520), LinkCut{60, 90}}, {"", "0:45", mediaNumber(29520), LinkCut{0, 45}}, {"4:00", "", mediaNumber(300), LinkCut{240, 300}}, {"4:00", "6:00", mediaNumber(300), LinkCut{240, 300}}, {"1:02:03", "1:05:00", nil, LinkCut{3723, 3900}}} {
		got, e := ResolveCut(c.from, c.to, c.duration)
		if e != nil || got == nil || *got != c.want {
			t.Fatalf("cut %v %v", got, e)
		}
	}
	for _, c := range []struct {
		from, to string
		duration *float64
	}{{"", "", mediaNumber(601)}, {"1:30", "1:30", mediaNumber(600)}, {"0:00", "10:01", mediaNumber(29520)}, {"90", "2:00", mediaNumber(600)}, {"12:00", "13:00", mediaNumber(600)}, {"1:00", "", nil}} {
		_, e := ResolveCut(c.from, c.to, c.duration)
		mediaErrorCode(t, e, "invalid_request")
	}
}
func fakeMediaLinkRunner(t *testing.T, info LinkInfo, calls *[][]string) mediaRunnerFunc {
	t.Helper()
	return func(_ context.Context, binary string, args []string, _ tools.Spec) (tools.Result, error) {
		*calls = append(*calls, append([]string(nil), args...))
		if len(args) > 0 && args[0] == "-J" {
			b, e := json.Marshal(info)
			return tools.Result{Stdout: string(b)}, e
		}
		if strings.Contains(strings.Join(args, " "), "rawvideo") {
			return tools.Result{Stdout: string(mediaSolidFrame(0, 255, 0))}, nil
		}
		output := ""
		for i, arg := range args {
			if arg == "-o" && i+1 < len(args) {
				output = strings.ReplaceAll(args[i+1], "%(ext)s", "mp4")
			}
		}
		if output == "" && len(args) > 0 {
			last := args[len(args)-1]
			if strings.HasSuffix(last, "asset.mp4") || strings.HasSuffix(last, "asset.mkv") {
				output = last
			}
		}
		if output == "" {
			return tools.Result{}, errors.New("unexpected tool invocation: " + binary + " " + strings.Join(args, " "))
		}
		b, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "mp4-h264.mp4"))
		if e != nil {
			return tools.Result{}, e
		}
		return tools.Result{}, os.WriteFile(output, b, 0600)
	}
}
func mediaLinkTestService(t *testing.T, info LinkInfo) (*Service, *[][]string) {
	t.Helper()
	s := newMediaTestService(t)
	s.FFmpeg = "/managed/ffmpeg"
	s.YTDLP = "/managed/yt-dlp"
	calls := [][]string{}
	s.Runner = fakeMediaLinkRunner(t, info, &calls)
	s.Prober = mediaProberFunc(func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Kind: "video", MIME: "video/mp4", Width: mediaNumber(160), Height: mediaNumber(90), DurationSec: mediaNumber(1)}, nil
	})
	return s, &calls
}

type mediaProberFunc func(context.Context, string) (ProbeResult, error)

func (f mediaProberFunc) Probe(ctx context.Context, file string) (ProbeResult, error) {
	return f(ctx, file)
}
func TestMediaLinkTwitchCutAttributedOverlayAndReuse(t *testing.T) {
	ctx := context.Background()
	info := LinkInfo{Title: "A / stream", Duration: mediaNumber(29520), Channel: mediaString("Streamer"), UploaderURL: mediaString("https://twitch.tv/streamer"), Formats: []LinkFormat{{URL: "https://cdn.example/video.m3u8", VideoCodec: mediaString("avc1"), AudioCodec: mediaString("aac"), Height: mediaNumber(480)}}}
	s, calls := mediaLinkTestService(t, info)
	mediaTestProject(t, s, "holder")
	deps := &LinkImportDeps{FetchText: func(context.Context, string) (string, error) {
		return "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:60,\n0.ts\n#EXTINF:60,\n1.ts\n#EXTINF:60,\n2.ts\n", nil
	}}
	opts := FetchLinkMediaOptions{URL: "http://www.twitch.tv/videos/2899740602", From: "1:05", To: "1:35", ProjectID: "holder", Overlay: true}
	m, reused, e := s.FetchLinkMedia(ctx, opts, deps)
	if e != nil || reused || !m.Overlay || m.Attribution == nil || m.Attribution.Provider != "Twitch" || m.Background == nil || m.Background.Kind != "green" || !strings.Contains(m.FileName, "(1.05-1.35)") {
		t.Fatalf("import: %+v %v %v", m, reused, e)
	}
	again, reused, e := s.FetchLinkMedia(ctx, opts, deps)
	if e != nil || !reused || again.ID != m.ID {
		t.Fatalf("reuse: %+v %v %v", again, reused, e)
	}
	foundCut := false
	for _, args := range *calls {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-c copy") && strings.Contains(joined, "-ss 5") && strings.Contains(joined, "-t 30") {
			foundCut = true
		}
		if strings.Contains(joined, "--download-sections") {
			t.Fatal("HLS used serial yt-dlp sections")
		}
	}
	if !foundCut {
		t.Fatal("HLS remux was not invoked")
	}
}
func TestMediaLinkWholeClipAndUnlinkedSuppliedInfo(t *testing.T) {
	ctx := context.Background()
	info := LinkInfo{Title: "Short clip", Duration: mediaNumber(30)}
	s, calls := mediaLinkTestService(t, info)
	mediaTestProject(t, s, "holder")
	m, reused, e := s.FetchLinkMedia(ctx, FetchLinkMediaOptions{URL: "https://clips.twitch.tv/SomeClip", ProjectID: "holder"}, nil)
	if e != nil || reused || m.Overlay {
		t.Fatalf("whole %+v %v", m, e)
	}
	for _, args := range *calls {
		if strings.Contains(strings.Join(args, " "), "--download-sections") {
			t.Fatal("whole clip got cut")
		}
	}
	s, calls = mediaLinkTestService(t, info)
	m, _, e = s.FetchLinkMedia(ctx, FetchLinkMediaOptions{URL: "https://clips.twitch.tv/SomeClip", Info: &info}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, args := range *calls {
		if len(args) > 0 && args[0] == "-J" {
			t.Fatal("supplied info still probed")
		}
	}
	records, e := s.ListMedia(ctx, "")
	if e != nil || len(records) != 1 || records[0].ID != m.ID {
		t.Fatalf("unlinked %v %v", records, e)
	}
}
func TestMediaLinkLongLiveMissingAndUnavailableFailBeforeImport(t *testing.T) {
	ctx := context.Background()
	for _, info := range []LinkInfo{{Duration: mediaNumber(601)}, {Duration: mediaNumber(30), LiveStatus: mediaString("is_live")}, {LiveStatus: mediaString("is_upcoming")}} {
		s, _ := mediaLinkTestService(t, info)
		_, _, e := s.FetchLinkMedia(ctx, FetchLinkMediaOptions{URL: "https://twitch.tv/videos/123"}, nil)
		mediaErrorCode(t, e, "invalid_request")
		records, e := s.ListMedia(ctx, "")
		if e != nil || len(records) != 0 {
			t.Fatalf("failed import stored record: %v %v", records, e)
		}
	}
	s, calls := mediaLinkTestService(t, LinkInfo{})
	_, _, e := s.FetchLinkMedia(ctx, FetchLinkMediaOptions{URL: "https://twitch.tv/videos/123", ProjectID: "missing"}, nil)
	if e == nil || len(*calls) != 0 {
		t.Fatal("missing project reached downloader")
	}
	for _, code := range []tools.YtDlpErrorCode{tools.YtDlpUnavailable, tools.YtDlpRateLimited, tools.YtDlpLoginRequired} {
		s, _ := mediaLinkTestService(t, LinkInfo{})
		s.Runner = mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
			return tools.Result{}, &tools.YtDlpError{Code: code, Message: "provider rejected"}
		})
		_, _, e = s.FetchLinkMedia(ctx, FetchLinkMediaOptions{URL: "https://bilibili.com/video/BV1fspq6cEty"}, nil)
		want := "internal_error"
		if code == tools.YtDlpUnavailable {
			want = "not_found"
		}
		if code == tools.YtDlpLoginRequired {
			want = "invalid_request"
		}
		mediaErrorCode(t, e, want)
		var api *Error
		if !errors.As(e, &api) || api.Extra["ytDlpCode"] != string(code) {
			t.Fatal("stable yt-dlp code lost")
		}
		if code == tools.YtDlpRateLimited && strings.Contains(api.Message, "yt-dlp") {
			t.Fatal("rate limit advised update")
		}
	}
}
func TestMediaLinkPlaylistSliceMasterMapAndPrivateAddressPolicy(t *testing.T) {
	p, e := SliceMediaPlaylist("#EXTM3U\n#EXT-X-STREAM-INF:RESOLUTION=1920x1080\nhigh.m3u8\n#EXT-X-STREAM-INF:RESOLUTION=854x480\nlow.m3u8\n", "https://cdn.example/master.m3u8", 0, 30)
	if e != nil || p.Variant != "https://cdn.example/low.m3u8" {
		t.Fatalf("master %v %v", p, e)
	}
	p, e = SliceMediaPlaylist("#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXT-X-PROGRAM-DATE-TIME:ignore\n#EXTINF:10,\na.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:10,\nb.ts\n#EXTINF:10,\nc.ts\n", "https://cdn.example/path/list.m3u8", 15, 25)
	if e != nil || p.OffsetSec != 5 || strings.Contains(p.Playlist, "PROGRAM-DATE-TIME") || !strings.Contains(p.Playlist, "https://cdn.example/path/init.mp4") || !strings.Contains(p.Playlist, "#EXT-X-ENDLIST") {
		t.Fatalf("slice %v %v", p, e)
	}
	for _, raw := range []string{"http://127.0.0.1/a", "http://[::1]/a", "http://10.0.0.1/a", "http://169.254.169.254/a", "http://localhost/a", "file:///tmp/a", "http://user:pass@cdn.example/a"} {
		if _, e := mediaRemoteURL(raw); e == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
	_, e = SliceMediaPlaylist("#EXTM3U\n#EXTINF:1,\nhttp://127.0.0.1/private\n", "https://cdn.example/a", 0, 1)
	if e == nil {
		t.Fatal("playlist private segment accepted")
	}
	p, e = SliceMediaPlaylist("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128\n", "https://cdn.example/a", 0, 1)
	if e != nil || p != nil {
		t.Fatalf("encrypted fallback %v %v", p, e)
	}
	cut, e := ResolveCut("1:00", "1:30", mediaNumber(600))
	if e != nil || !reflect.DeepEqual(cut, &LinkCut{60, 90}) {
		t.Fatal(e)
	}
}
