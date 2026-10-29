package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// fileBackend is the filesystem storage backend: it owns the concrete
// substores (file*Store) and implements cross-domain coordination with the
// documented saga semantics — single-file atomic writes plus ordered,
// idempotent replays, never a pretend multi-file transaction.
type fileBackend struct {
	dir     string
	crossMu sync.Mutex // 串行化跨域协调；不代表多个文件具备事务原子性
	domains Substores
	docs    *IO
	// comments is nil for the document-over-rows variant (FileBackendOver):
	// a postgres backend supplies its own scoped comment store.
	comments *comments.Store

	progress *fileProgressStore
	book     *fileBookStore
	outline  *fileOutlineStore
	drafts   *fileDraftStore
	summary  *fileSummaryStore
	runMeta  *fileRunMetaStore
	world    *fileWorldStore
	chars    *fileCharacterStore
}

func newFileBackend(dir string) *fileBackend {
	b := newFileBackendOver(func() *IO { return newIO(dir) }, dir)
	// The comment lifecycle lives beside the engine documents but not inside
	// them; bind it to the same story directory the file reader uses.
	b.comments = comments.NewForEngine(dir)
	return b
}

// FileBackendOver builds the file substores over caller-supplied IO instances.
// The postgres backend uses it to run the exact same domain logic on document
// rows: the constructor owns the mapping, the caller owns durability.
func FileBackendOver(mk func() *IO) Backend {
	return newFileBackendOver(mk, "")
}

func newFileBackendOver(mk func() *IO, dir string) *fileBackend {
	io := mk()
	naoIO := mk() // Nhân vật, quan hệ, xưng hô và kho ghi chú dùng chung khóa ghi.
	outline := newFileOutlineStore(io)
	b := &fileBackend{
		dir:      dir,
		docs:     mk(),
		progress: newFileProgressStore(mk()),
		book:     newFileBookStore(mk()),
		outline:  outline,
		drafts:   newFileDraftStore(mk()),
		summary:  newFileSummaryStore(mk(), outline),
		runMeta:  newFileRunMetaStore(mk()),
		world:    newFileWorldStore(naoIO),
		chars:    newFileCharacterStore(naoIO, outline),
	}
	b.domains = Substores{
		Progress:       b.progress,
		Book:           b.book,
		Outline:        b.outline,
		Drafts:         b.drafts,
		Summaries:      b.summary,
		RunMeta:        b.runMeta,
		UserRules:      newFileUserRulesStore(mk()),
		Signals:        newFileSignalStore(mk()),
		Runtime:        newFileRuntimeStore(mk()),
		Characters:     b.chars,
		Cast:           newFileCastStore(mk()),
		World:          b.world,
		Checkpoints:    newFileCheckpointStore(io),
		Sessions:       newFileSessionStore(mk()),
		Usage:          newFileUsageStore(mk()),
		Simulation:     newFileSimulationStore(mk()),
		Decisions:      newFileDecisionStore(mk()),
		ChapterRecords: newFileChapterRecordStore(mk()),
		Revisions:      newFileRevisionStore(mk()),
	}
	return b
}

func (b *fileBackend) Kind() BackendKind    { return BackendFile }
func (b *fileBackend) Dir() string          { return b.dir }
func (b *fileBackend) Docs() *IO            { return b.docs }
func (b *fileBackend) Substores() Substores { return b.domains }
func (b *fileBackend) Close() error         { return nil }

// Comments binds the file comment lifecycle to this engine directory.
func (b *fileBackend) Comments() *comments.Store { return b.comments }

// chapterMarkers lists chapter numbers from engine directories that hold one
// file per marker: drafts/NN.plan.json and reviews/NN.json|NN-global.json.
func (b *fileBackend) chapterMarkers(dir string, re *regexp.Regexp) ([]int, error) {
	entries, err := b.docs.ListDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		m := re.FindStringSubmatch(e.Name)
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			out = append(out, n)
		}
	}
	return out, nil
}

var planFilePattern = regexp.MustCompile(`^(\d+)\.plan\.json$`)

// PlanChapters reports chapters with a plan document.
func (b *fileBackend) PlanChapters() ([]int, error) {
	return b.chapterMarkers("drafts", planFilePattern)
}

// ReviewChapters reports chapters with any review document.
func (b *fileBackend) ReviewChapters() ([]int, error) {
	return b.chapterMarkers("reviews", reviewFilePattern)
}

// reviewFilePattern nhận đúng tên tệp bài thẩm duyệt trong reviews/: NN.json
// và NN-global.json. reviews/decisions.json (GAP-47) và reviews/rounds/ không
// phải bài thẩm duyệt nên LatestReview phải bỏ qua.
var reviewFilePattern = regexp.MustCompile(`^(\d+)(?:-global)?\.json$`)

// LatestReview returns the review with the highest chapter; when both a
// chapter and a global review exist for that chapter the global one wins.
// Listing goes through the document surface, never os.ReadDir.
func (b *fileBackend) LatestReview() (*domain.ReviewEntry, error) {
	entries, err := b.docs.ListDir("reviews")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var best *domain.ReviewEntry
	bestGlobal := false
	for _, e := range entries {
		if e.IsDir || !reviewFilePattern.MatchString(e.Name) {
			continue
		}
		var rv domain.ReviewEntry
		data, err := b.docs.ReadFile("reviews/" + e.Name)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &rv); err != nil {
			return nil, fmt.Errorf("đọc tệp thẩm duyệt reviews/%s: %w", e.Name, err)
		}
		isGlobal := rv.Scope == "global" || rv.Scope == "arc"
		if best == nil || rv.Chapter > best.Chapter || (rv.Chapter == best.Chapter && isGlobal && !bestGlobal) {
			cp := rv
			best = &cp
			bestGlobal = isGlobal
		}
	}
	return best, nil
}
func (b *fileBackend) LoadProjectFormatVersion() (int, error) {
	var format projectFormat
	if err := b.progress.io.ReadJSON(projectFormatPath, &format); err != nil {
		if os.IsNotExist(err) {
			return LegacyProjectFormatVersion, nil
		}
		return 0, err
	}
	if format.Version <= 0 {
		return 0, fmt.Errorf("Phiên bản định dạng dự án không hợp lệ: %d", format.Version)
	}
	return format.Version, nil
}

// SaveProjectFormatVersion 在一次迁移全部完成后原子更新项目格式版本。
func (b *fileBackend) SaveProjectFormatVersion(version int) error {
	if version <= 0 {
		return fmt.Errorf("Phiên bản định dạng dự án phải lớn hơn 0: %d", version)
	}
	return b.progress.io.WriteJSON(projectFormatPath, projectFormat{Version: version})
}

// Init 创建所需的子目录结构。
func (b *fileBackend) Init() error {
	if err := b.domains.Checkpoints.InitError(); err != nil {
		return fmt.Errorf("load checkpoints: %w", err)
	}
	return b.progress.io.EnsureDirs([]string{
		"chapters", "summaries", "drafts", "reviews", "meta", "meta/chapter_records", "meta/runtime", "meta/runtime/tasks", "meta/sessions", "meta/sessions/agents",
	})
}

// FoundationFingerprint 返回当前基础设定工件的内容指纹。Architect 必须把
// novel_context 读到的这个值原样交回审查工具，确保结论针对的是实际落盘版本，
// 而不是会话中尚未保存或已经过期的内容。
func (b *fileBackend) FoundationFingerprint() (string, error) {
	files := []string{"meta/book.json", "premise.md", "outline.json", "characters.json", "world_rules.json"}
	if _, err := os.Stat(filepath.Join(b.dir, "meta", "story_retention.json")); err == nil {
		files = append(files, "meta/story_retention.json")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat story retention: %w", err)
	}
	layered, err := b.outline.LoadLayeredOutline()
	if err != nil {
		return "", fmt.Errorf("load layered outline: %w", err)
	}
	if len(layered) > 0 {
		files = append(files, "layered_outline.json", "meta/compass.json")
	}

	h := sha256.New()
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(b.dir, filepath.FromSlash(rel)))
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

// ── 跨域协调方法 ──

// ExpandArc 将骨架弧校准并展开为详细章节（Outline + Progress 联动）。
func (b *fileBackend) ExpandArc(volumeIdx, arcIdx int, expansion domain.ArcExpansion) (err error) {
	b.crossMu.Lock()
	defer b.crossMu.Unlock()

	defer b.outline.io.lockWrite()(&err)

	volumes, err := b.outline.expandArcUnlocked(volumeIdx, arcIdx, expansion)
	if err != nil {
		return err
	}

	defer b.progress.io.lockWrite()(&err)

	p, err := b.progress.loadUnlocked()
	if err != nil {
		return err
	}
	if p == nil {
		p = &domain.Progress{}
	}
	p.TotalChapters = domain.EstimatedChapterCapacity(volumes)
	return b.progress.saveUnlocked(p)
}

// AppendVolume 追加新卷到分层大纲末尾（Outline + Progress 联动）。
func (b *fileBackend) AppendVolume(vol domain.VolumeOutline) (err error) {
	b.crossMu.Lock()
	defer b.crossMu.Unlock()

	defer b.outline.io.lockWrite()(&err)

	volumes, err := b.outline.appendVolumeUnlocked(vol)
	if err != nil {
		return err
	}

	defer b.progress.io.lockWrite()(&err)

	p, err := b.progress.loadUnlocked()
	if err != nil {
		return err
	}
	if p == nil {
		p = &domain.Progress{}
	}
	p.TotalChapters = domain.EstimatedChapterCapacity(volumes)
	return b.progress.saveUnlocked(p)
}

// ReviseOutline 从 fromChapter 起替换尚未发生的计划尾段。
// 扁平大纲替换全书尾段；分层大纲只替换目标章所在弧的尾段。这个定义让同一载荷
// 重放仍得到同一结果，同时避免 JSON Patch 和 insert/delete 等操作枚举。
func (b *fileBackend) ReviseOutline(fromChapter int, replacement []domain.OutlineEntry) (_ int, err error) {
	if fromChapter <= 0 {
		return 0, fmt.Errorf("from_chapter must be > 0: %w", errs.ErrToolArgs)
	}

	b.crossMu.Lock()
	defer b.crossMu.Unlock()

	defer b.outline.io.lockWrite()(&err)
	defer b.progress.io.lockWrite()(&err)

	p, err := b.progress.loadUnlocked()
	if err != nil {
		return 0, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if p == nil {
		return 0, fmt.Errorf("progress chưa được khởi tạo: %w", errs.ErrToolPrecondition)
	}
	if p.Phase == domain.PhaseComplete {
		return 0, fmt.Errorf("Truyện đã hoàn thành, không được sửa đại cương: %w", errs.ErrToolPrecondition)
	}
	protected := p.InProgressChapter
	if latest := p.LatestCompleted(); latest > protected {
		protected = latest
	}
	if fromChapter <= protected {
		// 只报"不许改"会把调用方逼进死路：实测架构师在此连试 4 次（from=21/22/13，
		// 再退到 save_foundation(outline)）全被拒，空转到熔断。错误必须同时说明谁负责
		// 返工，否则架构师会继续在自己的工具集里找一个并不存在的出口。
		return 0, fmt.Errorf(
			"Chương %d đã hoàn thành hoặc đang được viết; revise_outline chỉ được sửa các chương chưa xảy ra, phải bắt đầu sau chương %d. "+
				"Các chương đã viết nằm trong pending_rewrites không thuộc quyền sửa đại cương: việc làm lại do writer chạy theo hàng đợi; "+
				"nếu Architect không còn chương tương lai nào cần sửa, hãy gọi resolve_outline_feedback để xác nhận quy hoạch hiện tại vẫn dùng được rồi kết thúc: %w",
			fromChapter, protected, errs.ErrToolPrecondition)
	}

	if p.Layered {
		volumes, err := b.outline.reviseLayeredTailUnlocked(fromChapter, replacement)
		if err != nil {
			return 0, err
		}
		p.TotalChapters = domain.EstimatedChapterCapacity(volumes)
		if err := b.progress.saveUnlocked(p); err != nil {
			return 0, fmt.Errorf("save progress: %w: %w", errs.ErrStoreWrite, err)
		}
		return p.TotalChapters, nil
	}

	outline, err := b.outline.reviseFlatTailUnlocked(fromChapter, replacement)
	if err != nil {
		return 0, err
	}
	p.TotalChapters = len(outline)
	if err := b.progress.saveUnlocked(p); err != nil {
		return 0, fmt.Errorf("save progress: %w: %w", errs.ErrStoreWrite, err)
	}
	return p.TotalChapters, nil
}

// ClearHandledSteer 清除 PendingSteer 并重置旧版 FlowSteering 状态。
// 两个文件无法组成文件系统事务，因此先写可重复的 Progress，最后才删除恢复意图；
// 任一步失败都至少保留 PendingSteer，下一次 Resume 可以安全重放。
func (b *fileBackend) ClearHandledSteer() (err error) {
	b.crossMu.Lock()
	defer b.crossMu.Unlock()

	defer b.runMeta.io.lockWrite()(&err)
	defer b.progress.io.lockWrite()(&err)

	meta, err := b.runMeta.loadUnlocked()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	p, err := b.progress.loadUnlocked()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if p != nil && p.Flow == domain.FlowSteering {
		if err := domain.ValidateFlowTransition(p.Flow, domain.FlowWriting); err != nil {
			return err
		}
		p.Flow = domain.FlowWriting
		if err := b.progress.saveUnlocked(p); err != nil {
			return err
		}
	}
	if meta != nil && meta.PendingSteer != "" {
		meta.PendingSteer = ""
		if err := b.runMeta.saveUnlocked(*meta); err != nil {
			return err
		}
	}
	return nil
}

// InvalidateChapterAggregates 删除输入范围包含修订章节的模型派生工件。
// 章节级投影由 revision.Projector 单独重建。
func (b *fileBackend) InvalidateChapterAggregates(fromChapter int) error {
	if fromChapter <= 0 {
		return fmt.Errorf("from chapter must be > 0")
	}
	volumes, err := b.outline.LoadLayeredOutline()
	if err != nil {
		return fmt.Errorf("Đọc đại cương phân tầng: %w", err)
	}
	chapter := 1
	arcEnds := make(map[[2]int]int)
	for _, volume := range volumes {
		volumeEnd := chapter - 1
		for _, arc := range volume.Arcs {
			if len(arc.Chapters) == 0 {
				continue
			}
			end := chapter + len(arc.Chapters) - 1
			arcEnds[[2]int{volume.Index, arc.Index}] = end
			if end >= fromChapter {
				if err := b.summary.io.RemoveFile(fmt.Sprintf("summaries/arc-v%02da%02d.json", volume.Index, arc.Index)); err != nil {
					return err
				}
				if err := b.chars.io.RemoveFile(fmt.Sprintf("meta/snapshots/v%02da%02d.json", volume.Index, arc.Index)); err != nil {
					return err
				}
			}
			chapter = end + 1
			volumeEnd = end
		}
		if volumeEnd >= fromChapter {
			if err := b.summary.io.RemoveFile(fmt.Sprintf("summaries/vol-v%02d.json", volume.Index)); err != nil {
				return err
			}
		}
	}
	if style, err := b.world.LoadStyleRules(); err != nil {
		return fmt.Errorf("Đọc luật văn phong: %w", err)
	} else if style != nil {
		end, ok := arcEnds[[2]int{style.Volume, style.Arc}]
		if !ok {
			return fmt.Errorf("Luật văn phong tham chiếu cung không tồn tại V%dA%d", style.Volume, style.Arc)
		}
		if end >= fromChapter {
			if err := b.world.io.RemoveFile("meta/style_rules.json"); err != nil {
				return err
			}
		}
	}
	return b.invalidateReviewsFrom(fromChapter)
}

func (b *fileBackend) invalidateReviewsFrom(fromChapter int) error {
	entries, err := b.world.io.ListDirUnlocked("reviews")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Name, ".json") {
			continue
		}
		rel := "reviews/" + entry.Name
		data, err := b.world.io.ReadFile(rel)
		if err != nil {
			return err
		}
		var review domain.ReviewEntry
		if err := json.Unmarshal(data, &review); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		if review.Chapter >= fromChapter {
			if err := b.world.io.RemoveFile(rel); err != nil {
				return err
			}
		}
	}
	// 旧轮次存档与被作废的评审同生共死：评审因外部修订失效后，其历史轮次
	// 不能再作为"lần chấm trước"出现在 dải xanh / ma trận chất lượng.
	return b.invalidateReviewRoundsFrom(fromChapter)
}

// invalidateReviewRoundsFrom 删除 reviews/rounds/ 下属于第 fromChapter 章及以后
// 的旧轮次存档（NN-r<K>.json 与 NN-global-r<K>.json）。
func (b *fileBackend) invalidateReviewRoundsFrom(fromChapter int) error {
	entries, err := b.world.io.ListDirUnlocked(reviewRoundsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`^(\d+)(?:-global)?-r\d+\.json$`)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		m := re.FindStringSubmatch(entry.Name)
		if m == nil {
			continue
		}
		chapter, err := strconv.Atoi(m[1])
		if err != nil || chapter < fromChapter {
			continue
		}
		if err := b.world.io.RemoveFile(reviewRoundsDir + "/" + entry.Name); err != nil {
			return err
		}
	}
	return nil
}
