// Kiểu wire bám sát platform/webapp + platform/identity (UserView).

// AppId khớp app_grants.app của kho tài khoản chung (identity.App): chính
// hai app chia sẻ kho. `grants` đi cùng user và mang quyền hiệu lực từng app —
// admin nền tảng được kế thừa quyền admin cả hai app mà không cần dòng grant.
export type AppId = 'noverse' | 'viderse'
export type AppGrants = Partial<Record<AppId, Role>>

export type AppLink = { id: AppId; name: string; origin: string }
export type SessionEnded = { reason: string; app?: AppId }

export type Role = 'admin' | 'member'

export type User = {
  username: string
  name: string
  role: Role
  /** Quyền hiệu lực từng app của kho chung; {} = member chưa được cấp app nào. */
  grants?: AppGrants | null
  /** Server đánh dấu phải đổi mật khẩu tạm trước khi dùng tiếp. */
  mustChangePassword?: boolean
  createdAt?: string
}

// Phiên tài khoản — wire của GET /api/me/sessions (identity.Session).
export type Session = {
  id: string
  device: string
  ip?: string
  createdAt: string
  lastSeen: string
  expiresAt: string
  current: boolean
}

/** GET /api/setup — `needed` false khi máy chủ đã có tài khoản. */
export type SetupState = {
  needed: boolean
  expiresAt?: string
  expired?: boolean
  attemptsLeft: number
  lockedUntil?: string
}

/** POST /api/setup/verify — token chỉ sống vài phút, đủ để tạo quản trị. */
export type SetupVerifyResult = { setupToken: string; expiresAt: string }

/** POST /api/setup/admin — mã khôi phục chỉ trả đúng lần này; `null` nghĩa là
 * admin đã tạo + phiên đã đặt nhưng server chưa cấp được mã (cấp lại qua luồng
 * khôi phục sau khi đăng nhập), KHÔNG phải thất bại. */
export type SetupAdminResult = { user: User; recoveryCode: string | null }
