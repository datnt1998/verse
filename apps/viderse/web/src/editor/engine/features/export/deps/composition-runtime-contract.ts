/**
 * Adapter exports for composition-runtime dependencies.
 * Export modules should import composition-runtime utilities from here.
 */

export {
  createFrameCompositionSceneCache,
  resolveItemTransformAtFrame,
  resolveActiveShapeMasksAtFrame,
} from '@editor/runtime/composition-runtime/utils/frame-scene'
export {
  applyPreviewPathVerticesToItem,
  applyPreviewPathVerticesToShape,
  type PreviewPathVerticesOverride,
} from '@editor/runtime/composition-runtime/utils/preview-path-override'
export { expandTextTransformToFitContent } from '@editor/runtime/composition-runtime/utils/text-layout'
export {
  resolveCompositionRenderPlan,
  resolveLiveTransitionRenderPlan,
  collectFrameVideoCandidates,
  resolveFrameRenderScene,
  resolveTrackRenderState,
} from '@editor/runtime/composition-runtime/utils/scene-assembly'
export type { FrameRenderTask } from '@editor/runtime/composition-runtime/utils/scene-assembly'
export { getShapePath, rotatePath } from '@editor/runtime/composition-runtime/utils/shape-path'
export {
  hasCornerPin,
  computeCornerPinHomography,
  invertCornerPinHomography,
  drawCornerPinImage,
  computeProjectiveCornerPinWarp,
  resolveCornerPinTargetRect,
  resolveCornerPinForSize,
} from '@editor/runtime/composition-runtime/utils/corner-pin'
export { getVideoTargetTimeSeconds } from '@editor/runtime/composition-runtime/utils/video-timing'
