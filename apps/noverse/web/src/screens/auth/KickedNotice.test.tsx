// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { KickedReason } from '../../api/sso'
const state = vi.hoisted(() => ({ sharedAccounts: true, kicked: null as KickedReason | null }))
vi.mock('../../api/session', () => ({ useSession: () => state }))
import { KickedNotice } from './KickedNotice'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root, host: HTMLDivElement
beforeEach(() => { state.sharedAccounts = true; state.kicked = null; host = document.createElement('div'); document.body.append(host); root = createRoot(host) })
afterEach(async () => { await act(async () => root.unmount()); host.remove() })
it('kho file không dùng thông báo phiên liên app', async () => {
  state.sharedAccounts = false; state.kicked = 'self'
  await act(async () => root.render(<KickedNotice />))
  expect(host.querySelector('[role="status"]')).toBeNull()
})
it('hết hạn giữ màn cũ, không thêm khối lý do AO', async () => {
  state.kicked = 'expired'
  await act(async () => root.render(<KickedNotice />))
  expect(host.querySelector('[role="status"]')).toBeNull()
})
