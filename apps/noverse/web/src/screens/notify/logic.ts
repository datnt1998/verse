import type { InboxNotification } from '../../api/types'

/** So sánh ngày lịch tại máy người đọc, không chia mili giây (ngày đổi DST có thể không đủ 24 giờ). */
export function groupNotifications(items: readonly InboxNotification[], now = new Date()) {
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(start); yesterday.setDate(yesterday.getDate() - 1)
  const groups: { label: string; items: InboxNotification[] }[] = []
  const byLabel = new Map<string, InboxNotification[]>()
  for (const item of [...items].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))) {
    const date = new Date(item.createdAt)
    const label = date >= start ? 'Hôm nay' : date >= yesterday ? 'Hôm qua' : date.toLocaleDateString('vi-VN', { day: 'numeric', month: 'long', year: 'numeric' })
    let group = byLabel.get(label)
    if (!group) { group = []; byLabel.set(label, group); groups.push({ label, items: group }) }
    group.push(item)
  }
  return groups
}

export function notificationPath(item: Pick<InboxNotification, 'type' | 'story' | 'chapter' | 'job' | 'url'>): string {
  // Chỉ nhận liên kết cùng nguồn. URL tải tệp API được giữ nguyên, không biến thành route.
  if (item.url?.startsWith('/') && !item.url.startsWith('//')) return item.url
  const story = encodeURIComponent(item.story)
  switch (item.type) {
    case 'gate': return `/read/${story}/${item.chapter ?? 1}?gate=1`
    case 'error': return `/write/${story}`
    case 'job': return `/listen/${story}`
    case 'done': return `/read/${story}/${item.chapter ?? 1}`
    case 'security': return '/more/devices'
    case 'import': case 'sync': case 'cocreate': return item.job ? `/cmd/${encodeURIComponent(item.job)}` : `/write/${story}`
    default: return '/settings/notify'
  }
}

export const notificationMeta: Record<string, { icon: string; color: string; cta: string }> = {
  gate: { icon: 'flag-bulk', color: '#F6CB8C', cta: 'Mở cổng duyệt' },
  error: { icon: 'danger', color: '#FF9FA8', cta: 'Xem & chạy lại' },
  job: { icon: 'headphone-bulk', color: '#C9A6F5', cta: 'Nghe ngay' },
  download: { icon: 'document-download', color: '#C9A6F5', cta: 'Tải tệp' },
  done: { icon: 'book-1', color: '#A6EDC1', cta: 'Đọc chương' },
  security: { icon: 'shield-security-bulk', color: '#9DB0F7', cta: 'Xem thiết bị' },
  import: { icon: 'document-download', color: '#C9A6F5', cta: 'Xem kết quả nhập' },
  sync: { icon: 'refresh-2', color: '#9DB0F7', cta: 'Xem đồng bộ' },
  cocreate: { icon: 'messages-2', color: '#9DB0F7', cta: 'Xem hướng đi' },
}

export function vapidBytes(key: string): Uint8Array<ArrayBuffer> {
  const raw = atob(key.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(key.length / 4) * 4, '='))
  return Uint8Array.from(raw, (char) => char.charCodeAt(0))
}

export function pushUnavailable(): string | null {
  if (!window.isSecureContext) return 'Thông báo đẩy cần HTTPS hoặc localhost. Hãy mở Noverse qua địa chỉ HTTPS; hộp thư vẫn nhận đủ thông báo.'
  if (!('Notification' in window) || !('serviceWorker' in navigator) || !('PushManager' in window)) return 'Trình duyệt này chưa hỗ trợ thông báo đẩy. Trên iPhone (iOS 16.4+), hãy Thêm vào Màn hình chính rồi mở Noverse từ đó. Hộp thư vẫn nhận đủ thông báo.'
  return null
}

export type DeviceEnv = { userAgent: string; platform: string; maxTouchPoints: number; standalone: boolean }

/** iPhone/iPad chưa được Thêm vào Màn hình chính: Web Push (iOS 16.4+) chỉ chạy trong ứng dụng web đã cài. iPadOS 13+ tự nhận là Mac, phân biệt bằng cảm ứng. */
export function needsHomeScreenInstall(env: DeviceEnv): boolean {
  const ios = /iPhone|iPad|iPod/.test(env.userAgent) || (env.platform === 'MacIntel' && env.maxTouchPoints > 1)
  return ios && !env.standalone
}

export function currentDeviceEnv(): DeviceEnv {
  return {
    userAgent: navigator.userAgent,
    platform: navigator.platform,
    maxTouchPoints: navigator.maxTouchPoints,
    standalone: (navigator as Navigator & { standalone?: boolean }).standalone === true || window.matchMedia('(display-mode: standalone)').matches,
  }
}
