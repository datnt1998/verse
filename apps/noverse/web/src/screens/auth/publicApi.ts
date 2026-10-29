// POST công khai cho màn chưa đăng nhập. 401 ở /api/login (sai mật khẩu) và /api/pair (mã hỏng)
// là trạng thái giao diện, KHÔNG phải phiên hết hạn — không đi qua handler 401 toàn cục của App.
import { ApiError } from '../../api/client'
import type { User } from '../../api/types'

async function publicPost<T>(path: string, body: object): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      credentials: 'same-origin', method: 'POST',
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
    })
  } catch { throw new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.') }
  if (!response.ok) {
    const payload: unknown = await response.json().catch(() => null)
    const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string'
      ? payload.error : response.statusText
    const retry = payload && typeof payload === 'object' && 'retryAfter' in payload && typeof payload.retryAfter === 'number'
      ? payload.retryAfter : undefined
    throw new ApiError(response.status, message || `Lỗi HTTP ${response.status}`, retry)
  }
  return response.json() as Promise<T>
}

export const loginPublic = (username: string, password: string) =>
  publicPost<{ user: User }>('/api/login', { username, password })

export const pairPublic = (code: string) => publicPost<{ user: User }>('/api/pair', { code })
