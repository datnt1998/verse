import type { TimelineItem, TimelineTrack, ProjectMarker } from '@editor/types/timeline'
import type { AudioEqSettings } from '@editor/types/audio'
import type { Transition } from '@editor/types/transition'
import type { ItemKeyframes } from '@editor/types/keyframe'
import type { SubComposition } from '@editor/features/timeline/stores/compositions-store'
import type { ProjectResolution } from '@editor/types/project'

/**
 * Timeline data captured by editing gestures and the independent Freecut test oracle.
 * Excludes transient dirty flags; the canonical project adapter extends this data shape.
 */
export interface TimelineSnapshot {
  items: TimelineItem[]
  tracks: TimelineTrack[]
  transitions: Transition[]
  keyframes: ItemKeyframes[]
  markers: ProjectMarker[]
  compositions: SubComposition[]
  /** Standalone-timeline tab membership — undone with the compositions it references. */
  topLevelSequenceIds: string[]
  inPoint: number | null
  outPoint: number | null
  fps: number
  scrollPosition: number
  snapEnabled: boolean
  currentFrame: number
  busAudioEq?: AudioEqSettings
  /** Project-scoped master bus gain in dB (0 = unity). */
  masterBusDb: number
  projectId: string | null
  projectMetadata: ProjectResolution | null
}

