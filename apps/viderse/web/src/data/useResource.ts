// Hook đọc khoá cache — bám bản Noverse (useResource.ts) trên cache dùng chung
// của @noverse/ui/data: khoá ổn định theo nội dung, stale-while-revalidate.
import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from 'react'
import { cache, keyHash, type EntrySnapshot, type Fetcher, type Key } from '@noverse/ui/data'

export type ResourceOptions = {
  enabled?: boolean
  staleTime?: number
  /** Giữ dữ liệu của khoá trước tới khi khoá mới có dữ liệu (đổi kênh/video không nháy về khung chờ). */
  keepPrevious?: boolean
}
/** `placeholder`: `data` là của khoá trước (keepPrevious), khoá hiện tại chưa có dữ liệu. */
export type Resource<T> = EntrySnapshot<T> & { key: Key; initialLoading: boolean; placeholder: boolean; refresh: () => Promise<unknown> }

/**
 * Mặc định 30 s. Màn điện thoại nằm trong pane giữ-alive (`MobileRouteStage`),
 * nên đổi tab không còn là lần "mount lại" cần làm mới: `<Activity>` gỡ effect
 * khi ẩn rồi gắn lại khi hiện, và mốc 2 s cũ khiến mỗi lần quay lại tab bắn lại
 * toàn bộ request của màn đó. Dữ liệu vẫn tươi theo SSE (`data/events.ts` gọi
 * `invalidate`) và `refresh()` sau thao tác ghi; hook nào cần nhịp riêng
 * (status 2,5 s) tự truyền `staleTime`.
 */
const DEFAULT_STALE_MS = 30_000

export function useResource<T>(key: Key, fetcher: Fetcher<T>, options: ResourceOptions = {}): Resource<T> {
  const hash = keyHash(key)
  const stableKey = useMemo<Key>(() => JSON.parse(hash) as Key, [hash])
  const latest = useRef(fetcher)
  latest.current = fetcher
  const enabled = options.enabled !== false
  const subscribe = useCallback((listener: () => void) => enabled ? cache.listen(stableKey, listener) : () => {}, [stableKey, enabled])
  const rendered = useRef<EntrySnapshot<T> | undefined>(undefined)
  const snapshot = useCallback(() => {
    const next = cache.get<T>(stableKey)
    const previous = rendered.current
    if (previous && previous.data === next.data && previous.error === next.error && previous.status === next.status) return previous
    rendered.current = next
    return next
  }, [stableKey])
  const state = useSyncExternalStore(subscribe, snapshot, snapshot)
  useEffect(() => {
    if (!enabled) return
    return cache.retain(stableKey, (context) => latest.current(context), { staleTime: options.staleTime ?? DEFAULT_STALE_MS })
  }, [stableKey, enabled, options.staleTime])
  const refresh = useCallback(() => enabled ? cache.refresh(stableKey) : Promise.resolve(), [stableKey, enabled])
  const previous = useRef<T | undefined>(undefined)
  useEffect(() => { if (state.data !== undefined) previous.current = state.data }, [state.data])
  const data = state.data === undefined && options.keepPrevious && enabled ? previous.current : state.data
  return { ...state, key: stableKey, data, initialLoading: enabled && data === undefined && !state.error, placeholder: data !== state.data, refresh }
}
