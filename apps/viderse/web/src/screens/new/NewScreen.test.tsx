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
    if (path === '/api/owner-channels') return response({ channels: [
      { id: 'mine', name: 'Kênh của tôi', pacePerDay: 5, templates: [{ id: 'standard', name: 'Mẫu chuẩn', position: 0 }], sources: [] },
      // Kênh để trống tên lúc tạo: nhận diện sẽ đặt tên sau.
      { id: 'other', name: '', pacePerDay: 3, templates: [{ id: 'doc', name: 'Phim tài liệu', position: 0 }], sources: [] },
    ] })
    if (path.includes('/videos')) return response({ videos: [], nextCursor: null, total: 0 })
    if (path === '/api/clone/plan') { calls.push({ path, body: JSON.parse(String(init?.body)) }); return response(plan) }
    if (path === '/api/clone/enqueue') { calls.push({ path, body: JSON.parse(String(init?.body)) }); return response({ items: [] }) }
    return response({ rivals: [], totalVideos: 0, newOutliers: 0 })
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
  await act(async () => { root.render(<NewScreen route={{ name: 'new' }} />) })
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

const viewport = async (mobile: boolean) => {
  vi.mocked(matchMedia).mockReturnValue({ matches: mobile, addEventListener() {}, removeEventListener() {} } as unknown as MediaQueryList)
  await act(async () => { root.render(<NewScreen key={String(mobile)} route={{ name: 'new' }} />) })
  await tick(0)
}

const multiInput = async (value: string) => {
  await act(async () => el.querySelectorAll<HTMLButtonElement>('[role="radio"]')[1].click())
  await input(value)
}

it.each([false, true])('uses the accepted count and priority plan before enqueueing (mobile=%s)', async (mobile) => {
  await viewport(mobile)
  plan = { ...plan, position: 'back', duplicates: [{ url: 'https://youtu.be/duplicate00', jobId: 'old', state: 'ready' }], invalid: [{ url: 'bad-link', reason: 'unreadable' }] }
  await multiInput('https://youtu.be/abcdefghijk\nhttps://youtu.be/duplicate00\nbad-link')
  await tick(300)
  const toggle = el.querySelector<HTMLButtonElement>('[role="switch"]')!
  expect(toggle.textContent).toContain('Ưu tiên cả 1')
  expect(toggle.getAttribute('aria-checked')).toBe('false')
  expect(calls.at(-1)?.body.priority).toBe('batch')
  plan = { ...plan, position: 'front', etaStart: '2026-11-01T00:40:00Z' }
  await act(async () => toggle.click())
  await tick(0)
  expect(toggle.getAttribute('aria-checked')).toBe('true')
  expect(calls.at(-1)?.body.priority).toBe('pick')
  expect(el.querySelector('.nv-plantext')?.textContent).toContain('Cả 1 lên đầu hàng chờ, không theo nhịp kênh')
  expect(el.querySelector('.nv-summary')?.textContent).toContain('Ưu tiên: cả 1 chạy ngay khi có luồng trống.')
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(calls.find((call) => call.path.endsWith('enqueue'))?.body).toEqual({
    channelId: 'mine', urls: ['https://youtu.be/abcdefghijk'], priority: 'pick',
  })
})

it.each([false, true])('keeps image choices in the selected template, not web plan/enqueue (mobile=%s)', async (mobile) => {
  await viewport(mobile)
  expect(el.querySelector('.nv-noimages')).toBeNull()
  expect(el.querySelector('input[type="checkbox"]')).toBeNull()
  await input('https://youtu.be/abcdefghijk')
  await tick(300)
  expect(calls[0].body).not.toHaveProperty('noImages')
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  const enqueued = calls.find((call) => call.path.endsWith('enqueue'))
  expect(enqueued).toBeDefined()
  expect(enqueued?.body).not.toHaveProperty('noImages')
})

it('returns to the batch plan when priority is unchecked and never counts rejected links', async () => {
  await viewport(true)
  plan = { ...plan, position: 'back' }
  await multiInput('https://youtu.be/abcdefghijk\nbad-link')
  await tick(300)
  const toggle = el.querySelector<HTMLButtonElement>('[role="switch"]')!
  plan = { ...plan, position: 'front' }
  await act(async () => toggle.click())
  await tick(0)
  plan = { ...plan, position: 'back', accepted: [], invalid: [{ url: 'bad-link', reason: 'unreadable' }] }
  await act(async () => toggle.click())
  await tick(0)
  expect(toggle.getAttribute('aria-checked')).toBe('false')
  expect(toggle.textContent).toContain('Ưu tiên cả 0')
  expect(calls.at(-1)?.body.priority).toBe('batch')
  await act(async () => el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })))
  expect(calls.some((call) => call.path.endsWith('enqueue'))).toBe(false)
})

it('uses daily pace separately from remaining turns without repeating skipped links on mobile', async () => {
  await viewport(true)
  const accepted = plan.accepted as object[]
  plan = { ...plan, position: 'back', paceLeftToday: 2, accepted: [...accepted, { ...accepted[0], url: 'https://youtu.be/lmnopqrstuv' }, { ...accepted[0], url: 'https://youtu.be/another0000' }], duplicates: [{ url: 'https://youtu.be/duplicate00', jobId: 'old', state: 'ready' }] }
  await multiInput('https://youtu.be/abcdefghijk\nhttps://youtu.be/lmnopqrstuv\nhttps://youtu.be/another0000\nhttps://youtu.be/duplicate00')
  await tick(300)
  expect(el.querySelector('.nv-plantext')?.textContent).toContain('theo nhịp 5 video/ngày của Kênh của tôi. Hôm nay còn 2 lượt.')
  expect(el.querySelector('.nv-summary')?.textContent).toBe('3 link đọc được1 đã làm cho kênh này · sẽ bỏ quaHôm nay kênh còn 2 lượt, video thứ 3 chạy ngày mai.')
})

// Ô chọn Kênh/Template là ô chọn của kit (nút mở danh sách), không phải <select> gốc.
const picker = (label: string) => el.querySelector<HTMLButtonElement>(`button[role="combobox"][aria-label="${label}"]`)!
const pick = async (label: string, option: string) => {
  await act(async () => picker(label).click())
  const target = [...el.querySelectorAll<HTMLElement>('[role="option"]')].find((node) => node.textContent === option)
  expect(target, `lựa chọn “${option}” trong ${label}`).toBeDefined()
  await act(async () => target!.click())
}

it.each([false, true])('plans with the picked template and returns to rotation when the channel changes (mobile=%s)', async (mobile) => {
  await viewport(mobile)
  await input('https://youtu.be/abcdefghijk'); await tick(300)
  expect(calls.at(-1)?.body).not.toHaveProperty('templateId')
  await pick('Template', 'Mẫu chuẩn')
  await tick(300)
  expect(calls.at(-1)?.body).toMatchObject({ channelId: 'mine', templateId: 'standard' })
  expect(picker('Template').textContent).toContain('Mẫu chuẩn')
  await pick('Kênh', 'Kênh chưa đặt tên')
  await tick(300)
  // Đổi kênh luôn trả template về luân phiên, kể cả khi kênh mới có template riêng.
  expect(calls.at(-1)?.body).toMatchObject({ channelId: 'other' })
  expect(calls.at(-1)?.body).not.toHaveProperty('templateId')
  expect(picker('Kênh').textContent).toContain('Kênh chưa đặt tên')
  expect(picker('Template').textContent).toContain('Theo luân phiên')
})

it('names a channel that was created without a name instead of showing an empty value', async () => {
  await act(async () => picker('Kênh').click())
  expect([...el.querySelectorAll('[role="option"]')].map((node) => node.textContent)).toEqual(['Kênh của tôi', 'Kênh chưa đặt tên'])
  expect(picker('Kênh').textContent).toContain('Kênh của tôi')
})
