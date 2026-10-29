//go:build !integration

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// TestPGAudioScriptPutIsCAS proves the ETag is a write contract, not only a
// handler precheck: the version token the caller read is carried into the
// Postgres write transaction. A second save with the same token conflicts even
// though it was read while the row was still current, and a malformed token
// can never match.
func TestPGAudioScriptPutIsCAS(t *testing.T) {
	ctx := context.Background()
	pool, nar, scope := provisionWebStory(t, "cas-proj", "cas-story")
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		"Trời đã tối. Nàng nhìn ra cửa sổ.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil || !ref.ID.Valid {
		t.Fatalf("chapter ref: %v %v", ref.ID.Valid, err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	script := audio.Script{Chapter: 1, Volume: 1, Status: audio.StatusReview,
		SourceHash: audio.SourceHash("Trời đã tối. Nàng nhìn ra cửa sổ."),
		Segments: []audio.Segment{{ID: "s0001", Speaker: audio.NarratorID,
			Text: "Trời đã tối. Nàng nhìn ra cửa sổ."}}}
	payload, _ := json.Marshal(script)
	if _, err := audioRepo.SaveScript(ctx, scope, postgres.Script{
		ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256: script.SourceHash, Status: audio.StatusReview, Payload: payload}); err != nil {
		t.Fatalf("save script: %v", err)
	}
	if _, err := audioRepo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{
		SegmentID: "s0001", Ordinal: 0, Speaker: audio.NarratorID,
		Text: script.Segments[0].Text, Payload: []byte(`{}`)}); err != nil {
		t.Fatalf("save segment: %v", err)
	}

	tg := newTestGateway(t)
	st := library.Story{Project: "cas-proj", Story: "cas-story", ID: "cas-proj/cas-story"}
	tg.g.SetFactStore(func(_ library.Story, _ *webauth.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, "cas-proj", "cas-story")
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: postgres.NewJobs(pool), Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})
	admin := webauth.User{Username: "admin", Role: webauth.RoleAdmin}

	get := httptest.NewRequest(http.MethodGet, "http://x/audio/volumes/1/scripts/1", nil)
	getW := httptest.NewRecorder()
	if !tg.g.serveStoryAudio(getW, get, admin, st, "/audio/volumes/1/scripts/1") {
		t.Fatal("GET script not handled")
	}
	if getW.Code != http.StatusOK {
		t.Fatalf("GET script = %d %s", getW.Code, getW.Body.String())
	}
	var current struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &current); err != nil || current.Version == "" {
		t.Fatalf("GET script version = %q (%v)", current.Version, err)
	}

	svc, err := tg.g.pgAudioService(st)
	if err != nil {
		t.Fatalf("audio service: %v", err)
	}
	edit := audio.ScriptEdit{Segments: []audio.Segment{{
		ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối.",
	}}}
	if _, err := svc.SaveScript(ctx, 1, 1, current.Version, edit); err != nil {
		t.Fatalf("first versioned save: %v", err)
	}
	if _, err := svc.SaveScript(ctx, 1, 1, current.Version, edit); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale save = %v, want ErrStorageConflict", err)
	}
	if _, err := svc.SaveScript(ctx, 1, 1, "not-a-token", edit); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("malformed token = %v, want ErrStorageConflict", err)
	}

	// The handler still answers the established 409 envelope when the
	// precheck sees a stale token.
	body := []byte(`{"segments":[{"id":"s0001","speaker":"","text":"Trời đã tối."}]}`)
	put := httptest.NewRequest(http.MethodPut, "http://x/audio/volumes/1/scripts/1", bytesReader(body))
	put.Header.Set("Content-Type", "application/json")
	put.Header.Set("If-Match", current.Version)
	putW := httptest.NewRecorder()
	if !tg.g.serveStoryAudio(putW, put, admin, st, "/audio/volumes/1/scripts/1") {
		t.Fatal("PUT script not handled")
	}
	if putW.Code != http.StatusConflict {
		t.Fatalf("stale PUT = %d %s, want 409", putW.Code, putW.Body.String())
	}
	var conflict struct {
		Conflict bool        `json:"conflict"`
		Current  interface{} `json:"current"`
	}
	if err := json.Unmarshal(putW.Body.Bytes(), &conflict); err != nil {
		t.Fatalf("conflict body: %v (%s)", err, putW.Body.String())
	}
	if !conflict.Conflict || conflict.Current == nil {
		t.Fatalf("conflict envelope = %+v", conflict)
	}
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
