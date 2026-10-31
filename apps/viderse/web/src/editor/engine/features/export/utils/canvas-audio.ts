/**
 * Canvas Audio Processing System
 *
 * Handles audio extraction, processing, mixing, and encoding for client-side export.
 * Supports audio from video items and standalone audio items.
 */

import type { CompositionInputProps } from '@editor/types/export'
import type {
  VideoItem,
  AudioItem,
  AudioDuckingSettings,
  CompositionItem,
  TimelineItem,
  TimelineTrack,
} from '@editor/types/timeline'
import type { AudioEqSettings, ResolvedAudioEqSettings } from '@editor/types/audio'
import type { Keyframe as VolumeKeyframe } from '@editor/types/keyframe'
import type { Transition } from '@editor/types/transition'
import { createLogger } from '@editor/shared/logging/logger'
import { resolveTransitionWindows } from '@editor/shared/timeline/transitions/transition-planner'
import {
  timelineToSourceFrames,
  sourceToTimelineFrames,
} from '@editor/features/export/deps/timeline-frame'
import {
  useCompositionsStore,
  collectReachableCompositionIdsFromTracks,
} from '@editor/features/export/deps/timeline-compositions'
import { getPropertyKeyframes, interpolatePropertyValue } from '@editor/features/export/deps/keyframes'
import { blobUrlManager } from '@editor/infrastructure/browser/blob-url-manager'
import { getMediaAudioCodecById, resolveMediaUrl } from '@editor/features/export/deps/media-library'
import { ensureAc3DecoderRegistered, isAc3AudioCodec } from '@editor/shared/utils/ac3-decoder'
import {
  getLinkedAudioCompanion,
  getLinkedCompositionAudioCompanion,
  getLinkedVideoIdsWithAudio,
  isCompositionAudioItem,
} from '@editor/shared/utils/linked-media'
import {
  evaluateAudioFadeInCurve,
  evaluateAudioFadeOutCurve,
  type AudioClipFadeSpan,
} from '@editor/shared/utils/audio-fade-curve'
import { createMediabunnyInputSource } from '@editor/infrastructure/browser/mediabunny-input-source'
import {
  appendResolvedAudioEqSources,
  applyAudioEqStages,
  areAudioEqStagesEqual,
  getAudioEqSettings,
  isAudioEqStageActive,
} from '@editor/shared/utils/audio-eq'
import {
  getAudioPitchRatioFromSemitones,
  getAudioPitchShiftSemitones,
  isAudioPitchShiftActive,
} from '@editor/shared/utils/audio-pitch'
import {
  StreamingSegmentProcessor,
} from '@editor/features/export/utils/audio-stream/streaming-segment-dsp'
import {
  PolyphaseResampler,
  resamplePcm,
} from '@editor/features/export/utils/audio-stream/polyphase-resampler'
import { PlanarFrameQueue } from '@editor/features/export/utils/audio-stream/planar-queue'
import type { AudioFxEffect } from '@editor/types/audio'
import { createAudioFxStages, createAudioFxChain } from '@editor/features/audio-fx/stages'

const log = createLogger('CanvasAudio')

// =============================================================================
// PERFORMANCE OPTIMIZATION: Audio Decode Cache
// =============================================================================

/**
 * Cache for decoded audio to avoid re-decoding the same source file.
 * Key: source URL, Value: decoded audio data
 */
const audioDecodeCache = new Map<string, DecodedAudio>()

/**
 * Clear the audio decode cache (call after export completes)
 */
export function clearAudioDecodeCache(): void {
  audioDecodeCache.clear()
  log.debug('Audio decode cache cleared')
}

/**
 * Audio segment representing a timeline item's audio
 */
interface AudioSegment {
  itemId: string
  trackId: string
  src: string
  startFrame: number // Timeline position
  durationFrames: number
  sourceStartFrame: number // In source media (for trim) — in source-native FPS frames
  sourceFps: number // Source media FPS (sourceStartFrame is in these frames)
  volume: number // -60 to +12 dB
  fadeInFrames: number
  fadeOutFrames: number
  fadeInCurve: number
  fadeOutCurve: number
  fadeInCurveX: number
  fadeOutCurveX: number
  pitchShiftSemitones: number
  audioEqStages: ResolvedAudioEqSettings[]
  audioFx?: AudioFxEffect[]
  contentStartOffsetFrames?: number
  contentEndOffsetFrames?: number
  fadeInDelayFrames?: number
  fadeOutLeadFrames?: number
  clipFadeSpans?: AudioClipFadeSpan[]
  crossfadeFadeInFrames?: number
  crossfadeFadeOutFrames?: number
  speed: number // Playback rate
  isReversed: boolean
  muted: boolean
  type: 'video' | 'audio'
  audioCodec?: string // Audio codec for lazy AC-3 decoder registration
  volumeKeyframes?: VolumeKeyframe[] // Animated volume keyframes
  itemFrom: number // Item's timeline start frame (for keyframe offset)
}

export interface AudioPacketPassthroughPlan {
  src: string
  durationSeconds: number
}

type TransitionAudioItem = VideoItem | AudioItem

interface TransitionAudioEntry<TItem extends TransitionAudioItem> {
  item: TItem
  trackId: string
  muted: boolean
  trackVolume: number
  trackAudioEq?: AudioEqSettings
  audioEqStages?: ResolvedAudioEqSettings[]
  type: 'video' | 'audio'
  audioCodec?: string
  volumeKeyframes?: VolumeKeyframe[]
  itemFrom: number
}

function isMediaPair(left: TimelineItem, right: TimelineItem): boolean {
  return (
    (left.type === 'video' && right.type === 'audio') ||
    (left.type === 'audio' && right.type === 'video')
  )
}

function getHeuristicTrimStart(item: TimelineItem): number | null {
  const timelineItem = item as TimelineItem & { offset?: number }
  return timelineItem.sourceStart ?? timelineItem.trimStart ?? timelineItem.offset ?? null
}

function isImportedLegacyLinkedPair(anchor: TimelineItem, candidate: TimelineItem): boolean {
  if (!isMediaPair(anchor, candidate)) return false
  if (anchor.linkedGroupId || candidate.linkedGroupId) return false
  if (anchor.originId || candidate.originId) return false
  if (!anchor.mediaId || anchor.mediaId !== candidate.mediaId) return false
  if (anchor.from !== candidate.from) return false
  if (anchor.durationInFrames !== candidate.durationInFrames) return false
  if (getHeuristicTrimStart(anchor) !== getHeuristicTrimStart(candidate)) return false
  if ((anchor.sourceEnd ?? null) !== (candidate.sourceEnd ?? null)) return false
  return (anchor.speed ?? 1) === (candidate.speed ?? 1)
}

function getLinkedAudioCompanionForExport(
  items: TimelineItem[],
  anchor: TimelineItem,
): AudioItem | null {
  const linked = getLinkedAudioCompanion(items, anchor)
  if (linked) return linked
  if (anchor.type !== 'video') return null
  return (
    (items.find(
      (candidate) => candidate.type === 'audio' && isImportedLegacyLinkedPair(anchor, candidate),
    ) as AudioItem | undefined) ?? null
  )
}

function getLinkedVideoIdsWithAudioForExport(items: TimelineItem[]): Set<string> {
  const linkedVideoIds = new Set<string>()

  for (const item of items) {
    if (item.type !== 'video') continue
    if (getLinkedAudioCompanionForExport(items, item)) {
      linkedVideoIds.add(item.id)
    }
  }

  return linkedVideoIds
}

function getManagedLinkedAudioTransitionsForExport(
  items: TimelineItem[],
  transitions: Transition[],
): Array<{ transition: Transition; leftAudio: AudioItem; rightAudio: AudioItem }> {
  const itemById = new Map(items.map((item) => [item.id, item]))
  const managed: Array<{ transition: Transition; leftAudio: AudioItem; rightAudio: AudioItem }> = []

  for (const transition of transitions) {
    const leftClip = itemById.get(transition.leftClipId)
    const rightClip = itemById.get(transition.rightClipId)
    if (leftClip?.type !== 'video' || rightClip?.type !== 'video') continue

    const leftAudio = getLinkedAudioCompanionForExport(items, leftClip)
    const rightAudio = getLinkedAudioCompanionForExport(items, rightClip)
    if (!leftAudio || !rightAudio) continue
    if (leftAudio.trackId !== rightAudio.trackId) continue
    if (leftAudio.from !== leftClip.from || rightAudio.from !== rightClip.from) continue
    if (
      leftAudio.durationInFrames !== leftClip.durationInFrames ||
      rightAudio.durationInFrames !== rightClip.durationInFrames
    )
      continue

    managed.push({ transition, leftAudio, rightAudio })
  }

  return managed
}

function buildClipFadeSpan(params: {
  startFrame: number
  durationInFrames: number
  fadeInFrames?: number
  fadeOutFrames?: number
  fadeInCurve?: number
  fadeOutCurve?: number
  fadeInCurveX?: number
  fadeOutCurveX?: number
}): AudioClipFadeSpan {
  return {
    startFrame: params.startFrame,
    durationInFrames: params.durationInFrames,
    fadeInFrames: params.fadeInFrames ?? 0,
    fadeOutFrames: params.fadeOutFrames ?? 0,
    fadeInCurve: params.fadeInCurve ?? 0,
    fadeOutCurve: params.fadeOutCurve ?? 0,
    fadeInCurveX: params.fadeInCurveX ?? 0.52,
    fadeOutCurveX: params.fadeOutCurveX ?? 0.52,
  }
}

function getTransitionAudioTrimBefore(item: TransitionAudioItem): number {
  return item.sourceStart ?? item.trimStart ?? item.offset ?? 0
}

function hasExplicitTransitionAudioTrimStart(item: TransitionAudioItem): boolean {
  return item.sourceStart !== undefined || item.trimStart !== undefined || item.offset !== undefined
}

function isContinuousAudioTransition(
  left: TransitionAudioItem,
  right: TransitionAudioItem,
  fps: number,
): boolean {
  const leftSpeed = left.speed ?? 1
  const rightSpeed = right.speed ?? 1
  const leftSourceFps = left.sourceFps ?? fps
  if (Math.abs(leftSpeed - rightSpeed) > 0.0001) return false

  const sameMedia =
    (left.mediaId && right.mediaId && left.mediaId === right.mediaId) ||
    (!!left.src && !!right.src && left.src === right.src)
  if (!sameMedia) return false

  if (left.originId && right.originId && left.originId !== right.originId) return false

  if (Math.abs(getAudioPitchShiftSemitones(left) - getAudioPitchShiftSemitones(right)) > 0.0001)
    return false

  const expectedRightFrom = left.from + left.durationInFrames
  if (Math.abs(right.from - expectedRightFrom) > 2) return false

  const leftTrim = getTransitionAudioTrimBefore(left)
  const rightTrim = getTransitionAudioTrimBefore(right)
  const computedLeftSourceEnd =
    leftTrim + timelineToSourceFrames(left.durationInFrames, leftSpeed, fps, leftSourceFps)
  const storedLeftSourceEnd = left.sourceEnd
  const computedContinuous = Math.abs(rightTrim - computedLeftSourceEnd) <= 2
  const storedContinuous =
    storedLeftSourceEnd !== undefined ? Math.abs(rightTrim - storedLeftSourceEnd) <= 2 : false

  if (computedContinuous || storedContinuous) return true

  return !hasExplicitTransitionAudioTrimStart(right)
}

function buildManagedTransitionAudioSegments<TItem extends TransitionAudioItem>(
  entriesById: Map<string, TransitionAudioEntry<TItem>>,
  transitions: Transition[],
  fps: number,
): AudioSegment[] {
  // Entries that don't participate in any transition still yield plain segments below
  // (zero extensions) — bailing out on empty `transitions` would silently drop the
  // embedded audio of every video item in compositions without transitions.
  if (entriesById.size === 0) return []

  const extensionByClipId = new Map<
    string,
    {
      before: number
      after: number
      overlapFadeOut: number
      overlapFadeIn: number
      fadeInDelay: number
      fadeOutLead: number
    }
  >()
  const ensureExtension = (
    clipId: string,
  ): {
    before: number
    after: number
    overlapFadeOut: number
    overlapFadeIn: number
    fadeInDelay: number
    fadeOutLead: number
  } => {
    const existing = extensionByClipId.get(clipId)
    if (existing) return existing
    const created = {
      before: 0,
      after: 0,
      overlapFadeOut: 0,
      overlapFadeIn: 0,
      fadeInDelay: 0,
      fadeOutLead: 0,
    }
    extensionByClipId.set(clipId, created)
    return created
  }

  const clipsById = new Map<string, TItem>()
  for (const [id, entry] of entriesById) {
    clipsById.set(id, entry.item)
  }

  const resolvedWindows = resolveTransitionWindows(transitions, clipsById)
  for (const window of resolvedWindows) {
    const leftEntry = entriesById.get(window.transition.leftClipId)
    const rightEntry = entriesById.get(window.transition.rightClipId)
    if (!leftEntry || !rightEntry) continue

    const left = leftEntry.item
    const right = rightEntry.item
    if (isContinuousAudioTransition(left, right, fps)) continue

    const rightPreRoll = Math.max(0, right.from - window.startFrame)
    const leftPostRoll = Math.max(0, window.endFrame - (left.from + left.durationInFrames))

    if (rightPreRoll > 0) {
      const rightExt = ensureExtension(right.id)
      rightExt.before = Math.max(rightExt.before, rightPreRoll)
    }

    if (leftPostRoll > 0) {
      const leftExt = ensureExtension(left.id)
      leftExt.after = Math.max(leftExt.after, leftPostRoll)
    }

    if (window.durationInFrames > 0) {
      const leftExt = ensureExtension(left.id)
      leftExt.overlapFadeOut = Math.max(leftExt.overlapFadeOut, window.durationInFrames)
      leftExt.fadeOutLead = Math.max(leftExt.fadeOutLead, window.leftPortion)
      const rightExt = ensureExtension(right.id)
      rightExt.overlapFadeIn = Math.max(rightExt.overlapFadeIn, window.durationInFrames)
      rightExt.fadeInDelay = Math.max(rightExt.fadeInDelay, window.rightPortion)
    }
  }

  const resolvedTrimBeforeById = new Map<string, number>()
  const sortedByTrackAndTime = Array.from(entriesById.entries())
    .map(([id, entry]) => ({
      id,
      trackId: entry.trackId,
      item: entry.item,
    }))
    .toSorted((a, b) => {
      if (a.trackId !== b.trackId) return a.trackId.localeCompare(b.trackId)
      if (a.item.from !== b.item.from) return a.item.from - b.item.from
      return a.id.localeCompare(b.id)
    })

  const previousByTrack = new Map<string, TItem>()
  for (const entry of sortedByTrackAndTime) {
    const clip = entry.item
    const explicitTrimBefore = getTransitionAudioTrimBefore(clip)
    let resolvedTrimBefore = explicitTrimBefore

    if (!hasExplicitTransitionAudioTrimStart(clip)) {
      const previous = previousByTrack.get(entry.trackId)
      if (previous && isContinuousAudioTransition(previous, clip, fps)) {
        const previousTrimBefore =
          resolvedTrimBeforeById.get(previous.id) ?? getTransitionAudioTrimBefore(previous)
        resolvedTrimBefore =
          previousTrimBefore +
          timelineToSourceFrames(
            previous.durationInFrames,
            previous.speed ?? 1,
            fps,
            previous.sourceFps ?? fps,
          )
      }
    }

    resolvedTrimBeforeById.set(clip.id, resolvedTrimBefore)
    previousByTrack.set(entry.trackId, clip)
  }

  type ExpandedTransitionAudioSegment = AudioSegment & {
    clip: TransitionAudioItem
    beforeFrames: number
    afterFrames: number
  }

  const expandedSegments: ExpandedTransitionAudioSegment[] = []
  for (const [, entry] of entriesById) {
    const item = entry.item
    const speed = item.speed ?? 1
    const sourceFps = item.sourceFps ?? fps
    const baseTrimBefore = resolvedTrimBeforeById.get(item.id) ?? getTransitionAudioTrimBefore(item)
    const extension = extensionByClipId.get(item.id) ?? {
      before: 0,
      after: 0,
      overlapFadeOut: 0,
      overlapFadeIn: 0,
      fadeInDelay: 0,
      fadeOutLead: 0,
    }
    const maxBeforeBySource =
      speed > 0 ? sourceToTimelineFrames(baseTrimBefore, speed, sourceFps, fps) : 0
    const before = Math.max(0, Math.min(extension.before, maxBeforeBySource))
    const after = Math.max(0, extension.after)
    const crossfadeFadeInFrames =
      extension.overlapFadeIn > 0 ? extension.overlapFadeIn : before > 0 ? before : undefined
    const crossfadeFadeOutFrames =
      extension.overlapFadeOut > 0 ? extension.overlapFadeOut : after > 0 ? after : undefined

    expandedSegments.push({
      itemId: item.id,
      trackId: entry.trackId,
      clip: item,
      src: item.src,
      startFrame: item.from - before,
      durationFrames: item.durationInFrames + before + after,
      sourceStartFrame: baseTrimBefore - timelineToSourceFrames(before, speed, fps, sourceFps),
      sourceFps,
      volume: (item.volume ?? 0) + entry.trackVolume,
      fadeInFrames: (item.audioFadeIn ?? 0) * fps,
      fadeOutFrames: (item.audioFadeOut ?? 0) * fps,
      fadeInCurve: item.audioFadeInCurve ?? 0,
      fadeOutCurve: item.audioFadeOutCurve ?? 0,
      fadeInCurveX: item.audioFadeInCurveX ?? 0.52,
      fadeOutCurveX: item.audioFadeOutCurveX ?? 0.52,
      pitchShiftSemitones: getAudioPitchShiftSemitones(item),
      contentStartOffsetFrames: before,
      contentEndOffsetFrames: after,
      fadeInDelayFrames: extension.fadeInDelay,
      fadeOutLeadFrames: extension.fadeOutLead,
      clipFadeSpans: [
        buildClipFadeSpan({
          startFrame: before,
          durationInFrames: item.durationInFrames,
          fadeInFrames: (item.audioFadeIn ?? 0) * fps,
          fadeOutFrames: (item.audioFadeOut ?? 0) * fps,
          fadeInCurve: item.audioFadeInCurve,
          fadeOutCurve: item.audioFadeOutCurve,
          fadeInCurveX: item.audioFadeInCurveX,
          fadeOutCurveX: item.audioFadeOutCurveX,
        }),
      ],
      crossfadeFadeInFrames,
      crossfadeFadeOutFrames,
      speed,
      isReversed: item.isReversed === true,
      muted: entry.muted,
      type: entry.type,
      audioCodec: entry.audioCodec,
      audioEqStages: appendResolvedAudioEqSources(
        entry.audioEqStages,
        entry.trackAudioEq,
        getAudioEqSettings(item),
      ),
      beforeFrames: before,
      afterFrames: after,
      volumeKeyframes: entry.volumeKeyframes,
      itemFrom: entry.itemFrom,
    })
  }

  const sortedSegments = expandedSegments.toSorted((a, b) => {
    if (a.startFrame !== b.startFrame) return a.startFrame - b.startFrame
    return a.itemId.localeCompare(b.itemId)
  })

  const mergedSegments: AudioSegment[] = []
  let active: ExpandedTransitionAudioSegment | null = null

  const canMergeContinuousBoundary = (
    left: ExpandedTransitionAudioSegment,
    right: ExpandedTransitionAudioSegment,
  ): boolean => {
    if (!isContinuousAudioTransition(left.clip, right.clip, fps)) return false
    if (left.src !== right.src) return false
    if (Math.abs(left.speed - right.speed) > 0.0001) return false
    if (left.isReversed !== right.isReversed) return false
    if (Math.abs(left.volume - right.volume) > 0.0001) return false
    if (left.muted !== right.muted) return false
    if (!areAudioEqStagesEqual(left.audioEqStages, right.audioEqStages)) return false
    if (
      left.clip.audioFx?.some((effect) => effect.enabled) ||
      right.clip.audioFx?.some((effect) => effect.enabled)
    ) return false
    if (Math.abs(left.pitchShiftSemitones - right.pitchShiftSemitones) > 0.0001) return false
    if (left.afterFrames !== 0 || right.beforeFrames !== 0) return false
    if (left.volumeKeyframes || right.volumeKeyframes) return false
    return true
  }

  const toAudioSegment = (segment: ExpandedTransitionAudioSegment): AudioSegment => ({
    itemId: segment.itemId,
    trackId: segment.trackId,
    src: segment.src,
    startFrame: segment.startFrame,
    durationFrames: segment.durationFrames,
    sourceStartFrame: segment.sourceStartFrame,
    sourceFps: segment.sourceFps,
    volume: segment.volume,
    fadeInFrames: segment.fadeInFrames,
    fadeOutFrames: segment.fadeOutFrames,
    fadeInCurve: segment.fadeInCurve,
    fadeOutCurve: segment.fadeOutCurve,
    fadeInCurveX: segment.fadeInCurveX,
    fadeOutCurveX: segment.fadeOutCurveX,
    pitchShiftSemitones: segment.pitchShiftSemitones,
    audioEqStages: segment.audioEqStages,
    audioFx: segment.clip.audioFx,
    contentStartOffsetFrames: segment.contentStartOffsetFrames,
    contentEndOffsetFrames: segment.contentEndOffsetFrames,
    fadeInDelayFrames: segment.fadeInDelayFrames,
    fadeOutLeadFrames: segment.fadeOutLeadFrames,
    clipFadeSpans: segment.clipFadeSpans,
    crossfadeFadeInFrames: segment.crossfadeFadeInFrames,
    crossfadeFadeOutFrames: segment.crossfadeFadeOutFrames,
    speed: segment.speed,
    isReversed: segment.isReversed,
    muted: segment.muted,
    type: segment.type,
    audioCodec: segment.audioCodec,
    volumeKeyframes: segment.volumeKeyframes,
    itemFrom: segment.itemFrom,
  })

  for (const segment of sortedSegments) {
    if (!active) {
      active = { ...segment }
      continue
    }

    if (canMergeContinuousBoundary(active, segment)) {
      const activeStartFrame = active.startFrame
      const mergedEnd = segment.startFrame + segment.durationFrames
      active.durationFrames = mergedEnd - active.startFrame
      active.fadeOutFrames = segment.fadeOutFrames
      active.fadeOutCurve = segment.fadeOutCurve
      active.fadeOutCurveX = segment.fadeOutCurveX
      active.contentEndOffsetFrames = segment.contentEndOffsetFrames
      active.fadeOutLeadFrames = segment.fadeOutLeadFrames
      active.crossfadeFadeOutFrames = segment.crossfadeFadeOutFrames
      active.clipFadeSpans = [
        ...(active.clipFadeSpans ?? []),
        ...(segment.clipFadeSpans ?? []).map((span) => ({
          ...span,
          startFrame: span.startFrame + (segment.startFrame - activeStartFrame),
        })),
      ]
      active.clip = segment.clip
      active.afterFrames = segment.afterFrames
      continue
    }

    mergedSegments.push(toAudioSegment(active))
    active = { ...segment }
  }

  if (active) {
    mergedSegments.push(toAudioSegment(active))
  }

  return mergedSegments
}

/**
 * Decoded audio data
 */
interface DecodedAudio {
  itemId: string
  sampleRate: number
  channels: number
  samples: Float32Array[] // Per-channel samples
  duration: number // Duration in seconds
}

/**
 * Audio processing configuration
 */
interface AudioProcessingConfig {
  sampleRate: number
  channels: number
  fps: number
  totalFrames: number
}

/** The wrapper composition item's timing, resolved once per expansion. */
interface CompositionWrapperTiming {
  compFrom: number
  wrapperSpeed: number
  wrapperSourceFps: number
  sourceOffset: number
  wrapperSourceEnd: number
}

function resolveCompositionWrapper(
  compositionItem: { from: number; durationInFrames: number } & Partial<{
    speed: number
    sourceFps: number
    sourceStart: number
    trimStart: number
    sourceEnd: number
  }>,
  fps: number,
): CompositionWrapperTiming {
  const wrapperSpeed = compositionItem.speed ?? 1
  const wrapperSourceFps = compositionItem.sourceFps ?? fps
  const sourceOffset = compositionItem.sourceStart ?? compositionItem.trimStart ?? 0
  return {
    compFrom: compositionItem.from,
    wrapperSpeed,
    wrapperSourceFps,
    sourceOffset,
    wrapperSourceEnd:
      compositionItem.sourceEnd ??
      sourceOffset +
        timelineToSourceFrames(compositionItem.durationInFrames, wrapperSpeed, fps, wrapperSourceFps),
  }
}

/** One nested item's visible window, expressed in PARENT timeline frames. */
interface NestedItemWindow {
  overlapStart: number
  overlapEnd: number
  effectiveStart: number
  effectiveEnd: number
  effectiveDuration: number
  effectiveSourceStart: number
}

/**
 * Map a sub-composition item into parent-timeline coordinates, honouring the
 * wrapper's trim, speed and source fps. Returns null when the item falls
 * entirely outside the wrapper's visible source range.
 *
 * Shared on purpose. The audio mix and the ducking-source scan both have to
 * agree on where a nested item sits, and a second implementation is how they
 * would silently drift apart — a source could be expanded into the mix while
 * not being seen as a duck source, which is precisely the bug this fixes.
 */
function mapNestedItemWindow(
  subItem: TimelineItem,
  wrapper: CompositionWrapperTiming,
  fps: number,
): NestedItemWindow | null {
  const { compFrom, wrapperSpeed, wrapperSourceFps, sourceOffset, wrapperSourceEnd } = wrapper
  const overlapStart = Math.max(subItem.from, sourceOffset)
  const overlapEnd = Math.min(subItem.from + subItem.durationInFrames, wrapperSourceEnd)
  if (overlapEnd <= overlapStart) return null

  const effectiveStart =
    compFrom + sourceToTimelineFrames(overlapStart - sourceOffset, wrapperSpeed, wrapperSourceFps, fps)
  const effectiveEnd =
    compFrom + sourceToTimelineFrames(overlapEnd - sourceOffset, wrapperSpeed, wrapperSourceFps, fps)
  const effectiveDuration = Math.max(1, effectiveEnd - effectiveStart)

  const baseSourceStart = subItem.sourceStart ?? subItem.trimStart ?? 0
  const effectiveSourceStart =
    baseSourceStart +
    timelineToSourceFrames(
      overlapStart - subItem.from,
      subItem.speed ?? 1,
      wrapperSourceFps,
      subItem.sourceFps ?? wrapperSourceFps,
    )

  return {
    overlapStart,
    overlapEnd,
    effectiveStart,
    effectiveEnd,
    effectiveDuration,
    effectiveSourceStart,
  }
}

/**
 * Rebuild a nested composition item as a wrapper expressed in PARENT coordinates.
 *
 * Shared with the duck-source scan for the same reason the window mapping is: the
 * `sourceEnd` remap in particular is easy to omit, and omitting it makes a
 * clipped intermediate composition report a different window on each side.
 */
function buildNestedWrapper(
  subItem: TimelineItem,
  window: NestedItemWindow,
  wrapper: CompositionWrapperTiming,
): CompositionItem | (AudioItem & { compositionId: string }) {
  const { wrapperSpeed, wrapperSourceFps } = wrapper
  return {
    ...subItem,
    from: window.effectiveStart,
    durationInFrames: window.effectiveDuration,
    speed: (subItem.speed ?? 1) * wrapperSpeed,
    sourceStart: window.effectiveSourceStart,
    sourceFps: subItem.sourceFps ?? wrapperSourceFps,
    ...(subItem.sourceEnd !== undefined && {
      sourceEnd: Math.max(
        window.effectiveSourceStart + 1,
        subItem.sourceEnd -
          timelineToSourceFrames(
            subItem.from + subItem.durationInFrames - window.overlapEnd,
            subItem.speed ?? 1,
            wrapperSourceFps,
            subItem.sourceFps ?? wrapperSourceFps,
          ),
      ),
    }),
  } as CompositionItem | (AudioItem & { compositionId: string })
}

function appendCompositionAudioSegments(params: {
  segments: AudioSegment[]
  track: CompositionInputProps['tracks'][number]
  compositionItem: CompositionItem | (AudioItem & { compositionId: string })
  subComp: {
    items: TimelineItem[]
    tracks: TimelineTrack[]
    keyframes?: CompositionInputProps['keyframes']
    durationInFrames: number
  }
  fps: number
  audioEqStages?: ResolvedAudioEqSettings[]
  audioPitchShiftSemitones?: number
  visited?: Set<string>
}): void {
  const { segments, track, compositionItem, subComp, fps } = params
  const visited = params.visited ?? new Set<string>()
  const wrapperAudioEqStages = appendResolvedAudioEqSources(
    params.audioEqStages,
    getAudioEqSettings(compositionItem),
  )
  const wrapperAudioPitchShiftSemitones =
    (params.audioPitchShiftSemitones ?? 0) + getAudioPitchShiftSemitones(compositionItem)
  const linkedSubCompVideoIds = getLinkedVideoIdsWithAudio(subComp.items)
  const wrapper = resolveCompositionWrapper(compositionItem, fps)
  const { wrapperSpeed, wrapperSourceFps } = wrapper
  const trackMuted = track.muted ?? false

  for (const subItem of subComp.items) {
    const subTrack = subComp.tracks.find((candidate) => candidate.id === subItem.trackId)
    const subTrackMuted = subTrack?.muted ?? false
    const window = mapNestedItemWindow(subItem, wrapper, fps)
    if (!window) continue
    const { overlapStart, overlapEnd, effectiveStart, effectiveDuration } = window

    const speed = (subItem.speed ?? 1) * wrapperSpeed
    const effectiveSourceStart = window.effectiveSourceStart

    if (subItem.type === 'composition' || isCompositionAudioItem(subItem)) {
      if (
        subItem.type === 'composition' &&
        getLinkedCompositionAudioCompanion(subComp.items, subItem)
      )
        continue
      if (visited.has(subItem.compositionId)) continue

      const nestedSubComp = useCompositionsStore.getState().getComposition(subItem.compositionId)
      if (!nestedSubComp) continue

      const nestedWrapper = buildNestedWrapper(subItem, window, wrapper)

      // Volumes are dB offsets — sum them so nested levels accumulate correctly.
      const nestedVisited = new Set(visited)
      nestedVisited.add(subItem.compositionId)
      appendCompositionAudioSegments({
        segments,
        track: {
          ...track,
          muted: trackMuted || subTrackMuted,
          volume: (track.volume ?? 0) + (subTrack?.volume ?? 0),
        },
        compositionItem: nestedWrapper,
        subComp: nestedSubComp,
        fps,
        audioEqStages: appendResolvedAudioEqSources(wrapperAudioEqStages, subTrack?.audioEq),
        audioPitchShiftSemitones: wrapperAudioPitchShiftSemitones,
        visited: nestedVisited,
      })
      continue
    }

    if (subItem.type !== 'video' && subItem.type !== 'audio') continue
    if (subItem.type === 'video' && linkedSubCompVideoIds.has(subItem.id)) continue
    const src =
      (subItem.mediaId ? blobUrlManager.get(subItem.mediaId) : null) ??
      (subItem as VideoItem | AudioItem).src ??
      ''
    if (!src) continue

    const subItemKeyframes = subComp.keyframes?.find((keyframe) => keyframe.itemId === subItem.id)
    const subVolumeKfs = getPropertyKeyframes(subItemKeyframes, 'volume')

    const rawFadeInFrames = sourceToTimelineFrames(
      (subItem.audioFadeIn ?? 0) * wrapperSourceFps,
      wrapperSpeed,
      wrapperSourceFps,
      fps,
    )
    const rawFadeOutFrames = sourceToTimelineFrames(
      (subItem.audioFadeOut ?? 0) * wrapperSourceFps,
      wrapperSpeed,
      wrapperSourceFps,
      fps,
    )
    const clippedStartFrames = sourceToTimelineFrames(
      overlapStart - subItem.from,
      wrapperSpeed,
      wrapperSourceFps,
      fps,
    )
    const clippedEndFrames = sourceToTimelineFrames(
      subItem.from + subItem.durationInFrames - overlapEnd,
      wrapperSpeed,
      wrapperSourceFps,
      fps,
    )
    const adjustedFadeInFrames = Math.max(0, rawFadeInFrames - clippedStartFrames)
    const adjustedFadeOutFrames = Math.max(0, rawFadeOutFrames - clippedEndFrames)

    segments.push({
      itemId: subItem.id,
      trackId: track.id,
      src,
      startFrame: effectiveStart,
      durationFrames: effectiveDuration,
      sourceStartFrame: effectiveSourceStart,
      sourceFps: subItem.sourceFps ?? fps,
      volume: (subItem.volume ?? 0) + (track.volume ?? 0) + (subTrack?.volume ?? 0),
      fadeInFrames: adjustedFadeInFrames,
      fadeOutFrames: adjustedFadeOutFrames,
      fadeInCurve: subItem.audioFadeInCurve ?? 0,
      fadeOutCurve: subItem.audioFadeOutCurve ?? 0,
      fadeInCurveX: subItem.audioFadeInCurveX ?? 0.52,
      fadeOutCurveX: subItem.audioFadeOutCurveX ?? 0.52,
      pitchShiftSemitones: wrapperAudioPitchShiftSemitones + getAudioPitchShiftSemitones(subItem),
      audioEqStages: appendResolvedAudioEqSources(
        wrapperAudioEqStages,
        subTrack?.audioEq,
        getAudioEqSettings(subItem),
      ),
      audioFx: subItem.audioFx,
      contentStartOffsetFrames: 0,
      contentEndOffsetFrames: 0,
      fadeInDelayFrames: 0,
      fadeOutLeadFrames: 0,
      clipFadeSpans: [
        buildClipFadeSpan({
          startFrame: 0,
          durationInFrames: effectiveDuration,
          fadeInFrames: adjustedFadeInFrames,
          fadeOutFrames: adjustedFadeOutFrames,
          fadeInCurve: subItem.audioFadeInCurve,
          fadeOutCurve: subItem.audioFadeOutCurve,
          fadeInCurveX: subItem.audioFadeInCurveX,
          fadeOutCurveX: subItem.audioFadeOutCurveX,
        }),
      ],
      speed,
      isReversed: subItem.isReversed === true,
      muted: trackMuted || subTrackMuted,
      type: subItem.type as 'video' | 'audio',
      audioCodec: getMediaAudioCodecById(subItem.mediaId),
      volumeKeyframes: subVolumeKfs.length > 0 ? subVolumeKfs : undefined,
      itemFrom: effectiveStart,
    })
  }
}

/**
 * Extract audio segments from composition.
 *
 * @param composition - The composition with tracks
 * @returns Array of audio segments to process
 */
export function extractAudioSegments(
  composition: CompositionInputProps,
  fps: number,
): AudioSegment[] {
  const { tracks = [], transitions = [] } = composition
  const segments: AudioSegment[] = []
  const audioOnlySegments: AudioSegment[] = []
  const videoById = new Map<string, TransitionAudioEntry<VideoItem>>()
  const audioById = new Map<string, TransitionAudioEntry<AudioItem>>()
  const managedLinkedAudioById = new Map<string, TransitionAudioEntry<AudioItem>>()
  const timelineItems = tracks.flatMap((track) => track.items)
  const linkedRootVideoIds = getLinkedVideoIdsWithAudioForExport(timelineItems)
  const managedLinkedAudioTransitions = getManagedLinkedAudioTransitionsForExport(
    timelineItems,
    transitions,
  )
  const managedLinkedAudioIds = new Set<string>()
  for (const managed of managedLinkedAudioTransitions) {
    managedLinkedAudioIds.add(managed.leftAudio.id)
    managedLinkedAudioIds.add(managed.rightAudio.id)
  }
  const managedLinkedAudioTransitionDefs: Transition[] = managedLinkedAudioTransitions.map(
    ({ transition, leftAudio, rightAudio }) => ({
      ...transition,
      leftClipId: leftAudio.id,
      rightClipId: rightAudio.id,
      trackId: leftAudio.trackId,
    }),
  )
  const busAudioEqStages = appendResolvedAudioEqSources(undefined, composition.busAudioEq)
  const audioTransitionItemIds = new Set<string>()
  const audioTransitionDefs: Transition[] = transitions.filter((transition) => {
    const leftItem = timelineItems.find((item) => item.id === transition.leftClipId)
    const rightItem = timelineItems.find((item) => item.id === transition.rightClipId)
    if (leftItem?.type !== 'audio' || rightItem?.type !== 'audio') {
      return false
    }
    if (isCompositionAudioItem(leftItem) || isCompositionAudioItem(rightItem)) {
      return false
    }
    audioTransitionItemIds.add(leftItem.id)
    audioTransitionItemIds.add(rightItem.id)
    return true
  })

  for (const track of tracks) {
    if (track.visible === false) continue

    for (const item of track.items) {
      if (item.type === 'video') {
        const videoItem = item as VideoItem
        if (linkedRootVideoIds.has(videoItem.id)) continue
        if (videoItem.embeddedAudioMuted) continue
        if (!videoItem.src) continue
        videoById.set(item.id, {
          item: videoItem,
          trackId: track.id,
          muted: track.muted ?? false,
          trackVolume: track.volume ?? 0,
          trackAudioEq: track.audioEq,
          audioEqStages: busAudioEqStages,
          type: 'video',
          audioCodec: getMediaAudioCodecById(videoItem.mediaId),
          volumeKeyframes: (() => {
            const videoItemKeyframes = composition.keyframes?.find((k) => k.itemId === item.id)
            const videoVolumeKfs = getPropertyKeyframes(videoItemKeyframes, 'volume')
            return videoVolumeKfs.length > 0 ? videoVolumeKfs : undefined
          })(),
          itemFrom: videoItem.from,
        })
      } else if (item.type === 'audio') {
        const audioItem = item as AudioItem
        if (isCompositionAudioItem(audioItem)) {
          const subComp = useCompositionsStore.getState().getComposition(audioItem.compositionId)
          if (!subComp) continue
          appendCompositionAudioSegments({
            segments: audioOnlySegments,
            track,
            compositionItem: audioItem,
            subComp,
            fps,
            audioEqStages: appendResolvedAudioEqSources(busAudioEqStages, track.audioEq),
          })
          continue
        }
        if (!audioItem.src) continue

        // Use sourceStart as primary for consistency with video items
        // This ensures split audio clips and IO markers work correctly
        const audioItemKeyframes = composition.keyframes?.find((k) => k.itemId === item.id)
        const audioVolumeKfs = getPropertyKeyframes(audioItemKeyframes, 'volume')
        const audioEntry: TransitionAudioEntry<AudioItem> = {
          item: audioItem,
          trackId: track.id,
          muted: track.muted ?? false,
          trackVolume: track.volume ?? 0,
          trackAudioEq: track.audioEq,
          audioEqStages: busAudioEqStages,
          type: 'audio',
          audioCodec: getMediaAudioCodecById(item.mediaId),
          volumeKeyframes: audioVolumeKfs.length > 0 ? audioVolumeKfs : undefined,
          itemFrom: item.from,
        }

        if (managedLinkedAudioIds.has(item.id)) {
          managedLinkedAudioById.set(item.id, audioEntry)
          continue
        }

        if (audioTransitionItemIds.has(item.id)) {
          audioById.set(item.id, audioEntry)
          continue
        }

        audioOnlySegments.push({
          itemId: item.id,
          trackId: track.id,
          src: audioItem.src,
          startFrame: item.from,
          durationFrames: item.durationInFrames,
          sourceStartFrame: audioItem.sourceStart ?? item.trimStart ?? 0,
          sourceFps: audioItem.sourceFps ?? fps,
          volume: (item.volume ?? 0) + (track.volume ?? 0),
          fadeInFrames: (item.audioFadeIn ?? 0) * fps,
          fadeOutFrames: (item.audioFadeOut ?? 0) * fps,
          fadeInCurve: item.audioFadeInCurve ?? 0,
          fadeOutCurve: item.audioFadeOutCurve ?? 0,
          fadeInCurveX: item.audioFadeInCurveX ?? 0.52,
          fadeOutCurveX: item.audioFadeOutCurveX ?? 0.52,
          pitchShiftSemitones: getAudioPitchShiftSemitones(item),
          audioEqStages: appendResolvedAudioEqSources(
            busAudioEqStages,
            track.audioEq,
            getAudioEqSettings(item),
          ),
          audioFx: item.audioFx,
          contentStartOffsetFrames: 0,
          contentEndOffsetFrames: 0,
          fadeInDelayFrames: 0,
          fadeOutLeadFrames: 0,
          clipFadeSpans: [
            buildClipFadeSpan({
              startFrame: 0,
              durationInFrames: item.durationInFrames,
              fadeInFrames: (item.audioFadeIn ?? 0) * fps,
              fadeOutFrames: (item.audioFadeOut ?? 0) * fps,
              fadeInCurve: item.audioFadeInCurve,
              fadeOutCurve: item.audioFadeOutCurve,
              fadeInCurveX: item.audioFadeInCurveX,
              fadeOutCurveX: item.audioFadeOutCurveX,
            }),
          ],
          speed: audioItem.speed ?? 1,
          isReversed: audioItem.isReversed === true,
          muted: track.muted ?? false,
          type: 'audio',
          audioCodec: audioEntry.audioCodec,
          volumeKeyframes: audioEntry.volumeKeyframes,
          itemFrom: item.from,
        })
      }
    }
  }

  const managedVideoSegments = buildManagedTransitionAudioSegments(videoById, transitions, fps)
  const managedAudioSegments = buildManagedTransitionAudioSegments(
    audioById,
    audioTransitionDefs,
    fps,
  )
  const managedLinkedAudioSegments = buildManagedTransitionAudioSegments(
    managedLinkedAudioById,
    managedLinkedAudioTransitionDefs,
    fps,
  )

  segments.push(
    ...managedVideoSegments,
    ...managedAudioSegments,
    ...managedLinkedAudioSegments,
    ...audioOnlySegments,
  )

  // === Extract audio from sub-compositions (pre-comps) ===
  // Composition items reference sub-comps that may contain video/audio items with audio.
  // We offset each sub-comp audio segment by the composition item's timeline position.
  for (const track of tracks) {
    if (track.visible === false) continue
    for (const item of track.items) {
      if (item.type !== 'composition') continue
      const compItem = item as CompositionItem
      if (getLinkedCompositionAudioCompanion(timelineItems, compItem)) continue
      const subComp = useCompositionsStore.getState().getComposition(compItem.compositionId)
      if (!subComp) continue
      appendCompositionAudioSegments({
        segments,
        track,
        compositionItem: compItem,
        subComp,
        fps,
        audioEqStages: appendResolvedAudioEqSources(busAudioEqStages, track.audioEq),
      })
    }
  }

  log.info('Extracted audio segments', {
    count: segments.length,
    videoCount: segments.filter((s) => s.type === 'video').length,
    audioCount: segments.filter((s) => s.type === 'audio').length,
  })

  return segments
}

type MediabunnyAudioSampleSink = InstanceType<(typeof import('mediabunny'))['AudioSampleSink']>
type MediabunnyAudioSample = InstanceType<(typeof import('mediabunny'))['AudioSample']>

interface AudioDecodeAccumulator {
  channelChunks: Float32Array[][]
  totalFrames: number
  sampleRate: number
  channels: number
}

function createAudioChannelBackfill(totalFrames: number): Float32Array[] {
  if (totalFrames === 0) return []
  return [new Float32Array(totalFrames)]
}

function appendDecodedAudioSample(
  state: AudioDecodeAccumulator,
  sample: MediabunnyAudioSample,
  itemId: string,
): void {
  const frameCount = Math.max(0, sample.numberOfFrames)
  const sampleChannels = Math.max(1, sample.numberOfChannels)
  if (frameCount === 0) return

  if (state.channels === 0) {
    state.channels = sampleChannels
    for (let channel = 0; channel < state.channels; channel++) state.channelChunks.push([])
  } else if (sampleChannels > state.channels) {
    for (let channel = state.channels; channel < sampleChannels; channel++) {
      state.channelChunks.push(createAudioChannelBackfill(state.totalFrames))
    }
    state.channels = sampleChannels
  } else if (sampleChannels < state.channels) {
    log.warn('Inconsistent channel count during mediabunny audio decode', {
      itemId,
      expectedChannels: state.channels,
      actualChannels: sampleChannels,
    })
  }

  for (let channel = 0; channel < state.channels; channel++) {
    const channelData = new Float32Array(frameCount)
    sample.copyTo(channelData, {
      planeIndex: Math.min(channel, sampleChannels - 1),
      format: 'f32-planar',
    })
    state.channelChunks[channel]!.push(channelData)
  }
  state.totalFrames += frameCount
  if (sample.sampleRate > 0) state.sampleRate = sample.sampleRate
}

function mergeDecodedAudioChannels(state: AudioDecodeAccumulator): Float32Array[] {
  return state.channelChunks.map((chunks) => {
    const merged = new Float32Array(state.totalFrames)
    let offset = 0
    for (const chunk of chunks) {
      merged.set(chunk, offset)
      offset += chunk.length
    }
    return merged
  })
}

async function decodeAudioRangeFromSink(
  sink: MediabunnyAudioSampleSink,
  itemId: string,
  startTime: number,
  endTime: number,
): Promise<DecodedAudio> {
  const state: AudioDecodeAccumulator = {
    channelChunks: [],
    totalFrames: 0,
    sampleRate: 48_000,
    channels: 0,
  }

  for await (const sample of sink.samples(startTime, endTime)) {
    try {
      appendDecodedAudioSample(state, sample, itemId)
    } finally {
      sample.close()
    }
  }

  if (state.channels === 0 || state.totalFrames === 0) {
    throw new Error('Audio decode produced no output')
  }

  return {
    itemId,
    sampleRate: state.sampleRate,
    channels: state.channels,
    samples: mergeDecodedAudioChannels(state),
    duration: endTime - startTime,
  }
}

async function decodeAudioWithMediabunny(params: {
  src: string
  itemId: string
  startTime?: number
  endTime?: number
  audioCodec?: string
}): Promise<DecodedAudio> {
  if (isAc3AudioCodec(params.audioCodec)) await ensureAc3DecoderRegistered()
  const mb = await import('mediabunny')
  const input = new mb.Input({
    formats: mb.ALL_FORMATS,
    source: createMediabunnyInputSource(mb, params.src),
  })
  try {
    const audioTrack = await input.getPrimaryAudioTrack()
    if (!audioTrack) throw new Error('No audio track found')
    const duration = await input.computeDuration()
    const actualStartTime = params.startTime ?? 0
    const actualEndTime = params.endTime ?? duration
    log.debug('Extracting audio range', {
      itemId: params.itemId,
      startTime: actualStartTime,
      endTime: actualEndTime,
      totalDuration: duration,
    })
    return await decodeAudioRangeFromSink(
      new mb.AudioSampleSink(audioTrack),
      params.itemId,
      actualStartTime,
      actualEndTime,
    )
  } finally {
    input.dispose()
  }
}

function getCachedAudioDecode(src: string, itemId: string): DecodedAudio | null {
  const cached = audioDecodeCache.get(src)
  if (!cached) return null
  log.debug('Using cached decoded audio', { itemId, src: src.substring(0, 50) })
  return { ...cached, itemId }
}

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

async function recoverAudioDecode(params: {
  src: string
  itemId: string
  startTime?: number
  endTime?: number
  audioCodec?: string
  ac3RetryAttempted: boolean
  allowWebAudioFallback: boolean
  error: unknown
}): Promise<DecodedAudio> {
  if (!params.ac3RetryAttempted && !isAc3AudioCodec(params.audioCodec)) {
    try {
      await ensureAc3DecoderRegistered()
      return await decodeAudioFromSource(
        params.src,
        params.itemId,
        params.startTime,
        params.endTime,
        params.audioCodec,
        true,
        params.allowWebAudioFallback,
      )
    } catch {
      // Continue to Web Audio fallback.
    }
  }
  if (!params.allowWebAudioFallback) throw params.error
  log.warn(
    `Mediabunny audio decode failed for ${params.itemId}, using fallback: ${getErrorMessage(params.error)}`,
  )
  return decodeAudioFallback(params.src, params.itemId, params.startTime, params.endTime)
}

/**
 * Decode audio from a media source using mediabunny for efficient range extraction.
 * Only decodes the portion of audio actually needed, not the entire file.
 *
 * @param src - Source URL (blob URL or regular URL)
 * @param itemId - Item ID for logging
 * @param startTime - Start time in seconds (optional, defaults to 0)
 * @param endTime - End time in seconds (optional, defaults to full duration)
 * @returns Decoded audio data for the specified range
 */
async function decodeAudioFromSource(
  src: string,
  itemId: string,
  startTime?: number,
  endTime?: number,
  audioCodec?: string,
  ac3RetryAttempted: boolean = false,
  allowWebAudioFallback: boolean = true,
): Promise<DecodedAudio> {
  // Check cache first (only for full file decodes for backward compatibility)
  if (startTime === undefined && endTime === undefined) {
    const cached = getCachedAudioDecode(src, itemId)
    if (cached) return cached
  }

  log.debug('Decoding audio with mediabunny', {
    itemId,
    src: src.substring(0, 50),
    startTime,
    endTime,
    audioCodec,
  })

  try {
    const result = await decodeAudioWithMediabunny({
      src,
      itemId,
      startTime,
      endTime,
      audioCodec,
    })

    log.debug('Decoded audio with mediabunny', {
      itemId,
      sampleRate: result.sampleRate,
      channels: result.channels,
      duration: result.duration,
      samples: result.samples[0]?.length,
    })

    if (startTime === undefined && endTime === undefined) audioDecodeCache.set(src, result)

    return result
  } catch (error) {
    return recoverAudioDecode({
      src,
      itemId,
      startTime,
      endTime,
      audioCodec,
      ac3RetryAttempted,
      allowWebAudioFallback,
      error,
    })
  }
}

function sliceDecodedAudioRange(
  decoded: DecodedAudio,
  itemId: string,
  startTime?: number,
  endTime?: number,
): DecodedAudio {
  if (startTime === undefined && endTime === undefined) return { ...decoded, itemId }

  const startFrame = Math.max(0, Math.floor((startTime ?? 0) * decoded.sampleRate))
  const endFrame = Math.max(
    startFrame,
    Math.min(
      decoded.samples[0]?.length ?? 0,
      Math.ceil((endTime ?? decoded.duration) * decoded.sampleRate),
    ),
  )
  return {
    ...decoded,
    itemId,
    samples: decoded.samples.map((samples) => samples.subarray(startFrame, endFrame)),
    duration: (endFrame - startFrame) / decoded.sampleRate,
  }
}

/**
 * Fallback audio decoder using Web Audio API (decodes entire file)
 */
async function decodeAudioFallback(
  src: string,
  itemId: string,
  startTime?: number,
  endTime?: number,
): Promise<DecodedAudio> {
  // Check cache
  const cached = audioDecodeCache.get(src)
  if (cached) {
    log.debug('Using cached decoded audio (fallback)', { itemId })
    return sliceDecodedAudioRange(cached, itemId, startTime, endTime)
  }

  // decodeAudioData is a Web Audio API — absent inside workers. Surface a
  // structured fallback error so the render pipeline moves this export to
  // the main thread instead of silently exporting a muted segment.
  if (typeof OfflineAudioContext === 'undefined') {
    throw new Error('WORKER_REQUIRES_MAIN_THREAD:audio-decode-fallback')
  }

  log.debug('Decoding audio with Web Audio API fallback', { itemId, src: src.substring(0, 50) })

  const response = await fetch(src)
  if (!response.ok) {
    throw new Error(`Failed to fetch audio: ${response.status}`)
  }

  const arrayBuffer = await response.arrayBuffer()

  const offlineContext = new OfflineAudioContext(2, 1, 48000)
  const audioBuffer = await offlineContext.decodeAudioData(arrayBuffer)

  const samples: Float32Array[] = []
  for (let i = 0; i < audioBuffer.numberOfChannels; i++) {
    samples.push(audioBuffer.getChannelData(i))
  }

  const result: DecodedAudio = {
    itemId,
    sampleRate: audioBuffer.sampleRate,
    channels: audioBuffer.numberOfChannels,
    samples,
    duration: audioBuffer.duration,
  }

  // Cache the result
  audioDecodeCache.set(src, result)

  log.debug('Decoded audio (fallback)', {
    itemId,
    sampleRate: audioBuffer.sampleRate,
    channels: audioBuffer.numberOfChannels,
    duration: audioBuffer.duration,
    samples: samples[0]?.length,
  })

  return sliceDecodedAudioRange(result, itemId, startTime, endTime)
}

/**
 * Convert dB to linear gain.
 */
function dbToGain(db: number): number {
  return Math.pow(10, db / 20)
}

// ---------------------------------------------------------------------------
// Sidechain ducking: while a duck-source item is audible, other audio is
// attenuated by its `duckOthersDb` with attack/release ramps. Applied as a
// third per-segment gain layer (after volume/fades, before the additive mix),
// so the source itself and out-of-scope tracks stay untouched.
// ---------------------------------------------------------------------------

const DUCKING_DEFAULT_ATTACK_SEC = 0.08
const DUCKING_DEFAULT_RELEASE_SEC = 0.25

export interface DuckingSource {
  itemId: string
  trackId: string
  /** Audible span on the timeline, in project frames. */
  startFrame: number
  endFrame: number
  /** Attenuation while audible, dB (negative). */
  duckDb: number
  attackFrames: number
  releaseFrames: number
  /** Restrict ducking to these tracks (default: every other track). */
  targetTrackIds?: string[]
}

type DuckSourceCandidate = CompositionInputProps['tracks'][number]['items'][number] & {
  audioDucking?: AudioDuckingSettings
  embeddedAudioMuted?: boolean
}

/** Map one item to a duck source, or null when it cannot duck anything. */
function duckingSourceFromItem(
  item: DuckSourceCandidate,
  trackId: string,
  fps: number,
  /** Parent-timeline window, for items reached through a pre-composition. */
  window?: { startFrame: number; endFrame: number },
): DuckingSource | null {
  const ducking = item.audioDucking
  if (!ducking || !(ducking.duckOthersDb < 0)) return null
  const carriesAudio = item.type === 'audio' || (item.type === 'video' && !item.embeddedAudioMuted)
  if (!carriesAudio) return null
  return {
    itemId: item.id,
    trackId,
    startFrame: window?.startFrame ?? item.from,
    endFrame: window?.endFrame ?? item.from + item.durationInFrames,
    duckDb: ducking.duckOthersDb,
    attackFrames: (ducking.attackSec ?? DUCKING_DEFAULT_ATTACK_SEC) * fps,
    releaseFrames: (ducking.releaseSec ?? DUCKING_DEFAULT_RELEASE_SEC) * fps,
    ...(ducking.targetTrackIds ? { targetTrackIds: ducking.targetTrackIds } : {}),
  }
}

/**
 * Collect duck sources nested inside a pre-composition, in PARENT timeline frames.
 *
 * The mix expands nested composition audio, so a source living one level down is
 * audible; scanning only the root tracks would leave it audible but not ducking,
 * making the same arrangement sound different at root level and inside a
 * pre-comp. The window mapping is shared with `appendCompositionAudioSegments`
 * so the two cannot disagree about where a nested item sits.
 */
function collectNestedDuckingSources(
  compositionItem: CompositionItem | (AudioItem & { compositionId: string }),
  rootTrackId: string,
  fps: number,
  visited: ReadonlySet<string>,
): DuckingSource[] {
  if (visited.has(compositionItem.compositionId)) return []
  const subComp = useCompositionsStore.getState().getComposition(compositionItem.compositionId)
  if (!subComp) return []

  const wrapper = resolveCompositionWrapper(compositionItem, fps)
  const nestedVisited = new Set(visited)
  nestedVisited.add(compositionItem.compositionId)

  const sources: DuckingSource[] = []
  for (const subItem of subComp.items) {
    const subTrack = subComp.tracks.find((candidate) => candidate.id === subItem.trackId)
    // Only `muted` silences a nested track — the expansion above ignores `visible`,
    // so excluding hidden tracks here would leave audible sources not ducking.
    if (subTrack?.muted === true) continue

    const window = mapNestedItemWindow(subItem, wrapper, fps)
    if (!window) continue

    if (subItem.type === 'composition' || isCompositionAudioItem(subItem)) {
      // A composition-backed AudioItem can carry `audioDucking` of its own, and it
      // is audible in its own right — collect it as well as descending.
      const wrapperSource = duckingSourceFromItem(
        subItem as DuckSourceCandidate,
        rootTrackId,
        fps,
        { startFrame: window.effectiveStart, endFrame: window.effectiveEnd },
      )
      if (wrapperSource) sources.push(wrapperSource)
      sources.push(
        ...collectNestedDuckingSources(
          buildNestedWrapper(subItem, window, wrapper),
          rootTrackId,
          fps,
          nestedVisited,
        ),
      )
      continue
    }

    // Nested sources belong to the ROOT track: that is the track their audio is
    // mixed onto, so it is also what "never duck yourself" must compare against.
    const source = duckingSourceFromItem(subItem as DuckSourceCandidate, rootTrackId, fps, {
      startFrame: window.effectiveStart,
      endFrame: window.effectiveEnd,
    })
    if (source) sources.push(source)
  }
  return sources
}

/** Collect duck-source windows from composition items carrying `audioDucking`. */
export function collectDuckingSources(
  composition: CompositionInputProps,
  fps: number,
): DuckingSource[] {
  return composition.tracks
    .filter((track) => track.visible !== false && track.muted !== true)
    .flatMap((track) =>
      (track.items ?? []).flatMap((item) => {
        // The item's own `audioDucking` counts even when it is a composition
        // wrapper — it is audible in its own right — so collect it either way
        // and additionally descend when it wraps a pre-comp.
        const own = duckingSourceFromItem(item as DuckSourceCandidate, track.id, fps)
        const sources = own ? [own] : []
        if (item.type === 'composition' || isCompositionAudioItem(item)) {
          sources.push(
            ...collectNestedDuckingSources(
              item as CompositionItem | (AudioItem & { compositionId: string }),
              track.id,
              fps,
              new Set<string>(),
            ),
          )
        }
        return sources
      }),
    )
}

/** Piecewise duck gain of one source at a timeline frame, in dB (0 = no duck). */
function duckingSourceGainDb(frame: number, source: DuckingSource): number {
  if (frame < source.startFrame || frame > source.endFrame + source.releaseFrames) return 0
  if (frame < source.startFrame + source.attackFrames) {
    const progress = (frame - source.startFrame) / source.attackFrames
    return source.duckDb * progress
  }
  if (frame <= source.endFrame) return source.duckDb
  const progress = (frame - source.endFrame) / source.releaseFrames
  return source.duckDb * (1 - progress)
}

/**
 * Multiply the ducking envelope into one channel of a target segment.
 * `segmentStartFrame` is the absolute timeline frame of `samples[0]` (same
 * mapping as `applyAnimatedVolume`). The source item never ducks itself;
 * overlapping sources take the deepest (minimum dB) gain.
 */
export function applyDucking(
  samples: Float32Array,
  sources: readonly DuckingSource[],
  segment: { itemId: string; trackId: string },
  segmentStartFrame: number,
  fps: number,
  sampleRate: number,
): Float32Array {
  const spanFrames = (samples.length / sampleRate) * fps
  const applicable = sources.filter(
    (source) =>
      source.itemId !== segment.itemId &&
      (!source.targetTrackIds || source.targetTrackIds.includes(segment.trackId)) &&
      source.startFrame < segmentStartFrame + spanFrames &&
      source.endFrame + source.releaseFrames > segmentStartFrame,
  )
  if (applicable.length === 0) return samples

  const output = new Float32Array(samples.length)
  for (let i = 0; i < samples.length; i++) {
    const frame = segmentStartFrame + (i / sampleRate) * fps
    let db = 0
    for (const source of applicable) {
      const sourceDb = duckingSourceGainDb(frame, source)
      if (sourceDb < db) db = sourceDb
    }
    output[i] = db === 0 ? samples[i]! : samples[i]! * dbToGain(db)
  }
  return output
}

/**
 * Apply volume (in dB) to audio samples.
 */
function applyVolume(samples: Float32Array, volumeDb: number): Float32Array {
  const gain = dbToGain(volumeDb)
  const output = new Float32Array(samples.length)

  for (let i = 0; i < samples.length; i++) {
    output[i] = samples[i]! * gain
  }

  return output
}

/**
 * Apply animated volume envelope from keyframes to audio samples.
 * Interpolates dB value per-frame and applies per-sample gain.
 *
 * @param samples - Audio samples for one channel
 * @param volumeKeyframes - Volume keyframes (frame-relative to item start)
 * @param staticVolumeDb - Static volume dB fallback
 * @param segmentStartFrame - Timeline frame where this segment starts
 * @param itemFrom - Timeline frame where the original item starts
 * @param fps - Frames per second
 * @param sampleRate - Audio sample rate
 */
function applyAnimatedVolume(
  samples: Float32Array,
  volumeKeyframes: VolumeKeyframe[],
  staticVolumeDb: number,
  segmentStartFrame: number,
  itemFrom: number,
  fps: number,
  sampleRate: number,
): Float32Array {
  const output = new Float32Array(samples.length)

  for (let i = 0; i < samples.length; i++) {
    // Convert sample index to timeline frame
    const timelineFrame = segmentStartFrame + (i / sampleRate) * fps
    // Convert to item-relative frame for keyframe interpolation
    const relativeFrame = timelineFrame - itemFrom
    const db = interpolatePropertyValue(volumeKeyframes, relativeFrame, staticVolumeDb)
    const gain = dbToGain(db)
    output[i] = samples[i]! * gain
  }

  return output
}

/**
 * Apply fade in/out to audio samples.
 *
 * @param samples - Audio samples
 * @param fadeInSamples - Number of samples for fade in
 * @param fadeOutSamples - Number of samples for fade out
 * @param useEqualPower - Use equal-power (sin/cos) fades for smoother crossfades
 */
function applyFades(
  samples: Float32Array,
  fadeInSamples: number,
  fadeOutSamples: number,
  useEqualPower: boolean = false,
  fadeInCurve: number = 0,
  fadeOutCurve: number = 0,
  fadeInCurveX: number = 0.52,
  fadeOutCurveX: number = 0.52,
  contentStartOffsetSamples: number = 0,
  contentEndOffsetSamples: number = 0,
  fadeInDelaySamples: number = 0,
  fadeOutLeadSamples: number = 0,
): Float32Array {
  const output = new Float32Array(samples.length)
  output.set(samples)
  const contentStart = Math.max(
    0,
    Math.min(contentStartOffsetSamples + Math.max(0, fadeInDelaySamples), output.length),
  )
  const contentEnd = Math.max(
    contentStart,
    output.length - Math.max(0, contentEndOffsetSamples + Math.max(0, fadeOutLeadSamples)),
  )
  const contentLength = Math.max(0, contentEnd - contentStart)

  // Apply fade in
  if (fadeInSamples > 0) {
    for (let i = 0; i < contentStart && i < output.length; i++) {
      output[i] = 0
    }
    for (let i = 0; i < fadeInSamples && i < contentLength; i++) {
      const sampleIndex = contentStart + i
      const progress = i / fadeInSamples
      const gain = useEqualPower
        ? Math.sin((progress * Math.PI) / 2)
        : evaluateAudioFadeInCurve(progress, fadeInCurve, fadeInCurveX)
      output[sampleIndex] = output[sampleIndex]! * gain
    }
  }

  // Apply fade out
  if (fadeOutSamples > 0) {
    const fadeOutStart = Math.max(contentStart, contentEnd - fadeOutSamples)
    for (let i = 0; i < fadeOutSamples; i++) {
      const sampleIndex = fadeOutStart + i
      if (sampleIndex < contentStart || sampleIndex >= contentEnd) continue

      const progress = i / fadeOutSamples
      const gain = useEqualPower
        ? Math.cos((progress * Math.PI) / 2)
        : evaluateAudioFadeOutCurve(progress, fadeOutCurve, fadeOutCurveX)
      output[sampleIndex] = output[sampleIndex]! * gain
    }
    for (let i = contentEnd; i < output.length; i++) {
      output[i] = 0
    }
  }

  return output
}

function applyClipFadeSpans(
  samples: Float32Array,
  fadeSpans: AudioClipFadeSpan[] | undefined,
  sampleRate: number,
  fps: number,
): Float32Array {
  if (!fadeSpans || fadeSpans.length === 0) return samples

  const output = new Float32Array(samples.length)
  output.set(samples)

  for (const span of fadeSpans) {
    const spanStart = Math.max(0, Math.floor((span.startFrame / fps) * sampleRate))
    const spanEnd = Math.min(
      output.length,
      Math.floor(((span.startFrame + span.durationInFrames) / fps) * sampleRate),
    )
    const spanLength = Math.max(0, spanEnd - spanStart)
    if (spanLength === 0) continue

    const fadeInSamples = Math.max(
      0,
      Math.min(spanLength, Math.floor(((span.fadeInFrames ?? 0) / fps) * sampleRate)),
    )
    const fadeOutSamples = Math.max(
      0,
      Math.min(spanLength, Math.floor(((span.fadeOutFrames ?? 0) / fps) * sampleRate)),
    )

    for (let i = 0; i < fadeInSamples; i++) {
      const progress = i / Math.max(1, fadeInSamples)
      const gain = evaluateAudioFadeInCurve(progress, span.fadeInCurve, span.fadeInCurveX)
      output[spanStart + i] = output[spanStart + i]! * gain
    }

    const fadeOutStart = spanEnd - fadeOutSamples
    for (let i = 0; i < fadeOutSamples; i++) {
      const sampleIndex = fadeOutStart + i
      if (sampleIndex < spanStart || sampleIndex >= spanEnd) continue
      const progress = i / Math.max(1, fadeOutSamples)
      const gain = evaluateAudioFadeOutCurve(progress, span.fadeOutCurve, span.fadeOutCurveX)
      output[sampleIndex] = output[sampleIndex]! * gain
    }
  }

  return output
}

/**
 * Apply speed change to audio with pitch preservation using the local time-stretch processor.
 * Processes all channels together through a single processor instance so that
 * WSOLA overlap windows are consistent across channels (prevents phase drift
 * between L/R that causes a hollow sound).
 *
 * @param channels - Input audio channels (mono or stereo)
 * @param speed - Playback rate (1.0 = normal, 2.0 = double speed, 0.5 = half speed)
 * @param sampleRate - Sample rate for the audio
 * @returns Processed channels with the same channel count
 */
async function applySpeedAndPitch(
  channels: Float32Array[],
  speed: number,
  pitchShiftSemitones: number,
  sampleRate: number,
): Promise<Float32Array[]> {
  const requiresTimeStretch =
    Math.abs(speed - 1) > 0.0001 || isAudioPitchShiftActive(pitchShiftSemitones)
  if (!requiresTimeStretch) return channels
  if (channels.length === 0 || channels[0]!.length === 0) return channels

  const numChannels = channels.length
  const samplesPerChannel = channels[0]!.length

  log.debug('Applying speed/pitch change with time-stretch processor', {
    speed,
    pitchShiftSemitones,
    sampleRate,
    numChannels,
  })

  try {
    const timeStretch = await import('@editor/infrastructure/audio/time-stretch')
    const st = new timeStretch.TimeStretchProcessor()

    st.tempo = speed
    st.pitch = getAudioPitchRatioFromSemitones(pitchShiftSemitones)
    st.rate = 1.0

    // The processor consumes interleaved stereo. Interleave all channels
    // (for mono, duplicate to stereo so it gets valid input).
    const stereoInput = new Float32Array(samplesPerChannel * 2)
    const left = channels[0]!
    const right = numChannels >= 2 ? channels[1]! : left
    for (let i = 0; i < samplesPerChannel; i++) {
      stereoInput[i * 2] = left[i]!
      stereoInput[i * 2 + 1] = right[i]!
    }

    let inputOffset = 0
    const source = {
      extract: (target: Float32Array, numFrames: number): number => {
        const samplesToRead = Math.min(numFrames * 2, stereoInput.length - inputOffset)
        if (samplesToRead <= 0) return 0

        for (let i = 0; i < samplesToRead; i++) {
          target[i] = stereoInput[inputOffset + i]!
        }
        inputOffset += samplesToRead
        return samplesToRead / 2
      },
    }

    const filter = new timeStretch.TimeStretchFilter(source, st)

    const expectedOutputLength = Math.floor(samplesPerChannel / speed)
    const stereoOutput = new Float32Array(expectedOutputLength * 2)

    let outputOffset = 0
    const chunkSize = 4096
    const chunk = new Float32Array(chunkSize * 2)

    while (outputOffset < stereoOutput.length) {
      const framesExtracted = filter.extract(chunk, chunkSize)
      if (framesExtracted === 0) break

      const samplesToWrite = Math.min(framesExtracted * 2, stereoOutput.length - outputOffset)
      for (let i = 0; i < samplesToWrite; i++) {
        stereoOutput[outputOffset + i] = chunk[i]!
      }
      outputOffset += framesExtracted * 2
    }

    // De-interleave back to separate channels
    const actualOutputLength = Math.floor(outputOffset / 2)
    const outputChannels: Float32Array[] = []

    // Always extract both L and R from the interleaved output
    const outLeft = new Float32Array(actualOutputLength)
    const outRight = new Float32Array(actualOutputLength)
    for (let i = 0; i < actualOutputLength; i++) {
      outLeft[i] = stereoOutput[i * 2]!
      outRight[i] = stereoOutput[i * 2 + 1]!
    }

    if (numChannels >= 2) {
      outputChannels.push(outLeft, outRight)
      // Pass through any additional channels beyond stereo (rare)
      for (let c = 2; c < numChannels; c++) {
        outputChannels.push(outLeft) // duplicate left for extra channels
      }
    } else {
      // Mono source: return left channel only
      outputChannels.push(outLeft)
    }

    log.debug('Time-stretch processing complete', {
      inputLength: samplesPerChannel,
      outputLength: actualOutputLength,
      expectedLength: expectedOutputLength,
      speed,
      numChannels,
    })

    return outputChannels
  } catch (error) {
    log.warn('Time-stretch processing failed, falling back to simple resampling', {
      error,
      speed,
      pitchShiftSemitones,
    })

    if (isAudioPitchShiftActive(pitchShiftSemitones)) {
      log.warn('Independent export pitch shift was skipped because time-stretch processing failed')
    }

    // Fallback: simple resampling per-channel (speed only, pitch shift omitted)
    const outputLength = Math.floor(samplesPerChannel / speed)
    return channels.map((samples) => {
      const output = new Float32Array(outputLength)
      for (let i = 0; i < outputLength; i++) {
        const sourceIndex = i * speed
        const index0 = Math.floor(sourceIndex)
        const index1 = Math.min(index0 + 1, samples.length - 1)
        const fraction = sourceIndex - index0
        output[i] = samples[index0]! * (1 - fraction) + samples[index1]! * fraction
      }
      return output
    })
  }
}

/**
 * Resample audio to the target sample rate with the pure-JS polyphase
 * sinc resampler (windowed-sinc, ~90 dB stopband). Works in workers where
 * OfflineAudioContext does not exist.
 */
function resample(
  samples: Float32Array,
  sourceSampleRate: number,
  targetSampleRate: number,
): Float32Array {
  if (sourceSampleRate === targetSampleRate) return samples
  return resamplePcm(samples, sourceSampleRate, targetSampleRate)
}

/**
 * Downmix a multi-channel decoded source to the target channel count using
 * the ITU-R BS.775 / Dolby Pro Logic II coefficients. Standard channel
 * order from mediabunny (and the FFmpeg-backed AC-3/E-AC-3 decoder) is
 * `L, R, C, LFE, Ls, Rs` for 5.1 and `L, R, C, LFE, Ls, Rs, Lr, Rr` for
 * 7.1.
 *
 * Without this, a 5.1 EAC3 source going to a stereo export was dropping
 * the center channel — meaning all the dialogue was missing from the
 * exported audio while music + effects on L/R sounded thin.
 *
 * If the source already has the same channel count as the output, samples
 * are returned untouched. If the source has fewer channels (e.g. mono into
 * stereo) the lone channel is duplicated.
 */
export function downmixToOutputChannels(
  source: readonly Float32Array[],
  outputChannels: number,
): Float32Array[] {
  if (source.length === 0) return []
  if (source.length === outputChannels) return source.map((c) => c)

  // Mono source going into multi-channel output: duplicate.
  if (source.length === 1) {
    const mono = source[0]!
    const out: Float32Array[] = []
    for (let c = 0; c < outputChannels; c++) out.push(mono)
    return out
  }

  // Stereo output is the common case for export — handle it directly.
  if (outputChannels === 2) {
    return downmixToStereo(source)
  }

  // Mono output: average all source channels with center/LFE attenuation.
  if (outputChannels === 1) {
    const stereo = downmixToStereo(source)
    const length = stereo[0]!.length
    const merged = new Float32Array(length)
    const left = stereo[0]!
    const right = stereo[1]!
    for (let i = 0; i < length; i++) {
      merged[i] = (left[i]! + right[i]!) * 0.5
    }
    return [merged]
  }

  // Unrecognised target — slice to the requested channels and pad with
  // silence; the prior behavior. Real broadcast targets stop at stereo
  // here, so this branch is just defensive.
  const out: Float32Array[] = []
  for (let c = 0; c < outputChannels; c++) {
    out.push(source[c] ?? new Float32Array(source[0]!.length))
  }
  return out
}

const SQRT_HALF = Math.SQRT1_2 // ≈ 0.7071068

function downmixToStereo(source: readonly Float32Array[]): Float32Array[] {
  // Mediabunny 5.1 / 7.1 channel order (FFmpeg standard):
  //   0 L, 1 R, 2 C, 3 LFE, 4 Ls, 5 Rs, [6 Lr, 7 Rr]
  // Stereo downmix coefficients (ITU-R BS.775 / Dolby):
  //   Lo = L + sqrt(0.5)·C + sqrt(0.5)·Ls (+ surround back if 7.1)
  //   Ro = R + sqrt(0.5)·C + sqrt(0.5)·Rs
  // LFE is dropped — re-mixing it into stereo without bandlimiting causes
  // muddy bass. Matches Dolby's recommended Lo/Ro downmix.
  const [L, R, C, , Ls, Rs, Lr, Rr] = source
  const length = L?.length ?? R?.length ?? 0
  if (length === 0) return [new Float32Array(0), new Float32Array(0)]

  const left = new Float32Array(length)
  const right = new Float32Array(length)
  for (let i = 0; i < length; i++) {
    let lo = L?.[i] ?? 0
    let ro = R?.[i] ?? 0
    const c = C?.[i]
    if (c !== undefined) {
      lo += SQRT_HALF * c
      ro += SQRT_HALF * c
    }
    const ls = Ls?.[i]
    if (ls !== undefined) lo += SQRT_HALF * ls
    const rs = Rs?.[i]
    if (rs !== undefined) ro += SQRT_HALF * rs
    const lr = Lr?.[i]
    if (lr !== undefined) lo += SQRT_HALF * lr
    const rr = Rr?.[i]
    if (rr !== undefined) ro += SQRT_HALF * rr
    left[i] = lo
    right[i] = ro
  }
  return [left, right]
}

function reverseAudioChannels(channels: Float32Array[]): Float32Array[] {
  return channels.map((samples) => {
    const reversed = new Float32Array(samples.length)
    for (let i = 0; i < samples.length; i++) {
      reversed[i] = samples[samples.length - 1 - i] ?? 0
    }
    return reversed
  })
}

/**
 * Mix multiple audio tracks together.
 *
 * @param segments - Processed audio segments with timing
 * @param config - Audio processing configuration
 * @returns Mixed stereo audio samples
 */
function createAudioMixBuffer(config: AudioProcessingConfig): Float32Array[] {
  const { sampleRate, channels, fps, totalFrames } = config
  const totalSamples = Math.ceil((totalFrames / fps) * sampleRate)
  const output: Float32Array[] = []
  for (let c = 0; c < channels; c++) {
    output.push(new Float32Array(totalSamples))
  }
  return output
}

function mixAudioSegmentInto(
  output: Float32Array[],
  segment: { samples: Float32Array[]; startSample: number },
  config: AudioProcessingConfig,
): void {
  const { channels } = config
  const totalSamples = output[0]?.length ?? 0
  const downmixed = downmixToOutputChannels(segment.samples, channels)

  for (let c = 0; c < channels; c++) {
    const channelSamples = downmixed[c]!
    const outputChannel = output[c]!
    const sourceStart = Math.max(0, -segment.startSample)
    const sourceEnd = Math.min(channelSamples.length, totalSamples - segment.startSample)
    for (let i = sourceStart; i < sourceEnd; i++) {
      const outputIndex = segment.startSample + i
      outputChannel[outputIndex] = outputChannel[outputIndex]! + channelSamples[i]!
    }
  }
}

function softClipAudioMix(output: Float32Array[]): void {
  // Soft-clip to prevent harsh digital clipping while preserving overall loudness.
  // This matches browser preview behavior where audio peaks are naturally saturated
  // rather than the entire mix being reduced in volume.
  let clippedSamples = 0
  for (const channel of output) {
    for (let i = 0; i < channel.length; i++) {
      const sample = channel[i]
      if (sample !== undefined && Math.abs(sample) > 1.0) {
        // tanh soft limiter: smoothly compresses peaks above 1.0
        channel[i] = Math.tanh(sample)
        clippedSamples++
      }
    }
  }
  if (clippedSamples > 0) {
    log.debug('Soft-clipped audio peaks', { clippedSamples })
  }
}

/**
 * Pre-resolve sub-composition media URLs so extractAudioSegments can access them.
 * blobUrlManager.get() is synchronous but may not have URLs for sub-comp items
 * until they're acquired via resolveMediaUrl (async OPFS read).
 */
async function resolveSubCompMediaUrls(composition: CompositionInputProps): Promise<void> {
  const tracks = composition.tracks ?? []
  const urlResolutions: Promise<void>[] = []
  const compositionById = useCompositionsStore.getState().compositionById
  const reachableCompositionIds = collectReachableCompositionIdsFromTracks(tracks, compositionById)
  for (const compositionId of reachableCompositionIds) {
    const subComp = compositionById[compositionId]
    if (!subComp) continue
    for (const subItem of subComp.items) {
      if (subItem.type !== 'video' && subItem.type !== 'audio') continue
      if (subItem.mediaId && !blobUrlManager.get(subItem.mediaId)) {
        urlResolutions.push(resolveMediaUrl(subItem.mediaId).then(() => {}))
      }
    }
  }
  if (urlResolutions.length > 0) {
    log.debug('Pre-resolving sub-comp audio URLs', { count: urlResolutions.length })
    await Promise.all(urlResolutions)
  }
}

const STREAMING_AUDIO_CHUNK_SECONDS = 30

/**
 * Windowed export audio supports every segment shape: DSP state (EQ filter
 * memory, SoundTouch internals, resampler history) is carried per segment
 * across windows by StreamingSegmentProcessor, so a single DSP'd clip no
 * longer forces whole-timeline processing.
 */
export function supportsWindowedAudioProcessing(composition: CompositionInputProps): boolean {
  const segments = extractAudioSegments(composition, composition.fps).filter(
    (segment) => !segment.muted,
  )
  return segments.length > 0
}

function hasPacketCopyTimingChanges(segment: AudioSegment, durationInFrames: number): boolean {
  return (
    segment.startFrame !== 0 ||
    segment.durationFrames !== durationInFrames ||
    segment.sourceStartFrame !== 0 ||
    Math.abs(segment.speed - 1) > 0.0001 ||
    segment.isReversed
  )
}

function hasPacketCopyGainChanges(segment: AudioSegment): boolean {
  return (
    segment.volume !== 0 ||
    Boolean(segment.volumeKeyframes?.length) ||
    segment.fadeInFrames !== 0 ||
    segment.fadeOutFrames !== 0 ||
    (segment.crossfadeFadeInFrames ?? 0) !== 0 ||
    (segment.crossfadeFadeOutFrames ?? 0) !== 0
  )
}

function hasPacketCopyContentOffsets(segment: AudioSegment): boolean {
  return (
    (segment.contentStartOffsetFrames ?? 0) !== 0 ||
    (segment.contentEndOffsetFrames ?? 0) !== 0 ||
    (segment.fadeInDelayFrames ?? 0) !== 0 ||
    (segment.fadeOutLeadFrames ?? 0) !== 0
  )
}

function hasPacketCopyFadeSpans(segment: AudioSegment, durationInFrames: number): boolean {
  if ((segment.clipFadeSpans?.length ?? 0) > 1) return true
  return Boolean(
    segment.clipFadeSpans?.some(
      (span) =>
        span.startFrame !== 0 ||
        span.durationInFrames !== durationInFrames ||
        span.fadeInFrames !== 0 ||
        span.fadeOutFrames !== 0,
    ),
  )
}

function hasPacketCopyEffects(segment: AudioSegment): boolean {
  return (
    isAudioPitchShiftActive(segment.pitchShiftSemitones) ||
    segment.audioEqStages.some(isAudioEqStageActive) ||
    segment.audioFx?.some((effect) => effect.enabled) === true
  )
}

/**
 * Return a packet-copy plan only when the composition's audio is one continuous,
 * untouched source starting at timestamp zero. Any gain, fade, DSP, speed,
 * reverse, trim-at-start, overlap, or master-bus change requires PCM processing.
 */
export function getAudioPacketPassthroughPlan(
  composition: CompositionInputProps,
): AudioPacketPassthroughPlan | null {
  const durationInFrames = composition.durationInFrames ?? 0
  if (durationInFrames <= 0 || composition.fps <= 0 || (composition.masterBusDb ?? 0) !== 0) {
    return null
  }
  if (composition.tracks.some((track) => track.audioFx?.some((effect) => effect.enabled))) return null

  // Ducking needs no guard here: passthrough requires exactly ONE audible
  // segment, and a lone duck source has nothing to duck.
  const segments = extractAudioSegments(composition, composition.fps).filter(
    (segment) => !segment.muted,
  )
  if (segments.length !== 1) return null

  const segment = segments[0]!
  const hasProcessing = [
    hasPacketCopyTimingChanges(segment, durationInFrames),
    hasPacketCopyGainChanges(segment),
    hasPacketCopyContentOffsets(segment),
    hasPacketCopyFadeSpans(segment, durationInFrames),
    hasPacketCopyEffects(segment),
  ].some(Boolean)

  if (hasProcessing) return null
  return { src: segment.src, durationSeconds: durationInFrames / composition.fps }
}

type FramesToSamples = (frames: number | undefined) => number

function applyClipFadeSpanToWindow(
  output: Float32Array,
  span: AudioClipFadeSpan,
  segmentOffsetSamples: number,
  toSamples: FramesToSamples,
): void {
  const spanStart = toSamples(span.startFrame)
  const spanEnd = spanStart + toSamples(span.durationInFrames)
  const fadeInSamples = Math.min(spanEnd - spanStart, toSamples(span.fadeInFrames))
  const fadeOutSamples = Math.min(spanEnd - spanStart, toSamples(span.fadeOutFrames))

  for (let i = 0; i < output.length; i++) {
    const globalIndex = segmentOffsetSamples + i
    if (globalIndex < spanStart || globalIndex >= spanEnd) continue
    if (fadeInSamples > 0 && globalIndex < spanStart + fadeInSamples) {
      const progress = (globalIndex - spanStart) / Math.max(1, fadeInSamples)
      output[i] =
        output[i]! * evaluateAudioFadeInCurve(progress, span.fadeInCurve, span.fadeInCurveX)
    }
    const fadeOutStart = spanEnd - fadeOutSamples
    if (fadeOutSamples > 0 && globalIndex >= fadeOutStart) {
      const progress = (globalIndex - fadeOutStart) / Math.max(1, fadeOutSamples)
      output[i] =
        output[i]! * evaluateAudioFadeOutCurve(progress, span.fadeOutCurve, span.fadeOutCurveX)
    }
  }
}

function applyClipFadeSpansToWindow(
  output: Float32Array,
  segment: AudioSegment,
  segmentOffsetSamples: number,
  toSamples: FramesToSamples,
): void {
  for (const span of segment.clipFadeSpans ?? []) {
    applyClipFadeSpanToWindow(output, span, segmentOffsetSamples, toSamples)
  }
}

function applyBasicFadesToWindow(
  output: Float32Array,
  segment: AudioSegment,
  segmentOffsetSamples: number,
  totalSegmentSamples: number,
  toSamples: FramesToSamples,
): void {
  const contentStart =
    toSamples(segment.contentStartOffsetFrames) + toSamples(segment.fadeInDelayFrames)
  const contentEnd = Math.max(
    contentStart,
    totalSegmentSamples -
      toSamples(segment.contentEndOffsetFrames) -
      toSamples(segment.fadeOutLeadFrames),
  )
  const fadeInSamples = toSamples(segment.fadeInFrames)
  const fadeOutSamples = toSamples(segment.fadeOutFrames)
  const fadeOutStart = Math.max(contentStart, contentEnd - fadeOutSamples)

  for (let i = 0; i < output.length; i++) {
    const globalIndex = segmentOffsetSamples + i
    if (globalIndex < contentStart || globalIndex >= contentEnd) {
      output[i] = 0
    } else if (fadeInSamples > 0 && globalIndex < contentStart + fadeInSamples) {
      const progress = (globalIndex - contentStart) / fadeInSamples
      output[i] =
        output[i]! * evaluateAudioFadeInCurve(progress, segment.fadeInCurve, segment.fadeInCurveX)
    } else if (fadeOutSamples > 0 && globalIndex >= fadeOutStart) {
      const progress = (globalIndex - fadeOutStart) / fadeOutSamples
      output[i] =
        output[i]! *
        evaluateAudioFadeOutCurve(progress, segment.fadeOutCurve, segment.fadeOutCurveX)
    }
  }
}

function applyCrossfadesToWindow(
  output: Float32Array,
  segment: AudioSegment,
  segmentOffsetSamples: number,
  totalSegmentSamples: number,
  toSamples: FramesToSamples,
): void {
  const crossfadeIn = toSamples(segment.crossfadeFadeInFrames)
  const crossfadeOut = toSamples(segment.crossfadeFadeOutFrames)
  const fadeOutStart = totalSegmentSamples - crossfadeOut
  for (let i = 0; i < output.length; i++) {
    const globalIndex = segmentOffsetSamples + i
    if (crossfadeIn > 0 && globalIndex < crossfadeIn) {
      output[i] = output[i]! * Math.sin(((globalIndex / crossfadeIn) * Math.PI) / 2)
    }
    if (crossfadeOut > 0 && globalIndex >= fadeOutStart) {
      output[i] =
        output[i]! * Math.cos((((globalIndex - fadeOutStart) / crossfadeOut) * Math.PI) / 2)
    }
  }
}

function applyWindowedSegmentFades(
  samples: Float32Array,
  segment: AudioSegment,
  segmentOffsetSamples: number,
  totalSegmentSamples: number,
  sampleRate: number,
  fps: number,
): Float32Array {
  const output = new Float32Array(samples)
  const toSamples: FramesToSamples = (frames) =>
    Math.max(0, Math.floor(((frames ?? 0) / fps) * sampleRate))

  if (segment.clipFadeSpans?.length) {
    applyClipFadeSpansToWindow(output, segment, segmentOffsetSamples, toSamples)
  } else {
    applyBasicFadesToWindow(output, segment, segmentOffsetSamples, totalSegmentSamples, toSamples)
  }
  applyCrossfadesToWindow(output, segment, segmentOffsetSamples, totalSegmentSamples, toSamples)

  return output
}


interface WindowedAudioDecoderPool {
  decode: (segment: AudioSegment, startTime: number, endTime: number) => Promise<DecodedAudio>
  dispose: () => Promise<void>
}

async function createWindowedAudioDecoderPool(): Promise<WindowedAudioDecoderPool> {
  const mb = await import('mediabunny')
  type Session = {
    input: InstanceType<typeof mb.Input>
    sink: InstanceType<typeof mb.AudioSampleSink>
    duration: number
  }
  const sessions = new Map<string, Promise<Session>>()

  const getSession = (segment: AudioSegment): Promise<Session> => {
    const existing = sessions.get(segment.src)
    if (existing) return existing
    const created = (async () => {
      const input = new mb.Input({
        formats: mb.ALL_FORMATS,
        source: createMediabunnyInputSource(mb, segment.src),
      })
      try {
        const track = await input.getPrimaryAudioTrack()
        if (!track) throw new Error('No audio track found')
        return {
          input,
          sink: new mb.AudioSampleSink(track),
          duration: await input.computeDuration(),
        }
      } catch (error) {
        input.dispose()
        throw error
      }
    })()
    sessions.set(segment.src, created)
    return created
  }

  return {
    async decode(segment, startTime, endTime) {
      try {
        const session = await getSession(segment)
        return await decodeAudioRangeFromSink(
          session.sink,
          segment.itemId,
          Math.max(0, startTime),
          Math.min(session.duration, endTime),
        )
      } catch (error) {
        log.warn('Persistent audio decoder failed; retrying with isolated decoder', {
          itemId: segment.itemId,
          error,
        })
        return decodeAudioFromSource(
          segment.src,
          segment.itemId,
          startTime,
          endTime,
          segment.audioCodec,
        )
      }
    },
    async dispose() {
      const settledSessions = await Promise.allSettled(sessions.values())
      for (const result of settledSessions) {
        if (result.status === 'fulfilled') result.value.input.dispose()
      }
      sessions.clear()
    },
  }
}


/**
 * Per-segment runtime carried across output windows. `produced` frames are
 * post-EQ (and post-stretch, post-reverse) audio at the DECODED sample rate,
 * indexed from the segment start — exactly the index the whole-buffer path
 * applies volume/fades/ducking at. `out` frames are the same stream after
 * per-window channel processing and streaming resample, at the 48 kHz output
 * rate, so a window mixes a contiguous span [a, b).
 */
interface WindowedSegmentRuntime {
  /** Learned on first decode (0 until then). */
  decodedRate: number
  decodedChannels: number
  /** Stretch + EQ state for this segment (passthrough uses only its EQ). */
  dsp: StreamingSegmentProcessor | null
  hasEq: boolean
  /** Produced-frame queues (decoded rate), one per decoded channel. */
  producedQueue: PlanarFrameQueue[]
  /** Absolute produced index one past the last produced frame. */
  producedEnd: number
  /** Absolute produced index already channel-processed and resampled. */
  producedConsumed: number
  /** Nominal produced length in decoded-rate frames (0 until first decode). */
  producedTotal: number
  nominalProducedTotal: number
  /** Seconds of source the segment consumes, precomputed. */
  sourceTotalSeconds: number
  productionDone: boolean
  /** Output-rate queues after channel processing + resample. */
  outQueue: PlanarFrameQueue[]
  /** Absolute output-rate index of outQueue's head (segment-relative). */
  outHead: number
  resamplers: PolyphaseResampler[] | null
  resamplersFinished: boolean
}

/**
 * Apply volume, fades and ducking to a produced chunk at decoded rate.
 * `producedOffset` is the absolute produced index of chunk[0] — the same
 * anchor the whole-buffer path uses, so envelopes land identically
 * regardless of chunking.
 */
function processProducedChunk(
  samples: Float32Array,
  segment: AudioSegment,
  producedOffset: number,
  producedTotal: number,
  decodedRate: number,
  fps: number,
  duckingSources: readonly DuckingSource[],
): Float32Array {
  let processed = samples
  const timelineFrame = segment.startFrame + (producedOffset / decodedRate) * fps
  if (segment.volumeKeyframes?.length) {
    processed = applyAnimatedVolume(
      processed,
      segment.volumeKeyframes,
      segment.volume,
      timelineFrame,
      segment.itemFrom,
      fps,
      decodedRate,
    )
  } else if (segment.volume !== 0) {
    processed = applyVolume(processed, segment.volume)
  }
  const nominalFrames = Math.min(processed.length, Math.max(0, producedTotal - producedOffset))
  if (nominalFrames > 0) {
    const faded = applyWindowedSegmentFades(
      processed.subarray(0, nominalFrames),
      segment,
      producedOffset,
      producedTotal,
      decodedRate,
      fps,
    )
    const withTail = processed.slice()
    withTail.set(faded)
    processed = withTail
  }
  if (duckingSources.length > 0) {
    processed = applyDucking(
      processed,
      duckingSources,
      segment,
      timelineFrame,
      fps,
      decodedRate,
    )
  }
  return processed
}

function createWindowedSegmentRuntime(
  segment: AudioSegment,
  fps: number,
): WindowedSegmentRuntime {
  return {
    decodedRate: 0,
    decodedChannels: 0,
    dsp: null,
    hasEq: segment.audioEqStages?.some(isAudioEqStageActive) ?? false,
    producedQueue: [],
    producedEnd: 0,
    producedConsumed: 0,
    producedTotal: 0,
    nominalProducedTotal: 0,
    sourceTotalSeconds: segmentSourceSeconds(segment, fps),
    productionDone: false,
    outQueue: [],
    outHead: 0,
    resamplers: null,
    resamplersFinished: false,
  }
}

/** Seconds of source the whole segment consumes (speed-scaled). */
function segmentSourceSeconds(segment: AudioSegment, fps: number): number {
  return (segment.durationFrames / fps) * segment.speed
}

function nominalProducedFrames(segment: AudioSegment, fps: number, decodedRate: number): number {
  return Math.ceil((segment.durationFrames / fps) * decodedRate)
}

interface SegmentSourceSpan {
  startSec: number
  endSec: number
  /** True when the decoded range must be channel-reversed after decode. */
  reverseAfterDecode: boolean
}

/**
 * Map a source-frames range [offset, offset+frames) in PLAYBACK order to the
 * physical source seconds to decode. For reversed segments the range is
 * mirrored around the consumed source span and the decoded chunk is reversed
 * afterwards, exactly like reverseAudioChannels on the whole range.
 */
function mapSourceSpan(
  segment: AudioSegment,
  fps: number,
  decodedRate: number,
  offsetFrames: number,
  frames: number,
): SegmentSourceSpan {
  const srcStartSec = segment.sourceStartFrame / segment.sourceFps
  const srcEndSec = srcStartSec + segmentSourceSeconds(segment, fps)
  const startSec = offsetFrames / decodedRate
  const endSec = (offsetFrames + frames) / decodedRate
  if (!segment.isReversed) {
    return {
      startSec: srcStartSec + startSec,
      endSec: srcStartSec + endSec,
      reverseAfterDecode: false,
    }
  }
  return {
    startSec: srcEndSec - endSec,
    endSec: srcEndSec - startSec,
    reverseAfterDecode: true,
  }
}

/**
 * First decode for this segment: learn rate/channels, allocate queues and
 * resamplers, create the per-segment DSP (stretch + stateful EQ).
 */
function initWindowedRuntime(
  runtime: WindowedSegmentRuntime,
  segment: AudioSegment,
  fps: number,
  sampleRate: number,
  samples: Float32Array[],
  decodedRate: number,
): void {
  if (runtime.decodedRate !== 0) return
  runtime.decodedRate = decodedRate
  runtime.decodedChannels = samples.length
  for (let c = 0; c < samples.length; c++) {
    runtime.producedQueue.push(new PlanarFrameQueue())
    runtime.outQueue.push(new PlanarFrameQueue())
  }
  if (decodedRate !== sampleRate) {
    runtime.resamplers = samples.map(
      () => new PolyphaseResampler(decodedRate, sampleRate),
    )
  }
  runtime.dsp = new StreamingSegmentProcessor({
    sampleRate: decodedRate,
    channels: samples.length,
    sourceFrames: Math.round(runtime.sourceTotalSeconds * decodedRate),
    speed: segment.speed,
    isReversed: segment.isReversed,
    pitchShiftSemitones: segment.pitchShiftSemitones,
    audioEqStages: segment.audioEqStages,
    audioFxStages: createAudioFxStages(segment.audioFx, samples.length, decodedRate),
  })
  runtime.nominalProducedTotal = runtime.dsp.stretchActive
    ? runtime.dsp.expectedOutputFrames
    : nominalProducedFrames(segment, fps, decodedRate)
  runtime.producedTotal = runtime.nominalProducedTotal + runtime.dsp.tailOutputFrames
}
/**
 * Decode `frames` source frames at playback-order offset `offsetFrames`
 * (decoded-rate frames along the consumed source span). Reversed segments
 * get a mirrored range that is channel-reversed after decode.
 */
async function decodeSourceSpan(
  runtime: WindowedSegmentRuntime,
  segment: AudioSegment,
  fps: number,
  offsetFrames: number,
  frames: number,
  decoderPool: WindowedAudioDecoderPool,
): Promise<{ samples: Float32Array[]; sampleRate: number; requestedFrames: number } | null> {
  if (frames <= 0) return null
  // Before the first decode the rate is unknown; estimate at the output
  // rate — the decode returns whatever the media actually carries.
  const rate = runtime.decodedRate || 48_000
  const span = mapSourceSpan(segment, fps, rate, offsetFrames, frames)
  if (span.endSec <= span.startSec) return null
  const decoded = await decoderPool.decode(segment, span.startSec, span.endSec)
  const samples = span.reverseAfterDecode
    ? decoded.samples.map((channel) => {
        const reversed = new Float32Array(channel.length)
        for (let i = 0; i < channel.length; i++) {
          reversed[i] = channel[channel.length - 1 - i] ?? 0
        }
        return reversed
      })
    : decoded.samples
  return {
    samples,
    sampleRate: decoded.sampleRate,
    requestedFrames: Math.round((span.endSec - span.startSec) * decoded.sampleRate),
  }
}

/**
 * Push produced chunks through channel processing + resample into outQueue.
 * Consumes produced frames from producedQueue up to `through` (absolute).
 */
function consumeProducedThrough(
  runtime: WindowedSegmentRuntime,
  segment: AudioSegment,
  through: number,
  fps: number,
  duckingSources: readonly DuckingSource[],
): void {
  while (runtime.producedConsumed < through) {
    const available = runtime.producedEnd - runtime.producedConsumed
    if (available <= 0) break
    const take = Math.min(available, through - runtime.producedConsumed)
    for (let c = 0; c < runtime.producedQueue.length; c++) {
      const queue = runtime.producedQueue[c]!
      const chunk = new Float32Array(take)
      queue.readInto(chunk, 0, runtime.producedConsumed - (runtime.producedEnd - queue.length), take)
      const processed = processProducedChunk(
        chunk,
        segment,
        runtime.producedConsumed,
        runtime.nominalProducedTotal,
        runtime.decodedRate,
        fps,
        duckingSources,
      )
      const resampler = runtime.resamplers?.[c]
      runtime.outQueue[c]!.push(resampler ? resampler.push(processed) : processed)
    }
    runtime.producedConsumed += take
  }
}

/** Drop produced frames that were already channel-processed. */
function dropConsumedProduced(runtime: WindowedSegmentRuntime): void {
  if (runtime.producedConsumed <= 0) return
  const keep = runtime.producedEnd - runtime.producedConsumed
  for (const queue of runtime.producedQueue) {
    queue.drop(Math.max(0, queue.length - keep))
  }
}

/** Frames of input lookahead the resampler needs to emit one output. */
function resamplerLookaheadFrames(runtime: WindowedSegmentRuntime): number {
  return runtime.resamplers?.[0]?.lookaheadFrames ?? 0
}

/**
 * Feed the DSP/produced pipeline until producedEnd reaches `through`
 * (absolute produced index) or the segment's produced stream is done.
 * Sequential segments decode exactly the produced range still needed;
 * stretch/pitch segments feed SoundTouch in ≤30 s source windows and pull
 * its output, with the source FIFO bounded to two windows.
 */
async function produceThrough(
  runtime: WindowedSegmentRuntime,
  segment: AudioSegment,
  through: number,
  fps: number,
  sampleRate: number,
  decoderPool: WindowedAudioDecoderPool,
  signal?: AbortSignal,
): Promise<void> {
  const useStretch =
    Math.abs(segment.speed - 1) > 0.0001 ||
    isAudioPitchShiftActive(segment.pitchShiftSemitones)
  const maxPushFrames = 30 * 48_000
  let stalledIterations = 0

  while (runtime.producedEnd < through && !runtime.productionDone) {
    if (signal?.aborted) throw new DOMException('Audio processing cancelled', 'AbortError')
    if (++stalledIterations > 1024) {
      throw new Error('Windowed audio production stalled')
    }

    if (!useStretch) {
      const nominal =
        runtime.nominalProducedTotal ||
        nominalProducedFrames(segment, fps, runtime.decodedRate || 48_000)
      const need = Math.min(through, nominal) - runtime.producedEnd
      if (need <= 0) {
        runtime.productionDone = true
        break
      }
      const take = Math.min(need, maxPushFrames)
      const decoded = await decodeSourceSpan(
        runtime,
        segment,
        fps,
        runtime.producedEnd,
        take,
        decoderPool,
      )
      if (!decoded || decoded.samples.every((c) => c.length === 0)) {
        runtime.productionDone = true
        break
      }
      initWindowedRuntime(
        runtime,
        segment,
        fps,
        sampleRate,
        decoded.samples,
        decoded.sampleRate,
      )
      const frames = decoded.samples[0]!.length
      const produced = runtime.hasEq || (segment.audioFx?.length ?? 0) > 0
        ? runtime.dsp!.processPassthroughWindow(decoded.samples, frames)
        : decoded.samples
      for (let c = 0; c < produced.length; c++) {
        runtime.producedQueue[c]!.push(produced[c]!)
      }
      runtime.producedEnd += frames
      stalledIterations = 0
      // A short read means the media ran out before the segment's claimed
      // source span — the produced stream ends with this chunk.
      if (frames < decoded.requestedFrames) runtime.productionDone = true
      continue
    }

    // Stretch mode: pull output, then push more source when the pull is dry.
    const dsp = runtime.dsp
    if (dsp) {
      const pullSize = Math.max(4096, through - runtime.producedEnd)
      const pulled = dsp.pullOutput(pullSize)
      if (pulled && pulled[0]!.length > 0) {
        const frames = pulled[0]!.length
        for (let c = 0; c < pulled.length; c++) {
          runtime.producedQueue[c]!.push(pulled[c]!)
        }
        runtime.producedEnd += frames
        stalledIterations = 0
        continue
      }
      if (dsp.inputFinished) {
        // Filter fully drained — the produced stream ends here (the same
        // WSOLA tail drop as the whole-buffer path).
        runtime.productionDone = true
        break
      }
      if (dsp.sourceExhausted) {
        dsp.finishInput()
        continue
      }
      const rate = runtime.decodedRate
      const srcTotal = Math.round(runtime.sourceTotalSeconds * rate)
      const srcRemaining = srcTotal - dsp.sourceFramesConsumed
      if (srcRemaining <= 0) {
        dsp.finishInput()
        continue
      }
      // Bound the source FIFO to two windows of source audio.
      const pushFrames = Math.min(30 * rate, srcRemaining)
      if (dsp.bufferedSourceFrames > 2 * 30 * rate) continue
      const decoded = await decodeSourceSpan(
        runtime,
        segment,
        fps,
        dsp.sourceFramesConsumed,
        pushFrames,
        decoderPool,
      )
      if (!decoded || decoded.samples.every((c) => c.length === 0)) {
        dsp.finishInput()
        continue
      }
      dsp.pushInput(decoded.samples, decoded.samples[0]!.length)
      // Short read: media shorter than the segment's claimed source span.
      if (decoded.samples[0]!.length < decoded.requestedFrames) dsp.finishInput()
      stalledIterations = 0
      continue
    }

    // First source window: decode a slice to learn the source's real rate.
    const decoded = await decodeSourceSpan(
      runtime,
      segment,
      fps,
      0,
      Math.min(maxPushFrames, Math.round(runtime.sourceTotalSeconds * 48_000)),
      decoderPool,
    )
    if (!decoded || decoded.samples.every((c) => c.length === 0)) {
      runtime.productionDone = true
      break
    }
    initWindowedRuntime(
      runtime,
      segment,
      fps,
      sampleRate,
      decoded.samples,
      decoded.sampleRate,
    )
    runtime.dsp!.pushInput(decoded.samples, decoded.samples[0]!.length)
    stalledIterations = 0
  }
  if (runtime.productionDone && runtime.dsp && runtime.producedEnd < through) {
    const tail = runtime.dsp.pullTail(through - runtime.producedEnd)
    if (tail) {
      for (let channel = 0; channel < tail.length; channel++) {
        runtime.producedQueue[channel]!.push(tail[channel]!)
      }
      runtime.producedEnd += tail[0]?.length ?? 0
    }
  }
}


function mixAudioWindowChannels(
  mixed: Float32Array[],
  processed: Float32Array[],
  destinationOffset: number,
  requestedFrames: number,
): void {
  const downmixed = downmixToOutputChannels(processed, mixed.length)
  for (let channel = 0; channel < mixed.length; channel++) {
    const source = downmixed[channel]!
    const destination = mixed[channel]!
    const framesToMix = Math.min(
      requestedFrames,
      source.length,
      destination.length - destinationOffset,
    )
    for (let i = 0; i < framesToMix; i++) {
      destination[destinationOffset + i] = destination[destinationOffset + i]! + source[i]!
    }
  }
}

async function mixSegmentIntoAudioWindow(params: {
  segment: AudioSegment
  runtime: WindowedSegmentRuntime
  mixed: Float32Array[]
  chunkStart: number
  chunkEnd: number
  sampleRate: number
  fps: number
  decoderPool: WindowedAudioDecoderPool
  duckingSources: readonly DuckingSource[]
  signal?: AbortSignal
}): Promise<void> {
  const {
    segment,
    runtime,
    mixed,
    chunkStart,
    chunkEnd,
    sampleRate,
    fps,
    decoderPool,
    duckingSources,
    signal,
  } = params
  const segmentStart = Math.floor((segment.startFrame / fps) * sampleRate)
  const nominalLength = Math.ceil((segment.durationFrames / fps) * sampleRate)
  if (runtime.decodedRate === 0) {
    if (chunkEnd <= segmentStart || chunkStart >= segmentStart + nominalLength) return
    await produceThrough(runtime, segment, 1, fps, sampleRate, decoderPool, signal)
  }
  const rate = runtime.decodedRate || sampleRate
  const tailLength = Math.ceil(((runtime.dsp?.tailOutputFrames ?? 0) * sampleRate) / rate)
  const intersection = {
    segmentStart,
    intersectionStart: Math.max(chunkStart, segmentStart),
    intersectionEnd: Math.min(chunkEnd, segmentStart + nominalLength + tailLength),
  }
  if (intersection.intersectionEnd <= intersection.intersectionStart) return
  const spanStart = intersection.intersectionStart - segmentStart
  const spanEnd = intersection.intersectionEnd - segmentStart
  const wanted = Math.ceil((spanEnd / sampleRate) * rate) +
    resamplerLookaheadFrames(runtime) + 4096
  const producedNeed = runtime.producedTotal > 0
    ? Math.min(runtime.producedTotal, wanted)
    : wanted
  await produceThrough(runtime, segment, producedNeed, fps, sampleRate, decoderPool, signal)
  consumeProducedThrough(runtime, segment, producedNeed, fps, duckingSources)
  dropConsumedProduced(runtime)

  // At segment end, flush resampler tails into the queue.
  if (
    runtime.productionDone &&
    (runtime.dsp?.tailFramesRemaining ?? 0) === 0 &&
    runtime.producedEnd - runtime.producedConsumed <= 0 &&
    !runtime.resamplersFinished
  ) {
    runtime.resamplersFinished = true
    if (runtime.resamplers) {
      for (let c = 0; c < runtime.outQueue.length; c++) {
        runtime.outQueue[c]!.push(runtime.resamplers[c]!.finish())
      }
    }
  }

  // Mix the span out of the 48 kHz queues. A segment whose decodes all
  // failed contributes nothing.
  if (runtime.outQueue.length === 0) return
  const need = spanEnd - spanStart
  const windowChannels: Float32Array[] = []
  for (let c = 0; c < runtime.outQueue.length; c++) {
    const queue = runtime.outQueue[c]!
    const scratch = new Float32Array(need)
    const copyOffset = Math.max(0, spanStart - runtime.outHead)
    queue.readInto(scratch, 0, copyOffset, need)
    windowChannels.push(scratch)
  }
  mixAudioWindowChannels(mixed, windowChannels, intersection.intersectionStart - chunkStart, need)

  // Advance outQueue heads to the end of this span.
  const covered = Math.min(spanEnd, runtime.outHead + (runtime.outQueue[0]?.length ?? 0))
  const advance = Math.max(0, covered - runtime.outHead)
  for (const queue of runtime.outQueue) queue.drop(advance)
  runtime.outHead += advance
}

function applyAudioWindowMasterGain(mixed: Float32Array[], masterGain: number): void {
  for (const channel of mixed) {
    for (let i = 0; i < channel.length; i++) channel[i] = channel[i]! * masterGain
  }
}

/**
 * Mix long, non-stateful audio timelines in fixed windows. Every yielded chunk
 * begins exactly where the previous one ended, including silent windows.
 */
export async function* processAudioWindows(
  composition: CompositionInputProps,
  signal?: AbortSignal,
): AsyncGenerator<{ samples: Float32Array[]; sampleRate: number; channels: number }> {
  const { fps, durationInFrames = 0 } = composition
  await resolveSubCompMediaUrls(composition)
  const segments = extractAudioSegments(composition, fps).filter((segment) => !segment.muted)
  if (segments.length === 0) {
    throw new Error('Audio timeline has no segments')
  }

  if (segments.some((segment) => isAc3AudioCodec(segment.audioCodec))) {
    await ensureAc3DecoderRegistered()
  }

  const sampleRate = 48_000
  const channels = 2
  const totalSamples = Math.ceil((durationInFrames / fps) * sampleRate)
  const chunkSamples = STREAMING_AUDIO_CHUNK_SECONDS * sampleRate
  const masterGain =
    typeof composition.masterBusDb === 'number' && composition.masterBusDb !== 0
      ? dbToGain(composition.masterBusDb)
      : 1

  const duckingSources = collectDuckingSources(composition, fps)

  const decoderPool = await createWindowedAudioDecoderPool()
  const segmentRuntimes = new Map<AudioSegment, WindowedSegmentRuntime>()
  const trackEffects = new Map(
    composition.tracks
      .filter((track) => (track.audioFx?.length ?? 0) > 0)
      .map((track) => [track.id, createAudioFxChain(track.audioFx, channels, sampleRate)] as const),
  )
  try {
    for (let chunkStart = 0; chunkStart < totalSamples; chunkStart += chunkSamples) {
      if (signal?.aborted) throw new DOMException('Audio processing cancelled', 'AbortError')

      const chunkLength = Math.min(chunkSamples, totalSamples - chunkStart)
      const chunkEnd = chunkStart + chunkLength
      const mixed = [new Float32Array(chunkLength), new Float32Array(chunkLength)]
      const trackMixes = new Map<string, Float32Array[]>()
      for (const segment of segments) {
        let runtime = segmentRuntimes.get(segment)
        if (!runtime) {
          runtime = createWindowedSegmentRuntime(segment, fps)
          segmentRuntimes.set(segment, runtime)
        }
        let destination: Float32Array[] = mixed
        if (trackEffects.has(segment.trackId)) {
          destination = trackMixes.get(segment.trackId) ?? [
            new Float32Array(chunkLength), new Float32Array(chunkLength),
          ]
          trackMixes.set(segment.trackId, destination)
        }
        await mixSegmentIntoAudioWindow({
          segment,
          runtime,
          mixed: destination,
          chunkStart,
          chunkEnd,
          sampleRate,
          fps,
          decoderPool,
          duckingSources,
          signal,
        })
        // Once this window ends past the segment, its runtime is done.
        const segmentEnd =
          Math.floor((segment.startFrame / fps) * sampleRate) +
          Math.ceil((segment.durationFrames / fps) * sampleRate) +
          Math.ceil(((runtime.dsp?.tailOutputFrames ?? 0) / (runtime.decodedRate || sampleRate)) * sampleRate)
        if (chunkEnd >= segmentEnd) {
          segmentRuntimes.delete(segment)
        }
      }
      // Nonlinear stages must see the sum of all clips on their track.
      // Feed every bus every window, including silence, to carry latency and tails.
      for (const [trackId, effects] of trackEffects) {
        const bus = trackMixes.get(trackId) ?? [
          new Float32Array(chunkLength), new Float32Array(chunkLength),
        ]
        for (let channel = 0; channel < channels; channel++) {
          const processed = effects.processChunk(bus[channel]!, channel)
          for (let frame = 0; frame < chunkLength; frame++) {
            mixed[channel]![frame] = mixed[channel]![frame]! + processed[frame]!
          }
        }
      }

      softClipAudioMix(mixed)
      applyAudioWindowMasterGain(mixed, masterGain)

      yield { samples: mixed, sampleRate, channels }
    }
  } finally {
    await decoderPool.dispose()
  }
}

/**
 * Process all audio for the composition.
 *
 * @param composition - The composition with tracks
 * @param signal - Optional abort signal
 * @returns Processed audio ready for encoding
 */
export async function processAudio(
  composition: CompositionInputProps,
  signal?: AbortSignal,
): Promise<{
  samples: Float32Array[]
  sampleRate: number
  channels: number
} | null> {
  const { fps, durationInFrames = 0 } = composition

  await resolveSubCompMediaUrls(composition)

  // Extract audio segments
  const segments = extractAudioSegments(composition, fps)

  // Filter out muted segments early
  const activeSegments = segments.filter((s) => !s.muted)

  if (activeSegments.length === 0) {
    log.info('No audio segments to process')
    return null
  }
  if (
    activeSegments.some((segment) => (segment.audioFx?.length ?? 0) > 0) ||
    composition.tracks.some((track) => (track.audioFx?.length ?? 0) > 0)
  ) {
    // Use the same per-clip and per-track DSP code for short and long exports.
    const chunks: Float32Array[][] = [[], []]
    let length = 0
    for await (const window of processAudioWindows(composition, signal)) {
      length += window.samples[0]!.length
      for (let channel = 0; channel < 2; channel++) chunks[channel]!.push(window.samples[channel]!)
    }
    const samples = chunks.map((parts) => {
      const output = new Float32Array(length)
      let offset = 0
      for (const part of parts) { output.set(part, offset); offset += part.length }
      return output
    })
    return { samples, sampleRate: 48_000, channels: 2 }
  }

  const requiresAc3Decoder = activeSegments.some((segment) => isAc3AudioCodec(segment.audioCodec))
  if (requiresAc3Decoder) {
    await ensureAc3DecoderRegistered()
    log.debug('AC-3 decoder pre-registered for export audio decode')
  }

  const duckingSources = collectDuckingSources(composition, fps)

  // Configuration
  const config: AudioProcessingConfig = {
    sampleRate: 48000, // Standard export sample rate
    channels: 2, // Stereo
    fps,
    totalFrames: durationInFrames,
  }

  log.info('Processing audio', {
    segmentCount: activeSegments.length,
    sampleRate: config.sampleRate,
    channels: config.channels,
    durationSeconds: durationInFrames / fps,
  })

  // Allocate the final mix once, then fold each processed segment into it
  // immediately. Keeping every decoded segment alive until the end multiplies
  // memory use on long, multi-clip timelines.
  const mixedSamples = createAudioMixBuffer(config)
  let processedSegmentCount = 0

  for (const segment of activeSegments) {
    if (signal?.aborted) {
      throw new DOMException('Audio processing cancelled', 'AbortError')
    }

    try {
      // Calculate the time range we actually need from the source
      // sourceStartFrame is in source-native FPS frames, so divide by sourceFps (not project fps)
      const sourceStartTime = segment.sourceStartFrame / segment.sourceFps
      // Account for speed: at 2x speed, we need twice as much source audio
      const sourceDurationNeeded = (segment.durationFrames / fps) * segment.speed
      const sourceEndTime = sourceStartTime + sourceDurationNeeded

      // Decode ONLY the needed range using mediabunny (huge performance improvement!)
      const decoded = await decodeAudioFromSource(
        segment.src,
        segment.itemId,
        sourceStartTime,
        sourceEndTime,
        segment.audioCodec,
      )

      // Process audio channels.
      // Note: decoded audio is already trimmed to the range we requested.

      // Apply speed across ALL channels at once to maintain phase coherence
      // between L/R (the WSOLA pipeline finds shared overlap windows).
      let processedChannels = decoded.samples
      if (segment.isReversed) {
        processedChannels = reverseAudioChannels(processedChannels)
      }
      if (
        Math.abs(segment.speed - 1) > 0.0001 ||
        isAudioPitchShiftActive(segment.pitchShiftSemitones)
      ) {
        processedChannels = await applySpeedAndPitch(
          processedChannels,
          segment.speed,
          segment.pitchShiftSemitones,
          decoded.sampleRate,
        )
      }
      processedChannels = applyAudioEqStages(
        processedChannels,
        decoded.sampleRate,
        segment.audioEqStages,
      )

      // Apply per-channel volume, fades, and resampling
      const fadeInSamples = Math.floor((segment.fadeInFrames / fps) * decoded.sampleRate)
      const fadeOutSamples = Math.floor((segment.fadeOutFrames / fps) * decoded.sampleRate)
      const crossfadeFadeInSamples = Math.floor(
        ((segment.crossfadeFadeInFrames ?? 0) / fps) * decoded.sampleRate,
      )
      const crossfadeFadeOutSamples = Math.floor(
        ((segment.crossfadeFadeOutFrames ?? 0) / fps) * decoded.sampleRate,
      )
      const contentStartOffsetSamples = Math.floor(
        ((segment.contentStartOffsetFrames ?? 0) / fps) * decoded.sampleRate,
      )
      const contentEndOffsetSamples = Math.floor(
        ((segment.contentEndOffsetFrames ?? 0) / fps) * decoded.sampleRate,
      )
      const fadeInDelaySamples = Math.floor(
        ((segment.fadeInDelayFrames ?? 0) / fps) * decoded.sampleRate,
      )
      const fadeOutLeadSamples = Math.floor(
        ((segment.fadeOutLeadFrames ?? 0) / fps) * decoded.sampleRate,
      )

      for (let c = 0; c < processedChannels.length; c++) {
        let channelSamples = processedChannels[c]!

        // Apply volume (animated if keyframes exist, static otherwise)
        if (segment.volumeKeyframes && segment.volumeKeyframes.length > 0) {
          channelSamples = applyAnimatedVolume(
            channelSamples,
            segment.volumeKeyframes,
            segment.volume,
            segment.startFrame,
            segment.itemFrom,
            fps,
            decoded.sampleRate,
          )
        } else if (segment.volume !== 0) {
          channelSamples = applyVolume(channelSamples, segment.volume)
        }

        // Apply fades
        if (segment.clipFadeSpans && segment.clipFadeSpans.length > 0) {
          channelSamples = applyClipFadeSpans(
            channelSamples,
            segment.clipFadeSpans,
            decoded.sampleRate,
            fps,
          )
        } else if (fadeInSamples > 0 || fadeOutSamples > 0) {
          channelSamples = applyFades(
            channelSamples,
            fadeInSamples,
            fadeOutSamples,
            false,
            segment.fadeInCurve,
            segment.fadeOutCurve,
            segment.fadeInCurveX,
            segment.fadeOutCurveX,
            contentStartOffsetSamples,
            contentEndOffsetSamples,
            fadeInDelaySamples,
            fadeOutLeadSamples,
          )
        }

        if (crossfadeFadeInSamples > 0 || crossfadeFadeOutSamples > 0) {
          channelSamples = applyFades(
            channelSamples,
            crossfadeFadeInSamples,
            crossfadeFadeOutSamples,
            true,
          )
        }

        if (duckingSources.length > 0) {
          channelSamples = applyDucking(
            channelSamples,
            duckingSources,
            segment,
            segment.startFrame,
            fps,
            decoded.sampleRate,
          )
        }

        // Resample to target sample rate
        if (decoded.sampleRate !== config.sampleRate) {
          channelSamples = resample(channelSamples, decoded.sampleRate, config.sampleRate)
        }

        processedChannels[c] = channelSamples
      }

      // Calculate start position in output
      const startSample = Math.floor((segment.startFrame / fps) * config.sampleRate)

      mixAudioSegmentInto(mixedSamples, { samples: processedChannels, startSample }, config)
      processedSegmentCount++

      log.debug('Processed audio segment', {
        itemId: segment.itemId,
        type: segment.type,
        startSample,
        outputSamples: processedChannels[0]?.length,
      })
    } catch (error) {
      // Main-thread-only work must bubble up so renderInWorker can route
      // the whole export to the main thread; other failures keep the
      // long-standing skip-and-continue behavior.
      if (getErrorMessage(error).startsWith('WORKER_REQUIRES_MAIN_THREAD:')) {
        throw error
      }
      log.error(`Failed to process audio segment ${segment.itemId}: ${getErrorMessage(error)}`)
      // Continue with other segments
    }
  }

  if (processedSegmentCount === 0) {
    log.warn('No audio segments were successfully processed')
    return null
  }

  softClipAudioMix(mixedSamples)

  // Apply project-scoped master bus gain to the final mix. Monitor volume
  // (per-device) is intentionally NOT applied during export — it's a
  // preview-only setting.
  const masterBusDb = composition.masterBusDb
  if (typeof masterBusDb === 'number' && masterBusDb !== 0) {
    const masterBusGain = dbToGain(masterBusDb)
    for (const channel of mixedSamples) {
      for (let i = 0; i < channel.length; i++) {
        channel[i] = channel[i]! * masterBusGain
      }
    }
  }

  log.info('Audio processing complete', {
    outputSamples: mixedSamples[0]?.length,
    channels: mixedSamples.length,
    durationSeconds: (mixedSamples[0]?.length ?? 0) / config.sampleRate,
    masterBusDb: masterBusDb ?? 0,
  })

  return {
    samples: mixedSamples,
    sampleRate: config.sampleRate,
    channels: config.channels,
  }
}

/**
 * Check if composition has any audio content.
 * Async because sub-composition media URLs may need to be resolved from OPFS
 * before extractAudioSegments can see valid src values.
 */
export async function hasAudioContent(composition: CompositionInputProps): Promise<boolean> {
  await resolveSubCompMediaUrls(composition)
  const segments = extractAudioSegments(composition, composition.fps)
  return segments.some((s) => !s.muted)
}
