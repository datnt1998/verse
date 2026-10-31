// @vitest-environment happy-dom
// Đổi mật khẩu xong: máy chủ đã thu hồi mọi phiên + xoá cookie → UI hiện "đổi
// thành công, đăng nhập lại" và endSession ngay — KHÔNG refresh /api/me (sẽ 401
// như thể đổi thất bại).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const mocks = vi.hoisted(() => ({
  changePassword: vi.fn(), endSession: vi.fn(), refresh: vi.fn(),
  toast: vi.fn(), onClose: vi.fn(),
}))
vi.mock('../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/client')>()),
  changePassword: mocks.changePassword,
}))
vi.mock('../../api/session', () => ({
  useSession: () => ({ user: { username: 'linh', name: 'Linh', role: 'member', version: 'v1' }, refresh: mocks.refresh, endSession: mocks.endSession }),
}))
vi.mock('../../data', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../data')>()),
  keys: { sessions: () => ['sessions'], users: () => ['users'] },
  invalidate: vi.fn(async () => {}),
  useConnection: () => ({ writable: true }),
  useResource: () => ({ data: { sessions: [] } }),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: mocks.toast, Icon: () => null }))
vi.mock('../../ui/confirmPresets', () => ({ confirmAllDevicesLogout: vi.fn(), confirmDeviceLogout: vi.fn() }))
vi.mock('../../ui/useUnsavedGuard', () => ({ useUnsavedGuard: vi.fn() }))
import { DevicesDialog } from './DevicesDialog'

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
const submit = async () => {
  const button = [...document.querySelectorAll<HTMLButtonElement>('button')].find((el) => el.textContent === 'Đổi mật khẩu')
  if (!button) throw new Error('Missing password button')
  await act(async () => { button.click() })
}

beforeEach(async () => {
  vi.clearAllMocks()
  mocks.changePassword.mockResolvedValue({ ok: true, reauthenticate: true })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<DevicesDialog onClose={mocks.onClose} />))
})
afterEach(async () => { await act(async () => root.unmount()); document.body.replaceChildren() })

describe('đổi mật khẩu', () => {
  it('reauthenticate → toast báo thành công, endSession, không refresh-me-then-401', async () => {
    await fill('pw-cur', 'cu-khoa-123')
    await fill('pw-new', 'mat-khau-moi-123')
    await fill('pw-re', 'mat-khau-moi-123')
    await submit()
    expect(mocks.changePassword).toHaveBeenCalledWith('cu-khoa-123', 'mat-khau-moi-123', 'v1')
    expect(mocks.toast).toHaveBeenCalledWith('Đã đổi mật khẩu. Vui lòng đăng nhập lại.', 'ok')
    expect(mocks.endSession).toHaveBeenCalled()
    expect(mocks.onClose).toHaveBeenCalled()
    expect(mocks.refresh).not.toHaveBeenCalled()
  })
  it('đổi lỗi → hiện lỗi, không endSession', async () => {
    const { ApiError } = await import('../../api/client')
    mocks.changePassword.mockRejectedValue(new ApiError(403, 'Mật khẩu hiện tại chưa đúng'))
    await fill('pw-cur', 'sai-roi')
    await fill('pw-new', 'mat-khau-moi-123')
    await fill('pw-re', 'mat-khau-moi-123')
    await submit()
    expect(mocks.endSession).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Mật khẩu hiện tại chưa đúng')
  })
})
