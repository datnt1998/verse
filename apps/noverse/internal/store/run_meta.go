package store

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
)

// RunMetaStore 管理运行元信息（模型、干预历史、规划级别等）。
type fileRunMetaStore struct{ io *IO }

func newFileRunMetaStore(io *IO) *fileRunMetaStore { return &fileRunMetaStore{io: io} }

// Save 保存运行元信息到 meta/run.json。
func (s *fileRunMetaStore) Save(meta domain.RunMeta) (err error) {
	defer s.io.lockWrite()(&err)
	return s.saveUnlocked(meta)
}

// Load 读取运行元信息。
func (s *fileRunMetaStore) Load() (*domain.RunMeta, error) {
	s.io.mu.RLock()
	defer s.io.mu.RUnlock()
	return s.loadUnlocked()
}

func (s *fileRunMetaStore) loadUnlocked() (*domain.RunMeta, error) {
	var meta domain.RunMeta
	if err := s.io.ReadJSONUnlocked("meta/run.json", &meta); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}

func (s *fileRunMetaStore) saveUnlocked(meta domain.RunMeta) error {
	return s.io.WriteJSONUnlocked("meta/run.json", meta)
}

// Init 初始化或更新运行元信息;跨重启保留全部运行意图事实——
// PlanStart 尤其关键:规划期(启动裁定已落盘、首个 foundation 未落盘)崩溃后,
// 它是恢复规划师身份的唯一依据,被 Init 覆盖会让恢复直接停机。
func (s *fileRunMetaStore) Init(style, provider, model string) error {
	return s.io.WithWriteLock(func() error {
		existing, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		meta := domain.RunMeta{
			StartedAt: time.Now().Format(time.RFC3339),
			Provider:  provider,
			Style:     style,
			Model:     model,
		}
		if existing != nil {
			meta.PendingSteer = existing.PendingSteer
			meta.PlanningTier = existing.PlanningTier
			meta.PlanStart = existing.PlanStart
			meta.StartPrompt = existing.StartPrompt
			meta.AdvanceMode = existing.AdvanceMode
			meta.AdvancePermitChapter = existing.AdvancePermitChapter
			meta.AdvanceHold = existing.AdvanceHold
			meta.PendingCommentReview = existing.PendingCommentReview
			meta.PendingAsk = existing.PendingAsk
		}
		if meta.AdvanceMode == "" {
			meta.AdvanceMode = domain.ChapterAdvanceAuto
		}
		if err := validateAdvanceControl(meta); err != nil {
			return err
		}
		return s.saveUnlocked(meta)
	})
}

func validateAdvanceControl(meta domain.RunMeta) error {
	if !meta.AdvanceMode.Valid() {
		return &domain.UnsupportedAdvanceModeError{Mode: meta.AdvanceMode}
	}
	if meta.AdvancePermitChapter < 0 {
		return fmt.Errorf("Giấy phép chương không được âm: %d", meta.AdvancePermitChapter)
	}
	if meta.AdvanceMode == domain.ChapterAdvanceAuto && meta.AdvancePermitChapter != 0 {
		return fmt.Errorf("Chế độ auto không được giữ giấy phép chương: %d", meta.AdvancePermitChapter)
	}
	if meta.AdvanceHold != nil {
		if err := meta.AdvanceHold.Validate(); err != nil {
			return err
		}
	}
	if meta.PendingAsk != nil {
		if err := meta.PendingAsk.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SetStartPrompt 固化用户的原始创作需求——输入事实,在启动裁定**之前**落盘。
// 裁定失败(如模型故障)时它仍然在,恢复/继续由引擎据此补裁(engine.planStartFallback),
// 启动失败不再是死局。
func (s *fileRunMetaStore) SetStartPrompt(prompt string) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			meta = &domain.RunMeta{}
		}
		meta.StartPrompt = prompt
		return s.saveUnlocked(*meta)
	})
}

// SetPendingSteer 记录未完成的 Steer 指令。
func (s *fileRunMetaStore) SetPendingSteer(input string) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			meta = &domain.RunMeta{}
		}
		meta.PendingSteer = input
		return s.saveUnlocked(*meta)
	})
}

// ClearPendingSteer 清除已处理的 Steer 指令。
func (s *fileRunMetaStore) ClearPendingSteer() error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.PendingSteer == "" {
			return nil
		}
		meta.PendingSteer = ""
		return s.saveUnlocked(*meta)
	})
}

// SetAdvanceMode 切换章节推进模式。切回 auto 时在同一写锁内清除章节许可。
func (s *fileRunMetaStore) SetAdvanceMode(mode domain.ChapterAdvanceMode) error {
	if !mode.Valid() {
		return &domain.UnsupportedAdvanceModeError{Mode: mode}
	}
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("run meta chưa được khởi tạo")
		}
		meta.AdvanceMode = mode
		if mode == domain.ChapterAdvanceAuto {
			meta.AdvancePermitChapter = 0
		}
		return s.saveUnlocked(*meta)
	})
}

// GrantAdvancePermit 为 review 模式持久化一个精确章节许可。
func (s *fileRunMetaStore) GrantAdvancePermit(chapter int) error {
	if chapter <= 0 {
		return fmt.Errorf("Giấy phép chương phải lớn hơn 0: %d", chapter)
	}
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("run meta chưa được khởi tạo")
		}
		if meta.AdvanceMode != domain.ChapterAdvanceReview {
			return fmt.Errorf("Chỉ chế độ nghiệm thu từng chương mới được cấp phép chương tiếp theo (hiện tại %s)", meta.AdvanceMode)
		}
		if meta.AdvancePermitChapter == chapter {
			return nil
		}
		if meta.AdvancePermitChapter != 0 {
			return fmt.Errorf("Đã có giấy phép chương %d, từ chối ghi đè bằng chương %d", meta.AdvancePermitChapter, chapter)
		}
		meta.AdvancePermitChapter = chapter
		return s.saveUnlocked(*meta)
	})
}

// ClearAdvancePermit 仅消费匹配的章节许可；目标已不存在时幂等。
func (s *fileRunMetaStore) ClearAdvancePermit(chapter int) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.AdvancePermitChapter == 0 {
			return nil
		}
		if meta.AdvancePermitChapter != chapter {
			return fmt.Errorf("Giấy phép chương đã thay đổi: mong đợi chương %d, thực tế chương %d", chapter, meta.AdvancePermitChapter)
		}
		meta.AdvancePermitChapter = 0
		return s.saveUnlocked(*meta)
	})
}

// SetAdvanceHold 登记一次性暂停意图；在途意图不允许被另一条静默覆盖。
func (s *fileRunMetaStore) SetAdvanceHold(hold domain.AdvanceHold) error {
	if err := hold.Validate(); err != nil {
		return err
	}
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("run meta chưa được khởi tạo")
		}
		if meta.AdvanceHold != nil {
			if *meta.AdvanceHold == hold {
				return nil
			}
			return fmt.Errorf("Đã có ý định tạm dừng một lần (%s: %s), từ chối ghi đè", meta.AdvanceHold.After, meta.AdvanceHold.Reason)
		}
		meta.AdvanceHold = &hold
		return s.saveUnlocked(*meta)
	})
}

// ClearAdvanceHold 只消费调用方刚读取的同一个意图；目标已不存在时幂等。
func (s *fileRunMetaStore) ClearAdvanceHold(expected domain.AdvanceHold) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.AdvanceHold == nil {
			return nil
		}
		if *meta.AdvanceHold != expected {
			return fmt.Errorf("Ý định tạm dừng một lần đã thay đổi, từ chối xoá nhầm")
		}
		meta.AdvanceHold = nil
		return s.saveUnlocked(*meta)
	})
}

// SetPendingAsk 登记仲裁等待用户回答的提问；在途提问不允许被另一条静默覆盖
// （相同的提问幂等，不同提问显式报错让调用方先结清旧的）。
func (s *fileRunMetaStore) SetPendingAsk(ask domain.PendingAsk) error {
	if err := ask.Validate(); err != nil {
		return err
	}
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("run meta chưa được khởi tạo")
		}
		if meta.PendingAsk != nil {
			if reflect.DeepEqual(*meta.PendingAsk, ask) {
				return nil
			}
			return fmt.Errorf("Đã có câu hỏi đang chờ trả lời (%s), từ chối ghi đè", meta.PendingAsk.Question)
		}
		meta.PendingAsk = &ask
		return s.saveUnlocked(*meta)
	})
}

// ClearPendingAsk 只结清调用方刚读取的同一个提问；目标已不存在时幂等。
func (s *fileRunMetaStore) ClearPendingAsk(expected domain.PendingAsk) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil || meta.PendingAsk == nil {
			return nil
		}
		if !reflect.DeepEqual(*meta.PendingAsk, expected) {
			return fmt.Errorf("Câu hỏi đang chờ đã thay đổi, từ chối xoá nhầm")
		}
		meta.PendingAsk = nil
		return s.saveUnlocked(*meta)
	})
}

// SetPlanningTier 记录当前作品的规划级别。
func (s *fileRunMetaStore) SetPlanningTier(tier domain.PlanningTier) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			meta = &domain.RunMeta{}
		}
		meta.PlanningTier = tier
		return s.saveUnlocked(*meta)
	})
}

// SetPlanStart 固化启动裁定事实(裁定先落事实再起执行;规划期崩溃恢复据此续跑)。
func (s *fileRunMetaStore) SetPlanStart(rec domain.PlanStartRecord) error {
	return s.io.WithWriteLock(func() error {
		meta, err := s.loadUnlocked()
		if err != nil {
			return err
		}
		if meta == nil {
			meta = &domain.RunMeta{}
		}
		meta.PlanStart = &rec
		return s.saveUnlocked(*meta)
	})
}
