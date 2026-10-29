package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSettingsModelsWCatalog(t *testing.T) {
	tg := newConfigTestGateway(t)
	tg.globalConfig(t, `{"providers":{"openai":{"models":[{"name":"unknown-w-model","context_window":256000}]}}}`)
	reader := tg.login("reader", "reader-pass")
	w := tg.do("GET", "/api/config/models?provider=openai", reader, "")
	if w.Code != 200 {
		t.Fatalf("catalog read = %d: %s", w.Code, w.Body.String())
	}
	model := decodeResp(t, w)["models"].([]any)[0].(map[string]any)
	if model["contextWindow"] != float64(256000) || model["contextSource"] != "model_config" || model["registryWindow"] != float64(0) {
		t.Fatalf("context metadata = %v", model)
	}
}

func TestSettingsModelsWRangeAndPermission(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	for _, n := range []int{1, 7999, 2000001, -1} {
		w := tg.do("PUT", "/api/config?scope=global", admin, fmt.Sprintf(`{"providers":{"openai":{"models":[{"name":"unknown-w-model","context_window":%d}]}}}`, n))
		if w.Code != 400 {
			t.Errorf("window %d = %d, want 400", n, w.Code)
		}
	}
	writer := tg.login("writer", "writer-pass")
	w := tg.do("PUT", "/api/config?scope=story&project=proj&story=01-story", writer, `{"providers":{"openai":{"models":[{"name":"unknown-w-model","context_window":256000}]}}}`)
	if w.Code != 403 {
		t.Errorf("writer context save = %d, want 403", w.Code)
	}
	for _, n := range []int{0, 8000, 256000, 2000000} {
		w := tg.do("PUT", "/api/config?scope=global", admin, fmt.Sprintf(`{"providers":{"openai":{"models":[{"name":"unknown-w-model","context_window":%d}]}}}`, n))
		if w.Code != 200 {
			t.Errorf("window %d = %d: %s", n, w.Code, w.Body.String())
		}
	}
}

func TestSettingsModelsWReloadsRunningWorkers(t *testing.T) {
	tg := newConfigTestGateway(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/context-reload" || r.Method != http.MethodPost || r.Header.Get("X-Ainovel-Worker-Token") != tg.g.workerToken {
			w.WriteHeader(403)
			return
		}
		calls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()
	worker := &launchedWorker{addr: strings.TrimPrefix(server.URL, "http://")}
	tg.g.workers["proj/01-story"] = &storyWorker{worker: worker, running: true}
	w := tg.do("PUT", "/api/config?scope=global", tg.login("admin", "admin-pass"), `{"providers":{"openai":{"models":[{"name":"unknown-w-model","context_window":256000}]}}}`)
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatalf("saved context must reach live worker: status=%d calls=%d", w.Code, calls.Load())
	}
	if tg.g.workers["proj/01-story"].worker != worker {
		t.Fatal("live engine was restarted")
	}
}
