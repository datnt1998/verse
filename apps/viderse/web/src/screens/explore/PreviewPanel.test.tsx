// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi, type Mock } from 'vitest'
import { PreviewPanel } from './PreviewPanel'
import type { Video, VideoPreview } from '../../api/rivals'
import type { ClonePlan } from '../../api/clone'

let root: Root, el: HTMLDivElement
let enqueue: Mock
let duplicate = false
const video = (id: string): Video => ({ id, channelId: 'r', title: id, durationSec: 60, viewCount: 10, publishedAt: null, exactTime: false, score: null, hasCaptions: null, thumbnailUrl: '', url: `https://youtu.be/${id}`, usage: [] })
const render = async (id: string) => {
  await act(async () => { root.render(<PreviewPanel video={video(id)} rival={undefined} channels={[{ id: 'mine', name: 'Kênh' }]} onClose={() => {}} />) })
}
const make = () => [...el.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent?.includes('Làm video') || b.textContent?.includes('Đã vào hàng chờ'))!
beforeEach(() => {
  duplicate = false; enqueue = vi.fn()
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path === '/api/jobs') { enqueue(); return new Response(JSON.stringify({ items: [{ id: 'q', jobId: null }], duplicates: [] })) }
    if (path.endsWith('/preview')) {
      const preview: VideoPreview = { videoId: decodeURIComponent(path.split('/')[5] ?? ''), source: 'youtube', state: 'ready', chapters: [] }
      return new Response(JSON.stringify(preview))
    }
    const plan: ClonePlan = { accepted: duplicate ? [] : [{ url: 'https://youtu.be/one', videoId: 'one', title: 'one', durationSec: 60, outlier: null, hasCaptions: false }], duplicates: duplicate ? [{ url: 'https://youtu.be/one', jobId: null, state: 'queued' }] : [], invalid: [], position: 'front', etaStart: null, paceLeftToday: 5, overflowDays: 0, template: { id: 't', name: 'Mẫu' }, imageCalls: 0, priceUSD: null }
    return new Response(JSON.stringify(plan))
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); vi.unstubAllGlobals() })
it('hands the selected source to the shell composer without enqueueing', async () => {
  const open = vi.fn(); window.addEventListener('viderse:quick-create', open)
  await render('one'); await act(async () => make().click())
  expect(open.mock.calls[0][0].detail).toEqual({ channelId: 'mine', urls: ['https://youtu.be/one'], fromSelection: true })
  expect(enqueue).not.toHaveBeenCalled()
  await render('two'); await act(async () => make().click())
  expect(open.mock.calls[1][0].detail.urls).toEqual(['https://youtu.be/two'])
  expect([...el.querySelectorAll('button')].some((button) => button.textContent?.includes('Vì sao'))).toBe(false)
  window.removeEventListener('viderse:quick-create', open)
})
