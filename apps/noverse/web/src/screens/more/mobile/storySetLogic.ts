import type { Config } from '../../../api/types'

/** Bản nháp chưa áp dụng của trang Tuỳ chỉnh truyện (MMore.dc.html · storyset). */
export type StorySetDraft = { advance?: 'auto' | 'review'; budget?: number }

export type StorySetValues = { advance: 'auto' | 'review'; budget: number }

export function storySetValues(effective: Config, draft: StorySetDraft): StorySetValues {
  return {
    advance: draft.advance ?? (effective.advance_mode === 'review' ? 'review' : 'auto'),
    budget: draft.budget ?? effective.budget?.book_usd ?? 0,
  }
}

/** Trường nào của bản nháp thật sự khác giá trị đang có hiệu lực. */
export function storySetChangedKeys(effective: Config, draft: StorySetDraft): (keyof StorySetDraft)[] {
  const base = storySetValues(effective, {})
  const keys: (keyof StorySetDraft)[] = []
  if (draft.advance !== undefined && draft.advance !== base.advance) keys.push('advance')
  if (draft.budget !== undefined && draft.budget !== base.budget) keys.push('budget')
  return keys
}

/** Tài liệu cấp `target` sau khi áp dụng: chỉ ghi các khoá đã đổi, giữ nguyên phần còn lại. */
export function storySetConfig(target: Config, effective: Config, draft: StorySetDraft): Config {
  const next: Config = { ...target }
  for (const key of storySetChangedKeys(effective, draft)) {
    if (key === 'advance') next.advance_mode = draft.advance
    if (key === 'budget') next.budget = { ...target.budget, book_usd: draft.budget }
  }
  return next
}

/** Nút ± ngân sách: bước 1 USD, tối thiểu 1 (đúng artboard). */
export function stepBudget(value: number, direction: 1 | -1): number {
  return Math.max(1, Math.round((value + direction) * 100) / 100)
}

export function usd(value: number): string {
  return `$${value.toLocaleString('vi-VN', { minimumFractionDigits: Number.isInteger(value) ? 0 : 2, maximumFractionDigits: 2 })}`
}

export function budgetPercent(spent: number, limit: number): string {
  return `${limit > 0 ? Math.min(100, Math.round((spent / limit) * 100)) : 0}%`
}
