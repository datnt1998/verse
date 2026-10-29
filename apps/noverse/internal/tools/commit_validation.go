package tools

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/chapterfacts"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// validateCommitArgs 在创建 PendingCommit 前校验模型提交的完整语义载荷。
// 错误直接返回模型修正；不生成半成品状态，也不猜测缺失值。
func (t *CommitChapterTool) validateCommitArgs(a commitArgs) error {
	if err := chapterfacts.Validate(a.ChapterFacts); err != nil {
		return fmt.Errorf("%v: %w", err, errs.ErrToolArgs)
	}
	if err := t.validateNaoFacts(a); err != nil {
		return err
	}

	if len(a.ForeshadowUpdates) > 0 {
		ledger, err := t.store.World.LoadForeshadowLedger()
		if err != nil {
			return fmt.Errorf("load foreshadow ledger: %w: %w", errs.ErrStoreRead, err)
		}
		// 账本是全书投影，而 Projector 按章序重放章节记录。重写早期章节时账本里
		// 还躺着后续章节才种下的伏笔——放行它们，提交前校验就与重放结论相反，
		// 模型无从修正，返工队列随之锁死。故一律以"本章可见"为准。
		plantedAt := make(map[string]int, len(ledger))
		for _, entry := range ledger {
			plantedAt[entry.ID] = entry.PlantedAt
		}
		declared := make(map[string]struct{}, len(a.ForeshadowUpdates))
		for i, update := range a.ForeshadowUpdates {
			if update.Action == "plant" {
				declared[update.ID] = struct{}{}
				// due_by 必须晚于种植章。这条约束此前只在 UpdateForeshadow 落账时校验，
				// 而那时 PendingCommit 已冻结、终稿与章节记录已落盘——重试回放同一
				// 载荷，模型永远没有修正机会。预检在冻结前拦下，模型可改 due 再提交。
				if update.DueBy != 0 && update.DueBy <= a.Chapter {
					return fmt.Errorf("foreshadow_updates[%d]: due_by của phục bút %q phải sau chương gieo %d, nhận %d: %w",
						i, update.ID, a.Chapter, update.DueBy, errs.ErrToolArgs)
				}
				continue
			}
			if !foreshadowActionNeedsLedger(update.Action) {
				continue
			}
			if _, ok := declared[update.ID]; ok {
				continue
			}
			at, known := plantedAt[update.ID]
			if !known {
				return fmt.Errorf("foreshadow_updates[%d] references unknown id %q: %w", i, update.ID, errs.ErrToolPrecondition)
			}
			if at > a.Chapter {
				return fmt.Errorf("foreshadow_updates[%d] phục bút %q được gieo ở chương %d, không thể %s ở chương %d: %w",
					i, update.ID, at, update.Action, a.Chapter, errs.ErrToolPrecondition)
			}
		}
	}
	return nil
}

// foreshadowActionNeedsLedger 报告该动作是否必须作用于账本里已存在的伏笔：
// plant 自建条目，其余动作都依赖前置 plant 才能成立。
func foreshadowActionNeedsLedger(action string) bool {
	return action == "advance" || action == "resolve" || action == "abandon"
}
