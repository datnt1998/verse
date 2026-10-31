// Client fetch bám sát platform/webapp (server.go) — kho tài khoản chung:
//   GET  /api/setup         → { needed, expiresAt?, expired?, attemptsLeft, lockedUntil? } · 503 khi kho lỗi (KHÔNG phải "trống")
//   POST /api/setup/verify  → { setupToken, expiresAt } · 400 {error, attemptsLeft?|format?} · 410 · 423 {error, lockedUntil} · 409 {error, at}
//   POST /api/setup/admin   → { user, recoveryCode } · 400 {error, field} · 410 · 409
//                             (recoveryCode có thể null: admin đã tạo nhưng chưa cấp được mã)
//   POST /api/login         → { user } · 401 · 429 {error, retryAfter}
//                             · 403 {code:"app_access_denied", app:"viderse", user} — cookie đã đặt, chưa được cấp quyền Viderse
//   POST /api/logout        → { ok } · 401 khi chưa đăng nhập
//   GET  /api/me            → { user } · 401 · 403 {code:"app_access_denied", user}
//   POST /api/me/password   → { ok, reauthenticate } · 403 sai mật khẩu hiện tại · 503 kho tài khoản lỗi
//   GET  /api/me/sessions   → { sessions }
//   POST /api/me/sessions/revoke-all      → { ok } — thu hồi MỌI phiên của chính mình
//   POST /api/sessions/{id}/revoke        → { ok } — chỉ phiên của chính mình
// Route nghiệp vụ khi phiên bắt buộc đổi mật khẩu → 403
// {code:"password_change_required", mustChangePassword:true}; route auth-neutral
// trên vẫn đi qua. Lỗi server luôn là {"error": "<tiếng Việt>"}; request kèm
// cookie phiên same-origin (verse_session chung của cả hai app).
import { ApiError, request } from './request'
import type { Session, SetupState, SetupVerifyResult, SetupAdminResult, User } from './types'

export { ApiError }

export const login = (username: string, password: string) => request<{ user: User }>('/api/login', { body: { username, password } })
export const logout = () => request<{ ok: boolean }>('/api/logout', { body: {} })
export const me = () => request<{ user: User }>('/api/me')

/**
 * Đổi mật khẩu của chính mình. Thành công thì máy chủ thu hồi MỌI phiên kể cả
 * phiên đang dùng (cả phiên của app kia trên kho chung) và xoá cookie —
 * `reauthenticate` báo UI phải dọn phiên cục bộ và mời đăng nhập lại, không
 * refresh /api/me (sẽ 401 như thể đổi thất bại). Sai mật khẩu hiện tại → 403,
 * kho tài khoản lỗi → 503.
 */
export const changePassword = (current: string, next: string) =>
  request<{ ok: boolean; reauthenticate?: boolean }>('/api/me/password', { body: { current, new: next } })

export const mySessions = () => request<{ sessions: Session[] }>('/api/me/sessions')
export const revokeAllMySessions = () => request<{ ok: boolean }>('/api/me/sessions/revoke-all', { body: {} })
/** Chỉ thu hồi phiên của chính mình — server từ chối phiên người khác. */
export const revokeSession = (id: string) => request<{ ok: boolean }>(`/api/sessions/${encodeURIComponent(id)}/revoke`, { body: {} })

/**
 * 403 `{code:"app_access_denied"}` — phiên hợp lệ nhưng tài khoản chưa được cấp
 * quyền vào Viderse: trả view user an toàn đi kèm (grants kèm trong), khác hẳn
 * 401 phiên hết hạn. UI giữ người đang đăng nhập, không quay vòng màn login.
 */
export const accessDeniedUser = (error: unknown): User | null => {
  const payload = error instanceof ApiError && error.status === 403 ? error.payload : null
  return payload && typeof payload === 'object' && 'code' in payload && payload.code === 'app_access_denied' &&
    'user' in payload && payload.user && typeof payload.user === 'object' ? payload.user as User : null
}

/**
 * 403 `{code:"password_change_required"}` — phiên hợp lệ nhưng server ép đổi
 * mật khẩu tạm trước khi dùng tiếp. Khác app_access_denied: tài khoản vẫn có
 * quyền app, chỉ bị chặn cho tới khi đổi xong.
 */
export const passwordChangeRequired = (error: unknown): boolean =>
  error instanceof ApiError && error.status === 403 &&
  !!error.payload && typeof error.payload === 'object' && 'code' in error.payload &&
  error.payload.code === 'password_change_required'

// Mã thiết lập chỉ in ở máy chủ, không bao giờ trả qua API — màn Setup gửi code lên
// để đổi setupToken, rồi gửi token cùng tài khoản quản trị đầu tiên.
export const getSetup = () => request<SetupState>('/api/setup')
export const verifySetup = (code: string) => request<SetupVerifyResult>('/api/setup/verify', { body: { code } })
export const createSetupAdmin = (body: { setupToken: string; name: string; username: string; password: string }) =>
  request<SetupAdminResult>('/api/setup/admin', { body })
