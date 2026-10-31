/**
 * Adapter exports for composition-runtime dependencies.
 * Preview modules should import composition-runtime modules from here.
 */

export { MainComposition } from '@editor/runtime/composition-runtime/compositions/main-composition'
export {
  resolveTransform,
  getSourceDimensions,
} from '@editor/runtime/composition-runtime/utils/transform-resolver'
export { resolveItemTransformAtFrame } from '@editor/runtime/composition-runtime/utils/frame-scene'
export type { PreviewPathVerticesOverride } from '@editor/runtime/composition-runtime/utils/preview-path-override'
export { expandTextTransformToFitContent } from '@editor/runtime/composition-runtime/utils/text-layout'
export {
  computeCornerPinHomography,
  invertCornerPinHomography,
  hasCornerPin,
  resolveCornerPinTargetRect,
  resolveCornerPinForSize,
  withCornerPinReferenceSize,
} from '@editor/runtime/composition-runtime/utils/corner-pin'
export { getBestDomVideoElementForItem } from '@editor/runtime/composition-runtime/utils/dom-video-element-registry'
export {
  getVideoTargetTimeSeconds,
  snapSourceTime,
} from '@editor/runtime/composition-runtime/utils/video-timing'
export { resolveTrackRenderState } from '@editor/runtime/composition-runtime/utils/scene-assembly'
export {
  ensureAudioContextResumed,
  getPreviewAudioContextState,
  transitionSafePlay,
} from '@editor/runtime/composition-runtime/components/video-audio-context'
