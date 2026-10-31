import { useTimelineSettingsStore } from '../timeline-settings-store'
import { executeCommands, getCommandExecutor, type SerializedCommand } from '@editor/commands'
import { emitUiSound } from '@editor/shared/ui/ui-sound'
function dispatch(command: SerializedCommand): void {
  const result = executeCommands([command])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
}
export function toggleSnap(): void {
  dispatch({ type: 'project.setTimelineSettings', params: { snapEnabled: !useTimelineSettingsStore.getState().snapEnabled } })
  if (typeof window !== 'undefined') emitUiSound(useTimelineSettingsStore.getState().snapEnabled ? 'toggleOn' : 'toggleOff')
}
export function toggleAudioSkimming(): void {
  useTimelineSettingsStore.getState().toggleAudioSkimming()
  if (typeof window !== 'undefined') emitUiSound(useTimelineSettingsStore.getState().audioSkimmingEnabled ? 'toggleOn' : 'toggleOff')
}
export function setScrollPosition(position: number): void {
  dispatch({ type: 'project.setView', params: { scrollPosition: position } })
}
export function clearTimeline(): void {
  dispatch({ type: 'project.clearTimeline', params: {} })
  getCommandExecutor().clearHistory()
  useTimelineSettingsStore.getState().markClean()
}
export function markDirty(): void { useTimelineSettingsStore.getState().markDirty() }
export function markClean(): void { useTimelineSettingsStore.getState().markClean() }
