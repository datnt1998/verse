// Định dạng riêng của mảng Video nền: màu nền tảng, nhãn kind, tuổi video,
// đoạn m:ss. Số/giờ/ngày đi qua fmt.ts của Thư viện (cùng chuẩn 06-copy).
import { t } from '../../../i18n/library'
import { bytes, clock, duration, editedLabel, initials, num } from '../fmt'

export { bytes, clock, duration, initials, num }

/** Tuổi video "2 giờ trước" / "3 ngày trước" (06-copy §Thời gian tương đối). */
export function ageLabel(iso: string | null | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const age = Math.max(0, Date.now() - date.getTime())
  const c = t.backdrop.tab.channel
  const minutes = Math.floor(age / 60_000)
  if (minutes < 1) return c.ageJust
  if (minutes < 60) return c.ageMinutes(num(minutes))
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return c.ageHours(num(hours))
  const days = Math.floor(hours / 24)
  if (days < 7) return c.ageDays(num(days))
  if (days < 31) return c.ageWeeks(num(Math.floor(days / 7)))
  return c.ageMonths(num(Math.floor(days / 30)))
}

/** Nhãn trạng thái "Cập nhật hôm nay 23:10" / "Cập nhật 25/10" — khớp artboard. */
export function updatedLabel(iso: string): string {
  const c = t.backdrop.tab.channel
  return editedLabel(iso, { today: c.updatedToday, date: c.updatedAt })
}
/** `m:ss` (dưới 1 giờ) / `h:mm:ss` — như duration() nhưng làm tròn cận trên
 *  giây (đoạn 89,2 s hiện 1:30). */
export function timeRange(sec: number | null | undefined): string {
  if (sec === null || sec === undefined || !Number.isFinite(sec)) return '—'
  const total = Math.max(0, Math.ceil(sec))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const two = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${h}:${two(m)}:${two(s)}` : `${m}:${two(s)}`
}

/** Đoạn "0:00–1:30" của video lẻ / dialog. */
export function segmentLabel(fromSec: number, toSec: number): string {
  return t.backdrop.tab.videos.segmentRange(timeRange(fromSec), timeRange(toSec))
}

/** Màu đại diện nền tảng (token --vv-plat-*, artboard Twitch tím / Bilibili xanh). */
export function platformColor(platform: string): string {
  return `var(--vv-plat-${platform === 'bilibili' ? 'bilibili' : 'twitch'})`
}

export function platformName(platform: string): string {
  return t.backdrop.tab.platformName[platform] ?? platform
}

export function kindName(kind: string): string {
  return t.backdrop.tab.kindName[kind] ?? kind
}
