import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from 'react'
import { cache, sameData, type EntrySnapshot, type Fetcher } from './cache'
import { keyHash, type Key } from './keys'

export type ResourceOptions = {
  enabled?: boolean
  staleTime?: number
  /**
   * Khoá phụ thuộc nhau (chương, nguồn, tập): giữ dữ liệu của khoá trước cho tới khi khoá mới
   * có dữ liệu, thay vì rơi về `loading` (16-feedback-refresh: không bao giờ nháy về khung chờ).
   */
  keepPrevious?: boolean
}
export type Resource<T> = EntrySnapshot<T> & { key: Key; initialLoading: boolean; refresh: () => Promise<unknown> }

/** Khoá ổn định theo nội dung; fetcher mới không gây vòng lặp tải lại. */
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
    // Background bookkeeping belongs to the single app-level sync indicator.
    // A successful unchanged response must not render content just to update a timestamp.
    if (previous && previous.data === next.data && previous.error === next.error && previous.status === next.status) return previous
    rendered.current = next
    return next
  }, [stableKey])
  const state = useSyncExternalStore(subscribe, snapshot, snapshot)
  useEffect(() => {
    if (!enabled) return
    return cache.retain(stableKey, (context) => latest.current(context), { staleTime: options.staleTime })
  }, [stableKey, enabled, options.staleTime])
  const refresh = useCallback(() => enabled ? cache.refresh(stableKey) : Promise.resolve(), [stableKey, enabled])
  const previous = useRef<T | undefined>(undefined)
  useEffect(() => { if (state.data !== undefined) previous.current = state.data }, [state.data])
  const data = state.data === undefined && options.keepPrevious && enabled ? previous.current : state.data
  return { ...state, key: stableKey, data, initialLoading: enabled && data === undefined && !state.error, refresh }
}

/** Memoizes the selected slice so unchanged fields never invalidate consumers. */
export function useResourceValue<T, S>(key: Key, fetcher: Fetcher<T>, select: (data: T | undefined) => S, options: ResourceOptions = {}): S {
  const hash = keyHash(key)
  const stableKey = useMemo<Key>(() => JSON.parse(hash) as Key, [hash])
  const latest = useRef(fetcher)
  latest.current = fetcher
  const enabled = options.enabled !== false
  const subscribe = useCallback((listener: () => void) => enabled ? cache.listen(stableKey, listener) : () => {}, [stableKey, enabled])
  const snapshot = useMemo(() => {
    let previous = cache.get<T>(stableKey).data
    let selected = select(previous)
    return () => {
      const data = cache.get<T>(stableKey).data
      if (data === previous) return selected
      const next = select(data)
      if (!sameData(selected, next)) selected = next
      previous = data
      return selected
    }
  }, [stableKey, select])
  const value = useSyncExternalStore(subscribe, snapshot, snapshot)
  useEffect(() => {
    if (!enabled) return
    return cache.retain(stableKey, (context) => latest.current(context), { staleTime: options.staleTime })
  }, [stableKey, enabled, options.staleTime])
  return value
}
