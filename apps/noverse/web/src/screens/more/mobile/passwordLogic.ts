import { strengthOf } from '../../auth/parts'

/** Cùng công thức bốn nấc với màn tài khoản trên máy tính. */
export const passwordStrength = (value: string) => strengthOf(value)

export function passwordChecklist(current: string, next: string, repeat: string, first = false) {
  return [
    { label: 'Ít nhất 8 ký tự', ok: next.length >= 8 },
    { label: 'Có cả chữ và số', ok: /\d/.test(next) && /[a-zA-Z]/.test(next) },
    { label: first ? 'Khác mật khẩu tạm' : 'Khác mật khẩu hiện tại', ok: next.length > 0 && next !== current },
    { label: 'Hai ô nhập khớp nhau', ok: next.length > 0 && next === repeat },
  ]
}
