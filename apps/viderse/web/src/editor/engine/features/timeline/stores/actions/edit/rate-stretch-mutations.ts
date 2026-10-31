/**
 * Rate-stretch mutations (clip speed / duration change with ripple) and the
 * "reset speed to 1x" ripple. Run inside command handlers; the wrappers in
 * `rate-stretch-actions.ts` own warm-up / dirty flagging.
 */

import type { TimelineItem } from '@editor/types/timeline'
import { useItemsStore } from '../../items-store'
import { useTransitionsStore } from '../../transitions-store'
import { useKeyframesStore } from '../../keyframes-store'
import { useTimelineSettingsStore } from '../../timeline-settings-store'
import { applyTransitionRepairs } from '../shared'
import { getSynchronizedLinkedItemsForEdit } from '../linked-edit'
import { timelineToSourceFrames, sourceToTimelineFrames } from '../../../utils/source-calculations'
import {
  expandItemIdsWithAttachedCaptions,
  getAttachedCaptionItemIds,
  getLinkedItemIds,
} from '../../../utils/linked-items'
import { scaleCaptionWords, trimCaptionWords } from '@editor/features/captions/word-timing'

type MoveUpdate = { id: string; from: number }

/**
 * Queue a move for `itemId` plus its linked companions / attached captions,
 * skipping anything already queued.
 */
function queueShiftWithCompanions(
  freshItems: TimelineItem[],
  itemId: string,
  shiftFrom: (from: number) => number,
  movedIds: Set<string>,
  moveUpdates: MoveUpdate[],
): void {
  const item = freshItems.find((candidate) => candidate.id === itemId)
  if (!item || movedIds.has(itemId)) return
  movedIds.add(itemId)
  moveUpdates.push({ id: itemId, from: shiftFrom(item.from) })

  const linkedIds = expandItemIdsWithAttachedCaptions(freshItems, getLinkedItemIds(freshItems, itemId))
  for (const linkedId of linkedIds) {
    if (linkedId === itemId || movedIds.has(linkedId)) continue
    const linked = freshItems.find((candidate) => candidate.id === linkedId)
    if (linked) {
      movedIds.add(linkedId)
      moveUpdates.push({ id: linkedId, from: shiftFrom(linked.from) })
    }
  }
}

/**
 * Retime the captions attached to a clip after its bounds changed under a speed
 * edit: each caption's span is scaled by the clip's duration ratio and clipped
 * to the new bounds, while word timings (caption-local frames) are scaled by
 * the same factor so karaoke highlights stay aligned. Captions left entirely
 * outside the stretched clip are removed.
 */
export function retimeAttachedCaptionsToClip(
  clipId: string,
  oldFrom: number,
  oldDuration: number,
  newFrom: number,
  newDuration: number,
): string[] {
  if (oldDuration <= 0 || newDuration <= 0) return []
  const store = useItemsStore.getState()
  const scale = newDuration / oldDuration
  const clipEnd = newFrom + newDuration
  const touchedIds: string[] = []

  for (const captionId of getAttachedCaptionItemIds(store.items, clipId)) {
    const caption = store.itemById[captionId]
    if (!caption || caption.type !== 'text') continue

    const scaledFrom = newFrom + Math.round((caption.from - oldFrom) * scale)
    const scaledEnd = newFrom + Math.round((caption.from + caption.durationInFrames - oldFrom) * scale)
    const nextStart = Math.max(scaledFrom, newFrom)
    const nextEnd = Math.min(scaledEnd, clipEnd)

    if (nextEnd <= nextStart) {
      store._removeItems([caption.id])
      useKeyframesStore.getState()._removeKeyframesForItems([caption.id])
      touchedIds.push(caption.id)
      continue
    }

    const nextDuration = nextEnd - nextStart
    const boundsChanged =
      nextStart !== caption.from || nextDuration !== caption.durationInFrames
    const words =
      caption.words && (boundsChanged || scale !== 1)
        ? trimCaptionWords(
            // scaleCaptionWords multiplies timings by arg1/arg2 — the caption
            // duration ratio is the same scale applied to its span above.
            scaleCaptionWords(caption.words, scaledEnd - scaledFrom, caption.durationInFrames),
            nextStart - scaledFrom,
            nextEnd - scaledFrom,
          )
        : undefined

    if (!boundsChanged && words === undefined) continue
    store._updateItem(caption.id, {
      from: nextStart,
      durationInFrames: nextDuration,
      ...(words ? { words } : {}),
    })
    touchedIds.push(caption.id)
  }
  return touchedIds
}

export interface RateStretchMutationParams {
  id: string
  newFrom: number
  newDuration: number
  newSpeed: number
  linkedSelection: boolean
}

/** Ids of every item touched; `null` when the anchor clip does not exist. */
export function applyRateStretchMutation(params: RateStretchMutationParams): string[] | null {
  const { id, newFrom, newDuration, newSpeed, linkedSelection } = params
  const itemsStore = useItemsStore.getState()
  const itemsBefore = itemsStore.items
  const synchronizedItems = getSynchronizedLinkedItemsForEdit(itemsBefore, id, linkedSelection)
  const anchorBefore = synchronizedItems.find((item) => item.id === id)
  if (!anchorBefore) return null

  // Capture old boundaries BEFORE stretch (needed for ripple + keyframe scaling)
  const oldDuration = anchorBefore.durationInFrames
  const oldFrom = anchorBefore.from
  const oldEnd = oldFrom + oldDuration

  itemsStore._rateStretchItem(id, newFrom, newDuration, newSpeed)

  const anchorAfter = useItemsStore.getState().itemById[id]
  if (!anchorAfter) return null

  const actualFrom = anchorAfter.from
  const actualDuration = anchorAfter.durationInFrames
  const actualSpeed = anchorAfter.speed ?? newSpeed
  const fromDelta = actualFrom - anchorBefore.from

  for (const synchronizedItem of synchronizedItems) {
    if (synchronizedItem.id === id) continue
    itemsStore._rateStretchItem(
      synchronizedItem.id,
      synchronizedItem.from + fromDelta,
      actualDuration,
      actualSpeed,
    )
  }

  // Scale keyframes proportionally to match new duration so animations keep
  // their relative timing within the clip.
  if (oldDuration !== actualDuration) {
    for (const synchronizedItem of synchronizedItems) {
      useKeyframesStore
        .getState()
        ._scaleKeyframesForItem(synchronizedItem.id, synchronizedItem.durationInFrames, actualDuration)
    }
  }

  // Retime captions attached to every stretched clip into its new bounds
  // (word timings scale by the same duration ratio). Captions that fall fully
  // outside the stretched clip are dropped.
  const retimedCaptionIds: string[] = []
  if (oldDuration !== actualDuration || fromDelta !== 0) {
    for (const synchronizedItem of synchronizedItems) {
      const stretched = useItemsStore.getState().itemById[synchronizedItem.id]
      if (!stretched) continue
      retimedCaptionIds.push(
        ...retimeAttachedCaptionsToClip(
          synchronizedItem.id,
          synchronizedItem.from,
          synchronizedItem.durationInFrames,
          stretched.from,
          stretched.durationInFrames,
        ),
      )
    }
  }

  // Ripple phase: push/pull adjacent clips to maintain adjacency and prevent overlaps.
  // End handle: endDelta !== 0 → shift downstream clips.
  // Start handle: fromDelta !== 0, end stays fixed → shift upstream clips.
  const endDelta = actualFrom + actualDuration - oldEnd
  const allSynchronizedIds = new Set(synchronizedItems.map((si) => si.id))
  const freshItems = useItemsStore.getState().items
  const transitions = useTransitionsStore.getState().transitions
  const movedIds = new Set<string>()
  const moveUpdates: MoveUpdate[] = []

  // Collect all track IDs touched by the stretched item + its linked companions
  const touchedTrackIds = new Set<string>()
  for (const si of synchronizedItems) {
    const freshSi = freshItems.find((i) => i.id === si.id)
    if (freshSi) touchedTrackIds.add(freshSi.trackId)
  }

  if (endDelta !== 0) {
    const shiftDownstream = (from: number) => from + endDelta
    // End handle changed — shift downstream clips (at or past old end) on touched tracks
    for (const trackId of touchedTrackIds) {
      const downstreamItems = freshItems
        .filter((i) => i.trackId === trackId && !allSynchronizedIds.has(i.id) && i.from >= oldEnd)
        .sort((a, b) => a.from - b.from)
      for (const downstream of downstreamItems) {
        queueShiftWithCompanions(freshItems, downstream.id, shiftDownstream, movedIds, moveUpdates)
      }
    }

    // Also shift transition-connected neighbors that aren't downstream by position
    // but are directly bridged to the stretched clip's end
    for (const si of synchronizedItems) {
      for (const t of transitions) {
        if (t.leftClipId === si.id && !allSynchronizedIds.has(t.rightClipId)) {
          queueShiftWithCompanions(freshItems, t.rightClipId, shiftDownstream, movedIds, moveUpdates)
        }
      }
    }
  }

  if (fromDelta !== 0) {
    const shiftUpstream = (from: number) => Math.max(0, from + fromDelta)
    // Start handle changed — shift upstream clips (ending at or before old from) on touched tracks
    for (const trackId of touchedTrackIds) {
      const upstreamItems = freshItems
        .filter((i) => {
          if (i.trackId !== trackId || allSynchronizedIds.has(i.id)) return false
          return i.from + i.durationInFrames <= oldFrom
        })
        .sort((a, b) => a.from - b.from)
      for (const upstream of upstreamItems) {
        queueShiftWithCompanions(freshItems, upstream.id, shiftUpstream, movedIds, moveUpdates)
      }
    }

    // Also shift transition-connected neighbors bridged to the stretched clip's start
    for (const si of synchronizedItems) {
      for (const t of transitions) {
        if (t.rightClipId === si.id && !allSynchronizedIds.has(t.leftClipId)) {
          queueShiftWithCompanions(freshItems, t.leftClipId, shiftUpstream, movedIds, moveUpdates)
        }
      }
    }
  }

  if (moveUpdates.length > 0) {
    useItemsStore.getState()._moveItems(moveUpdates)
  }

  const allAffectedIds = [...allSynchronizedIds, ...movedIds, ...retimedCaptionIds]
  applyTransitionRepairs(allAffectedIds)
  return allAffectedIds
}

export interface ResetSpeedMutationParams {
  itemIds: string[]
  linkedSelection: boolean
}

const SPEED_RESET_TOLERANCE = 0.01

/**
 * Reset speed to 1x for the given items and push subsequent clips right to avoid
 * overlaps (a variable-speed clip such as 1.23x grows when reset). Downstream clips
 * and their linked companions shift right by the growth amount.
 */
export function applyResetSpeedMutation(params: ResetSpeedMutationParams): string[] | null {
  const { itemIds, linkedSelection } = params
  const itemsStore = useItemsStore.getState()
  const fps = useTimelineSettingsStore.getState().fps

  // Collect all items that need resetting (deduplicate via synchronized links)
  const processedIds = new Set<string>()
  const stretchOps: Array<{
    id: string
    oldEnd: number
    newDuration: number
    synchronizedIds: string[]
  }> = []

  for (const id of itemIds) {
    if (processedIds.has(id)) continue
    const item = itemsStore.items.find((i) => i.id === id)
    if (!item || (item.type !== 'video' && item.type !== 'audio')) continue

    const currentSpeed = item.speed || 1
    if (Math.abs(currentSpeed - 1) <= SPEED_RESET_TOLERANCE) continue

    const synchronizedItems = getSynchronizedLinkedItemsForEdit(
      itemsStore.items,
      id,
      linkedSelection,
    )
    for (const si of synchronizedItems) processedIds.add(si.id)

    const sourceFps = item.sourceFps ?? fps
    const effectiveSourceFrames =
      item.sourceEnd !== undefined && item.sourceStart !== undefined
        ? item.sourceEnd - item.sourceStart
        : timelineToSourceFrames(item.durationInFrames, currentSpeed, fps, sourceFps)

    stretchOps.push({
      id,
      oldEnd: item.from + item.durationInFrames,
      newDuration: Math.max(1, sourceToTimelineFrames(effectiveSourceFrames, 1, sourceFps, fps)),
      synchronizedIds: synchronizedItems.map((si) => si.id),
    })
  }

  if (stretchOps.length === 0) return null

  // Phase 1: Apply all rate stretches
  const retimedCaptionIds: string[] = []
  for (const op of stretchOps) {
    const anchor = itemsStore.items.find((i) => i.id === op.id)
    if (!anchor) continue

    const oldDuration = anchor.durationInFrames
    const boundsBefore = new Map(
      useItemsStore
        .getState()
        .items.filter((i) => op.synchronizedIds.includes(i.id))
        .map((i) => [i.id, { from: i.from, durationInFrames: i.durationInFrames }]),
    )
    itemsStore._rateStretchItem(op.id, anchor.from, op.newDuration, 1)

    // Synchronize linked items
    const anchorAfter = useItemsStore.getState().itemById[op.id]
    if (!anchorAfter) continue

    const actualDuration = anchorAfter.durationInFrames
    const fromDelta = anchorAfter.from - anchor.from

    for (const siId of op.synchronizedIds) {
      if (siId === op.id) continue
      const si = useItemsStore.getState().items.find((i) => i.id === siId)
      if (!si) continue
      itemsStore._rateStretchItem(siId, si.from + fromDelta, actualDuration, anchorAfter.speed ?? 1)
    }

    if (oldDuration !== actualDuration) {
      for (const siId of op.synchronizedIds) {
        useKeyframesStore.getState()._scaleKeyframesForItem(siId, oldDuration, actualDuration)
      }
    }

    // Retime attached captions against the stretched bounds (words scale too).
    for (const siId of op.synchronizedIds) {
      const before = boundsBefore.get(siId)
      const stretched = useItemsStore.getState().itemById[siId]
      if (!before || !stretched) continue
      if (before.durationInFrames === stretched.durationInFrames && before.from === stretched.from) {
        continue
      }
      retimedCaptionIds.push(
        ...retimeAttachedCaptionsToClip(
          siId,
          before.from,
          before.durationInFrames,
          stretched.from,
          stretched.durationInFrames,
        ),
      )
    }
  }

  // Phase 2: Push subsequent clips right to resolve overlaps
  const freshItems = useItemsStore.getState().items
  const allChangedIds = new Set(stretchOps.flatMap((op) => op.synchronizedIds))
  const moveUpdates: MoveUpdate[] = []
  const movedIds = new Set<string>()

  for (const op of stretchOps) {
    const stretchedItem = freshItems.find((i) => i.id === op.id)
    if (!stretchedItem) continue

    const growth = stretchedItem.from + stretchedItem.durationInFrames - op.oldEnd
    if (growth <= 0) continue

    const touchedTrackIds = new Set<string>()
    for (const siId of op.synchronizedIds) {
      const si = freshItems.find((i) => i.id === siId)
      if (si) touchedTrackIds.add(si.trackId)
    }

    // On each touched track, push subsequent clips right (with their linked companions)
    for (const trackId of touchedTrackIds) {
      const trackItems = freshItems
        .filter((i) => i.trackId === trackId && !allChangedIds.has(i.id) && i.from >= op.oldEnd)
        .sort((a, b) => a.from - b.from)
      for (const downstream of trackItems) {
        queueShiftWithCompanions(
          freshItems,
          downstream.id,
          (from) => from + growth,
          movedIds,
          moveUpdates,
        )
      }
    }
  }

  if (moveUpdates.length > 0) {
    useItemsStore.getState()._moveItems(moveUpdates)
  }

  const allAffectedIds = [...allChangedIds, ...movedIds, ...retimedCaptionIds]
  applyTransitionRepairs(allAffectedIds)
  return allAffectedIds
}
