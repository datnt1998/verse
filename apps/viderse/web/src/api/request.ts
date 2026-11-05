// Request JSON bám sát platform/webapp: cookie phiên same-origin, body JSON,
// lỗi server luôn là {"error": "<tiếng Việt>"} (ApiError.status 0 = không kết nối được).
// Chỉ 401 trên BẤT KỲ request nào mới báo hết phiên (sessionSignal). 403 mang
// hai mã cổng của kho tài khoản chung — app_access_denied (chưa cấp quyền app)
// và password_change_required (ép đổi mật khẩu tạm) — cũng báo lên session
// layer để mọi caller kể cả không đọc payload được đưa về đúng màn; mọi 403
// nghiệp vụ khác chỉ là lỗi của riêng request đó.
//
// Quy ước transport một chỗ (webguard chặn trước khi tới handler):
//  - POST/PUT/PATCH luôn gửi body JSON — gửi `{}` khi caller không truyền body,
//    vì middleware bắt buộc content-type JSON trên mọi POST/PUT.
//  - GET/DELETE không gửi body (DELETE có body vẫn hợp lệ nhưng hiện không có route nào cần).
//  - {body: undefined} ở `request()` nghĩa là GET; có body nghĩa là POST.
import { notifyAccessDenied, notifyPasswordChangeRequired, notifySessionExpired } from './sessionSignal'
import type { User } from './types'
import { readAppLinks, readSessionEnded } from './sso'
export class ApiError extends Error {
  retryAfter?: number
  /** thân JSON lỗi gốc — vd 400 setup/verify mang `attemptsLeft`/`format`, 423 mang `lockedUntil`, 403 cổng mang `code`/`user` */
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
  return settle<T>(response, path)
}

/** Đọc phản hồi theo đúng hợp đồng lỗi của webapp (401 → hết phiên). */
async function settle<T>(response: Response, path: string): Promise<T> {
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const body = payload && typeof payload === 'object' ? payload : null
    if (response.status === 401) {
      // An invalid login must not overwrite a previously supplied session reason.
      if (path !== '/api/login') notifySessionExpired(readSessionEnded(body))
    } else if (response.status === 403 && body && 'code' in body) {
      // 403 app_access_denied mang user an toàn trong body (giữ phiên, không
      // phải hết phiên); 403 password_change_required chỉ đổi cổng. Hai tín
      // hiệu này không đụng vào cookie hay trạng thái của app kia.
      if (body.code === 'app_access_denied') {
        notifyAccessDenied('user' in body && body.user && typeof body.user === 'object' ? (body.user as User) : null, readAppLinks(body))
      } else if (body.code === 'password_change_required') {
        notifyPasswordChangeRequired()
      }
    }
    const message =
      body && 'error' in body && typeof body.error === 'string' ? body.error : response.statusText
    const retry = body && 'retryAfter' in body && typeof body.retryAfter === 'number' ? body.retryAfter : undefined
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

/**
 * POST `multipart/form-data` — chỉ hai route tải tệp của webapp chấp nhận
 * (lớp phủ, nhập prompt). Không đặt `Content-Type` tay: trình duyệt tự thêm
 * boundary. Mọi route khác vẫn phải đi qua JSON của `request`/`mutate`.
 */
export async function upload<T>(path: string, form: FormData, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { credentials: 'same-origin', method: 'POST', body: form, signal })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new ApiError(0, OFFLINE_MSG)
  }
  return settle<T>(response, path)
}

export type DownloadedFile = { blob: Blob; filename?: string }

/** Tải tệp có `Content-Disposition: attachment` qua cùng hợp đồng lỗi/phản hồi. */
export async function downloadFile(path: string, signal?: AbortSignal): Promise<DownloadedFile> {
  let response: Response
  try {
    response = await fetch(path, { credentials: 'same-origin', signal })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new ApiError(0, OFFLINE_MSG)
  }
  if (!response.ok) {
    await settle<never>(response, path)
    throw new ApiError(response.status, `Lỗi HTTP ${response.status}`)
  }
  const disposition = response.headers.get('Content-Disposition') ?? ''
  const match = /filename\*?=(?:UTF-8'')?("?)([^";]+)\1/i.exec(disposition)
  let filename: string | undefined
  if (match) {
    try { filename = decodeURIComponent(match[2]) } catch { filename = match[2] }
  }
  return { blob: await response.blob(), filename }
}
