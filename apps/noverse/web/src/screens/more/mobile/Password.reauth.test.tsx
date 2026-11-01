// @vitest-environment happy-dom
// Đổi mật khẩu xong (mobile): máy chủ đã thu hồi mọi phiên kể cả phiên này →
// hiện "đổi thành công, đăng nhập lại" và endSession; KHÔNG gọi onDone/invalidate
// rồi nhận 401 như thể đổi thất bại.
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const mocks = vi.hoisted(() => ({
  changePassword: vi.fn(), endSession: vi.fn(), toast: vi.fn(), onDone: vi.fn(), invalidate: vi.fn(async () => {}),
}))
vi.mock('../../../api/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api/client')>()),
  changePassword: mocks.changePassword,
}))
vi.mock('../../../api/session', () => ({
  useSession: () => ({ user: { username: 'linh', name: 'Linh', role: 'member' }, endSession: mocks.endSession }),
}))
vi.mock('../../../mobile', () => ({
  MobilePage: ({ children, title, footer }: { children?: ReactNode; title?: string; footer?: ReactNode }) => <div data-page={title}>{children}{footer}</div>,
  useMobileToast: () => mocks.toast,
}))
vi.mock('../../../mobile/runtime', () => ({ useTabMode: vi.fn() }))
vi.mock('@noverse/ui/data', async (importOriginal) => ({ ...(await importOriginal<object>()), invalidate: mocks.invalidate }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null }))
import { MobilePassword } from './Password'

let root: Root
let host: HTMLDivElement
const fill = async (idx: number, value: string) => {
  const input = document.querySelectorAll<HTMLInputElement>('.mm-field')[idx]
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
  })
}

beforeEach(async () => {
  vi.clearAllMocks()
  Object.defineProperty(navigator, 'onLine', { value: true, configurable: true })
  mocks.changePassword.mockResolvedValue({ ok: true, reauthenticate: true })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<MobilePassword onDone={mocks.onDone} />))
})
afterEach(async () => { await act(async () => root.unmount()); document.body.replaceChildren() })

describe('đổi mật khẩu mobile', () => {
  it('reauthenticate → toast báo đăng nhập lại + endSession, không onDone', async () => {
    await fill(0, 'cu-khoa-123')
    await fill(1, 'mat-khau-moi-123')
    await fill(2, 'mat-khau-moi-123')
    const button = [...document.querySelectorAll<HTMLButtonElement>('.mm-primary')][0]
    await act(async () => { button.click() })
    expect(mocks.changePassword).toHaveBeenCalledWith('cu-khoa-123', 'mat-khau-moi-123')
    expect(mocks.toast).toHaveBeenCalledWith('Đã đổi mật khẩu. Vui lòng đăng nhập lại.')
    expect(mocks.endSession).toHaveBeenCalled()
    expect(mocks.onDone).not.toHaveBeenCalled()
    expect(mocks.invalidate).not.toHaveBeenCalled()
  })
})
