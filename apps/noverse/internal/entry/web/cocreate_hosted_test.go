package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/datnt1998/verse/core/llmconfig"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/identity"
)

const coCreateScriptReply = `<reply>Đã hiểu hướng đi.</reply><draft>Truyện trinh thám về nữ giám định pháp y.</draft><ready>true</ready><suggestions>Thêm bối cảnh</suggestions>`

func installFileCoCreateLauncher(t *testing.T, g *Gateway, modelURL string) {
	t.Helper()
	g.launch = func(_, dir, _, _ string) (*launchedWorker, error) {
		cfg := bootstrap.Config{Provider: "cutover", ModelName: "scripted", Style: "default", OutputDir: filepath.Join(dir, "output", "novel"), AdvanceMode: "review", Providers: map[string]llmconfig.ProviderConfig{"cutover": {Type: "openai", BaseURL: modelURL, APIKey: "test-only"}}}
		bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(cfg.OutputDir))
		if bundle.Err != nil {
			return nil, bundle.Err
		}
		h, err := host.New(cfg, bundle)
		if err != nil {
			return nil, err
		}
		stop := make(chan struct{})
		done := make(chan struct{})
		handler := newWorkerServer(h, g.workerToken, stop, newBroadcaster())
		handler.storyDir = dir
		go handler.pump()
		server := httptest.NewServer(handler)
		var once sync.Once
		closeWorker := func() { once.Do(func() { close(stop); server.Close(); h.Close(); close(done) }) }
		return &launchedWorker{addr: strings.TrimPrefix(server.URL, "http://"), stdin: closerFunc(func() error { closeWorker(); return nil }), done: done, kill: func() error { closeWorker(); return nil }}, nil
	}
}

func TestHostedCoCreateHTTP(t *testing.T) {
	for _, backend := range []string{"file", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			work := t.TempDir()
			old, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Chdir(work); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chdir(old); err != nil {
					t.Error(err)
				}
			})
			var g *Gateway
			var pool *postgres.Pool
			var project string
			var owner, outsider, viewer identity.User
			if backend == "postgres" {
				f := provisionHostedGateway(t, "cocreate-project", "existing")
				g, pool, project, owner, outsider, viewer = f.g, f.pool, f.project, f.owner, f.outsider, f.viewer
				g.Root = t.TempDir()
				if err = os.Chmod(g.Root, 0555); err != nil {
					t.Fatal(err)
				}
			} else {
				f := newConfigTestGateway(t)
				g, project = f.g, "proj"
				owner = identity.User{Username: "admin", Role: identity.RoleAdmin}
				outsider = identity.User{Username: "outsider", Role: identity.RoleMember}
				viewer = identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{project: identity.ProjectView}}
				t.Cleanup(g.Close)
			}
			model := newCutoverModelServer(t, coCreateScriptReply)
			if pool != nil {
				installCutoverLauncher(t, g, pool, model.URL)
			} else {
				installFileCoCreateLauncher(t, g, model.URL)
			}
			base := "/api/projects/" + project + "/cocreate"
			request := func(user identity.User, method, path, body string) *httptest.ResponseRecorder {
				return callProjects(g, user, method, path, body)
			}
			create := func() string {
				w := request(owner, http.MethodPost, base+"/message", `{"message":"Một truyện trinh thám"}`)
				if w.Code != http.StatusOK {
					t.Fatalf("create draft: %d %s", w.Code, w.Body.String())
				}
				var response struct {
					DraftID string `json:"draftId"`
					Draft   string `json:"draft"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.DraftID == "" || response.Draft == "" {
					t.Fatalf("scripted engine did not produce draft: %s", w.Body.String())
				}
				return response.DraftID
			}
			id := create()
			// A worker restart must restore the conversation from backend facts.
			g.dropSlot(draftKey(project, id))
			if pool != nil {
				// Exercise the production env scope/config/source/lease startup,
				// not just the in-process launcher used for scripted interaction.
				t.Setenv("STORE_DRIVER", "postgres")
				t.Setenv("DATABASE_URL", pool.Pool().Config().ConnConfig.ConnString())
				t.Setenv(workerScopeEnv, project+"/draft/"+id)
				h, closeStore, err := openWorkerHost(bootstrap.Config{}, assets.Bundle{}, version.Info{})
				if err != nil {
					t.Fatalf("open draft worker scope: %v", err)
				}
				if _, _, _, err := openWorkerStorage(context.Background()); !errors.Is(err, postgres.ErrLeaseLost) {
					t.Fatalf("duplicate env-scoped draft worker: %v", err)
				}
				h.Close()
				closeStore()
			}
			w := request(owner, http.MethodGet, base+"?draft="+id, "")
			if w.Code != 200 || !strings.Contains(w.Body.String(), "Đã hiểu hướng đi.") {
				t.Fatalf("restored conversation: %d %s", w.Code, w.Body.String())
			}
			for _, action := range []struct{ method, path, body string }{
				{"GET", base + "?draft=" + id, ""}, {"POST", base + "/message", fmt.Sprintf(`{"draft":%q,"message":"tiếp"}`, id)},
				{"POST", base + "/commit", fmt.Sprintf(`{"draft":%q}`, id)}, {"POST", base + "/cancel", fmt.Sprintf(`{"draft":%q}`, id)},
			} {
				w = request(outsider, action.method, action.path, action.body)
				if w.Code != 404 {
					t.Fatalf("outsider %s: %d %s", action.path, w.Code, w.Body.String())
				}
			}
			if w = request(viewer, "POST", base+"/message", `{"message":"x"}`); w.Code != 403 {
				t.Fatalf("viewer write: %d %s", w.Code, w.Body.String())
			}
			if pool != nil {
				repo := postgres.NewLibraryRepository(pool)
				stories, err := repo.Stories(context.Background(), actorOf(owner), project)
				if err != nil || len(stories) != 1 {
					t.Fatalf("draft counted: %+v %v", stories, err)
				}
				if _, ok := repo.ResolveStory(context.Background(), actorOf(owner), project, "draft-"+id); ok {
					t.Fatal("draft has a public story route")
				}
			}
			w = request(owner, "POST", base+"/commit", fmt.Sprintf(`{"draft":%q,"name":"Truyện mới"}`, id))
			if w.Code != 201 || strings.Contains(w.Body.String(), `"warning"`) {
				t.Fatalf("commit: %d %s", w.Code, w.Body.String())
			}
			var committed struct {
				Story struct {
					ID string `json:"id"`
				} `json:"story"`
			}
			if err = json.Unmarshal(w.Body.Bytes(), &committed); err != nil {
				t.Fatal(err)
			}
			if committed.Story.ID != project+"/02-truyen-moi" {
				t.Fatalf("story number: %s", w.Body.String())
			}
			w = request(owner, "GET", base+"?draft="+id, "")
			if w.Code != 404 {
				t.Fatalf("committed draft still reachable: %d %s", w.Code, w.Body.String())
			}
			// Different drafts committed simultaneously must receive distinct numbers.
			ids := []string{create(), create()}
			results := make(chan *httptest.ResponseRecorder, 2)
			var wg sync.WaitGroup
			for _, draft := range ids {
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					results <- request(owner, "POST", base+"/commit", fmt.Sprintf(`{"draft":%q}`, id))
				}(draft)
			}
			wg.Wait()
			close(results)
			seen := map[string]bool{}
			for w := range results {
				if w.Code != 201 || strings.Contains(w.Body.String(), `"warning"`) {
					t.Fatalf("concurrent commit: %d %s", w.Code, w.Body.String())
				}
				var result struct {
					Story struct {
						ID string `json:"id"`
					} `json:"story"`
				}
				if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if seen[result.Story.ID] {
					t.Fatalf("duplicate story number: %s", result.Story.ID)
				}
				seen[result.Story.ID] = true
			}
			if !seen[project+"/03-truyen-moi"] || !seen[project+"/04-truyen-moi"] {
				t.Fatalf("concurrent story numbers: %v", seen)
			}
			id = create()
			w = request(owner, "POST", base+"/cancel", fmt.Sprintf(`{"draft":%q}`, id))
			if w.Code != 200 {
				t.Fatalf("remove draft: %d %s", w.Code, w.Body.String())
			}
			w = request(owner, "GET", base+"?draft="+id, "")
			if w.Code != 404 {
				t.Fatalf("removed draft: %d %s", w.Code, w.Body.String())
			}
			if pool != nil {
				repo := postgres.NewLibraryRepository(pool)
				stories, err := repo.Stories(context.Background(), actorOf(owner), project)
				if err != nil || len(stories) != 4 {
					t.Fatalf("abandoned draft counted: %+v %v", stories, err)
				}
				assertNoBusinessFiles(t, g.Root, "hosted library")
			}
			assertNoBusinessFiles(t, work, "process cwd")
		})
	}
}
