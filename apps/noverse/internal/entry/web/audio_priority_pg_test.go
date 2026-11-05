//go:build !integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// POST /audio/volumes/:v/chapters/:ch/prioritize is a durable queue operation:
// 200 for a queued chapter, 409 for a running one, 404 for a missing one.
func TestPGPriorityHTTPContract(t *testing.T) {
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "prio-proj", "prio-story")
	jobs := postgres.NewJobs(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	for _, chapter := range []int{1, 2, 3} {
		if _, err := nar.AcceptRecord(ctx, scope, chapter, domain.ChapterOriginUser,
			fmt.Sprintf("Nội dung chương %d để kiểm tra hàng đợi.", chapter),
			domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "import"}); err != nil {
			t.Fatalf("accept chapter %d: %v", chapter, err)
		}
		if _, err := jobs.Enqueue(ctx, scope, postgres.JobTTS,
			fmt.Sprintf("prio-http-%d", chapter),
			map[string]any{"volume": 1, "chapter": chapter, "kind": "create"},
			nil, pgtype.UUID{}); err != nil {
			t.Fatalf("enqueue chapter %d: %v", chapter, err)
		}
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{CurrentChapter: 4, CompletedChapters: []int{1, 2, 3}}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	tg := newTestGateway(t)
	st := library.Story{Project: "prio-proj", Story: "prio-story", ID: "prio-proj/prio-story"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{
		Jobs: jobs, Audio: postgres.NewAudio(pool), Cache: postgres.NewExportCache(pool),
		Nar: nar, Blobs: blobs,
	})
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	call := func(chapter int) *httptest.ResponseRecorder {
		t.Helper()
		path := fmt.Sprintf("/audio/volumes/1/chapters/%d/prioritize", chapter)
		r := httptest.NewRequest(http.MethodPost, "http://x"+path, bytes.NewReader(nil))
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, r, admin, st, path) {
			t.Fatalf("%s not handled", path)
		}
		return w
	}

	res := call(2)
	if res.Code != http.StatusOK {
		t.Fatalf("prioritize queued: %d %s", res.Code, res.Body.String())
	}
	var ok map[string]any
	if err := json.NewDecoder(res.Body).Decode(&ok); err != nil || ok["ok"] != true {
		t.Fatalf("prioritize body: %v %v", ok, err)
	}
	list, err := jobs.List(ctx, scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 || chapterNumberOf(list[0]) != 2 {
		t.Fatalf("durable order = %+v, want chapter 2 first", list)
	}

	res = call(9)
	if res.Code != http.StatusNotFound {
		t.Fatalf("prioritize missing: %d %s", res.Code, res.Body.String())
	}

	token := sha256.Sum256([]byte("priority-worker-token"))
	worker, err := jobs.RegisterWorker(ctx, "priority-worker", token[:], []string{"tts"})
	if err != nil {
		t.Fatalf("register worker: %v", err)
	}
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if chapterNumberOf(claimed) != 2 {
		t.Fatalf("claimed chapter %d, want 2", chapterNumberOf(claimed))
	}
	res = call(2)
	if res.Code != http.StatusConflict {
		t.Fatalf("prioritize running: %d %s", res.Code, res.Body.String())
	}
}

// chapterNumberOf keeps the test readable when asserting the frozen input order.
func chapterNumberOf(j postgres.Job) int {
	var in struct {
		Chapter int `json:"chapter"`
	}
	_ = json.Unmarshal(j.Input, &in)
	return in.Chapter
}
