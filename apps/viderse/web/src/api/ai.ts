// Client khu vực AI — hợp đồng 07 §15, khớp `internal/web/ai.go`:
// provider {id,kind,auth,label,keyHint,baseUrl,status,quota,catalogAt,
// catalogCount,selected}; model {id,caps[],contextK,price{inPerM,outPerM,
// perImage},recommended,manual}; agents {director{model,effort,fallback},
// artist{promptModel,draw,fallbackOn,drawFallback}} với ModelRef {provider,model}.
// Lỗi: ApiError.payload giữ {reason, field?, need?, agent?}.
import { mutate, request } from './request'

// ---------- Hình dạng wire ----------

/** Capability model làm được (domain D1..D3). */
export type Cap = 'text' | 'vision' | 'image'

export type ProviderKind =
  | 'chatgpt' | 'grok' | 'openai' | 'gemini' | 'openrouter'
  | 'deepseek' | 'xai' | 'anthropic' | 'ollama' | 'openai_compat'

export type ProviderAuth = 'subscription' | 'api_key' | 'local'

export type ProviderStatus = 'ready' | 'logged_out' | 'login_pending' | 'auth_failed' | 'unreachable'

export type ProviderQuota = { used: number; limit: number; period: 'day' | 'week'; resetAt: string }

export type AiProvider = {
  id: string
  kind: ProviderKind
  auth: ProviderAuth
  label: string
  /** "••••7c2e" — chỉ để hiện, không bao giờ là khoá. */
  keyHint?: string
  baseUrl?: string
  status: ProviderStatus
  /** Gói đã lưu của credential đăng nhập gói ("plus"/"pro"); rỗng với
   *  khoá API và với xAI (token xAI không mang thông tin gói). */
  plan?: string
  /** Trần ảnh trả phí kỳ này (nguồn quota duy nhất hiện có). */
  quota?: ProviderQuota
  /** Lần lấy danh mục gần nhất (ISO) — 0/undefined = chưa lấy. */
  catalogAt?: string
  catalogCount: number
  selected: string[]
  /** Trần ảnh trả phí mỗi ngày của provider khoá API (D7); 0 = không tự tiêu. */
  dailyPaidImageLimit: number
}

export type ModelPrice = { inPerM?: number; outPerM?: number; perImage?: number }

export type AiModel = {
  id: string
  caps: Cap[]
  contextK?: number
  price?: ModelPrice
  /** Gợi ý của Viderse (filter=recommended). */
  recommended?: boolean
  /** Thêm tay qua models/check. */
  manual?: boolean
}

export type ModelRef = { provider: string; model: string }

export type AiAgents = {
  director: { model: ModelRef; effort: string; fallback: ModelRef[] }
  artist: { promptModel: ModelRef; draw: ModelRef; fallbackOn: boolean; drawFallback: ModelRef[] }
}

export type AiLogin = {
  phase: 'idle' | 'authorize' | 'code' | 'exchanging' | 'done' | 'failed' | 'cancelled'
  authUrl?: string
  code?: string
  verifyUrl?: string
  /** Hạn mã thiết bị (ISO) — provider trả expires_in; UI đếm ngược. */
  expiresAt?: string
  account?: { email?: string; plan?: string }
  error?: string
}

export const hasCap = (m: AiModel, cap: Cap) => m.caps.includes(cap)
export const refKey = (ref: ModelRef) => (ref.model ? `${ref.provider}/${ref.model}` : '')

// ---------- Nhà cung cấp ----------

export const listProviders = (signal?: AbortSignal) => request<AiProvider[]>('/api/ai/providers', { signal })

export function addProvider(input: { kind: string; apiKey?: string; baseUrl?: string }): Promise<AiProvider> {
  return mutate('POST', '/api/ai/providers', input)
}

/** Thử kết nối: {ok, ms, error?}. */
export function testProvider(id: string): Promise<{ ok: boolean; ms: number; error?: string }> {
  return mutate('POST', `/api/ai/providers/${id}/test`)
}

export function updateProvider(id: string, input: { selected?: string[]; apiKey?: string; baseUrl?: string }): Promise<AiProvider> {
  return mutate('PATCH', `/api/ai/providers/${id}`, input)
}

export function deleteProvider(id: string): Promise<{ ok: boolean }> {
  return mutate('DELETE', `/api/ai/providers/${id}`)
}

/** Bắt đầu phiên đăng nhập subscription → 202 kèm wire phiên (phase=authorize|code). */
export function startProviderLogin(id: string, mode?: 'browser' | 'device'): Promise<AiLogin> {
  return mutate('POST', `/api/ai/providers/${id}/login`, mode ? { mode } : undefined)
}

/** Poll phiên đăng nhập — chỉ gọi khi thẻ/hộp đang mở. */
export function getProviderLogin(id: string, signal?: AbortSignal): Promise<AiLogin> {
  return request<AiLogin>(`/api/ai/providers/${id}/login`, { signal })
}

export function cancelProviderLogin(id: string): Promise<{ ok: boolean }> {
  return mutate('POST', `/api/ai/providers/${id}/login/cancel`)
}

/** ChatGPT browser-flow: dán mã từ trang authorize (máy khác). */
export function submitProviderLoginCode(id: string, code: string): Promise<{ ok: boolean }> {
  return mutate('POST', `/api/ai/providers/${id}/login/code`, { code })
}

/** Đăng xuất gói subscription → status="logged_out". */
export function logoutProvider(id: string): Promise<{ ok: boolean }> {
  return mutate('POST', `/api/ai/providers/${id}/logout`)
}

// ---------- Danh mục model ----------

export type ModelFilter = 'all' | 'selected' | 'recommended'

/** Một trang danh mục — hợp đồng GET .../models của AIConn: `cursor` = id của
 *  model cuối trang trước (keyset, bỏ trống trang đầu); `total` đếm mọi model
 *  khớp q+filter (không tính cursor/limit); `nextCursor` null → hết trang. */
export type ModelPage = { models: AiModel[]; total: number; nextCursor: string | null }

export function listModels(providerId: string, opts?: {
  q?: string
  filter?: ModelFilter
  cursor?: string | null
  limit?: number
  signal?: AbortSignal
}): Promise<ModelPage> {
  const params = new URLSearchParams()
  if (opts?.q) params.set('q', opts.q)
  if (opts?.filter && opts.filter !== 'all') params.set('filter', opts.filter)
  if (opts?.cursor) params.set('cursor', opts.cursor)
  if (opts?.limit) params.set('limit', String(opts.limit))
  const qs = params.toString()
  return request<ModelPage>(`/api/ai/providers/${providerId}/models${qs ? `?${qs}` : ''}`, { signal: opts?.signal })
}

/** Toàn bộ model đã chọn của provider (đi hết các trang) — cho Trợ lý AI,
 *  để metadata/caps của model đã chọn không phụ thuộc trang đầu danh mục. */
export async function listSelectedModels(providerId: string, signal?: AbortSignal): Promise<AiModel[]> {
  const out: AiModel[] = []
  let cursor: string | null = null
  do {
    const page: ModelPage = await listModels(providerId, { filter: 'selected', cursor, limit: 500, signal })
    out.push(...page.models)
    cursor = page.nextCursor
  } while (cursor != null)
  return out
}

/** models/check — kiểm tra model theo tên (thêm thủ công). */
export function checkModel(providerId: string, model: string): Promise<{
  ok: boolean
  ms?: number
  caps?: Cap[]
  contextK?: number
  error?: string
}> {
  return mutate('POST', `/api/ai/providers/${providerId}/models/check`, { model })
}

// ---------- Trợ lý AI ----------

export const getAgents = (signal?: AbortSignal) => request<AiAgents>('/api/ai/agents', { signal })

/** PUT toàn bộ; 422 {reason:"capability_missing", agent, field, need}. */
export function putAgents(agents: AiAgents): Promise<AiAgents> {
  return mutate('PUT', '/api/ai/agents', agents)
}
