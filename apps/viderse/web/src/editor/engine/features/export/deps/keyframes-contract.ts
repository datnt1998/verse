/**
 * Adapter exports for keyframes dependencies.
 * Export modules should import keyframe utilities from here.
 */

export {
  getPropertyKeyframes,
  interpolatePropertyValue,
} from '@editor/features/keyframes/utils/interpolation'
export { interpolateVectorPropertyValue } from '@editor/features/keyframes/utils/vector-interpolation'
export { resolveAnimatedCrop } from '@editor/features/keyframes/utils/animated-crop-resolver'
export { resolveAnimatedColorEffects } from '@editor/features/keyframes/utils/effect-animatable-properties'
export { resolveAnimatedTextItem } from '@editor/features/keyframes/utils/animated-text-item'
export { resolveAnimatedShapeItem } from '@editor/features/keyframes/utils/animated-shape-item'
