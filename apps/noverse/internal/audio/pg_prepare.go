package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Hosted preparation ("AI sẽ soạn bản đọc" — GenPickEmpty): a queued tts row
// with input.needsScript never reaches the remote worker. The web process owns
// the model runtime, so the runner (internal/entry/web) claims the row, resolves
// the story's composer, and calls PrepareClaimed. This file owns everything
// after the claim: chapter source + script CAS snapshot, the model call, the
// merge/lock, and the atomic publish that turns the SAME row into a render job.
//
// A provider call never runs inside a transaction. The script snapshot and its
// CAS version are read BEFORE the call; the write happens inside
// CompletePreparationWith's transaction, which locks the job row first. A
// pause/cancel/retry during the call therefore moves the fence and the publish
// fails with ErrJobLeaseLost — no script, no render claim.
//
// The same transaction also locks the chapter row and re-checks that the
// committed revision and its text are still the source this composition was
// built from (LockChapterSourceTx). The queue may have read the prose before
// the revision writer accepted a newer one, or the revision may advance during
// the provider call; either way the composed text is not the accepted source,
// so the publish is refused and the row stays a retryable preparation. A
// script/render input must never carry a revision number that names different
// text: when the pointer advanced during the call the native ACK drops every
// attachment, and when the prose was read before the pointer moved the ACK
// attaches audio synthesized from text that revision does not contain.

// PreparationInput is what the hosted runner resolves for one claimed
// composition: the story model, the chapter source the HTTP queue validated
// (same resolution as the request path), and the character profiles used for
// speaker detection. The runner owns provider configuration; the audio service
// never fabricates one.
type PreparationInput struct {
	Composer Composer
	Ref      CharacterRef
	Source   SourceChapter
}

// PreparationCoords reports the volume/chapter carried by a claimed
// preparation row (ok=false for any other tts row).
func PreparationCoords(job postgres.Job) (volume, chapter int, ok bool) {
	var in pgTTSInput
	if json.Unmarshal(job.Input, &in) != nil || !in.NeedsScript {
		return 0, 0, false
	}
	return in.Volume, in.Chapter, in.Volume > 0 && in.Chapter > 0
}

// ChapterSource picks one chapter out of a resolved volume source list.
func ChapterSource(source []SourceChapter, chapter int) (SourceChapter, bool) {
	return findSource(source, chapter)
}

// preparationSourceMatches reports whether the row-locked committed revision is
// the exact source a composition was built from. The runner resolves the
// chapter prose before PrepareClaimed snapshots the revision pointer, so the
// prose may already belong to a newer revision; and the pointer may move while
// the provider is composing. Both the pointer and the accepted text therefore
// have to agree before anything is published:
//
//   - revision: the script/render input stamps source_revision, and the native
//     ACK only attaches while it equals the chapter's current revision.
//   - text: the same number can name different prose when the queue read the
//     chapter earlier, so a matching revision alone would lock old text. The
//     comparison uses the revision store's canonical digest
//     (domain.ChapterContentSHA256), which is exactly chapter_versions.sha256.
//
// ok=false is a missing chapter/revision row and is treated like a mismatch.
func preparationSourceMatches(ref postgres.ChapterRef, source postgres.ChapterSource, ok bool, composedSHA256 string) bool {
	if !ok || !ref.ID.Valid || source.ID != ref.ID {
		return false
	}
	if int(source.Revision) != ref.CurrentRevision {
		return false
	}
	return source.SHA256 == composedSHA256
}

// prepReleaseTimeout bounds the best-effort lease release after a graceful
// shutdown cancelled an in-flight composition.
const prepReleaseTimeout = 5 * time.Second

// PrepareClaimed composes and locks the bản đọc of one claimed needsScript job.
// It never returns ErrJobLeaseLost as an error: a pause/cancel during the model
// call is a normal outcome (the caller's job is no longer ours). Real failures
// are recorded on the row as retryable errors, keeping needsScript so the
// existing Retry path re-enters preparation.
func (s *PGService) PrepareClaimed(ctx context.Context, scope postgres.ScopeRef, job postgres.Job, in PreparationInput) error {
	own, err := s.resolve(ctx)
	if err != nil {
		return s.failPreparation(ctx, scope, job, pgErr(err))
	}
	if own.ProjectID != scope.ProjectID || own.StoryID != scope.StoryID {
		return s.failPreparation(ctx, scope, job, fmt.Errorf("việc soạn bản đọc thuộc truyện khác: %w", errs.ErrStorageNotFound))
	}
	var frozen pgTTSInput
	if err := json.Unmarshal(job.Input, &frozen); err != nil {
		return s.failPreparation(ctx, scope, job, fmt.Errorf("việc soạn bản đọc hỏng input: %w", err))
	}
	if !frozen.NeedsScript {
		return s.failPreparation(ctx, scope, job, errors.New("việc này không cần soạn bản đọc."))
	}
	volume, chapter := frozen.Volume, frozen.Chapter
	if volume <= 0 || chapter <= 0 {
		return s.failPreparation(ctx, scope, job, errors.New("việc soạn bản đọc thiếu số chương/tập."))
	}
	kind := frozen.Kind
	if kind == "" {
		kind = JobKindCreate
	}
	redo := kind == JobKindRedo

	// Snapshot the chapter source (already resolved by the runner) and the
	// script row before any provider call.
	ref, err := s.chapterRef(ctx, scope, chapter)
	if err != nil {
		return s.failPreparation(ctx, scope, job, err)
	}
	if !ref.Committed || !ref.ID.Valid {
		return s.failPreparation(ctx, scope, job,
			conflict("Chương %d chưa nghiệm thu hoặc chưa có chính văn để soạn bản đọc.", chapter))
	}
	// The queue validated this exact text (the same StoryReader resolution the
	// request path uses), so composition and the freshness hash agree with the
	// chapter the UI shows.
	if !in.Source.Completed || strings.TrimSpace(in.Source.Text) == "" || in.Source.Chapter != chapter {
		return s.failPreparation(ctx, scope, job,
			conflict("Chương %d chưa nghiệm thu hoặc chưa có chính văn để soạn bản đọc.", chapter))
	}
	title := in.Source.Title
	if title == "" {
		title = ref.Title
	}
	old, meta, err := s.loadScriptRef(ctx, scope, ref)
	if err != nil {
		return s.failPreparation(ctx, scope, job, err)
	}
	expected := int64(0)
	if meta != nil {
		expected = meta.Version
	}
	if in.Composer == nil {
		return s.failPreparation(ctx, scope, job, ErrNoModel)
	}
	d, _, err := s.docFor(ctx, scope)
	if err != nil {
		return s.failPreparation(ctx, scope, job, err)
	}
	cast, err := s.castForRender(ctx)
	if err != nil {
		return s.failPreparation(ctx, scope, job, err)
	}
	enriched := EnsureCast(cast, in.Ref.Characters, in.Ref.Support)
	voices, err := s.Voices(ctx)
	if err != nil {
		return s.failPreparation(ctx, scope, job, err)
	}
	voiceMap := VoiceMap(voices)
	settings := d.Settings
	// The queue froze the requested quality on the job; the render input must
	// keep it (the file backend applies job.Quality at render time).
	if frozen.Quality != "" {
		settings.Quality = frozen.Quality
	}

	s.publishCompose(chapter, "drafting", 0, nil)
	callCtx, cancel := context.WithTimeout(ctx, pgComposeChapterTimeout)
	defer cancel()
	res, cerr := in.Composer.Compose(callCtx, ComposeInput{
		Volume: volume, Chapter: chapter, Title: title, Text: in.Source.Text,
		Characters: composeSpeakers(&enriched, in.Ref), Mode: enriched.Mode,
		Normalize: settings.Normalize, Dict: d.Pronunciations,
	})
	if cerr != nil {
		if ctx.Err() != nil {
			// Graceful shutdown: hand the row back so the next runner pass (or
			// the restarted process) retries instead of recording a fake error.
			s.releasePreparation(scope, job)
			s.publishCompose(chapter, "error", 0, context.Canceled)
			return nil
		}
		return s.failPreparation(ctx, scope, job,
			fmt.Errorf("Không soạn được bản đọc chương %d: %w", chapter, cerr))
	}
	if len(res.Segments) == 0 {
		return s.failPreparation(ctx, scope, job,
			conflict("AI chưa trả về đoạn đọc nào cho chương %d. Hãy thử lại.", chapter))
	}

	now := time.Now()
	fresh := &Script{
		Chapter: chapter, Volume: volume, Status: StatusLocked, LockedAt: &now,
		SourceHash: SourceHash(in.Source.Text), ComposedAt: &now, Model: res.Model,
		Segments: res.Segments,
	}
	if old != nil {
		merged, _ := mergeScript(old, fresh)
		fresh = merged
		// A queue-time composition always ends locked, even when the previous
		// revision was still under review.
		fresh.Status = StatusLocked
		if old.LockedAt != nil {
			fresh.LockedAt = old.LockedAt
		} else {
			fresh.LockedAt = &now
		}
		fresh.pgRevision = old.pgRevision
	}
	applyVoiceFlags(fresh, &enriched)
	edited := changedComposeSegments(old, fresh)
	// The canonical digest of the text the provider actually saw. It must equal
	// the locked revision's sha256 at publish; SourceHash stays the audio
	// fingerprint stored on the script.
	composedSHA256 := domain.ChapterContentSHA256(in.Source.Text)

	var total int32
	_, err = s.deps.Jobs.CompletePreparationWith(ctx, scope, job.ID, job.FencingToken,
		func(txCtx context.Context, tx pgx.Tx, _ postgres.Job) (any, int32, error) {
			source, ok, serr := s.deps.Nar.LockChapterSourceTx(txCtx, tx, scope, chapter)
			if serr != nil {
				return nil, 0, pgErr(serr)
			}
			if !preparationSourceMatches(ref, source, ok, composedSHA256) {
				return nil, 0, conflict("Chương %d vừa thay đổi trong lúc soạn bản đọc — bản đọc cũ không được lưu, vui lòng thử lại.", chapter)
			}
			saved, serr := s.persistScriptTx(txCtx, tx, scope, ref, fresh, edited, expected)
			if serr != nil {
				return nil, 0, serr
			}
			// Ownership is the chapter's mutable-input fingerprint at publish
			// time; the render ACK compares it like every other hosted job.
			revision, rerr := s.deps.Audio.PlaybackRevisionTx(txCtx, tx, scope, ref.ID, PlaybackProof{})
			if rerr != nil {
				return nil, 0, pgErr(rerr)
			}
			segs := s.wantSegments(fresh, cast, voiceMap, settings, redo)
			total = int32(len(segs))
			return pgTTSInput{
				Kind: kind, Chapter: chapter, Volume: volume,
				ChapterID:      postgres.UUIDString(ref.ID),
				ScriptRevision: saved.RevisionNo,
				SourceRevision: int32(ref.CurrentRevision),
				Ownership:      revision.Ownership,
				Redo:           redo,
				Quality:        settings.Quality,
				Segments:       segs,
			}, total, nil
		})
	if err != nil {
		if errors.Is(err, postgres.ErrJobLeaseLost) {
			// Paused, cancelled or reclaimed while the model was running: the
			// script was never written and the render claim is gone.
			s.publishCompose(chapter, "error", 0, errPreparationInterrupted)
			return nil
		}
		if errors.Is(err, errs.ErrStorageConflict) {
			return s.failPreparation(ctx, scope, job,
				conflict("Bản đọc vừa được sửa trong lúc soạn — dữ liệu mới đã được giữ, vui lòng thử lại."))
		}
		return s.failPreparation(ctx, scope, job, err)
	}
	s.publishCompose(chapter, "done", int(total), nil)
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return nil
}

// errPreparationInterrupted is the terminal compose frame for a job that was
// paused/cancelled mid-call: nothing was stored, the row keeps its control
// state.
var errPreparationInterrupted = conflict("Việc soạn bản đọc đã dừng — không có bản đọc nào được lưu.")

// failPreparation records a retryable compose error on the row. A row whose
// lease moved (pause/cancel/retry) is not ours to fail and stays untouched.
func (s *PGService) failPreparation(ctx context.Context, scope postgres.ScopeRef, job postgres.Job, cause error) error {
	msg := preparationReason(cause)
	if _, err := s.deps.Jobs.FailPreparation(ctx, scope, job.ID, job.FencingToken,
		map[string]any{"message": msg, "error": cause.Error()}); err != nil {
		if errors.Is(err, postgres.ErrJobLeaseLost) {
			return nil
		}
		return pgErr(err)
	}
	var frozen pgTTSInput
	if err := json.Unmarshal(job.Input, &frozen); err == nil && frozen.Chapter > 0 {
		s.publishCompose(frozen.Chapter, "error", 0, cause)
	}
	s.hub.Publish(NewEvent(EventQueue, map[string]any{"jobs": s.Queue()}))
	return cause
}

// releasePreparation is the shutdown path: give the lease back so recovery can
// claim it immediately instead of waiting for expiry.
func (s *PGService) releasePreparation(scope postgres.ScopeRef, job postgres.Job) {
	ctx, cancel := context.WithTimeout(context.Background(), prepReleaseTimeout)
	defer cancel()
	// A moved lease means the row is no longer ours; either way the lease
	// lapses and startup recovery schedules it.
	_, _ = s.deps.Jobs.ReleasePreparation(ctx, scope, job.ID, job.FencingToken)
}

// preparationReason turns a compose failure into the Vietnamese reason the
// queue shows under the job (the same text as the file backend's errors).
func preparationReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrNoModel) {
		return ErrNoModel.Error()
	}
	var state *StateError
	if errors.As(err, &state) {
		return state.Msg
	}
	return err.Error()
}
