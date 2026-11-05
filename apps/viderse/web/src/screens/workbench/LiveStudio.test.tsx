// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { ImageVersion } from '../../api/images'
import type { CloneJobView, JobImageList } from '../../api/workbench'
import { LiveStudio } from './LiveStudio'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const toastSpies = vi.hoisted(() => ({ err: vi.fn(), ok: vi.fn() }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: Object.assign(vi.fn(), toastSpies) }))
const state = vi.hoisted(() => ({
  images: undefined as JobImageList | undefined,
  live: undefined as { data: unknown; error: boolean; loading: boolean; reload: () => Promise<void>; progress: Record<string, unknown> } | undefined,
  refresh: vi.fn(),
  reload: vi.fn(async () => {}),
}))
vi.mock('../../data/useResource', () => ({ useResource: (key: string[]) => ({ data: key[1] === 'images' ? state.images : undefined, refresh: state.refresh, initialLoading: false }) }))
vi.mock('../../data/events', () => ({ subscribeEvents: () => () => {} }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../api/images', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/images')>()
  return { ...actual, useJobImages: () => state.live }
})
const job = { id: 'job', channelId: 'channel', activeStep: 'images', stageGroup: 'working', source: { title: 'Nguồn' }, steps: [] } as unknown as CloneJobView
const manifest = { revision: 7, scenePrompt: { id: 's', name: 'Cảnh', overridden: false }, coverPrompt: { id: 'c', name: 'Bìa', overridden: false } }
const tile = (id: string, url: string, narration: string) => ({ id, kind: 'scene', status: 'review', currentVersion: 1, reviewVersion: 1, versions: [{ version: 1, originalUrl: url }], narration })
const version = (n: number, url: string): ImageVersion => ({
  n, state: 'ready', url, previewUrl: null, width: 1280, height: 720,
  parts: [{ kind: 'scene', text: `Chỉ dẫn bản ${n}`, source: { type: 'artist', label: 'Hoạ sĩ' } }],
  sentText: `Chỉ dẫn bản ${n}`, refs: [], provider: 'chatgpt', model: 'gpt-image-2', params: { size: '1280x720' },
  feedback: null, diffFromPrev: null, startedAt: '2026-01-01T10:00:00Z', finishedAt: '2026-01-01T10:01:00Z',
  durationSec: 44, costImages: 1, error: null,
})
const liveBundle = (ids: string[]) => ({
  images: ids.map((id, index) => ({
    id, kind: 'scene', index, label: `Cảnh ${index + 1}`, usedVersion: 1, refLimit: 4, status: 'review',
    versions: [version(1, `/${id}-1.webp`), version(2, `/${id}-2.webp`)],
  })),
  quota: { providers: [{ provider: 'chatgpt', label: 'ChatGPT', left: 7, total: 10, resetAt: null, paid: true }], left: 7, fallback: null },
})
const setLive = (ids: string[]) => { state.live = { data: liveBundle(ids), error: false, loading: false, reload: state.reload, progress: {} } }
const setLiveVersions = (versions: ImageVersion[], usedVersion = 1) => {
  state.live = {
    data: { images: [{ id: 'scene-one', kind: 'scene', index: 0, label: 'Cảnh 1', usedVersion, refLimit: 4, status: 'review', versions }], quota: liveBundle(['scene-one']).quota },
    error: false, loading: false, reload: state.reload, progress: {},
  }
}

let root: Root, host: HTMLDivElement
beforeEach(() => {
  history.replaceState(null, '', '/')
  state.images = { manifest, items: [tile('scene-one', '/one.webp', 'Cảnh một'), tile('scene-two', '/two.webp', 'Cảnh hai')] }
  setLive(['scene-one', 'scene-two'])
  state.refresh.mockClear(); state.reload.mockClear(); toastSpies.err.mockClear(); toastSpies.ok.mockClear()
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => { await act(async () => root.unmount()); history.replaceState(null, '', '/'); host.remove(); document.body.replaceChildren(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

async function clickAndWaitForRoute(button: HTMLElement) {
  await act(async () => {
    const changed = new Promise<void>(resolve => window.addEventListener('hashchange', () => resolve(), { once: true }))
    button.click()
    await changed
  })
}

it.each(['running', 'pending', 'queued'])('shows the real %s redraw over the retained version instead of a finished tile', async status => {
  state.images!.items![0].status = status
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  expect(host.querySelector('[role="status"]')?.textContent).toContain(status === 'running' ? 'Hoạ sĩ đang vẽ…' : 'Chờ lượt vẽ')
  expect(host.querySelector('img')?.getAttribute('src')).toBe('/one.webp')
  expect(host.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('50')
  expect(host.querySelector('[aria-label="Vẽ lại cảnh 1"]')).toBeNull()
})

it('opens the shared viewer from a tile and sends feedback for exactly that image', async () => {
  const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'scene-one', n: 2 }] }))
  vi.stubGlobal('fetch', fetcher)
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog.textContent).toContain('Cảnh 1')
  expect(dialog.querySelector('textarea')).toBeNull()
  const openFeedback = [...dialog.querySelectorAll('button')].find(button => button.textContent?.includes('Góp ý'))!
  await act(async () => openFeedback.click())
  expect(dialog.textContent).toContain('Vẽ lại · tốn 1 ảnh (còn 7)')
  const textarea = dialog.querySelector('textarea')!
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    setter.call(textarea, 'ánh sáng lạnh hơn')
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
  })
  const send = [...dialog.querySelectorAll('button')].find(button => button.textContent?.startsWith('Vẽ lại · tốn 1 ảnh'))!
  await act(async () => send.click())
  await act(async () => { for (let i = 0; i < 6; i += 1) await Promise.resolve() })
  expect(fetcher).toHaveBeenCalledOnce()
  const [url, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
  expect(url).toBe('/api/clone/job/images/redraw')
  expect(JSON.parse(String(init.body))).toEqual({ imageIds: ['scene-one'], baseVersion: 2, feedback: 'ánh sáng lạnh hơn' })
  expect(state.refresh).toHaveBeenCalled()
  expect(state.reload).toHaveBeenCalled()
  expect(document.body.textContent).toContain('Đã gửi Hoạ sĩ · bản 2')
})

it('uses the version explicitly selected in the viewer as the feedback source', async () => {
  const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'scene-one', n: 3 }] }))
  vi.stubGlobal('fetch', fetcher)
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  const dialog = document.querySelector('[role="dialog"]')!
  await act(async () => dialog.querySelector<HTMLButtonElement>('[aria-label^="Bản 1 ·"]')!.click())
  const openFeedback = [...dialog.querySelectorAll('button')].find(button => button.textContent?.includes('Góp ý'))!
  await act(async () => openFeedback.click())
  const textarea = dialog.querySelector('textarea')!
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    setter.call(textarea, 'giữ đúng khung bản 1')
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
  })
  const send = [...dialog.querySelectorAll('button')].find(button => button.textContent?.startsWith('Vẽ lại · tốn 1 ảnh'))!
  await act(async () => send.click())
  await act(async () => { for (let i = 0; i < 6; i += 1) await Promise.resolve() })
  expect(JSON.parse(String((fetcher.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({
    imageIds: ['scene-one'], baseVersion: 1, feedback: 'giữ đúng khung bản 1',
  })
})

it('skips a cancelled draw in history and redraws from the version still in use', async () => {
  const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'scene-one', n: 3 }] }))
  vi.stubGlobal('fetch', fetcher)
  const drawingVersion: ImageVersion = { ...version(2, '/one-2.webp'), state: 'drawing', url: null, finishedAt: null, durationSec: null }
  setLiveVersions([version(1, '/one-1.webp')])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  const dialog = document.querySelector('[role="dialog"]')!
  setLiveVersions([version(1, '/one-1.webp'), drawingVersion])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  // Chốt lựa chọn đang vẽ, rồi lần vẽ đó bị huỷ: lịch sử còn nhưng không phải nguồn.
  await act(async () => dialog.querySelector<HTMLButtonElement>('[aria-label^="Bản 2 ·"]')!.click())
  const cancelledVersion: ImageVersion = { ...version(2, '/one-2.webp'), state: 'cancelled', url: null }
  setLiveVersions([version(1, '/one-1.webp'), cancelledVersion])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  const cancelled = dialog.querySelector<HTMLButtonElement>('[aria-label^="Bản 2 · đã huỷ"]')!
  expect(cancelled.disabled).toBe(true)
  expect(dialog.querySelector('[aria-label^="Bản 1 ·"]')?.getAttribute('aria-pressed')).toBe('true')
  await act(async () => cancelled.click())
  expect(dialog.querySelector('[aria-label^="Bản 1 ·"]')?.getAttribute('aria-pressed')).toBe('true')
  const openFeedback = [...dialog.querySelectorAll('button')].find(button => button.textContent?.includes('Góp ý'))!
  await act(async () => openFeedback.click())
  const textarea = dialog.querySelector('textarea')!
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    setter.call(textarea, 'giữ bản đang dùng')
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
  })
  const send = [...dialog.querySelectorAll('button')].find(button => button.textContent?.startsWith('Vẽ lại · tốn 1 ảnh'))!
  await act(async () => send.click())
  await act(async () => { for (let i = 0; i < 6; i += 1) await Promise.resolve() })
  expect(fetcher).toHaveBeenCalledOnce()
  expect(JSON.parse(String((fetcher.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({
    imageIds: ['scene-one'], baseVersion: 1, feedback: 'giữ bản đang dùng',
  })
})

it('does not use a cancelled-only history as the redraw base', async () => {
  const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'scene-one', n: 2 }] }))
  vi.stubGlobal('fetch', fetcher)
  const cancelledVersion: ImageVersion = { ...version(1, '/one-1.webp'), state: 'cancelled', url: null }
  setLiveVersions([cancelledVersion])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog.querySelector<HTMLButtonElement>('[aria-label^="Bản 1 · đã huỷ"]')!.disabled).toBe(true)
  const openFeedback = [...dialog.querySelectorAll('button')].find(button => button.textContent?.includes('Góp ý'))!
  await act(async () => openFeedback.click())
  const textarea = dialog.querySelector('textarea')!
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    setter.call(textarea, 'vẽ lại từ đầu')
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
  })
  const send = [...dialog.querySelectorAll('button')].find(button => button.textContent?.startsWith('Vẽ lại · tốn 1 ảnh'))!
  await act(async () => send.click())
  await act(async () => { for (let i = 0; i < 6; i += 1) await Promise.resolve() })
  expect(fetcher).toHaveBeenCalledOnce()
  expect(JSON.parse(String((fetcher.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({
    imageIds: ['scene-one'], feedback: 'vẽ lại từ đầu',
  })
})

it('reports the displayed failed version and redraws from it instead of an older failure', async () => {
  const fetcher = vi.fn(async () => Response.json({ versions: [{ imageId: 'scene-one', n: 4 }] }))
  vi.stubGlobal('fetch', fetcher)
  const failedVersion: ImageVersion = {
    ...version(2, '/one-2.webp'), state: 'failed', url: null,
    error: { reason: 'provider', message: 'Chưa vẽ được' },
  }
  setLiveVersions([version(1, '/one-1.webp'), failedVersion, version(3, '/one-3.webp')])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog.querySelector('[aria-label^="Bản 3 ·"]')?.getAttribute('aria-pressed')).toBe('true')
  expect(dialog.textContent).not.toContain('Bản 2 chưa vẽ được')
  await act(async () => dialog.querySelector<HTMLButtonElement>('[aria-label^="Bản 2 ·"]')!.click())
  expect(dialog.textContent).toContain('Bản 2 chưa vẽ được')
  const openFeedback = [...dialog.querySelectorAll('button')].find(button => button.textContent?.includes('Góp ý'))!
  await act(async () => openFeedback.click())
  const textarea = dialog.querySelector('textarea')!
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!
    setter.call(textarea, 'vẽ tiếp từ bản lỗi')
    textarea.dispatchEvent(new Event('input', { bubbles: true }))
  })
  const send = [...dialog.querySelectorAll('button')].find(button => button.textContent?.startsWith('Vẽ lại · tốn 1 ảnh'))!
  await act(async () => send.click())
  await act(async () => { for (let i = 0; i < 6; i += 1) await Promise.resolve() })
  expect(fetcher).toHaveBeenCalledOnce()
  expect(JSON.parse(String((fetcher.mock.calls[0] as unknown as [string, RequestInit])[1].body))).toEqual({
    imageIds: ['scene-one'], baseVersion: 2, feedback: 'vẽ tiếp từ bản lỗi',
  })
})

it('closes the viewer and keeps a valid selection when the viewed image disappears', async () => {
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLButtonElement>('[aria-label="Vẽ lại cảnh 2"]')!)
  expect(document.querySelector('[role="dialog"]')).not.toBeNull()
  expect(document.querySelector('[role="dialog"] textarea')).not.toBeNull()
  state.images = { manifest, items: [tile('scene-one', '/one.webp', 'Cảnh một')] }
  setLive(['scene-one'])
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await act(async () => {})
  expect(document.querySelector('[role="dialog"]')).toBeNull()
  expect(toastSpies.err).toHaveBeenCalledWith('Ảnh đã chọn không còn trong bộ ảnh. Đã chọn ảnh còn lại.')
  expect(host.querySelectorAll('[role="option"]')[0]?.getAttribute('aria-selected')).toBe('true')
})

it('opens Duyệt at the exact image id for a review-stage tile', async () => {
  const reviewJob = { ...job, stageGroup: 'review' } as unknown as CloneJobView
  await act(async () => root.render(<LiveStudio job={reviewJob} onRefresh={() => {}} />))
  await act(async () => { host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 2"]')!.click() })
  const params = new URLSearchParams(location.hash.split('?')[1] ?? '')
  expect(params.get('video')).toBe('job')
  expect(params.get('img')).toBe('scene-two')
})

it('giữ lựa chọn nền khi trình xem đang mở', async () => {
  await act(async () => root.render(<LiveStudio job={job} onRefresh={() => {}} />))
  await clickAndWaitForRoute(host.querySelector<HTMLElement>('[role="option"][aria-label="Cảnh 1"]')!)
  expect(document.querySelector('[role="dialog"]')).not.toBeNull()
  await act(async () => {
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '2', bubbles: true }))
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
  })
  await act(async () => {})
  expect(document.querySelector('[role="dialog"]')).toBeNull()
  expect(host.querySelector('[role="option"][aria-label="Cảnh 1"]')?.getAttribute('aria-selected')).toBe('true')
})

it('phát bản xuất thật của editor khi job đã xong mà không có artifact video', async () => {
  const doneJob = {
    ...job, stageGroup: 'ready', status: 'succeeded', exportBytes: 630653,
    source: { ...job.source, durationSec: 6 },
    steps: [{
      step: 'export', status: 'succeeded',
      output: { artifacts: [], data: { projectId: 'clone-3225b629-g1', exportId: 'b6e47266', fileName: 'video.mp4', size: 630653, durationMs: 28031, accepted: true } },
    }],
  } as unknown as CloneJobView
  await act(async () => root.render(<LiveStudio job={doneJob} onRefresh={() => {}} />))
  expect(host.querySelector('video')?.getAttribute('src')).toBe('/api/editor/projects/clone-3225b629-g1/exports/b6e47266')
  expect(host.textContent).toContain('MP4 0:28')
  expect(host.textContent).not.toContain('File video không còn trên máy.')
})

it('không phát lại bản xuất đã xoá dù bước xuất còn dữ liệu file', async () => {
  const removedJob = {
    ...job, stageGroup: 'posted', status: 'succeeded', exportBytes: null,
    steps: [{
      step: 'export', status: 'succeeded',
      output: { artifacts: [], data: { projectId: 'p', exportId: 'e', removedAt: '2026-11-01T00:00:00Z', accepted: true } },
    }],
  } as unknown as CloneJobView
  await act(async () => root.render(<LiveStudio job={removedJob} onRefresh={() => {}} />))
  expect(host.querySelector('video')).toBeNull()
  expect(host.textContent).toContain('File video không còn trên máy.')
})

