package audio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgtest"
)

// Hosted single narration voice: "single" is a mode, not a speaker. Selecting
// a voice must persist Cast.Single.VoiceID, survive a reload and never create
// a fake "single" cast row — while cast↔single toggles keep every per-role
// choice. RerenderOld=true must target every segment that actually went stale
// (the whole chapter in single mode), and false must leave existing audio
// untouched.
func TestPGSingleVoiceSelectionPersistsAndRerenders(t *testing.T) {
	ctx := context.Background()
	_, nar, scope, repo, svc := pgCastFixture(t, 0.5)

	// A cast already in single mode, with real per-role choices retained from
	// cast mode before the user switched.
	danVoice, nvVoice := "giong-dan", "giong-nv"
	if _, err := svc.SaveCast(&Cast{
		Mode: ModeSingle, VoiceMode: ModeSingle,
		Single: SingleCast{DialogueShift: true, KeepTones: true},
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &danVoice},
			{ID: "nv", Name: "Nhân vật", Role: RoleMain, VoiceID: &nvVoice},
		},
	}); err != nil {
		t.Fatalf("seed cast: %v", err)
	}
	text := "Trời đã tối. Nàng nhìn ra cửa sổ."
	chapter := pgCastLockedScript(t, ctx, nar, scope, repo, text)

	saved, queued, err := svc.ChangeRoleVoice(ctx, ModeSingle, "giong-ke", "", false)
	if err != nil {
		t.Fatalf("select single voice: %v", err)
	}
	if queued != 0 {
		t.Fatalf("rerenderOld=false queued %d segments", queued)
	}
	if saved.Single.VoiceID != "giong-ke" {
		t.Fatalf("single voice = %q, want giong-ke", saved.Single.VoiceID)
	}
	if saved.VoiceMode != ModeSingle {
		t.Fatalf("voiceMode = %q, want existing mode kept", saved.VoiceMode)
	}
	for _, speaker := range saved.Speakers {
		if speaker.ID == "single" {
			t.Fatalf("single mode created a fake speaker row: %+v", speaker)
		}
	}
	if voiceOf(saved, "nv") != nvVoice || voiceOf(saved, NarratorID) != danVoice {
		t.Fatalf("single selection overwrote per-role choices: %+v", saved.Speakers)
	}
	if rows, err := svc.deps.Jobs.List(ctx, scope); err != nil || len(rows) != 0 {
		t.Fatalf("rerenderOld=false wrote jobs: %d %v", len(rows), err)
	}

	// Reload through the public read path: the narration voice is still there.
	reloaded, err := svc.Cast(CharacterRef{})
	if err != nil {
		t.Fatalf("reload cast: %v", err)
	}
	if reloaded.Single.VoiceID != "giong-ke" || voiceOf(reloaded, "nv") != nvVoice || voiceOf(reloaded, NarratorID) != danVoice {
		t.Fatalf("reload lost state: %+v", reloaded)
	}

	// Old-audio confirmation: every rendered segment is stale under the new
	// narration voice, and the ETA uses the worker's measured RTF.
	if err := svc.CastOldSegments(reloaded); err != nil {
		t.Fatalf("old segments: %v", err)
	}
	total, eta := 0, 0.0
	for _, speaker := range reloaded.Speakers {
		total += speaker.OldSegments
		eta += speaker.OldETASec
	}
	if total != len(chapter.Segments) {
		t.Fatalf("old segments = %d, want %d", total, len(chapter.Segments))
	}
	if eta <= 0 {
		t.Fatalf("old ETA missing with a measured RTF: %+v", reloaded.Speakers)
	}

	// Explicit true: queue every stale single-mode segment, not a role filter.
	saved, queued, err = svc.ChangeRoleVoice(ctx, ModeSingle, "giong-moi", "", true)
	if err != nil {
		t.Fatalf("rerender single voice: %v", err)
	}
	if queued != len(chapter.Segments) {
		t.Fatalf("queued single segments = %d, want %d", queued, len(chapter.Segments))
	}
	rows, err := svc.deps.Jobs.List(ctx, scope)
	if err != nil || len(rows) != 1 {
		t.Fatalf("single rerender jobs = %d %v", len(rows), err)
	}
	var input pgTTSInput
	if err := json.Unmarshal(rows[0].Input, &input); err != nil {
		t.Fatalf("job input: %v", err)
	}
	if len(input.Segments) != len(chapter.Segments) {
		t.Fatalf("job segments = %d, want all %d", len(input.Segments), len(chapter.Segments))
	}
	if saved.Single.VoiceID != "giong-moi" {
		t.Fatalf("saved single voice = %q", saved.Single.VoiceID)
	}
}

// A builtin preset comes from the worker catalogue, not the project voice
// table; selecting it must be accepted and saved as the narration voice.
func TestPGSingleVoiceAcceptsWorkerPreset(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, svc := pgCastFixture(t, 0)
	svc.deps.RemoteVoices = func(context.Context) []ProjectVoice {
		return []ProjectVoice{{ID: "Adam", Name: "Adam", BuiltIn: true}}
	}
	saved, _, err := svc.ChangeRoleVoice(ctx, ModeSingle, "Adam", "", false)
	if err != nil {
		t.Fatalf("select builtin preset: %v", err)
	}
	if saved.Single.VoiceID != "Adam" {
		t.Fatalf("builtin single voice = %q", saved.Single.VoiceID)
	}
	reloaded, err := svc.Cast(CharacterRef{})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Single.VoiceID != "Adam" {
		t.Fatalf("reload lost the builtin preset: %+v", reloaded.Single)
	}
}

// A single voice saved before the cast document has speakers (first panel
// open, or an API client) must survive Cast() filling in the default roster.
func TestPGSingleVoiceSurvivesCastInitialization(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, svc := pgCastFixture(t, 0)
	if _, _, err := svc.ChangeRoleVoice(ctx, ModeSingle, "giong-ke", "", false); err != nil {
		t.Fatalf("select single voice: %v", err)
	}
	cast, err := svc.Cast(CharacterRef{})
	if err != nil {
		t.Fatalf("initialize cast: %v", err)
	}
	if cast.Single.VoiceID != "giong-ke" {
		t.Fatalf("cast initialization dropped the single voice: %+v", cast.Single)
	}
	if len(cast.Speakers) == 0 {
		t.Fatal("cast initialization produced no speakers")
	}
}

// Mode toggles write the whole cast document; they must not overwrite the
// per-role voices or the single narration voice.
func TestPGVoiceModeToggleKeepsPerRoleChoices(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, svc := pgCastFixture(t, 0)

	danVoice, nvVoice := "giong-dan", "giong-nv"
	seed := &Cast{
		Mode: ModeCast, VoiceMode: ModeCast,
		Single: SingleCast{DialogueShift: true, KeepTones: true},
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &danVoice},
			{ID: "nv", Name: "Nhân vật", Role: RoleMain, VoiceID: &nvVoice},
		},
	}
	if _, err := svc.SaveCast(seed); err != nil {
		t.Fatalf("seed cast: %v", err)
	}
	if _, _, err := svc.ChangeRoleVoice(ctx, ModeSingle, "giong-ke", "", false); err != nil {
		t.Fatalf("select single voice: %v", err)
	}
	for _, mode := range []string{ModeSingle, ModeCast} {
		cast, err := svc.Cast(CharacterRef{})
		if err != nil {
			t.Fatalf("load cast: %v", err)
		}
		cast.Mode = mode
		cast.VoiceMode = mode
		saved, err := svc.SaveCast(cast)
		if err != nil {
			t.Fatalf("toggle to %s: %v", mode, err)
		}
		if saved.Single.VoiceID != "giong-ke" || voiceOf(saved, "nv") != nvVoice || voiceOf(saved, NarratorID) != danVoice {
			t.Fatalf("toggle to %s overwrote choices: %+v", mode, saved)
		}
	}
}

// Invalid role, an unavailable/deleted voice and a sameAs cycle must all fail
// before anything is written, and the narrator/character share must be broken
// cleanly when the character takes its own voice.
func TestPGRoleVoiceValidationContracts(t *testing.T) {
	ctx := context.Background()
	_, _, _, _, svc := pgCastFixture(t, 0)

	danVoice, nvVoice := "giong-dan", "giong-nv"
	seed := &Cast{
		Mode: ModeCast, VoiceMode: ModeCast,
		Single: SingleCast{DialogueShift: true, KeepTones: true},
		Speakers: []Speaker{
			{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &danVoice},
			{ID: "nv", Name: "Nhân vật", Role: RoleMain, VoiceID: &nvVoice},
		},
	}
	if _, err := svc.SaveCast(seed); err != nil {
		t.Fatalf("seed cast: %v", err)
	}

	if _, _, err := svc.ChangeRoleVoice(ctx, "khong-co", "giong-dan", "", false); err == nil {
		t.Fatal("unknown role must be rejected")
	} else {
		requireStateError(t, err)
	}
	if _, _, err := svc.ChangeRoleVoice(ctx, ModeSingle, "giong-da-xoa", "", false); err == nil {
		t.Fatal("deleted voice must be rejected")
	} else {
		requireStateError(t, err)
	}
	// "single" is a mode, not a speaker: a share request is a client error,
	// not something to silently drop while saving the voice.
	if _, _, err := svc.ChangeRoleVoice(ctx, ModeSingle, "giong-dan", NarratorID, false); err == nil {
		t.Fatal("single sameAs must be rejected")
	} else {
		requireStateError(t, err)
	}
	if cast, _ := svc.Cast(CharacterRef{}); cast.Single.VoiceID != "" {
		t.Fatalf("rejected voice wrote single voice: %q", cast.Single.VoiceID)
	}

	// The character shares the narrator's voice; picking its own voice must
	// clear the narrator's sameAs and record the narration identity, never
	// leave the two pointing at each other.
	shared, err := svc.Cast(CharacterRef{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range shared.Speakers {
		if shared.Speakers[i].ID == NarratorID {
			shared.Speakers[i].SameAs = "nv"
		}
	}
	if _, err := svc.SaveCast(shared); err != nil {
		t.Fatal(err)
	}
	saved, _, err := svc.ChangeRoleVoice(ctx, "nv", "giong-moi", "", false)
	if err != nil {
		t.Fatalf("recast shared role: %v", err)
	}
	if saved.NarratorCharacterID != "nv" {
		t.Fatalf("narratorCharacterId = %q", saved.NarratorCharacterID)
	}
	for _, speaker := range saved.Speakers {
		if speaker.ID == NarratorID && speaker.SameAs != "" {
			t.Fatalf("narrator kept a sameAs cycle: %+v", speaker)
		}
	}
	// Only the narrator may be borrowed, and never by the narrator itself.
	if _, _, err := svc.ChangeRoleVoice(ctx, "nv", "giong-moi", "nv", false); err == nil {
		t.Fatal("self sameAs must be rejected")
	}
	if _, _, err := svc.ChangeRoleVoice(ctx, NarratorID, "giong-moi", NarratorID, false); err == nil {
		t.Fatal("narrator self sameAs must be rejected")
	}
	allowed, _, err := svc.ChangeRoleVoice(ctx, "nv", "giong-moi", NarratorID, false)
	if err != nil {
		t.Fatalf("narrator share: %v", err)
	}
	for _, speaker := range allowed.Speakers {
		if speaker.ID == "nv" && speaker.SameAs != NarratorID {
			t.Fatalf("narrator share not saved: %+v", speaker)
		}
	}
}

// Without a measured RTF the hosted backend must not invent an ETA; the
// mobile confirmation then shows its designed "chưa đủ dữ liệu" state.
func TestPGCastOldSegmentsOmitsEtaWithoutWorkerRTF(t *testing.T) {
	ctx := context.Background()
	_, nar, scope, repo, svc := pgCastFixture(t, 0)
	text := "Trời đã tối. Nàng nhìn ra cửa sổ."
	pgCastLockedScript(t, ctx, nar, scope, repo, text)
	cast, err := svc.Cast(CharacterRef{})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CastOldSegments(cast); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, speaker := range cast.Speakers {
		total += speaker.OldSegments
		if speaker.OldETASec != 0 {
			t.Fatalf("invented ETA without RTF: %+v", speaker)
		}
	}
	if total == 0 {
		t.Fatal("test script produced no stale segments")
	}
}

// pgCastFixture provisions one isolated story with the project voices the
// cast tests reference and a hosted audio service. rtf <= 0 leaves the worker
// RTF unmeasured.
func pgCastFixture(t *testing.T, rtf float64) (*postgres.Pool, *postgres.Narrative, postgres.ScopeRef, *postgres.Audio, *PGService) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.Up(context.Background(), db)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()
	projectID, err := postgres.ParseUUID("2f0f5b30-4d24-4b7c-9d0c-3f6f7c2a1b10")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.New(pool.Pool()).EnsureProject(ctx, gen.EnsureProjectParams{ID: projectID, Slug: "cast-proj", Name: "Cast project"}); err != nil {
		t.Fatal(err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, "cast-proj", nil, postgres.StoryInfo{Slug: "cast-story", Title: "Truyện thử", Status: "writing"}); err != nil {
		t.Fatal(err)
	}
	scope, err := nar.Resolve(ctx, "cast-proj", "cast-story", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewAudio(pool)
	for _, voice := range []struct{ id, name string }{{"giong-dan", "Giọng dẫn"}, {"giong-nv", "Giọng nhân vật"}, {"giong-ke", "Giọng kể"}, {"giong-moi", "Giọng mới"}} {
		if _, err := repo.UpsertVoice(ctx, scope.ProjectID, voice.id, map[string]any{"name": voice.name}, pgtype.UUID{}); err != nil {
			t.Fatalf("seed voice %s: %v", voice.id, err)
		}
	}
	deps := PGDeps{
		Jobs: postgres.NewJobs(pool), Audio: repo, Cache: postgres.NewExportCache(pool),
		Nar: nar, Project: "cast-proj", Story: "cast-story", Engine: "remote", Model: "remote",
	}
	if rtf > 0 {
		value := rtf
		deps.RemoteRTF = func(context.Context) *float64 { return &value }
	}
	svc := NewPGService(deps)
	t.Cleanup(svc.Close)
	return pool, nar, scope, repo, svc
}

// pgCastLockedScript stores one committed chapter and a locked script with a
// rendered segment per speaker, keyed to parameters that the next cast change
// invalidates.
func pgCastLockedScript(t *testing.T, ctx context.Context, nar *postgres.Narrative, scope postgres.ScopeRef, repo *postgres.Audio, text string) *Script {
	t.Helper()
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated, text, domain.ChapterFacts{}, domain.StyleDelta{}, postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatal(err)
	}
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	script := &Script{
		Chapter: 1, Volume: 1, Status: StatusLocked, SourceHash: SourceHash(text),
		Segments: []Segment{
			{ID: "s0001", Speaker: NarratorID, Text: "Trời đã tối."},
			{ID: "s0002", Speaker: "nv", Text: "Nàng nhìn ra cửa sổ."},
		},
	}
	payload, _ := json.Marshal(script)
	if _, err := repo.SaveScript(ctx, scope, postgres.Script{
		ChapterID: ref.ID, RevisionNo: 1, SourceRevision: int32(ref.CurrentRevision),
		SourceSHA256: script.SourceHash, Status: StatusLocked, Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}
	store, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wav := silenceWAV(200, 48000)
	sum := sha256.Sum256(wav)
	digest := hex.EncodeToString(sum[:])
	blob, err := postgres.UploadAndPublishBlob(ctx, repo, store, postgres.PublishInput{
		ProjectID: scope.ProjectID, Key: "test/cast-single.wav", Driver: "local",
		ContentType: "audio/wav", SHA256: digest, Size: int64(len(wav)),
	}, bytes.NewReader(wav))
	if err != nil {
		t.Fatal(err)
	}
	// An old key that cannot match the current cast parameters: after the
	// voice change every rendered segment is stale.
	staleKey := strings.Repeat("a", 64)
	for i, segment := range script.Segments {
		if _, err := repo.SaveSegment(ctx, scope.ProjectID, ref.ID, 1, postgres.Segment{
			SegmentID: segment.ID, Ordinal: int32(i), Speaker: segment.Speaker, Text: segment.Text, Payload: []byte(`{}`),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.AddRender(ctx, scope.ProjectID, ref.ID, 1, postgres.Render{
			SegmentID: segment.ID, SynthKey: staleKey, BlobID: blob.ID, DurationSec: 4, SampleRate: 48000,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.SelectRender(ctx, scope.ProjectID, ref.ID, 1, segment.ID, staleKey); err != nil {
			t.Fatal(err)
		}
	}
	return script
}

func voiceOf(cast *Cast, id string) string {
	for _, speaker := range cast.Speakers {
		if speaker.ID == id && speaker.VoiceID != nil {
			return *speaker.VoiceID
		}
	}
	return ""
}

func requireStateError(t *testing.T, err error) {
	t.Helper()
	var state *StateError
	if !errors.As(err, &state) {
		t.Fatalf("error = %v (%T), want state error", err, err)
	}
}
