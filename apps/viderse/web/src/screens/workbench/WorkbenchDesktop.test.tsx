// Bàn dựng máy tính: mở "Vẽ lại tất cả" rồi đổi video thì lần bấm sau vẫn mở
// được hộp (trạng thái mở không kẹt), và video đã đăng còn file MP4 thì có nút
// xoá riêng file qua đúng luồng xoá MP4 đã có.
// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CloneJobView, JobImageList, JobStep } from '../../api/workbench'
import type { ImageVersion } from '../../api/images'
import type { Route } from '../../router'
import { WorkbenchDesktop } from './WorkbenchDesktop'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({
  jobs: [] as CloneJobView[],
  images: { manifest: null, items: null } as JobImageList,
  refresh: vi.fn(),
  confirm: vi.fn(),
  markPosted: vi.fn(),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: state.confirm }))

vi.mock('../../data/useResource', () => {
  const snap = (data: unknown) => ({ data, error: null, initialLoading: false, placeholder: false, refresh: state.refresh })
  return {
    useResource: (key: unknown[]) => {
      const [group, name, id] = key as string[]
      if (group === 'workbench' && name === 'list') return snap({ jobs: state.jobs, defaultJobId: state.jobs[0]?.id, queuedTotal: 0 })
      if (group === 'workbench' && name === 'job') return snap(state.jobs.find((job) => job.id === id))
      if (group === 'workbench' && name === 'images') return snap(state.images)
      if (group === 'channels' && name === 'summary') return snap([])
      return snap(undefined)
    },
  }
})
vi.mock('../../data/events', () => ({ subscribeEvents: () => () => {}, events: { onReconnect: () => () => {} } }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../router', async (importOriginal) => ({ ...(await importOriginal<typeof import('../../router')>()), navigate: vi.fn() }))
vi.mock('../../components/performance', async (importOriginal) => ({ ...(await importOriginal<object>()), requestMarkPosted: state.markPosted }))
vi.mock('../../components', () => ({
  ChannelAvatar: () => null,
  EmptyWorkbench: () => null,
  Orb: () => null,
  DevelopEffect: () => null,
  SystemLoading: () => null,
  SystemNotFound: () => null,
  ASSISTANT_INK: { scout: 'ink', director: 'ink', artist: 'ink', qa: 'ink' },
  assistantName: (agent: string) => agent,
}))
vi.mock('../../components/suggestions/SuggestionPanels', () => ({ SuggestionToggles: () => null, SuggestionDock: () => null }))
vi.mock('../../components/suggestions/useSuggestions', () => ({ useSuggestionList: () => ({ data: { total: 0 } }) }))
vi.mock('../../components/assistant/AssistantComposer', () => ({ AssistantComposer: () => null }))
vi.mock('./OverflowMenu', () => ({ OverflowMenu: () => null }))
vi.mock('./PromptMenu', () => ({ PromptMenu: () => null }))
vi.mock('./recap/Recap', () => ({ Recap: () => null }))
vi.mock('./panels/WorkbenchPanels', () => ({ WorkbenchPanels: () => null }))

const jobView = (id: string, stageGroup: string, extra: Partial<CloneJobView> = {}): CloneJobView => ({
  id,
  channelId: 'c1',
  source: { id: `s-${id}`, url: '', title: `Video ${id}`, channelId: 'c1', channelTitle: 'Kênh', thumbnailUrl: null, durationSec: 600 },
  sourceKey: `k-${id}`,
  generation: 1,
  templateId: 't1',
  templateName: 'Mẫu chuẩn',
  status: 'running',
  stageGroup,
  activeStep: 'images',
  version: 1,
  noImages: false,
  subline: '',
  agentView: [],
  steps: [],
  ...extra,
})
const tile = (id: string) => ({ id, kind: 'scene', status: 'review', currentVersion: 1, reviewVersion: 1, versions: [{ version: 1, originalUrl: `/${id}.webp` }], narration: id })
const imageVersion = (n: number, state: ImageVersion['state'], scene: string): ImageVersion => ({
  n, state, url: `/api/clone/a/images/scene-one/versions/${n}`, previewUrl: null, width: 1280, height: 720,
  parts: [{ kind: 'scene', text: scene, source: { type: 'artist', label: 'Hoạ sĩ' } }],
  sentText: scene, refs: [], provider: 'chatgpt', model: 'gpt-image-2', params: { size: '1280x720' },
  feedback: null, diffFromPrev: null, startedAt: '2026-11-02T10:00:00Z', finishedAt: '2026-11-02T10:01:00Z',
  durationSec: 44, costImages: 1, error: null,
})

let root: Root | undefined
let host: HTMLDivElement
const render = async (route: Extract<Route, { name: 'workbench' }>) => {
  if (!root) {
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)
  }
  await act(async () => root!.render(<WorkbenchDesktop route={route} />))
}
const button = (text: string) => [...host.querySelectorAll('button')].find((entry) => entry.textContent?.includes(text))
const dialog = () => document.querySelector('[role="dialog"]')
const fetcher = vi.fn()
const lastPost = () => {
  const [url, init] = fetcher.mock.calls.at(-1) as unknown as [string, RequestInit]
  return { url, body: JSON.parse(String(init.body)) as Record<string, unknown> }
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (query: string) => ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {} }))
  state.jobs = [jobView('a', 'review'), jobView('b', 'review')]
  state.images = {
    manifest: { revision: 7, scenePrompt: { id: 's', name: 'Cảnh', overridden: false }, coverPrompt: { id: 'c', name: 'Bìa', overridden: false } },
    items: [tile('scene-one'), tile('scene-two')],
  }
  state.confirm.mockReset().mockImplementation(async (options: { onConfirm: (value: string) => Promise<void> }) => { await options.onConfirm(''); return true })
  fetcher.mockReset().mockImplementation(async () => Response.json({ results: [{ ok: true }] }))
  vi.stubGlobal('fetch', fetcher)
})
afterEach(async () => {
  if (root) await act(async () => root!.unmount())
  root = undefined
  vi.unstubAllGlobals()
  document.body.replaceChildren()
})

describe('hộp Vẽ lại tất cả sau khi đổi video', () => {
  it('đổi video lúc hộp đang mở thì lần bấm sau vẫn mở lại được', async () => {
    await render({ name: 'workbench', video: 'a' })
    await act(async () => button('Vẽ lại tất cả')!.click())
    expect(dialog()?.textContent).toContain('Vẽ lại cả 2 ảnh?')
    await render({ name: 'workbench', video: 'b' })
    expect(dialog()).toBeNull()
    await act(async () => button('Vẽ lại tất cả')!.click())
    expect(dialog()?.textContent).toContain('Vẽ lại cả 2 ảnh?')
  })
})

describe('xoá riêng file MP4 của video đã đăng', () => {
  it('video đã đăng còn file xuất thì bấm Xoá MP4 xoá đúng file MP4, không xoá cả video', async () => {
    state.jobs = [jobView('p', 'posted', { status: 'posted', exportBytes: 2097152 })]
    await render({ name: 'workbench', video: 'p' })
    const remove = button('Xoá MP4')
    expect(remove).toBeTruthy()
    await act(async () => remove!.click())
    expect(state.confirm).toHaveBeenCalledOnce()
    expect(fetcher).toHaveBeenCalledOnce()
    expect(lastPost()).toEqual({ url: '/api/clone/bulk', body: { action: 'deleteExport', jobIds: ['p'] } })
  })

  it('video đã đăng mà không còn file xuất thì không mời xoá', async () => {
    state.jobs = [jobView('p', 'posted', { status: 'posted', exportBytes: null })]
    await render({ name: 'workbench', video: 'p' })
    expect(button('Xoá MP4')).toBeUndefined()
  })
})

describe('băng Cần xử lý nói đúng lý do của video đang chọn', () => {
  const banner = () => host.querySelector('main header [role="alert"]')?.textContent ?? ''
  const stopped = (code: string, message: string): JobStep => ({
    step: 'images', status: 'failed', progress: { unitsDone: 0, unitsTotal: null, phase: '' }, output: null, error: { code, message }, startedAt: null, finishedAt: null,
  })

  it('hiện nguyên nhân thật của chính video đang chọn và đổi theo video, không lộ lỗi thô', async () => {
    state.jobs = [
      jobView('a', 'attention', { status: 'failed', subline: 'Hoạ sĩ · ChatGPT hết hạn đăng nhập.', steps: [stopped('provider_auth', 'ChatGPT hết hạn đăng nhập. Đăng nhập lại rồi chạy tiếp.')] }),
      jobView('b', 'attention', { status: 'failed', subline: 'Hoạ sĩ · invalid api key', steps: [stopped('auth_failed', 'invalid api key')] }),
    ]
    await render({ name: 'workbench', video: 'a' })
    expect(banner()).toContain('hết hạn')
    expect(banner()).not.toContain('khoá')

    await render({ name: 'workbench', video: 'b' })
    expect(banner()).toContain('khoá')
    expect(banner()).not.toContain('hết hạn')
    expect(banner()).not.toContain('invalid api key')
  })
})

describe('vẽ tiếp bằng Grok từ bản lỗi', () => {
  it('retries the latest failed version rather than an older failed prompt', async () => {
    const stoppedImages: JobStep = {
      step: 'images', status: 'failed', progress: { unitsDone: 0, unitsTotal: null, phase: '' }, output: null,
      error: { code: 'provider', message: 'ChatGPT lỗi' }, startedAt: null, finishedAt: null,
    }
    state.jobs = [jobView('a', 'attention', { status: 'failed', steps: [stoppedImages] })]
    state.images = {
      manifest: { revision: 7, scenePrompt: { id: 's', name: 'Cảnh', overridden: false }, coverPrompt: { id: 'c', name: 'Bìa', overridden: false } },
      items: [{ ...tile('scene-one'), status: 'failed' }],
    }
    fetcher.mockImplementation(async (input: RequestInfo | URL) => {
      if (String(input).endsWith('/redraw')) return Response.json({ versions: [{ imageId: 'scene-one', n: 4 }] })
      return Response.json({ images: [{
        id: 'scene-one', kind: 'scene', index: 0, label: 'Cảnh 1', usedVersion: 1, refLimit: 4,
        versions: [imageVersion(1, 'ready', 'Cảnh 1 gốc'), imageVersion(2, 'failed', 'Cảnh 1 lỗi cũ'), imageVersion(3, 'failed', 'Cảnh 1 lỗi mới')],
      }] })
    })
    await render({ name: 'workbench', video: 'a' })
    await act(async () => button('Vẽ tiếp bằng Grok')!.click())
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(lastPost()).toEqual({
      url: '/api/clone/a/images/redraw',
      body: { imageIds: ['scene-one'], provider: 'grok', baseVersion: 3 },
    })
  })
})
