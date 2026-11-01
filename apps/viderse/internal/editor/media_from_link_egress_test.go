package editor

// Cổng ra của tiến trình con: link của thành viên không được để yt-dlp hay
// ffmpeg tự nối tới địa chỉ nội bộ của máy chủ chia sẻ.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// hitServer là máy chủ "nội bộ" đếm request: mọi test dưới khẳng định không
// request nào tới được nó.
type hitServer struct {
	*httptest.Server
	hits atomic.Int64
}

func newHitServer(t *testing.T) *hitServer {
	t.Helper()
	h := &hitServer{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h.hits.Add(1)
		_, _ = w.Write([]byte("internal-secret"))
	}))
	t.Cleanup(h.Close)
	return h
}

func (h *hitServer) count() int64 { return h.hits.Load() }

// installHostedGuard dựng cổng ra hosted cho test và dọn sau khi xong.
func installHostedGuard(t *testing.T) *netguard.Guard {
	t.Helper()
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	return guard
}

func requireBlockedMessage(t *testing.T, err error) {
	t.Helper()
	var api *Error
	if !errors.As(err, &api) {
		t.Fatalf("lỗi = %v, muốn lỗi API", err)
	}
	if api.Message != netguard.BlockedMessage {
		t.Fatalf("thông báo = %q, muốn %q", api.Message, netguard.BlockedMessage)
	}
	for key, value := range api.Extra {
		if strings.Contains(fmt.Sprint(value), "127.0.0.1") || strings.Contains(fmt.Sprint(value), "internal-secret") {
			t.Fatalf("extra %q lộ địa chỉ/nội dung nội bộ: %v", key, value)
		}
	}
}

// TestMediaLinkHostedRefusesLoopbackWithRealYtDlp chạy binary yt-dlp thật qua
// cổng ra: yt-dlp tự phân giải và tự nối, nên đây là bằng chứng đường thật sự
// bị chặn — máy chủ nội bộ nhận 0 request.
func TestMediaLinkHostedRefusesLoopbackWithRealYtDlp(t *testing.T) {
	binary, err := exec.LookPath("yt-dlp")
	if err != nil {
		t.Skip("bỏ qua: không có binary yt-dlp trong môi trường test (lượt chạy thật cần yt-dlp)")
	}
	srv := newHitServer(t)
	guard := installHostedGuard(t)
	s := newMediaTestService(t)
	s.YTDLP = binary
	s.Runner = &tools.ExecRunner{}

	_, err = s.ProbeLinkInfo(context.Background(), srv.URL+"/video")
	if got := srv.count(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — yt-dlp nối thẳng, cổng ra bị vòng qua", got)
	}
	requireBlockedMessage(t, err)
	if strings.Contains(err.Error(), guard.ProxyURL()) {
		t.Fatal("lỗi lộ credential của proxy")
	}
}

// TestMediaLinkHostedFakeYtDlpGoesThroughProxy chứng minh phần dây nối mà
// không cần yt-dlp: binary giả nhận --proxy và env proxy, rồi tự GET qua proxy.
func TestMediaLinkHostedFakeYtDlpGoesThroughProxy(t *testing.T) {
	srv := newHitServer(t)
	guard := installHostedGuard(t)
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	envFile := filepath.Join(dir, "env.txt")
	resultFile := filepath.Join(dir, "result.txt")
	binary := filepath.Join(dir, "yt-dlp")
	// Binary giả: ghi argv/env rồi GET URL cuối qua proxy như yt-dlp/curl, và
	// thất bại như yt-dlp khi proxy từ chối (stderr mang dấu của cổng chặn).
	headersFile := filepath.Join(dir, "headers.txt")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argsFile + "\n" +
		"env > " + envFile + "\n" +
		"url=\"\"\n" +
		"for a in \"$@\"; do case \"$a\" in http*) url=\"$a\";; esac; done\n" +
		"curl -sS -o /dev/null -D " + headersFile + " -w '%{http_code}' --max-time 15 -x \"$http_proxy\" \"$url\" > " + resultFile + " 2>/dev/null\n" +
		"if grep -q viderse-egress-blocked " + headersFile + " 2>/dev/null; then\n" +
		"  echo 'ERROR: Unable to download webpage: HTTP Error 403: viderse-egress-blocked' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"printf '{}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("bỏ qua: không có curl để binary giả tự GET qua proxy")
	}

	s := newMediaTestService(t)
	s.YTDLP = binary
	s.Runner = &tools.ExecRunner{}
	_, err := s.ProbeLinkInfo(context.Background(), srv.URL+"/video")
	if got := srv.count(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — binary giả nối thẳng", got)
	}
	requireBlockedMessage(t, err)

	args, readErr := os.ReadFile(argsFile)
	if readErr != nil {
		t.Fatalf("binary giả không chạy: %v", readErr)
	}
	if !strings.Contains(string(args), "--proxy\n"+guard.ProxyURL()) {
		t.Fatalf("yt-dlp không nhận --proxy của cổng ra: %s", args)
	}
	env, readErr := os.ReadFile(envFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(env), "http_proxy="+guard.ProxyURL()) {
		t.Fatalf("env con thiếu http_proxy của cổng ra: %s", env)
	}
	result, readErr := os.ReadFile(resultFile)
	if readErr != nil {
		t.Fatalf("binary giả không GET được: %v", readErr)
	}
	if strings.TrimSpace(string(result)) != "403" {
		t.Fatalf("proxy trả %q, muốn 403 (đích nội bộ)", strings.TrimSpace(string(result)))
	}
	if got := srv.count(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request", got)
	}
}

// TestMediaLinkDesktopKeepsEnvironment: chế độ desktop không thêm proxy và
// tiến trình con vẫn thừa kế môi trường của app như trước.
func TestMediaLinkDesktopKeepsEnvironment(t *testing.T) {
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	envFile := filepath.Join(dir, "env.txt")
	binary := filepath.Join(dir, "yt-dlp")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\nenv > " + envFile + "\nprintf '{}'\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIDERSE_TEST_MARKER", "desktop")
	s := newMediaTestService(t)
	s.YTDLP = binary
	s.Runner = &tools.ExecRunner{}
	if _, err := s.ProbeLinkInfo(context.Background(), "https://www.bilibili.com/video/BV1fspq6cEty"); err != nil {
		t.Fatalf("desktop phải chạy được như cũ: %v", err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "--proxy") {
		t.Fatalf("desktop không được thêm proxy: %s", args)
	}
	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "VIDERSE_TEST_MARKER=desktop") {
		t.Fatal("desktop phải thừa kế môi trường của app")
	}
	if strings.Contains(string(env), "http_proxy="+netguard.BlockedMessage) {
		t.Fatal("desktop không được đặt proxy")
	}
}

// recordingRunner ghi lại args + Spec để kiểm dây nối ffmpeg.
type recordingRunner struct {
	calls [][]string
	specs []tools.Spec
	run   func(args []string) (tools.Result, error)
}

func (r *recordingRunner) Run(_ context.Context, _ string, args []string, spec tools.Spec) (tools.Result, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	r.specs = append(r.specs, spec)
	if r.run != nil {
		return r.run(args)
	}
	return tools.Result{}, nil
}

// TestMediaLinkHostedFFmpegUsesProxyAndWhitelist: ffmpeg cắt HLS phải nhận
// -http_proxy (kèm httpproxy trong protocol_whitelist) và env proxy.
func TestMediaLinkHostedFFmpegUsesProxyAndWhitelist(t *testing.T) {
	guard := installHostedGuard(t)
	info := LinkInfo{Title: "clip", Duration: mediaNumber(600), Formats: []LinkFormat{{
		URL: "https://cdn.example/video.m3u8", VideoCodec: mediaString("avc1"), AudioCodec: mediaString("aac"), Height: mediaNumber(480),
	}}}
	s := newMediaTestService(t)
	s.FFmpeg = "/managed/ffmpeg"
	s.YTDLP = "/managed/yt-dlp"
	s.Prober = mediaProberFunc(func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Kind: "video", MIME: "video/mp4", Width: mediaNumber(160), Height: mediaNumber(90), DurationSec: mediaNumber(30)}, nil
	})
	runner := &recordingRunner{run: func(args []string) (tools.Result, error) {
		if strings.Contains(strings.Join(args, " "), "rawvideo") {
			return tools.Result{Stdout: string(mediaSolidFrame(0, 255, 0))}, nil
		}
		for i, arg := range args {
			if arg == "-y" && i+1 < len(args) {
				path := args[len(args)-1]
				if err := os.WriteFile(path, []byte("mp4"), 0o600); err != nil {
					return tools.Result{}, err
				}
			}
		}
		return tools.Result{}, nil
	}}
	s.Runner = runner
	deps := &LinkImportDeps{FetchText: func(context.Context, string) (string, error) {
		return "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:60,\n0.ts\n#EXTINF:60,\n1.ts\n#EXT-X-ENDLIST\n", nil
	}}
	_, _, err := s.FetchLinkMedia(context.Background(), FetchLinkMediaOptions{URL: "https://www.bilibili.com/video/BV1fspq6cEty", From: "0:10", To: "0:40", Info: &info}, deps)
	if err != nil {
		t.Fatalf("cắt HLS: %v", err)
	}
	var ffmpegArgs []string
	var ffmpegSpec tools.Spec
	for i, args := range runner.calls {
		if strings.Contains(strings.Join(args, " "), "-c copy") {
			ffmpegArgs, ffmpegSpec = args, runner.specs[i]
		}
	}
	if ffmpegArgs == nil {
		t.Fatal("không có lượt ffmpeg cắt HLS")
	}
	joined := strings.Join(ffmpegArgs, " ")
	if !strings.Contains(joined, "-http_proxy "+guard.ProxyURL()) {
		t.Fatalf("ffmpeg thiếu -http_proxy của cổng ra: %s", joined)
	}
	if !strings.Contains(joined, "-protocol_whitelist file,http,https,tcp,tls,crypto,httpproxy") {
		t.Fatalf("protocol_whitelist thiếu httpproxy nên https không đi qua proxy: %s", joined)
	}
	if ffmpegSpec.EnvExact["http_proxy"] != guard.ProxyURL() {
		t.Fatalf("env ffmpeg = %v, muốn http_proxy của cổng ra", ffmpegSpec.EnvExact["http_proxy"])
	}
}

// TestMediaLinkRejectsNonHTTPMediaURLsBeforeFFmpeg: file:, concat:, subfile:
// của thành viên bị từ chối trước khi ffmpeg chạy — không đọc được tệp máy.
func TestMediaLinkRejectsNonHTTPMediaURLsBeforeFFmpeg(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "concat:file:///etc/passwd|file:///etc/hosts", "subfile,,start,0,end,10,,:file:///etc/passwd", "data:text/plain,hello", "rtmp://127.0.0.1/live"} {
		if _, err := mediaRemoteURL(raw); err == nil {
			t.Fatalf("URL %q phải bị từ chối", raw)
		}
	}
	// Cùng đường qua FetchLinkMedia: playlist có segment file: không bao giờ tới ffmpeg.
	info := LinkInfo{Title: "clip", Duration: mediaNumber(600), Formats: []LinkFormat{{
		URL: "https://cdn.example/video.m3u8", VideoCodec: mediaString("avc1"), AudioCodec: mediaString("aac"), Height: mediaNumber(480),
	}}}
	for _, segment := range []string{"file:///etc/passwd", "concat:file:///etc/passwd"} {
		s := newMediaTestService(t)
		s.FFmpeg = "/managed/ffmpeg"
		s.YTDLP = "/managed/yt-dlp"
		runner := &recordingRunner{run: func([]string) (tools.Result, error) {
			return tools.Result{}, errors.New("không được chạy ffmpeg")
		}}
		s.Runner = runner
		deps := &LinkImportDeps{FetchText: func(context.Context, string) (string, error) {
			return "#EXTM3U\n#EXTINF:10,\n" + segment + "\n", nil
		}}
		_, _, err := s.FetchLinkMedia(context.Background(), FetchLinkMediaOptions{URL: "https://www.bilibili.com/video/BV1fspq6cEty", From: "0:00", To: "0:05", Info: &info}, deps)
		if err == nil {
			t.Fatalf("segment %q phải bị từ chối", segment)
		}
		for _, args := range runner.calls {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "-c copy") || strings.Contains(joined, "/etc/passwd") {
				t.Fatalf("segment %q vào tới ffmpeg: %s", segment, joined)
			}
		}
	}
}

// TestMediaLinkBlockedPlaylistHostIsRefused: máy chủ của đoạn video trỏ về
// loopback bị từ chối trước khi ghi playlist cho ffmpeg.
func TestMediaLinkBlockedPlaylistHostIsRefused(t *testing.T) {
	s := newMediaTestService(t)
	s.FFmpeg = "/managed/ffmpeg"
	s.YTDLP = "/managed/yt-dlp"
	runner := &recordingRunner{}
	s.Runner = runner
	info := LinkInfo{Title: "clip", Duration: mediaNumber(600), Formats: []LinkFormat{{
		URL: "https://cdn.example/video.m3u8", VideoCodec: mediaString("avc1"), AudioCodec: mediaString("aac"), Height: mediaNumber(480),
	}}}
	deps := &LinkImportDeps{FetchText: func(context.Context, string) (string, error) {
		return "#EXTM3U\n#EXTINF:10,\nhttp://127.0.0.1:9/private.ts\n", nil
	}}
	_, _, err := s.FetchLinkMedia(context.Background(), FetchLinkMediaOptions{URL: "https://www.bilibili.com/video/BV1fspq6cEty", From: "0:00", To: "0:05", Info: &info}, deps)
	requireBlockedMessage(t, err)
	for _, args := range runner.calls {
		if strings.Contains(strings.Join(args, " "), "-c copy") {
			t.Fatal("ffmpeg chạy dù playlist trỏ địa chỉ nội bộ")
		}
	}
}

var _ = time.Second

// rtmpOnlyInfo là thông tin yt-dlp của một video chỉ có format rtmp: nối trực
// tiếp, không qua proxy HTTP của cổng ra.
const rtmpOnlyInfo = `{"title":"clip","duration":120,"formats":[
 {"url":"rtmp://127.0.0.1:1935/v","protocol":"rtmp","vcodec":"h264","acodec":"none","height":360},
 {"url":"rtmp://127.0.0.1:1935/a","protocol":"rtmp","vcodec":"none","acodec":"aac"}]}`

// TestMediaLinkRefusesNonProxiedFormatProtocols: video chỉ có rtmp bị từ chối
// với câu tiếng Việt rõ ràng, và không có lời gọi tải nào chạy.
func TestMediaLinkRefusesNonProxiedFormatProtocols(t *testing.T) {
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{Hosted: true}); err != nil {
		t.Fatal(err)
	}
	s := newMediaTestService(t)
	s.FFmpeg = "/managed/ffmpeg"
	s.YTDLP = "/managed/yt-dlp"
	runner := &recordingRunner{run: func(args []string) (tools.Result, error) {
		if strings.Contains(strings.Join(args, " "), "-J") {
			return tools.Result{Stdout: rtmpOnlyInfo}, nil
		}
		return tools.Result{}, errors.New("không được tải khi chỉ có định dạng rtmp")
	}}
	s.Runner = runner

	_, err := s.ProbeLinkInfo(context.Background(), "https://www.bilibili.com/video/BV1fspq6cEty")
	var api *Error
	if !errors.As(err, &api) || api.Message != netguard.UnsafeFormatMessage {
		t.Fatalf("probe: err = %v, muốn %q", err, netguard.UnsafeFormatMessage)
	}

	_, _, err = s.FetchLinkMedia(context.Background(), FetchLinkMediaOptions{URL: "https://www.bilibili.com/video/BV1fspq6cEty", From: "0:00", To: "0:05"}, nil)
	if !errors.As(err, &api) || api.Message != netguard.UnsafeFormatMessage {
		t.Fatalf("fetch: err = %v, muốn %q", err, netguard.UnsafeFormatMessage)
	}
	// Thông tin dựng tay cũng không lách được.
	info := LinkInfo{Title: "clip", Duration: mediaNumber(120), Formats: []LinkFormat{{URL: "rtmp://127.0.0.1:1935/v", Protocol: "rtmp"}}}
	if _, _, err = s.FetchLinkMedia(context.Background(), FetchLinkMediaOptions{URL: "https://www.bilibili.com/video/BV1fspq6cEty", From: "0:00", To: "0:05", Info: &info}, nil); !errors.As(err, &api) || api.Message != netguard.UnsafeFormatMessage {
		t.Fatalf("info dựng tay: err = %v", err)
	}
	for _, args := range runner.calls {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-f ") || strings.Contains(joined, "--download-sections") {
			t.Fatalf("đã chạy lời gọi tải dù chỉ có rtmp: %s", joined)
		}
	}
}

// TestMediaLinkHostedYtDlpArgsCarryProtocolFilter: lời gọi tải của editor phải
// mang bộ lọc protocol ở chế độ hosted, và không mang gì ở desktop.
func TestMediaLinkHostedYtDlpArgsCarryProtocolFilter(t *testing.T) {
	t.Cleanup(netguard.Close)
	s := newMediaTestService(t)
	s.YTDLP = "/managed/yt-dlp"
	runner := &recordingRunner{run: func([]string) (tools.Result, error) { return tools.Result{}, nil }}
	s.Runner = runner
	call := []string{"-f", "bv*+ba/b", "-S", "res:1080", "https://www.bilibili.com/video/BV1fspq6cEty"}

	if _, err := netguard.Install(netguard.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runMediaYTDLP(context.Background(), append([]string(nil), call...), time.Minute); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(runner.calls[len(runner.calls)-1], " "), "protocol") {
		t.Fatalf("desktop không được lọc protocol: %v", runner.calls[len(runner.calls)-1])
	}

	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.runMediaYTDLP(context.Background(), append([]string(nil), call...), time.Minute); err != nil {
		t.Fatal(err)
	}
	hosted := runner.calls[len(runner.calls)-1]
	joined := strings.Join(hosted, " ")
	if !strings.Contains(joined, netguard.YtDlpFormatFilter) {
		t.Fatalf("hosted thiếu bộ lọc protocol: %s", joined)
	}
	if !strings.Contains(joined, "--proxy "+guard.ProxyURL()) {
		t.Fatalf("hosted thiếu --proxy: %s", joined)
	}
	if !strings.Contains(joined, "bv*"+netguard.YtDlpFormatFilter+"+ba"+netguard.YtDlpFormatFilter) {
		t.Fatalf("bộ lọc phải gắn vào từng nhánh format: %s", joined)
	}
}
