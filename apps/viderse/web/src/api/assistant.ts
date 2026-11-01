// Nhắn trợ lý (data-contract §9 + §22). Mỗi câu → `plan` (1–5 bước, chưa chạy
// gì); chỉ `run-plan` mới chạy và chỉ các bước người dùng giữ tích. Câu người
// dùng chỉ là dữ liệu gửi đi; guard ở đây từ chối intent/trợ lý ngoài danh sách
// thay vì đoán, nên payload lạ thành lỗi "không đọc được" chứ không thành nút bấm.
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: AssistantComposer nằm trong
// chunk của màn đầu (Bàn dựng).
import { bool, list, maybe, num, oneOf, record, str, ShapeError } from './guard'
import { ApiError, mutate, request } from './request'

export const INTENTS = [
  'redraw_image', 'redraw_all', 'skip_image', 'edit_metadata', 'reroll_backdrop', 'no_images', 'pause', 'resume',
  'set_pace', 'filter_auto', 'make_latest', 'prioritize', 'set_autonomy', 'add_rule', 'make_videos',
] as const
export type IntentId = (typeof INTENTS)[number]

export type AssistantScope = { jobId: string } | { channelId: string } | 'all'

const AGENTS = ['scout', 'director', 'artist', 'qa'] as const
export type AssistantAgent = (typeof AGENTS)[number]

const STATES = ['queued', 'running', 'done', 'failed'] as const
export type StepState = (typeof STATES)[number]

const nonEmptyStr = (value: unknown, path: string) => {
  const text = str(value, path)
  if (text.length === 0) throw new ShapeError(path)
  return text
}

export type PlanStep = {
  id: string
  intent: IntentId
  params: unknown
  title: string
  sub: string
  agent: AssistantAgent
  cost?: { images?: number }
  enqueuesVideo: boolean
  detail: string
  // Go gửi `null` cho slice rỗng chưa khởi tạo.
  reruns: { agent: AssistantAgent; label: string }[]
}

const readStep = (value: unknown, path: string): PlanStep => {
  const o = record(value, path)
  const cost = maybe(o.cost, (entry, at) => {
    const c = record(entry, at)
    return c.images === undefined ? {} : { images: num(c.images, `${at}.images`) }
  }, `${path}.cost`)
  return {
    id: nonEmptyStr(o.id, `${path}.id`),
    intent: oneOf(o.intent, INTENTS, `${path}.intent`),
    params: o.params,
    title: str(o.title, `${path}.title`),
    sub: str(o.sub, `${path}.sub`),
    agent: oneOf(o.agent, AGENTS, `${path}.agent`),
    ...(cost === undefined ? {} : { cost }),
    enqueuesVideo: bool(o.enqueuesVideo, `${path}.enqueuesVideo`),
    detail: str(o.detail, `${path}.detail`),
    reruns: o.reruns == null ? [] : list(o.reruns, (entry, at) => {
      const rerun = record(entry, at)
      return { agent: oneOf(rerun.agent, AGENTS, `${at}.agent`), label: str(rerun.label, `${at}.label`) }
    }, `${path}.reruns`),
  }
}

export type Plan = {
  ok: true
  planId: string
  steps: PlanStep[]
  total: { images: number; note?: string }
  imagesLeftToday?: number
}

const readPlan = (value: unknown): Plan => {
  const o = record(value, 'plan')
  const steps = list(o.steps, readStep, 'plan.steps')
  if (steps.length === 0 || steps.length > 5) throw new ShapeError('plan.steps')
  const total = record(o.total, 'plan.total')
  return {
    ok: true,
    planId: nonEmptyStr(o.planId, 'plan.planId'),
    steps,
    total: { images: num(total.images, 'plan.total.images'), ...(total.note == null ? {} : { note: str(total.note, 'plan.total.note') }) },
    ...(o.imagesLeftToday == null ? {} : { imagesLeftToday: num(o.imagesLeftToday, 'plan.imagesLeftToday') }),
  }
}

// Mỗi lựa chọn/gợi ý là một câu đầy đủ gửi lại làm kế hoạch mới; bấm chỉ lập kế hoạch, không chạy.
export type Choice = { label: string; text: string; icon?: string }

const readChoice = (value: unknown, path: string): Choice => {
  const o = record(value, path)
  return {
    label: nonEmptyStr(o.label, `${path}.label`),
    text: nonEmptyStr(o.text, `${path}.text`),
    ...(o.icon == null ? {} : { icon: str(o.icon, `${path}.icon`) }),
  }
}

export type PlanAsk = { question: string; options: Choice[] }
export type PlanUnknown = { lead: string; suggestions: Choice[]; link?: { label: string; href: string } }

export type PlanReply =
  | { kind: 'plan'; plan: Plan }
  | { kind: 'ask'; ask: PlanAsk }
  | { kind: 'unknown'; unknown: PlanUnknown }

const readAsk = (value: unknown): PlanAsk => {
  const o = record(value, 'ask')
  const options = list(o.options, readChoice, 'ask.options')
  if (options.length === 0 || options.length > 4) throw new ShapeError('ask.options')
  return { question: str(o.question, 'ask.question'), options }
}

const readUnknown = (value: unknown): PlanUnknown => {
  const o = record(value, 'unknown')
  const suggestions = list(o.suggestions, readChoice, 'unknown.suggestions')
  if (suggestions.length > 2) throw new ShapeError('unknown.suggestions')
  const link = maybe(o.link, (entry, at) => {
    const l = record(entry, at)
    return { label: nonEmptyStr(l.label, `${at}.label`), href: nonEmptyStr(l.href, `${at}.href`) }
  }, 'unknown.link')
  return {
    lead: str(o.lead, 'unknown.lead'),
    suggestions,
    ...(link === undefined ? {} : { link }),
  }
}

const unreadable = (raw: unknown): ApiError => new ApiError(502, 'Trợ lý trả về dữ liệu không đọc được.', undefined, raw)

const readPlanReply = (raw: unknown): PlanReply => {
  try {
    const o = record(raw, 'reply')
    if (o.ok === true) return { kind: 'plan', plan: readPlan(o) }
    if (o.ok !== false) throw new ShapeError('reply.ok')
    return o.ask === undefined
      ? { kind: 'unknown', unknown: readUnknown(o.unknown) }
      : { kind: 'ask', ask: readAsk(o.ask) }
  } catch {
    throw unreadable(raw)
  }
}

export type RunStep = { id: string; state: StepState; message?: string }
export type PlanRun = { runId: string; state?: 'running' | 'done' | 'failed'; steps: RunStep[] }

const readRun = (raw: unknown): PlanRun => {
  try {
    const o = record(raw, 'run')
    return {
      runId: nonEmptyStr(o.runId, 'run.runId'),
      ...(o.state == null ? {} : { state: oneOf(o.state, ['running', 'done', 'failed'] as const, 'run.state') }),
      steps: list(o.steps, (entry, at) => {
        const step = record(entry, at)
        return {
          id: str(step.id, `${at}.id`),
          state: oneOf(step.state, STATES, `${at}.state`),
          ...(step.message == null ? {} : { message: str(step.message, `${at}.message`) }),
        }
      }, 'run.steps'),
    }
  } catch {
    throw unreadable(raw)
  }
}

export const MAX_TEXT_RUNES = 500

export async function planAssistant(input: { scope: AssistantScope; text: string }, signal?: AbortSignal): Promise<PlanReply> {
  return readPlanReply(await mutate<unknown>('POST', '/api/assistant/plan', { scope: input.scope, text: input.text }, signal))
}

/** Chạy đúng các bước đã chọn của kế hoạch `planId`; server kiểm lại từng bước. */
export async function runPlan(planId: string, steps: string[], signal?: AbortSignal): Promise<PlanRun> {
  return readRun(await mutate<unknown>('POST', '/api/assistant/run-plan', { planId, steps }, signal))
}

export async function getPlanRun(runId: string, signal?: AbortSignal): Promise<PlanRun> {
  return readRun(await request<unknown>(`/api/assistant/runs/${encodeURIComponent(runId)}`, { signal }))
}

export type AssistantErrorKind = 'offline' | 'model' | 'expired' | 'busy' | 'invalid' | 'notFound' | 'failed'

/** Mã lỗi máy chủ (`code`) → loại lỗi để thẻ chọn đúng câu và nút Thử lại. */
export function assistantErrorKind(error: unknown): AssistantErrorKind {
  if (!(error instanceof ApiError)) return 'failed'
  if (error.status === 0) return 'offline'
  let code: string | undefined
  try {
    code = str(record(error.payload, 'error').code, 'error.code')
  } catch {
    code = undefined
  }
  switch (code) {
    case 'model_unavailable': return 'model'
    case 'plan_expired': return 'expired'
    case 'rate_limited': return 'busy'
    case 'text_invalid': return 'invalid'
    case 'not_found': return 'notFound'
    default: return error.status === 410 ? 'expired' : error.status === 503 ? 'model' : error.status === 429 ? 'busy' : 'failed'
  }
}
