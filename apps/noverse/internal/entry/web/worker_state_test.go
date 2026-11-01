package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// stubStates is a scriptable WorkerStateStore: it records what the worker
// published and answers with the queued command, so the HTTP contract is tested
// without a database (the durable behaviour lives in the postgres tests).
type stubStates struct {
	mu       sync.Mutex
	workerID string
	state    []byte
	saves    int
	seq      int64
	op       string
	saveErr  error
	cmdErr   error
}

func (s *stubStates) SaveWorkerState(_ context.Context, workerID string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.workerID, s.state, s.saves = workerID, append([]byte(nil), state...), s.saves+1
	return nil
}

func (s *stubStates) WorkerCommand(context.Context, string) (int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seq, s.op, s.cmdErr
}

func newStateWorkerAPI(t *testing.T, states WorkerStateStore) *WorkerAPI {
	t.Helper()
	api, _ := newTestWorkerAPI(t, &stubQueue{})
	api.States = states
	return api
}

// The state channel rides the same worker credential as the job queue: no
// session, no admin token, and a revoked credential stops publishing.
func TestWorkerStateEndpointAuth(t *testing.T) {
	states := &stubStates{}
	api := newStateWorkerAPI(t, states)
	h := api.Handler()
	body := `{"state":"ready","percent":100}`

	cases := []struct {
		name  string
		token string
		auth  func([]byte) (WorkerIdentity, error)
		want  int
	}{
		{name: "no token", want: http.StatusUnauthorized},
		{name: "unknown token", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{}, errors.New("không có worker")
		}, want: http.StatusUnauthorized},
		{name: "revoked", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{}, errWorkerFencedOut
		}, want: http.StatusUnauthorized},
		{name: "no tts capability", token: testWorkerToken, auth: func([]byte) (WorkerIdentity, error) {
			return WorkerIdentity{ID: "w", Capabilities: []string{"export"}}, nil
		}, want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQueue{authenticate: tc.auth}
			api, _ := newTestWorkerAPI(t, q)
			api.States = states
			rr := httptest.NewRecorder()
			api.Handler().ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/state", tc.token, strings.NewReader(body)))
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
	if states.saves != 0 {
		t.Fatalf("rejected requests stored %d state(s)", states.saves)
	}

	// The authenticated path stores the state and answers 204.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/state", testWorkerToken, strings.NewReader(body)))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", rr.Code, rr.Body.String())
	}
	states.mu.Lock()
	defer states.mu.Unlock()
	if states.saves != 1 || states.workerID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("stored %d state(s) for %q", states.saves, states.workerID)
	}
	var got voice.ProvisionState
	if err := json.Unmarshal(states.state, &got); err != nil {
		t.Fatalf("stored json %q: %v", states.state, err)
	}
	if got.State != voice.ProvisionStateReady || got.Percent != 100 {
		t.Fatalf("stored state = %+v", got)
	}
}

// The public status API renders whatever the worker published, so the listener
// refuses states it could not render.
func TestWorkerStateEndpointValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "unknown state", body: `{"state":"sleeping","percent":10}`, want: http.StatusBadRequest},
		{name: "missing state", body: `{"percent":10}`, want: http.StatusBadRequest},
		{name: "unknown step", body: `{"state":"provisioning","step":"vibes","percent":10}`, want: http.StatusBadRequest},
		{name: "percent too high", body: `{"state":"provisioning","percent":140}`, want: http.StatusBadRequest},
		{name: "error without code", body: `{"state":"error","error":{"message":"hỏng"}}`, want: http.StatusBadRequest},
		{name: "unknown field", body: `{"state":"ready","bogus":1}`, want: http.StatusBadRequest},
		{name: "valid provisioning", body: `{"state":"provisioning","step":"deps","percent":42,"message":"Đang cài thư viện…"}`, want: http.StatusNoContent},
		{name: "valid error", body: `{"state":"error","error":{"code":"download_failed","message":"Mạng hỏng"},"percent":-1}`, want: http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			states := &stubStates{}
			api := newStateWorkerAPI(t, states)
			rr := httptest.NewRecorder()
			api.Handler().ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/state", testWorkerToken, strings.NewReader(tc.body)))
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rr.Code, tc.want, rr.Body.String())
			}
			if tc.want != http.StatusNoContent && states.saves != 0 {
				t.Fatalf("invalid payload stored anyway: %q", states.state)
			}
		})
	}
}

// GET /internal/worker/command answers {seq, op} so the worker can tell a fresh
// click from one it already ran.
func TestWorkerCommandEndpoint(t *testing.T) {
	states := &stubStates{seq: 4, op: "install"}
	api := newStateWorkerAPI(t, states)
	rr := httptest.NewRecorder()
	api.Handler().ServeHTTP(rr, workerReq(http.MethodGet, "/internal/worker/command", testWorkerToken, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	var got struct {
		Seq int64  `json:"seq"`
		Op  string `json:"op"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q: %v", rr.Body.String(), err)
	}
	if got.Seq != 4 || got.Op != "install" {
		t.Fatalf("command = %+v, want {4 install}", got)
	}

	// No store configured: the listener still serves jobs, so the channel says so.
	api, _ = newTestWorkerAPI(t, &stubQueue{})
	rr = httptest.NewRecorder()
	api.Handler().ServeHTTP(rr, workerReq(http.MethodGet, "/internal/worker/command", testWorkerToken, nil))
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rr.Code)
	}
}

// A long job's heartbeat is the only traffic for minutes, so it carries the
// reader state; the listener stores it.
func TestWorkerHeartbeatPersistsState(t *testing.T) {
	states := &stubStates{}
	q := &stubQueue{heartbeat: func(_, _ string, fence int64, _ time.Duration) (WorkerJob, error) {
		return WorkerJob{ID: "j", Fence: fence}, nil
	}}
	api, _ := newTestWorkerAPI(t, q)
	api.States = states
	body := `{"fence":2,"leaseSec":60,"state":{"state":"ready","percent":100,"model":"vieneu-1"}}`
	rr := httptest.NewRecorder()
	api.Handler().ServeHTTP(rr, workerReq(http.MethodPost, "/internal/worker/jobs/j/heartbeat", testWorkerToken, strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	states.mu.Lock()
	defer states.mu.Unlock()
	if states.saves != 1 {
		t.Fatalf("heartbeat stored %d state(s), want 1", states.saves)
	}
	var got voice.ProvisionState
	if err := json.Unmarshal(states.state, &got); err != nil {
		t.Fatalf("stored json: %v", err)
	}
	if got.State != voice.ProvisionStateReady || got.Model != "vieneu-1" {
		t.Fatalf("stored state = %+v", got)
	}
}
