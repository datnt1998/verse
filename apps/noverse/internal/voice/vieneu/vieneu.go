// Package vieneu là engine giọng đọc mặc định: máy chủ VieNeu-TTS chính thức
// (apps/openai_speech.py của https://github.com/pnnbao97/VieNeu-TTS) do ainovel
// quản lý — cài bằng git + uv vào ~/.ainovel/voice/vieneu, bật khi cần trên một
// cổng ngẫu nhiên, tắt sau 10 phút rảnh, tự bật lại khi chết.
//
// Giao thức với máy chủ là OpenAI Speech API (xem package openaispeech), nên phần
// nói chuyện HTTP không phụ thuộc VieNeu; gói này chỉ thêm phần quản tiến trình.
package vieneu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/apps/noverse/internal/voice/openaispeech"
)

const (
	// repoURL là kho mã nguồn máy chủ chính thức.
	repoURL = "https://github.com/pnnbao97/VieNeu-TTS"
	// pinnedTag là bản phát hành đã kiểm chứng với ainovel (uv.lock tương ứng).
	pinnedTag = "v3.8.3"
	// idleTimeout là thời gian rảnh trước khi tắt máy đọc để trả RAM.
	idleTimeout = 10 * time.Minute
	// readyTimeout là thời gian chờ nạp mô hình (lần đầu gồm cả tải ~600 MB).
	readyTimeout = 8 * time.Minute
	// refPrefix đánh dấu giọng nạp tạm từ clip mẫu của dự án.
	refPrefix = "ref-"
)

func init() {
	voice.Register("vieneu", newEngine)
}

// Engine quản lý tiến trình máy chủ VieNeu và nói chuyện với nó qua HTTP.
type Engine struct {
	cfg voice.Config
	dir string

	// boot là lượt bật máy đọc đang chạy nền. Người gọi chỉ *chờ* lượt đó, không
	// sở hữu nó: một màn hình huỷ yêu cầu không được giết lượt bật dùng chung —
	// nếu không, mỗi lần tải lại trang lại nạp mô hình từ đầu và không bao giờ xong.
	boot *startCall

	mu        sync.Mutex
	proc      *proc
	starting  bool
	startErr  error
	closed    bool
	precision string
	note      string
	lastUsed  time.Time
	refs      map[string]string // tên giọng tạm của clip mẫu → tên đã nạp trên máy chủ
	clonesOn  map[string]bool   // giọng clone đã nạp vào tiến trình máy đọc hiện tại

	// cloneMu giữ thao tác đọc-ghi index giọng clone nhất quán khi AddVoice và
	// replayClones chạy cùng lúc (máy chủ mới bật đang nạp lại giọng).
	cloneMu sync.Mutex
	usageMu sync.Mutex
	usageAt time.Time
	usage   int64
	rtfMu   sync.Mutex
	rtf     float64
	rtfSeen bool
}

func newEngine(cfg voice.Config) (voice.Engine, error) {
	return &Engine{
		cfg:      cfg,
		dir:      filepath.Join(cfg.Home, "vieneu"),
		refs:     map[string]string{},
		clonesOn: map[string]bool{},
	}, nil
}

// Name trả tên engine.
func (e *Engine) Name() string { return "vieneu" }

// Capabilities mô tả VieNeu v3 Turbo: clone từ clip 3–8 giây, 48 kHz, không có
// cờ sắc thái riêng trong giao thức.
func (e *Engine) Capabilities() voice.Capabilities {
	return voice.Capabilities{
		Cloning:    true,
		SampleRate: 48000,
		Qualities:  voice.Qualities,
	}
}

func (e *Engine) srcDir() string    { return filepath.Join(e.dir, "src") }
func (e *Engine) hfHome() string    { return filepath.Join(e.dir, "hf") }
func (e *Engine) voicesDir() string { return filepath.Join(e.dir, "voices") }
func (e *Engine) logPath() string   { return filepath.Join(e.dir, "server.log") }
func (e *Engine) indexPath() string { return filepath.Join(e.voicesDir(), "index.json") }

func (e *Engine) venvPython() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(e.srcDir(), ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(e.srcDir(), ".venv", "bin", "python")
}

// Installed cho biết mã nguồn + môi trường Python đã có trên máy.
func (e *Engine) Installed() bool {
	if _, err := os.Stat(filepath.Join(e.srcDir(), "apps", "openai_speech.py")); err != nil {
		return false
	}
	if _, err := os.Stat(e.venvPython()); err != nil {
		return false
	}
	return true
}

// DiskUsage là dung lượng bộ đọc chiếm trên đĩa (môi trường Python + mô hình + giọng).
// Cây thư mục có hàng nghìn tệp nên kết quả được nhớ trong 1 phút — UI hỏi liên tục.
func (e *Engine) DiskUsage() int64 {
	e.usageMu.Lock()
	defer e.usageMu.Unlock()
	if time.Since(e.usageAt) < time.Minute {
		return e.usage
	}
	e.usage, e.usageAt = dirSize(e.dir), time.Now()
	return e.usage
}

// Status báo trạng thái thật: chưa cài / đang bật / đang chạy / lỗi.
func (e *Engine) Status(ctx context.Context) voice.Status {
	st := voice.Status{Engine: e.Name(), Model: e.cfg.Model, Device: e.cfg.Device}
	if !e.Installed() {
		st.State = voice.StateMissing
		return st
	}
	e.mu.Lock()
	p := e.proc
	starting := e.starting
	lastErr := e.startErr
	note := e.note
	e.mu.Unlock()
	st.Note = note
	if starting {
		st.State = voice.StateStarting
		return st
	}
	if p == nil {
		st.State = voice.StateStopped
		if lastErr != nil {
			st.State = voice.StateError
			st.Error = lastErr.Error()
		}
		return st
	}
	h, err := healthOnce(ctx, p.client)
	if err != nil {
		st.State = voice.StateError
		st.Error = fmt.Sprintf("máy đọc không phản hồi: %v", err)
		return st
	}
	if !h.OK() {
		st.State = voice.StateError
		st.Error = h.Error
		if st.Error == "" {
			st.Error = "máy đọc báo lỗi"
		}
		return st
	}
	st.State = voice.StateReady
	st.Ready = true
	st.Model = fmt.Sprintf("%s (%d Hz)", e.cfg.Model, h.SampleRate)
	if p.precision != "" && e.cfg.Device != "cuda" {
		st.Model += " · " + p.precision
	}
	return st
}

// Voices liệt kê giọng có sẵn + giọng đã clone, bật máy đọc nếu đang tắt.
func (e *Engine) Voices(ctx context.Context) ([]voice.Voice, error) {
	c, err := e.ensureStarted(ctx, e.cfg.Quality, nil)
	if err != nil {
		return nil, err
	}
	list, err := c.Voices(ctx)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(list))
	for _, v := range list {
		have[v.ID] = true
	}
	// Giọng clone thêm từ tiến trình khác (vd `voice say --ref`/CLI trong lúc serve
	// đang chạy) chỉ có trong index trên đĩa: nạp bù rồi liệt kê lại.
	if e.syncClones(ctx, c, have) > 0 {
		if list, err = c.Voices(ctx); err != nil {
			return nil, err
		}
	}
	clones := map[string]cloneEntry{}
	for _, entry := range e.loadClones() {
		clones[entry.Name] = entry
	}
	out := make([]voice.Voice, 0, len(list))
	for _, v := range list {
		if strings.HasPrefix(v.ID, refPrefix) {
			continue // giọng nạp tạm từ clip mẫu, không phải giọng người dùng chọn
		}
		if entry, ok := clones[v.ID]; ok {
			v.Source = voice.SourceClone
			v.File = entry.File
			v.DurationSec = entry.DurationSec
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Synthesize đọc một đoạn. RefAudio (clip mẫu của dự án) được nạp tạm thành một
// giọng theo nội dung clip, nên cùng một mẫu chỉ nạp một lần cho mỗi tiến trình.
func (e *Engine) Synthesize(ctx context.Context, req voice.SynthRequest) (voice.Audio, error) {
	req = req.Normalized()
	c, err := e.ensureStarted(ctx, req.Quality, nil)
	if err != nil {
		return voice.Audio{}, err
	}
	voiceID := req.VoiceID
	if req.RefAudio != "" {
		voiceID, err = e.ensureRefVoice(ctx, c, req.RefAudio)
		if err != nil {
			return voice.Audio{}, err
		}
	} else if voiceID != "" {
		e.mu.Lock()
		loaded := e.clonesOn[voiceID]
		e.mu.Unlock()
		if !loaded {
			// Có thể là giọng clone do tiến trình khác thêm sau khi máy đọc này bật.
			e.syncClones(ctx, c, map[string]bool{})
		}
	}
	// Chỉ đo phần tổng hợp: khởi động máy đọc / nạp clip mẫu không phải tốc độ đọc.
	started := time.Now()
	audio, err := c.Speak(ctx, req.Text, voiceID, openaispeech.SpeakOptions{SampleRate: 48000, Speed: req.Speed})
	if err != nil {
		return voice.Audio{}, err
	}
	e.observeRTF(time.Since(started), audio)
	e.markUsed()
	return audio, nil
}

// AddVoice nạp clip mẫu thành giọng clone và giữ lại để lần khởi động sau tự nạp lại
// (máy chủ VieNeu chỉ giữ giọng clone trong bộ nhớ tiến trình).
func (e *Engine) AddVoice(ctx context.Context, name, wavPath string) (voice.Voice, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return voice.Voice{}, errors.New("tên giọng đang trống")
	}
	data, err := os.ReadFile(wavPath)
	if err != nil {
		return voice.Voice{}, err
	}
	dur, _ := openaispeech.WAVDuration(data) // clip mp3/flac vẫn nạp được, chỉ không đo được

	c, err := e.ensureStarted(ctx, e.cfg.Quality, nil)
	if err != nil {
		return voice.Voice{}, err
	}
	if err := c.AddVoice(ctx, name, wavPath, "", true); err != nil {
		return voice.Voice{}, err
	}
	e.mu.Lock()
	e.clonesOn[name] = true
	e.mu.Unlock()

	local := filepath.Join(e.voicesDir(), slug(name)+".wav")
	if err := os.MkdirAll(e.voicesDir(), 0o755); err != nil {
		return voice.Voice{}, err
	}
	if same, _ := filepath.Abs(wavPath); same != local {
		if err := os.WriteFile(local, data, 0o644); err != nil {
			return voice.Voice{}, err
		}
	}
	entry := cloneEntry{Name: name, File: local, DurationSec: dur, CreatedAt: time.Now()}
	if err := e.saveClone(entry); err != nil {
		e.cfg.Logf("vieneu: không lưu được giọng %q: %v", name, err)
	}
	e.markUsed()
	return voice.Voice{ID: name, Name: name, Source: voice.SourceClone, File: local, DurationSec: dur}, nil
}

// Close tắt máy đọc.
func (e *Engine) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	p := e.proc
	e.proc = nil
	e.mu.Unlock()
	if p != nil {
		p.stop()
	}
	return nil
}

// startCall là một lượt bật máy đọc, do engine sở hữu thay vì yêu cầu nào.
type startCall struct {
	done   chan struct{}
	client *openaispeech.Client
	err    error
}

// ensureStarted trả client đã sẵn sàng. precision suy từ chất lượng yêu cầu; đổi
// chất lượng làm đổi precision thì máy đọc được bật lại (chất lượng là thiết lập
// của truyện nên hiếm khi đổi giữa chừng).
//
// Lượt bật chạy ở nền và không thuộc về người gọi: ctx của người gọi chỉ giới hạn
// việc *chờ*, không huỷ việc nạp mô hình. Nhờ vậy một GET bị huỷ (đổi trang, tải
// lại, trình duyệt cắt kết nối) không làm hỏng lượt bật mà tab khác đang chờ.
func (e *Engine) ensureStarted(ctx context.Context, quality string, onDownload func(int64)) (*openaispeech.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	want, note := e.precisionFor(quality)
	for {
		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			return nil, errors.New("engine giọng đọc đã đóng")
		}
		e.note = note
		if p := e.proc; p != nil {
			if p.precision == want && p.alive() {
				e.lastUsed = time.Now()
				c := p.client
				e.mu.Unlock()
				return c, nil
			}
			e.proc = nil
			e.mu.Unlock()
			e.cfg.Logf("vieneu: bật lại máy đọc (precision %s → %s, hoặc tiến trình đã dừng)", p.precision, want)
			p.stop()
			continue
		}
		if call := e.boot; call != nil {
			e.mu.Unlock()
			select {
			case <-call.done:
				if call.err != nil {
					return nil, call.err
				}
				// Lượt bật khác vừa xong: xét lại precision trước khi dùng.
				e.mu.Lock()
				p := e.proc
				usable := call.client != nil && p != nil && p.client == call.client && p.precision == want && p.alive()
				e.mu.Unlock()
				if usable {
					return call.client, nil
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		call := &startCall{done: make(chan struct{})}
		e.boot = call
		e.starting = true
		e.startErr = nil
		e.mu.Unlock()
		go e.runStart(call, want, onDownload)
		select {
		case <-call.done:
			if call.err != nil {
				return nil, call.err
			}
			return call.client, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// runStart bật máy đọc ở nền và ghi kết quả vào call. Bật lỗi thì để nguyên
// startErr cho Status báo, xoá lượt đang chạy để lần sau thử lại.
func (e *Engine) runStart(call *startCall, precision string, onDownload func(int64)) {
	// Trần tuyệt đối cho cả lượt bật (nạp mô hình đã có trần riêng trong waitReady).
	ctx, cancel := context.WithTimeout(context.Background(), readyTimeout+2*time.Minute)
	defer cancel()
	p, err := e.start(ctx, precision, onDownload)
	if err == nil {
		loaded := e.replayClones(ctx, p.client)
		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			p.stop()
			err = errors.New("engine giọng đọc đã đóng")
		} else {
			e.clonesOn = loaded
			e.starting = false
			e.startErr = nil
			e.proc = p
			e.precision = precision
			e.lastUsed = time.Now()
			// Tiến trình mới quên hết giọng clone/ref nạp trong bộ nhớ: index trên đĩa vừa
			// được nạp lại, còn bảng ref theo nội dung clip phải tính từ đầu.
			e.refs = map[string]string{}
			e.boot = nil
			e.mu.Unlock()
			go e.watchIdle(p)
			call.client = p.client
			close(call.done)
			return
		}
	}
	e.mu.Lock()
	e.boot = nil
	e.starting = false
	e.startErr = err
	e.mu.Unlock()
	call.err = err
	close(call.done)
}

// replayClones nạp lại các giọng clone đã lưu (máy chủ mất giọng khi tắt) và trả
// tập tên đã nạp được.
func (e *Engine) replayClones(ctx context.Context, c *openaispeech.Client) map[string]bool {
	loaded := map[string]bool{}
	for _, entry := range e.loadClones() {
		if e.enrollClone(ctx, c, entry) {
			loaded[entry.Name] = true
		}
	}
	return loaded
}

// syncClones nạp các giọng trong index mà máy đọc đang chạy chưa có (have = tên
// máy chủ đã có, cộng các tên đã nạp trong tiến trình này). Trả số giọng vừa nạp.
func (e *Engine) syncClones(ctx context.Context, c *openaispeech.Client, have map[string]bool) int {
	e.mu.Lock()
	known := make(map[string]bool, len(have)+len(e.clonesOn))
	for k := range e.clonesOn {
		known[k] = true
	}
	e.mu.Unlock()
	for k := range have {
		known[k] = true
	}
	added := 0
	for _, entry := range missingClones(e.loadClones(), known) {
		if e.enrollClone(ctx, c, entry) {
			added++
		}
		// Kể cả khi nạp lỗi cũng ghi nhận, để mỗi lượt đọc không thử lại mãi.
		e.mu.Lock()
		e.clonesOn[entry.Name] = true
		e.mu.Unlock()
	}
	return added
}

// missingClones lọc các giọng trong index chưa có trong known.
func missingClones(index []cloneEntry, known map[string]bool) []cloneEntry {
	var out []cloneEntry
	for _, entry := range index {
		if !known[entry.Name] {
			out = append(out, entry)
		}
	}
	return out
}

// enrollClone nạp một giọng clone đã lưu vào máy đọc.
func (e *Engine) enrollClone(ctx context.Context, c *openaispeech.Client, entry cloneEntry) bool {
	if _, err := os.Stat(entry.File); err != nil {
		e.cfg.Logf("vieneu: bỏ qua giọng %q vì thiếu tệp %s", entry.Name, entry.File)
		return false
	}
	if err := c.AddVoice(ctx, entry.Name, entry.File, "", true); err != nil {
		e.cfg.Logf("vieneu: không nạp lại được giọng %q: %v", entry.Name, err)
		return false
	}
	return true
}

// watchIdle tắt máy đọc sau idleTimeout không dùng, và dọn khi tiến trình chết.
func (e *Engine) watchIdle(p *proc) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.done:
			e.mu.Lock()
			if e.proc == p {
				e.proc = nil
			}
			if !e.closed && p.exitErr != nil {
				e.startErr = fmt.Errorf("máy đọc dừng bất thường: %v · %s", p.exitErr, e.logTail(4))
			}
			e.mu.Unlock()
			return
		case <-t.C:
			e.mu.Lock()
			cur, last := e.proc, e.lastUsed
			e.mu.Unlock()
			if cur != p {
				return
			}
			if time.Since(last) >= idleTimeout {
				e.cfg.Logf("vieneu: tắt máy đọc sau %s không dùng", idleTimeout)
				e.mu.Lock()
				if e.proc == p {
					e.proc = nil
				}
				e.mu.Unlock()
				p.stop()
				return
			}
		}
	}
}

// observeRTF cập nhật đường trung bình động của tốc độ tạo (wall/audio).
func (e *Engine) observeRTF(wall time.Duration, a voice.Audio) {
	if wall <= 0 || a.DurationSec < 0.2 {
		return
	}
	v := wall.Seconds() / a.DurationSec
	e.rtfMu.Lock()
	if !e.rtfSeen {
		e.rtf, e.rtfSeen = v, true
	} else {
		e.rtf = 0.7*e.rtf + 0.3*v
	}
	e.rtfMu.Unlock()
}

// RTF trả tỉ lệ thời gian tạo / thời lượng audio (nil = chưa đo được).
func (e *Engine) RTF() *float64 {
	e.rtfMu.Lock()
	defer e.rtfMu.Unlock()
	if !e.rtfSeen {
		return nil
	}
	v := e.rtf
	return &v
}

func (e *Engine) markUsed() {
	e.mu.Lock()
	e.lastUsed = time.Now()
	e.mu.Unlock()
}

// precisionFor map chất lượng → precision ONNX. int8 chỉ dùng khi CPU có VNNI;
// không có thì giữ fp32 và ghi chú để UI/CLI báo lại.
func (e *Engine) precisionFor(quality string) (string, string) {
	if e.cfg.Device == "cuda" {
		return "fp32", "" // GPU chạy PyTorch, precision chỉ áp cho đường ONNX/CPU
	}
	if voice.NormalizeQuality(quality) == voice.QualityFast {
		if supportsVNNI() {
			return "int8", ""
		}
		return "fp32", "CPU không hỗ trợ int8 (VNNI) nên vẫn dùng fp32 cho chất lượng nhanh"
	}
	return "fp32", ""
}

// supportsVNNI cho biết CPU có tập lệnh int8 mà backbone ONNX cần để không bị méo.
func supportsVNNI() bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			return false
		}
		s := strings.ToLower(string(data))
		return strings.Contains(s, "avx_vnni") || strings.Contains(s, "avx512_vnni")
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "machdep.cpu.leaf7_features").Output()
		if err != nil {
			return false
		}
		s := strings.ToUpper(string(out))
		return strings.Contains(s, "AVX512_VNNI") || strings.Contains(s, "AVX_VNNI")
	}
	return false
}

// ensureRefVoice nạp clip mẫu thành giọng tạm (theo nội dung clip) rồi trả tên giọng.
func (e *Engine) ensureRefVoice(ctx context.Context, c *openaispeech.Client, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được clip mẫu %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	name := refPrefix + hex.EncodeToString(sum[:6])

	e.mu.Lock()
	_, done := e.refs[name]
	e.mu.Unlock()
	if done {
		return name, nil
	}
	if err := c.AddVoice(ctx, name, path, "", true); err != nil {
		return "", fmt.Errorf("không nạp được clip mẫu làm giọng đọc: %w", err)
	}
	e.mu.Lock()
	e.refs[name] = name
	e.mu.Unlock()
	return name, nil
}

// cloneEntry là một giọng clone đã lưu trên đĩa.
type cloneEntry struct {
	Name        string    `json:"name"`
	File        string    `json:"file"`
	DurationSec float64   `json:"durationSec,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (e *Engine) loadClones() []cloneEntry {
	e.cloneMu.Lock()
	defer e.cloneMu.Unlock()
	return e.loadClonesLocked()
}

// loadClonesLocked đọc index khi đã giữ cloneMu.
func (e *Engine) loadClonesLocked() []cloneEntry {
	data, err := os.ReadFile(e.indexPath())
	if err != nil {
		return nil
	}
	var entries []cloneEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		e.cfg.Logf("vieneu: index giọng hỏng (%v), bỏ qua", err)
		return nil
	}
	return entries
}

func (e *Engine) saveClone(entry cloneEntry) error {
	e.cloneMu.Lock()
	defer e.cloneMu.Unlock()
	entries := e.loadClonesLocked()
	replaced := false
	for i := range entries {
		if entries[i].Name == entry.Name {
			entries[i] = entry
			replaced = true
		}
	}
	if !replaced {
		entries = append(entries, entry)
	}
	if err := os.MkdirAll(e.voicesDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.indexPath(), data, 0o644)
}

// slug biến tên giọng thành tên tệp an toàn.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteRune('-')
		case r > 127:
			// Giữ nguyên chữ có dấu: hệ tệp macOS/Linux đều chấp nhận UTF-8.
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = "giong"
	}
	return out
}

// dirSize tính dung lượng một cây thư mục (bỏ qua lỗi từng tệp).
func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
