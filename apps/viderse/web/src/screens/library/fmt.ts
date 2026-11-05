// Định dạng số/giờ/độ dài của Thư viện (06-copy §Số): giờ 24h, ngày 26/10,
// thời lượng mono, số kiểu Việt.
const number = new Intl.NumberFormat('vi-VN')
const two = (n: number) => String(n).padStart(2, '0')

export const num = (value: number) => number.format(value)

export const usd = (value: number) => value.toFixed(2)

/** `23:52` theo giờ máy — giờ 24h, không phụ thuộc locale của trình duyệt. */
export function clock(iso: string | null | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${two(date.getHours())}:${two(date.getMinutes())}`
}

/** `25/10` */
export function dayMonth(iso: string | null | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${two(date.getDate())}/${two(date.getMonth() + 1)}`
}

const sameDay = (a: Date, b: Date) =>
  a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()

/** "sửa 23:52" nếu hôm nay, không thì "sửa 25/10". */
export function editedLabel(iso: string | null | undefined, format: { today: (time: string) => string; date: (date: string) => string }): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return sameDay(date, new Date()) ? format.today(clock(iso)) : format.date(dayMonth(iso))
}

/** `1:30` / `2:05:00` — thời lượng mono của overlay và clip. */
export function duration(sec: number | null | undefined): string {
  if (sec === null || sec === undefined || !Number.isFinite(sec)) return ''
  const total = Math.max(0, Math.round(sec))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return h > 0 ? `${h}:${two(m)}:${two(s)}` : `${m}:${two(s)}`
}

/** `214 MB` / `1,2 GB` theo 1024. */
export function bytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return ''
  if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(1).replace('.', ',')} GB`
  return `${Math.round(value / 1024 ** 2)} MB`
}

/** Chữ cái đầu cho avatar chữ của kênh (tối đa 2 ký tự). */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return (words[0][0] + words[1][0]).toUpperCase()
}
