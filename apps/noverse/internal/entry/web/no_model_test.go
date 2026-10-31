package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/llmconfig"
)

type noModelInput func()

func (closeInput noModelInput) Close() error { closeInput(); return nil }

func installRealWorkerLauncher(t *testing.T, g *Gateway) *int {
	t.Helper()
	launches := 0
	g.launch = func(id, dir, root, token string) (*launchedWorker, error) {
		cfg, err := bootstrap.LoadConfigAt(dir)
		if err != nil {
			return nil, err
		}
		cfg.FillDefaults()
		if !filepath.IsAbs(cfg.OutputDir) {
			cfg.OutputDir = filepath.Join(dir, cfg.OutputDir)
		}
		h, err := host.New(cfg, assets.Load(cfg.Style, assets.DefaultLoadOptions(cfg.OutputDir)), host.WithUnconfiguredReads())
		if err != nil {
			return nil, err
		}
		stop := make(chan struct{})
		done := make(chan struct{})
		server := httptest.NewServer(newWorkerHandler(h, token, stop))
		var once sync.Once
		closeWorker := func() { once.Do(func() { close(stop); server.Close(); h.Close(); close(done) }) }
		launches++
		return &launchedWorker{addr: strings.TrimPrefix(server.URL, "http://"), noModel: !h.ModelReady(), done: done, stdin: noModelInput(closeWorker), kill: func() error { closeWorker(); return nil }}, nil
	}
	return &launches
}

func TestNoModelSnapshotReadsForEveryRoleAndConfigRestartsWorker(t *testing.T) {
	tg := newTestGateway(t)
	defer tg.g.Close()
	st := store.NewStore(filepath.Join(tg.root, "oldbook", "output", "novel"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Book.Save(domain.BookMetadata{Title: "Truyện chưa thiết lập model", Synopsis: "Truyện đã có dữ liệu nhưng chưa cấu hình nhà cung cấp."}); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	launches := installRealWorkerLauncher(t, tg.g)
	sessions := map[string]string{}
	for _, role := range []string{"admin", "writer", "reader"} {
		sessions[role] = tg.login(role, role+"-pass")
	}
	for _, role := range []string{"admin", "writer", "reader"} {
		response := tg.do(http.MethodGet, "/api/projects/_/stories/oldbook/snapshot", sessions[role], "")
		if response.Code != 200 || !strings.Contains(response.Body.String(), "Truyện chưa thiết lập model") {
			t.Fatalf("%s snapshot must not need a model: %d %s", role, response.Code, response.Body.String())
		}
	}
	response := tg.do(http.MethodPost, "/api/projects/_/stories/oldbook/resume", sessions["writer"], `{}`)
	if response.Code != 409 || decodeLiveBody(t, response)["needsConfig"] != true {
		t.Fatalf("missing actionable configuration guard: %d %s", response.Code, response.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/projects/_/stories/oldbook/events/live", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: fileSessionCookie, Value: sessions["reader"]})
	writer := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	done := make(chan struct{})
	go func() { tg.g.ServeHTTP(writer, request); close(done) }()
	writer.until(t, "Truyện chưa thiết lập model")
	cancel()
	<-done
	response = tg.do(http.MethodPut, "/api/config?scope=global", sessions["admin"], `{"provider":"local","model":"test","providers":{"local":{"type":"openai","base_url":"http://127.0.0.1:1/v1","api_key":"test-only","models":[{"name":"test"}]}}}`)
	if response.Code != 200 {
		t.Fatalf("configure provider: %d %s", response.Code, response.Body.String())
	}
	response = tg.do(http.MethodGet, "/api/projects/_/stories/oldbook/snapshot", sessions["reader"], "")
	if response.Code != 200 || decodeLiveBody(t, response)["provider"] != "local" || *launches != 2 {
		t.Fatalf("config.changed must retire no-model worker: launches=%d %d %s", *launches, response.Code, response.Body.String())
	}
	response = tg.do(http.MethodPost, "/api/projects/_/stories/oldbook/models", sessions["writer"], `{"role":"default","provider":"local","model":"test"}`)
	if response.Code != 200 {
		t.Fatalf("next action must use configured host: %d %s", response.Code, response.Body.String())
	}
}

func TestNoModelActionsAreGuardedButLocalReadsWork(t *testing.T) {
	st, s := storyFixture(t, []int{1})
	if _, err := s.ChapterRecords.Accept(1, domain.ChapterOriginUser, "nội dung chương", domain.ChapterFacts{}, domain.StyleDelta{}); err != nil {
		t.Fatal(err)
	}
	cfg := bootstrap.Config{OutputDir: s.Dir()}
	h, err := host.New(cfg, assets.Bundle{}, host.WithUnconfiguredReads())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	server := newWorkerServer(h, "secret", nil, newBroadcaster())
	server.storyDir = st.Dir
	server.root = st.Root
	for _, path := range []string{"snapshot", "chapters", "models", "import", "cocreate", "diag"} {
		response := doReq(t, server, "GET", "/api/"+path, "")
		if response.Code != 200 {
			t.Fatalf("read %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	actions := []struct{ path, body string }{{"input", `{"text":"Tiếp tục"}`}, {"resume", `{}`}, {"next", `{}`}, {"start-file", `{"path":"input.md"}`}, {"cocreate/message", `{"text":"Chào"}`}, {"cocreate/commit", `{}`}, {"import", `{}`}, {"import/preview", `{"path":"input.md"}`}, {"import/confirm", `{"accept":true}`}, {"simulate", `{}`}, {"importsim", `{}`}, {"sync", `{"check":false}`}, {"diag/actions/example", `{}`}, {"command", `{"line":"/next"}`}}
	for _, action := range actions {
		response := doReq(t, server, "POST", "/api/"+action.path, action.body)
		if response.Code != 409 {
			t.Fatalf("%s must return 409: %d %s", action.path, response.Code, response.Body.String())
		}
		body := decodeLiveBody(t, response)
		if body["needsConfig"] != true || body["error"] != needsModelMessage {
			t.Fatalf("%s returned wrong configuration error: %+v", action.path, body)
		}
	}
	response := doReq(t, server, "POST", "/api/sync", `{"check":true}`)
	if response.Code != 200 {
		t.Fatalf("local sync scan must work without model: %d %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(st.Dir, ".ainovel", "config.json")); !os.IsNotExist(err) {
		t.Fatal("test accidentally configured the story")
	}
}

func TestNoModelRejectedActionAppearsInActivity(t *testing.T) {
	_, st := storyFixture(t, []int{1})
	h, err := host.New(bootstrap.Config{OutputDir: st.Dir()}, assets.Bundle{}, host.WithUnconfiguredReads())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	server := newWorkerServer(h, "secret", nil, newBroadcaster())
	response := doReq(t, server, "POST", "/api/input", `{"text":"/cocreate"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected needs-config rejection: %d", response.Code)
	}
	select {
	case event := <-h.Events():
		if event.Category != "ERROR" || !event.Failed || event.Summary != needsModelMessage || event.FinishedAt.IsZero() {
			t.Fatalf("rejected action missing completed error event: %+v", event)
		}
	default:
		t.Fatal("needs-config rejection never reached Activity")
	}
}

func TestNoModelInvalidProviderKeepsSnapshotReadable(t *testing.T) {
	_, s := storyFixture(t, []int{1})
	cfg := bootstrap.Config{OutputDir: s.Dir(), Provider: "broken", ModelName: "test", Providers: map[string]llmconfig.ProviderConfig{"broken": {Type: "not-a-provider", APIKey: "test-only"}}}
	h, err := host.New(cfg, assets.Bundle{}, host.WithUnconfiguredReads())
	if err != nil {
		t.Fatalf("invalid provider must not block reading: %v", err)
	}
	defer h.Close()
	if h.ModelReady() {
		t.Fatal("invalid provider unexpectedly constructed a model")
	}
	server := newWorkerServer(h, "secret", nil, newBroadcaster())
	response := doReq(t, server, "GET", "/api/snapshot", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Truyện Kiểm") {
		t.Fatalf("invalid provider snapshot: %d %s", response.Code, response.Body.String())
	}
}
