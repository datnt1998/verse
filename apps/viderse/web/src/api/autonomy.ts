// Client Mức tự chủ + điểm ảnh (data-contract §20) — khớp internal/web/channels.go:
//   GET/POST/PATCH /api/owner-channels[/:id]  mang  autonomy + priority
//   PATCH {autonomy?, priority?, confirmed?}  → Channel
//        409 {code:'autonomy_confirm_required'} khi lên Tự chạy / Tự duyệt hết mà thiếu confirmed
//        400 fields[{field:'autonomy.approve.min'|…}]
//   GET /api/clone/:id/images  items[].score + autonomy (tóm tắt điểm của video)
// Mọi phản hồi qua guard tay (./guard): thiếu/sai hình dạng thì ném lỗi, UI hiện
// trạng thái lỗi thay vì một Mức tự chủ bịa. Không dùng zod vì thẻ Mức tự chủ
// nằm trong chunk của màn đầu (Bàn dựng).
import { list, num, oneOf, record, str, ShapeError } from './guard'
import { ApiError, mutate } from './request'
import type { Channel } from './channels'

export const pickModes = ['suggest', 'score2', 'all'] as const
export const approveModes = ['ask', 'score', 'all'] as const
export const errorModes = ['stop', 'retry_once'] as const
export const qualityModes = ['ask', 'fix_then_ask'] as const
export const presetIds = ['careful', 'balanced', 'auto'] as const
const prioritySchemaValues = ['steady', 'views'] as const
const presetSchemaValues = ['careful', 'balanced', 'auto', 'custom'] as const

export const APPROVE_MIN = 70
export const APPROVE_MAX = 95
export const APPROVE_STEP = 5
export const APPROVE_DEFAULT = 85

export type Autonomy = {
  preset: 'careful' | 'balanced' | 'auto' | 'custom'
  pick: (typeof pickModes)[number]
  approve: { mode: (typeof approveModes)[number]; min: number }
  onError: (typeof errorModes)[number]
  belowQuality: (typeof qualityModes)[number]
}
export type AutonomyRow = 'pick' | 'approve' | 'onError' | 'belowQuality'
export type PresetId = (typeof presetIds)[number]

export type Priority = (typeof prioritySchemaValues)[number]

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

const readAutonomy = (value: unknown, path: string): Autonomy => {
  const o = record(value, path)
  const approve = record(o.approve, `${path}.approve`)
  const min = num(approve.min, `${path}.approve.min`)
  if (min < APPROVE_MIN || min > APPROVE_MAX) throw new ShapeError(`${path}.approve.min`)
  return {
    preset: oneOf(o.preset, presetSchemaValues, `${path}.preset`),
    pick: oneOf(o.pick, pickModes, `${path}.pick`),
    approve: { mode: oneOf(approve.mode, approveModes, `${path}.approve.mode`), min },
    onError: oneOf(o.onError, errorModes, `${path}.onError`),
    belowQuality: oneOf(o.belowQuality, qualityModes, `${path}.belowQuality`),
  }
}

const readPriority = (value: unknown, path: string): Priority => oneOf(value, prioritySchemaValues, path)

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

export type ChannelAutonomy = { autonomy: Autonomy; priority: Priority }

/** Đọc Mức tự chủ + Ưu tiên từ kênh; null = server chưa trả hoặc sai hình dạng (hiện trạng thái lỗi). */
export function readChannelAutonomy(channel: Pick<Channel, 'autonomy' | 'priority'>): ChannelAutonomy | null {
  try {
    return { autonomy: readAutonomy(channel.autonomy, 'channel.autonomy'), priority: readPriority(channel.priority, 'channel.priority') }
  } catch {
    return null
  }
}

export type AutonomyPatch = { autonomy?: Autonomy; priority?: Priority; confirmed?: boolean }

/** PATCH Mức tự chủ / Ưu tiên; trả kênh mới (đã kiểm hình dạng ở hai trường này). */
export async function patchAutonomy(channelId: string, patch: AutonomyPatch): Promise<Channel> {
  const channel = await mutate<Channel>('PATCH', `/api/owner-channels/${encodeURIComponent(channelId)}`, patch)
  if (!readChannelAutonomy(channel)) throw new ApiError(0, 'Máy chủ trả Mức tự chủ sai hình dạng.')
  return channel
}

/** 409 `autonomy_confirm_required`: client thiếu bước hỏi — chỉ xảy ra khi bản server mới hơn UI. */
export const isConfirmRequired = (error: unknown): boolean => {
  if (!(error instanceof ApiError) || error.status !== 409) return false
  try {
    return record(error.payload, 'error').code === 'autonomy_confirm_required'
  } catch {
    return false
  }
}

export type ImageScore = {
  value: number
  parts: { match: number; artifacts: number; identity: number; duplicate: number }
  reason: { agent: 'qa'; text: string; facts: { label: string; value: string; source: string }[] }
}

/** Tóm tắt điểm cả video do server tính (GET …/images › autonomy). */
export type ScoreSummary = {
  mode: (typeof approveModes)[number]
  min: number
  total: number
  passed: number
  pending: number
  below: { id: string; label: string; value: number }[]
} | null

/** Đọc `score` của một ảnh; thiếu/null/sai hình dạng → null (không hiện số bịa). */
export function readImageScore(value: unknown): ImageScore | null {
  try {
    const o = record(value, 'score')
    const parts = record(o.parts, 'score.parts')
    const reason = record(o.reason, 'score.reason')
    if (reason.agent !== 'qa') throw new ShapeError('score.reason.agent')
    const score = num(o.value, 'score.value')
    if (score < 0 || score > 100) throw new ShapeError('score.value')
    return {
      value: score,
      parts: {
        match: num(parts.match, 'score.parts.match'),
        artifacts: num(parts.artifacts, 'score.parts.artifacts'),
        identity: num(parts.identity, 'score.parts.identity'),
        duplicate: num(parts.duplicate, 'score.parts.duplicate'),
      },
      reason: {
        agent: 'qa',
        text: str(reason.text, 'score.reason.text'),
        facts: list(reason.facts, (entry, at) => {
          const fact = record(entry, at)
          return { label: str(fact.label, `${at}.label`), value: str(fact.value, `${at}.value`), source: str(fact.source, `${at}.source`) }
        }, 'score.reason.facts'),
      },
    }
  } catch {
    return null
  }
}

export function readScoreSummary(value: unknown): ScoreSummary {
  if (value == null) return null
  try {
    const o = record(value, 'summary')
    return {
      mode: oneOf(o.mode, approveModes, 'summary.mode'),
      min: num(o.min, 'summary.min'),
      total: num(o.total, 'summary.total'),
      passed: num(o.passed, 'summary.passed'),
      pending: num(o.pending, 'summary.pending'),
      below: list(o.below, (entry, at) => {
        const row = record(entry, at)
        return { id: str(row.id, `${at}.id`), label: str(row.label, `${at}.label`), value: num(row.value, `${at}.value`) }
      }, 'summary.below'),
    }
  } catch {
    return null
  }
}
