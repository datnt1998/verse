package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5"
)

type pgPlaybackInput struct {
	Playback  bool   `json:"playback"`
	Volume    int    `json:"volume"`
	Chapter   int    `json:"chapter"`
	ChapterID string `json:"chapterId"`
	Revision  string `json:"revision"`
}

type pgPlaybackManifest struct {
	Revision string `json:"playbackRevision"`
	Volume   int    `json:"volume"`
	Chapter  int    `json:"chapter"`
}

// processPlaybackJob uses the export runner's lease, immutable blob publisher,
// scratch materialization and the same chapter assembler used by exports. It
// never synthesizes, runs in a GET, or requires a volume export first.
func (s *PGService) processPlaybackJob(ctx, completionCtx context.Context, lease postgres.WorkerLease, job postgres.Job, in pgPlaybackInput, stopHeartbeat func() error) error {
	chapterID, err := postgres.ParseUUID(in.ChapterID)
	if err != nil || in.Volume <= 0 || in.Chapter <= 0 || in.Revision == "" {
		return fmt.Errorf("invalid chapter playback job")
	}
	scope := postgres.ScopeRef{ProjectID: job.ProjectID, StoryID: job.StoryID}
	revision, err := s.deps.Audio.PlaybackRevision(ctx, scope, chapterID)
	if err != nil {
		return err
	}
	finish := func(attach func(context.Context, pgx.Tx, postgres.Job) error) error {
		if err := stopHeartbeat(); err != nil {
			return err
		}
		_, err := s.deps.Jobs.CompleteWith(completionCtx, lease, map[string]any{"playback": true}, 0, attach)
		return err
	}
	if revision.Content != in.Revision || revision.SourceRevision != revision.CurrentRevision {
		return finish(nil)
	}
	// A repeated ACK/restart already published this exact immutable chapter.
	if cached, err := s.deps.Cache.Get(ctx, scope, ChapterAudioCacheKey(in.Volume, in.Chapter), "mp3"); err == nil {
		var manifest pgPlaybackManifest
		if json.Unmarshal(cached.InputManifest, &manifest) == nil && manifest.Revision == in.Revision {
			return finish(nil)
		}
	}
	export := pgExportInput{Volume: in.Volume, Chapters: []int{in.Chapter},
		Source: []SourceChapter{{Chapter: in.Chapter}}, Options: ExportOptions{Format: "mp3"}, Engine: "remote", Model: "remote"}
	scratch, err := s.materializeExport(ctx, scope, export)
	if err != nil {
		return err
	}
	defer scratch.cleanup()
	fileSvc, err := NewService(scratch.layout, NullSynth{Reason: "hosted"})
	if err != nil {
		return err
	}
	defer fileSvc.Close()
	path, err := fileSvc.ensureChapterAudio(ctx, in.Volume, in.Chapter, nil)
	if err != nil {
		return err
	}
	upload, err := s.uploadExportFile(ctx, scope, job.ID, in.Volume, in.Chapter, filepath.Dir(path), ExportFile{Name: filepath.Base(path), Path: filepath.Base(path), Chapter: in.Chapter})
	if err != nil {
		return err
	}
	manifest, err := json.Marshal(pgPlaybackManifest{Revision: in.Revision, Volume: in.Volume, Chapter: in.Chapter})
	if err != nil {
		return err
	}
	return finish(func(ctx context.Context, tx pgx.Tx, _ postgres.Job) error {
		current, err := s.deps.Audio.PlaybackRevisionTx(ctx, tx, scope, chapterID)
		if err != nil {
			return err
		}
		// CAS against every input and the newest render attempt, not just the
		// export worker fence. Superseded work completes without publication.
		if current.Content != in.Revision {
			return nil
		}
		return s.deps.Audio.PutExportCacheTx(ctx, tx, scope, postgres.ExportCacheEntry{
			CacheKey: ChapterAudioCacheKey(in.Volume, in.Chapter), Kind: "mp3", BlobID: upload.blob.ID, InputManifest: manifest,
		}, nil)
	})
}
