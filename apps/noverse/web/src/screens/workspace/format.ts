// Định dạng chỉ-dùng-ở-Workspace. Số/giờ/tiền vi-VN đã chuyển sang src/lib/format.ts
// (07-copy.md › “Số, đơn vị, viết tắt”) — đừng viết lại hàm trùng ở đây.
import { formatDuration } from '../../lib/format'

/**
 * `Date#toLocaleTimeString(locale, opts)` dựng một `Intl.DateTimeFormat` mới ở **mỗi** lần gọi
 * (~1 ms trên máy thật). Nhật ký 40 dòng gọi hàm này mỗi giây (`useNow`) nên nó từng chiếm
 * ~140 ms mỗi lần mở Bàn viết — đủ để mọi cú bấm điều hướng bị nghẽn. Dựng formatter một lần,
 * nhớ kết quả theo chuỗi ISO (chỉ vài trăm giá trị khác nhau trong một phiên).
 */
const CLOCK = (() => {
  try {
    return new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  } catch {
    return null
  }
})()

const CLOCK_MEMO = new Map<string, string>()
const CLOCK_MEMO_MAX = 600

export function formatClock(iso: string | null | undefined): string {
  if (!iso) return '—'
  const hit = CLOCK_MEMO.get(iso)
  if (hit !== undefined) return hit
  const date = new Date(iso)
  let out = '—'
  if (!Number.isNaN(date.getTime())) {
    try {
      out = CLOCK ? CLOCK.format(date) : date.toLocaleTimeString('vi-VN', { hour12: false })
    } catch {
      out = '—'
    }
  }
  if (CLOCK_MEMO.size >= CLOCK_MEMO_MAX) CLOCK_MEMO.clear()
  CLOCK_MEMO.set(iso, out)
  return out
}

export function formatShortClock(iso: string | null | undefined): string {
  const full = formatClock(iso)
  return full === '—' ? full : full.slice(0, 5)
}

/**
 * Thời lượng một thao tác của luồng viết (Gói AS · 21-dev-prompt mục 5): dưới 0,1 giây ghi
 * “<0,1 giây”; 0 hoặc không đo được thì trả '' để nơi gọi ẩn hẳn — không bao giờ “0 giây”.
 */
export function formatRunDuration(ms: number | null | undefined): string {
  const value = ms ?? 0
  if (!Number.isFinite(value) || value <= 0) return ''
  if (value < 100) return '<0,1 giây'
  return formatDuration(value)
}

export const percentOf = (part: number | null | undefined, total: number | null | undefined): number =>
  total && total > 0 ? Math.round(((part ?? 0) / total) * 100) : 0

/** Màu theo ngưỡng: ok <70% · warn 70–85% · err ≥85% (context) / ok ≥50 · warn ≥20 (cache). */
export function contextTone(percent: number): string {
  if (percent >= 85) return 'var(--color-err)'
  if (percent >= 70) return 'var(--color-warn)'
  return 'var(--color-ok)'
}

export function cacheTone(percent: number): string {
  if (percent >= 50) return 'var(--color-ok)'
  if (percent >= 20) return 'var(--color-warn)'
  return 'var(--color-err)'
}

/**
 * Số chương ở đầu một dòng “Tóm tắt gần đây” — tab Truyện mở đúng chương đó trong Trình đọc.
 * Máy chủ thật ghi `Chương 3: …` (host.go); fixture cũ còn dạng host `第3章: …`.
 */
export function summaryChapter(summary: string): number | null {
  const match = /(?:Chương|第)\s*(\d+)/.exec(summary ?? '')
  const chapter = match ? Number.parseInt(match[1], 10) : NaN
  return Number.isFinite(chapter) && chapter > 0 ? chapter : null
}
