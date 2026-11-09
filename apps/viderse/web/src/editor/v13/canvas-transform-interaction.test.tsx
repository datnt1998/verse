// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { ImageItem } from '@editor/types/timeline'
import type { ItemKeyframes } from '@editor/types/keyframe'
import type { TimelineItem, TextItem, ShapeItem } from '@editor/types/timeline'
import type { ResolvedTransform } from '@editor/types/transform'
import { useAutoKeyframeStore } from '@editor/features/keyframes/stores/auto-keyframe-store'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { CanvasTransformInteraction, canvasTransformCommit } from './canvas-transform-interaction'
import { bindCommandExecutor, CommandExecutor } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useGizmoStore } from '@editor/features/preview/stores/gizmo-store'
import { makeTimelineTrack, resetTimelineCompositionTestState } from '@editor/features/timeline/test-helpers'
import { ShortcutProvider } from '../ui/shortcuts'
import { createTransformParentBinding } from '@editor/shared/utils/transform-parenting'
import { useEditorStore } from '@editor/shared/state/editor'
import { getTransformAnchor, rotatePoint } from '@editor/features/preview/utils/coordinate-transform'
import { computeItemAabb } from '@editor/features/preview/utils/canvas-snap-utils'
import { resolveAnimatedTextItem } from '@editor/features/keyframes/utils/animated-text-item'
import { resolveItemTransformAtFrame } from '@editor/runtime/composition-runtime/utils/frame-scene'

const item: ImageItem = {
  id: 'canvas-image', type: 'image', trackId: 'visual', label: 'Image', src: '/image.png',
  from: 10, durationInFrames: 100, sourceWidth: 1920, sourceHeight: 1080,
}
const initial: ResolvedTransform = {
  x: 0, y: 0, width: 400, height: 200, anchorX: 200, anchorY: 100,
  rotation: 0, opacity: 0.7, cornerRadius: 0,
}

beforeEach(() => {
  useAutoKeyframeStore.getState().reset()
  useTransitionsStore.getState().setTransitions([])
})

describe('canvas transform command payload', () => {
  it('commits only changed position, never resolved opacity, dimensions or implicit anchors', () => {
    expect(canvasTransformCommit(item, undefined, 20, initial, { ...initial, x: 30, y: -15 })).toEqual({
      transform: { x: 30, y: -15 }, autoKeyframeOperations: [],
    })
  })

  it('does not create history payload for a click without a change', () => {
    expect(canvasTransformCommit(item, undefined, 20, initial, initial)).toEqual({ transform: {}, autoKeyframeOperations: [] })
  })

  it('keeps implicit resize anchors implicit', () => {
    expect(canvasTransformCommit(item, undefined, 20, initial, { ...initial, width: 800, height: 400, anchorX: 400, anchorY: 200 })).toEqual({
      transform: { width: 800, height: 400 }, autoKeyframeOperations: [],
    })
  })

  it('preserves explicitly authored anchors on resize', () => {
    const explicit = { ...item, transform: { anchorX: 100, anchorY: 50 } }
    const start = { ...initial, anchorX: 100, anchorY: 50 }
    expect(canvasTransformCommit(explicit, undefined, 20, start, { ...start, width: 800, height: 400, anchorX: 200, anchorY: 100 }).transform).toEqual({
      width: 800, height: 400, anchorX: 200, anchorY: 100,
    })
  })

  it('updates an existing scalar keyframe at the clip-relative frame without changing the base', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [{ property: 'x', keyframes: [{ id: 'x-key', frame: 10, value: 0, easing: 'linear' }] }] }
    expect(canvasTransformCommit(item, lanes, 20, initial, { ...initial, x: 50 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'update', itemId: item.id, property: 'x', keyframeId: 'x-key', updates: { value: 50 } }],
    })
  })

  it('extends an existing scalar lane rather than baking animation into the base transform', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [{ property: 'rotation', keyframes: [{ id: 'rotation-key', frame: 0, value: 0, easing: 'linear' }] }] }
    expect(canvasTransformCommit(item, lanes, 30, initial, { ...initial, rotation: 45 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'add', itemId: item.id, property: 'rotation', frame: 20, value: 45, easing: 'linear' }],
    })
  })

  it('updates coupled position keyframes atomically', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [], vectorProperties: [{ property: 'position', keyframes: [{ id: 'position-key', frame: 10, value: { x: 0, y: 0 }, easing: 'linear' }] }] }
    expect(canvasTransformCommit(item, lanes, 20, initial, { ...initial, x: 50, y: 25 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'vector-update', itemId: item.id, property: 'position', keyframeId: 'position-key', updates: { value: { x: 50, y: 25 } } }],
    })
  })

  it('keeps existing scalar width lanes in pixels rather than converting them to percentages', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [{ property: 'width', keyframes: [{ id: 'width-key', frame: 10, value: 400, easing: 'linear' }] }] }
    expect(canvasTransformCommit(item, lanes, 20, initial, { ...initial, width: 600 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'update', itemId: item.id, property: 'width', keyframeId: 'width-key', updates: { value: 600 } }],
    })
  })

  it('uses the existing auto-key toggle to start a coupled scale lane', () => {
    useAutoKeyframeStore.getState().setAutoKeyframeEnabled(item.id, 'width', true)
    expect(canvasTransformCommit(item, undefined, 20, initial, { ...initial, width: 800, height: 400, anchorX: 400, anchorY: 200 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'vector-add', itemId: item.id, property: 'scale', frame: 10, value: { x: 200, y: 200 }, easing: 'linear' }],
    })
  })

  it('does not bake motion-layer and procedural offsets into the editable transform', () => {
    const presented = { ...initial, x: 12, y: -7, rotation: 5 }
    expect(canvasTransformCommit(item, undefined, 20, presented, { ...presented, x: 42, rotation: 20 }, initial)).toEqual({
      transform: { x: 30, rotation: 15 }, autoKeyframeOperations: [],
    })
  })
})

describe('driven-property command ownership', () => {
  it('rejects expression-owned resize without changing base data or lanes', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [], expressions: [{ type: 'expression', targetProperty: 'width', source: 'preValue * 2', enabled: true }] }
    const before = structuredClone({ item, lanes })
    expect(canvasTransformCommit(item, lanes, 20, initial, { ...initial, width: 800 })).toEqual({
      transform: {}, autoKeyframeOperations: [], blockedProperties: ['width'],
    })
    expect({ item, lanes }).toEqual(before)
  })

  it('keeps unaffected dimensions editable when a scalar axis is linked', () => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [], propertyLinks: [{ type: 'link', targetProperty: 'x', sourceItemId: 'driver', sourceProperty: 'x', enabled: true, timeOffsetFrames: 0 }] }
    expect(canvasTransformCommit(item, lanes, 20, initial, { ...initial, width: 800, height: 400, anchorX: 400, anchorY: 200 })).toEqual({
      transform: { width: 800, height: 400 }, autoKeyframeOperations: [],
    })
  })
})

function pointer(target: Element, type: string, clientX: number, clientY: number, options: MouseEventInit = {}) {
  const event = new MouseEvent(type, { bubbles: true, button: 0, clientX, clientY, altKey: true, ...options })
  Object.defineProperty(event, 'pointerId', { value: 1 })
  fireEvent(target, event)
}

describe('canvas gesture consumer and history', () => {
  let executor: CommandExecutor
  let unbind: () => void
  beforeEach(() => {
    resetTimelineCompositionTestState()
    useEditorStore.setState({ linkedSelectionEnabled: true })
    executor = new CommandExecutor()
    unbind = bindCommandExecutor(executor)
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'visual', name: 'Visual', order: 0 })])
    useItemsStore.getState().setItems([{ ...item, transform: { width: 400, height: 200 } }])
    // Exercise the canonical materialized transform shape, not a partial fixture.
    useItemsStore.getState()._updateItemTransform(item.id, {})
    useSelectionStore.getState().selectItems([item.id])
    usePlaybackStore.setState({ currentFrame: 20, previewFrame: null, isPlaying: false })
    useGizmoStore.getState().clearPreview()
    useGizmoStore.getState().cancelInteraction()
  })
  afterEach(() => {
    cleanup()
    unbind()
    resetTimelineCompositionTestState()
    useSelectionStore.getState().clearItemSelection()
    useGizmoStore.getState().clearPreview()
    vi.restoreAllMocks()
  })

  function mount(scale = 0.5) {
    const view = render(<ShortcutProvider><div data-editor-root data-shortcut-scope="preview"><CanvasTransformInteraction width={1920} height={1080} fps={30} /></div></ShortcutProvider>)
    const root = view.container.querySelector<HTMLElement>('[data-editor-root]')!
    const rect = new DOMRect(0, 0, 1000, 800)
    // jsdom has no layout; satisfy the real provider's visible-editor guard.
    vi.spyOn(root, 'getClientRects').mockReturnValue(Object.assign([rect], { item: (index: number) => index === 0 ? rect : null }))
    const surface = screen.getByRole('group', { name: 'Biến đổi clip' })
    vi.spyOn(surface, 'getBoundingClientRect').mockReturnValue({ left: 0, top: 0, width: 1920 * scale, height: 1080 * scale, right: 1920 * scale, bottom: 1080 * scale, x: 0, y: 0, toJSON: () => ({}) })
    Object.assign(surface, { setPointerCapture: vi.fn(), hasPointerCapture: () => false, releasePointerCapture: vi.fn() })
    return surface
  }

  it.each([0.25, 0.5, 1])('resizes through a real handle at %s fit scale and creates exactly one undo entry', scale => {
    const surface = mount(scale)
    const before = structuredClone(useItemsStore.getState().items[0].transform)
    const handle = screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' })
    pointer(handle, 'pointerdown', 1160 * scale, 640 * scale)
    pointer(surface, 'pointermove', 1260 * scale, 690 * scale)
    expect(useItemsStore.getState().items[0].transform?.width).toBe(400)
    pointer(surface, 'pointerup', 1260 * scale, 690 * scale)
    const resized = useItemsStore.getState().items[0].transform
    expect(resized).toEqual({ ...before, width: before!.width! * 1.5, height: before!.height! * 1.5 })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items[0].transform).toEqual(before)
    act(() => { executor.redo() })
    expect(useItemsStore.getState().items[0].transform).toEqual(resized)
  })

  it('cancels without a command or loss of an existing redo entry', () => {
    const surface = mount()
    const before = structuredClone(useItemsStore.getState().items[0].transform)
    const handle = screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' })
    pointer(handle, 'pointerdown', 580, 320)
    pointer(surface, 'pointerup', 630, 345)
    act(() => { executor.undo() })
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (nw)' }), 'pointerdown', 380, 220)
    pointer(surface, 'pointermove', 330, 195)
    fireEvent.keyDown(surface, { key: 'Escape', code: 'Escape' })
    expect(useGizmoStore.getState().activeGizmo).toBeNull()
    pointer(surface, 'pointerup', 330, 195)
    expect(useItemsStore.getState().items[0].transform).toEqual(before)
    expect(executor.canRedo).toBe(true)
    expect(useGizmoStore.getState().preview).toBeNull()
  })

  it('does not dispatch or change history when an expression owns the resized dimension', () => {
    useKeyframesStore.getState().setKeyframes([{ itemId: item.id, properties: [], expressions: [{ type: 'expression', targetProperty: 'width', source: 'preValue * 2', enabled: true }] }])
    const surface = mount()
    const before = structuredClone({ items: useItemsStore.getState().items, lanes: useKeyframesStore.getState().keyframes })
    const handle = screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' })
    expect(handle).toHaveAttribute('aria-disabled', 'true')
    expect(handle.title).toContain('Tắt biểu thức')
    pointer(handle, 'pointerdown', 680, 320)
    pointer(surface, 'pointerup', 730, 345)
    expect({ items: useItemsStore.getState().items, lanes: useKeyframesStore.getState().keyframes }).toEqual(before)
    expect(executor.canUndo).toBe(false)
    expect(useGizmoStore.getState().activeGizmo).toBeNull()
  })

  function seedGroup() {
    const common = { trackId: 'visual', from: 10, durationInFrames: 100, linkedGroupId: 'subscribe-group' }
    const backdrop: ShapeItem = { ...common, id: 'backdrop', label: 'Nền nút Subscribe', type: 'shape', shapeType: 'rectangle', fillColor: '#ff0000', cornerRadius: 12, strokeWidth: 4, strokeColor: '#ffffff', strokeEnabled: true, transform: { x: 0, y: 0, width: 400, height: 200 } }
    const text: TextItem = { ...common, id: 'label', label: 'Subscribe', type: 'text', text: 'Nhấn Subscribe', color: '#ffffff', fontSize: 24, textPadding: 8, lineHeight: 1.2, letterSpacing: 2, textShadow: { offsetX: 4, offsetY: -2, blur: 6, color: '#123456' }, stroke: { width: 2, color: '#abcdef' }, textSpans: [{ text: 'Nhấn ', fontSize: 24, color: '#ffffff', fontWeight: 'bold', letterSpacing: 1 }, { text: 'Subscribe', fontSize: 12, color: '#ffdd00', fontStyle: 'italic' }], transform: { x: 40, y: 10, width: 140, height: 60 } }
    const icon: TimelineItem = { ...item, ...common, id: 'icon', label: 'Icon', transform: { x: -100, y: 0, width: 80, height: 80, rotation: 30, anchorX: 20, anchorY: 25 } }
    useItemsStore.getState().setItems([backdrop, text, icon])
    for (const member of [backdrop, text, icon]) useItemsStore.getState()._updateItemTransform(member.id, {})
    useSelectionStore.getState().selectItems(['backdrop', 'label', 'icon'])
  }

  function rendered(id: string, preview = false) {
    const store = useItemsStore.getState()
    const lanes = useKeyframesStore.getState().keyframes
    const getKeyframes = (id: string) => lanes.find(entry => entry.itemId === id)
    return resolveItemTransformAtFrame(store.itemById[id], { canvas: { width: 1920, height: 1080, fps: 30 }, frame: 20, keyframes: getKeyframes(id), getItem: id => store.itemById[id], getKeyframes, getPreviewTransform: preview ? id => useGizmoStore.getState().preview?.[id]?.transform : undefined })
  }

  function corner(transform: ResolvedTransform, x: number, y: number) {
    return rotatePoint({ x: 960 + transform.x + (x - 0.5) * transform.width, y: 540 + transform.y + (y - 0.5) * transform.height }, getTransformAnchor(transform, 1920, 1080), transform.rotation)
  }

  it.each([0.25, 0.5, 1])('resizes a linked text/icon/backdrop visually at %s fit without collapsing selection', scale => {
    seedGroup()
    const before = structuredClone(useItemsStore.getState().items)
    const initial = new Map(before.map(member => [member.id, rendered(member.id)]))
    const surface = mount(scale)
    expect(screen.getAllByRole('button', { name: /Đổi tỉ lệ clip/u })).toHaveLength(8)
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 1160 * scale, 640 * scale)
    pointer(surface, 'pointermove', 1260 * scale, 690 * scale)
    expect(useItemsStore.getState().items).toEqual(before)
    const preview = new Map(before.map(member => [member.id, rendered(member.id, true)]))
    const properties = structuredClone(useGizmoStore.getState().preview?.label.properties)
    expect(properties?.fontSize).toBeCloseTo(36)
    expect(properties?.textPadding).toBeCloseTo(12)
    expect(useGizmoStore.getState().activeGizmo?.itemId).not.toBe('backdrop')
    pointer(surface, 'pointerup', 1260 * scale, 690 * scale)
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['backdrop', 'label', 'icon'])
    for (const member of before) {
      const after = rendered(member.id)
      for (const property of Object.keys(after) as (keyof ResolvedTransform)[]) expect(after[property]).toBeCloseTo(preview.get(member.id)![property], 8)
      for (const [x, y] of [[0, 0], [1, 0], [0, 1]]) {
        const start = corner(initial.get(member.id)!, x, y)
        const end = corner(after, x, y)
        expect(end.x).toBeCloseTo(960 + (start.x - 960) * 1.5, 8)
        expect(end.y).toBeCloseTo(540 + (start.y - 540) * 1.5, 8)
      }
    }
    const text = useItemsStore.getState().itemById.label as TextItem
    expect(text.fontSize).toBe(properties?.fontSize)
    expect(text.text).toBe((before[1] as TextItem).text)
    expect(text.textSpans).toEqual((before[1] as TextItem).textSpans?.map(span => ({ ...span, fontSize: expect.closeTo(span.fontSize! * 1.5), ...(span.letterSpacing === undefined ? {} : { letterSpacing: expect.closeTo(span.letterSpacing * 1.5) }) })))
    expect(text.textShadow).toEqual({ offsetX: expect.closeTo(6), offsetY: expect.closeTo(-3), blur: expect.closeTo(9), color: '#123456' })
    expect(text.stroke).toEqual({ width: expect.closeTo(3), color: '#abcdef' })
    expect(text.lineHeight).toBe(1.2)
    expect(useItemsStore.getState().itemById.backdrop).toMatchObject({ strokeWidth: expect.closeTo(6), cornerRadius: expect.closeTo(18) })
    const resized = structuredClone(useItemsStore.getState().items)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
    act(() => { executor.redo() })
    expect(useItemsStore.getState().items).toEqual(resized)
  })

  it('expands a canvas hit to its linked group and moves every member without changing sizes or selection', () => {
    seedGroup()
    useSelectionStore.getState().clearItemSelection()
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    pointer(surface, 'pointerdown', 480, 270)
    expect(new Set(useSelectionStore.getState().selectedItemIds)).toEqual(new Set(['backdrop', 'label', 'icon']))
    pointer(surface, 'pointerup', 490, 285)
    for (const member of before) expect(useItemsStore.getState().itemById[member.id]).toEqual({ ...member, transform: { ...member.transform, x: member.transform!.x! + 20, y: member.transform!.y! + 30 } })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('only presents group handles when at least one selected member is visible at the playhead', () => {
    seedGroup()
    usePlaybackStore.setState({ currentFrame: 0 })
    mount()
    expect(screen.queryAllByRole('button', { name: /Đổi tỉ lệ clip/u })).toHaveLength(0)
    act(() => { usePlaybackStore.getState().finishScrub(20) })
    expect(screen.getAllByRole('button', { name: /Đổi tỉ lệ clip/u })).toHaveLength(8)
  })

  it('bounds uniform shrink at the smallest glyph rather than deforming or dropping a member', () => {
    seedGroup()
    const text = useItemsStore.getState().itemById.label as TextItem
    useItemsStore.getState()._updateItem('label', { fontSize: 4, textSpans: text.textSpans?.map(span => ({ ...span, fontSize: 2 })) })
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointerup', 480, 270)
    const resized = useItemsStore.getState().itemById.label as TextItem
    expect(resized.fontSize).toBe(2)
    expect(resized.textSpans?.map(span => span.fontSize)).toEqual([1, 1])
    expect(rendered('backdrop')).toMatchObject({ width: 200, height: 100 })
    expect(rendered('icon')).toMatchObject({ width: 40, height: 40 })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
  })

  it('removes live glyph overrides when a group resize returns to its starting geometry', () => {
    seedGroup()
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointermove', 630, 345)
    expect(useGizmoStore.getState().preview?.label.properties?.fontSize).toBeCloseTo(36)
    pointer(surface, 'pointermove', 580, 320)
    expect(useGizmoStore.getState().preview?.label.properties).toBeUndefined()
    pointer(surface, 'pointerup', 580, 320)
    expect(useItemsStore.getState().items).toEqual(before)
    expect(executor.canUndo).toBe(false)
    expect(useGizmoStore.getState().preview).toBeNull()
  })

  it('keeps opposite-corner group resize fixed and all member proportions uniform', () => {
    seedGroup()
    const before = new Map(useItemsStore.getState().items.map(member => [member.id, rendered(member.id)]))
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320, { ctrlKey: true })
    pointer(surface, 'pointerup', 680, 370, { ctrlKey: true })
    expect(corner(rendered('backdrop'), 0, 0)).toEqual(corner(before.get('backdrop')!, 0, 0))
    for (const [id, initial] of before) {
      expect(rendered(id).width).toBeCloseTo(initial.width * 1.5)
      expect(rendered(id).height).toBeCloseTo(initial.height * 1.5)
    }
    expect((useItemsStore.getState().itemById.label as TextItem).fontSize).toBeCloseTo(36)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
  })

  it('keeps uniform group scaling even with Shift on an edge handle', () => {
    seedGroup()
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (e)' }), 'pointerdown', 580, 270, { shiftKey: true })
    pointer(surface, 'pointerup', 630, 270, { shiftKey: true })
    expect(rendered('backdrop')).toMatchObject({ width: 600, height: 300, x: 0, y: 0 })
    expect(rendered('icon').width / rendered('icon').height).toBeCloseTo(1)
  })

  it('nudges the whole selected group with the real shortcut provider', () => {
    seedGroup()
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    fireEvent.keyDown(surface, { key: 'ArrowRight', code: 'ArrowRight', ctrlKey: true, shiftKey: true })
    for (const member of before) expect(useItemsStore.getState().itemById[member.id]).toEqual({ ...member, transform: { ...member.transform, x: member.transform!.x! + 10 } })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('retains existing single-text box resize semantics without scaling glyph properties', () => {
    seedGroup()
    useEditorStore.setState({ linkedSelectionEnabled: false })
    useSelectionStore.getState().selectItems(['label'])
    const before = structuredClone(useItemsStore.getState().itemById.label) as TextItem
    const surface = mount()
    const initial = rendered('label')
    const start = { x: (960 + initial.x + initial.width / 2) / 2, y: (540 + initial.y + initial.height / 2) / 2 }
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', start.x, start.y)
    pointer(surface, 'pointerup', start.x + initial.width / 4, start.y + initial.height / 4)
    const after = useItemsStore.getState().itemById.label as TextItem
    expect(after.fontSize).toBe(before.fontSize)
    expect(after.textSpans).toEqual(before.textSpans)
    expect(after.textShadow).toEqual(before.textShadow)
    expect(rendered('label').width).toBeCloseTo(initial.width * 2)
  })

  it('rotates member centers and existing rotations about one shared group center', () => {
    seedGroup()
    const before = structuredClone(useItemsStore.getState().items)
    const initial = new Map(before.map(member => [member.id, rendered(member.id)]))
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Xoay clip' }), 'pointerdown', 505, 233)
    pointer(surface, 'pointerup', 517, 295)
    for (const member of before) {
      const after = rendered(member.id)
      expect(after.rotation).toBeCloseTo(initial.get(member.id)!.rotation + 90)
      for (const [x, y] of [[0, 0], [1, 0], [0, 1]]) {
        const start = corner(initial.get(member.id)!, x, y)
        const expected = rotatePoint(start, { x: 960, y: 540 }, 90)
        const end = corner(after, x, y)
        expect(end.x).toBeCloseTo(expected.x, 8)
        expect(end.y).toBeCloseTo(expected.y, 8)
      }
    }
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('cancels every member and visual property with Escape while preserving an existing group redo', () => {
    seedGroup()
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointerup', 630, 345)
    const resized = structuredClone(useItemsStore.getState().items)
    act(() => { executor.undo() })
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointermove', 630, 345)
    fireEvent.keyDown(surface, { key: 'Escape', code: 'Escape' })
    pointer(surface, 'pointerup', 630, 345)
    expect(useItemsStore.getState().items).toEqual(before)
    expect(useGizmoStore.getState().preview).toBeNull()
    expect(executor.canUndo).toBe(false)
    expect(executor.canRedo).toBe(true)
    act(() => { executor.redo() })
    expect(useItemsStore.getState().items).toEqual(resized)
  })

  it('updates mixed geometric and font keyframes atomically while retaining their bases', () => {
    seedGroup()
    useItemsStore.getState()._updateItem('label', { words: [{ text: 'Nhấn', startFrame: 0, endFrame: 20 }, { text: 'Subscribe', startFrame: 20, endFrame: 60 }], wordsEstimated: false })
    const lanes: ItemKeyframes[] = [
      { itemId: 'backdrop', properties: [{ property: 'width', keyframes: [{ id: 'w', frame: 10, value: 400, easing: 'linear' }] }] },
      { itemId: 'label', properties: [{ property: 'fontSize', keyframes: [{ id: 'font', frame: 10, value: 30, easing: 'linear' }] }], vectorProperties: [{ property: 'position', keyframes: [{ id: 'p', frame: 10, value: { x: 40, y: 10 }, easing: 'linear' }] }] },
      { itemId: 'icon', properties: [], vectorProperties: [{ property: 'scale', keyframes: [{ id: 's', frame: 10, value: { x: 125, y: 125 }, easing: 'linear' }] }] },
    ]
    useKeyframesStore.getState().setKeyframes(lanes)
    const before = structuredClone(useItemsStore.getState().items)
    const initial = new Map(before.map(member => [member.id, rendered(member.id)]))
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointermove', 630, 345)
    expect(useGizmoStore.getState().preview?.label.properties?.fontSize).toBeCloseTo(45)
    pointer(surface, 'pointerup', 630, 345)
    for (const member of before) {
      expect(rendered(member.id).width).toBeCloseTo(initial.get(member.id)!.width * 1.5)
      expect(rendered(member.id).height).toBeCloseTo(initial.get(member.id)!.height * 1.5)
    }
    const current = useKeyframesStore.getState().keyframes
    expect(current.find(entry => entry.itemId === 'backdrop')?.properties[0].keyframes[0].value).toBeCloseTo(600)
    expect(current.find(entry => entry.itemId === 'icon')?.vectorProperties?.[0].keyframes[0].value).toEqual({ x: expect.closeTo(187.5), y: expect.closeTo(187.5) })
    const text = useItemsStore.getState().itemById.label as TextItem
    expect(text.fontSize).toBe((before[1] as TextItem).fontSize)
    expect(text.words).toEqual((before[1] as TextItem).words)
    expect(text.wordsEstimated).toBe((before[1] as TextItem).wordsEstimated)
    expect(resolveAnimatedTextItem(text, current.find(entry => entry.itemId === 'label'), 10, { width: 1920, height: 1080, fps: 30 }).fontSize).toBeCloseTo(45)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
    expect(useKeyframesStore.getState().keyframes).toEqual(lanes)
  })

  it('scales an existing animated text preset through its own lane with matching live/final typography', () => {
    seedGroup()
    useItemsStore.getState()._updateItem('label', { textStylePresetId: 'headline-stack', textStyleScale: 1 })
    const lanes: ItemKeyframes[] = [{ itemId: 'label', properties: [{ property: 'textStyleScale', keyframes: [{ id: 'style', frame: 10, value: 1, easing: 'linear' }] }] }]
    useKeyframesStore.getState().setKeyframes(lanes)
    const before = structuredClone(useItemsStore.getState().items)
    const boxes = before.map(member => computeItemAabb(rendered(member.id), 1920, 1080))
    const left = Math.min(...boxes.map(box => box.left)), right = Math.max(...boxes.map(box => box.right))
    const top = Math.min(...boxes.map(box => box.top)), bottom = Math.max(...boxes.map(box => box.bottom))
    const center = { x: (left + right) / 2, y: (top + bottom) / 2 }
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', right / 2, bottom / 2)
    const end = { x: (center.x + (right - center.x) * 1.5) / 2, y: (center.y + (bottom - center.y) * 1.5) / 2 }
    pointer(surface, 'pointermove', end.x, end.y)
    const properties = structuredClone(useGizmoStore.getState().preview?.label.properties)
    pointer(surface, 'pointerup', end.x, end.y)
    const text = useItemsStore.getState().itemById.label as TextItem
    const resolved = resolveAnimatedTextItem(text, useKeyframesStore.getState().keyframes.find(entry => entry.itemId === 'label'), 10, { width: 1920, height: 1080, fps: 30 })
    expect(resolved.textStyleScale).toBeCloseTo(1.5)
    for (const property of ['fontSize', 'textPadding', 'backgroundRadius', 'textShadow', 'stroke', 'textSpans', 'letterSpacing'] as const) {
      if (properties && property in properties) expect(resolved[property]).toEqual(properties[property])
    }
    expect(text.text).toBe((before[1] as TextItem).text)
    expect(text.textSpans).toEqual((before[1] as TextItem).textSpans)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
    expect(useKeyframesStore.getState().keyframes).toEqual(lanes)
  })

  it('keeps every member and history untouched when a driven width prevents visual group scale', () => {
    seedGroup()
    const lanes: ItemKeyframes[] = [{ itemId: 'icon', properties: [], expressions: [{ type: 'expression', targetProperty: 'width', source: 'preValue * 2', enabled: true }] }]
    useKeyframesStore.getState().setKeyframes(lanes)
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    const handle = screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' })
    expect(handle).toHaveAttribute('aria-disabled', 'true')
    pointer(handle, 'pointerdown', 580, 320)
    pointer(surface, 'pointerup', 630, 345)
    expect(useItemsStore.getState().items).toEqual(before)
    expect(useKeyframesStore.getState().keyframes).toEqual(lanes)
    expect(executor.canUndo).toBe(false)
  })

  it('rejects an expression-conflicting group gesture without moving unowned members', () => {
    seedGroup()
    const lanes: ItemKeyframes[] = [{ itemId: 'label', properties: [], expressions: [{ type: 'expression', targetProperty: 'x', source: '42', enabled: true }] }]
    useKeyframesStore.getState().setKeyframes(lanes)
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Di chuyển clip' }), 'pointerdown', 480, 270)
    pointer(surface, 'pointermove', 490, 285)
    pointer(surface, 'pointerup', 490, 285)
    expect(useItemsStore.getState().items).toEqual(before)
    expect(useKeyframesStore.getState().keyframes).toEqual(lanes)
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['backdrop', 'label', 'icon'])
    expect(useGizmoStore.getState().preview).toBeNull()
    expect(executor.canUndo).toBe(false)
  })

  it('does not double-scale a selected child whose selected parent supplies the same group transform', () => {
    const parent = { ...item, id: 'parent', linkedGroupId: 'nested', transform: { x: 0, y: 0, width: 400, height: 200 } }
    const childLocal = { ...initial, x: 100, width: 80, height: 40, anchorX: 40, anchorY: 20 }
    const child = { ...item, id: 'child', linkedGroupId: 'nested', transform: { x: 100, y: 0, width: 80, height: 40 }, transformParent: createTransformParentBinding({ childLocal, childWorld: childLocal, parentItemId: parent.id, parentWorld: initial }) }
    useItemsStore.getState().setItems([parent, child])
    for (const id of ['parent', 'child']) useItemsStore.getState()._updateItemTransform(id, {})
    useSelectionStore.getState().selectItems(['parent', 'child'])
    const before = structuredClone(useItemsStore.getState().items)
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', 580, 320)
    pointer(surface, 'pointerup', 630, 345)
    expect(rendered('parent').width).toBeCloseTo(600)
    expect(rendered('child').width).toBeCloseTo(120)
    expect(rendered('child').x).toBeCloseTo(150)
    expect(useItemsStore.getState().itemById.child).toEqual(before[1])
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useItemsStore.getState().items).toEqual(before)
  })

  it('converts a world-space move back into a rotated parent local space without editing the parent', () => {
    const parent = { ...item, id: 'parent', transform: { width: 400, height: 200, rotation: 90 } }
    const local = { ...initial, x: 100, width: 100, height: 50, anchorX: 50, anchorY: 25 }
    const child = { ...item, transform: { x: 100, y: 0, width: 100, height: 50 }, transformParent: createTransformParentBinding({ childLocal: local, childWorld: local, parentItemId: parent.id, parentWorld: initial }) }
    useItemsStore.getState().setItems([parent, child])
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Di chuyển clip' }), 'pointerdown', 480, 320)
    pointer(surface, 'pointerup', 490, 320)
    expect(useItemsStore.getState().itemById[parent.id]).toEqual(parent)
    expect(useItemsStore.getState().itemById[item.id].transform?.x).toBeCloseTo(100)
    expect(useItemsStore.getState().itemById[item.id].transform?.y).toBeCloseTo(-20)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
  })

  it('never saves unrelated fractional dimensions while moving with snapping bypassed', () => {
    useItemsStore.getState().setItems([{ ...item, transform: { width: 400.25, height: 200.75, rotation: 12.5 } }])
    const surface = mount()
    pointer(screen.getByRole('button', { name: 'Di chuyển clip' }), 'pointerdown', 480, 270)
    pointer(surface, 'pointerup', 490, 285)
    expect(useItemsStore.getState().items[0].transform).toMatchObject({ x: 20, y: 30, width: 400.25, height: 200.75, rotation: 12.5 })
  })

  it('allows vertical motion while an expression owns only horizontal motion', () => {
    useKeyframesStore.getState().setKeyframes([{ itemId: item.id, properties: [], expressions: [{ type: 'expression', targetProperty: 'x', source: '42', enabled: true }] }])
    const surface = mount()
    const before = structuredClone(useItemsStore.getState().items[0].transform)
    pointer(screen.getByRole('button', { name: 'Di chuyển clip' }), 'pointerdown', 501, 270)
    pointer(surface, 'pointerup', 511, 285)
    expect(useItemsStore.getState().items[0].transform).toEqual({ ...before, y: 30 })
    expect(useKeyframesStore.getState().keyframes[0].expressions?.[0]).toMatchObject({ source: '42', enabled: true })
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
  })

  it.each([100, 150])('keeps %s%% vector-keyframed scale editable in percentage units without baking the base', scale => {
    const lanes: ItemKeyframes = { itemId: item.id, properties: [], vectorProperties: [{ property: 'scale', keyframes: [{ id: 'scale-key', frame: 10, value: { x: scale, y: scale }, easing: 'linear' }] }] }
    useKeyframesStore.getState().setKeyframes([lanes])
    const before = structuredClone(useItemsStore.getState().items[0])
    const canvas = { width: 1920, height: 1080, fps: 30 }
    const rendered = resolveItemTransformAtFrame(before, { canvas, frame: 20, keyframes: lanes })
    const surface = mount()
    const start = { x: (canvas.width / 2 + rendered.x + rendered.width / 2) * 0.5, y: (canvas.height / 2 + rendered.y + rendered.height / 2) * 0.5 }
    const end = { x: start.x + rendered.width * 0.125, y: start.y + rendered.height * 0.125 }
    pointer(screen.getByRole('button', { name: 'Đổi tỉ lệ clip (se)' }), 'pointerdown', start.x, start.y)
    pointer(surface, 'pointerup', end.x, end.y)
    const after = useItemsStore.getState().items[0]
    const currentLanes = useKeyframesStore.getState().keyframes[0]
    expect(after.transform).toEqual(before.transform)
    expect(currentLanes.vectorProperties?.[0].keyframes[0].value).toEqual({ x: scale * 1.5, y: scale * 1.5 })
    const resized = resolveItemTransformAtFrame(after, { canvas, frame: 20, keyframes: currentLanes })
    expect(resized.width).toBeCloseTo(rendered.width * 1.5)
    expect(resized.height).toBeCloseTo(rendered.height * 1.5)
    expect(executor.history.stacks(executor.contextKey).undo).toHaveLength(1)
    act(() => { executor.undo() })
    expect(useKeyframesStore.getState().keyframes).toEqual([lanes])
    act(() => { executor.redo() })
    expect(useKeyframesStore.getState().keyframes[0]).toEqual(currentLanes)
  })
})
