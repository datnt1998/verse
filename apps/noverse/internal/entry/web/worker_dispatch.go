package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

// actionAckWindow lets deterministic precondition errors (already running,
// exclusive job, budget) return as HTTP 409 while model-backed work continues
// in the background and reports through the worker event stream.
var actionAckWindow = 250 * time.Millisecond

var errActionBusy = errors.New("Đang xử lý yêu cầu trước, vui lòng chờ")

// dispatchAction runs one engine-control action without tying the HTTP request
// to model calls. pending=true means fn is still running; its failure, if any,
// is emitted as an ERROR event.
func (s *workerServer) dispatchAction(pendingSummary string, fn func() error) (pending bool, err error) {
	s.actionMu.Lock()
	if s.actionBusy {
		s.actionMu.Unlock()
		return false, errActionBusy
	}
	s.actionBusy = true
	s.actionMu.Unlock()

	result := make(chan error, 1)
	go func() {
		err := fn()
		s.actionMu.Lock()
		s.actionBusy = false
		s.actionMu.Unlock()
		result <- err
	}()
	timer := time.NewTimer(actionAckWindow)
	defer timer.Stop()
	select {
	case err := <-result:
		return false, err
	case <-timer.C:
	}
	s.emitWorkerEvent("SYSTEM", "info", pendingSummary)
	go func() {
		if err := <-result; err != nil {
			s.emitWorkerEvent("ERROR", "error", "Không thực hiện được yêu cầu: "+err.Error())
		}
		s.b.emit(message{kind: "snapshot", data: s.snapshot()})
	}()
	return true, nil
}

func (s *workerServer) emitWorkerEvent(category, level, summary string) {
	s.b.emit(message{kind: "event", data: host.Event{Time: time.Now(), Category: category, Summary: summary, Level: level}})
}

// writeDispatched writes 202 while an action continues in the background, or
// the final synchronous result/error when it finished inside the ack window.
func writeDispatched(w http.ResponseWriter, pending bool, err error, body map[string]any) {
	if err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	if pending {
		body["pending"] = true
		writeJSON(w, http.StatusAccepted, body)
		return
	}
	writeJSON(w, http.StatusOK, body)
}
