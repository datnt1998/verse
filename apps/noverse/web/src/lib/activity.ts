import type { HostEvent } from '../api/types'
import { CMD_COPY } from '../i18n/commands.vi'
import { translateHost } from '../i18n/hostStrings'
import { t } from '../i18n/vi'
import { toolEventText } from './labels'

const USER_PREFIX = /^\[(Tiếp tục|继续|Can thiệp|用户干预|干预)\]\s*/
type UserAction = { command: string; text: string }

export function isUserEvent(event: HostEvent): boolean {
  return event.category?.toLowerCase() === 'user' || event.kind === 'user' || event.agent === 'user' || event.agent === 'me' || USER_PREFIX.test(event.summary)
}

/** Reasoning remains audit-only, including system bubbles written before kind tagging. */
export function isInternalEvent(event: HostEvent): boolean {
  if (event.kind === 'internal') return true
  return event.category?.toUpperCase() === 'SYSTEM' && /^(?:Phán định|裁定)[:：]/.test(event.summary.trim())
}

/** Keep the user's prose intact; only unwrap old system-added command markers. */
export function userAction(event: HostEvent): UserAction | null {
  if (!isUserEvent(event)) return null
  if (event.command) return { command: event.command, text: event.input ?? event.summary }
  const tag = USER_PREFIX.exec(event.summary)
  if (tag) return { command: ['Tiếp tục', '继续'].includes(tag[1]) ? 'continue' : 'steer', text: event.summary.slice(tag[0].length) }
  const slash = /^\/([^\s]+)(?:\s+([\s\S]*))?$/.exec(event.summary)
  if (slash) {
    if (slash[1] === 'command') return userAction({ ...event, summary: slash[2] ?? '' })
    return { command: slash[1], text: slash[2] ?? '' }
  }
  if (event.summary.startsWith('Can thiệp: ')) return { command: 'steer', text: event.summary.slice('Can thiệp: '.length) }
  return { command: '', text: event.summary }
}

export function userEventText(event: HostEvent): string | null {
  const action = userAction(event)
  if (!action) return null
  const copy = t.userActivity
  const labels: Record<string, string> = {
    input: copy.input, continue: copy.continue, steer: copy.steer, start: copy.start,
    resume: copy.resume, abort: copy.abort, pause: copy.pause, retry: copy.retry,
    'cocreate/message': copy.message, 'cocreate/commit': copy.commit,
  }
  const { command, text } = action
  if (!command) return text
  if (command === 'input' && text.trim() === copy.continue) return copy.continue
  const label = labels[command] ?? CMD_COPY[command]?.title ?? copy.unknown
  if (['input', 'continue', 'steer', 'start', 'cocreate/message'].includes(command) && text.trim() && text.trim() !== label) {
    // A typed slash command is audit data, not prose to echo into the feed.
    if (text.trim().startsWith('/')) return label
    return copy.detail(label, text)
  }
  return label
}

/** The same visible title is used by desktop, mobile, history and search. */
export function activityText(event: HostEvent & { tool?: string }, total?: number | null): string {
  const user = userEventText(event)
  if (user !== null) return user
  if (event.scope === 'arc' && event.tool === 'save_review') return t.activity.saveArcReview(event.arc)
  if (event.scope === 'arc' && event.arc && /thẩm duyệt|save_review/i.test(event.summary)) {
    return t.activity.arcReview(event.arc, event.volume, event.chapterFrom, event.chapterTo)
  }
  // Flow task suffixes are machine instructions, not the action sentence.
  const task = /^(?:architect(?:_short|_long)?|writer|editor|arbiter)\s*\(([\s\S]*)\)$/.exec(event.summary)
  const summary = (task?.[1] ?? event.summary)
    .replace(/[:：;]\s*(?:gọi|dùng|lưu)[\s\S]*$/i, '')
    .replace(/\s*[（(]\s*[a-z]+_[a-z_]+\b[^)）]*[)）]/g, '')
  return (toolEventText({ ...event, summary }, total) ?? translateHost(summary))
    .replace(/(chương\s+\d+)\s*-\s*(\d+)/gi, '$1–$2')
}

export function userEventActor(event: HostEvent, username?: string): string {
  return !event.username || event.username === username ? t.userActivity.me : event.username
}

const legacySource = (event: HostEvent): 'host' | 'audit' | null => !isUserEvent(event) ? null
  : USER_PREFIX.test(event.summary) ? 'host' : /^\/(input|continue|steer)\s/.test(event.summary) ? 'audit' : null

/** Only opposite legacy sources pair, one-to-one, within the same 5s action window. */
export function deduplicateUserEvents<T extends HostEvent>(events: readonly T[]): T[] {
  const paired = new Set<number>()
  const rows: T[] = []
  for (let i = 0; i < events.length; i++) {
    if (paired.has(i)) continue
    let event = events[i]
    const source = legacySource(event)
    const action = userAction(event)
    let best = -1
    let distance = 5001
    if (source && action) {
      for (let j = i + 1; j < events.length; j++) {
        const other = events[j]
        const otherSource = legacySource(other)
        if (paired.has(j) || !otherSource || source === otherSource) continue
        const otherAction = userAction(other)!
        if (action.text !== otherAction.text || (event.sessionId && other.sessionId && event.sessionId !== other.sessionId)) continue
        if (action.command !== 'input' && otherAction.command !== 'input' && action.command !== otherAction.command) continue
        const delta = Math.abs(Date.parse(event.time) - Date.parse(other.time))
        if (delta < distance) { best = j; distance = delta }
      }
    }
    if (best >= 0 && action) {
      paired.add(best)
      const other = events[best]
      const host = source === 'host' ? event : other
      const audit = source === 'audit' ? event : other
      event = { ...host, ...audit, summary: action.text, command: userAction(host)!.command, input: action.text }
    }
    rows.push(event)
  }
  return rows
}
