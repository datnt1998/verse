import type { AnimatableProperty, EasingType, Keyframe, KeyframeRef, DirectLinkableProperty, DirectPropertyLink, PropertyExpression, PropertyKeyframes, VectorAnimatableProperty, VectorKeyframe, VectorPropertyKeyframes, TransformAnimatableProperty } from '@editor/types/keyframe'
import type { KeyframeAddPayload, KeyframeUpdatePayload, VectorKeyframeInput } from '../keyframes-store'
import type { AutoKeyframeOperation } from '@editor/features/timeline/deps/keyframes'
import { useKeyframesStore } from '../keyframes-store'
import { executeCommands } from '@editor/commands'
export type { MotionPresetClear, MotionPresetVectorApply } from '@editor/commands/handlers/keyframe-mutations'
import type { MotionPresetClear, MotionPresetVectorApply } from '@editor/commands/handlers/keyframe-mutations'
import { canAddKeyframeAtFrame } from './shared'

function dispatch<T>(type: string, params: unknown): T {
  const result = executeCommands([{ type, params: JSON.parse(JSON.stringify(params)) }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
  return result.results[0] as T
}
function encodedUpdates(updates: object): { updates: object; clearFields: string[] } {
  const entries = Object.entries(updates)
  return { updates: Object.fromEntries(entries.filter(([, value]) => value !== undefined)), clearFields: entries.filter(([, value]) => value === undefined).map(([field]) => field) }
}
export function addKeyframe(itemId: string, property: AnimatableProperty, frame: number, value: number, easing?: EasingType): string {
  if (!canAddKeyframeAtFrame(itemId, frame)) return ''
  return dispatch('keyframe.add', { itemId, property, frame, value, easing })
}
export function setDirectPropertyLink(itemId: string, link: DirectPropertyLink): void { dispatch('keyframe.setLink', { itemId, link }) }
export function removeDirectPropertyLink(itemId: string, property: DirectLinkableProperty): void { dispatch('keyframe.removeLink', { itemId, property }) }
export function setPropertyExpression(itemId: string, expression: PropertyExpression): void { dispatch('keyframe.setExpression', { itemId, expression }) }
export function removePropertyExpression(itemId: string, property: DirectLinkableProperty): void { dispatch('keyframe.removeExpression', { itemId, property }) }
export function upsertVectorKeyframe(itemId: string, property: VectorAnimatableProperty, input: VectorKeyframeInput): string {
  if (!canAddKeyframeAtFrame(itemId, input.frame)) return ''
  return dispatch('keyframe.vectorUpsert', { itemId, property, input })
}
export function updateVectorKeyframe(itemId: string, property: VectorAnimatableProperty, keyframeId: string, updates: Partial<Omit<VectorKeyframe, 'id'>>): void { dispatch('keyframe.vectorUpdate', { itemId, property, keyframeId, ...encodedUpdates(updates) }) }
export function removeVectorKeyframe(itemId: string, property: VectorAnimatableProperty, keyframeId: string): void { dispatch('keyframe.vectorRemove', { itemId, property, keyframeId }) }
export function promoteTransformToVector(itemId: string, vectorProperty: VectorPropertyKeyframes, removeScalarProperties: readonly TransformAnimatableProperty[]): void { dispatch('keyframe.vectorPromote', { itemId, vectorProperty, removeScalarProperties }) }
export function setVectorDimensionsSeparated(itemId: string, property: VectorAnimatableProperty, separated: boolean, conversion: { scalarProperties?: readonly PropertyKeyframes[]; vectorProperty?: VectorPropertyKeyframes } = {}): void { dispatch('keyframe.vectorSeparate', { itemId, property, separated, conversion }) }
export function addKeyframes(payloads: KeyframeAddPayload[]): string[] {
  const valid = payloads.filter(payload => canAddKeyframeAtFrame(payload.itemId, payload.frame))
  return valid.length ? dispatch('keyframe.addMany', { payloads: valid }) : []
}
export function applyMotionPresetKeyframes(payloads: KeyframeAddPayload[], clearProperties: MotionPresetClear[] = [], vectorApplies: MotionPresetVectorApply[] = []): string[] { return dispatch('keyframe.applyMotionPreset', { payloads, clearProperties, vectorApplies }) }
export function updateKeyframe(itemId: string, property: AnimatableProperty, keyframeId: string, updates: Partial<Omit<Keyframe, 'id'>>): void { dispatch('keyframe.update', { itemId, property, keyframeId, ...encodedUpdates(updates) }) }
export function updateKeyframes(updates: KeyframeUpdatePayload[]): void { dispatch('keyframe.updateMany', { updates: updates.map(update => ({ ...update, ...encodedUpdates(update.updates) })) }) }
export function applyAutoKeyframeOperations(operations: AutoKeyframeOperation[]): void { dispatch('keyframe.auto', { operations }) }
export function removeKeyframe(itemId: string, property: AnimatableProperty, keyframeId: string): void { dispatch('keyframe.remove', { itemId, property, keyframeId }) }
export function removeKeyframesForItem(itemId: string): void { dispatch('keyframe.removeForItem', { itemId }) }
export function removeKeyframesForProperty(itemId: string, property: AnimatableProperty): void { dispatch('keyframe.removeForProperty', { itemId, property }) }
export function removePresetKeyframeApplication(itemId: string, applicationId: string): void { dispatch('keyframe.removePresetApplication', { itemId, applicationId }) }
export function removeManualKeyframes(itemId: string): void { dispatch('keyframe.removeManual', { itemId }) }
export function trimAnimationToItemBounds(itemId: string): number { return dispatch('keyframe.trimToBounds', { itemId }) }
export function removeKeyframes(refs: KeyframeRef[]): void { dispatch('keyframe.removeMany', { refs }) }
export function getKeyframesForItem(itemId: string) { return useKeyframesStore.getState().getKeyframesForItem(itemId) }
export function hasKeyframesAtFrame(itemId: string, property: AnimatableProperty, frame: number): boolean { return useKeyframesStore.getState().hasKeyframesAtFrame(itemId, property, frame) }
