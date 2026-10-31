/**
 * Adapter exports for keyframe logic used by timeline actions.
 * The dopesheet/graph editor UI is product chrome and is not part of the
 * engine; it will consume {@link ./keyframes-contract} when rebuilt.
 */

export {
  getAnimatablePropertiesForItem,
  getEffectPropertyBaseValue,
  getProceduralBands,
  buildVectorPromotionPlan,
  resolveAnimatedTransform,
  getShapeAnimatableBaseValue,
  resolveAnimatedShapeItem,
  resolveExpressionReferenceValue,
  captureAnimationFromItem,
  getPresetCompatibility,
  buildBakeMotionPlan,
  cloneMotionAnimationLayer,
} from './keyframes-contract'
export type { PresetIncompatibilityReason, ProceduralPreviewInput } from './keyframes-contract'
