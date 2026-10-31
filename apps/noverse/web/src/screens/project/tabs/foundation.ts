import type { WorldRule } from '../../../api/client'
export type FoundationRow = { heading: string; text: string; missing: boolean }
const HEADING_ORDER = ['Thể loại và giọng điệu', 'Định vị thể loại', 'Xung đột cốt lõi', 'Mục tiêu nhân vật chính', 'Hướng kết cục', 'Vùng cấm sáng tác', 'Điểm bán hàng khác biệt', 'Móc câu khác biệt', 'Cam kết cốt lõi', 'Động cơ câu chuyện', 'Tuyến quan hệ/trưởng thành', 'Lộ trình nâng cấp', 'Chuyển hướng trung kỳ', 'Mệnh đề kết cục', 'Tính phù hợp với truyện ngắn']
/** The server decides which headings are required for the actual planning tier. */
export function premiseRows(sections: { heading: string; text: string }[], missing: string[]): FoundationRow[] {
  const text = new Map(sections.map((section) => [section.heading, section.text]))
  const headings = [...new Set([...sections.map((section) => section.heading), ...missing])]
  const position = (heading: string) => { const index = HEADING_ORDER.indexOf(heading); return index < 0 ? HEADING_ORDER.length : index }
  headings.sort((a, b) => position(a) - position(b))
  return headings.map((heading) => ({ heading, text: text.get(heading) ?? '', missing: missing.includes(heading) }))
}
export const WORLD_CATEGORIES = [
  { category: 'society', label: 'Xã hội', icon: 'profile-2user' },
  { category: 'technology', label: 'Công nghệ', icon: 'cpu' },
  { category: 'geography', label: 'Địa lý', icon: 'global' },
  { category: 'magic', label: 'Phép thuật', icon: 'magic-star' },
  { category: 'other', label: 'Khác', icon: 'layer' },
] as const
export function groupWorldRules(rules: WorldRule[]) {
  return WORLD_CATEGORIES.map((group) => ({ ...group, rules: rules.filter((rule) => (WORLD_CATEGORIES.some((item) => item.category === rule.category) ? rule.category : 'other') === group.category) })).filter((group) => group.rules.length > 0)
}
