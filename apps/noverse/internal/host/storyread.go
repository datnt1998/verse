package host

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

// engineSubdir 是故事目录下引擎输出目录的固定相对路径（bootstrap 默认 output/novel）。
const engineSubdir = "output/novel"

// StoryReader 直接读取故事引擎目录，不持有故事租约：gateway 的直读端点
// （故事被 TUI/worker 持锁时也能读）与自动导出走这里。全部方法只读，
// 与正在运行的写方并发安全（所有 loader 本就按"缺失即降级"设计）。
type StoryReader struct {
	storyDir string
	store    *storepkg.Store
}

// NewStoryReader 以故事目录创建读取器；store 指向 <story>/output/novel。
func NewStoryReader(storyDir string) *StoryReader {
	return &StoryReader{storyDir: storyDir, store: storepkg.NewStore(filepath.Join(storyDir, engineSubdir))}
}

// NewStoryReaderWithStore 以既有 store 创建（worker 内复用，不再 new 一份；story
// list 也用它从同一个 store 读完整卡片，避免重复读 checkpoints）。
// 故事目录 = 引擎目录（output/novel）的上一级；hosted store 没有目录，storyDir
// 为空而不是 "."——调用方必须靠 store，不得从空目录推导路径。
func NewStoryReaderWithStore(s *storepkg.Store) *StoryReader {
	storyDir := ""
	if dir, ok := s.FileRoot(); ok {
		storyDir = filepath.Dir(filepath.Dir(dir))
	}
	return &StoryReader{storyDir: storyDir, store: s}
}

// Store 暴露底层 store 给同包内需要更多 loader 的调用方（自动导出）。
func (r *StoryReader) Store() *storepkg.Store { return r.store }

// EngineDir 返回引擎输出目录。
func (r *StoryReader) EngineDir() string { return r.store.Dir() }

// PremiseSection contains a canonical heading and its stored, unabridged text.
type PremiseSection struct {
	Heading string `json:"heading"`
	Text    string `json:"text"`
}

type StoryPremise struct {
	Sections  []PremiseSection `json:"sections"`
	Missing   []string         `json:"missing"`
	UpdatedAt *time.Time       `json:"updatedAt"`
}

// Premise uses the same parser and tier-specific requirements as the engine.
func (r *StoryReader) Premise() (*StoryPremise, error) {
	text, err := r.store.Outline.LoadPremise()
	if err != nil {
		return nil, err
	}
	meta, err := r.store.RunMeta.Load()
	if err != nil {
		return nil, err
	}
	tier := domain.PlanningTier("")
	if meta != nil {
		tier = meta.PlanningTier
	}
	parsed := tools.ParsePremiseSections(text)
	required := tools.RequiredPremiseHeadings(tier)
	result := &StoryPremise{Sections: []PremiseSection{}, Missing: []string{}}
	for _, heading := range required {
		if _, ok := parsed[heading]; !ok {
			result.Missing = append(result.Missing, heading)
		}
	}
	// Canonical template order is stable, including populated optional headings.
	for _, heading := range append(tools.RequiredPremiseHeadings(domain.PlanningTierLong), "Tính phù hợp với truyện ngắn") {
		if body, ok := parsed[heading]; ok {
			result.Sections = append(result.Sections, PremiseSection{Heading: heading, Text: body})
			delete(parsed, heading)
		}
	}
	extra := make([]string, 0, len(parsed))
	for heading := range parsed {
		extra = append(extra, heading)
	}
	slices.Sort(extra)
	for _, heading := range extra {
		result.Sections = append(result.Sections, PremiseSection{Heading: heading, Text: parsed[heading]})
	}
	updated, err := r.store.Docs().ModifiedAt("premise.md")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		updated = updated.UTC()
		result.UpdatedAt = &updated
	}
	return result, nil
}

// WorldRules preserves all stored fields and returns [] before foundation exists.
func (r *StoryReader) WorldRules() ([]domain.WorldRule, error) {
	rules, err := r.store.World.LoadWorldRules()
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []domain.WorldRule{}
	}
	return rules, nil
}

// ── 章节 ──

// StoryChapter 对应契约 ChapterInfo（camelCase 扩展字段）。
type StoryChapter struct {
	Chapter   int    `json:"chapter"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Words     int    `json:"words"`
	HasDraft  bool   `json:"hasDraft"`
	Volume    int    `json:"volume"`
	Arc       int    `json:"arc"`
	// Comments đếm góp ý cần chú ý (§AI): need = lost+unsure, pending = chờ gửi.
	// Chỉ gateway điền (nó đọc tệp .ainovel/gop-y); host.Chapters để nil.
	Comments *comments.Counts `json:"comments,omitempty"`
}

// StoryChapterText 对应 GET /chapters/:n?source= 的响应。
type StoryChapterText struct {
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Source  string `json:"source"`
	Words   int    `json:"words"`
}

// Chapters 汇总大纲章节与已完成章节，补齐 draft/卷/弧归属。
func (r *StoryReader) Chapters() ([]StoryChapter, error) {
	progress, err := r.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	outline, err := r.store.Outline.LoadOutline()
	if err != nil {
		return nil, err
	}

	locate := map[int][2]int{}
	if progress != nil && progress.Layered {
		if vols, err := r.store.Outline.LoadLayeredOutline(); err == nil {
			locate = chapterVolumes(vols)
		} else {
			return nil, err
		}
	}

	byCh := map[int]*StoryChapter{}
	order := []int{}
	put := func(ch int) *StoryChapter {
		c, ok := byCh[ch]
		if !ok {
			c = &StoryChapter{Chapter: ch, Volume: 1}
			if va, ok := locate[ch]; ok {
				c.Volume, c.Arc = va[0], va[1]
			}
			byCh[ch] = c
			order = append(order, ch)
		}
		return c
	}

	for _, e := range outline {
		put(e.Chapter).Title = e.Title
	}
	completed := map[int]bool{}
	if progress != nil {
		for _, n := range progress.CompletedChapters {
			completed[n] = true
			c := put(n)
			c.Completed = true
			c.Words = progress.ChapterWordCounts[n]
		}
		if progress.InProgressChapter > 0 {
			put(progress.InProgressChapter)
		}
	}

	// 章节标题：大纲为准，缺省回退已提交摘要标题（导出器同一约定）。
	for _, n := range order {
		c := byCh[n]
		if c.Title == "" {
			if sum, err := r.store.Summaries.LoadSummary(n); err == nil && sum != nil {
				c.Title = sum.Title
			}
		}
		draft, err := r.store.Drafts.LoadDraft(n)
		if err != nil {
			return nil, err
		}
		c.HasDraft = strings.TrimSpace(draft) != ""
	}

	out := make([]StoryChapter, 0, len(order))
	slices.Sort(order)
	for _, n := range order {
		out = append(out, *byCh[n])
	}
	return out, nil
}

// ChapterText 读取单章正文。source: "committed"（默认）| "draft"。
func (r *StoryReader) ChapterText(n int, source string) (*StoryChapterText, error) {
	if n <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	if source == "" {
		source = "committed"
	}
	if source != "committed" && source != "draft" {
		return nil, fmt.Errorf("source phải là committed hoặc draft")
	}
	var text string
	var err error
	if source == "draft" {
		text, err = r.store.Drafts.LoadDraft(n)
	} else {
		text, err = r.store.Drafts.LoadChapterText(n)
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	return &StoryChapterText{
		Chapter: n,
		Title:   r.chapterTitle(n),
		Text:    text,
		Source:  source,
		Words:   domain.WordCount(text),
	}, nil
}

// chapterTitle 取章节标题：大纲优先，摘要兜底。
func (r *StoryReader) chapterTitle(n int) string {
	if e, err := r.store.Outline.GetChapterOutline(n); err == nil && e != nil && e.Title != "" {
		return e.Title
	}
	if sum, err := r.store.Summaries.LoadSummary(n); err == nil && sum != nil {
		return sum.Title
	}
	return ""
}

// ChapterPlan 读取章节构思；未落盘返回 ErrChapterNotFound。
func (r *StoryReader) ChapterPlan(n int) (*domain.ChapterPlan, error) {
	plan, err := r.store.Drafts.LoadChapterPlan(n)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	return plan, nil
}

// ChapterFacts 读取章节事实（章节记录中的结构化事实）；未落盘返回 ErrChapterNotFound。
func (r *StoryReader) ChapterFacts(n int) (*domain.ChapterFacts, error) {
	rec, err := r.store.ChapterRecords.Load(n)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("%w: %d", ErrChapterNotFound, n)
	}
	return &rec.Facts, nil
}

// ChapterRevision 对应契约 revisions 数组元素。当前引擎不持久化编辑历史
// （edit_chapter 只改 draft 文件，无逐条记录），端点返回空数组。
type ChapterRevision struct {
	Time       *time.Time `json:"time"`
	Kind       string     `json:"kind"`
	Old        string     `json:"old"`
	New        string     `json:"new"`
	ReplaceAll bool       `json:"replaceAll"`
}

// ChapterRevisions 返回该章已记录的修订。引擎不落盘修订历史时为空数组。
func (r *StoryReader) ChapterRevisions(n int) ([]ChapterRevision, error) {
	return []ChapterRevision{}, nil
}

// ── 大纲 ──

// 契约 §4 Outline 结构。
type StoryOutlineChapter struct {
	Chapter   int    `json:"chapter"`
	Title     string `json:"title"`
	CoreEvent string `json:"coreEvent"`
	Summary   string `json:"summary"`
	Completed bool   `json:"completed"`
	Words     int    `json:"words"`
	Status    string `json:"status"` // done | writing | pending | rewrite | rewriting
	HasPlan   bool   `json:"hasPlan"`
	HasReview bool   `json:"hasReview"`
}

type StoryOutlineArc struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Chapters []int  `json:"chapters"`
	// EstimatedChapters: 骨架卷预计章数（骨架卷无实体章节，供 TOC 显示“N ch. · chưa dàn ý”）。
	EstimatedChapters int `json:"estimatedChapters,omitempty"`
}

type StoryOutlineVolume struct {
	Index    int                   `json:"index"`
	Title    string                `json:"title"`
	Summary  string                `json:"summary"`
	Arcs     []StoryOutlineArc     `json:"arcs"`
	Chapters []StoryOutlineChapter `json:"chapters"`
}

type StoryOutline struct {
	Layered          bool                 `json:"layered"`
	Path             string               `json:"path"`
	Volumes          []StoryOutlineVolume `json:"volumes"`
	NextVolumeTitle  string               `json:"nextVolumeTitle"`
	CompassDirection string               `json:"compassDirection"`
	CompassScale     string               `json:"compassScale"`
}

// chapterVolumes 把分层大纲展开成 {全局章号 → [卷, 弧]}，章号规则与
// FlattenOutline 一致（卷内弧内顺序累加）。
func chapterVolumes(volumes []domain.VolumeOutline) map[int][2]int {
	m := make(map[int][2]int)
	ch := 0
	for _, v := range volumes {
		for _, a := range v.Arcs {
			for range a.Chapters {
				ch++
				m[ch] = [2]int{v.Index, a.Index}
			}
		}
	}
	return m
}

// fileMarkers 返回"有计划文件的章"与"有评审文件的章"。
// store backend 决定来源（file 列目录，postgres 读 plan artifact/review 行），
// reader 不再直接扫目录。
func (r *StoryReader) fileMarkers() (plans, reviews map[int]bool) {
	plans = map[int]bool{}
	reviews = map[int]bool{}
	if list, err := r.store.PlanChapters(); err == nil {
		for _, n := range list {
			plans[n] = true
		}
	}
	if list, err := r.store.ReviewChapters(); err == nil {
		for _, n := range list {
			reviews[n] = true
		}
	}
	return plans, reviews
}

// chapterStatus 推订单章状态：rewrite 队列优先于写作中；重写且正在写 = rewriting。
func chapterStatus(ch int, progress *domain.Progress) string {
	completed := progress != nil && slices.Contains(progress.CompletedChapters, ch)
	pendingRewrite := progress != nil && slices.Contains(progress.PendingRewrites, ch)
	inProgress := progress != nil && progress.InProgressChapter == ch
	switch {
	case pendingRewrite && inProgress:
		return "rewriting"
	case pendingRewrite:
		return "rewrite"
	case completed:
		return "done"
	case inProgress:
		return "writing"
	default:
		return "pending"
	}
}

// OutlineAllVolumes：Outline() 的卷参数——返回全部章（默认行为）。
const OutlineAllVolumes = -1

// Outline 组装契约 §4 Outline：扁平故事折进单卷，分层故事按卷/弧展开，
// 每章带状态（含重写队列）、计划与评审标记。
// volume：-1 = 全部章；0 = 只有卷头（章列表空，arcs 保留章号）；N = 只有第 N 卷有章。
func (r *StoryReader) Outline(volume int) (*StoryOutline, error) {
	progress, err := r.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	outline, err := r.store.Outline.LoadOutline()
	if err != nil {
		return nil, err
	}
	// 书名只用于展示；meta/book.json 不完整（如缺梗概）时不应让整份大纲读取失败。
	bookTitle := r.BookTitle()
	plans, reviews := r.fileMarkers()
	completedSet := map[int]bool{}
	if progress != nil {
		for _, n := range progress.CompletedChapters {
			completedSet[n] = true
		}
	}

	chapterOf := func(e domain.OutlineEntry) (StoryOutlineChapter, error) {
		c := StoryOutlineChapter{
			Chapter:   e.Chapter,
			Title:     e.Title,
			CoreEvent: e.CoreEvent,
			Completed: completedSet[e.Chapter],
			HasPlan:   plans[e.Chapter],
			HasReview: reviews[e.Chapter],
		}
		if progress != nil {
			c.Words = progress.ChapterWordCounts[e.Chapter]
		}
		c.Status = chapterStatus(e.Chapter, progress)
		if sum, err := r.store.Summaries.LoadSummary(e.Chapter); err != nil {
			return c, err
		} else if sum != nil {
			c.Summary = sum.Summary
			if c.Title == "" {
				c.Title = sum.Title
			}
		}
		return c, nil
	}

	result := &StoryOutline{
		Path:    "output/novel/outline.json",
		Volumes: []StoryOutlineVolume{},
	}
	layered := progress != nil && progress.Layered
	result.Layered = layered

	if layered {
		volumes, err := r.store.Outline.LoadLayeredOutline()
		if err != nil {
			return nil, err
		}
		// 指南针与下一卷标题是分层模式的导航信息。
		if compass, err := r.store.Outline.LoadCompass(); err != nil {
			return nil, err
		} else if compass != nil {
			result.CompassDirection = compass.EndingDirection
			result.CompassScale = compass.EstimatedScale
		}
		ch := 1
		for i := range volumes {
			v := &volumes[i]
			vol := StoryOutlineVolume{Index: v.Index, Title: v.Title, Arcs: []StoryOutlineArc{}, Chapters: []StoryOutlineChapter{}}
			if vsum, err := r.store.Summaries.LoadVolumeSummary(v.Index); err != nil {
				return nil, err
			} else if vsum != nil {
				vol.Summary = vsum.Summary
			}
			want := volume == OutlineAllVolumes || v.Index == volume
			for _, a := range v.Arcs {
				arc := StoryOutlineArc{Index: a.Index, Title: a.Title, EstimatedChapters: a.EstimatedChapters, Chapters: []int{}}
				if asum, err := r.store.Summaries.LoadArcSummary(v.Index, a.Index); err != nil {
					return nil, err
				} else if asum != nil {
					arc.Summary = asum.Summary
				}
				for _, e := range a.Chapters {
					e.Chapter = ch // 全局章号 = 卷内弧内顺序累加（同 FlattenOutline）
					ch++
					arc.Chapters = append(arc.Chapters, e.Chapter)
					if !want {
						continue
					}
					c, err := chapterOf(e)
					if err != nil {
						return nil, err
					}
					vol.Chapters = append(vol.Chapters, c)
				}
				vol.Arcs = append(vol.Arcs, arc)
			}
			result.Volumes = append(result.Volumes, vol)
		}
		// 下一卷标题 = 首个尚未展开的骨架卷。
		for i := range volumes {
			if !volumes[i].IsExpanded() {
				result.NextVolumeTitle = volumes[i].Title
				break
			}
		}
		return result, nil
	}

	// 扁平故事 = 单卷（契约：index 1，title = 故事名）。
	vol := StoryOutlineVolume{Index: 1, Title: bookTitle, Arcs: []StoryOutlineArc{}, Chapters: []StoryOutlineChapter{}}
	arc := StoryOutlineArc{Index: 1, Chapters: []int{}}
	want := volume == OutlineAllVolumes || volume == 1
	for _, e := range outline {
		arc.Chapters = append(arc.Chapters, e.Chapter)
		if !want {
			continue
		}
		c, err := chapterOf(e)
		if err != nil {
			return nil, err
		}
		vol.Chapters = append(vol.Chapters, c)
	}
	if len(arc.Chapters) > 0 {
		vol.Arcs = append(vol.Arcs, arc)
	}
	result.Volumes = append(result.Volumes, vol)
	return result, nil
}

// ── 角色 / 伏笔 / 评审 ──

// Characters 返回核心角色档案与配角名册（原始 engine tag）。
func (r *StoryReader) Characters() ([]domain.Character, []domain.CastEntry, error) {
	chars, err := r.store.Characters.Load()
	if err != nil {
		return nil, nil, err
	}
	cast, err := r.store.Cast.Load()
	if err != nil {
		return nil, nil, err
	}
	return chars, cast, nil
}

// Foreshadow 返回伏笔台账（原始 engine tag）。
func (r *StoryReader) Foreshadow() ([]domain.ForeshadowEntry, error) {
	return r.store.World.LoadForeshadowLedger()
}

// StoryReviews 对应 GET /reviews/:chapter。
type StoryReviews struct {
	Chapter *domain.ReviewEntry `json:"chapter"`
	Global  *domain.ReviewEntry `json:"global"`
}

// Reviews 读取指定章的章节评审与同章落盘的全局评审。
func (r *StoryReader) Reviews(chapter int) (StoryReviews, error) {
	ch, err := r.store.World.LoadReview(chapter)
	if err != nil {
		return StoryReviews{}, err
	}
	gl, err := r.store.World.LoadGlobalReview(chapter)
	if err != nil {
		return StoryReviews{}, err
	}
	return StoryReviews{Chapter: ch, Global: gl}, nil
}

// LatestReview 返回最新的一篇评审（章号最大；同章时全局评审优先）。
// 由 store backend 枚举：file 走文档面，postgres 走 review 行——这里不再
// 直接 os.ReadDir，hosted 路径没有目录可扫。无评审返回 nil（调用方映射 404）。
func (r *StoryReader) LatestReview() (*domain.ReviewEntry, error) {
	return r.store.LatestReview()
}

// ── 导出 ──

// Export 透传到 exp.Run（只读 store，引擎运行中也可导出）。
func (r *StoryReader) Export(ctx context.Context, opts exp.Options) (*exp.Result, error) {
	return exp.Run(ctx, exp.Deps{Store: r.store}, opts)
}

// BookTitle 返回书名（下载文件名用）。
func (r *StoryReader) BookTitle() string {
	if book, err := r.store.Book.Load(); err == nil && book != nil {
		return book.Title
	}
	return ""
}

// VolumeRange 是一卷对应的全局章号区间（闭区间）。
type VolumeRange struct {
	Volume int
	Title  string
	From   int
	To     int
}

// VolumeRanges 计算每卷覆盖的章节范围：分层按弧内章数累加（与 FlattenOutline 同规则），
// 扁平故事恒为单卷 [1, max(大纲章数, 最大完成章)]。无大纲返回空。
func (r *StoryReader) VolumeRanges() ([]VolumeRange, error) {
	progress, err := r.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	volumes, err := r.store.Outline.LoadLayeredOutline()
	if err != nil {
		return nil, err
	}
	if len(volumes) > 0 {
		var ranges []VolumeRange
		ch := 1
		for _, v := range volumes {
			from := ch
			for _, a := range v.Arcs {
				ch += len(a.Chapters)
			}
			ranges = append(ranges, VolumeRange{Volume: v.Index, Title: v.Title, From: from, To: ch - 1})
		}
		return ranges, nil
	}
	outline, err := r.store.Outline.LoadOutline()
	if err != nil {
		return nil, err
	}
	latest := 0
	if progress != nil {
		latest = progress.LatestCompleted()
	}
	to := max(latest, len(outline))
	if to == 0 {
		return nil, nil
	}
	return []VolumeRange{{Volume: 1, Title: r.BookTitle(), From: 1, To: to}}, nil
}

// CompletedSet 返回已完成章节集合（空进度返回空 map 而非 nil，便于直接索引）。
func (r *StoryReader) CompletedSet() (map[int]bool, error) {
	progress, err := r.store.Progress.Load()
	if err != nil {
		return nil, err
	}
	set := map[int]bool{}
	if progress != nil {
		for _, n := range progress.CompletedChapters {
			set[n] = true
		}
	}
	return set, nil
}

// Progress 暴露进度（自动导出判定 phase / 边界用）。
func (r *StoryReader) Progress() (*domain.Progress, error) {
	return r.store.Progress.Load()
}

// ArcBoundaryOf 透传弧/卷边界判定（自动导出 onVolume 触发用）。
func (r *StoryReader) ArcBoundaryOf(chapter int) (*storepkg.ArcBoundary, error) {
	return r.store.Outline.CheckArcBoundary(chapter)
}
