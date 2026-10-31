import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { AppNotification } from '../api/activity'
import { toast } from '@noverse/ui'
import { ActivityBridge } from './ActivityBridge'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const fixture = vi.hoisted(() => ({ activity: { running: [] as unknown[], videoJobs: 0, queued: 0, unread: [] as AppNotification[], badge: 0 }, settings: { kinds: {} as Record<string, { inApp: boolean; system: boolean }>, doNotDisturb: { enabled: false, start: '23:00', end: '07:00' }, badge: false, merge: true, recapOnReturn: true }, toast: vi.fn(), refresh: vi.fn() }))
vi.mock('../data/useResource', () => ({ useResource: (key: string[]) => ({ data: key.length === 1 ? fixture.activity : fixture.settings, refresh: fixture.refresh }) }))
vi.mock('@noverse/ui', () => ({ toast: Object.assign(fixture.toast, { card: vi.fn() }) }))
let root: Root | undefined
afterEach(async () => { if (root) await act(async () => root?.unmount()); document.body.replaceChildren(); vi.clearAllMocks() })
it('does not toast when a video moves between progress stages', async () => {
  root = createRoot(document.body.appendChild(document.createElement('div')))
  await act(async () => root?.render(<ActivityBridge />))
  fixture.activity = { ...fixture.activity, running: [{ id: 'one', phase: 'Đạo diễn' }] }
  await act(async () => root?.render(<ActivityBridge />))
  expect(fixture.toast).not.toHaveBeenCalled()
})
it('passes both persisted recovery actions to a single attention toast', async () => {
  fixture.activity = { running: [], videoJobs: 0, queued: 0, unread: [], badge: 0 }
  fixture.settings.kinds = { needs_attention: { inApp: true, system: false } }
  root = createRoot(document.body.appendChild(document.createElement('div')))
  await act(async () => root?.render(<ActivityBridge />))
  fixture.activity = { ...fixture.activity, unread: [{ id: 'failure', kind: 'needs_attention', count: 1, text: 'Cần xử lý', actions: [{ label: 'Đăng nhập lại', href: '#/settings' }, { label: 'Mở video', href: '#/?video=one' }], createdAt: '2026-11-01T12:00:00Z', readAt: null }] }
  await act(async () => root?.render(<ActivityBridge />))
  const card = vi.mocked(toast.card)
  expect(card).toHaveBeenCalledOnce()
  expect(card).toHaveBeenCalledWith(expect.objectContaining({ action: expect.objectContaining({ label: 'Đăng nhập lại' }), secondaryAction: expect.objectContaining({ label: 'Mở video' }) }))
})
