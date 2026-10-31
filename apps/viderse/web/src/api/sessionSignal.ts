// Tín hiệu phiên bên trong app — một nơi để transport (request.ts) báo cho
// SessionProvider:
//   · 401 trên bất kỳ request nào (kể cả probe của luồng sự kiện) → phiên hết:
//     listener dọn phiên UI, Root trả về Đăng nhập; việc tháo SSE/poll/cache
//     nằm ở unmount của Root.
//   · 403 app_access_denied → phiên thật nhưng chưa được cấp quyền Viderse:
//     listener đổi trạng thái cổng sang màn bị chặn, KHÔNG phải hết phiên.
//   · 403 password_change_required → phiên thật nhưng bị ép đổi mật khẩu tạm:
//     listener đưa về màn đổi mật khẩu. Không coi mọi 403 nghiệp vụ là mất
//     quyền app — chỉ đúng hai mã này mới đổi cổng.
// Listener chỉ đổi trạng thái, không phát request mới → không vòng lặp.
import type { User } from './types'

const expired = new Set<() => void>()
const denied = new Set<(user: User | null) => void>()
const passwordRequired = new Set<() => void>()

export function onSessionExpired(fn: () => void): () => void {
  expired.add(fn)
  return () => expired.delete(fn)
}

export function onAccessDenied(fn: (user: User | null) => void): () => void {
  denied.add(fn)
  return () => denied.delete(fn)
}

export function onPasswordChangeRequired(fn: () => void): () => void {
  passwordRequired.add(fn)
  return () => passwordRequired.delete(fn)
}

export function notifySessionExpired(): void {
  for (const fn of [...expired]) fn()
}

export function notifyAccessDenied(user: User | null): void {
  for (const fn of [...denied]) fn(user)
}

export function notifyPasswordChangeRequired(): void {
  for (const fn of [...passwordRequired]) fn()
}
