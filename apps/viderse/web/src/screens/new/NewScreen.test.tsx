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
let failJobs: boolean, failPlan: boolean
let pendingPlan: Promise<Response> | null
const close = vi.fn()
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const tick = async (ms = 0) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const input = async (value: string) => {
  const field = document.querySelector<HTMLTextAreaElement>('#quick-create-urls')!
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(field, value); field.dispatchEvent(new Event('input', { bubbles: true })) })
}
const submit = async () => { await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))) }
beforeEach(async () => {
  vi.useFakeTimers(); cache.clear(); close.mockClear(); calls = []; failJobs = false; failPlan = false; pendingPlan = null
  plan = { accepted: [{ url: 'https://youtu.be/abcdefghijk', videoId: 'abcdefghijk', title: 'Video thật', durationSec: 180, outlier: null, hasCaptions: true, thumbnailUrl: '/source.jpg' }], duplicates: [], invalid: [], position: 'front', etaStart: null, paceLeftToday: 2, overflowDays: 0, template: { id: 'standard', name: 'Mẫu chuẩn' }, imageCalls: 12, priceUSD: 0.24 }
  result = { items: [{ id: 'real-queue-row', jobId: null }], duplicates: [], invalid: [] }
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/owner-channels') return json({ channels: [{ id: 'mine', name: 'Kênh của tôi', defaultTemplateId: 'standard', defaultTemplateName: 'Mẫu chuẩn', sources: [] }, { id: 'other', name: 'Kênh khác', defaultTemplateId: 'doc', defaultTemplateName: 'Phim tài liệu', sources: [] }] })
    if (path === '/api/templates') return json({ items: [{ id: 'standard', name: 'Mẫu chuẩn' }, { id: 'doc', name: 'Phim tài liệu' }] })
    if (path === '/api/clone/plan' || path === '/api/jobs') {
      calls.push({ path, body: JSON.parse(String(init?.body)) })
      return path === '/api/jobs' ? json(failJobs ? { error: 'Lỗi mạng' } : result, failJobs ? 500 : 200) : pendingPlan ?? json(failPlan ? { error: 'Không đọc được nguồn' } : plan, failPlan ? 500 : 200)
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

it('starts with neutral source guidance, not a completed empty plan', async () => {
  expect(document.querySelector('header')?.textContent).toContain('video nguồn trên YouTube')
  expect(document.querySelector('textarea')?.placeholder).toContain('video nguồn YouTube')
  expect(document.body.textContent).not.toContain('đối thủ')
  expect(document.querySelector('footer')?.textContent).toContain('để bắt đầu')
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
  await submit()
  expect(calls).toHaveLength(0)
})

it('shows pending metadata during debounce and an in-flight request without claiming zero new videos', async () => {
  const { promise, resolve: resolvePlan } = Promise.withResolvers<Response>()
  pendingPlan = promise
  await input('https://youtu.be/abcdefghijk\nhttps://youtu.be/lmnopqrstuv')
  expect(document.querySelector('footer')?.textContent).toContain('Đang đọc thông tin video')
  expect(document.body.textContent).toContain('Đang kiểm tra thông tin')
  expect(document.body.textContent).not.toContain('0 video mới')
  await tick(300)
  expect(calls).toHaveLength(1)
  await submit()
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(0)
  await act(async () => resolvePlan(json(plan)))
  expect(document.querySelector('footer')?.textContent).toContain('Lên đầu hàng chờ')
})

it('distinguishes failed metadata from duplicates and recovers through explicit retry', async () => {
  failPlan = true
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(document.querySelector('footer')?.textContent).toContain('Không đọc được thông tin video')
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
  await submit()
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(0)
  failPlan = false
  await act(async () => [...document.querySelectorAll('button')].find((button) => button.textContent === 'Thử đọc lại thông tin')!.click())
  await tick()
  expect(document.body.textContent).toContain('Video thật')
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(false)
})

it('explains duplicate-only and invalid-only plans without enabling submission', async () => {
  plan.accepted = []
  plan.duplicates = [{ url: 'https://youtu.be/abcdefghijk', jobId: null, queueItemId: 'waiting', state: 'queued' }]
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(document.querySelector('footer')?.textContent).toContain('Các video nguồn đã có cho kênh này')
  plan.duplicates = []; plan.invalid = [{ url: 'bad-link', reason: 'not_youtube' }]
  await input('bad-link'); await tick(300)
  expect(document.querySelector('footer')?.textContent).toContain('Không có link video YouTube đọc được')
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(true)
})

it('invalidates an accepted plan immediately when the source draft changes or clears', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')?.disabled).toBe(false)
  await input('https://youtu.be/lmnopqrstuv')
  await submit()
  expect(document.querySelector('footer')?.textContent).toContain('Đang đọc thông tin video')
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(0)
  await input('')
  expect(document.querySelector('footer')?.textContent).toContain('để bắt đầu')
  expect(document.querySelector('footer')?.textContent).not.toContain('$0.24')
})

it('keeps the accepted-plan template guard and explains a missing channel default', async () => {
  plan.template = { id: '', name: '' }
  await input('https://youtu.be/abcdefghijk'); await tick(300); await submit()
  expect(document.querySelector('footer')?.textContent).toContain('Kênh chưa có template mặc định')
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(0)
})

it('closes the template menu with Escape without discarding the video dialog', async () => {
  const trigger = document.querySelector<HTMLButtonElement>('[aria-label="Template"]')!
  await act(async () => trigger.click())
  expect(document.querySelector('[data-select-pop]')).not.toBeNull()
  await act(async () => trigger.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
  expect(document.querySelector('[data-select-pop]')).toBeNull()
  expect(document.querySelector('form')).not.toBeNull()
  expect(close).not.toHaveBeenCalled()
})

it('keeps mobile template selection in a separate sheet and preserves the source draft', async () => {
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: query === '(max-width: 767px)', addEventListener() {}, removeEventListener() {} })))
  await act(async () => root.render(<NewVideoDialog key="mobile" request={{ urls: ['https://youtu.be/abcdefghijk'] }} onClose={close} />))
  await tick(300)
  const trigger = document.querySelector<HTMLButtonElement>('[aria-label="Template"]')!
  expect(trigger.getAttribute('aria-haspopup')).toBe('dialog')
  await act(async () => trigger.click()); await tick(260)
  expect(document.querySelector('form')).toBeNull()
  expect(document.querySelector('[data-select-pop]')).toBeNull()
  expect(document.querySelector('.m-sheet-title')?.textContent).toContain('Template')
  await act(async () => [...document.querySelectorAll('button')].find((button) => button.textContent === 'Phim tài liệu')!.click())
  await tick(260)
  expect(document.querySelector<HTMLTextAreaElement>('textarea')?.value).toBe('https://youtu.be/abcdefghijk')
  expect(calls.filter((call) => call.path === '/api/clone/plan').at(-1)?.body.templateId).toBe('doc')
  expect(calls.filter((call) => call.path === '/api/jobs')).toHaveLength(0)
  expect(close).not.toHaveBeenCalled()
})

it('ignores an aborted plan response after the source draft changes', async () => {
  const { promise, resolve } = Promise.withResolvers<Response>()
  pendingPlan = promise
  const oldPlan = plan
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  pendingPlan = null
  plan = { ...plan, accepted: [{ ...plan.accepted[0], url: 'https://youtu.be/lmnopqrstuv', videoId: 'lmnopqrstuv', title: 'Nguồn mới' }] }
  await input('https://youtu.be/lmnopqrstuv'); await tick(300)
  expect(document.body.textContent).toContain('Nguồn mới')
  await act(async () => resolve(json(oldPlan)))
  expect(document.body.textContent).toContain('Nguồn mới')
  expect(document.body.textContent).not.toContain('Video thật')
  await submit()
  expect(calls.find((call) => call.path === '/api/jobs')?.body.urls).toEqual(['https://youtu.be/lmnopqrstuv'])
})