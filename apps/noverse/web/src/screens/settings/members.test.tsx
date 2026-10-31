// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '../../api/types'
const fixture = vi.hoisted(() => ({ role: 'admin', created: vi.fn(), updated: vi.fn(), deleted: vi.fn(), audit: vi.fn(), grant: vi.fn(), revokeApp: vi.fn(), resource: { data: { sessions: [], items: [], next: '' }, refresh: vi.fn() } }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { username: 'dat', name: 'Dat', role: fixture.role, projects: {} } }) }))
vi.mock('../../api/library', () => ({ useLibrary: () => ({ library: { projects: [{ id: 'p', name: 'Kênh truyện audio', storyCount: 2 }] }, reload: vi.fn() }) }))
vi.mock('../../api/client', () => ({
  createUser: (...args: unknown[]) => fixture.created(...args), updateUser: (...args: unknown[]) => fixture.updated(...args), deleteUser: (...args: unknown[]) => fixture.deleted(...args), getAdminAudit: (...args: unknown[]) => fixture.audit(...args),
  listUsers: vi.fn(), mySessions: vi.fn(), userSessions: vi.fn(), revokeSession: vi.fn(), revokeUserSessions: vi.fn(),
  grantAppAccess: (...args: unknown[]) => fixture.grant(...args), revokeAppAccess: (...args: unknown[]) => fixture.revokeApp(...args),
}))
vi.mock('../../data', () => ({
  keys: { users: () => ['users'], sessions: () => ['sessions'], library: () => ['library'] }, invalidate: vi.fn(async () => {}),
  useConnection: () => ({ writable: true }), useResource: () => fixture.resource,
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  toast: vi.fn(),  Icon: () => null }))
vi.mock('../../ui/useUnsavedGuard', () => ({ useUnsavedGuard: vi.fn() }))
vi.mock('../account/PairingDialog', () => ({ PairingDialog: () => null }))
import { MembersScreen, suggestUsername } from './members'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const account = (username: string, role: User['role'] = 'member'): User => ({ username, name: username === 'dat' ? 'Dat' : 'Linh', role, projects: {}, disabled: false, createdAt: '2026-10-12', updatedAt: '2026-10-13', onboardedAt: null, lastSeenAt: null, mustChangePassword: false })
let root: Root, host: HTMLDivElement
const buttons = (label: string) => [...document.querySelectorAll<HTMLButtonElement>('button')].filter((b) => b.textContent?.trim() === label && !b.closest('[aria-hidden="true"]'))
const click = async (label: string) => { const button = buttons(label).at(-1); if (!button) throw new Error(`Missing button: ${label}`); await act(async () => { button.click() }) }
const fill = async (label: string, value: string) => {
  const input = [...document.querySelectorAll<HTMLInputElement>('label input')].find((el) => el.closest('label')?.textContent?.startsWith(label))
  if (!input) throw new Error(`Missing input: ${label}`)
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value); input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true })) })
}
beforeEach(async () => {
  fixture.role = 'admin'; vi.clearAllMocks()
  fixture.created.mockResolvedValue(account('ngoc')); fixture.updated.mockResolvedValue(account('linh')); fixture.deleted.mockResolvedValue({ ok: true })
  fixture.grant.mockResolvedValue(account('linh')); fixture.revokeApp.mockResolvedValue(account('linh'))
  window.matchMedia = vi.fn().mockReturnValue({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<MembersScreen users={[account('dat', 'admin'), account('linh')]} refresh={() => {}} />))
})
afterEach(async () => { await act(async () => root.unmount()); document.body.replaceChildren() })
describe('Thành viên §AD', () => {
  it('không render nội dung quản trị cho thành viên kể cả mở liên kết trực tiếp', async () => {
    fixture.role = 'member'
    await act(async () => root.render(<MembersScreen users={[account('linh')]} refresh={() => {}} />))
    expect(host.textContent).toBe('')
  })
  it('gợi ý tên đăng nhập bỏ dấu', () => { expect(suggestUsername('Ngọc Đặng')).toBe('ngoc.dang') })
  it('mật khẩu tạm chỉ hiện một lần và không có trong chi tiết sau khi đóng', async () => {
    await click('Thêm thành viên'); await fill('Tên hiển thị', 'Ngọc')
    const temporary = document.querySelector<HTMLInputElement>('input[aria-label="Mật khẩu tạm"]')!.value
    expect(temporary.length).toBeGreaterThanOrEqual(8)
    await click('Tạo tài khoản')
    expect(fixture.created).toHaveBeenCalledWith('ngoc', 'Ngọc', 'member', {}, temporary)
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain(temporary)
    await click('Xong')
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(host.textContent).not.toContain(temporary)
    await click('Thêm thành viên')
    expect(document.querySelector<HTMLInputElement>('input[aria-label="Mật khẩu tạm"]')!.value).not.toBe(temporary)
  })
  it('luôn có lý do khoá, không bắt buộc, và gửi lý do đã nhập', async () => {
    await click('Khoá')
    expect(document.querySelector('[role="alertdialog"]')?.textContent).toContain('Lý do (ghi vào nhật ký, không bắt buộc)')
    expect(buttons('Khoá tài khoản').at(-1)?.disabled).toBe(false)
    await fill('Lý do', 'Nghỉ cộng tác'); await click('Khoá tài khoản')
    expect(fixture.updated).toHaveBeenCalledWith('linh', { disabled: true, reason: 'Nghỉ cộng tác' }, undefined)
  })
  it('cho phép khoá không ghi lý do và luôn có lý do ở hộp xoá', async () => {
    await click('Khoá'); await click('Khoá tài khoản')
    expect(fixture.updated).toHaveBeenCalledWith('linh', { disabled: true, reason: undefined }, undefined)
    await click('Xoá')
    expect(document.querySelector('[role="alertdialog"] input')).not.toBeNull()
    expect(buttons('Xoá tài khoản').at(-1)?.disabled).toBe(false)
  })
})

// Kho tài khoản chung (grants là object — kể cả {} = thành viên chưa được cấp app):
// checkbox lưu ngay từng request, lỗi một request chỉ hiện cạnh checkbox đó.
// grants = null (backend file độc lập) → toàn bộ điều khiển bị ẩn.
const granted = (username: string, role: User['role'], grants: User['grants']): User => ({ ...account(username, role), grants })
const mountSso = async () => {
  await act(async () => root.render(<MembersScreen
    users={[granted('dat', 'admin', { noverse: 'admin', viderse: 'admin' }), granted('linh', 'member', { noverse: 'member' })]}
    refresh={() => {}} />))
}
const checkbox = (label: string) => {
  const element = [...document.querySelectorAll<HTMLButtonElement>('button[role="checkbox"]')].find((el) => el.getAttribute('aria-label') === label)
  if (!element) throw new Error(`Missing checkbox: ${label}`)
  return element
}
describe('Quyền vào ứng dụng', () => {
  it('kho file độc lập (grants null) không có điều khiển cấp quyền app', async () => {
    await act(async () => root.render(<MembersScreen users={[account('dat', 'admin'), { ...account('linh'), grants: null }]} refresh={() => {}} />))
    expect(host.textContent).not.toContain('Quyền vào ứng dụng')
    expect(document.querySelectorAll('button[role="checkbox"]')).toHaveLength(0)
  })
  it('thành viên: hai checkbox theo grants, tắt Noverse gọi DELETE và làm mới danh sách', async () => {
    const refresh = vi.fn()
    await act(async () => root.render(<MembersScreen
      users={[granted('dat', 'admin', { noverse: 'admin', viderse: 'admin' }), granted('linh', 'member', { noverse: 'member' })]}
      refresh={refresh} />))
    expect(checkbox('Cho Linh vào Noverse').getAttribute('aria-checked')).toBe('true')
    expect(checkbox('Cho Linh vào Viderse').getAttribute('aria-checked')).toBe('false')
    await act(async () => { checkbox('Cho Linh vào Noverse').click() })
    expect(fixture.revokeApp).toHaveBeenCalledWith('linh', 'noverse')
    expect(fixture.grant).not.toHaveBeenCalled()
    expect(refresh).toHaveBeenCalled()
  })
  it('bật Viderse gọi PUT {role:member}, không động vào quyền dự án', async () => {
    await mountSso()
    await act(async () => { checkbox('Cho Linh vào Viderse').click() })
    expect(fixture.grant).toHaveBeenCalledWith('linh', 'viderse')
    expect(fixture.updated).not.toHaveBeenCalled()
  })
  it('request lỗi hiện ngay cạnh checkbox, không đếm là đã lưu', async () => {
    fixture.revokeApp.mockRejectedValue(new Error('Máy chủ bận'))
    await mountSso()
    await act(async () => { checkbox('Cho Linh vào Noverse').click() })
    const alert = [...document.querySelectorAll('[role="alert"]')].find((el) => el.textContent?.includes('Máy chủ bận'))
    expect(alert?.textContent).toBe('Máy chủ bận')
    // Checkbox render lại theo payload (vẫn có grant) — không phải trạng thái đã tắt.
    expect(checkbox('Cho Linh vào Noverse').getAttribute('aria-checked')).toBe('true')
  })
  it('quản trị kế thừa cả hai app: checkbox chỉ đọc, không gọi API khi bấm', async () => {
    await act(async () => root.render(<MembersScreen users={[granted('dat', 'admin', { noverse: 'admin', viderse: 'admin' }), granted('linh', 'admin', { noverse: 'admin', viderse: 'admin' })]} refresh={() => {}} />))
    // Chi tiết mặc định hiện thành viên đầu tiên không phải admin → mở đúng linh.
    await act(async () => { [...document.querySelectorAll('button')].find((b) => b.textContent?.includes('Linh'))?.click() })
    const box = checkbox('Cho Linh vào Viderse')
    expect(box.disabled).toBe(true)
    await act(async () => { box.click() })
    expect(fixture.grant).not.toHaveBeenCalled()
    expect(fixture.revokeApp).not.toHaveBeenCalled()
  })
  it('tạo thành viên: Noverse mặc định bật, bỏ tick sẽ thu grant mặc định của máy chủ', async () => {
    fixture.created.mockResolvedValue(granted('ngoc', 'member', { noverse: 'member' }))
    await mountSso()
    await click('Thêm thành viên'); await fill('Tên hiển thị', 'Ngọc')
    // Viderse mặc định tắt; bật nó rồi tắt Noverse.
    const dialogBoxes = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button[role="checkbox"]')]
    const viderse = dialogBoxes.find((el) => el.getAttribute('aria-label')?.includes('Viderse'))!
    const noverse = dialogBoxes.find((el) => el.getAttribute('aria-label')?.includes('Noverse'))!
    await act(async () => { viderse.click(); noverse.click() })
    await click('Tạo tài khoản')
    expect(fixture.revokeApp).toHaveBeenCalledWith('ngoc', 'noverse')
    expect(fixture.grant).toHaveBeenCalledWith('ngoc', 'viderse')
  })
})
