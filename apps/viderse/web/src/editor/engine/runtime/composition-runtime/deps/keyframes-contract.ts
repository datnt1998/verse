/**
 * Adapter exports for keyframe-layer dependencies.
 */

export {
  resolveAnimatedTransform,
  hasKeyframeAnimation,
} from '@editor/features/keyframes/utils/animated-transform-resolver'
export type { LinkedPropertyEvaluationContext } from '@editor/features/keyframes/utils/animated-transform-resolver'
export { applyMotionModifiers } from '@editor/features/keyframes/utils/motion-modifier-eval'
export { applyMotionAnimationLayers } from '@editor/features/keyframes/utils/motion-layer-eval'
export { resolveAnimatedCrop } from '@editor/features/keyframes/utils/animated-crop-resolver'
export {
  getPropertyKeyframes,
  interpolatePropertyValue,
} from '@editor/features/keyframes/utils/interpolation'
export { resolveAnimatedTextItem } from '@editor/features/keyframes/utils/animated-text-item'
export { resolveAnimatedShapeItem } from '@editor/features/keyframes/utils/animated-shape-item'
