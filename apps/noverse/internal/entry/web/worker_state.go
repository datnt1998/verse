package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Kênh trạng thái worker (plan §4.4): máy đọc công bố nó đang làm gì, máy chủ
// trả lại lệnh mà quản trị viên đã bấm. Cả hai đi qua credential worker — không
// dùng phiên người dùng, không có đường nào từ ngoài vào máy Mac.
//
// `offline` KHÔNG bao giờ được lưu: nó suy ra từ last_seen_at so với lease +
// 2 nhịp tim, nên một máy Mac đang ngủ không cần ai ghi gì.

// WorkerStateStore is the worker-row surface the private listener needs on top
// of the job queue: the published provision state and the queued command.
type WorkerStateStore interface {
	SaveWorkerState(ctx context.Context, workerID string, state []byte) error
	WorkerCommand(ctx context.Context, workerID string) (seq int64, op string, err error)
}

// workerStateBody is the POST /internal/worker/state payload: the shared
// provision state plus what only the Mac itself can answer.
type workerStateBody struct {
	voice.ProvisionState
	FFmpeg bool `json:"ffmpeg"`
}

var workerStateValues = map[string]bool{
	voice.ProvisionStateMissing:      true,
	voice.ProvisionStateProvisioning: true,
	voice.ProvisionStateStarting:     true,
	voice.ProvisionStateReady:        true,
	voice.ProvisionStateError:        true,
}

var workerStepValues = map[string]bool{
	voice.ProvisionStepTools:  true,
	voice.ProvisionStepPython: true,
	voice.ProvisionStepSource: true,
	voice.ProvisionStepDeps:   true,
	voice.ProvisionStepModel:  true,
}

// normalized validates the payload and returns the JSON to store. An unknown
// state or step is refused: this column feeds the public status API, so a typo
// must fail loudly here instead of becoming a UI state nobody renders.
func (b workerStateBody) normalized() ([]byte, error) {
	if !workerStateValues[b.State] {
		return nil, fmt.Errorf("trạng thái máy đọc không hợp lệ: %q", b.State)
	}
	if b.Step != "" && !workerStepValues[b.Step] {
		return nil, fmt.Errorf("bước chuẩn bị không hợp lệ: %q", b.Step)
	}
	if b.Percent < voice.ProvisionPercentUnknown || b.Percent > 100 {
		return nil, fmt.Errorf("phần trăm ngoài khoảng: %d", b.Percent)
	}
	if b.Error != nil {
		if strings.TrimSpace(b.Error.Code) == "" {
			return nil, fmt.Errorf("lỗi chuẩn bị phải có mã")
		}
		b.Error.LogTail = clipWorkerLogTail(b.Error.LogTail)
	}
	b.Message = strings.TrimSpace(b.Message)
	b.Model = strings.TrimSpace(b.Model)
	b.Version = strings.TrimSpace(b.Version)
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = time.Now().UTC()
	}
	return json.Marshal(b)
}

// clipWorkerLogTail bounds the log tail the UI may show in Nâng cao. The cut is
// moved forward over UTF-8 continuation bytes so a Vietnamese line never breaks.
func clipWorkerLogTail(s string) string {
	const limit = 600
	if len(s) <= limit {
		return s
	}
	cut := s[len(s)-limit:]
	for len(cut) > 0 && cut[0]&0xC0 == 0x80 {
		cut = cut[1:]
	}
	return cut
}

// handleState stores the worker's provision state (POST /internal/worker/state).
func (a *WorkerAPI) handleState(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var body workerStateBody
	if !decodeJSON(w, r, &body) {
		return
	}
	raw, err := body.normalized()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.States == nil {
		jsonError(w, http.StatusNotImplemented, "Máy chủ chưa bật kênh trạng thái máy đọc")
		return
	}
	if err := a.States.SaveWorkerState(r.Context(), worker.ID, raw); err != nil {
		writeWorkerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCommand returns the operator's queued command (GET /internal/worker/command).
// The worker keeps the last seq it ran, so the same row is never executed twice.
func (a *WorkerAPI) handleCommand(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	if a.States == nil {
		jsonError(w, http.StatusNotImplemented, "Máy chủ chưa bật kênh trạng thái máy đọc")
		return
	}
	seq, op, err := a.States.WorkerCommand(r.Context(), worker.ID)
	if err != nil {
		writeWorkerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "op": op})
}

// persistHeartbeatState stores the state a heartbeat carries, if any. A long job
// is the only traffic for minutes, so this is what keeps the reader's state (and
// its freshness) truthful while work runs. Best effort: a state write must never
// fail a heartbeat.
func (a *WorkerAPI) persistHeartbeatState(ctx context.Context, workerID string, raw json.RawMessage) {
	if len(raw) == 0 || a.States == nil {
		return
	}
	var body workerStateBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return
	}
	if payload, err := body.normalized(); err == nil {
		_ = a.States.SaveWorkerState(ctx, workerID, payload)
	}
}
