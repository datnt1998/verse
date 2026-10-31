/**
 * Adapter exports for keyframes dependencies.
 * Timeline modules should import keyframe components/utilities from here.
 */

export type { AutoKeyframeOperation } from '@editor/features/keyframes/utils/auto-keyframe'
export { interpolatePropertyValue } from '@editor/features/keyframes/utils/interpolation'
export { sampleVectorSpeedGraph } from '@editor/features/keyframes/utils/vector-interpolation'
export {
  cleanupTrimmedKeyframes,
  countTrimmedKeyframes,
} from '@editor/features/keyframes/utils/trimmed-keyframes'
export {
  buildVectorPromotionPlan,
  remapLegacyVectorPromotionIdentities,
} from '@editor/features/keyframes/utils/vector-promotion'
export {
  resolveAnimatedTransform,
  resolveExpressionReferenceValue,
  wouldCreateDirectPropertyLinkCycle,
} from '@editor/features/keyframes/utils/animated-transform-resolver'
export {
  getShapeAnimatableBaseValue,
  resolveAnimatedShapeItem,
} from '@editor/features/keyframes/utils/animated-shape-item'
export {
  BEZIER_PRESETS,
  areBezierPointsEqual,
  findMatchingBezierPreset,
  clampBezierValue,
  clampSpringValue,
  buildEasingConfig,
} from '@editor/features/keyframes/utils/easing-presets'
export type { BezierPresetValue } from '@editor/features/keyframes/utils/easing-presets'
export {
  getTransitionBlockedRanges,
  isFrameInTransitionRegion,
} from '@editor/features/keyframes/utils/transition-region'
// Keyframe-editor UI (dopesheet/graph editor) is product chrome built later
// against this contract; only logic helpers stay on the engine side.
export { getAnimatablePropertiesForItem } from '@editor/features/keyframes/utils/animatable-properties'
export { getAnimatablePropertyBaseValue } from '@editor/features/keyframes/utils/animatable-property-base-value'
export {
  getProceduralBands,
  type ProceduralPreviewInput,
} from '@editor/features/keyframes/utils/procedural-preview'
export { buildBakeMotionPlan } from '@editor/features/keyframes/utils/bake-motion'
export { cloneMotionAnimationLayer } from '@editor/features/keyframes/utils/motion-layer-eval'
export { getEffectPropertyBaseValue } from '@editor/features/keyframes/utils/effect-animatable-properties'
export {
  captureAnimationFromItem,
  getPresetCompatibility,
} from '@editor/features/keyframes/utils/animation-preset-compat'
export type { PresetIncompatibilityReason } from '@editor/features/keyframes/utils/animation-preset-compat'
