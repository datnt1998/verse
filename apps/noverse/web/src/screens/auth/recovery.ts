// Logic thuần của form “Dùng mã khôi phục” ở màn đăng nhập (Login.dc.html › recover).
import { ApiError } from '../../api/client'
import { t } from '../../i18n/vi'

/** Mã dạng XXXX-XXXX-XXXX-XXXX: đủ 16 ký tự chữ-số mới cho gửi (dấu gạch không tính); mật khẩu đếm theo rune như máy chủ. */
export const recoveryReady = (code: string, password: string) =>
  code.toUpperCase().replace(/[^A-Z0-9]/g, '').length >= 16 && [...password].length >= 8

export type RecoveryFailure =
  | { kind: 'code'; message: string }
  | { kind: 'password'; message: string }
  | { kind: 'limited'; message: string; retryAfter: number }
  | { kind: 'message'; message: string }

/** POST /api/recover: 401 mã sai/đã dùng · 400 mật khẩu ngắn · 429 thử sai quá nhiều lần. */
export function recoveryFailure(error: unknown): RecoveryFailure {
  if (!(error instanceof ApiError)) return { kind: 'message', message: 'Không kết nối được máy chủ. Vui lòng thử lại.' }
  if (error.status === 401) return { kind: 'code', message: t.auth.errorRecoveryCode }
  if (error.status === 400) return { kind: 'password', message: t.auth.errorRecoveryShort }
  if (error.status === 429) return { kind: 'limited', message: t.auth.errorLimited, retryAfter: error.retryAfter ?? 60 }
  return { kind: 'message', message: error.message }
}
