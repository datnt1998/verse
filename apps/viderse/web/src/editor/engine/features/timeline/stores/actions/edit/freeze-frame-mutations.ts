/**
 * Freeze-frame commit: split a video at the playhead, insert a still image between
 * the halves and ripple everything after it. The async frame extraction + media
 * persistence stays in `freeze-frame-actions.ts`; this runs inside the
 * `edit.insertFreezeFrame` command handler with the already-persisted image.
 */

import type { ImageItem, TimelineItem } from '@editor/types/timeline'
import type { Transition } from '@editor/types/transition'
import { mintId } from '@editor/commands/id-factory'
import { useItemsStore } from '../../items-store'
import { useTransitionsStore } from '../../transitions-store'
import { applyTransitionRepairs } from '../shared'
import { isInTransitionOverlapIn } from './transition-overlap'

export type FreezeFrameBlockReason = 'not-video' | 'out-of-range' | 'transition-overlap'

export interface FreezeFrameImageParams {
  label: string
  mediaId: string
  src: string
  sourceWidth: number
  sourceHeight: number
}

export interface FreezeFrameMutationParams {
  itemId: string
  playheadFrame: number
  freezeDurationFrames: number
  image: FreezeFrameImageParams
}

export interface FreezeFrameMutationResult {
  freezeFrameItemId: string
}

/** Why a freeze frame cannot be inserted into `itemId` at `playheadFrame` (null = allowed). */
export function getFreezeFrameBlockReason(
  items: readonly TimelineItem[],
  transitions: readonly Transition[],
  itemId: string,
  playheadFrame: number,
): FreezeFrameBlockReason | null {
  const item = items.find((candidate) => candidate.id === itemId)
  if (!item || item.type !== 'video') return 'not-video'
  // Playhead must be strictly inside the clip — we need room to split.
  if (playheadFrame <= item.from || playheadFrame >= item.from + item.durationInFrames) {
    return 'out-of-range'
  }
  if (
    isInTransitionOverlapIn(transitions, itemId, playheadFrame - item.from, item.durationInFrames)
  ) {
    return 'transition-overlap'
  }
  return null
}

/** `null` when the split failed (clip removed or bounds invalid); nothing is mutated then. */
export function applyFreezeFrameMutation(
  params: FreezeFrameMutationParams,
): FreezeFrameMutationResult | null {
  const { itemId, playheadFrame, freezeDurationFrames, image } = params
  const item = useItemsStore.getState().itemById[itemId]
  if (!item) return null

  // Split the video at playhead
  const splitResult = useItemsStore.getState()._splitItem(itemId, playheadFrame)
  if (!splitResult) return null

  const { leftItem, rightItem } = splitResult

  // Transitions that led out of the split clip now lead out of the right half
  const transitions = useTransitionsStore.getState().transitions
  useTransitionsStore
    .getState()
    .setTransitions(
      transitions.map((t) => (t.leftClipId === itemId ? { ...t, leftClipId: rightItem.id } : t)),
    )

  const freezeFrameItem: ImageItem = {
    id: mintId(),
    type: 'image',
    trackId: item.trackId,
    from: playheadFrame,
    durationInFrames: freezeDurationFrames,
    label: image.label,
    mediaId: image.mediaId,
    src: image.src,
    sourceWidth: image.sourceWidth,
    sourceHeight: image.sourceHeight,
    transform: item.transform ? { ...item.transform } : undefined,
  }
  useItemsStore.getState()._addItem(freezeFrameItem)

  // Shift the right half forward by the freeze duration
  useItemsStore.getState()._moveItem(rightItem.id, rightItem.from + freezeDurationFrames)

  // Also shift all items on the same track that come after the right half
  const itemsToShift = useItemsStore
    .getState()
    .items.filter(
      (i) =>
        i.trackId === item.trackId &&
        i.id !== rightItem.id &&
        i.id !== leftItem.id &&
        i.id !== freezeFrameItem.id &&
        i.from > playheadFrame,
    )
  for (const shiftItem of itemsToShift) {
    useItemsStore.getState()._moveItem(shiftItem.id, shiftItem.from + freezeDurationFrames)
  }

  applyTransitionRepairs([leftItem.id, rightItem.id])
  return { freezeFrameItemId: freezeFrameItem.id }
}
