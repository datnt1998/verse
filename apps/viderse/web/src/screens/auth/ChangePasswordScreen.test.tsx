// @vitest-environment happy-dom
// Cổng đổi mật khẩu (bắt buộc) và form dùng chung: thành công → toast + dọn
// phiên UI (server đã thu hồi mọi phiên), KHÔNG gọi lại /api/me; sai mật khẩu
// hiện tại (403) và lỗi kho (503) hiện hai thông báo khác nhau; không nút bỏ qua.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SessionProvider, useSession, type SessionState } from '../../api/session'
import { ChangePasswordScreen } from './ChangePasswordScreen'
import { ToastHost } from '@noverse/ui'
import { t } from '../../i18n/all'
import type { User } from '../../api/types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const FORCED_USER: User = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member', viderse: 'member' }, mustChangePassword: true }

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
  await act(async () => {
    root.render(<SessionProvider><Probe /><ChangePasswordScreen /><ToastHost /></SessionProvider>)
  })
  return container
}

const fill = (input: HTMLInputElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

const fillForm = (container: HTMLElement, current: string, next: string, confirm: string) => {
  fill(container.querySelector<HTMLInputElement>('#pw-current')!, current)
  fill(container.querySelector<HTMLInputElement>('#pw-next')!, next)
  fill(container.querySelector<HTMLInputElement>('#pw-confirm')!, confirm)
}

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

const passwordFlow = (pwResponse: () => Promise<Response> | Response) => (path: string) => {
  if (path === '/api/me') return respond({ user: FORCED_USER })
  if (path === '/api/me/password') return pwResponse()
  if (path === '/api/setup') return respond({ needed: false, attemptsLeft: 5 })
  throw new Error(`fetch lạ: ${path}`)
}

describe('đổi mật khẩu', () => {
  it('thành công → POST {current,new}, phiên UI dọn, toast đăng nhập lại — không gọi lại /api/me', async () => {
    const fn = stubFetch(passwordFlow(() => respond({ ok: true, reauthenticate: true })))
    const container = await render()
    expect(latest.mustChangePassword).toBe(true)
    await act(async () => {
      fillForm(container, 'temp-pass-1', 'matkhau-moi-1', 'matkhau-moi-1')
      container.querySelector('form')!.requestSubmit()
    })
    const pwCall = fn.mock.calls.find(([p]) => p === '/api/me/password')! as unknown as [string, RequestInit]
    expect(JSON.parse(pwCall[1].body as string)).toEqual({ current: 'temp-pass-1', new: 'matkhau-moi-1' })
    expect(latest.user).toBeNull()
    expect(latest.mustChangePassword).toBe(false)
    expect(document.body.textContent).toContain('Đã đổi mật khẩu. Vui lòng đăng nhập lại.')
    // Không refresh-me-then-fake-failure: /api/me chỉ bị gọi đúng lần boot.
    expect(fn.mock.calls.filter(([p]) => p === '/api/me')).toHaveLength(1)
  })

  it('sai mật khẩu hiện tại (403) → lỗi riêng, giữ phiên và form', async () => {
    stubFetch(passwordFlow(() => respond({ error: 'Mật khẩu hiện tại không đúng' }, 403)))
    const container = await render()
    await act(async () => {
      fillForm(container, 'sai-pass-1', 'matkhau-moi-1', 'matkhau-moi-1')
      container.querySelector('form')!.requestSubmit()
    })
    expect(container.textContent).toContain(t.auth.passwordForm.wrongTemp)
    expect(latest.user?.username).toBe('linh')
    expect(container.querySelector('#pw-current')).toBeTruthy()
  })

  it('kho tài khoản lỗi (503) → thông báo khác lỗi sai mật khẩu, giữ phiên', async () => {
    stubFetch(passwordFlow(() => respond({ error: 'Máy chủ tạm thời không sẵn sàng, vui lòng thử lại sau.' }, 503)))
    const container = await render()
    await act(async () => {
      fillForm(container, 'temp-pass-1', 'matkhau-moi-1', 'matkhau-moi-1')
      container.querySelector('form')!.requestSubmit()
    })
    const text = container.textContent ?? ''
    expect(text).toContain(t.auth.passwordForm.failed)
    expect(text).not.toContain(t.auth.passwordForm.wrongTemp)
    expect(latest.user?.username).toBe('linh')
  })

  it('xác nhận không khớp → không submit, hiện lỗi nhập lại', async () => {
    const fn = stubFetch(passwordFlow(() => respond({ ok: true, reauthenticate: true })))
    const container = await render()
    await act(async () => {
      fillForm(container, 'temp-pass-1', 'matkhau-moi-1', 'khac-matkhau-1')
      container.querySelector('form')!.requestSubmit()
    })
    expect(container.textContent).toContain(t.auth.passwordForm.mismatch)
    expect(fn.mock.calls.filter(([p]) => p === '/api/me/password')).toHaveLength(0)
    expect(latest.user?.username).toBe('linh')
  })

  it('không có nút bỏ qua hay đường tắt nào khác ngoài đổi mật khẩu', async () => {
    stubFetch(passwordFlow(() => respond({ ok: true })))
    const container = await render()
    const buttons = [...container.querySelectorAll('button')].map((b) => b.textContent ?? '')
    expect(buttons.some((label) => label.includes(t.auth.passwordForm.submit))).toBe(true)
    // Đăng xuất được phép; không thể đi vào app mà bỏ qua mật khẩu tạm.
    expect(buttons.some((label) => /bỏ qua|để sau|tài khoản khác/i.test(label))).toBe(false)
  })
  it('mật khẩu quá ngắn đánh dấu ô lỗi và chặn gửi', async () => {
    const fn = stubFetch(passwordFlow(() => respond({ ok: true })))
    const container = await render()
    await act(() => fillForm(container, 'tam-pass', 'short', 'short'))
    expect(container.querySelector('#pw-next')?.getAttribute('aria-invalid')).toBe('true')
    expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
    expect(fn.mock.calls.some(([path]) => path === '/api/me/password')).toBe(false)
  })
})
