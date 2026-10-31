package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// acceptTestChapter writes chapter 1 with an accepted revision so audio rows
// can bind to a real (chapter, revision) pair, then returns the chapter UUID.
func acceptTestChapter(t *testing.T, f pgFixture) (pgtype.UUID, *domain.ChapterRecord) {
	t.Helper()
	rec, err := f.nar.AcceptRecord(context.Background(), f.scope, 1, domain.ChapterOriginGenerated,
		"Trời đã tối. Nàng nhìn ra cửa sổ, lặng im.", domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"})
	if err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	var chapterID pgtype.UUID
	if err := f.pool.Pool().QueryRow(context.Background(),
		`SELECT id FROM chapters WHERE project_id = $1 AND story_id = $2 AND number = 1`,
		f.scope.ProjectID, f.scope.StoryID).Scan(&chapterID); err != nil {
		t.Fatalf("load chapter id: %v", err)
	}
	return chapterID, rec
}

// readyTestBlob registers verified bytes as one ready registry row.
func readyTestBlob(t *testing.T, repo *postgres.Audio, f pgFixture, key, body string) postgres.Blob {
	t.Helper()
	ctx := context.Background()
	sum := sha256.Sum256([]byte(body))
	ready, err := repo.PublishBlob(ctx, postgres.PublishInput{ProjectID: f.scope.ProjectID, Key: key, Driver: "local", ContentType: "audio/mpeg", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))})
	if err != nil {
		t.Fatalf("mark blob ready: %v", err)
	}
	if ready.State != "ready" {
		t.Fatalf("blob state = %q, want ready", ready.State)
	}
	return ready
}

// TestAudioSettingsRoundTrip: settings are per story, survive an overwrite and
// report a missing row honestly instead of inventing defaults.
func TestAudioSettingsRoundTrip(t *testing.T) {
	f := provisionNarrative(t, "audio-set-proj", "audio-set-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()

	if _, _, err := repo.Settings(ctx, f.scope); !errors.Is(err, postgres.ErrAudioSettingsMissing) {
		t.Fatalf("missing settings = %v, want ErrAudioSettingsMissing", err)
	}
	v1, err := repo.SaveSettings(ctx, f.scope, map[string]any{"speed": 1.0, "tone": "kể chuyện"})
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}
	body, ver, err := repo.Settings(ctx, f.scope)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if ver != v1 {
		t.Fatalf("settings version = %d, want %d", ver, v1)
	}
	// jsonb normalises formatting/key order, so compare the decoded shape.
	var stored map[string]any
	if err := json.Unmarshal(body, &stored); err != nil {
		t.Fatalf("settings payload is not JSON: %v (%s)", err, body)
	}
	if stored["speed"] != 1.0 || stored["tone"] != "kể chuyện" {
		t.Fatalf("settings payload = %v", stored)
	}
	v2, err := repo.SaveSettings(ctx, f.scope, map[string]any{"speed": 1.25})
	if err != nil {
		t.Fatalf("overwrite settings: %v", err)
	}
	if v2 != v1+1 {
		t.Fatalf("settings version = %d, want %d", v2, v1+1)
	}
}

// TestAudioVoicesAndCast: voice ids stay stable across metadata updates, cast
// references them, and a referenced voice cannot be deleted silently.
func TestAudioVoicesAndCast(t *testing.T) {
	f := provisionNarrative(t, "audio-voice-proj", "audio-voice-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()

	voice, err := repo.UpsertVoice(ctx, f.scope.ProjectID, "vieneu-nu-bac", map[string]any{"label": "Nữ Bắc"}, pgtype.UUID{})
	if err != nil {
		t.Fatalf("upsert voice: %v", err)
	}
	updated, err := repo.UpsertVoice(ctx, f.scope.ProjectID, "vieneu-nu-bac", map[string]any{"label": "Nữ Bắc (mới)"}, pgtype.UUID{})
	if err != nil {
		t.Fatalf("update voice: %v", err)
	}
	if updated.ID != voice.ID {
		t.Fatalf("voice id changed on update: %v -> %v", voice.ID, updated.ID)
	}
	if updated.Version != voice.Version+1 {
		t.Fatalf("voice version = %d, want %d", updated.Version, voice.Version+1)
	}

	cast, err := repo.SaveCast(ctx, f.scope, "nguoi-ke", voice.ID, map[string]any{"role": "narration"})
	if err != nil {
		t.Fatalf("save cast: %v", err)
	}
	if cast.VoiceID != voice.ID {
		t.Fatalf("cast voice = %v, want %v", cast.VoiceID, voice.ID)
	}
	roles, err := repo.ListCast(ctx, f.scope)
	if err != nil {
		t.Fatalf("list cast: %v", err)
	}
	if len(roles) != 1 || roles[0].RoleKey != "nguoi-ke" {
		t.Fatalf("cast = %+v", roles)
	}

	if err := repo.DeleteVoice(ctx, f.scope.ProjectID, "vieneu-nu-bac"); err == nil {
		t.Fatal("deleted a voice still referenced by the cast")
	}
	if err := repo.DeleteCast(ctx, f.scope, "nguoi-ke"); err != nil {
		t.Fatalf("delete cast: %v", err)
	}
	if err := repo.DeleteVoice(ctx, f.scope.ProjectID, "vieneu-nu-bac"); err != nil {
		t.Fatalf("delete unreferenced voice: %v", err)
	}
	if _, err := repo.ListVoices(ctx, f.scope.ProjectID); err != nil {
		t.Fatalf("list voices: %v", err)
	}
}

// TestAudioScriptSegmentsRenders is the audio-metadata gate: a script binds the
// accepted revision + hash, segments key by script revision, and exactly one
// render per segment is selected — a re-render with new parameters adds a
// historical candidate instead of overwriting the selected one.
func TestAudioScriptSegmentsRenders(t *testing.T) {
	f := provisionNarrative(t, "audio-render-proj", "audio-render-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	script, err := repo.SaveScript(ctx, f.scope, postgres.Script{
		ChapterID:      chapterID,
		RevisionNo:     1,
		SourceRevision: int32(record.Revision),
		SourceSHA256:   record.ContentSHA256,
		Status:         "draft",
		Payload:        []byte(`{"segments":2}`),
	})
	if err != nil {
		t.Fatalf("save script: %v", err)
	}
	if script.SourceSHA256 != record.ContentSHA256 {
		t.Fatalf("script source hash = %q, want %q", script.SourceSHA256, record.ContentSHA256)
	}
	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script: %v", err)
	}
	if latest.RevisionNo != 1 || latest.SourceRevision != int32(record.Revision) {
		t.Fatalf("latest script = %+v", latest)
	}

	for i, text := range []string{"Trời đã tối.", "Nàng nhìn ra cửa sổ."} {
		seg, err := repo.SaveSegment(ctx, f.scope.ProjectID, chapterID, 1, postgres.Segment{
			SegmentID: segmentKey(i), Ordinal: int32(i), Speaker: "nguoi-ke", Text: text, Payload: []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("save segment %d: %v", i, err)
		}
		if seg.Ordinal != int32(i) {
			t.Fatalf("segment ordinal = %d, want %d", seg.Ordinal, i)
		}
	}
	segments, err := repo.Segments(ctx, f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("segments: %v", err)
	}
	if len(segments) != 2 || segments[0].SegmentID != segmentKey(0) {
		t.Fatalf("segments = %+v", segments)
	}

	oldBlob := readyTestBlob(t, repo, f, "audio/old.mp3", "old-voice")
	newBlob := readyTestBlob(t, repo, f, "audio/new.mp3", "new-voice")
	// Keys come from the audio domain's SynthKey.Hash — one algorithm shared by
	// file and postgres modes.
	oldKey := audio.SynthKey{Text: "Trời đã tối.", VoiceID: "vieneu-nu-bac", Tone: "ke", Speed: 1, Pitch: 1, Quality: "high", Engine: "vieneu", Model: "v1"}.Hash()
	newKey := audio.SynthKey{Text: "Trời đã tối.", VoiceID: "vieneu-nu-bac", Tone: "ke", Speed: 1.2, Pitch: 1, Quality: "high", Engine: "vieneu", Model: "v1"}.Hash()
	if oldKey == newKey {
		t.Fatal("changing speed did not change the synth key")
	}

	if _, err := repo.AddRender(ctx, f.scope.ProjectID, chapterID, 1, postgres.Render{
		SegmentID: segmentKey(0), SynthKey: oldKey, BlobID: oldBlob.ID, DurationSec: 1.5, SampleRate: 24000,
		Parameters: []byte(`{"speed":1}`),
	}); err != nil {
		t.Fatalf("add old render: %v", err)
	}
	selected, err := repo.SelectRender(ctx, f.scope.ProjectID, chapterID, 1, segmentKey(0), oldKey)
	if err != nil {
		t.Fatalf("select old render: %v", err)
	}
	if !selected.Selected {
		t.Fatal("selected render is not selected")
	}

	// Re-rendering with new parameters is a new candidate; selecting it must
	// leave the old one as an unselected historical candidate.
	if _, err := repo.AddRender(ctx, f.scope.ProjectID, chapterID, 1, postgres.Render{
		SegmentID: segmentKey(0), SynthKey: newKey, BlobID: newBlob.ID, DurationSec: 1.7, SampleRate: 24000,
		Parameters: []byte(`{"speed":1.2}`),
	}); err != nil {
		t.Fatalf("add new render: %v", err)
	}
	if _, err := repo.SelectRender(ctx, f.scope.ProjectID, chapterID, 1, segmentKey(0), newKey); err != nil {
		t.Fatalf("select new render: %v", err)
	}

	var selectedCount int
	if err := f.pool.Pool().QueryRow(ctx, `
		SELECT count(*) FROM audio_renders
		WHERE project_id = $1 AND chapter_id = $2 AND script_revision = 1
		  AND segment_id = $3 AND selected`,
		f.scope.ProjectID, chapterID, segmentKey(0)).Scan(&selectedCount); err != nil {
		t.Fatalf("count selected: %v", err)
	}
	if selectedCount != 1 {
		t.Fatalf("selected renders = %d, want exactly 1", selectedCount)
	}
	renders, err := repo.Renders(ctx, f.scope.ProjectID, chapterID, 1)
	if err != nil {
		t.Fatalf("list renders: %v", err)
	}
	if len(renders) != 2 {
		t.Fatalf("candidates = %d, want 2 (old kept as history)", len(renders))
	}
	current, err := repo.SelectedRender(ctx, f.scope.ProjectID, chapterID, 1, segmentKey(0))
	if err != nil {
		t.Fatalf("selected render: %v", err)
	}
	if current.SynthKey != newKey || current.BlobID != newBlob.ID {
		t.Fatalf("current render = %+v, want the new candidate", current)
	}

	// A malformed synth key is rejected before it reaches the database.
	if _, err := repo.AddRender(ctx, f.scope.ProjectID, chapterID, 1, postgres.Render{
		SegmentID: segmentKey(0), SynthKey: "not-a-hash", BlobID: newBlob.ID, DurationSec: 1, SampleRate: 24000,
	}); err == nil {
		t.Fatal("accepted an invalid synth key")
	}
}

func segmentKey(i int) string {
	return "seg-" + string(rune('a'+i))
}

// TestAudioSynthKeyIsDeterministic proves the render key is a pure function of
// the parameters, matching what the DB stores: same inputs → same 64-hex key,
// a changed engine/speed → a different key.
func TestAudioSynthKeyIsDeterministic(t *testing.T) {
	params := audio.SynthKey{
		Text: "Trời đã tối.", VoiceID: "vieneu-nu-bac", Tone: "ke", Speed: 1.1, Pitch: 1,
		Quality: "high", Engine: "vieneu", Model: "v1",
	}
	a := params.Hash()
	if b := params.Hash(); a != b {
		t.Fatalf("synth key not deterministic: %s vs %s", a, b)
	}
	params.Engine = "other"
	if c := params.Hash(); a == c {
		t.Fatal("changing the engine did not change the synth key")
	}
	params = audio.SynthKey{Text: "Trời đã tối.", VoiceID: "vieneu-nu-bac", Tone: "ke", Speed: 1.1, Pitch: 1,
		Quality: "high", Engine: "vieneu", Model: "v1"}
	if a != params.Hash() {
		t.Fatal("synth key is not stable across identical inputs")
	}
}

// TestExportCacheHashAndExpiry: entries resolve by content key only; the app
// marks expiry without deleting registry rows or shared objects.
func TestExportCacheHashAndExpiry(t *testing.T) {
	f := provisionNarrative(t, "audio-cache-proj", "audio-cache-story")
	repo := postgres.NewAudio(f.pool)
	cache := postgres.NewExportCache(f.pool)
	ctx := context.Background()

	blob := readyTestBlob(t, repo, f, "exports/chapter-1.m4b", "m4b-bytes")
	key := sha256Hex("src=rev3|cast=v1|settings=v2|opts=intro")

	if _, err := cache.Get(ctx, f.scope, key, "m4b"); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("cold cache = %v, want ErrJobNotFound", err)
	}
	if _, err := cache.Put(ctx, f.scope, postgres.ExportCacheEntry{
		CacheKey: key, Kind: "m4b", BlobID: blob.ID, InputManifest: []byte(`{"revision":3}`),
	}, nil); err != nil {
		t.Fatalf("put cache: %v", err)
	}
	hit, err := cache.Get(ctx, f.scope, key, "m4b")
	if err != nil {
		t.Fatalf("hit cache: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(hit.InputManifest, &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	if hit.BlobID != blob.ID || manifest["revision"] != 3.0 {
		t.Fatalf("cache hit = %+v manifest=%v", hit, manifest)
	}

	// A different content key must not hit the same entry.
	other := sha256Hex("src=rev4|cast=v1|settings=v2|opts=intro")
	if _, err := cache.Get(ctx, f.scope, other, "m4b"); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("different key = %v, want ErrJobNotFound", err)
	}

	// Expiry is data, not a filesystem timestamp.
	past := time.Now().Add(-time.Minute)
	if _, err := cache.Put(ctx, f.scope, postgres.ExportCacheEntry{
		CacheKey: other, Kind: "m4b", BlobID: blob.ID, InputManifest: []byte(`{}`),
	}, &past); err != nil {
		t.Fatalf("put expired: %v", err)
	}
	if _, err := cache.Get(ctx, f.scope, other, "m4b"); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("expired entry = %v, want ErrJobNotFound", err)
	}
	if n, err := cache.Expire(ctx, f.scope); err != nil || n != 1 {
		t.Fatalf("expire live entry = %d, %v; want 1", n, err)
	}
	if _, err := cache.Get(ctx, f.scope, key, "m4b"); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("invalidated live entry = %v, want ErrJobNotFound", err)
	}
	var rows int
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM export_cache`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("app expiry deleted rows: count=%d err=%v", rows, err)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestAudioScriptReplaceIsCAS: an externally versioned script replacement
// carries the expected row version inside the write transaction. A stale
// version (two editors/two requests) must be a conflict and leave the stored
// script untouched; type-0 creates only when no row exists.
func TestAudioScriptReplaceIsCAS(t *testing.T) {
	f := provisionNarrative(t, "audio-cas-proj", "audio-cas-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	created, err := repo.SaveScript(ctx, f.scope, postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "draft",
		Payload: []byte(`{"v":1}`),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("created version = %d, want 1", created.Version)
	}

	next := postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "draft",
		Payload: []byte(`{"v":2}`),
	}
	saved, err := repo.SaveScriptBundle(ctx, f.scope, next, nil, nil, created.Version)
	if err != nil {
		t.Fatalf("first CAS save: %v", err)
	}
	if saved.Version != created.Version+1 {
		t.Fatalf("saved version = %d, want %d", saved.Version, created.Version+1)
	}

	// Same expected version now: zero rows in UPDATE ... WHERE version = $n.
	next.Payload = []byte(`{"v":3}`)
	if _, err := repo.SaveScriptBundle(ctx, f.scope, next, nil, nil, created.Version); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("stale CAS save = %v, want ErrStorageConflict", err)
	}
	latest, err := repo.LatestScript(ctx, f.scope.ProjectID, chapterID)
	if err != nil {
		t.Fatalf("latest script: %v", err)
	}
	var stored map[string]int
	if err := json.Unmarshal(latest.Payload, &stored); err != nil {
		t.Fatalf("latest payload is not JSON: %v (%s)", err, latest.Payload)
	}
	if stored["v"] != 2 || latest.Version != saved.Version {
		t.Fatalf("stale write changed the row: payload=%s version=%d", latest.Payload, latest.Version)
	}

	// Create-required (expected 0) against an existing row cannot overwrite it.
	if _, err := repo.SaveScriptBundle(ctx, f.scope, next, nil, nil, 0); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("create over existing = %v, want ErrStorageConflict", err)
	}

	// A genuinely new revision can be created with expected 0.
	next.RevisionNo = 2
	if _, err := repo.SaveScriptBundle(ctx, f.scope, next, nil, nil, 0); err != nil {
		t.Fatalf("create revision 2: %v", err)
	}
}

// TestAudioScriptRejectsInvalidMetadata: the repository returns a domain
// input error before the SQL CHECK, and the CHECK itself is the backstop for
// callers that bypass it.
func TestAudioScriptRejectsInvalidMetadata(t *testing.T) {
	f := provisionNarrative(t, "audio-check-proj", "audio-check-story")
	repo := postgres.NewAudio(f.pool)
	ctx := context.Background()
	chapterID, record := acceptTestChapter(t, f)

	bad := postgres.Script{
		ChapterID: chapterID, RevisionNo: 1, SourceRevision: int32(record.Revision),
		SourceSHA256: record.ContentSHA256, Status: "not-a-status", Payload: []byte(`{}`),
	}
	if _, err := repo.SaveScript(ctx, f.scope, bad); err == nil {
		t.Fatal("invalid status accepted by the repository")
	}
	bad.Status = "draft"
	bad.SourceSHA256 = "XYZ"
	if _, err := repo.SaveScript(ctx, f.scope, bad); err == nil {
		t.Fatal("invalid source_sha256 accepted by the repository")
	}

	// Direct SQL bypass: the migration CHECK is the integrity backstop.
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO audio_scripts
		(project_id, story_id, chapter_id, revision_no, source_revision,
		 source_sha256, status, payload)
		VALUES ($1, $2, $3, 1, $4, $5, 'bogus', '{}')`,
		f.scope.ProjectID, f.scope.StoryID, chapterID, int32(record.Revision),
		record.ContentSHA256); err == nil {
		t.Fatal("audio_scripts.status CHECK did not reject an invalid state")
	}
	if _, err := f.pool.Pool().Exec(ctx, `INSERT INTO audio_scripts
		(project_id, story_id, chapter_id, revision_no, source_revision,
		 source_sha256, status, payload)
		VALUES ($1, $2, $3, 1, $4, 'not-a-digest', 'draft', '{}')`,
		f.scope.ProjectID, f.scope.StoryID, chapterID, int32(record.Revision)); err == nil {
		t.Fatal("audio_scripts.source_sha256 CHECK did not reject a malformed digest")
	}
}
