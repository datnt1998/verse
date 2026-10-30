// Thẻ "Quét lịch sử" — ánh xạ ScanStatus (hợp đồng /api/rivals/:id/scan) sang
// câu và nút theo bảng 06-copy › Khám phá: bốn trạng thái có artboard (running ·
// paused · limited · capped) cộng queued · complete · idle · cancelled · error ·
// đang kiểm tra video mới. Hàm thuần để test theo bảng.
import { vi as t } from '../../i18n/vi'
import { fill } from '../settings/fmt'
import type { ScanAction, ScanStatus } from '../../api/rivals'

export type ScanButton = { action: ScanAction; label: string; icon: 'pause' | 'refresh' | 'video-add' }

export type ScanView = {
  /** "1.284 / 2.000" */
  count: string
  /** 0..1 cho thanh tiến độ */
  pct: number
  /** video đã lập chỉ mục / giới hạn — cho aria-valuemax/now */
  now: number
  max: number
  text: string
  live: boolean
  button: ScanButton | null
}

const num = new Intl.NumberFormat('vi-VN')

const fmtHour = (iso: string | null): string | null => {
  if (!iso) return null
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? null : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

/**
 * history.state → giao diện thẻ. Kiểm tra video mới (discovery.running) chỉ lấn
 * át khi lịch sử đứng yên mà câu của nó không nói gì về video mới (idle,
 * complete, cancelled); lúc đó thẻ không có nút như bảng câu.
 */
export function scanView(s: ScanStatus): ScanView {
  const tt = t.screens.explore.scanStates
  const bt = t.screens.explore.scanButtons
  const every = t.screens.explore.everyHours(s.rescanHours)
  const max = s.historyCap
  const now = Math.min(s.indexedVideos, max)
  // Số đã quét không vượt giới hạn đang đặt (hạ giới hạn sau khi quét xong).
  const count = `${num.format(now)} / ${num.format(max)}`
  const pct = max > 0 ? Math.min(1, s.indexedVideos / max) : 0
  const base = { count, pct, now, max }
  const discovery = { ...base, text: tt.discovery, live: true, button: null }

  switch (s.history.state) {
    case 'running':
      return { ...base, text: tt.running, live: true, button: { action: 'pause', label: bt.pause, icon: 'pause' } }
    case 'queued':
      return { ...base, text: tt.queued, live: true, button: null }
    case 'backoff': {
      const at = fmtHour(s.history.nextAttemptAt)
      return { ...base, text: at ? fill(tt.limited, { time: at }) : tt.limitedNoTime, live: false, button: { action: 'pause', label: bt.pause, icon: 'pause' } }
    }
    case 'paused':
      return { ...base, text: tt.paused, live: false, button: { action: 'resume', label: bt.resume, icon: 'refresh' } }
    case 'error':
      return { ...base, text: tt.error, live: false, button: { action: 'resume', label: bt.resume, icon: 'refresh' } }
    case 'capped':
      return { ...base, text: tt.capped(num.format(now), every), live: s.discovery.running, button: { action: 'start', label: bt.rescan, icon: 'refresh' } }
    case 'complete':
      return s.discovery.running ? discovery : { ...base, text: tt.complete(num.format(s.indexedVideos), every), live: false, button: null }
    case 'cancelled':
      return s.discovery.running ? discovery : { ...base, text: tt.cancelled, live: false, button: { action: 'start', label: bt.rescan, icon: 'refresh' } }
    case 'idle':
    default:
      return s.discovery.running ? discovery : { ...base, text: tt.idle, live: false, button: { action: 'start', label: bt.start, icon: 'video-add' } }
  }
}
