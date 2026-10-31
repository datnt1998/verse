// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { Devices } from './Devices'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
const sessions = [
  { id: 'current', device: 'Máy này', current: true, lastSeen: '2026-10-31T09:00:00Z' },
  { id: 'other', device: 'Điện thoại', current: false, lastSeen: '2026-10-31T08:00:00Z' },
]
async function mount(fail: boolean) {
  const fetcher = vi.fn(async (path: string) => {
    if (path === '/api/me/sessions') return Response.json({ sessions })
    return Response.json(fail ? { error: 'Kho tài khoản tạm lỗi' } : { ok: true }, { status: fail ? 503 : 200 })
  })
  vi.stubGlobal('fetch', fetcher)
  const el = document.createElement('div'); document.body.append(el); root = createRoot(el)
  await act(async () => root!.render(<Devices />))
  return { el, fetcher }
}
afterEach(async () => { if (root) await act(() => root!.unmount()); document.body.replaceChildren(); vi.unstubAllGlobals() })
it.each([false, true])('other-device revocation preserves this device, error=%s', async (fail) => {
  const { el, fetcher } = await mount(fail)
  const button = [...el.querySelectorAll('button')].find((item) => item.textContent?.includes('Đăng xuất mọi thiết bị khác'))!
  await act(async () => button.click())
  expect(fetcher.mock.calls.some(([path]) => path === '/api/me/sessions/revoke-all')).toBe(true)
  expect(el.textContent).toContain('Máy này')
  expect(el.textContent?.includes('Điện thoại')).toBe(fail)
  expect(Boolean(el.querySelector('[role="alert"]'))).toBe(fail)
})
it('individual revoke leaves the current session untouched', async () => {
  const { el, fetcher } = await mount(false)
  const button = [...el.querySelectorAll('button')].find((item) => item.textContent === 'Đăng xuất')!
  await act(async () => button.click())
  expect(fetcher.mock.calls.some(([path]) => path === '/api/sessions/other/revoke')).toBe(true)
  expect(fetcher.mock.calls.some(([path]) => path === '/api/sessions/current/revoke')).toBe(false)
  expect(el.textContent).toContain('Máy này')
  expect(el.textContent).not.toContain('Điện thoại')
})
