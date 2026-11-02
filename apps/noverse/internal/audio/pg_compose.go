package audio

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// Hosted "soạn bản đọc": the web process owns the model runtime for hosted
// stories (the same `bootstrap.NewModelSet` path the ledger uses), so the
// gateway passes a real Composer in and this service persists the result with
// the same script/segment tables the file backend writes to files.
//
// Like the file backend, the model call runs on a background goroutine after
// synchronous validation, so the HTTP request returns at once and progress
// travels over `audio.compose`. A provider call is capped by ChatComposer
// itself (two minutes); the context here only releases the slot afterwards.

// pgComposeChapterTimeout bounds one chapter's model call plus persistence.
const pgComposeChapterTimeout = 2*time.Minute + 30*time.Second

func (s *PGService) isComposing(volume, chapter int) bool {
	s.compMu.Lock()
	defer s.compMu.Unlock()
	return s.composing[chapterKey{volume: volume, chapter: chapter}]
}

// claimComposing reserves the chapter before any work starts. The check and the
// set are one critical section, so two concurrent requests cannot both launch a
// provider call for the same chapter.
func (s *PGService) claimComposing(volume, chapter int) bool {
	s.compMu.Lock()
	defer s.compMu.Unlock()
	key := chapterKey{volume: volume, chapter: chapter}
	if s.composing[key] {
		return false
	}
	s.composing[key] = true
	return true
}

func (s *PGService) releaseComposing(volume int, chapters []int) {
	s.compMu.Lock()
	defer s.compMu.Unlock()
	for _, chapter := range chapters {
		delete(s.composing, chapterKey{volume: volume, chapter: chapter})
	}
}

// Compose starts composing the selected chapters from the chapter text. The
// synchronous part mirrors Service.Compose: missing model/chapter and an
// already-running compose are immediate errors; model calls happen in the
// background and report over audio.compose.
func (s *PGService) Compose(source []SourceChapter, volume int, chapters []int, composer Composer, ref CharacterRef) error {
	if composer == nil {
		return ErrNoModel
	}
	if len(chapters) == 0 {
		return conflict("Chưa chọn chương nào để soạn.")
	}
	byChapter := make(map[int]SourceChapter, len(source))
	for _, sc := range source {
		byChapter[sc.Chapter] = sc
	}
	for _, ch := range chapters {
		if _, ok := byChapter[ch]; !ok {
			return conflict("Không có chương %d trong tập này.", ch)
		}
	}
	list := make([]int, 0, len(chapters))
	for _, ch := range chapters {
		if s.claimComposing(volume, ch) {
			list = append(list, ch)
		}
	}
	if len(list) == 0 {
		return conflict("Các chương này đang được soạn rồi.")
	}
	s.compWG.Add(1)
	go func() {
		defer s.compWG.Done()
		defer s.releaseComposing(volume, list)
		s.composeChapters(volume, list, byChapter, composer, ref, false)
	}()
	return nil
}

// Refresh re-composes one chapter whose source changed, keeping the segments
// that did not change and their generated audio. A chapter that is not stale
// is skipped without a model call.
func (s *PGService) Refresh(source []SourceChapter, volume, chapter int, composer Composer, ref CharacterRef) error {
	if composer == nil {
		return ErrNoModel
	}
	sc, ok := findSource(source, chapter)
	if !ok {
		return conflict("Không có chương %d trong tập này.", chapter)
	}
	if !sc.Completed {
		return conflict("Chương %d chưa nghiệm thu — chỉ soạn bản đọc cho chương đã nghiệm thu.", chapter)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	scope, err := s.resolve(ctx)
	if err != nil {
		return pgErr(err)
	}
	script, _, err := s.loadScript(ctx, scope, chapter)
	if err != nil {
		return err
	}
	if script == nil {
		return conflict("Chương %d chưa có bản đọc — hãy soạn trước.", chapter)
	}
	if !script.needsComposition(SourceHash(sc.Text)) {
		return nil
	}
	if !s.claimComposing(volume, chapter) {
		return conflict("Chương này đang được soạn rồi.")
	}
	s.compWG.Add(1)
	go func() {
		defer s.compWG.Done()
		defer s.releaseComposing(volume, []int{chapter})
		s.composeChapters(volume, []int{chapter}, map[int]SourceChapter{chapter: sc}, composer, ref, true)
	}()
	return nil
}

// composeChapters runs the model sequentially (no parallel provider calls) and
// publishes the same `audio.compose` frames the file backend emits.
func (s *PGService) composeChapters(volume int, chapters []int, source map[int]SourceChapter, composer Composer, ref CharacterRef, refresh bool) {
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer setupCancel()
	scope, err := s.resolve(setupCtx)
	if err != nil {
		for _, chapter := range chapters {
			s.publishCompose(chapter, "error", 0, pgErr(err))
		}
		return
	}
	// Settings/cast are persisted state, not optional presentation: a storage
	// failure must surface instead of silently composing with defaults.
	d, _, derr := s.docFor(setupCtx, scope)
	if derr != nil {
		for _, chapter := range chapters {
			s.publishCompose(chapter, "error", 0, derr)
		}
		return
	}
	cast, cerr := s.castForRender(setupCtx)
	if cerr != nil {
		for _, chapter := range chapters {
			s.publishCompose(chapter, "error", 0, cerr)
		}
		return
	}
	enriched := EnsureCast(cast, ref.Characters, ref.Support)
	speakers := composeSpeakers(&enriched, ref)
	settings := d.Settings
	dict := d.Pronunciations

	for _, chapter := range chapters {
		sc := source[chapter]
		if !sc.Completed {
			s.publishCompose(chapter, "error", 0,
				fmt.Errorf("Chương %d chưa nghiệm thu — chỉ soạn bản đọc cho chương đã nghiệm thu.", chapter))
			continue
		}
		s.publishCompose(chapter, "drafting", 0, nil)

		ctx, cancel := context.WithTimeout(context.Background(), pgComposeChapterTimeout)
		// Snapshot the script row BEFORE the provider call. An edit made while
		// the model runs must fail our CAS (existing conflict) instead of being
		// overwritten by a revision we only read afterwards.
		refCh, rerr := s.chapterRef(ctx, scope, chapter)
		if rerr != nil {
			cancel()
			s.publishCompose(chapter, "error", 0, rerr)
			continue
		}
		old, meta, lerr := s.loadScriptRef(ctx, scope, refCh)
		if lerr != nil {
			cancel()
			s.publishCompose(chapter, "error", 0, lerr)
			continue
		}
		expected := int64(0)
		if meta != nil {
			expected = meta.Version
		}
		res, cerr := composer.Compose(ctx, ComposeInput{
			Volume: volume, Chapter: chapter, Title: sc.Title, Text: sc.Text,
			Characters: speakers, Mode: enriched.Mode,
			Normalize: settings.Normalize, Dict: dict,
		})
		if cerr != nil {
			cancel()
			s.publishCompose(chapter, "error", 0, cerr)
			continue
		}

		now := time.Now()
		fresh := &Script{
			Chapter: chapter, Volume: volume, Status: StatusReview,
			SourceHash: SourceHash(sc.Text), ComposedAt: &now, Model: res.Model,
			Segments: res.Segments,
		}
		if old != nil {
			if refresh {
				merged, _ := mergeScript(old, fresh)
				fresh = merged
				if old.Status == StatusLocked {
					fresh.Status = StatusLocked
					fresh.LockedAt = old.LockedAt
				}
			}
			// The update targets the revision we read before the model call,
			// exactly like WriteScript targets the existing file.
			fresh.pgRevision = old.pgRevision
		}
		applyVoiceFlags(fresh, &enriched)
		bundle, segments, werr := s.scriptBundle(refCh, fresh)
		if werr == nil {
			_, werr = s.deps.Audio.SaveComposedScriptBundle(ctx, scope, refCh, domain.ChapterContentSHA256(sc.Text), bundle, segments, changedComposeSegments(old, fresh), expected)
		}
		if werr != nil {
			cancel()
			if errors.Is(werr, errs.ErrStorageConflict) {
				s.publishCompose(chapter, "error", 0,
					conflict("Chương hoặc bản đọc vừa thay đổi trong lúc soạn — dữ liệu mới đã được giữ, vui lòng thử lại."))
			} else {
				s.publishCompose(chapter, "error", 0, werr)
			}
			continue
		}
		s.publishCompose(chapter, "done", len(fresh.Segments), nil)
		// A refreshed locked chapter is stale by definition: queue the changed
		// segments now, matching the file backend.
		if fresh.Status == StatusLocked {
			_, _ = s.Enqueue(ctx, volume, chapter, fresh.SourceHash)
		}
		cancel()
	}
}

// changedComposeSegments names the surviving segment ids whose audible text or
// parameters differ from the previous script. Their selected take is detached
// (the row stays as a candidate) exactly like SaveScript does for an edit;
// unchanged segments keep their existing audio.
func changedComposeSegments(old, fresh *Script) []string {
	if old == nil {
		return nil
	}
	prev := make(map[string]Segment, len(old.Segments))
	for _, seg := range old.Segments {
		prev[seg.ID] = seg
	}
	out := make([]string, 0)
	for _, seg := range fresh.Segments {
		before, ok := prev[seg.ID]
		if !ok {
			continue
		}
		if before.Text != seg.Text || before.Tone != seg.Tone ||
			before.Speaker != seg.Speaker || before.PauseMs != seg.PauseMs {
			out = append(out, seg.ID)
		}
	}
	return out
}

func (s *PGService) publishCompose(chapter int, state string, segments int, cause error) {
	data := map[string]any{
		"chapter": chapter, "state": state, "segments": segments, "percent": float64(0),
	}
	if state == "done" {
		data["percent"] = float64(100)
	}
	if cause != nil {
		data["error"] = cause.Error()
	}
	s.hub.Publish(NewEvent(EventCompose, data))
}
