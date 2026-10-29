// @vitest-environment happy-dom
// A0 · Setup.dc.html — hành vi 8 trạng thái + tự kiểm khi đủ 8 ký tự (09-auth-onboard §11).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../../api/client'
import type { SetupAdminResult, SetupState, SetupVerifyResult, User } from '../../../api/types'
import { SetupScreen } from './index'

const getSetup = vi.fn<() => Promise<SetupState>>()
const verifySetup = vi.fn<(code: string) => Promise<SetupVerifyResult>>()
const createSetupAdmin = vi.fn<(body: unknown) => Promise<SetupAdminResult>>()
const navigate = vi.fn()
const setUser = vi.fn()
const refresh = vi.fn(async () => {})
const setSetupNeeded = vi.fn()

vi.mock('../../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../api/client')>()
  return { ...actual, getSetup: () => getSetup(), verifySetup: (code: string) => verifySetup(code), createSetupAdmin: (body: unknown) => createSetupAdmin(body) }
})
vi.mock('../../../router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../router')>()
  return { ...actual, navigate: (path: string, replace?: boolean) => navigate(path, replace) }
})
vi.mock('../../../api/session', () => ({
  useSession: () => ({ checked: true, user: null, session: null, setUser, refresh, setupNeeded: true, setSetupNeeded }),
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })
beforeEach(() => {
  vi.clearAllMocks()
  getSetup.mockResolvedValue({ needed: true, expiresAt: new Date(Date.now() + 27 * 60_000).toISOString(), attemptsLeft: 5, version: '0.9.3' })
})

const admin: User = {
  username: 'dat', name: 'Đạt', role: 'admin', projects: {}, disabled: false,
  createdAt: '2026-10-25T13:00:00Z', updatedAt: '2026-10-25T13:00:00Z',
  onboardedAt: null, mustChangePassword: false, lastSeenAt: null,
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<SetupScreen />) })
  await act(async () => { await Promise.resolve() })
  return container
}

const typeCode = async (input: HTMLInputElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  await act(async () => {
    setter.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await act(async () => { await Promise.resolve() })
}

describe('Bước 1 · mã thiết lập', () => {
  it('đủ 8 ký tự thì tự kiểm đúng một lần, gửi mã đã chuẩn hoá', async () => {
    verifySetup.mockResolvedValue({ setupToken: 'setup-1', expiresAt: new Date(Date.now() + 27 * 60_000).toISOString() })
    const container = await render()
    const input = container.querySelector<HTMLInputElement>('#setup-code')!
    await typeCode(input, 'k7q29xmb')
    expect(verifySetup).toHaveBeenCalledTimes(1)
    expect(verifySetup).toHaveBeenCalledWith('K7Q2-9XMB')
    expect(container.textContent).toContain('Tạo tài khoản quản trị')
    expect(container.textContent).toContain('Mã đúng. Còn 27 phút để tạo tài khoản.')
    // Chân trang artboard: “Trang thiết lập tự đóng… · Noverse 0.9.3” (version chỉ có khi needed=true).
    expect(container.textContent).toContain('Trang thiết lập tự đóng ngay khi có tài khoản quản trị · Noverse 0.9.3')
  })

  it('máy chủ không trả version thì chân trang bỏ phần “· Noverse …”', async () => {
    getSetup.mockResolvedValue({ needed: true, expiresAt: new Date(Date.now() + 20 * 60_000).toISOString(), attemptsLeft: 5 })
    const container = await render()
    expect(container.textContent).toContain('Trang thiết lập tự đóng ngay khi có tài khoản quản trị')
    expect(container.textContent).not.toContain('Noverse 0.9.3')
  })

  it('mã sai: thông báo nằm trong vùng aria-live polite, nêu số lần thử còn lại', async () => {
    verifySetup.mockRejectedValue(new ApiError(400, 'Mã thiết lập chưa đúng', undefined, { attemptsLeft: 4 }))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'AAAAAAAA')
    const message = container.querySelector('#setup-msg')!
    expect(message.getAttribute('aria-live')).toBe('polite')
    expect(message.textContent).toContain('Mã chưa đúng')
    expect(message.textContent).toContain('Còn 4 lần thử.')
    // Không tự thử lại cùng một mã.
    expect(verifySetup).toHaveBeenCalledTimes(1)
  })

  it('sai 5 lần: khoá 10 phút, câu “Thử lại lúc HH:MM (sau 10 phút)…”, nút đổi thành “Cần mã mới” và bị khoá', async () => {
    const lockedUntil = new Date('2026-10-25T14:14:00Z')
    verifySetup.mockRejectedValue(new ApiError(423, 'Đã thử sai 5 lần', undefined, { lockedUntil: lockedUntil.toISOString() }))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'AAAAAAAA')
    expect(container.textContent).toContain('Đã thử sai 5 lần')
    const clock = `${String(lockedUntil.getHours()).padStart(2, '0')}:${String(lockedUntil.getMinutes()).padStart(2, '0')}`
    expect(container.textContent).toContain(`Thử lại lúc ${clock} (sau 10 phút). Muốn nhanh hơn: khởi động lại Noverse để có mã mới và mở khoá ngay.`)
    const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!
    expect(submit.disabled).toBe(true)
    expect(submit.textContent).toContain('Cần mã mới')
  })

  it('GET /api/setup nói đã hết hạn: mở codeExpired dù còn khoá', async () => {
    getSetup.mockResolvedValue({
      needed: true, expired: true, expiresAt: new Date(Date.now() - 60_000).toISOString(),
      lockedUntil: new Date(Date.now() + 5 * 60_000).toISOString(), attemptsLeft: 2, version: '0.9.3',
    })
    const container = await render()
    expect(container.textContent).toContain('Mã đã hết hạn')
    expect(container.textContent).not.toContain('Đã thử sai 5 lần')
  })

  it('mã sai định dạng (400 format:true): báo sai nhưng không trừ lượt thử', async () => {
    verifySetup.mockRejectedValue(new ApiError(400, 'Mã thiết lập sai định dạng', undefined, { format: true }))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'AAAAAAAA')
    const message = container.querySelector('#setup-msg')!
    expect(message.textContent).toContain('Mã chưa đúng')
    // Số lượt của GET /api/setup (5) giữ nguyên — mã sai định dạng không bị tính.
    expect(message.textContent).toContain('Còn 5 lần thử.')
    // Không khoá: vẫn ở trạng thái mã sai, nút còn bấm lại được như codeWrong.
    expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')!.textContent).toContain('Tiếp tục')
  })

  it('mã hết hạn (410): nút khoá, nói rõ vì sao', async () => {
    verifySetup.mockRejectedValue(new ApiError(410, 'Mã thiết lập đã hết hạn'))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'AAAAAAAA')
    expect(container.textContent).toContain('Mã đã hết hạn')
    expect(container.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled).toBe(true)
  })
})

describe('Bước 2 · tài khoản quản trị', () => {
  const fill = async (container: HTMLElement, values: { name: string; username: string; password: string; repeat: string }) => {
    const set = async (selector: string, value: string) => {
      const input = container.querySelector<HTMLInputElement>(selector)!
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
      await act(async () => { setter.call(input, value); input.dispatchEvent(new Event('input', { bubbles: true })) })
    }
    await set('#setup-name', values.name)
    await set('#setup-username', values.username)
    await set('#setup-password', values.password)
    await set('#setup-repeat', values.repeat)
  }

  it('tạo xong: đăng nhập luôn (setUser + refresh) rồi sang A3 bước Hồ sơ', async () => {
    verifySetup.mockResolvedValue({ setupToken: 'setup-1', expiresAt: new Date(Date.now() + 20 * 60_000).toISOString() })
    createSetupAdmin.mockResolvedValue({ user: admin, recoveryCode: 'TRAX-8K2M-QP7D-41ZN' })
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'TRAX41ZN')
    await fill(container, { name: 'Đạt', username: 'dat', password: 'matkhau-123', repeat: 'matkhau-123' })
    const submit = container.querySelector<HTMLButtonElement>('form button[type="submit"]')!
    expect(submit.disabled).toBe(false)
    await act(async () => { submit.click() })
    await act(async () => { await Promise.resolve() })
    expect(createSetupAdmin).toHaveBeenCalledWith({ setupToken: 'setup-1', name: 'Đạt', username: 'dat', password: 'matkhau-123' })
    expect(setUser).toHaveBeenCalledWith(admin)
    expect(setSetupNeeded).toHaveBeenCalledWith(false)
    expect(navigate).toHaveBeenCalledWith('/welcome/profile', true)
  })

  it('mã hết hạn khi tạo (410): giữ nguyên dữ liệu đã điền, chờ mã mới', async () => {
    verifySetup.mockResolvedValue({ setupToken: 'setup-1', expiresAt: new Date(Date.now() + 20 * 60_000).toISOString() })
    createSetupAdmin.mockRejectedValue(new ApiError(410, 'Mã thiết lập đã hết hạn hoặc đã dùng'))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'TRAX41ZN')
    await fill(container, { name: 'Đạt', username: 'dat', password: 'matkhau-123', repeat: 'matkhau-123' })
    await act(async () => { container.querySelector<HTMLButtonElement>('form button[type="submit"]')!.click() })
    await act(async () => { await Promise.resolve() })
    expect(container.textContent).toContain('Mã thiết lập vừa hết hạn')
    expect(container.querySelector<HTMLInputElement>('#setup-name')!.value).toBe('Đạt')
    expect(container.querySelector<HTMLInputElement>('#setup-password')!.value).toBe('matkhau-123')
    const submit = container.querySelector<HTMLButtonElement>('form button[type="submit"]')!
    expect(submit.disabled).toBe(true)
    expect(submit.textContent).toContain('Nhập mã mới để tạo tài khoản')

    // Nhập mã mới → về lại bước tài khoản với dữ liệu cũ.
    verifySetup.mockResolvedValue({ setupToken: 'setup-2', expiresAt: new Date(Date.now() + 20 * 60_000).toISOString() })
    await typeCode(container.querySelector<HTMLInputElement>('#setup-newcode')!, 'TRAX41ZN')
    await act(async () => { [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.includes('Kiểm mã'))!.click() })
    await act(async () => { await Promise.resolve() })
    expect(container.textContent).toContain('Mã đúng. Còn')
    expect(container.querySelector<HTMLInputElement>('#setup-name')!.value).toBe('Đạt')
  })
})

describe('Đã có quản trị viên', () => {
  it('409 khi kiểm mã: thẻ taken nêu mốc giờ và lối sang Đăng nhập, không có dòng phiên bản ở chân trang', async () => {
    const at = new Date('2026-10-25T21:04:00Z')
    verifySetup.mockRejectedValue(new ApiError(409, 'Đã có tài khoản quản trị', undefined, { at: at.toISOString() }))
    const container = await render()
    await typeCode(container.querySelector<HTMLInputElement>('#setup-code')!, 'AAAAAAAA')
    expect(container.textContent).toContain('Noverse đã có quản trị viên')
    const link = container.querySelector<HTMLAnchorElement>('a[href="#/login"]')!
    expect(link.textContent).toContain('Đến trang đăng nhập')
    expect(container.textContent).toContain(`${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`)
    // Câu 2: thẻ taken bỏ hẳn dòng chân trang (cả “Trang thiết lập tự đóng…” lẫn “· Noverse 0.9.3”).
    expect(container.textContent).not.toContain('Trang thiết lập tự đóng')
    expect(container.textContent).not.toContain('Noverse 0.9.3')
  })

  it('máy chủ đã có tài khoản từ trước: trang tự đóng, về Đăng nhập', async () => {
    getSetup.mockResolvedValue({ needed: false, attemptsLeft: 0 })
    await render()
    expect(navigate).toHaveBeenCalledWith('/login', true)
    expect(setSetupNeeded).toHaveBeenCalledWith(false)
  })
})

describe('Logo thanh trên cùng', () => {
  // 23-brand.md §4: mọi màn máy tính 22 px; điện thoại 20 px; không có cỡ 21 px.
  it('biểu tượng cao 22 px', async () => {
    const container = await render()
    const mark = container.querySelector<SVGElement>('header svg')!
    expect(mark.style.height).toBe('22px')
  })
})
