// Client fetch bám sát platform/webapp (server.go):
//   GET  /api/setup         → { needed, expiresAt?, expired?, attemptsLeft, lockedUntil? }
//   POST /api/setup/verify  → { setupToken, expiresAt } · 400 {error, attemptsLeft?|format?} · 410 · 423 {error, lockedUntil} · 409 {error, at}
//   POST /api/setup/admin   → { user, recoveryCode } · 400 {error, field} · 410 · 409
//   POST /api/login         → { user } · 401 · 429 {error, retryAfter}
//   POST /api/logout        → { ok } · 401 khi chưa đăng nhập
//   GET  /api/me            → { user } · 401
// Lỗi server luôn là {"error": "<tiếng Việt>"}; request kèm cookie phiên same-origin.
import { request } from './request'
import type { SetupState, SetupVerifyResult, SetupAdminResult, User } from './types'

export { ApiError } from './request'

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
