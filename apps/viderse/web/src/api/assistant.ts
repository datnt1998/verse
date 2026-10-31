// Nhắn trợ lý (data-contract §9 + §22). Mỗi câu → `plan` (1–5 bước, chưa chạy
// gì); chỉ `run-plan` mới chạy và chỉ các bước người dùng giữ tích. Câu người
// dùng chỉ là dữ liệu gửi đi; schema ở đây từ chối intent/trợ lý ngoài danh sách
// thay vì đoán, nên payload lạ thành lỗi "không đọc được" chứ không thành nút bấm.
import { z } from 'zod'
import { ApiError, mutate, request } from './request'

export const INTENTS = [
  'redraw_image', 'redraw_all', 'skip_image', 'edit_metadata', 'reroll_backdrop', 'no_images', 'pause', 'resume',
  'set_pace', 'filter_auto', 'make_latest', 'prioritize', 'set_autonomy', 'add_rule', 'make_videos',
] as const
export type IntentId = (typeof INTENTS)[number]

export type AssistantScope = { jobId: string } | { channelId: string } | 'all'

const agentSchema = z.enum(['scout', 'director', 'artist', 'qa'])
export type AssistantAgent = z.infer<typeof agentSchema>

const stepSchema = z.object({
  id: z.string().min(1),
  intent: z.enum(INTENTS),
  params: z.unknown(),
  title: z.string(),
  sub: z.string(),
  agent: agentSchema,
  cost: z.object({ images: z.number().int().nonnegative().optional() }).optional(),
  enqueuesVideo: z.boolean(),
  detail: z.string(),
  // Go gửi `null` cho slice rỗng chưa khởi tạo.
  reruns: z.array(z.object({ agent: agentSchema, label: z.string() })).nullable().transform((value) => value ?? []),
})
export type PlanStep = z.infer<typeof stepSchema>

const planSchema = z.object({
  ok: z.literal(true),
  planId: z.string().min(1),
  steps: z.array(stepSchema).min(1).max(5),
  total: z.object({ images: z.number().int().nonnegative(), note: z.string().optional() }),
  imagesLeftToday: z.number().int().nonnegative().optional(),
})
export type Plan = z.infer<typeof planSchema>

// Mỗi lựa chọn/gợi ý là một câu đầy đủ gửi lại làm kế hoạch mới; bấm chỉ lập kế hoạch, không chạy.
const choiceSchema = z.object({ label: z.string().min(1), text: z.string().min(1), icon: z.string().optional() })
export type Choice = z.infer<typeof choiceSchema>

const askSchema = z.object({
  ok: z.literal(false),
  ask: z.object({ question: z.string(), options: z.array(choiceSchema).min(1).max(4) }),
})
export type PlanAsk = z.infer<typeof askSchema>['ask']

const unknownSchema = z.object({
  ok: z.literal(false),
  unknown: z.object({
    lead: z.string(),
    suggestions: z.array(choiceSchema).max(2),
    link: z.object({ label: z.string().min(1), href: z.string().min(1) }).optional(),
  }),
})
export type PlanUnknown = z.infer<typeof unknownSchema>['unknown']

const planReplySchema = z.union([planSchema, askSchema, unknownSchema])

export type PlanReply =
  | { kind: 'plan'; plan: Plan }
  | { kind: 'ask'; ask: PlanAsk }
  | { kind: 'unknown'; unknown: PlanUnknown }

const stepStateSchema = z.enum(['queued', 'running', 'done', 'failed'])
export type StepState = z.infer<typeof stepStateSchema>
const runStepSchema = z.object({ id: z.string(), state: stepStateSchema, message: z.string().optional() })
export type RunStep = z.infer<typeof runStepSchema>

const runSchema = z.object({
  runId: z.string().min(1),
  state: z.enum(['running', 'done', 'failed']).optional(),
  steps: z.array(runStepSchema),
})
export type PlanRun = z.infer<typeof runSchema>

export const MAX_TEXT_RUNES = 500

export async function planAssistant(input: { scope: AssistantScope; text: string }, signal?: AbortSignal): Promise<PlanReply> {
  const raw = await mutate<unknown>('POST', '/api/assistant/plan', { scope: input.scope, text: input.text }, signal)
  const parsed = planReplySchema.safeParse(raw)
  if (!parsed.success) throw new ApiError(502, 'Trợ lý trả về dữ liệu không đọc được.', undefined, raw)
  const reply = parsed.data
  if (reply.ok) return { kind: 'plan', plan: reply }
  if ('ask' in reply) return { kind: 'ask', ask: reply.ask }
  return { kind: 'unknown', unknown: reply.unknown }
}

const readRun = (raw: unknown): PlanRun => {
  const parsed = runSchema.safeParse(raw)
  if (!parsed.success) throw new ApiError(502, 'Trợ lý trả về dữ liệu không đọc được.', undefined, raw)
  return parsed.data
}

/** Chạy đúng các bước đã chọn của kế hoạch `planId`; server kiểm lại từng bước. */
export async function runPlan(planId: string, steps: string[], signal?: AbortSignal): Promise<PlanRun> {
  return readRun(await mutate<unknown>('POST', '/api/assistant/run-plan', { planId, steps }, signal))
}

export async function getPlanRun(runId: string, signal?: AbortSignal): Promise<PlanRun> {
  return readRun(await request<unknown>(`/api/assistant/runs/${encodeURIComponent(runId)}`, { signal }))
}

export type AssistantErrorKind = 'offline' | 'model' | 'expired' | 'busy' | 'invalid' | 'notFound' | 'failed'

const errorCodeSchema = z.object({ code: z.string() })

/** Mã lỗi máy chủ (`code`) → loại lỗi để thẻ chọn đúng câu và nút Thử lại. */
export function assistantErrorKind(error: unknown): AssistantErrorKind {
  if (!(error instanceof ApiError)) return 'failed'
  if (error.status === 0) return 'offline'
  const code = errorCodeSchema.safeParse(error.payload).data?.code
  switch (code) {
    case 'model_unavailable': return 'model'
    case 'plan_expired': return 'expired'
    case 'rate_limited': return 'busy'
    case 'text_invalid': return 'invalid'
    case 'not_found': return 'notFound'
    default: return error.status === 410 ? 'expired' : error.status === 503 ? 'model' : error.status === 429 ? 'busy' : 'failed'
  }
}
