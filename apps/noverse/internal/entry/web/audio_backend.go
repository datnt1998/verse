package web

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/platform/blobstore"
)

// audioBackend is the per-story audio surface the HTTP handlers need. The file
// backend is *audio.Service; the Postgres backend is *audio.PGService, which
// keeps durable state in the phase-07 tables and binary objects in BlobStore.
// Endpoints a backend cannot serve answer ErrStorageUnsupported (→501) —
// never a silent no-op.
type audioBackend interface {
	Hub() *audio.Hub
	Close()
	SetBasePath(string)
	SetStoryNames(title, short string)

	Ready(ctx context.Context) audio.ReadyView

	Settings() audio.Settings
	SaveSettings(in audio.Settings) (audio.Settings, error)
	SettingsVersion() string

	Flags() ([]audio.Flag, error)
	AddFlag(ctx context.Context, f audio.Flag) (audio.Flag, error)
	DeleteFlag(ctx context.Context, id string) error

	ExportPrefs() (audio.ExportPrefs, error)
	SaveExportPrefs(p audio.ExportPrefs) error

	Cast(ref audio.CharacterRef) (*audio.Cast, error)
	SaveCast(c *audio.Cast) (*audio.Cast, error)
	CastVersion() string
	CastOldSegments(c *audio.Cast) error
	ApplyPackVoice(c *audio.Cast) error
	ChangeRoleVoice(ctx context.Context, role, voiceID, sameAs string, rerenderOld bool) (*audio.Cast, int, error)
	Voices(ctx context.Context) ([]audio.ProjectVoice, error)
	VoiceSample(ctx context.Context, id string) (audio.Object, error)

	Volumes(ctx context.Context, source []audio.SourceChapter, volume int) (audio.VolumeView, error)
	Timeline(ctx context.Context, source []audio.SourceChapter, volume int) (audio.QueueView, error)
	Queue() []audio.Job
	QueueChapters(ctx context.Context, source []audio.SourceChapter, volume int, in audio.QueueInput) (audio.QueueResult, error)
	Enqueue(ctx context.Context, volume, chapter int, currentHash string) (string, error)
	Pause(jobID string) error
	Resume(jobID string) error
	Cancel(jobID string) error
	Retry(ctx context.Context, jobID, quality string) error
	Prioritize(ctx context.Context, volume, chapter int) error

	Script(ctx context.Context, volume, chapter int) (*audio.Script, error)
	ScriptView(ctx context.Context, volume, chapter int, currentHash string) (*audio.ScriptView, error)
	ScriptVersion(volume, chapter int) string
	// SaveScript replaces the chapter script. expectedVersion is the ETag
	// token the caller read; the Postgres backend carries it into the write
	// transaction as a CAS predicate (the file backend keeps its atomic
	// whole-file replacement).
	SaveScript(ctx context.Context, volume, chapter int, expectedVersion string, edit audio.ScriptEdit) (*audio.Script, error)
	Lock(ctx context.Context, volume, chapter int, currentHash string) (*audio.Script, error)
	Unlock(ctx context.Context, volume, chapter int) (*audio.Script, error)
	Compose(source []audio.SourceChapter, volume int, chapters []int, composer audio.Composer, ref audio.CharacterRef) error
	Refresh(source []audio.SourceChapter, volume, chapter int, composer audio.Composer, ref audio.CharacterRef) error

	Preview(ctx context.Context, volume, chapter int, segID string) (audio.PreviewResult, error)
	Rerender(ctx context.Context, volume, chapter int, segID string, opts audio.RerenderOptions) (audio.Candidate, error)
	AcceptCandidate(ctx context.Context, volume, chapter int, segID, candidateID string) error
	RejectCandidate(ctx context.Context, candidateID string) error

	// Playback returns an audio.Object: a file path or in-memory bytes for the
	// file backend, an immutable blob reference for the postgres backend.
	OpenSegmentAudio(ctx context.Context, volume, chapter int, segID string) (audio.Object, error)
	OpenChapterAudio(ctx context.Context, volume, chapter int) (audio.Object, error)
	OpenCandidate(ctx context.Context, id string) (audio.Object, error)
	Peaks(ctx context.Context, volume, chapter int) ([]float64, float64, error)

	StartExport(ctx context.Context, volume int, bookTitle string, source []audio.SourceChapter, opts audio.ExportOptions) error
	ExportInfo(ctx context.Context, volume int) (audio.ExportInfo, error)
}

// serveAudioObject streams a resolved audio.Object. Blob refs stream through
// BlobStore with the same single-range contract as the blob delivery
// endpoint; path refs use ServeContent; byte refs serve an in-memory object.
func serveAudioObject(w http.ResponseWriter, r *http.Request, obj audio.Object) {
	switch {
	case len(obj.Data) > 0:
		w.Header().Set("Content-Type", orDefault(obj.ContentType, "application/octet-stream"))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(w, r, orDefault(obj.Name, "audio"), time.Time{}, bytes.NewReader(obj.Data))
	case obj.Store == nil:
		serveAudioFile(w, r, obj.Path)
	default:
		serveStoredBlob(w, r, obj.Store, blobstore.BlobInfo{Key: obj.Key, Size: obj.Size, ContentType: obj.ContentType, SHA256: obj.SHA256}, "private, max-age=60", "inline", orDefault(obj.Name, fileNameOf(obj.Key, r)), 0)
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ── file backend adapter ─────────────────────────────────────────────────────

// fileAudioBackend adapts *audio.Service to audioBackend with zero behaviour
// change: every call is a straight delegate.
type fileAudioBackend struct{ svc *audio.Service }

func (f *fileAudioBackend) Hub() *audio.Hub           { return f.svc.Hub() }
func (f *fileAudioBackend) Close()                    { f.svc.Close() }
func (f *fileAudioBackend) SetBasePath(p string)      { f.svc.SetBasePath(p) }
func (f *fileAudioBackend) SetStoryNames(t, s string) { f.svc.SetStoryNames(t, s) }
func (f *fileAudioBackend) Layout() audio.Layout      { return f.svc.Layout() }

func (f *fileAudioBackend) Ready(ctx context.Context) audio.ReadyView { return f.svc.Ready(ctx) }

func (f *fileAudioBackend) Settings() audio.Settings { return f.svc.Settings() }
func (f *fileAudioBackend) SaveSettings(in audio.Settings) (audio.Settings, error) {
	return f.svc.SaveSettings(in)
}
func (f *fileAudioBackend) SettingsVersion() string {
	return fileVersion(f.svc.Layout().SettingsPath())
}

func (f *fileAudioBackend) Flags() ([]audio.Flag, error) { return audio.ReadFlags(f.svc.Layout()) }
func (f *fileAudioBackend) AddFlag(_ context.Context, in audio.Flag) (audio.Flag, error) {
	return audio.AddFlag(f.svc.Layout(), in)
}
func (f *fileAudioBackend) DeleteFlag(_ context.Context, id string) error {
	return audio.DeleteFlag(f.svc.Layout(), id)
}

func (f *fileAudioBackend) ExportPrefs() (audio.ExportPrefs, error) {
	return audio.ReadExportPrefs(f.svc.Layout())
}
func (f *fileAudioBackend) SaveExportPrefs(p audio.ExportPrefs) error {
	return audio.WriteExportPrefs(f.svc.Layout(), p)
}

func (f *fileAudioBackend) Cast(ref audio.CharacterRef) (*audio.Cast, error) {
	return f.svc.Cast(ref)
}
func (f *fileAudioBackend) SaveCast(c *audio.Cast) (*audio.Cast, error) { return f.svc.SaveCast(c) }
func (f *fileAudioBackend) CastVersion() string                         { return fileVersion(f.svc.Layout().CastPath()) }
func (f *fileAudioBackend) CastOldSegments(c *audio.Cast) error         { return f.svc.CastOldSegments(c) }
func (f *fileAudioBackend) ApplyPackVoice(c *audio.Cast) error          { return f.svc.ApplyPackVoice(c) }
func (f *fileAudioBackend) ChangeRoleVoice(_ context.Context, role, voiceID, sameAs string, rerenderOld bool) (*audio.Cast, int, error) {
	return f.svc.ChangeRoleVoice(role, voiceID, sameAs, rerenderOld)
}
func (f *fileAudioBackend) Voices(ctx context.Context) ([]audio.ProjectVoice, error) {
	return f.svc.Voices(ctx)
}
func (f *fileAudioBackend) VoiceSample(ctx context.Context, id string) (audio.Object, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	wav, err := audio.VoiceSample(cctx, f.svc.Layout(), audioSynthesizer(), id)
	if err != nil {
		return audio.Object{}, err
	}
	return audio.Object{Data: wav, ContentType: "audio/wav", Name: "mau-giong.wav"}, nil
}

func (f *fileAudioBackend) Volumes(ctx context.Context, source []audio.SourceChapter, volume int) (audio.VolumeView, error) {
	return f.svc.Volumes(ctx, source, volume)
}
func (f *fileAudioBackend) Timeline(_ context.Context, source []audio.SourceChapter, volume int) (audio.QueueView, error) {
	return f.svc.Timeline(source, volume)
}
func (f *fileAudioBackend) Queue() []audio.Job { return f.svc.Queue() }
func (f *fileAudioBackend) QueueChapters(_ context.Context, source []audio.SourceChapter, volume int, in audio.QueueInput) (audio.QueueResult, error) {
	return f.svc.QueueChapters(source, volume, in)
}
func (f *fileAudioBackend) Enqueue(_ context.Context, volume, chapter int, currentHash string) (string, error) {
	return f.svc.Enqueue(volume, chapter, currentHash)
}
func (f *fileAudioBackend) Pause(jobID string) error  { return f.svc.Pause(jobID) }
func (f *fileAudioBackend) Resume(jobID string) error { return f.svc.Resume(jobID) }
func (f *fileAudioBackend) Cancel(jobID string) error { return f.svc.Cancel(jobID) }
func (f *fileAudioBackend) Retry(_ context.Context, jobID, quality string) error {
	return f.svc.Retry(jobID, quality)
}
func (f *fileAudioBackend) Prioritize(_ context.Context, volume, chapter int) error {
	return f.svc.Prioritize(volume, chapter)
}

func (f *fileAudioBackend) Script(_ context.Context, volume, chapter int) (*audio.Script, error) {
	return f.svc.Script(volume, chapter)
}
func (f *fileAudioBackend) ScriptView(ctx context.Context, volume, chapter int, currentHash string) (*audio.ScriptView, error) {
	return f.svc.ScriptView(ctx, volume, chapter, currentHash)
}
func (f *fileAudioBackend) ScriptVersion(volume, chapter int) string {
	return fileVersion(f.svc.Layout().ScriptPath(volume, chapter))
}
func (f *fileAudioBackend) SaveScript(_ context.Context, volume, chapter int, _ string, edit audio.ScriptEdit) (*audio.Script, error) {
	return f.svc.SaveScript(volume, chapter, edit)
}
func (f *fileAudioBackend) Lock(ctx context.Context, volume, chapter int, currentHash string) (*audio.Script, error) {
	return f.svc.Lock(ctx, volume, chapter, currentHash)
}
func (f *fileAudioBackend) Unlock(_ context.Context, volume, chapter int) (*audio.Script, error) {
	return f.svc.Unlock(volume, chapter)
}
func (f *fileAudioBackend) Compose(source []audio.SourceChapter, volume int, chapters []int, composer audio.Composer, ref audio.CharacterRef) error {
	return f.svc.Compose(source, volume, chapters, composer, ref)
}
func (f *fileAudioBackend) Refresh(source []audio.SourceChapter, volume, chapter int, composer audio.Composer, ref audio.CharacterRef) error {
	return f.svc.Refresh(source, volume, chapter, composer, ref)
}

func (f *fileAudioBackend) Preview(ctx context.Context, volume, chapter int, segID string) (audio.PreviewResult, error) {
	return f.svc.Preview(ctx, volume, chapter, segID)
}
func (f *fileAudioBackend) Rerender(ctx context.Context, volume, chapter int, segID string, opts audio.RerenderOptions) (audio.Candidate, error) {
	return f.svc.Rerender(ctx, volume, chapter, segID, opts)
}
func (f *fileAudioBackend) AcceptCandidate(_ context.Context, volume, chapter int, segID, candidateID string) error {
	return f.svc.AcceptCandidate(volume, chapter, segID, candidateID)
}
func (f *fileAudioBackend) RejectCandidate(_ context.Context, candidateID string) error {
	return f.svc.RejectCandidate(candidateID)
}

func (f *fileAudioBackend) OpenSegmentAudio(ctx context.Context, volume, chapter int, segID string) (audio.Object, error) {
	path, err := f.svc.SegmentAudioPath(ctx, volume, chapter, segID)
	return audio.Object{Path: path}, err
}
func (f *fileAudioBackend) OpenChapterAudio(ctx context.Context, volume, chapter int) (audio.Object, error) {
	path, err := f.svc.ChapterAudioFile(ctx, volume, chapter)
	return audio.Object{Path: path}, err
}
func (f *fileAudioBackend) OpenCandidate(_ context.Context, id string) (audio.Object, error) {
	path, err := f.svc.CandidatePath(id)
	return audio.Object{Path: path}, err
}
func (f *fileAudioBackend) Peaks(ctx context.Context, volume, chapter int) ([]float64, float64, error) {
	return f.svc.Peaks(ctx, volume, chapter)
}

func (f *fileAudioBackend) StartExport(_ context.Context, volume int, bookTitle string, source []audio.SourceChapter, opts audio.ExportOptions) error {
	return f.svc.StartExport(volume, bookTitle, source, opts)
}
func (f *fileAudioBackend) ExportInfo(_ context.Context, volume int) (audio.ExportInfo, error) {
	return f.svc.ExportInfo(volume)
}

// errAudioUnsupportedPG is the explicit hosted capability error for endpoints
// the Postgres backend does not implement yet.
func errAudioUnsupportedPG(feature string) error {
	return &audioUnsupported{feature: feature}
}

type audioUnsupported struct{ feature string }

func (e *audioUnsupported) Error() string {
	return e.feature + " chưa hỗ trợ trên kho dữ liệu Postgres"
}
func (e *audioUnsupported) Unwrap() error { return errs.ErrStorageUnsupported }
