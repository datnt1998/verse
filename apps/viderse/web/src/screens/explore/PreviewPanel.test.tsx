// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi, type Mock } from 'vitest'
import { PreviewPanel } from './PreviewPanel'
import type { Video } from '../../api/rivals'
import type { ClonePlan } from '../../api/clone'

let root: Root, el: HTMLDivElement
let enqueue: Mock
let duplicate = false
const video = (id: string): Video => ({ id, channelId: 'r', title: id, durationSec: 60, viewCount: 10, publishedAt: null, exactTime: false, score: null, hasCaptions: null, thumbnailUrl: '', url: `https://youtu.be/${id}` })
const render = async (id: string) => {
  await act(async () => { root.render(<PreviewPanel video={video(id)} rival={undefined} channels={[{ id: 'mine', name: 'Kênh' }]} onClose={() => {}} />) })
}
const make = () => [...el.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent?.includes('Làm video') || b.textContent?.includes('Đã vào hàng chờ'))!
beforeEach(() => {
  duplicate = false; enqueue = vi.fn()
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path === '/api/clone/enqueue') { enqueue(); return new Response(JSON.stringify({ items: [{ id: 'q', jobId: null }], duplicates: [] })) }
    const plan: ClonePlan = { accepted: duplicate ? [] : [{ url: 'https://youtu.be/one', videoId: 'one', title: 'one', durationSec: 60, outlier: null, hasCaptions: false }], duplicates: duplicate ? [{ url: 'https://youtu.be/one', jobId: null, state: 'queued' }] : [], invalid: [], position: 'front', etaStart: null, paceLeftToday: 5, overflowDays: 0, template: { id: 't', name: 'Mẫu', fromRotation: true }, imageCalls: 0, priceUSD: null }
    return new Response(JSON.stringify(plan))
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); vi.unstubAllGlobals() })
it('blocks a duplicate that is queued without a job id', async () => {
  duplicate = true; await render('one')
  expect(make().getAttribute('aria-disabled')).toBe('true')
  await act(async () => make().click())
  expect(enqueue).not.toHaveBeenCalled()
})
it('allows the next selected video after enqueueing the previous one', async () => {
  await render('one'); await act(async () => make().click())
  expect(enqueue).toHaveBeenCalledTimes(1)
  await render('two')
  expect(make().getAttribute('aria-disabled')).not.toBe('true')
  await act(async () => make().click())
  expect(enqueue).toHaveBeenCalledTimes(2)
})


it('shows the supplied score evidence and dismisses it on outside click', async () => {
  const source: Video = { ...video('one'), score: 4, scoreReason: { agent: 'scout', text: 'Dựa trên tổng lượt xem.', facts: [{ label: 'Trung vị', value: '25', source: '150 video gần nhất' }] } }
  await act(async () => { root.render(<PreviewPanel video={source} rival={undefined} channels={[{ id: 'mine', name: 'Kênh' }]} onClose={() => {}} />) })
  const trigger = [...el.querySelectorAll('button')].find(b => b.textContent?.includes('Vì sao'))!
  await act(async () => trigger.click())
  expect(el.querySelector('[role="dialog"]')?.textContent).toContain('150 video gần nhất')
  await act(async () => { document.body.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true })) })
  expect(el.querySelector('[role="dialog"]')).toBeNull()
})
