import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MReviewScreen } from './MReviewScreen'
import type * as ReviewModule from '../useReview'
import { dismissToasts, ToastHost } from '@noverse/ui'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({ jobId: 'j', mutate: vi.fn().mockResolvedValue(true), loadImages: vi.fn(), status: 'review', nextVersion: 'none' as 'none' | 'drawing' | 'ready', cancelledVersions: [] as number[], pending: {} as Record<string, boolean> }))
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
      images: { [state.jobId]: { manifest: { revision: 3 }, images: [{
        id: image.id,
        versions: [
          { n: 1, state: 'ready', parts: [{ kind: 'scene', text: 'Cảnh 1 phần Cảnh: góc máy rộng.', source: { type: 'artist', label: 'Hoạ sĩ' } }] },
          { n: 2, state: 'ready', parts: [{ kind: 'scene', text: 'Cảnh 2 phần Cảnh: đêm mưa, quán nhỏ ven đường.', source: { type: 'artist', label: 'Hoạ sĩ' } }] },
          ...(state.nextVersion === 'none' ? [] : [{ n: 3, state: state.nextVersion, parts: [{ kind: 'scene', text: 'Bản mới', source: { type: 'artist', label: 'Hoạ sĩ' } }] }]),
        ].map(version => state.cancelledVersions.includes(version.n) ? { ...version, state: 'cancelled' } : version),
      }], items: [{ ...image, status: state.status, versions: [...image.versions, ...(state.nextVersion === 'none' ? [] : [{ version: 3, cutout: null, originalUrl: state.nextVersion === 'ready' ? '/v3.jpg' : '', prompt: 'Bản mới' }])].map(version => state.cancelledVersions.includes(version.version) ? { ...version, originalUrl: '' } : version) }], approved: false, approvedToday: null, learn: null, summary: null } },
      imageErrors: {}, pending: state.pending, loadImages: state.loadImages, loadJobs: vi.fn(), mutate: state.mutate,
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
  state.cancelledVersions = []
  state.pending = {}
  history.replaceState(null, '', '/')
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

it('keeps next-video navigation while approval hides the previous video before route propagation', async () => {
  history.replaceState(null, '', '/#/review?video=k')
  state.jobId = 'k'
  state.pending = { j: true }
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<MReviewScreen route={{ name: 'review', video: 'j' }} />) })
  expect(location.hash).toBe('#/review?video=k')
})

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
    expect(document.querySelector('textarea')?.value).toBe('Cảnh 1 phần Cảnh: góc máy rộng.')
    expect((button('Vẽ lại với chỉ dẫn này') as HTMLButtonElement).disabled).toBe(true)
    expect(state.mutate).not.toHaveBeenCalled()
  })

  it('gửi góp ý cho đúng bản đang chọn qua redrawImages với base của bản đó', async () => {
    const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'b', n: 3 }] }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    act(() => button('Vẽ lại')!.click())
    const textarea = document.querySelector<HTMLTextAreaElement>('textarea')!
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
      setter.call(textarea, 'Giữ đúng khung bản 1, thêm mưa nhẹ')
      textarea.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {
      button('Gửi cho Hoạ sĩ')!.click()
      for (let i = 0; i < 6; i += 1) await Promise.resolve()
    })
    expect(fetcher).toHaveBeenCalledOnce()
    const [, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({ imageIds: ['b'], baseVersion: 1, feedback: 'Giữ đúng khung bản 1, thêm mưa nhẹ' })
    expect(state.mutate).not.toHaveBeenCalled()
  })

  it('chỉ gửi phần Cảnh đã sửa của bản đang chọn, phần còn lại do base giữ', async () => {
    const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'b', n: 3 }] }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Xem prompt đã gửi"]')!.click())
    act(() => button('Sửa chỉ dẫn rồi vẽ lại')!.click())
    const textarea = document.querySelector<HTMLTextAreaElement>('textarea')!
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
      setter.call(textarea, 'Cảnh 1 phần Cảnh: góc máy rất rộng, có mưa.')
      textarea.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {
      button('Vẽ lại với chỉ dẫn này')!.click()
      for (let i = 0; i < 6; i += 1) await Promise.resolve()
    })
    expect(fetcher).toHaveBeenCalledOnce()
    const [, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({ imageIds: ['b'], baseVersion: 1, parts: { scene: 'Cảnh 1 phần Cảnh: góc máy rất rộng, có mưa.' } })
    expect(state.mutate).not.toHaveBeenCalled()
  })

  it('giữ bản nháp và base khi bản mới về trong lúc đang sửa', async () => {
    const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'b', n: 4 }] }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Bản 1"]')!.click())
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Xem prompt đã gửi"]')!.click())
    act(() => button('Sửa chỉ dẫn rồi vẽ lại')!.click())
    const textarea = document.querySelector<HTMLTextAreaElement>('textarea')!
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
      setter.call(textarea, 'Cảnh 1: bản nháp trước khi bản mới về.')
      textarea.dispatchEvent(new Event('input', { bubbles: true }))
    })
    state.nextVersion = 'ready'
    await act(async () => { root!.render(view()) })
    expect(document.querySelector<HTMLTextAreaElement>('textarea')?.value).toBe('Cảnh 1: bản nháp trước khi bản mới về.')
    await act(async () => {
      button('Vẽ lại với chỉ dẫn này')!.click()
      for (let i = 0; i < 6; i += 1) await Promise.resolve()
    })
    expect(JSON.parse(String((fetcher.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({
      imageIds: ['b'], baseVersion: 1, parts: { scene: 'Cảnh 1: bản nháp trước khi bản mới về.' },
    })
  })

  it('does not keep another video’s draft or close its prompt when an earlier redraw finishes', async () => {
    let reply!: (response: Response) => void
    const fetcher = vi.fn(() => new Promise<Response>(resolve => { reply = resolve }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    act(() => button('Vẽ lại')!.click())
    await act(async () => { button('Gửi cho Hoạ sĩ')!.click() })
    expect(fetcher).toHaveBeenCalledOnce()
    state.jobId = 'j2'
    await act(async () => { root!.render(view()) })
    const activeDialogs = () => [...document.querySelectorAll('[role="dialog"]')].filter(dialog => !dialog.closest('[inert]'))
    expect(activeDialogs()).toHaveLength(0)
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Xem prompt đã gửi"]')!.click())
    await act(async () => { reply(Response.json({ versions: [{ imageId: 'b', n: 3 }] })) })
    expect(activeDialogs()).toHaveLength(1)
    const edit = button('Sửa chỉ dẫn rồi vẽ lại')!
    expect(activeDialogs()[0]?.contains(edit)).toBe(true)
    act(() => edit.click())
    expect(activeDialogs()).toHaveLength(1)
    expect(activeDialogs()[0]?.querySelector<HTMLTextAreaElement>('textarea')?.value).toBe('Cảnh 2 phần Cảnh: đêm mưa, quán nhỏ ven đường.')
    expect(state.loadImages).toHaveBeenCalledWith('j', true)
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

  it('does not choose a cancelled version as the default or the edit source', async () => {
    state.cancelledVersions = [2]
    const node = await render()
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v1.jpg')
    const cancelled = node.querySelector<HTMLButtonElement>('[aria-label="Bản 2 · đã huỷ · không dùng làm nguồn vẽ lại"]')
    expect(cancelled?.disabled).toBe(true)
    expect(cancelled?.getAttribute('aria-pressed')).toBe('false')
    act(() => cancelled!.click())
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/v1.jpg')
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Xem prompt đã gửi"]')!.click())
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain(image.versions[0].prompt)
    act(() => button('Sửa chỉ dẫn rồi vẽ lại')!.click())
    expect(document.querySelector('textarea')?.value).toBe('Cảnh 1 phần Cảnh: góc máy rộng.')
  })

  it('does not redraw from a version cancelled while its feedback sheet was open', async () => {
    const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'b', n: 4 }] }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    act(() => button('Vẽ lại')!.click())
    state.cancelledVersions = [2]
    await act(async () => { root!.render(view()) })
    const textarea = document.querySelector<HTMLTextAreaElement>('textarea')!
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
      setter.call(textarea, 'Giữ nguyên khung, thêm mưa nhẹ')
      textarea.dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect((button('Gửi cho Hoạ sĩ') as HTMLButtonElement).disabled).toBe(true)
    await act(async () => {
      button('Gửi cho Hoạ sĩ')!.click()
      for (let i = 0; i < 6; i += 1) await Promise.resolve()
    })
    expect(fetcher).not.toHaveBeenCalled()
    expect(state.mutate).not.toHaveBeenCalled()
  })

  it('allows a fresh redraw after all earlier attempts were cancelled without using a cancelled source', async () => {
    state.cancelledVersions = [1, 2]
    const fetcher = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => Response.json({ versions: [{ imageId: 'b', n: 3 }] }))
    vi.stubGlobal('fetch', fetcher)
    const node = await render()
    expect(node.querySelector('[aria-label="Bản 2 · đã huỷ · không dùng làm nguồn vẽ lại"]')?.hasAttribute('disabled')).toBe(true)
    act(() => button('Vẽ lại')!.click())
    const textarea = document.querySelector<HTMLTextAreaElement>('textarea')!
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
      setter.call(textarea, 'Vẽ mới theo lời kể, thêm mưa nhẹ')
      textarea.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {
      button('Gửi cho Hoạ sĩ')!.click()
      for (let i = 0; i < 6; i += 1) await Promise.resolve()
    })
    expect(fetcher).toHaveBeenCalledOnce()
    const body = JSON.parse(fetcher.mock.calls[0][1]!.body as string)
    expect(body).toEqual({ imageIds: ['b'], feedback: 'Vẽ mới theo lời kể, thêm mưa nhẹ' })
    expect(state.loadImages).toHaveBeenCalledWith('j', true)
  })
})
