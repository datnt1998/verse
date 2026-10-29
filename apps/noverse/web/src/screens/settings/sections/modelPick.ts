// Logic thuần của bảng chọn model trong thẻ Kết nối AI (Settings.dc.html conn=models/search/manual).
import type { Config, ModelListContextSource, ProviderModel } from '../../../api/types'

/** Bỏ dấu tiếng Việt + hạ chữ — tìm model không phân biệt dấu (artboard `plain`). */
export const fold = (text: string): string =>
  text.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd')

export type PickFilter = 'sel' | 'sug' | 'all'

/** Một hàng hiển thị: model từ danh sách nhà cung cấp hoặc model đã chọn thêm theo tên. */
export type PickItem = {
  name: string
  subtitle: string
  /** 0 = chưa tra được → hiện “chưa rõ” */
  contextWindow: number
  price?: { input: number; output: number }
  recommended: boolean
  contextSource: ModelListContextSource
  registryWindow: number
}

/** Lọc theo ô tìm (tên hoặc tên hiển thị) rồi theo tab; Đã chọn luôn lên đầu (artboard `rows`).
 *  Tách từ khoá theo khoảng trắng/dấu nối: “glm 5.1” khớp `glm-5.1`, “gpt mini” khớp `openai/gpt-4o-mini`. */
export function pickRows(items: readonly PickItem[], query: string, filter: PickFilter, selected: readonly string[]): PickItem[] {
  const words = fold(query).split(/[\s\-_/:]+/).filter(Boolean)
  const sel = new Set(selected)
  let rows = items.filter((item) => {
    const hay = fold(`${item.name} ${item.subtitle}`)
    return words.every((word) => hay.includes(word))
  })
  if (filter === 'sel') rows = rows.filter((item) => sel.has(item.name))
  else if (filter === 'sug') rows = rows.filter((item) => item.recommended || sel.has(item.name))
  return rows.slice().sort((a, b) => Number(sel.has(b.name)) - Number(sel.has(a.name)))
}

// ─── Khoá bỏ chọn khi model đang gán cho trợ lý (artboard `inUse`) ─────────────

/** Nhãn trợ lý dùng trong khoá bỏ chọn + toast + 409 (architect/writer/editor → tiếng Việt, vai khác giữ nguyên — cùng nhãn với máy chủ). */
export const ROLE_USAGE_LABEL: Record<string, string> = {
  default: 'Mặc định', architect: 'Kiến trúc sư', writer: 'Người viết', editor: 'Biên tập viên',
}

type Ref = { provider: string; model: string }

/** Mọi tham chiếu provider+model trong cấu hình đã gộp: Mặc định (cấp trên) · từng vai · dự phòng của vai. */
export function modelRefsOf(config: Config): { ref: Ref; label: string }[] {
  const top: Ref = { provider: config.provider ?? '', model: config.model ?? '' }
  const out: { ref: Ref; label: string }[] = []
  if (top.provider && top.model) out.push({ ref: top, label: 'Mặc định' })
  for (const [key, role] of Object.entries(config.roles ?? {})) {
    const provider = role.provider || top.provider
    const model = role.model || top.model
    const label = ROLE_USAGE_LABEL[key] ?? key
    if (provider && model) out.push({ ref: { provider, model }, label })
    for (const fb of role.fallbacks ?? []) {
      if (fb.provider && fb.model) out.push({ ref: { provider: fb.provider, model: fb.model }, label: `${label} (dự phòng)` })
    }
  }
  return out
}

/** Nhãn trợ lý đang dùng `provider/model` (khử trùng lặp, giữ thứ tự) — rỗng = bỏ chọn được. */
export function usageOf(config: Config, provider: string, model: string): string[] {
  const labels: string[] = []
  for (const { ref, label } of modelRefsOf(config)) {
    if (ref.provider === provider && ref.model === model && !labels.includes(label)) labels.push(label)
  }
  return labels
}

/** Câu khoá bỏ chọn (tooltip + toast khi bấm ô đang dùng). */
export const lockMessage = (labels: readonly string[]): string =>
  `Đang dùng cho ${labels.join(', ')}. Đổi model của trợ lý đó trước khi bỏ chọn.`

/**
 * Ô Model ở Trợ lý AI: nhóm theo nhà cung cấp, chỉ model đã chọn (`providers.<id>.models`).
 * Nhà cung cấp được vai trỏ tới nhưng chưa chọn model vẫn có nhóm (rỗng) để hiện “Chưa chọn model · Chọn”.
 */
export function providerGroups(config: Config): { provider: string; models: { provider: string; model: string }[] }[] {
  const ids = new Set(Object.keys(config.providers ?? {}))
  for (const { ref } of modelRefsOf(config)) if (ref.provider) ids.add(ref.provider)
  return [...ids].sort().map((provider) => ({
    provider,
    models: (config.providers?.[provider]?.models ?? [])
      .filter((model) => model.name)
      .map((model) => ({ provider, model: model.name })),
  }))
}

// ─── Ghi vào bản nháp ────────────────────────────────────────────────────────

/**
 * Chọn model chỉ ghi `{name}`; ghi thêm `context_window` khi ngữ cảnh
 * đến từ API nhà cung cấp và danh mục Noverse không biết model (registryWindow = 0).
 */
export const shouldWriteContext = (item: { contextSource: ModelListContextSource; registryWindow: number }): boolean =>
  item.contextSource === 'provider' && item.registryWindow === 0

/** Giá “vào · ra” mỗi 1 triệu token: $0,13 vào · $0,60 ra (artboard `.replace(' · ', ' vào · ') + ' ra'`). */
const usd = (n: number): string => `$${String(Math.round(n * 100) / 100).replace('.', ',').replace(/,0+$/, '')}`
export const priceText = (price: { input: number; output: number } | undefined, kind: 'subscription' | 'local' | 'key'): string => {
  if (price) return `${usd(price.input)} vào · ${usd(price.output)} ra`
  return kind === 'subscription' ? 'theo gói' : kind === 'local' ? 'miễn phí' : '—'
}

/** ProviderModel của API → hàng của bảng; `ctx` từ context_window đã khai trong bản nháp (ghi đè hiển thị). */
export function itemOf(model: ProviderModel, draftContext?: number): PickItem {
  return {
    name: model.name,
    // Tên hiển thị trùng id (OpenAI-compatible, Ollama…) thì không lặp lại ở dòng phụ.
    subtitle: model.description || (model.displayName !== model.name ? model.displayName : ''),
    contextWindow: draftContext ?? model.contextWindow,
    price: model.price,
    recommended: model.recommended,
    contextSource: model.contextSource,
    registryWindow: model.registryWindow,
  }
}

/** Model đã chọn nhưng không có trong danh sách lấy được (thêm theo tên / đã gỡ khỏi catalog). */
export function draftItem(name: string, contextWindow?: number): PickItem {
  return {
    name, subtitle: 'Thêm theo tên', contextWindow: contextWindow ?? 0,
    recommended: false, contextSource: 'default', registryWindow: 0,
  }
}

// ─── Câu chữ của bảng (artboard provs → rows/srcLine/noneText/selLine/manOkText…) ──

export const searchPlaceholder = (total: number): string => (total > 12 ? `Tìm trong ${total} model` : 'Tìm model')

export const noneText = (query: string, filter: PickFilter): string =>
  query.trim()
    ? `Không có model nào khớp “${query.trim()}”. Thử tên khác, hoặc thêm theo tên.`
    : filter === 'sel' ? 'Chưa chọn model nào.' : 'Không có model gợi ý.'

const clock = (isoText: string): string => {
  const time = new Date(isoText)
  if (Number.isNaN(time.getTime())) return ''
  return `${String(time.getHours()).padStart(2, '0')}:${String(time.getMinutes()).padStart(2, '0')}`
}

/** Chân bảng: “Lấy từ OpenRouter lúc 10:24 · đang hiện 9 trong 312, gõ tên để tìm”. */
export function sourceLine(input: { from: string; fetchedAt: string; shown: number; filtered: number; total: number; hasPrice: boolean }): string {
  const at = clock(input.fetchedAt)
  let text = `Lấy từ ${input.from}${at ? ` lúc ${at}` : ''}`
  text += input.filtered > 50 ? ` · đang hiện ${input.shown} trong ${input.filtered}, gõ tên để tìm` : ` · ${input.filtered} model`
  if (input.hasPrice) text += ' · giá tính trên 1 triệu token'
  return text
}

export const selectedLine = (count: number): string =>
  count === 0
    ? 'Chưa chọn model nào.'
    : `Đã chọn ${count} model. Model đã chọn hiện trong Trợ lý AI để gán cho từng trợ lý; bấm Lưu ở thanh dưới để giữ.`

/** contextSource → nhãn trong câu “ngữ cảnh 164K (tự nhận)”. */
const SOURCE_NOTE: Record<ModelListContextSource, string> = {
  model_config: 'đã khai', provider: 'tự nhận', registry: 'tự nhận', default: 'dự phòng',
}

export type ManualResult = { kind: 'ok' | 'warn'; text: string }

/** Kết quả “Kiểm tra & thêm”: ok kèm ngữ cảnh tự nhận; contextSource 'default' → cảnh báo mức dự phòng. */
export function manualResult(name: string, latencyMs: number, window: number, source: ModelListContextSource | undefined, fmt: (n: number) => string): ManualResult {
  const ctx = window || 200000
  if (source === 'default' || !source && ctx === 200000) {
    return { kind: 'warn', text: `${name} trả lời được, nhưng Noverse chưa biết ngữ cảnh của model này nên tạm dùng mức dự phòng 200K. Đổi ở Trợ lý AI › Ngữ cảnh nếu cần.` }
  }
  return { kind: 'ok', text: `${name} trả lời được · ${Math.round(latencyMs)} ms · ngữ cảnh ${fmt(ctx)} (${SOURCE_NOTE[source ?? 'registry']}). Đã thêm và chọn.` }
}
