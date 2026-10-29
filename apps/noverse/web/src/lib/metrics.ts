// D6 — chữ của một số đo “Đo bằng máy” (22-brief-2510.md D6, Settings.dc.html › metrics,
// Activity.dc.html › “Đo bằng máy · chương 3”). Máy chủ gửi số trần + đơn vị + chuẩn dạng chữ;
// ở đây chỉ đổi cách viết số và cách đọc chuẩn, không tự đặt ngưỡng.
import type { ChapterMetric, TiredWord } from '../api/types'
import { t } from '../i18n/vi'

/** Số kiểu Việt Nam: 34 → “34”, 2,4 → “2,4” (không bao giờ có dấu chấm thập phân). */
export const viNumber = (value: number): string => String(value).replace('.', ',')

/** Số đo không áp cho chương (mở đầu trùng truyện cũ ngoài chương 1) — artboard ghi “—”. */
export const METRIC_NONE = '—'

/**
 * Số kèm đơn vị: `34 %` → “34%”, `38 tiếng` → “38 tiếng”, `2,4 /1.000 tiếng` → “2,4 / 1.000 tiếng”.
 * D6: không bao giờ hiện điểm số trần — đơn vị luôn đi cùng số; `value` null → “—”.
 */
export function metricValue(metric: Pick<ChapterMetric, 'value' | 'unit'>): string {
  if (metric.value === null) return METRIC_NONE
  const unit = metric.unit.replace(/^\/(\d)/, '/ $1')
  return metric.unit === '%' ? `${viNumber(metric.value)}%` : [viNumber(metric.value), unit].filter(Boolean).join(' ')
}

/**
 * Chuẩn đọc thành lời: “≤ 40” → “tối đa 40”, “40–55%” → “chuẩn 40–55%”, “0” → “chuẩn 0”.
 * Artboard dùng đúng hai cách này (Settings.dc.html `metrics[].std`, Activity.dc.html `m.std`).
 */
export function metricStandard(limit: string): string {
  return limit.startsWith('≤ ') ? `tối đa ${limit.slice(2)}` : `chuẩn ${limit}`
}

/** Nhãn dòng thêm của D6: “Từ mệt nhiều nhất · “dường như””. */
export const tiredWordName = t.metrics.tiredWordName

/** Mật độ từ lặp: Settings ghi “2,4 / 1.000”, Nhật ký ghi “3,1 / 1.000 tiếng” (artboard). */
export function tiredWordValue(tired: Pick<TiredWord, 'per1000'>, withUnit = false): string {
  return `${viNumber(tired.per1000)} / 1.000${withUnit ? ' tiếng' : ''}`
}

/** Ngưỡng từ lặp là số (`limit: 1.5`) → “≤ 1,5” như artboard. */
export const tiredWordLimit = (tired: Pick<TiredWord, 'limit'>): string => `≤ ${viNumber(tired.limit)}`
