// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { ToastHost } from '@noverse/ui'
import type { QueuePatch, QueueQueuedItem, QueueView } from '../../api/clone'
import { QueueScreen } from './QueueScreen'
import { QueueSummarySheet } from './QueueSummarySheet'
import { parseHash, paths, routePath } from '../../router'

let online = true
vi.mock('../../data/status', () => ({ useOnline: () => online, useStatus: () => ({ data: {}, status: 'ready' }) }))
let root: Root, host: HTMLDivElement, current: QueueView, removed: QueueQueuedItem[], patches: QueuePatch[], rejectPatch: boolean
const queued: QueueQueuedItem[] = [
  { id: 'first', channelId: 'one', title: 'First video', source: 'Original channel', sourceUrl: 'https://youtu.be/abcdefghijk', templateName: 'Original recipe', durationSec: 120, priority: 'pick', position: 10, etaStart: null, waitReason: 'workers_busy' },
  { id: 'second', channelId: 'two', title: 'Second video', source: 'Another channel', sourceUrl: 'https://youtu.be/lmnopqrstuv', templateName: 'Chosen recipe', durationSec: 240, priority: 'batch', position: 20, etaStart: null, waitReason: 'rotation' },
]
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } })
const click = async (node: Element | null) => { expect(node).not.toBeNull(); await act(async () => (node as HTMLElement).click()) }
const button = (text: string) => [...document.querySelectorAll('button')].find((node) => node.textContent?.includes(text)) ?? null
beforeEach(async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  online = true; rejectPatch = false; removed = []; patches = []; cache.clear()
  history.replaceState(null, '', '#/queue')
  current = { paused: false, workers: { used: 1, total: 2 }, imageWorkers: 3, rescanHours: 1, running: [{ jobId: 'job', channelId: 'one', title: 'Real running job', now: 'Đang làm phụ đề', agent: 'scout', stepIndex: 2, etaMinutes: 4 }], items: structuredClone(queued), pace: [], imageQuota: [], doneToday: [], rotation: { order: ['one'], next: 'one', skipped: [] } }
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: query.includes('max-width'), media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url === '/api/queue') {
      if (init?.method === 'PATCH') {
        const body = JSON.parse(String(init.body)) as QueuePatch; patches.push(body)
        if (rejectPatch) return json({ error: 'Mục hàng chờ đã chạy', code: 'conflict' }, 409)
        if ('paused' in body) current = { ...current, paused: body.paused, pausedAt: body.paused ? new Date().toISOString() : null }
        else if ('action' in body && body.action === 'remove') { removed.push(current.items.find((item) => item.id === body.id)!); current = { ...current, items: current.items.filter((item) => item.id !== body.id) } }
        else if ('action' in body && body.action === 'restore') current = { ...current, items: [...current.items, removed.find((item) => item.id === body.id)!] }
        else if ('action' in body && body.action === 'front') current = { ...current, items: current.items.map((item) => item.id === body.id ? { ...item, priority: 'pick', position: 0 } : item) }
        else if ('workers' in body) current = { ...current, workers: { ...current.workers, total: body.workers } }
      }
      return json(current)
    }
    return json({ channels: [{ id: 'one', name: 'First channel' }, { id: 'two', name: 'Second channel' }] })
  }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<><QueueScreen /><ToastHost /></>))
})
afterEach(async () => { await act(async () => root.unmount()); host.remove(); document.querySelectorAll('.m-overlay').forEach((node) => node.remove()); cache.clear(); vi.unstubAllGlobals() })
it('promotes a selected row through the real front mutation and moves it before the current pick', async () => {
  await click(document.querySelector('[aria-label="Thao tác với Second video"]'))
  await click(button('Lên đầu hàng chờ'))
  expect(patches).toContainEqual({ id: 'second', action: 'front' })
  expect(host.querySelector('.mq-row')?.id).toBe('queue-item-second')
})
it('removes immediately and restores the same queued row identity, priority and recipe with Undo', async () => {
  await click(document.querySelector('[aria-label="Thao tác với Second video"]')); await click(button('Bỏ khỏi hàng chờ'))
  expect(patches).toContainEqual({ id: 'second', action: 'remove' }); expect(host.querySelector('#queue-item-second')).toBeNull()
  await click(button('Hoàn tác'))
  expect(patches).toContainEqual({ id: 'second', action: 'restore' }); expect(host.querySelector('#queue-item-second')?.textContent).toContain('Chosen recipe')
  expect(current.items.find((item) => item.id === 'second')).toEqual(queued[1])
})
it('keeps removal receipt and displays a failure rather than fabricating restoration after a conflict', async () => {
  await click(document.querySelector('[aria-label="Thao tác với Second video"]')); await click(button('Bỏ khỏi hàng chờ'))
  rejectPatch = true; await click(button('Hoàn tác'))
  expect(host.querySelector('#queue-item-second')).toBeNull(); expect(button('Hoàn tác')).not.toBeNull(); expect(document.body.textContent).toContain('Chưa lưu được')
})
it('pauses real running work without hiding it and gives every waiting row the paused reason', async () => {
  await click(button('Tạm dừng'))
  expect(patches).toContainEqual({ paused: true }); expect(host.querySelectorAll('.mq-run')).toHaveLength(1)
  expect([...host.querySelectorAll('.mq-row')].every((node) => node.textContent?.includes('Hàng chờ đang tạm dừng'))).toBe(true)
  expect(host.textContent).toContain('Đã tạm dừng lúc')
})
it('bounds mobile workers at six and exposes no image or rescan edit controls', async () => {
  current = { ...current, workers: { used: 1, total: 6 }, imageWorkers: 3, rescanHours: 1 }
  await act(async () => { cache.setData(['queue'], current) })
  await click(button('Quy tắc chạy'))
  expect(document.querySelector<HTMLButtonElement>('[aria-label="Số luồng chạy cùng lúc tăng"]')?.disabled).toBe(true)
  expect(document.querySelector('[aria-label="Số ảnh vẽ cùng lúc tăng"]')).toBeNull()
  expect(document.querySelector('[aria-label="Khoảng giờ xem lại đối thủ giảm"]')).toBeNull()
  await click(document.querySelector('[aria-label="Số luồng chạy cùng lúc giảm"]'))
  expect(patches).toContainEqual({ workers: 5 }); expect(current.workers.total).toBe(5)
  await click(button('Xong')); expect(patches).toHaveLength(1)
})
it('filters actual channel rows and disables promotion of the current first row', async () => {
  await click([...host.querySelectorAll('[role="radio"]')].find((node) => node.textContent?.includes('Second channel')) ?? null)
  expect(host.querySelectorAll('.mq-row')).toHaveLength(1)
  await click(document.querySelector('[aria-label="Thao tác với Second video"]'))
  expect(document.querySelector('a[href="https://youtu.be/lmnopqrstuv"]')?.getAttribute('target')).toBe('_blank')
  await act(async () => root.render(<QueueSummarySheet open onClose={() => {}} />))
  expect(document.querySelector('a[href="#/queue?item=first"]')).not.toBeNull()
})
it('keeps cached running and waiting rows offline and locks pause in the summary sheet', async () => {
  const close = vi.fn(); online = false
  await act(async () => root.render(<QueueSummarySheet open onClose={close} />))
  expect(document.body.textContent).toContain('Real running job'); expect(document.body.textContent).toContain('Second video')
  expect((button('Tạm dừng') as HTMLButtonElement).disabled).toBe(true)
  expect(document.body.textContent).toContain('lần cập nhật lúc')
  await click(document.querySelector('a[href="#/queue"]')); expect(close).toHaveBeenCalledOnce()
})
it('preserves queue focus and rules navigation during router canonicalization', () => {
  const path = paths.queue({ item: 'row / two', sheet: 'rules' })
  expect(routePath(parseHash(`#${path}`).route)).toBe(path)
  expect(parseHash('#/queue?sheet=wrong').route).toEqual({ name: 'queue', item: undefined, sheet: undefined })
})

it('focuses and highlights the exact queue item requested by a summary-sheet link', async () => {
  await act(async () => {
    history.replaceState(null, '', '#/queue?item=second')
    window.dispatchEvent(new HashChangeEvent('hashchange'))
  })
  await act(async () => { await new Promise<void>((resolve) => requestAnimationFrame(() => resolve())) })
  expect(document.activeElement?.id).toBe('queue-item-second')
  expect(host.querySelector('#queue-item-second')?.className).not.toBe(host.querySelector('#queue-item-first')?.className)
})

it('renders the actual empty summary and closes when navigating to Video mới', async () => {
  current = { ...current, items: [], running: [] }
  await act(async () => { cache.setData(['queue'], current) })
  const close = vi.fn()
  await act(async () => root.render(<QueueSummarySheet open onClose={close} />))
  expect(document.body.textContent).toContain('Máy rảnh')
  expect(button('Tạm dừng')).toBeNull()
  await click(document.querySelector('a[href="#/new"]'))
  await vi.waitFor(() => expect(close).toHaveBeenCalledOnce())
  expect(close).toHaveBeenCalledOnce()
})

it('shows exhausted image limits while keeping real running and queued rows in the summary', async () => {
  current = { ...current, imageQuota: [
    { provider: 'codex', used: 50, limit: 50, period: 'day', role: 'main', resetsAt: '2026-11-02T00:00:00+07:00' },
    { provider: 'grok', used: 50, limit: 50, period: 'day', role: 'fallback', resetsAt: '2026-11-02T00:00:00+07:00' },
  ] }
  await act(async () => { cache.setData(['queue'], current) })
  await act(async () => root.render(<QueueSummarySheet open onClose={() => {}} />))
  expect(document.body.textContent).toContain('Hết hạn mức vẽ ảnh')
  expect(document.body.textContent).toContain('50/50')
  expect(document.querySelector('a[href="#/?video=job"]')).not.toBeNull()
  expect(document.querySelector('a[href="#/queue?item=first"]')).not.toBeNull()
})

it('dismisses rules on Escape without inventing an additional save mutation', async () => {
  await click(button('Quy tắc chạy'))
  await act(async () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
  expect(patches).toHaveLength(0)
  expect(document.querySelector('.m-overlay')?.classList.contains('m-out')).toBe(true)
})

it('returns to every mobile row when the filter receives the Home key', async () => {
  const choices = host.querySelectorAll<HTMLButtonElement>('[role="radiogroup"] [role="radio"]')
  await click(choices[2])
  expect(choices[2].getAttribute('aria-checked')).toBe('true')
  expect(host.querySelectorAll('.mq-row')).toHaveLength(1)
  expect(host.querySelector('.mq-row')?.textContent).toContain(queued[1].title)
  await act(async () => { choices[2].dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true })) })
  expect(choices[0].getAttribute('aria-checked')).toBe('true')
  expect(host.querySelectorAll('.mq-row')).toHaveLength(2)
})
