import type { ActivityEvent, ActivityHistory, ActivityHistoryQuery } from '../../../api/types'
import { mergeLogEvents } from '../../../api/useActivityHistory'
import { translateHost } from '../../../i18n/hostStrings'
import { activityText, isUserEvent } from '../../../lib/activity'
import { errorLabel, fieldLabel, fieldValueLabel, isKnownTool, parseToolSummary, saneChapter, toolChipText, toolIcon } from '../../../lib/labels'
import { formatNumber } from '../../../lib/format'
import { norm } from '../../../commands/model'
import { t } from '../../../i18n/vi'

export type LogFilter = 'all' | 'writer' | 'architect' | 'editor' | 'arbiter' | 'error' | 'me'
export const LOG_FILTERS: { id: LogFilter; label: string }[] = [
  { id: 'all', label: 'Tất cả' }, { id: 'writer', label: 'Người viết' },
  { id: 'architect', label: 'Kiến trúc sư' }, { id: 'editor', label: 'Biên tập' },
  { id: 'arbiter', label: 'Trọng tài' }, { id: 'error', label: 'Lỗi' }, { id: 'me', label: 'Của bạn' },
]
export const logAgent = (agent: string) => agent === 'architect_long' || agent === 'architect_short' ? 'architect' : agent
export const isLogError = (event: Pick<ActivityEvent, 'failed' | 'level' | 'status'>) => event.failed || event.level === 'error' || event.status === 'error'
export const isMine = (event: ActivityEvent, username?: string) => username ? event.username === username : isUserEvent(event)
export function logQuery(filter: LogFilter, q: string): ActivityHistoryQuery {
  return { ...(filter === 'error' ? { level: 'error' } : filter === 'me' ? { mine: true } : filter !== 'all' ? { agent: filter } : {}), ...(q.trim() ? { q: q.trim() } : {}), limit: 20 }
}
/**
 * MLog.dc.html › `state=review`: mở sẵn bộ lọc “Biên tập” khi vào bằng `?loc=review`
 * (cùng hash route với nhật ký máy tính). Không có tham số → “Tất cả”.
 */
export function initialLogFilter(href: string): LogFilter {
  const url = new URL(href, 'http://local')
  const query = url.hash.includes('?') ? url.hash.slice(url.hash.indexOf('?') + 1) : url.search.slice(1)
  return new URLSearchParams(query).get('loc') === 'review' ? 'editor' : 'all'
}
export function matchesLog(event: ActivityEvent, filter: LogFilter, q: string, username?: string): boolean {
  if (filter === 'error' ? !isLogError(event) && !event.retries?.length : filter === 'me' ? !isMine(event, username) : filter !== 'all' && logAgent(event.agent) !== filter) return false
  return !q.trim() || norm(`${activityText(event)} ${event.tool ? toolChipText({ tool: event.tool, args: event.args }) : ''} ${event.args ?? ''} ${event.result ?? ''}`).includes(norm(q.trim()))
}

export function logGroupKey(event: ActivityEvent): string {
  return event.scope === 'arc'
    ? `arc/${event.volume ?? 0}/${event.arc ?? 0}/${event.chapterFrom ?? 0}/${event.chapterTo ?? event.chapter}`
    : `chapter/${event.chapter || 0}`
}
type LogGroup = { key: string; label: string; chapter: number; title: string; score?: number; count: number; live: boolean; work: string; agent?: string; events: ActivityEvent[] }
export function groupChapters(events: ActivityEvent[], chapters: ActivityHistory['chapters'], currentChapter: number, running: boolean) {
  const metadata = new Map(chapters.map((chapter) => [chapter.chapter, chapter]))
  const ordered = mergeLogEvents(events)
  const active = running ? ordered.find(event => event.category === 'DISPATCH' && !isLogError(event) && (event.status === 'running' || !event.finishedAt || Date.parse(event.finishedAt) <= 0))
    ?? ordered.find(event => event.status === 'running') : undefined
  const liveKey = active ? logGroupKey(active) : `chapter/${currentChapter}`
  const groups = new Map<string, LogGroup>()
  for (const event of ordered) {
    const chapter = event.chapter || 0
    const key = logGroupKey(event)
    let group = groups.get(key)
    if (!group) {
      const meta = metadata.get(chapter)
      const title = meta?.title?.replace(/^Chương\s+\d+\s*[:·.-]?\s*/i, '').trim() || t.activity.untitledChapter
      const arc = event.scope === 'arc'
      const live = running && key === liveKey && (chapter > 0 || arc)
      const work = /viết lại|\"mode\"\s*:\s*\"rewrite\"/i.test(active?.summary ?? '') || /\"mode\"\s*:\s*\"rewrite\"/.test(active?.args ?? '') ? t.activity.rewriting
        : /gọt giũa|edit_chapter/i.test(`${active?.summary ?? ''} ${active?.tool ?? ''}`) ? t.activity.polishing
        : /thẩm duyệt/i.test(active?.summary ?? '') ? t.activity.reviewing
        : active?.agent === 'writer' || !active ? t.activity.writing : t.activity.working
      group = { key, chapter, title, label: arc ? t.activity.arcGroup(event.arc, event.chapterFrom, event.chapterTo) : chapter ? t.activity.chapterGroup(chapter, title) : t.activity.generalActivity,
        score: arc ? undefined : meta?.score, count: arc ? 0 : meta?.eventCount ?? 0, live, work, agent: logAgent((active ?? event).agent), events: [] }
      groups.set(key, group)
    }
    group.events.push(event)
  }
  // Insertion order follows the newest event, including multi-chapter work.
  return [...groups.values()]
}

/** Legacy dispatch instructions can name several tools; each has its own card. */
export function eventToolCards(event: ActivityEvent): ActivityEvent[] {
  if (event.tool) return [event]
  if (event.category !== 'DISPATCH') return []
  const matches = [...event.summary.matchAll(/\b[a-z]+(?:_[a-z]+)+\b/g)]
  const seen = new Set<string>()
  return matches.flatMap((match, index) => {
    const tool = match[0]
    if (!isKnownTool(tool) || seen.has(tool)) return []
    seen.add(tool)
    const details = event.summary.slice(match.index! + tool.length, matches[index + 1]?.index)
    const parameters = Object.fromEntries([...details.matchAll(/\b(chapter|scope|volume|arc)=([a-z]+|\d+)/g)].map(([, key, value]) => [key, /^\d+$/.test(value) ? Number(value) : value]))
    return [{ ...event, scope: typeof parameters.scope === 'string' ? parameters.scope : undefined, tool, summary: tool, args: JSON.stringify(parameters), status: undefined, result: undefined, durationMs: 0, durationNs: 0, failed: false, level: 'info' }]
  })
}

/** Pending IDs, not raw event updates: retries and replay never inflate the jump pill. */
export function nextUnseenEvents(known: ReadonlySet<string>, pending: ReadonlySet<string>, incoming: ActivityEvent[], away: boolean, since: number): { known: Set<string>; pending: Set<string> } {
  const nextKnown = new Set(known)
  const nextPending = away ? new Set(pending) : new Set<string>()
  for (const event of incoming) {
    if (away && !nextKnown.has(event.id) && Date.parse(event.time) > since) nextPending.add(event.id)
    nextKnown.add(event.id)
  }
  return { known: nextKnown, pending: nextPending }
}
export function highlightedParts(text: string, query: string): { text: string; match: boolean }[] {
  text = text.normalize('NFC')
  const needle = norm(query.trim())
  if (!needle) return [{ text, match: false }]
  const lower = norm(text)
  const parts: { text: string; match: boolean }[] = []
  let from = 0
  for (let at = lower.indexOf(needle); at >= 0; at = lower.indexOf(needle, from)) {
    if (at > from) parts.push({ text: text.slice(from, at), match: false })
    parts.push({ text: text.slice(at, at + needle.length), match: true })
    from = at + needle.length
  }
  if (from < text.length) parts.push({ text: text.slice(from), match: false })
  return parts
}

const IDENT = /^[A-Za-z_$][\w$]*$/
const INLINE_MAX = 72
function inline(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(inline).join(', ')}]`
  if (value && typeof value === 'object') {
    const entries = Object.entries(value)
    return entries.length ? `{ ${entries.map(([key, item]) => `${IDENT.test(key) ? key : JSON.stringify(key)}: ${inline(item)}`).join(', ')} }` : '{}'
  }
  return JSON.stringify(value) ?? 'null'
}
/** Khối “Chi tiết kỹ thuật” của M6: `tên({ tham số })` rồi `→ kết quả`, mỗi lần thử lại một dòng `↻`. Ngắn thì một dòng, dài thì mỗi tham số một dòng. */
export function toolDetailText(event: Pick<ActivityEvent, 'tool' | 'args' | 'result' | 'status' | 'retries'> & Partial<Pick<ActivityEvent, 'summary'>>): string {
  const name = event.tool ?? ''
  let parsed: unknown
  try { parsed = event.args ? JSON.parse(event.args) : undefined } catch { parsed = undefined }
  let args = event.args ?? ''
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    const entries = Object.entries(parsed)
    const one = inline(parsed)
    args = !entries.length ? '' : one.length + name.length + 2 <= INLINE_MAX ? one : `{\n${entries.map(([key, item]) => `  ${IDENT.test(key) ? key : JSON.stringify(key)}: ${inline(item)}`).join(',\n')}\n}`
  }
  const result = event.result || parseToolSummary(event.summary ?? '')?.error || (event.status === 'running' ? 'đang chạy…' : 'Không có kết quả trả về')
  const retries = (event.retries ?? []).map((retry, i) => `↻ thử lại lần ${i + 1} sau ${retry.waitSec}s → ${retry.ok ? 'OK' : 'chưa được'}`)
  return [`${name}(${args})`, `${event.status === 'error' ? '✗' : '→'} ${result}`, ...retries].join('\n')
}

// MLog.dc.html › .i-shield = shield-security.svg (khác Activity.dc.html › shield-tick — icon theo từng artboard).
const MLOG_TOOL_ICONS: Record<string, string> = { check_consistency: 'shield-security', audit_foundation: 'shield-security' }
/** Icon công cụ trên MLog: bản ghi cục bộ theo artboard, còn lại dùng bảng chung `toolIcon`. */
export const mlogToolIcon = (tool: string): string => MLOG_TOOL_ICONS[tool] ?? toolIcon(tool)

// ─── Thẻ chi tiết công cụ (Gói J mục 5 — MLog.dc.html › .toolio) ───────────

const WORD_KEYS = new Set(['target_words', 'words'])

function argText(key: string, raw: unknown): string | null {
  if (raw === null || raw === undefined || raw === '') return null
  if (typeof raw === 'number') return WORD_KEYS.has(key) ? `${formatNumber(raw)} chữ` : formatNumber(raw)
  if (typeof raw === 'boolean') return raw ? 'Có' : 'Không'
  if (Array.isArray(raw)) {
    const parts = raw.map((item) => argText(key, item)).filter((part): part is string => !!part)
    return parts.length ? parts.join(' · ') : null
  }
  if (typeof raw === 'object') return null
  const text = String(raw).trim()
  return text ? (fieldValueLabel(key, text) ?? translateHost(text)) : null
}

/** `event.args` (JSON) → các dòng “nhãn · giá trị”. `from`/`to` gộp thành “Chương N–M”; khóa kỹ thuật không hiện. */
export function toolArgRows(event: Pick<ActivityEvent, 'args'> & Partial<Pick<ActivityEvent, 'scope' | 'chapter'>>): { label: string; value: string }[] {
  let parsed: Record<string, unknown> = event.scope === 'arc' ? { chapter: event.chapter, scope: event.scope } : {}
  try { if (event.args) parsed = { ...parsed, ...JSON.parse(event.args) } } catch { /* Old arguments may be truncated; only recorded arc fields remain. */ }
  const rows: { label: string; value: string }[] = []
  const from = typeof parsed.from === 'number' ? saneChapter(parsed.from) ?? undefined : undefined
  const to = typeof parsed.to === 'number' ? saneChapter(parsed.to) ?? undefined : undefined
  const chapter = typeof parsed.chapter === 'number' ? saneChapter(parsed.chapter) ?? undefined : undefined
  if (from !== undefined && to !== undefined) rows.push({ label: 'Chương', value: `${from}–${to}` })
  else if (chapter !== undefined) rows.push({ label: fieldLabel(parsed.scope === 'arc' ? 'chapterTo' : 'chapter')!, value: formatNumber(chapter) })
  else if (from !== undefined) rows.push({ label: 'Chương', value: `từ ${from}` })
  else if (to !== undefined) rows.push({ label: 'Chương', value: `đến ${to}` })
  for (const [key, raw] of Object.entries(parsed)) {
    if (key === 'chapter' || key === 'from' || key === 'to' || key === 'type') continue
    const label = fieldLabel(key)
    if (!label) continue
    const value = argText(key, raw)
    if (value) rows.push({ label, value })
  }
  return rows
}

// Kết quả trông kỹ thuật (JSON, đường dẫn, mã snake_case) — chỉ nằm trong “Chép chi tiết kỹ thuật”.
const TECHNICAL = /^[[{]|\/|\w_[a-z0-9_]|[a-z_]+\s*=/
/**
 * Một dòng kết quả của thẻ công cụ: lỗi dịch qua bảng (`429` → “… vượt hạn mức (mã 429)”, `ENOSPC` →
 * “Ổ đĩa máy chủ đã đầy” — errorLabel trong lib/labels); đang chạy thì giữ nguyên chữ host phát.
 */
export function toolResultText(event: Pick<ActivityEvent, 'result' | 'status' | 'failed' | 'level'>): string {
  const raw = (event.result ?? '').trim()
  if (isLogError(event)) return errorLabel(translateHost(raw)) || 'Công cụ báo lỗi'
  if (!raw) return ''
  if (TECHNICAL.test(raw)) return event.status === 'running' ? '' : 'Đã xong'
  return translateHost(raw)
}

const two = (n: number) => String(n).padStart(2, '0')

/** Giờ trong dòng sự kiện: hôm nay `14:31:40`, hôm qua `hôm qua 21:04`, cũ hơn `25/10 21:04`. */
export function logTime(iso: string, now = new Date()): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const hm = `${two(date.getHours())}:${two(date.getMinutes())}`
  const diff = Math.round((day(now) - day(date)) / 86400000)
  if (diff <= 0) return `${hm}:${two(date.getSeconds())}`
  return diff === 1 ? `hôm qua ${hm}` : `${two(date.getDate())}/${two(date.getMonth() + 1)} ${hm}`
}
