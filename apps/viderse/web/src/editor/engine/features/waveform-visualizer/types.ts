import { z } from 'zod'

export const ENVELOPE_RATE = 50
export const MAX_ENVELOPE_SECONDS = 24 * 60 * 60
export const waveformStyleSchema = z.object({
  windowSeconds: z.number().finite().min(0.1).max(10),
  bars: z.number().int().min(4).max(256),
  amplitude: z.number().finite().min(0).max(2),
  color: z.string().regex(/^#[a-fA-F0-9]{6}$/),
  strokeWidth: z.number().finite().min(0.5).max(20),
  baseline: z.number().finite().min(0).max(1),
  visible: z.boolean(),
}).strict()
export const waveformEnvelopeSchema = z.object({
  sourceHash: z.string().regex(/^sha256:[a-f0-9]{64}$/),
  samplesPerSecond: z.literal(ENVELOPE_RATE),
  durationSeconds: z.number().finite().positive().max(MAX_ENVELOPE_SECONDS),
  envelopeBase64: z.string().max(Math.ceil(MAX_ENVELOPE_SECONDS * ENVELOPE_RATE / 3) * 4).regex(/^[A-Za-z0-9+/]*={0,2}$/).refine(value => value.length % 4 === 0, 'Base64 cần đủ nhóm bốn ký tự'),
  normalization: z.number().finite().nonnegative(),
}).strict().superRefine((value, ctx) => {
  const padding = value.envelopeBase64.endsWith('==') ? 2 : value.envelopeBase64.endsWith('=') ? 1 : 0
  const bytes = value.envelopeBase64.length / 4 * 3 - padding
  if (bytes !== Math.ceil(value.durationSeconds * ENVELOPE_RATE - 1e-8)) ctx.addIssue({ code: 'custom', path: ['envelopeBase64'], message: 'Số mẫu sóng không khớp thời lượng nguồn' })
})
export const waveformVisualizerSchema = waveformStyleSchema.extend({
  version: z.literal(1),
  sourceHash: waveformEnvelopeSchema.shape.sourceHash,
  samplesPerSecond: z.literal(ENVELOPE_RATE),
  durationSeconds: waveformEnvelopeSchema.shape.durationSeconds.optional(),
  envelopeBase64: waveformEnvelopeSchema.shape.envelopeBase64.optional(),
  normalization: waveformEnvelopeSchema.shape.normalization.optional(),
}).superRefine((value, ctx) => {
  if (value.envelopeBase64 !== undefined || value.durationSeconds !== undefined || value.normalization !== undefined) {
    const result = waveformEnvelopeSchema.safeParse({ sourceHash: value.sourceHash, samplesPerSecond: value.samplesPerSecond, durationSeconds: value.durationSeconds, envelopeBase64: value.envelopeBase64, normalization: value.normalization })
    if (!result.success) for (const issue of result.error.issues) ctx.addIssue({ code: 'custom', path: issue.path, message: issue.message })
  }
})
export type WaveformStyle = z.infer<typeof waveformStyleSchema>
export type WaveformEnvelope = z.infer<typeof waveformEnvelopeSchema>
export type WaveformVisualizer = z.infer<typeof waveformVisualizerSchema>
export const DEFAULT_WAVEFORM_STYLE: WaveformStyle = { windowSeconds: 1, bars: 64, amplitude: 1, color: '#ffffff', strokeWidth: 3, baseline: 0.5, visible: true }

/** Also validates raw item commands, which otherwise accept loose JSON payloads. */
export function waveformItemIssue(item: Record<string, unknown>, currentHash?: string): string | null {
  if (item.waveformVisualizer === undefined) return null
  if (item.type !== 'shape' || item.shapeType !== 'rectangle' || typeof item.mediaId !== 'string' || !item.mediaId || item.isMask) return 'Sóng giọng cần shape chữ nhật, nguồn giọng và không được làm mask'
  const result = waveformVisualizerSchema.safeParse(item.waveformVisualizer)
  if (!result.success) return `Dữ liệu sóng giọng không hợp lệ: ${result.error.issues[0]?.message}`
  if (currentHash !== undefined && result.data.sourceHash !== currentHash && result.data.envelopeBase64 !== undefined) return 'Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại'
  return null
}
