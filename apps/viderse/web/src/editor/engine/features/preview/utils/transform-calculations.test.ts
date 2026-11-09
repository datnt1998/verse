import { afterEach, describe, expect, it } from 'vitest'
import type { ShapeItem } from '@editor/types/timeline'
import type { ItemKeyframes } from '@editor/types/keyframe'
import type { Transform, GizmoHandle, GizmoState } from '../types/gizmo'
import { useGizmoStore } from '../stores/gizmo-store'
import { calculateTransform, prepareScaleStartTransform } from './transform-calculations'
import { getTransformAnchor, rotatePoint } from './coordinate-transform'

const currentTransform: Transform = {
  x: 0,
  y: 0,
  width: 200,
  height: 100,
  anchorX: 100,
  anchorY: 50,
  rotation: 0,
  opacity: 1,
}

function createShape(transform: ShapeItem['transform']): ShapeItem {
  return {
    id: 'shape-1',
    type: 'shape',
    trackId: 'track-1',
    from: 0,
    durationInFrames: 60,
    label: 'Solid',
    shapeType: 'rectangle',
    fillColor: '#ffffff',
    transform,
  }
}

describe('scale preview anchor semantics', () => {
  afterEach(() => {
    useGizmoStore.getState().cancelInteraction()
  })

  it('keeps an implicit anchor centered throughout preview and commit', () => {
    const start = prepareScaleStartTransform(
      currentTransform,
      createShape({ x: 0, y: 0, width: 200, height: 100 }),
      undefined,
      undefined,
    )

    useGizmoStore.getState().setCanvasSize(1920, 1080)
    useGizmoStore.getState().startScale('shape-1', 'se', { x: 1060, y: 590 }, start, 'shape', false)
    useGizmoStore.getState().updateInteraction({ x: 1160, y: 640 }, true)

    const preview = useGizmoStore.getState().previewTransform!
    expect(preview.anchorX).toBeUndefined()
    expect(preview.anchorY).toBeUndefined()
    expect(preview.anchorX ?? preview.width / 2).toBe(preview.width / 2)
    expect(preview.anchorY ?? preview.height / 2).toBe(preview.height / 2)

    const committed = useGizmoStore.getState().endInteraction()
    expect(committed?.anchorX).toBeUndefined()
    expect(committed?.anchorY).toBeUndefined()
  })

  it('preserves explicit and animated anchor axes', () => {
    const item = createShape({
      x: 0,
      y: 0,
      width: 200,
      height: 100,
      anchorX: 24,
    })
    const keyframes: ItemKeyframes = {
      itemId: item.id,
      properties: [
        {
          property: 'anchorY',
          keyframes: [{ id: 'anchor-y', frame: 0, value: 36, easing: 'linear' }],
        },
      ],
    }

    expect(
      prepareScaleStartTransform(
        { ...currentTransform, anchorX: 24, anchorY: 36 },
        item,
        keyframes,
        undefined,
      ),
    ).toMatchObject({ anchorX: 24, anchorY: 36 })
  })
})

function renderedPoint(transform: Transform, fractionX: number, fractionY: number) {
  return rotatePoint({
    x: 960 + transform.x + (fractionX - 0.5) * transform.width,
    y: 540 + transform.y + (fractionY - 0.5) * transform.height,
  }, getTransformAnchor(transform, 1920, 1080), transform.rotation)
}

describe('rotated opposite-corner scaling', () => {
  for (const rotation of [-45, 45, 90]) for (const handle of ['nw', 'se', 'e', 'n'] as const) for (const explicit of [false, true]) for (const locked of [false, true]) {
    it(`preserves the opposite ${handle} point at ${rotation}°, explicit anchor ${explicit}, aspect lock ${locked}`, () => {
      const start: Transform = { ...currentTransform, x: 24, y: -30, rotation, anchorX: explicit ? 24 : undefined, anchorY: explicit ? 36 : undefined }
      const fractionX = handle.includes('e') ? 1 : handle.includes('w') ? 0 : 0.5
      const fractionY = handle.includes('s') ? 1 : handle.includes('n') ? 0 : 0.5
      const point = renderedPoint(start, fractionX, fractionY)
      const delta = rotatePoint({ x: (fractionX - 0.5) * 80, y: (fractionY - 0.5) * 40 }, { x: 0, y: 0 }, rotation)
      const gizmo: GizmoState = { interactionId: 1, mode: 'scale', activeHandle: handle as GizmoHandle, itemId: 'shape-1', startTransform: start, startPoint: point, currentPoint: point, shiftKey: false, ctrlKey: true, altKey: true }
      const result = calculateTransform(gizmo, { x: point.x + delta.x, y: point.y + delta.y }, !locked, 1920, 1080, true)
      const before = renderedPoint(start, 1 - fractionX, 1 - fractionY)
      const after = renderedPoint(result, 1 - fractionX, 1 - fractionY)
      expect(after.x).toBeCloseTo(before.x, 8)
      expect(after.y).toBeCloseTo(before.y, 8)
      expect(result.width).toBeGreaterThanOrEqual(start.width)
      expect(result.height).toBeGreaterThanOrEqual(start.height)
      expect(result.width * result.height).toBeGreaterThan(start.width * start.height)
      expect(result.anchorX).toBe(start.anchorX)
      expect(result.anchorY).toBe(start.anchorY)
    })
  }

  it('keeps default center scaling position unchanged', () => {
    const start = { ...currentTransform, rotation: 45, anchorX: undefined, anchorY: undefined }
    const point = renderedPoint(start, 1, 1)
    const gizmo: GizmoState = { interactionId: 1, mode: 'scale', activeHandle: 'se', itemId: 'shape-1', startTransform: start, startPoint: point, currentPoint: point, shiftKey: false, ctrlKey: false, altKey: true }
    const result = calculateTransform(gizmo, { x: point.x, y: point.y + 100 }, false, 1920, 1080)
    expect(result.x).toBe(start.x)
    expect(result.y).toBe(start.y)
  })
})

describe('scale snapping ownership', () => {
  afterEach(() => {
    useGizmoStore.getState().cancelInteraction()
  })

  it.each([{ rotation: 0, ctrl: true }, { rotation: 45, ctrl: false }, { rotation: 45, ctrl: true }])('keeps exact scale geometry with no stale guides for $rotation° / corner anchor $ctrl', ({ rotation, ctrl }) => {
    const store = useGizmoStore.getState()
    const start: Transform = { ...currentTransform, x: 12.75, y: -23.5, width: 193.5, height: 97.25, rotation, anchorX: undefined, anchorY: undefined }
    const point = renderedPoint(start, 1, 1)
    const delta = rotatePoint({ x: 17.1, y: 8.55 }, { x: 0, y: 0 }, rotation)
    const current = { x: point.x + delta.x, y: point.y + delta.y }
    store.setCanvasSize(1920, 1080)
    store.setCanvasScale(1)
    store.setSnappingEnabled(true)
    store.startScale('shape-1', 'se', point, start, 'shape', false)
    const expected = calculateTransform(useGizmoStore.getState().activeGizmo!, current, true, 1920, 1080, ctrl)
    useGizmoStore.setState({ snapLines: [{ type: 'vertical', position: 960, label: 'center' }] })
    store.updateInteraction(current, false, ctrl, false)
    expect(useGizmoStore.getState().previewTransform).toEqual(expected)
    expect(useGizmoStore.getState().snapLines).toEqual([])
    if (ctrl) {
      const before = renderedPoint(start, 0, 0)
      const after = renderedPoint(useGizmoStore.getState().previewTransform!, 0, 0)
      expect(after.x).toBeCloseTo(before.x, 8)
      expect(after.y).toBeCloseTo(before.y, 8)
    }
  })

  it('retains valid axis-aligned center scale snapping', () => {
    const store = useGizmoStore.getState()
    const start: Transform = { ...currentTransform, width: 1900, height: 1000, anchorX: undefined, anchorY: undefined }
    store.setCanvasSize(1920, 1080)
    store.setCanvasScale(1)
    store.setSnappingEnabled(true)
    store.startScale('shape-1', 'se', { x: 1910, y: 1040 }, start, 'shape', false)
    store.updateInteraction({ x: 1918, y: 1048 }, false, false, false)
    expect(useGizmoStore.getState().previewTransform?.width).toBe(1920)
    expect(useGizmoStore.getState().snapLines.some(line => line.type === 'vertical')).toBe(true)
  })
})
