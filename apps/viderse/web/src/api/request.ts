// Request JSON bám sát platform/webapp: cookie phiên same-origin, body JSON,
// lỗi server luôn là {"error": "<tiếng Việt>"} (ApiError.status 0 = không kết nối được).
// 401 trên BẤT KỲ request nào → báo hết phiên cho app (sessionSignal) trước khi ném.
import { notifySessionExpired } from './sessionSignal'
export class ApiError extends Error {
  retryAfter?: number
  /** thân JSON lỗi gốc — vd 400 setup/verify mang `attemptsLeft`/`format`, 423 mang `lockedUntil` */
  payload?: unknown
  constructor(public status: number, message: string, retryAfter?: number, payload?: unknown) {
    // Lỗi lấy nguyên từ server, chỉ viết hoa chữ đầu khi hiển thị.
    super(message.charAt(0).toLocaleUpperCase('vi') + message.slice(1))
    this.name = 'ApiError'
    this.retryAfter = retryAfter
    this.payload = payload
  }
}

const OFFLINE_MSG = 'Không kết nối được máy chủ. Vui lòng thử lại.'

export async function request<T>(path: string, options?: { body?: object; signal?: AbortSignal }): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      credentials: 'same-origin',
      method: options?.body === undefined ? 'GET' : 'POST',
      signal: options?.signal,
      ...(options?.body === undefined
        ? {}
        : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(options.body) }),
    })
  } catch (error) {
    if (options?.signal?.aborted) throw error
    throw new ApiError(0, OFFLINE_MSG)
  }
  if (!response.ok) {
    if (response.status === 401) notifySessionExpired()
    const payload: unknown = await response.json().catch(() => null)
    const message =
      payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string'
        ? payload.error
        : response.statusText
    const retry =
      payload && typeof payload === 'object' && 'retryAfter' in payload && typeof payload.retryAfter === 'number'
        ? payload.retryAfter
        : undefined
    throw new ApiError(response.status, message || `Lỗi HTTP ${response.status}`, retry, payload)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
