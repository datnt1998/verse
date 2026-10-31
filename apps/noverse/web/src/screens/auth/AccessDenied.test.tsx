// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { AppLink, User } from '../../api/types'
const state = vi.hoisted(() => ({ deniedUser: null as User | null, apps: [] as AppLink[], refresh: vi.fn(), signOut: vi.fn(), endSession: vi.fn() }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: null, ...state }) }))
import { AccessDeniedScreen } from './AccessDenied'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let host: HTMLDivElement, root: Root
beforeEach(() => {
  vi.clearAllMocks()
  state.deniedUser = { username: 'linh', name: 'Linh', role: 'member', grants: { viderse: 'member' }, projects: {}, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, lastSeenAt: null, mustChangePassword: false }
  state.apps = [{ id: 'viderse', name: 'Viderse', origin: 'https://video.example.test:8443' }]
  state.refresh.mockResolvedValue('denied'); state.signOut.mockResolvedValue(undefined)
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => { await act(async () => root.unmount()); host.remove() })
it('nút chính dùng địa chỉ app từ máy chủ; thử lại giữ phiên khi chưa được cấp quyền', async () => {
  await act(async () => root.render(<AccessDeniedScreen />))
  const link = host.querySelector('a[target="_blank"]')!
  expect(link.getAttribute('href')).toBe('https://video.example.test:8443')
  expect(link.textContent).toContain('video.example.test:8443')
  await act(async () => [...host.querySelectorAll('button')].find((button) => button.textContent?.includes('Thử lại'))!.click())
  expect(state.refresh).toHaveBeenCalledOnce()
  expect(state.signOut).not.toHaveBeenCalled()
  expect(state.endSession).not.toHaveBeenCalled()
})
it('không có quyền app kia thì không hiện lối mở app kể cả apps cũ còn trong bộ nhớ', async () => {
  state.deniedUser!.grants = {}
  await act(async () => root.render(<AccessDeniedScreen />))
  expect(host.querySelector('a[target="_blank"]')).toBeNull()
  expect(host.textContent).toContain('Thử lại')
})
it('kho file không hiện điều khiển liên app dù có apps cũ', async () => {
  state.deniedUser!.grants = null
  await act(async () => root.render(<AccessDeniedScreen />))
  expect(host.querySelector('a[target="_blank"]')).toBeNull()
  expect(host.textContent).not.toContain('cả Noverse và Viderse')
  expect(host.textContent).not.toContain('Chép lời nhờ')
})
