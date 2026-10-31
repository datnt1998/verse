import type { ProjectMarker } from '@editor/types/timeline'
import { executeCommands, type SerializedCommand } from '@editor/commands'
function dispatch(command: SerializedCommand): void {
  const result = executeCommands([command])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
}
export function addMarker(frame: number, color?: string, label?: string): void {
  dispatch({ type: 'marker.add', params: { frame, ...(color === undefined ? {} : { color }), ...(label === undefined ? {} : { label }) } })
}
export function updateMarker(id: string, updates: Partial<Omit<ProjectMarker, 'id'>>): void {
  dispatch({ type: 'marker.update', params: { id, updates } })
}
export function removeMarker(id: string): void { dispatch({ type: 'marker.remove', params: { id } }) }
export function clearAllMarkers(): void { dispatch({ type: 'marker.clear', params: {} }) }
export function setInPoint(frame: number): void { dispatch({ type: 'marker.setIn', params: { frame } }) }
export function setOutPoint(frame: number): void { dispatch({ type: 'marker.setOut', params: { frame } }) }
export function clearInOutPoints(): void { dispatch({ type: 'marker.setInOut', params: { inPoint: null, outPoint: null } }) }
/** Continuous range changes now share the command history and gesture merge boundary. */
export function setInOutPoints(inPoint: number | null, outPoint: number | null): void {
  dispatch({ type: 'marker.setInOut', params: { inPoint, outPoint } })
}
