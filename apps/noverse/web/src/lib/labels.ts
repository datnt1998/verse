// Nguồn duy nhất đổi tên công cụ / trường / vai trò sang lời thường
// (20-settings-rules.md §5, invariant #7 — không lộ chữ kỹ thuật ra giao diện).
// Tên công cụ gốc (`commit_chapter`…) chỉ được hiện trong khối “Chi tiết kỹ thuật” do người dùng mở.
import { AGENT_PALETTE } from '../ui/agentPalette'
import { formatNumber } from './format'

export const OTHER_TOOL = 'Công cụ khác'
export const OTHER_VALUE = 'Khác'

type ToolSpec = { label: string; /** dạng đang chạy: “đang lưu chương” */ active: string }

const TOOLS: Record<string, ToolSpec> = {
  novel_context: { label: 'Đọc bối cảnh', active: 'đang đọc bối cảnh' },
  read_context: { label: 'Đọc bối cảnh', active: 'đang đọc bối cảnh' },
  read_chapter: { label: 'Đọc chương', active: 'đang đọc chương' },
  draft_chapter: { label: 'Viết bản nháp', active: 'đang viết bản nháp' },
  edit_chapter: { label: 'Gọt giũa', active: 'đang gọt giũa' },
  check_consistency: { label: 'Soát nhất quán', active: 'đang soát nhất quán' },
  commit_chapter: { label: 'Lưu chương', active: 'đang lưu chương' },
  save_chapter: { label: 'Lưu chương', active: 'đang lưu chương' },
  plan_chapter: { label: 'Lập kế hoạch chương', active: 'đang lập kế hoạch chương' },
  save_book: { label: 'Lưu thông tin truyện', active: 'đang lưu thông tin truyện' },
  save_foundation: { label: 'Lưu nền truyện', active: 'đang lưu nền truyện' },
  update_outline: { label: 'Cập nhật dàn ý', active: 'đang cập nhật dàn ý' },
  score_chapter: { label: 'Chấm điểm chương', active: 'đang chấm điểm chương' },
  save_review: { label: 'Lưu thẩm duyệt', active: 'đang lưu thẩm duyệt' },
  save_arc_summary: { label: 'Tóm tắt cung', active: 'đang tóm tắt cung' },
  save_volume_summary: { label: 'Tóm tắt tập', active: 'đang tóm tắt tập' },
  audit_foundation: { label: 'Soát nền truyện', active: 'đang soát nền truyện' },
  revise_outline: { label: 'Sửa đại cương', active: 'đang sửa đại cương' },
  resolve_outline_feedback: { label: 'Xử lý góp ý đại cương', active: 'đang xử lý góp ý đại cương' },
  reopen_book: { label: 'Mở lại truyện', active: 'đang mở lại truyện' },
  arbitrate: { label: 'Phán định', active: 'đang phán định' },
}

/** `save_foundation[type]` — `type` là loại thiết lập host gửi kèm. */
const FOUNDATION: Record<string, ToolSpec> = {
  premise: { label: 'Lưu tiền đề', active: 'đang lưu tiền đề' },
  outline: { label: 'Lưu dàn ý', active: 'đang lưu dàn ý' },
  layered_outline: { label: 'Lưu dàn ý', active: 'đang lưu dàn ý' },
  expand_arc: { label: 'Lưu dàn ý', active: 'đang lưu dàn ý' },
  append_volume: { label: 'Lưu dàn ý', active: 'đang lưu dàn ý' },
  characters: { label: 'Lưu nhân vật', active: 'đang lưu nhân vật' },
  world_rules: { label: 'Lưu quy tắc thế giới', active: 'đang lưu quy tắc thế giới' },
  update_compass: { label: 'Cập nhật hướng đi', active: 'đang cập nhật hướng đi' },
  complete_book: { label: 'Hoàn tất truyện', active: 'đang hoàn tất truyện' },
}

const specFor = (tool: string, variant = ''): ToolSpec | undefined =>
  (tool === 'save_foundation' ? FOUNDATION[variant] : undefined) ?? TOOLS[tool]

/** Công cụ nằm trong bảng §5? */
export const isKnownTool = (tool: string): boolean => tool in TOOLS

/** `snake_case` của engine → chữ thường có cách ('export_book' → 'export book'); lạ hẳn → ''. */
export const readable = (name: string): string => name.trim().toLowerCase().split('_').filter(Boolean).join(' ')

/** Nhãn thường của công cụ; chưa có trong bảng → tên engine viết liền ('export book'), rỗng → “Công cụ khác”. */
export function toolLabel(tool: string, variant = ''): string {
  return specFor(tool, variant)?.label ?? (readable(tool) || OTHER_TOOL)
}

// Icon theo loại công cụ — 07-copy.md › Bảng từ (tên file trong web/src/assets/icons).
const TOOL_ICONS: Record<string, string> = {
  novel_context: 'note-text', read_context: 'note-text',
  read_chapter: 'book-1', draft_chapter: 'edit-2', edit_chapter: 'magicpen',
  check_consistency: 'shield-tick', audit_foundation: 'shield-tick',
  commit_chapter: 'folder-open', save_chapter: 'folder-open', save_book: 'folder-open', save_foundation: 'folder-open',
  plan_chapter: 'routing-2', update_outline: 'routing-2', score_chapter: 'chart-2',
  save_arc_summary: 'note-text',
}

/** Tên icon cho loại công cụ; chưa có trong bảng → 'code'. */
export const toolIcon = (tool: string): string => TOOL_ICONS[tool] ?? 'code'

/** Mẫu `{nhãn} · chương {n}` (chữ thường, không mono). Số chương vô lý (canh `2147483647`, ≤ 0) bị bỏ. */
export function formatToolLabel(label: string, chapter?: number | null, ...extras: string[]): string {
  const n = chapter == null ? null : saneChapter(chapter)
  return [label, n ? `chương ${n}` : '', ...extras].filter(Boolean).join(' · ')
}

// ─── Vai trò ─────────────────────────────────────────────────────────────

const ROLE_ALIAS: Record<string, keyof typeof AGENT_PALETTE> = {
  writer: 'writer', architect: 'architect', architect_long: 'architect', architect_short: 'architect',
  editor: 'editor', arbiter: 'arbiter',
}

/** `writer` → “Người viết”. Vai lạ → ''. */
export function agentRoleLabel(agent: string | null | undefined): string {
  const kind = ROLE_ALIAS[(agent ?? '').trim().toLowerCase()]
  return kind ? AGENT_PALETTE[kind].name : ''
}

/** Vai `system` và `agent` (chữ chung) — nhãn riêng ngoài bảng màu agent. */
const EXTRA_ROLES: Record<string, string> = { system: 'Hệ thống', agent: 'trợ lý' }

/** `writer` → “Người viết”, `system` → “Hệ thống”. Vai lạ → tên engine chữ thường; rỗng → ''. */
export function agentLabel(role: string | null | undefined): string {
  const key = (role ?? '').trim().toLowerCase()
  return agentRoleLabel(key) || EXTRA_ROLES[key] || readable(key)
}

const ROLE_IN_TEXT = new RegExp(`\\b(${[...Object.keys(ROLE_ALIAS), ...Object.keys(EXTRA_ROLES)].join('|')})\\b`, 'g')
const TOOL_IN_TEXT = new RegExp(`\\b(${Object.keys(TOOLS).join('|')})\\b(?:\\([^)]*(?:\\)|$))?`, 'g')

/** Bản giao việc host lưu cả tên vai và ngoặc bao nhiệm vụ, đôi khi bị cắt giữa lời gọi công cụ. */
function agentTaskOf(text: string): { role: string; task: string } | null {
  const match = /^([a-z_]+)\s*[（(]([\s\S]*)[）)]$/.exec(text.trim())
  return match && agentRoleLabel(match[1]) ? { role: match[1], task: match[2] } : null
}

// Gợi ý kỹ thuật trong ngoặc của bản giao việc: “(save_foundation type=expand_arc)”, “(save_review scope=global, chapter=8)”,
// “(save_volume_summary)”. Câu tiếng Việt phía trước đã nói việc gì → bỏ cả cụm, không dịch dở thành “(Lưu nền truyện type=…)”.
const TECH_HINT = /\s*[（(]\s*[a-z]+_[a-z_]+\b[^)）]*[)）]/g
// “save_foundation type=expand_arc” / “save_foundation(type=expand_arc)” → nhãn theo loại (“Lưu dàn ý”).
const TOOL_WITH_TYPE = new RegExp(`\\b(${Object.keys(TOOLS).join('|')})\\s*(?:\\(\\s*type=([a-z_]+)\\s*\\)|\\s+type=([a-z_]+))`, 'g')
// `type=append_volume` còn sót giữa câu → nhãn thiết lập tương ứng.
const BARE_TYPE = /\btype=([a-z_]+)/g

export function humanizeFreeText(text: string): string {
  // Chuỗi cũ tiếng Trung vẫn đi qua bảng dịch host, không chặn nó bằng một bản dịch vai dở dang.
  if (/[\u3400-\u9fff]/.test(text)) return text
  return text
    .replace(TECH_HINT, '')
    .replace(TOOL_WITH_TYPE, (_, tool: string, a?: string, b?: string) => `“${toolLabel(tool, a ?? b ?? '')}”`)
    .replace(BARE_TYPE, (_, variant: string) => `“${toolLabel('save_foundation', variant)}”`)
    .replace(TOOL_IN_TEXT, (_, tool: string) => toolLabel(tool))
    .replace(ROLE_IN_TEXT, (_, role: string) => agentLabel(role))
}

/**
 * Dòng tiến độ của agent: “Người viết · đang lưu chương 4”.
 * Không có công cụ → chỉ tên vai; công cụ lạ → “đang {tên engine liền}”.
 */
export function agentProgressLine(agent: string, tool: string, chapter?: number | null, variant = ''): string {
  const role = agentLabel(agent) || OTHER_VALUE
  if (!tool) return role
  const n = chapter == null ? null : saneChapter(chapter)
  const spec = specFor(tool, variant)
  if (!spec) return `${role} · đang ${readable(tool) || OTHER_TOOL.toLowerCase()}${n ? ` · chương ${n}` : ''}`
  const text = n
    ? spec.active.endsWith('chương') ? `${spec.active} ${n}` : `${spec.active} chương ${n}`
    : spec.active
  return `${role} · ${text}`
}

/** Nhãn đang chạy cho tiêu đề luồng viết: “Đang lưu chương”. Công cụ lạ → “Đang {tên engine liền}”. */
export function activeToolLabel(tool: string, variant = ''): string {
  const spec = specFor(tool, variant)
  if (!spec) return `Đang ${readable(tool) || OTHER_TOOL.toLowerCase()}`
  return spec.active.charAt(0).toLocaleUpperCase('vi') + spec.active.slice(1)
}

/** Cụm việc đang làm, chữ thường để ghép sau tên trợ lý: “đang lưu dàn ý”. Trợ lý không có công cụ → “đang làm việc”. */
export function activeToolDoing(tool: string, variant = ''): string {
  if (!tool) return 'đang làm việc'
  const spec = specFor(tool, variant)
  if (!spec) return `đang ${readable(tool) || 'làm việc'}`
  return spec.active
}

// ─── Lời gọi công cụ trong chuỗi host ────────────────────────────────────

export type ToolCall = {
  tool: string
  chapter: number | null
  variant: string
  /** mảnh phụ đã là lời thường: “bản nháp”, “toàn truyện”, “gọt giũa”… */
  extras: string[]
  error: string
}

const VERDICT_INLINE: Record<string, string> = { accept: 'đạt', polish: 'gọt giũa', rewrite: 'viết lại' }

// `tool`, `tool[type]`, `tool(chương 8)`, `tool(chương 1 · bản nháp)`, `tool(toàn cục·polish)`, + ` lỗi: …`
const CALL = /^([a-z][a-z0-9]*(?:_[a-z0-9]+)+)(?:\[([a-z_]+)\]|\(([^()]*)\))?(?: lỗi:\s*([\s\S]*))?$/
const CHAPTER_IN = /(?:chương|Ch\.?|第)\s*(\d+)/i

function parseInner(tool: string, inner: string): { chapter: number | null; extras: string[] } {
  const parts = inner.split(/\s*·\s*/).map((part) => part.trim()).filter(Boolean)
  let chapter: number | null = null
  const extras: string[] = []
  for (const part of parts) {
    const chapterMatch = CHAPTER_IN.exec(part)
    if (chapterMatch && chapter === null) { chapter = Number(chapterMatch[1]); continue }
    if (/^(bản nháp|草稿)$/i.test(part)) { extras.push('bản nháp'); continue }
    if (/^thoại\s+/i.test(part)) { extras.push(part); continue }
    if (tool === 'save_review') {
      if (/^(toàn cục|全局)$/.test(part)) { extras.push('toàn truyện'); continue }
      if (/^(cung này|本弧)$/.test(part)) { extras.push('hồi này'); continue }
      if (VERDICT_INLINE[part]) { extras.push(VERDICT_INLINE[part]); continue }
    }
  }
  return { chapter, extras }
}

/** Chuỗi host dạng lời gọi công cụ → cấu trúc; chuỗi khác → null. */
export function parseToolSummary(summary: string): ToolCall | null {
  const match = CALL.exec(summary.trim())
  if (!match) return null
  const [, tool, variant = '', inner = '', error = ''] = match
  const { chapter, extras } = parseInner(tool, inner)
  return { tool, chapter, variant, extras, error: error.trim() }
}

/**
 * Cấu trúc → “Lưu chương · chương 4”; lỗi → “Lưu chương 5 không thành công[: {lý do}]” (Activity.dc.html).
 * `total` = tổng số chương của truyện: số chương lớn hơn tổng (model tự bịa, vd. `100` khi truyện có 64)
 * bị bỏ như canh `2147483647` — không bao giờ hiện số chương ngoài phạm vi.
 */
export function toolCallText(call: ToolCall, total?: number | null): string {
  const label = toolLabel(call.tool, call.variant)
  const chapter = call.chapter == null ? null : saneChapter(call.chapter, total)
  if (call.error) {
    const head = chapter && label.endsWith('chương') ? `${label} ${chapter}` : formatToolLabel(label, chapter)
    const reason = errorLabel(call.error)
    return [head, ...call.extras].join(' · ') + ' không thành công' + (reason ? `: ${reason}` : '')
  }
  return formatToolLabel(label, chapter, ...call.extras)
}

export type ToolEventLike = { summary: string; chapter?: number; args?: string; agent?: string }

function argsOf(args: string | undefined): { chapter?: number; type?: string } {
  if (!args) return {}
  try {
    const parsed = JSON.parse(args) as { chapter?: unknown; type?: unknown }
    return {
      chapter: typeof parsed.chapter === 'number' ? parsed.chapter : undefined,
      type: typeof parsed.type === 'string' ? parsed.type : undefined,
    }
  } catch { return {} }
}

/**
 * Dòng Nhật ký của một sự kiện công cụ: `summary` kiểu `commit_chapter(chương 8)` → “Lưu chương · chương 8”.
 * Phần thiếu (số chương, loại thiết lập) lấy từ `chapter`/`args` host gửi kèm.
 * Giao việc bỏ vai đã có ở nhãn người gửi; văn bản thường dùng chung bộ đổi tên vai/công cụ.
 * Chuỗi không cần đổi → null: người gọi dùng lại `summary` đã dịch.
 */
export function toolEventText(event: ToolEventLike, total?: number | null): string | null {
  const parsed = parseToolSummary(event.summary)
  if (!parsed) {
    const task = agentTaskOf(event.summary)
    if (task && event.agent && agentRoleLabel(task.role) === agentRoleLabel(event.agent)) return humanizeFreeText(task.task)
    const text = humanizeToolText(event.summary, total)
    return text === event.summary ? null : text
  }
  const fromArgs = argsOf(event.args)
  return toolCallText({
    ...parsed,
    chapter: parsed.chapter ?? fromArgs.chapter ?? null,
    variant: parsed.variant || fromArgs.type || '',
  }, total)
}

/** Nhãn chip công cụ (Nhật ký điện thoại, MLog.dc.html): luôn từ trường `tool` — “Lưu chương”. */
export function toolChipText(event: { tool: string; args?: string }): string {
  const call = parseToolSummary(event.tool)
  return toolLabel(call?.tool ?? event.tool, call?.variant || argsOf(event.args).type)
}

/**
 * Dịch mọi chuỗi host dạng tool, vai và giao việc: `writer → commit_chapter(chương 8)` →
 * “Người viết · đang lưu chương 8”; `editor (Thẩm duyệt…)` → “Biên tập viên · Thẩm duyệt…”.
 */
export function humanizeToolText(text: string, total?: number | null): string {
  const trimmed = text.trim()
  const arrow = /^([a-z_]+)\s*→\s*(.+)$/.exec(trimmed)
  if (arrow) {
    const call = parseToolSummary(arrow[2])
    if (call && agentRoleLabel(arrow[1])) {
      const chapter = call.chapter == null ? null : saneChapter(call.chapter, total)
      return agentProgressLine(arrow[1], call.tool, chapter, call.variant)
    }
    // Dòng giao việc không phải lời gọi công cụ vẫn đổi vai/công cụ trong phần văn bản bên dưới.
  }
  const call = parseToolSummary(trimmed)
  if (call) return toolCallText(call, total)
  if (/[\u3400-\u9fff]/.test(text)) return text
  const task = agentTaskOf(trimmed)
  return task ? `${agentLabel(task.role)} · ${humanizeFreeText(task.task)}` : humanizeFreeText(text)
}

// ─── Trường của khối lưu chương & các vòng ✻ ──────────────────────────────

const FIELDS: Record<string, string> = {
  chapter: 'Chương',
  chapterTo: 'Đến chương',
  scope: 'Phạm vi',
  volume: 'Tập',
  instruction: 'Góp ý',
  from: 'Chương',      // ghép với `to` → “Chương N–M”, xem chapterRangeLabel
  to: 'Chương',
  arc: 'Hồi',
  source: 'Bản',
  target_words: 'Độ dài mục tiêu',
  words: 'Số chữ',
  hook_type: 'Kiểu móc',
  dominant_strand: 'Tuyến chính',
  feedback: 'Góp ý',
  cast_intros: 'Nhân vật mới',
  characters: 'Nhân vật xuất hiện',
  foreshadow: 'Chi tiết cài cắm',
  foreshadow_updates: 'Chi tiết cài cắm',
  key_events: 'Sự kiện chính',
  // kế hoạch chương
  title: 'Tiêu đề',
  goal: 'Mục tiêu',
  conflict: 'Xung đột',
  hook: 'Móc cuối chương',
  emotion_arc: 'Mạch cảm xúc',
  opening_state: 'Trạng thái mở chương',
  emotion_target: 'Cảm xúc chính',
  hook_goal: 'Mục tiêu móc',
  notes: 'Ghi chú',
}

/** `from`/`to` của lời gọi công cụ → một dòng “Chương N–M”. */
export const chapterRangeLabel = (from: number, to: number): string => `Chương ${from}–${to}`

/**
 * Nhãn thường của trường. Trường kỹ thuật (`replace_all`, `max_runes`, id phục bút…)
 * và trường lạ → null: không hiện, chỉ nằm trong “Chi tiết kỹ thuật”.
 */
export const fieldLabel = (key: string): string | null => FIELDS[key] ?? null

export const HOOK_TYPE_LABELS: Record<string, string> = {
  // Bốn kiểu móc Master §7 (07-copy: không dùng “khủng hoảng”, “cliffhanger”)
  lat_thong_tin: 'lật thông tin', de_doa_moi: 'đe dọa mới', quyet_dinh_treo: 'quyết định treo', cau_thoai_no: 'câu thoại nổ',
  crisis: 'khủng hoảng', cliffhanger: 'treo', reveal: 'hé lộ', question: 'câu hỏi',
  mystery: 'bí ẩn', desire: 'khát vọng', emotion: 'cảm xúc', choice: 'lựa chọn',
}
export const STRAND_LABELS: Record<string, string> = {
  quest: 'nhiệm vụ', romance: 'tình cảm', mystery: 'bí ẩn', fire: 'tình cảm', constellation: 'nhiều tuyến',
}
export const FORESHADOW_ACTION_LABELS: Record<string, string> = { plant: 'Đã gieo', advance: 'Đẩy tiếp', resolve: 'Thu hồi', abandon: 'Bỏ tuyến' }

const enumValue = (map: Record<string, string>, value: string): string => {
  const key = value.trim().toLowerCase()
  return map[key] ?? (/^[a-z_]+$/.test(key) ? OTHER_VALUE : value)
}

export const hookTypeLabel = (value: string): string => enumValue(HOOK_TYPE_LABELS, value)
export const strandLabel = (value: string): string => enumValue(STRAND_LABELS, value)
export const foreshadowActionLabel = (value: string): string => enumValue(FORESHADOW_ACTION_LABELS, value)

/** Giá trị `source` của công cụ đọc: 'draft' → 'nháp'. */
const SOURCE_LABELS: Record<string, string> = { draft: 'nháp', polished: 'bản chính thức', committed: 'đã lưu' }

/** Giá trị trường đã dịch (hook_type, dominant_strand, source…); null/rỗng → null để người gọi tự chọn chữ thay. */
export function fieldValueLabel(key: string, value: string | null | undefined): string | null {
  const raw = value?.trim() ?? ''
  if (raw === '' || raw === 'null') return null
  if (['chapter', 'from', 'to', 'from_chapter', 'chapterTo'].includes(key)) return saneChapter(Number(raw)) ? raw : null
  if (key === 'scope') return ({ arc: 'cả cung', global: 'toàn truyện', chapter: 'một chương' } as Record<string, string>)[raw] ?? null
  if (key === 'hook_type') return hookTypeLabel(raw)
  if (key === 'dominant_strand') return strandLabel(raw)
  if (key === 'source') return SOURCE_LABELS[raw.toLowerCase()] ?? raw
  if (key === 'target_words') return /^\d+$/.test(raw) ? `${formatNumber(Number(raw))} chữ` : raw
  if (key === 'words' && /^\d+$/.test(raw)) return formatNumber(Number(raw))
  return raw
}

// ─── Thẩm duyệt (Trình đọc) ─────────────────────────────────────────────

const VERDICT_LABELS: Record<string, string> = { accept: 'Đạt', polish: 'Cần gọt giũa', rewrite: 'Viết lại' }
const SEVERITY_LABELS: Record<string, string> = { critical: 'nghiêm trọng', error: 'lỗi', warning: 'lưu ý' }
const CONTRACT_LABELS: Record<string, string> = { met: 'đạt', partial: 'một phần', missed: 'chưa đạt' }

const lookup = (map: Record<string, string>, value: string) => map[value.trim().toLowerCase()] ?? OTHER_VALUE

export const verdictLabel = (value: string): string => lookup(VERDICT_LABELS, value)
export const severityLabel = (value: string): string => lookup(SEVERITY_LABELS, value)
export const contractStatusLabel = (value: string): string => lookup(CONTRACT_LABELS, value)

// ─── Phán quyết, cảnh báo, lỗi của engine — 07-copy.md › Trạng thái ───────

const DECISION_LABELS: Record<string, string> = {
  retry: 'thử lại', reroute: 'giao trợ lý khác', abort: 'dừng hẳn',
}

/** Quyết định của Trọng tài; giá trị lạ → chữ thường có cách. */
export const decisionLabel = (value: string): string => DECISION_LABELS[value.trim().toLowerCase()] ?? readable(value)

const WARNING_LABELS: Record<string, string> = { stopguard: 'Bộ chống lặp', stop_guard: 'Bộ chống lặp' }

/** Nguồn/tên cảnh báo của engine (`StopGuard` → 'Bộ chống lặp'); lạ → chữ thường có cách. */
export const warningLabel = (value: string): string => WARNING_LABELS[value.trim().toLowerCase()] ?? readable(value)

const CACHE_EVENT_LABELS: Record<string, string> = {
  read: 'Đọc bộ nhớ đệm', write: 'Ghi bộ nhớ đệm', break: 'Lần mất bộ nhớ đệm', miss: 'Lần mất bộ nhớ đệm',
}

/** `cache read/write/break` → 'Đọc bộ nhớ đệm · Ghi bộ nhớ đệm · Lần mất bộ nhớ đệm'. */
export const cacheEventLabel = (value: string): string => CACHE_EVENT_LABELS[value.trim().toLowerCase()] ?? readable(value)

/** “{Trợ lý} đã dùng N% bộ nhớ ngữ cảnh (a/b) · sẽ tóm tắt phần cũ để viết tiếp”. `a`,`b` sẵn chuỗi đã format. */
export const contextMemoryNote = (agent: string, percent: number, used: string, total: string): string =>
  `${agentLabel(agent) || 'Trợ lý'} đã dùng ${percent}% bộ nhớ ngữ cảnh (${used}/${total}) · sẽ tóm tắt phần cũ để viết tiếp`

/** “Đã đổi model cho Người viết → {nhà cung cấp · model}”. `target` = 'provider/model' hoặc 'provider · model'. */
export function modelSwitchedNote(agent: string, target: string): string {
  return `Đã đổi model cho ${agentLabel(agent) || 'trợ lý'} → ${target.replace('/', ' · ')}`
}

/** Go ghi “chưa có chương” bằng `math.MaxInt32`; số này không bao giờ là chương thật. */
const CHAPTER_SENTINEL = 2147483647

/**
 * Số chương đọc được, hoặc null khi vô lý: không hữu hạn, <= 0, bằng giá trị canh của Go
 * (`2147483647`), hoặc lớn hơn tổng số chương đã biết. Dùng chung cho Luồng viết (thẻ bước)
 * và các chỗ hiện `currentChapter`/`inProgressChapter` trên giao diện.
 */
export function saneChapter(chapter: number, total?: number | null): number | null {
  if (!Number.isFinite(chapter) || chapter <= 0 || chapter >= CHAPTER_SENTINEL) return null
  if (total && total > 0 && chapter > total) return null
  return Math.trunc(chapter)
}

/**
 * Mã/thông báo lỗi kỹ thuật → câu thường: '429' → '{Nhà cung cấp} báo vượt hạn mức (mã 429)',
 * 'ENOSPC' → 'Ổ đĩa máy chủ đã đầy'. Chỉ giữ phần lời thường tiếng Việt trước lỗi gốc;
 * không nhận ra → rỗng. Chuỗi gốc vẫn nằm trong dữ liệu sự kiện để chép chi tiết kỹ thuật.
 */
export function errorLabel(raw: string | null | undefined, provider?: string): string {
  const text = (raw ?? '').trim()
  if (/\b429\b|too many requests|rate.?limit/i.test(text)) return `${provider || 'Nhà cung cấp'} báo vượt hạn mức (mã 429)`
  if (/\bENOSPC\b|no space left/i.test(text)) return 'Ổ đĩa máy chủ đã đầy'
  // Lỗi công cụ thật gặp trong nhật ký live (bug 2710): model gọi sai tên/thiếu tham số.
  if (/tool\s*"[^"]*"\s*not found/i.test(text)) return 'trợ lý gọi một công cụ không có trong danh sách'
  if (/InputValidationError|required parameter .* is missing/i.test(text)) return 'thiếu tham số bắt buộc'
  if (/context payload exceeds budget/i.test(text)) return 'ngữ cảnh vượt hạn mức, đã cắt bớt'
  if (/no content found for chapter\s*(\d+)/i.test(text)) {
    const chapter = /no content found for chapter\s*(\d+)/i.exec(text)?.[1]
    return `chưa có nội dung chương ${chapter}`
  }
  if (/chapter must be > 0/i.test(text)) return 'số chương chưa hợp lệ'
  const reason = text.split(':', 1)[0].trim()
  const technical = /[\\\/`{}[\]=]|[a-z0-9]_[a-z0-9_]|\b[\w-]+\.[a-z]{1,8}\b|--[\w-]+|\b[A-Z][A-Z0-9_]{2,}\b/
  return /[àáảãạăằắẳẵặâầấẩẫậèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵđ]/i.test(reason) && !technical.test(reason) ? reason : ''
}
