package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

type coCreateProposalResponse struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Active        bool                   `json:"active"`
	Committed     bool                   `json:"committed"`
	NextChapter   *int                   `json:"nextChapter"`
	Proposal      *host.CoCreateProposal `json:"proposal"`
	Messages      []host.CoCreateMessage `json:"messages"`
	Action        string                 `json:"action"`
}

func proposalRequest(t *testing.T, s *workerServer, method, path, body string) coCreateProposalResponse {
	t.Helper()
	w := doReq(t, s, method, path, body)
	if w.Code != 200 {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
	}
	var out coCreateProposalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.SchemaVersion != 1 {
		t.Fatalf("missing response schemaVersion: %s", w.Body.String())
	}
	return out
}

func TestCoCreateProposalPersistence(t *testing.T) {
	for _, stage := range []bool{false, true} {
		dir := t.TempDir()
		h := newLiveFakeHost()
		if stage {
			h.snapshot = host.UISnapshot{Phase: "writing"}
		}
		h.coReply = host.CoCreateReply{Message: "Đề xuất", Prompt: "## Hướng đi", Ready: true,
			Raw:      `<reply>Đề xuất</reply><proposal><title>Hướng đi</title></proposal>`,
			Proposal: &host.CoCreateProposal{SchemaVersion: 1, Title: "Hướng đi", Chapters: []host.CoCreateProposalChapter{{SchemaVersion: 1, Chapter: 25, Title: "Cánh cửa", Summary: "Nhân vật chọn hướng đi."}}, Impacts: []string{"Sửa dàn ý ch.25"}}}
		s := newLiveServer(h)
		s.storyDir = dir
		empty := proposalRequest(t, s, "GET", "/api/cocreate", "")
		if empty.Proposal != nil || empty.Committed || empty.NextChapter != nil {
			t.Fatalf("fabricated empty state: %+v", empty)
		}
		first := proposalRequest(t, s, "POST", "/api/cocreate/message", `{"message":"Bàn hướng đi"}`)
		if first.Proposal == nil || first.Proposal.Revision != 1 || first.NextChapter == nil || *first.NextChapter != 25 || first.Committed {
			t.Fatalf("missing real v1 proposal: %+v", first)
		}
		h.coReply.Proposal.Title = "Hướng đã chỉnh"
		second := proposalRequest(t, s, "POST", "/api/cocreate/message", `{"message":"Chỉnh thêm"}`)
		if second.Proposal.Revision != 2 || second.Proposal.Title != "Hướng đã chỉnh" {
			t.Fatalf("feedback proposal not v2: %+v", second.Proposal)
		}
		// A fresh worker must restore both model history and the stage hold.
		restoredHost := newLiveFakeHost()
		restoredHost.snapshot = h.snapshot
		restored := newLiveServer(restoredHost)
		restored.storyDir = dir
		if snapshot := decodeLiveBody(t, doReq(t, restored, "GET", "/api/snapshot", "")); snapshot["waitingFor"] != "cocreate" {
			t.Fatalf("first snapshot did not restore pending proposal: %+v", snapshot)
		}
		state := proposalRequest(t, restored, "GET", "/api/cocreate", "")
		if !state.Active || state.Proposal.Revision != 2 || len(state.Messages) != 4 || state.Messages[1].Content != "Đề xuất" {
			t.Fatalf("restart lost proposal/history: %+v", state)
		}
		committed := proposalRequest(t, restored, "POST", "/api/cocreate/commit", `{}`)
		wantAction := "start"
		if stage {
			wantAction = "resume"
		}
		if !committed.Committed || committed.Action != wantAction || committed.Proposal.Revision != 2 || *committed.NextChapter != 25 {
			t.Fatalf("commit lost proposal: %+v", committed)
		}
		if restored.snapshot().WaitingFor == "cocreate" {
			t.Fatal("committed proposal must not hold snapshot in cocreate")
		}
		if stage && callsJoined(restoredHost) != "pauseCoCreate,resumeCoCreate" {
			t.Fatalf("restart did not restore actual host stage hold: %s", callsJoined(restoredHost))
		}
		afterRestartHost := newLiveFakeHost()
		afterRestartHost.coReply = host.CoCreateReply{Message: "Cần thêm thông tin"}
		afterRestart := newLiveServer(afterRestartHost)
		afterRestart.storyDir = dir
		state = proposalRequest(t, afterRestart, "GET", "/api/cocreate", "")
		if state.Active || !state.Committed || state.Proposal.Revision != 2 || len(state.Messages) != 4 || *state.NextChapter != 25 {
			t.Fatalf("committed proposal not retained on restart: %+v", state)
		}
		if w := doReq(t, afterRestart, "POST", "/api/cocreate/commit", `{}`); w.Code != 409 {
			t.Fatalf("committed proposal started engine twice: %d %s", w.Code, w.Body.String())
		}
		reopened := proposalRequest(t, afterRestart, "POST", "/api/cocreate/message", `{"message":"Bàn hướng khác"}`)
		if reopened.Committed || reopened.Proposal != nil || reopened.NextChapter != nil {
			t.Fatalf("new session reused committed proposal: %+v", reopened)
		}
		state = proposalRequest(t, afterRestart, "GET", "/api/cocreate", "")
		if !state.Active || len(state.Messages) != 2 {
			t.Fatalf("new session reused old history: %+v", state)
		}
		proposalRequest(t, afterRestart, "POST", "/api/cocreate/cancel", `{}`)
		if _, err := os.Stat(filepath.Join(dir, coCreateSessionPath)); !os.IsNotExist(err) {
			t.Fatalf("cancel retained persisted state: %v", err)
		}
		state = proposalRequest(t, afterRestart, "GET", "/api/cocreate", "")
		if state.Active || state.Committed || state.Proposal != nil || len(state.Messages) != 0 {
			t.Fatalf("cancel retained proposal: %+v", state)
		}
	}
}

func TestCoCreateProposalStorageErrors(t *testing.T) {
	s := newLiveServer(newLiveFakeHost())
	s.storyDir = t.TempDir()
	// The cause of a 500 is a server-side fact: operator logs keep it, the body
	// is the one generic sentence (httperr.go policy), so a client never learns
	// paths or SQL text from a failure it cannot act on.
	logs := captureLogs(t)
	expectInternal := func(t *testing.T, w *httptest.ResponseRecorder, cause string) {
		t.Helper()
		if w.Code != 500 {
			t.Fatalf("status = %d, want 500: %s", w.Code, w.Body.String())
		}
		if msg := errBody(t, w); msg != errInternalMessage {
			t.Fatalf("body = %q, want generic %q", msg, errInternalMessage)
		}
		if !strings.Contains(logs.String(), cause) {
			t.Fatalf("cause %q not logged server-side: %s", cause, logs.String())
		}
	}
	proposalRequest(t, s, "GET", "/api/cocreate", "")
	if err := os.WriteFile(filepath.Join(s.storyDir, ".ainovel"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := doReq(t, s, "POST", "/api/cocreate/message", `{"message":"Bàn hướng đi"}`)
	expectInternal(t, w, "Không thể lưu phiên đồng sáng tác")
	corrupt := newLiveServer(newLiveFakeHost())
	corrupt.storyDir = t.TempDir()
	mustWriteFile(t, filepath.Join(corrupt.storyDir, coCreateSessionPath), `{broken`)
	w = doReq(t, corrupt, "GET", "/api/cocreate", "")
	expectInternal(t, w, "Không thể đọc phiên đồng sáng tác")
	mustWriteFile(t, filepath.Join(corrupt.storyDir, coCreateSessionPath), `{"schemaVersion":2}`)
	w = doReq(t, corrupt, "GET", "/api/cocreate", "")
	expectInternal(t, w, "phiên bản không được hỗ trợ")
	cannotCancel := newLiveServer(newLiveFakeHost())
	cannotCancel.storyDir = t.TempDir()
	proposalRequest(t, cannotCancel, "GET", "/api/cocreate", "")
	mustWriteFile(t, filepath.Join(cannotCancel.storyDir, coCreateSessionPath, "blocked"), "blocked")
	w = doReq(t, cannotCancel, "POST", "/api/cocreate/cancel", `{}`)
	expectInternal(t, w, "Không thể xoá phiên đồng sáng tác")
}

func TestProjectCoCreateProposalMoveCommit(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	var current *workerServer
	var currentHost *liveFakeHost
	tg.g.launch = func(_, dir, _, _ string) (*launchedWorker, error) {
		currentHost = newLiveFakeHost()
		currentHost.coReply = host.CoCreateReply{
			Message: "Đã rõ", Prompt: "## Hướng đi\n- Cánh cửa", Ready: true,
			Proposal: &host.CoCreateProposal{SchemaVersion: 1, Title: "Hướng đi",
				Chapters: []host.CoCreateProposalChapter{{SchemaVersion: 1, Chapter: 1, Title: "Cánh cửa", Summary: "Nhân vật chọn lối đi."}},
				Impacts:  []string{"Lập dàn ý ch.1"}},
		}
		current = newLiveServer(currentHost)
		current.storyDir = dir
		return fakeWorker(make(chan struct{}), &sync.Once{}), nil
	}
	tg.g.transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		r.Header.Set("X-Ainovel-Worker-Token", "secret")
		w := httptest.NewRecorder()
		current.ServeHTTP(w, r)
		return w.Result(), nil
	})
	empty := decodeResp(t, tg.do("GET", "/api/projects/proj/cocreate", admin, ""))
	if empty["schemaVersion"] != float64(1) || empty["proposal"] != nil || empty["committed"] != false || empty["nextChapter"] != nil {
		t.Fatalf("project empty contract: %+v", empty)
	}
	w := tg.do("POST", "/api/projects/proj/cocreate/message", admin, `{"message":"Hướng đi mới"}`)
	if w.Code != 200 {
		t.Fatalf("project message: %d %s", w.Code, w.Body.String())
	}
	id, _ := decodeResp(t, w)["draftId"].(string)
	w = tg.do("POST", "/api/projects/proj/cocreate/commit", admin, fmt.Sprintf(`{"draft":%q}`, id))
	if w.Code != 201 {
		t.Fatalf("project commit: %d %s", w.Code, w.Body.String())
	}
	var committed coCreateProposalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &committed); err != nil {
		t.Fatal(err)
	}
	if committed.SchemaVersion != 1 || !committed.Committed || committed.Proposal == nil || committed.Proposal.Revision != 1 || committed.NextChapter == nil || *committed.NextChapter != 1 {
		t.Fatalf("project commit lost proposal: %s", w.Body.String())
	}
	if callsJoined(currentHost) != "prepare,start" {
		t.Fatalf("project must commit through actual host startup: %s", callsJoined(currentHost))
	}
	restarted := newLiveServer(newLiveFakeHost())
	restarted.storyDir = current.storyDir
	state := proposalRequest(t, restarted, "GET", "/api/cocreate", "")
	if state.Active || !state.Committed || state.Proposal == nil || state.Proposal.Title != "Hướng đi" || len(state.Messages) != 2 {
		t.Fatalf("moving draft retained false active session: %+v", state)
	}
	if restarted.snapshot().WaitingFor == "cocreate" {
		t.Fatal("moved committed proposal must not block story startup")
	}
}
