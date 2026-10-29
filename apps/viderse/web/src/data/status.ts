// Trạng thái đầu trang và kết nối máy chủ.
// useStatus: đọc khoá 'status' (GET /api/status) — SSE `status`/`reset` invalidate
// khoá này; khi luồng mất thì vòng poll 3 s trong data/events.ts làm mới.
// useOnline: false ngay khi trình duyệt báo offline HOẶC luồng sự kiện không
// còn "open" → dải mất kết nối + khoá nút ghi trong ≤5 s sau khi mất nối.
import { useSyncExternalStore } from 'react'
import { useResource } from './useResource'
import { keys } from './keys'
import { status as fetchStatus } from '../api/status'
import { useEventsStatus } from './events'

export function useStatus() {
  return useResource(keys.status(), ({ signal }) => fetchStatus(signal), { staleTime: POLL_STALE_MS })
}

/** Tải lại xấp xỉ nhịp poll dự phòng — tránh tải trùng khi vừa nối lại. */
export const POLL_STALE_MS = 2500

const subscribeNet = (listener: () => void) => {
  window.addEventListener('online', listener)
  window.addEventListener('offline', listener)
  return () => {
    window.removeEventListener('online', listener)
    window.removeEventListener('offline', listener)
  }
}

export function useOnline(): boolean {
  const netDown = useSyncExternalStore(subscribeNet, () => !navigator.onLine)
  return !netDown && useEventsStatus().state === 'open'
}
