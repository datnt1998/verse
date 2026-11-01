package tools

// Mọi lượt yt-dlp của app (bước tải của pipeline, mảng video nền, probe nền
// tảng) phải đi qua cổng ra khi máy chủ đang phục vụ người khác.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
)

// runnableYtDlp dựng một binary yt-dlp giả để Resolve() tìm thấy.
func runnableYtDlp(t *testing.T) (base string, y *YtDlp, runner *fakeRunner) {
	t.Helper()
	base = t.TempDir()
	managed := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	asset, _ := ytDlpPlatformAsset()
	if err := os.WriteFile(filepath.Join(managed, asset.exe), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner = &fakeRunner{execFn: func(string, []string, Spec) (Result, error) {
		return Result{Stdout: "ok"}, nil
	}}
	y = &YtDlp{d: Deps{Runner: runner, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	return base, y, runner
}

func TestYtDlpRunUsesGuardProxyWhenHosted(t *testing.T) {
	_, y, runner := runnableYtDlp(t)
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := y.Run(context.Background(), []string{"-J", "https://www.bilibili.com/video/BV1fspq6cEty"}, RunYtDlpOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(runner.ran) != 1 {
		t.Fatalf("số lượt chạy = %d", len(runner.ran))
	}
	argv := runner.ran[0]
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--proxy "+guard.ProxyURL()) {
		t.Fatalf("argv thiếu --proxy của cổng ra: %v", argv)
	}
	// --proxy phải đứng trước URL để yt-dlp nhận đúng.
	proxyAt, urlAt := -1, -1
	for i, arg := range argv {
		if arg == "--proxy" {
			proxyAt = i
		}
		if strings.HasPrefix(arg, "https://") {
			urlAt = i
		}
	}
	if proxyAt < 0 || urlAt < 0 || proxyAt > urlAt {
		t.Fatalf("--proxy phải trước URL: %v", argv)
	}
}

func TestYtDlpRunPassesGuardEnvironmentWhenHosted(t *testing.T) {
	_, y, _ := runnableYtDlp(t)
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	var seen Spec
	runner := &fakeRunner{execFn: func(_ string, _ []string, spec Spec) (Result, error) {
		seen = spec
		return Result{Stdout: "ok"}, nil
	}}
	y.d.Runner = runner
	if _, err := y.Run(context.Background(), []string{"-J", "https://cdn.example/v.mp4"}, RunYtDlpOptions{}); err != nil {
		t.Fatal(err)
	}
	if seen.EnvExact["http_proxy"] != guard.ProxyURL() {
		t.Fatalf("env con http_proxy = %q, muốn %q", seen.EnvExact["http_proxy"], guard.ProxyURL())
	}
	if seen.EnvExact["no_proxy"] != "" || seen.EnvExact["NO_PROXY"] != "" {
		t.Fatalf("env con giữ no_proxy thừa kế: %q/%q", seen.EnvExact["no_proxy"], seen.EnvExact["NO_PROXY"])
	}
}

func TestYtDlpRunLeavesDesktopUntouched(t *testing.T) {
	_, y, _ := runnableYtDlp(t)
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{}); err != nil {
		t.Fatal(err)
	}
	var seen Spec
	var argv []string
	runner := &fakeRunner{execFn: func(_ string, args []string, spec Spec) (Result, error) {
		argv, seen = args, spec
		return Result{Stdout: "ok"}, nil
	}}
	y.d.Runner = runner
	if _, err := y.Run(context.Background(), []string{"-J", "https://cdn.example/v.mp4"}, RunYtDlpOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(argv, " "), "--proxy") {
		t.Fatalf("desktop không được thêm --proxy: %v", argv)
	}
	if seen.EnvExact != nil || seen.Env != nil {
		t.Fatal("desktop phải thừa kế môi trường của app như cũ")
	}
}

func TestYtDlpRunRedactsProxyCredentialFromErrors(t *testing.T) {
	_, y, _ := runnableYtDlp(t)
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	y.d.Runner = &fakeRunner{execFn: func(_ string, _ []string, _ Spec) (Result, error) {
		return Result{}, &ProcessFailedError{
			Message: "yt-dlp exited 1", Code: 1,
			Stderr: "ERROR: unable to download video data: proxy " + guard.ProxyURL() + " refused",
		}
	}}
	_, err = y.Run(context.Background(), []string{"-J", "https://cdn.example/v.mp4"}, RunYtDlpOptions{})
	var yde *YtDlpError
	if !errors.As(err, &yde) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(yde.Message, guard.ProxyURL()) || strings.Contains(yde.StderrTail, guard.ProxyURL()) {
		t.Fatalf("credential proxy còn trong lỗi: %v", yde)
	}
}

// TestYtDlpRunRefusesNonProxiedProtocolsWhenHosted: video chỉ có format rtmp
// (nối trực tiếp, không qua proxy) phải bị từ chối trước khi có kết nối nào.
// Chạy yt-dlp thật với info JSON dựng sẵn, đích rtmp là một listener của test.
func TestYtDlpRunRefusesNonProxiedProtocolsWhenHosted(t *testing.T) {
	binary, err := exec.LookPath("yt-dlp")
	if err != nil {
		t.Skip("bỏ qua: không có binary yt-dlp trong môi trường test")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var connections atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			conn.Close()
		}
	}()
	rtmpBase := "rtmp://" + listener.Addr().String()
	info := fmt.Sprintf(`{"id":"t","title":"t","extractor":"generic","extractor_key":"Generic",
"webpage_url":"%[1]s/page","original_url":"%[1]s/page","formats":[
 {"format_id":"rtmp-v","url":"%[1]s/v","protocol":"rtmp","vcodec":"h264","acodec":"none","ext":"flv","width":640,"height":360},
 {"format_id":"rtmp-a","url":"%[1]s/a","protocol":"rtmp","vcodec":"none","acodec":"aac","ext":"flv","abr":128}]}`, rtmpBase)
	dir := t.TempDir()
	infoFile := filepath.Join(dir, "info.json")
	if err := os.WriteFile(infoFile, []byte(info), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{Hosted: true}); err != nil {
		t.Fatal(err)
	}
	y := &YtDlp{d: Deps{Runner: &ExecRunner{}, BaseDir: dir, PathEnv: filepath.Dir(binary)}}
	// --downloader rtmp:ffmpeg để chính ffmpeg nối rtmp: nếu bộ lọc protocol
	// không chạy, listener của test sẽ thấy kết nối (rtmpdump không có trên máy
	// này nên đường native không nối được gì).
	_, err = y.Run(context.Background(), []string{"--load-info-json", infoFile, "--downloader", "rtmp:ffmpeg", "-f", "bv*+ba/b", "-o", filepath.Join(dir, "out.%(ext)s")}, RunYtDlpOptions{Timeout: 2 * time.Minute})
	var yde *YtDlpError
	if !errors.As(err, &yde) {
		t.Fatalf("err = %v, muốn YtDlpError", err)
	}
	if got := connections.Load(); got != 0 {
		t.Fatalf("đích rtmp nhận %d kết nối — yt-dlp chọn format nối trực tiếp", got)
	}
	if !strings.Contains(yde.Message, "Requested format is not available") {
		t.Fatalf("yt-dlp phải từ chối format rtmp, được: %s", yde.Message)
	}
}

// TestYtDlpRunKeepsNonProxiedProtocolsInDesktop: desktop không đổi hành vi.
func TestYtDlpRunKeepsNonProxiedProtocolsInDesktop(t *testing.T) {
	_, y, _ := runnableYtDlp(t)
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{}); err != nil {
		t.Fatal(err)
	}
	var argv []string
	y.d.Runner = &fakeRunner{execFn: func(_ string, args []string, _ Spec) (Result, error) {
		argv = args
		return Result{Stdout: "ok"}, nil
	}}
	if _, err := y.Run(context.Background(), []string{"-f", "bv*+ba/b", "https://cdn.example/v.mp4"}, RunYtDlpOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(argv, " "), "protocol") {
		t.Fatalf("desktop không được lọc protocol: %v", argv)
	}
}
