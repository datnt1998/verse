// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { CloneJobView, JobImageList } from '../../api/workbench'
import { LiveStudio } from './LiveStudio'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const state = vi.hoisted(() => ({ images: undefined as JobImageList | undefined, refresh: vi.fn() }))
vi.mock('../../data/useResource', () => ({ useResource: (key: string[]) => ({ data: key[1] === 'images' ? state.images : undefined, refresh: state.refresh, initialLoading: false }) }))
vi.mock('../../data/events', () => ({ subscribeEvents: () => () => {} }))
const job = { id: 'job', channelId: 'channel', activeStep: 'images', stageGroup: 'working', source: { title: 'Nguồn' }, steps: [] } as unknown as CloneJobView
let root: Root, host: HTMLDivElement
beforeEach(() => {
  state.images = { manifest: { revision: 7, scenePrompt: { id: 's', name: 'Cảnh', overridden: false }, coverPrompt: { id: 'c', name: 'Bìa', overridden: false } }, items: [{ id: 'scene-one', kind: 'scene', status: 'review', currentVersion: 1, reviewVersion: 2, versions: [{ version: 1, originalUrl: '/old.webp' }, { version: 2, originalUrl: '/review.webp' }], narration: 'Lời kể của cảnh' }] }
  state.refresh.mockClear()
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => { await act(async () => root.unmount()); host.remove(); vi.unstubAllGlobals() })
it.each(['running', 'pending', 'queued'])('shows the real %s redraw over the retained version instead of a finished tile', async status => {
  state.images!.items![0].status = status
  await act(async () => root.render(<LiveStudio job={job} onChangePrompt={() => {}} onRefresh={() => {}} />))
  expect(host.querySelector('[role="status"]')?.textContent).toContain(status === 'running' ? 'Hoạ sĩ đang vẽ…' : 'Chờ lượt vẽ')
  expect(host.querySelector('img')?.getAttribute('src')).toBe('/review.webp')
  expect(host.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('0')
  expect(host.querySelector('[aria-label="Vẽ lại cảnh 1"]')).toBeNull()
})
it('confirms a single tile redraw with its real manifest revision without redrawing other tiles', async () => {
  const fetcher = vi.fn(async () => Response.json({ revision: 8 }))
  vi.stubGlobal('fetch', fetcher)
  await act(async () => root.render(<LiveStudio job={job} onChangePrompt={() => {}} onRefresh={() => {}} />))
  await act(async () => host.querySelector<HTMLButtonElement>('[aria-label="Vẽ lại cảnh 1"]')!.click())
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog.textContent).toContain('Dùng 1 ảnh trong hạn mức hôm nay')
  const submit = [...dialog.querySelectorAll('button')].find(button => button.textContent === 'Vẽ lại cảnh 1')!
  await act(async () => submit.click())
  expect(fetcher).toHaveBeenCalledTimes(1)
  const [url, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
  expect(url).toBe('/api/clone/job/images/scene-one')
  expect(JSON.parse(String(init.body))).toEqual({ op: 'regenerate', expectedRevision: 7 })
  expect(state.refresh).toHaveBeenCalledOnce()
})

const manifest = { revision: 7, scenePrompt: { id: 's', name: 'Cảnh', overridden: false }, coverPrompt: { id: 'c', name: 'Bìa', overridden: false } }
const tile = (id: string, url: string, narration: string) => ({ id, kind: 'scene', status: 'review', currentVersion: 1, reviewVersion: 1, versions: [{ version: 1, originalUrl: url }], narration })

it('huỷ hộp vẽ lại một ảnh đã biến mất thay vì lặng lẽ gửi vẽ lại cả bộ', async () => {
  const fetcher = vi.fn(async () => Response.json({ revision: 8 }))
  vi.stubGlobal('fetch', fetcher)
  state.images = { manifest, items: [tile('scene-one', '/one.webp', 'Cảnh một'), tile('scene-two', '/two.webp', 'Cảnh hai')] }
  await act(async () => root.render(<LiveStudio job={job} onChangePrompt={() => {}} onRefresh={() => {}} />))
  // Chọn ảnh thứ hai rồi mở hộp vẽ lại đúng ảnh đó.
  await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '2', bubbles: true })) })
  await act(async () => host.querySelector<HTMLButtonElement>('[aria-label="Vẽ lại cảnh 2"]')!.click())
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Vẽ lại cảnh 2?')
  // Bộ ảnh làm mới trong lúc hộp mở: ảnh đã chọn không còn trong danh sách.
  state.images = { manifest, items: [tile('scene-one', '/one.webp', 'Cảnh một')] }
  await act(async () => root.render(<LiveStudio job={job} onChangePrompt={() => {}} onRefresh={() => {}} />))
  expect(document.querySelector('[role="dialog"]')).toBeNull()
  expect(fetcher).not.toHaveBeenCalled()
  expect(host.textContent).toContain('Chưa vẽ lại ảnh nào')
  expect(host.querySelectorAll('[role="option"]')[0]?.getAttribute('aria-selected')).toBe('true')
})

it('vẫn gửi vẽ lại cả bộ khi người dùng chọn rõ Vẽ lại tất cả', async () => {
  const fetcher = vi.fn(async () => Response.json({ revision: 8 }))
  vi.stubGlobal('fetch', fetcher)
  state.images = { manifest, items: [tile('scene-one', '/one.webp', 'Cảnh một'), tile('scene-two', '/two.webp', 'Cảnh hai')] }
  await act(async () => root.render(<LiveStudio job={job} onChangePrompt={() => {}} onRefresh={() => {}} redrawAllOpen onRedrawAllChange={() => {}} />))
  const dialog = document.querySelector('[role="dialog"]')!
  expect(dialog.textContent).toContain('Vẽ lại cả 2 ảnh?')
  const submit = [...dialog.querySelectorAll('button')].find(button => button.textContent === 'Vẽ lại 2 ảnh')!
  await act(async () => submit.click())
  expect(fetcher).toHaveBeenCalledOnce()
  const [url, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
  expect(url).toBe('/api/clone/job/images')
  expect(JSON.parse(String(init.body))).toEqual({ op: 'regenerateAll', expectedRevision: 7, confirmCount: 2, feedback: '' })
})

it.each([
  { name: 'editor export without artifacts', artifacts: [], data: { projectId: 'project', exportId: 'export', durationMs: 859471 }, url: '/api/editor/projects/project/exports/export' },
  { name: 'legacy MP4 artifact', artifacts: [{ id: 'mp4', path: 'video.mp4', contentType: 'video/mp4' }], data: {}, url: '/api/clone/job/artifacts/mp4' },
])('plays the $name without replacing the source duration with export elapsed time', async ({ artifacts, data, url }) => {
  const completed = {
    ...job, stageGroup: 'ready', activeStep: null, source: { ...job.source, durationSec: 3380.7 },
    steps: [{ step: 'export', status: 'succeeded', output: { artifacts, data }, progress: { unitsDone: 1, unitsTotal: 1, phase: '' }, error: null, startedAt: null, finishedAt: null }],
  }
  await act(async () => root.render(<LiveStudio job={completed} onChangePrompt={() => {}} onRefresh={() => {}} />))
  expect(host.querySelector('video')?.getAttribute('src')).toBe(url)
  expect(host.textContent).not.toContain('File video không còn trên máy.')
  expect(host.textContent).toContain('56:20')
  expect(host.textContent).not.toContain('14:19')
})

it('removes the player when the export is deleted after the completed job refreshes', async () => {
  const completed = {
    ...job, stageGroup: 'ready', activeStep: null,
    steps: [{ step: 'export', status: 'succeeded', output: { artifacts: [], data: { projectId: 'project', exportId: 'export', removedAt: '' } }, progress: { unitsDone: 1, unitsTotal: 1, phase: '' }, error: null, startedAt: null, finishedAt: null }],
  }
  await act(async () => root.render(<LiveStudio job={completed} onChangePrompt={() => {}} onRefresh={() => {}} />))
  expect(host.querySelector('video')).not.toBeNull()
  completed.steps[0].output.data.removedAt = '2026-11-05T00:00:00Z'
  await act(async () => root.render(<LiveStudio job={{ ...completed }} onChangePrompt={() => {}} onRefresh={() => {}} />))
  expect(host.querySelector('video')).toBeNull()
  expect(host.textContent).toContain('File video không còn trên máy.')
})
