import type { Transition, TransitionType, TransitionPresentation, WipeDirection, SlideDirection, FlipDirection } from '@editor/types/transition'
import { executeCommands } from '@editor/commands'

type TransitionUpdates = Partial<Pick<Transition, 'durationInFrames' | 'type' | 'presentation' | 'direction' | 'timing' | 'alignment' | 'bezierPoints' | 'presetId' | 'properties'>>
function dispatch<T>(type: string, params: unknown): T {
  const result = executeCommands([{ type, params: JSON.parse(JSON.stringify(params)) }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
  return result.results[0] as T
}
export function addTransition(leftClipId: string, rightClipId: string, type: TransitionType = 'crossfade', durationInFrames?: number, presentation?: TransitionPresentation, direction?: WipeDirection | SlideDirection | FlipDirection, alignment: number = 0.5): boolean {
  return dispatch('transition.add', { leftClipId, rightClipId, type, durationInFrames, presentation, direction, alignment })
}
export function updateTransition(id: string, updates: TransitionUpdates): void { dispatch('transition.update', { id, updates }) }
export function updateTransitions(updates: Array<{ id: string; updates: TransitionUpdates }>): void { dispatch('transition.updateMany', { updates }) }
export function removeTransition(id: string): void { dispatch('transition.remove', { id }) }
