/**
 * Legacy A/V track layout repair. Runs inside the `edit.repairLegacyAvTracks` command
 * handler: the repair is recomputed from canonical state plus the (async-resolved)
 * "does this media have audio" map, so the command stays replayable.
 */

import type { ItemKeyframes } from '@editor/types/keyframe'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import { mintId } from '@editor/commands/id-factory'
import { useItemsStore } from '../items-store'
import { useKeyframesStore } from '../keyframes-store'
import { useTimelineSettingsStore } from '../timeline-settings-store'
import { useCompositionNavigationStore } from '../composition-navigation-store'
import { useCompositionsStore } from '../compositions-store'
import { repairLegacyAvTrackLayout } from '../../utils/legacy-av-track-repair'

export interface LegacyAvRepairMutationParams {
  videoHasAudioByMediaId: Record<string, boolean>
}

/** `true` when the layout changed and the stores were rewritten. */
export function applyLegacyAvRepairMutation(params: LegacyAvRepairMutationParams): boolean {
  const { items, tracks } = useItemsStore.getState()
  const repair = repairLegacyAvTrackLayout({
    tracks,
    items,
    keyframes: useKeyframesStore.getState().keyframes,
    fps: useTimelineSettingsStore.getState().fps,
    videoHasAudioByMediaId: params.videoHasAudioByMediaId,
    createId: mintId,
  })
  if (!repair.changed) return false

  useItemsStore.getState().setTracks(repair.tracks as TimelineTrack[])
  useItemsStore.getState().setItems(repair.items as TimelineItem[])
  useKeyframesStore.getState().setKeyframes(repair.keyframes as ItemKeyframes[])

  const activeCompositionId = useCompositionNavigationStore.getState().activeCompositionId
  if (activeCompositionId) {
    useCompositionsStore.getState().updateComposition(activeCompositionId, {
      tracks: repair.tracks,
      items: repair.items,
      keyframes: repair.keyframes,
    })
  }
  return true
}
