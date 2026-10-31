// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import App from '../../App'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
function fill(input: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}
afterEach(async () => { if (root) await act(() => root!.unmount()); document.body.replaceChildren(); vi.unstubAllGlobals() })
it('keeps the recovery code visible until the administrator confirms saving it', async () => {
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path === '/api/me') return Response.json({ reason: 'unknown' }, { status: 401 })
    if (path === '/api/setup') return Response.json({ needed: true, attemptsLeft: 5 })
    if (path === '/api/setup/verify') return Response.json({ verified: true })
    if (path === '/api/setup/admin') return Response.json({ user: { username: 'dat', name: 'Đạt', role: 'admin', onboarded: true }, recoveryCode: 'TEST-RECOVERY-CODE' })
    return Response.json({})
  }))
  const el = document.createElement('div'); document.body.append(el); root = createRoot(el)
  await act(async () => root!.render(<App />))
  await act(() => fill(el.querySelector('#setup-code')!, 'TEST-SETUP'))
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  await act(() => {
    fill(el.querySelector('#setup-user')!, 'dat')
    fill(el.querySelector('#setup-name')!, 'Đạt')
    fill(el.querySelector('#setup-pass')!, 'password-123')
  })
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(el.textContent).toContain('TEST-RECOVERY-CODE')
  const enter = [...el.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Vào Viderse')!
  expect(enter.disabled).toBe(true)
  await act(() => el.querySelector<HTMLButtonElement>('[role="checkbox"]')!.click())
  expect(enter.disabled).toBe(false)
  expect(el.textContent).toContain('TEST-RECOVERY-CODE')
})
