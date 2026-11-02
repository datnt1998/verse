//go:build !integration

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

const pgComposeChapterText = "Trời đã tối. Nàng nhìn ra cửa sổ."

// TestPGHostedComposeMakesGenerationReachable is the regression for the
// hosted "Tạo giọng" flow: POST /audio/volumes/1/scripts used to answer 501
// (ErrStorageUnsupported) because the gateway only built a composer when the
// story had a directory, so a hosted story could never obtain a bản đọc and
// therefore never enqueue TTS. The test runs the real ChatComposer against a
// local OpenAI-compatible endpoint (no commercial provider call), then locks
// the composed script and proves the normal render endpoint creates a durable
// TTS job. Only narrative/progress rows are seeded; no audio script/segment.
func TestPGHostedComposeMakesGenerationReachable(t *testing.T) {
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "story-proj", "novel-b")

	jobs := postgres.NewJobs(pool)
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		pgComposeChapterText, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	tg := newTestGateway(t)
	tg.g.SetLibraryRepository(postgres.NewLibraryRepository(pool))
	st := library.Story{Project: "story-proj", Story: "novel-b", ID: "story-proj/novel-b"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}

	// A local OpenAI-compatible endpoint: the real provider client, prompt and
	// response parser run; only the network peer is a test server.
	composeModel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"{\"segments\":[{\"speaker\":\"dan\",\"text\":\"Trời đã tối.\",\"tone\":\"\",\"pauseMs\":0}]}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":10,"total_tokens":20}}`)
	}))
	defer composeModel.Close()
	home := os.Getenv("HOME")
	if err := os.MkdirAll(filepath.Join(home, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","api_key":"sk-local-test","base_url":%q}}}`, composeModel.URL)
	if err := os.WriteFile(filepath.Join(home, ".ainovel", "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	svc, err := tg.g.pgAudioService(st)
	if err != nil {
		t.Fatalf("pg audio service: %v", err)
	}
	eventsCtx, stopEvents := context.WithTimeout(ctx, 10*time.Second)
	defer stopEvents()
	events := svc.Hub().Subscribe(eventsCtx)

	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var r *http.Request
		if body != nil {
			buf, _ := json.Marshal(body)
			r = httptest.NewRequest(method, "http://x"+path, bytes.NewReader(buf))
		} else {
			r = httptest.NewRequest(method, "http://x"+path, nil)
		}
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, r, admin, st, path) {
			t.Fatalf("%s %s not handled", method, path)
		}
		return w
	}

	res := call("POST", "/audio/volumes/1/scripts", map[string]any{"chapters": []int{1}})
	if res.Code != http.StatusOK {
		t.Fatalf("compose: %d %s", res.Code, res.Body.String())
	}

	// The model call is asynchronous: wait for the terminal audio.compose frame.
composeWait:
	for {
		select {
		case <-eventsCtx.Done():
			t.Fatal("compose did not finish")
		case ev := <-events:
			if ev.Type != audio.EventCompose {
				continue
			}
			if ev.Data["chapter"] != 1 {
				continue
			}
			if ev.Data["state"] == "error" {
				t.Fatalf("compose error: %v", ev.Data["error"])
			}
			if ev.Data["state"] == "done" {
				break composeWait
			}
		}
	}

	script, err := svc.Script(ctx, 1, 1)
	if err != nil {
		t.Fatalf("composed script: %v", err)
	}
	if script == nil || len(script.Segments) != 1 {
		t.Fatalf("script = %+v, want one composed segment", script)
	}
	if script.Status != audio.StatusReview {
		t.Fatalf("status = %q, want review", script.Status)
	}
	if script.SourceHash != audio.SourceHash(pgComposeChapterText) {
		t.Fatalf("source hash = %q", script.SourceHash)
	}
	if script.Model != "openai/gpt-4o" {
		t.Fatalf("model = %q", script.Model)
	}

	// The normal review→lock→generate path must now work: lock the composed
	// script and render the chapter.
	res = call("POST", "/audio/volumes/1/scripts/1/lock", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("lock: %d %s", res.Code, res.Body.String())
	}
	res = call("POST", "/audio/volumes/1/chapters/1/render", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("render: %d %s", res.Code, res.Body.String())
	}
	rows, err := jobs.List(ctx, scope)
	if err != nil {
		t.Fatalf("jobs: %v", err)
	}
	if len(rows) != 1 || rows[0].Kind != postgres.JobTTS {
		t.Fatalf("jobs = %+v, want one tts job", rows)
	}
}
