import type { MotionModifierType } from '@editor/types/motion'
import type { TimelineSnapshot } from '@editor/types/timeline-state'
import { captureProjectState, executeCommands, getCommandExecutor, mintId } from '@editor/commands'
export type { MotionModifierAssignment, MotionLayerAssignment, BakeMotionPlanEntry } from '@editor/commands/handlers/motion-modifier-mutations'
import type { MotionModifierAssignment, MotionLayerAssignment, BakeMotionPlanEntry } from '@editor/commands/handlers/motion-modifier-mutations'

let activeGestureId: string | undefined
const gestures = new WeakMap<TimelineSnapshot, string>()
function dispatch<T>(type: string, params: unknown): T {
  const result = executeCommands([{ type, params: JSON.parse(JSON.stringify(params)) }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
  return result.results[0] as T
}
export function applyMotionLayersToItems(assignments: MotionLayerAssignment[]): number { return dispatch('keyframe.motionLayersAdd', { assignments }) }
export function removeMotionLayerFromItems(itemIds: string[], layerId: string): number { return dispatch('keyframe.motionLayerRemove', { itemIds, layerId }) }
export function applyMotionModifierToItems(assignments: MotionModifierAssignment[]): number { return dispatch('keyframe.motionModifiersSet', { assignments }) }
export function updateMotionModifiersLive(assignments: MotionModifierAssignment[]): void { dispatch('keyframe.motionModifiersSet', { assignments, gestureId: activeGestureId }) }
export function beginMotionModifierEdit(): TimelineSnapshot {
  const before = captureProjectState()
  activeGestureId = mintId()
  gestures.set(before, activeGestureId)
  return before
}
export function commitMotionModifierEdit(before: TimelineSnapshot, meta?: { type?: MotionModifierType; itemIds?: string[] }): void {
  const gestureId = gestures.get(before)
  const executor = getCommandExecutor()
  if (gestureId === activeGestureId) executor.closeMergeGroup()
  gestures.delete(before)
  if (gestureId === activeGestureId) activeGestureId = undefined
  void meta
}
export function bakeMotionToKeyframes(plan: BakeMotionPlanEntry[]): number { return dispatch('keyframe.motionBake', { plan }) }
export function removeMotionModifierFromItems(itemIds: string[], type: MotionModifierType): number { return dispatch('keyframe.motionModifierRemove', { itemIds, type }) }
export function removeAudioPulseFromItems(itemIds: string[]): number { return dispatch('keyframe.audioPulseRemove', { itemIds }) }
