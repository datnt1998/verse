// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { AppLink, User } from '../api/types'
const state = vi.hoisted(() => ({ user: null as User | null, apps: [] as AppLink[] }))
vi.mock('../api/session', () => ({ useSession: () => state }))
import { AppSwitcher } from './AppSwitcher'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let host: HTMLDivElement, root: Root
beforeEach(() => {
  state.user = { username: 'linh', name: 'Linh', role: 'member', grants: { noverse: 'member', viderse: 'member' }, projects: {}, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null }
  state.apps = [{ id: 'noverse', name: 'Noverse', origin: 'https://novel.example.test' }, { id: 'viderse', name: 'Viderse', origin: 'https://video.example.test' }]
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => { await act(async () => root.unmount()); host.remove() })
it.each([false, true])('ẩn cả nhóm khi không có quyền, mobile=%s', async (mobile) => {
  state.user!.grants = { noverse: 'member' }
  await act(async () => root.render(<AppSwitcher mobile={mobile} />))
  expect(host.querySelector('section')).toBeNull()
})
it('không cấu hình app kia cũng không hiện bộ chuyển', async () => {
  state.apps = state.apps.filter((app) => app.id === 'noverse')
  await act(async () => root.render(<AppSwitcher />))
  expect(host.querySelector('section')).toBeNull()
})
it('kho file không có bộ chuyển kể cả quản trị', async () => {
  state.user!.grants = null; state.user!.role = 'admin'
  await act(async () => root.render(<AppSwitcher />))
  expect(host.querySelector('section')).toBeNull()
})
it.each([false, true])('quyền hợp lệ mở app kia ở thẻ mới, mobile=%s', async (mobile) => {
  await act(async () => root.render(<AppSwitcher mobile={mobile} />))
  const links = [...host.querySelectorAll('a')]
  expect(links.map((link) => link.textContent)).toEqual([expect.stringContaining('Noverse'), expect.stringContaining('Viderse')])
  expect(links[0].getAttribute('aria-current')).toBe('true')
  expect(links[1].href).toBe('https://video.example.test/')
  expect(links[1].target).toBe('_blank')
  expect(links[1].rel).toBe('noopener noreferrer')
})
