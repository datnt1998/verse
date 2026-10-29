// Client fetch bám sát platform/webapp (server.go):
//   GET  /api/setup         → { needed, expiresAt?, expired?, attemptsLeft, lockedUntil? }
//   POST /api/setup/verify  → { setupToken, expiresAt } · 400 {error, attemptsLeft?|format?} · 410 · 423 {error, lockedUntil} · 409 {error, at}
//   POST /api/setup/admin   → { user, recoveryCode } · 400 {error, field} · 410 · 409
//   POST /api/login         → { user } · 401 · 429 {error, retryAfter}
//   POST /api/logout        → { ok } · 401 khi chưa đăng nhập
//   GET  /api/me            → { user } · 401
// Lỗi server luôn là {"error": "<tiếng Việt>"}; request kèm cookie phiên same-origin.
import type { SetupState, SetupVerifyResult, SetupAdminResult, User } from './types'

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

async function request<T>(path: string, options?: { body?: object; signal?: AbortSignal }): Promise<T> {
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

// ─── Phiên & thiết lập lần đầu ────────────────────────────────────────────

export const login = (username: string, password: string) => request<{ user: User }>('/api/login', { body: { username, password } })
export const logout = () => request<{ ok: boolean }>('/api/logout', { body: {} })
export const me = () => request<{ user: User }>('/api/me')

// Mã thiết lập chỉ in ở máy chủ, không bao giờ trả qua API — màn Setup gửi code lên
// để đổi setupToken, rồi gửi token cùng tài khoản quản trị đầu tiên.
export const getSetup = () => request<SetupState>('/api/setup')
export const verifySetup = (code: string) => request<SetupVerifyResult>('/api/setup/verify', { body: { code } })
export const createSetupAdmin = (body: { setupToken: string; name: string; username: string; password: string }) =>
  request<SetupAdminResult>('/api/setup/admin', { body })
