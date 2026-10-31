import { isLinkedSelectionEnabled } from './shared'
import { runAffectedEdit } from './command-bridge'

function rateStretchParams(id: string, newFrom: number, newDuration: number, newSpeed: number) {
  return { id, newFrom, newDuration, newSpeed, linkedSelection: isLinkedSelectionEnabled() }
}

/**
 * Rate-stretch used for live drag updates: every call executes the same
 * `edit.rateStretch` command, but they share a merge key so consecutive updates
 * of one clip collapse into a single undo entry.
 */
export function rateStretchItemWithoutHistory(
  id: string,
  newFrom: number,
  newDuration: number,
  newSpeed: number,
): void {
  runAffectedEdit('edit.rateStretch', {
    ...rateStretchParams(id, newFrom, newDuration, newSpeed),
    mergeKey: `edit.rateStretch:${id}`,
  })
}

export function rateStretchItem(
  id: string,
  newFrom: number,
  newDuration: number,
  newSpeed: number,
): void {
  runAffectedEdit('edit.rateStretch', rateStretchParams(id, newFrom, newDuration, newSpeed))
}

/**
 * Reset speed to 1x for the given items and push subsequent clips right to
 * avoid overlaps. Everything happens in a single undo entry.
 */
export function resetSpeedWithRipple(itemIds: string[]): void {
  runAffectedEdit('edit.resetSpeedWithRipple', {
    itemIds,
    linkedSelection: isLinkedSelectionEnabled(),
  })
}
