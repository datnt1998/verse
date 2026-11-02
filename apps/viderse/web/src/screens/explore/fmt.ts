// Định dạng riêng của Khám phá: điểm ×, lượt xem (1,2 Tr / 944 N), tuổi video
// (2 ngày · 3 tuần · dd/mm), người theo dõi và thời lượng mono. Mẫu số liệu
// theo artboard Explore: "1,2 Tr lượt xem · 2 ngày", "×4,2", "1,24 Tr người theo dõi".

const num = new Intl.NumberFormat('vi-VN')
const num1 = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 })

/** Điểm nổi trội: "×4,2"; null khi chưa đủ dữ liệu (lượt xem/mức thường). */
export const fmtScore = (score: number | null | undefined): string =>
  score == null ? '×—' : `×${num1.format(score)}`

/** "4,2" không tiền tố — dùng trong câu/title. */
export const scoreText = (score: number | null | undefined): string =>
  score == null ? '—' : num1.format(score)

/** Lượt xem: ≥1 Tr → "1,2 Tr"; ≥10 N → "944 N"; còn lại số thường. */
export function fmtViews(count: number | null | undefined): string {
  if (count == null) return '—'
  if (count >= 1_000_000) return `${num1.format(count / 1_000_000)} Tr`
  if (count >= 10_000) return `${num.format(Math.round(count / 1_000))} N`
  return num.format(count)
}

/** Người theo dõi của kênh: "1,24 Tr" (hai số lẻ), "286 N", số thường. */
export function fmtSubs(count: number | null | undefined): string {
  if (count == null) return '—'
  if (count >= 1_000_000) {
    const s = (Math.round(count / 10_000) / 100).toLocaleString('vi-VN', { maximumFractionDigits: 2 })
    return `${s} Tr`
  }
  if (count >= 1_000) return `${num.format(Math.round(count / 1_000))} N`
  return num.format(count)
}

/** Tuổi video: "12 giờ" · "2 ngày" · "3 tuần" · "26/10" khi quá 13 tuần; rỗng khi không có giờ. */
export function fmtAge(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return ''
  const at = new Date(iso).getTime()
  if (Number.isNaN(at)) return ''
  const hours = (now - at) / 3_600_000
  if (hours < 1) return 'mới đăng'
  if (hours < 24) return `${Math.floor(hours)} giờ`
  const days = Math.floor(hours / 24)
  if (days < 14) return `${days} ngày`
  const weeks = Math.floor(days / 7)
  if (weeks < 13) return `${weeks} tuần`
  const d = new Date(at)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
}

/** "dd/mm HH:MM" — mốc đăng trong popover Vì sao. */
export function fmtDateTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}`
}

/** Thời lượng mono: "39:51" · "1:02:03"; rỗng khi YouTube không ghi. */
export const fmtDuration = (sec: number | null | undefined): string => {
  if (sec == null) return ''
  const s = Math.max(0, Math.floor(sec))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const r = s % 60
  const mm = h > 0 ? String(m).padStart(2, '0') : String(m)
  return `${h > 0 ? `${h}:` : ''}${mm}:${String(r).padStart(2, '0')}`
}

/** Dòng meta dưới tiêu đề: "1,2 Tr lượt xem · 2 ngày". */
export function videoMeta(viewCount: number | null, publishedAt: string | null): string {
  const views = viewCount == null ? '' : `${fmtViews(viewCount)} lượt xem`
  const age = fmtAge(publishedAt)
  return [views, age].filter(Boolean).join(' · ')
}

/** Mốc chương dạng mm:ss (chính xác tới giây từ source.info.json). */
export function fmtChapterTime(sec: number): string {
  const total = Math.max(0, Math.floor(sec))
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`
}

/** Link tua tới mốc chương trên trình phát YouTube có sẵn (không dựng player riêng). */
export function chapterHref(url: string, startSec: number): string {
  const at = Math.max(0, Math.floor(startSec))
  if (at === 0) return url
  return `${url}${url.includes('?') ? '&' : '?'}t=${at}s`
}
