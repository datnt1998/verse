import { useTimelineSettingsStore } from '../../timeline-settings-store'
import { isLinkedSelectionEnabled } from './shared'
import { runEditCommand, type EditAffected } from './command-bridge'

export function joinItems(itemIds: string[]): void {
  const outcome = runEditCommand<EditAffected>('edit.joinItems', {
    itemIds,
    linkedSelection: isLinkedSelectionEnabled(),
  })
  if (outcome.ok && outcome.result) useTimelineSettingsStore.getState().markDirty()
}
