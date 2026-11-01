package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/identity"
)

// Hosted: /api/tts/* đọc trạng thái do MÁY ĐỌC công bố (plan §4.4). Trong
// container không có bộ đọc cục bộ nào dùng được — máy Mac native tự chuẩn bị
// bộ đọc rồi đẩy trạng thái lên qua worker listener. Vì vậy các endpoint công
// khai giữ nguyên hình dạng cũ, chỉ đổi nguồn số liệu; chế độ file/desktop
// không đổi một dòng nào.

// workerSnapshot là trạng thái mới nhất của máy đọc còn hiệu lực, kèm nhịp sống
// do máy chủ tự suy ra. Revoked = credential đã bị thu hồi: chỉ dùng để Nâng cao
// còn thấy máy đó (trạng thái `error · token_rejected`) thay vì biến mất.
type workerSnapshot struct {
	Found        bool
	Revoked      bool
	ID           pgtype.UUID
	Name         string
	State        voice.ProvisionState
	UpdatedAt    time.Time
	LastSeen     *time.Time
	GrantedAt    time.Time
	RevokedAt    *time.Time
	Capabilities []string
}

// offline là "máy đọc đã ngừng nói chuyện": lần kết nối cuối cũ hơn lease +
// 2 nhịp tim. Không có writer nào đặt giá trị này — nó luôn được suy ra.
func (s workerSnapshot) offline(now time.Time) bool {
	if s.LastSeen == nil {
		return true
	}
	return now.Sub(*s.LastSeen) > postgres.DefaultLease+2*postgres.DefaultHeartbeat
}

// previewSupported: máy đọc tự báo có nhận nghe thử theo yêu cầu hay không.
func (s workerSnapshot) previewSupported() bool {
	return s.Found && !s.Revoked && s.State.Capabilities != nil && s.State.Capabilities.Preview
}

// installedStates là các trạng thái nghĩa là "bộ đọc đã có trên máy".
var installedStates = map[string]bool{
	voice.ProvisionStateProvisioning: true,
	voice.ProvisionStateStarting:     true,
	voice.ProvisionStateReady:        true,
}

// workerSnapshot trả máy đọc có trạng thái mới nhất trong số credential 'tts'
// còn hiệu lực; không còn máy nào sống thì lấy credential đã thu hồi gần nhất để
// Nâng cao vẫn thấy nó (trạng thái `error · token_rejected`).
func (g *Gateway) workerSnapshot(ctx context.Context) workerSnapshot {
	if g.pgAudio == nil || g.pgAudio.Jobs == nil {
		return workerSnapshot{}
	}
	workers, err := g.pgAudio.Jobs.ListWorkers(ctx)
	if err != nil {
		return workerSnapshot{}
	}
	var best postgres.Worker
	found := false
	for _, w := range workers {
		if w.RevokedAt != nil || !hasWorkerCapability(w.Capabilities, "tts") {
			continue
		}
		if !found || fresherWorker(w, best) {
			best, found = w, true
		}
	}
	revoked := false
	if !found {
		for _, w := range workers {
			if w.RevokedAt == nil || !hasWorkerCapability(w.Capabilities, "tts") {
				continue
			}
			if !found || fresherWorker(w, best) {
				best, found, revoked = w, true, true
			}
		}
	}
	if !found {
		return workerSnapshot{}
	}
	snap := workerSnapshot{
		Found: true, Revoked: revoked, ID: best.ID, Name: best.Name,
		LastSeen: best.LastSeenAt, GrantedAt: best.CreatedAt,
		RevokedAt: best.RevokedAt, Capabilities: best.Capabilities,
	}
	if best.StateUpdatedAt != nil {
		snap.UpdatedAt = *best.StateUpdatedAt
	}
	if len(best.State) > 0 {
		var state voice.ProvisionState
		if json.Unmarshal(best.State, &state) == nil {
			snap.State = state
		}
	}
	if snap.State.State == "" {
		snap.State.State = voice.ProvisionStateMissing
		snap.State.Percent = voice.ProvisionPercentUnknown
	}
	return snap
}

// fresherWorker so sánh hai credential theo độ tươi của trạng thái, rồi tới
// lần kết nối cuối (một credential chưa từng đẩy trạng thái vẫn có thể là máy
// đang sống).
func fresherWorker(a, b postgres.Worker) bool {
	switch {
	case a.StateUpdatedAt != nil && b.StateUpdatedAt != nil:
		return a.StateUpdatedAt.After(*b.StateUpdatedAt)
	case a.StateUpdatedAt != nil:
		return true
	case b.StateUpdatedAt != nil:
		return false
	case a.LastSeenAt != nil && b.LastSeenAt != nil:
		return a.LastSeenAt.After(*b.LastSeenAt)
	default:
		return a.LastSeenAt != nil
	}
}

func hasWorkerCapability(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

// hostedTTSStatus dựng GET /api/tts/status từ trạng thái máy đọc (Gói AT).
func (g *Gateway) hostedTTSStatus(ctx context.Context, user identity.User) ttsStatusResponse {
	svc := ttsService()
	caps := svc.Capabilities()
	snap := g.workerSnapshot(ctx)
	now := time.Now()
	resp := ttsStatusResponse{
		Mode:       "hosted",
		Engine:     "remote",
		State:      voice.ProvisionStateMissing,
		Quality:    svc.Config().Quality,
		SampleRate: caps.SampleRate,
		Cloning:    caps.Cloning,
		Qualities:  caps.Qualities,
		FFmpeg:     ffmpegOnPath(),
	}
	if !snap.Found {
		resp.QueueWaiting = g.queuedVoiceChapters(ctx)
		resp.Note = "Chưa có máy đọc nào kết nối"
		return resp
	}
	resp.Machine = &ttsMachine{Name: snap.Name}
	if user.Role == identity.RoleAdmin {
		// Chi tiết kỹ thuật chỉ có ở Tuỳ chỉnh › Nâng cao (10-audio.md §8.5).
		resp.Machine.RevokedAt, resp.Machine.GrantedAt = snap.RevokedAt, &snap.GrantedAt
		resp.Machine.Capabilities = snap.Capabilities
		resp.Channel = snap.State.Channel
		resp.LogTail = workerLogTail(snap.State.Error, 6)
	}
	resp.LastSeenAt = snap.LastSeen
	resp.Capabilities = ttsCapabilities{Preview: snap.previewSupported()}
	resp.Version = snap.State.Version
	resp.Model = snap.State.Model
	resp.DiskBytes = snap.State.DiskBytes
	resp.InstalledBytes = snap.State.DiskBytes
	resp.Installed = installedStates[snap.State.State]
	if snap.State.RTF > 0 {
		rtf := snap.State.RTF
		resp.RTF = &rtf
	}
	if snap.Revoked {
		// Credential đã bị thu hồi: máy không còn được nhận việc, và câu 8.2
		// `token_rejected` nói đúng việc phải làm (cấp lại quyền ở Nâng cao).
		resp.State = voice.ProvisionStateError
		resp.Error = &ttsStatusError{Code: voice.ProvisionErrTokenRejected, Message: provisionReason(voice.ProvisionErrTokenRejected)}
		resp.QueueWaiting = g.queuedVoiceChapters(ctx)
		return resp
	}
	if snap.offline(now) {
		resp.State = voice.ProvisionStateOffline
		resp.Note = "Máy đọc chưa kết nối"
		resp.QueueWaiting = g.queuedVoiceChapters(ctx)
		return resp
	}
	resp.Note = "Máy đọc đang kết nối"
	switch snap.State.State {
	case voice.ProvisionStateReady:
		resp.State, resp.Ready = voice.ProvisionStateReady, true
	case voice.ProvisionStateProvisioning:
		resp.State = voice.ProvisionStateProvisioning
		resp.Step, resp.Percent = snap.State.Step, statePercent(snap.State.Percent)
		resp.BytesDone, resp.BytesTotal, resp.EtaSec = snap.State.BytesDone, snap.State.BytesTotal, snap.State.EtaSec
	case voice.ProvisionStateStarting:
		resp.State, resp.Step = voice.ProvisionStateStarting, voice.ProvisionStepStart
	case voice.ProvisionStateError:
		resp.State = voice.ProvisionStateError
		resp.Error = statusErrorOf(snap.State)
	default:
		resp.State = voice.ProvisionStateMissing
	}
	if resp.State != voice.ProvisionStateReady {
		resp.QueueWaiting = g.queuedVoiceChapters(ctx)
	}
	return resp
}

// statePercent đổi sentinel -1 ("bước không đo được") thành nil: UI không vẽ %.
func statePercent(v int) *int {
	if v < 0 || v > 100 {
		return nil
	}
	return &v
}

// statusErrorOf lấy mã + câu của một lượt chuẩn bị hỏng. Người dùng thường chỉ
// thấy đúng câu 8.2 của mã lỗi; lý do kỹ thuật (đường dẫn, URL, traceback của
// uv) chỉ đi qua logTail ở Nâng cao. Thiếu mã nghĩa là máy đọc không nói rõ lý
// do — quy về start_failed, đúng câu "không khởi động được".
func statusErrorOf(st voice.ProvisionState) *ttsStatusError {
	code := voice.ProvisionErrStartFailed
	if st.Error != nil && strings.TrimSpace(st.Error.Code) != "" {
		code = st.Error.Code
	}
	return &ttsStatusError{Code: code, Message: provisionReason(code)}
}

// provisionPublicReason là câu 8.2 cho từng mã lỗi — toàn bộ phần người dùng
// thường thấy. Không suy ra câu này từ lỗi thật: lỗi thật mang đường dẫn, URL
// và traceback của tiến trình, những thứ chỉ thuộc Nâng cao (10-audio.md §8.5).
var provisionPublicReason = map[string]string{
	voice.ProvisionErrNoDisk:         "Máy đọc không đủ chỗ trống để chuẩn bị bộ đọc. Dọn bớt tệp trên máy đọc rồi thử lại.",
	voice.ProvisionErrDownloadFailed: "Mạng của máy đọc bị ngắt giữa chừng. Máy đọc tự thử lại sau ít phút; bấm Thử lại để thử ngay.",
	voice.ProvisionErrUVFailed:       "Mạng của máy đọc bị ngắt giữa chừng. Máy đọc tự thử lại sau ít phút; bấm Thử lại để thử ngay.",
	voice.ProvisionErrHashMismatch:   "Tệp vừa tải khác bản Noverse đã kiểm nên đã bị bỏ để giữ an toàn. Máy đọc không tự thử tiếp; thử lại khi mạng ổn định, vẫn lỗi thì báo quản trị.",
	voice.ProvisionErrDepsFailed:     "Bước cài thư viện cho bộ đọc dừng giữa chừng, thường do mạng chập chờn. Phần đã tải được giữ lại nên lần sau nhanh hơn.",
	voice.ProvisionErrModelFailed:    "Mẫu giọng (khoảng 580 MB) mới tải được một phần. Phần đã tải được giữ, bấm Thử lại để tải tiếp.",
	voice.ProvisionErrStartFailed:    "Bộ đọc đã chuẩn bị xong nhưng không bật lên được, thường do máy đọc thiếu bộ nhớ. Đóng bớt ứng dụng trên máy đọc rồi thử lại.",
	voice.ProvisionErrTokenRejected:  "Máy đọc này không còn được nhận việc tạo giọng. Quản trị cấp lại quyền ở Nâng cao.",
	// Kết quả chưa rõ, không được khẳng định bộ đọc đã khởi động thất bại.
	voice.ProvisionErrInterrupted: "Máy đọc dừng giữa chừng khi chuẩn bị bộ đọc nên chưa rõ kết quả. Phần đã xong được giữ nguyên; bấm Thử lại để làm tiếp.",
}

// provisionReason trả câu 8.2 của mã; mã lạ rơi về câu "không khởi động được"
// thay vì để lộ chuỗi kỹ thuật của máy đọc.
func provisionReason(code string) string {
	if reason, ok := provisionPublicReason[code]; ok {
		return reason
	}
	return provisionPublicReason[voice.ProvisionErrStartFailed]
}

// workerLogTail gom chẩn đoán của máy đọc cho Nâng cao: lý do kỹ thuật trước
// (có thể mang đường dẫn/URL/traceback), rồi tới n dòng nhật ký gần nhất. Đây
// là chỗ DUY NHẤT những chuỗi đó được ra khỏi máy chủ.
func workerLogTail(pe *voice.ProvisionError, n int) []string {
	if pe == nil || n <= 0 {
		return nil
	}
	out := workerLines(pe.Message)
	if len(out) > n {
		out = out[len(out)-n:]
	}
	tail := workerLines(pe.LogTail)
	if room := n - len(out); room > 0 {
		if len(tail) > room {
			tail = tail[len(tail)-room:]
		}
		out = append(out, tail...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// workerLines tách một chuỗi chẩn đoán thành dòng, bỏ dòng trống.
func workerLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(line))
	}
	return out
}

// hostedPreviewSupported: máy đọc báo nhận nghe thử thì endpoint mới đi tiếp
// (đường đọc vẫn là engine đã cấu hình, không phải một kênh riêng).
func (g *Gateway) hostedPreviewSupported(ctx context.Context) bool {
	return g.workerSnapshot(ctx).previewSupported()
}

// queuedVoiceChapters đếm số chương tạo giọng đang xếp hàng trên toàn máy chủ:
// đó là số việc người dùng đang mất khi máy đọc chưa sẵn sàng (cờ Cảnh báo của
// thẻ Máy đọc và `waitingMachine` của từng việc).
func (g *Gateway) queuedVoiceChapters(ctx context.Context) int {
	if g.pgAudio == nil || g.pgAudio.Jobs == nil {
		return 0
	}
	n, err := g.pgAudio.Jobs.CountQueued(ctx, postgres.JobTTS)
	if err != nil {
		return 0
	}
	return n
}

// voiceMachineReady là "có máy đọc đang sẵn sàng tạo giọng" — trạng thái ready
// VÀ đã kết nối. Bản cá nhân tự đọc trên máy này nên luôn có thể nhận việc;
// hàng/chương xếp hàng không cần cờ waitingMachine ở đó.
func (g *Gateway) voiceMachineReady(ctx context.Context) bool {
	if g.pgAudio == nil {
		return true
	}
	snap := g.workerSnapshot(ctx)
	return snap.Found && !snap.Revoked && !snap.offline(time.Now()) &&
		snap.State.State == voice.ProvisionStateReady
}

// hostedInstallProgress đổi trạng thái máy đọc sang InstallProgress — đúng
// hình dạng UI đã đọc/nghe, nên thanh % có sẵn mà không phải sửa web.
func hostedInstallProgress(snap workerSnapshot, now time.Time) voice.InstallProgress {
	if !snap.Found {
		return voice.InstallProgress{State: "idle"}
	}
	p := voice.InstallProgress{
		Step:       snap.State.Step,
		Percent:    statePercent(snap.State.Percent),
		BytesDone:  snap.State.BytesDone,
		BytesTotal: snap.State.BytesTotal,
		EtaSec:     snap.State.EtaSec,
		Version:    snap.State.Version,
		Message:    snap.State.Message,
		Updated:    snap.State.UpdatedAt,
	}
	switch snap.State.State {
	case voice.ProvisionStateReady:
		p.State, p.Percent = "done", new(100)
	case voice.ProvisionStateError:
		p.State = "error"
		if snap.State.Error != nil {
			p.Error, p.ErrorCode = snap.State.Error.Message, snap.State.Error.Code
			if p.Error == "" {
				p.Error = snap.State.Error.Code
			}
		}
	case voice.ProvisionStateProvisioning, voice.ProvisionStateStarting:
		p.State = "running"
	default:
		// Chưa từng chuẩn bị: không có hàng nào trong dock để nói.
		p.State = "idle"
	}
	if snap.Revoked && p.State != "idle" {
		p.State = "error"
		p.ErrorCode = voice.ProvisionErrTokenRejected
		p.Error = "Máy đọc đã bị thu hồi quyền"
		p.Percent = nil
	}
	if snap.offline(now) && (p.State == "running" || snap.State.State == voice.ProvisionStateProvisioning) {
		p.State = "error"
		p.ErrorCode = voice.ProvisionErrStartFailed
		p.Error = "Máy đọc chưa kết nối"
		p.Percent = nil
	}
	if p.Updated.IsZero() {
		p.Updated = now
	}
	return p
}

// hostedTTSInstall queues the install command on the worker row. The Mac polls
// it within seconds, so the button keeps its old contract (202 running) without
// any endpoint reaching into the machine.
func (g *Gateway) hostedTTSInstall(w http.ResponseWriter, r *http.Request, user identity.User) {
	if user.Role != identity.RoleAdmin {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	ctx := r.Context()
	snap := g.workerSnapshot(ctx)
	now := time.Now()
	if snap.Found && !snap.offline(now) && snap.State.State == voice.ProvisionStateReady {
		writeJSON(w, http.StatusOK, map[string]any{"job": "", "state": "done", "message": "Bộ đọc trên máy đã có sẵn"})
		return
	}
	if !snap.Found {
		jsonError(w, http.StatusConflict, "Chưa có máy đọc nào kết nối — khởi động máy đọc rồi thử lại")
		return
	}
	if _, err := g.pgAudio.Jobs.BumpWorkerCommand(ctx, snap.ID, "install"); err != nil {
		jsonError(w, http.StatusInternalServerError, "Không gửi được lệnh chuẩn bị bộ đọc tới máy đọc")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": voiceInstallJobID, "state": "running"})
}

// hostedTTSInstallState is the polling fallback of the install progress.
func (g *Gateway) hostedTTSInstallState(ctx context.Context) voice.InstallProgress {
	return hostedInstallProgress(g.workerSnapshot(ctx), time.Now())
}

// hostedTTSInstallEvents streams the install progress derived from the worker
// state, one event per change, closing on done/error like the local SSE.
func (g *Gateway) hostedTTSInstallEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, http.StatusInternalServerError, "Máy chủ không hỗ trợ luồng sự kiện")
		return
	}
	send := func(p voice.InstallProgress) bool {
		data, err := json.Marshal(p)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "event: tts.install\ndata: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	ctx := r.Context()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	var last voice.InstallProgress
	have := false
	for {
		p := hostedInstallProgress(g.workerSnapshot(ctx), time.Now())
		if !have || !p.Same(last) {
			if !send(p) {
				return
			}
			last, have = p, true
		}
		if p.State == "done" || p.State == "error" {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// hostedTTSEngines exposes a single "remote" engine: writing tts.engine into the
// container's config would be meaningless — the reader lives on the Mac.
func (g *Gateway) hostedTTSEngines(ctx context.Context) map[string]any {
	caps := ttsService().Capabilities()
	snap := g.workerSnapshot(ctx)
	info := ttsEngineInfo{
		ID:           "remote",
		Name:         "Máy đọc của bạn",
		Description:  "Bộ đọc chạy trên máy của bạn và nối tới máy chủ này; máy chủ không tự đọc.",
		Installed:    snap.Found && installedStates[snap.State.State],
		Capabilities: caps,
	}
	return map[string]any{"engines": []ttsEngineInfo{info}, "active": "remote"}
}

func ffmpegOnPath() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
