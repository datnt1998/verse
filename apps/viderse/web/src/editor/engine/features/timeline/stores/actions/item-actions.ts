/**
 * Item Actions - Cross-domain operations that affect items, transitions, and keyframes.
 *
 * Every mutation is a registered command (see commands/handlers/item.ts); the functions
 * here keep their public signatures, translate arguments into JSON-only command params
 * and perform browser-only side effects (selection, UI sounds, reverse dialog) after the
 * command has been committed.
 */

import '@editor/commands'
import type { ControllerItem, TimelineItem, TimelineTrack, VideoItem } from '@editor/types/timeline'
import type { CanvasSettings, TransformParentingBehavior } from '@editor/types/transform'
import type { ReverseConformResult } from '../../services/reverse-conform-service'
import { useTimelineSettingsStore } from '../timeline-settings-store'
import { useEditorStore } from '@editor/shared/state/editor'
import { useSelectionStore } from '@editor/shared/state/selection'
import { emitUiSound } from '@editor/shared/ui/ui-sound'
import {
  resolveReverseTargets,
  runItemCommand,
  splitItemUpdates,
} from '@editor/commands/handlers/item'
import { useItemsStore } from '../items-store'
import { useReverseConformDialogStore } from '../reverse-conform-dialog-store'

function isLinkedSelectionEnabled(): boolean {
  return useEditorStore.getState().linkedSelectionEnabled
}

/** Attach, detach, or reparent visual layers with an explicit pose policy. */
export function setTransformParents(params: {
  childItemIds: string[]
  parentItemId?: string
  behavior?: TransformParentingBehavior
  frame: number
  canvas: CanvasSettings
}): boolean {
  const outcome = runItemCommand<boolean>('item.setTransformParents', params)
  return outcome.ok && outcome.result === true
}

/** Create one Null Object spanning the selected layers and parent them in one undo step. */
export function createNullParentForItems(params: {
  childItemIds: string[]
  frame: number
  canvas: CanvasSettings
}): ControllerItem | null {
  const outcome = runItemCommand<ControllerItem | null>('item.createNullParent', params)
  return outcome.ok ? outcome.result : null
}

/** Attach, detach, or reparent one visual layer while preserving its current world pose. */
export function setTransformParent(params: {
  childItemId: string
  parentItemId?: string
  behavior?: TransformParentingBehavior
  frame: number
  canvas: CanvasSettings
}): boolean {
  return setTransformParents({
    childItemIds: [params.childItemId],
    parentItemId: params.parentItemId,
    behavior: params.behavior,
    frame: params.frame,
    canvas: params.canvas,
  })
}

export function addItem(item: TimelineItem): void {
  runItemCommand('item.add', { item })
}

export function addItems(items: TimelineItem[]): void {
  if (items.length === 0) return
  runItemCommand('item.addMany', { items })
}

/**
 * Add a video together with a linked audio companion in one undoable step.
 *
 * Used by entry points that place a bare visual item (e.g. the preview canvas
 * drop): when the video carries embedded audio, its audio is split onto a
 * separate audio track so video and audio stay on their own tracks. The video
 * and audio placements are already collision-free (the caller chose the video
 * slot; the audio lands on a free/created audio track), so they're committed
 * directly without re-running overlap placement, which would desync the pair.
 */
export function addItemWithLinkedAudio(video: VideoItem): void {
  runItemCommand('item.addWithLinkedAudio', { video })
}

/**
 * Add a single item together with a freshly created track in one undoable step.
 *
 * Used for overlay layers (text/shape presets dropped on the canvas) that get
 * their own new track above existing content. The caller has already planned
 * the track (so `tracks` includes it) and positioned the item on that empty
 * track, so the placement is collision-free and is committed directly without
 * re-running overlap placement.
 */
export function addItemOnNewTrack(item: TimelineItem, tracks: TimelineTrack[]): void {
  runItemCommand('item.addOnNewTracks', { items: [item], tracks })
}

/** Add several layer items and their already-planned backing tracks atomically. */
export function addItemsOnNewTracks(items: TimelineItem[], tracks: TimelineTrack[]): void {
  if (items.length === 0) return
  runItemCommand('item.addOnNewTracks', { items, tracks })
}

export function updateItem(id: string, updates: Partial<TimelineItem>): void {
  runItemCommand('item.update', { id, ...splitItemUpdates(updates) })
}

export function unlinkItems(ids: string[]): void {
  if (ids.length === 0) return
  const outcome = runItemCommand<string[]>('item.unlink', { itemIds: ids })
  if (outcome.ok && outcome.result.length > 0) {
    useSelectionStore.getState().selectItems(outcome.result)
  }
}

export function linkItems(ids: string[]): boolean {
  if (ids.length === 0) return false
  const outcome = runItemCommand<string[]>('item.link', { itemIds: ids })
  if (!outcome.ok) return false
  useSelectionStore.getState().selectItems(outcome.result)
  return true
}

export function reverseItems(ids: string[]): void {
  const items = useItemsStore.getState().items
  const { reversibleItems, shouldReverse } = resolveReverseTargets(items, ids)
  if (reversibleItems.length === 0) return

  if (shouldReverse) {
    const videoItems = reversibleItems.filter((item) => item.type === 'video')
    if (videoItems.length > 0) {
      useReverseConformDialogStore.getState().open({
        items: reversibleItems,
        videoItems,
        timelineFps: useTimelineSettingsStore.getState().fps,
      })
      return
    }
  }

  runItemCommand('item.reverse', { itemIds: reversibleItems.map((item) => item.id) })
}

export function commitPreparedReverseItems(
  items: TimelineItem[],
  results: ReverseConformResult[],
): void {
  if (items.length === 0) return
  const resultByItemId = new Map(results.map((result) => [result.itemId, result]))
  const entries = items.map((item) => {
    const result = resultByItemId.get(item.id)
    if (!result) return { itemId: item.id }
    const conform: Omit<ReverseConformResult, 'itemId'> & { itemId?: string } = { ...result }
    delete conform.itemId
    return { itemId: item.id, conform }
  })
  runItemCommand('item.commitPreparedReverse', { entries })
}

export function removeItems(ids: string[]): void {
  if (ids.length === 0) return
  const outcome = runItemCommand<string[]>('item.remove', {
    itemIds: ids,
    linkedSelection: isLinkedSelectionEnabled(),
  })
  if (outcome.ok && outcome.result.length > 0) emitUiSound('delete')
}

export function rippleDeleteItems(ids: string[]): void {
  if (ids.length === 0) return
  runItemCommand('item.rippleDelete', {
    itemIds: ids,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}

export function closeGapAtPosition(trackId: string, frame: number): void {
  runItemCommand('item.closeGap', { trackId, frame })
}

export function closeAllGapsOnTrack(trackId: string): void {
  runItemCommand('item.closeAllGaps', { trackId, linkedSelection: isLinkedSelectionEnabled() })
}

/**
 * Track push: move ALL items at or after the anchor clip's position — across
 * every track — by the given frame delta.  This is a multi-track ripple
 * move that closes or opens a gap at the anchor point.
 * Commits as a single undo entry.
 */
export function trackPushItems(anchorId: string, delta: number): void {
  if (delta === 0) return
  runItemCommand('item.trackPush', { anchorId, delta })
}

export function moveItem(id: string, newFrom: number, newTrackId?: string): void {
  runItemCommand('item.move', { id, from: newFrom, trackId: newTrackId })
}

export function moveItems(updates: Array<{ id: string; from: number; trackId?: string }>): void {
  if (updates.length === 0) return
  runItemCommand('item.moveMany', { updates })
}

export function moveItemsWithTrackChanges(
  tracks: TimelineTrack[],
  updates: Array<{ id: string; from: number; trackId?: string }>,
): void {
  if (updates.length === 0) return
  runItemCommand('item.moveMany', { tracks, updates })
}

export function duplicateItems(
  itemIds: string[],
  positions: Array<{ from: number; trackId: string }>,
): TimelineItem[] {
  if (itemIds.length === 0) return []
  const outcome = runItemCommand<TimelineItem[]>('item.duplicate', { itemIds, positions })
  emitUiSound('confirm')
  return outcome.ok ? outcome.result : []
}

export function duplicateItemsWithTrackChanges(
  tracks: TimelineTrack[],
  itemIds: string[],
  positions: Array<{ from: number; trackId: string }>,
): TimelineItem[] {
  if (itemIds.length === 0) return []
  const outcome = runItemCommand<TimelineItem[]>('item.duplicate', { tracks, itemIds, positions })
  return outcome.ok ? outcome.result : []
}

export * from './item-edit-actions'
