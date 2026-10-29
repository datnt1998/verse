// Thao tác trên Config theo đường dẫn khoá dạng "roles.writer.model" (11-api-contract.md §6).
// Mọi hàm trả về bản sao — draft trong SettingsScreen là bất biến.
import type { Config } from '../../../api/types'

export type JsonObject = Record<string, unknown>

export const isJsonObject = (value: unknown): value is JsonObject =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

export function getPath(source: unknown, path: string): unknown {
  let cursor: unknown = source
  for (const key of path.split('.')) {
    if (!isJsonObject(cursor)) return undefined
    cursor = cursor[key]
  }
  return cursor
}

/** Ghi giá trị; `undefined` = xoá khoá (dùng cho "Khôi phục" về cấp kế thừa). */
export function setPath<T extends JsonObject>(source: T, path: string, value: unknown): T {
  const keys = path.split('.')
  const write = (node: JsonObject, index: number): JsonObject => {
    const key = keys[index]
    const next: JsonObject = { ...node }
    if (index === keys.length - 1) {
      if (value === undefined) delete next[key]
      else next[key] = value
      return next
    }
    const child = isJsonObject(node[key]) ? (node[key] as JsonObject) : {}
    next[key] = write(child, index + 1)
    return next
  }
  return write(source, 0) as T
}

export const deletePath = <T extends JsonObject>(source: T, path: string): T => setPath(source, path, undefined)

/** Bỏ object rỗng/undefined để so sánh và gửi PUT không kèm rác. */
export function pruneEmpty(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(pruneEmpty)
  if (!isJsonObject(value)) return value
  const out: JsonObject = {}
  for (const [key, child] of Object.entries(value)) {
    if (child === undefined) continue
    const pruned = pruneEmpty(child)
    if (isJsonObject(pruned) && Object.keys(pruned).length === 0) continue
    out[key] = pruned
  }
  return out
}

/**
 * Cấu hình "như sẽ lưu": object gộp sâu, mảng/giá trị thay thế (giống mock/mergeConfig).
 * Dùng để suy ra model đang dùng và danh sách model đã chọn từ `effective` + `draft` chưa lưu.
 */
export function mergeConfig(base: Config, over: Config): Config {
  const out: JsonObject = { ...base }
  for (const [key, value] of Object.entries(over)) {
    if (value === undefined) continue
    const prev = out[key]
    out[key] = isJsonObject(value) && isJsonObject(prev) ? mergeConfig(prev as Config, value as Config) : value
  }
  return out as Config
}

/** JSON ổn định (sắp khoá) — dùng cho dirty tracking. */
export function stableStringify(value: unknown): string {
  const walk = (node: unknown): unknown => {
    if (Array.isArray(node)) return node.map(walk)
    if (isJsonObject(node)) {
      const sorted: JsonObject = {}
      for (const key of Object.keys(node).sort()) sorted[key] = walk(node[key])
      return sorted
    }
    return node
  }
  return JSON.stringify(walk(pruneEmpty(value)))
}

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T

const providerEntries = (config: JsonObject): [string, JsonObject][] => {
  const providers = isJsonObject(config.providers) ? config.providers : {}
  return Object.entries(providers).filter((entry): entry is [string, JsonObject] => isJsonObject(entry[1]))
}

/** Bỏ trường chỉ-server (`api_key_set`/`api_key_hint`) và api_key rỗng trước khi PUT.
 *  Nhà cung cấp chỉ còn khoá (server giấu khoá) hay đăng nhập gói vẫn giữ entry `{}`: PUT thiếu entry
 *  = server coi như xoá nhà cung cấp và mất khoá; thẻ ở Kết nối AI cũng dựa vào entry này. */
export function configForSave(config: Config): Config {
  const next = clone(config) as JsonObject
  const entries = providerEntries(next)
  for (const [, provider] of entries) {
    delete provider.api_key_set
    delete provider.api_key_hint
    if (provider.api_key === '') delete provider.api_key
    if (Array.isArray(provider.models)) {
      provider.models = provider.models.filter((model) => !isJsonObject(model) || String(model.name ?? '').trim() !== '')
    }
  }
  const out = pruneEmpty(next) as JsonObject
  if (entries.length) {
    const kept = isJsonObject(out.providers) ? out.providers : {}
    out.providers = Object.fromEntries(entries.map(([name]) => [name, kept[name] ?? {}]))
  }
  return out as Config
}

/**
 * Dạng so sánh: như `configForSave` nhưng GIỮ `clear_api_key` — đó là lệnh chưa lưu.
 * Tệp đã lưu không bao giờ chứa cờ này, nên nó chỉ xuất hiện trong bản nháp.
 */
export function configForCompare(config: Config): Config {
  return configForSave(config)
}

export const configsEqual = (a: Config, b: Config): boolean =>
  stableStringify(configForCompare(a)) === stableStringify(configForCompare(b))

/** PUT thay cả tài liệu: chỉ áp dụng các lá đã sửa lên bản server mới nhất. */
export function rebaseConfig(baseline: Config, draft: Config, current: Config): Config {
  const apply = (before: JsonObject, edited: JsonObject, latest: JsonObject): JsonObject => {
    const result = { ...latest }
    for (const key of new Set([...Object.keys(before), ...Object.keys(edited)])) {
      const oldValue = before[key]
      const value = edited[key]
      if (stableStringify(oldValue) === stableStringify(value)) continue
      if (isJsonObject(oldValue) && isJsonObject(value)) {
        result[key] = apply(oldValue, value, isJsonObject(latest[key]) ? latest[key] : {})
      } else if (value === undefined) delete result[key]
      else result[key] = value
    }
    return result
  }
  return configForSave(apply(configForCompare(baseline), configForCompare(draft), configForCompare(current)) as Config)
}

/**
 * Giao thức API (`type`) mà backend chấp nhận. `ProviderType` (internal/bootstrap/config.go)
 * trả `type` nguyên văn, nên phải phủ mọi provider đã đăng ký trong litellm
 * (agentcore `llm.RegisteredProviders()`) cộng `openai-codex`, `xai-oauth` và `anthropic-oauth` (đăng nhập gói thuê bao).
 * Đồng bộ với internal/bootstrap/setup.go khi backend thêm provider mới.
 */
export const PROVIDER_TYPES: readonly string[] = [
  'openai', 'openai-codex', 'xai-oauth', 'anthropic-oauth', 'anthropic', 'gemini', 'openrouter', 'opencode-go', 'deepseek', 'qwen', 'glm', 'grok',
  'bedrock', 'ollama', 'minimax', 'mimo',
]

/** Nhãn gói dưới tên tài khoản OAuth: “ChatGPT Plus”, “SuperGrok / X Premium+”, “Claude Pro/Max”… */
export const oauthPlanLabel = (preset: { id: string; label: string }, plan: string): string => {
  const name = preset.label.split(' (')[0]
  const service = preset.id === 'openai-codex' ? 'ChatGPT' : name
  return plan ? `${service} ${plan}` : service
}

/** Dòng chính của thẻ tài khoản OAuth: Claude hiện “email · tổ chức” (một email có thể giữ nhiều gói), còn lại email. */
export const oauthAccountName = (preset: { id: string }, account: { email: string; label: string }): string =>
  preset.id === 'anthropic-oauth' ? account.label || account.email : account.email || account.label

/**
 * Lựa chọn cho select “Giao thức”: danh sách chuẩn + `type` từ presets của máy chủ + giá trị
 * đang lưu (kể cả giá trị lạ do người dùng tự đặt). Nhờ vậy select luôn hiển thị đúng giá trị
 * đã lưu thay vì rơi về option đầu tiên.
 */
export function providerTypeOptions(stored?: string, presetTypes: readonly string[] = []): string[] {
  const out: string[] = []
  for (const value of [...PROVIDER_TYPES, ...presetTypes, stored ?? '']) {
    if (value && !out.includes(value)) out.push(value)
  }
  return out
}

export type ModelOption = { provider: string; model: string }

export function modelOptions(config: Config): ModelOption[] {
  const out: ModelOption[] = []
  for (const [provider, entry] of Object.entries(config.providers ?? {})) {
    for (const model of entry.models ?? []) if (model.name) out.push({ provider, model: model.name })
  }
  return out
}

export const modelOptionId = (option: ModelOption): string => `${option.provider}/${option.model}`

/**
 * G6 — model gửi kèm POST /api/config/test: chỉ dùng model khai báo trong chính provider đó.
 * Không có ⇒ `undefined` để máy chủ lấy `defaultModel` của preset (14 §A/G6); gửi model chính
 * của provider khác sẽ thử sai model.
 */
export function testModelFor(models: { name: string }[] | undefined): string | undefined {
  return models?.find((model) => model.name)?.name || undefined
}

export const roleLabel = (key: string): string => (key === 'default' ? 'Mặc định' : key.toUpperCase())

/** Settings.dc.html: mỗi vai trò và "Mặc định" chỉ có 4 mức suy nghĩ; '' = Tự động. */
const REASONING_ARTBOARD: { id: string; label: string }[] = [
  { id: '', label: 'Tự động' }, { id: 'low', label: 'Thấp' }, { id: 'medium', label: 'Vừa' }, { id: 'high', label: 'Cao' },
]
const REASONING_EXTRA_LABEL: Record<string, string> = { off: 'Tắt', minimal: 'Tối thiểu', xhigh: 'Rất cao', max: 'Tối đa' }

/** 4 đoạn theo artboard; giá trị đã lưu nằm ngoài 4 mức (off, xhigh, max…) được thêm làm đoạn thứ 5 đang chọn để không trình bày sai cấu hình — chọn mức khác thì đoạn đó tự biến mất. */
export function reasoningSegments(current: string): { id: string; label: string }[] {
  if (!current || REASONING_ARTBOARD.some((segment) => segment.id === current)) return REASONING_ARTBOARD
  return [...REASONING_ARTBOARD, { id: current, label: REASONING_EXTRA_LABEL[current] ?? current }]
}

/** Nhãn vai trò đang gán cho một model (artboard: "Mặc định · Architect · Writer"). */
export function rolesUsing(config: Config, provider: string, model: string): string[] {
  const labels: string[] = []
  if (config.provider === provider && config.model === model) labels.push('Mặc định')
  for (const [key, role] of Object.entries(config.roles ?? {})) {
    if (key === 'default') continue
    if (role.provider === provider && role.model === model) labels.push(roleLabel(key))
  }
  return labels
}

/** Nguồn của một khoá: ưu tiên `sources` từ server, rồi suy ra từ draft. */
export function sourceOf(
  path: string,
  sources: Record<string, string> | undefined,
  draft: Config,
  scope: string,
): string {
  const reported = sources?.[path]
  if (reported) return reported
  return getPath(draft, path) !== undefined ? scope : 'global'
}
