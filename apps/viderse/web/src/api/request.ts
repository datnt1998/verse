// Request JSON bám sát platform/webapp: cookie phiên same-origin, body JSON,
// lỗi server luôn là {"error": "<tiếng Việt>"} (ApiError.status 0 = không kết nối được).
// 401 trên BẤT KỲ request nào → báo hết phiên cho app (sessionSignal) trước khi ném.
//
// Quy ước transport một chỗ (webguard chặn trước khi tới handler):
//  - POST/PUT/PATCH luôn gửi body JSON — gửi `{}` khi caller không truyền body,
//    vì middleware bắt buộc content-type JSON trên mọi POST/PUT.
//  - GET/DELETE không gửi body (DELETE có body vẫn hợp lệ nhưng hiện không có route nào cần).
//  - {body: undefined} ở `request()` nghĩa là GET; có body nghĩa là POST.
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

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/** Transport chung duy nhất: mọi client đi qua đây nên quy tắc JSON chỉ sửa một chỗ. */
async function call<T>(method: Method, path: string, body?: object, signal?: AbortSignal): Promise<T> {
  // POST/PUT/PATCH: gateway bắt buộc body JSON kể cả rỗng — gửi {} để không bị 400.
  const wantsJson = method === 'POST' || method === 'PUT' || method === 'PATCH'
  const sendBody = body !== undefined && method !== 'GET'
  let response: Response
  try {
    response = await fetch(path, {
      credentials: 'same-origin',
      method,
      signal,
      ...(wantsJson || sendBody
        ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body ?? {}) }
        : {}),
    })
  } catch (error) {
    if (signal?.aborted) throw error
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

/** GET khi không truyền `body`, POST khi có — bám hợp đồng webapp. */
export function request<T>(path: string, options?: { body?: object; signal?: AbortSignal }): Promise<T> {
  return call(options?.body === undefined ? 'GET' : 'POST', path, options?.body, options?.signal)
}

/** Mutation có method POST/PUT/PATCH/DELETE — cùng hợp đồng lỗi với request(). */
export function mutate<T>(method: Method, path: string, body?: object, signal?: AbortSignal): Promise<T> {
  return call(method, path, body, signal)
}
