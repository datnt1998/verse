import type { SerializedCommand } from '@editor/commands/types'
import { getMediaType } from '@editor/features/media-library/utils/validation'
import { getGpuEffectDefaultParams } from '@editor/infrastructure/gpu-effects/registry'
import type { MediaMetadata } from '@editor/types/storage'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'

export interface OverlayCommandInput {
  media: MediaMetadata
  /** Resolved source URL of the media (the item's `src`). */
  src: string
  /** Timeline frame rate. */
  fps: number
  tracks: readonly TimelineTrack[]
  items: readonly Pick<TimelineItem, 'from' | 'durationInFrames'>[]
  createId?: () => string
}

/**
 * One-click overlay insert: drops the media on a fresh top video track at
 * frame 0, covering the timeline from its first frame to the content tail
 * (never extending the project). Every overlay is anchored to the whole video
 * (`templateAnchor: 'full'`), so a clone fill re-spans it to the new length.
 * Detected black backgrounds composite via a screen blend; detected greens get
 * a chroma key preset to the detected colour. Video clips are picture-only
 * (muted, no linked audio) and loop when shorter than the span; image media
 * spans as a still.
 *
 * A new video track is inserted first (order 0): the composition paints tracks
 * by descending order (order 0 is the top layer), so a track appended at the
 * end would sit under the image layers and the overlay would be invisible.
 */
export function buildOverlayCommands(input: OverlayCommandInput): { commands: SerializedCommand[]; itemId: string } {
  const { media, src, fps, createId = () => crypto.randomUUID() } = input
  const type = getMediaType(media.mimeType)
  if (type !== 'image' && type !== 'video') throw new Error('Lớp phủ cần là video hoặc ảnh')
  const sourceFps = media.fps || fps
  const sourceFrames = Math.round(media.duration * sourceFps)
  const mediaTimelineFrames = Math.max(1, Math.round((type === 'image' ? 5 : media.duration) * fps))
  const contentEnd = input.items.reduce((end, item) => Math.max(end, item.from + item.durationInFrames), 0)
  const span = Math.max(1, contentEnd > 0 ? contentEnd : mediaTimelineFrames)
  const sourceWindow = Math.max(1, Math.round((span / fps) * sourceFps))
  const sourceEnd = Math.min(sourceWindow, sourceFrames)
  const created: TimelineTrack = { id: createId(), name: `Lớp phủ: ${media.fileName}`, kind: 'video', height: 56, locked: false, visible: true, muted: false, solo: false, order: 0, items: [] }
  const tracks = [created, ...input.tracks.map(track => ({ ...track, order: track.order + 1 }))]
  const greenKey = media.background?.kind === 'green' ? media.background.keyColor : undefined
  const screenBlend = media.background?.kind === 'black'
  const item: TimelineItem = {
    id: createId(),
    type,
    mediaId: media.id,
    label: media.fileName,
    from: 0,
    durationInFrames: span,
    trackId: created.id,
    src,
    sourceWidth: media.width,
    sourceHeight: media.height,
    sourceFps,
    sourceDuration: sourceFrames,
    sourceStart: 0,
    ...(screenBlend ? { blendMode: 'screen' as const } : {}),
    templateAnchor: 'full',
    ...(type === 'video'
      ? { sourceEnd, embeddedAudioMuted: true, ...(mediaTimelineFrames < span ? { loop: true as const } : {}) }
      : {}),
    ...(greenKey ? { effects: [{ id: createId(), effect: { type: 'gpu-effect' as const, gpuEffectType: 'gpu-chroma-key', params: { ...getGpuEffectDefaultParams('gpu-chroma-key'), keyColor: greenKey } }, enabled: true }] } : {}),
  }
  return { commands: [{ type: 'item.addOnNewTracks', params: { items: [item], tracks } }], itemId: item.id }
}
