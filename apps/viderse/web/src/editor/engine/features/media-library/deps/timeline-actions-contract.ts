import { registerMediaRelinkingTimelineActions } from '../stores/media-relinking-timeline-actions'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { getSynchronizedLinkedItems } from '@editor/features/timeline/utils/linked-items'
import {
  deleteCompoundClips,
  getCompoundClipDeletionImpact,
  getMediaDeletionImpact,
  openComposition,
  openCompositionAsTab,
  removeProjectItems,
  renameCompoundClip,
  updateProjectItem,
} from '@editor/features/timeline/stores/timeline-actions'
import { executeCommands } from '@editor/commands'

const unregisterMediaRelinkingTimelineActions = registerMediaRelinkingTimelineActions({
  removeProjectItems,
  updateProjectItem,
  getItemsState: () => {
    const { items, itemById } = useItemsStore.getState()
    return { items, itemById }
  },
  getFps: () => useTimelineSettingsStore.getState().fps,
  getSynchronizedLinkedItems,
})

if (globalThis.__vite_hmr__) {
  globalThis.__vite_hmr__.dispose(unregisterMediaRelinkingTimelineActions)
}

function removeTimelineItemsExact(ids: string[]): void {
  const existingIds = ids.filter((id) => useItemsStore.getState().itemById[id] !== undefined)
  if (existingIds.length === 0) return

  const result = executeCommands([{ type: 'item.removeExact', params: { itemIds: existingIds } }])
  if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join('; '))
}

export {
  deleteCompoundClips,
  getCompoundClipDeletionImpact,
  getMediaDeletionImpact,
  openComposition,
  openCompositionAsTab,
  removeTimelineItemsExact,
  removeProjectItems,
  renameCompoundClip,
}
