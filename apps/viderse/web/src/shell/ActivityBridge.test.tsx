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
it('maps review-ready to a one-line work title, channels in the sub, six-second card', async () => {
  fixture.activity = { running: [], videoJobs: 0, queued: 0, unread: [], badge: 0 }
  fixture.settings.kinds = { review_ready: { inApp: true, system: false } }
  root = createRoot(document.body.appendChild(document.createElement('div')))
  await act(async () => root?.render(<ActivityBridge />))
  fixture.activity = { ...fixture.activity, unread: [{ id: 'review', kind: 'review_ready', count: 3, text: '3 video chờ duyệt', actions: [{ label: 'Duyệt ảnh', href: '#/review' }], createdAt: '2026-11-01T12:00:00Z', readAt: null, detail: { channels: [{ channelId: 'a', name: 'Midnight Tales', count: 2 }, { channelId: 'b', name: 'Chuyện Lạ Đêm Khuya', count: 1 }] } }] }
  await act(async () => root?.render(<ActivityBridge />))
  const card = vi.mocked(toast.card)
  expect(card).toHaveBeenCalledWith(expect.objectContaining({ title: '3 video chờ duyệt ảnh', text: 'Midnight Tales 2 · Chuyện Lạ Đêm Khuya 1', tone: 'warn', duration: 6000 }))
  expect(card.mock.calls[0]?.[0]).not.toHaveProperty('meta')
})
it('does not poll /api/activity on a timer — SSE events refresh it', async () => {
  vi.useFakeTimers()
  try {
    root = createRoot(document.body.appendChild(document.createElement('div')))
    await act(async () => root?.render(<ActivityBridge />))
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000) })
    expect(fixture.refresh).not.toHaveBeenCalled()
  } finally { vi.useRealTimers() }
})
