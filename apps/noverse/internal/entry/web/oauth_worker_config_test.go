package web

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/auth"
)

type oauthWorkerTransport func(*http.Request) (*http.Response, error)

func (f oauthWorkerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// This reproduces the reported file-mode order, not just a pre-authenticated
// startup: open → save OAuth config without a token → reopen → OAuth success →
// continue. All three subscription adapters must read the newly stored token;
// no real provider or credential is contacted by this test.
func TestFileOAuthLoginAfterUnconfiguredWorker(t *testing.T) {
	for _, provider := range []struct{ id, model string }{
		{"xai-oauth", "grok-4.7"}, {"openai-codex", "gpt-5-codex"}, {"anthropic-oauth", "claude-sonnet-4-5"},
	} {
		// DefaultManager is process-scoped and deliberately pins its Store
		// HOME. A fresh process per provider avoids sharing another fixture's
		// credentials and also matches the real worker process boundary.
		selected := os.Getenv("AINOVEL_TEST_OAUTH_WORKER")
		if selected == "" {
			t.Run(provider.id, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestFileOAuthLoginAfterUnconfiguredWorker$", "-test.count=1", "-test.timeout=30s")
				cmd.Env = append(os.Environ(), "AINOVEL_TEST_OAUTH_WORKER="+provider.id)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("OAuth worker subprocess: %v\n%s", err, output)
				}
			})
			continue
		}
		if selected != provider.id {
			continue
		}
		t.Run(provider.id, func(t *testing.T) {
			tg := newConfigTestGateway(t)
			defer tg.g.Close()
			oldProvider, ok := auth.Lookup(provider.id)
			if !ok {
				t.Fatalf("OAuth provider %s is not registered", provider.id)
			}
			auth.Register(&fakeOAuthProvider{id: provider.id})
			t.Cleanup(func() { auth.Register(oldProvider) })
			oldTransport := http.DefaultTransport
			authenticated := make(chan string, 8)
			http.DefaultTransport = oauthWorkerTransport(func(r *http.Request) (*http.Response, error) {
				host := r.URL.Hostname()
				if ip := net.ParseIP(host); (ip != nil && ip.IsLoopback()) || host == "localhost" {
					return oldTransport.RoundTrip(r)
				}
				status, body := http.StatusOK, `{"data":[],"models":[]}`
				if r.Method == http.MethodPost {
					if r.Header.Get("Authorization") == "Bearer fake-at" || r.Header.Get("x-api-key") == "fake-at" {
						select {
						case authenticated <- host:
						default:
						}
					}
					status, body = http.StatusBadRequest, `{"error":{"message":"scripted provider boundary reached","type":"invalid_request_error"}}`
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = oldTransport })
			st := store.NewStore(filepath.Join(tg.story, "output", "novel"))
			if err := st.Init(); err != nil {
				t.Fatal(err)
			}
			if err := st.Progress.Init(2); err != nil {
				t.Fatal(err)
			}
			if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
				t.Fatal(err)
			}
			if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Chương 1", CoreEvent: "Tiếp tục truyện"}, {Chapter: 2, Title: "Chương 2", CoreEvent: "Kết thúc"}}); err != nil {
				t.Fatal(err)
			}
			launches := installRealWorkerLauncher(t, tg.g)
			admin, owner := tg.login("admin", "admin-pass"), tg.login("writer", "writer-pass")
			base := "/api/projects/proj/stories/01-story"
			initial := tg.do(http.MethodGet, base+"/snapshot", owner, "")
			if initial.Code != 200 {
				t.Fatalf("open: %d %s", initial.Code, initial.Body.String())
			}
			slot := tg.g.slot("proj/01-story")
			slot.mu.Lock()
			first := slot.worker
			slot.mu.Unlock()
			if first == nil || !first.noModel {
				t.Fatal("expected unconfigured reading host")
			}
			config := fmt.Sprintf(`{"provider":%q,"model":%q,"providers":{%q:{}}}`, provider.id, provider.model, provider.id)
			saved := tg.do(http.MethodPut, "/api/config?scope=global", admin, config)
			if saved.Code != 200 {
				t.Fatalf("save OAuth model: %d %s", saved.Code, saved.Body.String())
			}
			reopened := tg.do(http.MethodGet, base+"/snapshot", owner, "")
			if reopened.Code != 200 {
				t.Fatalf("reopen before token: %d %s", reopened.Code, reopened.Body.String())
			}
			if *launches != 2 {
				t.Fatalf("config save did not retire reading host: launches=%d", *launches)
			}
			started := tg.do(http.MethodPost, "/api/config/oauth/start", admin, fmt.Sprintf(`{"provider":%q,"device":true}`, provider.id))
			if started.Code != 200 {
				t.Fatalf("OAuth start: %d %s", started.Code, started.Body.String())
			}
			id := decodeResp(t, started)["id"].(string)
			deadline := time.Now().Add(5 * time.Second)
			for {
				status := decodeResp(t, tg.do(http.MethodGet, "/api/config/oauth/status?id="+id, admin, ""))
				if status["state"] == "done" {
					break
				}
				if status["state"] != "pending" || time.Now().After(deadline) {
					t.Fatalf("OAuth completion: %v", status)
				}
				time.Sleep(10 * time.Millisecond)
			}
			resumed := tg.do(http.MethodPost, base+"/resume", owner, `{}`)
			if resumed.Code != 200 {
				t.Fatalf("continue after OAuth: %d %s", resumed.Code, resumed.Body.String())
			}
			select {
			case <-authenticated:
			case <-time.After(5 * time.Second):
				t.Fatal("continued engine did not send newly saved OAuth token to provider transport")
			}
			slot.mu.Lock()
			ready := slot.worker != nil && !slot.worker.noModel
			slot.mu.Unlock()
			if !ready {
				t.Fatal("OAuth continuation still has an unconfigured worker")
			}
		})
	}
}
