// Định dạng số/giờ/tiền theo vi-VN — 07-copy.md › “Số, đơn vị, viết tắt”.
// Phẩy cho phần thập phân, chấm cho hàng nghìn: '3,5 triệu token' · '$2,05' · '58.699'.
// Mọi màn gọi hàm ở đây; không tự ghép chuỗi (21-dev-prompt.md › Gói J mục 7).

const nf = new Intl.NumberFormat('vi-VN')
const dec = (n: number): string => String(n).replace('.', ',')
/** Bỏ đuôi ',0'/'00' của toFixed: '12,0' → '12', '0,10' → '0,1'. */
const trim = (s: string): string => s.replace(/\.?0+$/, '')

/**
 * Thời lượng (mili giây) thành lời: '0,1 giây' · '15,6 giây' · '1 phút 39 giây' · '1 giờ 2 phút'.
 * ≤ 0 → '' (giữ hợp đồng cũ của workspace/format).
 */
export function formatDuration(ms: number | null | undefined): string {
  const v = ms ?? 0
  if (v <= 0) return ''
  if (v < 1000) { const s = v / 1000; return `${(s < 0.1 ? s.toFixed(2) : s.toFixed(1)).replace('.', ',')} giây` }
  if (v < 60_000) return `${dec(Math.round(v / 100) / 10)} giây`
  const total = Math.round(v / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  if (minutes < 60) return `${minutes} phút${seconds ? ` ${seconds} giây` : ''}`
  const hours = Math.floor(minutes / 60)
  return `${hours} giờ${minutes % 60 ? ` ${minutes % 60} phút` : ''}`
}

/** Đồng hồ đếm lên '0:49' · '12:05' · '1:02:03' — font mono + tabular-nums do người gọi lo. */
export function formatTimer(ms: number | null | undefined): string {
  const total = Math.max(0, Math.floor((ms ?? 0) / 1000))
  const s = String(total % 60).padStart(2, '0')
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return `${minutes}:${s}`
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, '0')}:${s}`
}

/**
 * Token: `long` → '3,5 triệu token' · '488,7 nghìn token'; `short` (thanh trạng thái)
 * → '4,0 tr' · '87,0K'. Dưới 1000 → số nguyên thường.
 */
export function formatTokens(n: number | null | undefined, style: 'long' | 'short' = 'long'): string {
  const v = n ?? 0
  if (v >= 1e6) return `${style === 'short' ? (v / 1e6).toFixed(1) : trim((v / 1e6).toFixed(1))} ${style === 'short' ? 'tr' : 'triệu token'}`.replace('.', ',')
  if (v >= 1e3) return `${style === 'short' ? (v / 1e3).toFixed(1) : trim((v / 1e3).toFixed(1))}${style === 'short' ? 'K' : ' nghìn token'}`.replace('.', ',')
  return String(v)
}

/** Số chữ/số đếm vi-VN: 58699 → '58.699'. */
export const formatNumber = (n: number | null | undefined): string => nf.format(n ?? 0)

/** Tiền USD: '$2,05'. */
export const formatUSD = (n: number | null | undefined): string => `$${(n ?? 0).toFixed(2).replace('.', ',')}`

/** 'Chương 4' — không viết tắt 'Ch.'. */
export const chapterLabel = (n: number): string => `Chương ${n}`
