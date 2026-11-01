package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/bootcfg"
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/openaispeech"
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/vieneu"
	"github.com/datnt1998/verse/platform/identity"
)

// Nhóm endpoint GAP-21: bộ đọc trên máy. Trạng thái/engine/giọng đọc thì mọi
// tài khoản đã đăng nhập đều xem được; cài đặt và tiến độ cài (chạm vào máy chủ,
// lộ đường dẫn/log của máy) chỉ admin — cùng quyền với hàng việc của máy trong
// dock (Job Project/Story rỗng).
//
// Tiến độ cài có hai đường: SSE `tts.install` ở GET /api/tts/install/events, và
// polling GET /api/tts/install (cùng một InstallProgress) — UI dùng SSE, tự
// polling làm dự phòng khi mạng chập chờn.

// ttsStatusResponse là JSON của GET /api/tts/status (Gói AT, 10-audio.md §8).
//
// Một hình dạng cho cả hai bản chạy: `mode` nói số liệu đến từ đâu — bản máy
// chủ đọc trạng thái do máy đọc công bố, bản cá nhân đọc lượt chuẩn bị cục bộ.
// `percent` là null khi bước đang chạy không đo được (không bao giờ 0 thay thế);
// `error` chỉ có mã + câu, UI tự dịch mã sang câu 8.2.
type ttsStatusResponse struct {
	Mode           string          `json:"mode"` // hosted | local
	Ready          bool            `json:"ready"`
	Engine         string          `json:"engine"`
	Model          string          `json:"model"`
	Device         string          `json:"device"`
	Quality        string          `json:"quality"`
	State          string          `json:"state"`
	Step           string          `json:"step"`
	Percent        *int            `json:"percent"`
	BytesDone      int64           `json:"bytesDone"`
	BytesTotal     int64           `json:"bytesTotal"`
	EtaSec         int             `json:"etaSec"`
	Error          *ttsStatusError `json:"error"`
	LastSeenAt     *time.Time      `json:"lastSeenAt"`
	Machine        *ttsMachine     `json:"machine"`
	Version        string          `json:"version"`
	DiskBytes      int64           `json:"diskBytes"`
	RTF            *float64        `json:"rtf"`
	Capabilities   ttsCapabilities `json:"capabilities"`
	QueueWaiting   int             `json:"queueWaiting"`
	FFmpeg         bool            `json:"ffmpeg"`
	SampleRate     int             `json:"sampleRate"`
	Cloning        bool            `json:"cloning"`
	Qualities      []string        `json:"qualities"`
	Installed      bool            `json:"installed"`
	InstalledBytes int64           `json:"installedBytes"`
	Note           string          `json:"note,omitempty"`
	// Chỉ quản trị (Tuỳ chỉnh › Nâng cao): 6 dòng nhật ký gần nhất và kênh nối.
	LogTail []string `json:"logTail,omitempty"`
	Channel string   `json:"channel,omitempty"`
}

// ttsStatusError là lý do một lượt chuẩn bị thất bại. Mã chỉ hiện ở Nâng cao.
type ttsStatusError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ttsMachine là máy đọc đang phục vụ. Bản cá nhân không có máy đọc riêng
// (`mode: "local"`, `machine: null` — UI tự ghi "Máy này").
type ttsMachine struct {
	Name         string     `json:"name"`
	RevokedAt    *time.Time `json:"revokedAt"`
	GrantedAt    *time.Time `json:"grantedAt"`
	Capabilities []string   `json:"capabilities"`
}

// ttsCapabilities là những gì máy đọc nhận làm thêm ngoài tạo hàng loạt.
// `preview` = nghe thử giọng theo yêu cầu; UI ẩn nút Nghe thử tới khi có.
type ttsCapabilities struct {
	Preview bool `json:"preview"`
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
func (g *Gateway) serveTTS(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	path, get, post := r.URL.Path, r.Method == http.MethodGet, r.Method == http.MethodPost
	switch {
	case path == "/api/tts/status" && get:
		g.ttsStatus(w, r, user)
	case path == "/api/tts/machine/revoke" && post:
		g.ttsRevokeMachine(w, r, user)
	case path == "/api/tts/engines" && get:
		if g.pgAudio != nil {
			writeJSON(w, 200, g.hostedTTSEngines(r.Context()))
			return true
		}
		g.ttsEngines(w, r)
	case path == "/api/tts/install" && post:
		if g.pgAudio != nil {
			g.hostedTTSInstall(w, r, user)
			return true
		}
		g.ttsInstall(w, r, user)
	case path == "/api/tts/install" && get:
		if !canControlMachineJobs(user) {
			jsonError(w, 403, "Không có quyền")
			return true
		}
		if g.pgAudio != nil {
			writeJSON(w, 200, g.hostedTTSInstallState(r.Context()))
			return true
		}
		writeJSON(w, 200, ttsInstallState(ttsService().InstallState()))
	case path == "/api/tts/install/events" && get:
		if !canControlMachineJobs(user) {
			jsonError(w, 403, "Không có quyền")
			return true
		}
		if g.pgAudio != nil {
			g.hostedTTSInstallEvents(w, r)
			return true
		}
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

func (g *Gateway) ttsStatus(w http.ResponseWriter, r *http.Request, user identity.User) {
	if g.pgAudio != nil {
		writeJSON(w, 200, g.hostedTTSStatus(r.Context(), user))
		return
	}
	writeJSON(w, 200, g.localTTSStatus(r.Context()))
}

// localTTSState đổi trạng thái engine cục bộ sang enum của thẻ Máy đọc
// (missing | provisioning | starting | ready | error). `stopped` = đã cài nhưng
// tiến trình đang tắt, tự bật lại khi cần — với người dùng đó là "chưa chuẩn bị".
func localTTSState(state string) string {
	switch state {
	case voice.StateInstalling:
		return voice.ProvisionStateProvisioning
	case voice.StateStarting:
		return voice.ProvisionStateStarting
	case voice.StateReady:
		return voice.ProvisionStateReady
	case voice.StateError:
		return voice.ProvisionStateError
	default:
		return voice.ProvisionStateMissing
	}
}

// localTTSStatus dựng cùng hình dạng với bản máy chủ từ lượt chuẩn bị cục bộ:
// `mode: "local"`, không có `machine` (UI ghi "Máy này"), Nghe thử luôn có vì
// chính tiến trình này đọc được.
func (g *Gateway) localTTSStatus(ctx context.Context) ttsStatusResponse {
	svc := ttsService()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	st := svc.Status(ctx)
	caps := svc.Capabilities()
	inst := svc.InstallState()
	resp := ttsStatusResponse{
		Mode:           "local",
		Ready:          st.Ready,
		Engine:         st.Engine,
		Model:          st.Model,
		Device:         st.Device,
		Quality:        svc.Config().Quality,
		State:          localTTSState(st.State),
		Note:           st.Note,
		RTF:            st.RTF,
		SampleRate:     caps.SampleRate,
		Cloning:        caps.Cloning,
		Qualities:      caps.Qualities,
		FFmpeg:         ffmpegOnPath(),
		Installed:      svc.Installed(),
		InstalledBytes: svc.DiskUsage(),
		DiskBytes:      svc.DiskUsage(),
		Capabilities:   ttsCapabilities{Preview: true},
	}
	switch inst.State {
	case "running":
		resp.State = voice.ProvisionStateProvisioning
		resp.Step = inst.Step
		resp.Percent = installPercentValue(inst.Percent)
		resp.BytesDone, resp.BytesTotal, resp.EtaSec = inst.BytesDone, inst.BytesTotal, inst.EtaSec
		resp.Version = inst.Version
		if resp.Version == "" {
			resp.Version = st.Model
		}
	case "error":
		if !st.Ready {
			resp.State = voice.ProvisionStateError
			resp.Error = &ttsStatusError{Code: inst.ErrorCode, Message: firstNonEmpty(inst.Message, inst.Error)}
			resp.Version = inst.Version
		}
	case "done":
		resp.Version = firstNonEmpty(inst.Version, resp.Version)
	}
	return resp
}

// installPercentValue đổi con trỏ phần trăm của lượt cài sang hình dạng API:
// nil = bước không đo được (không bao giờ 0 thay thế).
func installPercentValue(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// ttsRevokeMachine thu hồi quyền của máy đọc đang phục vụ (một bước, Nâng cao):
// credential ngừng nhận việc ngay, máy không còn claim/heartbeat/publish được,
// và trạng thái chuyển sang `error · token_rejected` để cấp lại quyền là việc
// tiếp theo. Không xoá dữ liệu nào — cấp lại quyền là dùng lại được.
func (g *Gateway) ttsRevokeMachine(w http.ResponseWriter, r *http.Request, user identity.User) {
	if !canControlMachineJobs(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	if g.pgAudio == nil || g.pgAudio.Jobs == nil {
		jsonError(w, http.StatusNotFound, "Bản này không có máy đọc nào để thu hồi quyền")
		return
	}
	ctx := r.Context()
	snap := g.workerSnapshot(ctx)
	if !snap.Found {
		jsonError(w, http.StatusNotFound, "Chưa có máy đọc nào kết nối")
		return
	}
	if snap.Revoked {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine": snap.Name})
		return
	}
	if err := g.pgAudio.Jobs.RevokeWorker(ctx, snap.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, "Chưa thu hồi được quyền của máy đọc")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine": snap.Name})
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

func (g *Gateway) ttsInstall(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
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
	if g.pgAudio != nil {
		// Bản máy chủ chỉ nghe thử được khi máy đọc báo nhận việc này
		// (`capabilities.preview`); không thì UI ẩn nút và endpoint nói thẳng
		// thay vì treo một lời gọi không bao giờ xong (10-audio.md §8.4 #6).
		if !g.hostedPreviewSupported(r.Context()) {
			jsonError(w, http.StatusServiceUnavailable, "Máy đọc chưa hỗ trợ nghe thử")
			return
		}
	}
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
