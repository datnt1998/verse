import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react'

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))

export type AsyncActionState = { busy: boolean; pending: boolean; error: unknown }

export type AsyncAction<A extends unknown[]> = {
  get(): AsyncActionState
  subscribe(fn: () => void): () => void
  run(...args: A): Promise<unknown>
  dispose(): void
}

/**
 * Phiên bản thu gọn của data/useAction cho kit: chặn bấm lặp ngay, spinner chỉ bật
 * sau 150 ms và giữ tối thiểu 400 ms — cùng nhịp hiển thị với app, không toast/invalidate.
 */
export function createAsyncAction<A extends unknown[]>(fn: (...args: A) => Promise<unknown>): AsyncAction<A> {
  let state: AsyncActionState = { busy: false, pending: false, error: null }
  const listeners = new Set<() => void>()
  let pendingTimer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  const update = (patch: Partial<AsyncActionState>) => { state = { ...state, ...patch }; if (!disposed) listeners.forEach((listener) => listener()) }
  const run = async (...args: A) => {
    if (state.busy) return undefined
    update({ busy: true, pending: false, error: null })
    let shownAt: number | null = null
    pendingTimer = setTimeout(() => { shownAt = Date.now(); update({ pending: true }) }, 150)
    try {
      const result = await fn(...args)
      return result
    } catch (error) {
      update({ error })
      return undefined
    } finally {
      clearTimeout(pendingTimer)
      if (shownAt !== null) {
        const remaining = 400 - (Date.now() - shownAt)
        if (remaining > 0) {
          await delay(remaining)
        }
      }
    }
  }
  return {
    get: () => state,
    subscribe(listener) { disposed = false; listeners.add(listener); return () => { listeners.delete(listener) } },
    run,
    dispose() { disposed = true; listeners.clear() },
  }
}

export function useAsyncAction<A extends unknown[]>(fn: (...args: A) => Promise<unknown>) {
  const latest = useRef(fn)
  latest.current = fn
  const controller = useMemo(() => createAsyncAction<A>((...args) => latest.current(...args)), [])
  useEffect(() => () => controller.dispose(), [controller])
  const state = useSyncExternalStore(controller.subscribe, controller.get, controller.get)
  return { ...state, run: controller.run }
}
