// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'
import type { ImageItem } from '@editor/types/timeline'
import type { ItemKeyframes } from '@editor/types/keyframe'
import type { ResolvedTransform } from '@editor/types/transform'
import { useAutoKeyframeStore } from '@editor/features/keyframes/stores/auto-keyframe-store'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { canvasTransformCommit } from './canvas-transform-interaction'

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

  it('uses the existing auto-key toggle to start a coupled scale lane', () => {
    useAutoKeyframeStore.getState().setAutoKeyframeEnabled(item.id, 'width', true)
    expect(canvasTransformCommit(item, undefined, 20, initial, { ...initial, width: 800, height: 400, anchorX: 400, anchorY: 200 })).toEqual({
      transform: {}, autoKeyframeOperations: [{ type: 'vector-add', itemId: item.id, property: 'scale', frame: 10, value: { x: 800, y: 400 }, easing: 'linear' }],
    })
  })

  it('does not bake motion-layer and procedural offsets into the editable transform', () => {
    const presented = { ...initial, x: 12, y: -7, rotation: 5 }
    expect(canvasTransformCommit(item, undefined, 20, presented, { ...presented, x: 42, rotation: 20 }, initial)).toEqual({
      transform: { x: 30, rotation: 15 }, autoKeyframeOperations: [],
    })
  })
})
