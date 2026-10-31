// Trạng thái đầu trang và kết nối máy chủ.
// useStatus: đọc khoá 'status' (GET /api/status) — SSE `status`/`reset` invalidate
// khoá này; khi luồng mất thì vòng poll 2,5 s trong data/events.ts làm mới.
// Kết nối API vẫn hợp lệ khi SSE hỏng nhưng poll thành công; chỉ offline
// khi trình duyệt mất mạng hoặc status API thất bại.
import { useSyncExternalStore } from 'react'
import { useResource } from './useResource'
import { keys } from './keys'
import { status as fetchStatus } from '../api/status'

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
  const status = useStatus()
  return !netDown && status.status !== 'error'
}
