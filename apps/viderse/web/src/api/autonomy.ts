// Client Mức tự chủ + điểm ảnh (data-contract §20) — khớp internal/web/channels.go:
//   GET/POST/PATCH /api/owner-channels[/:id]  mang  autonomy + priority
//   PATCH {autonomy?, priority?, confirmed?}  → Channel
//        409 {code:'autonomy_confirm_required'} khi lên Tự chạy / Tự duyệt hết mà thiếu confirmed
//        400 fields[{field:'autonomy.approve.min'|…}]
//   GET /api/clone/:id/images  items[].score + autonomy (tóm tắt điểm của video)
// Mọi phản hồi qua zod: thiếu/sai hình dạng thì ném lỗi, UI hiện trạng thái lỗi
// thay vì một Mức tự chủ bịa.
import { z } from 'zod'
import { ApiError, mutate } from './request'
import type { Channel } from './channels'

export const pickModes = ['suggest', 'score2', 'all'] as const
export const approveModes = ['ask', 'score', 'all'] as const
export const errorModes = ['stop', 'retry_once'] as const
export const qualityModes = ['ask', 'fix_then_ask'] as const
export const presetIds = ['careful', 'balanced', 'auto'] as const

export const APPROVE_MIN = 70
export const APPROVE_MAX = 95
export const APPROVE_STEP = 5
export const APPROVE_DEFAULT = 85

export const autonomySchema = z.object({
  preset: z.enum(['careful', 'balanced', 'auto', 'custom']),
  pick: z.enum(pickModes),
  approve: z.object({
    mode: z.enum(approveModes),
    min: z.number().int().min(APPROVE_MIN).max(APPROVE_MAX),
  }),
  onError: z.enum(errorModes),
  belowQuality: z.enum(qualityModes),
})
export type Autonomy = z.infer<typeof autonomySchema>
export type AutonomyRow = 'pick' | 'approve' | 'onError' | 'belowQuality'
export type PresetId = (typeof presetIds)[number]

export const prioritySchema = z.enum(['steady', 'views'])
export type Priority = z.infer<typeof prioritySchema>

/** Ba mức có sẵn (ia-flows §9.4); ngưỡng điểm giữ nguyên khi đổi mức. */
export const PRESETS: Record<PresetId, Omit<Autonomy, 'preset' | 'approve'> & { approve: Autonomy['approve']['mode'] }> = {
  careful: { pick: 'suggest', approve: 'ask', onError: 'stop', belowQuality: 'ask' },
  balanced: { pick: 'all', approve: 'ask', onError: 'retry_once', belowQuality: 'ask' },
  auto: { pick: 'all', approve: 'score', onError: 'retry_once', belowQuality: 'fix_then_ask' },
}

export const DEFAULT_AUTONOMY: Autonomy = {
  preset: 'balanced',
  pick: 'all',
  approve: { mode: 'ask', min: APPROVE_DEFAULT },
  onError: 'retry_once',
  belowQuality: 'ask',
}

/** Mức hiện tại suy từ 4 dòng chọn (không tính ngưỡng) — khác ba mức sẵn thì Tuỳ chỉnh. */
export function presetOf(value: Pick<Autonomy, 'pick' | 'onError' | 'belowQuality'> & { approve: { mode: string } }): Autonomy['preset'] {
  for (const id of presetIds) {
    const preset = PRESETS[id]
    if (preset.pick === value.pick && preset.approve === value.approve.mode && preset.onError === value.onError && preset.belowQuality === value.belowQuality) return id
  }
  return 'custom'
}

export function applyPreset(current: Autonomy, id: PresetId): Autonomy {
  const preset = PRESETS[id]
  return { preset: id, pick: preset.pick, approve: { mode: preset.approve, min: current.approve.min }, onError: preset.onError, belowQuality: preset.belowQuality }
}

/** Đổi một dòng; `preset` tính lại để UI hiện Tuỳ chỉnh ngay, server vẫn tự suy lại. */
export function withRow<K extends AutonomyRow>(current: Autonomy, row: K, value: K extends 'approve' ? Autonomy['approve']['mode'] : Autonomy[K]): Autonomy {
  const next: Autonomy = row === 'approve'
    ? { ...current, approve: { ...current.approve, mode: value as Autonomy['approve']['mode'] } }
    : { ...current, [row]: value }
  return { ...next, preset: presetOf(next) }
}

export const withThreshold = (current: Autonomy, min: number): Autonomy => ({ ...current, approve: { ...current.approve, min } })

/** Việc cần hỏi trước khi lưu: lên Tự chạy hoặc Tự duyệt hết (không hỏi khi đang ở đó rồi). */
export function confirmKind(current: Autonomy, next: Autonomy): 'auto' | 'all' | null {
  if (next.approve.mode === 'all' && current.approve.mode !== 'all') return 'all'
  if (presetOf(next) === 'auto' && presetOf(current) !== 'auto') return 'auto'
  return null
}

const channelAutonomySchema = z.object({ autonomy: autonomySchema, priority: prioritySchema })
export type ChannelAutonomy = z.infer<typeof channelAutonomySchema>

/** Đọc Mức tự chủ + Ưu tiên từ kênh; null = server chưa trả hoặc sai hình dạng (hiện trạng thái lỗi). */
export function readChannelAutonomy(channel: Pick<Channel, 'autonomy' | 'priority'>): ChannelAutonomy | null {
  const parsed = channelAutonomySchema.safeParse({ autonomy: channel.autonomy, priority: channel.priority })
  return parsed.success ? parsed.data : null
}

export type AutonomyPatch = { autonomy?: Autonomy; priority?: Priority; confirmed?: boolean }

/** PATCH Mức tự chủ / Ưu tiên; trả kênh mới (đã kiểm hình dạng ở hai trường này). */
export async function patchAutonomy(channelId: string, patch: AutonomyPatch): Promise<Channel> {
  const channel = await mutate<Channel>('PATCH', `/api/owner-channels/${encodeURIComponent(channelId)}`, patch)
  if (!readChannelAutonomy(channel)) throw new ApiError(0, 'Máy chủ trả Mức tự chủ sai hình dạng.')
  return channel
}

const confirmRequiredSchema = z.object({ code: z.literal('autonomy_confirm_required') })

/** 409 `autonomy_confirm_required`: client thiếu bước hỏi — chỉ xảy ra khi bản server mới hơn UI. */
export const isConfirmRequired = (error: unknown): boolean =>
  error instanceof ApiError && error.status === 409 && confirmRequiredSchema.safeParse(error.payload).success

export const reasonFactSchema = z.object({ label: z.string(), value: z.string(), source: z.string() })
export const imageScoreSchema = z.object({
  value: z.number().int().min(0).max(100),
  parts: z.object({ match: z.number(), artifacts: z.number(), identity: z.number(), duplicate: z.number() }),
  reason: z.object({ agent: z.literal('qa'), text: z.string(), facts: z.array(reasonFactSchema) }),
})
export type ImageScore = z.infer<typeof imageScoreSchema>

/** Tóm tắt điểm cả video do server tính (GET …/images › autonomy). */
export const scoreSummarySchema = z.object({
  mode: z.enum(approveModes),
  min: z.number(),
  total: z.number().int(),
  passed: z.number().int(),
  pending: z.number().int(),
  below: z.array(z.object({ id: z.string(), label: z.string(), value: z.number() })),
}).nullable()
export type ScoreSummary = z.infer<typeof scoreSummarySchema>

/** Đọc `score` của một ảnh; thiếu/null/sai hình dạng → null (không hiện số bịa). */
export function readImageScore(value: unknown): ImageScore | null {
  const parsed = imageScoreSchema.safeParse(value)
  return parsed.success ? parsed.data : null
}

export function readScoreSummary(value: unknown): ScoreSummary {
  const parsed = scoreSummarySchema.safeParse(value ?? null)
  return parsed.success ? parsed.data : null
}
