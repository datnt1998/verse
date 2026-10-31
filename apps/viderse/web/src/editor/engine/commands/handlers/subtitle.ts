import { z } from 'zod'
import { parseSrt, parseVtt } from '@editor/shared/utils/subtitles'
import { buildCaptionTrack, buildSubtitleTextItems } from '@editor/features/media-library/utils/caption-items'
import { placeItemsWithoutTimelineOverlap } from '@editor/features/timeline/stores/actions/item-placement'
import { normalizeFrameFields } from '@editor/features/timeline/stores/items-store-normalize'
import { textStyleSchema } from './text'
import { registerCommand } from '../registry'
import type { CommandIssue, Inverse, ProjectStateView } from '../types'

const id = z.string().min(1)
const number = z.number().finite()
export const subtitleCueSchema = z.object({
  id, startSeconds: number.nonnegative(), endSeconds: number.nonnegative(), text: z.string(),
}).strict().refine((cue) => cue.endSeconds > cue.startSeconds, { message: 'Thời điểm kết thúc phải sau thời điểm bắt đầu', path: ['endSeconds'] })
const inverse = (): Inverse => ({ kind: 'patch', patch: {} })
function validateSubtitle(state: ProjectStateView, params: { itemId: string }): CommandIssue[] {
  const item = state.items.find((candidate) => candidate.id === params.itemId)
  return !item ? [{ message: `Không tìm thấy phụ đề ${params.itemId}`, path: 'itemId' }]
    : item.type !== 'subtitle' ? [{ message: 'Clip không phải đoạn phụ đề', path: 'itemId' }] : []
}

export function registerSubtitleCommands(): void {
  registerCommand({
    type: 'subtitle.import',
    params: z.object({
      trackId: id, fileName: z.string().min(1), format: z.enum(['srt', 'vtt']), content: z.string(),
      startFrame: number.nonnegative().default(0), importedAt: number.default(0),
      createTrack: z.boolean().default(false), style: textStyleSchema.optional(),
    }).strict(),
    validate(state, params) {
      const issues: CommandIssue[] = []
      const track = state.tracks.find((candidate) => candidate.id === params.trackId)
      if (!track && !params.createTrack) issues.push({ message: `Không tìm thấy rãnh ${params.trackId}`, path: 'trackId' })
      if (track?.locked || track?.isGroup || track?.kind === 'audio') issues.push({ message: 'Rãnh này không nhận phụ đề', path: 'trackId' })
      const parsed = params.format === 'srt' ? parseSrt(params.content) : parseVtt(params.content)
      if (!parsed.cues.length) issues.push({ message: 'Tệp không có phụ đề hợp lệ', path: 'content' })
      return issues
    },
    apply(ctx, params) {
      const parsed = params.format === 'srt' ? parseSrt(params.content) : parseVtt(params.content)
      if (!ctx.draft.tracks.some((track) => track.id === params.trackId)) {
        ctx.draft.tracks = [...ctx.draft.tracks, { ...buildCaptionTrack(ctx.draft.tracks), id: params.trackId }]
      }
      const items = buildSubtitleTextItems({
        trackId: params.trackId, cues: parsed.cues, timelineFps: ctx.draft.fps,
        canvasWidth: ctx.draft.projectMetadata?.width ?? 1920, canvasHeight: ctx.draft.projectMetadata?.height ?? 1080,
        fileName: params.fileName, format: params.format, startFrame: params.startFrame,
        styleTemplate: params.style ? { ...params.style, color: params.style.color ?? '#ffffff' } : undefined,
        idFactory: () => ctx.ids.mint(), importedAt: params.importedAt,
      })
      ctx.draft.items = [...ctx.draft.items, ...placeItemsWithoutTimelineOverlap(items).map(normalizeFrameFields)]
      return { inverse: inverse(), result: { itemIds: items.map((item) => item.id), warnings: parsed.warnings } }
    },
  })
  registerCommand({
    type: 'subtitle.setCues',
    params: z.object({ itemId: id, cues: z.array(subtitleCueSchema) }).strict(),
    validate(state, params) {
      const issues = validateSubtitle(state, params)
      if (new Set(params.cues.map((cue) => cue.id)).size !== params.cues.length) issues.push({ message: 'Mã câu phụ đề bị trùng', path: 'cues' })
      return issues
    },
    apply(ctx, params) {
      ctx.draft.items = ctx.draft.items.map((item) => item.id === params.itemId && item.type === 'subtitle' ? { ...item, cues: params.cues } : item)
      return inverse()
    },
  })
  registerCommand({
    type: 'subtitle.updateCue',
    params: z.object({ itemId: id, cueId: id, updates: z.object({ startSeconds: number.nonnegative().optional(), endSeconds: number.nonnegative().optional(), text: z.string().optional() }).strict() }).strict(),
    mergeKey: (params) => `subtitle:${params.itemId}:${params.cueId}:${Object.keys(params.updates).sort().join(',')}`,
    validate(state, params) {
      const issues = validateSubtitle(state, params)
      if (issues.length) return issues
      const item = state.items.find((candidate) => candidate.id === params.itemId)
      const cue = item?.type === 'subtitle' ? item.cues.find((candidate) => candidate.id === params.cueId) : undefined
      if (!cue) return [{ message: `Không tìm thấy câu phụ đề ${params.cueId}`, path: 'cueId' }]
      return subtitleCueSchema.safeParse({ ...cue, ...params.updates }).success ? [] : [{ message: 'Thời điểm kết thúc phải sau thời điểm bắt đầu', path: 'updates' }]
    },
    apply(ctx, params) {
      ctx.draft.items = ctx.draft.items.map((item) => item.id === params.itemId && item.type === 'subtitle'
        ? { ...item, cues: item.cues.map((cue) => cue.id === params.cueId ? { ...cue, ...params.updates } : cue) } : item)
      return inverse()
    },
  })
  registerCommand({
    type: 'subtitle.setStyle', params: z.object({ itemId: id, style: textStyleSchema }).strict(), validate: (state, params) => validateSubtitle(state, params),
    mergeKey: (params) => `subtitle:style:${params.itemId}:${Object.keys(params.style).sort().join(',')}`,
    apply(ctx, params) {
      ctx.draft.items = ctx.draft.items.map((item) => item.id === params.itemId && item.type === 'subtitle' ? normalizeFrameFields({ ...item, ...params.style }) : item)
      return inverse()
    },
  })
}
