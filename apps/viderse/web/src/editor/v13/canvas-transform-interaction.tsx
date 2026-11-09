import { useEffect, useRef, type PointerEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Move, Maximize2, RotateCw } from 'lucide-react'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useGizmoStore, type ItemPreview, type ItemPropertiesPreview } from '@editor/features/preview/stores/gizmo-store'
import { calculateTransform, prepareScaleStartTransform } from '@editor/features/preview/utils/transform-calculations'
import { getTransformAnchor, rotatePoint, screenToCanvas } from '@editor/features/preview/utils/coordinate-transform'
import { resolveItemTransformAtFrame } from '@editor/runtime/composition-runtime/utils/frame-scene'
import { resolveTransform, getSourceDimensions } from '@editor/runtime/composition-runtime/utils/transform-resolver'
import { resolveAnimatedTransform } from '@editor/features/keyframes/utils/animated-transform-resolver'
import { worldToLocalTransform } from '@editor/shared/utils/transform-parenting'
import { getAutoKeyframeOperation, getVectorAutoKeyframeOperation, type AutoKeyframeOperation } from '@editor/features/keyframes/utils/auto-keyframe'
import { resolveAnimatedTextItem, type TextAnimatableProperty } from '@editor/features/keyframes/utils/animated-text-item'
import { resolveTextStyle, TEXT_DEFAULTS } from '@editor/shared/typography/text-style'
import { applyTextStylePresetToItem } from '@editor/shared/typography/text-style-presets'
import { executeCommands, withCommandMergeKey } from '@editor/commands'
import { computeItemAabb, applyGroupScaleSnapping } from '@editor/features/preview/utils/canvas-snap-utils'
import { useEditorStore } from '@editor/shared/state/editor'
import { computeItemSelection } from '../ui/timeline/selection'
import type { TimelineItem, TextItem } from '@editor/types/timeline'
import type { CanvasSettings, ResolvedTransform, TransformProperties } from '@editor/types/transform'
import type { Transform, GizmoHandle, GroupTransformState } from '@editor/features/preview/types/gizmo'
import type { ItemKeyframes } from '@editor/types/keyframe'
import { getDirectPropertyLinks, getPropertyExpressions } from '@editor/types/keyframe'
import { useShortcutAction } from '../ui/shortcuts'
import { announce } from '../ui/shell/announcer'
import { toast } from '../ui/toast'
import { previewStyles as s } from './preview.stylex'

type Mode = 'translate' | 'scale' | 'rotate'
const resizeHandles = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'] as const satisfies readonly GizmoHandle[]
type Member = {
  item: TimelineItem
  initial: ResolvedTransform
  local: ResolvedTransform
  previousPreview: ItemPreview | null
  text?: TextItem
}
type Gesture = {
  members: Member[]
  selectionIds: string[]
  items: TimelineItem[]
  keyframes: ItemKeyframes[]
  frame: number
  pointerId: number
  interactionId: number
  mode: Mode
  initial: ResolvedTransform
  group?: GroupTransformState
  moved: boolean
}

const driverMessage = 'Thuộc tính đang được điều khiển bởi biểu thức hoặc liên kết. Tắt biểu thức hoặc bỏ liên kết để biến đổi thuộc tính này.'
const groupFrameMessage = 'Không thể biến đổi nhóm tại vị trí phát này. Chọn khung hình chỉnh sửa được của tất cả clip.'

export function canvasTransformDrivenProperties(keyframes: ItemKeyframes | undefined): Set<string> {
  const driven = new Set<string>()
  for (const driver of [...getDirectPropertyLinks(keyframes), ...getPropertyExpressions(keyframes)]) {
    if (!driver.enabled) continue
    const properties = driver.targetProperty === 'position' ? ['x', 'y'] : driver.targetProperty === 'scale' ? ['width', 'height'] : driver.targetProperty === 'anchor' ? ['anchorX', 'anchorY'] : [driver.targetProperty]
    for (const property of properties) driven.add(property)
  }
  return driven
}

/** Build one atomic transform/keyframe command, without baking unrelated animated values. */
export function canvasTransformCommit(item: TimelineItem, keyframes: ItemKeyframes | undefined, frame: number, initial: ResolvedTransform, current: ResolvedTransform, editable: ResolvedTransform = initial, base: ResolvedTransform = initial) {
  const transform: Partial<TransformProperties> = {}
  const autoKeyframeOperations: AutoKeyframeOperation[] = []
  const authored = prepareScaleStartTransform(current, item, keyframes, undefined)
  current = {
    ...current,
    anchorX: authored.anchorX === undefined ? initial.anchorX : current.anchorX,
    anchorY: authored.anchorY === undefined ? initial.anchorY : current.anchorY,
  }
  const driven = canvasTransformDrivenProperties(keyframes)
  const blockedProperties = (['x', 'y', 'width', 'height', 'anchorX', 'anchorY', 'rotation'] as const).filter(property => driven.has(property) && Math.abs(current[property] - initial[property]) >= 0.000001)
  if (blockedProperties.length) return { transform, autoKeyframeOperations, blockedProperties }
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
    // Coupled Scale lanes use percentages of the unanimated base; scalar
    // width/height lanes and gizmo geometry use absolute canvas pixels.
    const value = vector === 'scale'
      ? { x: base.width === 0 ? 100 : current.width / base.width * 100, y: base.height === 0 ? 100 : current.height / base.height * 100 }
      : { x: current[x], y: current[y] }
    const operation = !scalarLane || vectorLane ? getVectorAutoKeyframeOperation(item, keyframes, vector, value, frame) : null
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

function groupTransform(members: Member[], width: number, height: number) {
  const boxes = members.map(member => computeItemAabb(member.initial, width, height))
  const left = Math.min(...boxes.map(box => box.left))
  const top = Math.min(...boxes.map(box => box.top))
  const right = Math.max(...boxes.map(box => box.right))
  const bottom = Math.max(...boxes.map(box => box.bottom))
  const groupCenter = { x: (left + right) / 2, y: (top + bottom) / 2 }
  const groupBounds = { left, top, right, bottom, width: right - left, height: bottom - top }
  const state: GroupTransformState = {
    itemIds: members.map(member => member.item.id), groupCenter, groupBounds,
    itemTransforms: new Map(members.map(member => [member.item.id, member.initial])),
    itemOffsets: new Map(members.map(member => [member.item.id, { x: width / 2 + member.initial.x - groupCenter.x, y: height / 2 + member.initial.y - groupCenter.y }])),
    itemRotations: new Map(members.map(member => [member.item.id, member.initial.rotation])),
  }
  const transform: ResolvedTransform = { x: groupCenter.x - width / 2, y: groupCenter.y - height / 2, width: groupBounds.width, height: groupBounds.height, rotation: 0, anchorX: groupBounds.width / 2, anchorY: groupBounds.height / 2, opacity: 1, cornerRadius: 0 }
  return { state, transform }
}

type VisualUpdates = ItemPropertiesPreview & Pick<Partial<TextItem>, 'textStyleScale'>
type VisualScale = { properties: ItemPropertiesPreview; updates: VisualUpdates; operations: AutoKeyframeOperation[]; blocked: 'driver' | 'frame' | null }

function groupVisualScale(member: Member, lanes: ItemKeyframes | undefined, factor: number, frame: number, canvas: CanvasSettings): VisualScale {
  const result: VisualScale = { properties: {}, updates: {}, operations: [], blocked: null }
  if (Math.abs(factor - 1) < 0.000001) return result
  const driven = canvasTransformDrivenProperties(lanes)
  if (member.item.type === 'shape') {
    for (const property of ['strokeWidth', 'cornerRadius'] as const) {
      const value = member.previousPreview?.properties?.[property] ?? member.item[property] ?? 0
      if (!value) continue
      if (driven.has(property)) { result.blocked = 'driver'; return result }
      result.properties[property] = value * factor
      result.updates[property] = value * factor
    }
    return result
  }
  if (!member.text) return result
  const text = member.text
  const style = resolveTextStyle(text)
  const presetAnimated = !!text.textStylePresetId && !!lanes?.properties.some(lane => lane.property === 'textStyleScale' && lane.keyframes.length)
  let scaledPreset = text
  if (presetAnimated && text.textStylePresetId) {
    if (driven.has('textStyleScale')) { result.blocked = 'driver'; return result }
    const nextScale = (text.textStyleScale ?? 1) * factor
    const operation = getAutoKeyframeOperation(member.item, lanes, 'textStyleScale', nextScale, frame)
    if (!operation) { result.blocked = 'frame'; return result }
    result.operations.push(operation)
    scaledPreset = { ...text, ...applyTextStylePresetToItem(text, text.textStylePresetId, canvas, nextScale) }
  } else if (text.textStylePresetId) result.updates.textStyleScale = (text.textStyleScale ?? 1) * factor
  const presetStyle = resolveTextStyle(scaledPreset)
  const rawStroke = member.item.type === 'text' || member.item.type === 'subtitle' ? member.item.stroke : undefined
  const rawShadow = member.item.type === 'text' || member.item.type === 'subtitle' ? member.item.textShadow : undefined
  const setDistance = (target: ItemPropertiesPreview, property: TextAnimatableProperty, value: number, raw = false) => {
    if (property === 'fontSize' || property === 'textPadding' || property === 'backgroundRadius') target[property] = value
    else if (property === 'strokeWidth') {
      const stroke = target.stroke ?? (raw ? rawStroke : text.stroke) ?? { width: 0, color: text.stroke?.color ?? '#111827' }
      target.stroke = { ...stroke, width: value }
    }
    else {
      const field = property === 'textShadowOffsetX' ? 'offsetX' : property === 'textShadowOffsetY' ? 'offsetY' : 'blur'
      const original = raw ? rawShadow : text.textShadow
      target.textShadow = { ...(target.textShadow ?? original ?? { offsetX: 0, offsetY: 0, blur: 0, color: text.textShadow?.color ?? '#000000' }), [field]: value }
    }
  }
  const distances: [TextAnimatableProperty, number, number][] = [
    ['fontSize', text.fontSize ?? TEXT_DEFAULTS.fontSize, scaledPreset.fontSize ?? TEXT_DEFAULTS.fontSize],
    ['textPadding', style.textPadding, presetStyle.textPadding],
    ['backgroundRadius', style.backgroundRadius, presetStyle.backgroundRadius],
  ]
  if (text.stroke) distances.push(['strokeWidth', text.stroke.width, scaledPreset.stroke?.width ?? 0])
  if (text.textShadow) distances.push(['textShadowOffsetX', text.textShadow.offsetX, scaledPreset.textShadow?.offsetX ?? 0], ['textShadowOffsetY', text.textShadow.offsetY, scaledPreset.textShadow?.offsetY ?? 0], ['textShadowBlur', text.textShadow.blur, scaledPreset.textShadow?.blur ?? 0])
  for (const [property, initial, preset] of distances) {
    const animated = !!lanes?.properties.some(lane => lane.property === property && lane.keyframes.length)
    const value = presetAnimated && !animated ? preset : initial * factor
    if (Math.abs(value - initial) < 0.000001) continue
    if (driven.has(property)) { result.blocked = 'driver'; return result }
    const operation = getAutoKeyframeOperation(member.item, lanes, property, value, frame)
    if (operation) result.operations.push(operation)
    else if (animated) { result.blocked = 'frame'; return result }
    else if (!presetAnimated) setDistance(result.updates, property, value, true)
    setDistance(result.properties, property, value)
  }
  const letterSpacing = presetAnimated ? presetStyle.letterSpacing : style.letterSpacing * factor
  if (letterSpacing !== style.letterSpacing) {
    result.properties.letterSpacing = letterSpacing
    if (!presetAnimated) result.updates.letterSpacing = letterSpacing
  }
  if (presetAnimated) result.properties.textSpans = scaledPreset.textSpans
  else if (text.textSpans?.some(span => span.fontSize !== undefined || span.letterSpacing !== undefined)) {
    const spans = text.textSpans.map(span => ({ ...span, ...(span.fontSize === undefined ? {} : { fontSize: span.fontSize * factor }), ...(span.letterSpacing === undefined ? {} : { letterSpacing: span.letterSpacing * factor }) }))
    result.properties.textSpans = spans
    result.updates.textSpans = spans
  }
  return result
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
  const activeTransform = useGizmoStore(state => state.previewTransform)
  const surface = useRef<HTMLDivElement>(null)
  const gesture = useRef<Gesture | null>(null)
  const canvas = { width, height, fps }
  const getItem = (id: string) => items.find(item => item.id === id)
  const getKeyframes = (id: string) => keyframes.find(lanes => lanes.itemId === id)
  const resolve = (item: TimelineItem, preview = true) => resolveItemTransformAtFrame(item, {
    canvas, frame, keyframes: getKeyframes(item.id), getItem, getKeyframes,
    getPreviewTransform: preview ? id => previews?.[id]?.transform : undefined,
  })
  const locked = (item: TimelineItem) => tracks.find(track => track.id === item.trackId)?.locked === true
  const solo = tracks.some(track => track.solo)
  const visible = items.filter(item => {
    const track = tracks.find(track => track.id === item.trackId)
    return track && (solo ? track.solo : track.visible !== false) && item.type !== 'audio' && item.type !== 'adjustment' && frame >= item.from && frame < item.from + item.durationInFrames
  }).sort((a, b) => (tracks.find(track => track.id === b.trackId)?.order ?? 0) - (tracks.find(track => track.id === a.trackId)?.order ?? 0))
  const participants = (selection: string[]) => items.filter(item => selection.includes(item.id) && item.type !== 'audio' && item.type !== 'adjustment')
  const snapshot = (selection: string[]): Member[] => participants(selection).map(item => {
    const initial = resolve(item)
    const parentItem = item.transformParent?.parentItemId ? getItem(item.transformParent.parentItemId) : undefined
    const parent = parentItem ? resolve(parentItem) : undefined
    const textSource = item.type === 'text' ? item : item.type === 'subtitle' ? { ...item, type: 'text' as const, text: '' } : undefined
    const text = textSource ? { ...resolveAnimatedTextItem(textSource, getKeyframes(item.id), frame - item.from, canvas), ...previews?.[item.id]?.properties } : undefined
    return { item, initial, local: worldToLocalTransform(initial, item.transformParent, parent), previousPreview: previews?.[item.id] ?? null, text }
  })
  const selection = snapshot(ids)
  const selected = selection.find(member => visible.some(item => item.id === member.item.id))?.item
  const group = selection.length > 1 ? groupTransform(selection, width, height) : null
  let selectedTransform = group?.transform ?? selection[0]?.initial
  if (gesture.current?.group && activeTransform) selectedTransform = { ...gesture.current.initial, ...activeTransform, anchorX: activeTransform.anchorX ?? activeTransform.width / 2, anchorY: activeTransform.anchorY ?? activeTransform.height / 2 }
  const selectedDrivers = new Set(selection.flatMap(member => [...canvasTransformDrivenProperties(getKeyframes(member.item.id))]))
  const resizeBlocked = (members: Member[], handle: GizmoHandle, shiftKey = false) => members.some(({ item }) => {
    const driven = canvasTransformDrivenProperties(getKeyframes(item.id))
    if (members.length > 1 && (item.type === 'text' || item.type === 'subtitle') && ['fontSize', 'textStyleScale', 'textPadding', 'backgroundRadius', 'textShadowOffsetX', 'textShadowOffsetY', 'textShadowBlur', 'strokeWidth'].some(property => driven.has(property))) return true
    const aspectLock = item.transform?.aspectRatioLocked ?? (item.type !== 'shape' && item.type !== 'text')
    const aspectLocked = members.length > 1 || (shiftKey ? !aspectLock : aspectLock)
    return ((aspectLocked || handle.includes('e') || handle.includes('w')) && driven.has('width')) ||
      ((aspectLocked || handle.includes('n') || handle.includes('s')) && driven.has('height'))
  })
  const point = (event: PointerEvent) => {
    const rect = surface.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0 || rect.height <= 0) return null
    return screenToCanvas(event.clientX, event.clientY, { containerRect: rect, playerSize: rect, projectSize: canvas, zoom: -1 })
  }
  const restore = (drag: Gesture) => {
    const store = useGizmoStore.getState()
    if (store.activeGizmo?.interactionId === drag.interactionId) store.cancelInteraction()
    for (const member of drag.members) store.replaceItemPreview(member.item.id, member.previousPreview)
  }
  const cancel = () => {
    const drag = gesture.current
    if (!drag) return
    gesture.current = null
    restore(drag)
  }
  useEffect(() => {
    const stopPlayback = usePlaybackStore.subscribe(state => {
      const drag = gesture.current
      if (drag && (state.isPlaying || (state.previewFrame ?? state.currentFrame) !== drag.frame)) cancel()
    })
    const stopSelection = useSelectionStore.subscribe(state => {
      const drag = gesture.current
      if (drag && (state.selectedItemIds.length !== drag.selectionIds.length || state.selectedItemIds.some((id, index) => id !== drag.selectionIds[index]))) cancel()
    })
    const stopItems = useItemsStore.subscribe(state => {
      const drag = gesture.current
      if (drag && (state.items !== drag.items || drag.members.some(member => state.tracks.find(track => track.id === member.item.trackId)?.locked))) cancel()
    })
    const stopKeyframes = useKeyframesStore.subscribe(state => {
      if (gesture.current && state.keyframes !== gesture.current.keyframes) cancel()
    })
    const blur = () => cancel()
    window.addEventListener('blur', blur)
    return () => { stopPlayback(); stopSelection(); stopItems(); stopKeyframes(); window.removeEventListener('blur', blur); cancel() }
  }, [])
  const start = (event: PointerEvent, item: TimelineItem, mode: Mode, handle: GizmoHandle = 'se') => {
    if (event.button !== 0 || playing || gesture.current) return
    const currentIds = useSelectionStore.getState().selectedItemIds
    const selectionIds = currentIds.includes(item.id) ? currentIds : computeItemSelection({ items, currentIds: [], itemId: item.id, anchorId: null, append: false, range: false, linkedSelection: useEditorStore.getState().linkedSelectionEnabled })
    const members = snapshot(selectionIds)
    useSelectionStore.getState().selectItems(selectionIds)
    if (!members.length || members.some(member => locked(member.item))) return
    const driven = new Set(members.flatMap(member => [...canvasTransformDrivenProperties(getKeyframes(member.item.id))]))
    if ((mode === 'rotate' && driven.has('rotation')) || (mode === 'scale' && resizeBlocked(members, handle, event.shiftKey)) || (mode === 'translate' && driven.has('x') && driven.has('y'))) {
      event.preventDefault(); event.stopPropagation(); toast(driverMessage); return
    }
    const location = point(event)
    if (!location) return
    const store = useGizmoStore.getState()
    if (store.activeGizmo) return
    event.preventDefault(); event.stopPropagation()
    surface.current?.focus({ preventScroll: true })
    const group = members.length > 1 ? groupTransform(members, width, height) : null
    const initial = group?.transform ?? members[0].initial
    const rect = surface.current!.getBoundingClientRect()
    store.setCanvasSize(width, height)
    store.setCanvasScale(rect.width / width)
    store.setOtherItemBounds([])
    const stroke = !group && item.type === 'shape' && item.strokeEnabled !== false ? item.strokeWidth : undefined
    const itemType = group ? 'image' : item.type
    const scaleStart = group ? { ...initial, anchorX: undefined, anchorY: undefined } : prepareScaleStartTransform(initial, item, getKeyframes(item.id), previews?.[item.id]?.transform)
    // A virtual group must never enter any member's single-item DOM preview path.
    const gizmoId = group ? `group:${crypto.randomUUID()}` : item.id
    const interactionId = mode === 'translate' ? store.startTranslate(gizmoId, location, initial, stroke, itemType)
      : mode === 'rotate' ? store.startRotate(gizmoId, location, initial, stroke, itemType)
        : store.startScale(gizmoId, handle, location, scaleStart, itemType, group ? true : item.transform?.aspectRatioLocked, stroke)
    gesture.current = { members, selectionIds, items, keyframes, frame, pointerId: event.pointerId, interactionId, mode, initial, group: group?.state, moved: false }
    surface.current?.setPointerCapture(event.pointerId)
  }
  const localPreviews = (drag: Gesture, transform: Transform): Map<string, ResolvedTransform> => {
    const worlds = new Map<string, ResolvedTransform>()
    if (drag.group) {
      const scale = drag.mode === 'scale' ? transform.width / drag.initial.width : 1
      const rotation = drag.mode === 'rotate' ? transform.rotation : 0
      const center = drag.mode === 'rotate' ? drag.initial : transform
      for (const member of drag.members) {
        const offset = drag.group.itemOffsets.get(member.item.id)!
        // Rotate each member's actual pivot, then recover its unrotated box
        // center. Rotating the box center alone drifts off-center anchors.
        const anchorOffset = { x: member.initial.anchorX - member.initial.width / 2, y: member.initial.anchorY - member.initial.height / 2 }
        const rotated = rotatePoint({ x: (offset.x + anchorOffset.x) * scale, y: (offset.y + anchorOffset.y) * scale }, { x: 0, y: 0 }, rotation)
        worlds.set(member.item.id, { ...member.initial, x: center.x + rotated.x - anchorOffset.x * scale, y: center.y + rotated.y - anchorOffset.y * scale, width: member.initial.width * scale, height: member.initial.height * scale, anchorX: member.initial.anchorX * scale, anchorY: member.initial.anchorY * scale, rotation: member.initial.rotation + rotation })
      }
    } else {
      worlds.set(drag.members[0].item.id, { ...drag.initial, ...transform, anchorX: transform.anchorX ?? transform.width / 2, anchorY: transform.anchorY ?? transform.height / 2, cornerRadius: transform.cornerRadius ?? drag.initial.cornerRadius })
    }
    const locals = new Map<string, ResolvedTransform>()
    const members = new Map(drag.members.map(member => [member.item.id, member]))
    const converting = new Set<string>()
    const convert = (member: Member): void => {
      if (locals.has(member.item.id) || converting.has(member.item.id)) return
      converting.add(member.item.id)
      const ancestors = new Set<string>([member.item.id])
      let ancestor = member.item.transformParent?.parentItemId ? getItem(member.item.transformParent.parentItemId) : undefined
      while (ancestor && !ancestors.has(ancestor.id)) {
        ancestors.add(ancestor.id)
        const selectedAncestor = members.get(ancestor.id)
        if (selectedAncestor) convert(selectedAncestor)
        ancestor = ancestor.transformParent?.parentItemId ? getItem(ancestor.transformParent.parentItemId) : undefined
      }
      const parentItem = member.item.transformParent?.parentItemId ? getItem(member.item.transformParent.parentItemId) : undefined
      const parent = parentItem ? resolveItemTransformAtFrame(parentItem, { canvas, frame: drag.frame, keyframes: getKeyframes(parentItem.id), getItem, getKeyframes, getPreviewTransform: id => locals.get(id) ?? previews?.[id]?.transform }) : undefined
      let local = worldToLocalTransform(worlds.get(member.item.id)!, member.item.transformParent, parent)
      if (!drag.group && drag.mode === 'translate') {
        const driven = canvasTransformDrivenProperties(getKeyframes(member.item.id))
        local = { ...member.local, x: driven.has('x') ? member.local.x : local.x, y: driven.has('y') ? member.local.y : local.y }
      } else if (!drag.group && drag.mode === 'rotate') local = { ...member.local, rotation: local.rotation }
      locals.set(member.item.id, local)
      converting.delete(member.item.id)
    }
    for (const member of drag.members) convert(member)
    return locals
  }
  const move = (event: PointerEvent) => {
    const drag = gesture.current
    const location = point(event)
    if (!drag || drag.pointerId !== event.pointerId || !location) return
    const store = useGizmoStore.getState()
    const active = store.activeGizmo
    if (active?.interactionId !== drag.interactionId) { cancel(); return }
    if (!drag.moved && location.x === active.startPoint.x && location.y === active.startPoint.y) return
    if (drag.group && drag.mode === 'scale') {
      const ctrl = event.ctrlKey || event.metaKey
      const raw = calculateTransform(active, location, false, width, height, ctrl)
      const textSizes = drag.members.flatMap(member => {
        if (!member.text) return []
        const size = member.text.fontSize ?? TEXT_DEFAULTS.fontSize
        return [size, ...(member.text.textSpans ?? []).map(span => span.fontSize ?? size)]
      })
      const fontMinimum = textSizes.reduce((minimum, size) => size > 0 ? Math.max(minimum, 1 / size) : minimum, 0)
      const minimum = Math.max(20 / drag.initial.width, 20 / drag.initial.height, fontMinimum)
      let factor = Math.max(raw.width / drag.initial.width, raw.height / drag.initial.height, minimum)
      let snapLines: typeof store.snapLines = []
      if (store.snappingEnabled && !event.altKey && !ctrl) {
        const snap = applyGroupScaleSnapping(computeItemAabb(raw, width, height), drag.group.groupCenter, drag.initial.width, drag.initial.height, width, height, store.snapLines, store.canvasScale)
        factor = Math.max(snap.scaleFactor, minimum)
        snapLines = factor === snap.scaleFactor ? snap.snapLines : []
      }
      const scaled = { ...raw, width: drag.initial.width * factor, height: drag.initial.height * factor }
      if (ctrl) {
        const handle = active.activeHandle ?? 'se'
        const fractionX = handle.includes('e') ? 0 : handle.includes('w') ? 1 : 0.5
        const fractionY = handle.includes('s') ? 0 : handle.includes('n') ? 1 : 0.5
        scaled.x = drag.initial.x + (fractionX - 0.5) * (drag.initial.width - scaled.width)
        scaled.y = drag.initial.y + (fractionY - 0.5) * (drag.initial.height - scaled.height)
      }
      useGizmoStore.setState({ activeGizmo: { ...active, currentPoint: location, shiftKey: false, ctrlKey: ctrl, altKey: event.altKey }, previewTransform: scaled, snapLines })
    } else store.updateInteraction(location, event.shiftKey, event.ctrlKey || event.metaKey, event.altKey)
    const transform = useGizmoStore.getState().previewTransform
    if (!transform) return
    drag.moved = true
    const locals = localPreviews(drag, transform)
    const factor = drag.group && drag.mode === 'scale' ? transform.width / drag.initial.width : 1
    const visualProperties: Record<string, ItemPropertiesPreview> = {}
    for (const member of drag.members) {
      const driven = canvasTransformDrivenProperties(getKeyframes(member.item.id))
      if ((['x', 'y', 'width', 'height', 'anchorX', 'anchorY', 'rotation'] as const).some(property => driven.has(property) && Math.abs(locals.get(member.item.id)![property] - member.local[property]) >= 0.000001)) {
        cancel(); toast(driverMessage); return
      }
      const visual = groupVisualScale(member, getKeyframes(member.item.id), factor, drag.frame, canvas)
      if (visual.blocked) { cancel(); toast(visual.blocked === 'driver' ? driverMessage : groupFrameMessage); return }
      if (Object.keys(visual.properties).length) visualProperties[member.item.id] = visual.properties
    }
    for (const member of drag.members) {
      const authored = prepareScaleStartTransform(locals.get(member.item.id)!, member.item, getKeyframes(member.item.id), member.previousPreview?.transform)
      const properties = visualProperties[member.item.id]
      // Rebuild from the captured preview so returning to scale 1 also removes
      // this gesture's font/shape overrides instead of leaving stale properties.
      store.replaceItemPreview(member.item.id, { ...member.previousPreview, transform: { ...member.previousPreview?.transform, ...authored }, ...(properties ? { properties: { ...member.previousPreview?.properties, ...properties } } : {}) })
    }
  }
  const commit = (members: Member[], locals: Map<string, ResolvedTransform>, atFrame: number, factor = 1) => {
    const entries: { id: string; transform: Partial<TransformProperties> }[] = []
    const itemUpdates: { id: string; updates: VisualUpdates }[] = []
    const autoKeyframeOperations: AutoKeyframeOperation[] = []
    for (const member of members) {
      const lanes = getKeyframes(member.item.id)
      const base = resolveTransform(member.item, canvas, getSourceDimensions(member.item))
      const editable = resolveAnimatedTransform(base, lanes, atFrame - member.item.from)
      const payload = canvasTransformCommit(member.item, lanes, atFrame, member.local, locals.get(member.item.id)!, editable, base)
      if ('blockedProperties' in payload) { toast(driverMessage); return false }
      if (members.length > 1 && Object.keys(payload.transform).some(property => {
        const vector = property === 'x' || property === 'y' ? 'position' : property === 'width' || property === 'height' ? 'scale' : property === 'anchorX' || property === 'anchorY' ? 'anchor' : null
        return lanes?.properties.some(lane => lane.property === property && lane.keyframes.length) || (vector && lanes?.vectorProperties?.some(lane => lane.property === vector && lane.keyframes.length))
      })) { toast(groupFrameMessage); return false }
      const visual = groupVisualScale(member, lanes, factor, atFrame, canvas)
      if (visual.blocked) { toast(visual.blocked === 'driver' ? driverMessage : groupFrameMessage); return false }
      if (Object.keys(visual.updates).length) itemUpdates.push({ id: member.item.id, updates: visual.updates })
      autoKeyframeOperations.push(...visual.operations)
      if (Object.keys(payload.transform).length) entries.push({ id: member.item.id, transform: payload.transform })
      autoKeyframeOperations.push(...payload.autoKeyframeOperations)
    }
    if (!entries.length && !itemUpdates.length && !autoKeyframeOperations.length) return true
    const result = withCommandMergeKey(null, () => executeCommands([{ type: 'item.updateTransformsMap', params: { entries, itemUpdates, autoKeyframeOperations } }]))
    if (!result.ok) { toast.error('Không áp dụng được biến đổi clip'); announce('Không áp dụng được biến đổi clip', 'assertive') }
    return result.ok
  }
  const finish = (event: PointerEvent) => {
    const drag = gesture.current
    if (!drag || drag.pointerId !== event.pointerId) return
    move(event)
    if (gesture.current !== drag) return
    const transform = useGizmoStore.getState().endInteraction()
    gesture.current = null
    try {
      if (drag.moved && transform) commit(drag.members, localPreviews(drag, transform), drag.frame, drag.group && drag.mode === 'scale' ? transform.width / drag.initial.width : 1)
    } finally { restore(drag) }
    if (surface.current?.hasPointerCapture(event.pointerId)) surface.current.releasePointerCapture(event.pointerId)
  }
  const nudge = (dx: number, dy: number) => {
    if (!selection.length || playing || selection.some(member => locked(member.item)) || gesture.current) return
    const initial = group?.transform ?? selection[0].initial
    const drag: Gesture = { members: selection, selectionIds: ids, items, keyframes, frame, pointerId: -1, interactionId: -1, mode: 'translate', initial, group: group?.state, moved: true }
    commit(selection, localPreviews(drag, { ...initial, x: initial.x + dx, y: initial.y + dy }), frame)
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
    const hit = [...visible].reverse().find(item => {
      const transform = resolve(item)
      const local = rotatePoint(location, getTransformAnchor(transform, width, height), -transform.rotation)
      return Math.abs(local.x - width / 2 - transform.x) <= transform.width / 2 && Math.abs(local.y - height / 2 - transform.y) <= transform.height / 2
    })
    if (hit) start(event, hit, 'translate')
    else useSelectionStore.getState().selectItems([])
  }}>
    {selected && selectedTransform && !playing && <div {...stylex.props(s.selectionBounds)} style={bounds(selectedTransform)}>
      {!selection.some(member => locked(member.item)) && <>
        <button type="button" aria-disabled={selectedDrivers.has('x') && selectedDrivers.has('y')} aria-label="Di chuyển clip" title={selectedDrivers.has('x') && selectedDrivers.has('y') ? driverMessage : 'Di chuyển clip (Ctrl+mũi tên; Shift: 10px)'} {...stylex.props(s.transformHandle, s.moveHandle)} onPointerDown={event => start(event, selected, 'translate')}><Move size={14} /></button>
        {resizeHandles.map(handle => <button key={handle} type="button" aria-disabled={resizeBlocked(selection, handle)} aria-label={`Đổi tỉ lệ clip (${handle})`} title={resizeBlocked(selection, handle) ? driverMessage : group ? 'Đổi tỉ lệ nhóm (giữ tỉ lệ; Ctrl: neo góc; Alt: bỏ bắt dính)' : 'Đổi tỉ lệ clip (Shift: đảo khoá tỉ lệ; Ctrl: neo góc; Alt: bỏ bắt dính)'} {...stylex.props(s.transformHandle, s.resizeHandle(handle), handle === 'se' && s.scaleHandle)} onPointerDown={event => start(event, selected, 'scale', handle)}>{handle === 'se' && <Maximize2 size={14} />}</button>)}
        <button type="button" aria-disabled={selectedDrivers.has('rotation')} aria-label="Xoay clip" title={selectedDrivers.has('rotation') ? driverMessage : 'Xoay clip (Alt: bỏ bắt dính góc 15°)'} {...stylex.props(s.transformHandle, s.rotateHandle)} onPointerDown={event => start(event, selected, 'rotate')}><RotateCw size={14} /></button>
      </>}
    </div>}
  </div>
}
