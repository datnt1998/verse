import type { TextMotionEffect, TextMotionEffectBase, TextMotionSlot } from '@editor/types/text-motion'
import type { TimelineSnapshot } from '@editor/types/timeline-state'
import { captureProjectState, executeCommands, getCommandExecutor, mintId } from '@editor/commands'

let activeGestureId: string | undefined
const gestures = new WeakMap<TimelineSnapshot, string>()
function dispatch<T>(type: string, params: unknown): T {
  const result = executeCommands([{ type, params: JSON.parse(JSON.stringify(params)) }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
  return result.results[0] as T
}
export function applyTextMotionEffect(itemIds: string[], slot: TextMotionSlot, effect: TextMotionEffect): number { return dispatch('keyframe.textMotionSet', { itemIds, slot, effect }) }
export function updateTextMotionLive(itemIds: string[], slot: TextMotionSlot, partial: Partial<TextMotionEffectBase>): void { dispatch('keyframe.textMotionUpdate', { itemIds, slot, partial, gestureId: activeGestureId }) }
export function beginTextMotionEdit(): TimelineSnapshot {
  const before = captureProjectState()
  activeGestureId = mintId()
  gestures.set(before, activeGestureId)
  return before
}
export function commitTextMotionEdit(before: TimelineSnapshot, meta?: { slot?: TextMotionSlot; itemIds?: string[] }): void {
  const gestureId = gestures.get(before)
  const executor = getCommandExecutor()
  if (gestureId === activeGestureId) executor.closeMergeGroup()
  gestures.delete(before)
  if (gestureId === activeGestureId) activeGestureId = undefined
  void meta
}
export function removeTextMotionEffect(itemIds: string[], slot: TextMotionSlot): number { return dispatch('keyframe.textMotionRemove', { itemIds, slot }) }
