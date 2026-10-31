import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, accessDeniedUser, changePassword, getSetup, login, me, mySessions, passwordChangeRequired, revokeSession, verifySetup } from './client'

afterEach(() => vi.unstubAllGlobals())

const stubFetch = (impl: (...args: unknown[]) => Promise<Response>) => {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

describe('ánh xạ lỗi server', () => {
  it('lấy nguyên message {"error": "…"} của server, viết hoa chữ đầu', async () => {
    stubFetch(() => Promise.resolve(Response.json({ error: 'sai tên đăng nhập hoặc mật khẩu' }, { status: 401 })))
    await expect(me()).rejects.toMatchObject({ status: 401, message: 'Sai tên đăng nhập hoặc mật khẩu' })
  })

  it('mất kết nối (fetch reject) → ApiError status 0 với thông báo thử lại', async () => {
    stubFetch(() => Promise.reject(new TypeError('network down')))
    await expect(me()).rejects.toMatchObject({ status: 0, message: 'Không kết nối được máy chủ. Vui lòng thử lại.' })
  })

  it('body không phải JSON lỗi → rơi về statusText', async () => {
    stubFetch(() => Promise.resolve(new Response('bad gateway', { status: 502, statusText: 'Bad Gateway' })))
    await expect(me()).rejects.toMatchObject({ status: 502, message: 'Bad Gateway' })
  })

  it('423 giữ payload (lockedUntil…) và error; 429 giữ retryAfter', async () => {
    stubFetch(() => Promise.resolve(Response.json({ error: 'đã thử sai quá nhiều', lockedUntil: '2026-10-29T21:44:00Z' }, { status: 423 })))
    const err = await verifySetup('AAAA-BBBB').then(() => null, (e: unknown) => e) as ApiError
    expect(err.status).toBe(423)
    expect(err.payload).toMatchObject({ lockedUntil: '2026-10-29T21:44:00Z' })

    stubFetch(() => Promise.resolve(Response.json({ error: 'quá nhiều lần thử', retryAfter: 60 }, { status: 429 })))
    await expect(login('a', 'b')).rejects.toMatchObject({ status: 429, retryAfter: 60 })
  })

  it('403 app_access_denied giữ user an toàn trong payload; mã khác thì không', async () => {
    const denied = { code: 'app_access_denied', app: 'viderse', error: 'chưa được cấp quyền', user: { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member' } } }
    stubFetch(() => Promise.resolve(Response.json(denied, { status: 403 })))
    const err = await me().then(() => null, (e: unknown) => e) as ApiError
    expect(err.status).toBe(403)
    expect(accessDeniedUser(err)).toMatchObject({ username: 'linh', grants: { noverse: 'member' } })
    expect(passwordChangeRequired(err)).toBe(false)
    // 403 không mã cổng: không phải denied, không phải ép đổi mật khẩu.
    stubFetch(() => Promise.resolve(Response.json({ error: 'thiếu quyền thao tác' }, { status: 403 })))
    const plain = await me().then(() => null, (e: unknown) => e) as ApiError
    expect(accessDeniedUser(plain)).toBeNull()
    expect(passwordChangeRequired(plain)).toBe(false)
    // 401 mang code giống nhau cũng không được nhận là denied.
    stubFetch(() => Promise.resolve(Response.json(denied, { status: 401 })))
    const wrongStatus = await me().then(() => null, (e: unknown) => e) as ApiError
    expect(accessDeniedUser(wrongStatus)).toBeNull()
  })

  it('403 password_change_required nhận đúng mã', async () => {
    stubFetch(() => Promise.resolve(Response.json({ code: 'password_change_required', mustChangePassword: true, error: 'cần đổi mật khẩu tạm' }, { status: 403 })))
    const err = await me().then(() => null, (e: unknown) => e) as ApiError
    expect(passwordChangeRequired(err)).toBe(true)
    expect(accessDeniedUser(err)).toBeNull()
  })

  it('POST /api/me/password gửi {current,new}; reauthenticate đi cùng 200', async () => {
    const fn = stubFetch(() => Promise.resolve(Response.json({ ok: true, reauthenticate: true })))
    await expect(changePassword('cu-123', 'moi-456')).resolves.toMatchObject({ ok: true, reauthenticate: true })
    const [path, init] = fn.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/me/password')
    expect(JSON.parse(init.body as string)).toEqual({ current: 'cu-123', new: 'moi-456' })
  })

  it('GET /api/me/sessions và POST /api/sessions/{id}/revoke theo hợp đồng', async () => {
    const sessions = [{ id: 's1', device: 'Mac', createdAt: '2026-10-29T00:00:00Z', lastSeen: '2026-10-29T01:00:00Z', expiresAt: '2026-11-28T00:00:00Z', current: true }]
    const fn = stubFetch((path) => Promise.resolve(
      path === '/api/me/sessions' ? Response.json({ sessions }) : Response.json({ ok: true }),
    ))
    await expect(mySessions()).resolves.toEqual({ sessions })
    await revokeSession('s1')
    const [revokePath, revokeInit] = fn.mock.calls[1] as unknown as [string, RequestInit]
    expect(revokePath).toBe('/api/sessions/s1/revoke')
    expect(revokeInit.method).toBe('POST')
    expect(JSON.parse(revokeInit.body as string)).toEqual({})
  })
})

describe('dạng request', () => {
  it('POST kèm JSON body + Content-Type, cookie same-origin', async () => {
    const fn = stubFetch(() => Promise.resolve(Response.json({ user: { username: 'a', name: 'A', role: 'admin' } })))
    await login('admin', 'matkhau123')
    const [path, init] = fn.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/login')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('same-origin')
    expect(init.headers).toMatchObject({ 'Content-Type': 'application/json' })
    expect(JSON.parse(init.body as string)).toEqual({ username: 'admin', password: 'matkhau123' })
  })

  it('GET không kèm body', async () => {
    const fn = stubFetch(() => Promise.resolve(Response.json({ needed: true, attemptsLeft: 5 })))
    await getSetup()
    const [path, init] = fn.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/setup')
    expect(init.method).toBe('GET')
    expect(init.body).toBeUndefined()
  })
})
