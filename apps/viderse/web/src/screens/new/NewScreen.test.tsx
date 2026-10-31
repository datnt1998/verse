// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { NewScreen } from './NewScreen'
import { readFileSync } from 'node:fs'
import { URL as NodeURL } from 'node:url'
const golden = JSON.parse(readFileSync(new NodeURL('../../../../internal/pipeline/testdata/clone-views.golden.json', import.meta.url), 'utf8'))

let root: Root
let el: HTMLDivElement
let calls: { path: string; body: Record<string, unknown> }[]
let plan: Record<string, unknown>
const response = (body: unknown) => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } })
const tick = async (ms: number) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const input = async (value: string) => {
  const field = el.querySelector('input,textarea') as HTMLInputElement | HTMLTextAreaElement
  expect(field).not.toBeNull()
  await act(async () => {
    const proto = field.tagName === 'TEXTAREA' ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype
    Object.getOwnPropertyDescriptor(proto, 'value')!.set!.call(field, value)
    field.dispatchEvent(new Event('input', { bubbles: true }))
  })
}
beforeEach(async () => {
  vi.useFakeTimers()
  cache.clear()
  calls = []
  plan = { accepted: [{ url: 'https://youtu.be/abcdefghijk', videoId: golden[1].source.id, title: golden[1].source.title, durationSec: 180, outlier: null, hasCaptions: true }], duplicates: [], invalid: [], position: 'front', etaStart: null, paceLeftToday: 2, overflowDays: 0, template: { id: 'standard', name: 'Mẫu chuẩn', fromRotation: true } }
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/owner-channels') return response({ channels: [{ id: 'mine', name: 'Kênh của tôi', pacePerDay: 5, templates: [{ id: 'standard', name: 'Mẫu chuẩn', position: 0 }], sources: [] }] })
    if (path.includes('/videos')) return response({ videos: [], nextCursor: null, total: 0 })
    if (path === '/api/clone/plan') { calls.push({ path, body: JSON.parse(String(init?.body)) }); return response(plan) }
    if (path === '/api/clone/enqueue') { calls.push({ path, body: JSON.parse(String(init?.body)) }); return response({ items: [] }) }
    return response({ rivals: [], totalVideos: 0, newOutliers: 0 })
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
  await act(async () => { root.render(<NewScreen />) })
  await tick(0)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); cache.clear(); vi.useRealTimers(); vi.unstubAllGlobals() })
it('waits 300 ms and cancels the superseded plan request', async () => {
  await input('https://youtu.be/abcdefghijk'); await tick(299)
  expect(calls).toHaveLength(0)
  await input('https://youtu.be/lmnopqrstuv'); await tick(299)
  expect(calls).toHaveLength(0)
  await tick(1)
  expect(calls).toHaveLength(1)
  expect(calls[0].body.urls).toEqual(['https://youtu.be/lmnopqrstuv'])
})
it('prevents enqueue for a duplicate and opens the existing video', async () => {
  plan = { ...plan, accepted: [], duplicates: [{ url: 'https://youtu.be/abcdefghijk', jobId: 'existing', state: 'working' }] }
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  const link = [...el.querySelectorAll('a')].find(a => a.textContent === 'Mở video đang làm')
  expect(link?.getAttribute('href')).toContain('existing')
  expect(el.querySelector('button[type="submit"]')?.getAttribute('aria-disabled')).toBe('true')
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(calls.filter(c => c.path.endsWith('enqueue'))).toHaveLength(0)
})
it('plans multiple links as batch and only enqueues accepted links', async () => {
  const accepted = plan.accepted as object[]
  plan = { ...plan, accepted: [...accepted, { ...accepted[0], url: 'https://youtu.be/lmnopqrstuv', videoId: 'lmnopqrstuv' }], duplicates: [{ url: 'https://youtu.be/duplicate00', jobId: 'old', state: 'ready' }], position: 'back', overflowDays: 1 }
  await act(async () => (el.querySelectorAll<HTMLButtonElement>('[role="radio"]')[1]).click())
  await input('https://youtu.be/abcdefghijk\nhttps://youtu.be/lmnopqrstuv\nhttps://youtu.be/duplicate00'); await tick(300)
  expect(calls[0].body.priority).toBe('batch')
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(calls.find(c => c.path.endsWith('enqueue'))?.body.urls).toEqual(['https://youtu.be/abcdefghijk', 'https://youtu.be/lmnopqrstuv'])
})
it('does not enqueue when the real plan has no template assigned', async () => {
  plan = { ...plan, template: { id: '', name: '', fromRotation: false } }
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(calls.filter((call) => call.path.endsWith('enqueue'))).toHaveLength(0)
})
it('renders the source preview for an already queued video', async () => {
  plan = { ...plan, accepted: [], duplicates: [{ url: 'https://youtu.be/abcdefghijk', jobId: null, state: 'queued', title: 'Nguồn đã xếp', thumbnailUrl: '/duplicate-source.jpg', durationSec: 120, outlier: 4.2, hasCaptions: true, sourceChannelTitle: 'Đối thủ', viewCount: 1000, publishedAt: null }] }
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(el.querySelector('.nv-preview img')?.getAttribute('src')).toBe('/duplicate-source.jpg')
})
it('shows the linked-source badge only when the plan confirms the relationship', async () => {
  plan = { ...plan, accepted: [{ ...(plan.accepted as object[])[0], rivalLinked: true }] }
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(el.querySelector('.nv-linked')).not.toBeNull()
})
