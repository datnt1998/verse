import { useTimelineSettingsStore } from '../../timeline-settings-store'
import { isLinkedSelectionEnabled, finishEdit } from './shared'
import { runEditCommand } from './command-bridge'
import type {
  RemoveRangesMutationResult,
  RemoveSilenceRange,
  RemoveSilenceResult,
} from './range-removal-mutations'

export type { RemoveSilenceRange, RemoveSilenceResult } from './range-removal-mutations'

type RangeRemovalCommand =
  | 'edit.removeSilence'
  | 'edit.removeFillerWords'
  | 'edit.removeTranscriptRanges'

function removeRangesFromItems(
  type: RangeRemovalCommand,
  itemIds: string[],
  rangesByMediaId: Record<string, RemoveSilenceRange[]>,
): RemoveSilenceResult {
  if (itemIds.length === 0) {
    return { analyzedItemCount: 0, removedRangeCount: 0, removedItemCount: 0, splitCount: 0 }
  }

  const outcome = runEditCommand<RemoveRangesMutationResult>(type, {
    itemIds,
    rangesByMediaId,
    linkedSelection: isLinkedSelectionEnabled(),
  })
  if (!outcome.ok) {
    return { analyzedItemCount: 0, removedRangeCount: 0, removedItemCount: 0, splitCount: 0 }
  }

  const { summary, affectedIds } = outcome.result
  if (affectedIds.length > 0) finishEdit(affectedIds)
  else if (summary.splitCount > 0) useTimelineSettingsStore.getState().markDirty()
  return summary
}

export function removeSilenceFromItems(
  itemIds: string[],
  silenceRangesByMediaId: Record<string, RemoveSilenceRange[]>,
): RemoveSilenceResult {
  return removeRangesFromItems('edit.removeSilence', itemIds, silenceRangesByMediaId)
}

export function removeFillerWordsFromItems(
  itemIds: string[],
  fillerRangesByMediaId: Record<string, RemoveSilenceRange[]>,
): RemoveSilenceResult {
  return removeRangesFromItems('edit.removeFillerWords', itemIds, fillerRangesByMediaId)
}

/**
 * Remove the source-time ranges a user selected in the transcript editor.
 * Ranges are in source-native seconds, keyed by mediaId — the same shape the
 * silence/filler removers use — so this reuses the split-then-ripple machinery
 * and lands as a single "remove transcript selection" undo step.
 */
export function removeTranscriptRangesFromItems(
  itemIds: string[],
  rangesByMediaId: Record<string, RemoveSilenceRange[]>,
): RemoveSilenceResult {
  return removeRangesFromItems('edit.removeTranscriptRanges', itemIds, rangesByMediaId)
}
