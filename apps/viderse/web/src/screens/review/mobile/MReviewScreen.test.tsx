import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MReviewScreen } from './MReviewScreen'
import type * as ReviewModule from '../useReview'
import { dismissToasts, ToastHost } from '@noverse/ui'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({ jobId: 'j', mutate: vi.fn().mockResolvedValue(true), loadImages: vi.fn(), status: 'review', nextVersion: 'none' as 'none' | 'drawing' | 'ready' }))
const image = {
  id: 'b', kind: 'scene', status: 'review', prompt: 'Đêm mưa, quán nhỏ ven đường', sceneId: '2',
  currentVersion: null, reviewVersion: null, startSec: 3, endSec: 9, narration: 'Lời kể trong cảnh',
  versions: [
    { version: 1, prompt: 'Góc máy rộng, chưa có mưa', cutout: null, originalUrl: '/v1.jpg', createdAt: '2026-11-02T10:00:00Z', source: 'gpt-image-2', width: 1280, height: 720 },
    { version: 2, prompt: 'Đêm mưa, quán nhỏ ven đường', cutout: null, originalUrl: '/v2.jpg', createdAt: '2026-11-02T10:03:00Z', source: 'gpt-image-2', width: 1280, height: 720 },
  ],
}
vi.mock('../useReview', async importOriginal => {
  const actual = await importOriginal<typeof ReviewModule>()
  return {
    ...actual,
    useReview: () => ({
      jobs: [{ id: state.jobId, channelId: 'c', stageGroup: 'review', subline: '', createdAt: '2026-11-02T10:00:00Z', source: { title: 'Video', thumbnailUrl: null } }],
      images: { [state.jobId]: { manifest: { revision: 3 }, images: [{ id: image.id, versions: state.nextVersion === 'none' ? [] : [{ n: 3, state: state.nextVersion }] }], items: [{ ...image, status: state.status, versions: [...image.versions, ...(state.nextVersion === 'none' ? [] : [{ version: 3, cutout: null, originalUrl: state.nextVersion === 'ready' ? '/v3.jpg' : '', prompt: 'Bản mới' }])] }], approved: false, approvedToday: null, learn: null, summary: null } },
      imageErrors: {}, pending: {}, loadImages: state.loadImages, loadJobs: vi.fn(), mutate: state.mutate,
    }),
  }
})
vi.mock('../../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../../data/useResource', () => ({ useResource: () => ({ data: { channels: [{ id: 'c', name: 'Kênh' }] } }) }))
let root: Root | undefined
let container: HTMLDivElement | undefined
afterEach(async () => {
  await act(async () => {
    dismissToasts()
    root?.unmount()
    container?.remove()
    await Promise.resolve()
  })
  root = undefined
  container = undefined
  state.jobId = 'j'
  state.status = 'review'
  state.nextVersion = 'none'
  vi.useRealTimers()
  vi.clearAllMocks()
  vi.unstubAllGlobals()
})

const view = () => <><MReviewScreen route={{ name: 'review' }} /><ToastHost /></>
const render = async () => {
  const node = document.createElement('div'); document.body.append(node); container = node; root = createRoot(node)
  await act(async () => { root!.render(view()) })
  return node
}
const button = (text: string) => [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === text)

describe('MReview — duyệt ảnh trên điện thoại', () => {
  it('keeps the preview and review open after a failed version change and prevents duplicate submissions', async () => {
    let reply!: (response: Response) => void
    const fetch = vi.fn(() => new Promise<Response>(resolve => { reply = resolve }))
    vi.stubGlobal('fetch', fetch)
    const node = await render()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    const use = button('Dùng bản 1')!
    await act(async () => { use.click(); use.click() })
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(use.disabled).toBe(true)
    await act(async () => { reply(new Response(JSON.stringify({ error: 'Đường truyền bị gián đoạn' }), { status: 503, headers: { 'Content-Type': 'application/json' } })) })
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v1.jpg')
    expect(node.querySelector('[aria-label="Bản 1"]')?.getAttribute('aria-pressed')).toBe('true')
    expect(button('Dùng bản 1')?.disabled).toBe(false)
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Đường truyền bị gián đoạn')
    expect(state.mutate).not.toHaveBeenCalled()
  })


  it('so bản trước không thay bản đã chọn, prompt và bản sửa lấy đúng phiên bản', async () => {
    const node = await render()
    const current = node.querySelector('main img')
    expect(current?.getAttribute('src')).toBe('/v2.jpg')
    act(() => button('So với bản 1')!.click())
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v1.jpg')
    expect(node.querySelector('[aria-label="Bản 2"]')?.getAttribute('aria-pressed')).toBe('true')
    act(() => button('Đang xem bản 1')!.click())
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v2.jpg')
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Xem prompt đã gửi"]')!.click())
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain(image.versions[0].prompt)
    expect(document.querySelector('[role="dialog"]')?.textContent).not.toContain(image.prompt)
    act(() => button('Sửa chỉ dẫn rồi vẽ lại')!.click())
    expect(document.querySelector('textarea')?.value).toBe(image.versions[0].prompt)
    expect(button('Vẽ lại với chỉ dẫn này')?.disabled).toBe(true)
    expect(state.mutate).not.toHaveBeenCalled()
  })

  it('selects the completed redraw when its drawing row was already visible', async () => {
    const node = await render()
    state.status = 'running'
    state.nextVersion = 'drawing'
    await act(async () => { root!.render(view()) })
    expect(node.textContent).toContain('Hoạ sĩ đang vẽ bản 3')
    expect(node.textContent).not.toContain('Hoạ sĩ đang vẽ bản 4')
    expect(node.querySelector('[aria-label="Bản 3"]')).toBeNull()
    state.status = 'review'
    state.nextVersion = 'ready'
    await act(async () => { root!.render(view()) })
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v3.jpg')
    expect(node.querySelector('[aria-label="Bản 3"]')?.getAttribute('aria-pressed')).toBe('true')
  })

  it('keeps version choices scoped to one video when another uses the same image id', async () => {
    const node = await render()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    state.jobId = 'another-video'
    await act(async () => { root!.render(view()) })
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v2.jpg')
    expect(node.querySelector('[aria-label="Bản 2"]')?.getAttribute('aria-pressed')).toBe('true')
    expect(state.mutate).not.toHaveBeenCalled()
  })
})
