/**
 * Adapter exports for timeline dependencies.
 * Keyframes modules should import timeline stores from here.
 */

export { useTimelineStore } from '@editor/features/timeline/stores/timeline-store'
export { useItemsStore } from '@editor/features/timeline/stores/items-store'
export { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
export { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
export {
  getEdgeScrollDelta,
  getPlayheadEdgeScrollVelocity,
} from '@editor/features/timeline/utils/playhead-edge-scroll'
