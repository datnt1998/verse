// @vitest-environment jsdom
// Vẽ lại ảnh tại chỗ trong Editor (Gói V29 · §26): vẽ xong thì đổi đúng media của
// clip bằng một lệnh hoàn tác được, giữ chuyển động/keyframe/thời lượng; ⌘Z quay
// lại bản trước và báo máy chủ bản đang dùng; thao tác đang dở của người dùng
// không bị xoá.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, renderHook, screen, waitFor, within } from '@testing-library/react'
import { cache } from '@noverse/ui/data'
import { dismissConfirmations } from '@noverse/ui'
import { applyViderseLabels } from '../../kitLabels'
import { getCommandExecutor, useCommandHistory } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { setEditorStorage, type EditorStorage } from '@editor/storage/storage-backend'
import type { MediaMetadata } from '@editor/types/storage'
import type { ImageItem, SubtitleSegmentItem, TimelineTrack } from '@editor/types/timeline'
import type { JobImage } from '../../api/images'
import { bindFinishedVersion, resetSceneVersionTraces, undoSceneSwap, useSceneRedraws, useSceneSwapStore } from './scene-images'
import { SceneImagePanel } from './SceneImagePanel'
import { Inspector } from './Inspector'
import { editorCommand } from './ContentPanels'
import { Timeline } from './Timeline'

const stream = vi.hoisted(() => {
  const listeners = new Set<(data: unknown) => void>()
  return {
    listeners,
    fire: (data: unknown) => { for (const listener of [...listeners]) listener(data) },
    reset: () => listeners.clear(),
  }
})
vi.mock('../../data/events', () => ({
  events: { onReconnect: () => () => {} },
  subscribeEvents: (_type: unknown, handler: (data: unknown) => void) => {
    stream.listeners.add(handler)
    return () => { stream.listeners.delete(handler) }
  },
}))

const tracks: TimelineTrack[] = [
  { id: 'scenes', name: 'Cảnh', kind: 'video', height: 64, locked: false, visible: true, muted: false, solo: false, order: 0, templateSlot: 'scenes' },
  { id: 'subs', name: 'Phụ đề', kind: 'video', height: 40, locked: false, visible: true, muted: false, solo: false, order: 1, templateSlot: 'captions' },
]

function sceneItem(patch: Partial<ImageItem> = {}): ImageItem {
  return {
    id: 'p1-scene-img-1', type: 'image', trackId: 'scenes', label: 'Đèn pin soi phòng trống',
    from: 120, durationInFrames: 90, mediaId: 'm1', src: '/api/editor/media/m1?v=h1',
    sourceWidth: 1920, sourceHeight: 1080,
    transform: { x: 0, y: 0, width: 1920, height: 1080, opacity: 1 },
    motionModifiers: [{ id: 'drift', type: 'float-drift', enabled: true, amplitude: 0.3, frequency: 0.08, phaseFrames: 0, seed: 1 }],
    ...patch,
  }
}

const neighbour = sceneItem({ id: 'p1-scene-img-2', label: 'Cảnh 2', from: 210, mediaId: 'm9', src: '/api/editor/media/m9?v=h9' })

const media = (id: string, hash: string): MediaMetadata => ({
  id, storageType: 'server', contentHash: hash, fileName: `${id}.jpg`, fileSize: 1024,
  mimeType: 'image/jpeg', duration: 0, width: 1920, height: 1080, fps: 0, codec: '', bitrate: 0,
})

function version(n: number, patch: Partial<JobImage['versions'][number]> = {}): JobImage['versions'][number] {
  return {
    n, state: 'ready', url: `/api/clone/job-1/images/img-1/versions/${n}`, previewUrl: null, width: 1920, height: 1080,
    parts: [{ kind: 'scene', text: `Cảnh 5 bản ${n}: đèn pin soi vào phòng trống, bụi lơ lửng trong chùm sáng.`, source: { type: 'artist', label: 'Hoạ sĩ' } }],
    sentText: `Cảnh 5 bản ${n}`, refs: [], provider: 'chatgpt', model: 'gpt-image-1', params: { size: '1536x1024' },
    feedback: n > 1 ? 'thêm bóng người cạnh cửa sổ' : null, diffFromPrev: null,
    startedAt: '2026-10-31T16:58:00Z', finishedAt: '2026-10-31T16:58:41Z', durationSec: 41, costImages: 1, error: null,
    ...patch,
  }
}

function jobImage(patch: Partial<JobImage> = {}): JobImage {
  return { id: 'img-1', kind: 'scene', index: 0, label: 'Cảnh 1', usedVersion: 1, versions: [version(1), version(2)], refLimit: 4, ...patch }
}

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { 'content-type': 'application/json' } })
}

const useResult = {
  imageId: 'img-1', n: 2, usedVersion: 2, needsReview: true, projectId: 'p1',
  clipId: 'p1-scene-img-1', mediaId: 'm2', src: '/api/editor/media/m2?v=h2',
  previousMediaId: 'm1', previousVersion: 1,
}

beforeEach(() => {
  applyViderseLabels()
  cache.clear()
  getCommandExecutor().clearHistory()
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false, media: query, onchange: null,
    addEventListener: () => undefined, removeEventListener: () => undefined,
    addListener: () => undefined, removeListener: () => undefined, dispatchEvent: () => false,
  }))
  setEditorStorage({
    mediaUrl: (id: string, hash: string) => `/api/editor/media/${id}?v=${hash}`,
    linkMedia: async () => undefined,
    listMedia: async () => [],
  } as unknown as EditorStorage)
  useItemsStore.getState().setTracks(tracks)
  useItemsStore.getState().setItems([sceneItem(), neighbour])
  useKeyframesStore.setState({ keyframes: [{ itemId: 'p1-scene-img-1', properties: [{ property: 'x', keyframes: [{ id: 'k1', frame: 0, value: 0, easing: 'linear' }] }] }] })
  useMediaLibraryStore.setState({ mediaItems: [media('m1', 'h1'), media('m2', 'h2'), media('m9', 'h9')], loadMediaItems: async () => undefined })
  useProjectStore.setState({ currentProject: { id: 'p1', name: 'Video thử', metadata: { width: 1920, height: 1080, fps: 30 } } as never })
  useTimelineSettingsStore.setState({ fps: 30 })
  usePlaybackStore.setState({ currentFrame: 150, isPlaying: false })
  useSelectionStore.setState({ selectedIds: ['p1-scene-img-1'] })
  useSceneSwapStore.getState().clear()
  resetSceneVersionTraces()
})

afterEach(async () => {
  await act(async () => { dismissConfirmations(); cleanup(); await Promise.resolve() })
  stream.reset(); vi.unstubAllGlobals(); vi.restoreAllMocks()
})

describe('redrawn scene replaces the clip media in place', () => {
  it('swaps only the picture of that clip and keeps motion, keyframes, timing and every other clip', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/use')) return json(useResult)
      return json({ images: [jobImage({ usedVersion: 2, needsReview: true })] })
    }))
    const before = useItemsStore.getState().items
    const notice = await bindFinishedVersion('job-1', 'img-1', 2)
    const after = useItemsStore.getState().items
    const clip = after.find(entry => entry.id === 'p1-scene-img-1') as ImageItem
    expect(clip.mediaId).toBe('m2')
    expect(clip.src).toBe('/api/editor/media/m2?v=h2')
    expect(clip.sourceWidth).toBe(1920)
    // Thời lượng, vị trí, chuyển động giữ nguyên; clip khác không bị đụng tới.
    expect([clip.from, clip.durationInFrames]).toEqual([120, 90])
    expect(clip.motionModifiers).toEqual(sceneItem().motionModifiers)
    expect(after.find(entry => entry.id === 'p1-scene-img-2')).toBe(before.find(entry => entry.id === 'p1-scene-img-2'))
    expect(useKeyframesStore.getState().keyframes[0]?.properties[0]?.keyframes).toHaveLength(1)
    // Thao tác đang dở của người dùng (khung phát, vùng chọn) không bị đặt lại.
    expect(usePlaybackStore.getState().currentFrame).toBe(150)
    expect(useSelectionStore.getState().selectedIds).toEqual(['p1-scene-img-1'])
    expect(notice).toMatchObject({ label: 'Cảnh 1', n: 2, mediaId: 'm2', previousMediaId: 'm1' })
    expect(useCommandHistory.getState().canUndo).toBe(true)
  })

  it('is one undo step and ⌘Z tells the server the clip is back on version 1', async () => {
    let used = 1
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/use')) {
        used = JSON.parse(String(init?.body)).n
        return json({ ...useResult, usedVersion: used })
      }
      return json({ images: [jobImage({ usedVersion: used })] })
    })
    vi.stubGlobal('fetch', fetchMock)
    const { result } = renderHook(() => useSceneRedraws('job-1'))
    await waitFor(() => expect(result.current.images).toHaveLength(1))
    await act(async () => { await bindFinishedVersion('job-1', 'img-1', 2) })
    await act(async () => { await result.current.reload() })
    await waitFor(() => expect(result.current.images[0].usedVersion).toBe(2))
    act(() => { getCommandExecutor().undo() })
    expect((useItemsStore.getState().itemById['p1-scene-img-1'] as ImageItem).mediaId).toBe('m1')
    await waitFor(() => {
      const uses = fetchMock.mock.calls.map(([input]) => String(input)).filter(url => url.includes('/use'))
      expect(uses).toHaveLength(2)
    })
    await waitFor(() => expect(result.current.images[0].usedVersion).toBe(1))
    const bodies = fetchMock.mock.calls
      .filter(([input]) => String(input).includes('/use'))
      .map(([, init]) => JSON.parse(String((init as RequestInit | undefined)?.body ?? '{}')))
    expect(bodies[0]).toEqual({ n: 2, project: false })
    expect(bodies[1]).toEqual({ n: 1, project: false })
    // Hoàn tác xong thì thông báo “đã dùng bản 2” không còn treo.
    expect(useSceneSwapStore.getState().notices).toHaveLength(0)
  })

  it('undoes repeated redraws to the local image even after the worker advanced the server version', async () => {
    let next = 2
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('/use')) return json({ ...useResult, n: next, usedVersion: next, mediaId: `m${next}`, previousMediaId: `m${next}`, previousVersion: next })
      return json({ images: [jobImage()] })
    })
    vi.stubGlobal('fetch', fetchMock)
    useMediaLibraryStore.setState(state => ({ mediaItems: [...state.mediaItems, media('m3', 'h3')] }))
    const { result } = renderHook(() => useSceneRedraws('job-1'))
    await waitFor(() => expect(result.current.images).toHaveLength(1))
    await act(async () => { expect(await bindFinishedVersion('job-1', 'img-1', 2)).toMatchObject({ previousMediaId: 'm1', previousVersion: 1 }) })
    next = 3
    await act(async () => { expect(await bindFinishedVersion('job-1', 'img-1', 3)).toMatchObject({ previousMediaId: 'm2', previousVersion: 2 }) })
    act(() => { getCommandExecutor().undo() })
    await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => String(input).includes('/use'))).toHaveLength(3))
    expect((useItemsStore.getState().itemById['p1-scene-img-1'] as ImageItem).mediaId).toBe('m2')
    act(() => { getCommandExecutor().undo() })
    await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => String(input).includes('/use'))).toHaveLength(4))
    expect((useItemsStore.getState().itemById['p1-scene-img-1'] as ImageItem).mediaId).toBe('m1')
    const uses = fetchMock.mock.calls.filter(([input]) => String(input).includes('/use'))
    expect(uses.slice(2).map(([, init]) => JSON.parse(String((init as RequestInit).body)).n)).toEqual([2, 1])
  })

  it.each(['opacity', 'other clip'] as const)('toast undo restores only media after a later %s edit', async (edit) => {
    let used = 1
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes('/use')) {
        used = JSON.parse(String(init?.body)).n
        return json({ ...useResult, usedVersion: used })
      }
      return json({ images: [jobImage({ usedVersion: used })] })
    })
    vi.stubGlobal('fetch', fetchMock)
    const original = sceneItem({ sourceWidth: 1280, sourceHeight: 720, thumbnailUrl: '/old-thumbnail.jpg', trimStart: 2, trimEnd: 3 })
    useItemsStore.getState().setItems([original, neighbour])
    const { result } = renderHook(() => useSceneRedraws('job-1'))
    await waitFor(() => expect(result.current.images).toHaveLength(1))
    const notice = await act(async () => bindFinishedVersion('job-1', 'img-1', 2))
    expect(notice).not.toBeNull()
    if (!notice) throw new Error('Expected a scene swap notice')
    act(() => {
      useSceneSwapStore.getState().push(notice)
      if (edit === 'opacity') {
        const clip = useItemsStore.getState().itemById[original.id]
        editorCommand([{ type: 'item.update', params: { id: clip.id, updates: { transform: { ...clip.transform, opacity: 0.35 } } } }])
      } else {
        editorCommand([{ type: 'item.update', params: { id: neighbour.id, updates: { label: 'Cảnh 2 đã sửa', from: 240, durationInFrames: 120 } } }])
      }
    })
    const beforeUndo = useItemsStore.getState().items
    const keysBeforeUndo = useKeyframesStore.getState().keyframes
    const editedEntry = getCommandExecutor().history.stacks(getCommandExecutor().contextKey).undo.at(-1)
    act(() => undoSceneSwap(notice))
    const restored = useItemsStore.getState().itemById[original.id]
    expect(restored).toEqual({
      ...beforeUndo.find(clip => clip.id === original.id),
      mediaId: original.mediaId, src: original.src,
      sourceWidth: original.sourceWidth, sourceHeight: original.sourceHeight,
      thumbnailUrl: original.thumbnailUrl, trimStart: original.trimStart, trimEnd: original.trimEnd,
    })
    expect(useItemsStore.getState().itemById[neighbour.id]).toBe(beforeUndo.find(clip => clip.id === neighbour.id))
    expect(useKeyframesStore.getState().keyframes).toBe(keysBeforeUndo)
    expect(useSceneSwapStore.getState().notices).toHaveLength(0)
    const undo = getCommandExecutor().history.stacks(getCommandExecutor().contextKey).undo
    expect(undo).toContain(editedEntry)
    expect(undo).toContain(notice.historyEntry)
    expect(undo).toHaveLength(3)
    await waitFor(() => expect(result.current.images[0].usedVersion).toBe(1))
    expect(fetchMock.mock.calls.filter(([input]) => String(input).includes('/use')).map(([, init]) => JSON.parse(String(init?.body)))).toEqual([
      { n: 2, project: false }, { n: 1, project: false },
    ])
    // A targeted restore is a normal compensating command: CmdZ undoes just it.
    act(() => { getCommandExecutor().undo() })
    expect(useItemsStore.getState().items).toEqual(beforeUndo)
    await waitFor(() => expect(result.current.images[0].usedVersion).toBe(2))
  })

  it('uses ordinary undo/redo when the toast still owns the newest history entry', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(useResult)))
    const original = useItemsStore.getState().items
    const notice = await bindFinishedVersion('job-1', 'img-1', 2)
    if (!notice) throw new Error('Expected a scene swap notice')
    undoSceneSwap(notice)
    expect(useItemsStore.getState().items).toEqual(original)
    const stacks = getCommandExecutor().history.stacks(getCommandExecutor().contextKey)
    expect(stacks.undo).toHaveLength(0)
    expect(stacks.redo).toEqual([notice.historyEntry])
    getCommandExecutor().redo()
    expect(useItemsStore.getState().itemById[notice.clipId].mediaId).toBe('m2')
  })

  it('does not undo an unrelated edit after the swap history was cleared', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(useResult)))
    const notice = await bindFinishedVersion('job-1', 'img-1', 2)
    if (!notice) throw new Error('Expected a scene swap notice')
    getCommandExecutor().clearHistory()
    editorCommand([{ type: 'item.update', params: { id: neighbour.id, updates: { label: 'Giữ sửa đổi' } } }])
    const before = useItemsStore.getState().items
    undoSceneSwap(notice)
    expect(useItemsStore.getState().items).toBe(before)
    expect(getCommandExecutor().history.stacks(getCommandExecutor().contextKey).undo).toHaveLength(1)
  })

  it('shows the drawing version as soon as progress arrives, before the list reload carries it', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json({ images: [jobImage({ versions: [version(1)] })] })))
    const { result, unmount } = renderHook(() => useSceneRedraws('job-1'))
    await waitFor(() => { expect(result.current.images).toHaveLength(1) })
    act(() => { stream.fire({ jobId: 'job-1', imageId: 'img-1', n: 2, phase: 'drawing', elapsedSec: 4 }) })
    await waitFor(() => {
      const drawing = result.current.images[0]?.versions.find(entry => entry.n === 2)
      expect(drawing?.state).toBe('drawing')
    })
    expect(result.current.progress['img-1']?.n).toBe(2)
    unmount()
  })

  it('applies a finished redraw from the progress event without touching other clips', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/use')) return json(useResult)
      return json({ images: [jobImage({ usedVersion: 2 })] })
    }))
    const otherClip = useItemsStore.getState().itemById['p1-scene-img-2']
    const { unmount } = renderHook(() => useSceneRedraws('job-1'))
    act(() => stream.fire({ jobId: 'job-1', imageId: 'img-1', n: 2, phase: 'done', elapsedSec: 21 }))
    await waitFor(() => { expect((useItemsStore.getState().itemById['p1-scene-img-1'] as ImageItem).mediaId).toBe('m2') })
    expect(useSceneSwapStore.getState().notices[0]).toMatchObject({ label: 'Cảnh 1', n: 2 })
    expect(useItemsStore.getState().itemById['p1-scene-img-2']).toBe(otherClip)
    unmount()
  })
})

describe('scene panel', () => {
  it('shows the used version, its prompt and refs, and can pick another version', () => {
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage()} />)
    expect(screen.getByRole('heading', { name: /Cảnh 1/ })).toBeTruthy()
    expect(screen.getByRole('img', { name: 'Cảnh 1 · bản 1' }).getAttribute('src')).toBe('/api/clone/job-1/images/img-1/versions/1')
    expect(screen.getByText(/đèn pin soi vào phòng trống/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Bản 1 · Lần đầu · Hoạ sĩ viết từ lời kể · đang dùng' }).getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(screen.getByRole('button', { name: /Bản 2 · Theo góp ý/ }))
    expect(screen.getByRole('img', { name: 'Cảnh 1 · bản 2' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Dùng bản 2' })).toBeTruthy()
  })

  it('opens the same historical version in the shared viewer and keeps its version rail aligned', async () => {
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage()} />)
    fireEvent.click(screen.getByRole('button', { name: /Bản 2 · Theo góp ý/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Xem lớn Đèn pin soi phòng trống' }))
    const viewer = await screen.findByRole('dialog')
    expect(within(viewer).getByRole('img', { name: 'Cảnh 1', exact: true }).getAttribute('src')).toBe('/api/clone/job-1/images/img-1/versions/2')
    expect(within(viewer).getByRole('button', { name: /Bản 2 · Theo góp ý/ }).getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(within(viewer).getByRole('button', { name: /Bản 1 · Lần đầu/ }))
    expect(within(viewer).getByRole('img', { name: 'Cảnh 1', exact: true }).getAttribute('src')).toBe('/api/clone/job-1/images/img-1/versions/1')
    expect(within(viewer).getByRole('button', { name: /Bản 1 · Lần đầu/ }).getAttribute('aria-pressed')).toBe('true')
  })

  it('keeps the open Editor as the only project writer for redraw', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => json({ versions: [{ imageId: 'img-1', n: 3 }] }))
    vi.stubGlobal('fetch', fetchMock)
    const save = vi.fn(async () => true)
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage()} beforeMutation={save} />)
    fireEvent.change(screen.getByLabelText('Góp ý để vẽ lại cảnh 1'), { target: { value: 'Giữ chuyển động, thêm đèn đỏ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Vẽ lại', exact: true }))
    const confirmation = await screen.findByRole('alertdialog')
    expect(fetchMock).not.toHaveBeenCalled()
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Vẽ lại', exact: true }))
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(save).toHaveBeenCalledTimes(1)
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ imageIds: ['img-1'], project: false, feedback: 'Giữ chuyển động, thêm đèn đỏ' })
  })

  it('does not request a redraw when saving the editor fails', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage()} beforeMutation={async () => false} />)
    fireEvent.click(screen.getByRole('button', { name: 'Vẽ lại', exact: true }))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('Chưa lưu được video'))
    expect(fetchMock).not.toHaveBeenCalled()
    expect((screen.getByRole('button', { name: 'Vẽ lại', exact: true }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('keeps a prompt edit in progress when a redraw of the same scene finishes', () => {
    const { rerender } = render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Sửa' }))
    const area = screen.getByLabelText('Phần Cảnh của chỉ dẫn') as HTMLTextAreaElement
    fireEvent.change(area, { target: { value: 'Cảnh 5: đèn pin soi vào phòng trống, có bóng người cạnh cửa sổ' } })
    expect(screen.getByText('Đã sửa')).toBeTruthy()
    rerender(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage({ usedVersion: 2, needsReview: true })} />)
    expect((screen.getByLabelText('Phần Cảnh của chỉ dẫn') as HTMLTextAreaElement).value).toBe('Cảnh 5: đèn pin soi vào phòng trống, có bóng người cạnh cửa sổ')
  })

  it('shows the drawing state in the box, keeps the used version selectable and can cancel the draw', () => {
    const onRedrawn = vi.fn()
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage({ versions: [version(1), version(2, { state: 'drawing', url: null, finishedAt: null, durationSec: null })] })} progress={{ jobId: 'job-1', imageId: 'img-1', n: 2, phase: 'drawing', elapsedSec: 21 }} beforeMutation={async () => true} onRedrawn={onRedrawn} />)
    // Đang vẽ bản mới vẫn giữ bản đã duyệt và provenance; ẩn thao tác sửa/xoá ảnh.
    expect(screen.getByText('Đã duyệt')).toBeTruthy()
    expect(screen.queryByText('Đang vẽ bản 2')).toBeNull()
    expect(screen.queryByLabelText('Góp ý để vẽ lại cảnh 1')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Thay ảnh' })).toBeNull()
    expect(screen.getByText('Hoạ sĩ đang vẽ bản 2')).toBeTruthy()
    expect(screen.getByText('Đang vẽ')).toBeTruthy()
    expect(screen.getByText('0:21 · thường khoảng 41 giây')).toBeTruthy()
    expect(screen.getByText(/Bản 1 · đang vẽ bản 2/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Huỷ lần vẽ' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /^Vẽ lại$/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Sửa' }))
    const redraw = screen.getByRole('button', { name: 'Vẽ lại với chỉ dẫn này' })
    expect(redraw.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(redraw)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('marks a redrawn scene as needing review again and offers the way back', () => {
    render(<SceneImagePanel item={sceneItem()} number={1} jobId="job-1" image={jobImage({ usedVersion: 2, needsReview: true })} />)
    expect(screen.getByText('Bản mới · cần duyệt lại')).toBeTruthy()
    expect(screen.getByText(/⌘Z để quay lại bản 1/)).toBeTruthy()
  })
})

describe('inspector scene controls', () => {
  beforeEach(() => {
    vi.stubGlobal('ResizeObserver', class {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    useTransitionsStore.setState({ transitions: [] })
    useSelectionStore.getState().selectItems(['p1-scene-img-1'])
  })

  function renderInspector() {
    return render(<Inspector ranges={[]} selectedRange={null} onSelectRange={() => undefined} onAlign={async () => undefined} aligning={false} />)
  }

  it('preserves custom motion until choosing a preset, which is undoable', () => {
    const original = useItemsStore.getState().itemById['p1-scene-img-1']
    const originalKeyframes = useKeyframesStore.getState().keyframes
    renderInspector()
    const motion = screen.getByRole('radiogroup', { name: 'Chuyển động của ảnh' })
    expect(within(motion).getAllByRole('radio').every(radio => radio.getAttribute('aria-checked') === 'false')).toBe(true)
    fireEvent.click(within(motion).getByRole('radio', { name: 'Zoom chậm' }))
    expect((useItemsStore.getState().itemById[original.id] as ImageItem).motionModifiers).toEqual([])
    const lanes = useKeyframesStore.getState().keyframes.find(entry => entry.itemId === original.id)?.properties
    expect(lanes?.find(lane => lane.property === 'width')?.keyframes.map(key => [key.frame, key.value])).toEqual([[0, 1920], [89, 2112]])
    expect(lanes?.find(lane => lane.property === 'height')?.keyframes.map(key => [key.frame, key.value])).toEqual([[0, 1080], [89, 1188]])
    expect(within(motion).getByRole('radio', { name: 'Zoom chậm' }).getAttribute('aria-checked')).toBe('true')
    act(() => { getCommandExecutor().undo() })
    expect(useItemsStore.getState().itemById[original.id]).toEqual(original)
    expect(useKeyframesStore.getState().keyframes).toEqual(originalKeyframes)
    expect(within(motion).getAllByRole('radio').every(radio => radio.getAttribute('aria-checked') === 'false')).toBe(true)
  })

  it('uses keyboard selection for adjacent-scene transitions and retains undo', () => {
    renderInspector()
    const transition = screen.getByRole('radiogroup', { name: 'Chuyển sang cảnh 2' })
    fireEvent.keyDown(within(transition).getByRole('radio', { name: 'Cắt' }), { key: 'End' })
    expect(useTransitionsStore.getState().transitions).toEqual([expect.objectContaining({
      leftClipId: 'p1-scene-img-1', rightClipId: 'p1-scene-img-2',
      presentation: 'slide', direction: 'from-left', durationInFrames: 36,
    })])
    fireEvent.click(within(transition).getByRole('radio', { name: 'Cắt' }))
    expect(useTransitionsStore.getState().transitions).toEqual([])
    act(() => { getCommandExecutor().undo() })
    expect(useTransitionsStore.getState().transitions[0]?.presentation).toBe('slide')
  })
})

it('keeps the selected subtitle cue highlighted when moving between timeline cues', () => {
  const width = vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1000)
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  const caption: SubtitleSegmentItem = {
    id: 'sub-1', type: 'subtitle', trackId: 'captions', from: 0, durationInFrames: 180,
    label: 'Lời thoại', mediaId: 'speech', source: { type: 'transcript', mediaId: 'speech', clipId: 'voice' },
    color: '#ffffff',
    cues: [
      { id: 'cue-1', startSeconds: 0, endSeconds: 3, text: 'Lời thứ nhất' },
      { id: 'cue-2', startSeconds: 3, endSeconds: 6, text: 'Lời thứ hai' },
    ],
  }
  useItemsStore.setState({
    items: [caption], itemById: { [caption.id]: caption },
    tracks: [{ id: 'captions', name: 'Phụ đề', order: 0, visible: true, locked: false }],
  })
  useSelectionStore.getState().selectItems([])
  usePlaybackStore.setState({ currentFrame: 0 })
  try {
    render(<Timeline ranges={[]} selectedRange={-1} onSelectRange={() => undefined} />)
    const first = screen.getByRole('button', { name: /^Lời thứ nhất,/ })
    const second = screen.getByRole('button', { name: /^Lời thứ hai,/ })
    fireEvent.click(second)
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['sub-1'])
    expect(usePlaybackStore.getState().currentFrame).toBe(90)
    expect(second.getAttribute('aria-pressed')).toBe('true')
    expect(first.getAttribute('aria-pressed')).toBe('false')
    fireEvent.click(first)
    expect(usePlaybackStore.getState().currentFrame).toBe(0)
    expect(first.getAttribute('aria-pressed')).toBe('true')
    expect(second.getAttribute('aria-pressed')).toBe('false')
    act(() => { usePlaybackStore.setState({ currentFrame: 90 }) })
    expect(second.getAttribute('aria-pressed')).toBe('true')
    expect(first.getAttribute('aria-pressed')).toBe('false')
    act(() => { usePlaybackStore.setState({ currentFrame: 180 }) })
    expect(first.getAttribute('aria-pressed')).toBe('false')
    expect(second.getAttribute('aria-pressed')).toBe('false')
  } finally {
    cleanup()
    width.mockRestore()
    vi.unstubAllGlobals()
  }
})
