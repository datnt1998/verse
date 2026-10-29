// Kiểu wire bám sát platform/webapp + platform/identity (UserView).

export type Role = 'admin' | 'member'

export type User = {
  username: string
  name: string
  role: Role
  createdAt?: string
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

/** POST /api/setup/admin — mã khôi phục chỉ trả đúng lần này, không lấy lại được. */
export type SetupAdminResult = { user: User; recoveryCode: string }
