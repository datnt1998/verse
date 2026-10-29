// Package voice là tầng giọng đọc dùng chung của ainovel.
//
// Nó không gắn với Audio truyện: đọc to trong Trình đọc, thông báo bằng giọng nói,
// nghe thử cách đọc… đều đi qua cùng một Service. Mỗi engine (máy đọc) chỉ cần cài
// đặt interface Engine rồi Register; Service lo hàng đợi một luồng cho mỗi engine,
// ưu tiên nghe thử trước tạo hàng loạt, tạm dừng/huỷ theo job, và đo RTF.
//
// Giao thức chuẩn giữa Go và máy đọc là OpenAI Speech API (xem package
// openaispeech): POST /v1/audio/speech, GET/POST /v1/voices, GET /health. Nhờ vậy
// cắm máy đọc khác (VieNeu chạy Docker, máy khác trong mạng, dịch vụ TTS tương
// thích OpenAI) không cần sửa Go.
package voice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Trạng thái engine. Giá trị ổn định vì đi thẳng ra API (GET /api/tts/status).
const (
	StateMissing    = "missing"    // chưa cài bộ đọc trên máy
	StateInstalling = "installing" // đang cài
	StateStopped    = "stopped"    // đã cài, tiến trình đang tắt (tự bật lại khi cần)
	StateStarting   = "starting"   // đang bật / đang nạp mô hình
	StateReady      = "ready"      // dùng được ngay
	StateError      = "error"      // lỗi (kèm Error)
)

// Chất lượng giọng đọc — cấu hình của truyện, cũng là khoá cache đoạn audio.
const (
	QualityFast     = "fast"
	QualityStandard = "standard"
	QualityHigh     = "high"
)

// Qualities là danh sách chất lượng hợp lệ, theo thứ tự tăng dần.
var Qualities = []string{QualityFast, QualityStandard, QualityHigh}

// Lỗi ổn định để tầng trên map sang HTTP/CLI mà không phải so chuỗi.
var (
	// ErrUnsupported: engine không hỗ trợ thao tác (vd clone giọng trên máy đọc từ xa).
	ErrUnsupported = errors.New("máy đọc không hỗ trợ thao tác này")
	// ErrNotInstalled: bộ đọc trên máy chưa cài (UI hiện thẻ hướng dẫn cài).
	ErrNotInstalled = errors.New("bộ đọc trên máy chưa được cài")
	// ErrCanceled: job bị huỷ qua Service.Cancel.
	ErrCanceled = errors.New("đã huỷ")
	// ErrInstallRunning: đang có một lượt cài chạy.
	ErrInstallRunning = errors.New("đang cài bộ đọc")
	// ErrBusy: máy đọc từ chối vì quá tải (429) sau khi đã thử lại.
	ErrBusy = errors.New("máy đọc đang bận")
)

// Capabilities mô tả một engine: dùng để UI ẩn/hiện nút và để tầng trên biết
// yêu cầu nào chắc chắn thất bại.
type Capabilities struct {
	Cloning     bool     `json:"cloning"`     // clone giọng từ clip mẫu
	EmotionCues bool     `json:"emotionCues"` // hiểu sắc thái trong chữ (vd [cười])
	Streaming   bool     `json:"streaming"`   // trả audio theo luồng
	SampleRate  int      `json:"sampleRate"`  // Hz mặc định
	Qualities   []string `json:"qualities"`   // chất lượng hỗ trợ
}

// Voice là một giọng đọc mà engine biết.
type Voice struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Gender      string  `json:"gender"` // male | female | ""
	Source      string  `json:"source"` // builtin | project | clone
	File        string  `json:"file,omitempty"`
	DurationSec float64 `json:"durationSec,omitempty"`
}

// Nguồn giọng.
const (
	SourceBuiltin = "builtin"
	SourceProject = "project"
	SourceClone   = "clone"
)

// SynthRequest là một lượt đọc.
type SynthRequest struct {
	Text     string  `json:"text"`
	VoiceID  string  `json:"voiceId"`  // giọng có sẵn của engine (preset/clone)
	RefAudio string  `json:"refAudio"` // clip mẫu cho lần đọc này (rỗng = dùng VoiceID)
	Tone     string  `json:"tone"`     // sắc thái; engine không hỗ trợ thì bỏ qua
	Quality  string  `json:"quality"`  // fast|standard|high; rỗng = chất lượng cấu hình
	Speed    float64 `json:"speed"`    // 1 = tự nhiên; engine bỏ qua speed thì chỉnh bằng ffmpeg
}

// Normalized trả bản sao đã chuẩn hoá: chất lượng hợp lệ, tốc độ trong khoảng cho phép.
func (r SynthRequest) Normalized() SynthRequest {
	r.Quality = NormalizeQuality(r.Quality)
	if r.Speed <= 0 {
		r.Speed = 1
	}
	if r.Speed < 0.5 {
		r.Speed = 0.5
	}
	if r.Speed > 2 {
		r.Speed = 2
	}
	return r
}

// Audio là kết quả một lượt đọc: WAV 16-bit PCM kèm số đo đã tính sẵn.
type Audio struct {
	WAV         []byte  `json:"-"`
	SampleRate  int     `json:"sampleRate"`
	DurationSec float64 `json:"durationSec"`
}

// Status là ảnh chụp trạng thái engine tại một thời điểm.
type Status struct {
	Ready  bool   `json:"ready"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Device string `json:"device"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
	// Note là ghi chú không phải lỗi (vd "CPU không hỗ trợ int8 nên giữ fp32").
	Note string   `json:"note,omitempty"`
	RTF  *float64 `json:"rtf"` // thời gian tạo / thời lượng audio (càng nhỏ càng nhanh)
}

// Engine là một máy đọc. Cài đặt mới chỉ cần thoả interface này + Register.
type Engine interface {
	Name() string
	Capabilities() Capabilities
	Status(ctx context.Context) Status
	Voices(ctx context.Context) ([]Voice, error)
	Synthesize(ctx context.Context, req SynthRequest) (Audio, error)
	AddVoice(ctx context.Context, name, wavPath string) (Voice, error)
	Close() error
}

// Installer là engine cài được trên máy (bộ đọc cục bộ). Service phát tiến độ qua
// InstallProgress để CLI in ra và gateway đẩy SSE.
type Installer interface {
	Installed() bool
	Install(ctx context.Context, report func(InstallProgress)) error
}

// Sizer là engine báo được dung lượng đã cài trên đĩa (cho Cài đặt › Giọng đọc).
type Sizer interface {
	DiskUsage() int64
}

// RTFReporter là engine tự đo RTF phần tổng hợp (loại trừ thời gian khởi động).
type RTFReporter interface {
	RTF() *float64
}

// Factory dựng engine từ cấu hình; gọi lười ở lần dùng đầu.
type Factory func(cfg Config) (Engine, error)

// Config là cấu hình giọng đọc, map 1-1 từ khoá `tts` trong config.json.
type Config struct {
	Engine   string // vieneu (mặc định) | openai-speech
	Model    string // model gửi lên máy đọc (vieneu: vieneu-v3-turbo)
	Device   string // cpu | gpu
	Quality  string // fast | standard | high
	Endpoint string // engine openai-speech: địa chỉ máy chủ (vd http://127.0.0.1:8000)
	APIKey   string // Bearer token của máy chủ (chỉ ở cấu hình toàn cục)
	Home     string // thư mục dữ liệu engine (mặc định ~/.ainovel/voice)
	Logf     func(format string, args ...any)
}

// DefaultModel là model mặc định của máy đọc cục bộ.
const DefaultModel = "vieneu-v3-turbo"

// WithDefaults điền giá trị thiếu.
func (c Config) WithDefaults() Config {
	if strings.TrimSpace(c.Engine) == "" {
		c.Engine = "vieneu"
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = DefaultModel
	}
	c.Device = strings.ToLower(strings.TrimSpace(c.Device))
	if c.Device == "" || c.Device == "auto" {
		c.Device = "cpu"
	}
	if c.Device == "gpu" {
		c.Device = "cuda"
	}
	c.Quality = NormalizeQuality(c.Quality)
	if strings.TrimSpace(c.Home) == "" {
		c.Home = DefaultHome()
	}
	if c.Logf == nil {
		c.Logf = func(string, ...any) {}
	}
	return c
}

// DefaultHome là thư mục dữ liệu của các engine (~/.ainovel/voice).
func DefaultHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "ainovel-voice")
	}
	return filepath.Join(home, ".ainovel", "voice")
}

// NormalizeQuality chuẩn hoá chất lượng; rỗng/không hợp lệ → standard.
func NormalizeQuality(q string) string {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case QualityFast:
		return QualityFast
	case QualityHigh:
		return QualityHigh
	default:
		return QualityStandard
	}
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register đăng ký engine mới. Trùng tên thì ghi đè (tiện cho test).
func Register(name string, f Factory) {
	name = strings.TrimSpace(name)
	if name == "" || f == nil {
		return
	}
	registryMu.Lock()
	registry[name] = f
	registryMu.Unlock()
}

// Registered liệt kê tên các engine đã đăng ký, đã sắp xếp.
func Registered() []string {
	registryMu.RLock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	registryMu.RUnlock()
	sort.Strings(names)
	return names
}

// NewEngine dựng engine theo cấu hình (đã điền mặc định).
func NewEngine(cfg Config) (Engine, error) {
	cfg = cfg.WithDefaults()
	registryMu.RLock()
	f, ok := registry[cfg.Engine]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("chưa hỗ trợ máy đọc %q (đã đăng ký: %s)", cfg.Engine, strings.Join(Registered(), ", "))
	}
	return f(cfg)
}
