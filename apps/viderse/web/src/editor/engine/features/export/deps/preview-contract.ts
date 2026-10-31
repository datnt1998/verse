/**
 * Adapter exports for preview dependencies.
 * Export modules should import preview utilities from here.
 */

export { ScrubbingCache } from '@editor/features/preview/utils/scrubbing-cache'
export { getCachedPredecodedBitmap } from '@editor/features/preview/utils/decoder-prewarm'
export { getCachedActivePreviewFallbackBitmap } from '@editor/features/preview/utils/decoder-prewarm'
export { isActivePreviewTargetSuperseded } from '@editor/features/preview/utils/decoder-prewarm'
export { isActivePreviewFrameSuperseded } from '@editor/features/preview/utils/decoder-prewarm'
export { isActivePreviewFrameCurrent } from '@editor/features/preview/utils/decoder-prewarm'
export { isActivePreviewFrameDecodeReady } from '@editor/features/preview/utils/decoder-prewarm'
export { isActivePreviewSourceTarget } from '@editor/features/preview/utils/decoder-prewarm'
export { waitForInflightPredecodedBitmap } from '@editor/features/preview/utils/decoder-prewarm'
