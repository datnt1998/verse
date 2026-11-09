// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { bindCommandExecutor, CommandExecutor } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useZoomStore } from '@editor/features/timeline/stores/zoom-store'
import { useSelectionStore } from '@editor/shared/state/selection'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useEditorStore } from '@editor/shared/state/editor'
import { makeTimelineTrack, makeTimelineVideoItem, resetTimelineCompositionTestState } from '@editor/features/timeline/test-helpers'
import { ShortcutProvider } from '../ui/shortcuts'
import { Timeline, clampClipGestureDelta } from './Timeline'
import type { SubtitleSegmentItem } from '@editor/types/timeline'
import { buildCaptionPages } from '@editor/features/editing-templates/caption-pages'

const clip = makeTimelineVideoItem({ id: 'moving', trackId: 'visual', label: 'Scene', from: 30, durationInFrames: 60, sourceStart: 30, sourceEnd: 90, sourceDuration: 180 })

function pointer(target: Element, type: string, clientX: number, options: MouseEventInit = {}) {
  const event = new MouseEvent(type, { bubbles: true, button: 0, clientX, clientY: 100, altKey: true, ...options })
  Object.defineProperty(event, 'pointerId', { value: 1 })
  fireEvent(target, event)
}

function capture(target: HTMLElement) {
  Object.assign(target, { setPointerCapture: vi.fn(), hasPointerCapture: () => false, releasePointerCapture: vi.fn() })
}

let executor: CommandExecutor
let unbind: () => void
beforeEach(() => {
  resetTimelineCompositionTestState()
  executor = new CommandExecutor()
  unbind = bindCommandExecutor(executor)
  useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'visual', name: 'Visual', order: 0, kind: 'video' })])
  useItemsStore.getState().setItems([clip])
  useTimelineSettingsStore.setState({ fps: 30, snapEnabled: false })
  useSelectionStore.getState().clearItemSelection()
  useEditorStore.setState({ linkedSelectionEnabled: true })
  usePlaybackStore.setState({ currentFrame: 45, previewFrame: null, isPlaying: false })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1000)
})
afterEach(() => {
  cleanup()
  unbind()
  resetTimelineCompositionTestState()
  useSelectionStore.getState().clearItemSelection()
  vi.restoreAllMocks()
})

function mount(name = /^Scene,/u) {
  const view = render(<ShortcutProvider><div data-editor-root><Timeline ranges={[]} selectedRange={-1} onSelectRange={() => {}} /></div></ShortcutProvider>)
  const root = view.container.querySelector<HTMLElement>('[data-editor-root]')!
  const rect = new DOMRect(0, 0, 1000, 800)
  // jsdom has no layout; retain the real shortcut provider's visible-root contract.
  vi.spyOn(root, 'getClientRects').mockReturnValue(Object.assign([rect], { item: (index: number) => index === 0 ? rect : null }))
  act(() => { useZoomStore.getState().setZoomLevelSynchronized(0.3) }) // One screen pixel per frame at 30fps.
  const button = screen.getByRole('button', { name })
  capture(button)
  return button
}

describe('v13 timeline real command gestures', () => {
  it('selects/seeks, splits, deletes and restores actual timeline items through history', () => {
    fireEvent.click(mount())
    expect(useSelectionStore.getState().selectedItemIds).toEqual([clip.id])
    expect(usePlaybackStore.getState().currentFrame).toBe(30)
    act(() => { usePlaybackStore.getState().finishScrub(45) })
    fireEvent.click(screen.getByRole('button', { name: /^Tách/u }))
    expect(useItemsStore.getState().items.map(item => [item.from, item.durationInFrames]).sort((a, b) => a[0] - b[0])).toEqual([[30, 15], [45, 45]])
    const rightId = useSelectionStore.getState().selectedItemIds[0]
    expect(useItemsStore.getState().itemById[rightId].from).toBe(45)
    fireEvent.click(screen.getByRole('button', { name: 'Xoá' }))
    expect(useItemsStore.getState().itemById[rightId]).toBeUndefined()
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(2)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().itemById[rightId].durationInFrames).toBe(45)
  })

  it('changes actual track visibility and seeks by the ruler keyboard', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Ẩn rãnh Visual' }))
    expect(useItemsStore.getState().tracks[0].visible).toBe(false)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().tracks[0].visible).toBe(true)
    const ruler = screen.getByRole('slider', { name: 'Vị trí phát' })
    fireEvent.keyDown(ruler, { key: 'End', code: 'End' })
    expect(usePlaybackStore.getState().currentFrame).toBe(89)
    fireEvent.keyDown(ruler, { key: 'Home', code: 'Home' })
    expect(usePlaybackStore.getState().currentFrame).toBe(0)
  })

  it('moves only on release, suppresses the trailing seek click and undoes/redoes as one edit', () => {
    const button = mount()
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointermove', 220)
    expect(useItemsStore.getState().items[0].from).toBe(30)
    pointer(button, 'pointerup', 220)
    fireEvent.click(button, { detail: 1 })
    expect(useItemsStore.getState().items[0].from).toBe(50)
    expect(usePlaybackStore.getState().currentFrame).toBe(45)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items[0].from).toBe(30)
    act(() => { executor.redo() })
    expect(useItemsStore.getState().items[0].from).toBe(50)
  })

  it('cancels a real trim with Escape without touching source bounds, keyframes or history', () => {
    useKeyframesStore.getState().setKeyframes([{ itemId: clip.id, properties: [{ property: 'x', keyframes: [{ id: 'key', frame: 15, value: 20, easing: 'linear' }] }] }])
    const button = mount()
    const start = button.querySelector('[data-trim-handle="start"]')!
    const before = structuredClone({ items: useItemsStore.getState().items, lanes: useKeyframesStore.getState().keyframes })
    pointer(start, 'pointerdown', 200)
    pointer(button, 'pointermove', 215)
    fireEvent.keyDown(button, { key: 'Escape', code: 'Escape' })
    pointer(button, 'pointerup', 215)
    expect({ items: useItemsStore.getState().items, lanes: useKeyframesStore.getState().keyframes }).toEqual(before)
    expect(executor.canUndo).toBe(false)
    fireEvent.click(button) // Keyboard activation has detail zero and must not be swallowed after cancellation.
    expect(usePlaybackStore.getState().currentFrame).toBe(30)
  })

  it('routes trim through source-aware commands and restores the whole edit on undo', () => {
    const button = mount()
    const before = structuredClone(useItemsStore.getState().items)
    pointer(button.querySelector('[data-trim-handle="start"]')!, 'pointerdown', 200)
    pointer(button, 'pointerup', 215)
    expect(useItemsStore.getState().items[0]).toMatchObject({ from: 45, durationInFrames: 45, sourceStart: 45 })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('snaps the trailing edge even when the leading edge has no nearby target', () => {
    useItemsStore.getState().setItems([clip, { ...clip, id: 'next', label: 'Next', from: 110 }])
    useTimelineSettingsStore.setState({ snapEnabled: true })
    const button = mount()
    pointer(button, 'pointerdown', 200, { altKey: false })
    pointer(button, 'pointerup', 215, { altKey: false })
    expect(useItemsStore.getState().itemById.moving.from).toBe(50)
    expect(useItemsStore.getState().itemById.next.from).toBe(110)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().itemById.moving.from).toBe(30)
  })

  it('keeps trim preview and committed bounds constrained by selected neighbors', () => {
    const second = { ...clip, id: 'next', label: 'Next', from: 100 }
    useItemsStore.getState().setItems([clip, second])
    useSelectionStore.getState().selectItems([clip.id, second.id])
    const before = structuredClone(useItemsStore.getState().items)
    const previewDelta = clampClipGestureDelta(before, before, 'end', 20, 30)
    expect(previewDelta).toBe(10)
    const button = mount()
    pointer(button.querySelector('[data-trim-handle="end"]')!, 'pointerdown', 200)
    pointer(button, 'pointerup', 220)
    for (const item of before) expect(useItemsStore.getState().itemById[item.id].durationInFrames).toBe(item.durationInFrames + previewDelta)
    expect(useItemsStore.getState().itemById.moving.from + useItemsStore.getState().itemById.moving.durationInFrames).toBe(second.from)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('never moves a locked participant', () => {
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'visual', name: 'Visual', order: 0, locked: true })])
    const button = mount()
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointerup', 230)
    expect(useItemsStore.getState().items[0].from).toBe(30)
    expect(executor.canUndo).toBe(false)
  })

  it('moves a multiselection by one shared bounded delta in one undo entry', () => {
    const second = { ...clip, id: 'second', trackId: 'other', label: 'Other', from: 10 }
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'visual', name: 'Visual', order: 0 }), makeTimelineTrack({ id: 'other', name: 'Other', order: 1 })])
    useItemsStore.getState().setItems([clip, second])
    useSelectionStore.getState().selectItems([clip.id, second.id])
    const button = mount()
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointerup', 100)
    expect(useItemsStore.getState().items.map(item => item.from)).toEqual([20, 0])
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items.map(item => item.from)).toEqual([30, 10])
  })

  it('keeps the last scrub frame on pointer cancellation instead of seeking to the cancellation coordinates', () => {
    mount()
    const ruler = screen.getByRole('slider', { name: 'Vị trí phát' })
    capture(ruler)
    vi.spyOn(ruler, 'getBoundingClientRect').mockReturnValue({ x: 0, y: 0, left: 0, top: 0, width: 832, height: 24, right: 832, bottom: 24, toJSON: () => ({}) })
    pointer(ruler, 'pointerdown', 40)
    pointer(ruler, 'pointermove', 60)
    pointer(ruler, 'pointercancel', 0)
    expect(usePlaybackStore.getState().currentFrame).toBe(60)
    expect(usePlaybackStore.getState().previewFrame).toBeNull()
  })
})

const subtitle: SubtitleSegmentItem = {
  id: 'subtitle', type: 'subtitle', label: 'Captions', trackId: 'visual', from: 30, durationInFrames: 180, color: '#ffffff',
  source: { type: 'subtitle-import', fileName: 'captions.srt', format: 'srt', importedAt: 0 },
  cues: [{ id: 'first', startSeconds: 1, endSeconds: 2, text: 'First' }, { id: 'second', startSeconds: 4, endSeconds: 5, text: 'Second' }],
  words: [{ text: 'First', startFrame: 30, endFrame: 60 }, { text: 'Second', startFrame: 120, endFrame: 150 }],
  wordsEstimated: false,
}

describe('individual subtitle cue timeline gestures', () => {
  it('moves only the dragged cue and its original words in one atomic undoable command batch', () => {
    useItemsStore.getState().setItems([subtitle])
    const button = mount(/^First,/u)
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointermove', 215)
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(subtitle)
    pointer(button, 'pointerup', 215)
    const moved = useItemsStore.getState().itemById[subtitle.id] as SubtitleSegmentItem
    expect(moved.from).toBe(subtitle.from)
    expect(moved.durationInFrames).toBe(subtitle.durationInFrames)
    expect(moved.cues).toEqual([{ ...subtitle.cues[0], startSeconds: 1.5, endSeconds: 2.5 }, subtitle.cues[1]])
    expect(moved.words).toEqual([{ ...subtitle.words![0], startFrame: 45, endFrame: 75 }, subtitle.words![1]])
    expect(moved.wordsEstimated).toBe(false)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(subtitle)
    act(() => { executor.redo() })
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(moved)
  })

  it.each([
    { name: /^First,/u, handle: 'start', delta: 200, cue: 0, start: 59 / 30, end: 2 },
    { name: /^First,/u, handle: 'end', delta: 200, cue: 0, start: 1, end: 4 },
    { name: /^Second,/u, handle: 'start', delta: -200, cue: 1, start: 2, end: 5 },
    { name: /^Second,/u, handle: 'end', delta: 200, cue: 1, start: 4, end: 6 },
  ])('trims $handle for cue $cue within neighbors/item duration without rewriting words', ({ name, handle, delta, cue, start, end }) => {
    useItemsStore.getState().setItems([subtitle])
    const button = mount(name)
    pointer(button.querySelector(`[data-trim-handle="${handle}"]`)!, 'pointerdown', 200)
    pointer(button, 'pointerup', 200 + delta)
    const trimmed = useItemsStore.getState().itemById[subtitle.id] as SubtitleSegmentItem
    expect({ ...trimmed.cues[cue], startSeconds: subtitle.cues[cue].startSeconds, endSeconds: subtitle.cues[cue].endSeconds }).toEqual(subtitle.cues[cue])
    expect(trimmed.cues[cue].startSeconds * 30).toBeCloseTo(start * 30, 10)
    expect(trimmed.cues[cue].endSeconds * 30).toBeCloseTo(end * 30, 10)
    expect(trimmed.cues[1 - cue]).toEqual(subtitle.cues[1 - cue])
    expect(trimmed.words).toEqual(subtitle.words)
    expect(trimmed.from).toBe(subtitle.from)
    expect(trimmed.durationInFrames).toBe(subtitle.durationInFrames)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(subtitle)
  })

  it('uses renderer clipping for a shortened cue without destroying original word timing', () => {
    useItemsStore.getState().setItems([subtitle])
    const button = mount(/^First,/u)
    pointer(button.querySelector('[data-trim-handle="end"]')!, 'pointerdown', 200)
    pointer(button, 'pointerup', 185)
    const trimmed = useItemsStore.getState().itemById[subtitle.id] as SubtitleSegmentItem
    expect(trimmed.words).toEqual(subtitle.words)
    const rendered = buildCaptionPages(trimmed, 30).pages.flatMap(page => page.words)
    expect(rendered.find(word => word.text === 'First')).toMatchObject({ startFrame: 30, endFrame: 45 })
    expect(rendered.find(word => word.text === 'Second')).toMatchObject(subtitle.words![1])
    act(() => { executor.undo() })
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(subtitle)
  })

  it('cancels a cue drag without changing any cue, word or history entry', () => {
    useItemsStore.getState().setItems([subtitle])
    const button = mount(/^First,/u)
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointermove', 215)
    fireEvent.keyDown(button, { key: 'Escape', code: 'Escape' })
    pointer(button, 'pointerup', 215)
    expect(useItemsStore.getState().itemById[subtitle.id]).toEqual(subtitle)
    expect(executor.canUndo).toBe(false)
  })

  it('bounds a cue move at its next cue and does not move other selected items', () => {
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'visual', name: 'Visual', order: 0 }), makeTimelineTrack({ id: 'other', name: 'Other', order: 1 })])
    useItemsStore.getState().setItems([subtitle, { ...clip, trackId: 'other' }])
    useSelectionStore.getState().selectItems([subtitle.id, clip.id])
    const button = mount(/^First,/u)
    pointer(button, 'pointerdown', 200)
    pointer(button, 'pointerup', 400)
    const moved = useItemsStore.getState().itemById[subtitle.id] as SubtitleSegmentItem
    expect(moved.cues[0]).toEqual({ ...subtitle.cues[0], startSeconds: 3, endSeconds: 4 })
    expect(moved.cues[1]).toEqual(subtitle.cues[1])
    expect(moved.words![1]).toEqual(subtitle.words![1])
    expect(useItemsStore.getState().itemById[clip.id]).toEqual({ ...clip, trackId: 'other' })
  })
})

describe('v13 timeline gesture boundaries', () => {
  it('stops moves at adjacent clips rather than creating an overlap', () => {
    const next = { ...clip, id: 'next', from: 100 }
    expect(clampClipGestureDelta([clip], [clip, next], 'move', 50, 30)).toBe(10)
  })

  it('respects minimum duration and media source start/end', () => {
    expect(clampClipGestureDelta([clip], [clip], 'start', 100, 30)).toBe(59)
    expect(clampClipGestureDelta([clip], [clip], 'start', -100, 30)).toBe(-30)
    expect(clampClipGestureDelta([clip], [clip], 'end', 200, 30)).toBe(90)
    expect(clampClipGestureDelta([clip], [clip], 'end', -100, 30)).toBe(-59)
  })
})
