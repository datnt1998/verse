/**
 * Trim-family edit actions. Each export builds serializable params, runs one typed
 * command (`edit.*`, see commands/handlers/edit.ts) and finishes with the shared
 * post-edit side effects. The mutation bodies live in ./trim-mutations.ts.
 */

import { isLinkedSelectionEnabled } from './shared'
import { runAffectedEdit } from './command-bridge'

interface SynchronizedTrimOptions {
  forceLinked?: boolean
  itemIds?: readonly string[]
}

function trimParams(
  id: string,
  handle: 'start' | 'end',
  amount: number,
  options: SynchronizedTrimOptions,
) {
  return {
    id,
    handle,
    amount,
    ...(options.itemIds ? { itemIds: [...options.itemIds] } : {}),
    linkedSelection: options.forceLinked === true || isLinkedSelectionEnabled(),
  }
}

export function trimItemStart(
  id: string,
  trimAmount: number,
  options: SynchronizedTrimOptions = {},
): void {
  runAffectedEdit('edit.trimItemStart', trimParams(id, 'start', trimAmount, options))
}

export function trimItemEnd(
  id: string,
  trimAmount: number,
  options: SynchronizedTrimOptions = {},
): void {
  runAffectedEdit('edit.trimItemEnd', trimParams(id, 'end', trimAmount, options))
}

export function trimItemBreakingTransition(
  id: string,
  handle: 'start' | 'end',
  trimAmount: number,
  transitionIdsToRemove: string[],
  options: Pick<SynchronizedTrimOptions, 'itemIds'> = {},
): void {
  runAffectedEdit('edit.trimItemBreakingTransition', {
    ...trimParams(id, handle, trimAmount, options),
    removeTransitionIds: transitionIdsToRemove,
  })
}

/**
 * Ripple edit: trim a clip and shift all downstream items on the same track.
 *
 * @param id - ID of the clip being trimmed
 * @param handle - Which handle is being dragged ('start' or 'end')
 * @param trimDelta - Frames to trim (positive = shrink start / extend end,
 *                    negative = extend start / shrink end)
 */
export function rippleTrimItem(id: string, handle: 'start' | 'end', trimDelta: number): void {
  if (trimDelta === 0) return
  runAffectedEdit('edit.rippleTrim', {
    id,
    handle,
    delta: trimDelta,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}

/**
 * Rolling edit: trims the left clip's end and the right clip's start by the same
 * amount, keeping total timeline duration unchanged.
 *
 * @param editPointDelta - Frames to move the edit point (positive = right, negative = left)
 */
export function rollingTrimItems(leftId: string, rightId: string, editPointDelta: number): void {
  if (editPointDelta === 0) return
  runAffectedEdit('edit.rollingTrim', {
    leftId,
    rightId,
    delta: editPointDelta,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}

/**
 * Slip edit: shift the source window within a clip without changing its position
 * or duration on the timeline.
 *
 * @param slipDelta - Frames to shift the source window (positive = later in source)
 */
export function slipItem(id: string, slipDelta: number): void {
  if (slipDelta === 0) return
  runAffectedEdit('edit.slip', {
    id,
    delta: slipDelta,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}

/**
 * Slide edit: move a clip on the timeline while adjusting its neighboring clips.
 *
 * @param slideDelta - Frames to slide (positive = right, negative = left)
 * @param leftNeighborId - ID of the left adjacent clip (null if none)
 * @param rightNeighborId - ID of the right adjacent clip (null if none)
 */
export function slideItem(
  id: string,
  slideDelta: number,
  leftNeighborId: string | null,
  rightNeighborId: string | null,
): void {
  if (slideDelta === 0) return
  runAffectedEdit('edit.slide', {
    id,
    delta: slideDelta,
    leftNeighborId,
    rightNeighborId,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}
