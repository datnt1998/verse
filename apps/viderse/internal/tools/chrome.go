// Chọn Chrome/Chromium cho bước measure và export headless (launcher sở hữu
// trình duyệt đã mở; module này tìm, cài và probe binary). Thứ tự: đường dẫn
// người dùng, rồi Chrome/Chromium có sẵn trên máy, rồi Chrome for Testing app
// quản dưới bin/chrome. Một trang headless thật chứng minh năng lực giải mã
// audio. Probe dùng profile bỏ đi và không bao giờ đọc profile hay cookie
// của trình duyệt thật.
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const chromeToolID = "chrome"

// ChromeSettingKey là key PathStore của chrome.
const ChromeSettingKey = "tools:chrome"

// ChromeCaps là năng lực audio/decode mà trang probe chứng minh.
type ChromeCaps struct {
	OfflineAudioContext bool `json:"offlineAudioContext"`
	AAC                 bool `json:"aac"`
	Opus                bool `json:"opus"`
}

// ChromeSource đánh dấu nguồn của binary đã chọn (alias ToolSource để giữ
// tương thích tên cũ: configured/detected/managed).
type ChromeSource = ToolSource

const (
	ChromeConfigured = SourceConfigured
	ChromeDetected   = SourceSystem
	ChromeManaged    = SourceManaged
)

// ChromeState là trạng thái của Chrome đã chọn.
type ChromeState string

const (
	ChromeReady        ChromeState = "ready"
	ChromeMissing      ChromeState = "missing"
	ChromeIncompatible ChromeState = "incompatible"
	ChromeBroken       ChromeState = "broken"
)

// ChromeStatus tóm tắt trạng thái cho Cài đặt.
type ChromeStatus struct {
	State    ChromeState
	Source   ToolSource
	Path     string
	Version  string
	Caps     ChromeCaps
	Detected []struct {
		Path    string
		Version string
	}
	Error string
}

// Chrome quản lý binary Chrome/Chromium và probe năng lực headless.
type Chrome struct {
	d              Deps
	verifiedProbe  *FileVerificationCache[chromeProbe]
	verifiedExport *FileVerificationCache[exportProbe]
}

type chromeProbe struct {
	Path    string
	Version string
	Caps    *ChromeCaps
	Err     string
}

type exportProbe struct {
	Path    string
	GPU     bool
	Encoder bool
	Err     string
}

var chromeVersionRe = regexp.MustCompile(`(\d+\.\d+\.\d+(?:\.\d+)?)`)

// ParseChromeVersion đọc banner `Google Chrome X.Y.Z.W`.
func ParseChromeVersion(stdout string) string {
	if m := chromeVersionRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	return ""
}

// Candidates liệt kê Chrome/Chromium hệ thống: PATH + chỗ cài chuẩn.
func (c *Chrome) Candidates() []string {
	var out []string
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if hit := FindOnPath(name, c.d.pathEnv()); hit != "" {
			out = append(out, hit)
		}
	}
	if c.d.SystemDirs != nil {
		// Danh sách chuẩn đã được inject (test cô lập): chỉ tìm trong đó, không
		// dùng chỗ cài thật của máy.
		for _, dir := range c.d.SystemDirs {
			for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "chrome"} {
				out = append(out, filepath.Join(dir, name))
			}
		}
	} else {
		switch runtime.GOOS {
		case "darwin":
			out = append(out,
				"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				"/Applications/Chromium.app/Contents/MacOS/Chromium",
				filepath.Join(c.d.home(), "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"))
		case "linux":
			for _, dir := range c.d.standardDirs() {
				out = append(out, filepath.Join(dir, "google-chrome"), filepath.Join(dir, "chromium"), filepath.Join(dir, "chromium-browser"))
			}
			out = append(out, "/snap/bin/chromium")
		case "windows":
			for _, env := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)", "LOCALAPPDATA"} {
				if base := os.Getenv(env); base != "" {
					out = append(out, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
				}
			}
		}
	}
	var existing []string
	for _, p := range dedupeStrings(out) {
		if fileExists(p) {
			existing = append(existing, p)
		}
	}
	return existing
}

// ConfiguredPath trả Chrome do người dùng chọn; "" khi không có.
func (c *Chrome) ConfiguredPath() string { return c.d.configuredPath(ChromeSettingKey) }

// SetConfiguredPath ghi/xoá Chrome do người dùng chọn.
func (c *Chrome) SetConfiguredPath(exe string) error {
	return c.d.SetConfiguredPath(ChromeSettingKey, exe)
}

// managedChromeExecutable là launcher Mach-O trong bundle Chrome for Testing,
// tương đối với thư mục trên cùng của archive đã bung.
const managedChromeExecutable = "Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"

// chromeExecutableIn trả executable trong một bản cài đã bung (dir = thư mục
// phiên bản hoặc staging); "" khi vắng.
func chromeExecutableIn(dir string) string {
	key := CurrentPlatformKey()
	assets := ChromePin().Assets[key]
	if len(assets) == 0 {
		return ""
	}
	// Tên asset ghim cũng là thư mục trên cùng của archive (chrome-mac-arm64).
	exe := filepath.Join(dir, assets[0].Name, filepath.FromSlash(managedChromeExecutable))
	if fileExists(exe) {
		return exe
	}
	return ""
}

// ManagedPath trả Chrome for Testing do app cài; "" khi chưa có bản active.
func (c *Chrome) ManagedPath() string {
	dir := ActiveVersionDir(c.d.BaseDir, chromeToolID)
	if dir == "" {
		return ""
	}
	return chromeExecutableIn(dir)
}

// ChromeResolution là binary app sẽ dùng và nguồn của nó.
type ChromeResolution struct {
	Path   string
	Source ToolSource
}

// Resolve trả Chrome app sẽ dùng: đường dẫn đã cấu hình, rồi hệ thống, rồi
// bản app quản.
func (c *Chrome) Resolve() *ChromeResolution {
	if configured := c.ConfiguredPath(); configured != "" {
		return &ChromeResolution{Path: configured, Source: ChromeConfigured}
	}
	if detected := c.Candidates(); len(detected) > 0 {
		return &ChromeResolution{Path: detected[0], Source: ChromeDetected}
	}
	if managed := c.ManagedPath(); managed != "" {
		return &ChromeResolution{Path: managed, Source: ChromeManaged}
	}
	return nil
}

// InstallSpec trả pipeline cài Chrome for Testing ghim vào bin/chrome.
func (c *Chrome) InstallSpec() InstallSpec {
	return InstallSpec{
		ToolID: chromeToolID,
		Pin:    ChromePin(),
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			exe := chromeExecutableIn(staging)
			if exe == "" {
				return "", fmt.Errorf("Chrome tải về thiếu executable")
			}
			res, err := c.d.Runner.Run(ctx, exe, []string{"--version"}, Spec{Timeout: 30 * time.Second})
			if err != nil {
				return "", fmt.Errorf("Chrome tải về không chạy được: %w", err)
			}
			version := ParseChromeVersion(res.Stdout)
			if version == "" {
				return "", fmt.Errorf("Chrome tải về không chạy được")
			}
			return version, nil
		},
	}
}

// probeHTML là trang probe: đổ `PROBE:ctx:<bool>,aac:<bool>,opus:<bool>` vào DOM.
const probeHTML = `<!doctype html><meta charset=utf-8><body><script>
(async () => {
  const out = [];
  try { out.push("ctx:" + (typeof OfflineAudioContext === "function")); } catch (e) { out.push("ctx:false"); }
  try {
    if (typeof AudioDecoder === "undefined") out.push("aac:false", "opus:false");
    else {
      const a = await AudioDecoder.isConfigSupported({ codec: "mp4a.40.2", sampleRate: 48000, numberOfChannels: 2 });
      const o = await AudioDecoder.isConfigSupported({ codec: "opus", sampleRate: 48000, numberOfChannels: 2 });
      out.push("aac:" + !!a.supported, "opus:" + !!o.supported);
    }
  } catch (e) { out.push("aac:false", "opus:false"); }
  document.body.textContent = "PROBE:" + out.join(",");
})();
</script></body>`

var probeDomRe = regexp.MustCompile(`PROBE:ctx:(true|false),aac:(true|false),opus:(true|false)`)

// ParseProbeDom đọc marker PROBE trong DOM đổ ra.
func ParseProbeDom(dom string) *ChromeCaps {
	m := probeDomRe.FindStringSubmatch(dom)
	if m == nil {
		return nil
	}
	return &ChromeCaps{
		OfflineAudioContext: m[1] == "true",
		AAC:                 m[2] == "true",
		Opus:                m[3] == "true",
	}
}

// ChromeCapsOk đòi OfflineAudioContext cùng ít nhất một decoder audio.
func chromeCapsOk(caps ChromeCaps) bool {
	return caps.OfflineAudioContext && (caps.AAC || caps.Opus)
}

// ChromeCapsOk là bản export của kiểm năng lực.
func ChromeCapsOk(caps ChromeCaps) bool { return chromeCapsOk(caps) }

// staleProbeAge xa hơn một probe (30s phiên bản + 20s trần trang), nên probe
// đang chạy không bao giờ bị quét.
const staleProbeAge = 10 * time.Minute

// probeExitGrace là thời gian Chrome được thoát sau SIGTERM trước khi nhóm
// của nó bị giết.
const probeExitGrace = 2 * time.Second

// sweepStaleProbeDirs xoá profile probe mà owner chết trước khi `finally`
// chạy (server bị giết giữa probe; Chrome chạy nhóm riêng và sống sót). Mỗi
// profile rò ~75 MB nên không được tích luỹ qua các lần khởi động lại.
func sweepStaleProbeDirs() {
	SweepStaleTempDirs(os.TempDir(), []string{ChromeProbePrefix}, staleProbeAge)
}

func (c *Chrome) probe(ctx context.Context, exe string) chromeProbe {
	if c.verifiedProbe == nil {
		return c.runProbe(ctx, exe)
	}
	p, err := c.verifiedProbe.Check(exe, func(ctx context.Context) (chromeProbe, error) {
		return c.runProbe(ctx, exe), nil
	}, ctx)
	if err != nil {
		return chromeProbe{Path: exe, Err: err.Error()}
	}
	return p
}

func (c *Chrome) runProbe(ctx context.Context, exe string) chromeProbe {
	res, err := c.d.Runner.Run(ctx, exe, []string{"--version"}, Spec{Timeout: 30 * time.Second})
	if err != nil {
		return chromeProbe{Path: exe, Err: err.Error()}
	}
	version := ParseChromeVersion(res.Stdout)
	if version == "" {
		return chromeProbe{Path: exe, Err: "phiên bản không đọc được"}
	}
	sweepStaleProbeDirs()
	work, err := os.MkdirTemp("", ChromeProbePrefix)
	if err != nil {
		return chromeProbe{Path: exe, Err: err.Error()}
	}
	defer os.RemoveAll(work)
	if err := os.WriteFile(filepath.Join(work, "probe.html"), []byte(probeHTML), 0o644); err != nil {
		return chromeProbe{Path: exe, Err: err.Error()}
	}
	// Chrome headless in DOM trong ~1s nhưng cứ chạy mãi, nên nhóm bị giết
	// ngay khi marker tới. Cửa sổ mà server chết giữa probe có thể để Chrome
	// mồ côi (nó dẫn nhóm riêng) co về khoảng một giây.
	domCtx, markerSeen := context.WithCancel(context.Background())
	defer markerSeen()
	var domOut strings.Builder
	_, runErr := c.d.Runner.Run(domCtx, exe, []string{
		"--headless=new",
		"--disable-gpu",
		"--mute-audio",
		"--no-first-run",
		"--disable-extensions",
		"--disable-background-networking",
		"--user-data-dir=" + filepath.Join(work, "profile"),
		"--virtual-time-budget=3000",
		"--dump-dom",
		"file://" + filepath.Join(work, "probe.html"),
	}, Spec{
		Timeout: 20 * time.Second,
		// --dump-dom đẩy DOM một khối: gom theo chunk thô qua Log thay vì đợi
		// đủ dòng; Log được tuần tự hoá nên dom/marker không bị tách.
		Log: &domOutWriter{dom: &domOut, seen: markerSeen},
	})
	caps := ParseProbeDom(domOut.String())
	if caps == nil {
		if runErr != nil {
			return chromeProbe{Path: exe, Err: runErr.Error()}
		}
		return chromeProbe{Path: exe, Err: "headless probe không sinh marker PROBE"}
	}
	return chromeProbe{Path: exe, Version: version, Caps: caps}
}

// domOutWriter gom stdout thô của trang probe và huỷ ctx ngay khi marker
// đầy đủ — Chrome headless cứ chạy mãi nên phải chủ động dừng sớm.
type domOutWriter struct {
	dom  *strings.Builder
	seen context.CancelFunc
}

func (w *domOutWriter) Write(p []byte) (int, error) {
	w.dom.Write(p)
	if ParseProbeDom(w.dom.String()) != nil {
		w.seen()
	}
	return len(p), nil
}

// Status báo trên binary mà export/measure sẽ thật sự mở: probe một ứng viên
// thân thiện hơn sẽ hứa hẹn trình duyệt mà export không dùng được.
func (c *Chrome) Status(ctx context.Context) ChromeStatus {
	resolved := c.Resolve()
	if resolved == nil {
		return ChromeStatus{State: ChromeMissing}
	}
	probe := c.probe(ctx, resolved.Path)
	if probe.Err != "" || probe.Caps == nil {
		return ChromeStatus{State: ChromeBroken, Path: resolved.Path, Error: probe.Err}
	}
	if !chromeCapsOk(*probe.Caps) {
		return ChromeStatus{State: ChromeIncompatible, Source: resolved.Source,
			Path: probe.Path, Version: probe.Version, Caps: *probe.Caps}
	}
	return ChromeStatus{State: ChromeReady, Source: resolved.Source,
		Path: probe.Path, Version: probe.Version, Caps: *probe.Caps}
}

/* ------------------------- năng lực export GPU ------------------------- */

// ChromeExportState là trạng thái probe GPU cho export headless.
type ChromeExportState string

const (
	ExportReady       ChromeExportState = "ready"
	ExportUnavailable ChromeExportState = "unavailable"
	ExportMissing     ChromeExportState = "missing"
	ExportBroken      ChromeExportState = "broken"
)

// ChromeExportCaps là kết quả kiểm năng lực WebGPU + encode H.264.
type ChromeExportCaps struct {
	State   ChromeExportState
	Path    string
	GPU     bool
	Encoder bool
	Error   string
}

// exportProbeExpression là script JS trả {gpu, encoder} — adapter WebGPU và
// đúng cấu hình H.264 1080p30 mà bước export dựng.
const exportProbeExpression = `(async () => {
  let gpu = false;
  try { gpu = !!(await navigator.gpu?.requestAdapter()); } catch {}
  let encoder = false;
  try {
    encoder = !!(await VideoEncoder.isConfigSupported({ codec: "avc1.640028", width: 1920, height: 1080, framerate: 30 })).supported;
  } catch {}
  return { gpu, encoder };
})()`

// cdpMessage là một khung DevTools qua --remote-debugging-pipe.
type cdpMessage struct {
	ID        int             `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// evaluateInHeadlessPage mở url trong Chrome headless và trả giá trị await
// của expression, nói DevTools qua --remote-debugging-pipe (fd 3/4) nên không
// mở cổng nào. Chrome dẫn nhóm process riêng, bị giết khi có câu trả lời,
// lỗi, hoặc hết timeout.
func (c *Chrome) evaluateInHeadlessPage(ctx context.Context, exe string, args []string, url, expression string, timeout time.Duration) (json.RawMessage, error) {
	// fd 3 = parent ghi → chrome đọc; fd 4 = chrome ghi → parent đọc.
	toChromeR, toChromeW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	fromChromeR, fromChromeW, err := os.Pipe()
	if err != nil {
		toChromeR.Close()
		toChromeW.Close()
		return nil, err
	}
	defer toChromeR.Close()
	defer fromChromeW.Close()

	cmd := exec.Command(exe, append(append([]string{}, args...), "--remote-debugging-pipe", "about:blank")...)
	makeDetached(cmd)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.ExtraFiles = []*os.File{toChromeR, fromChromeW}

	spawn := c.d.CDPSpawn
	if spawn == nil {
		spawn = func(cmd *exec.Cmd) (int, error) {
			if err := cmd.Start(); err != nil {
				return 0, err
			}
			return cmd.Process.Pid, nil
		}
	}
	pgid, err := spawn(cmd)
	if err != nil {
		toChromeW.Close()
		fromChromeR.Close()
		return nil, err
	}
	toChromeR.Close()   // đầu đọc thuộc con
	fromChromeW.Close() // đầu ghi thuộc con
	// Ghi nhóm vào sổ (khi bootstrap cấp) để server chết giữa probe không để
	// Chrome headless mồ côi: lần chạy sau dọn qua ReapOrphaned.
	if c.d.Registry != nil {
		c.d.Registry.Track(pgid, exe, "chrome-cdp")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Đọc khung phân cách NUL từ chrome.
	replies := map[int]chan cdpMessage{}
	var events []func(cdpMessage)
	var mu sync.Mutex
	readErr := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(fromChromeR)
		for {
			frame, err := reader.ReadBytes(0)
			if err != nil {
				readErr <- err
				return
			}
			frame = frame[:len(frame)-1]
			var msg cdpMessage
			if json.Unmarshal(frame, &msg) != nil {
				continue
			}
			mu.Lock()
			if msg.ID != 0 {
				if ch, ok := replies[msg.ID]; ok {
					delete(replies, msg.ID)
					ch <- msg
				}
			} else {
				for _, l := range events {
					l(msg)
				}
			}
			mu.Unlock()
		}
	}()

	var nextID int
	send := func(method string, params map[string]any, sessionID string) (json.RawMessage, error) {
		mu.Lock()
		nextID++
		id := nextID
		ch := make(chan cdpMessage, 1)
		replies[id] = ch
		mu.Unlock()
		msg := map[string]any{"id": id, "method": method, "params": params}
		if sessionID != "" {
			msg["sessionId"] = sessionID
		}
		data, _ := json.Marshal(msg)
		data = append(data, 0)
		if _, err := toChromeW.Write(data); err != nil {
			return nil, err
		}
		select {
		case reply := <-ch:
			if reply.Error != nil {
				return nil, fmt.Errorf("%s: %s", method, reply.Error.Message)
			}
			return reply.Result, nil
		case err := <-readErr:
			return nil, fmt.Errorf("Chrome đóng pipe: %w", err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Dọn khi xong: giết cả nhóm và chờ nó mất để caller xoá được profile.
	defer func() {
		_, _ = TerminateGroup(context.Background(), GroupTarget{PGID: pgid}, TerminateOptions{Grace: probeExitGrace})
		toChromeW.Close()
		fromChromeR.Close()
		_ = cmd.Wait()
		if c.d.Registry != nil {
			c.d.Registry.UntrackForget(pgid)
		}
	}()

	var target struct {
		TargetID string `json:"targetId"`
	}
	raw, err := send("Target.createTarget", map[string]any{"url": "about:blank"}, "")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, err
	}
	var attach struct {
		SessionID string `json:"sessionId"`
	}
	raw, err = send("Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true}, "")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &attach); err != nil {
		return nil, err
	}
	session := attach.SessionID
	if _, err := send("Page.enable", map[string]any{}, session); err != nil {
		return nil, err
	}
	loaded := make(chan struct{}, 1)
	mu.Lock()
	events = append(events, func(m cdpMessage) {
		if m.SessionID == session && m.Method == "Page.loadEventFired" {
			select {
			case loaded <- struct{}{}:
			default:
			}
		}
	})
	mu.Unlock()
	if _, err := send("Page.navigate", map[string]any{"url": url}, session); err != nil {
		return nil, err
	}
	select {
	case <-loaded:
	case err := <-readErr:
		return nil, fmt.Errorf("Chrome đóng pipe: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	raw, err = send("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
	}, session)
	if err != nil {
		return nil, err
	}
	var evaluated struct {
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
		Result *struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &evaluated); err != nil {
		return nil, err
	}
	if evaluated.ExceptionDetails != nil {
		return nil, fmt.Errorf("script trang ném lỗi: %s", evaluated.ExceptionDetails.Text)
	}
	if evaluated.Result == nil {
		return nil, errors.New("evaluate không trả kết quả")
	}
	return evaluated.Result.Value, nil
}

func parseExportProbeValue(raw json.RawMessage) (gpu, encoder bool, ok bool) {
	var v struct {
		GPU     bool `json:"gpu"`
		Encoder bool `json:"encoder"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return false, false, false
	}
	return v.GPU, v.Encoder, true
}

func (c *Chrome) runExportProbe(ctx context.Context, exe string) exportProbe {
	sweepStaleProbeDirs()
	work, err := os.MkdirTemp("", ChromeProbePrefix)
	if err != nil {
		return exportProbe{Path: exe, Err: err.Error()}
	}
	defer os.RemoveAll(work)
	// Trang secure (file://) là điều kiện của WebGPU.
	if err := os.WriteFile(filepath.Join(work, "probe.html"), []byte("<!doctype html><meta charset=utf-8><title>probe</title>"), 0o644); err != nil {
		return exportProbe{Path: exe, Err: err.Error()}
	}
	raw, err := c.evaluateInHeadlessPage(ctx, exe, []string{
		"--headless=new",
		"--mute-audio",
		"--no-first-run",
		"--disable-extensions",
		"--disable-background-networking",
		"--user-data-dir=" + filepath.Join(work, "profile"),
	}, "file://"+filepath.Join(work, "probe.html"), exportProbeExpression, 25*time.Second)
	if err != nil {
		return exportProbe{Path: exe, Err: err.Error()}
	}
	gpu, encoder, ok := parseExportProbeValue(raw)
	if !ok {
		return exportProbe{Path: exe, Err: "export probe không trả năng lực"}
	}
	return exportProbe{Path: exe, GPU: gpu, Encoder: encoder}
}

// ExportCaps là năng lực đường-GPU cho bước export headless: resolve binary
// mà launcher sẽ dùng rồi chứng minh WebGPU + encode H.264 1080p30 headless.
// Kết quả cache theo identity binary như Status.
func (c *Chrome) ExportCaps(ctx context.Context) ChromeExportCaps {
	resolved := c.Resolve()
	if resolved == nil {
		return ChromeExportCaps{State: ExportMissing}
	}
	exe := resolved.Path
	var probe exportProbe
	if c.verifiedExport != nil {
		p, err := c.verifiedExport.Check(exe, func(ctx context.Context) (exportProbe, error) {
			return c.runExportProbe(ctx, exe), nil
		}, ctx)
		if err != nil {
			return ChromeExportCaps{State: ExportBroken, Path: exe, Error: err.Error()}
		}
		probe = p
	} else {
		probe = c.runExportProbe(ctx, exe)
	}
	if probe.Err != "" {
		return ChromeExportCaps{State: ExportBroken, Path: exe, Error: probe.Err}
	}
	if probe.GPU && probe.Encoder {
		return ChromeExportCaps{State: ExportReady, Path: exe}
	}
	return ChromeExportCaps{State: ExportUnavailable, Path: exe, GPU: probe.GPU, Encoder: probe.Encoder}
}
