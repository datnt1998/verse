import type { Config } from '../../../api/types'

/** Bản nháp chưa áp dụng của trang Tuỳ chỉnh truyện (MMore.dc.html · storyset). */
export type StorySetDraft = { style?: string; advance?: 'auto' | 'review'; budget?: number }

export type StorySetValues = { style: string; advance: 'auto' | 'review'; budget: number }

export function storySetValues(effective: Config, draft: StorySetDraft): StorySetValues {
  return {
    style: draft.style ?? effective.style ?? '',
    advance: draft.advance ?? (effective.advance_mode === 'review' ? 'review' : 'auto'),
    budget: draft.budget ?? effective.budget?.book_usd ?? 0,
  }
}

/** Trường nào của bản nháp thật sự khác giá trị đang có hiệu lực. */
export function storySetChangedKeys(effective: Config, draft: StorySetDraft): (keyof StorySetDraft)[] {
  const base = storySetValues(effective, {})
  const keys: (keyof StorySetDraft)[] = []
  if (draft.style !== undefined && draft.style !== base.style) keys.push('style')
  if (draft.advance !== undefined && draft.advance !== base.advance) keys.push('advance')
  if (draft.budget !== undefined && draft.budget !== base.budget) keys.push('budget')
  return keys
}

/** Tài liệu cấp `target` sau khi áp dụng: chỉ ghi các khoá đã đổi, giữ nguyên phần còn lại. */
export function storySetConfig(target: Config, effective: Config, draft: StorySetDraft): Config {
  const next: Config = { ...target }
  for (const key of storySetChangedKeys(effective, draft)) {
    if (key === 'style') next.style = draft.style
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

/** Tên văn phong lưu dạng chữ thường (tệp style) → hiển thị viết hoa chữ đầu như artboard (“Hồi hộp”). */
export function styleLabel(name: string): string {
  return name ? name.charAt(0).toLocaleUpperCase('vi') + name.slice(1) : name
}

/** Một dòng mô tả dưới tên kiểu viết (MMore.dc.html › storyset, `MSTY`); tên tự tạo chưa có mô tả → rỗng. */
const STYLE_DESC: Record<string, string> = {
  default: 'Nhịp hài hoà, thoại tự nhiên',
  romance: 'Cảm xúc tinh tế, nội tâm',
  suspense: 'Cài manh mối, kết chương bỏ ngỏ',
  fantasy: 'Thế giới rộng, hệ thống sức mạnh',
  wuxia: 'Giang hồ, chiêu thức',
}

export function styleDesc(name: string): string {
  return STYLE_DESC[name] ?? ''
}
