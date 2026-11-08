import { z } from 'zod'
import { templateBackgroundAppearanceSchema, templateBackdropSchema, templateScenesSchema } from '@editor/features/editing-templates/template-schemas'
import type {
  ControllerItem,
  TimelineItem,
  TimelineTrack,
  VideoItem,
} from '@editor/types/timeline'
import type { TransformProperties, TransformParentBinding, TransformParentingBehavior, CanvasSettings, ResolvedTransform } from '@editor/types/transform'
import type {
  AnimatableProperty,
  DirectPropertyLink,
  EasingConfig,
  ItemKeyframes,
  Keyframe,
  PropertyKeyframes,
  StoredPropertyExpression,
  VectorKeyframe,
  VectorPropertyKeyframes,
} from '@editor/types/keyframe'
import type { MaskVertex } from '@editor/types/masks'
import type { Transition } from '@editor/types/transition'
import type { AutoKeyframeOperation } from '@editor/features/timeline/deps/keyframes'
import { registerCommand } from '../registry'
import { waveformItemIssue } from '@editor/features/waveform-visualizer/types'
import { executeCommands } from '../executor'
import type { ApplyContext, CommandIssue, Inverse, ProjectStateView } from '../types'
import { useItemsStore } from '../../features/timeline/stores/items-store'
import { useTransitionsStore } from '../../features/timeline/stores/transitions-store'
import { useKeyframesStore } from '../../features/timeline/stores/keyframes-store'
import { useCompositionsStore } from '../../features/timeline/stores/compositions-store'
import { useCompositionNavigationStore } from '../../features/timeline/stores/composition-navigation-store'
import {
  applyTransitionRepairs,
  canAddKeyframeAtFrame,
  getCurrentTimelineSnapshot,
  getLogger,
  warnIfOverlapping,
  type TimelineSnapshotLike,
} from '../../features/timeline/stores/actions/shared'
import {
  buildLinkedLeftShiftUpdates,
  expandIdsWithLinkedItems,
} from '../../features/timeline/stores/actions/linked-edit'
import { propagateRemovedIntervalsToSyncLockedTracks } from '../../features/timeline/stores/actions/sync-lock-ripple'
import { placeItemsWithoutTimelineOverlap } from '../../features/timeline/stores/actions/item-placement'
import {
  canLinkSelection,
  expandSelectionWithLinkedItems,
  getLinkedItemIds,
  getSynchronizedLinkedItems,
} from '../../features/timeline/utils/linked-items'
import { isTrackSyncLockEnabled } from '../../features/timeline/utils/track-sync-lock'
import { pruneEmptyLayerGroupHierarchy } from '../../features/timeline/utils/group-utils'
import { buildLinkedAudioForVideo } from '../../features/timeline/utils/embedded-audio-split'
import { sourceToTimelineFrames, videoLoopBarrier } from '../../features/timeline/utils/source-calculations'
import { useTimelineSettingsStore } from '../../features/timeline/stores/timeline-settings-store'
import { createDefaultControllerItem } from '../../features/timeline/utils/generated-layer-items'
import { buildTransitionIndexes } from '../../features/timeline/utils/transition-indexes'
import {
  buildTransitionChains,
  computeLayout,
  type LayoutConfig,
} from '../../features/timeline/utils/bento-layout'
import {
  resolveItemTransformAtFrame,
  resolveItemTransformAtRelativeFrame,
} from '../../features/timeline/deps/composition-runtime'
import {
  createTransformParentBinding,
  hasRedundantTransformParentLink,
  wouldCreateTransformParentCycle,
} from '../../shared/utils/transform-parenting'

// ---------------------------------------------------------------------------
// Schemas (JSON-only, replayable)
// ---------------------------------------------------------------------------

const id = z.string().min(1)
const idList = z.array(id).min(1)
const frame = z.number().finite()
const jsonRecord = z.record(z.string(), z.json())

const ITEM_TYPES = [
  'video',
  'audio',
  'text',
  'image',
  'lottie',
  'shape',
  'adjustment',
  'controller',
  'composition',
  'subtitle',
] as const

/** A full TimelineItem as JSON. `id` is optional on input and minted when absent. */
export const itemJsonSchema = z.looseObject({
  id: id.optional(),
  type: z.enum(ITEM_TYPES),
  trackId: id,
  from: frame,
  durationInFrames: z.number().finite().positive(),
  label: z.string(),
  templateAnchor: z.enum(['full', 'start', 'end']).optional(),
}).superRefine((item, ctx) => {
  const issue = waveformItemIssue(item)
  if (issue) ctx.addIssue({ code: 'custom', path: ['waveformVisualizer'], message: issue })
  if (item.templateAnchor === 'full' && item.type === 'video' && videoLoopBarrier(item as unknown as TimelineItem) !== null) {
    ctx.addIssue({ code: 'custom', path: ['templateAnchor'], message: 'Clip neo toàn video phải lặp được (đã tắt tiếng, có khoảng nguồn)' })
  }
  if (item.loop !== true) return
  if (item.type !== 'video') {
    ctx.addIssue({ code: 'custom', path: ['loop'], message: 'Chỉ clip video mới lặp được' })
    return
  }
  if (item.embeddedAudioMuted !== true) {
    ctx.addIssue({ code: 'custom', path: ['loop'], message: 'Chỉ lặp được clip đã tắt tiếng' })
    return
  }
  const windowEnd = (item.sourceEnd ?? item.sourceDuration) as number | undefined
  const windowStart = (item.sourceStart ?? item.trimStart ?? item.offset ?? 0) as number
  if (windowEnd === undefined || !Number.isFinite(windowEnd) || windowEnd <= windowStart) {
    ctx.addIssue({ code: 'custom', path: ['loop'], message: 'Clip cần biết khoảng nguồn để lặp' })
  }
})

/** Template slot roles a track can hold; at most one track per role. */
export const templateSlotSchema = z.enum(['scenes', 'captions', 'source-audio', 'background-video'])

/** A full TimelineTrack as JSON. */
export const trackJsonSchema = z.looseObject({
  id,
  name: z.string(),
  height: z.number().finite().positive(),
  locked: z.boolean(),
  visible: z.boolean(),
  muted: z.boolean(),
  solo: z.boolean(),
  order: z.number().finite(),
  kind: z.enum(['video', 'audio']).optional(),
  templateSlot: templateSlotSchema.optional(),
  /** Clone loudness target; meaningful only while the track holds the `source-audio` slot. */
  templateLoudnessLufs: z.number().min(-24).max(-9).optional(),
  /** Scene-subject recipe; meaningful only while the track holds the `scenes` slot. */
  templateScenes: templateScenesSchema.optional(),
  /** Backdrop-channel settings; meaningful only while the track holds the `background-video` slot. */
  templateBackdrop: templateBackdropSchema.optional(),
  templateBackgroundAppearance: templateBackgroundAppearanceSchema.optional(),
}).superRefine((track, ctx) => {
  // This shared boundary covers track.set, project.patch (including compositions),
  // addOnNewTracks, moveMany and duplicate, not just track.update.
  if (track.templateBackgroundAppearance !== undefined && (track.templateSlot !== 'background-video' || track.kind === 'audio')) {
    ctx.addIssue({ code: 'custom', path: ['templateBackgroundAppearance'], message: 'Cài đặt nền chỉ đặt trên track video vai trò Nền video' })
  }
})

const itemMoveSchema = z
  .object({ id, from: frame, trackId: id.optional() })
  .strict()

const positionSchema = z.object({ from: frame, trackId: id }).strict()

const canvasSchema = z
  .object({
    width: z.number().finite().positive(),
    height: z.number().finite().positive(),
    fps: z.number().finite().positive(),
  })
  .strict()

const transformSchema = z
  .object({
    x: z.number().finite().optional(),
    y: z.number().finite().optional(),
    width: z.number().finite().optional(),
    height: z.number().finite().optional(),
    anchorX: z.number().finite().optional(),
    anchorY: z.number().finite().optional(),
    rotation: z.number().finite().optional(),
    flipHorizontal: z.boolean().optional(),
    flipVertical: z.boolean().optional(),
    opacity: z.number().finite().optional(),
    cornerRadius: z.number().finite().optional(),
    aspectRatioLocked: z.boolean().optional(),
  })
  .strict()

const transformOperationSchema = z.enum([
  'move',
  'resize',
  'rotate',
  'opacity',
  'corner_radius',
  'transform',
])

const easingSchema = z.enum(['linear', 'ease-in', 'ease-out', 'ease-in-out', 'hold', 'cubic-bezier', 'spring'])
const vector2Schema = z.object({ x: z.number().finite(), y: z.number().finite() }).strict()

const autoKeyframeOperationSchema = z.discriminatedUnion('type', [
  z
    .object({
      type: z.literal('add'),
      itemId: id,
      property: z.string().min(1),
      frame: frame,
      value: z.number().finite(),
      easing: easingSchema.optional(),
    })
    .strict(),
  z
    .object({
      type: z.literal('update'),
      itemId: id,
      property: z.string().min(1),
      keyframeId: id,
      updates: z.object({ value: z.number().finite().optional() }).strict(),
    })
    .strict(),
  z
    .object({
      type: z.literal('vector-add'),
      itemId: id,
      property: z.enum(['position', 'scale', 'anchor']),
      frame: frame,
      value: vector2Schema,
      easing: easingSchema.optional(),
    })
    .strict(),
  z
    .object({
      type: z.literal('vector-update'),
      itemId: id,
      property: z.enum(['position', 'scale', 'anchor']),
      keyframeId: id,
      updates: z.object({ value: vector2Schema }).strict(),
    })
    .strict(),
])

const maskVertexSchema = z
  .object({
    position: z.tuple([z.number().finite(), z.number().finite()]),
    inHandle: z.tuple([z.number().finite(), z.number().finite()]),
    outHandle: z.tuple([z.number().finite(), z.number().finite()]),
    tangentMode: z.enum(['corner', 'smooth', 'continuous', 'broken']).optional(),
  })
  .strict()

const layoutConfigSchema = z
  .object({
    preset: z.enum(['auto', 'row', 'column', 'pip', 'focus-sidebar', 'grid']),
    cols: z.number().int().positive().optional(),
    rows: z.number().int().positive().optional(),
    gap: z.number().finite().optional(),
    padding: z.number().finite().optional(),
  })
  .strict()

const reverseConformSchema = z
  .object({
    src: z.string(),
    path: z.string(),
    key: z.string(),
    quality: z.enum(['preview', 'full']),
    usesProxy: z.boolean(),
    isSourceLevel: z.boolean(),
    sourceDuration: z.number().finite().optional(),
    conformFps: z.number().finite().optional(),
  })
  .strict()

/** Fields cleared by the "un-reverse" branch of item.reverse. */
const REVERSE_CLEAR_KEYS = [
  'reverseConformSrc',
  'reverseConformPath',
  'reverseConformKey',
  'reverseConformPreviewSrc',
  'reverseConformPreviewPath',
  'reverseConformPreviewKey',
  'reverseConformPreviewUsesProxy',
  'reverseConformPreviewIsSourceLevel',
  'reverseConformPreviewSourceDuration',
  'reverseConformPreviewFps',
  'reverseConformStatus',
] as const

// ---------------------------------------------------------------------------
// Helpers shared with the action wrappers
// ---------------------------------------------------------------------------

const emptyInverse = (): Inverse => ({ kind: 'patch', patch: {} })


function materializeItem(ctx: ApplyContext, raw: z.infer<typeof itemJsonSchema>): TimelineItem {
  return { ...raw, id: raw.id ?? ctx.ids.mint() } as unknown as TimelineItem
}

/**
 * Split a Partial<TimelineItem> into JSON `updates` (defined values) and `clearFields`
 * (keys whose value is `undefined`, meaning "unset this field").
 */
export function splitItemUpdates(updates: Partial<TimelineItem>): {
  updates: Record<string, unknown>
  clearFields?: string[]
} {
  const set: Record<string, unknown> = {}
  const clear: string[] = []
  for (const [key, value] of Object.entries(updates)) {
    if (value === undefined) clear.push(key)
    else set[key] = value
  }
  return clear.length > 0 ? { updates: set, clearFields: clear } : { updates: set }
}

function mergeClear(updates: Record<string, unknown>, clear: string[] | undefined): Partial<TimelineItem> {
  const merged: Record<string, unknown> = { ...updates }
  for (const key of clear ?? []) merged[key] = undefined
  return merged as Partial<TimelineItem>
}

/** Strip `undefined` values so params are JSON-only before reaching the executor. */
function toJsonParams<T>(params: T): T {
  return JSON.parse(JSON.stringify(params)) as T
}

export type ItemCommandResult<T> = { ok: true; result: T } | { ok: false; issues: CommandIssue[] }

/** Execute one registered command from an action wrapper; results[0] is the command result. */
export function runItemCommand<T = unknown>(type: string, params: unknown): ItemCommandResult<T> {
  const outcome = executeCommands([{ type, params: toJsonParams(params) }])
  if (!outcome.ok) {
    getLogger().warn(`Command ${type} rejected`, { issues: outcome.issues })
    return { ok: false, issues: outcome.issues }
  }
  return { ok: true, result: outcome.results[0] as T }
}

function missingItems(state: ProjectStateView, ids: readonly string[], path = 'itemIds'): CommandIssue[] {
  const known = new Set(state.items.map((item) => item.id))
  return ids
    .filter((itemId) => !known.has(itemId))
    .map((itemId) => ({ message: `Không tìm thấy clip ${itemId}`, code: 'item.notFound', path }))
}

function transformItemIssues(state: ProjectStateView, ids: readonly string[], path = 'itemIds'): CommandIssue[] {
  const requested = new Set(ids)
  return [
    ...missingItems(state, ids, path),
    ...state.items.filter(item => requested.has(item.id) && item.type === 'audio').map(item => ({
      message: `Clip âm thanh ${item.id} không hỗ trợ biến đổi hình ảnh`,
      code: 'item.unsupportedTransform',
      path,
    })),
  ]
}

function missingTracks(
  tracks: ReadonlyArray<{ id: string }>,
  trackIds: ReadonlyArray<string | undefined>,
  path = 'trackId',
): CommandIssue[] {
  const known = new Set(tracks.map((track) => track.id))
  return [...new Set(trackIds)]
    .filter((trackId): trackId is string => trackId !== undefined && !known.has(trackId))
    .map((trackId) => ({ message: `Không tìm thấy track ${trackId}`, code: 'track.notFound', path }))
}

function duplicateIds(ids: readonly string[], path: string): CommandIssue[] {
  return new Set(ids).size === ids.length
    ? []
    : [{ message: 'Danh sách id bị trùng lặp', code: 'id.duplicate', path }]
}

function existingItemIdIssues(state: ProjectStateView, ids: readonly (string | undefined)[], path: string): CommandIssue[] {
  const known = new Set(state.items.map((item) => item.id))
  return ids
    .filter((itemId): itemId is string => itemId !== undefined && known.has(itemId))
    .map((itemId) => ({ message: `Clip ${itemId} đã tồn tại`, code: 'item.exists', path }))
}

function pruneLayerGroupsAfterItemRemoval(): void {
  const store = useItemsStore.getState()
  const nextTracks = pruneEmptyLayerGroupHierarchy(store.tracks, store.items)
  if (nextTracks !== store.tracks) store.setTracks(nextTracks)
}

/** Update transition trackIds when both clips of a pair moved onto the same track. */
function syncTransitionTracksAfterMove(movedItemIds: ReadonlySet<string>): void {
  const items = useItemsStore.getState().items
  const transitions = useTransitionsStore.getState().transitions
  const updated = transitions.map((transition) => {
    if (!movedItemIds.has(transition.leftClipId) || !movedItemIds.has(transition.rightClipId)) {
      return transition
    }
    const leftClip = items.find((item) => item.id === transition.leftClipId)
    const rightClip = items.find((item) => item.id === transition.rightClipId)
    return leftClip && rightClip && leftClip.trackId === rightClip.trackId
      ? { ...transition, trackId: leftClip.trackId }
      : transition
  })
  useTransitionsStore.getState().setTransitions(updated)
}

function canParticipateInTransformHierarchy(item: TimelineItem): boolean {
  return item.type !== 'audio' && item.type !== 'adjustment'
}

// ---------------------------------------------------------------------------
// item.add*
// ---------------------------------------------------------------------------

function validateNewItems(
  state: ProjectStateView,
  items: ReadonlyArray<z.infer<typeof itemJsonSchema>>,
  tracks: ReadonlyArray<{ id: string }>,
): CommandIssue[] {
  const ids = items.flatMap((item) => (item.id ? [item.id] : []))
  return [
    ...duplicateIds(ids, 'items'),
    ...existingItemIdIssues(state, ids, 'items'),
    ...missingTracks(tracks, items.map((item) => item.trackId), 'items.trackId'),
    ...items.flatMap(item => {
      if (item.type !== 'video' || item.templateAnchor !== 'full' || !item.linkedGroupId) return []
      const linkedAudio = [...state.items, ...items].some(other => other.type === 'audio' && other.linkedGroupId === item.linkedGroupId)
      return linkedAudio ? [{ message: 'Clip neo toàn video phải tắt tiếng và không gắn âm thanh', path: 'items.templateAnchor' }] : []
    }),
    ...items.flatMap(item => {
      if (!item.waveformVisualizer) return []
      const media = state.mediaItems?.find(media => media.id === item.mediaId)
      if (!media || !media.mimeType.startsWith('audio/')) return [{ message: 'Chọn tệp âm thanh làm nguồn giọng', path: 'items.mediaId' }]
      // itemJsonSchema already validated the payload; only authoritative source
      // identity remains, so don't scan a large base64 envelope a second time.
      const payload = item.waveformVisualizer as { sourceHash: string; envelopeBase64?: string }
      return payload.envelopeBase64 !== undefined && payload.sourceHash !== media.contentHash ? [{ message: 'Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại', path: 'items.waveformVisualizer' }] : []
    }),
  ]
}

// ---------------------------------------------------------------------------
// item.remove / item.rippleDelete / gaps
// ---------------------------------------------------------------------------

function removeItemsWithCascade(ids: string[]): void {
  useItemsStore.getState()._removeItems(ids)
  useTransitionsStore.getState()._removeTransitionsForItems(ids)
  useKeyframesStore.getState()._removeKeyframesForItems(ids)
  pruneLayerGroupsAfterItemRemoval()
  
}

function rippleDeleteWithCascade(ids: string[], linkedSelectionEnabled: boolean): string[] {
  const items = useItemsStore.getState().items
  const expandedIds = expandIdsWithLinkedItems(items, ids, linkedSelectionEnabled)
  if (expandedIds.length === 0) return []

  const idsToDelete = new Set(expandedIds)
  const remainingItems = items.filter((item) => !idsToDelete.has(item.id))
  const baseShiftByItemId = new Map<string, number>()
  const editedTrackIds = new Set(
    items.filter((item) => idsToDelete.has(item.id)).map((item) => item.trackId),
  )
  const removedIntervals = items
    .filter((item) => idsToDelete.has(item.id))
    .map((item) => ({ start: item.from, end: item.from + item.durationInFrames }))

  // Per-track: shift downstream items on the same track as each deleted item.
  // Linked counterparts and attached captions on tracks that won't be handled
  // by sync-lock ripple get shifted manually. Solo clips on unrelated tracks
  // are left in place.
  for (const item of remainingItems) {
    const shiftAmount = items
      .filter((candidate) => idsToDelete.has(candidate.id))
      .filter(
        (deletedItem) =>
          deletedItem.trackId === item.trackId &&
          deletedItem.from + deletedItem.durationInFrames <= item.from,
      )
      .reduce((sum, deletedItem) => sum + deletedItem.durationInFrames, 0)
    if (shiftAmount > 0) baseShiftByItemId.set(item.id, shiftAmount)
  }

  const trackById = new Map(useItemsStore.getState().tracks.map((track) => [track.id, track]))
  const itemById = new Map(remainingItems.map((item) => [item.id, item]))
  const shiftByItemId = new Map<string, number>()

  for (const [itemId, shiftAmount] of baseShiftByItemId) {
    if (shiftAmount <= 0) continue
    const relatedIds = expandIdsWithLinkedItems(remainingItems, [itemId], linkedSelectionEnabled)
    for (const relatedId of relatedIds) {
      const relatedItem = itemById.get(relatedId)
      if (!relatedItem) continue
      const handledBySyncLock =
        !editedTrackIds.has(relatedItem.trackId) &&
        isTrackSyncLockEnabled(trackById.get(relatedItem.trackId))
      if (handledBySyncLock) continue
      shiftByItemId.set(relatedId, Math.max(shiftByItemId.get(relatedId) ?? 0, shiftAmount))
    }
  }

  const updates = remainingItems.flatMap((item) => {
    const shiftAmount = shiftByItemId.get(item.id) ?? 0
    return shiftAmount > 0 ? [{ id: item.id, from: item.from - shiftAmount }] : []
  })

  // Detect non-shifted items that would be overlapped by shifted items.
  // These get deleted rather than creating overlaps.
  const shiftedById = new Map(updates.map((update) => [update.id, update.from]))
  const coveredIds: string[] = []
  for (const item of remainingItems) {
    if (shiftedById.has(item.id) || idsToDelete.has(item.id)) continue
    const itemEnd = item.from + item.durationInFrames
    for (const other of remainingItems) {
      const newFrom = shiftedById.get(other.id)
      if (newFrom === undefined || other.trackId !== item.trackId) continue
      const newEnd = newFrom + other.durationInFrames
      if (newFrom < itemEnd && newEnd > item.from) {
        coveredIds.push(item.id)
        break
      }
    }
  }

  // Expand covered IDs with linked companions so we don't orphan them
  const expandedCoveredIds = expandIdsWithLinkedItems(remainingItems, coveredIds, linkedSelectionEnabled)
  const allRemoveIds = [...expandedIds, ...expandedCoveredIds]
  const coveredSet = new Set(expandedCoveredIds)
  const filteredUpdates = coveredSet.size > 0 ? updates.filter((update) => !coveredSet.has(update.id)) : updates

  useItemsStore.getState()._removeItems(allRemoveIds)
  if (filteredUpdates.length > 0) useItemsStore.getState()._moveItems(filteredUpdates)

  const syncLockResult = propagateRemovedIntervalsToSyncLockedTracks({
    editedTrackIds,
    intervals: removedIntervals,
  })

  const cascadedRemoveIds = Array.from(new Set([...allRemoveIds, ...syncLockResult.removedIds]))
  useTransitionsStore.getState()._removeTransitionsForItems(cascadedRemoveIds)
  useKeyframesStore.getState()._removeKeyframesForItems(cascadedRemoveIds)

  if (filteredUpdates.length > 0) applyTransitionRepairs(filteredUpdates.map((update) => update.id))

  const repairedClipIds = Array.from(
    new Set([...updates.map((update) => update.id), ...syncLockResult.affectedIds]),
  )
  if (repairedClipIds.length > 0) applyTransitionRepairs(repairedClipIds, new Set(cascadedRemoveIds))

  pruneLayerGroupsAfterItemRemoval()
  
  return allRemoveIds
}

function closeGap(trackId: string, rawFrame: number): void {
  const items = useItemsStore.getState().items
  const targetFrame = Math.max(0, Math.round(rawFrame))
  const trackItems = items
    .filter((item) => item.trackId === trackId)
    .sort((left, right) => left.from - right.from)
  if (trackItems.length === 0) return

  let gapStart = 0
  let gapEnd = 0
  for (const item of trackItems) {
    if (targetFrame >= gapStart && targetFrame < item.from) {
      gapEnd = item.from
      break
    }
    gapStart = item.from + item.durationInFrames
  }
  if (gapEnd <= gapStart) return

  const gapSize = gapEnd - gapStart
  const updates = items
    .filter((item) => item.trackId === trackId && item.from >= gapEnd)
    .map((item) => ({ id: item.id, from: item.from - gapSize }))
  if (updates.length === 0) return

  useItemsStore.getState()._moveItems(updates)
  const syncLockResult = propagateRemovedIntervalsToSyncLockedTracks({
    editedTrackIds: new Set([trackId]),
    intervals: [{ start: gapStart, end: gapEnd }],
  })
  const removedIds = syncLockResult.removedIds
  if (removedIds.length > 0) {
    useTransitionsStore.getState()._removeTransitionsForItems(removedIds)
    useKeyframesStore.getState()._removeKeyframesForItems(removedIds)
  }
  applyTransitionRepairs(
    Array.from(new Set([...updates.map((update) => update.id), ...syncLockResult.affectedIds])),
    removedIds.length > 0 ? new Set(removedIds) : undefined,
  )
  
}

function closeAllGaps(trackId: string, linkedSelectionEnabled: boolean): void {
  const items = useItemsStore.getState().items
  const trackItems = items
    .filter((item) => item.trackId === trackId)
    .sort((left, right) => left.from - right.from)
  if (trackItems.length === 0) return

  let cursor = 0
  const baseShiftByItemId = new Map<string, number>()
  for (const item of trackItems) {
    const newFrom = item.from > cursor ? cursor : item.from
    const shiftAmount = item.from - newFrom
    if (shiftAmount > 0) baseShiftByItemId.set(item.id, shiftAmount)
    cursor = newFrom + item.durationInFrames
  }

  const updates = buildLinkedLeftShiftUpdates(items, baseShiftByItemId, linkedSelectionEnabled)
  if (updates.length === 0) return

  useItemsStore.getState()._moveItems(updates)
  applyTransitionRepairs(updates.map((update) => update.id))
  
}

// ---------------------------------------------------------------------------
// Duplicate bookkeeping (transitions, keyframes, property links)
// ---------------------------------------------------------------------------

function copyInternalTransitionsForDuplicatedItems(
  ctx: ApplyContext,
  itemIds: string[],
  newItems: TimelineItem[],
): void {
  if (newItems.length === 0) return

  const duplicatedItemByOriginalId = new Map<string, TimelineItem>()
  for (let index = 0; index < itemIds.length; index += 1) {
    const newItem = newItems[index]
    if (newItem) duplicatedItemByOriginalId.set(itemIds[index]!, newItem)
  }

  const copiedTransitions: Transition[] = []
  for (const transition of useTransitionsStore.getState().transitions) {
    const leftClip = duplicatedItemByOriginalId.get(transition.leftClipId)
    const rightClip = duplicatedItemByOriginalId.get(transition.rightClipId)
    if (!leftClip || !rightClip || leftClip.trackId !== rightClip.trackId) continue
    copiedTransitions.push({
      ...transition,
      id: ctx.ids.mint(),
      leftClipId: leftClip.id,
      rightClipId: rightClip.id,
      trackId: leftClip.trackId,
    })
  }

  if (copiedTransitions.length > 0) {
    useTransitionsStore
      .getState()
      .setTransitions([...useTransitionsStore.getState().transitions, ...copiedTransitions])
  }
}

function cloneEasingConfig(config: EasingConfig | undefined): EasingConfig | undefined {
  if (!config) return undefined
  return {
    ...config,
    ...(config.bezier && { bezier: { ...config.bezier } }),
    ...(config.spring && { spring: { ...config.spring } }),
  }
}

function cloneScalarKeyframe(keyframe: Keyframe, mint: () => string): Keyframe {
  return { ...keyframe, id: mint(), easingConfig: cloneEasingConfig(keyframe.easingConfig) }
}

function cloneVectorKeyframeForDuplicate(keyframe: VectorKeyframe, mint: () => string): VectorKeyframe {
  return {
    ...keyframe,
    id: mint(),
    value: { ...keyframe.value },
    easingConfig: cloneEasingConfig(keyframe.easingConfig),
    temporalEase: keyframe.temporalEase
      ? {
          ...(keyframe.temporalEase.in && { in: { ...keyframe.temporalEase.in } }),
          ...(keyframe.temporalEase.out && { out: { ...keyframe.temporalEase.out } }),
        }
      : undefined,
    spatial: keyframe.spatial
      ? {
          ...keyframe.spatial,
          inTangent: { ...keyframe.spatial.inTangent },
          outTangent: { ...keyframe.spatial.outTangent },
        }
      : undefined,
  }
}

function clonePropertyKeyframes(property: PropertyKeyframes, mint: () => string): PropertyKeyframes {
  return {
    property: property.property,
    keyframes: property.keyframes.map((keyframe) => cloneScalarKeyframe(keyframe, mint)),
  }
}

function cloneVectorPropertyKeyframes(
  property: VectorPropertyKeyframes,
  mint: () => string,
): VectorPropertyKeyframes {
  return {
    property: property.property,
    keyframes: property.keyframes.map((keyframe) => cloneVectorKeyframeForDuplicate(keyframe, mint)),
  }
}

function remapDirectPropertyLink(
  link: DirectPropertyLink,
  duplicatedItemIdMap: Map<string, string>,
): DirectPropertyLink {
  return { ...link, sourceItemId: duplicatedItemIdMap.get(link.sourceItemId) ?? link.sourceItemId }
}

function cloneStoredExpression(
  expression: StoredPropertyExpression,
  duplicatedItemIdMap: Map<string, string>,
): StoredPropertyExpression {
  return expression.type === 'link'
    ? remapDirectPropertyLink(expression, duplicatedItemIdMap)
    : { ...expression }
}

function cloneItemKeyframes(
  source: ItemKeyframes,
  itemId: string,
  duplicatedItemIdMap: Map<string, string>,
  mint: () => string,
): ItemKeyframes {
  const clone: ItemKeyframes = {
    itemId,
    properties: source.properties.map((property) => clonePropertyKeyframes(property, mint)),
  }
  if (source.animationVersion) clone.animationVersion = source.animationVersion
  if (source.expressions?.length) {
    clone.expressions = source.expressions.map((expression) =>
      cloneStoredExpression(expression, duplicatedItemIdMap),
    )
  }
  if (source.propertyLinks?.length) {
    clone.propertyLinks = source.propertyLinks.map((link) =>
      remapDirectPropertyLink(link, duplicatedItemIdMap),
    )
  }
  if (source.vectorProperties?.length) {
    clone.vectorProperties = source.vectorProperties.map((property) =>
      cloneVectorPropertyKeyframes(property, mint),
    )
  }
  if (source.separatedVectorProperties?.length) {
    clone.separatedVectorProperties = [...source.separatedVectorProperties]
  }
  return clone
}

function copyKeyframesForDuplicatedItems(
  ctx: ApplyContext,
  itemIds: string[],
  newItems: TimelineItem[],
): void {
  if (newItems.length === 0) return

  const keyframesState = useKeyframesStore.getState()
  const duplicatedItemIdMap = new Map(
    itemIds.flatMap((itemId, index) => {
      const newItem = newItems[index]
      return newItem ? [[itemId, newItem.id] as const] : []
    }),
  )
  const mint = () => ctx.ids.mint()
  const copiedKeyframes = itemIds.flatMap((itemId, index) => {
    const newItem = newItems[index]
    const source = keyframesState.keyframesByItemId[itemId]
    if (!newItem || !source) return []
    return [cloneItemKeyframes(source, newItem.id, duplicatedItemIdMap, mint)]
  })

  if (copiedKeyframes.length > 0) {
    keyframesState.setKeyframes([...keyframesState.keyframes, ...copiedKeyframes])
  }
}

// ---------------------------------------------------------------------------
// Reverse
// ---------------------------------------------------------------------------

/** Expand ids over synchronized linked items and pick the video/audio items that can reverse. */
export function resolveReverseTargets(
  items: readonly TimelineItem[],
  ids: readonly string[],
): { reversibleItems: TimelineItem[]; shouldReverse: boolean } {
  const expandedIds = new Set<string>()
  for (const itemId of ids) {
    const synchronized = getSynchronizedLinkedItems(items as TimelineItem[], itemId)
    if (synchronized.length === 0) {
      expandedIds.add(itemId)
      continue
    }
    for (const item of synchronized) expandedIds.add(item.id)
  }
  const reversibleItems = Array.from(expandedIds)
    .map((itemId) => items.find((item) => item.id === itemId))
    .filter(
      (item): item is TimelineItem =>
        item !== undefined && (item.type === 'video' || item.type === 'audio'),
    )
  return {
    reversibleItems,
    shouldReverse: !reversibleItems.every((item) => item.isReversed === true),
  }
}

// ---------------------------------------------------------------------------
// Transform parenting
// ---------------------------------------------------------------------------

type TransformParentUpdateContext = {
  parentItemId?: string
  parentWorld?: ResolvedTransform
  behavior: TransformParentingBehavior
  frame: number
  canvas: CanvasSettings
  getItem: (itemId: string) => TimelineItem | undefined
  getKeyframes: (itemId: string) => ItemKeyframes | undefined
}

type TransformParentUpdateResult =
  | { status: 'invalid' | 'unchanged' }
  | { status: 'update'; binding: TransformParentBinding | undefined }

function isInvalidTransformParentUpdate(
  childItemId: string,
  child: TimelineItem | undefined,
  context: TransformParentUpdateContext,
): boolean {
  if (!child || !canParticipateInTransformHierarchy(child)) return true
  return Boolean(
    context.parentItemId &&
      (wouldCreateTransformParentCycle(childItemId, context.parentItemId, context.getItem, context.getKeyframes) ||
        hasRedundantTransformParentLink(childItemId, context.parentItemId, context.getItem, context.getKeyframes)),
  )
}

function getTransformParentEarlyResult(
  childItemId: string,
  child: TimelineItem | undefined,
  context: TransformParentUpdateContext,
): TransformParentUpdateResult | null {
  if (isInvalidTransformParentUpdate(childItemId, child, context)) return { status: 'invalid' }
  if (context.behavior === 'preserve-world' && child?.transformParent?.parentItemId === context.parentItemId) {
    return { status: 'unchanged' }
  }
  if (!context.parentItemId && context.behavior === 'restore-local') {
    return child?.transformParent ? { status: 'update', binding: undefined } : { status: 'unchanged' }
  }
  return null
}

function chooseChildWorldReference(
  behavior: TransformParentingBehavior,
  childWorld: ResolvedTransform,
  childLocal: ResolvedTransform,
  parentWorld: ResolvedTransform | undefined,
): ResolvedTransform {
  if (behavior === 'snap-to-parent' && parentWorld) {
    return { ...childWorld, x: parentWorld.x, y: parentWorld.y }
  }
  return behavior === 'restore-local' ? childLocal : childWorld
}

function buildTransformParentUpdate(
  childItemId: string,
  context: TransformParentUpdateContext,
): TransformParentUpdateResult {
  const { parentItemId, parentWorld, behavior, frame: atFrame, canvas, getItem, getKeyframes } = context
  const child = getItem(childItemId)
  const earlyResult = getTransformParentEarlyResult(childItemId, child, context)
  if (earlyResult) return earlyResult
  if (!child) return { status: 'invalid' }

  const childKeyframes = getKeyframes(child.id)
  const childWorld = resolveItemTransformAtFrame(child, {
    canvas,
    frame: atFrame,
    keyframes: childKeyframes,
    getItem,
    getKeyframes,
  })
  const childLocal = resolveItemTransformAtRelativeFrame(child, {
    canvas,
    relativeFrame: atFrame - child.from,
    keyframes: childKeyframes,
    expressionContext: { globalFrame: atFrame, canvas, getItem, getKeyframes },
  })
  return {
    status: 'update',
    binding: createTransformParentBinding({
      childLocal,
      childWorld: chooseChildWorldReference(behavior, childWorld, childLocal, parentWorld),
      parentItemId,
      parentWorld,
    }),
  }
}

function collectTransformParentUpdates(
  childItemIds: string[],
  context: TransformParentUpdateContext,
): Map<string, TransformParentBinding | undefined> | null {
  const updates = new Map<string, TransformParentBinding | undefined>()
  for (const childItemId of childItemIds) {
    const result = buildTransformParentUpdate(childItemId, context)
    if (result.status === 'invalid') return null
    if (result.status === 'update') updates.set(childItemId, result.binding)
  }
  return updates
}

function createTransformParentUpdateContext(params: {
  parentItemId?: string
  parent?: TimelineItem
  behavior?: TransformParentingBehavior
  frame: number
  canvas: CanvasSettings
  getItem: (itemId: string) => TimelineItem | undefined
  getKeyframes: (itemId: string) => ItemKeyframes | undefined
}): TransformParentUpdateContext {
  const { parentItemId, parent, behavior = 'preserve-world', frame: atFrame, canvas, getItem, getKeyframes } = params
  const parentWorld = parent
    ? resolveItemTransformAtFrame(parent, {
        canvas,
        frame: atFrame,
        keyframes: getKeyframes(parent.id),
        getItem,
        getKeyframes,
      })
    : undefined
  return { parentItemId, parentWorld, behavior, frame: atFrame, canvas, getItem, getKeyframes }
}

function isValidTransformParentRequest(
  childItemIds: string[],
  parentItemId: string | undefined,
  parent: TimelineItem | undefined,
): boolean {
  if (childItemIds.length === 0) return false
  if (parentItemId && childItemIds.includes(parentItemId)) return false
  if (!parentItemId) return true
  return Boolean(parent && canParticipateInTransformHierarchy(parent))
}

function getUniqueNullObjectLabel(items: TimelineItem[]): string {
  const baseLabel = 'Null Object'
  const labels = new Set(items.map((item) => item.label))
  if (!labels.has(baseLabel)) return baseLabel
  let suffix = 2
  while (labels.has(`${baseLabel} ${suffix}`)) suffix += 1
  return `${baseLabel} ${suffix}`
}

function getNullParentTrackPlacement(
  children: TimelineItem[],
  tracks: TimelineTrack[],
): { order: number; parentTrackId?: string } {
  const trackById = new Map(tracks.map((track) => [track.id, track]))
  const childTracks = children
    .map((item) => trackById.get(item.trackId))
    .filter((track): track is TimelineTrack => Boolean(track))
  if (childTracks.length === 0) {
    return { order: tracks.reduce((max, track) => Math.max(max, track.order), -1) + 1 }
  }

  const parentTrackIds = new Set(childTracks.map((track) => track.parentTrackId ?? null))
  const parentTrackId = parentTrackIds.size === 1 ? (childTracks[0]?.parentTrackId ?? undefined) : undefined
  const anchors =
    parentTrackIds.size === 1
      ? childTracks
      : childTracks.map((track) => (track.parentTrackId && trackById.get(track.parentTrackId)) || track)
  const siblingTracks = tracks.filter((track) => (track.parentTrackId ?? null) === (parentTrackId ?? null))
  const anchorOrders = anchors
    .filter((track) => (track.parentTrackId ?? null) === (parentTrackId ?? null))
    .map((track) => track.order)
  const order =
    anchorOrders.length > 0
      ? Math.min(...anchorOrders)
      : siblingTracks.reduce((max, track) => Math.max(max, track.order), -1) + 1
  return { order, ...(parentTrackId && { parentTrackId }) }
}

function createNullParentLayer(
  ctx: ApplyContext,
  params: { children: TimelineItem[]; items: TimelineItem[]; tracks: TimelineTrack[]; canvas: CanvasSettings },
): { parent: ControllerItem; track: TimelineTrack } {
  const { children, items, tracks, canvas } = params
  const trackId = ctx.ids.mint()
  const from = Math.min(...children.map((item) => item.from))
  const to = Math.max(...children.map((item) => item.from + item.durationInFrames))
  const label = getUniqueNullObjectLabel(items)
  const placement = getNullParentTrackPlacement(children, tracks)
  const parent: ControllerItem = {
    ...createDefaultControllerItem({
      trackId,
      from,
      durationInFrames: Math.max(1, to - from),
      canvasWidth: canvas.width,
      canvasHeight: canvas.height,
      fps: canvas.fps,
    }),
    id: ctx.ids.mint(),
    label,
  }
  const track: TimelineTrack = {
    id: trackId,
    name: label,
    kind: 'video',
    height: 34,
    locked: false,
    syncLock: true,
    visible: true,
    muted: false,
    solo: false,
    order: placement.order,
    ...(placement.parentTrackId && { parentTrackId: placement.parentTrackId }),
    items: [],
  }
  return { parent, track }
}

function itemGetters(): {
  getItem: (itemId: string) => TimelineItem | undefined
  getKeyframes: (itemId: string) => ItemKeyframes | undefined
} {
  const itemsStore = useItemsStore.getState()
  const keyframesStore = useKeyframesStore.getState()
  return {
    getItem: (itemId) => itemsStore.itemById[itemId],
    getKeyframes: (itemId) => keyframesStore.keyframesByItemId[itemId],
  }
}

// ---------------------------------------------------------------------------
// Transforms + auto keyframes
// ---------------------------------------------------------------------------

function applyAutoKeyframeOperations(operations: readonly AutoKeyframeOperation[]): boolean {
  if (operations.length === 0) return false
  const keyframesStore = useKeyframesStore.getState()
  let changed = false

  for (const operation of operations) {
    if (operation.type === 'vector-update') {
      keyframesStore._updateVectorKeyframe(operation.itemId, operation.property, operation.keyframeId, operation.updates)
      changed = true
      continue
    }
    if (operation.type === 'update') {
      keyframesStore._updateKeyframe(operation.itemId, operation.property, operation.keyframeId, operation.updates)
      changed = true
      continue
    }
    if (!canAddKeyframeAtFrame(operation.itemId, operation.frame)) {
      getLogger().warn('Cannot add auto keyframe in transition region', {
        itemId: operation.itemId,
        property: operation.property,
        frame: operation.frame,
      })
      continue
    }
    if (operation.type === 'vector-add') {
      keyframesStore._upsertVectorKeyframe(operation.itemId, operation.property, {
        frame: operation.frame,
        value: operation.value,
        easing: operation.easing,
      })
    } else {
      keyframesStore._addKeyframe(operation.itemId, operation.property, operation.frame, operation.value, operation.easing)
    }
    changed = true
  }
  return changed
}

/** Transform properties that bento layout controls (cleared from keyframes) */
const BENTO_PROPERTIES: AnimatableProperty[] = ['x', 'y', 'width', 'height', 'rotation']

function transformKeysKey(transform: Record<string, unknown>): string {
  return Object.keys(transform).sort().join(',')
}

function validateKeyframeOperationItems(
  state: ProjectStateView,
  operations: readonly { itemId: string }[] | undefined,
): CommandIssue[] {
  return missingItems(state, [...new Set((operations ?? []).map((operation) => operation.itemId))], 'autoKeyframeOperations')
}

// ---------------------------------------------------------------------------
// Project-wide item update/removal (current timeline + compositions + nav stash)
// ---------------------------------------------------------------------------

function sanitizeSnapshotByItemIds<TSnapshot extends TimelineSnapshotLike>(
  snapshot: TSnapshot,
  targetIds: ReadonlySet<string>,
): TSnapshot & { removedItemIds: string[] } {
  const removedItemIds = snapshot.items.filter((item) => targetIds.has(item.id)).map((item) => item.id)
  if (removedItemIds.length === 0) return { ...snapshot, removedItemIds }

  const removedIdSet = new Set(removedItemIds)
  return {
    ...snapshot,
    items: snapshot.items.filter((item) => !removedIdSet.has(item.id)),
    transitions: snapshot.transitions.filter(
      (transition) => !removedIdSet.has(transition.leftClipId) && !removedIdSet.has(transition.rightClipId),
    ),
    keyframes: snapshot.keyframes.filter((keyframe) => !removedIdSet.has(keyframe.itemId)),
    removedItemIds,
  }
}

function applyItemUpdates(
  items: TimelineItem[],
  itemId: string,
  updates: Partial<TimelineItem>,
): { items: TimelineItem[]; changed: boolean } {
  let changed = false
  const nextItems = items.map((item) => {
    if (item.id !== itemId) return item
    changed = true
    return { ...item, ...updates } as TimelineItem
  })
  return { items: changed ? nextItems : items, changed }
}

function removeProjectItemsEverywhere(targetIds: string[]): boolean {
  const targetIdSet = new Set(targetIds)
  const currentSnapshot = getCurrentTimelineSnapshot()
  const sanitizedCurrent = sanitizeSnapshotByItemIds(currentSnapshot, targetIdSet)
  let removedAny = sanitizedCurrent.removedItemIds.length > 0

  if (sanitizedCurrent.removedItemIds.length > 0) {
    useItemsStore.getState().setItems(sanitizedCurrent.items)
    useTransitionsStore.getState().setTransitions(sanitizedCurrent.transitions)
    useKeyframesStore.getState().setKeyframes(sanitizedCurrent.keyframes)
  }

  const activeCompositionId = useCompositionNavigationStore.getState().activeCompositionId
  const nextCompositions = useCompositionsStore.getState().compositions.map((composition) => {
    if (composition.id === activeCompositionId && sanitizedCurrent.removedItemIds.length > 0) {
      return {
        ...composition,
        items: sanitizedCurrent.items,
        tracks: sanitizedCurrent.tracks,
        transitions: sanitizedCurrent.transitions,
        keyframes: sanitizedCurrent.keyframes,
      }
    }

    const baseSnapshot: TimelineSnapshotLike =
      composition.id === activeCompositionId
        ? {
            items: sanitizedCurrent.items,
            tracks: sanitizedCurrent.tracks,
            transitions: sanitizedCurrent.transitions,
            keyframes: sanitizedCurrent.keyframes,
          }
        : {
            items: composition.items,
            tracks: composition.tracks,
            transitions: composition.transitions,
            keyframes: composition.keyframes,
          }

    const sanitizedComposition = sanitizeSnapshotByItemIds(baseSnapshot, targetIdSet)
    if (sanitizedComposition.removedItemIds.length > 0) removedAny = true

    return sanitizedComposition.removedItemIds.length === 0
      ? composition
      : {
          ...composition,
          items: sanitizedComposition.items,
          tracks: sanitizedComposition.tracks,
          transitions: sanitizedComposition.transitions,
          keyframes: sanitizedComposition.keyframes,
        }
  })

  const latestNavState = useCompositionNavigationStore.getState()
  const nextStashStack = latestNavState.stashStack.map((stash) => {
    const sanitizedStash = sanitizeSnapshotByItemIds(stash, targetIdSet)
    if (sanitizedStash.removedItemIds.length > 0) removedAny = true
    return sanitizedStash.removedItemIds.length === 0
      ? stash
      : {
          ...stash,
          items: sanitizedStash.items,
          transitions: sanitizedStash.transitions,
          keyframes: sanitizedStash.keyframes,
        }
  })

  // Main, held aside while a sequence tab is active, must lose the items too.
  const mainHolder = latestNavState.mainHolder
  let nextMainHolder = mainHolder
  if (mainHolder) {
    const sanitizedMain = sanitizeSnapshotByItemIds(mainHolder, targetIdSet)
    if (sanitizedMain.removedItemIds.length > 0) {
      removedAny = true
      nextMainHolder = {
        ...mainHolder,
        items: sanitizedMain.items,
        transitions: sanitizedMain.transitions,
        keyframes: sanitizedMain.keyframes,
      }
    }
  }

  if (removedAny) {
    useCompositionsStore.getState().setCompositions(nextCompositions)
    useCompositionNavigationStore.setState((state) => ({
      ...state,
      stashStack: nextStashStack,
      mainHolder: nextMainHolder,
    }))
    
  }
  return removedAny
}

function updateProjectItemEverywhere(itemId: string, updates: Partial<TimelineItem>): boolean {
  const currentSnapshot = getCurrentTimelineSnapshot()
  const updatedCurrent = applyItemUpdates(currentSnapshot.items, itemId, updates)
  let changed = updatedCurrent.changed

  if (updatedCurrent.changed) useItemsStore.getState().setItems(updatedCurrent.items)

  const activeCompositionId = useCompositionNavigationStore.getState().activeCompositionId
  const nextCompositions = useCompositionsStore.getState().compositions.map((composition) => {
    if (composition.id === activeCompositionId && updatedCurrent.changed) {
      return { ...composition, items: updatedCurrent.items }
    }
    const sourceItems = composition.id === activeCompositionId ? updatedCurrent.items : composition.items
    const nextItems = applyItemUpdates(sourceItems, itemId, updates)
    if (nextItems.changed) changed = true
    return nextItems.changed ? { ...composition, items: nextItems.items } : composition
  })

  const latestNavState = useCompositionNavigationStore.getState()
  const nextStashStack = latestNavState.stashStack.map((stash) => {
    const nextItems = applyItemUpdates(stash.items, itemId, updates)
    if (nextItems.changed) changed = true
    return nextItems.changed ? { ...stash, items: nextItems.items } : stash
  })

  // Main, held aside while a sequence tab is active, must see the update too.
  const mainHolder = latestNavState.mainHolder
  let nextMainHolder = mainHolder
  if (mainHolder) {
    const nextItems = applyItemUpdates(mainHolder.items, itemId, updates)
    if (nextItems.changed) {
      changed = true
      nextMainHolder = { ...mainHolder, items: nextItems.items }
    }
  }

  if (!changed) return false

  useCompositionsStore.getState().setCompositions(nextCompositions)
  useCompositionNavigationStore.setState((state) => ({
    ...state,
    stashStack: nextStashStack,
    mainHolder: nextMainHolder,
  }))

  const positionChanged = 'from' in updates || 'durationInFrames' in updates || 'trackId' in updates
  if (positionChanged && updatedCurrent.changed) applyTransitionRepairs([itemId])

  
  return true
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

const updatePayload = { updates: jsonRecord, clearFields: z.array(z.string().min(1)).optional() }

function validateUpdatePayload(updates: Record<string, unknown>, clear: string[] | undefined): CommandIssue[] {
  const issues: CommandIssue[] = []
  if ('templateAnchor' in updates && !z.enum(['full', 'start', 'end']).safeParse(updates.templateAnchor).success) {
    issues.push({ message: 'Mốc neo template không hợp lệ', path: 'updates.templateAnchor' })
  }
  if ('id' in updates || clear?.includes('id')) {
    issues.push({ message: 'Không thể đổi id của clip', code: 'item.idImmutable', path: 'updates.id' })
  }
  if ('from' in updates && !(typeof updates.from === 'number' && Number.isFinite(updates.from))) {
    issues.push({ message: 'from phải là số frame hợp lệ', path: 'updates.from' })
  }
  if (
    'durationInFrames' in updates &&
    !(typeof updates.durationInFrames === 'number' && Number.isFinite(updates.durationInFrames) && updates.durationInFrames > 0)
  ) {
    issues.push({ message: 'durationInFrames phải lớn hơn 0', path: 'updates.durationInFrames' })
  }
  return issues
}
const WAVEFORM_VALIDATED_FIELDS: Record<string, true> = { waveformVisualizer: true, type: true, shapeType: true, mediaId: true, isMask: true }



/** Every scope that can hold items: main timeline, open compositions, stashes, held main. */
function itemScopes(state: ProjectStateView): TimelineItem[][] {
  return [state.items, ...state.compositions.map(composition => composition.items), ...(state.stashStack ?? []).map(stash => stash.items), ...(state.mainHolder ? [state.mainHolder.items] : [])]
}


function validateLoopUpdate(state: ProjectStateView, itemId: string, updates: Record<string, unknown>, clear?: string[]): CommandIssue[] {
  if (updates.loop !== true) return []
  return itemScopes(state).flatMap(items => items.filter(item => item.id === itemId).flatMap(item => {
    // Lottie repeats its own animation; picture-only video source guards do not apply.
    if (item.type === 'lottie') return []
    if (item.type !== 'video') return [{ message: 'Chỉ clip video mới lặp được', code: 'item.loopVideoOnly', path: 'updates.loop' }]
    // The merged view must keep the video subtype for the window check.
    const merged: VideoItem = { ...item, ...(mergeClear(updates, clear) as Partial<VideoItem>) }
    const hasLinkedAudio = items.some(other => other.type === 'audio' && !!item.linkedGroupId && other.linkedGroupId === item.linkedGroupId)
    const barrier = videoLoopBarrier(merged, hasLinkedAudio)
    if (barrier === 'unmuted') return [{ message: 'Chỉ lặp được clip đã tắt tiếng', code: 'item.loopNeedsMuted', path: 'updates.loop' }]
    if (barrier === 'no-window') return [{ message: 'Clip cần biết khoảng nguồn để lặp', code: 'item.loopNeedsWindow', path: 'updates.loop' }]
    return []
  }))
}

/** `templateAnchor: 'full'` on a video clip makes the clone loop it — same preconditions as `loop`. */
function validateTemplateAnchorUpdate(state: ProjectStateView, itemId: string, updates: Record<string, unknown>, clear?: string[]): CommandIssue[] {
  return itemScopes(state).flatMap(items => items.filter(item => item.id === itemId).flatMap(item => {
    const merged = { ...item, ...mergeClear(updates, clear) } as TimelineItem
    if (merged.type !== 'video' || merged.templateAnchor !== 'full') return []
    const hasLinkedAudio = items.some(other => other.type === 'audio' && !!merged.linkedGroupId && other.linkedGroupId === merged.linkedGroupId)
    const barrier = videoLoopBarrier(merged, hasLinkedAudio)
    if (barrier === 'unmuted') return [{ message: 'Clip neo toàn video phải tắt tiếng', code: 'item.anchorNeedsMuted', path: 'updates.templateAnchor' }]
    if (barrier === 'no-window') return [{ message: 'Clip neo toàn video cần biết khoảng nguồn', code: 'item.anchorNeedsWindow', path: 'updates.templateAnchor' }]
    return []
  }))
}

/**
 * Side effects folded into an `item.update` that turns a looping clip back
 * into a normal one: unmuting clears `loop`, and a clip left longer than its
 * source window is trimmed back in the same command (one undo step).
 */
function loopSideEffectUpdates(item: TimelineItem, updates: Partial<TimelineItem>): Partial<TimelineItem> {
  if (item.type !== 'video' || item.loop !== true) return {}
  const merged = { ...item, ...updates }
  // Still looping requires loop on AND the clip muted; anything else ends the loop.
  if (merged.loop === true && merged.embeddedAudioMuted === true) return {}
  const extras: Partial<TimelineItem> = { loop: undefined }
  const timelineFps = useTimelineSettingsStore.getState().fps
  const sourceFps = item.sourceFps ?? timelineFps
  const speed = item.speed ?? 1
  const start = item.sourceStart ?? item.trimStart ?? item.offset ?? 0
  const end = item.sourceEnd ?? item.sourceDuration
  if (end !== undefined && Number.isFinite(end) && end > start && merged.durationInFrames !== undefined) {
    const maxDuration = Math.max(1, sourceToTimelineFrames(end - start, speed, sourceFps, timelineFps))
    if (merged.durationInFrames > maxDuration) extras.durationInFrames = maxDuration
  }
  return extras
}
function validateWaveformUpdate(state: ProjectStateView, itemId: string, updates: Record<string, unknown>, clear?: string[]): CommandIssue[] {
  let relevant = false
  for (const field in updates) if (WAVEFORM_VALIDATED_FIELDS[field]) { relevant = true; break }
  if (!relevant && clear) for (const field of clear) if (WAVEFORM_VALIDATED_FIELDS[field]) { relevant = true; break }
  if (!relevant) return []
  return itemScopes(state).flatMap(items => items.filter(item => item.id === itemId).flatMap(item => {
    const merged = { ...item, ...mergeClear(updates, clear) } as Record<string, unknown>
    const media = state.mediaItems?.find(media => media.id === merged.mediaId)
    const issue = waveformItemIssue(merged, media?.contentHash)
    if (merged.waveformVisualizer && (!media || !media.mimeType.startsWith('audio/'))) return [{ message: 'Chọn tệp âm thanh làm nguồn giọng', path: 'updates.mediaId' }]
    return issue ? [{ message: issue, path: 'updates.waveformVisualizer' }] : []
  }))
}

export function registerItemCommands(): void {
  // --- add ---------------------------------------------------------------
  registerCommand({
    type: 'item.add',
    params: z.object({ item: itemJsonSchema }).strict(),
    validate: (state, params) => validateNewItems(state, [params.item], state.tracks),
    apply(ctx, params) {
      const [placed] = placeItemsWithoutTimelineOverlap([materializeItem(ctx, params.item)])
      if (!placed) return { inverse: emptyInverse(), result: null }
      useItemsStore.getState()._addItem(placed)
      
      return { inverse: emptyInverse(), result: placed }
    },
  })
  registerCommand({
    type: 'item.addMany',
    params: z.object({ items: z.array(itemJsonSchema).min(1) }).strict(),
    validate: (state, params) => validateNewItems(state, params.items, state.tracks),
    apply(ctx, params) {
      const placed = placeItemsWithoutTimelineOverlap(params.items.map((item) => materializeItem(ctx, item)))
      useItemsStore.getState()._addItems(placed)
      
      return { inverse: emptyInverse(), result: placed }
    },
  })
  registerCommand({
    type: 'item.addWithLinkedAudio',
    params: z.object({ video: itemJsonSchema.refine((item) => item.type === 'video', { message: 'Chỉ hỗ trợ clip video' }).refine((item) => item.loop !== true, { message: 'Clip lặp không gắn âm thanh được' }) }).strict(),
    validate: (state, params) => validateNewItems(state, [params.video], state.tracks),
    apply(ctx, params) {
      const store = useItemsStore.getState()
      const itemsByTrackId = new Map<string, TimelineItem[]>()
      for (const item of store.items) {
        const existing = itemsByTrackId.get(item.trackId)
        if (existing) existing.push(item)
        else itemsByTrackId.set(item.trackId, [item])
      }
      const { updatedVideo, audioItem, newTrack } = buildLinkedAudioForVideo({
        video: materializeItem(ctx, params.video) as VideoItem,
        tracks: store.tracks,
        itemsByTrackId,
        createId: () => ctx.ids.mint(),
      })
      if (newTrack) store.setTracks([...store.tracks, newTrack])
      store._addItems([updatedVideo, audioItem])
      
      return { inverse: emptyInverse(), result: { video: updatedVideo, audio: audioItem, track: newTrack } }
    },
  })
  registerCommand({
    type: 'item.addOnNewTracks',
    params: z.object({ items: z.array(itemJsonSchema).min(1), tracks: z.array(trackJsonSchema).min(1) }).strict(),
    validate: (state, params) => validateNewItems(state, params.items, params.tracks),
    apply(ctx, params) {
      const store = useItemsStore.getState()
      const items = params.items.map((item) => materializeItem(ctx, item))
      store.setTracks(params.tracks as unknown as TimelineTrack[])
      store._addItems(items)
      
      return { inverse: emptyInverse(), result: items }
    },
  })

  // --- update ------------------------------------------------------------
  registerCommand({
    type: 'item.update',
    params: z.object({ id, ...updatePayload }).strict(),
    validate: (state, params) => [
      ...missingItems(state, [params.id], 'id'),
      ...validateUpdatePayload(params.updates, params.clearFields),
      ...validateWaveformUpdate(state, params.id, params.updates, params.clearFields),
      ...validateLoopUpdate(state, params.id, params.updates, params.clearFields),
      ...validateTemplateAnchorUpdate(state, params.id, params.updates, params.clearFields),
      ...(typeof params.updates.trackId === 'string' ? missingTracks(state.tracks, [params.updates.trackId], 'updates.trackId') : []),
    ],
    mergeKey: (params) => `item.update:${params.id}:${[...Object.keys(params.updates), ...(params.clearFields ?? [])].sort().join(',')}`,
    apply(_ctx, params) {
      const item = useItemsStore.getState().itemById[params.id]
      const updates = {
        ...mergeClear(params.updates, params.clearFields),
        ...(item ? loopSideEffectUpdates(item, mergeClear(params.updates, params.clearFields)) : {}),
      }
      useItemsStore.getState()._updateItem(params.id, updates)
      const positionChanged = 'from' in updates || 'durationInFrames' in updates || 'trackId' in updates
      if (positionChanged) applyTransitionRepairs([params.id])

      return emptyInverse()
    },
  })

  // --- move --------------------------------------------------------------
  registerCommand({
    type: 'item.move',
    params: itemMoveSchema,
    validate: (state, params) => [
      ...missingItems(state, [params.id], 'id'),
      ...missingTracks(state.tracks, [params.trackId], 'trackId'),
    ],
    mergeKey: (params) => `item.move:${params.id}`,
    apply(_ctx, params) {
      useItemsStore.getState()._moveItem(params.id, params.from, params.trackId)
      applyTransitionRepairs([params.id])
      
      warnIfOverlapping('item.move')
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.moveMany',
    params: z
      .object({ updates: z.array(itemMoveSchema).min(1), tracks: z.array(trackJsonSchema).optional() })
      .strict(),
    validate: (state, params) => [
      ...missingItems(state, params.updates.map((update) => update.id), 'updates.id'),
      ...duplicateIds(params.updates.map((update) => update.id), 'updates.id'),
      ...missingTracks(params.tracks ?? state.tracks, params.updates.map((update) => update.trackId), 'updates.trackId'),
    ],
    mergeKey: (params) =>
      `item.moveMany:${params.updates.map((update) => update.id).sort().join(',')}${params.tracks ? ':tracks' : ''}`,
    apply(_ctx, params) {
      const store = useItemsStore.getState()
      if (params.tracks) store.setTracks(params.tracks as unknown as TimelineTrack[])
      store._moveItems(params.updates)
      syncTransitionTracksAfterMove(new Set(params.updates.map((update) => update.id)))
      applyTransitionRepairs(params.updates.map((update) => update.id))
      
      warnIfOverlapping('item.moveMany')
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.trackPush',
    params: z.object({ anchorId: id, delta: z.number().finite().refine((value) => value !== 0, { message: 'delta phải khác 0' }) }).strict(),
    validate: (state, params) => missingItems(state, [params.anchorId], 'anchorId'),
    mergeKey: (params) => `item.trackPush:${params.anchorId}`,
    apply(_ctx, params) {
      const items = useItemsStore.getState().items
      const anchor = items.find((item) => item.id === params.anchorId)
      if (!anchor) return emptyInverse()
      const updates = items
        .filter((item) => item.from >= anchor.from)
        .map((item) => ({ id: item.id, from: Math.max(0, item.from + params.delta) }))
      if (updates.length === 0) return emptyInverse()
      useItemsStore.getState()._moveItems(updates)
      applyTransitionRepairs(updates.map((update) => update.id))
      
      return emptyInverse()
    },
  })

  // --- duplicate ---------------------------------------------------------
  registerCommand({
    type: 'item.duplicate',
    params: z
      .object({ itemIds: idList, positions: z.array(positionSchema).min(1), tracks: z.array(trackJsonSchema).optional() })
      .strict(),
    validate: (state, params) => [
      ...missingItems(state, params.itemIds),
      ...(params.itemIds.length === params.positions.length
        ? []
        : [{ message: 'positions phải cùng số lượng với itemIds', path: 'positions' }]),
      ...missingTracks(params.tracks ?? state.tracks, params.positions.map((position) => position.trackId), 'positions.trackId'),
    ],
    apply(ctx, params) {
      const store = useItemsStore.getState()
      if (params.tracks) store.setTracks(params.tracks as unknown as TimelineTrack[])
      const newItems = store._duplicateItems(params.itemIds, params.positions)
      copyInternalTransitionsForDuplicatedItems(ctx, params.itemIds, newItems)
      copyKeyframesForDuplicatedItems(ctx, params.itemIds, newItems)
      
      return { inverse: emptyInverse(), result: newItems }
    },
  })

  // --- remove ------------------------------------------------------------
  const removeIssues = (state: ProjectStateView, ids: readonly string[]): CommandIssue[] => {
    const known = new Set(state.items.map((item) => item.id))
    return ids.some((itemId) => known.has(itemId)) ? [] : [{ message: 'Không tìm thấy clip nào để xóa', code: 'item.notFound', path: 'itemIds' }]
  }
  registerCommand({
    type: 'item.remove',
    params: z.object({ itemIds: idList, linkedSelection: z.boolean() }).strict(),
    validate: (state, params) => removeIssues(state, params.itemIds),
    apply(_ctx, params) {
      const expandedIds = expandIdsWithLinkedItems(useItemsStore.getState().items, params.itemIds, params.linkedSelection)
      if (expandedIds.length > 0) removeItemsWithCascade(expandedIds)
      return { inverse: emptyInverse(), result: expandedIds }
    },
  })
  registerCommand({
    type: 'item.removeExact',
    params: z.object({ itemIds: idList }).strict(),
    validate: (state, params) => removeIssues(state, params.itemIds),
    apply(_ctx, params) {
      const known = new Set(useItemsStore.getState().items.map((item) => item.id))
      const ids = [...new Set(params.itemIds)].filter((itemId) => known.has(itemId))
      if (ids.length > 0) removeItemsWithCascade(ids)
      return { inverse: emptyInverse(), result: ids }
    },
  })
  registerCommand({
    type: 'item.rippleDelete',
    params: z.object({ itemIds: idList, linkedSelection: z.boolean() }).strict(),
    validate: (state, params) => removeIssues(state, params.itemIds),
    apply(_ctx, params) {
      const removed = rippleDeleteWithCascade(params.itemIds, params.linkedSelection)
      return { inverse: emptyInverse(), result: removed }
    },
  })
  registerCommand({
    type: 'item.closeGap',
    params: z.object({ trackId: id, frame }).strict(),
    validate: (state, params) => missingTracks(state.tracks, [params.trackId], 'trackId'),
    apply(_ctx, params) {
      closeGap(params.trackId, params.frame)
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.closeAllGaps',
    params: z.object({ trackId: id, linkedSelection: z.boolean() }).strict(),
    validate: (state, params) => missingTracks(state.tracks, [params.trackId], 'trackId'),
    apply(_ctx, params) {
      closeAllGaps(params.trackId, params.linkedSelection)
      return emptyInverse()
    },
  })

  // --- link / unlink -----------------------------------------------------
  registerCommand({
    type: 'item.link',
    params: z.object({ itemIds: idList }).strict(),
    validate: (state, params) => {
      const issues = missingItems(state, params.itemIds)
      if (issues.length > 0) return issues
      const selected = expandSelectionWithLinkedItems(state.items as TimelineItem[], params.itemIds)
      return canLinkSelection(state.items as TimelineItem[], params.itemIds) && selected.length >= 2
        ? []
        : [{ message: 'Không thể liên kết các clip đã chọn', code: 'item.cannotLink', path: 'itemIds' }]
    },
    apply(ctx, params) {
      const items = useItemsStore.getState().items
      const selectedItems = expandSelectionWithLinkedItems(items, params.itemIds)
        .map((itemId) => items.find((item) => item.id === itemId))
        .filter((item): item is TimelineItem => item !== undefined)
      const linkedGroupId = ctx.ids.mint()
      const store = useItemsStore.getState()
      const linksAudio = selectedItems.some((item) => item.type === 'audio')
      for (const item of selectedItems) {
        store._updateItem(item.id, {
          linkedGroupId,
          // Looping is picture-only; a clip that gains linked audio stops looping.
          ...(item.type === 'video' && { embeddedAudioMuted: undefined, ...(linksAudio && { loop: undefined }) }),
        })
      }
      
      return { inverse: emptyInverse(), result: selectedItems.map((item) => item.id) }
    },
  })
  registerCommand({
    type: 'item.unlink',
    params: z.object({ itemIds: idList }).strict(),
    validate: (state, params) => missingItems(state, params.itemIds),
    apply(_ctx, params) {
      const items = useItemsStore.getState().items
      const unlinkIds = new Set<string>()
      for (const itemId of params.itemIds) {
        for (const linkedId of getLinkedItemIds(items, itemId)) unlinkIds.add(linkedId)
      }
      const linkedItems = items.filter((item) => unlinkIds.has(item.id) && item.linkedGroupId)
      if (linkedItems.length === 0) return { inverse: emptyInverse(), result: [] }

      // A video with a linked audio companion has its embedded audio muted after
      // unlinking so it doesn't start playing when the audio is deleted.
      const videoIdsWithAudioCompanion = new Set<string>()
      for (const item of linkedItems) {
        if (item.type !== 'video') continue
        const hasAudio = linkedItems.some(
          (other) => other.type === 'audio' && other.linkedGroupId === item.linkedGroupId,
        )
        if (hasAudio) videoIdsWithAudioCompanion.add(item.id)
      }

      const store = useItemsStore.getState()
      for (const item of linkedItems) {
        store._updateItem(item.id, {
          linkedGroupId: item.id,
          ...(videoIdsWithAudioCompanion.has(item.id) && { embeddedAudioMuted: true }),
        })
      }
      
      return { inverse: emptyInverse(), result: linkedItems.map((item) => item.id) }
    },
  })

  // --- reverse -----------------------------------------------------------
  registerCommand({
    type: 'item.reverse',
    params: z.object({ itemIds: idList }).strict(),
    validate: (state, params) => {
      const issues = missingItems(state, params.itemIds)
      if (issues.length > 0) return issues
      return resolveReverseTargets(state.items, params.itemIds).reversibleItems.length > 0
        ? []
        : [{ message: 'Chỉ clip video hoặc âm thanh mới đảo ngược được', code: 'item.notReversible', path: 'itemIds' }]
    },
    apply(_ctx, params) {
      const { reversibleItems, shouldReverse } = resolveReverseTargets(useItemsStore.getState().items, params.itemIds)
      const store = useItemsStore.getState()
      for (const item of reversibleItems) {
        const updates: Record<string, unknown> = { isReversed: shouldReverse ? true : undefined }
        if (!shouldReverse) for (const key of REVERSE_CLEAR_KEYS) updates[key] = undefined
        store._updateItem(item.id, updates as Partial<TimelineItem>)
      }
      
      return { inverse: emptyInverse(), result: { ids: reversibleItems.map((item) => item.id), reversed: shouldReverse } }
    },
  })
  registerCommand({
    type: 'item.commitPreparedReverse',
    params: z
      .object({ entries: z.array(z.object({ itemId: id, conform: reverseConformSchema.optional() }).strict()).min(1) })
      .strict(),
    validate: (state, params) => missingItems(state, params.entries.map((entry) => entry.itemId), 'entries.itemId'),
    apply(_ctx, params) {
      const store = useItemsStore.getState()
      for (const entry of params.entries) {
        const item = store.itemById[entry.itemId]
        if (!item) continue
        const result = entry.conform
        const conformFields =
          item.type === 'video' && result
            ? {
                ...(result.quality === 'full'
                  ? { reverseConformSrc: result.src, reverseConformPath: result.path, reverseConformKey: result.key }
                  : {
                      reverseConformPreviewSrc: result.src,
                      reverseConformPreviewPath: result.path,
                      reverseConformPreviewKey: result.key,
                      reverseConformPreviewUsesProxy: result.usesProxy,
                      reverseConformPreviewIsSourceLevel: result.isSourceLevel,
                      reverseConformPreviewSourceDuration: result.sourceDuration,
                      reverseConformPreviewFps: result.conformFps,
                    }),
                reverseConformStatus: 'ready' as const,
              }
            : {}
        store._updateItem(item.id, { isReversed: true, ...conformFields } as Partial<TimelineItem>)
      }
      
      return emptyInverse()
    },
  })

  // --- transform parenting ----------------------------------------------
  const parentParams = z
    .object({
      childItemIds: idList,
      parentItemId: id.optional(),
      behavior: z.enum(['preserve-world', 'snap-to-parent', 'restore-local']).optional(),
      frame,
      canvas: canvasSchema,
    })
    .strict()
  registerCommand({
    type: 'item.setTransformParents',
    params: parentParams,
    validate: (state, params) => {
      const issues = missingItems(state, [...params.childItemIds, ...(params.parentItemId ? [params.parentItemId] : [])])
      if (issues.length > 0) return issues
      const parent = state.items.find((item) => item.id === params.parentItemId)
      return isValidTransformParentRequest([...new Set(params.childItemIds)], params.parentItemId, parent)
        ? []
        : [{ message: 'Yêu cầu gán parent transform không hợp lệ', code: 'item.invalidTransformParent', path: 'parentItemId' }]
    },
    apply(_ctx, params) {
      const childItemIds = [...new Set(params.childItemIds)]
      const store = useItemsStore.getState()
      const parent = params.parentItemId ? store.itemById[params.parentItemId] : undefined
      const { getItem, getKeyframes } = itemGetters()
      const context = createTransformParentUpdateContext({
        parentItemId: params.parentItemId,
        parent,
        behavior: params.behavior,
        frame: params.frame,
        canvas: params.canvas,
        getItem,
        getKeyframes,
      })
      const updates = collectTransformParentUpdates(childItemIds, context)
      if (!updates || updates.size === 0) return { inverse: emptyInverse(), result: false }
      for (const [childItemId, transformParent] of updates) {
        store._updateItem(childItemId, { transformParent })
      }
      
      return { inverse: emptyInverse(), result: true }
    },
  })
  registerCommand({
    type: 'item.createNullParent',
    params: z.object({ childItemIds: idList, frame, canvas: canvasSchema }).strict(),
    validate: (state, params) => {
      const issues = missingItems(state, params.childItemIds, 'childItemIds')
      if (issues.length > 0) return issues
      return params.childItemIds.every((childId) => {
        const child = state.items.find((item) => item.id === childId)
        return child !== undefined && canParticipateInTransformHierarchy(child)
      })
        ? []
        : [{ message: 'Clip này không thể có parent transform', code: 'item.invalidTransformParent', path: 'childItemIds' }]
    },
    apply(ctx, params) {
      const childItemIds = [...new Set(params.childItemIds)]
      const store = useItemsStore.getState()
      const children = childItemIds.map((childId) => store.itemById[childId]).filter((item): item is TimelineItem => item !== undefined)
      if (children.length !== childItemIds.length) return { inverse: emptyInverse(), result: null }

      const { parent, track } = createNullParentLayer(ctx, {
        children,
        items: store.items,
        tracks: store.tracks,
        canvas: params.canvas,
      })
      const { getItem: getStoreItem, getKeyframes } = itemGetters()
      const context = createTransformParentUpdateContext({
        parentItemId: parent.id,
        parent,
        frame: params.frame,
        canvas: params.canvas,
        getItem: (itemId) => (itemId === parent.id ? parent : getStoreItem(itemId)),
        getKeyframes,
      })
      const updates = collectTransformParentUpdates(childItemIds, context)
      if (!updates || updates.size !== childItemIds.length) return { inverse: emptyInverse(), result: null }

      store.setTracks([
        ...store.tracks.map((candidate) =>
          (candidate.parentTrackId ?? null) === (track.parentTrackId ?? null) && candidate.order >= track.order
            ? { ...candidate, order: candidate.order + 1 }
            : candidate,
        ),
        track,
      ])
      store._addItem(parent)
      for (const [childItemId, transformParent] of updates) {
        store._updateItem(childItemId, { transformParent })
      }
      
      return { inverse: emptyInverse(), result: parent }
    },
  })

  // --- transforms --------------------------------------------------------
  registerCommand({
    type: 'item.updateTransform',
    params: z
      .object({
        id,
        transform: transformSchema,
        operation: transformOperationSchema.optional(),
        autoKeyframeOperations: z.array(autoKeyframeOperationSchema).optional(),
      })
      .strict(),
    validate: (state, params) => [
      ...transformItemIssues(state, [params.id], 'id'),
      ...validateKeyframeOperationItems(state, params.autoKeyframeOperations),
    ],
    mergeKey: (params) => `item.updateTransform:${params.id}:${transformKeysKey(params.transform)}:${params.autoKeyframeOperations?.length ? 'kf' : ''}`,
    apply(_ctx, params) {
      if (Object.keys(params.transform).length > 0) {
        useItemsStore.getState()._updateItemTransform(params.id, params.transform as Partial<TransformProperties>)
      }
      applyAutoKeyframeOperations((params.autoKeyframeOperations ?? []) as AutoKeyframeOperation[])
      
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.commitMaskEdit',
    params: z
      .object({
        id,
        pathVertices: z.array(maskVertexSchema).optional(),
        transform: transformSchema.optional(),
        operation: transformOperationSchema.optional(),
        autoKeyframeOperations: z.array(autoKeyframeOperationSchema).optional(),
      })
      .strict(),
    validate: (state, params) => [
      ...transformItemIssues(state, [params.id], 'id'),
      ...validateKeyframeOperationItems(state, params.autoKeyframeOperations),
    ],
    mergeKey: (params) => `item.commitMaskEdit:${params.id}:${params.pathVertices ? 'path' : ''}:${transformKeysKey(params.transform ?? {})}:${params.autoKeyframeOperations?.length ? 'kf' : ''}`,
    apply(_ctx, params) {
      const store = useItemsStore.getState()
      if (params.pathVertices) {
        store._updateItem(params.id, { pathVertices: params.pathVertices as MaskVertex[] } as Partial<TimelineItem>)
      }
      if (params.transform && Object.keys(params.transform).length > 0) {
        store._updateItemTransform(params.id, params.transform as Partial<TransformProperties>)
      }
      applyAutoKeyframeOperations((params.autoKeyframeOperations ?? []) as AutoKeyframeOperation[])
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.resetTransform',
    params: z.object({ id }).strict(),
    validate: (state, params) => transformItemIssues(state, [params.id], 'id'),
    apply(_ctx, params) {
      useItemsStore.getState()._resetItemTransform(params.id)
      
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.updateTransforms',
    params: z.object({ ids: idList, transform: transformSchema, operation: transformOperationSchema.optional() }).strict(),
    validate: (state, params) => transformItemIssues(state, params.ids, 'ids'),
    mergeKey: (params) => `item.updateTransforms:${[...params.ids].sort().join(',')}:${transformKeysKey(params.transform)}`,
    apply(_ctx, params) {
      useItemsStore.getState()._updateItemsTransform(params.ids, params.transform as Partial<TransformProperties>)
      
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.updateTransformsMap',
    params: z
      .object({
        entries: z.array(z.object({ id, transform: transformSchema }).strict()),
        itemUpdates: z.array(z.object({ id, ...updatePayload }).strict()).optional(),
        operation: transformOperationSchema.optional(),
        autoKeyframeOperations: z.array(autoKeyframeOperationSchema).optional(),
      })
      .strict(),
    validate: (state, params) => [
      ...missingItems(state, [...params.entries.map((entry) => entry.id), ...(params.itemUpdates ?? []).map((entry) => entry.id)], 'entries'),
      ...transformItemIssues(state, params.entries.map(entry => entry.id), 'entries'),
      ...(params.itemUpdates ?? []).flatMap((entry) => validateUpdatePayload(entry.updates, entry.clearFields)),
      ...validateKeyframeOperationItems(state, params.autoKeyframeOperations),
    ],
    mergeKey: (params) =>
      `item.updateTransformsMap:${[...params.entries.map((entry) => entry.id), ...(params.itemUpdates ?? []).map((entry) => `u:${entry.id}`)].sort().join(',')}:${params.autoKeyframeOperations?.length ? 'kf' : ''}`,
    apply(_ctx, params) {
      const transformsMap = new Map(params.entries.map((entry) => [entry.id, entry.transform as Partial<TransformProperties>]))
      const itemUpdates = params.itemUpdates?.length
        ? new Map(params.itemUpdates.map((entry) => [entry.id, mergeClear(entry.updates, entry.clearFields)]))
        : undefined
      useItemsStore.getState()._updateItemsTransformMap(transformsMap, itemUpdates)
      applyAutoKeyframeOperations((params.autoKeyframeOperations ?? []) as AutoKeyframeOperation[])
      
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'item.applyBentoLayout',
    params: z
      .object({
        itemIds: z.array(id).min(2),
        canvasWidth: z.number().finite().positive(),
        canvasHeight: z.number().finite().positive(),
        config: layoutConfigSchema.optional(),
        /** Pre-ordered chains from dialog drag-swap; skips buildTransitionChains when provided. */
        orderedChains: z.array(z.array(id)).optional(),
      })
      .strict(),
    validate: (state, params) => [
      ...missingItems(state, params.itemIds),
      ...(params.itemIds.filter((itemId) => state.items.find((item) => item.id === itemId)?.type !== 'audio').length >= 2
        ? []
        : [{ message: 'Cần ít nhất 2 clip hình ảnh để xếp bố cục', code: 'item.bentoNeedsVisuals', path: 'itemIds' }]),
    ],
    apply(_ctx, params) {
      const items = useItemsStore.getState().items
      const visualItemIds = params.itemIds.filter((itemId) => {
        const item = items.find((candidate) => candidate.id === itemId)
        return item != null && item.type !== 'audio'
      })
      if (visualItemIds.length < 2) return emptyInverse()

      let chains: string[][]
      if (params.orderedChains && params.orderedChains.length > 0) {
        const visualSet = new Set(visualItemIds)
        chains = params.orderedChains
          .map((chain) => chain.filter((chainId) => visualSet.has(chainId)))
          .filter((chain) => chain.length > 0)
      } else {
        const { transitionsByClipId } = buildTransitionIndexes(useTransitionsStore.getState().transitions)
        chains = buildTransitionChains(visualItemIds, transitionsByClipId)
      }

      const layoutItems = chains.map((chain) => {
        const firstItem = items.find((candidate) => candidate.id === chain[0])
        const sourceWidth = (firstItem && 'sourceWidth' in firstItem && firstItem.sourceWidth) || params.canvasWidth
        const sourceHeight = (firstItem && 'sourceHeight' in firstItem && firstItem.sourceHeight) || params.canvasHeight
        return { id: chain[0]!, sourceWidth, sourceHeight }
      })
      const config: LayoutConfig = params.config ?? { preset: 'auto' }
      const chainTransformsMap = computeLayout(layoutItems, params.canvasWidth, params.canvasHeight, config)

      const transformsMap = new Map<string, TransformProperties>()
      for (const chain of chains) {
        const transform = chainTransformsMap.get(chain[0]!)
        if (!transform) continue
        for (const chainId of chain) transformsMap.set(chainId, transform)
      }

      const keyframesStore = useKeyframesStore.getState()
      for (const itemId of visualItemIds) {
        for (const property of BENTO_PROPERTIES) keyframesStore._removeKeyframesForProperty(itemId, property)
      }
      useItemsStore.getState()._updateItemsTransformMap(transformsMap)
      
      return emptyInverse()
    },
  })

  // --- project-wide (compositions, nav stash, mainHolder) -----------------
  registerCommand({
    type: 'item.updateProjectItem',
    params: z.object({ itemId: id, ...updatePayload }).strict(),
    validate: (state, params) => [...validateUpdatePayload(params.updates, params.clearFields), ...validateWaveformUpdate(state, params.itemId, params.updates, params.clearFields), ...validateLoopUpdate(state, params.itemId, params.updates, params.clearFields)],
    apply(_ctx, params) {
      const merged = mergeClear(params.updates, params.clearFields)
      const item = useItemsStore.getState().itemById[params.itemId]
      const changed = updateProjectItemEverywhere(params.itemId, {
        ...merged,
        ...(item ? loopSideEffectUpdates(item, merged) : {}),
      })
      return { inverse: emptyInverse(), result: changed }
    },
  })
  registerCommand({
    type: 'item.removeProjectItems',
    params: z.object({ itemIds: idList }).strict(),
    apply(_ctx, params) {
      const removed = removeProjectItemsEverywhere([...new Set(params.itemIds)])
      return { inverse: emptyInverse(), result: removed }
    },
  })
}
