import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react'
import { toast } from '@noverse/ui'
import { createAction, type ActionOptions } from './action'

export function useAction<T, A extends unknown[]>(fn: (...args: A) => Promise<T>, options: ActionOptions<T, A> = {}) {
  const latest = useRef({ fn, options })
  latest.current = { fn, options }
  const controller = useMemo(() => createAction<T, A>((...args) => latest.current.fn(...args), {
    get success() { return latest.current.options.success },
    get error() { return latest.current.options.error },
    get invalidate() { return latest.current.options.invalidate },
    get optimistic() { return latest.current.options.optimistic },
    get undo() { return latest.current.options.undo },
  }, (notice) => toast(notice.text, notice.error ? 'err' : 'ok', notice.action, { key: notice.error ? `action-error:${notice.text}` : undefined, undo: !!notice.undoMs, duration: notice.undoMs })), [])
  useEffect(() => () => controller.dispose(), [controller])
  const state = useSyncExternalStore(controller.subscribe, controller.get, controller.get)
  return { ...state, run: controller.run }
}
