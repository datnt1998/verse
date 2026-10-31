// @vitest-environment happy-dom
// Bước mật khẩu của Thiết lập lần đầu: đổi xong máy chủ đã thu hồi mọi phiên →
// toast báo đăng nhập lại + endSession; KHÔNG refresh /api/me rồi 401 như thể
// đổi thất bại.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const mocks = vi.hoisted(() => ({
  changePassword: vi.fn(), endSession: vi.fn(), refresh: vi.fn(async () => 'user' as const),
  toast: vi.fn(), navigate: vi.fn(),
  shared: false, mustChange: true,
}))
vi.mock('../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/client')>()),
  changePassword: mocks.changePassword,
}))
vi.mock('../../api/session', () => ({
  useSession: () => ({
    user: { username: 'linh', name: 'Linh', role: 'member', onboardedAt: null, mustChangePassword: mocks.mustChange, projects: {}, grants: mocks.shared ? { noverse: 'member' } : null },
    session: null, refresh: mocks.refresh, endSession: mocks.endSession, setUser: vi.fn(), pendingLessons: 0,
  }),
}))
vi.mock('../../router', () => ({ navigate: mocks.navigate }))
vi.mock('../../shell/AppShell', () => ({ useMediaQuery: () => true }))
vi.mock('../../data', () => ({
  keys: { me: () => ['me'], library: () => ['library'], config: () => ['config'], sessions: () => ['sessions'], users: () => ['users'] },
  useAction: (fn: (arg?: unknown) => Promise<unknown>) => ({
    run: async (arg?: unknown) => { try { return await fn(arg) } catch (e) { return e } },
    busy: false, pending: false, success: false, error: null,
  }),
  useConnection: () => ({ writable: true }),
  useResource: (_key: unknown, _fetcher: unknown, options?: { enabled?: boolean }) => options?.enabled === false
    ? { data: null, initialLoading: false, refresh: vi.fn() }
    : { data: { projects: [], config: { dirs: {}, autoExport: { onVolume: true, formats: [] } } }, initialLoading: false, refresh: vi.fn() },
  invalidate: vi.fn(async () => {}),
}))
vi.mock('./libraryChoice', () => ({
  useLibraryChoice: () => ({ loading: false, options: [], sel: 'keep', select: vi.fn(), status: '', blocked: false, empty: false, display: null, picker: null, commit: async () => null }),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: mocks.toast }))
vi.mock('../../ui/useUnsavedGuard', () => ({ useUnsavedGuard: vi.fn() }))
import { OnboardScreen } from './index'

let root: Root
let host: HTMLDivElement
const fill = async (id: string, value: string) => {
  const input = document.getElementById(id) as HTMLInputElement
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
  })
}

beforeEach(async () => {
  vi.clearAllMocks()
  mocks.shared = false; mocks.mustChange = true
  mocks.changePassword.mockResolvedValue({ ok: true, reauthenticate: true })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<OnboardScreen step="password" />))
})
afterEach(async () => { await act(async () => root.unmount()); document.body.replaceChildren() })

describe('thiết lập lần đầu · đổi mật khẩu', () => {
  it('reauthenticate → toast báo đăng nhập lại + endSession, không refresh-me-then-401', async () => {
    await fill('ob-pw-cur', 'cu-khoa-123')
    await fill('ob-pw-new', 'mat-khau-moi-123')
    await fill('ob-pw-re', 'mat-khau-moi-123')
    const button = [...document.querySelectorAll<HTMLButtonElement>('button')].find((el) => el.textContent?.includes('Tiếp tục'))
    expect(button).toBeTruthy()
    await act(async () => { button!.click() })
    expect(mocks.changePassword).toHaveBeenCalled()
    expect(mocks.toast).toHaveBeenCalledWith('Đã đổi mật khẩu. Vui lòng đăng nhập lại.')
    expect(mocks.endSession).toHaveBeenCalled()
    expect(mocks.refresh).not.toHaveBeenCalled()
  })
})

it.each([false, true])('đổi mật khẩu riêng tuân theo kho tài khoản, shared=%s', async (shared) => {
  mocks.shared = shared; mocks.mustChange = false
  await act(async () => root.render(<OnboardScreen step="password" />))
  expect(host.textContent).toContain(shared
    ? 'Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả máy này.'
    : 'máy này giữ nguyên phiên')
  await fill('ob-pw-cur', 'mat-khau-cu')
  await fill('ob-pw-new', 'mat-khau-moi')
  await fill('ob-pw-re', 'mat-khau-moi')
  await act(async () => [...host.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.includes('Tiếp tục'))!.click())
  expect(mocks.changePassword).toHaveBeenCalled()
  if (shared) expect(mocks.endSession).toHaveBeenCalledWith('password')
  else expect(mocks.endSession).toHaveBeenCalledWith()
  expect(mocks.refresh).not.toHaveBeenCalled()
})
