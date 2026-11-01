// Tác vụ nền của editor. Tải video từ link và giải mã sóng giọng chạy phía
// server và có thể mất hàng phút; Cloudflare cắt mọi request không gửi byte nào
// trong 100 s, nên hai việc đó trả 202 kèm job và client hỏi trạng thái ở đây
// cho tới khi xong. Kết quả cuối cùng nằm trên server (media đã lưu, phong bì
// sóng giọng trả về), nên rời tab không làm mất việc đang chạy.
export const JOB_POLL_MS = 1500

export type EditorJobState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface EditorJobError {
  code: string
  error: string
}

export interface EditorJob {
  id: string
  kind: string
  state: EditorJobState
  createdAt: number
  startedAt?: number
  finishedAt?: number
  result?: Record<string, unknown> | null
  error?: EditorJobError | null
}

export type JobFetch = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export interface JobOptions {
  signal?: AbortSignal
  /** Nhịp hỏi trạng thái; test truyền 0 để không phải chờ. */
  intervalMs?: number
  /** Transport và gốc URL của storage (mặc định fetch + /api/editor). */
  request?: JobFetch
  baseUrl?: string
}

const DEFAULT_BASE_URL = '/api/editor'

const sleep = (ms: number, signal?: AbortSignal) => {
  const { promise, resolve, reject } = Promise.withResolvers<void>()
  if (signal?.aborted) { reject(signal.reason ?? new DOMException('Đã huỷ tác vụ', 'AbortError')); return promise }
  const abort = () => { clearTimeout(timer); reject(signal?.reason ?? new DOMException('Đã huỷ tác vụ', 'AbortError')) }
  const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve() }, ms)
  signal?.addEventListener('abort', abort, { once: true })
  return promise
}

/** Đọc một job; lỗi mạng/404 ném thông điệp server để lớp UI hiện nguyên văn. */
export async function readJob(id: string, options: JobOptions = {}): Promise<EditorJob> {
  const send = options.request ?? fetch
  const response = await send.call(globalThis, `${options.baseUrl ?? DEFAULT_BASE_URL}/jobs/${encodeURIComponent(id)}`, { cache: 'no-store', signal: options.signal })
  const body = await response.json().catch(() => ({})) as { job?: EditorJob; error?: string }
  if (!response.ok || !body.job) throw new Error(typeof body.error === 'string' ? body.error : `Không đọc được tác vụ (${response.status})`)
  return body.job
}

/** Chờ job tới trạng thái cuối; trả `result` hoặc ném lỗi/huỷ bằng tiếng Việt. */
export async function waitForJob(id: string, options: JobOptions = {}): Promise<Record<string, unknown>> {
  for (;;) {
    if (options.signal?.aborted) throw options.signal.reason ?? new DOMException('Đã huỷ tác vụ', 'AbortError')
    const job = await readJob(id, options)
    if (job.state === 'succeeded') return job.result ?? {}
    if (job.state === 'failed') throw new Error(job.error?.error ?? 'Không xong được tác vụ')
    if (job.state === 'cancelled') throw new Error(job.error?.error ?? 'Đã huỷ tác vụ')
    await sleep(options.intervalMs ?? JOB_POLL_MS, options.signal)
  }
}

/** Huỷ job phía server (best effort): bấm Huỷ phải dừng cả việc đang chạy. */
export async function cancelJob(id: string, options: JobOptions = {}): Promise<void> {
  const send = options.request ?? fetch
  try {
    await send.call(globalThis, `${options.baseUrl ?? DEFAULT_BASE_URL}/jobs/${encodeURIComponent(id)}`, { method: 'DELETE', cache: 'no-store' })
  } catch { /* huỷ là best effort, không chặn luồng UI */ }
}
