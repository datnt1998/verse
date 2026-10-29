package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// Backend is the Postgres storage backend for one (project, story) scope: it
// supplies every store domain substore and implements the compound operations
// inside real transactions. Hosted startup still gates on the capability list
// (store.NewFromBackend) — nothing here is a silent partial stub.
//
// The file backend remains the offline/CLI backend; this one never touches a
// library directory.
type Backend struct {
	ctx   context.Context
	ds    *DocStore
	nar   *Narrative
	scope ScopeRef
	doms  store.Substores
	// docs is the scope-pinned document surface; comments the scoped lifecycle.
	docs     *store.IO
	comments *comments.Store
	// author attributes accepted chapter revisions when the caller does not
	// pass one; the engine's accept path is the writer agent.
	author ChapterAuthor
}

// BackendOption mutates backend construction.
type BackendOption func(*Backend)

// WithDefaultAuthor sets the attribution used by ChapterRecordStore.Accept.
func WithDefaultAuthor(author ChapterAuthor) BackendOption {
	return func(b *Backend) { b.author = author }
}

// WithLeaseFence makes every write transaction verify this story lease before
// mutating: documents and the typed rows (chapter versions, reviews) alike. It
// is required for engine runs: a stale writer must fail, never overwrite the
// new owner's chapters.
func WithLeaseFence(lease *StoryLease) BackendOption {
	return func(b *Backend) {
		if lease != nil {
			check := lease.Checker()
			b.ds.SetFence(check)
			b.nar = b.nar.WithFence(check)
		}
	}
}

// NewBackend builds the backend for an already-resolved scope. The pool must
// outlive the backend; Close is a no-op and the composition root owns it.
func NewBackend(ctx context.Context, p *Pool, nar *Narrative, scope ScopeRef, opts ...BackendOption) *Backend {
	b := &Backend{
		ctx:    ctx,
		ds:     NewDocStore(p),
		nar:    nar,
		scope:  scope,
		author: ChapterAuthor{Kind: "agent", AgentName: "writer"},
	}
	for _, opt := range opts {
		opt(b)
	}
	// Build the document-backed substores over per-store IO instances. The
	// constructor owns the path→row mapping, so every file-backend rule
	// (validation, ordering, idempotence) runs unchanged on rows.
	file := store.FileBackendOver(func() *store.IO {
		return store.NewIOFromOps(b.ds.ops(ctx, scope))
	})
	doms := file.Substores()
	// Reviews are typed rows (rounds + kept decisions), not documents.
	doms.World = &pgWorldStore{inner: doms.World, nar: b.nar, ctx: ctx, scope: scope}
	// Accepted chapter records are the immutable version table.
	doms.ChapterRecords = &pgChapterRecords{nar: b.nar, ctx: ctx, scope: scope, author: b.author}
	b.doms = doms
	b.docs = store.NewIOFromOps(b.ds.ops(ctx, scope))
	b.comments = comments.NewFromRepository(b.nar.Comments(scope))
	return b
}

func (b *Backend) Kind() store.BackendKind { return store.BackendPostgres }

// Dir is empty: postgres has no engine directory. Callers that need one must
// check file capability instead of writing into a fake path.
func (b *Backend) Dir() string { return "" }

func (b *Backend) Substores() store.Substores { return b.doms }

// Docs exposes the document surface pinned to this scope: the same names the
// file backend uses, mapped to story_state/outlines/artifacts/streams rows.
func (b *Backend) Docs() *store.IO { return b.docs }

// PlanChapters lists the chapter numbers that own a chapter_plan artifact row.
func (b *Backend) PlanChapters() ([]int, error) {
	var out []int
	err := RunInTx(b.ctx, b.ds.pool, func(tx pgx.Tx) error {
		rows, err := gen.New(tx).ListStoryArtifactsByKind(b.ctx, gen.ListStoryArtifactsByKindParams{
			ProjectID: b.scope.ProjectID, StoryID: b.scope.StoryID, Kind: "chapter_plan"})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, row := range rows {
			if n, ok := chapterFromKey(row.ArtifactKey); ok {
				out = append(out, n)
			}
		}
		return nil
	})
	return out, err
}

// ReviewChapters lists the chapters that still have a live review row.
func (b *Backend) ReviewChapters() ([]int, error) {
	list, err := b.nar.LatestReviewsForStory(b.ctx, b.scope)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	out := make([]int, 0, len(list))
	for _, rv := range list {
		if rv.Chapter <= 0 || seen[rv.Chapter] {
			continue
		}
		seen[rv.Chapter] = true
		out = append(out, rv.Chapter)
	}
	return out, nil
}

// chapterFromKey reads the numeric chapter from an artifact key ("03" or 3).
func chapterFromKey(key string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimLeft(key, "0"))
	if err != nil {
		return 0, false
	}
	return n, n > 0
}

// Comments binds the comment lifecycle to this scope's comment rows. The
// business rules live in internal/comments, identical to file mode.
func (b *Backend) Comments() *comments.Store { return b.comments }

// LatestReview returns the newest review row for the story: the highest
// chapter, with the global review winning a chapter tie — the same rule the
// file backend applies to reviews/NN.json vs NN-global.json.
func (b *Backend) LatestReview() (*domain.ReviewEntry, error) {
	list, err := b.nar.LatestReviewsForStory(b.ctx, b.scope)
	if err != nil {
		return nil, err
	}
	var best *domain.ReviewEntry
	bestGlobal := false
	for i := range list {
		rv := list[i]
		isGlobal := rv.Scope == "global" || rv.Scope == "arc"
		if best == nil || rv.Chapter > best.Chapter || (rv.Chapter == best.Chapter && isGlobal && !bestGlobal) {
			cp := rv
			best = &cp
			bestGlobal = isGlobal
		}
	}
	return best, nil
}

// Close is a no-op: the pool is process-scoped and closed by the composition
// root, never by a request-scoped backend.
func (b *Backend) Close() error { return nil }

func (b *Backend) docIO() *store.IO {
	return store.NewIOFromOps(b.ds.ops(b.ctx, b.scope))
}

func (b *Backend) LoadProjectFormatVersion() (int, error) {
	var format struct {
		Version int `json:"version"`
	}
	if err := b.docIO().ReadJSON(projectFormatDoc, &format); err != nil {
		if isNotExist(err) {
			return store.LegacyProjectFormatVersion, nil
		}
		return 0, err
	}
	if format.Version <= 0 {
		return 0, fmt.Errorf("Phiên bản định dạng dự án không hợp lệ: %d", format.Version)
	}
	return format.Version, nil
}

func (b *Backend) SaveProjectFormatVersion(version int) error {
	if version <= 0 {
		return fmt.Errorf("Phiên bản định dạng dự án phải lớn hơn 0: %d", version)
	}
	return b.docIO().WriteJSON(projectFormatDoc, map[string]int{"version": version})
}

// Init validates the checkpoint mirror (the document equivalent of loading
// meta/checkpoints.jsonl at startup) and creates nothing else: rows exist.
func (b *Backend) Init() error {
	if err := b.doms.Checkpoints.InitError(); err != nil {
		return fmt.Errorf("load checkpoints: %w", err)
	}
	return nil
}

// FoundationFingerprint hashes the same foundation documents as the file
// backend, in the same order. The hash is backend-local by construction: the
// architect only echoes back what novel_context read in this backend.
func (b *Backend) FoundationFingerprint() (string, error) {
	io := b.docIO()
	files := []string{"meta/book.json", "premise.md", "outline.json", "characters.json", "world_rules.json"}
	hasRetention, err := io.ExistsUnlocked("meta/story_retention.json")
	if err != nil {
		return "", fmt.Errorf("stat story retention: %w", err)
	}
	if hasRetention {
		files = append(files, "meta/story_retention.json")
	}
	layered, err := b.doms.Outline.LoadLayeredOutline()
	if err != nil {
		return "", fmt.Errorf("load layered outline: %w", err)
	}
	if len(layered) > 0 {
		files = append(files, "layered_outline.json", "meta/compass.json")
	}

	h := sha256.New()
	for _, rel := range files {
		data, err := io.ReadFileUnlocked(rel)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", rel, err)
		}
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

const projectFormatDoc = "meta/project_format.json"

func isNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist))
}

// withTxBackend runs fn over a temporary substore set bound to one real
// transaction, so a compound operation keeps the file backend's exact domain
// rules while committing atomically.
func (b *Backend) withTxBackend(fn func(tmp store.Backend) error) error {
	return RunInTx(b.ctx, b.ds.pool, func(tx pgx.Tx) error {
		if b.ds.fence != nil {
			if err := b.ds.fence(b.ctx, tx); err != nil {
				return err
			}
		}
		tmp := store.FileBackendOver(func() *store.IO {
			return store.NewIOFromOps(b.ds.BoundOps(b.ctx, b.scope, tx))
		})
		return fn(tmp)
	})
}

// ExpandArc mirrors the file backend's outline+progress expansion atomically.
func (b *Backend) ExpandArc(volumeIdx, arcIdx int, expansion domain.ArcExpansion) error {
	return b.withTxBackend(func(tmp store.Backend) error {
		return tmp.ExpandArc(volumeIdx, arcIdx, expansion)
	})
}

// AppendVolume appends a volume and rewrites derived outline views atomically.
func (b *Backend) AppendVolume(vol domain.VolumeOutline) error {
	return b.withTxBackend(func(tmp store.Backend) error { return tmp.AppendVolume(vol) })
}

// ReviseOutline replaces the future plan tail from fromChapter atomically.
func (b *Backend) ReviseOutline(fromChapter int, replacement []domain.OutlineEntry) (int, error) {
	var total int
	err := b.withTxBackend(func(tmp store.Backend) error {
		var err error
		total, err = tmp.ReviseOutline(fromChapter, replacement)
		return err
	})
	return total, err
}

// ClearHandledSteer clears the pending steer and the flow marker in one
// transaction (file mode orders two writes; postgres does not need a saga).
func (b *Backend) ClearHandledSteer() error {
	return b.withTxBackend(func(tmp store.Backend) error { return tmp.ClearHandledSteer() })
}

// InvalidateChapterAggregates deletes the model artefacts whose input range
// covers a revised chapter and invalidates reviews from it. Derived chapter
// projections are rebuilt by revision.Projector.
func (b *Backend) InvalidateChapterAggregates(fromChapter int) error {
	if fromChapter <= 0 {
		return fmt.Errorf("from chapter must be > 0")
	}
	volumes, err := b.doms.Outline.LoadLayeredOutline()
	if err != nil {
		return fmt.Errorf("Đọc đại cương phân tầng: %w", err)
	}
	chapter := 1
	arcEnds := make(map[[2]int]int)
	io := b.docIO()
	for _, volume := range volumes {
		volumeEnd := chapter - 1
		for _, arc := range volume.Arcs {
			if len(arc.Chapters) == 0 {
				continue
			}
			end := chapter + len(arc.Chapters) - 1
			arcEnds[[2]int{volume.Index, arc.Index}] = end
			if end >= fromChapter {
				for _, rel := range []string{
					fmt.Sprintf("summaries/arc-v%02da%02d.json", volume.Index, arc.Index),
					fmt.Sprintf("meta/snapshots/v%02da%02d.json", volume.Index, arc.Index),
				} {
					if err := io.RemoveFile(rel); err != nil {
						return err
					}
				}
			}
			chapter = end + 1
			volumeEnd = end
		}
		if volumeEnd >= fromChapter {
			if err := io.RemoveFile(fmt.Sprintf("summaries/vol-v%02d.json", volume.Index)); err != nil {
				return err
			}
		}
	}
	if style, err := b.doms.World.LoadStyleRules(); err != nil {
		return fmt.Errorf("Đọc luật văn phong: %w", err)
	} else if style != nil {
		end, ok := arcEnds[[2]int{style.Volume, style.Arc}]
		if !ok {
			return fmt.Errorf("Luật văn phong tham chiếu cung không tồn tại V%dA%d", style.Volume, style.Arc)
		}
		if end >= fromChapter {
			if err := io.RemoveFile("meta/style_rules.json"); err != nil {
				return err
			}
		}
	}
	return b.invalidateReviewsFrom(fromChapter)
}

// invalidateReviewsFrom marks every review at or after fromChapter invalid and
// drops the kept decisions attached to it (the row twin of deleting the file
// and its archived rounds).
func (b *Backend) invalidateReviewsFrom(fromChapter int) error {
	return b.nar.InvalidateReviewsFromChapter(b.ctx, b.scope, fromChapter)
}

var _ store.Backend = (*Backend)(nil)
