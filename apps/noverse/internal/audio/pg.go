package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Postgres audio backend (phase 07). Durable state lives in the audio_*/jobs
// tables; every binary object is an immutable BlobStore object referenced by a
// blobs registry row — no story directory is ever touched.
//
// Equivalence with the file backend:
//   - queue/enqueue go through the durable jobs table with the same Gói AK
//     fields (kind create|update|redo, redo flag, position, updateEtaSec)
//     frozen into the job input;
//   - timeline/flags/segments/renders read from the tables;
//   - playback streams the selected render's blob;
//   - flags/pronunciations/export prefs live in the story's settings document
//     (audio_settings.payload) since they are small user-edited JSON docs.
//
// The remote TTS worker pulls jobs from the private worker listener; this
// service never synthesizes or runs ffmpeg in the web process — the endpoints
// that need a local engine answer ErrStorageUnsupported (501) instead of
// pretending. Script composition ("soạn bản đọc") is a model call, not a
// local engine: the gateway passes a Composer and the result is persisted in
// the same script/segment tables as the file backend.
//
// Object is a playable audio object handed to the HTTP layer: Path for the
// file backend, Data for small in-memory payloads, or an immutable blob
// reference (Store+Key) for the Postgres backend.
type Object struct {
	Path        string
	Data        []byte
	Store       blobstore.BlobStore
	Key         string
	Size        int64
	SHA256      string
	ContentType string
	Name        string
	ETag        string
}

// PGDeps wires the Postgres audio backend.
type PGDeps struct {
	Jobs     *postgres.Jobs
	Audio    *postgres.Audio
	Cache    *postgres.ExportCache
	Nar      *postgres.Narrative
	Blobs    blobstore.BlobStore
	Project  string
	Story    string
	Engine   string // engine identity frozen into synth keys (default "remote")
	Model    string // model identity frozen into synth keys (default "remote")
	WorkerID pgtype.UUID
	// Keys builds BlobStore object keys for the process STORAGE_ENV. The zero
	// value is the dev environment; hosted wiring passes blobkey.FromEnv().
	Keys blobkey.Keys
	// RemoteVoices returns the real catalogue advertised by the connected TTS
	// worker. Hosted has no local engine, so without this the picker only sees
	// uploaded project samples and a cast cannot reference an engine preset.
	RemoteVoices func(context.Context) []ProjectVoice
}

// PGService is the hosted sibling of Service: same HTTP surface, durable rows.
type PGService struct {
	deps PGDeps
	hub  *Hub

	mu         sync.Mutex
	scope      postgres.ScopeRef
	scoped     bool
	scopeErr   error
	basePath   string
	storyTitle string
	storyShort string

	// compMu guards the in-flight compose set: the same chapter must not be
	// composed twice concurrently (a second request would overwrite the first
	// result and burn a model call). compWG lets Close wait briefly.
	compMu    sync.Mutex
	composing map[chapterKey]bool
	compWG    sync.WaitGroup

	// prepWake notifies the hosted preparation runner that a needsScript tts
	// row is waiting, so a queued composition starts without polling. nil when
	// no runner is installed (tests, export runner).
	prepWake func()
}

// NewPGService builds the Postgres audio service for one story.
func NewPGService(deps PGDeps) *PGService {
	return &PGService{deps: deps, hub: NewHub(), composing: map[chapterKey]bool{}}
}

// SetPreparationWake installs the hosted runner hook. The callback must be
// non-blocking (a buffered-channel send); it is invoked when a composition row
// is created or resumed.
func (s *PGService) SetPreparationWake(fn func()) {
	s.mu.Lock()
	s.prepWake = fn
	s.mu.Unlock()
}

func (s *PGService) wakePreparation() {
	s.mu.Lock()
	fn := s.prepWake
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ── plumbing ─────────────────────────────────────────────────────────────────

// Hub exposes the event hub for SSE.
func (s *PGService) Hub() *Hub { return s.hub }

func (s *PGService) resolve(ctx context.Context) (postgres.ScopeRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scoped {
		return s.scope, s.scopeErr
	}
	s.scope, s.scopeErr = s.deps.Nar.Resolve(ctx, s.deps.Project, s.deps.Story, nil, false)
	s.scoped = true
	return s.scope, s.scopeErr
}

// pgErr maps storage errors onto the domain errors the HTTP layer understands.
func pgErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, postgres.ErrJobNotFound), errors.Is(err, postgres.ErrAudioSettingsMissing):
		return ErrNotFound
	case errors.Is(err, postgres.ErrJobState):
		return conflict("%v", err)
	case errors.Is(err, errs.ErrStorageNotFound):
		return ErrNotFound
	case errors.Is(err, errs.ErrStorageForbidden):
		return err
	default:
		return err
	}
}

// pgUnsupported is an explicit capability error (handler maps to 501).
func pgUnsupported(feature string) error {
	return fmt.Errorf("%s chưa hỗ trợ trên kho dữ liệu Postgres: %w", feature, errs.ErrStorageUnsupported)
}

// Close closes the event hub after giving an in-flight compose a short window
// (a provider call for a whole chapter can outlive a gateway shutdown; the
// process exit aborts it, exactly like the file backend's independent ctx).
func (s *PGService) Close() {
	done := make(chan struct{})
	go func() {
		s.compWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	s.hub.Close()
}

// SetBasePath sets the API prefix used to build segment URLs.
func (s *PGService) SetBasePath(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.basePath = p
}

// SetStoryNames injects the canonical title for export naming.
func (s *PGService) SetStoryNames(title, short string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storyTitle, s.storyShort = title, short
}

func (s *PGService) base() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.basePath
}

func (s *PGService) segmentURL(volume, chapter int, segID string) string {
	return fmt.Sprintf("%s/audio/segments/%d-%s/audio?volume=%d", s.base(), chapter, segID, volume)
}

// ── settings document (settings + flags + pronunciations + export prefs) ────

type pgDoc struct {
	Settings       Settings        `json:"settings"`
	Flags          []Flag          `json:"flags"`
	Pronunciations []Pronunciation `json:"pronunciations"`
	ExportPrefs    ExportPrefs     `json:"exportPrefs"`
}

func (s *PGService) doc(ctx context.Context) (pgDoc, int64, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgDoc{}, 0, pgErr(err)
	}
	return s.docFor(ctx, scope)
}

// docFor reads the settings document for an explicit scope (the export
// runner works on the job's story, not the cached service scope).
func (s *PGService) docFor(ctx context.Context, scope postgres.ScopeRef) (pgDoc, int64, error) {
	raw, ver, err := s.deps.Audio.Settings(ctx, scope)
	if err != nil {
		if errors.Is(err, postgres.ErrAudioSettingsMissing) {
			return pgDoc{Settings: DefaultSettings(), Flags: []Flag{}, Pronunciations: []Pronunciation{}}, 0, nil
		}
		return pgDoc{}, 0, pgErr(err)
	}
	var d pgDoc
	if err := json.Unmarshal(raw, &d); err != nil {
		return pgDoc{}, 0, fmt.Errorf("audio settings document hỏng: %w", err)
	}
	if !ValidateQuality(d.Settings.Quality) {
		d.Settings.Quality = DefaultSettings().Quality
	}
	if d.Flags == nil {
		d.Flags = []Flag{}
	}
	return d, ver, nil
}

func (s *PGService) saveDoc(ctx context.Context, d pgDoc) error {
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	_, err = s.deps.Audio.SaveSettings(ctx, scope, d)
	return pgErr(err)
}

// Settings returns the current generation settings.
func (s *PGService) Settings() Settings {
	d, _, err := s.doc(context.Background())
	if err != nil {
		return DefaultSettings()
	}
	return d.Settings
}

// SaveSettings validates and persists settings, keeping flags and the other
// document sections untouched.
func (s *PGService) SaveSettings(in Settings) (Settings, error) {
	if !ValidateQuality(in.Quality) {
		return in, conflict("quality phải là fast, standard hoặc high")
	}
	ctx := context.Background()
	d, _, err := s.doc(ctx)
	if err != nil {
		return in, err
	}
	d.Settings = in
	if err := s.saveDoc(ctx, d); err != nil {
		return in, err
	}
	return in, nil
}

// SettingsVersion is the ETag token for the settings document: it tracks the
// audio_settings row's optimistic version, so any save (settings, flags,
// pronunciations, export prefs) moves the token.
func (s *PGService) SettingsVersion() string {
	_, ver, err := s.doc(context.Background())
	if err != nil {
		return "pg-err"
	}
	return "pgv-" + strconv.FormatInt(ver, 10)
}

// Flags returns the listening-review marks.
func (s *PGService) Flags() ([]Flag, error) {
	d, _, err := s.doc(context.Background())
	if err != nil {
		return nil, err
	}
	return d.Flags, nil
}

// AddFlag appends one mark (same segment+reason dedupes, same as the file backend).
func (s *PGService) AddFlag(ctx context.Context, f Flag) (Flag, error) {
	if f.Chapter <= 0 {
		return Flag{}, conflict("thiếu số chương")
	}
	f.Reason = strings.TrimSpace(f.Reason)
	if f.Reason == "" {
		return Flag{}, conflict("thiếu lý do cần nghe lại")
	}
	if f.AtSec < 0 {
		f.AtSec = 0
	}
	d, _, err := s.doc(ctx)
	if err != nil {
		return Flag{}, err
	}
	for _, old := range d.Flags {
		if old.Chapter == f.Chapter && old.SegmentID == f.SegmentID && strings.EqualFold(old.Reason, f.Reason) {
			return old, nil
		}
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	if f.ID == "" {
		f.ID = fmt.Sprintf("f%d", f.CreatedAt.UnixNano())
	}
	d.Flags = append(d.Flags, f)
	if err := s.saveDoc(ctx, d); err != nil {
		return Flag{}, err
	}
	return f, nil
}

// DeleteFlag removes one mark.
func (s *PGService) DeleteFlag(ctx context.Context, id string) error {
	d, _, err := s.doc(ctx)
	if err != nil {
		return err
	}
	out := make([]Flag, 0, len(d.Flags))
	found := false
	for _, f := range d.Flags {
		if f.ID == id {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return ErrNotFound
	}
	d.Flags = out
	return s.saveDoc(ctx, d)
}

// ExportPrefs returns the export preferences.
func (s *PGService) ExportPrefs() (ExportPrefs, error) {
	d, _, err := s.doc(context.Background())
	if err != nil {
		return ExportPrefs{}, err
	}
	return d.ExportPrefs, nil
}

// SaveExportPrefs persists the export preferences.
func (s *PGService) SaveExportPrefs(p ExportPrefs) error {
	ctx := context.Background()
	d, _, err := s.doc(ctx)
	if err != nil {
		return err
	}
	d.ExportPrefs = p
	return s.saveDoc(ctx, d)
}
