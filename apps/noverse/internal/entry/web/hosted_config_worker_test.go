package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
)

// Production openWorkerHost performs scoped lookup and config resolution; only
// process transport is replaced here. The scripted model still executes and
// commits a real chapter through the PG-fenced engine store.
func installConfigWorkerLauncher(t *testing.T, f *hostedFixture) {
	t.Helper()
	t.Setenv("STORE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", f.pool.Pool().Config().ConnConfig.ConnString())
	t.Setenv(workerScopeEnv, f.project+"/"+f.story)
	f.g.launchHosted = func(key, project, story string) (*launchedWorker, error) {
		h, cleanup, err := openWorkerHost(bootstrap.Config{}, assets.Bundle{}, version.Info{})
		if err != nil {
			return nil, err
		}
		stop, done := make(chan struct{}), make(chan struct{})
		server := httptest.NewServer(newWorkerHandler(h, f.g.workerToken, stop))
		var once sync.Once
		closeWorker := func() { once.Do(func() { close(stop); server.Close(); h.Close(); cleanup(); close(done) }) }
		reader, writer := io.Pipe()
		go func() { _, _ = io.Copy(io.Discard, reader); closeWorker(); _ = reader.Close() }()
		return &launchedWorker{addr: strings.TrimPrefix(server.URL, "http://"), stdin: writer, done: done, kill: func() error { closeWorker(); return nil }, noModel: !h.ModelReady()}, nil
	}
}

func TestHostedWorkerConfigChainAndReadOnlyRecovery(t *testing.T) {
	f := provisionHostedGateway(t, "worker-config", "story")
	tg := &testGateway{t: t, g: f.g}
	admin := tg.login("hosted-admin", "mat-khau-12345")
	owner := tg.login("hosted-owner", "mat-khau-12345")
	installConfigWorkerLauncher(t, f)
	preseedCutoverStory(t, f.pool, f.project, f.story)
	base := "/api/projects/" + f.project + "/stories/" + f.story
	request := func(method, path, cookie, body string) *httptest.ResponseRecorder {
		return doFrom(t, f.g, method, path, cookie, body, "127.0.0.1:1234", nil)
	}
	mustOK := func(w *httptest.ResponseRecorder) {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	// Opening before configuring credentials creates the same no-model worker as
	// hosted onboarding. It must not remain pinned after a successful config save.
	mustOK(request(http.MethodGet, base+"/snapshot", owner, ""))
	slot := f.g.slot(f.project + "/" + f.story)
	slot.mu.Lock()
	initial := slot.worker
	slot.mu.Unlock()
	if initial == nil || !initial.noModel {
		t.Fatal("expected initial read-only worker")
	}
	model := newCutoverModelServer(t)
	config := map[string]any{"provider": "cutover", "model": "scripted", "advance_mode": "review",
		"providers": map[string]any{"cutover": map[string]any{"type": "openai", "base_url": model.URL, "api_key": "test-only"}}}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	mustOK(request(http.MethodPut, "/api/config?scope=global", admin, string(data)))
	select {
	case <-initial.done:
	default:
		t.Fatal("read-only worker still alive after provider configuration")
	}
	mustOK(request(http.MethodPut, "/api/config?scope=project&project="+f.project, owner, `{"style":"xianxia"}`))
	mustOK(request(http.MethodPut, "/api/config?scope=story&project="+f.project+"&story="+f.story, owner, `{"style":"nu-tan-bao-thu"}`))
	snapshot := request(http.MethodGet, base+"/snapshot", owner, "")
	mustOK(snapshot)
	var snap struct {
		Style string `json:"style"`
	}
	if err := json.Unmarshal(snapshot.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Style != "nu-tan-bao-thu" {
		t.Fatalf("story style did not reach engine: %s", snapshot.Body.String())
	}
	slot.mu.Lock()
	ready := slot.worker != nil && !slot.worker.noModel
	slot.mu.Unlock()
	if !ready {
		t.Fatal("configured provider did not replace read-only worker")
	}
	mustOK(request(http.MethodPost, base+"/next", owner, `{}`))
	finished := waitCutoverChapter(t, f.g, owner, base)
	if finished.CompletedCount != 1 {
		t.Fatalf("scripted engine did not commit chapter: %+v", finished)
	}
}
