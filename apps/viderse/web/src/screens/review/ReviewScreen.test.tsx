// Duyệt ảnh: khung không phóng quá bề rộng gốc, sửa prompt gửi đúng phần đã
// đổi + ảnh tham chiếu, Esc huỷ, ⌘↵ gửi, chọn nhiều gửi góp ý chung, thẻ lỗi
// có giờ đặt lại thật và vẽ bằng Grok đúng prompt đã sửa (§26, artboard Review).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReviewScreen } from './ReviewScreen'
import type * as ReviewModule from './useReview'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.stubGlobal('matchMedia', (query: string) => ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {} }))

const state = vi.hoisted(() => ({
  redraw: vi.fn().mockResolvedValue({ versions: [{ imageId: 'a', n: 3 }] }),
  cancel: vi.fn().mockResolvedValue({ imageId: 'a', n: 3, cancelled: true, costImages: 0, usedVersion: 2 }),
  use: vi.fn().mockResolvedValue({ imageId: 'a', n: 2, usedVersion: 2 }),
  upload: vi.fn().mockResolvedValue('media-1'),
  reload: vi.fn(),
  loadImages: vi.fn(),
  mutate: vi.fn().mockResolvedValue(true),
  images: { current: { images: [] as unknown[], quota: undefined as unknown } },
  progress: { current: {} as Record<string, unknown> },
}))

const part = (kind: string, text: string, label: string) => ({ kind, text, source: { type: 'artist', label } })
const version = (n: number, overrides: Record<string, unknown> = {}) => ({
  n, state: 'ready', url: `/img/v${n}.jpg`, previewUrl: null, width: 1280, height: 720,
  parts: [
    part('scene', `Cảnh ${n}: a guard on a stopped escalator.`, 'Hoạ sĩ viết từ lời kể'),
    part('style', 'Flat cinematic, teal palette.', 'prompt “Đêm xanh rêu”'),
    part('refs', 'Image 1: keep the camera angle.', '1 ảnh bên dưới'),
    part('learned', 'No readable text except signs.', 'prompt “Đêm xanh rêu”'),
    part('avoid', 'blurry, watermark', 'template Mẫu chuẩn'),
  ],
  sentText: `Cảnh ${n}: a guard on a stopped escalator.`,
  refs: [{ id: 'r1', role: 'layout', url: '/img/ref1.jpg', source: { type: 'rival_frame', label: 'Khung đối thủ · Nocturne Stories 14:02', videoId: 'v', atSec: 842 } }],
  provider: 'chatgpt', model: 'gpt-image-2', params: { size: '1280x720' },
  feedback: n === 1 ? null : 'tông lạnh hơn',
  diffFromPrev: n === 1 ? null : [['Cảnh 2', ''], [' trên sân thượng', 'ins'], [' dưới thang', 'del'], [' —', ''], [' ánh đèn xanh', 'ins']],
  startedAt: '2026-11-01T16:58:00Z', finishedAt: '2026-11-01T16:58:44Z', durationSec: 44, costImages: 1, error: null,
  ...overrides,
})
const image = (id: string, index: number, versions: unknown[], usedVersion = versions.length) => ({
  id, kind: 'scene', index, label: `Cảnh ${index}`, usedVersion, needsReview: false, status: 'review', refLimit: 4, versions,
})

vi.mock('./useReview', async importOriginal => {
  const actual = await importOriginal<typeof ReviewModule>()
  return {
    ...actual,
    imageURL: (_job: string, item: { id: string }) => `/legacy/${item.id}.jpg`,
    useReview: () => ({
      jobs: [{ id: 'j', channelId: 'c', source: { title: 'Video' }, subline: '' }],
      images: { j: { manifest: { revision: 4 }, approved: false, items: [
        { id: 'a', kind: 'scene', status: 'review', prompt: 'p', versions: [], sceneId: '3', narration: 'Lời kể cảnh 3.' },
        { id: 'b', kind: 'scene', status: 'review', prompt: 'q', versions: [], sceneId: '4' },
      ] } },
      error: false, imageErrors: {}, pending: {}, loadJobs: vi.fn(), loadImages: state.loadImages, mutate: state.mutate,
    }),
  }
})
vi.mock('../../api/images', () => ({
  useJobImages: () => ({ data: { images: state.images.current.images, quota: state.images.current.quota, revision: 4 }, error: false, loading: false, reload: state.reload, progress: state.progress.current }),
  redrawImages: state.redraw,
  cancelImageVersion: state.cancel,
  useImageVersion: state.use,
  uploadRefImage: state.upload,
  tooManyRefs: () => null,
}))
vi.mock('./mobile/MReviewScreen', () => ({ MReviewScreen: () => <div>phone</div> }))
vi.mock('../../components', () => ({
  Orb: () => null,
  SystemLoading: () => null,
  DevelopEffect: ({ state: effectState, title, detail, onCancel, children }: { state: string; title?: string; detail?: string; onCancel?: () => void; children?: React.ReactNode }) => (
    <div data-develop={effectState}>{children}{title}{detail}{onCancel && <button type="button" onClick={onCancel}>Huỷ lần vẽ</button>}</div>
  ),
}))
vi.mock('../../components/learn/LearnSlot', () => ({ LearnSlot: () => null }))
vi.mock('../../components/autonomy/ScoreParts', () => ({ ScoreBadge: () => null, ScoreBanner: () => null, ScorePanel: () => null }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../data/useResource', () => ({ useResource: (key: unknown[]) => key?.[0] === 'channels' ? ({ data: { channels: [{ id: 'c', name: 'Kênh' }] } }) : ({ data: undefined }) }))

let root: Root | undefined
afterEach(() => { if (root) act(() => root!.unmount()); root = undefined; document.body.innerHTML = ''; vi.clearAllMocks(); state.progress.current = {} })

const render = async () => {
  const node = document.createElement('div')
  document.body.append(node)
  root = createRoot(node)
  await act(async () => { root!.render(<ReviewScreen />) })
  return node
}
const button = (node: HTMLElement, text: string) => [...node.querySelectorAll('button')].find(entry => entry.textContent?.includes(text))
const type = async (field: HTMLInputElement | HTMLTextAreaElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype, 'value')!.set!
  await act(async () => { setter.call(field, value); field.dispatchEvent(new Event('input', { bubbles: true })) })
}
const key = async (init: KeyboardEventInit) => { await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, ...init })) }) }


describe('chỉ dẫn đã gửi', () => {
  it('hiện theo phần kèm nguồn và dòng meta của bản', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    expect(node.textContent).toContain('Chỉ dẫn đã gửi')
    expect(node.textContent).toContain('· Hoạ sĩ viết từ lời kể')
    expect(node.textContent).toContain('ChatGPT · gpt-image-2 · 1280×720 · vẽ 44 giây')
    expect(node.textContent).toContain('So với bản 1: Hoạ sĩ đổi 2 chỗ theo góp ý của bạn.')
    // Thay đổi so với bản trước: thêm xanh, bỏ gạch — chỉ ở phần Cảnh.
    const inserted = [...node.querySelectorAll('ins')].map(entry => entry.textContent)
    const deleted = [...node.querySelectorAll('del')].map(entry => entry.textContent)
    expect(inserted).toEqual([' trên sân thượng', ' ánh đèn xanh'])
    expect(deleted).toEqual([' dưới thang'])
    // Ẩn thay đổi → hiện nguyên văn phần Cảnh của bản đang xem.
    await act(async () => { button(node, 'Ẩn thay đổi')!.click() })
    expect(node.querySelectorAll('ins')).toHaveLength(0)
    expect(node.textContent).toContain('Cảnh 2: a guard on a stopped escalator.')
  })
})

describe('sửa chỉ dẫn', () => {
  it('chỉ gửi phần đã đổi và ảnh tham chiếu khi ảnh tham chiếu đổi', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    await act(async () => { button(node, 'Sửa chỉ dẫn')!.click() })
    const scene = node.querySelector<HTMLTextAreaElement>('#prompt-scene')!
    await type(scene, 'A guard on the roof at dawn.')
    await act(async () => { node.querySelector<HTMLButtonElement>('[aria-label="Bỏ ảnh tham chiếu 1"]')!.click() })
    await act(async () => { button(node, 'Vẽ lại với chỉ dẫn này')!.click() })
    expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['a'], parts: { scene: 'A guard on the roof at dawn.' }, refs: [] })
  })

  it('sửa rồi gõ lại đúng nguyên văn thì không còn gì để gửi', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    await act(async () => { button(node, 'Sửa chỉ dẫn')!.click() })
    const scene = node.querySelector<HTMLTextAreaElement>('#prompt-scene')!
    const original = scene.value
    await type(scene, 'A guard on the roof at dawn.')
    expect(node.textContent).toContain('Đã sửa 1 chỗ trong chỉ dẫn cảnh 3')
    await type(scene, original)
    expect(node.textContent).toContain('Sửa chỉ dẫn cảnh 3')
    expect(button(node, 'Vẽ lại với chỉ dẫn này')!.getAttribute('aria-disabled')).toBe('true')
    await act(async () => { button(node, 'Vẽ lại với chỉ dẫn này')!.click() })
    expect(state.redraw).not.toHaveBeenCalled()
  })

  it('Esc huỷ sửa, ⌘↵ gửi', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    await act(async () => { button(node, 'Sửa chỉ dẫn')!.click() })
    expect(node.querySelector('#prompt-scene')).not.toBeNull()
    await key({ key: 'Escape' })
    expect(node.querySelector('#prompt-scene')).toBeNull()
    expect(node.textContent).toContain('Sửa chỉ dẫn')

    await act(async () => { button(node, 'Sửa chỉ dẫn')!.click() })
    await type(node.querySelector<HTMLTextAreaElement>('#prompt-style')!, 'Grainy 35mm, cold teal.')
    await key({ key: 'Enter', metaKey: true })
    expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['a'], parts: { style: 'Grainy 35mm, cold teal.' } })
  })
})

describe('chọn nhiều', () => {
  it('Shift + bấm chọn 2 ảnh, gửi góp ý chung cho cả hai', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)]), image('b', 4, [version(1)])], quota: undefined }
    const node = await render()
    await act(async () => {
      for (const at of [0, 1]) node.querySelectorAll<HTMLButtonElement>('[role="option"]')[at].dispatchEvent(new MouseEvent('click', { bubbles: true, shiftKey: true }))
    })
    expect(node.textContent).toContain('2 ảnh đã chọn')
    await type(node.querySelector<HTMLInputElement>('#redraw-batch')!, 'không có chữ trong ảnh')
    await act(async () => { button(node, 'Vẽ lại 2 ảnh')!.click() })
    expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['a', 'b'], feedback: 'không có chữ trong ảnh' })
  })
})

describe('thẻ lỗi', () => {
  it('giờ đặt lại thật và Vẽ bằng Grok đúng prompt đã sửa của bản lỗi', async () => {
    const failed = version(3, {
      state: 'failed', url: null, finishedAt: null, durationSec: null,
      error: { reason: 'quota', retryAt: '2026-11-02T00:30:00Z', message: 'ChatGPT đã hết hạn mức ảnh hôm nay (50/50)' },
      parts: [part('scene', 'Cảnh 3 đã sửa: guard on the roof.', 'Hoạ sĩ sửa theo góp ý')],
      refs: [],
    })
    state.images.current = { images: [image('a', 3, [version(1), version(2), failed], 2)], quota: { providers: [{ provider: 'chatgpt', label: 'ChatGPT', left: 0, total: 50, resetAt: '2026-11-02T00:30:00Z', paid: true }], left: 0, fallback: { provider: 'grok', label: 'Grok', left: 190 } } }
    const node = await render()
    expect(node.textContent).toContain('Chưa vẽ được bản 3')
    const resetAt = new Date('2026-11-02T00:30:00Z').toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(node.textContent).toContain(`đặt lại lúc ${resetAt}`)
    await act(async () => { button(node, 'Vẽ bằng Grok')!.click() })
    expect(state.redraw).toHaveBeenCalledWith('j', {
      imageIds: ['a'],
      provider: 'grok',
      parts: { scene: 'Cảnh 3 đã sửa: guard on the roof.' },
    })
  })
})

describe('đang vẽ', () => {
  it('hiệu ứng vẽ và Huỷ lần vẽ giữ nguyên bản đang dùng', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2), version(3, { state: 'drawing', url: null, finishedAt: null, durationSec: null, error: null })], 2)], quota: undefined }
    state.progress.current = { a: { jobId: 'j', imageId: 'a', n: 3, phase: 'drawing', elapsedSec: 24 } }
    const node = await render()
    expect(node.querySelector('[data-develop="drawing"]')).not.toBeNull()
    expect(node.textContent).toContain('Hoạ sĩ đang vẽ bản 3')
    expect(node.textContent).toContain('0:24 · thường khoảng 40 giây')
    await act(async () => { button(node, 'Huỷ lần vẽ')!.click() })
    expect(state.cancel).toHaveBeenCalledWith('j', 'a', 3)
    expect(node.textContent).toContain('Bản 2 vẫn dùng cho video tới khi bản mới xong.')
  })

  it('selects a completed redraw received without a live progress event', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2), version(3, { state: 'drawing', url: null })], 2)], quota: undefined }
    const node = await render()
    await act(async () => { node.querySelector<HTMLButtonElement>('[aria-label^="Bản 1 ·"]')!.click() })
    state.images.current = { images: [image('a', 3, [version(1), version(2), version(3)], 2)], quota: undefined }
    await act(async () => { root!.render(<ReviewScreen />) })
    expect(node.querySelector('[aria-label^="Bản 3 ·"]')?.getAttribute('aria-pressed')).toBe('true')
    expect(node.querySelector('main img[alt="Cảnh 3"]')?.getAttribute('src')).toBe('/img/v3.jpg')
    expect(node.querySelector('[data-develop="done"]')).not.toBeNull()
    expect(state.redraw).not.toHaveBeenCalled()
  })
})
