// Logic thuần của bản đồ giữ chân (Outline.dc.html › tab outline · omap=map, D5).
// Nhãn và màu chép từ artboard: chip móc Master §7, chip trả thưởng, câu tổng kết cạnh nút chuyển.
import type { Frame, FrameAudience, Retention, RetentionCheck, RetentionHookType, RetentionPayoffTag } from '../../../api/types'
import { t } from '../../../i18n/vi'

/** `.chip` của artboard: nền + chữ. */
export type Tone = { background: string; color: string }

/** HOOK của artboard — đúng bốn kiểu móc Master §7 + `seed` chương cuối (Câu 19). */
export const HOOK_TONE: Record<RetentionHookType, Tone> = {
  info: { background: 'var(--frame-info-12)', color: 'var(--frame-info-fg)' },
  threat: { background: 'var(--tint-err)', color: 'var(--kit-err-soft)' },
  decision: { background: 'var(--kit-acc-12)', color: 'var(--kit-badge-run)' },
  line: { background: 'var(--kit-warn-12)', color: 'var(--kit-warn-soft)' },
  // Chương cuối: chip “Hạt giống phần hai” thay cho “—” nên giữ nguyên tông của ô “—” (Câu 19).
  seed: { background: 'var(--kit-wash-5)', color: 'var(--kit-fg-icon)' },
}
/** Chương không có móc (artboard: chương cuối “—” trước khi có kiểu `seed`). */
export const HOOK_NONE: Tone = { background: 'var(--kit-wash-5)', color: 'var(--kit-fg-icon)' }

/** Nhãn tiếng Việt của bốn kiểu móc (Master §7) và `seed` của chương cuối. */
export const HOOK_LABEL: Record<RetentionHookType, string> = {
  info: 'Lật thông tin',
  threat: 'Đe dọa mới',
  decision: 'Quyết định treo',
  line: 'Câu thoại nổ',
  seed: t.retention.seed,
}

/** Nhãn chip “Trả thưởng · cú lật” — ba mốc của Master §21. */
export const PAYOFF_LABEL: Record<RetentionPayoffTag, string> = {
  promise: 'Cảnh lời hứa',
  midpoint: 'Cú lật giữa truyện',
  climax: 'Cao trào',
}

/** Thẻ “Khung truyện”: nhãn dòng, mục luật và khoá của từng dòng. */
export const FRAME_ROWS = [
  { key: 'audience', label: 'Người nghe', ref: 'Master §1.2' },
  { key: 'difference', label: 'Điểm khác', ref: 'Master §17' },
  { key: 'sequelSeed', label: 'Hạt giống phần hai', ref: 'Master §19' },
] as const

export type FrameRowKey = typeof FRAME_ROWS[number]['key']

/**
 * Dòng tổng kết cạnh nút chuyển: “8 chương · 2 cảnh báo · đúng vị trí lời hứa và cú lật”.
 * Vế cuối chỉ hiện khi cả §3 và §8 đạt — ngoài vùng thì hai dòng kiểm đã nói rõ lệch ở đâu.
 */
export function mapSummary(retention: Retention): string {
  const warnings = retention.checks.filter((check) => !check.ok).length
  const inZone = (ref: RetentionCheck['ref']) => retention.checks.find((check) => check.ref === ref)?.ok === true
  const head = `${retention.chapters.length} chương · ${warnings} cảnh báo`
  return inZone('Master §3') && inZone('Master §8') ? `${head} · đúng vị trí lời hứa và cú lật` : head
}

/**
 * “Muốn …” / “Họ muốn …” nối sau từ của gói thể loại: hạ chữ đầu để câu đọc liền
 * (gói ghi “Được làm người thông minh…”, artboard ghi “Muốn được làm…”).
 */
export function wantsSentence(wants: string): string {
  const text = wants.trim()
  return text ? text.charAt(0).toLocaleLowerCase('vi') + text.slice(1) : ''
}

/**
 * Chip đầu thẻ Khung truyện: ai điền ba dòng này. Artboard chỉ vẽ trạng thái gói thể loại
 * (“Gói Nữ tần báo thù điền sẵn”); hai nguồn còn lại của `source` dùng cùng khuôn câu.
 */
export function frameSourceChip(source: 'pack' | 'architect' | 'user', packName: string | null): string {
  if (source === 'user') return 'Bạn đã sửa'
  if (source === 'architect') return 'Kiến trúc sư điền sẵn'
  return packName ? `Gói ${packName} điền sẵn` : 'Gói thể loại điền sẵn'
}

/** Hai mảng của GET …/frame mà BE cũ có thể gửi null (gói không khai nhóm người nghe). */
type FramePick = { audience?: readonly number[] | null; audiences?: readonly FrameAudience[] | null }

/**
 * “Nam trung niên · Nữ 18–35” — nhãn các nhóm người nghe đã chọn, đúng thứ tự trong
 * `audience` (Câu 18: gói C mặc định `[2, 1]`; phần tử là chỉ số trong `audiences[]`).
 * Chỉ số ngoài bảng của gói thì bỏ.
 */
export function audienceLabels(frame: FramePick): string {
  const groups = frame.audiences ?? []
  return (frame.audience ?? [])
    .map((index) => groups[index]?.label ?? '')
    .filter((label) => label !== '')
    .join(' · ')
}

/** Vế “Họ muốn …” của các nhóm đã chọn; nhiều nhóm thì nối bằng “ · ” như nhãn. */
export function audienceWants(frame: FramePick): string {
  const groups = frame.audiences ?? []
  return (frame.audience ?? [])
    .map((index) => groups[index])
    .filter((group): group is FrameAudience => !!group)
    .map((group) => wantsSentence(group.wants))
    .filter((wants) => wants !== '')
    .join(' · ')
}

/** Dữ liệu cũ có thể chép lời giải thích ngữ cảnh vào ô Điểm khác. */
export function storyDifference(value: string): string {
  return value.trim().split(/(?<=\.) /)
    .filter((sentence) => !sentence.includes('other_stories') && !sentence.includes('foundation_memory'))
    .join(' ').trim()
}
