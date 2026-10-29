// Mock API Quy tắc viết (GAP-32) — hình dạng đúng 11-api-contract.md §6 “Quy tắc của truyện”:
//   POST /api/rules/understand · GET /api/rules/effective?scope=global|project · S/rules/{effective,apply,violations,runtime/:id[/keep]}
// Tệp `quy-tac.md` dùng chung kho `ruleStore` với GET/PUT /api/rules/file của server.ts (mỗi cấp một tệp, mỗi dòng `- …`).
import type { IncomingMessage, ServerResponse } from 'node:http'
import { setTimeout as delay } from 'node:timers/promises'
import type {
  ConfigScope, RuleChip, RuleItem, RuleStatus, RuleSnapshot, RulesEffective, RuntimeRule, RuleUnderstanding, RuleViolationsResponse,
} from '../src/api/types'
import { golden } from './golden'
export const RULES_FILE = 'quy-tac.md'

type Json = Record<string, unknown>
const send = (res: ServerResponse, status: number, data: unknown) => {
  const body = JSON.stringify(data)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(body) })
  res.end(body)
}
const fail = (res: ServerResponse, status: number, message: string) => send(res, status, { error: message })

/** Dòng của artboard (Settings.dc.html R0) theo cấp — dùng làm dữ liệu khởi tạo cho mọi dự án/truyện. */
const SEEDS: Record<ConfigScope, string[]> = {
  story: [
    'Kể ở ngôi thứ nhất, nhân vật chính luôn xưng “tôi”.',
    'Không dùng từ “bỗng nhiên” và “đột nhiên”.',
    'Mỗi chương kết bằng một tình huống hoặc câu hỏi còn bỏ ngỏ.',
    'Giọng châm biếm nhẹ, lời thoại chiếm khoảng một phần ba chương.',
  ],
  project: ['Thể loại showbiz, bối cảnh đô thị hiện đại.', 'Không dùng ký tự “~” và biểu tượng cảm xúc trong lời thoại.'],
  global: ['Viết tiếng Việt tự nhiên, hạn chế từ Hán Việt khó hiểu.'],
}
/** Cách hiểu của câu mơ hồ (artboard NEWRULE.fixes, bỏ “Giữ như lời dặn” — nút đó luôn có ở giao diện). */
const VAGUE_OPTIONS = ['Khoảng 3.000 chữ mỗi chương', 'Ngắn hơn chương 24 một chút']

const keyOf = (scope: ConfigScope, project: string, story: string) => `${scope}:${scope === 'global' ? '' : project}:${scope === 'story' ? story : ''}`

export function ruleFileKey(scope: ConfigScope, project: string, story: string): string {
  return `${keyOf(scope, project, story)}:${RULES_FILE}`
}

export const parseLines = (text: string): string[] =>
  text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line && !line.startsWith('#')).map((line) => line.replace(/^[-*]\s+/, '').trim()).filter(Boolean)

/** Đọc `quy-tac.md` của một cấp; lần đầu thì gieo dòng của artboard để màn hình có dữ liệu. */
function linesOf(store: Map<string, string>, scope: ConfigScope, project: string, story: string): string[] {
  const key = ruleFileKey(scope, project, story)
  if (!store.has(key)) store.set(key, SEEDS[scope].map((line) => `- ${line}`).join('\n') + '\n')
  return parseLines(store.get(key) ?? '')
}

const quoted = (text: string): string[] => [...text.matchAll(/[“‘'"]([^”’'"]{1,40})[”’'"]/g)].map((match) => match[1].trim()).filter(Boolean)

/** Chuẩn hoá giả lập (không model): nhận dạng vài mẫu câu thường gặp — đủ cho 6 trạng thái của artboard. */
export function understandText(text: string): Omit<RuleUnderstanding, 'structured' | 'preferences'> & { preferences: string } {
  const words = quoted(text)
  const chips: RuleChip[] = []
  let uncertain: string[] = []
  let options: string[] = []
  const limit = text.match(/(\d+)\s*lần/)?.[1]
  if (/quá dài|quá ngắn/i.test(text)) {
    chips.push({ kind: 'pref', text: 'Độ dài chương (chưa rõ)' })
    uncertain = ['Noverse chưa chắc “quá dài” là bao nhiêu. Chọn một cách hiểu, hoặc giữ nguyên như lời dặn.']
    options = VAGUE_OPTIONS
  } else if (words.length && limit && /tối đa|quá|lần/i.test(text)) {
    chips.push(...words.map((word): RuleChip => ({ kind: 'max', text: word, limit: Number(limit) })))
  } else if (words.length && /không dùng|đừng dùng|cấm|bỏ/i.test(text)) {
    chips.push(...words.map((word): RuleChip => ({ kind: /^.$/u.test(word) ? 'chr' : 'ban', text: word })))
  } else if (/thể loại/i.test(text)) {
    chips.push({ kind: 'genre', text: text.replace(/^.*thể loại\s*/i, '').replace(/[.,].*$/, '').trim() || 'Thể loại' })
  } else {
    chips.push({ kind: 'pref', text: /gọi|xưng/i.test(text) ? 'Xưng hô nhân vật' : text.replace(/[.!?]$/, '').slice(0, 48) })
  }
  return { chips, uncertain, options, status: 'ready' as RuleStatus, preferences: chips.filter((chip) => chip.kind === 'pref').map((chip) => chip.text).join('; ') }
}

/** Chip của các dòng khởi tạo, đúng như artboard (R0.u) — để mock không cần model thật. */
const SEED_CHIPS: Record<string, RuleChip[]> = {
  'Kể ở ngôi thứ nhất, nhân vật chính luôn xưng “tôi”.': [{ kind: 'pref', text: 'Giọng kể ngôi thứ nhất' }],
  'Không dùng từ “bỗng nhiên” và “đột nhiên”.': [{ kind: 'ban', text: 'bỗng nhiên' }, { kind: 'ban', text: 'đột nhiên' }],
  'Mỗi chương kết bằng một tình huống hoặc câu hỏi còn bỏ ngỏ.': [{ kind: 'pref', text: 'Móc cuối chương' }],
  'Giọng châm biếm nhẹ, lời thoại chiếm khoảng một phần ba chương.': [{ kind: 'pref', text: 'Giọng châm biếm nhẹ' }, { kind: 'pref', text: 'Thoại ~1/3' }],
  'Tri Hạ không bao giờ khóc trước mặt người khác.': [{ kind: 'pref', text: 'Nhân vật: Tri Hạ' }],
  '“dường như” chỉ dùng tối đa 1 lần mỗi chương.': [{ kind: 'max', text: 'dường như', limit: 1 }],
  'Thể loại showbiz, bối cảnh đô thị hiện đại.': [{ kind: 'genre', text: 'showbiz · đô thị' }],
  'Không dùng ký tự “~” và biểu tượng cảm xúc trong lời thoại.': [{ kind: 'chr', text: '~' }, { kind: 'pref', text: 'Không biểu tượng cảm xúc' }],
  'Viết tiếng Việt tự nhiên, hạn chế từ Hán Việt khó hiểu.': [{ kind: 'pref', text: 'Tiếng Việt tự nhiên' }],
}

const FAIL_MARK = /\[lỗi\]/i
/** Giả lập lỗi model (dòng chứa “[lỗi]”) → hạ cấp thành lời dặn nguyên văn, đúng `status:'degraded'` của backend. */
export function itemOf(text: string, scope: ConfigScope): RuleItem {
  if (FAIL_MARK.test(text)) return { text, scope, chips: [{ kind: 'pref', text }], uncertain: false, uncertainNote: '', options: [], status: 'degraded' }
  const out = understandText(text)
  // Câu 26a: items[] mang `uncertain` boolean + `uncertainNote` (danh sách lý do chỉ còn ở understand/runtime).
  return {
    text, scope, chips: SEED_CHIPS[text] ?? out.chips,
    uncertain: out.uncertain.length > 0, uncertainNote: out.uncertain[0] ?? '',
    options: out.options, status: 'ready',
  }
}

const snapshot: RuleSnapshot = { version: 1, status: 'ready', structured: {}, preferences: '', sources: ['system_defaults'], uncertain: [] }

type RuntimeState = { rules: RuntimeRule[]; appliedAt?: string }
const runtimes = new Map<string, RuntimeState>()
function runtimeOf(p: string, s: string): RuntimeState {
  const key = `${p}/${s}`
  let state = runtimes.get(key)
  if (!state) {
    const mk = (id: string, text: string, chapter: number, createdAt: string): RuntimeRule => {
      const item = itemOf(text, 'story')
      return {
        id, text, chapter, createdAt, chips: item.chips,
        uncertain: item.uncertainNote ? [item.uncertainNote] : [], uncertainNote: item.uncertainNote,
        options: item.options, status: item.status,
      }
    }
    state = { rules: [
      mk('rt-1', 'Tri Hạ không bao giờ khóc trước mặt người khác.', 12, '2026-10-21T09:12:00.000Z'),
      mk('rt-2', '“dường như” chỉ dùng tối đa 1 lần mỗi chương.', 20, new Date(Date.now() - 26 * 3600_000).toISOString()),
    ] }
    runtimes.set(key, state)
  }
  return state
}

export type RulesStoryContext = { completed: number; running: boolean }

export type RulesApiContext = {
  req: IncomingMessage
  res: ServerResponse
  p: string
  s: string
  /** path sau 'rules' */
  rest: string[]
  store: Map<string, string>
  story: RulesStoryContext
  canWrite: boolean
  readBody: () => Promise<Json>
  emit: () => void
}

export async function handleRulesApi(ctx: RulesApiContext): Promise<void> {
  const { req, res, p, s, rest, store, story, canWrite } = ctx
  const [head, id, action] = rest
  const timing = () => {
    const inProgress = story.running ? story.completed + 1 : 0
    return { effectiveFrom: story.completed + (story.running ? 2 : 1), inProgress, completed: story.completed }
  }
  if (head === 'effective' && req.method === 'GET') {
    const items: RuleItem[] = [
      ...linesOf(store, 'global', p, s).map((line) => itemOf(line, 'global')),
      ...linesOf(store, 'project', p, s).map((line) => itemOf(line, 'project')),
      ...linesOf(store, 'story', p, s).map((line) => itemOf(line, 'story')),
    ]
    const body: RulesEffective = { snapshot, items, runtime: runtimeOf(p, s).rules, timing: timing() }
    return send(res, 200, body)
  }
  if (head === 'apply' && req.method === 'POST') {
    if (!canWrite) return fail(res, 404, 'Không tìm thấy truyện')
    const state = runtimeOf(p, s)
    state.appliedAt = new Date().toISOString()
    ctx.emit()
    return send(res, 200, { snapshot, effectiveFrom: timing().effectiveFrom, ...(story.running ? { pinnedChapter: story.completed + 1 } : {}) })
  }
  if (head === 'violations' && req.method === 'GET') {
    const last = Number(new URL(req.url ?? '/', 'http://x').searchParams.get('last') ?? '3')
    if (!Number.isInteger(last) || last < 1 || last > 50) return fail(res, 400, 'Số chương phải từ 1 đến 50')
    // Golden do Go test ghi (RecentRuleViolations + thứ tự chương giảm dần);
    // mock chỉ cắt theo last như handler thật.
    const g = golden<RuleViolationsResponse>('rules-violations.json')
    return send(res, 200, { chapters: story.completed > 0 ? g.chapters.slice(0, last) : [] } satisfies RuleViolationsResponse)
  }
  if (head === 'runtime' && id) {
    if (!canWrite) return fail(res, 404, 'Không tìm thấy truyện')
    const state = runtimeOf(p, s)
    const found = state.rules.find((rule) => rule.id === id)
    if (!found) return fail(res, 404, 'Không tìm thấy quy tắc này')
    if (action === 'keep' && req.method === 'POST') {
      const key = ruleFileKey('story', p, s)
      const lines = linesOf(store, 'story', p, s)
      if (!lines.includes(found.text)) store.set(key, [...lines, found.text].map((line) => `- ${line}`).join('\n') + '\n')
      state.rules = state.rules.filter((rule) => rule.id !== id)
      ctx.emit()
      return send(res, 200, { ok: true, text: found.text, file: RULES_FILE })
    }
    if (!action && req.method === 'DELETE') {
      state.rules = state.rules.filter((rule) => rule.id !== id)
      ctx.emit()
      return send(res, 200, { ok: true })
    }
  }
  return fail(res, 404, 'Không có endpoint này')
}

/** `/api/rules/understand` và `/api/rules/effective?scope=global|project` (không gắn truyện). Trả true nếu đã xử lý. */
export async function handleRulesGlobal(ctx: {
  req: IncomingMessage; res: ServerResponse; seg: string[]; store: Map<string, string>; readBody: () => Promise<Json>
}): Promise<boolean> {
  const { req, res, seg, store } = ctx
  if (seg[1] === 'understand' && req.method === 'POST') {
    const body = await ctx.readBody()
    const text = String(body.text ?? '').trim()
    if (!text) { fail(res, 400, 'Quy tắc không được để trống'); return true }
    let closed = false
    req.on('close', () => { closed = true })
    await delay(1500)
    if (closed) return true
    if (FAIL_MARK.test(text)) { fail(res, 409, 'Chưa có model để đọc quy tắc. Vào Tuỳ chỉnh › Kết nối AI.'); return true }
    const out = understandText(text)
    send(res, 200, { structured: {}, preferences: out.preferences, uncertain: out.uncertain, uncertainNote: out.uncertain[0] ?? '', options: out.options, chips: out.chips, status: out.status } satisfies RuleUnderstanding)
    return true
  }
  if (seg[1] === 'effective' && req.method === 'GET') {
    const url = new URL(req.url ?? '/', 'http://x')
    const scope = (url.searchParams.get('scope') ?? 'global') as ConfigScope
    const project = url.searchParams.get('project') ?? ''
    if (scope === 'story') { fail(res, 400, 'Cấp truyện dùng /rules/effective của truyện'); return true }
    if (scope === 'project' && !project) { fail(res, 400, 'Thiếu dự án'); return true }
    const items: RuleItem[] = [
      ...linesOf(store, 'global', project, '').map((line) => itemOf(line, 'global')),
      ...(scope === 'project' ? linesOf(store, 'project', project, '').map((line) => itemOf(line, 'project')) : []),
    ]
    send(res, 200, { snapshot, items } satisfies RulesEffective)
    return true
  }
  return false
}
