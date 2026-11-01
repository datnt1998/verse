// @vitest-environment happy-dom
// Màn bị chặn (403 app_access_denied): giữ user đang đăng nhập, đúng câu thông
// báo hợp đồng; Thử lại đọc lại /api/me; Đăng xuất revoke phiên trên máy chủ
// (logout thật, thoát cả hai app — không nút đổi tài khoản giấu phiên).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SessionProvider, useSession, type SessionState } from '../../api/session'
import { AccessDeniedScreen } from './AccessDeniedScreen'
import { t } from '../../i18n/all'
import type { User } from '../../api/types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const DENIED_USER: User = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member' }, createdAt: '2026-10-29T00:00:00Z' }
const deniedBody = { code: 'app_access_denied', app: 'viderse', error: t.auth.accessDenied, user: DENIED_USER }

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const stubFetch = (impl: (path: string) => Promise<Response> | Response) => {
  const fn = vi.fn((path: string) => Promise.resolve(impl(path)))
  vi.stubGlobal('fetch', fn)
  return fn
}

let latest: SessionState
function Probe() {
  latest = useSession()
  return null
}

const roots: Root[] = []
async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<SessionProvider><Probe /><AccessDeniedScreen /></SessionProvider>) })
  return container
}

const buttonWithText = (container: HTMLElement, text: string) =>
  [...container.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes(text))

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('màn bị chặn quyền app', () => {
  it('hiện user đang đăng nhập + đúng câu chưa cấp quyền + gợi ý thoát cả hai app', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.auth.accessDenied)
    expect(container.textContent).toContain('@linh')
    expect(container.textContent).toContain(t.account.sharedLogoutNote)
    // Có Thử lại + Đăng xuất; KHÔNG có nút đổi tài khoản/bỏ qua.
    expect(buttonWithText(container, t.common.retry)).toBeTruthy()
    expect(buttonWithText(container, t.account.logout)).toBeTruthy()
    expect(container.textContent).not.toContain('tài khoản khác')
  })

  it('Thử lại: được cấp quyền rồi → denied xoá, user vào app', async () => {
    let granted = false
    const fn = stubFetch((path) => {
      if (path === '/api/me') return granted ? respond({ user: DENIED_USER }) : respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(latest.accessDenied).not.toBeNull()
    const callsBefore = fn.mock.calls.length
    granted = true
    await act(async () => { buttonWithText(container, t.common.retry)!.click() })
    expect(fn.mock.calls.length).toBeGreaterThan(callsBefore)
    expect(latest.accessDenied).toBeNull()
    expect(latest.user?.username).toBe('linh')
  })

  it('Thử lại: phiên đã hết (401) → về cổng chưa đăng nhập', async () => {
    let live = false
    stubFetch((path) => {
      if (path === '/api/me') return live ? respond({ error: 'hết phiên' }, 401) : respond(deniedBody, 403)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    live = true
    await act(async () => { buttonWithText(container, t.common.retry)!.click() })
    expect(latest.accessDenied).toBeNull()
    expect(latest.user).toBeNull()
  })

  it('Đăng xuất: POST /api/logout rồi dọn phiên UI về chưa đăng nhập', async () => {
    const fn = stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      if (path === '/api/logout') return respond({ ok: true })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    await act(async () => { buttonWithText(container, t.account.logout)!.click() })
    expect(fn.mock.calls.some(([p]) => p === '/api/logout')).toBe(true)
    expect(latest.user).toBeNull()
    expect(latest.accessDenied).toBeNull()
  })

  it('Đăng xuất lỗi revoke → báo lỗi, giữ nguyên màn chặn (không báo giả đã thoát)', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      if (path === '/api/logout') return respond({ error: 'kho tài khoản tạm lỗi' }, 500)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    await act(async () => { buttonWithText(container, t.account.logout)!.click() })
    expect(container.textContent).toContain(t.account.logoutFailed)
    expect(latest.accessDenied?.username).toBe('linh')
  })
})
