import type { SerializedCommand } from '@editor/commands/types'
import type { WaveformEnvelope } from '@editor/features/waveform-visualizer/types'
import type { MediaMetadata } from '@editor/types/storage'
import type { MotionModifier } from '@editor/types/motion'
import type { AudioItem, CaptionWord, ImageItem, SubtitleSegmentItem, TimelineItem, VideoItem } from '@editor/types/timeline'
import { buildCaptionPages, type CaptionPage } from './caption-pages'
import { stillImageZoomKeyframes } from './resolver'
import { resolveTemplateRecipe } from './template-recipe'
import type { TemplateProjectShape } from './channel-template'
import { sourceToTimelineFrames, timelineToSourceFrames } from '@editor/features/timeline/utils/source-calculations'
import { calculateTransitionPortions } from '@editor/shared/timeline/transitions/transition-planner'
import { getMaxTransitionDurationForHandles } from '@editor/features/timeline/utils/transition-utils'
import { applyBackgroundAppearance, retimeBackgroundClip } from './background-appearance'
import { templateBackgroundAppearanceSchema } from './template-schemas'

/**
 * Turn a channel-template snapshot plus the clone inputs into the serialized
 * command batch that fills it. Pure: the caller applies the commands through
 * `applyCommands` (dry run then commit), the same path a browser batch takes.
 *
 * - Slot tracks are emptied and rebuilt: one image per scene in the first
 *   image clip's style (Ken Burns alternated, or the recipe's `wander`
 *   modifier seeded per scene; the `cutout-close` recipe enlarges each image
 *   around its subject box), one timed caption segment in the first
 *   subtitle's style, the source audio in the first audio clip's mix, and
 *   the `background-video` track re-tiled from its own clips under the job
 *   seed.
 * - Waveform-visualizer items bound to the template audio's media are rebound
 *   to the clone audio and re-spanned over it.
 * - Everything else is re-timed by its `templateAnchor`: `full` covers the
 *   clone (videos loop), `start` keeps its offset, `end` keeps its distance
 *   from the end; items falling outside the duration are dropped.
 */

const KEN_BURNS_SCALE = 1.12

/**
 * Close-up framing for `cutout-close`: the top {@link CLOSE_UP_BODY} of the
 * subject box (head to waist for a standing figure) fills
 * {@link CLOSE_UP_FILL} of the frame height, a wide group may overflow the
 * frame width by at most {@link CLOSE_UP_MAX_WIDTH}, and the zoom stays within
 * [{@link CLOSE_UP_MIN_SCALE}, {@link CLOSE_UP_MAX_SCALE}] so a tiny subject
 * is not blown up into a blur and a frame-filling one still reads as closer.
 */
const CLOSE_UP_BODY = 0.55
const CLOSE_UP_FILL = 0.95
const CLOSE_UP_MAX_WIDTH = 1.15
const CLOSE_UP_MIN_SCALE = 1.25
const CLOSE_UP_MAX_SCALE = 2.2
/** Gap left above the subject's top edge, as a share of the frame height. */
const CLOSE_UP_HEADROOM = 0.04

/** Opaque-pixel bounds of a cut-out image, as fractions (0…1) of the image size. */
export type SubjectBox = { x: number; y: number; width: number; height: number }

export type TemplateFillScene = {
  sceneId: string
  title: string
  startFrame: number
  endFrame: number
  mediaId: string
  src: string
  /** Required by the `cutout-close` recipe; ignored otherwise. */
  subjectBox?: SubjectBox
}

export type TemplateFillCueWord = {
  startMs: number
  endMs: number
  text: string
  /** True when the piece held multiple words — timing is shared, not per-word. */
  estimated?: boolean
}
export type TemplateFillCue = {
  id: string
  /** Absolute source seconds; cue ends are measured, never invented. */
  startSec: number
  endSec: number
  text: string
  words: TemplateFillCueWord[] | null
}

export interface TemplateFillInput {
  template: TemplateProjectShape
  idPrefix: string
  /** The clone's duration in frames. */
  durationInFrames: number
  /** Per-job deterministic seed (uint32) for the background shuffle and wander phases. */
  seed: number
  /** New media the fill adds to the project library (scenes, source audio…). */
  media: MediaMetadata[]
  scenes: TemplateFillScene[]
  cues: TemplateFillCue[]
  /** The source audio media plus its length in frames; `measuredLufs` feeds the slot's loudness target. */
  audio: { mediaId: string; src: string; durationInFrames: number; measuredLufs?: number }
  envelope: WaveformEnvelope
}

export interface TemplateFillResult {
  commands: SerializedCommand[]
  itemIds: {
    scenes: string[]
    captions: string | null
    /** Re-bound waveform item, or null when the template has none. */
    waveform: string | null
    subscribe: string[]
  }
  captions: { cueCount: number; wordsEstimated: boolean }
  /** Loudness normalisation applied to the source-audio clip, or null when the slot has no target/measurement. */
  loudness: { targetLufs: number; appliedGainDb: number } | null
}

export interface PreparedCaptions {
  cues: { id: string; text: string; startSeconds: number; endSeconds: number }[]
  words: CaptionWord[]
  /** Normalised pages the cues+words came from — the MP4 overlay consumes these directly. */
  pages: CaptionPage[]
  estimated: boolean
}

/**
 * Turn absolute-second cues into stored caption data through the shared page
 * normaliser: each page becomes one cue (≤7 words / ≤42 chars), multi-word
 * pieces split into single words with timing distributed by character length,
 * and `estimated` reports whether any timing was approximated or dropped.
 * Cues outside the project duration are trimmed off; nothing is invented.
 *
 * Word timings are first made to follow reading order (see
 * `orderCueWordFrames`): aligned or sampled words can overflow their cue or
 * run backwards, and the page normaliser assigns words to cues by time, so
 * unordered timing would drop words or interleave two cues into one page.
 */
export function prepareCaptionData(cues: TemplateFillCue[], fps: number, durationInFrames: number): PreparedCaptions {
  // A zero-length cue that still carries timed words gets its window from them in `orderCueWordFrames`.
  const valid = cues.filter((cue) => (cue.endSec > cue.startSec || (cue.words?.length ?? 0) > 0) && cue.startSec >= 0 && cue.startSec * fps < durationInFrames)
  const { cues: sourceCues, words } = orderCueWordFrames(valid, fps, durationInFrames)
  const { pages, degraded } = buildCaptionPages({ cues: sourceCues, words, durationInFrames }, fps)
  return {
    pages,
    cues: pages.map((page) => ({ id: page.id, text: page.text, startSeconds: page.startFrame / fps, endSeconds: page.endFrame / fps })),
    words: pages.flatMap((page) => page.words.map((word) => ({ text: word.text, startFrame: word.startFrame, endFrame: word.endFrame, estimated: word.estimated === true }))),
    estimated: degraded,
  }
}

/**
 * Frame-quantised cues and words in reading order. Invariants on the result:
 * every word lasts at least one frame; word starts strictly increase across
 * all cues; each cue starts at or before its first word and after the previous
 * cue's last word starts, and ends at or after its last word. An overlap with
 * the next word trims the earlier word's end; a word that starts no later than
 * the previous one moves after it and is marked estimated.
 */
export function orderCueWordFrames(
  cues: readonly TemplateFillCue[],
  fps: number,
  durationInFrames: number,
): { cues: { id: string; text: string; startSeconds: number; endSeconds: number }[]; words: CaptionWord[] } {
  const toFrame = (ms: number) => Math.round((ms / 1000) * fps)
  const sorted = [...cues].sort((a, b) => a.startSec - b.startSec)
  const outCues: { id: string; text: string; startSeconds: number; endSeconds: number }[] = []
  const words: CaptionWord[] = []
  // Start frame of the last emitted word (or wordless cue): the next cue must begin after it.
  let lastStart = -1
  for (const cue of sorted) {
    const firstIndex = words.length
    for (const word of cue.words ?? []) {
      let startFrame = toFrame(word.startMs)
      let estimated = word.estimated === true
      const previous = words.at(-1)
      if (previous && startFrame <= previous.startFrame) {
        startFrame = previous.endFrame
        estimated = true
      } else if (previous && startFrame < previous.endFrame) {
        previous.endFrame = startFrame
      }
      const endFrame = Math.max(startFrame + 1, toFrame(word.endMs))
      words.push({ text: word.text, startFrame, endFrame, ...(estimated ? { estimated: true } : {}) })
    }
    const own = words.slice(firstIndex)
    const startFrame = Math.max(lastStart + 1, Math.min(Math.round(cue.startSec * fps), own[0]?.startFrame ?? Infinity))
    const endFrame = Math.min(durationInFrames, Math.max(Math.round(cue.endSec * fps), own.at(-1)?.endFrame ?? 0))
    lastStart = own.at(-1)?.startFrame ?? startFrame
    if (endFrame <= startFrame) continue
    outCues.push({ id: cue.id, text: cue.text, startSeconds: startFrame / fps, endSeconds: endFrame / fps })
  }
  return { cues: outCues, words }
}

/** Identity, source timing and bindings must not leak from a prototype into a new clip. */
const CLIP_IDENTITY_FIELDS: Record<string, true> = {
  id: true, trackId: true, from: true, durationInFrames: true, label: true,
  src: true, audioSrc: true, mediaId: true, thumbnailUrl: true,
  sourceStart: true, sourceEnd: true, sourceDuration: true, sourceFps: true,
  trimStart: true, trimEnd: true, offset: true, speed: true, isReversed: true,
  linkedGroupId: true, originId: true, templateAnchor: true, transformParent: true,
}

/** Deep-copy of the clip's style fields — transform, effects, blend, audio mix — minus identity and `keep`. */
function prototypeCopy(item: TimelineItem, keep: readonly string[] = []): Record<string, unknown> {
  const copy: Record<string, unknown> = {}
  for (const key of Object.keys(item)) {
    if (CLIP_IDENTITY_FIELDS[key] || key.startsWith('reverseConform') || keep.includes(key)) continue
    const value = item[key as keyof TimelineItem]
    if (value === undefined) continue
    copy[key] = typeof value === 'object' && value !== null ? structuredClone(value) : value
  }
  return copy
}

/** The earliest clip of `type` on the track is the style prototype ("Chỉ dùng clip đầu làm mẫu"). */
function prototypeItem(items: readonly TimelineItem[], trackId: string, type: TimelineItem['type']): TimelineItem | undefined {
  let prototype: TimelineItem | undefined
  for (const item of items) {
    if (item.trackId === trackId && item.type === type && (!prototype || item.from < prototype.from)) prototype = item
  }
  return prototype
}

/** Mulberry32 PRNG — tiny, deterministic, good enough for order shuffles. */
export function mulberry32(seed: number): () => number {
  let a = (seed | 0) + 0x9e3779b9
  return () => {
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** Fold an index into the job seed so each consumer gets its own deterministic stream. */
function seedHash(seed: number, index: number): number {
  let h = (seed ^ 0x9e3779b9) >>> 0
  h = Math.imul(h ^ (index + 1), 0x85ebca6b) >>> 0
  h ^= h >>> 13
  h = Math.imul(h, 0xc2b2ae35) >>> 0
  h ^= h >>> 16
  return h >>> 0
}

/** One Fisher–Yates pass over the pool indexes, drawing from `random` in place. */
function shuffledIndexes(poolSize: number, random: () => number): number[] {
  const order = Array.from({ length: poolSize }, (_, index) => index)
  for (let i = order.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1))
    const swap = order[i] ?? i
    order[i] = order[j] ?? j
    order[j] = swap
  }
  return order
}

/**
 * The `base` image rect enlarged so the subject's upper body fills the frame:
 * centred horizontally on the subject, its top edge just under the frame top.
 * Item transforms position the rect centre relative to the canvas centre.
 */
export function closeUpTransform(base: { x: number; y: number; width: number; height: number }, box: SubjectBox, frame: { width: number; height: number }): { x: number; y: number; width: number; height: number } {
  const byHeight = (CLOSE_UP_FILL * frame.height) / (CLOSE_UP_BODY * box.height * base.height)
  const byWidth = (CLOSE_UP_MAX_WIDTH * frame.width) / (box.width * base.width)
  const scale = Math.min(CLOSE_UP_MAX_SCALE, Math.max(CLOSE_UP_MIN_SCALE, Math.min(byHeight, byWidth)))
  const width = base.width * scale
  const height = base.height * scale
  return {
    width,
    height,
    x: base.x + width * (0.5 - (box.x + box.width / 2)),
    y: CLOSE_UP_HEADROOM * frame.height - box.y * height - frame.height / 2 + height / 2,
  }
}

/**
 * The pool clip's source window, shrunk just enough to leave `head` and `tail`
 * timeline frames of unused footage, so a join transition has frames to blend.
 * Footage already outside the window counts. `null` keeps the clip as it is:
 * looped or unmeasured clips, and clips that would lose more than half.
 */
function joinReadyWindow(clip: VideoItem, head: number, tail: number, fps: number): { sourceStart: number; frames: number; speed: number; sourceFps: number } | null {
  if (clip.loop || clip.sourceEnd === undefined || clip.sourceDuration === undefined) return null
  const speed = clip.speed ?? 1
  const sourceFps = clip.sourceFps ?? fps
  const start = Math.max(clip.sourceStart ?? 0, timelineToSourceFrames(head, speed, fps, sourceFps))
  const end = Math.min(clip.sourceEnd, clip.sourceDuration - timelineToSourceFrames(tail, speed, fps, sourceFps))
  const frames = sourceToTimelineFrames(end - start, speed, sourceFps, fps)
  return frames * 2 >= clip.durationInFrames ? { sourceStart: start, frames: Math.min(frames, clip.durationInFrames), speed, sourceFps } : null
}

export function buildTemplateFill(input: TemplateFillInput): TemplateFillResult {
  const D = input.durationInFrames
  if (!input.idPrefix || !Number.isInteger(D) || D <= 0 || input.template.metadata.fps <= 0) {
    throw new Error('Thông số đổ template không hợp lệ.')
  }
  for (const scene of input.scenes) {
    if (!Number.isInteger(scene.startFrame) || !Number.isInteger(scene.endFrame) || scene.endFrame <= scene.startFrame || scene.startFrame < 0 || scene.endFrame > D) {
      throw new Error(`Cảnh ${scene.sceneId} vượt ngoài thời lượng project.`)
    }
  }
  const { template } = input
  const { width, height, fps } = template.metadata
  const prefix = input.idPrefix
  const seed = input.seed >>> 0
  const recipe = resolveTemplateRecipe(template)
  const appearance = templateBackgroundAppearanceSchema.parse(recipe.backgroundAppearance)
  const commands: SerializedCommand[] = []
  const addItem = (item: TimelineItem) => commands.push({ type: 'item.add', params: { item } })
  const slotTrack = (slot: 'scenes' | 'captions' | 'source-audio' | 'background-video') =>
    template.tracks.find((track) => track.templateSlot === slot)

  // Library first: `waveform.setEnvelope` validates against media records in state.
  for (const media of input.media) commands.push({ type: 'media.add', params: { media } })

  const itemIds: TemplateFillResult['itemIds'] = { scenes: [], captions: null, waveform: null, subscribe: [] }
  const removeIds: string[] = []
  const slotTrackIds = new Set<string>()
  /* ------------------------------- scenes slot ------------------------------- */
  const scenesTrack = slotTrack('scenes')
  const wander = recipe.motion.kind === 'wander'
  if (scenesTrack) {
    slotTrackIds.add(scenesTrack.id)
    removeIds.push(...template.items.filter((item) => item.trackId === scenesTrack.id).map((item) => item.id))
    const prototype = prototypeItem(template.items, scenesTrack.id, 'image') as ImageItem | undefined
    const style = prototype ? prototypeCopy(prototype) : {}
    const transform = (style.transform as ImageItem['transform']) ?? { x: 0, y: 0, width, height }
    const baseRect = { x: transform?.x ?? 0, y: transform?.y ?? 0, width: transform?.width ?? width, height: transform?.height ?? height }
    const closeUp = recipe.style === 'cutout-close'
    for (const [index, scene] of input.scenes.entries()) {
      const id = `${prefix}-scene-${scene.sceneId}`
      const frames = scene.endFrame - scene.startFrame
      if (closeUp && !scene.subjectBox) throw new Error(`Cảnh ${scene.sceneId} thiếu khung nhân vật để phóng to.`)
      const sceneTransform = closeUp && scene.subjectBox ? { ...transform, ...closeUpTransform(baseRect, scene.subjectBox, { width, height }) } : transform
      const motionModifiers: MotionModifier[] | undefined = wander
        ? [{
            version: 2,
            id: `${prefix}-mm-${scene.sceneId}`,
            type: 'wander',
            enabled: true,
            amplitude: recipe.motion.intensity,
            frequency: 0.25 / recipe.motion.speed,
            phaseFrames: 0,
            seed: seedHash(seed, index),
          }]
        : undefined
      addItem({
        ...style,
        id,
        type: 'image',
        trackId: scenesTrack.id,
        from: scene.startFrame,
        durationInFrames: frames,
        label: scene.title || `Cảnh ${index + 1}`,
        mediaId: scene.mediaId,
        src: scene.src,
        // Shared scene opacity multiplies whatever the prototype carries;
        // untouched at the default so unchanged templates fill identically.
        transform: recipe.sceneOpacity === 1
          ? sceneTransform
          : { ...sceneTransform, opacity: (sceneTransform.opacity ?? 1) * recipe.sceneOpacity },
        ...(motionModifiers ? { motionModifiers } : {}),
      } as ImageItem)
      itemIds.scenes.push(id)
      // Wander drifts procedurally at render time; otherwise a subtle Ken Burns
      // keyframed zoom, alternating direction per scene index.
      if (!wander) {
        commands.push(...stillImageZoomKeyframes(id, { width: sceneTransform?.width ?? width, height: sceneTransform?.height ?? height }, frames, index % 2 === 0 ? 1 : KEN_BURNS_SCALE, index % 2 === 0 ? KEN_BURNS_SCALE : 1))
      }
    }
  }

  /* ------------------------------ background slot ---------------------------- */
  // A 'background-video' slot track becomes the clone's picture-only base
  // layer: its video clips pool, shuffle per cycle under the job seed, and
  // tile [0, D) end to end. Each clone clip keeps its own source window and
  // style, muted, shrunk only as far as the join crossfade needs unused footage
  // on both sides. The track's own items are removed like the other slots;
  // only pool order comes from `from`, not timeline order.
  const backgroundJoins: { leftId: string; rightId: string; leftFrames: number; rightFrames: number; maxDuration?: number }[] = []
  const backgroundTrack = template.tracks.find((track) => track.id === recipe.backgroundTrackId)
  const backgroundPool = backgroundTrack
    ? template.items
        .filter((item): item is VideoItem => item.trackId === backgroundTrack.id && item.type === 'video' && item.durationInFrames > 0)
        .sort((a, b) => a.from - b.from)
        .map(clip => retimeBackgroundClip(clip, appearance.speed, fps))
    : []
  // Background clips come from unrelated sources, so a hard cut between them reads as a glitch.
  // A transition the author placed on the background track wins; otherwise joins crossfade for 1 s.
  const backgroundPoolIds = new Set(backgroundPool.map(item => item.id))
  const backgroundTransition = template.transitions?.find(entry =>
    entry.trackId === backgroundTrack?.id && backgroundPoolIds.has(entry.leftClipId) && backgroundPoolIds.has(entry.rightClipId))
    ?? { presentation: 'fade' as const, timing: 'ease-in-out' as const, alignment: 0.5, direction: undefined, durationInFrames: fps }
  if (backgroundTrack) {
    slotTrackIds.add(backgroundTrack.id)
    removeIds.push(...template.items.filter((item) => item.trackId === backgroundTrack.id).map((item) => item.id))
    // A pool clip's linked audio companion on another track would keep
    // playing over the muted picture, so it is removed too.
    const backgroundGroups = new Set(backgroundPool.map((item) => item.linkedGroupId).filter(Boolean))
    removeIds.push(...template.items
      .filter((item) => item.trackId !== backgroundTrack.id && !!item.linkedGroupId && backgroundGroups.has(item.linkedGroupId))
      .map((item) => item.id))
    // The incoming clip's head covers the transition's left portion, the outgoing tail its right one.
    const portions = calculateTransitionPortions(backgroundTransition.durationInFrames, backgroundTransition.alignment ?? 0.5)
    const windows = backgroundPool.map((clip) => {
      // At changed speed a short source still needs usable handles, not a
      // rejected full-second transition. Leave the identity path unchanged.
      const clipPortions = appearance.speed === 1 ? portions : calculateTransitionPortions(
        Math.max(1, Math.min(backgroundTransition.durationInFrames, Math.floor(clip.durationInFrames * 0.4))),
        backgroundTransition.alignment ?? 0.5,
      )
      return joinReadyWindow(clip, clipPortions.leftPortion, clipPortions.rightPortion, fps)
    })
    const random = mulberry32(seed)
    let previousPoolIndex = -1
    let previousFrames = 0
    let previousClip: VideoItem | undefined
    let cursor = 0
    let index = 0
    while (cursor < D && backgroundPool.length > 0) {
      const order = shuffledIndexes(backgroundPool.length, random)
      if (backgroundPool.length > 1 && order[0] === previousPoolIndex) {
        // A cycle must not open with the clip the last one ended on.
        const swap = order[0]!
        order[0] = order[1]!
        order[1] = swap
      }
      for (const poolIndex of order) {
        if (cursor >= D) break
        const clip = backgroundPool[poolIndex]!
        const window = windows[poolIndex]
        const frames = Math.min(window?.frames ?? clip.durationInFrames, D - cursor)
        const id = `${prefix}-bg-${index}`
        const output = applyBackgroundAppearance({
          ...clip,
          ...(window ? {
            sourceStart: window.sourceStart,
            sourceEnd: window.sourceStart + (appearance.speed === 1
              ? timelineToSourceFrames(frames, window.speed, fps, window.sourceFps)
              : Math.max(1, Math.ceil(frames * window.speed * window.sourceFps / fps))),
          } : appearance.speed !== 1 && !clip.loop ? {
            sourceEnd: Math.min(clip.sourceEnd!, clip.sourceStart! + Math.max(1, Math.ceil(frames * clip.speed! * clip.sourceFps! / fps))),
          } : {}),
          id,
          trackId: backgroundTrack.id,
          from: cursor,
          durationInFrames: frames,
          originId: undefined,
          linkedGroupId: undefined,
          templateAnchor: undefined,
          transformParent: undefined,
          embeddedAudioMuted: true,
        }, appearance, { width, height })
        addItem(output)
        if (previousClip) backgroundJoins.push({
          leftId: previousClip.id, rightId: id, leftFrames: previousFrames, rightFrames: frames,
          ...(appearance.speed !== 1 ? { maxDuration: getMaxTransitionDurationForHandles(previousClip, output, backgroundTransition.alignment, fps) } : {}),
        })
        previousClip = output
        previousFrames = frames
        previousPoolIndex = poolIndex
        cursor += frames
        index += 1
      }
    }
  }

  /* ------------------------------ captions slot ------------------------------ */
  const captions = prepareCaptionData(input.cues, fps, D)
  const captionsTrack = slotTrack('captions')
  if (captionsTrack) {
    slotTrackIds.add(captionsTrack.id)
    removeIds.push(...template.items.filter((item) => item.trackId === captionsTrack.id).map((item) => item.id))
    if (captions.cues.length > 0) {
      const prototype = prototypeItem(template.items, captionsTrack.id, 'subtitle') as SubtitleSegmentItem | undefined
      const id = `${prefix}-captions`
      const style: Record<string, unknown> = prototype
        ? {
            ...prototypeCopy(prototype, ['type', 'cues', 'words', 'wordsEstimated', 'source', 'sourceLabel']),
            source: prototype.source?.type === 'subtitle-import' ? structuredClone(prototype.source) : { type: 'subtitle-import', fileName: 'captions.json', format: 'vtt', importedAt: 0 },
          }
        : {
            captionPreset: captions.estimated ? 'fade' : 'karaoke',
            color: '#ffffff',
            fontSize: height * 0.055,
            textAlign: 'center',
            verticalAlign: 'middle',
            backgroundColor: '#111827',
            transform: { x: 0, y: height * 0.3, width: width * 0.85, height: height * 0.2 },
            source: { type: 'subtitle-import', fileName: 'captions.json', format: 'vtt', importedAt: 0 },
          }
      addItem({
        ...style,
        id,
        type: 'subtitle',
        trackId: captionsTrack.id,
        from: 0,
        durationInFrames: D,
        label: prototype?.label ?? 'Phụ đề',
        cues: captions.cues,
        words: captions.words,
        wordsEstimated: captions.estimated,
      } as SubtitleSegmentItem)
      itemIds.captions = id
      commands.push({
        type: 'caption.importTimed',
        params: {
          itemId: id,
          data: { words: captions.words, estimated: captions.estimated, cues: captions.cues, durationInFrames: D },
        },
      })
    }
  }

  /* ---------------------------- source-audio slot ---------------------------- */
  const audioTrack = slotTrack('source-audio')
  const audioProto = audioTrack ? (prototypeItem(template.items, audioTrack.id, 'audio') as AudioItem | undefined) : undefined
  const audioSpan = Math.max(1, Math.min(input.audio.durationInFrames, D))
  let loudness: TemplateFillResult['loudness'] = null
  if (audioTrack) {
    slotTrackIds.add(audioTrack.id)
    removeIds.push(...template.items.filter((item) => item.trackId === audioTrack.id).map((item) => item.id))
    const id = `${prefix}-audio`
    const style = audioProto ? prototypeCopy(audioProto, ['type', 'offset', 'waveformData']) : { volume: 0 }
    // The slot's loudness target shifts the clone's clip gain; the measured
    // source loudness decides how far, clamped so extreme sources stay sane.
    if (audioTrack.templateLoudnessLufs !== undefined && input.audio.measuredLufs !== undefined && Number.isFinite(input.audio.measuredLufs)) {
      const appliedGainDb = Math.max(-12, Math.min(12, audioTrack.templateLoudnessLufs - input.audio.measuredLufs))
      style.volume = (audioProto?.volume ?? 0) + appliedGainDb
      loudness = { targetLufs: audioTrack.templateLoudnessLufs, appliedGainDb }
    }
    addItem({
      ...style,
      id,
      type: 'audio',
      trackId: audioTrack.id,
      from: 0,
      durationInFrames: audioSpan,
      label: audioProto?.label ?? 'Âm thanh gốc',
      mediaId: input.audio.mediaId,
      src: input.audio.src,
      sourceStart: 0,
      sourceEnd: input.audio.durationInFrames,
      sourceDuration: input.audio.durationInFrames,
      sourceFps: fps,
    } as AudioItem)
  }

  /* ----------------------------- waveform rebind ----------------------------- */
  // Items bound to the template audio's media follow the new source audio,
  // spanning [0, min(D, audio)); waveforms of other media keep their anchor.
  const wavebound = new Set(
    audioProto?.mediaId
      ? template.items.filter((item) => item.type === 'shape' && item.waveformVisualizer && item.mediaId === audioProto.mediaId).map((item) => item.id)
      : [],
  )
  for (const item of template.items) {
    if (!wavebound.has(item.id)) continue
    commands.push({ type: 'waveform.setEnvelope', params: { itemId: item.id, mediaId: input.audio.mediaId, envelope: input.envelope } })
    commands.push({ type: 'item.update', params: { id: item.id, updates: { from: 0, durationInFrames: audioSpan } } })
    if (!itemIds.waveform) itemIds.waveform = item.id
  }

  /* --------------------------------- anchors --------------------------------- */
  const T = Math.max(0, ...template.items.map((item) => item.from + item.durationInFrames))
  const slotItems = new Set(template.items.filter((item) => slotTrackIds.has(item.trackId)).map((item) => item.id))
  // Slot removals also cover non-track items (a pool clip's linked audio
  // companion), so anchor re-timing must skip them too.
  const removedItems = new Set(removeIds)
  const dropIds: string[] = []
  for (const item of template.items) {
    if (slotItems.has(item.id) || removedItems.has(item.id) || wavebound.has(item.id)) continue
    const anchor = item.templateAnchor ?? 'start'
    if (anchor === 'full') {
      commands.push({ type: 'item.update', params: { id: item.id, updates: { from: 0, durationInFrames: D, ...(item.type === 'video' ? { loop: true } : {}) } } })
      continue
    }
    if (anchor === 'end') {
      const from = D - (T - item.from)
      if (from < 0) { dropIds.push(item.id); continue }
      commands.push({ type: 'item.update', params: { id: item.id, updates: { from } } })
      continue
    }
    // 'start' (default): keep the offset, clamp the end to D, drop what starts after it.
    if (item.from >= D) { dropIds.push(item.id); continue }
    if (item.from + item.durationInFrames > D) {
      commands.push({ type: 'item.update', params: { id: item.id, updates: { durationInFrames: D - item.from } } })
    }
  }
  const subscribeGroupIds = new Set(template.items
    .filter(item => item.type === 'text' && !!item.linkedGroupId && /subscribe|đăng ký/i.test(item.text))
    .map(item => item.linkedGroupId))
  const dropped = new Set(dropIds)
  itemIds.subscribe = template.items
    .filter(item => !!item.linkedGroupId && subscribeGroupIds.has(item.linkedGroupId) && !dropped.has(item.id) && !slotItems.has(item.id))
    .map(item => item.id)

  /* --------------------------- scene transitions --------------------------- */
  if (scenesTrack) {
    const prototypeIds = new Set(template.items.filter(item => item.trackId === scenesTrack.id && item.type === 'image').map(item => item.id))
    const prototypeTransition = template.transitions?.find(transition =>
      transition.trackId === scenesTrack.id && prototypeIds.has(transition.leftClipId) && prototypeIds.has(transition.rightClipId))
    if (prototypeTransition) {
      for (let index = 0; index < input.scenes.length - 1; index++) {
        const left = input.scenes[index]!
        const right = input.scenes[index + 1]!
        if (left.endFrame !== right.startFrame) continue
        const maximum = Math.floor(Math.min(left.endFrame - left.startFrame, right.endFrame - right.startFrame) * 0.4)
        if (maximum < 1) continue
        commands.push({ type: 'transition.add', params: {
          leftClipId: itemIds.scenes[index], rightClipId: itemIds.scenes[index + 1],
          presentation: prototypeTransition.presentation, timing: prototypeTransition.timing,
          alignment: prototypeTransition.alignment ?? 0.5, direction: prototypeTransition.direction,
          durationInFrames: Math.min(prototypeTransition.durationInFrames, maximum),
        } })
      }
    }
  }

  /* ------------------------ background transitions ------------------------- */
  if (backgroundTrack) {
    for (const join of backgroundJoins) {
      const maximum = Math.min(Math.floor(Math.min(join.leftFrames, join.rightFrames) * 0.4), join.maxDuration ?? Infinity)
      if (maximum < 1) continue
      commands.push({ type: 'transition.add', params: {
        leftClipId: join.leftId, rightClipId: join.rightId,
        presentation: backgroundTransition.presentation, timing: backgroundTransition.timing,
        alignment: backgroundTransition.alignment ?? 0.5, direction: backgroundTransition.direction,
        durationInFrames: Math.min(backgroundTransition.durationInFrames, maximum),
      } })
    }
  }

  // Removals lead the batch so re-added clips never collide with replaced ones.
  if (removeIds.length + dropIds.length > 0) {
    commands.unshift({ type: 'item.removeExact', params: { itemIds: [...removeIds, ...dropIds] } })
  }
  // Open authored track locks for the fill, then retain them on the clone.
  const lockedTracks = template.tracks.filter(track => track.locked)
  commands.unshift(...lockedTracks.map(track => ({ type: 'track.update', params: { id: track.id, updates: { locked: false } } })))
  commands.push(...lockedTracks.map(track => ({ type: 'track.update', params: { id: track.id, updates: { locked: true } } })))

  return {
    commands,
    itemIds,
    captions: { cueCount: captions.cues.length, wordsEstimated: captions.estimated || captions.pages.some(page => page.estimated) },
    loudness,
  }
}
