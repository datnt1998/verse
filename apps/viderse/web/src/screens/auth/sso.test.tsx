// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { SessionProvider } from '../../api/session'
import { AppHeader } from '../../shell/AppHeader'
import { LoginScreen } from './LoginScreen'
import { AccessDeniedScreen } from './AccessDeniedScreen'
import { AccountScreen } from '../account/AccountScreen'
import { t } from '../../i18n/all'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
const member = { username: 'linh', name: 'Linh', role: 'member', grants: { viderse: 'member' } }
const links = [{ id: 'viderse', name: 'Viderse', origin: 'https://vid.example.test' }, { id: 'noverse', name: 'Noverse', origin: 'https://nov.example.test' }]
function api(body: unknown, status = 200) {
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path === '/api/setup') return Response.json({ needed: false })
    if (path === '/api/me/sessions') return Response.json({ sessions: [] })
    return Response.json(body, { status })
  }))
}
async function mount(children: ReactNode) {
  const el = document.createElement('div'); document.body.append(el)
  const root = createRoot(el); roots.push(root)
  await act(async () => root.render(<SessionProvider>{children}</SessionProvider>))
  return el
}
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren(); location.hash = ''; vi.unstubAllGlobals()
})
it.each([false, true])('switcher uses granted apps only: other=%s', async (other) => {
  api({ user: member, apps: other ? links : links.slice(0, 1) })
  const el = await mount(<AppHeader route={{ name: 'account' }} online userName="Linh" status={undefined} />)
  await act(() => el.querySelector<HTMLElement>('.vv-avatar')!.click())
  const link = el.querySelector<HTMLAnchorElement>('a[href="https://nov.example.test"]')
  expect(Boolean(link)).toBe(other)
  if (link) expect(link.target).toBe('_blank')
  if (!other) expect(el.textContent).not.toContain('Ứng dụng')
})
it.each([
  ['logout', 'noverse', 'Bạn vừa đăng xuất ở Noverse'],
  ['logout', 'viderse', 'Đã đăng xuất'],
  ['password_changed', undefined, 'Mật khẩu vừa được đổi'],
  ['admin_changed', undefined, 'Quản trị viên vừa thay đổi tài khoản này'],
  ['locked', undefined, 'Quản trị viên vừa thay đổi tài khoản này'],
  ['expired', undefined, 'Phiên đăng nhập đã hết hạn'],
  ['future_reason', undefined, 'Phiên đăng nhập đã kết thúc'],
])('401 reason %s maps to the appropriate recovery explanation', async (reason, app, title) => {
  api({ code: 'session_ended', reason, app }, 401)
  const el = await mount(<LoginScreen />)
  expect(el.querySelector('[data-session-ended]')?.textContent).toContain(title)
})
it.each([false, true])('unknown session reason is shown only after a prior sign-in: %s', async (signedInBefore) => {
  localStorage.clear()
  if (signedInBefore) localStorage.setItem('vv.signedInBefore', '1')
  api({ code: 'session_ended', reason: 'unknown' }, 401)
  const el = await mount(<LoginScreen />)
  expect(Boolean(el.querySelector('[data-session-ended]'))).toBe(signedInBefore)
  localStorage.clear()
})
it.each([false, true])('denied screen offers only granted other app: %s', async (other) => {
  api({ code: 'app_access_denied', user: member, apps: other ? links.slice(1) : [] }, 403)
  const el = await mount(<AccessDeniedScreen />)
  expect(Boolean(el.querySelector('a[href="https://nov.example.test"]'))).toBe(other)
})
it.each(['member', 'admin'])('members management is restricted to %s role', async (role) => {
  api({ user: { ...member, role }, apps: links })
  const el = await mount(<AccountScreen />)
  const link = el.querySelector<HTMLAnchorElement>('a[href="https://nov.example.test/#/settings/members"]')
  expect(Boolean(link)).toBe(role === 'admin')
  if (link) expect(link.target).toBe('_blank')
})
it.each([false, true])('grant-aware account explanation: other=%s', async (other) => {
  api({ user: member, apps: other ? links : links.slice(0, 1) })
  const el = await mount(<AccountScreen />)
  const section = el.querySelector('section[aria-label="Ứng dụng của bạn"]')
  expect(section?.textContent).toContain(other ? t.account.memberApps : t.account.noOtherApp)
  expect(section?.textContent).not.toContain(other ? t.account.noOtherApp : t.account.memberApps)
})
it.each([false, true])('mobile app switcher requires another granted app: other=%s', async (other) => {
  api({ user: member, apps: other ? links : links.slice(0, 1) })
  vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener() {}, removeEventListener() {} }))
  const el = await mount(<AccountScreen />)
  expect(Boolean(el.querySelector('a[aria-current="true"]'))).toBe(other)
})
