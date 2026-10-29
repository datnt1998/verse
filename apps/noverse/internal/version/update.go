package version

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type UpdateOptions struct {
	Repo           string
	BinaryName     string
	TargetVersion  string
	CurrentVersion string
	Client         *http.Client
}

type UpdateResult struct {
	Version string
	Path    string
	Updated bool
}

type release struct {
	TagName     string         `json:"tag_name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("Windows không hỗ trợ tự cập nhật tại chỗ, hãy tải bản mới ở https://github.com/%s/releases", opts.Repo)
	}
	if opts.Repo == "" {
		return nil, fmt.Errorf("missing repo")
	}
	if opts.BinaryName == "" {
		return nil, fmt.Errorf("missing binary name")
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	rel, err := fetchRelease(ctx, client, opts.Repo, opts.TargetVersion)
	if err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("release thiếu tag_name")
	}
	if sameVersion(opts.CurrentVersion, rel.TagName) {
		return &UpdateResult{Version: rel.TagName, Path: executablePath()}, nil
	}
	asset, err := selectAsset(rel, opts.BinaryName)
	if err != nil {
		return nil, err
	}
	checksumAsset, err := selectChecksumAsset(rel, opts.BinaryName)
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "ainovel-cli-update-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, "pkg.tar.gz")
	if err := download(ctx, client, asset.BrowserDownloadURL, archivePath, asset.Size); err != nil {
		return nil, err
	}
	checksumPath := filepath.Join(tmp, "checksums.txt")
	if err := download(ctx, client, checksumAsset.BrowserDownloadURL, checksumPath, checksumAsset.Size); err != nil {
		return nil, err
	}
	if err := verifyChecksum(archivePath, checksumPath, asset.Name); err != nil {
		return nil, err
	}
	extracted, err := extractBinary(archivePath, tmp, opts.BinaryName)
	if err != nil {
		return nil, err
	}
	dst, err := replaceCurrentExecutable(extracted)
	if err != nil {
		return nil, err
	}
	return &UpdateResult{Version: rel.TagName, Path: dst, Updated: true}, nil
}

func fetchRelease(ctx context.Context, client *http.Client, repo, target string) (*release, error) {
	url := releaseURL(repo, target)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("query release: %s", resp.Status)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

func releaseURL(repo, target string) string {
	target = strings.TrimSpace(target)
	if target == "" || target == "latest" {
		return "https://api.github.com/repos/" + repo + "/releases/latest"
	}
	if !strings.HasPrefix(target, "v") {
		target = "v" + target
	}
	return "https://api.github.com/repos/" + repo + "/releases/tags/" + target
}

func selectAsset(rel *release, binaryName string) (releaseAsset, error) {
	suffix, err := assetSuffix()
	if err != nil {
		return releaseAsset{}, err
	}
	versions := []string{strings.TrimPrefix(rel.TagName, "v"), rel.TagName}
	for _, version := range versions {
		expected := binaryName + "_" + version + suffix
		for _, asset := range rel.Assets {
			if asset.Name == expected && asset.BrowserDownloadURL != "" {
				return asset, nil
			}
		}
	}
	return releaseAsset{}, fmt.Errorf("release %s không có gói cài chính xác cho nền tảng hiện tại %s_<version>%s", rel.TagName, binaryName, suffix)
}

func selectChecksumAsset(rel *release, binaryName string) (releaseAsset, error) {
	expected := binaryName + "_checksums.txt"
	for _, asset := range rel.Assets {
		if asset.Name == expected && asset.BrowserDownloadURL != "" {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("release %s thiếu tệp kiểm tra %s, từ chối tự cập nhật", rel.TagName, expected)
}

func assetSuffix() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	default:
		return "", fmt.Errorf("hệ điều hành không hỗ trợ %s", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("kiến trúc không hỗ trợ %s", runtime.GOARCH)
	}
	return "_" + osName + "_" + arch + ".tar.gz", nil
}

func download(ctx context.Context, client *http.Client, url, dst string, expectedSize int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download release asset: %s", resp.Status)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("download size mismatch: got %d bytes, expected %d", written, expectedSize)
	}
	return nil
}

// verifyChecksum 校验 GoReleaser 生成的 SHA256 清单。清单与安装包必须来自同一个
// release；缺项、格式错误或摘要不匹配均拒绝替换当前可执行文件。
func verifyChecksum(archivePath, checksumPath, assetName string) error {
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksum file: %w", err)
	}
	var expected string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("không tìm thấy %s trong danh sách checksum", assetName)
	}
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("định dạng SHA256 của %s không hợp lệ", assetName)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("kiểm tra SHA256 của %s thất bại: got %s, expected %s", assetName, actual, expected)
	}
	return nil
}

func extractBinary(archivePath, dstDir, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("read archive gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read archive tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != binaryName {
			continue
		}
		out := filepath.Join(dstDir, binaryName)
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", fmt.Errorf("extract binary: %w", err)
		}
		if _, err := io.Copy(w, tr); err != nil {
			_ = w.Close()
			return "", fmt.Errorf("extract binary: %w", err)
		}
		if err := w.Close(); err != nil {
			return "", fmt.Errorf("extract binary: %w", err)
		}
		return out, nil
	}
	return "", fmt.Errorf("không tìm thấy %s trong gói cài", binaryName)
}

func replaceCurrentExecutable(src string) (string, error) {
	return replaceExecutable(executablePath(), src)
}

func replaceExecutable(dst, src string) (string, error) {
	if dst == "" {
		return "", fmt.Errorf("không xác định được tệp thực thi hiện tại")
	}
	if real, err := filepath.EvalSymlinks(dst); err == nil {
		dst = real
	}
	perm := os.FileMode(0o755)
	if info, err := os.Stat(dst); err == nil {
		perm = info.Mode().Perm()
	}
	stage, err := stageExecutable(filepath.Dir(dst), filepath.Base(dst), src, perm)
	if err != nil {
		return "", err
	}
	stageInstalled := false
	defer func() {
		if !stageInstalled {
			_ = os.Remove(stage)
		}
	}()

	backup := dst + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(dst, backup); err != nil {
		return "", fmt.Errorf("backup current executable: %w", err)
	}
	if err := os.Rename(stage, dst); err != nil {
		_ = os.Rename(backup, dst)
		return "", fmt.Errorf("replace executable: %w", err)
	}
	stageInstalled = true
	if err := os.Remove(backup); err != nil {
		return "", fmt.Errorf("remove backup executable: %w", err)
	}
	return dst, nil
}

func stageExecutable(dir, base, src string, perm os.FileMode) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open new executable: %w", err)
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, base+".new-*")
	if err != nil {
		return "", fmt.Errorf("create staged executable: %w", err)
	}
	stage := out.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(stage)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("write staged executable: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close staged executable: %w", err)
	}
	if err := os.Chmod(stage, perm); err != nil {
		return "", fmt.Errorf("chmod staged executable: %w", err)
	}
	ok = true
	return stage, nil
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func sameVersion(a, b string) bool {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")
	return a != "" && b != "" && a == b
}

// ---------- 被动检查新版本 ----------

// CheckOptions 是 CheckLatest 的参数：Repo 默认 ReleaseRepo；CacheDir 是缓存
// 目录（应用统一用 ~/.ainovel，测试传临时目录；空则自行解析家目录）；
// Client 可注入 httptest 客户端；CacheTTL 默认 24 小时。
type CheckOptions struct {
	Repo           string
	CurrentVersion string
	Client         *http.Client
	CacheDir       string
	CacheTTL       time.Duration
}

// CheckResult 是被动检查的结果：Latest 为远端最新 tag；HasUpdate 仅在远端
// 版本严格高于当前版本时为真。LatestNotes/LatestDate 是最新 release 的正文
// markdown 原文与发布时间（GitHub published_at，RFC3339），无数据时为空。
type CheckResult struct {
	Latest      string
	HasUpdate   bool
	LatestNotes string
	LatestDate  string
}

// ReleaseNote 是 “Có gì mới” 的一条记录（/api/system › releases[]）。
type ReleaseNote struct {
	Version string `json:"version"`
	// Date 是 RFC3339：新版本取 GitHub published_at，当前版本取构建时间。
	Date string `json:"date"`
	// Notes 是 release 正文 markdown 原文，不裁剪不改写；无数据时为空串。
	Notes string `json:"notes"`
}

// Changelog 组装 /api/system 的 releases：有更新时先列新版本（带 release 说明），
// 再列当前版本；没有远端数据（离线 / 尚未检查完 / dev 构建）时只列当前版本，
// Notes 为空。currentDate 是当前构建时间（Info.Date），"unknown" 视为未知。
func Changelog(res CheckResult, current, currentDate string) []ReleaseNote {
	current = Normalize(current)
	out := make([]ReleaseNote, 0, 2)
	currentNotes := ""
	if res.Latest != "" && sameVersion(res.Latest, current) {
		currentNotes = res.LatestNotes
	}
	if res.Latest != "" && res.HasUpdate {
		out = append(out, ReleaseNote{Version: res.Latest, Date: res.LatestDate, Notes: res.LatestNotes})
	}
	if current != "" {
		date := strings.TrimSpace(currentDate)
		if date == "unknown" {
			date = ""
		}
		out = append(out, ReleaseNote{Version: current, Date: date, Notes: currentNotes})
	}
	return out
}

// latestCache 是磁盘缓存文件的内容：只缓存成功拿到的 tag、release 说明与检查
// 时间；hasUpdate 每次按当前版本现算，不受缓存影响。
type latestCache struct {
	Latest    string `json:"latest"`
	Notes     string `json:"notes,omitempty"`
	Date      string `json:"date,omitempty"`
	CheckedAt int64  `json:"checked_at"`
}

const latestCacheFile = "latest_release.json"

// CheckLatest 查询 ReleaseRepo 的最新 release 并与 CurrentVersion 比较。
// 24 小时内命中缓存不联网；网络错误静默返回 (nil, nil)；当前版本为 dev
// 时跳过（nil, nil）。
func CheckLatest(ctx context.Context, opts CheckOptions) (*CheckResult, error) {
	current := Normalize(opts.CurrentVersion)
	if current == "dev" || current == "" {
		return nil, nil
	}
	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		repo = ReleaseRepo
	}
	ttl := opts.CacheTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	latest := ""
	latestNotes, latestDate := "", ""
	cachePath := ""
	if dir := checkCacheDir(opts.CacheDir); dir != "" {
		cachePath = filepath.Join(dir, latestCacheFile)
	}
	if cached, ok := readLatestCache(cachePath, ttl); ok {
		latest, latestNotes, latestDate = cached.Latest, cached.Notes, cached.Date
	} else {
		rel, err := fetchRelease(ctx, client, repo, "latest")
		if err != nil || rel == nil || rel.TagName == "" {
			// 网络/解析失败静默按"无更新"处理，这是被动提示不是门禁。
			return nil, nil
		}
		latest, latestNotes, latestDate = rel.TagName, rel.Body, strings.TrimSpace(rel.PublishedAt)
		writeLatestCache(cachePath, latestCache{Latest: latest, Notes: latestNotes, Date: latestDate})
	}
	return &CheckResult{Latest: latest, HasUpdate: newerRelease(latest, current), LatestNotes: latestNotes, LatestDate: latestDate}, nil
}

// checkCacheDir 解析缓存目录：显式 CacheDir 优先，否则是应用状态目录 ~/.ainovel
// （与 bootstrap.DefaultConfigDir 同一位置；本包保持 stdlib-only 故自行解析）。
// 取不到家目录时返回空串，读写随后静默失败 = 不缓存。
func checkCacheDir(dir string) string {
	if dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".ainovel")
}

func readLatestCache(path string, ttl time.Duration) (latestCache, bool) {
	var cached latestCache
	if path == "" {
		return cached, false
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &cached) != nil {
		return latestCache{}, false
	}
	if cached.Latest == "" || time.Since(time.Unix(cached.CheckedAt, 0)) >= ttl {
		return latestCache{}, false
	}
	return cached, true
}

func writeLatestCache(path string, entry latestCache) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry.CheckedAt = time.Now().Unix()
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// newerRelease 比较 release tag 是否严格高于当前版本。tag 形如 v1.2.3；
// 无法解析为数字段时退化为"不同即更新"，保证提示不丢。
func newerRelease(latest, current string) bool {
	lNums, lOK := versionNums(latest)
	cNums, cOK := versionNums(current)
	if lOK && cOK {
		for i := range lNums {
			if lNums[i] != cNums[i] {
				return lNums[i] > cNums[i]
			}
		}
		return false
	}
	return !sameVersion(latest, current)
}

func versionNums(v string) ([3]int, bool) {
	var nums [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nums, false
	}
	parts := strings.SplitN(v, ".", 4)
	for i := range nums {
		if i >= len(parts) {
			break
		}
		// 预发布后缀（-rc1 等）按非数字处理，回退到字符串比较。
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return nums, false
		}
		nums[i] = n
	}
	return nums, true
}
