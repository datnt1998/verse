// @vitest-environment happy-dom
// Cổng phiên của App trên kho tài khoản chung: stub fetch đúng hợp đồng
// platform/webapp rồi render thật. Chỉ 401 của /api/me mới đưa tới
// Setup/Login; 403 app_access_denied → màn bị chặn (giữ user); user có
// mustChangePassword → màn đổi mật khẩu chặn mọi hash route; 503/mất mạng →
// màn lỗi + Thử lại, không bao giờ Setup/Login giả.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyViderseLabels } from './kitLabels'
import App from './App'
import { t } from './i18n/vi'
import type { User } from './api/types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const USER: User = { username: 'linh', name: 'Linh', role: 'admin', grants: { noverse: 'admin', viderse: 'admin' }, createdAt: '2026-10-29T00:00:00Z' }
const DENIED_USER: User = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member' }, createdAt: '2026-10-29T00:00:00Z' }
const FORCED_USER: User = { ...USER, role: 'member', mustChangePassword: true }

const deniedBody = { code: 'app_access_denied', app: 'viderse', error: t.auth.accessDenied, user: DENIED_USER }
const forcedBody = { code: 'password_change_required', mustChangePassword: true, error: 'Bạn cần đổi mật khẩu tạm trước khi tiếp tục.' }

const roots: Root[] = []

// Cổng phiên là chunk lazy: nạp sẵn để test không phụ thuộc thời gian tải chunk.
await Promise.all([
  import('./screens/auth/SetupScreen'),
  import('./screens/auth/LoginScreen'),
  import('./screens/auth/AccessDeniedScreen'),
  import('./screens/auth/ChangePasswordScreen'),
  import('./screens/auth/ServerDownScreen'),
])

/** Chờ React xử lý xong lượt render của chunk lazy đã nạp sẵn. */
const settle = () => act(async () => {})

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<App />) })
  await settle()
  return container
}

/** Điền ô input điều khiển React: setter native + input event (giống convention các test form). */
const fill = (input: HTMLInputElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

const buttonWithText = (container: HTMLElement, text: string) =>
  [...container.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes(text))

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const stubFetch = (impl: (path: string) => Promise<Response> | Response) => {
  const fn = vi.fn((path: string) => Promise.resolve(impl(path)))
  vi.stubGlobal('fetch', fn)
  return fn
}
/** Stub đầy đủ cho boot đã đăng nhập (shell gắn SSE + poll + các route nền). */
const authedRoutes = (user: User) => (path: string) => {
  if (path === '/api/me') return respond({ user })
  if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
  if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
  if (String(path).startsWith('/api/setup/state')) return respond({ state: 'done', complete: true, running: false, steps: [], disk: { freeBytes: 1, requiredBytes: 1, enough: true } })
  if (String(path).startsWith('/api/channels')) return respond({ channels: [] })
  throw new Error(`fetch lạ: ${path}`)
}

beforeEach(() => {
  // Mặc định desktop: matchMedia trả false cho mọi query.
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
    matches: false, media: query, onchange: null,
    addListener: vi.fn(), removeListener: vi.fn(),
    addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
  })))
  location.hash = '#/'
  applyViderseLabels()
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('cổng phiên', () => {
  it('máy chủ chưa có tài khoản (401 /me + needed) → màn Thiết lập', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: true, attemptsLeft: 5, expiresAt: '2026-10-30T00:00:00Z' })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.setup.title)
    expect(container.querySelector('#setup-code')).toBeTruthy()
  })

  it('đã có tài khoản nhưng chưa đăng nhập (401 /me + needed:false) → màn Đăng nhập', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.querySelector('#lg-user')).toBeTruthy()
    expect(container.textContent).toContain(t.auth.loginTitle)
  })

  it('brand của màn auth là Viderse (logo ống kính + wordmark)', async () => {
    // Đi qua applyViderseLabels() của production (kitLabels.ts) — nhãn kit
    // trả "Viderse"; auth dùng LensLogo, không BrandLockup của kit.
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    // Đây là lockup của auth (không phải chữ trong banner/trạng thái khác):
    // wordmark nằm cạnh SVG logo ngay trong header của AuthShell.
    const header = container.querySelector('header')
    expect(header?.querySelector('svg')).toBeTruthy()
    expect(header?.textContent).toContain('Viderse')
  })

  it('không tới được máy chủ → màn lỗi + Thử lại, KHÔNG màn Đăng nhập/Thiết lập giả', async () => {
    const fn = stubFetch(() => Promise.reject(new TypeError('Failed to fetch')))
    const container = await render()
    expect(container.textContent).toContain(t.auth.serverDownTitle)
    expect(container.querySelector('#lg-user')).toBeNull()
    expect(container.querySelector('#setup-code')).toBeNull()
    // Thử lại gọi lại /api/me (vẫn lỗi → ở lại màn lỗi, không crash).
    const callsBefore = fn.mock.calls.length
    await act(async () => { buttonWithText(container, t.common.retry)!.click() })
    expect(fn.mock.calls.length).toBeGreaterThan(callsBefore)
    expect(container.textContent).toContain(t.auth.serverDownTitle)
  })

  it('503 /api/me (kho tài khoản lỗi) → màn lỗi, KHÔNG màn Thiết lập/Đăng nhập giả', async () => {
    const fn = stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'kho tài khoản tạm lỗi' }, 503)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.auth.accountsDownTitle)
    expect(container.querySelector('#lg-user')).toBeNull()
    expect(container.querySelector('#setup-code')).toBeNull()
    expect(fn.mock.calls.filter(([p]) => p === '/api/setup')).toHaveLength(0)
  })

  it('401 /me nhưng /api/setup 503 → vẫn màn lỗi (setup 503 ≠ store trống)', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ error: 'kho tài khoản tạm lỗi' }, 503)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.auth.accountsDownTitle)
    expect(container.querySelector('#setup-code')).toBeNull()
    expect(container.querySelector('#lg-user')).toBeNull()
  })

  it('403 app_access_denied → màn bị chặn giữ user, không login loop, không gắn SSE', async () => {
    const fn = stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.auth.accessDenied)
    expect(container.textContent).toContain('@linh')
    // Không vòng login: form đăng nhập không render.
    expect(container.querySelector('#lg-user')).toBeNull()
    // Dữ liệu không mount: không /api/events, không /api/status.
    const paths = fn.mock.calls.map(([p]) => String(p))
    expect(paths.some((p) => p.startsWith('/api/events'))).toBe(false)
    expect(paths.some((p) => p === '/api/status')).toBe(false)
  })

  it('Thử lại ở màn bị chặn: đã được cấp quyền → vào shell', async () => {
    let granted = false
    stubFetch((path) => {
      if (path === '/api/me') return granted ? respond({ user: DENIED_USER }) : respond(deniedBody, 403)
      if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
      if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
      if (String(path).startsWith('/api/setup/state')) return respond({ state: 'done', complete: true, running: false, steps: [], disk: { freeBytes: 1, requiredBytes: 1, enough: true } })
      if (String(path).startsWith('/api/channels')) return respond({ channels: [] })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.textContent).toContain(t.auth.accessDenied)
    granted = true
    await act(async () => { buttonWithText(container, t.common.retry)!.click() })
    expect(container.textContent).toContain(t.nav.workbench)
    expect(container.textContent).not.toContain(t.auth.accessDenied)
  })

  it('Đăng xuất ở màn bị chặn → POST /api/logout (revoke) rồi về Đăng nhập', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(deniedBody, 403)
      if (path === '/api/logout') return respond({ ok: true })
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    // Gợi ý phiên chung: thoát Viderse là thoát cả Noverse.
    expect(container.textContent).toContain(t.account.sharedLogoutNote)
    await act(async () => { buttonWithText(container, t.account.logout)!.click() })
    await settle()
    expect(container.querySelector('#lg-user')).toBeTruthy()
  })

  it('user.mustChangePassword → cổng đổi mật khẩu, hash route không vượt được, không SSE', async () => {
    const fn = stubFetch(authedRoutes(FORCED_USER))
    location.hash = '#/settings'
    const container = await render()
    await act(async () => { await new Promise((r) => setTimeout(r, 30)) })
    // Màn đổi mật khẩu thay shell — không nav, không nút bỏ qua.
    expect(container.querySelector('#pw-current')).toBeTruthy()
    expect(container.textContent).toContain(t.auth.passwordForm.forceTitle)
    expect(container.querySelector('nav')).toBeNull()
    const paths = fn.mock.calls.map(([p]) => String(p))
    expect(paths.some((p) => p.startsWith('/api/events'))).toBe(false)
    expect(paths.some((p) => p === '/api/status')).toBe(false)
  })

  it('403 password_change_required trên /api/me → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond(forcedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    expect(container.querySelector('#pw-current')).toBeTruthy()
    expect(container.querySelector('#lg-user')).toBeNull()
  })

  it('đổi mật khẩu xong → phiên UI dọn, về Đăng nhập + thông báo đăng nhập lại', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ user: FORCED_USER })
      if (path === '/api/me/password') return respond({ ok: true, reauthenticate: true })
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
      if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    await act(async () => {
      fill(container.querySelector<HTMLInputElement>('#pw-current')!, 'temp-pass-1')
      fill(container.querySelector<HTMLInputElement>('#pw-next')!, 'matkhau-moi-1')
      fill(container.querySelector<HTMLInputElement>('#pw-confirm')!, 'matkhau-moi-1')
      container.querySelector('form')!.requestSubmit()
    })
    await settle()
    // Không gọi lại /api/me giả sau đổi — thẳng về màn đăng nhập + toast.
    expect(container.querySelector('#lg-user')).toBeTruthy()
    expect(document.body.textContent).toContain(t.auth.passwordForm.changed)
  })

  it('đăng nhập đúng mật khẩu nhưng chưa cấp quyền (403) → màn bị chặn, không báo lỗi form', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') return respond(deniedBody, 403)
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    await act(async () => {
      fill(container.querySelector<HTMLInputElement>('#lg-user')!, 'linh')
      fill(container.querySelector<HTMLInputElement>('#lg-pass')!, 'matkhau123')
      container.querySelector('form')!.requestSubmit()
    })
    await settle()
    expect(container.textContent).toContain(t.auth.accessDenied)
    expect(container.querySelector('#lg-user')).toBeNull()
    expect(container.querySelector('#lg-err')).toBeNull()
  })

  it('đăng nhập với tài khoản bị ép đổi mật khẩu → cổng đổi mật khẩu', async () => {
    stubFetch((path) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') return respond({ user: FORCED_USER })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    await act(async () => {
      fill(container.querySelector<HTMLInputElement>('#lg-user')!, 'linh')
      fill(container.querySelector<HTMLInputElement>('#lg-pass')!, 'temp-pass-1')
      container.querySelector('form')!.requestSubmit()
    })
    await settle()
    expect(container.querySelector('#pw-current')).toBeTruthy()
    expect(container.querySelector('nav')).toBeNull()
  })

  it('có phiên → shell hiện tên người dùng ở màn Tài khoản', async () => {
    stubFetch(authedRoutes(USER))
    location.hash = '#/account'
    const container = await render()
    // AccountScreen là chunk lazy — import() qua vite-node cần macrotask thật.
    await act(async () => { await vi.dynamicImportSettled() })
    expect(container.textContent).toContain('Linh')
    expect(container.textContent).toContain('@linh')
    expect(container.textContent).toContain(t.role.admin)
  })

  it('đăng nhập từ màn Login đưa vào shell', async () => {
    let authed = false
    stubFetch((path) => {
      if (path === '/api/me') return authed ? respond({ user: USER }) : respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') { authed = true; return respond({ user: USER }) }
      if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
      if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
      if (String(path).startsWith('/api/setup/state')) return respond({ state: 'done', complete: true, running: false, steps: [], disk: { freeBytes: 1, requiredBytes: 1, enough: true } })
      if (String(path).startsWith('/api/channels')) return respond({ channels: [] })
      throw new Error(`fetch lạ: ${path}`)
    })
    const container = await render()
    const user = container.querySelector<HTMLInputElement>('#lg-user')!
    const pass = container.querySelector<HTMLInputElement>('#lg-pass')!
    const form = container.querySelector('form')!
    await act(async () => {
      fill(user, 'linh')
      fill(pass, 'matkhau123')
      form.requestSubmit()
    })
    expect(container.textContent).toContain(t.nav.workbench)
  })
})
