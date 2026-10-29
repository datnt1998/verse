import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getSetup, login, me, verifySetup } from './client'

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
