import { z } from 'zod'
import { appendVirtualTranscriptCaptionTrack } from '@editor/features/media-library/utils/caption-items'
import type { TimelineTrack } from '@editor/types/timeline'
import { resolveEffectiveTrackStates } from '@editor/features/timeline/utils/group-utils'
import { resolveReverseConformedVideoItem } from '@editor/shared/utils/reverse-conform-item'

const sourceSchema = z.object({
  type: z.enum(['transcript', 'embedded-subtitles', 'subtitle-import']),
  clipId: z.string().optional(),
}).passthrough().superRefine((source, context) => {
  if ((source.type === 'transcript' || source.type === 'embedded-subtitles') && !source.clipId) {
    context.addIssue({ code: 'custom', message: 'Clip-bound subtitle source requires clipId' })
  }
})

const renderItemSchema = z.object({
  id: z.string(),
  type: z.string(),
  trackId: z.string(),
  from: z.number().finite(),
  durationInFrames: z.number().finite().nonnegative(),
  mediaId: z.string().optional(),
  isReversed: z.boolean().optional(),
  source: sourceSchema.optional(),
  captionSource: z.object({ clipId: z.string().optional() }).passthrough().optional(),
  sourceStart: z.number().finite().optional(),
  sourceEnd: z.number().finite().optional(),
  sourceDuration: z.number().finite().optional(),
  sourceFps: z.number().finite().positive().optional(),
  trimStart: z.number().finite().optional(),
  trimEnd: z.number().finite().optional(),
  speed: z.number().finite().positive().optional(),
  transcriptCaptions: z.object({
    type: z.literal('transcript'),
    mediaId: z.string(),
    enabled: z.boolean(),
    cues: z.array(z.object({
      id: z.string(),
      startSeconds: z.number().finite(),
      endSeconds: z.number().finite(),
      text: z.string(),
    }).passthrough()),
    style: z.object({}).passthrough().optional(),
  }).passthrough().optional(),
}).passthrough().superRefine((item, context) => {
  if (item.type === 'subtitle' && !item.source) {
    context.addIssue({ code: 'custom', message: 'Subtitle requires a source' })
  }
})

const renderTimelineSchema = z.object({
  tracks: z.array(z.object({
    id: z.string(),
    solo: z.boolean().optional(),
    visible: z.boolean().optional(),
    order: z.number().finite().optional(),
  }).passthrough()),
  items: z.array(renderItemSchema),
})
const renderCanvasSchema = z.object({
  width: z.number().finite().positive(),
  height: z.number().finite().positive(),
  fps: z.number().finite().positive(),
})

/** Shared headless render identities, including virtual transcripts and reverse-source suppression. */
export function renderCaptionTracks(
  timeline: { tracks: unknown; items: unknown },
  canvas: { width: number; height: number; fps: number },
  context: 'root' | 'nested' = 'root',
): TimelineTrack[] {
  const parsed = renderTimelineSchema.parse(timeline)
  const settings = renderCanvasSchema.parse(canvas)
  // The existing builder only consumes the fields validated above; passthrough
  // retains the rest of the saved TimelineTrack/TimelineItem render properties.
  let tracks = parsed.tracks.map(track => ({
    ...track,
    items: parsed.items.filter(item => item.trackId === track.id),
  })) as unknown as TimelineTrack[]
  if (context === 'root') {
    tracks = resolveEffectiveTrackStates(tracks).map(track => ({
      ...track,
      items: track.items.map(item => item.type === 'video'
        ? resolveReverseConformedVideoItem(item, settings.fps, { mode: 'export' })
        : item),
    }))
  }
  return appendVirtualTranscriptCaptionTrack(tracks, settings.fps, settings.width, settings.height)
}
