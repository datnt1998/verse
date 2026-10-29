package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/voice"
	_ "github.com/datnt1998/noverse/internal/voice/bootcfg"
	_ "github.com/datnt1998/noverse/internal/voice/openaispeech"
	_ "github.com/datnt1998/noverse/internal/voice/vieneu"
	"github.com/datnt1998/noverse/internal/webauth"
)

// Nhóm endpoint GAP-21: bộ đọc trên máy. Đây là thiết lập của máy (không thuộc
// truyện nào), nên đọc thì mọi tài khoản đã đăng nhập đều xem được, còn cài đặt
// (chạm vào máy chủ) chỉ admin.
//
// Tiến độ cài có hai đường: SSE `tts.install` ở GET /api/tts/install/events, và
// polling GET /api/tts/install (cùng một InstallProgress) — UI dùng SSE, tự
// polling làm dự phòng khi mạng chập chờn.

// ttsStatusResponse là JSON của GET /api/tts/status.
type ttsStatusResponse struct {
	Ready          bool     `json:"ready"`
	Engine         string   `json:"engine"`
	Model          string   `json:"model"`
	Device         string   `json:"device"`
	Quality        string   `json:"quality"`
	State          string   `json:"state"`
	Error          string   `json:"error"`
	Note           string   `json:"note,omitempty"`
	RTF            *float64 `json:"rtf"`
	SampleRate     int      `json:"sampleRate"`
	Cloning        bool     `json:"cloning"`
	Qualities      []string `json:"qualities"`
	FFmpeg         bool     `json:"ffmpeg"`
	Installed      bool     `json:"installed"`
	InstalledBytes int64    `json:"installedBytes"`
}

// ttsEngineInfo là một mục trong GET /api/tts/engines.
type ttsEngineInfo struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Installed    bool               `json:"installed"`
	Capabilities voice.Capabilities `json:"capabilities"`
}

// ttsService trả dịch vụ giọng đọc dùng chung của tiến trình.
func ttsService() *voice.Service { return voice.Shared() }

// serveTTS định tuyến /api/tts/*; trả false khi đường dẫn không thuộc nhóm này.
func (g *Gateway) serveTTS(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	path, get, post := r.URL.Path, r.Method == http.MethodGet, r.Method == http.MethodPost
	switch {
	case path == "/api/tts/status" && get:
		g.ttsStatus(w, r)
	case path == "/api/tts/engines" && get:
		g.ttsEngines(w, r)
	case path == "/api/tts/install" && post:
		g.ttsInstall(w, r, user)
	case path == "/api/tts/install" && get:
		writeJSON(w, 200, ttsInstallState(ttsService().InstallState()))
	case path == "/api/tts/install/events" && get:
		g.ttsInstallEvents(w, r)
	case path == "/api/tts/voices" && get:
		g.ttsVoices(w, r)
	case path == "/api/tts/preview" && post:
		g.ttsPreview(w, r)
	case strings.HasPrefix(path, "/api/tts"):
		jsonError(w, 404, "không tìm thấy")
	default:
		return false
	}
	return true
}

func (g *Gateway) ttsStatus(w http.ResponseWriter, r *http.Request) {
	svc := ttsService()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	st := svc.Status(ctx)
	caps := svc.Capabilities()
	_, ffmpegErr := exec.LookPath("ffmpeg")
	writeJSON(w, 200, ttsStatusResponse{
		Ready:          st.Ready,
		Engine:         st.Engine,
		Model:          st.Model,
		Device:         st.Device,
		Quality:        svc.Config().Quality,
		State:          st.State,
		Error:          st.Error,
		Note:           st.Note,
		RTF:            st.RTF,
		SampleRate:     caps.SampleRate,
		Cloning:        caps.Cloning,
		Qualities:      caps.Qualities,
		FFmpeg:         ffmpegErr == nil,
		Installed:      svc.Installed(),
		InstalledBytes: svc.DiskUsage(),
	})
}

// ttsEngineDescriptions là tên + mô tả tiếng Việt của engine đã biết.
var ttsEngineDescriptions = map[string][2]string{
	"vieneu":        {"VieNeu-TTS trên máy", "Chạy ngay trên máy của bạn, không cần mạng; tự cài bằng một nút."},
	"openai-speech": {"Máy chủ giọng đọc tương thích OpenAI", "VieNeu chạy Docker hoặc máy khác trong mạng, theo chuẩn OpenAI Speech API."},
}

// ttsEngines liệt kê engine đã đăng ký; installed hỏi thẳng engine chứ không
// gán cùng một cờ — vieneu báo được trạng thái cài, engine từ xa cần endpoint.
func (g *Gateway) ttsEngines(w http.ResponseWriter, r *http.Request) {
	svc := ttsService()
	cfg := svc.Config()
	infos := make([]ttsEngineInfo, 0, len(voice.Registered()))
	for _, name := range voice.Registered() {
		installed, caps := true, voice.Capabilities{SampleRate: 48000, Qualities: voice.Qualities}
		if eng, err := voice.NewEngine(voice.Config{Engine: name, Home: cfg.Home, Endpoint: cfg.Endpoint}); err == nil {
			if ins, ok := eng.(voice.Installer); ok {
				installed = ins.Installed()
			}
			caps = eng.Capabilities()
			// Máy chủ từ xa chỉ "sẵn sàng dùng" khi đã cấu hình endpoint.
			if name == "openai-speech" && strings.TrimSpace(cfg.Endpoint) == "" {
				installed = false
			}
		} else {
			installed = false
		}
		meta := ttsEngineDescriptions[name]
		if meta[0] == "" {
			meta[0] = name
		}
		infos = append(infos, ttsEngineInfo{ID: name, Name: meta[0], Description: meta[1], Installed: installed, Capabilities: caps})
	}
	writeJSON(w, 200, map[string]any{"engines": infos, "active": cfg.Engine})
}

func (g *Gateway) ttsInstall(w http.ResponseWriter, r *http.Request, user webauth.User) {
	if user.Role != webauth.RoleAdmin {
		jsonError(w, 403, "Không có quyền")
		return
	}
	svc := ttsService()
	if svc.Installed() {
		writeJSON(w, 200, map[string]any{"job": "", "state": "done", "message": "Bộ đọc trên máy đã có sẵn"})
		return
	}
	// Lượt cài phải sống tiếp kể cả khi người dùng đóng tab: không dùng ctx của request.
	if err := svc.StartInstall(context.Background()); err != nil {
		if errors.Is(err, voice.ErrInstallRunning) {
			jsonError(w, 409, "Đang cài bộ đọc, chờ lượt này xong")
			return
		}
		jsonError(w, 500, err.Error())
		return
	}
	writeJSON(w, 202, map[string]any{"job": voiceInstallJobID, "state": "running"})
}

// ttsInstallState chuẩn hoá trạng thái cài trước khi trả: chưa từng cài = "idle"
// để client có một enum dứt khoát thay vì chuỗi rỗng.
func ttsInstallState(p voice.InstallProgress) voice.InstallProgress {
	if p.State == "" {
		p.State = "idle"
	}
	return p
}

// ttsInstallEvents phát SSE `tts.install` (mỗi sự kiện là một InstallProgress).
func (g *Gateway) ttsInstallEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, 500, "Máy chủ không hỗ trợ luồng sự kiện")
		return
	}
	svc := ttsService()
	send := func(p voice.InstallProgress) bool {
		data, err := json.Marshal(p)
		if err != nil {
			return true
		}
		_, err = fmt.Fprintf(w, "event: tts.install\ndata: %s\n\n", data)
		flusher.Flush()
		return err == nil
	}
	// Gửi mốc hiện tại ngay để client vừa nối đã biết đang cài/kết thúc thế nào.
	cur := ttsInstallState(svc.InstallState())
	if cur.State != "" {
		if !send(cur) {
			return
		}
		if cur.State == "done" || cur.State == "error" {
			return
		}
	}
	ch := make(chan voice.InstallProgress, 8)
	unsub := svc.SubscribeInstall(func(p voice.InstallProgress) {
		select {
		case ch <- p:
		default: // UI chậm thì bỏ mốc cũ, mốc mới luôn có sau
		}
	})
	defer unsub()

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case p := <-ch:
			if !send(p) {
				return
			}
			if p.State == "done" || p.State == "error" {
				return
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (g *Gateway) ttsVoices(w http.ResponseWriter, r *http.Request) {
	svc := ttsService()
	// Lần đầu gọi có thể phải bật máy đọc và nạp mô hình (~1 phút).
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Minute)
	defer cancel()
	list, err := svc.Voices(ctx)
	if err != nil {
		ttsEngineError(w, err)
		return
	}
	type row struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Gender      string  `json:"gender"`
		Source      string  `json:"source"`
		BuiltIn     bool    `json:"builtIn"`
		File        string  `json:"file,omitempty"`
		DurationSec float64 `json:"durationSec,omitempty"`
	}
	rows := make([]row, 0, len(list))
	for _, v := range list {
		rows = append(rows, row{
			ID:          v.ID,
			Name:        v.Name,
			Gender:      v.Gender,
			Source:      v.Source,
			BuiltIn:     v.Source == voice.SourceBuiltin,
			File:        v.File,
			DurationSec: v.DurationSec,
		})
	}
	writeJSON(w, 200, map[string]any{"voices": rows})
}

// ttsPreviewRequest là thân POST /api/tts/preview.
type ttsPreviewRequest struct {
	VoiceID string  `json:"voiceId"`
	Text    string  `json:"text"`
	Tone    string  `json:"tone"`
	Quality string  `json:"quality"`
	Speed   float64 `json:"speed"`
}

// ttsPreview đọc thử một đoạn và trả thẳng audio/wav.
func (g *Gateway) ttsPreview(w http.ResponseWriter, r *http.Request) {
	var req ttsPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, 400, "Dữ liệu gửi lên không đọc được")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		jsonError(w, 400, "Chưa có chữ để đọc")
		return
	}
	if len([]rune(text)) > 20000 {
		jsonError(w, 400, "Đoạn quá dài để đọc thử (tối đa 20.000 ký tự)")
		return
	}
	svc := ttsService()
	// Lần đầu có thể phải bật máy đọc; bản thân một đoạn ngắn chỉ mất vài giây.
	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Minute)
	defer cancel()
	audio, err := svc.Preview(ctx, voice.SynthRequest{
		Text:    text,
		VoiceID: req.VoiceID,
		Tone:    req.Tone,
		Quality: req.Quality,
		Speed:   req.Speed,
	})
	if err != nil {
		ttsEngineError(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprint(len(audio.WAV)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Sample-Rate", fmt.Sprint(audio.SampleRate))
	_, _ = w.Write(audio.WAV)
}

// ttsEngineError map lỗi engine sang HTTP kèm câu tiếng Việt cho UI.
func ttsEngineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, voice.ErrNotInstalled):
		jsonError(w, 503, "Bộ đọc trên máy chưa được cài. Vào Cài đặt › Giọng đọc để cài.")
	case errors.Is(err, voice.ErrBusy):
		jsonError(w, 503, "Bộ đọc đang bận, thử lại sau ít giây.")
	case errors.Is(err, voice.ErrUnsupported):
		jsonError(w, 400, err.Error())
	case errors.Is(err, context.Canceled):
		jsonError(w, 499, "Đã huỷ")
	default:
		jsonError(w, 502, err.Error())
	}
}
