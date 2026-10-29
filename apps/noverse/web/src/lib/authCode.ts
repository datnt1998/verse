/**
 * Tách mã xác thực từ thứ người dùng dán ở luồng đăng nhập dán mã (Claude Pro/Max) — cùng quy tắc với
 * `auth.ParseAuthorizationInput` (Go): URL chuyển hướng đầy đủ, chuỗi query `code=…&state=…`,
 * hoặc mã thô `code#state` (phần sau `#` thứ hai bị bỏ). Trả null khi trống hoặc không có code.
 */
export function parseAuthCode(input: string): { code: string; state: string } | null {
  const value = input.trim()
  if (!value) return null
  let code = ''
  let state = ''
  let url: URL | null = null
  try { url = new URL(value) } catch { url = null }
  if (url && url.host) {
    code = url.searchParams.get('code') ?? ''
    state = url.searchParams.get('state') ?? ''
  } else if (value.includes('code=')) {
    const query = new URLSearchParams(value.replace(/^[?#]+/, ''))
    code = query.get('code') ?? ''
    state = query.get('state') ?? ''
  } else {
    const [head, tail = ''] = value.split('#')
    code = head
    state = tail
  }
  return code ? { code, state } : null
}

export const PASTE_CODE_PLACEHOLDER = 'Dán mã từ trang Claude'
export const PASTE_CODE_EMPTY_ERROR = 'Không tìm thấy mã xác thực — hãy dán nguyên mã (dạng code#state) hoặc cả URL chuyển hướng'
