// Đề xuất của trợ lý (data-contract §18) + hoàn tác việc máy tự làm (§22 auto[].undo).
// Hợp đồng đã chốt với backend: danh sách {suggestions,total}; accept có thể trả 409
// `confirm_required` kèm giá; `primary.href` có nghĩa chỉ là điều hướng.
import { z } from 'zod'
import { ApiError, mutate, request } from './request'

export const SUGGESTION_KINDS = ['hot_video', 'template_failing', 'learn_rule', 'raise_autonomy', 'pace', 'perf_insight', 'cleanup'] as const
export type SuggestionKind = (typeof SUGGESTION_KINDS)[number]

const assistant = z.enum(['scout', 'director', 'artist', 'qa'])

export const reasonSchema = z.object({
  agent: assistant,
  text: z.string(),
  facts: z.array(z.object({ label: z.string(), value: z.string(), source: z.string() })),
  fix: z.object({ label: z.string(), href: z.string() }).optional(),
})

const costSchema = z.object({ images: z.number().optional() })
const opSchema = z.object({ op: z.string(), params: z.record(z.string(), z.unknown()).nullable().transform((value) => value ?? {}) })

export const suggestionSchema = z.object({
  id: z.string(),
  kind: z.enum(SUGGESTION_KINDS),
  agent: assistant,
  channelId: z.string().nullable(),
  channelName: z.string().nullable(),
  title: z.string(),
  evidence: z.string(),
  /** câu đầy đủ của banner học (chỉ learn_rule) */
  banner: z.string().nullish().transform((value) => value ?? undefined),
  reason: reasonSchema,
  primary: z.object({
    label: z.string(),
    op: z.string(),
    params: z.record(z.string(), z.unknown()).nullable().transform((value) => value ?? {}),
    cost: costSchema.nullish().transform((value) => value ?? undefined),
    href: z.string().nullish().transform((value) => value ?? undefined),
  }),
  createdAt: z.string(),
  expiresAt: z.string(),
  state: z.enum(['open', 'accepted', 'dismissed', 'expired']),
})
export type Suggestion = z.infer<typeof suggestionSchema>

const listSchema = z.object({ suggestions: z.array(suggestionSchema).nullable().transform((value) => value ?? []), total: z.number() })
export type SuggestionList = z.infer<typeof listSchema>

export const undoSchema = opSchema
export type UndoOp = z.infer<typeof undoSchema>

const acceptSchema = z.object({
  ok: z.literal(true),
  summary: z.string(),
  undo: undoSchema.nullish().transform((value) => value ?? undefined),
  href: z.string().nullish().transform((value) => value ?? undefined),
  ruleId: z.string().nullish().transform((value) => value ?? undefined),
})
export type AcceptResult = z.infer<typeof acceptSchema>

const confirmSchema = z.object({ code: z.literal('confirm_required'), cost: costSchema })

/** Đề xuất đang mở (tối đa 5, xếp theo giá trị). `channelId` rỗng = mọi kênh. */
export const listSuggestions = async (channelId?: string, signal?: AbortSignal) =>
  listSchema.parse(await request<unknown>(`/api/suggestions${channelId ? `?channelId=${encodeURIComponent(channelId)}` : ''}`, { signal }))

export type AcceptOutcome = { kind: 'done'; result: AcceptResult } | { kind: 'confirm'; images: number } | { kind: 'gone' }

/** Nhận đề xuất. 409 `confirm_required` → hỏi lại người dùng; 404/409 khác → đề xuất đã không còn. */
export async function acceptSuggestion(id: string, confirm = false): Promise<AcceptOutcome> {
  try {
    const result = acceptSchema.parse(await mutate<unknown>('POST', `/api/suggestions/${encodeURIComponent(id)}/accept`, confirm ? { confirm: true } : {}))
    return { kind: 'done', result }
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) {
      const need = confirmSchema.safeParse(error.payload)
      if (need.success) return { kind: 'confirm', images: need.data.cost.images ?? 0 }
    }
    if (error instanceof ApiError && (error.status === 404 || error.status === 409)) return { kind: 'gone' }
    throw error
  }
}

/** Bỏ qua (không hiện lại 7 ngày) hoặc `forever` = tắt loại này cho kênh. Đã mất thì coi như xong. */
export async function dismissSuggestion(id: string, forever = false) {
  try {
    await mutate<unknown>('POST', `/api/suggestions/${encodeURIComponent(id)}/dismiss`, forever ? { forever: true } : {})
  } catch (error) {
    if (!(error instanceof ApiError && (error.status === 404 || error.status === 409))) throw error
  }
}

/** Hoàn tác một việc máy đã làm (op + params nguyên văn từ server). */
export const undoOp = async (undo: UndoOp) => { await mutate<unknown>('POST', '/api/undo', { op: undo.op, params: undo.params }) }

/** Loại đề xuất đã tắt: `channelId` null = mọi kênh. Nằm trong cài đặt thông báo (PUT cả khối). */
export const suggestionSettingsSchema = z.object({
  off: z.array(z.object({ channelId: z.string().nullable(), kind: z.enum(SUGGESTION_KINDS) })).nullable().transform((value) => value ?? []),
}).default({ off: [] })
