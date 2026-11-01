// Đề xuất của trợ lý (data-contract §18) + hoàn tác việc máy tự làm (§22 auto[].undo).
// Hợp đồng đã chốt với backend: danh sách {suggestions,total}; accept có thể trả 409
// `confirm_required` kèm giá; `primary.href` có nghĩa chỉ là điều hướng.
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: Bàn dựng — màn đầu của điện
// thoại — nhập tệp này qua SuggestionPanels và api/workbench.
import { list, maybe, num, oneOf, record, str } from './guard'
import { ApiError, mutate, request } from './request'

export const SUGGESTION_KINDS = ['hot_video', 'template_failing', 'learn_rule', 'raise_autonomy', 'pace', 'perf_insight', 'cleanup'] as const
export type SuggestionKind = (typeof SUGGESTION_KINDS)[number]

const AGENTS = ['scout', 'director', 'artist', 'qa'] as const
export type SuggestionAgent = (typeof AGENTS)[number]

const STATES = ['open', 'accepted', 'dismissed', 'expired'] as const

export type SuggestionReason = {
  agent: SuggestionAgent
  text: string
  facts: { label: string; value: string; source: string }[]
  fix?: { label: string; href: string }
}

export type Suggestion = {
  id: string
  kind: SuggestionKind
  agent: SuggestionAgent
  channelId: string | null
  channelName: string | null
  title: string
  evidence: string
  /** câu đầy đủ của banner học (chỉ learn_rule) */
  banner?: string
  reason: SuggestionReason
  primary: {
    label: string
    op: string
    params: Record<string, unknown>
    cost?: { images?: number }
    href?: string
  }
  createdAt: string
  expiresAt: string
  state: (typeof STATES)[number]
}

export type UndoOp = { op: string; params: Record<string, unknown> }

export type SuggestionList = { suggestions: Suggestion[]; total: number }

export const readReason = (value: unknown, path: string): SuggestionReason => {
  const o = record(value, path)
  return {
    agent: oneOf(o.agent, AGENTS, `${path}.agent`),
    text: str(o.text, `${path}.text`),
    facts: list(o.facts, (entry, at) => {
      const fact = record(entry, at)
      return { label: str(fact.label, `${at}.label`), value: str(fact.value, `${at}.value`), source: str(fact.source, `${at}.source`) }
    }, `${path}.facts`),
    ...(o.fix == null ? {} : (() => {
      const fix = record(o.fix, `${path}.fix`)
      return { fix: { label: str(fix.label, `${path}.fix.label`), href: str(fix.href, `${path}.fix.href`) } }
    })()),
  }
}

/** `op` + `params`: params vắng mặt đọc thành object rỗng (server có thể bỏ trống). */
export const readOp = (value: unknown, path: string): UndoOp => {
  const o = record(value, path)
  return { op: str(o.op, `${path}.op`), params: o.params == null ? {} : record(o.params, `${path}.params`) }
}

const readCost = (value: unknown, path: string): { images?: number } => {
  const o = record(value, path)
  return { ...(o.images === undefined ? {} : { images: num(o.images, `${path}.images`) }) }
}

export const readSuggestion = (value: unknown, path: string): Suggestion => {
  const o = record(value, path)
  const primary = record(o.primary, `${path}.primary`)
  const cost = maybe(primary.cost, readCost, `${path}.primary.cost`)
  const href = maybe(primary.href, str, `${path}.primary.href`)
  const banner = maybe(o.banner, str, `${path}.banner`)
  return {
    id: str(o.id, `${path}.id`),
    kind: oneOf(o.kind, SUGGESTION_KINDS, `${path}.kind`),
    agent: oneOf(o.agent, AGENTS, `${path}.agent`),
    channelId: o.channelId === null ? null : str(o.channelId, `${path}.channelId`),
    channelName: o.channelName === null ? null : str(o.channelName, `${path}.channelName`),
    title: str(o.title, `${path}.title`),
    evidence: str(o.evidence, `${path}.evidence`),
    ...(banner === undefined ? {} : { banner }),
    reason: readReason(o.reason, `${path}.reason`),
    primary: {
      label: str(primary.label, `${path}.primary.label`),
      op: str(primary.op, `${path}.primary.op`),
      params: primary.params == null ? {} : record(primary.params, `${path}.primary.params`),
      ...(cost === undefined ? {} : { cost }),
      ...(href === undefined ? {} : { href }),
    },
    createdAt: str(o.createdAt, `${path}.createdAt`),
    expiresAt: str(o.expiresAt, `${path}.expiresAt`),
    state: oneOf(o.state, STATES, `${path}.state`),
  }
}

export const readSuggestionList = (value: unknown): SuggestionList => {
  const o = record(value, 'suggestions')
  return {
    suggestions: o.suggestions == null ? [] : list(o.suggestions, readSuggestion, 'suggestions.suggestions'),
    total: num(o.total, 'suggestions.total'),
  }
}

export type AcceptResult = {
  ok: true
  summary: string
  undo?: UndoOp
  href?: string
  ruleId?: string
}

const readAccept = (value: unknown): AcceptResult => {
  const o = record(value, 'accept')
  if (o.ok !== true) throw new TypeError('Dữ liệu máy chủ sai dạng: accept.ok')
  const undo = maybe(o.undo, readOp, 'accept.undo')
  const href = maybe(o.href, str, 'accept.href')
  const ruleId = maybe(o.ruleId, str, 'accept.ruleId')
  return {
    ok: true,
    summary: str(o.summary, 'accept.summary'),
    ...(undo === undefined ? {} : { undo }),
    ...(href === undefined ? {} : { href }),
    ...(ruleId === undefined ? {} : { ruleId }),
  }
}

/** `code` có mặt (bất kỳ giá trị chuỗi) = lỗi có mã riêng, không phải "đề xuất đã mất". */
const hasCode = (payload: unknown) => {
  try {
    str(record(payload, 'error').code, 'error.code')
    return true
  } catch {
    return false
  }
}

/** Đọc `{code:'confirm_required', cost:{images}}`; trả `null` khi payload không phải dạng đó. */
const readConfirm = (payload: unknown): { images: number } | null => {
  try {
    const o = record(payload, 'error')
    if (o.code !== 'confirm_required') return null
    const cost = record(o.cost, 'error.cost')
    return { images: cost.images === undefined ? 0 : num(cost.images, 'error.cost.images') }
  } catch {
    return null
  }
}

/** Đề xuất đang mở (tối đa 5, xếp theo giá trị). `channelId` rỗng = mọi kênh. */
export const listSuggestions = async (channelId?: string, signal?: AbortSignal) =>
  readSuggestionList(await request<unknown>(`/api/suggestions${channelId ? `?channelId=${encodeURIComponent(channelId)}` : ''}`, { signal }))

export type AcceptOutcome = { kind: 'done'; result: AcceptResult } | { kind: 'confirm'; images: number } | { kind: 'gone' }

/** Nhận đề xuất. 409 `confirm_required` → hỏi lại người dùng; 404/409 khác → đề xuất đã không còn. */
export async function acceptSuggestion(id: string, confirm = false): Promise<AcceptOutcome> {
  try {
    const result = readAccept(await mutate<unknown>('POST', `/api/suggestions/${encodeURIComponent(id)}/accept`, confirm ? { confirm: true } : {}))
    return { kind: 'done', result }
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) {
      const need = readConfirm(error.payload)
      if (need) return { kind: 'confirm', images: need.images }
    }
    // 404, hoặc 409 không kèm mã lỗi riêng (vd. `limit`) = đề xuất đã không còn; lỗi có mã thì báo thật.
    if (error instanceof ApiError && (error.status === 404 || (error.status === 409 && !hasCode(error.payload)))) return { kind: 'gone' }
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
