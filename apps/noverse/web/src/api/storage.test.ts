// Phiên hết hạn (09-auth-onboard.md › F1b): 401 chỉ mang nghĩa "hết hạn" khi trình duyệt
// này từng có phiên; trình duyệt sạch thì chỉ là chưa đăng nhập.
import { describe, expect, it } from 'vitest'
import { loginPath, sessionExpired } from './storage'

describe('phiên hết hạn sau 401', () => {
  it('trình duyệt sạch (chưa từng đăng nhập, /api/me 401 lần đầu) → #/login thường', () => {
    expect(sessionExpired(false, 0)).toBe(false)
    expect(loginPath(sessionExpired(false, 0))).toBe('/login')
  })

  it('/api/me đã thành công rồi request sau đó 401 → #/login?expired=1', () => {
    expect(sessionExpired(true, 0)).toBe(true)
    expect(loginPath(sessionExpired(true, 0))).toBe('/login?expired=1')
  })

  it('máy đã từng đăng nhập (ainovel.accounts còn tài khoản) mà cookie mất → #/login?expired=1', () => {
    expect(sessionExpired(false, 1)).toBe(true)
    expect(loginPath(sessionExpired(false, 3))).toBe('/login?expired=1')
  })
})
