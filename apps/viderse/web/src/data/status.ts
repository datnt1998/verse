// Trạng thái đầu trang và ngưỡng mất kết nối theo data-contract §3.
import { useEffect, useState, useSyncExternalStore } from 'react'
import type { Fetcher } from '@noverse/ui/data'
import { useResource } from './useResource'
import { keys } from './keys'
import { status as fetchStatus, type Status } from '../api/status'
import { useEventsStatus } from './events'

let consecutiveFailures = 0
const failureListeners = new Set<() => void>()
function recordFailures(next: number) {
  if (consecutiveFailures === next) return
  consecutiveFailures = next
  for (const notify of failureListeners) notify()
}
const subscribeFailures = (notify: () => void) => {
  failureListeners.add(notify)
  return () => { failureListeners.delete(notify) }
}
const failureSnapshot = () => consecutiveFailures
const serverFailureSnapshot = () => 0
const trackedStatus: Fetcher<Status> = async ({ signal }) => {
  try {
    const value = await fetchStatus(signal)
    recordFailures(0)
    return value
  } catch (error) {
    if (!signal.aborted) recordFailures(consecutiveFailures + 1)
    throw error
  }
}

export const POLL_STALE_MS = 2500
export function useStatus() {
  return useResource(keys.status(), trackedStatus, { staleTime: POLL_STALE_MS })
}

const subscribeNet = (listener: () => void) => {
  window.addEventListener('online', listener)
  window.addEventListener('offline', listener)
  return () => {
    window.removeEventListener('online', listener)
    window.removeEventListener('offline', listener)
  }
}

/** One failed status poll is tolerated; two failures OR an SSE gap >10s is offline. */
export function useOnline(): boolean {
  const netDown = useSyncExternalStore(subscribeNet, () => !navigator.onLine, () => false)
  const failures = useSyncExternalStore(subscribeFailures, failureSnapshot, serverFailureSnapshot)
  const stream = useEventsStatus()
  const [sseExpired, setSseExpired] = useState(false)
  useStatus()
  const disconnected = stream.state !== 'open'
  useEffect(() => {
    if (!disconnected) { setSseExpired(false); return }
    const timer = window.setTimeout(() => setSseExpired(true), 10_001)
    return () => window.clearTimeout(timer)
  }, [disconnected])
  return !netDown && failures < 2 && !sseExpired
}
