// @vitest-environment happy-dom
// 403 app_access_denied ≠ 401: phiên thật nhưng chưa được cấp quyền app — session
// giữ người đang đăng nhập ở deniedUser, KHÔNG set user (dữ liệu/SSE không mount,
// không vòng lặp đăng nhập). 401 vẫn là cổng "chưa có phiên".
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './client'
import { SessionProvider, useSession, type SessionState } from './session'
import { DataProvider } from '../data/Provider'
import type { User } from './types'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const mocks = vi.hoisted(() => ({
  me: vi.fn(), getSetup: vi.fn(), startDataEvents: vi.fn(), logout: vi.fn(),
}))
vi.mock('./client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./client')>()),
  me: mocks.me, getSetup: mocks.getSetup, logout: mocks.logout,
}))
vi.mock('../data/events', () => ({
  events: {
    onReconnect: () => () => {}, onStatus: () => () => {},
    status: { state: 'closed', error: '' },
    retryNow: () => {}, subscribe: () => () => {}, stop: () => {},
  },
  subscribeEvents: () => () => {},
  startDataEvents: mocks.startDataEvents,
}))
vi.mock('../router', () => ({ navigate: vi.fn() }))

const user = (over: Partial<User> = {}): User => ({
  username: 'linh', name: 'Linh', role: 'member', projects: {},
  grants: {}, disabled: false, createdAt: '', updatedAt: '',
  onboardedAt: '2026-10-01T00:00:00Z', mustChangePassword: false, lastSeenAt: null,
  ...over,
})
const deniedBody = (account: User) => ({ code: 'app_access_denied', app: 'noverse', error: 'no grant', user: account })

let root: Root
let host: HTMLDivElement
let latest: SessionState | null = null

function Probe() { latest = useSession(); return null }
async function mount() {
  await act(async () => {
    root.render(<SessionProvider><DataProvider><Probe /></DataProvider></SessionProvider>)
  })
}

beforeEach(() => {
  mocks.me.mockReset(); mocks.getSetup.mockResolvedValue({ needed: false })
  mocks.startDataEvents.mockClear(); mocks.logout.mockReset()
  host = document.createElement('div'); document.body.append(host); root = createRoot(host); latest = null
})
afterEach(async () => { await act(async () => root.unmount()); host.remove() })

describe('phiên bị chặn cấp quyền app', () => {
  it('403 app_access_denied lúc mở app → deniedUser, user=null, không mount SSE', async () => {
    const account = user()
    mocks.me.mockRejectedValue(new ApiError(403, 'no grant', undefined, deniedBody(account)))
    await mount()
    expect(latest?.checked).toBe(true)
    expect(latest?.user).toBeNull()
    expect(latest?.deniedUser).toMatchObject({ username: 'linh' })
    expect(mocks.startDataEvents).not.toHaveBeenCalled()
  })
  it('401 vẫn là chưa có phiên — deniedUser không bật', async () => {
    mocks.me.mockRejectedValue(new ApiError(401, 'no session'))
    await mount()
    expect(latest?.checked).toBe(true)
    expect(latest?.user).toBeNull()
    expect(latest?.deniedUser).toBeNull()
    expect(mocks.startDataEvents).not.toHaveBeenCalled()
    expect(mocks.getSetup).toHaveBeenCalled()
  })
  it('có phiên đủ quyền → user set và SSE mount', async () => {
    mocks.me.mockResolvedValue({
      user: user({ grants: { noverse: 'member' } }),
      session: { id: 's1', username: 'linh', device: 'web', ip: '127.0.0.1', createdAt: '', lastSeen: '', expiresAt: '', current: true },
    })
    await mount()
    expect(latest?.user?.username).toBe('linh')
    expect(latest?.deniedUser).toBeNull()
    expect(mocks.startDataEvents).toHaveBeenCalledWith('linh')
  })
  it('refresh() sau khi được cấp quyền → denied xoá, user set', async () => {
    const account = user()
    mocks.me
      .mockRejectedValueOnce(new ApiError(403, 'no grant', undefined, deniedBody(account)))
      .mockResolvedValue({
        user: user({ grants: { noverse: 'member' } }),
        session: { id: 's1', username: 'linh', device: 'web', ip: '127.0.0.1', createdAt: '', lastSeen: '', expiresAt: '', current: true },
      })
    await mount()
    expect(latest?.deniedUser).not.toBeNull()
    let outcome: string | undefined
    await act(async () => { outcome = await latest!.refresh() })
    expect(outcome).toBe('user')
    expect(latest?.deniedUser).toBeNull()
    expect(latest?.user?.username).toBe('linh')
  })
  it('refresh() vẫn bị chặn → outcome denied, user vẫn null', async () => {
    const account = user()
    mocks.me.mockRejectedValue(new ApiError(403, 'no grant', undefined, deniedBody(account)))
    await mount()
    let outcome: string | undefined
    await act(async () => { outcome = await latest!.refresh() })
    expect(outcome).toBe('denied')
    expect(latest?.user).toBeNull()
    expect(latest?.deniedUser?.username).toBe('linh')
    expect(mocks.startDataEvents).not.toHaveBeenCalled()
  })
  it('signOut revoke phiên máy chủ rồi dọn trạng thái chặn', async () => {
    mocks.me.mockRejectedValue(new ApiError(403, 'no grant', undefined, deniedBody(user())))
    mocks.logout.mockResolvedValue({ ok: true })
    await mount()
    await act(async () => { await latest!.signOut() })
    expect(mocks.logout).toHaveBeenCalled()
    expect(latest?.deniedUser).toBeNull()
    expect(latest?.user).toBeNull()
  })
})

describe('phiên dùng chung Gói AO', () => {
  const apps = [{ id: 'viderse', name: 'Viderse', origin: 'https://vid.aarverse.studio' }]
  it('màn chặn giữ apps từ thân lỗi mà không khởi động dữ liệu', async () => {
    mocks.me.mockRejectedValue(new ApiError(403, 'Chưa được cấp quyền', undefined, { ...deniedBody(user({ grants: { viderse: 'member' } })), apps }))
    await mount()
    expect(latest?.apps).toEqual(apps)
    expect(mocks.startDataEvents).not.toHaveBeenCalled()
  })
  it('401 có nguồn đăng xuất hiển thị lý do app kia ngay lần tải đầu', async () => {
    mocks.me.mockRejectedValue(new ApiError(401, 'Phiên đã kết thúc', undefined, { code: 'session_ended', reason: 'logout', app: 'viderse' }))
    await mount()
    expect(latest?.kicked).toBe('other')
  })
  it('trình duyệt sạch nhận chế độ kho chung từ setup nhưng không bịa lý do kết thúc', async () => {
    mocks.me.mockRejectedValue(new ApiError(401, 'Chưa đăng nhập', undefined, { code: 'session_ended', reason: 'unknown' }))
    mocks.getSetup.mockResolvedValue({ needed: false, sharedAccounts: true })
    await mount()
    expect(latest?.sharedAccounts).toBe(true)
    expect(latest?.kicked).toBeNull()
  })
  it('đăng xuất thất bại giữ phiên và báo lỗi cho người gọi', async () => {
    mocks.me.mockRejectedValue(new ApiError(403, 'Chưa được cấp quyền', undefined, deniedBody(user())))
    mocks.logout.mockRejectedValue(new Error('Không kết nối được máy chủ'))
    await mount()
    await act(async () => { await expect(latest!.signOut()).rejects.toThrow('Không kết nối được máy chủ') })
    expect(latest?.deniedUser?.username).toBe('linh')
  })
})
