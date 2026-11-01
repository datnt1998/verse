// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { beforeEach, afterEach, it, expect, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { QueueScreen } from './QueueScreen'
import { JobsPill } from '../../shell/JobsPill'
import type { QueueView } from '../../api/clone'
import type { Status } from '../../api/status'
import { t } from '../../i18n/queue'
import { readFileSync } from 'node:fs'
import { URL as NodeURL } from 'node:url'
const golden = JSON.parse(readFileSync(new NodeURL('../../../../internal/pipeline/testdata/clone-views.golden.json', import.meta.url), 'utf8'))
import { movePosition, viewState } from './queueView'

let root: Root, el: HTMLDivElement
let patches: Record<string, unknown>[]
const view: QueueView = {
  paused: false, workers: { used: 1, total: 2 }, imageWorkers: 2, rescanHours: 6,
  running: [], rotation: { order: ['mine'], next: 'mine', skipped: [] },
  items: [
    { id: 'q1', channelId: 'mine', title: 'Video thứ nhất', source: 'Đối thủ', templateName: 'Mẫu', durationSec: 120, priority: 'batch', etaStart: null, waitReason: 'pace_full', position: 10, thumbnailUrl: '/thumbnail.jpg' },
    { id: 'q2', channelId: 'mine', title: 'Video thứ hai', source: 'Đối thủ', templateName: 'Mẫu', durationSec: 120, priority: 'batch', etaStart: null, waitReason: 'no_template', position: 20 },
  ], pace: [{ channelId: 'mine', used: 5, limit: 5 }], imageQuota: [], doneToday: [],
}
const response = (data: unknown) => new Response(JSON.stringify(data), { headers: { 'Content-Type': 'application/json' } })
beforeEach(async () => {
  cache.clear(); patches = []
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/queue') {
      if (init?.method === 'PATCH') patches.push(JSON.parse(String(init.body)))
      return response(view)
    }
    return response({ channels: [{ id: 'mine', name: 'Kênh của tôi' }] })
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
  await act(async () => root.render(<QueueScreen />))
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); cache.clear(); vi.unstubAllGlobals() })
it('saves the midpoint position when dropping the second row above the first', async () => {
  const handles = el.querySelectorAll<HTMLElement>('[draggable=true]')
  const dt = { effectAllowed: '', setData: vi.fn(), getData: () => 'q2' }
  await act(async () => { const e = new Event('dragstart', { bubbles: true }); Object.defineProperty(e, 'dataTransfer', { value: dt }); handles[1].dispatchEvent(e) })
  await act(async () => { const e = new Event('drop', { bubbles: true, cancelable: true }); Object.defineProperty(e, 'dataTransfer', { value: dt }); handles[0].closest('[role=row]')!.dispatchEvent(e) })
  expect(patches).toContainEqual({ id: 'q2', position: 9 })
  expect(movePosition(view.items, 'q1', 'q2')).toBe(19)
})
it('keeps the pill paused even when a finishing step is still running', async () => {
  await act(async () => root.render(<JobsPill online status={{ paused: golden[0].stageGroup === 'paused', running: 1, queued: 2 } as Status} />))
  expect(el.querySelector('.vv-jobs--paused')).not.toBeNull()
  expect(el.querySelector('.vv-jobs--live')).toBeNull()
})
it('keeps drawing when the primary quota is exhausted but fallback capacity remains', () => {
  expect(viewState({ ...view, imageQuota: [
    { provider: 'gpt', used: 50, limit: 50, period: 'day', resetsAt: null, role: 'main' },
    { provider: 'gemini', used: 0, limit: 50, period: 'day', resetsAt: null, role: 'fallback' },
  ] })).toBe('running')
})
it('shows limited only when every configured image source is exhausted', () => {
  expect(viewState({ ...view, imageQuota: [
    { provider: 'codex', used: 50, limit: 50, period: 'day', resetsAt: null, role: 'main' },
    { provider: 'grok', used: 400, limit: 400, period: 'week', resetsAt: null, role: 'fallback' },
  ] })).toBe('limited')
  expect(viewState({ ...view, imageQuota: [
    { provider: 'codex', used: 50, limit: 50, period: 'day', resetsAt: null, role: 'main' },
    { provider: 'grok', used: 400, limit: null, period: 'week', resetsAt: null, role: 'fallback' },
  ] })).toBe('running')
})
it('renders the queued source thumbnail from the queue response', () => {
  expect(el.querySelector('.vq-thumb img')?.getAttribute('src')).toBe('/thumbnail.jpg')
})
it.each([0, 5])('marks only an image step as quota-blocked (step %s)', async (stepIndex) => {
  const limited: QueueView = {
    ...view,
    running: [{ jobId: 'job1', channelId: 'mine', title: 'Video đang làm', agent: 'artist', now: 'Đang làm', stepIndex, etaMinutes: null }],
    imageQuota: [{ provider: 'codex', used: 50, limit: 50, period: 'day', resetsAt: null, role: 'main' }],
  }
  await act(async () => root.unmount())
  cache.clear()
  vi.stubGlobal('fetch', vi.fn(async (path: string) => response(path === '/api/queue' ? limited : { channels: [{ id: 'mine', name: 'Kênh của tôi' }] })))
  root = createRoot(el)
  await act(async () => root.render(<QueueScreen />))
  expect(el.querySelector('.vq')?.getAttribute('data-state')).toBe('limited')
  expect(el.querySelector('.vq-run')?.hasAttribute('data-limited')).toBe(stepIndex === 5)
})

it('filters queued rows with controlled keyboard choices and retains native menu navigation', async () => {
  const filteredView = { ...view, items: [{ ...view.items[0], sourceUrl: 'https://youtu.be/abcdefghijk' }, { ...view.items[1], channelId: 'other' }] }
  await act(async () => root.unmount())
  cache.clear()
  vi.stubGlobal('fetch', vi.fn(async (path: string) => response(path === '/api/queue' ? filteredView : { channels: [{ id: 'mine', name: 'Kênh của tôi' }, { id: 'other', name: 'Other channel' }] })))
  root = createRoot(el)
  await act(async () => root.render(<QueueScreen />))
  const group = el.querySelector(`[role="radiogroup"][aria-label="${t.screens.queue.filterAria}"]`)!
  const choices = group.querySelectorAll<HTMLButtonElement>('[role="radio"]')
  await act(async () => { choices[1].click() })
  expect(choices[1].getAttribute('aria-checked')).toBe('true')
  expect(el.querySelectorAll('.vq-row:not(.vq-row--head)')).toHaveLength(1)
  expect(el.querySelector('.vq-row:not(.vq-row--head)')?.textContent).toContain(view.items[0].title)
  await act(async () => { choices[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true })) })
  expect(choices[0].getAttribute('aria-checked')).toBe('true')
  expect(el.querySelectorAll('.vq-row:not(.vq-row--head)')).toHaveLength(2)
  const trigger = el.querySelector<HTMLButtonElement>(`[aria-label="${t.screens.queue.menuMore(view.items[0].title)}"]`)!
  await act(async () => { trigger.click() })
  const menu = document.querySelector('[role="menu"]')!
  expect(menu.querySelector<HTMLAnchorElement>(`a[href="#/channels/${view.items[0].channelId}"]`)).not.toBeNull()
  expect(menu.querySelector<HTMLAnchorElement>('a[target="_blank"]')?.href).toBe(filteredView.items[0].sourceUrl)
  expect(menu.querySelector('button:disabled')?.textContent).toContain(t.screens.queue.menuTop)
  await act(async () => { menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
  expect(document.querySelector('[role="menu"]')).toBeNull()
  expect(document.activeElement).toBe(trigger)
})
