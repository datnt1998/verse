/**
 * Pure transition-zone checks used by split-style edit commands. Kept free of UI
 * stores so command handlers can run unchanged in the Node session.
 */

import type { Transition } from '@editor/types/transition'
import { calculateTransitionPortions } from '@editor/shared/timeline/transitions/transition-planner'
import { useTransitionsStore } from '../../transitions-store'

/**
 * Check if a frame falls inside any transition bridge zone for a given item.
 * Uses cut-centered consumed portions so splits are only blocked on the actual
 * frames participating in the transition for that clip.
 */
export function isInTransitionOverlapIn(
  transitions: readonly Transition[],
  itemId: string,
  relativeFrame: number,
  itemDuration: number,
): boolean {
  return transitions.some((transition) => {
    const portions = calculateTransitionPortions(transition.durationInFrames, transition.alignment)
    return (
      (transition.leftClipId === itemId && relativeFrame >= itemDuration - portions.leftPortion) ||
      (transition.rightClipId === itemId && relativeFrame < portions.rightPortion)
    )
  })
}

export function isInTransitionOverlap(
  itemId: string,
  relativeFrame: number,
  itemDuration: number,
): boolean {
  return isInTransitionOverlapIn(
    useTransitionsStore.getState().transitions,
    itemId,
    relativeFrame,
    itemDuration,
  )
}
