package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

var pgSynthKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReadyView on hosted answers "can this story produce/play audio" without a
// local engine: ffmpeg handles chapter merge for playback; synthesis depends
// on at least one live 'tts' worker credential having heartbeated recently.
func (s *PGService) Ready(ctx context.Context) ReadyView {
	out := ReadyView{FFmpeg: FFmpegAvailable()}
	workers, err := s.deps.Jobs.ListWorkers(ctx)
	seen := ""
	online := false
	if err == nil {
		now := time.Now()
		for _, w := range workers {
			if w.RevokedAt != nil {
				continue
			}
			capable := false
			for _, c := range w.Capabilities {
				if c == "tts" {
					capable = true
				}
			}
			if !capable {
				continue
			}
			if w.LastSeenAt != nil {
				seen = w.LastSeenAt.Format("15:04")
				if now.Sub(*w.LastSeenAt) < postgres.DefaultLease+2*postgres.DefaultHeartbeat {
					online = true
				}
			}
		}
	}
	out.Engine = Status{Ready: online, Engine: "remote", State: "remote"}
	if online {
		out.Engine.Note = "Máy đọc từ xa đang kết nối"
	} else if seen != "" {
		out.Engine.Note = "Máy đọc từ xa lần cuối kết nối lúc " + seen
	} else {
		out.Engine.Note = "Chưa có máy đọc từ xa nào kết nối"
	}
	out.Ready = out.FFmpeg && online
	switch {
	case !out.FFmpeg && !online:
		out.Message = "Thiếu ffmpeg trên máy chủ và chưa có máy đọc từ xa nào kết nối."
	case !out.FFmpeg:
		out.Message = "Máy chủ chưa có ffmpeg — chưa ghép được chương hoặc xuất tập."
	case !online:
		out.Message = "Chưa có máy đọc từ xa nào kết nối — khởi động worker trên máy có VieNeu."
	default:
		out.Message = "Sẵn sàng tạo giọng."
	}
	return out
}

// Peaks reuses verified source bytes and a versioned, content-addressed waveform.
// A warm result needs neither a storage GET nor an ffmpeg decode.
func (s *PGService) Peaks(ctx context.Context, volume, chapter int) ([]float64, float64, error) {
	obj, err := s.OpenChapterAudio(ctx, volume, chapter)
	if err != nil {
		return nil, 0, err
	}
	if obj.Store == nil {
		return nil, 0, ErrNotFound
	}
	cache, err := configuredBlobCache()
	if err != nil {
		return nil, 0, err
	}
	return cache.peaks(ctx, obj, 500*time.Millisecond, toPCM)
}

// objectFor wraps a blob registry row as a playable Object.
func (s *PGService) objectFor(blob postgres.Blob, name string) Object {
	return Object{
		Store: s.deps.Blobs, Key: blob.ObjectKey, Size: blob.SizeBytes,
		SHA256: blob.SHA256, ContentType: blob.ContentType, Name: name,
	}
}

// blobObject resolves a blob row to an Object, rejecting non-ready uploads.
func (s *PGService) blobObject(ctx context.Context, blob postgres.Blob, name string) (Object, error) {
	if blob.State != "ready" {
		return Object{}, ErrNotFound
	}
	return s.objectFor(blob, name), nil
}

// OpenSegmentAudio resolves the selected render of one segment to its blob.
func (s *PGService) OpenSegmentAudio(ctx context.Context, volume, chapter int, segID string) (Object, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return Object{}, pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return Object{}, err
	}
	row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
	if err != nil {
		if errors.Is(err, postgres.ErrJobNotFound) {
			return Object{}, ErrNotFound
		}
		return Object{}, pgErr(err)
	}
	render, err := s.deps.Audio.SelectedRender(ctx, scope.ProjectID, ref.ID, row.RevisionNo, segID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, render.BlobID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	return s.blobObject(ctx, blob, fmt.Sprintf("%02d-%s.wav", chapter, segID))
}

// OpenCandidate streams one render candidate addressed as
// "<chapter>-<segment>-<synthkey>", the id ScriptView never exposes because
// hosted has no per-segment re-render endpoint — retained for the accept
// flow the worker itself publishes under.
func (s *PGService) OpenCandidate(ctx context.Context, id string) (Object, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return Object{}, pgErr(err)
	}
	chapter, segID, key, err := pgCandidateRef(id)
	if err != nil {
		return Object{}, ErrNotFound
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return Object{}, err
	}
	row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	renders, err := s.deps.Audio.Renders(ctx, scope.ProjectID, ref.ID, row.RevisionNo)
	if err != nil {
		return Object{}, pgErr(err)
	}
	for _, r := range renders {
		if r.SegmentID == segID && r.SynthKey == key {
			blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, r.BlobID)
			if err != nil {
				return Object{}, pgErr(err)
			}
			return s.blobObject(ctx, blob, segID+".wav")
		}
	}
	return Object{}, ErrNotFound
}

// pgCandidateRef parses "<chapter>-<segment>-<64-hex>".
func pgCandidateRef(id string) (chapter int, segID, key string, err error) {
	parts := strings.Split(id, "-")
	if len(parts) != 3 {
		return 0, "", "", ErrNotFound
	}
	chapter, err = strconv.Atoi(parts[0])
	if err != nil || chapter <= 0 {
		return 0, "", "", ErrNotFound
	}
	segID = parts[1]
	key = strings.ToLower(parts[2])
	if !pgSynthKeyRe.MatchString(key) || segID == "" {
		return 0, "", "", ErrNotFound
	}
	return chapter, segID, key, nil
}

// ChapterAudioCacheKey is the export_cache key of one assembled chapter file.
// The writer (export runner) and the reader (play endpoint) must agree, so the
// format lives in exactly one place.
func ChapterAudioCacheKey(volume, chapter int) string {
	return fmt.Sprintf("chapter-audio:v%d:c%d", volume, chapter)
}

// OpenChapterAudio only resolves a published chapter. Assembly belongs to the
// durable export runner; an obsolete revision is never served as current audio.
func (s *PGService) OpenChapterAudio(ctx context.Context, volume, chapter int) (Object, error) {
	scope, err := s.resolve(ctx)
	if err != nil {
		return Object{}, pgErr(err)
	}
	entry, err := s.deps.Cache.Get(ctx, scope, ChapterAudioCacheKey(volume, chapter), "mp3")
	if err != nil {
		return Object{}, pgErr(err)
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return Object{}, err
	}
	revision, err := s.deps.Audio.PlaybackRevision(ctx, scope, ref.ID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	var manifest pgPlaybackManifest
	if err := json.Unmarshal(entry.InputManifest, &manifest); err != nil {
		return Object{}, err
	}
	if manifest.Revision != revision.Content || revision.SourceRevision != revision.CurrentRevision {
		return Object{}, ErrNotFound
	}
	blob, err := s.deps.Audio.BlobByID(ctx, scope.ProjectID, entry.BlobID)
	if err != nil {
		return Object{}, pgErr(err)
	}
	return s.blobObject(ctx, blob, fmt.Sprintf("%02d.mp3", chapter))
}

// Preview synthesizes on demand; there is no local engine on hosted.
func (s *PGService) Preview(ctx context.Context, volume, chapter int, segID string) (PreviewResult, error) {
	return PreviewResult{}, pgUnsupported("nghe thử đoạn chưa có giọng")
}

// Rerender synthesizes a candidate; there is no local engine on hosted —
// queuing the chapter with the wanted cast produces the same candidates.
func (s *PGService) Rerender(ctx context.Context, volume, chapter int, segID string, opts RerenderOptions) (Candidate, error) {
	return Candidate{}, pgUnsupported("đọc lại từng đoạn")
}

// AcceptCandidate makes one render the selected take of its segment. The
// candidate id is "<chapter>-<segment>-<synthkey>".
func (s *PGService) AcceptCandidate(ctx context.Context, volume, chapter int, segID, candidateID string) error {
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	refCh, seg, key, err := pgCandidateRef(candidateID)
	if err != nil {
		return err
	}
	if refCh != chapter || seg != segID {
		return conflict("Bản đọc lại này không thuộc đoạn đang sửa.")
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return err
	}
	row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
	if err != nil {
		return pgErr(err)
	}
	return pgErr(s.deps.Audio.SelectPlaybackRender(ctx, scope, ref.ID, row.RevisionNo, segID, key, volume, chapter))
}

// RejectCandidate drops one candidate render (the current take is untouched).
func (s *PGService) RejectCandidate(ctx context.Context, candidateID string) error {
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	chapter, segID, key, err := pgCandidateRef(candidateID)
	if err != nil {
		return err
	}
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return err
	}
	row, err := s.deps.Audio.LatestScript(ctx, scope.ProjectID, ref.ID)
	if err != nil {
		return pgErr(err)
	}
	return pgErr(s.deps.Audio.DeleteRender(ctx, scope.ProjectID, ref.ID, row.RevisionNo, segID, key))
}
