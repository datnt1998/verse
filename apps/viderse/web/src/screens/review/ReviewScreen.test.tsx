// Duyệt ảnh: khung không phóng quá bề rộng gốc, sửa prompt gửi đúng phần đã
// đổi + ảnh tham chiếu, Esc huỷ, ⌘↵ gửi, chọn nhiều gửi góp ý chung, thẻ lỗi
// có giờ đặt lại thật và vẽ bằng Grok đúng prompt đã sửa (§26, artboard Review).
import { act } from 'react'
import { useImageVersion, type ImageUseResult } from '../../api/images'
import { applyViderseLabels } from '../../kitLabels'
import { dismissConfirmations } from '@noverse/ui'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ReviewScreen } from './ReviewScreen'
import type * as ReviewModule from './useReview'
import { useRoute } from '../../router'

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
  legacyStatus: 'review',
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
        { id: 'a', kind: 'scene', status: state.legacyStatus, prompt: 'p', versions: [], sceneId: '3', narration: 'Lời kể cảnh 3.' },
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

beforeEach(() => { applyViderseLabels(); state.legacyStatus = 'review' })
let root: Root | undefined
afterEach(async () => {
  await act(async () => { dismissConfirmations(); root?.unmount(); await Promise.resolve() })
  root = undefined
  document.body.innerHTML = ''
  history.replaceState(null, '', '/')
  vi.clearAllMocks()
  state.progress.current = {}
})
function ReviewRouteHost() {
  const { route } = useRoute()
  return <ReviewScreen route={route.name === 'review' ? route : { name: 'review' }} />
}

const render = async () => {
  const node = document.createElement('div')
  document.body.append(node)
  root = createRoot(node)
  await act(async () => { root!.render(<ReviewRouteHost />) })
  return node
}
const button = (node: HTMLElement, text: string) => [...node.querySelectorAll('button')].find(entry => entry.textContent?.includes(text))
const type = async (field: HTMLInputElement | HTMLTextAreaElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype, 'value')!.set!
  await act(async () => { setter.call(field, value); field.dispatchEvent(new Event('input', { bubbles: true })) })
}
async function answerRedraw(accept = true) {
  await vi.waitFor(async () => {
    await act(async () => { await vi.dynamicImportSettled() })
    expect(document.querySelector('[role="alertdialog"]')).not.toBeNull()
  })
  const dialog = document.querySelector<HTMLElement>('[role="alertdialog"]')!
  expect(state.redraw).not.toHaveBeenCalled()
  await act(async () => {
    const removed = new Promise<void>(resolve => {
      const observer = new MutationObserver(() => { if (!dialog.isConnected) { observer.disconnect(); resolve() } })
      observer.observe(document.body, { childList: true, subtree: true })
    })
    button(dialog, accept ? 'Vẽ lại' : 'Huỷ')!.click()
    await removed
  })
}
const key = async (init: KeyboardEventInit) => { await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, ...init })) }) }

it('keeps the addressed image selected across reordered polling data and browser navigation', async () => {
  history.replaceState(null, '', '/#/review?video=j&img=b')
  state.images.current = { images: [image('a', 3, [version(1)]), image('b', 4, [version(1)])], quota: undefined }
  const node = await render()
  expect(node.querySelector('[role="option"][aria-selected="true"]')?.getAttribute('aria-label')).toBe('Cảnh 4')
  state.images.current = { images: [image('b', 1, [version(1)]), image('a', 3, [version(1)])], quota: undefined }
  await act(async () => root!.render(<ReviewRouteHost />))
  expect(node.querySelector('[role="option"][aria-selected="true"]')?.getAttribute('aria-label')).toBe('Cảnh 1')
  await type(node.querySelector<HTMLInputElement>('#redraw')!, 'cold moonlight')
  await act(async () => [...node.querySelectorAll<HTMLButtonElement>('button')].find(entry => entry.textContent?.trim().startsWith('Vẽ lại') && !entry.textContent.includes('cả'))!.click())
  await answerRedraw()
  expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['b'], feedback: 'cold moonlight' })
  await act(async () => {
    history.replaceState(null, '', '/#/review?video=j&img=a')
    dispatchEvent(new HashChangeEvent('hashchange'))
  })

  expect(node.querySelector('[role="option"][aria-selected="true"]')?.getAttribute('aria-label')).toBe('Cảnh 3')
})
it('does not apply an old version response to the image selected while the request was pending', async () => {
  state.images.current = { images: [
    { ...image('a', 3, [version(1), version(2)]), usedVersion: 1 },
    { ...image('b', 4, [version(1), version(2)]), usedVersion: 1 },
  ], quota: undefined }
  let finish!: (result: ImageUseResult) => void
  const request = new Promise<ImageUseResult>(resolve => { finish = resolve })
  vi.mocked(useImageVersion).mockReturnValueOnce(request)
  const node = await render()
  await act(async () => node.querySelector<HTMLButtonElement>('[aria-label^="Bản 2"]')!.click())
  await act(async () => button(node, 'Dùng bản 2 cho video')!.click())
  await act(async () => { node.querySelector<HTMLButtonElement>('[role="option"][aria-label="Cảnh 4"]')!.click(); await new Promise(done => setTimeout(done, 0)) })
  await vi.waitFor(async () => {
    await act(async () => { await Promise.resolve() })
    expect(node.querySelector('[role="option"][aria-selected="true"]')?.getAttribute('aria-label')).toBe('Cảnh 4')
  })
  await act(async () => node.querySelector<HTMLButtonElement>('[aria-label^="Bản 1"]')!.click())
  expect(node.querySelector('[aria-label^="Bản 1"]')?.getAttribute('aria-pressed')).toBe('true')
  await act(async () => { finish({ imageId: 'a', n: 2, usedVersion: 2 }); await request })
  expect(node.querySelector('[aria-label^="Bản 1"]')?.getAttribute('aria-pressed')).toBe('true')
  expect(node.querySelector('[aria-label^="Bản 2"]')?.getAttribute('aria-pressed')).toBe('false')
})


describe('chỉ dẫn đã gửi', () => {
  it('hiện theo phần kèm nguồn và dòng meta của bản', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    expect(node.textContent).toContain('ChatGPT · gpt-image-2 · 1280×720 · vẽ 44 giây')
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
    await answerRedraw()
    expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['a'], parts: { scene: 'A guard on the roof at dawn.' }, refs: [] })
  })

  it('keeps edited instructions and removed references when the quota confirmation is cancelled', async () => {
    state.images.current = { images: [image('a', 3, [version(1), version(2)])], quota: undefined }
    const node = await render()
    await act(async () => button(node, 'Sửa chỉ dẫn')!.click())
    await type(node.querySelector<HTMLTextAreaElement>('#prompt-scene')!, 'A guard on the roof at dawn.')
    await act(async () => node.querySelector<HTMLButtonElement>('[aria-label="Bỏ ảnh tham chiếu 1"]')!.click())
    await act(async () => button(node, 'Vẽ lại với chỉ dẫn này')!.click())
    await answerRedraw(false)
    expect(node.querySelector<HTMLTextAreaElement>('#prompt-scene')?.value).toBe('A guard on the roof at dawn.')
    expect(node.querySelector('[aria-label="Bỏ ảnh tham chiếu 1"]')).toBeNull()
    expect(state.redraw).not.toHaveBeenCalled()
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
    await answerRedraw()
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
    await answerRedraw()
    expect(state.redraw).toHaveBeenCalledWith('j', { imageIds: ['a', 'b'], feedback: 'không có chữ trong ảnh' })
  })

  it('keeps the selected images and shared feedback when the quota confirmation is cancelled', async () => {
    state.images.current = { images: [image('a', 3, [version(1)]), image('b', 4, [version(1)])], quota: undefined }
    const node = await render()
    await act(async () => {
      for (const option of node.querySelectorAll<HTMLButtonElement>('[role="option"]')) option.dispatchEvent(new MouseEvent('click', { bubbles: true, shiftKey: true }))
    })
    await type(node.querySelector<HTMLInputElement>('#redraw-batch')!, 'No lettering anywhere')
    await act(async () => button(node, 'Vẽ lại 2 ảnh')!.click())
    await answerRedraw(false)
    expect(node.querySelector<HTMLInputElement>('#redraw-batch')?.value).toBe('No lettering anywhere')
    expect(node.querySelectorAll('[role="checkbox"][aria-checked="true"]')).toHaveLength(2)
    expect(state.redraw).not.toHaveBeenCalled()
  })
})

describe('thẻ lỗi', () => {
  it('giờ đặt lại thật và Vẽ bằng Grok đúng prompt đã sửa của bản lỗi', async () => {
    state.legacyStatus = 'failed'
    const failed = version(3, {
      state: 'failed', url: null, finishedAt: null, durationSec: null,
      error: { reason: 'quota', retryAt: '2026-11-02T00:30:00Z', message: 'ChatGPT đã hết hạn mức ảnh hôm nay (50/50)' },
      parts: [part('scene', 'Cảnh 3 đã sửa: guard on the roof.', 'Hoạ sĩ sửa theo góp ý')],
      refs: [],
    })
    state.images.current = { images: [image('a', 3, [version(1), version(2), failed], 2)], quota: { providers: [{ provider: 'chatgpt', label: 'ChatGPT', left: 0, total: 50, resetAt: '2026-11-02T00:30:00Z', paid: true }], left: 0, fallback: { provider: 'grok', label: 'Grok', left: 190 } } }
    const node = await render()
    expect(node.querySelector('main [role="alert"]')?.textContent).toContain('Chưa vẽ được bản 3')
    expect(node.textContent).not.toContain('Bộ lọc nội dung đã chặn ảnh này')
    const resetAt = new Date('2026-11-02T00:30:00Z').toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(node.textContent).toContain(`đặt lại lúc ${resetAt}`)
    await act(async () => { button(node, 'Vẽ bằng Grok')!.click() })
    await answerRedraw()
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
    await act(async () => { root!.render(<ReviewRouteHost />) })
    expect(node.querySelector('[aria-label^="Bản 3 ·"]')?.getAttribute('aria-pressed')).toBe('true')
    expect(node.querySelector('main img[alt="Cảnh 3"]')?.getAttribute('src')).toBe('/img/v3.jpg')
    expect(node.querySelector('[data-develop="done"]')).not.toBeNull()
    expect(state.redraw).not.toHaveBeenCalled()
  })
})

it('keeps legacy prompt parts read-only through the bottom action and edit shortcut', async () => {
  state.images.current = { images: [image('a', 3, [version(1, { partsLegacy: true })])], quota: undefined }
  const node = await render()
  expect(button(node, 'Sửa chỉ dẫn')?.disabled).toBe(true)
  await act(async () => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'e', bubbles: true })))
  expect(node.querySelector('#prompt-scene')).toBeNull()
})

it('preserves a draft when inspecting raw prompt and returning to editable parts', async () => {
  state.images.current = { images: [image('a', 3, [version(1)])], quota: undefined }
  const node = await render()
  await act(async () => button(node, 'Sửa chỉ dẫn')!.click())
  await type(node.querySelector<HTMLTextAreaElement>('#prompt-scene')!, 'Keep the same character with a red umbrella')
  await act(async () => button(node, 'Xem nguyên văn gửi máy chủ')!.click())
  await act(async () => button(node, 'Xem theo phần')!.click())
  expect(node.querySelector<HTMLTextAreaElement>('#prompt-scene')?.value).toBe('Keep the same character with a red umbrella')
  expect(button(node, 'Huỷ sửa')).toBeDefined()
})

it('makes an added reference immediately visible as an editable draft and cancels it without a draw', async () => {
  state.images.current = { images: [image('a', 3, [version(1)]), image('b', 4, [version(1)])], quota: undefined }
  const node = await render()
  await act(async () => button(node, 'Thêm')!.click())
  const picker = node.querySelector<HTMLElement>('[role="dialog"][aria-label="Thêm ảnh tham chiếu"]')!
  await act(async () => button(picker, 'Cảnh 4')!.click())
  await act(async () => button(picker, 'Thêm 1 ảnh')!.click())
  expect(node.querySelector('[role="dialog"][aria-label="Thêm ảnh tham chiếu"]')).toBeNull()
  expect(node.querySelector('button[aria-label="Bỏ ảnh tham chiếu 1"]')).not.toBeNull()
  expect(button(node, 'Huỷ sửa')).toBeDefined()
  await act(async () => button(node, 'Huỷ sửa')!.click())
  expect(node.querySelector('button[aria-label="Bỏ ảnh tham chiếu 1"]')).toBeNull()
  expect(state.redraw).not.toHaveBeenCalled()
})

it('returns reference-picker focus on cancellation without stealing focus from an outside field', async () => {
  state.images.current = { images: [image('a', 3, [version(1)])], quota: undefined }
  const node = await render()
  const opener = node.querySelector<HTMLButtonElement>('button[aria-label="Thêm"]')!
  await act(async () => opener.click())
  const picker = node.querySelector<HTMLElement>('[role="dialog"][aria-label="Thêm ảnh tham chiếu"]')!
  expect(picker.contains(document.activeElement)).toBe(true)
  await act(async () => button(picker, 'Huỷ')!.click())
  expect(document.activeElement).toBe(opener)
  await act(async () => opener.click())
  const outside = node.querySelector<HTMLInputElement>('#redraw')!
  outside.focus()
  await act(async () => outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })))
  expect(node.querySelector('[role="dialog"][aria-label="Thêm ảnh tham chiếu"]')).toBeNull()
  expect(document.activeElement).toBe(outside)
  expect(state.redraw).not.toHaveBeenCalled()
})
