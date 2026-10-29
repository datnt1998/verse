// Mô hình dùng chung cho popover `/` (Slash.dc.html) và trang Hướng dẫn lệnh (Commands.dc.html):
// tách dòng theo G12 (ngoặc kép/đơn), khớp lệnh không dấu, gợi ý Levenshtein, kiểm tra cú pháp
// phản chiếu backend (13-commands.md §1.3), và dựng “khung nhìn” cho cả hai màn.
// Dữ liệu cơ chế đến từ GET /api/commands (G11); chữ tiếng Việt ở i18n/commands.vi.ts.
import type { CommandParam, CommandSpec, Snapshot } from '../api/types'
import { parseHash } from '../router'
import {
  CMD_CHECK, CMD_GROUPS, CMD_ORDER, CMD_UI, copyOf, groupLabel, numSuggestions, paramCopy,
  type CmdCopy, type CmdExample,
} from '../i18n/commands.vi'

// ─── G12: tách tham số có ngoặc kép/đơn ─────────────────────────────────
export type Token = { value: string; quoted: boolean }

/**
 * Cổng của `commands.SplitArgs` (internal/commands/parse.go): khoảng trắng phân cách, `"…"` và `'…'`
 * gom phần có khoảng trắng (ngoặc bị bỏ, có thể dính tiền tố như `--guide="a b"`). Trong ngoặc,
 * `\` + (ngoặc đang mở | `\`) là ký tự thoát; ngoài ngoặc `\` giữ nguyên (đường dẫn Windows).
 * Ngoặc không đóng → `unterminated: true` (máy chủ trả lỗi “Cách dùng: …”); phần còn lại vẫn được
 * gom vào token cuối để gợi ý khi đang gõ.
 */
export function splitArgs(text: string): { tokens: Token[]; unterminated: boolean } {
  const tokens: Token[] = []
  let value = ''
  let quoted = false
  let inToken = false
  let quote = ''
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quote) {
      if (ch === '\\' && i + 1 < text.length && (text[i + 1] === quote || text[i + 1] === '\\')) {
        value += text[i + 1]
        i++
      } else if (ch === quote) {
        quote = ''
      } else {
        value += ch
      }
      continue
    }
    if (ch === ' ' || ch === '\t' || ch === '\r' || ch === '\n') {
      if (inToken) {
        tokens.push({ value, quoted })
        value = ''
        quoted = false
        inToken = false
      }
    } else if (ch === '"' || ch === "'") {
      quote = ch
      quoted = true
      inToken = true
    } else {
      value += ch
      inToken = true
    }
  }
  if (inToken) tokens.push({ value, quoted })
  return { tokens, unterminated: quote !== '' }
}

export const tokenize = (text: string): Token[] => splitArgs(text).tokens

/**
 * Đặt giá trị vào dòng lệnh sao cho `splitArgs` đọc lại đúng một token: bọc `"…"` khi có khoảng trắng
 * hoặc ngoặc, thoát `\` và `"` bên trong. Giá trị người dùng đã tự bọc ngoặc (đúng một token) giữ nguyên.
 */
export function quoteIfNeeded(value: string): string {
  const text = value.trim()
  if (/^["']/.test(text)) {
    const parsed = splitArgs(text)
    if (!parsed.unterminated && parsed.tokens.length === 1 && parsed.tokens[0].quoted) return text
  }
  if (!/[\s"']/.test(text)) return text
  return `"${text.replace(/[\\"]/g, '\\$&')}"`
}

// ─── Khớp không dấu ─────────────────────────────────────────────────────
/** Bỏ dấu tiếng Việt + hạ chữ thường để so khớp (`đ` → `d`). */
export function norm(text: string | null | undefined): string {
  return (text ?? '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd')
}

/** Khoảng cách Levenshtein — gợi ý “Có phải bạn muốn”. */
export function levenshtein(a: string, b: string): number {
  const prev = Array.from({ length: b.length + 1 }, (_, j) => j)
  const next = new Array<number>(b.length + 1)
  for (let i = 1; i <= a.length; i++) {
    next[0] = i
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      next[j] = Math.min(prev[j] + 1, next[j - 1] + 1, prev[j - 1] + cost)
    }
    for (let j = 0; j <= b.length; j++) prev[j] = next[j]
  }
  return prev[b.length]
}

// ─── Phân tích dòng ─────────────────────────────────────────────────────
export type ParsedLine =
  | { mode: 'none' }
  | { mode: 'list'; q: string }
  | { mode: 'args'; spec: CommandSpec; name: string; tokens: Token[]; current: string; unterminated: boolean }
  | { mode: 'unknown'; name: string; rest: string }

export function resolveSpec(name: string, specs: CommandSpec[]): CommandSpec | undefined {
  const key = name.toLowerCase()
  return specs.find((spec) => spec.name.toLowerCase() === key || spec.aliases.some((alias) => alias.toLowerCase() === key))
}

export function parseLine(line: string, specs: CommandSpec[]): ParsedLine {
  if (!line || line[0] !== '/') return { mode: 'none' }
  const body = line.slice(1)
  const space = body.search(/\s/)
  if (space < 0) return { mode: 'list', q: body }
  const name = body.slice(0, space)
  const rest = body.slice(space + 1)
  const spec = resolveSpec(name, specs)
  if (!spec) return { mode: 'unknown', name, rest }
  const { tokens, unterminated } = splitArgs(rest)
  // artboard: `rest.split(' ')` — dấu cách cuối mở một token rỗng (đang gõ tham số kế tiếp)
  if (!unterminated && rest !== '' && /\s$/.test(rest)) tokens.push({ value: '', quoted: false })
  return { mode: 'args', spec, name, tokens, current: tokens.length ? tokens[tokens.length - 1].value : '', unterminated }
}

// ─── Giá trị tham số ────────────────────────────────────────────────────
export type CommandValues = {
  values: Record<string, string | boolean>
  quoted: Record<string, boolean>
  extra: string
}

export function emptyValues(spec: CommandSpec): CommandValues {
  const values: Record<string, string | boolean> = {}
  const quoted: Record<string, boolean> = {}
  for (const param of spec.params) {
    values[param.key] = param.kind === 'flag' ? false : ''
    quoted[param.key] = false
  }
  return { values, quoted, extra: '' }
}

/** Đọc token → giá trị từng tham số (công tắc, tiền tố `tên=`, vị trí, chữ tự do). */
export function valuesOf(spec: CommandSpec, tokens: Token[]): CommandValues {
  const out = emptyValues(spec)
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i]
    if (!token.value) continue
    const flag = spec.params.find((param) => param.kind === 'flag' && param.flag === token.value)
    if (flag) { out.values[flag.key] = true; continue }
    const prefix = spec.params.find((param) => param.prefix && token.value.startsWith(param.prefix))
    if (prefix && prefix.prefix) {
      if (prefix.kind === 'text') {
        const tail = [token.value.slice(prefix.prefix.length), ...tokens.slice(i + 1).map((item) => item.value)]
        out.values[prefix.key] = tail.join(' ').trim()
        out.quoted[prefix.key] = token.quoted || tokens.slice(i + 1).some((item) => item.quoted)
        break
      }
      out.values[prefix.key] = token.value.slice(prefix.prefix.length)
      out.quoted[prefix.key] = token.quoted
      continue
    }
    const positional = spec.params.find((param) => !param.prefix && param.kind !== 'flag' && !out.values[param.key])
    if (positional) {
      if (positional.kind === 'text') {
        out.values[positional.key] = tokens.slice(i).map((item) => item.value).join(' ').trim()
        out.quoted[positional.key] = tokens.slice(i).some((item) => item.quoted)
        break
      }
      out.values[positional.key] = token.value
      out.quoted[positional.key] = token.quoted
      continue
    }
    if (!out.extra) out.extra = token.value
  }
  return out
}

/** Giá trị khởi tạo của bộ ghép lệnh: `init` trong i18n, công tắc = tắt. */
export function initialValues(spec: CommandSpec): CommandValues {
  const out = emptyValues(spec)
  for (const param of spec.params) {
    const init = paramCopy(spec, param.key)?.init
    if (init !== undefined) out.values[param.key] = init
  }
  return out
}

// ─── Dựng dòng lệnh + tô màu ────────────────────────────────────────────
export type Tone = 'name' | 'value' | 'flag'
export type LineToken = { text: string; tone: Tone }

export function composeTokens(spec: CommandSpec, values: CommandValues['values'], slash?: string): LineToken[] {
  const tokens: LineToken[] = [{ text: slash ?? `/${spec.name}`, tone: 'name' }]
  for (const param of spec.params) {
    const raw = values[param.key]
    if (param.kind === 'flag') {
      if (raw) tokens.push({ text: ` ${param.flag ?? param.token}`, tone: 'flag' })
      continue
    }
    const value = String(raw ?? '').trim()
    if (!value) continue
    // Tham số văn bản tự do (reopen, --guide=) ăn hết phần còn lại của dòng: artboard ghi nguyên chữ, không bọc ngoặc.
    const text = param.kind === 'text' && !/["'\\]/.test(value) ? value : quoteIfNeeded(value)
    if (param.prefix) {
      tokens.push({ text: ` ${param.prefix}`, tone: 'flag' })
      tokens.push({ text, tone: 'value' })
    } else {
      tokens.push({ text: ` ${text}`, tone: 'value' })
    }
  }
  return tokens
}

export const composeLine = (spec: CommandSpec, values: CommandValues['values'], slash?: string) =>
  composeTokens(spec, values, slash).map((token) => token.text).join('')

export type UsageTone = Tone | 'optional' | 'filled'
export type UsageToken = { text: string; tone: UsageTone; slot: boolean }

/** Tô màu chuỗi cú pháp theo 13 §3; tham số đã điền mờ đi, ô đang nhập có `.slot`. */
export function usageTokens(spec: CommandSpec, values?: CommandValues['values'], slotToken?: string | null): UsageToken[] {
  return spec.usage.split(' ').map((word, i) => {
    let tone: UsageTone = 'optional'
    if (i === 0) tone = 'name'
    else if (word.includes('--') || /^\[?\w+=/.test(word)) tone = 'flag'
    else if (word.startsWith('<') || (!word.startsWith('[') && word.includes('|'))) tone = 'value'
    const bare = word.replace(/^\[|\]$/g, '')
    const param = spec.params.find((item) => item.token.replace(/^\[|\]$/g, '') === bare)
    const filled = param && values ? Boolean(values[param.key]) : false
    return { text: word, tone: filled ? 'filled' : tone, slot: Boolean(param && slotToken && param.token === slotToken) }
  })
}

// ─── Kiểm tra cú pháp (13 §1.3) ─────────────────────────────────────────
export type CheckContext = {
  /** số chương đã hoàn thành — khoảng export hợp lệ */
  completedCount: number
  bookTitle: string
  isRunning: boolean
  /** chương đang viết, cho câu “Đang viết chương N” */
  writingChapter: number
  quoted?: Record<string, boolean>
  /** token thừa (dòng có nhiều phần hơn lệnh nhận) */
  extra?: string
  /** chưa tải được tiến độ truyện → bỏ qua kiểm tra khoảng chương */
  progressUnknown?: boolean
  /** dòng có ngoặc kép/đơn không đóng — máy chủ từ chối bằng “Cách dùng: …” */
  unterminated?: boolean
}
export type CheckResult = { err: string; warn: string; infos: string[]; fixExt: boolean }

export function checkLine(spec: CommandSpec, values: CommandValues['values'], ctx: CheckContext): CheckResult {
  let err = ctx.unterminated ? CMD_CHECK.unterminated(spec.usage) : ''
  let warn = ''
  const infos: string[] = []
  let fixExt = false
  const text = (key: string) => String(values[key] ?? '').trim()

  for (const param of spec.params) {
    if (param.required && !text(param.key)) {
      const label = paramCopy(spec, param.key)?.label ?? param.token
      err = err || CMD_CHECK.missing(label, param.token)
    }
  }
  for (const param of spec.params) {
    if (param.kind !== 'path') continue
    if (/\s/.test(text(param.key)) && !ctx.quoted?.[param.key]) err = err || CMD_CHECK.pathSpaces
  }
  // Đường dẫn không đặt ngoặc kép bị tách: phần đầu vào path, phần sau thành token thừa.
  if (ctx.extra && spec.params.some((param) => param.kind === 'path' && text(param.key))) {
    err = err || CMD_CHECK.pathSpaces
  }

  if (spec.name === 'export') {
    const path = text('path')
    const from = Number.parseInt(text('from'), 10)
    const to = Number.parseInt(text('to'), 10)
    const lo = Number.isNaN(from) ? 1 : from
    const hi = Number.isNaN(to) ? ctx.completedCount : to
    const ext = path && !path.endsWith('/') ? (/\.[^./]+$/.exec(path)?.[0].toLowerCase() ?? '') : ''
    if (!Number.isNaN(from) && !Number.isNaN(to) && from > to) err = err || CMD_CHECK.exportRange(from, to)
    else if (ext && ext !== '.txt' && ext !== '.epub') err = err || CMD_CHECK.exportExt(ext)
    else if (!ctx.progressUnknown && lo > ctx.completedCount) err = err || CMD_CHECK.exportEmpty(lo, hi, ctx.completedCount)
    if (!err) {
      const format = ext === '.epub' ? 'ebook EPUB' : 'văn bản TXT'
      const where = !path
        ? `thư mục truyện, tên “${ctx.bookTitle}.txt”`
        : path.endsWith('/') ? `thư mục ${path}, tên theo tên truyện` : path
      infos.push(CMD_CHECK.exportTarget(format, where))
      if (!ctx.progressUnknown) {
        const top = Math.min(hi, ctx.completedCount)
        infos.push(CMD_CHECK.exportRangeInfo(
          lo, top, ctx.completedCount + 1, hi > ctx.completedCount ? hi : null,
        ))
      }
      if (!values.ow && !values.keep) infos.push(CMD_CHECK.exportKeep)
      if (values.keep) infos.push(CMD_CHECK.exportKept)
    }
    fixExt = /\.pdf$|\.docx?$/i.test(path)
  }

  if (spec.name === 'import') {
    if (!text('path')) infos.push(CMD_CHECK.importResume)
    if (!values.yes) infos.push(CMD_CHECK.importPreview)
  }
  if (spec.name === 'importsim' && text('path') && !/\.json$/i.test(text('path'))) warn = CMD_CHECK.importsimExt
  if (spec.name === 'start') warn = CMD_CHECK.startOnly
  if (spec.name === 'review' && values.mode === 'on') infos.push(CMD_CHECK.reviewOn(ctx.writingChapter))
  if (spec.name === 'reopen' && ctx.bookTitle) infos.push(CMD_CHECK.reopenState(ctx.bookTitle))
  if (spec.name === 'sync' && values.check) infos.push(CMD_CHECK.syncCheck)
  if (ctx.isRunning && spec.needsIdle) warn = CMD_CHECK.busy(ctx.writingChapter)

  return { err, warn, infos, fixExt }
}

// ─── Lọc & gợi ý ────────────────────────────────────────────────────────
/** 3 = tên bắt đầu bằng · 2 = tên chứa · 1 = từ khoá/tiêu đề bắt đầu bằng · 0 = không khớp. */
export function scoreCommand(spec: CommandSpec, query: string): number {
  const q = norm(query)
  if (!q) return 1
  const names = [spec.name, ...spec.aliases].map(norm)
  if (names.some((name) => name.startsWith(q))) return 3
  if (names.some((name) => name.includes(q))) return 2
  const copy = copyOf(spec.name)
  const words = norm(`${copy?.title ?? ''} ${copy?.kw ?? ''}`).split(/\s+/)
  return words.some((word) => word.startsWith(q)) ? 1 : 0
}

export function filterCommands(specs: CommandSpec[], query: string): CommandSpec[] {
  return orderedSpecs(specs)
    .filter((spec) => scoreCommand(spec, query) > 0)
    .sort((a, b) => scoreCommand(b, query) - scoreCommand(a, query))
}

/** Tìm của trang Hướng dẫn: mọi từ khoá (không dấu) đều phải có trong tên/tiêu đề/lời dẫn/kw. */
export function matchesQuery(spec: CommandSpec, query: string): boolean {
  const q = norm(query).trim()
  if (!q) return true
  const copy = copyOf(spec.name)
  const hay = norm(['/' + spec.name, ...spec.aliases, copy?.title ?? '', copy?.lead ?? '', copy?.kw ?? ''].join(' '))
  return q.split(/\s+/).every((word) => hay.includes(word))
}

export function suggestCommands(specs: CommandSpec[], name: string, limit = 2): CommandSpec[] {
  return [...specs]
    .map((spec) => ({ spec, distance: levenshtein(norm(name), norm(spec.name)) }))
    .sort((a, b) => a.distance - b.distance)
    .slice(0, limit)
    .map((entry) => entry.spec)
}

/** Thứ tự artboard: help, model, config, …; lệnh lạ xếp cuối. */
export function orderedSpecs(specs: CommandSpec[]): CommandSpec[] {
  const rank = (spec: CommandSpec) => {
    const index = CMD_ORDER.indexOf(spec.name)
    return index < 0 ? CMD_ORDER.length : index
  }
  return [...specs].sort((a, b) => rank(a) - rank(b))
}

/** Nhóm theo thứ tự artboard (CMD_GROUPS); trong nhóm theo `cmds` của artboard, lệnh lạ xếp cuối nhóm. */
export function groupsOf(specs: CommandSpec[]): { key: string; label: string; specs: CommandSpec[] }[] {
  const inGroup = (spec: CommandSpec) => {
    const cmds = (CMD_GROUPS.find((group) => group.key === spec.group)?.cmds ?? []) as readonly string[]
    const index = cmds.indexOf(spec.name)
    return index < 0 ? cmds.length : index
  }
  const ordered = orderedSpecs(specs).sort((a, b) => inGroup(a) - inGroup(b))
  const keys = CMD_GROUPS.map((group) => group.key as string)
  for (const spec of ordered) if (!keys.includes(spec.group)) keys.push(spec.group)
  return keys
    .map((key) => ({ key, label: groupLabel(key), specs: ordered.filter((spec) => spec.group === key) }))
    .filter((group) => group.specs.length > 0)
}

// ─── Lệnh dùng gần đây (client, 13 §1.4) ────────────────────────────────
const RECENT_KEY = 'ainovel.commands.recent'
const RECENT_LIMIT = 3

export function readRecentCommands(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is string => typeof item === 'string').slice(0, RECENT_LIMIT)
  } catch { return [] }
}

export function pushRecentCommand(name: string): string[] {
  const next = [name, ...readRecentCommands().filter((item) => item !== name)].slice(0, RECENT_LIMIT)
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(next)) } catch { /* chế độ riêng tư */ }
  return next
}

// ─── Đường dẫn gần đây (client, theo truyện — 13 §1.4) ───────────────────
// `/importsim` dùng chung mọi truyện (gợi ý “hồ sơ đã dùng ở truyện khác”).
const PATHS_KEY = 'ainovel.commands.paths'
const PATHS_LIMIT = 3

function pathsScope(command: string): string {
  if (command === 'importsim') return '*'
  try {
    const route = parseHash(location.hash).route
    return route.name === 'workspace' ? `${route.project}/${route.story}` : ''
  } catch { return '' }
}

function readPathStore(): Record<string, Record<string, string[]>> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(PATHS_KEY) ?? '{}')
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, Record<string, string[]>> : {}
  } catch { return {} }
}

/** Đường dẫn gần đây của truyện đang mở: `{ export: [...], import: [...] }`. */
export function readRecentPaths(): Partial<Record<string, string[]>> {
  const store = readPathStore()
  const out: Record<string, string[]> = {}
  for (const command of ['export', 'import', 'start', 'importsim']) {
    const list = store[pathsScope(command)]?.[command]
    if (Array.isArray(list)) out[command] = list.filter((item): item is string => typeof item === 'string').slice(0, PATHS_LIMIT)
  }
  return out
}

/** Ghi đường dẫn của dòng lệnh vừa chạy (chỉ lệnh có tham số kiểu path). */
export function pushRecentPaths(line: string, specs: CommandSpec[]): void {
  const parsed = parseLine(line, specs)
  if (parsed.mode !== 'args') return
  const param = parsed.spec.params.find((item) => item.kind === 'path')
  if (!param) return
  const path = String(valuesOf(parsed.spec, parsed.tokens).values[param.key] ?? '').trim()
  if (!path) return
  const scope = pathsScope(parsed.spec.name)
  if (!scope) return
  const store = readPathStore()
  const mine = store[scope] ?? {}
  mine[parsed.spec.name] = [path, ...(mine[parsed.spec.name] ?? []).filter((item) => item !== path)].slice(0, PATHS_LIMIT)
  store[scope] = mine
  try { localStorage.setItem(PATHS_KEY, JSON.stringify(store)) } catch { /* chế độ riêng tư */ }
}

// ─── Khung nhìn của popover `/` ─────────────────────────────────────────
export function writingChapterOf(snapshot: Snapshot | null, completed: number): number {
  const chapter = snapshot?.inProgressChapter || snapshot?.currentChapter || 0
  return chapter > 0 ? chapter : completed + 1
}

export type SlashContext = {
  specs: CommandSpec[]
  completedCount: number
  bookTitle: string
  isRunning: boolean
  writingChapter: number
  recent: string[]
  recentPaths?: Partial<Record<string, string[]>>
  progressUnknown?: boolean
  /** Người dùng có quyền viết truyện này không — thiếu = có (popover không đổi gì). */
  writable?: boolean
  /** Người dùng đã bấm Esc để đóng popover của đúng dòng này: ẩn popover, giữ nguyên chữ đang gõ. */
  dismissed?: boolean
}

/** Lệnh cần quyền viết: ẩn khỏi gợi ý `/` và bảng lệnh khi người dùng chỉ đọc (brief 25/10). */
export const WRITE_COMMANDS: readonly string[] = ['reopen']

export const canRunCommand = (spec: CommandSpec, writable: boolean): boolean => writable || !WRITE_COMMANDS.includes(spec.name)

export function slashContext(
  snapshot: Snapshot | null,
  specs: CommandSpec[],
  recentPaths?: Partial<Record<string, string[]>>,
  flags?: { writable?: boolean; dismissed?: boolean },
): SlashContext {
  const completed = snapshot?.completedCount ?? 0
  return {
    specs,
    completedCount: completed,
    bookTitle: snapshot?.bookTitle ?? '',
    isRunning: Boolean(snapshot?.isRunning),
    writingChapter: writingChapterOf(snapshot, completed),
    recent: readRecentCommands(),
    recentPaths: recentPaths ?? readRecentPaths(),
    progressUnknown: !snapshot,
    writable: flags?.writable ?? true,
    dismissed: flags?.dismissed ?? false,
  }
}

export type SlashBadge = { icon: string; label: string; tone: 'neutral' | 'warn' | 'accent' | 'ok' }
export type SlashItem = {
  index: number
  /** tên lệnh (không dấu '/') — dùng cho aria và recent */
  key: string
  before: string
  match: string
  after: string
  desc: string
  tag?: string
  /** màu token của dòng gợi ý (tên lệnh để trống → tông tên) */
  tone?: Tone
  /** màu riêng của phần khớp/giá trị (artboard c2); trống → màu tên lệnh khi lọc, hoặc theo `tone` */
  matchTone?: UsageTone
  /** token gợi ý, dùng để so với phần đang gõ */
  token?: string
  /** dòng thay vào ô chat khi chọn */
  insert: string
  /** Enter chạy thẳng (lệnh chạy ngay, không tham số) */
  run?: string
}
export type SlashSection = { label: string; items: SlashItem[] }
export type SlashExample = { tokens: LineToken[]; text: string; insert: string }
export type SlashDetail = {
  slash: string
  title: string
  badges: SlashBadge[]
  lead: string
  usage: UsageToken[]
  slot: { title: string; desc: string; def: string } | null
  examples: SlashExample[]
  status: { tone: 'err' | 'warn' | 'ok'; icon: string; text: string } | null
  infos: string[]
  fix: { label: string } | null
  busyWarn: boolean
  tip: string
}
export type SlashView = {
  mode: 'list' | 'args' | 'unknown'
  line: string
  current: string
  title: string
  sub: string
  icon: string
  sections: SlashSection[]
  items: SlashItem[]
  empty: { title: string; text: string } | null
  enterHint: string
  hasError: boolean
  ghost: string
  detail: SlashDetail
  /** Enter chạy được nguyên dòng (args hợp lệ, không bị chặn) */
  submitLine: string | null
  /** chương đang viết — câu “Người viết đang viết chương N” */
  writingChapter: number
  /** lệnh đang xem — dùng để ghi “dùng gần đây” khi chạy */
  spec: CommandSpec
}

/** Nhãn trạng thái của lệnh — popover dùng bản ngắn, trang Hướng dẫn dùng bản dài. */
export function commandBadges(spec: CommandSpec, busy: boolean, long: boolean): SlashBadge[] {
  const badges: SlashBadge[] = []
  if (long && spec.aliases[0]) badges.push({ icon: 'swap', label: CMD_UI.alias(spec.aliases[0]), tone: 'accent' })
  badges.push(spec.autoExecute
    ? { icon: 'flash', label: long ? CMD_UI.autoLong : CMD_UI.auto, tone: 'neutral' }
    : { icon: 'edit', label: long ? CMD_UI.manualLong : CMD_UI.manual, tone: 'neutral' })
  if (spec.needsIdle) badges.push({ icon: 'timer', label: long ? CMD_UI.idleLong : CMD_UI.idle, tone: busy ? 'warn' : 'neutral' })
  if (spec.only) badges.push({ icon: 'info', label: CMD_UI.only[spec.only] ?? spec.only, tone: 'neutral' })
  if (!long && spec.aliases[0]) badges.push({ icon: 'swap', label: CMD_UI.aliasShort(spec.aliases[0]), tone: 'accent' })
  if (long && !spec.needsIdle && busy) badges.push({ icon: 'tick', label: CMD_UI.usableBusy, tone: 'ok' })
  return badges
}

function exampleOf(spec: CommandSpec, example: CmdExample): SlashExample {
  const values = emptyValues(spec).values
  for (const [key, value] of Object.entries(example.vals)) values[key] = value
  const tokens = composeTokens(spec, values, example.slash)
  return {
    tokens,
    text: example.text,
    insert: tokens.map((token) => token.text).join('') + (spec.params.length ? ' ' : ''),
  }
}

const clampIndex = (index: number, length: number) => (length === 0 ? 0 : Math.min(Math.max(index, 0), length - 1))

/** Dựng toàn bộ trạng thái popover cho một dòng + dòng đang chọn. */
export function buildSlashView(line: string, ctx: SlashContext, selected = 0): SlashView | null {
  if (!ctx.specs.length) return null
  // Esc đã đóng popover của dòng này: giữ chữ, không mở lại (17-kit › Phím Esc).
  if (ctx.dismissed) return null
  // Chỉ đọc: `/reopen` không nằm trong gợi ý; gõ tay thì báo rõ quyền.
  const writable = ctx.writable !== false
  const specs = writable ? ctx.specs : ctx.specs.filter((spec) => canRunCommand(spec, false))
  let parsed = parseLine(line, ctx.specs)
  if (parsed.mode === 'none') return null
  // Gõ đúng tên một lệnh cần quyền viết (`/reopen`, chưa có dấu cách → đang ở chế độ danh sách):
  // mở ngay thẻ lỗi quyền thay vì danh sách rỗng.
  if (!writable && parsed.mode === 'list') {
    const typed = parsed.q.trim().toLowerCase()
    const denied = ctx.specs.find((entry) => !canRunCommand(entry, false) && entry.name.toLowerCase() === typed)
    if (denied) parsed = { mode: 'args', spec: denied, name: denied.name, tokens: [], current: '', unterminated: false }
  }

  const items: SlashItem[] = []
  const sections: SlashSection[] = []
  let title = ''
  let sub = ''
  let icon = 'command'
  let empty: SlashView['empty'] = null
  let enterHint: string = CMD_UI.enterRun
  let hasError = false
  let ghost = ''
  let detail: SlashDetail | null = null
  let submitLine: string | null = null
  let spec: CommandSpec

  if (parsed.mode === 'list') {
    const q = norm(parsed.q)
    if (q) {
      const matched = filterCommands(specs, parsed.q)
      if (!matched.length) {
        const fallback = suggestCommands(specs, parsed.q)
        return buildUnknownView(line, parsed.q, '', fallback, ctx, selected)
      }
      const list = matched.map((entry, index) => listItem(entry, q, index))
      sections.push({ label: CMD_UI.matches(list.length), items: list })
      items.push(...list)
    } else {
      const recent = ctx.recent.filter((name) => specs.some((entry) => entry.name === name)).slice(0, 3)
      const recentSpecs = recent.map((name) => specs.find((entry) => entry.name === name)).filter((entry): entry is CommandSpec => Boolean(entry))
      if (recentSpecs.length) {
        const list = recentSpecs.map((entry, index) => listItem(entry, '', index))
        sections.push({ label: CMD_UI.recent, items: list })
        items.push(...list)
      }
      for (const group of groupsOf(specs)) {
        const rest = group.specs.filter((entry) => !recent.includes(entry.name))
        if (!rest.length) continue
        const list = rest.map((entry, index) => listItem(entry, '', items.length + index))
        sections.push({ label: group.label, items: list })
        items.push(...list)
      }
    }
    if (!items.length) {
      empty = { title: CMD_UI.emptyList(parsed.q), text: CMD_UI.emptyListHint }
      enterHint = CMD_UI.enterSend
      submitLine = line.trim()
    }
    const selectedItem = items[clampIndex(selected, items.length)]
    spec = specs.find((entry) => entry.name === selectedItem?.key) ?? specs[0] ?? ctx.specs[0]
    const copy = copyOf(spec.name)
    // artboard: ghost chỉ khi TÊN CHÍNH bắt đầu bằng phần đang gõ (gõ /pl → /plan thì Tab điền /cocreate, nên không ghost)
    if (q && selectedItem && norm(spec.name).startsWith(q)) ghost = spec.name.slice(q.length)
    if (selectedItem) enterHint = spec.params.length && !spec.autoExecute ? CMD_UI.enterFill : CMD_UI.enterRun
    title = q ? CMD_UI.popFilterTitle(parsed.q) : CMD_UI.popListTitle
    sub = q ? '' : CMD_UI.popListSub(specs.length)
    detail = {
      slash: '/' + spec.name,
      title: copy?.title ?? spec.usage,
      badges: commandBadges(spec, ctx.isRunning, false),
      lead: copy?.lead ?? '',
      usage: usageTokens(spec),
      slot: null,
      examples: (copy?.ex ?? []).slice(0, 3).map((example) => exampleOf(spec, example)),
      status: null,
      infos: [],
      fix: null,
      busyWarn: ctx.isRunning && spec.needsIdle,
      tip: copy?.tip ?? '',
    }
  } else if (parsed.mode === 'unknown') {
    return buildUnknownView(line, parsed.name, parsed.rest, suggestCommands(specs, parsed.name), ctx, selected)
  } else {
    spec = parsed.spec
    const copy = copyOf(spec.name)
    const done = parsed.tokens.slice(0, -1)
    const doneValues = valuesOf(spec, done)
    const full = valuesOf(spec, parsed.tokens)
    const current = parsed.current
    const textParam = spec.params.find((param) => param.kind === 'text' && Boolean(doneValues.values[param.key]))
    let slot: CommandParam | null = null
    let valueOf: string | null = null
    if (textParam) slot = textParam
    else {
      const prefix = spec.params.find((param) => param.prefix && current.startsWith(param.prefix))
      if (prefix) { slot = prefix; valueOf = current.slice(prefix.prefix?.length ?? 0) }
      else if (current.startsWith('-')) {
        slot = spec.params.find((param) => param.kind === 'flag' && (param.flag ?? '').startsWith(current))
          ?? spec.params.find((param) => param.prefix && param.prefix.startsWith(current))
          ?? null
      } else {
        slot = spec.params.find((param) => !param.prefix && param.kind !== 'flag' && !doneValues.values[param.key]) ?? null
      }
    }

    const base = done.length ? `/${parsed.name} ${done.map((token) => (token.value ? quoteIfNeeded(token.value) : '""')).join(' ')} ` : `/${parsed.name} `
    const put = (insert: string) => base + insert + (/=$/.test(insert) ? '' : ' ')

    if (slot && valueOf !== null && slot.kind !== 'text') {
      const options = slot.kind === 'enum'
        ? (slot.options ?? []).map((option) => ({ value: option.value, note: option.note }))
        : numSuggestions(slot.key, ctx.completedCount)
      for (const option of options.filter((entry) => entry.value.startsWith(valueOf ?? ''))) {
        items.push({
          index: items.length, key: spec.name,
          before: slot.prefix ?? '', match: option.value, after: '',
          desc: option.note,
          tone: 'flag', matchTone: 'value',
          token: (slot.prefix ?? '') + option.value,
          insert: put((slot.prefix ?? '') + option.value),
        })
      }
    } else if (!textParam) {
      for (const param of spec.params) {
        const paramLabel = paramCopy(spec, param.key)
        if (param.kind === 'flag') {
          const flag = param.flag ?? param.token
          if (!doneValues.values[param.key] && flag.startsWith(current)) {
            items.push({
              index: items.length, key: spec.name, before: flag, match: '', after: '',
              desc: paramLabel?.hint ?? paramLabel?.desc ?? param.token,
              tag: CMD_UI.tagSwitch, tone: 'flag', token: flag, insert: put(flag),
            })
          }
          continue
        }
        if (doneValues.values[param.key]) continue
        if (param.prefix) {
          if (!current || param.prefix.startsWith(current)) {
            items.push({
              index: items.length, key: spec.name,
              before: param.prefix,
              match: param.kind === 'text' ? '…' : param.kind === 'enum' ? (param.options ?? []).map((option) => option.value).join('|') : 'N',
              after: '', desc: paramLabel?.label ?? param.token,
              tone: 'flag', matchTone: 'optional', token: param.prefix, insert: put(param.prefix),
            })
          }
          continue
        }
        if (param.kind === 'enum') {
          for (const option of (param.options ?? []).filter((entry) => entry.value.startsWith(current))) {
            items.push({
              index: items.length, key: spec.name, before: option.value, match: '', after: '',
              desc: option.note, tone: 'value', token: option.value, insert: put(option.value),
            })
          }
        } else if (param.kind === 'path') {
          for (const path of (ctx.recentPaths?.[spec.name] ?? []).filter((entry) => entry.startsWith(current))) {
            items.push({
              index: items.length, key: spec.name, before: path, match: '', after: '',
              desc: CMD_UI.recentPathNote(spec.name, path), tag: CMD_UI.tagSuggestion, tone: 'value', token: path, insert: put(quoteIfNeeded(path)),
            })
          }
        }
      }
    }

    const check = checkLine(spec, full.values, { ...ctx, quoted: full.quoted, extra: full.extra, unterminated: parsed.unterminated })
    if (full.extra) check.err = check.err || CMD_UI.extraValue(full.extra)
    // Gõ tay `/reopen` khi chỉ có quyền đọc: thẻ lỗi rõ ràng, không cho Enter chạy (brief 25/10).
    if (!writable && !canRunCommand(spec, false)) check.err = check.err || CMD_CHECK.readOnly
    const blocked = ctx.isRunning && spec.needsIdle

    if (items.length) {
      const label = slot && valueOf !== null
        ? CMD_UI.valuesFor((slot.prefix ?? slot.token).replace(/=$/, ''))
        : CMD_UI.canAdd
      sections.push({ label, items })
    } else if (textParam) {
      empty = { title: CMD_UI.emptyTyping(paramCopy(spec, textParam.key)?.label ?? textParam.token), text: CMD_UI.emptyTypingHint }
    } else {
      // Chỉ báo “đủ thông tin · Enter để chạy” khi Enter thật sự gửi được (13-commands.md §1.2–1.3).
      const missing = spec.params.find((param) => param.required && !String(full.values[param.key] ?? '').trim())
      empty = {
        title: missing ? CMD_UI.emptyTyping(paramCopy(spec, missing.key)?.label ?? missing.token) : CMD_UI.emptyDone,
        text: missing ? (check.err || CMD_UI.emptyDoneHint) : CMD_UI.emptyDoneHint,
      }
    }

    const selectedItem = items[clampIndex(selected, items.length)]
    if (selectedItem?.token && current && selectedItem.token.startsWith(current)) ghost = selectedItem.token.slice(current.length)
    else if (!current && !textParam) {
      const rest = spec.params
        .filter((param) => !doneValues.values[param.key])
        .map((param) => param.kind === 'flag'
          ? `[${param.flag ?? param.token}]`
          : param.required ? param.token : `[${param.token.replace(/^\[|\]$/g, '')}]`)
      ghost = rest.join(' ')
    }

    hasError = Boolean(check.err)
    submitLine = !check.err && !blocked ? line.trim() : null
    enterHint = check.err ? CMD_UI.enterBlocked : CMD_UI.enterCommand
    title = CMD_UI.popArgsTitle(spec.name)
    sub = copy?.title ?? ''
    icon = 'edit'

    const status: SlashDetail['status'] = check.err
      ? { tone: 'err', icon: 'warning', text: check.err }
      : blocked ? { tone: 'warn', icon: 'timer', text: CMD_UI.rejected }
        : check.warn ? { tone: 'warn', icon: 'info', text: check.warn }
          : { tone: 'ok', icon: 'tick', text: CMD_UI.okSyntax }

    detail = {
      slash: '/' + spec.name,
      title: copy?.title ?? spec.usage,
      badges: commandBadges(spec, ctx.isRunning, false),
      lead: copy?.lead ?? '',
      usage: usageTokens(spec, doneValues.values, slot?.token ?? null),
      slot: slot ? {
        title: CMD_UI.slotTitle(paramCopy(spec, slot.key)?.label ?? slot.token, slot.required),
        desc: paramCopy(spec, slot.key)?.desc ?? '',
        def: paramCopy(spec, slot.key)?.defText ?? '',
      } : null,
      examples: [],
      status,
      infos: check.err ? [] : check.infos,
      fix: check.fixExt ? { label: CMD_UI.fixEpub } : null,
      busyWarn: blocked,
      tip: copy?.tip ?? '',
    }
  }

  return {
    mode: parsed.mode === 'list' ? 'list' : parsed.mode === 'args' ? 'args' : 'unknown',
    line,
    current: parsed.mode === 'args' ? parsed.current : '',
    title, sub, icon, sections, items, empty, enterHint, hasError, ghost,
    detail: detail ?? {
      slash: '', title: '', badges: [], lead: '', usage: [], slot: null,
      examples: [], status: null, infos: [], fix: null, busyWarn: false, tip: '',
    },
    submitLine,
    writingChapter: ctx.writingChapter,
    spec,
  }
}

function listItem(spec: CommandSpec, q: string, index: number): SlashItem {
  const copy = copyOf(spec.name)
  const names = [spec.name, ...spec.aliases]
  const matchedName = q ? (names.find((name) => norm(name).startsWith(q)) ?? spec.name) : spec.name
  const at = q ? matchedName.toLowerCase().indexOf(q) : -1
  return {
    index,
    key: spec.name,
    before: '/' + (at >= 0 ? matchedName.slice(0, at) : matchedName),
    match: at >= 0 ? matchedName.slice(at, at + q.length) : '',
    after: at >= 0 ? matchedName.slice(at + q.length) : '',
    desc: copy?.title ?? spec.usage,
    tag: spec.needsIdle ? CMD_UI.tagIdle : undefined,
    insert: `/${spec.name} `,
    run: spec.autoExecute && spec.params.length === 0 ? `/${spec.name}` : undefined,
  }
}

function buildUnknownView(
  line: string, name: string, rest: string, suggestions: CommandSpec[], ctx: SlashContext, selected: number,
): SlashView | null {
  if (!suggestions.length) return null
  const items: SlashItem[] = suggestions.map((spec, index) => ({
    index,
    key: spec.name,
    before: '/',
    match: spec.name,
    after: '',
    desc: copyOf(spec.name)?.title ?? spec.usage,
    insert: `/${spec.name}${rest ? ' ' + rest : ' '}`,
  }))
  const chosen = suggestions[clampIndex(selected, suggestions.length)]
  const copy: CmdCopy | undefined = copyOf(chosen.name)
  return {
    mode: 'unknown',
    line,
    current: '',
    title: CMD_UI.popUnknownTitle(name),
    sub: CMD_UI.popUnknownSub,
    icon: 'warning',
    sections: [{ label: CMD_UI.didYouMean, items }],
    items,
    empty: null,
    enterHint: CMD_UI.enterFix(chosen.name),
    hasError: true,
    ghost: '',
    detail: {
      slash: '/' + chosen.name,
      title: copy?.title ?? chosen.usage,
      badges: commandBadges(chosen, ctx.isRunning, false),
      lead: `Bạn gõ /${name} — có thể bạn muốn /${chosen.name}. ${copy?.lead ?? ''}`.trim(),
      usage: usageTokens(chosen),
      slot: null,
      examples: [],
      status: null,
      infos: [],
      fix: null,
      busyWarn: false,
      tip: CMD_UI.unknownTip(name),
    },
    submitLine: null,
    writingChapter: ctx.writingChapter,
    spec: chosen,
  }
}

/** Enter / nút “chạy” của popover. */
export function acceptSlash(view: SlashView, selected: number): { next?: string; submit?: string } {
  const item = view.items[clampIndex(selected, view.items.length)]
  if (view.mode === 'list') {
    if (!item) return view.submitLine ? { submit: view.submitLine } : {}
    return item.run ? { submit: item.run } : { next: item.insert }
  }
  if (view.mode === 'unknown') return item ? { next: item.insert } : {}
  if (item && view.current && item.token && item.token !== view.current) return { next: item.insert }
  return view.submitLine ? { submit: view.submitLine } : {}
}

/** Tab: luôn chỉ điền, không chạy. */
export function tabSlash(view: SlashView, selected: number): { next?: string } {
  const item = view.items[clampIndex(selected, view.items.length)]
  return item ? { next: item.insert } : {}
}

/** Phần còn lại của tên lệnh để hiện mờ trong ô chat. */
export function slashGhost(line: string, ctx: SlashContext, selected = 0): string {
  return buildSlashView(line, ctx, selected)?.ghost ?? ''
}

/** Ô chat có lỗi cú pháp → viền đỏ + chip “Chưa chạy được”. */
export function slashHasError(line: string, ctx: SlashContext, selected = 0): boolean {
  return buildSlashView(line, ctx, selected)?.hasError ?? false
}
