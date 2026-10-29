import type { Job } from '../api/types'
import { subscribeEvents } from '../data/events'

const preferenceKey = (user: string) => `ainovel.mobile.notifications.${encodeURIComponent(user)}`
let username: string | null = null
let preferred = false
let revision = 0
let unsubscribe: (() => void) | undefined
let registration: Promise<ServiceWorkerRegistration> | undefined
let listening = false

/** Hỗ trợ của thiết bị, không xin quyền khi mở trang hay đọc trạng thái. */
export function notificationSupport(): { supported: boolean; reason: string } {
  if (typeof window === 'undefined') return { supported: false, reason: 'Thông báo chỉ dùng trong trình duyệt.' }
  if (!window.isSecureContext) return {
    supported: false,
    reason: 'Thông báo không hoạt động qua HTTP không bảo mật. Hãy mở bằng HTTPS hoặc localhost; trên điện thoại, dùng địa chỉ HTTPS của máy chủ.',
  }
  if (!('Notification' in window) || !('serviceWorker' in navigator)
    || typeof ServiceWorkerRegistration === 'undefined' || !('showNotification' in ServiceWorkerRegistration.prototype)) {
    return { supported: false, reason: 'Trình duyệt này chưa hỗ trợ thông báo. Trên iPhone, hãy thêm ứng dụng vào Màn hình chính rồi mở lại.' }
  }
  return {
    supported: true,
    reason: Notification.permission === 'denied'
      ? 'Thông báo đã bị chặn. Hãy cho phép trong cài đặt trang của trình duyệt rồi bật lại.'
      : 'Báo khi xong việc lúc ứng dụng ở nền. Cần giữ ứng dụng đang mở để nhận sự kiện.',
  }
}

function registerWorker(): Promise<ServiceWorkerRegistration> {
  if (!registration) {
    registration = navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch((error: unknown) => {
      registration = undefined
      throw error
    })
  }
  return registration
}

function readPreference(): boolean {
  if (!username) return false
  try { return localStorage.getItem(preferenceKey(username)) === '1' } catch { return false }
}

function savePreference(enabled: boolean) {
  preferred = enabled
  if (username) {
    try { localStorage.setItem(preferenceKey(username), enabled ? '1' : '0') } catch { /* Vẫn dùng được trong phiên nếu bộ nhớ bị chặn. */ }
  }
}

export function notificationsEnabled(): boolean {
  return !!username && preferred && notificationSupport().supported && Notification.permission === 'granted'
}

function syncSubscription() {
  if (!notificationsEnabled()) {
    unsubscribe?.()
    unsubscribe = undefined
    return
  }
  if (unsubscribe) return
  unsubscribe = subscribeEvents('job.done', (data) => {
    if (!notificationsEnabled()) { syncSubscription(); return }
    if (!document.hidden || !data || typeof data !== 'object') return
    const job = data as Partial<Job>
    if (typeof job.id !== 'string' || typeof job.title !== 'string' || typeof job.href !== 'string') return
    const currentUser = username
    const currentRevision = revision
    void navigator.serviceWorker.ready.then(async (worker) => {
      // Tắt công tắc, đổi tài khoản hoặc quay lại trang trong lúc chờ → không báo muộn.
      if (currentRevision !== revision || currentUser !== username || !notificationsEnabled() || !document.hidden) return
      await worker.showNotification(`Đã xong · ${job.title}`, {
        body: job.result?.sub ?? 'Việc chạy nền đã hoàn tất. Chạm để xem kết quả.',
        icon: '/icons/icon-192.png',
        badge: '/icons/icon-192.png',
        tag: `ainovel-job-${job.id}`,
        lang: 'vi',
        data: { href: job.href },
      })
    }).catch(() => { /* Hệ điều hành có thể tắt thông báo dù trình duyệt đã cấp quyền. */ })
  })
}

/** Store việc nền gọi theo vòng đời đăng nhập; không dùng chung tuỳ chọn giữa tài khoản. */
export function setNotificationUser(user: string | null): void {
  if (username === user) return
  revision++
  unsubscribe?.()
  unsubscribe = undefined
  username = user
  preferred = readPreference()
  if (typeof Notification !== 'undefined' && Notification.permission === 'denied') savePreference(false)
  syncSubscription()
}

/** Chỉ gọi từ thao tác bật/tắt rõ ràng của người dùng; trả về trạng thái bật thực tế. */
export async function setNotificationsEnabled(enabled: boolean): Promise<boolean> {
  const currentRevision = ++revision
  const currentUser = username
  if (!enabled || !username || !notificationSupport().supported || Notification.permission === 'denied') {
    savePreference(false)
    syncSubscription()
    return false
  }
  try {
    // Xin quyền trước mọi await khác để giữ thao tác người dùng (đặc biệt trên iOS).
    const permission = Notification.permission === 'default' ? await Notification.requestPermission() : Notification.permission
    if (currentRevision !== revision || currentUser !== username) return notificationsEnabled()
    if (permission !== 'granted') {
      savePreference(false)
      syncSubscription()
      return false
    }
    await registerWorker()
    if (currentRevision !== revision || currentUser !== username) return notificationsEnabled()
    savePreference(true)
    syncSubscription()
    return true
  } catch {
    if (currentRevision === revision && currentUser === username) {
      savePreference(false)
      syncSubscription()
    }
    return false
  }
}

/** Gọi một lần lúc khởi động. Không cache dữ liệu hay yêu cầu quyền thông báo ở đây. */
export function registerMobilePWA(): void {
  if (typeof window === 'undefined' || !window.isSecureContext || !('serviceWorker' in navigator)) return
  void registerWorker().catch(() => { /* Không có worker thì ứng dụng trực tuyến vẫn hoạt động. */ })
  if (listening) return
  listening = true
  window.addEventListener('storage', (event) => {
    if (!username || (event.key !== null && event.key !== preferenceKey(username))) return
    revision++
    preferred = readPreference()
    syncSubscription()
  })
  const refreshPermission = () => {
    if (typeof Notification !== 'undefined' && Notification.permission === 'denied' && preferred) {
      revision++
      savePreference(false)
    }
    syncSubscription()
  }
  document.addEventListener('visibilitychange', refreshPermission)
  if (notificationSupport().supported && navigator.permissions) {
    void navigator.permissions.query({ name: 'notifications' }).then((permission) => {
      permission.addEventListener('change', refreshPermission)
    }).catch(() => { /* Một số trình duyệt không có truy vấn quyền thông báo. */ })
  }
  syncSubscription()
}
