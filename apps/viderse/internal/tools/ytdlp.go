// yt-dlp: cài/cập nhật binary app-owned dưới <data>/bin (tải từ release
// GitHub chính thức, kiểm SHA-256, cài nguyên tử) và chạy nó với mặc định của
// app. yt-dlp tự cập nhật theo release nên nằm ngoài manifest ghim.
package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const ytDlpReleases = "https://github.com/yt-dlp/yt-dlp/releases"

// YtDlpErrorCode là mã lỗi ổn định mà UI giải thích cho người dùng.
type YtDlpErrorCode string

const (
	YtDlpLoginRequired YtDlpErrorCode = "login_required"
	YtDlpUnavailable   YtDlpErrorCode = "unavailable"
	YtDlpCookiesFailed YtDlpErrorCode = "cookies_failed"
	YtDlpNoSubtitles   YtDlpErrorCode = "no_subtitles"
	YtDlpRateLimited   YtDlpErrorCode = "rate_limited"
	YtDlpFailed        YtDlpErrorCode = "failed"
)

// ytDlpErrorCodes là danh sách đầy đủ mã lỗi (bản chuyển YT_DLP_ERROR_CODES).
var ytDlpErrorCodes = []YtDlpErrorCode{
	YtDlpLoginRequired, YtDlpUnavailable, YtDlpCookiesFailed,
	YtDlpNoSubtitles, YtDlpRateLimited, YtDlpFailed,
}

// IsYtDlpErrorCode đúng khi chuỗi là một mã đã biết.
func IsYtDlpErrorCode(v string) bool {
	for _, c := range ytDlpErrorCodes {
		if string(c) == v {
			return true
		}
	}
	return false
}

// YtDlpError mang mã ổn định và đuôi stderr cho việc phân loại thêm.
type YtDlpError struct {
	Message    string
	Code       YtDlpErrorCode
	StderrTail string
}

func (e *YtDlpError) Error() string { return e.Message }

// ErrorCode trả mã lỗi ổn định để gói khác phân loại qua interface
// { ErrorCode() string } mà không cần import gói tools.
func (e *YtDlpError) ErrorCode() string { return string(e.Code) }

var (
	ytDlpRateLimitRe = regexp.MustCompile(`(?i)request is (?:blocked by server \((?:412|401)\)|rejected by server \(352\))`)
	ytDlpLoginRe     = regexp.MustCompile(`(?i)sign in|not a bot|log ?in|members[- ]only|\bage\b`)
	ytDlpCookieRe    = regexp.MustCompile(`(?i)cookie|keyring|keychain|decrypt`)
	ytDlpUnavailRe   = regexp.MustCompile(`(?i)unavailable|private video|removed|terminated|not available|does not exist`)
	ytDlpExtractorRe = regexp.MustCompile(`^\[[^\]]+\]\s*[\w.-]+:\s*`)
)

// ClassifyYtDlpError map stderr của yt-dlp về dòng ERROR cuối và một mã ổn
// định mà UI giải thích. stderrTail rỗng → giữ toàn stderr.
func ClassifyYtDlpError(stderr, stderrTail string) *YtDlpError {
	if stderrTail == "" {
		stderrTail = stderr
	}
	var lines []string
	for _, l := range strings.Split(stderr, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	last := "yt-dlp failed"
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "ERROR:") {
			last = lines[i]
			break
		}
	}
	if last == "yt-dlp failed" && len(lines) > 0 {
		last = lines[len(lines)-1]
	}
	message := strings.TrimPrefix(last, "ERROR:")
	message = strings.TrimSpace(message)
	message = ytDlpExtractorRe.ReplaceAllString(message, "")

	// Khóa chống cào của Bilibili (HTTP 412/401, mã 352): địa chỉ bị bóp một
	// lúc nên thử lại chỉ kéo dài khoá.
	if ytDlpRateLimitRe.MatchString(message) {
		return &YtDlpError{Message: message, Code: YtDlpRateLimited, StderrTail: stderrTail}
	}
	// Kiểm login trước: thông báo bot-check của YouTube tự gợi ý
	// "--cookies-from-browser".
	if ytDlpLoginRe.MatchString(message) {
		return &YtDlpError{Message: message, Code: YtDlpLoginRequired, StderrTail: stderrTail}
	}
	if ytDlpCookieRe.MatchString(message) {
		return &YtDlpError{Message: message, Code: YtDlpCookiesFailed, StderrTail: stderrTail}
	}
	if ytDlpUnavailRe.MatchString(message) {
		return &YtDlpError{Message: message, Code: YtDlpUnavailable, StderrTail: stderrTail}
	}
	return &YtDlpError{Message: message, Code: YtDlpFailed, StderrTail: stderrTail}
}

// YtDlp quản lý binary yt-dlp và mọi lời gọi của nó.
type YtDlp struct {
	d        Deps
	verified *FileVerificationCache[string]

	mu            sync.Mutex
	installFlight *ytDlpInstallFlight
}

// WithRunner retains configured binaries and network options, but binds this
// invocation to the caller's tracked process group runner.
func (y *YtDlp) WithRunner(r Runner) *YtDlp {
	deps := y.d
	deps.Runner = r
	return &YtDlp{d: deps, verified: y.verified}
}

// ytDlpInstallFlight giữ một lượt cài/cập nhật đang chạy để caller đồng thời
// chia cùng kết quả.
type ytDlpInstallFlight struct {
	done   chan struct{}
	result YtDlpInstallResult
	err    error
	cancel context.CancelFunc
}

// ytDlpAsset là asset release chính thức của nền tảng. macOS dùng zip onedir:
// khởi động ~0.3s so với ~10s của bản one-file.
type ytDlpAsset struct {
	name string
	zip  bool
	exe  string
}

func ytDlpPlatformAsset() (ytDlpAsset, error) {
	switch runtime.GOOS {
	case "darwin":
		return ytDlpAsset{name: "yt-dlp_macos.zip", zip: true, exe: "yt-dlp_macos"}, nil
	case "linux":
		name := "yt-dlp_linux"
		if runtime.GOARCH == "arm64" {
			name = "yt-dlp_linux_aarch64"
		}
		return ytDlpAsset{name: name, exe: name}, nil
	case "windows":
		name := "yt-dlp.exe"
		if runtime.GOARCH == "arm64" {
			name = "yt-dlp_arm64.exe"
		}
		return ytDlpAsset{name: name, exe: name}, nil
	}
	return ytDlpAsset{}, fmt.Errorf("yt-dlp không có bản chính thức cho %s/%s", runtime.GOOS, runtime.GOARCH)
}

func (y *YtDlp) installDir() string { return filepath.Join(y.d.BaseDir, "bin", "yt-dlp") }
func (y *YtDlp) cacheDir() string   { return filepath.Join(y.d.BaseDir, "cache", "yt-dlp") }

func (y *YtDlp) exePath() string {
	asset, err := ytDlpPlatformAsset()
	if err != nil {
		return ""
	}
	return filepath.Join(y.installDir(), asset.exe)
}

// ConfiguredPath trả yt-dlp do người dùng chọn; "" khi không có.
func (y *YtDlp) ConfiguredPath() string { return y.d.configuredPath("tools:yt-dlp") }

// ManagedPath trả binary app quản khi đã cài; "" khi chưa.
func (y *YtDlp) ManagedPath() string {
	exe := y.exePath()
	if exe != "" && fileExists(exe) {
		return exe
	}
	return ""
}

// Candidates liệt kê yt-dlp hệ thống (PATH + chỗ cài chuẩn).
func (y *YtDlp) Candidates() []string {
	var out []string
	if onPath := FindOnPath("yt-dlp", y.d.pathEnv()); onPath != "" {
		out = append(out, onPath)
	}
	for _, dir := range y.d.standardDirs() {
		out = append(out, filepath.Join(dir, "yt-dlp"))
	}
	if runtime.GOOS != "windows" {
		out = append(out, filepath.Join(y.d.home(), ".local", "bin", "yt-dlp"))
	}
	return dedupeStrings(out)
}

// Resolve trả binary theo thứ tự cấu hình → hệ thống → app quản; "" khi
// không có.
// Resolve chọn binary theo thứ tự: người dùng chỉ định → bản app quản lý →
// bản hệ thống. Bản app đứng trước hệ thống để "Cập nhật" thực sự có hiệu
// lực (nếu không, bản homebrew cũ luôn thắng và nút cập nhật thành vô nghĩa).
func (y *YtDlp) Resolve() string {
	if c := y.ConfiguredPath(); c != "" {
		return c
	}
	if m := y.ManagedPath(); m != "" {
		return m
	}
	return FirstExecutable(y.Candidates())
}

// IsInstalled đúng khi một binary yt-dlp dùng được đang hiện diện.
func (y *YtDlp) IsInstalled() bool { return y.Resolve() != "" }

// Status đọc phiên bản của binary đang resolve; version rỗng khi binary đổi
// giữa chừng sẽ được kiểm lại ở lần gọi sau (cache theo identity tệp).
func (y *YtDlp) Status(ctx context.Context) (version, path string, installed bool) {
	exe := y.Resolve()
	if exe == "" {
		return "", "", false
	}
	v, err := y.readVersion(ctx, exe)
	if err != nil {
		return "", exe, true
	}
	return v, exe, true
}

// readVersion đọc `yt-dlp --version` (lần chạy đầu sau cài trên macOS có thể
// ~10s trong khi hệ thống quét binary mới).
func (y *YtDlp) readVersion(ctx context.Context, exe string) (string, error) {
	verify := func(ctx context.Context) (string, error) {
		res, err := y.d.Runner.Run(ctx, exe, []string{"--version"}, Spec{Timeout: 2 * time.Minute})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(res.Stdout), nil
	}
	if y.verified != nil && fileExists(exe) {
		return y.verified.Check(exe, verify, ctx)
	}
	return verify(ctx)
}

// LatestVersion đọc tag release mới nhất từ redirect /releases/latest của
// GitHub (không qua API nên không dính rate limit).
func (y *YtDlp) LatestVersion(ctx context.Context) (string, error) {
	client := y.d.Client
	if client == nil {
		client = http.DefaultClient
	}
	// Không đi theo redirect: chỉ cần header Location.
	noRedirect := *client
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, ytDlpReleases+"/latest", nil)
	if err != nil {
		return "", err
	}
	res, err := noRedirect.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	loc := res.Header.Get("Location")
	m := regexp.MustCompile(`/tag/([^/?#]+)$`).FindStringSubmatch(loc)
	if m == nil {
		return "", fmt.Errorf("không đọc được phiên bản yt-dlp mới nhất (HTTP %d)", res.StatusCode)
	}
	return m[1], nil
}

func (y *YtDlp) fetchOk(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	client := y.d.Client
	if client == nil {
		client = http.DefaultClient
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || res.Body == nil {
		res.Body.Close()
		cancel()
		return nil, fmt.Errorf("tải thất bại: HTTP %d %s", res.StatusCode, rawURL)
	}
	return &cancelOnClose{ReadCloser: res.Body, cancel: cancel}, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// download stream một URL xuống đĩa và trả SHA-256 của byte đã ghi. Huỷ giữa
// chừng trả lỗi và xoá tệp dở — không để lại mảnh tải.
func (y *YtDlp) download(ctx context.Context, rawURL, file string) (string, error) {
	src, err := y.fetchOk(ctx, rawURL)
	if err != nil {
		return "", err
	}
	defer src.Close()
	out, err := os.Create(file)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, h), ctxReader{ctx: ctx, r: src})
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(file)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(file)
		return "", closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ctxReader dừng đọc khi context huỷ (body HTTP có thể còn buffer).
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// YtDlpInstallResult tóm tắt một lượt cài/cập nhật; Previous là phiên bản
// trước đó ("" lần đầu cài), bằng Version khi đã mới nhất.
type YtDlpInstallResult struct {
	Version  string
	Previous string
	Path     string
}

func (y *YtDlp) install(ctx context.Context) (YtDlpInstallResult, error) {
	asset, err := ytDlpPlatformAsset()
	if err != nil {
		return YtDlpInstallResult{}, err
	}
	version, err := y.LatestVersion(ctx)
	if err != nil {
		return YtDlpInstallResult{}, err
	}
	previous := ""
	if v, _, ok := y.Status(ctx); ok {
		previous = v
		if v == version {
			return YtDlpInstallResult{Version: version, Previous: previous, Path: y.Resolve()}, nil
		}
	}

	base := fmt.Sprintf("%s/download/%s", ytDlpReleases, version)
	sumsBody, err := y.fetchOk(ctx, base+"/SHA2-256SUMS")
	if err != nil {
		return YtDlpInstallResult{}, err
	}
	sumsText, err := io.ReadAll(sumsBody)
	sumsBody.Close()
	if err != nil {
		return YtDlpInstallResult{}, err
	}
	expected := ParseSha256Sums(string(sumsText))[asset.name]
	if expected == "" {
		return YtDlpInstallResult{}, fmt.Errorf("không có checksum cho %s trong yt-dlp %s", asset.name, version)
	}

	binDir := filepath.Join(y.d.BaseDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return YtDlpInstallResult{}, err
	}
	staging := filepath.Join(binDir, fmt.Sprintf(".staging-%d", time.Now().UnixNano()))
	outDir := filepath.Join(staging, "yt-dlp")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return YtDlpInstallResult{}, err
	}
	defer os.RemoveAll(staging)

	archive := filepath.Join(staging, asset.name)
	actual, err := y.download(ctx, base+"/"+asset.name, archive)
	if err != nil {
		return YtDlpInstallResult{}, err
	}
	if !strings.EqualFold(actual, expected) {
		return YtDlpInstallResult{}, &ChecksumError{Asset: asset.name, Expected: expected, Actual: actual}
	}
	if asset.zip {
		if err := extractArchive(AssetZip, archive, outDir, ""); err != nil {
			return YtDlpInstallResult{}, err
		}
		_ = os.Chmod(filepath.Join(outDir, asset.exe), 0o755)
	} else {
		if err := os.Rename(archive, filepath.Join(outDir, asset.exe)); err != nil {
			return YtDlpInstallResult{}, err
		}
		if err := os.Chmod(filepath.Join(outDir, asset.exe), 0o755); err != nil {
			return YtDlpInstallResult{}, err
		}
	}
	installed, err := y.readVersion(ctx, filepath.Join(outDir, asset.exe))
	if err != nil {
		return YtDlpInstallResult{}, fmt.Errorf("yt-dlp tải về không chạy được: %w", err)
	}
	if err := os.RemoveAll(y.installDir()); err != nil {
		return YtDlpInstallResult{}, err
	}
	if err := os.Rename(outDir, y.installDir()); err != nil {
		return YtDlpInstallResult{}, err
	}
	return YtDlpInstallResult{Version: installed, Previous: previous, Path: filepath.Join(y.installDir(), asset.exe)}, nil
}

// Install cài yt-dlp hoặc cập nhật khi có release mới hơn. Caller đồng thời
// chia một lượt chạy; lượt chung không thuộc caller nào nên huỷ của một
// caller chỉ rút chờ của caller đó.
func (y *YtDlp) Install(ctx context.Context) (YtDlpInstallResult, error) {
	if err := ctx.Err(); err != nil {
		return YtDlpInstallResult{}, cancelledErr(ctx)
	}
	y.mu.Lock()
	if y.installFlight == nil {
		flightCtx, cancel := context.WithCancel(ctx)
		f := &ytDlpInstallFlight{done: make(chan struct{}), cancel: cancel}
		y.installFlight = f
		go func() {
			f.result, f.err = y.install(flightCtx)
			cancel()
			close(f.done)
			y.mu.Lock()
			y.installFlight = nil
			y.mu.Unlock()
		}()
	}
	flight := y.installFlight
	y.mu.Unlock()
	select {
	case <-flight.done:
		return flight.result, flight.err
	case <-ctx.Done():
		return YtDlpInstallResult{}, cancelledErr(ctx)
	}
}

// CancelInstall huỷ lượt cài yt-dlp đang chạy: tải dừng, thư mục staging bị
// xoá (defer RemoveAll), nên không có tệp dở nào thành "sẵn sàng". Lượt sau
// tải lại từ đầu — pipeline của yt-dlp không chia đoạn.
func (y *YtDlp) CancelInstall() {
	y.mu.Lock()
	flight := y.installFlight
	y.mu.Unlock()
	if flight != nil && flight.cancel != nil {
		flight.cancel()
	}
}

// RunYtDlpOptions tinh chỉnh các lời gọi cần huỷ/tiến độ (tải media).
type RunYtDlpOptions struct {
	// Timeout giết nhóm yt-dlp sau khoảng này; 0 → 180s.
	Timeout time.Duration
	// LogLimit là trần byte giữ lại của stdout+stderr; 0 → 256 MB.
	LogLimit int
	// OnLine nhận mỗi dòng tiến độ (template [download], dòng muxer…).
	OnLine func(line string, stream Stream)
	// Log nhận byte thô cả hai luồng.
	Log io.Writer
	// CookieFile, khi khác rỗng, thêm `--cookies <file>` (gói accounts cung cấp).
	CookieFile string
	// JSRuntime, khi khác rỗng, thêm `--js-runtimes <spec>` (ví dụ "node:/path").
	JSRuntime string
}

const (
	ytDlpDefaultTimeout  = 180 * time.Second
	ytDlpDefaultLogLimit = 256 * 1024 * 1024
)

// Run chạy yt-dlp với mặc định của app, tự cài khi binary chưa có. Trả stdout.
func (y *YtDlp) Run(ctx context.Context, args []string, opts RunYtDlpOptions) (string, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = ytDlpDefaultTimeout
	}
	exe := y.Resolve()
	if exe == "" {
		if _, err := y.Install(ctx); err != nil {
			if errors.Is(err, ErrCancelled) || ctx.Err() != nil {
				return "", cancelledErr(ctx)
			}
			return "", &YtDlpError{Message: err.Error(), Code: YtDlpFailed}
		}
		exe = y.Resolve()
		if exe == "" {
			return "", &YtDlpError{Message: "yt-dlp chưa cài được", Code: YtDlpFailed}
		}
	}
	argv := []string{"--ignore-config", "--no-progress", "--no-colors", "--cache-dir", y.cacheDir()}
	if opts.JSRuntime != "" {
		argv = append(argv, "--js-runtimes", opts.JSRuntime)
	}
	if opts.CookieFile != "" {
		argv = append(argv, "--cookies", opts.CookieFile)
	}
	argv = append(argv, args...)

	logLimit := opts.LogLimit
	if logLimit <= 0 {
		logLimit = ytDlpDefaultLogLimit
	}
	res, err := y.d.Runner.Run(ctx, exe, argv, Spec{
		Timeout:  timeout,
		OnLine:   opts.OnLine,
		Log:      opts.Log,
		LogLimit: logLimit,
	})
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return "", err
		}
		var pfe *ProcessFailedError
		if errors.As(err, &pfe) {
			if pfe.TimedOut {
				return "", &YtDlpError{
					Message:    fmt.Sprintf("yt-dlp hết %s timeout", timeout),
					Code:       YtDlpFailed,
					StderrTail: pfe.Stderr,
				}
			}
			return "", ClassifyYtDlpError(firstNonEmpty(pfe.Stderr, pfe.Message), pfe.Stderr)
		}
		return "", ClassifyYtDlpError(err.Error(), "")
	}
	return res.Stdout, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// YtDlpErrorCodes trả bản sao danh sách: dữ liệu ghim/bảng hằng là bất biến,
// caller không chỉnh được bản gốc trong process.
func YtDlpErrorCodes() []YtDlpErrorCode { return append([]YtDlpErrorCode(nil), ytDlpErrorCodes...) }
