// @vitest-environment happy-dom
// Sheet Việc nền trên điện thoại, tab Thông báo (08-dev-prompt V32 · 06-copy
// › Thanh trên điện thoại và Thông báo). Dữ liệu là thông báo thật của
// GET /api/activity + GET /api/notifications — cùng nguồn với popover máy tính.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import type { AppNotification } from '../../api/activity'
import type { QueueView } from '../../api/clone'
import { QueueSummarySheet } from './QueueSummarySheet'

let online = true
vi.mock('../../data/status', () => ({ useOnline: () => online, useStatus: () => ({ data: {}, status: 'ready' }) }))

let root: Root, host: HTMLDivElement
let readCalls: unknown[]
let notifications: AppNotification[]
let activityUnread: AppNotification[]

const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } })
const click = async (node: Element | null) => { expect(node, 'phần tử để bấm phải có trong DOM').not.toBeNull(); await act(async () => (node as HTMLElement).click()) }
const button = (text: string) => [...document.querySelectorAll('button')].find((node) => node.textContent?.includes(text)) ?? null

function note(id: string, kind: AppNotification['kind'], text: string, readAt: string | null = null, hour = 0, minute = 7): AppNotification {
  const createdAt = new Date(2026, 10, 1, hour, minute).toISOString()
  return { id, kind, count: 1, text, actions: [{ label: 'Mở', href: '#/review' }], createdAt, readAt }
}

/** Một nhịp macrotask: đủ để lời hứa của fetch/`.then` trong cache chạy xong. */
async function flush() {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 0)
  await promise
}

const queue: QueueView = {
  paused: false, workers: { used: 0, total: 2 }, imageWorkers: 3, rescanHours: 1,
  running: [], items: [], pace: [], imageQuota: [], doneToday: [], rotation: { order: [], next: null, skipped: [] },
}

beforeEach(async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  online = true
  readCalls = []
  cache.clear()
  history.replaceState(null, '', '#/')
  notifications = [
    note('a', 'review_ready', '3 video chờ duyệt', null, 0, 7),
    note('b', 'needs_attention', 'Cần xử lý: The Last Bus Doesn’t Stop at Elm Street', null, 23, 58),
    note('c', 'ready_to_post', 'The Hotel That Wasn’t on Any Map sẵn sàng đăng', '2026-11-01T10:00:00Z', 23, 40),
  ]
  activityUnread = notifications.filter((n) => !n.readAt)
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: query.includes('max-width'), media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url === '/api/queue') return json(queue)
    if (url === '/api/activity') return json({ running: [], videoJobs: 0, queued: 0, unread: activityUnread })
    if (url === '/api/notifications/read') { readCalls.push(JSON.parse(String(init?.body))); activityUnread = []; notifications = notifications.map((n) => ({ ...n, readAt: n.readAt ?? new Date().toISOString() })); return json({}) }
    if (url === '/api/notifications') return json({ notifications })
    if (url === '/api/owner-channels') return json({ channels: [] })
    return json({})
  }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => {
  await act(async () => root.unmount())
  host.remove()
  document.querySelectorAll('.m-overlay').forEach((node) => node.remove())
  cache.clear(); vi.unstubAllGlobals()
})

const openSheet = async (initialTab: 'running' | 'inbox') => {
  await act(async () => root.render(<QueueSummarySheet open onClose={() => {}} initialTab={initialTab} />))
  // Trang Thông báo nạp GET /api/notifications sau khi tab mở.
  await act(flush)
}

it('mở thẳng tab Thông báo khi nút Việc nền có chấm, huy hiệu vàng và dòng phụ đếm số chưa đọc', async () => {
  await openSheet('inbox')
  expect(document.body.textContent).toContain('Việc nền')
  const tabs = [...document.querySelectorAll('[role="tab"]')].map((node) => node.textContent ?? '')
  expect(tabs).toHaveLength(2)
  expect(tabs[0]).toContain('Đang chạy')
  expect(tabs[1]).toContain('Thông báo')
  expect(tabs[1]).toContain('2')
  expect(document.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toContain('Thông báo')
  expect(document.querySelector('.vi-sub')?.textContent).toBe('2 chưa đọc')
  // Cùng câu với popover máy tính.
  expect(document.body.textContent).toContain('3 video chờ duyệt')
  expect(document.body.textContent).toContain('The Last Bus Doesn’t Stop at Elm Street')
})

it('in đậm và chấm vàng chỉ ở dòng chưa đọc', async () => {
  await openSheet('inbox')
  const rows = [...document.querySelectorAll('.vv-note')]
  expect(rows).toHaveLength(3)
  const unreadRow = rows.find((row) => row.textContent?.includes('3 video chờ duyệt'))!
  const readRow = rows.find((row) => row.textContent?.includes('sẵn sàng đăng'))!
  expect(unreadRow.querySelector('.vv-note__dot')).not.toBeNull()
  expect(readRow.querySelector('.vv-note__dot')).toBeNull()
  expect(unreadRow.querySelector('.vv-note__title')?.getAttribute('data-unread')).toBe('true')
  expect(readRow.querySelector('.vv-note__title')?.getAttribute('data-unread')).toBe('false')
})

it('Đánh dấu đã đọc gọi API thật rồi đổi dòng phụ và bỏ huy hiệu', async () => {
  await openSheet('inbox')
  await click(button('Đánh dấu đã đọc'))
  expect(readCalls).toContainEqual({ all: true })
  await act(flush)
  expect(document.querySelector('.vi-sub')?.textContent).toBe('Đã đọc hết')
  expect([...document.querySelectorAll('.vv-note .vv-note__dot')]).toHaveLength(0)
  expect(button('Đánh dấu đã đọc')).toBeNull()
})

it('trống thông báo: lời khuyên + nút Cài đặt thông báo, dòng phụ "Chưa có thông báo"', async () => {
  notifications = []
  activityUnread = []
  await openSheet('inbox')
  expect(document.querySelector('.vi-sub')?.textContent).toBe('Chưa có thông báo')
  expect(document.body.textContent).toContain('Khi video chờ bạn duyệt, cần xử lý hay sẵn sàng đăng, Viderse báo ở đây.')
  const settings = document.querySelector('a[href="#/settings?section=notifications"]')
  expect(settings?.textContent).toContain('Cài đặt thông báo')
})

it('cuối danh sách có lối vào Cài đặt thông báo của popover máy tính', async () => {
  await openSheet('inbox')
  const links = [...document.querySelectorAll('a[href="#/settings?section=notifications"]')]
  expect(links.some((link) => link.textContent?.includes('Cài đặt thông báo'))).toBe(true)
})

it('tab Đang chạy mở mặc định và giữ nguyên nội dung hàng chờ', async () => {
  await openSheet('running')
  expect(document.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toContain('Đang chạy')
  expect(document.body.textContent).toContain('Máy rảnh')
  expect(document.querySelector('a[href="#/new"]')).not.toBeNull()
})
