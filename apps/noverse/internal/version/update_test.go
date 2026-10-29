package version

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReleaseURL(t *testing.T) {
	cases := map[string]string{
		"":       "https://api.github.com/repos/datnt1998/noverse/releases/latest",
		"latest": "https://api.github.com/repos/datnt1998/noverse/releases/latest",
		"1.2.3":  "https://api.github.com/repos/datnt1998/noverse/releases/tags/v1.2.3",
		"v1.2.3": "https://api.github.com/repos/datnt1998/noverse/releases/tags/v1.2.3",
	}
	for target, want := range cases {
		if got := releaseURL("datnt1998/noverse", target); got != want {
			t.Fatalf("releaseURL(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestSelectAsset(t *testing.T) {
	suffix, err := assetSuffix()
	if err != nil {
		t.Skip(err)
	}
	rel := &release{
		TagName: "v1.2.3",
		Assets: []releaseAsset{
			{Name: "ainovel-cli_v1.2.3_Windows_x86_64.zip", BrowserDownloadURL: "wrong"},
			{Name: "ainovel-cli_v1.2.3" + suffix, BrowserDownloadURL: "right"},
		},
	}
	asset, err := selectAsset(rel, "ainovel-cli")
	if err != nil {
		t.Fatalf("selectAsset: %v", err)
	}
	if asset.BrowserDownloadURL != "right" {
		t.Fatalf("asset = %+v", asset)
	}
}

func TestSelectChecksumAsset(t *testing.T) {
	rel := &release{TagName: "v1.2.3", Assets: []releaseAsset{
		{Name: "ainovel-cli_checksums.txt", BrowserDownloadURL: "checksum"},
	}}
	asset, err := selectChecksumAsset(rel, "ainovel-cli")
	if err != nil {
		t.Fatalf("selectChecksumAsset: %v", err)
	}
	if asset.BrowserDownloadURL != "checksum" {
		t.Fatalf("asset = %+v", asset)
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "ainovel-cli_1.2.3_Linux_x86_64.tar.gz")
	content := []byte("release archive")
	if err := os.WriteFile(archive, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	checksums := filepath.Join(dir, "checksums.txt")
	line := fmt.Sprintf("%x  %s\n", sum, filepath.Base(archive))
	if err := os.WriteFile(checksums, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, checksums, filepath.Base(archive)); err != nil {
		t.Fatalf("verifyChecksum: %v", err)
	}
	if err := os.WriteFile(archive, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(archive, checksums, filepath.Base(archive)); err == nil {
		t.Fatal("tampered archive should fail checksum verification")
	}
}

func TestDownloadRejectsAssetSizeMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("abc"))
	}))
	defer server.Close()
	dst := filepath.Join(t.TempDir(), "asset.tar.gz")
	if err := download(context.Background(), server.Client(), server.URL, dst, 4); err == nil {
		t.Fatal("download with mismatched GitHub asset size should fail")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "ainovel-cli")
	src := filepath.Join(dir, "new")
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := replaceExecutable(dst, src)
	if err != nil {
		t.Fatalf("replaceExecutable: %v", err)
	}
	realDst, err := filepath.EvalSymlinks(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got != realDst {
		t.Fatalf("path = %q, want %q", got, realDst)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q", data)
	}
	// 权限保持断言只在有 POSIX 权限位语义的平台上有意义：Windows 把一切上报为
	// 0666/0444、执行位永不出现（可执行性来自 .exe 扩展名），此断言在该平台恒假。
	// 替换/回滚/备份清理断言与平台相关（Windows rename 语义不同），必须继续运行。
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %v", info.Mode().Perm())
		}
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed, err=%v", err)
	}
}

type githubRedirectTransport struct {
	base http.RoundTripper
	to   string
}

// RoundTrip 把 api.github.com 的请求改写到 httptest 服务器，让 fetchRelease
// 在测试里走完整 URL 组装逻辑而不出网。
func (t *githubRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = t.to
	return t.base.RoundTrip(clone)
}

// stubLatestClient 返回把 release API 指向 server 的 http.Client。
func stubLatestClient(server *httptest.Server) *http.Client {
	return &http.Client{Transport: &githubRedirectTransport{
		base: server.Client().Transport,
		to:   strings.TrimPrefix(server.URL, "http://"),
	}}
}

func TestCheckLatestReportsUpdateAndCaches(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.3.0","assets":[]}`))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	opts := CheckOptions{CurrentVersion: "v1.2.0", Client: stubLatestClient(server), CacheDir: cacheDir}

	res, err := CheckLatest(context.Background(), opts)
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if res == nil || res.Latest != "v1.3.0" || !res.HasUpdate {
		t.Fatalf("CheckLatest = %+v, want latest v1.3.0 with update", res)
	}

	// 24h 缓存窗口内第二次调用不联网，仍能判定有新版本。
	res, err = CheckLatest(context.Background(), opts)
	if err != nil {
		t.Fatalf("cached CheckLatest: %v", err)
	}
	if hits != 1 {
		t.Fatalf("cache hit should not hit network, hits=%d", hits)
	}
	if res == nil || res.Latest != "v1.3.0" || !res.HasUpdate {
		t.Fatalf("cached result = %+v", res)
	}
}

func TestCheckLatestNoUpdateWhenUpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.9","assets":[]}`))
	}))
	defer server.Close()

	// 数字段比较：v1.0.9 并不比 v1.0.10 新。
	res, err := CheckLatest(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.10", Client: stubLatestClient(server), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if res == nil || res.HasUpdate {
		t.Fatalf("CheckLatest = %+v, want known latest without update", res)
	}
	if res.Latest != "v1.0.9" {
		t.Fatalf("latest = %q", res.Latest)
	}
}

func TestCheckLatestSkipsDevAndNetworkErrors(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer server.Close()

	res, err := CheckLatest(context.Background(), CheckOptions{
		CurrentVersion: "dev", Client: stubLatestClient(server), CacheDir: t.TempDir(),
	})
	if err != nil || res != nil {
		t.Fatalf("dev CheckLatest = %+v, %v; want nil, nil", res, err)
	}
	if hits != 0 {
		t.Fatalf("dev build must not hit the network, hits=%d", hits)
	}

	// 网络失败静默按"无更新"返回，不冒错误。
	badClient := &http.Client{Transport: &githubRedirectTransport{base: http.DefaultTransport, to: "127.0.0.1:1"}}
	res, err = CheckLatest(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0", Client: badClient, CacheDir: t.TempDir(),
	})
	if err != nil || res != nil {
		t.Fatalf("offline CheckLatest = %+v, %v; want nil, nil", res, err)
	}
}

func TestCheckLatestStaleCacheRefetches(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	// 预置一条过期的缓存记录：25 小时前查到的旧版本，必须被重新拉取覆盖。
	old := latestCache{Latest: "v1.0.0", CheckedAt: time.Now().Add(-25 * time.Hour).Unix()}
	data, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(cacheDir, latestCacheFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := CheckLatest(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0", Client: stubLatestClient(server), CacheDir: cacheDir,
	})
	if err != nil || res == nil || res.Latest != "v9.9.9" {
		t.Fatalf("stale-cache CheckLatest = %+v, %v", res, err)
	}
	if hits != 1 {
		t.Fatalf("stale cache must refetch, hits=%d", hits)
	}
}
