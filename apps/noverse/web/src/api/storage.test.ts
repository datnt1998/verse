// @vitest-environment happy-dom
// Phiên hết hạn (09-auth-onboard.md › F1b): 401 chỉ mang nghĩa "hết hạn" khi trình duyệt
// này từng có phiên; trình duyệt sạch thì chỉ là chưa đăng nhập.
import { describe, expect, it } from 'vitest'
import { getLastRoute, loginPath, safeHashPath, sessionExpired, setLastRoute, takeNextRoute } from './storage'

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

// ainovel.next chỉ dùng một lần và luôn qua bộ lọc đường nội bộ trước khi điều hướng.
describe('next route sau đăng nhập', () => {
  it('trả về lần đầu rồi xoá luôn', () => {
    sessionStorage.setItem('ainovel.next', '#/p/abc')
    expect(takeNextRoute()).toBe('/p/abc')
    expect(takeNextRoute()).toBeNull()
  })
  it('rỗng trả null', () => {
    expect(takeNextRoute()).toBeNull()
    sessionStorage.setItem('ainovel.next', '')
    expect(takeNextRoute()).toBeNull()
  })
})

describe('đường quay lại sau đăng nhập', () => {
  const rejected: Array<string | null> = [
    // Đường ngoài / scheme / protocol-relative / backslash đều bị loại.
    '//evil.example', '#//evil.example', '/\\\\evil',
    'javascript:alert(1)', '#javascript:alert(1)',
    'https://evil.example/#/p/abc', '#https://evil.example',
    // Bypass đã mã hoá một lần: giải mã ra //, \\ hoặc đường không bắt đầu bằng /.
    '#/%2f%2fevil.example', '/%2fevil', '#%5C%5Cevil', '#%2F%2Fevil', '#/p%2f%2fevil',
    // Mã hoá hỏng (chuỗi % không hoàn chỉnh) cũng bị loại.
    '#/p/%e0%a4%a',
    null, '', 'abc', 'relative/path',
  ]
  it.each(rejected)('loại %j thay vì điều hướng ra ngoài', (input) => {
    expect(safeHashPath(input)).toBeNull()
  })
  it.each([
    ['#/p/abc/s/xyz/work', '/p/abc/s/xyz/work'],
    ['/settings', '/settings'],
    ['#/', '/'],
    ['/', '/'],
    // Tên dự án/truyện đã mã hoá hợp lệ vẫn đi qua — route nội bộ.
    ['#/p/abc%20def', '/p/abc%20def'],
    ['#/p/x%2Ey', '/p/x%2Ey'],
  ])('giữ đường nội bộ %j', (input, expected) => {
    expect(safeHashPath(input)).toBe(expected)
  })
  it('takeNextRoute loại đường ngoài thay vì điều hướng ra ngoài', () => {
    for (const bad of ['https://evil.example/#/p/abc', '#//evil.example', '#/%2f%2fevil.example', 'javascript:alert(1)']) {
      sessionStorage.setItem('ainovel.next', bad)
      expect(takeNextRoute()).toBeNull()
    }
  })
  it('last-route đọc qua bộ lọc trước khi dùng', () => {
    setLastRoute('linh', '#/p/abc')
    expect(safeHashPath(getLastRoute('linh'))).toBe('/p/abc')
    localStorage.setItem('ainovel.lastRoute.linh', 'javascript:alert(1)')
    expect(safeHashPath(getLastRoute('linh'))).toBeNull()
  })
})
