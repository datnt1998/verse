import { z } from 'zod'
import type { ShapeItem } from '@editor/types/timeline'
import { DEFAULT_WAVEFORM_STYLE, waveformEnvelopeSchema, waveformStyleSchema } from '@editor/features/waveform-visualizer/types'
import { registerCommand } from '../registry'
import { executeCommands } from '../executor'
import type { CommandIssue, ProjectStateView } from '../types'

const id = z.string().min(1)
function waveformTarget(state: ProjectStateView, itemId: string): CommandIssue[] {
  const item = state.items.find(item => item.id === itemId)
  return item?.type === 'shape' && item.waveformVisualizer ? [] : [{ path: 'itemId', message: 'Không tìm thấy mục Sóng giọng' }]
}
function sourceIssues(state: ProjectStateView, mediaId: string, sourceHash: string): CommandIssue[] {
  const media = state.mediaItems?.find(media => media.id === mediaId)
  if (!media || !media.mimeType.startsWith('audio/')) return [{ path: 'mediaId', message: 'Chọn tệp âm thanh làm nguồn giọng' }]
  return media.contentHash === sourceHash ? [] : [{ path: 'sourceHash', message: 'Dữ liệu sóng giọng không khớp hash nguồn hiện tại' }]
}

export function registerWaveformCommands(): void {
  registerCommand({
    type: 'waveform.add',
    params: z.object({ id: id.optional(), trackId: id, from: z.number().int().nonnegative(), durationInFrames: z.number().int().positive(), mediaId: id, envelope: waveformEnvelopeSchema, style: waveformStyleSchema.partial().optional(), transform: z.object({ x: z.number().finite().optional(), y: z.number().finite().optional(), width: z.number().positive().optional(), height: z.number().positive().optional() }).strict().optional() }).strict(),
    validate: (state, params) => sourceIssues(state, params.mediaId, params.envelope.sourceHash),
    apply(ctx, params) {
      const item: ShapeItem = {
        id: params.id ?? ctx.ids.mint(), type: 'shape', shapeType: 'rectangle', label: 'Sóng giọng', trackId: params.trackId, from: params.from, durationInFrames: params.durationInFrames, mediaId: params.mediaId,
        fillColor: params.style?.color ?? DEFAULT_WAVEFORM_STYLE.color,
        sourceStart: 0, sourceEnd: params.durationInFrames, sourceFps: ctx.draft.fps, sourceDuration: params.envelope.durationSeconds * ctx.draft.fps,
        transform: { width: 960, height: 160, x: 0, y: 360, ...params.transform },
        waveformVisualizer: { version: 1, ...DEFAULT_WAVEFORM_STYLE, ...params.style, ...params.envelope },
      }
      const result = executeCommands([{ type: 'item.add', params: { item } }])
      if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
      return { inverse: { kind: 'patch' as const, patch: {} }, result: item.id }
    },
  })
  registerCommand({
    type: 'waveform.setStyle', params: z.object({ itemId: id, style: waveformStyleSchema.partial() }).strict(),
    validate: (state, params) => waveformTarget(state, params.itemId),
    mergeKey: params => `waveform.setStyle:${params.itemId}:${Object.keys(params.style).sort().join(',')}`,
    apply(ctx, params) {
      ctx.draft.items = ctx.draft.items.map(item => item.id === params.itemId && item.type === 'shape' && item.waveformVisualizer ? { ...item, waveformVisualizer: { ...item.waveformVisualizer, ...params.style } } : item)
      return { kind: 'patch' as const, patch: {} }
    },
  })
  registerCommand({
    type: 'waveform.setEnvelope', params: z.object({ itemId: id, mediaId: id.optional(), envelope: waveformEnvelopeSchema }).strict(),
    validate(state, params) {
      const item = state.items.find(item => item.id === params.itemId)
      return [...waveformTarget(state, params.itemId), ...sourceIssues(state, params.mediaId ?? item?.mediaId ?? '', params.envelope.sourceHash)]
    },
    apply(ctx, params) {
      ctx.draft.items = ctx.draft.items.map(item => item.id === params.itemId && item.type === 'shape' && item.waveformVisualizer ? { ...item, mediaId: params.mediaId ?? item.mediaId, sourceDuration: params.envelope.durationSeconds * (item.sourceFps ?? ctx.draft.fps), waveformVisualizer: { ...item.waveformVisualizer, ...params.envelope } } : item)
      return { kind: 'patch' as const, patch: {} }
    },
  })
}
