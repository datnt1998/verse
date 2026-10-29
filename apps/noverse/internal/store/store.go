package store

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// Store 是状态管理的组合根与业务外观（facade），持有后端提供的领域契约。
// 字段是接口类型而非具体子存储：file backend 注入 file*Store 实现，postgres
// backend 在 04–05 阶段注入事务化实现，二者共享同一组 contract。
type Store struct {
	backend Backend

	Progress       ProgressStore
	Book           BookStore
	Outline        OutlineStore
	Drafts         DraftStore
	Summaries      SummaryStore
	RunMeta        RunMetaStore
	UserRules      UserRulesStore
	Signals        SignalStore
	Runtime        RuntimeStore
	Characters     CharacterStore
	Cast           CastStore
	World          WorldStore
	Checkpoints    CheckpointStore
	Sessions       SessionStore
	Usage          UsageStore
	Simulation     SimulationStore
	Decisions      DecisionStore
	ChapterRecords ChapterRecordStore
	Revisions      RevisionStore
}

const (
	projectFormatPath           = "meta/project_format.json"
	LegacyProjectFormatVersion  = 1
	CurrentProjectFormatVersion = 2
)

type projectFormat struct {
	Version int `json:"version"`
}

// NewStore 创建 file backend 状态管理器，dir 为小说输出根目录。
// 这是 CLI/离线真实构造器，不是 deprecated alias；hosted postgres 根由
// bootstrap.StorageConfig + NewFromBackend 走独立路径。
func NewStore(dir string) *Store {
	s, err := NewFromBackend(newFileBackend(dir))
	if err != nil {
		// fileBackend supplies every domain by construction; the capability
		// gate can only fail on a code bug, which should be loud.
		panic(fmt.Sprintf("file backend rejected by capability gate: %v", err))
	}
	return s
}

// NewFromBackend 把已选好的 backend 包装成业务外观。backend 必须能提供全部
// 19 个 domain substore；能力缺口返回 *CapabilityError，由组合根在启动期
// 失败，而不是在首个请求时 panic——postgres 域名在 04/05 阶段逐步补齐。
func NewFromBackend(b Backend) (*Store, error) {
	if b == nil {
		return nil, &CapabilityError{Backend: "nil"}
	}
	domains := b.Substores()
	if missing := domains.missingDomains(); len(missing) > 0 {
		return nil, &CapabilityError{Backend: b.Kind(), Domains: missing}
	}
	return &Store{
		backend:        b,
		Progress:       domains.Progress,
		Book:           domains.Book,
		Outline:        domains.Outline,
		Drafts:         domains.Drafts,
		Summaries:      domains.Summaries,
		RunMeta:        domains.RunMeta,
		UserRules:      domains.UserRules,
		Signals:        domains.Signals,
		Runtime:        domains.Runtime,
		Characters:     domains.Characters,
		Cast:           domains.Cast,
		World:          domains.World,
		Checkpoints:    domains.Checkpoints,
		Sessions:       domains.Sessions,
		Usage:          domains.Usage,
		Simulation:     domains.Simulation,
		Decisions:      domains.Decisions,
		ChapterRecords: domains.ChapterRecords,
		Revisions:      domains.Revisions,
	}, nil
}

// BackendKind 报告外观背后的存储驱动，供诊断与能力门控使用。
func (s *Store) BackendKind() BackendKind { return s.backend.Kind() }

// Dir 返回文件后端的输出根目录；无文件系统的后端返回 ""。
// 需要真实目录的调用方必须显式声明 file 能力，不得向假路径写数据。
func (s *Store) Dir() string { return s.backend.Dir() }

// Docs 返回引擎相对的文档面（file: 目录下文件；postgres: 有类型的行）。
// 调用方只能用它读写已知文档名，不得把结果路径交给 os 包。
func (s *Store) Docs() *IO { return s.backend.Docs() }

// Comments 返回该后端绑定的 góp ý store；backend 未提供时返回 nil，调用方
// 必须显式处理而不是退回目录写入。
func (s *Store) Comments() *comments.Store { return s.backend.Comments() }

// FileRoot reports the engine directory together with whether this store is
// really file-backed. Hosted stores answer ("", false): callers must not
// construct a path from "" or fall back to the process working directory.
func (s *Store) FileRoot() (string, bool) {
	dir := s.backend.Dir()
	return dir, dir != ""
}

// LatestReview returns the newest review for the story. Backends that do not
// provide a review surface fail with the capability error instead of scanning.
func (s *Store) LatestReview() (*domain.ReviewEntry, error) {
	lister, ok := s.backend.(ReviewLister)
	if !ok {
		return nil, &CapabilityError{Backend: s.backend.Kind(), Op: "LatestReview"}
	}
	return lister.LatestReview()
}

// PlanChapters returns the chapters that have a plan document.
func (s *Store) PlanChapters() ([]int, error) {
	lister, ok := s.backend.(ChapterMarkerLister)
	if !ok {
		return nil, &CapabilityError{Backend: s.backend.Kind(), Op: "PlanChapters"}
	}
	return lister.PlanChapters()
}

// ReviewChapters returns the chapters that have a review.
func (s *Store) ReviewChapters() ([]int, error) {
	lister, ok := s.backend.(ChapterMarkerLister)
	if !ok {
		return nil, &CapabilityError{Backend: s.backend.Kind(), Op: "ReviewChapters"}
	}
	return lister.ReviewChapters()
}

// Close 释放后端资源（文件后端为空操作）。
func (s *Store) Close() error { return s.backend.Close() }

// LoadProjectFormatVersion 返回作品目录的数据格式版本。旧作品没有版本文件，
// 视为 v1，由启动迁移统一升级，业务代码无需保留旧格式分支。
func (s *Store) LoadProjectFormatVersion() (int, error) {
	return s.backend.LoadProjectFormatVersion()
}

// SaveProjectFormatVersion 在一次迁移全部完成后原子更新项目格式版本。
func (s *Store) SaveProjectFormatVersion(version int) error {
	return s.backend.SaveProjectFormatVersion(version)
}

// CheckConsistency 对事实层做一次浅层校验，用于启动/恢复时生成 warning。
// 纯只读：不修正数据，仅返回可读的问题描述。调用方决定如何展示（log / UI）。
// 为避免扫全目录带来的 IO 开销，只校验 Progress 的关键点：
//   - 最后一个完成章节必须在 chapters/ 下存在终稿
//   - Layered 模式下，当前 Volume/Arc 必须能在 layered_outline 中找到
func (s *Store) CheckConsistency() []string {
	var warnings []string
	progress, err := s.Progress.Load()
	if err != nil {
		return append(warnings, fmt.Sprintf("Đọc progress thất bại: %v", err))
	}
	if progress == nil {
		return warnings
	}
	if n := len(progress.CompletedChapters); n > 0 {
		lastCh := progress.CompletedChapters[n-1]
		if text, err := s.Drafts.LoadChapterText(lastCh); err != nil {
			warnings = append(warnings, fmt.Sprintf("Đọc bản cuối chương %d thất bại: %v", lastCh, err))
		} else if text == "" {
			warnings = append(warnings, fmt.Sprintf("progress đánh dấu chương %d đã hoàn thành nhưng chapters/%02d.md không tồn tại hoặc rỗng", lastCh, lastCh))
		}
	}
	if progress.Layered && progress.CurrentVolume > 0 && progress.CurrentArc > 0 {
		volumes, err := s.Outline.LoadLayeredOutline()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Đọc đại cương phân tầng thất bại: %v", err))
		} else if len(volumes) > 0 {
			found := false
			for _, v := range volumes {
				if v.Index != progress.CurrentVolume {
					continue
				}
				for _, a := range v.Arcs {
					if a.Index == progress.CurrentArc {
						found = true
						break
					}
				}
				break
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("progress đang ở V%d A%d nhưng không tìm thấy mục tương ứng trong đại cương phân tầng", progress.CurrentVolume, progress.CurrentArc))
			}
		}
	}
	return warnings
}

// FoundationMissing 返回初始规划中尚缺的作品信息与基础设定，顺序稳定。
// 长篇模式（已有 layered_outline）额外要求 compass。读取失败必须原样返回，不能把
// 损坏或无权限读取的工件误判成“尚未创建”，否则调用方可能覆盖真实数据。
func (s *Store) FoundationMissing() ([]string, error) {
	var missing []string
	book, err := s.Book.Load()
	if err != nil {
		return nil, fmt.Errorf("load book metadata: %w", err)
	}
	if book == nil {
		missing = append(missing, "book")
	}
	premise, err := s.Outline.LoadPremise()
	if err != nil {
		return nil, fmt.Errorf("load premise: %w", err)
	}
	if premise == "" {
		missing = append(missing, "premise")
	}
	outline, err := s.Outline.LoadOutline()
	if err != nil {
		return nil, fmt.Errorf("load outline: %w", err)
	}
	if len(outline) == 0 {
		missing = append(missing, "outline")
	}
	characters, err := s.Characters.Load()
	if err != nil {
		return nil, fmt.Errorf("load characters: %w", err)
	}
	if len(characters) == 0 {
		missing = append(missing, "characters")
	}
	rules, err := s.World.LoadWorldRules()
	if err != nil {
		return nil, fmt.Errorf("load world rules: %w", err)
	}
	if len(rules) == 0 {
		missing = append(missing, "world_rules")
	}
	layered, err := s.Outline.LoadLayeredOutline()
	if err != nil {
		return nil, fmt.Errorf("load layered outline: %w", err)
	}
	if len(layered) > 0 {
		compass, err := s.Outline.LoadCompass()
		if err != nil {
			return nil, fmt.Errorf("load compass: %w", err)
		}
		if compass == nil {
			missing = append(missing, "compass")
		}
	}
	// 新书只有经过模型对已落盘工件的显式语义审查，才允许从规划进入写作。
	// PhaseWriting/Complete 代表旧书或已审查的新书，保持历史项目兼容；审查本身
	// 是一个动作而非文件缺失，因此只在其它工件齐全时追加。
	if len(missing) == 0 {
		progress, err := s.Progress.Load()
		if err != nil {
			return nil, fmt.Errorf("load progress: %w", err)
		}
		if progress == nil || (progress.Phase != domain.PhaseWriting && progress.Phase != domain.PhaseComplete) {
			missing = append(missing, "foundation_audit")
		}
	}
	return missing, nil
}

// FoundationFingerprint 返回当前基础设定工件的内容指纹。Architect 必须把
// novel_context 读到的这个值原样交回审查工具，确保结论针对的是实际落盘版本，
// 而不是会话中尚未保存或已经过期的内容。
func (s *Store) FoundationFingerprint() (string, error) {
	return s.backend.FoundationFingerprint()
}

// Init 创建所需的子目录结构。
func (s *Store) Init() error {
	return s.backend.Init()
}

// ── 跨域协调方法（由 backend 实现；file 走有序幂等写，postgres 走事务） ──

// ExpandArc 将骨架弧校准并展开为详细章节（Outline + Progress 联动）。
func (s *Store) ExpandArc(volumeIdx, arcIdx int, expansion domain.ArcExpansion) error {
	return s.backend.ExpandArc(volumeIdx, arcIdx, expansion)
}

// AppendVolume 追加新卷到分层大纲末尾（Outline + Progress 联动）。
func (s *Store) AppendVolume(vol domain.VolumeOutline) error {
	return s.backend.AppendVolume(vol)
}

// ReviseOutline 从 fromChapter 起替换尚未发生的计划尾段。
func (s *Store) ReviseOutline(fromChapter int, replacement []domain.OutlineEntry) (int, error) {
	return s.backend.ReviseOutline(fromChapter, replacement)
}

// ClearHandledSteer 清除 PendingSteer 并重置旧版 FlowSteering 状态。
func (s *Store) ClearHandledSteer() error {
	return s.backend.ClearHandledSteer()
}

// InvalidateChapterAggregates 删除输入范围包含修订章节的模型派生工件。
// 章节级投影由 revision.Projector 单独重建。
func (s *Store) InvalidateChapterAggregates(fromChapter int) error {
	return s.backend.InvalidateChapterAggregates(fromChapter)
}
