import { z } from 'zod'
import type { AnimatableProperty, DirectLinkableProperty, TransformAnimatableProperty } from '@editor/types/keyframe'
import { isDirectLinkableProperty, isTransformAnimatableProperty } from '@editor/types/keyframe'
import { registerCommand } from '../registry'
import type { CommandIssue, ProjectStateView } from '../types'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import * as mutations from './keyframe-mutations'
import { TEXT_MOTION_IN_PRESET_IDS, TEXT_MOTION_OUT_PRESET_IDS, TEXT_MOTION_LOOP_PRESET_IDS } from '@editor/shared/typography/text-motion/text-motion-preset-ids'
import * as motion from './motion-modifier-mutations'
import * as textMotion from './text-motion-mutations'
import * as presets from './preset-mutations'

const id = z.string().min(1)
const frame = z.number().int().nonnegative()
const number = z.number().finite()
export const animationPropertySchema: z.ZodType<AnimatableProperty> = z.union([
  z.enum(['x', 'y', 'width', 'height', 'anchorX', 'anchorY', 'rotation', 'opacity', 'cornerRadius', 'cropLeft', 'cropRight', 'cropTop', 'cropBottom', 'cropSoftness', 'volume', 'textStyleScale', 'fontSize', 'lineHeight', 'textPadding', 'backgroundRadius', 'textShadowOffsetX', 'textShadowOffsetY', 'textShadowBlur', 'strokeWidth', 'trimPathStart', 'trimPathEnd', 'trimPathOffset', 'taperStartWidth', 'taperEndWidth', 'taperStartLength', 'taperEndLength']),
  z.string().regex(/^effect:[^:]+:[^:]+:[^:]+$/),
  z.string().regex(/^pathVertex:\d+:(positionX|positionY|inX|inY|outX|outY)$/),
]) as z.ZodType<AnimatableProperty>
const vectorProperty = z.enum(['position', 'scale', 'anchor'])
const directProperty = z.string().refine(isDirectLinkableProperty) as z.ZodType<DirectLinkableProperty>
const transformProperty = z.string().refine(isTransformAnimatableProperty) as z.ZodType<TransformAnimatableProperty>
const easing = z.enum(['linear', 'ease-in', 'ease-out', 'ease-in-out', 'hold', 'cubic-bezier', 'spring'])
const vector = z.object({ x: number, y: number })
const easingConfig = z.object({ type: easing, bezier: z.object({ x1: number.min(0).max(1), y1: number, x2: number.min(0).max(1), y2: number }).optional(), spring: z.object({ tension: number.nonnegative(), friction: number.nonnegative(), mass: number.positive() }).optional() })
const source = z.object({ applicationId: id, kind: z.enum(['built-in-preset', 'saved-preset']), presetId: id, presetName: z.string() })
export const scalarKeyframeInputSchema = z.object({ frame, value: number, easing: easing.optional(), easingConfig: easingConfig.optional(), source: source.optional() })
export const vectorKeyframeInputSchema = z.object({ frame, value: vector, easing: easing.optional(), easingConfig: easingConfig.optional(), temporalEase: z.object({ in: z.object({ speed: number, influence: number.min(0).max(100) }).optional(), out: z.object({ speed: number, influence: number.min(0).max(100) }).optional() }).optional(), spatial: z.object({ inTangent: vector, outTangent: vector, continuous: z.boolean().optional() }).optional(), source: source.optional() })
const scalarKeyframe = scalarKeyframeInputSchema.extend({ id })
const vectorKeyframe = vectorKeyframeInputSchema.extend({ id, easing })
const ref = z.object({ itemId: id, property: animationPropertySchema, keyframeId: id })
const addPayload = scalarKeyframeInputSchema.extend({ itemId: id, property: animationPropertySchema })
const updatePayload = ref.extend({ updates: scalarKeyframeInputSchema.partial(), clearFields: z.array(z.enum(['easingConfig', 'source'])).optional() })
export const autoKeyframeOperationSchema = z.discriminatedUnion('type', [
  addPayload.extend({ type: z.literal('add') }),
  ref.extend({ type: z.literal('update'), updates: z.object({ value: number.optional() }) }),
  vectorKeyframeInputSchema.extend({ type: z.literal('vector-add'), itemId: id, property: vectorProperty }),
  z.object({ type: z.literal('vector-update'), itemId: id, property: vectorProperty, keyframeId: id, updates: z.object({ value: vector }) }),
])

function requireItem(itemId: string): void {
  if (!useItemsStore.getState().itemById[itemId]) throw new Error(`Không tìm thấy clip: ${itemId}`)
}
function register<P>(type: string, params: z.ZodType<P>, run: (params: P) => unknown, mergeKey?: (params: P) => string | null, validate?: (state: ProjectStateView, params: P) => CommandIssue[]): void {
  registerCommand({ type, params, validate, mergeKey, apply: (_ctx, input) => ({ inverse: { kind: 'patch', patch: {} }, result: run(input) }) })
}

function clearedUpdates<T extends object>(updates: T, fields: readonly string[] = []): T {
  return { ...updates, ...Object.fromEntries(fields.map(field => [field, undefined])) }
}

export function registerKeyframeCommands(): void {
  register('keyframe.add', addPayload, p => { requireItem(p.itemId); return mutations.addKeyframes([p])[0] ?? '' })
  register('keyframe.addMany', z.object({ payloads: z.array(addPayload) }), p => { for (const payload of p.payloads) requireItem(payload.itemId); return mutations.addKeyframes(p.payloads) })
  register('keyframe.update', updatePayload, p => mutations.updateKeyframe(p.itemId, p.property, p.keyframeId, clearedUpdates(p.updates, p.clearFields)), p => `keyframe:${p.itemId}:${p.property}:${p.keyframeId}`)
  register('keyframe.updateMany', z.object({ updates: z.array(updatePayload) }), p => mutations.updateKeyframes(p.updates.map(update => ({ ...update, updates: clearedUpdates(update.updates, update.clearFields) }))), p => `keyframes:${p.updates.map(u => `${u.itemId}:${u.property}:${u.keyframeId}`).sort().join(',')}`)
  register('keyframe.move', z.object({ moves: z.array(z.object({ ref, newFrame: frame })) }), p => mutations.updateKeyframes(p.moves.map(m => ({ ...m.ref, updates: { frame: m.newFrame } }))), p => `keyframes:${p.moves.map(m => `${m.ref.itemId}:${m.ref.property}:${m.ref.keyframeId}`).sort().join(',')}`)
  register('keyframe.remove', ref, p => mutations.removeKeyframe(p.itemId, p.property, p.keyframeId))
  register('keyframe.removeMany', z.object({ refs: z.array(ref) }), p => mutations.removeKeyframes(p.refs))
  register('keyframe.removeForItem', z.object({ itemId: id }), p => mutations.removeKeyframesForItem(p.itemId))
  register('keyframe.removeForProperty', z.object({ itemId: id, property: animationPropertySchema }), p => mutations.removeKeyframesForProperty(p.itemId, p.property))
  register('keyframe.removePresetApplication', z.object({ itemId: id, applicationId: id }), p => mutations.removePresetKeyframeApplication(p.itemId, p.applicationId))
  register('keyframe.removeManual', z.object({ itemId: id }), p => mutations.removeManualKeyframes(p.itemId))
  register('keyframe.trimToBounds', z.object({ itemId: id }), p => mutations.trimAnimationToItemBounds(p.itemId))
  register('keyframe.auto', z.object({ operations: z.array(autoKeyframeOperationSchema) }), p => { for (const op of p.operations) requireItem(op.itemId); return mutations.applyAutoKeyframeOperations(p.operations) })
  register('keyframe.vectorUpsert', z.object({ itemId: id, property: vectorProperty, input: vectorKeyframeInputSchema }), p => { requireItem(p.itemId); return mutations.upsertVectorKeyframe(p.itemId, p.property, p.input) })
  register('keyframe.vectorUpdate', z.object({ itemId: id, property: vectorProperty, keyframeId: id, updates: vectorKeyframeInputSchema.partial(), clearFields: z.array(z.enum(['easingConfig', 'temporalEase', 'spatial', 'source'])).optional() }), p => mutations.updateVectorKeyframe(p.itemId, p.property, p.keyframeId, clearedUpdates(p.updates, p.clearFields)), p => `vector:${p.itemId}:${p.property}:${p.keyframeId}`)
  register('keyframe.vectorRemove', z.object({ itemId: id, property: vectorProperty, keyframeId: id }), p => mutations.removeVectorKeyframe(p.itemId, p.property, p.keyframeId))
  const vectorLane = z.object({ property: vectorProperty, keyframes: z.array(vectorKeyframe) })
  const scalarLane = z.object({ property: animationPropertySchema, keyframes: z.array(scalarKeyframe.extend({ easing })) })
  register('keyframe.vectorPromote', z.object({ itemId: id, vectorProperty: vectorLane, removeScalarProperties: z.array(transformProperty) }), p => { requireItem(p.itemId); return mutations.promoteTransformToVector(p.itemId, p.vectorProperty, p.removeScalarProperties) })
  register('keyframe.vectorSeparate', z.object({ itemId: id, property: vectorProperty, separated: z.boolean(), conversion: z.object({ scalarProperties: z.array(scalarLane).optional(), vectorProperty: vectorLane.optional() }).optional() }), p => { requireItem(p.itemId); return mutations.setVectorDimensionsSeparated(p.itemId, p.property, p.separated, p.conversion) })
  register('keyframe.setLink', z.object({ itemId: id, link: z.object({ type: z.literal('link'), targetProperty: directProperty, sourceItemId: id, sourceProperty: directProperty, enabled: z.boolean(), timeOffsetFrames: z.number().int() }) }), p => { requireItem(p.itemId); requireItem(p.link.sourceItemId); return mutations.setDirectPropertyLink(p.itemId, p.link) })
  register('keyframe.removeLink', z.object({ itemId: id, property: directProperty }), p => mutations.removeDirectPropertyLink(p.itemId, p.property))
  register('keyframe.setExpression', z.object({ itemId: id, expression: z.object({ type: z.literal('expression'), targetProperty: directProperty, source: z.string(), enabled: z.boolean() }) }), p => { requireItem(p.itemId); return mutations.setPropertyExpression(p.itemId, p.expression) })
  register('keyframe.removeExpression', z.object({ itemId: id, property: directProperty }), p => mutations.removePropertyExpression(p.itemId, p.property))
  register('keyframe.applyMotionPreset', z.object({ payloads: z.array(addPayload), clearProperties: z.array(z.object({ itemId: id, property: animationPropertySchema, fromFrame: frame.optional(), toFrame: frame.optional() })).default([]), vectorApplies: z.array(z.object({ itemId: id, property: vectorProperty, keyframes: z.array(vectorKeyframeInputSchema), replaceRange: z.object({ fromFrame: frame, toFrame: frame }).optional() })).default([]) }), p => { for (const itemId of [...p.payloads.map(v => v.itemId), ...p.vectorApplies.map(v => v.itemId)]) requireItem(itemId); return mutations.applyMotionPresetKeyframes(p.payloads, p.clearProperties, p.vectorApplies) })
  register('keyframe.duplicate', z.object({ refs: z.array(ref), frameOffset: z.number().int(), targetItemId: id.optional(), targetProperty: animationPropertySchema.optional() }), p => {
    const store = useKeyframesStore.getState()
    const payloads = p.refs.flatMap(reference => { const key = store.getKeyframeById(reference.itemId, reference.property, reference.keyframeId); return key ? [{ ...key, itemId: p.targetItemId ?? reference.itemId, property: p.targetProperty ?? reference.property, frame: key.frame + p.frameOffset }] : [] })
    if (payloads.some(payload => payload.frame < 0)) throw new Error('Khung hình keyframe không được âm')
    for (const payload of payloads) requireItem(payload.itemId)
    return mutations.addKeyframes(payloads)
  })
  registerProceduralCommands()
}

function registerProceduralCommands(): void {
  const modifierType = z.enum(['float-drift', 'breath-pulse', 'micro-shake', 'sway', 'spin', 'wander'])
  const modifier = z.object({ version: z.literal(2).optional(), id, type: modifierType, enabled: z.boolean(), amplitude: number, frequency: number, phaseFrames: z.number().int(), seed: number, channelGains: z.object({ x: number.optional(), y: number.optional(), width: number.optional(), height: number.optional(), rotation: number.optional(), opacity: number.optional() }).optional() })
  const layer = z.object({ id, name: z.string(), enabled: z.boolean(), source: z.enum(['built-in-preset', 'saved-preset']), sourcePresetId: id, tracks: z.array(z.object({ property: transformProperty, blend: z.enum(['add', 'multiply']), keyframes: z.array(scalarKeyframe.extend({ easing })) })) })
  const assignments = z.array(z.object({ itemId: id, modifier }))
  register('keyframe.motionLayersAdd', z.object({ assignments: z.array(z.object({ itemId: id, layer })) }), p => motion.applyMotionLayersToItems(p.assignments))
  register('keyframe.motionLayerRemove', z.object({ itemIds: z.array(id), layerId: id }), p => motion.removeMotionLayerFromItems(p.itemIds, p.layerId))
  register('keyframe.motionModifiersSet', z.object({ assignments, gestureId: id.optional() }), p => motion.applyMotionModifierToItems(p.assignments), p => p.gestureId ? `motion:${p.gestureId}` : null)
  register('keyframe.motionModifierRemove', z.object({ itemIds: z.array(id), type: modifierType }), p => motion.removeMotionModifierFromItems(p.itemIds, p.type))
  register('keyframe.audioPulseRemove', z.object({ itemIds: z.array(id) }), p => motion.removeAudioPulseFromItems(p.itemIds))
  register('keyframe.motionBake', z.object({ plan: z.array(z.object({ itemId: id, keyframes: z.array(addPayload), clearProperties: z.array(animationPropertySchema), clearMotionModifiers: z.boolean(), clearMotionLayers: z.boolean(), clearAudioPulseEffectIds: z.array(id) })) }), p => motion.bakeMotionToKeyframes(p.plan))
  const slot = z.enum(['in', 'out', 'loop'])
  const textBase = z.object({ durationFrames: number.nonnegative(), offsetFrames: number.nonnegative().optional(), staggerFrames: number.nonnegative(), intensity: number, order: z.enum(['forward', 'backward', 'center', 'random']), easing: z.enum(['linear', 'ease-in', 'ease-out', 'ease-in-out', 'overshoot']), seed: number, unit: z.enum(['character', 'word', 'line', 'whole-clip']).optional() })
  const inEffect = textBase.extend({ presetId: z.enum(TEXT_MOTION_IN_PRESET_IDS) })
  const outEffect = textBase.extend({ presetId: z.enum(TEXT_MOTION_OUT_PRESET_IDS) })
  const loopEffect = textBase.extend({ presetId: z.enum(TEXT_MOTION_LOOP_PRESET_IDS) })
  const textEffect = z.union([inEffect, outEffect, loopEffect])
  register('keyframe.textMotionSet', z.object({ itemIds: z.array(id), slot, effect: textEffect }), p => textMotion.applyTextMotionEffect(p.itemIds, p.slot, p.effect))
  register('keyframe.textMotionUpdate', z.object({ itemIds: z.array(id), slot, partial: textBase.partial(), gestureId: id.optional() }), p => textMotion.updateTextMotionLive(p.itemIds, p.slot, p.partial), p => p.gestureId ? `text-motion:${p.gestureId}` : null)
  register('keyframe.textMotionRemove', z.object({ itemIds: z.array(id), slot }), p => textMotion.removeTextMotionEffect(p.itemIds, p.slot))
  const preset = z.object({
    id, name: z.string(), sourceItemType: z.enum(['video', 'audio', 'image', 'lottie', 'text', 'shape', 'adjustment', 'composition', 'controller', 'subtitle']),
    properties: z.array(z.object({ property: animationPropertySchema, keyframes: z.array(scalarKeyframe.extend({ easing })) })),
    vectorProperties: z.array(z.object({ property: vectorProperty, keyframes: z.array(vectorKeyframe) })).optional(),
    effects: z.array(z.object({ type: z.literal('gpu-effect'), gpuEffectType: id, params: z.record(z.string(), z.union([number, z.boolean(), z.string()])) })),
    motionModifiers: z.array(modifier).optional(), motionLayers: z.array(layer).optional(),
    textMotion: z.object({ in: inEffect.optional(), out: outEffect.optional(), loop: loopEffect.optional() }).optional(),
    sourceDurationInFrames: z.number().int().positive(), createdAt: number,
  })
  register('keyframe.applyPreset', z.object({ targetItemId: id, preset, anchorFrame: frame.default(0), options: z.object({ replace: z.boolean().optional(), retime: z.boolean().optional() }).default({}) }), p => presets.applyAnimationPreset(p.targetItemId, p.preset, p.anchorFrame, p.options))
}
