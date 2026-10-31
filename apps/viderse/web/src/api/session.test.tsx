// @vitest-environment happy-dom
// SessionProvider trên kho tài khoản chung: phân loại chặt phản hồi /api/me —
// chỉ 401 mới tới setup/login; 403 app_access_denied giữ user + màn bị chặn;
// 403 password_change_required đưa về cổng đổi mật khẩu; 503/mất mạng là
// serverDown, không bao giờ setup/login giả. Tín hiệu từ request khác
// (không phải /api/me) cũng đổi cổng qua sessionSignal.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SessionProvider, useSession, type SessionState } from './session'
import { request } from './request'
import type { User } from './types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const USER: User = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member', viderse: 'member' }, createdAt: '2026-10-29T00:00:00Z' }
const DENIED_USER: User = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member' }, createdAt: '2026-10-29T00:00:00Z' }
const FORCED_USER: User = { ...USER, mustChangePassword: true }

const deniedBody = { code: 'app_access_denied', app: 'viderse', error: 'chưa được cấp quyền', user: DENIED_USER }
const forcedBody = { code: 'password_change_required', mustChangePassword: true, error: 'cần đổi mật khẩu tạm' }

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const stubFetch = (impl: (path: string, init?: RequestInit) => Promise<Response> | Response) => {
  const fn = vi.fn((path: string, init?: RequestInit) => Promise.resolve(impl(path, init)))
  vi.stubGlobal('fetch', fn)
  return fn
}

let latest: SessionState
function Probe() {
  latest = useSession()
  return null
}

const roots: Root[] = []
async function mount() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<SessionProvider><Probe /></SessionProvider>) })
  return container
}

beforeEach(() => { location.hash = '#/' })

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('phân loại /api/me lúc khởi động', () => {
  it('200 → user (kèm grants), không accessDenied, không phải đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.user?.username).toBe('linh')
    expect(latest.user?.grants).toEqual({ noverse: 'member', viderse: 'member' })
    expect(latest.accessDenied).toBeNull()
    expect(latest.mustChangePassword).toBe(false)
    expect(latest.setupNeeded).toBe(false)
    expect(latest.serverReachable).toBe(true)
  })

  it('403 app_access_denied → giữ user + accessDenied, KHÔNG hỏi /api/setup', async () => {
    const fn = stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.accessDenied?.username).toBe('linh')
    expect(latest.user?.username).toBe('linh')
    expect(fn.mock.calls.filter(([p]) => p === '/api/setup')).toHaveLength(0)
    expect(latest.serverReachable).toBe(true)
  })

  it('200 với user.mustChangePassword → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: FORCED_USER })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.user?.username).toBe('linh')
    expect(latest.mustChangePassword).toBe(true)
  })

  it('403 password_change_required trên /api/me → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(forcedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.mustChangePassword).toBe(true)
    expect(latest.serverReachable).toBe(true)
  })

  it('401 + setup needed → cổng Setup', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: true, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.user).toBeNull()
    expect(latest.setupNeeded).toBe(true)
    expect(latest.serverReachable).toBe(true)
  })

  it('503 /api/me → serverDown: giữ serverReachable=false, KHÔNG hỏi /api/setup, không user giả', async () => {
    const fn = stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'kho tài khoản tạm lỗi' }, 503)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.serverReachable).toBe(false)
    expect(latest.setupNeeded).toBe(false)
    expect(latest.user).toBeNull()
    expect(fn.mock.calls.filter(([p]) => p === '/api/setup')).toHaveLength(0)
  })

  it('mất kết nối (fetch reject) → serverDown', async () => {
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')))
    await mount()
    expect(latest.serverReachable).toBe(false)
    expect(latest.setupNeeded).toBe(false)
  })

  it('401 rồi /api/setup 503 → vẫn serverDown (setup 503 ≠ store trống)', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ error: 'kho tài khoản tạm lỗi' }, 503)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.serverReachable).toBe(false)
    expect(latest.setupNeeded).toBe(false)
  })
})

describe('tín hiệu từ request khác', () => {
  it('403 app_access_denied trên route nghiệp vụ → cổng bị chặn, giữ user', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/status') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await request('/api/status').catch(() => {}) })
    expect(latest.accessDenied?.username).toBe('linh')
    expect(latest.user?.username).toBe('linh')
  })

  it('403 password_change_required trên route nghiệp vụ → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/status') return respond(forcedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.mustChangePassword).toBe(false)
    await act(async () => { await request('/api/status').catch(() => {}) })
    expect(latest.mustChangePassword).toBe(true)
  })

  it('403 nghiệp vụ thường (không code) KHÔNG đổi cổng, KHÔNG hết phiên', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/status') return respond({ error: 'thiếu quyền thao tác' }, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await request('/api/status').catch(() => {}) })
    expect(latest.user?.username).toBe('linh')
    expect(latest.accessDenied).toBeNull()
    expect(latest.mustChangePassword).toBe(false)
  })

  it('401 trên route nghiệp vụ → hết phiên, dọn user', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/status') return respond({ error: 'hết phiên' }, 401)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await request('/api/status').catch(() => {}) })
    expect(latest.user).toBeNull()
  })
})

describe('refresh / signIn / signOut', () => {
  it('refresh sau khi được cấp quyền → denied xoá, vào app', async () => {
    let granted = false
    stubFetch((path) => {
      if (path === '/api/me') return granted ? respond({ user: USER }) : respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    expect(latest.accessDenied).not.toBeNull()
    granted = true
    let outcome: string | undefined
    await act(async () => { outcome = await latest.refresh() })
    expect(outcome).toBe('user')
    expect(latest.accessDenied).toBeNull()
    expect(latest.user?.username).toBe('linh')
  })

  it('refresh vẫn denied → outcome denied, giữ người đăng nhập', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    let outcome: string | undefined
    await act(async () => { outcome = await latest.refresh() })
    expect(outcome).toBe('denied')
    expect(latest.accessDenied?.username).toBe('linh')
  })

  it('refresh khi phiên đã mất (401) → anon, dọn user', async () => {
    let live = true
    stubFetch((path) => {
      if (path === '/api/me') return live ? respond({ user: USER }) : respond({ error: 'hết phiên' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    live = false
    let outcome: string | undefined
    await act(async () => { outcome = await latest.refresh() })
    expect(outcome).toBe('anon')
    expect(latest.user).toBeNull()
  })

  it('signIn 403 app_access_denied → màn bị chặn (không ném lỗi form)', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await latest.signIn('linh', 'matkhau123') })
    expect(latest.accessDenied?.username).toBe('linh')
    expect(latest.user?.username).toBe('linh')
  })

  it('signIn 401 → ném lỗi để form hiển thị, không đổi cổng', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') return respond({ error: 'sai tên đăng nhập hoặc mật khẩu' }, 401)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    let outcome: unknown
    await act(async () => { outcome = await latest.signIn('linh', 'sai').then(() => 'ok', (e: { status: number }) => e.status) })
    expect(outcome).toBe(401)
    expect(latest.user).toBeNull()
    expect(latest.accessDenied).toBeNull()
  })

  it('signIn 200 với mustChangePassword → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') return respond({ user: FORCED_USER })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await latest.signIn('linh', 'temp-pass-1') })
    expect(latest.mustChangePassword).toBe(true)
    expect(latest.user?.username).toBe('linh')
  })

  it('signOut gọi /api/logout rồi dọn phiên UI', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/logout') return respond({ ok: true })
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    await act(async () => { await latest.signOut() })
    expect(latest.user).toBeNull()
  })

  it('signOut lỗi revoke (500) → ném lỗi, GIỮ phiên UI (không báo giả đã thoát)', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: USER })
      if (path === '/api/logout') return respond({ error: 'kho tài khoản tạm lỗi' }, 500)
      throw new Error(`fetch lạ: ${path}`)
    })
    await mount()
    let outcome: unknown
    await act(async () => { outcome = await latest.signOut().then(() => 'ok', (e: { status: number }) => e.status) })
    expect(outcome).toBe(500)
    expect(latest.user?.username).toBe('linh')
  })
})
