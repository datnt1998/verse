/**
 * Split-family edit mutations (single, all-at-frame, multi-frame). Run inside command
 * handlers; no selection / toast / sound side effects here — those belong to the
 * wrappers in `split-actions.ts`.
 */

import type { TimelineItem } from '@editor/types/timeline'
import type { Transition } from '@editor/types/transition'
import { useItemsStore } from '../../items-store'
import { applyTransitionRepairs } from '../shared'
import { getLinkedItemsForEdit } from '../linked-edit'
import { getUniqueLinkedItemAnchorIds } from '../../../utils/linked-items'
import { applySplitBookkeeping, type SplitResultEntry } from '../split-bookkeeping'
import { isInTransitionOverlap, isInTransitionOverlapIn } from './transition-overlap'

export type SplitBlockReason = 'missing-item' | 'out-of-range' | 'transition-overlap'

/**
 * Preflight for a single split: why (if at all) splitting `id` (plus its linked
 * counterparts) at `frame` is not allowed. Out-of-range is checked before the
 * transition zone so edge splits never false-positive as a transition block.
 */
export function getSplitBlockReason(
  items: readonly TimelineItem[],
  transitions: readonly Transition[],
  id: string,
  frame: number,
  linkedSelection: boolean,
): SplitBlockReason | null {
  const itemsToSplit = getLinkedItemsForEdit([...items], id, linkedSelection)
  if (itemsToSplit.length === 0) return 'missing-item'
  for (const item of itemsToSplit) {
    if (frame <= item.from || frame >= item.from + item.durationInFrames) return 'out-of-range'
    if (isInTransitionOverlapIn(transitions, item.id, frame - item.from, item.durationInFrames)) {
      return 'transition-overlap'
    }
  }
  return null
}

function splitItems(itemsToSplit: TimelineItem[], frame: number): SplitResultEntry[] {
  return itemsToSplit
    .map((item) => ({
      originalId: item.id,
      originalLinkedGroupId: item.linkedGroupId,
      result: useItemsStore.getState()._splitItem(item.id, frame),
    }))
    .filter((entry): entry is SplitResultEntry => entry.result !== null)
}

export interface SplitMutationParams {
  id: string
  frame: number
  linkedSelection: boolean
}

export interface SplitItemResult {
  leftItem: TimelineItem
  rightItem: TimelineItem
  /** Left halves of every split item; the wrapper selects them. */
  leftItemIds: string[]
}

export function applySplitMutation(params: SplitMutationParams): SplitItemResult | null {
  const { id, frame, linkedSelection } = params
  const itemsToSplit = getLinkedItemsForEdit(useItemsStore.getState().items, id, linkedSelection)
  const splitResults = splitItems(itemsToSplit, frame)

  const anchorResult = splitResults.find((entry) => entry.originalId === id)?.result ?? null
  if (!anchorResult) return null

  applySplitBookkeeping(splitResults)
  return { ...anchorResult, leftItemIds: splitResults.map((entry) => entry.result.leftItem.id) }
}

export interface SplitAllMutationParams {
  frame: number
  linkedSelection: boolean
}

export interface SplitAllResult {
  splitCount: number
  /** Groups skipped because the frame sits inside a transition zone. */
  blockedByTransitionCount: number
  /** Left halves of the last split group; the wrapper selects them. */
  leftItemIds: string[]
}

/** Split every item crossing `frame`; linked groups split together and count once. */
export function applySplitAllMutation(params: SplitAllMutationParams): SplitAllResult {
  const { frame, linkedSelection } = params
  const items = useItemsStore.getState().items
  const overlappingItemIds = items
    .filter((item) => frame > item.from && frame < item.from + item.durationInFrames)
    .map((item) => item.id)
  const anchorIds = getUniqueLinkedItemAnchorIds(items, overlappingItemIds)

  const result: SplitAllResult = { splitCount: 0, blockedByTransitionCount: 0, leftItemIds: [] }
  for (const anchorId of anchorIds) {
    const itemsToSplit = getLinkedItemsForEdit(
      useItemsStore.getState().items,
      anchorId,
      linkedSelection,
    )
    if (itemsToSplit.length === 0) continue

    let blockedByTransition = false
    const canSplitGroup = itemsToSplit.every((item) => {
      if (frame <= item.from || frame >= item.from + item.durationInFrames) return false
      if (isInTransitionOverlap(item.id, frame - item.from, item.durationInFrames)) {
        blockedByTransition = true
        return false
      }
      return true
    })

    if (!canSplitGroup) {
      if (blockedByTransition) result.blockedByTransitionCount += 1
      continue
    }

    const splitResults = splitItems(itemsToSplit, frame)
    const anchorResult = splitResults.find((entry) => entry.originalId === anchorId)?.result ?? null
    if (!anchorResult) continue

    applySplitBookkeeping(splitResults)
    result.leftItemIds = splitResults.map((entry) => entry.result.leftItem.id)
    result.splitCount += 1
  }
  return result
}

export interface SplitMultiMutationParams {
  id: string
  frames: number[]
  linkedSelection: boolean
}

/**
 * Split a clip at multiple absolute timeline frames. Splits from last to first so
 * the original item ID stays valid, then clears fadeIn/fadeOut on inner cuts so
 * only the outermost edges keep fades.
 */
export function applySplitMultiMutation(params: SplitMultiMutationParams): { splitCount: number } {
  const { id, linkedSelection } = params
  const sorted = [...params.frames].sort((a, b) => b - a)
  let splitCount = 0

  const itemsToSplit = getLinkedItemsForEdit(useItemsStore.getState().items, id, linkedSelection)
  if (itemsToSplit.length === 0) return { splitCount }

  const rightIdsByOriginalId = new Map(itemsToSplit.map((item) => [item.id, [] as string[]]))

  for (const frame of sorted) {
    const currentItemsById = useItemsStore.getState().itemById
    const canSplitFrame = itemsToSplit.every((item) => {
      const currentItem = currentItemsById[item.id]
      if (!currentItem) return false
      if (frame <= currentItem.from || frame >= currentItem.from + currentItem.durationInFrames) {
        return false
      }
      return !isInTransitionOverlap(
        currentItem.id,
        frame - currentItem.from,
        currentItem.durationInFrames,
      )
    })
    if (!canSplitFrame) continue

    const frameSplitResults = splitItems(itemsToSplit, frame)
    if (frameSplitResults.length !== itemsToSplit.length) continue

    applySplitBookkeeping(frameSplitResults)
    splitCount++

    for (const entry of frameSplitResults) {
      rightIdsByOriginalId.get(entry.originalId)?.push(entry.result.rightItem.id)
      applyTransitionRepairs([entry.result.leftItem.id, entry.result.rightItem.id])
    }
  }

  // Clear fades on inner split edges:
  // - Every right piece gets fadeIn cleared (it's an inner cut, not the clip's original start)
  // - Every right piece except the last (outermost) gets fadeOut cleared
  // - The left piece (original ID) gets fadeOut cleared (its right edge is an inner cut)
  if (splitCount > 0) {
    const store = useItemsStore.getState()
    for (const [originalId, rightIds] of rightIdsByOriginalId) {
      for (const rightId of rightIds) store._updateItem(rightId, { fadeIn: 0 })
      for (let index = 1; index < rightIds.length; index += 1) {
        store._updateItem(rightIds[index]!, { fadeOut: 0 })
      }
      store._updateItem(originalId, { fadeOut: 0 })
    }
  }

  return { splitCount }
}
