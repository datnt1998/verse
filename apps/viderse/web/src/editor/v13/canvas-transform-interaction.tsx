import { useEffect, useRef, type PointerEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Move, Maximize2, RotateCw } from 'lucide-react'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useGizmoStore, type ItemPreview } from '@editor/features/preview/stores/gizmo-store'
import { prepareScaleStartTransform } from '@editor/features/preview/utils/transform-calculations'
import { getTransformAnchor, rotatePoint, screenToCanvas } from '@editor/features/preview/utils/coordinate-transform'
import { resolveItemTransformAtFrame } from '@editor/runtime/composition-runtime/utils/frame-scene'
import { resolveTransform, getSourceDimensions } from '@editor/runtime/composition-runtime/utils/transform-resolver'
import { resolveAnimatedTransform } from '@editor/features/keyframes/utils/animated-transform-resolver'
import { worldToLocalTransform } from '@editor/shared/utils/transform-parenting'
import { getAutoKeyframeOperation, getVectorAutoKeyframeOperation, type AutoKeyframeOperation } from '@editor/features/keyframes/utils/auto-keyframe'
import { updateItemTransform } from '@editor/features/timeline/stores/timeline-actions'
import { withCommandMergeKey } from '@editor/commands'
import type { TimelineItem } from '@editor/types/timeline'
import type { ResolvedTransform, TransformProperties } from '@editor/types/transform'
import type { Transform } from '@editor/features/preview/types/gizmo'
import type { ItemKeyframes } from '@editor/types/keyframe'
import { useShortcutAction } from '../ui/shortcuts'
import { announce } from '../ui/shell/announcer'
import { toast } from '../ui/toast'
import { previewStyles as s } from './preview.stylex'

type Mode = 'translate' | 'scale' | 'rotate'
type Gesture = {
  item: TimelineItem
  frame: number
  pointerId: number
  interactionId: number
  mode: Mode
  initial: ResolvedTransform
  local: ResolvedTransform
  parent?: ResolvedTransform
  previousPreview: ItemPreview | null
  moved: boolean
}

/** Build one atomic transform/keyframe command, without baking unrelated animated values. */
export function canvasTransformCommit(item: TimelineItem, keyframes: ItemKeyframes | undefined, frame: number, initial: ResolvedTransform, current: ResolvedTransform, editable: ResolvedTransform = initial) {
  const transform: Partial<TransformProperties> = {}
  const autoKeyframeOperations: AutoKeyframeOperation[] = []
  const authored = prepareScaleStartTransform(current, item, keyframes, undefined)
  current = {
    ...current,
    anchorX: authored.anchorX === undefined ? initial.anchorX : current.anchorX,
    anchorY: authored.anchorY === undefined ? initial.anchorY : current.anchorY,
  }
  // Motion layers/modifiers stay live. Apply the gesture delta to the editable
  // animation value, not to its procedurally offset presentation.
  for (const property of ['x', 'y', 'width', 'height', 'anchorX', 'anchorY', 'rotation'] as const) {
    current[property] = editable[property] + current[property] - initial[property]
  }
  initial = editable
  const groups = [
    ['position', 'x', 'y'], ['scale', 'width', 'height'], ['anchor', 'anchorX', 'anchorY'],
  ] as const
  for (const [vector, x, y] of groups) {
    if (Math.abs(current[x] - initial[x]) < 0.000001 && Math.abs(current[y] - initial[y]) < 0.000001) continue
    // Existing scalar lanes remain scalar; coupled lanes and auto-key use the engine's vector helper.
    const scalarLane = keyframes?.properties.some(lane => (lane.property === x || lane.property === y) && lane.keyframes.length > 0)
    const vectorLane = keyframes?.vectorProperties?.some(lane => lane.property === vector && lane.keyframes.length > 0)
    const operation = !scalarLane || vectorLane ? getVectorAutoKeyframeOperation(item, keyframes, vector, { x: current[x], y: current[y] }, frame) : null
    if (operation) autoKeyframeOperations.push(operation)
    else for (const property of [x, y]) {
      if (Math.abs(current[property] - initial[property]) < 0.000001) continue
      const scalar = getAutoKeyframeOperation(item, keyframes, property, current[property], frame)
      if (scalar) autoKeyframeOperations.push(scalar)
      else transform[property] = current[property]
    }
  }
  if (Math.abs(current.rotation - initial.rotation) >= 0.000001) {
    const operation = getAutoKeyframeOperation(item, keyframes, 'rotation', current.rotation, frame)
    if (operation) autoKeyframeOperations.push(operation)
    else transform.rotation = current.rotation
  }
  return { transform, autoKeyframeOperations }
}

/** Interaction-only overlay: the player and export renderer retain ownership of all pixels. */
export function CanvasTransformInteraction({ width, height, fps }: { width: number; height: number; fps: number }) {
  const items = useItemsStore(state => state.items)
  const tracks = useItemsStore(state => state.tracks)
  const keyframes = useKeyframesStore(state => state.keyframes)
  const ids = useSelectionStore(state => state.selectedItemIds)
  const frame = usePlaybackStore(state => state.previewFrame ?? state.currentFrame)
  const playing = usePlaybackStore(state => state.isPlaying)
  const previews = useGizmoStore(state => state.preview)
  const surface = useRef<HTMLDivElement>(null)
  const gesture = useRef<Gesture | null>(null)
  const canvas = { width, height, fps }
  const getItem = (id: string) => items.find(item => item.id === id)
  const getKeyframes = (id: string) => keyframes.find(lanes => lanes.itemId === id)
  const resolve = (item: TimelineItem, preview = true) => resolveItemTransformAtFrame(item, {
    canvas, frame, keyframes: getKeyframes(item.id), getItem, getKeyframes,
    getPreviewTransform: preview ? id => previews?.[id]?.transform : undefined,
  })
  const solo = tracks.some(track => track.solo)
  const visible = items.filter(item => {
    const track = tracks.find(track => track.id === item.trackId)
    return track && (solo ? track.solo : track.visible !== false) && item.type !== 'audio' && item.type !== 'adjustment' && frame >= item.from && frame < item.from + item.durationInFrames
  }).sort((a, b) => (tracks.find(track => track.id === b.trackId)?.order ?? 0) - (tracks.find(track => track.id === a.trackId)?.order ?? 0))
  const selected = visible.find(item => item.id === ids[0])
  const selectedTransform = selected ? resolve(selected) : null
  const locked = (item: TimelineItem) => tracks.find(track => track.id === item.trackId)?.locked === true
  const point = (event: PointerEvent) => {
    const rect = surface.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0 || rect.height <= 0) return null
    return screenToCanvas(event.clientX, event.clientY, { containerRect: rect, playerSize: rect, projectSize: canvas, zoom: -1 })
  }
  const restore = (drag: Gesture) => {
    const store = useGizmoStore.getState()
    if (store.activeGizmo?.interactionId === drag.interactionId) store.cancelInteraction()
    store.replaceItemPreview(drag.item.id, drag.previousPreview)
  }
  const cancel = () => {
    const drag = gesture.current
    if (!drag) return
    gesture.current = null
    restore(drag)
  }
  useEffect(() => {
    // Cancel before another transport/selection owner can commit this gesture at a different frame.
    const stopPlayback = usePlaybackStore.subscribe(state => {
      const drag = gesture.current
      if (drag && (state.isPlaying || (state.previewFrame ?? state.currentFrame) !== drag.frame)) cancel()
    })
    const stopSelection = useSelectionStore.subscribe(state => {
      if (gesture.current && state.selectedItemIds[0] !== gesture.current.item.id) cancel()
    })
    const stopItems = useItemsStore.subscribe(state => {
      const drag = gesture.current
      if (drag && (state.itemById[drag.item.id] !== drag.item || state.tracks.find(track => track.id === drag.item.trackId)?.locked)) cancel()
    })
    const blur = () => cancel()
    window.addEventListener('blur', blur)
    return () => { stopPlayback(); stopSelection(); stopItems(); window.removeEventListener('blur', blur); cancel() }
  }, [])
  const start = (event: PointerEvent, item: TimelineItem, mode: Mode) => {
    if (event.button !== 0 || playing || locked(item) || gesture.current) return
    const location = point(event)
    if (!location) return
    event.preventDefault(); event.stopPropagation()
    surface.current?.focus({ preventScroll: true })
    useSelectionStore.getState().selectItems([item.id])
    const initial = resolve(item)
    const parentItem = item.transformParent?.parentItemId ? getItem(item.transformParent.parentItemId) : undefined
    const parent = parentItem ? resolve(parentItem) : undefined
    const local = worldToLocalTransform(initial, item.transformParent, parent)
    const store = useGizmoStore.getState()
    if (store.activeGizmo) return
    const rect = surface.current!.getBoundingClientRect()
    store.setCanvasSize(width, height)
    store.setCanvasScale(rect.width / width)
    store.setOtherItemBounds([])
    const stroke = item.type === 'shape' && item.strokeEnabled !== false ? item.strokeWidth : undefined
    const interactionId = mode === 'translate' ? store.startTranslate(item.id, location, initial, stroke, item.type)
      : mode === 'rotate' ? store.startRotate(item.id, location, initial, stroke, item.type)
        : store.startScale(item.id, 'se', location, prepareScaleStartTransform(initial, item, getKeyframes(item.id), previews?.[item.id]?.transform), item.type, item.transform?.aspectRatioLocked, stroke)
    gesture.current = { item, frame, pointerId: event.pointerId, interactionId, mode, initial, local, parent, previousPreview: previews?.[item.id] ?? null, moved: false }
    surface.current?.setPointerCapture(event.pointerId)
  }
  const localPreview = (drag: Gesture, transform: Transform): ResolvedTransform => worldToLocalTransform({
    ...drag.initial, ...transform,
    anchorX: transform.anchorX ?? transform.width / 2,
    anchorY: transform.anchorY ?? transform.height / 2,
    cornerRadius: transform.cornerRadius ?? drag.initial.cornerRadius,
  }, drag.item.transformParent, drag.parent)
  const move = (event: PointerEvent) => {
    const drag = gesture.current
    const location = point(event)
    if (!drag || drag.pointerId !== event.pointerId || !location) return
    const store = useGizmoStore.getState()
    if (store.activeGizmo?.interactionId !== drag.interactionId) { cancel(); return }
    if (!drag.moved && location.x === store.activeGizmo.startPoint.x && location.y === store.activeGizmo.startPoint.y) return
    store.updateInteraction(location, event.shiftKey, event.ctrlKey || event.metaKey, event.altKey)
    const transform = useGizmoStore.getState().previewTransform
    if (!transform) return
    drag.moved = true
    const local = localPreview(drag, transform)
    // Keep implicit anchors implicit so future resizes still follow the box center.
    const authored = prepareScaleStartTransform(local, drag.item, getKeyframes(drag.item.id), drag.previousPreview?.transform)
    store.setTransformPreview({ [drag.item.id]: authored })
  }
  const commit = (item: TimelineItem, initial: ResolvedTransform, current: ResolvedTransform, atFrame: number) => {
    const lanes = getKeyframes(item.id)
    const editable = resolveAnimatedTransform(resolveTransform(item, canvas, getSourceDimensions(item)), lanes, atFrame - item.from, {
      globalFrame: atFrame, canvas, getItem, getKeyframes,
    })
    const { transform, autoKeyframeOperations } = canvasTransformCommit(item, lanes, atFrame, initial, current, editable)
    const result = withCommandMergeKey(null, () => updateItemTransform(item.id, transform, { autoKeyframeOperations }))
    if (!result.ok) { toast.error('Không áp dụng được biến đổi clip'); announce('Không áp dụng được biến đổi clip', 'assertive') }
    return result.ok
  }
  const finish = (event: PointerEvent) => {
    const drag = gesture.current
    if (!drag || drag.pointerId !== event.pointerId) return
    move(event)
    if (gesture.current !== drag) return
    const store = useGizmoStore.getState()
    const transform = store.endInteraction()
    gesture.current = null
    try {
      if (drag.moved && transform) commit(drag.item, drag.local, localPreview(drag, transform), drag.frame)
    } finally { restore(drag) }
    if (surface.current?.hasPointerCapture(event.pointerId)) surface.current.releasePointerCapture(event.pointerId)
  }
  const nudge = (dx: number, dy: number) => {
    if (!selected || !selectedTransform || playing || locked(selected) || gesture.current) return
    const parentItem = selected.transformParent?.parentItemId ? getItem(selected.transformParent.parentItemId) : undefined
    const parent = parentItem ? resolve(parentItem) : undefined
    const initial = worldToLocalTransform(selectedTransform, selected.transformParent, parent)
    const current = worldToLocalTransform({ ...selectedTransform, x: selectedTransform.x + dx, y: selectedTransform.y + dy }, selected.transformParent, parent)
    commit(selected, initial, current, frame)
  }
  useShortcutAction({ id: 'gizmo-left', label: 'Dời hình sang trái', scope: 'preview', bindings: ['ctrl+ArrowLeft', 'ctrl+shift+ArrowLeft'], enabled: !!selected && !playing, run: event => nudge(event.shiftKey ? -10 : -1, 0) })
  useShortcutAction({ id: 'gizmo-right', label: 'Dời hình sang phải', scope: 'preview', bindings: ['ctrl+ArrowRight', 'ctrl+shift+ArrowRight'], enabled: !!selected && !playing, run: event => nudge(event.shiftKey ? 10 : 1, 0) })
  useShortcutAction({ id: 'gizmo-up', label: 'Dời hình lên', scope: 'preview', bindings: ['ctrl+ArrowUp', 'ctrl+shift+ArrowUp'], enabled: !!selected && !playing, run: event => nudge(0, event.shiftKey ? -10 : -1) })
  useShortcutAction({ id: 'gizmo-down', label: 'Dời hình xuống', scope: 'preview', bindings: ['ctrl+ArrowDown', 'ctrl+shift+ArrowDown'], enabled: !!selected && !playing, run: event => nudge(0, event.shiftKey ? 10 : 1) })
  useShortcutAction({ id: 'cancel-gizmo', label: 'Huỷ kéo biến đổi', scope: 'preview', bindings: ['Escape'], run: cancel })
  const bounds = (transform: ResolvedTransform) => ({
    left: `${50 + (transform.x - transform.width / 2) / width * 100}%`, top: `${50 + (transform.y - transform.height / 2) / height * 100}%`,
    width: `${transform.width / width * 100}%`, height: `${transform.height / height * 100}%`,
    transform: `rotate(${transform.rotation}deg)`, transformOrigin: `${transform.anchorX / transform.width * 100}% ${transform.anchorY / transform.height * 100}%`,
  })
  return <div ref={surface} tabIndex={-1} role="group" aria-label="Biến đổi clip" {...stylex.props(s.interaction)} onPointerMove={move} onPointerUp={finish} onPointerCancel={cancel} onLostPointerCapture={cancel} onPointerDown={event => {
    if (event.button !== 0 || playing || gesture.current) return
    const location = point(event)
    if (!location) return
    // Hit-test in the same anchor-rotated canvas space as composition rendering.
    const hit = [...visible].reverse().find(item => {
      const transform = resolve(item)
      const local = rotatePoint(location, getTransformAnchor(transform, width, height), -transform.rotation)
      return Math.abs(local.x - width / 2 - transform.x) <= transform.width / 2 && Math.abs(local.y - height / 2 - transform.y) <= transform.height / 2
    })
    if (hit) { if (locked(hit)) useSelectionStore.getState().selectItems([hit.id]); else start(event, hit, 'translate') }
    else useSelectionStore.getState().selectItems([])
  }}>
    {selected && selectedTransform && !playing && <div {...stylex.props(s.selectionBounds)} style={bounds(selectedTransform)}>
      {!locked(selected) && <>
        <button type="button" aria-label="Di chuyển clip" title="Di chuyển clip (Ctrl+mũi tên; Shift: 10px)" {...stylex.props(s.transformHandle, s.moveHandle)} onPointerDown={event => start(event, selected, 'translate')}><Move size={14} /></button>
        <button type="button" aria-label="Đổi tỉ lệ clip" title="Đổi tỉ lệ clip (Shift: đảo khoá tỉ lệ; Ctrl: neo góc; Alt: bỏ bắt dính)" {...stylex.props(s.transformHandle, s.scaleHandle)} onPointerDown={event => start(event, selected, 'scale')}><Maximize2 size={14} /></button>
        <button type="button" aria-label="Xoay clip" title="Xoay clip (Alt: bỏ bắt dính góc 15°)" {...stylex.props(s.transformHandle, s.rotateHandle)} onPointerDown={event => start(event, selected, 'rotate')}><RotateCw size={14} /></button>
      </>}
    </div>}
  </div>
}
