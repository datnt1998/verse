package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func bundleSegment(id string, ordinal int32, text string) postgres.Segment {
	return postgres.Segment{
		SegmentID: id, Ordinal: ordinal, Speaker: "nguoi-ke", Text: text, Payload: []byte(`{}`),
	}
}

func bundleScriptFor(chapterID pgtype.UUID, record *domain.ChapterRecord, payload string) postgres.Script {
	return postgres.Script{
		ChapterID:      chapterID,
		RevisionNo:     1,
		SourceRevision: int32(record.Revision),
		SourceSHA256:   record.ContentSHA256,
		Status:         "draft",
		Payload:        []byte(payload),
	}
}

func bundleSegmentsByID(t *testing.T, repo *postgres.Audio, f pgFixture, chapterID pgtype.UUID) map[string]postgres.Segment {
	t.Helper()
	segments, err := repo.Segments(context.Background(), f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("list segments: %v", err)
	}
	out := make(map[string]postgres.Segment, len(segments))
	for _, seg := range segments {
		out[seg.SegmentID] = seg
	}
	return out
}

func addSelectedBundleRender(t *testing.T, repo *postgres.Audio, f pgFixture, chapterID pgtype.UUID, segmentID, body string) string {
	t.Helper()
	ctx := context.Background()
	blob := readyTestBlob(t, repo, f, "audio/"+segmentID+".mp3", body)
	key := sha256Hex("synth|" + segmentID)
	if _, err := repo.AddRender(ctx, f.scope.ProjectID, chapterID, 1, postgres.Render{
		SegmentID: segmentID, SynthKey: key, BlobID: blob.ID,
		DurationSec: 1.25, SampleRate: 24000, Parameters: []byte(`{"voice":"test"}`),
	}); err != nil {
		t.Fatalf("add render %s: %v", segmentID, err)
	}
	if _, err := repo.SelectRender(ctx, f.scope.ProjectID, chapterID, 1, segmentID, key); err != nil {
		t.Fatalf("select render %s: %v", segmentID, err)
	}
	return key
}

func requireSelectedBundleRender(t *testing.T, repo *postgres.Audio, f pgFixture, chapterID pgtype.UUID, segmentID, wantKey string) {
	t.Helper()
	got, err := repo.SelectedRender(context.Background(), f.scope.ProjectID, chapterID, 1, segmentID)
	if err != nil {
		t.Fatalf("selected render %s: %v", segmentID, err)
	}
	if got.SynthKey != wantKey || !got.Selected {
		t.Fatalf("selected render %s = %+v, want key %s selected", segmentID, got, wantKey)
	}
}

// TestAudioSaveScriptBundleSwapsStableSegmentIDs: reordering a script keeps
// the segment ids and their renders; only the ordinal columns move. The named
// ordinal unique constraint is deferred for the rewrite, so the intermediate
// state (both ids momentarily claiming ordinal 1) is legal.
func TestAudioSaveScriptBundleSwapsStableSegmentIDs(t *testing.T) {
	f := provisionNarrative(t, "audio-bundle-swap-proj", "audio-bundle-swap-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	saved, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":1}`), []postgres.Segment{
		bundleSegment("seg-a", 0, "Trời đã tối."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, nil, 0)
	if err != nil {
		t.Fatalf("initial bundle: %v", err)
	}
	before := bundleSegmentsByID(t, repo, f, chapterID)
	keyA := addSelectedBundleRender(t, repo, f, chapterID, "seg-a", "audio-a")
	keyB := addSelectedBundleRender(t, repo, f, chapterID, "seg-b", "audio-b")

	if _, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":2}`), []postgres.Segment{
		bundleSegment("seg-a", 1, "Trời đã tối."),
		bundleSegment("seg-b", 0, "Nàng nhìn ra cửa sổ."),
	}, nil, saved.Version); err != nil {
		t.Fatalf("swap ordinals: %v", err)
	}

	segments, err := repo.Segments(ctx, f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("list swapped segments: %v", err)
	}
	if len(segments) != 2 || segments[0].SegmentID != "seg-b" || segments[0].Ordinal != 0 ||
		segments[1].SegmentID != "seg-a" || segments[1].Ordinal != 1 {
		t.Fatalf("swapped segments = %+v", segments)
	}
	after := bundleSegmentsByID(t, repo, f, chapterID)
	for _, id := range []string{"seg-a", "seg-b"} {
		if after[id].Version != before[id].Version+1 {
			t.Fatalf("%s version = %d, want %d (row must be updated, not recreated)",
				id, after[id].Version, before[id].Version+1)
		}
	}
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-a", keyA)
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-b", keyB)
}

// TestAudioSaveScriptBundleInsertsBeforePreservedIDs: a new segment claims
// ordinal 0 while a preserved id still holds it. Deferral makes the insert
// legal; the preserved rows and their selected renders survive the shift.
func TestAudioSaveScriptBundleInsertsBeforePreservedIDs(t *testing.T) {
	f := provisionNarrative(t, "audio-bundle-insert-proj", "audio-bundle-insert-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	saved, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":1}`), []postgres.Segment{
		bundleSegment("seg-a", 0, "Trời đã tối."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, nil, 0)
	if err != nil {
		t.Fatalf("initial bundle: %v", err)
	}
	before := bundleSegmentsByID(t, repo, f, chapterID)
	keyA := addSelectedBundleRender(t, repo, f, chapterID, "seg-a", "audio-a")
	keyB := addSelectedBundleRender(t, repo, f, chapterID, "seg-b", "audio-b")

	if _, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":2}`), []postgres.Segment{
		bundleSegment("seg-new", 0, "Một câu mở đầu mới."),
		bundleSegment("seg-a", 1, "Trời đã tối."),
		bundleSegment("seg-b", 2, "Nàng nhìn ra cửa sổ."),
	}, nil, saved.Version); err != nil {
		t.Fatalf("insert before preserved ids: %v", err)
	}

	segments, err := repo.Segments(ctx, f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("list shifted segments: %v", err)
	}
	want := []struct {
		id   string
		text string
	}{{"seg-new", "Một câu mở đầu mới."}, {"seg-a", "Trời đã tối."}, {"seg-b", "Nàng nhìn ra cửa sổ."}}
	if len(segments) != len(want) {
		t.Fatalf("segments = %+v, want %d rows", segments, len(want))
	}
	for i, w := range want {
		if segments[i].SegmentID != w.id || segments[i].Ordinal != int32(i) || segments[i].Text != w.text {
			t.Fatalf("segment %d = %+v, want id=%s ordinal=%d text=%q", i, segments[i], w.id, i, w.text)
		}
	}
	after := bundleSegmentsByID(t, repo, f, chapterID)
	for _, id := range []string{"seg-a", "seg-b"} {
		if after[id].Version != before[id].Version+1 {
			t.Fatalf("%s version = %d, want %d (preserved row must be updated)",
				id, after[id].Version, before[id].Version+1)
		}
	}
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-a", keyA)
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-b", keyB)
}

// TestAudioSaveScriptBundleRejectsDuplicateFinalOrdinal: the deferred check is
// forced before commit, so a bundle that ends with two segments on the same
// ordinal fails as a conflict and rolls back the script, segment and render
// changes together.
func TestAudioSaveScriptBundleRejectsDuplicateFinalOrdinal(t *testing.T) {
	f := provisionNarrative(t, "audio-bundle-dup-proj", "audio-bundle-dup-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	saved, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":1}`), []postgres.Segment{
		bundleSegment("seg-a", 0, "Trời đã tối."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, nil, 0)
	if err != nil {
		t.Fatalf("initial bundle: %v", err)
	}
	before := bundleSegmentsByID(t, repo, f, chapterID)
	keyA := addSelectedBundleRender(t, repo, f, chapterID, "seg-a", "audio-a")

	_, err = repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":"bad"}`), []postgres.Segment{
		bundleSegment("seg-a", 1, "Trời đã thay đổi."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, []string{"seg-a"}, saved.Version)
	if !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("duplicate final ordinal = %v, want ErrStorageConflict", err)
	}

	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script after rollback: %v", err)
	}
	if latest.Version != saved.Version {
		t.Fatalf("script version = %d, want %d (bundle must roll back)", latest.Version, saved.Version)
	}
	var payload map[string]any
	if err := json.Unmarshal(latest.Payload, &payload); err != nil {
		t.Fatalf("script payload is not JSON: %v (%s)", err, latest.Payload)
	}
	if payload["v"] != 1.0 {
		t.Fatalf("script payload = %s, want the pre-bundle revision", latest.Payload)
	}
	after := bundleSegmentsByID(t, repo, f, chapterID)
	if len(after) != 2 {
		t.Fatalf("segment rows = %d, want 2", len(after))
	}
	for id, want := range before {
		got := after[id]
		if got.Ordinal != want.Ordinal || got.Text != want.Text || got.Version != want.Version {
			t.Fatalf("%s after rollback = %+v, want %+v", id, got, want)
		}
	}
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-a", keyA)
}

// TestAudioSaveScriptBundleStaleCASLeavesSegments: a stale expected version
// aborts before any segment row or render is touched.
func TestAudioSaveScriptBundleStaleCASLeavesSegments(t *testing.T) {
	f := provisionNarrative(t, "audio-bundle-cas-proj", "audio-bundle-cas-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	created, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":1}`), []postgres.Segment{
		bundleSegment("seg-a", 0, "Trời đã tối."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, nil, 0)
	if err != nil {
		t.Fatalf("initial bundle: %v", err)
	}
	updated, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":2}`), []postgres.Segment{
		bundleSegment("seg-a", 0, "Trời đã tối."),
		bundleSegment("seg-b", 1, "Nàng nhìn ra cửa sổ."),
	}, nil, created.Version)
	if err != nil {
		t.Fatalf("second bundle: %v", err)
	}
	before := bundleSegmentsByID(t, repo, f, chapterID)
	keyA := addSelectedBundleRender(t, repo, f, chapterID, "seg-a", "audio-a")

	if _, err := repo.SaveScriptBundle(ctx, f.scope, bundleScriptFor(chapterID, record, `{"v":"stale"}`), []postgres.Segment{
		bundleSegment("seg-a", 1, "Trời đã thay đổi."),
		bundleSegment("seg-b", 0, "Nàng nhìn ra cửa sổ."),
	}, []string{"seg-a"}, created.Version); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale CAS bundle = %v, want ErrStorageConflict", err)
	}

	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script after stale CAS: %v", err)
	}
	if latest.Version != updated.Version {
		t.Fatalf("script version = %d, want %d", latest.Version, updated.Version)
	}
	var payload map[string]any
	if err := json.Unmarshal(latest.Payload, &payload); err != nil {
		t.Fatalf("script payload is not JSON: %v (%s)", err, latest.Payload)
	}
	if payload["v"] != 2.0 {
		t.Fatalf("stale bundle changed the script payload: %s", latest.Payload)
	}
	after := bundleSegmentsByID(t, repo, f, chapterID)
	for id, want := range before {
		got := after[id]
		if got.Ordinal != want.Ordinal || got.Text != want.Text || got.Version != want.Version {
			t.Fatalf("%s after stale CAS = %+v, want %+v", id, got, want)
		}
	}
	requireSelectedBundleRender(t, repo, f, chapterID, "seg-a", keyA)
}
