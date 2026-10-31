import type { TimelineItem } from '@editor/types/timeline'
import { toast } from 'sonner'
import { useTimelineSettingsStore } from '../../timeline-settings-store'
import { useSelectionStore } from '@editor/shared/state/selection'
import { emitUiSound } from '@editor/shared/ui/ui-sound'
import { isLinkedSelectionEnabled } from './shared'
import { runEditCommand } from './command-bridge'
import type { SplitAllResult, SplitItemResult } from './split-mutations'

function announceTransitionBlock(): void {
  toast.warning('Cannot split inside a transition zone')
  emitUiSound('error')
}

/**
 * Split an item (and its linked counterparts) at an absolute timeline frame.
 * Out-of-range splits are a silent no-op; frames inside a transition zone are
 * refused with a warning.
 */
export function splitItem(
  id: string,
  splitFrame: number,
): { leftItem: TimelineItem; rightItem: TimelineItem } | null {
  const outcome = runEditCommand<SplitItemResult | null>('edit.splitItem', {
    id,
    frame: splitFrame,
    linkedSelection: isLinkedSelectionEnabled(),
  })

  if (!outcome.ok) {
    if (outcome.issues.some((issue) => issue.code === 'transition-overlap')) {
      announceTransitionBlock()
    }
    return null
  }
  if (!outcome.result) return null

  // Keep selection anchored to the split clip for immediate downstream
  // adjacency/transition detection across all split entry points.
  useSelectionStore.getState().selectItems(outcome.result.leftItemIds)
  useTimelineSettingsStore.getState().markDirty()
  emitUiSound('confirm')
  return { leftItem: outcome.result.leftItem, rightItem: outcome.result.rightItem }
}

/**
 * Split every item crossing a timeline frame in a single undo operation.
 * Used by playhead-based "split across all tracks" shortcuts.
 */
export function splitAllItemsAtFrame(splitFrame: number): number {
  const outcome = runEditCommand<SplitAllResult>('edit.splitAllAtFrame', {
    frame: splitFrame,
    linkedSelection: isLinkedSelectionEnabled(),
  })
  if (!outcome.ok) return 0

  const { splitCount, blockedByTransitionCount, leftItemIds } = outcome.result
  if (blockedByTransitionCount > 0) announceTransitionBlock()
  if (splitCount === 0) return 0

  useSelectionStore.getState().selectItems(leftItemIds)
  useTimelineSettingsStore.getState().markDirty()
  emitUiSound('confirm')
  return splitCount
}

/**
 * Split a clip at multiple frames in one undo operation.
 * Frames must be in absolute timeline space.
 * Splits from last to first so the original item ID stays valid.
 * Clears fadeIn/fadeOut on inner cuts so only the outermost edges keep fades.
 */
export function splitItemAtFrames(id: string, splitFrames: number[]): number {
  if (splitFrames.length === 0) return 0

  const outcome = runEditCommand<{ splitCount: number }>('edit.splitItemAtFrames', {
    id,
    frames: splitFrames,
    linkedSelection: isLinkedSelectionEnabled(),
  })
  if (!outcome.ok) return 0

  useTimelineSettingsStore.getState().markDirty()
  return outcome.result.splitCount
}
