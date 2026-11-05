//go:build !integration

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// The mobile "Một giọng kể" sheet saves the chosen preset through
// PUT /audio/cast/single. "single" is the narration mode, not a cast role:
// the request must persist Cast.Single.VoiceID, keep every per-role choice,
// survive a reload and still honour the 409 contracts (unknown role, deleted
// voice, stale version). Before the fix this returned "Không có vai single
// trong phân vai" and no voice was saved.
func TestPGSingleVoiceHTTPContract(t *testing.T) {
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "single-proj", "single-story")
	repo := postgres.NewAudio(pool)
	for _, voice := range []struct{ id, name string }{{"giong-dan", "Giọng dẫn"}, {"giong-nv", "Giọng nhân vật"}, {"giong-ke", "Giọng kể"}} {
		if _, err := repo.UpsertVoice(ctx, scope.ProjectID, voice.id, map[string]any{"name": voice.name}, pgtype.UUID{}); err != nil {
			t.Fatalf("seed voice %s: %v", voice.id, err)
		}
	}
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tg := newTestGateway(t)
	st := library.Story{Project: "single-proj", Story: "single-story", ID: "single-proj/single-story"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{
		Jobs: postgres.NewJobs(pool), Audio: repo, Cache: postgres.NewExportCache(pool),
		Nar: nar, Blobs: blobs,
	})
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}

	call := func(method, path, ifMatch string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var r *http.Request
		if body != nil {
			buf, _ := json.Marshal(body)
			r = httptest.NewRequest(method, "http://x"+path, bytes.NewReader(buf))
		} else {
			r = httptest.NewRequest(method, "http://x"+path, nil)
		}
		if ifMatch != "" {
			r.Header.Set("If-Match", `"`+ifMatch+`"`)
		}
		w := httptest.NewRecorder()
		if !tg.g.serveStoryAudio(w, r, admin, st, path) {
			t.Fatalf("%s %s not handled", method, path)
		}
		return w
	}
	decodeCast := func(w *httptest.ResponseRecorder) pgCastResponse {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("cast: %d %s", w.Code, w.Body.String())
		}
		var got pgCastResponse
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("cast decode: %v (%s)", err, w.Body.String())
		}
		return got
	}

	first := decodeCast(call("GET", "/audio/cast", "", nil))
	if first.Version == "" {
		t.Fatal("cast GET has no version")
	}
	// Seed real per-role choices before the user switches to single mode.
	danVoice, nvVoice := "giong-dan", "giong-nv"
	seeded := decodeCast(call("PUT", "/audio/cast", first.Version, audio.Cast{
		Mode: audio.ModeCast, VoiceMode: audio.ModeCast,
		Single: audio.SingleCast{DialogueShift: true, KeepTones: true},
		Speakers: []audio.Speaker{
			{ID: audio.NarratorID, Name: "Dẫn truyện", Role: audio.RoleNarrator, VoiceID: &danVoice},
			{ID: "nv", Name: "Nhân vật", Role: audio.RoleMain, VoiceID: &nvVoice},
		},
	}))

	saved := decodeCast(call("PUT", "/audio/cast/single", seeded.Version, map[string]any{
		"voiceId": "giong-ke", "rerenderOld": false,
	}))
	if saved.Single.VoiceID != "giong-ke" {
		t.Fatalf("single voice = %q, want giong-ke", saved.Single.VoiceID)
	}
	if saved.QueuedSegments != 0 {
		t.Fatalf("rerenderOld=false queued %d", saved.QueuedSegments)
	}
	if saved.Version == seeded.Version {
		t.Fatal("saving the single voice did not move the version token")
	}
	for _, speaker := range saved.Speakers {
		if speaker.ID == "single" {
			t.Fatalf("single mode created a fake role row: %+v", speaker)
		}
	}
	if voiceID(saved, audio.NarratorID) != danVoice || voiceID(saved, "nv") != nvVoice {
		t.Fatalf("single selection overwrote per-role choices: %+v", saved.Speakers)
	}

	// Reload: the sheet reads the cast again on the next open.
	reloaded := decodeCast(call("GET", "/audio/cast", "", nil))
	if reloaded.Single.VoiceID != "giong-ke" || voiceID(reloaded, audio.NarratorID) != danVoice || voiceID(reloaded, "nv") != nvVoice {
		t.Fatalf("reload lost the single voice or role choices: %+v", reloaded)
	}

	// Mode toggles write the whole document; they must not reset choices.
	single := decodeCast(call("PUT", "/audio/cast", reloaded.Version, audio.Cast{
		Mode: audio.ModeSingle, VoiceMode: audio.ModeSingle, Single: reloaded.Single, Speakers: reloaded.Speakers,
	}))
	back := decodeCast(call("PUT", "/audio/cast", single.Version, audio.Cast{
		Mode: audio.ModeCast, VoiceMode: audio.ModeCast, Single: single.Single, Speakers: single.Speakers,
	}))
	if back.Single.VoiceID != "giong-ke" || voiceID(back, audio.NarratorID) != danVoice || voiceID(back, "nv") != nvVoice {
		t.Fatalf("mode toggle overwrote choices: %+v", back)
	}

	// Unknown role keeps the existing 409 envelope.
	if w := call("PUT", "/audio/cast/khong-co", back.Version, map[string]any{"voiceId": "giong-dan"}); w.Code != http.StatusConflict {
		t.Fatalf("unknown role: %d %s", w.Code, w.Body.String())
	}
	// A deleted/unavailable voice cannot be selected.
	if w := call("PUT", "/audio/cast/single", back.Version, map[string]any{"voiceId": "giong-da-xoa"}); w.Code != http.StatusConflict {
		t.Fatalf("deleted voice: %d %s", w.Code, w.Body.String())
	}
	// Optimistic concurrency still rejects a stale token with the conflict payload.
	stale := call("PUT", "/audio/cast/single", seeded.Version, map[string]any{"voiceId": "giong-ke"})
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"conflict":true`) {
		t.Fatalf("stale version: %d %s", stale.Code, stale.Body.String())
	}
	if cast := decodeCast(call("GET", "/audio/cast", "", nil)); cast.Single.VoiceID != "giong-ke" {
		t.Fatalf("rejected writes changed the single voice: %q", cast.Single.VoiceID)
	}
}

type pgCastResponse struct {
	audio.Cast
	Version        string `json:"version"`
	QueuedSegments int    `json:"queuedSegments"`
}

func voiceID(cast pgCastResponse, id string) string {
	for _, speaker := range cast.Speakers {
		if speaker.ID == id && speaker.VoiceID != nil {
			return *speaker.VoiceID
		}
	}
	return ""
}
