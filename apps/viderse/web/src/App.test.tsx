// @vitest-environment happy-dom
// Cổng phiên của App: stub fetch đúng hợp đồng platform/webapp rồi render thật.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyViderseLabels } from './kitLabels'
import App from './App'
import { t } from './i18n/vi'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const USER = { username: 'linh', name: 'Linh', role: 'admin', projects: {}, createdAt: '2026-10-29T00:00:00Z' }

const roots: Root[] = []

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<App />) })
  return container
}

beforeEach(() => {
  // Mặc định desktop: matchMedia trả false cho mọi query.
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
    matches: false, media: query,
    addEventListener: () => {}, removeEventListener: () => {},
    addListener: () => {}, removeListener: () => {},
  })))
  location.hash = '#/'
  applyViderseLabels()
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

describe('cổng phiên', () => {
  it('máy chủ chưa có tài khoản (401 /me + needed) → màn Thiết lập', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: true, attemptsLeft: 5, expiresAt: '2026-10-30T00:00:00Z' })
      throw new Error(`fetch lạ: ${path}`)
    }))
    const container = await render()
    expect(container.textContent).toContain(t.setup.title)
    expect(container.querySelector('#setup-code')).toBeTruthy()
  })

  it('đã có tài khoản nhưng chưa đăng nhập (401 /me + needed:false) → màn Đăng nhập', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    }))
    const container = await render()
    expect(container.querySelector('#lg-user')).toBeTruthy()
    expect(container.textContent).toContain(t.auth.loginTitle)
  })

  it('brand của màn auth là Viderse (logo ống kính + wordmark), không có Noverse', async () => {
    // Đi qua applyViderseLabels() của production (kitLabels.ts) — nhãn kit
    // trả "Viderse"; auth dùng LensLogo, không BrandLockup của kit.
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      throw new Error(`fetch lạ: ${path}`)
    }))
    const container = await render()
    const text = container.textContent ?? ''
    expect(text).toContain('Viderse')
    expect(text).not.toContain('Noverse')
    // Đây là lockup của auth (không phải chữ trong banner/trạng thái khác):
    // wordmark nằm cạnh SVG logo ngay trong header của AuthShell.
    const header = container.querySelector('header')
    expect(header?.querySelector('svg')).toBeTruthy()
    expect(header?.textContent).toContain('Viderse')
  })

  it('không tới được máy chủ → màn Đăng nhập báo mất kết nối, không báo "đang hoạt động"', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    const container = await render()
    expect(container.querySelector('#lg-user')).toBeTruthy()
    expect(container.textContent).toContain(t.auth.serverOffline)
    expect(container.textContent).not.toContain(t.auth.serverOnline)
  })

  it('máy chủ trả lỗi HTTP (500 /setup) vẫn là đang trả lời', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ error: 'lỗi máy chủ' }, 500)
      throw new Error(`fetch lạ: ${path}`)
    }))
    const container = await render()
    expect(container.textContent).toContain(t.auth.serverOnline)
  })

  it('có phiên → shell hiện tên người dùng ở màn Tài khoản', async () => {
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return respond({ user: USER })
      if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
      if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
      throw new Error(`fetch lạ: ${path}`)
    }))
    location.hash = '#/account'
    const container = await render()
    // AccountScreen là chunk lazy — import() qua vite-node cần macrotask thật.
    await act(async () => { await new Promise((r) => setTimeout(r, 30)) })
    expect(container.textContent).toContain('Linh')
    expect(container.textContent).toContain('@linh')
    expect(container.textContent).toContain(t.role.admin)
  })

  it('đăng nhập từ màn Login đưa vào shell', async () => {
    let authed = false
    vi.stubGlobal('fetch', vi.fn((path: string) => {
      if (path === '/api/me') return authed ? respond({ user: USER }) : respond({ error: 'chưa đăng nhập' }, 401)
      if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
      if (path === '/api/login') { authed = true; return respond({ user: USER }) }
      if (String(path).startsWith('/api/events')) return new Promise<Response>(() => {})
      if (path === '/api/status') return respond({ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [] })
      throw new Error(`fetch lạ: ${path}`)
    }))
    const container = await render()
    const user = container.querySelector<HTMLInputElement>('#lg-user')!
    const pass = container.querySelector<HTMLInputElement>('#lg-pass')!
    const form = container.querySelector('form')!
    await act(async () => {
      for (const [input, value] of [[user, 'linh'], [pass, 'matkhau123']] as const) {
        // TextField điều khiển bằng React — set qua setter native rồi bắn input event.
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
        setter.call(input, value)
        input.dispatchEvent(new Event('input', { bubbles: true }))
      }
      form.requestSubmit()
    })
    expect(container.textContent).toContain(t.nav.workbench)
  })
})
