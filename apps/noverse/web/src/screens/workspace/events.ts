// HostEvent → dòng Nhật ký (06-data-contract.md › HostEvent → ActivityRow).
// Category host phát là DISPATCH/TOOL/DECISION/SYSTEM/USER/ERROR; dữ liệu cũ
// (mock, event_log cũ) chỉ có agent + depth nên vẫn phân loại được.
import type { ActivityEvent, HostEvent } from '../../api/types'
import { activityText, isUserEvent } from '../../lib/activity'
import { agentLabel, toolIcon } from '../../lib/labels'
import { formatDuration } from '../../lib/format'

export type EventKind = 'dispatch' | 'review' | 'tool' | 'decision' | 'system' | 'user' | 'error'

const KNOWN: Record<string, EventKind> = {
  DISPATCH: 'dispatch', REVIEW: 'review', TOOL: 'tool', DECISION: 'decision', SYSTEM: 'system', USER: 'user', ERROR: 'error',
  dispatch: 'dispatch', review: 'review', tool: 'tool', decision: 'decision', system: 'system', user: 'user', error: 'error',
}


export function eventKind(event: HostEvent): EventKind {
  if (event.failed || event.level === 'error') return 'error'
  const known = KNOWN[event.category]
  if (known) return known
  if (isUserEvent(event)) return 'user'
  if (event.depth > 0) return 'tool'
  const agent = event.agent.toLowerCase()
  if (agent === 'arbiter') return 'decision'
  if (agent === 'architect' || agent === 'writer' || agent === 'editor' || agent === 'architect_long') return 'dispatch'
  return 'system'
}

const AGENT_COLOR: Record<string, string> = {
  architect: 'var(--color-architect)',
  architect_long: 'var(--color-architect)',
  writer: 'var(--color-writer)',
  editor: 'var(--color-editor)',
  arbiter: 'var(--color-arbiter)',
}

export type EventView = {
  kind: EventKind
  time: string
  who: string
  whoColor: string
  text: string
  /** toàn văn dòng (chưa kẹp lý do) — tooltip rê chuột của bản compact */
  title: string
  indent: number
  tree: boolean
  spin: boolean
  icon: string
  iconColor: string
  tone: string
  duration: string
  /** giây còn lại tới lần thử lại; 0 = không có */
  retryIn: number
}

// Gói O mục 3: lý do trong “Bắt đầu sáng tác (… — lý do)” tối đa ~80 ký tự trên dòng;
// bản đầy đủ nằm trong `title` (tooltip) của dòng.
const START_REASON = /^(Bắt đầu sáng tác \(.+ — )([\s\S]{81,})(\))$/
const startReasonClamp = (text: string): string =>
  text.replace(START_REASON, (_, head, reason: string, tail) => `${head}${[...reason].slice(0, 80).join('')}…${tail}`)

/** Go `time.Time{}` reaches the wire as `0001-01-01T00:00:00Z`; treat it like an empty field. */
export const wireTime = (value: string | null | undefined): string => (value && !value.startsWith('0001-01-01') ? value : '')

/** Một dòng nhật ký đã sẵn sàng render. `now` để đếm ngược `retryAt` mỗi giây. */
export function eventView(event: DetailedEvent, now: number, total?: number | null): EventView {
  const kind = eventKind(event)
  const pending = !wireTime(event.finishedAt) && kind !== 'system' && kind !== 'user' && kind !== 'error'
  const retryAt = wireTime(event.retryAt)
  const retryIn = retryAt ? Math.max(0, Math.ceil((Date.parse(retryAt) - now) / 1000)) : 0
  // Thời lượng chỉ có nghĩa với lượt gọi agent/công cụ/phán quyết/lỗi (artboard)
  // host gửi nano giây; lib/format nhận mili giây
  const duration = kind === 'system' || kind === 'user' ? '' : formatDuration((event.durationNs ?? 0) / 1e6)
  const fullText = activityText(event, total)
  const base: EventView = {
    kind, time: '', who: '', whoColor: 'var(--color-fg)', text: startReasonClamp(fullText), title: fullText,
    indent: 0, tree: false, spin: false, icon: '', iconColor: 'var(--color-fg)',
    tone: 'var(--color-fg-2)', duration, retryIn,
  }
  switch (kind) {
    case 'review': {
      // Activity.dc.html › `rev()`: Biên tập viên · nội dung, cờ biên tập, chữ fg-3.
      const who = agentLabel(event.agent || 'editor')
      return {
        ...base,
        who: who ? `${who} ` : '',
        whoColor: AGENT_COLOR[event.agent.toLowerCase()] ?? 'var(--color-editor)',
        text: `· ${base.text}`,
        spin: pending,
        icon: pending ? '' : event.failed ? 'close-circle-bulk' : 'flag',
        iconColor: event.failed ? 'var(--color-err)' : 'var(--color-editor)',
        tone: 'var(--color-fg-3)',
      }
    }
    case 'tool':
      // Gói O mục 5: nét nhánh “├” luôn giữ chỗ; đang chạy → vòng xoay thay vào đúng chỗ icon.
      return {
        ...base,
        indent: 22, tree: true, spin: pending,
        icon: pending ? '' : event.failed ? 'close-circle-bulk' : (event.tool ? toolIcon(event.tool) : ''),
        iconColor: event.failed ? 'var(--color-err)' : 'var(--color-fg-4)',
        tone: event.failed ? 'var(--color-err)' : 'var(--color-fg-3)',
      }
    case 'user':
      return { ...base, icon: 'edit-2', iconColor: 'var(--color-user)', tone: 'var(--color-user-fg)' }
    case 'system':
      return {
        ...base,
        icon: 'setting-2',
        iconColor: event.level === 'warn' ? 'var(--color-warn)' : 'var(--color-accent)',
        tone: event.level === 'warn' ? 'var(--color-warn)' : event.level === 'success' ? 'var(--color-ok)' : 'var(--color-fg-2)',
      }
    case 'error':
      return { ...base, icon: 'close-circle-bulk', iconColor: 'var(--color-err)', tone: 'var(--color-err)' }
    default: {
      // DISPATCH / DECISION: “{Trợ lý} · nhiệm vụ” + tick/spinner/✕ (không ngoặc toàn khổ)
      const who = agentLabel(event.agent || (kind === 'decision' ? 'arbiter' : ''))
      return {
        ...base,
        who: who ? `${who} ` : '',
        whoColor: AGENT_COLOR[event.agent.toLowerCase()] ?? 'var(--color-fg)',
        text: `· ${base.text}`,
        spin: pending,
        icon: pending ? '' : event.failed ? 'close-circle-bulk' : 'tick-circle-bulk',
        iconColor: event.failed ? 'var(--color-err)' : 'var(--color-ok)',
        tone: 'var(--color-fg-4)',
      }
    }
  }
}

/** Chương đang chờ duyệt trong một dòng hệ thống (để hiện nút `/next`). */
export function pendingGateChapter(text: string): number | null {
  const match = /chờ duyệt chương\s+(\d+)/i.exec(text) ?? /等待放行第\s*(\d+)\s*章/.exec(text)
  return match ? Number(match[1]) : null
}

/**
 * Trạng thái trống của Nhật ký: chưa có sự kiện nào (`noActivity`, 07-copy) khác với
 * có sự kiện nhưng bộ lọc/ẩn công cụ giấu hết (`filtered` → “Không có sự kiện nào thuộc loại này”).
 */
export type ActivityEmpty = 'none' | 'noActivity' | 'filtered'

export function activityEmpty(total: number, visible: number): ActivityEmpty {
  if (total === 0) return 'noActivity'
  return visible === 0 ? 'filtered' : 'none'
}

/**
 * G13: kết quả `POST …/command` → một dòng Nhật ký. Lỗi cú pháp/quyền không phát sinh sự kiện
 * host nên phải hiện nguyên văn ở đây; khi server có phát sự kiện cùng nội dung thì
 * `mergeCommandLog` bỏ dòng cục bộ để không lặp.
 */
export function commandEvent(seq: number, outcome: { message?: string; error?: string }, now = Date.now()): HostEvent | null {
  const text = (outcome.error ?? outcome.message ?? '').trim()
  if (text === '') return null
  const failed = outcome.error !== undefined
  const at = new Date(now).toISOString()
  return {
    id: `cmd-${seq}`, time: at, finishedAt: at, failed,
    category: failed ? 'ERROR' : 'SYSTEM', agent: 'host', summary: text, kind: 'command',
    level: failed ? 'error' : 'success', depth: 0, durationNs: 0, retryAt: '', sessionId: '', canRetryNow: false,
  }
}

/** Ghép dòng lệnh cục bộ vào Nhật ký, bỏ dòng đã có sự kiện host cùng nội dung. */
export function mergeCommandLog(events: HostEvent[], log: HostEvent[]): HostEvent[] {
  if (log.length === 0) return events
  const seen = new Set(events.map((event) => event.summary))
  const extra = log.filter((event) => !seen.has(event.summary))
  return extra.length === 0 ? events : [...events, ...extra]
}

/** G4: sự kiện đang chờ backoff (`retryAt`) → Nhật ký hiện “Thử lại sau Ns”. */
export const hasPendingRetry = (event: HostEvent): boolean => wireTime(event.retryAt) !== ''

/**
 * G4: chỉ lượt chờ đánh thức được (`canRetryNow`) mới có nút “Thử lại ngay”;
 * retry giữa luồng của agentcore trả 409 nên chỉ hiện đếm ngược.
 */
export const canRetryNow = (event: HostEvent): boolean => hasPendingRetry(event) && event.canRetryNow === true

/** Sự kiện có thể mang chi tiết công cụ (`tool`/`args`/`result`) — luôn có trên dây, kiểu HostEvent chỉ khai báo phần chung. */
export type DetailedEvent = HostEvent & Partial<Pick<ActivityEvent, 'tool' | 'args' | 'result' | 'status' | 'retries' | 'chapter' | 'review'>>

/** Văn bản “Chi tiết kỹ thuật” của một lượt gọi công cụ — chỉ hiện khi người dùng mở khối, hoặc chép. */
export function toolCode(event: DetailedEvent): string {
  const retries = event.retries?.map((retry) => `↻ Tự thử lại sau ${retry.waitSec} giây${retry.error ? ` · ${retry.error}` : ''} → ${retry.ok ? 'thành công' : 'chưa thành công'}`) ?? []
  return [`${event.tool}(${event.args || ''})`, `→ ${event.result || (event.status === 'running' ? 'đang chạy…' : 'Không có kết quả trả về')}`, ...retries].join('\n')
}
