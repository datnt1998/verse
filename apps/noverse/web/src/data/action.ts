import { isNeedsConfig } from '../api/needsConfig'
import { invalidate } from './cache'
import type { Key } from './keys'

export type ActionState = { busy: boolean; pending: boolean; success: boolean; error: unknown }
export type ActionOptions<T, A extends unknown[]> = {
  success?: string
  /** Chuỗi = toast lỗi dự phòng; `false` = lỗi chỉ hiện tại chỗ (lỗi theo trường, 16 §3), không toast. */
  error?: string | false
  invalidate?: Key[]
  optimistic?: (...args: A) => (() => void)
  undo?: (result: T, args: A) => Promise<void>
}
export type ActionNotice = { text: string; error?: boolean; action?: { label: string; onClick: () => void | Promise<void> }; undoMs?: number }
export interface ActionController<T, A extends unknown[]> {
  get(): ActionState
  subscribe(fn: () => void): () => void
  run(...args: A): Promise<T | undefined>
  dispose(): void
}

/** Chặn bấm lặp ngay, nhưng chỉ hiện spinner sau 150 ms và giữ ít nhất 400 ms. */
export function createAction<T, A extends unknown[]>(fn: (...args: A) => Promise<T>, options: ActionOptions<T, A> = {}, notify: (notice: ActionNotice) => void = () => {}): ActionController<T, A> {
  let state: ActionState = { busy: false, pending: false, success: false, error: null }
  const listeners = new Set<() => void>()
  let pendingTimer: ReturnType<typeof setTimeout> | undefined
  let successTimer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  const update = (patch: Partial<ActionState>) => { state = { ...state, ...patch }; if (!disposed) listeners.forEach((listener) => listener()) }
  const run = async (...args: A): Promise<T | undefined> => {
    if (state.busy) return undefined
    const undoAction = options.undo
    const successText = options.success
    const prefixes = options.invalidate
    const refresh = () => { prefixes?.forEach((key) => { void invalidate(key) }) }
    clearTimeout(successTimer)
    update({ busy: true, pending: false, success: false, error: null })
    let shownAt: number | null = null
    let rollback: (() => void) | undefined
    pendingTimer = setTimeout(() => { shownAt = Date.now(); update({ pending: true }) }, 150)
    try {
      rollback = options.optimistic?.(...args)
      const result = await fn(...args)
      refresh()
      update({ success: true })
      successTimer = setTimeout(() => update({ success: false }), 1200)
      if (successText || undoAction) {
        let undone = false
        const undo = async () => {
          if (undone) return
          undone = true
          try { await undoAction!(result, args); refresh(); notify({ text: 'Đã hoàn tác' }) }
          catch (error) {
            undone = false
            notify({ text: error instanceof Error ? error.message : 'Không hoàn tác được', error: true, action: { label: 'Thử lại', onClick: undo } })
          }
        }
        notify({ text: successText ?? 'Đã thực hiện', ...(undoAction ? { undoMs: 5000, action: { label: 'Hoàn tác', onClick: undo } } : {}) })
      }
      return result
    } catch (error) {
      rollback?.()
      update({ error })
      const errorText = options.error
      if (errorText !== false) notify({ text: error instanceof Error ? error.message : errorText ?? 'Không thực hiện được thao tác', error: true, action: isNeedsConfig(error) ? { label: 'Mở Tuỳ chỉnh', onClick: () => { location.hash = '#/settings/providers' } } : { label: 'Thử lại', onClick: () => { void run(...args) } } })
      return undefined
    } finally {
      clearTimeout(pendingTimer)
      if (shownAt !== null) {
        const remaining = 400 - (Date.now() - shownAt)
        if (remaining > 0) await new Promise<void>((resolve) => { pendingTimer = setTimeout(resolve, remaining) })
      }
      update({ busy: false, pending: false })
    }
  }
  return {
    get: () => state,
    subscribe(listener) { disposed = false; listeners.add(listener); return () => { listeners.delete(listener) } },
    run,
    dispose() { disposed = true; clearTimeout(successTimer); listeners.clear() },
  }
}
