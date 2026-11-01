/**
 * Toast façade for engine code.
 *
 * Feature/store actions run inside the engine subtree, which cannot import app
 * UI modules (rootDir). They emit intent here; the app layer installs the real
 * toast via {@link installToastSink} when it mounts. Calls made before the sink
 * is installed are queued and replayed in order; without a sink the queue is
 * capped and older entries are dropped.
 */

type ToastAction = { label: string; onClick: () => void | Promise<void> }
type SonnerOptions = { description?: string; action?: ToastAction; undo?: boolean }
type Emit = (text: string, options?: SonnerOptions) => number
export type ToastSink = {
  (text: string, options?: SonnerOptions): number
  error: Emit
  success: Emit
  warning: Emit
  info: Emit
}

type Queued = { method: 'show' | 'error' | 'success' | 'warning' | 'info'; text: string; options?: SonnerOptions }

let sink: ToastSink | undefined
let queue: Queued[] = []
const QUEUE_MAX = 32

const emit = (method: Queued['method'], text: string, options?: SonnerOptions): number => {
  const target = sink
  if (target) return method === 'show' ? target(text, options) : target[method](text, options)
  queue.push({ method, text, options })
  if (queue.length > QUEUE_MAX) queue = queue.slice(-QUEUE_MAX)
  return -1
}

/** Sonner-shaped surface: call sites read like the old `toast` import. */
export const toast = Object.assign(
  (text: string, options?: SonnerOptions) => emit('show', text, options),
  {
    error: (text: string, options?: SonnerOptions) => emit('error', text, options),
    success: (text: string, options?: SonnerOptions) => emit('success', text, options),
    warning: (text: string, options?: SonnerOptions) => emit('warning', text, options),
    info: (text: string, options?: SonnerOptions) => emit('info', text, options),
  },
)

/** Called once by the app-side adapter; replays anything emitted earlier. */
export function installToastSink(next: ToastSink): () => void {
  sink = next
  const pending = queue
  queue = []
  for (const item of pending) emit(item.method, item.text, item.options)
  return () => {
    if (sink === next) sink = undefined
  }
}
