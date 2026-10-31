/**
 * Adapter exports for composition-runtime dependencies.
 * Keyframes modules should import transform helpers from here.
 */

export {
  resolveTransform,
  getSourceDimensions,
} from '@editor/runtime/composition-runtime/utils/transform-resolver'
export { expandTextTransformToFitContent } from '@editor/runtime/composition-runtime/utils/text-layout'
export { hasCornerPin } from '@editor/runtime/composition-runtime/utils/corner-pin'
