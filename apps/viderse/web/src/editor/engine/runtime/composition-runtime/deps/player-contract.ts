/**
 * Adapter exports for player-layer dependencies.
 * Composition runtime modules should import player hooks/components from here.
 */

export {
  AbsoluteFill,
  Sequence,
  SequenceContext,
  interpolate,
  useSequenceContext,
} from '@editor/runtime/player/composition'
export { VideoConfigProvider } from '@editor/runtime/player/VideoConfigProvider'
export { useVideoConfig } from '@editor/runtime/player/video-config-context'
export { useBridgedCurrentFrame, useBridgedIsPlaying } from '@editor/runtime/player/clock'
export {
  useClock,
  useClockFrameSelector,
  useClockPlaybackRate,
} from '@editor/runtime/player/clock/clock-hooks'
export { useVideoSourcePool } from '@editor/runtime/player/video/VideoSourcePoolContext'
export { isVideoPoolAbortError } from '@editor/runtime/player/video/VideoSourcePool'
