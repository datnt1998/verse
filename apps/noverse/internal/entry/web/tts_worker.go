package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
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
// do máy chủ tự suy ra.
type workerSnapshot struct {
	Found     bool
	ID        pgtype.UUID
	Name      string
	State     voice.ProvisionState
	UpdatedAt time.Time
	LastSeen  *time.Time
}

// offline là "máy đọc đã ngừng nói chuyện": lần kết nối cuối cũ hơn lease +
// 2 nhịp tim. Không có writer nào đặt giá trị này — nó luôn được suy ra.
func (s workerSnapshot) offline(now time.Time) bool {
	if s.LastSeen == nil {
		return true
	}
	return now.Sub(*s.LastSeen) > postgres.DefaultLease+2*postgres.DefaultHeartbeat
}

// installedStates là các trạng thái nghĩa là "bộ đọc đã có trên máy".
var installedStates = map[string]bool{
	voice.ProvisionStateProvisioning: true,
	voice.ProvisionStateStarting:     true,
	voice.ProvisionStateReady:        true,
}

// hostedStateNames đổi enum worker sang enum trạng thái engine mà UI đã dùng.
var hostedStateNames = map[string]string{
	voice.ProvisionStateMissing:      voice.StateMissing,
	voice.ProvisionStateProvisioning: voice.StateInstalling,
	voice.ProvisionStateStarting:     voice.StateStarting,
	voice.ProvisionStateReady:        voice.StateReady,
	voice.ProvisionStateError:        voice.StateError,
}

// workerSnapshot trả máy đọc có trạng thái mới nhất trong số credential 'tts'
// còn hiệu lực.
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
	if !found {
		return workerSnapshot{}
	}
	snap := workerSnapshot{Found: true, ID: best.ID, Name: best.Name, LastSeen: best.LastSeenAt}
	if best.StateUpdatedAt != nil {
		snap.UpdatedAt = *best.StateUpdatedAt
	}
	if len(best.State) > 0 {
		_ = json.Unmarshal(best.State, &snap.State)
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

// hostedTTSStatus dựng GET /api/tts/status từ trạng thái máy đọc.
func (g *Gateway) hostedTTSStatus(ctx context.Context) ttsStatusResponse {
	svc := ttsService()
	caps := svc.Capabilities()
	snap := g.workerSnapshot(ctx)
	now := time.Now()
	resp := ttsStatusResponse{
		Engine:     "remote",
		State:      voice.StateMissing,
		Quality:    svc.Config().Quality,
		SampleRate: caps.SampleRate,
		Cloning:    caps.Cloning,
		Qualities:  caps.Qualities,
		FFmpeg:     ffmpegOnPath(),
	}
	if !snap.Found {
		resp.Note = "Chưa có máy đọc từ xa nào kết nối"
		return resp
	}
	resp.Model = snap.State.Model
	resp.Installed = installedStates[snap.State.State]
	resp.InstalledBytes = snap.State.DiskBytes
	if snap.State.RTF > 0 {
		rtf := snap.State.RTF
		resp.RTF = &rtf
	}
	if snap.offline(now) {
		resp.State = voice.ProvisionStateOffline
		if snap.LastSeen != nil {
			resp.Note = "Máy đọc từ xa lần cuối kết nối lúc " + snap.LastSeen.Format("15:04")
		} else {
			resp.Note = "Chưa có máy đọc từ xa nào kết nối"
		}
		return resp
	}
	resp.Note = "Máy đọc từ xa đang kết nối"
	resp.State = hostedStateNames[snap.State.State]
	if resp.State == "" {
		resp.State = voice.StateMissing
	}
	resp.Ready = snap.State.State == voice.ProvisionStateReady
	if snap.State.Error != nil {
		resp.Error = snap.State.Error.Message
		if resp.Error == "" {
			resp.Error = snap.State.Error.Code
		}
	}
	return resp
}

// hostedInstallProgress đổi trạng thái máy đọc sang InstallProgress — đúng
// hình dạng UI đã đọc/nghe, nên thanh % có sẵn mà không phải sửa web.
func hostedInstallProgress(snap workerSnapshot, now time.Time) voice.InstallProgress {
	if !snap.Found {
		return voice.InstallProgress{State: "idle"}
	}
	p := voice.InstallProgress{
		Step:    snap.State.Step,
		Percent: snap.State.Percent,
		Message: snap.State.Message,
		Bytes:   snap.State.DiskBytes,
		Updated: snap.State.UpdatedAt,
	}
	if p.Percent < 0 {
		p.Percent = 0
	}
	switch snap.State.State {
	case voice.ProvisionStateReady:
		p.State, p.Percent = "done", 100
	case voice.ProvisionStateError:
		p.State = "error"
		if snap.State.Error != nil {
			p.Error = snap.State.Error.Message
			if p.Error == "" {
				p.Error = snap.State.Error.Code
			}
		}
	case voice.ProvisionStateProvisioning, voice.ProvisionStateStarting:
		p.State = "running"
	default:
		p.State = "idle"
	}
	if snap.offline(now) && p.State != "done" {
		p.State = "error"
		if p.Error == "" {
			p.Error = "Máy đọc chưa kết nối"
		}
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
		if !have || p != last {
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
