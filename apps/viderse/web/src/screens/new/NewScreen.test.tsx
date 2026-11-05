// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { NewVideoDialog } from './NewScreen'
import type { ClonePlan, CloneEnqueueResult } from '../../api/clone'
import { cloneTarget } from '../../api/clone'
import { blocksQuickCreate, openQuickCreate, QUICK_CREATE_EVENT } from './quickCreate'
import { parseHash, paths, routePath } from '../../router'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root, host: HTMLDivElement
let calls: { path: string; body: Record<string, unknown> }[]
let plan: ClonePlan, result: CloneEnqueueResult
let failJobs: boolean
const close = vi.fn()
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const tick = async (ms = 0) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const input = async (value: string) => {
  const field = document.querySelector<HTMLTextAreaElement>('#quick-create-urls')!
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(field, value); field.dispatchEvent(new Event('input', { bubbles: true })) })
}
const submit = async () => { await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))) }
beforeEach(async () => {
  vi.useFakeTimers(); cache.clear(); close.mockClear(); calls = []; failJobs = false
  plan = { accepted: [{ url: 'https://youtu.be/abcdefghijk', videoId: 'abcdefghijk', title: 'Video thật', durationSec: 180, outlier: null, hasCaptions: true, thumbnailUrl: '/source.jpg' }], duplicates: [], invalid: [], position: 'front', etaStart: null, paceLeftToday: 2, overflowDays: 0, template: { id: 'standard', name: 'Mẫu chuẩn' }, imageCalls: 12, priceUSD: 0.24 }
  result = { items: [{ id: 'real-queue-row', jobId: null }], duplicates: [], invalid: [] }
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/owner-channels') return json({ channels: [{ id: 'mine', name: 'Kênh của tôi', defaultTemplateId: 'standard', defaultTemplateName: 'Mẫu chuẩn', sources: [] }, { id: 'other', name: 'Kênh khác', defaultTemplateId: 'doc', defaultTemplateName: 'Phim tài liệu', sources: [] }] })
    if (path === '/api/templates') return json({ items: [{ id: 'standard', name: 'Mẫu chuẩn' }, { id: 'doc', name: 'Phim tài liệu' }] })
    if (path === '/api/clone/plan' || path === '/api/jobs') {
      calls.push({ path, body: JSON.parse(String(init?.body)) })
      return path === '/api/jobs' ? json(failJobs ? { error: 'Lỗi mạng' } : result, failJobs ? 500 : 200) : json(plan)
    }
    return json({})
  }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<NewVideoDialog request={{}} onClose={close} />)); await tick()
})
afterEach(async () => { await act(async () => { root.unmount(); await vi.runOnlyPendingTimersAsync() }); host.remove(); cache.clear(); vi.useRealTimers(); vi.unstubAllGlobals() })
it('debounces actual metadata and renders real price and title', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(299); expect(calls).toHaveLength(0)
  await tick(1); expect(document.body.textContent).toContain('Video thật'); expect(document.body.textContent).toContain('$0.24')
  expect(document.querySelector('img')?.getAttribute('src')).toBe('/source.jpg')
})
it('sends mixed input to canonical jobs without public priority', async () => {
  plan.invalid = [{ url: 'bad-link', reason: 'not_youtube' }]
  result.invalid = plan.invalid
  await input('https://youtu.be/abcdefghijk\nbad-link'); await tick(300); await submit()
  expect(calls.find((c) => c.path === '/api/jobs')?.body).toEqual({ channelId: 'mine', urls: ['https://youtu.be/abcdefghijk', 'bad-link'] })
  expect(document.body.textContent).toContain('Link không phải video YouTube')
  expect(document.querySelector('a[href="#/?queueItem=real-queue-row"]')).not.toBeNull()
})
it('keeps the draft on mutation failure', async () => {
  failJobs = true; await input('https://youtu.be/abcdefghijk'); await tick(300); await submit()
  expect(document.querySelector<HTMLTextAreaElement>('textarea')?.value).toBe('https://youtu.be/abcdefghijk'); expect(close).not.toHaveBeenCalled()
})
it('does not enqueue on cancel', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  await act(async () => [...document.querySelectorAll('button')].find((button) => button.textContent === 'Huỷ')!.click())
  expect(calls.filter((c) => c.path === '/api/jobs')).toHaveLength(0); expect(close).toHaveBeenCalled()
})
it('links a duplicate pending row to its exact workbench target', async () => {
  plan.accepted = []; plan.duplicates = [{ url: 'https://youtu.be/abcdefghijk', jobId: null, queueItemId: 'waiting', state: 'queued' }]
  await input('https://youtu.be/abcdefghijk'); await tick(300); await submit()
  expect(document.querySelector('a[href="#/?queueItem=waiting"]')).not.toBeNull(); expect(calls.filter((c) => c.path === '/api/jobs')).toHaveLength(0)
})
it('opens selections without a URL input and keeps the source route', async () => {
  history.replaceState(null, '', '#/explore?rival=source')
  await act(async () => root.render(<NewVideoDialog key="selection" request={{ urls: ['https://youtu.be/abcdefghijk'], fromSelection: true }} onClose={close} />)); await tick(300)
  expect(document.querySelector('textarea')).toBeNull(); expect(location.hash).toBe('#/explore?rival=source')
})
it('blocks N in nested contenteditable and form inputs', () => {
  const editor = document.createElement('div'); editor.contentEditable = 'true'; editor.setAttribute('contenteditable', 'true'); const span = document.createElement('span'); editor.append(span)
  expect(blocksQuickCreate(span)).toBe(true); expect(blocksQuickCreate(document.createElement('textarea'))).toBe(true)
})
it('opens through a shell event without changing location', () => {
  const listener = vi.fn(); window.addEventListener(QUICK_CREATE_EVENT, listener)
  const before = location.hash; openQuickCreate({ channelId: 'mine', urls: ['source'] })
  expect(listener.mock.calls[0][0].detail).toEqual({ channelId: 'mine', urls: ['source'] }); expect(location.hash).toBe(before)
  window.removeEventListener(QUICK_CREATE_EVENT, listener)
})
it('round trips a pending workbench selection, then prefers an admitted job', () => {
  const path = paths.workbench({ queueItem: 'row / two' }); expect(routePath(parseHash(`#${path}`).route)).toBe(path)
  expect(cloneTarget({ jobId: 'job', queueItemId: 'row' })).toBe('/?video=job')
})

it('guards synchronous double submit before a second mutation can start', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  await act(async () => {
    const form = document.querySelector('form')!
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
  })
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(1)
})

it('uses a video-only override and resets to the new channel default without PATCH', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Template"]')!.click())
  await act(async () => [...document.querySelectorAll<HTMLElement>('[role="option"]')].find((option) => option.textContent?.includes('Phim tài liệu'))!.click())
  await tick()
  expect(calls.filter((call) => call.path === '/api/clone/plan').at(-1)?.body.templateId).toBe('doc')
  expect(document.body.textContent).toContain('Chỉ video này')
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Kênh"]')!.click())
  await act(async () => [...document.querySelectorAll<HTMLElement>('[role="option"]')].find((option) => option.textContent?.includes('Kênh khác'))!.click())
  await tick()
  expect(calls.filter((call) => call.path === '/api/clone/plan').at(-1)?.body).toEqual({ channelId: 'other', urls: ['https://youtu.be/abcdefghijk'] })
  expect(vi.mocked(fetch).mock.calls.some(([, init]) => init?.method === 'PATCH')).toBe(false)
})