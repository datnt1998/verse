// Luồng sự kiện Viderse: GET /api/events (SSE, resume bằng Last-Event-ID).
// Sự kiện → prefix khoá theo eventKeys; status, queue và activity có vòng poll
// dự phòng 10→60 s — chỉ chạy khi tab đang nhìn và luồng không còn "open".
// Luồng mở (bình thường) thì không có request nền nào: mọi thay đổi tới bằng
// sự kiện. Vòng dự phòng giãn dần để máy mất luồng không bắn request liên tục.
// Mất kết nối phải lộ trong ≤5 s (design 07 §5): server ping 2 s nên luồng
// không byte nào sau 3,5 s coi như chết (kể cả read() treo), headers không
// về sau 3,5 s cũng là mất kết nối; trình duyệt báo offline nối lại ngay.
import { useSyncExternalStore } from 'react'
import { cache, createEventsClient, fetchSource, invalidate, keyHash, matchesPrefix, type Key } from '@noverse/ui/data'
import { ApiError, me } from '../api/client'
import { notifySessionExpired } from '../api/sessionSignal'
import { keysForEvent, SERVER_EVENT_TYPES, type ServerEventType } from './eventKeys'
import { keys } from './keys'

export const events = createEventsClient({
  url: '/api/events',
  eventTypes: SERVER_EVENT_TYPES,
  createSource: (url) => fetchSource(url, {
    // 401 của chính luồng → chạy đường hết phiên của app (giống mọi request).
    onUnauthorized: () => notifySessionExpired(),
    stallMs: 3500,         // 3,5 s không byte nào (cả ping) → coi như rớt
    stallCheckMs: 500,     // nhịp kiểm tra watchdog
    connectTimeoutMs: 3500,// headers không về sau 3,5 s → kết nối không tới được
  }),
  probe: async () => {
    try { await me(); return 'ok' as const }
    catch (error) { return error instanceof ApiError && error.status === 401 ? 'unauthorized' as const : 'ok' as const }
  },
})

export const subscribeEvents = (type: ServerEventType | readonly ServerEventType[] | '*', handler: (data: unknown, meta: { type: string; id: string }) => void) =>
  events.subscribe(type, handler)

/** Nhịp đầu của vòng poll dự phòng (SSE xuống), rồi gấp đôi tới `POLL_MAX_MS`. */
export const POLL_MS = 10_000
export const POLL_MAX_MS = 60_000

/** Poll bộ đếm, hàng chờ và việc nền khi tab hiện + SSE xuống. */
const pollWanted = () => events.status().state !== 'open' && document.visibilityState === 'visible'

/**
 * Gắn luồng sự kiện vào cache — gọi một lần khi đã có phiên.
 * Trả hàm tháo (đăng xuất / test): dừng poll + luồng + quét sạch cache —
 * sau đó không request nào được phép ghi cache nữa.
 */
export function startDataEvents() {
  const pending = new Map<string, Key>()
  let timer: NodeJS.Timeout | undefined
  let pollTimer: NodeJS.Timeout | undefined
  const flush = () => {
    timer = undefined
    const prefixes = [...pending.values()]
    prefixes
      .filter((key, i) => !prefixes.some((other, j) => i !== j && other.length < key.length && matchesPrefix(key, other)))
      .forEach((key) => { void invalidate(key) })
  }
  const reset = () => { clearTimeout(timer); timer = undefined; pending.clear(); void invalidate() }

  const refreshQueueStatus = () => {
    void cache.refresh(keys.status()).catch(() => {})
    void cache.refresh(keys.queue()).catch(() => {})
    void cache.refresh(keys.activity()).catch(() => {})
  }
  // Tự hẹn giờ thay vì setInterval: mỗi lần chạy giãn nhịp gấp đôi tới trần
  // 60 s, nên máy để tab mở mà luồng sự kiện chết chỉ tốn ≤1 lượt/phút.
  const schedulePoll = (delay: number) => {
    if (pollTimer !== undefined) return
    pollTimer = setTimeout(() => {
      pollTimer = undefined
      refreshQueueStatus()
      schedulePoll(Math.min(delay * 2, POLL_MAX_MS))
    }, delay)
  }
  const stopPoll = () => {
    if (pollTimer === undefined) return
    clearTimeout(pollTimer)
    pollTimer = undefined
  }
  const syncPoll = () => {
    if (!pollWanted()) { stopPoll(); return }
    // Vừa có tab/vừa rớt luồng: làm mới ngay rồi mới hẹn nhịp đầu.
    if (pollTimer === undefined) {
      refreshQueueStatus()
      schedulePoll(POLL_MS)
    }
  }

  const unmap = events.subscribe('*', (data, meta) => {
    const prefixes = keysForEvent({ type: meta.type, data })
    if (prefixes === '*') { reset(); return }
    prefixes.forEach((key) => pending.set(keyHash(key), key))
    if (pending.size && timer === undefined) timer = setTimeout(flush, 250)
  })
  // Nối lại sau khi rớt: invalidate toàn bộ → cache tự fetch lại (đúng thứ
  // tự, có huỷ) — không fetch thô ngoài cache (tránh ghi đè dữ liệu mới hơn).
  const unReconnect = events.onReconnect(reset)
  const unStatus = events.onStatus(syncPoll)
  // Tab ẩn: ngừng poll; vừa trở lại tab thì làm mới ngay (syncPoll refresh khi bật).
  const onVisibility = () => syncPoll()
  // Trình duyệt báo offline/online: nối lại ngay thay vì chờ backoff/watchdog.
  const onNetChange = () => events.retryNow()
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('offline', onNetChange)
  window.addEventListener('online', onNetChange)

  syncPoll()
  events.start()
  return () => {
    clearTimeout(timer)
    pending.clear()
    if (pollTimer !== undefined) { clearTimeout(pollTimer); pollTimer = undefined }
    unmap(); unReconnect(); unStatus()
    document.removeEventListener('visibilitychange', onVisibility)
    window.removeEventListener('offline', onNetChange)
    window.removeEventListener('online', onNetChange)
    events.stop()
    cache.clear()
  }
}

/** Trạng thái kết nối SSE cho UI (dải mất kết nối). */
export function useEventsStatus() {
  return useSyncExternalStore(
    (listener) => events.onStatus(listener),
    () => events.status(),
    () => events.status(),
  )
}
