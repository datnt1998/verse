// G3 (19-gap-closure.md): nhóm Nhật ký theo `HostEvent.sessionId`; dữ liệu cũ không có
// sessionId vẫn nhóm được bằng khoảng lặng > 10 phút giữa hai sự kiện.
import type { HostEvent } from '../../api/types'
import { eventKind, wireTime } from '../workspace/events'

/** Khoảng lặng tối thiểu để coi là phiên mới khi HostEvent không có sessionId. */
export const SESSION_GAP_MS = 10 * 60 * 1000

export type SessionGroup = {
  /** sessionId khi có; nếu không thì `session-<n>` */
  id: string
  /** sessionId của phiên ('' khi dữ liệu cũ) */
  sessionId: string
  from: string
  to: string
  running: boolean
  events: HostEvent[]
}

const startsSession = (event: HostEvent, current: SessionGroup, at: number, previous: number): boolean => {
  const session = event.sessionId ?? ''
  if (session !== '' && session !== current.sessionId) return true
  // Không có sessionId ở cả hai phía → chỉ tách khi có khoảng lặng dài.
  if (session === '' && current.sessionId === '' && !Number.isNaN(at) && !Number.isNaN(previous)) {
    return at - previous > SESSION_GAP_MS
  }
  return false
}

export function groupSessions(events: HostEvent[]): SessionGroup[] {
  const groups: SessionGroup[] = []
  let current: SessionGroup | null = null
  for (const event of events) {
    const at = Date.parse(event.time)
    const previous = current ? Date.parse(current.to) : Number.NaN
    if (!current || startsSession(event, current, at, previous)) {
      current = {
        id: event.sessionId || `session-${groups.length}`,
        sessionId: event.sessionId ?? '',
        from: event.time, to: event.time, running: false, events: [],
      }
      groups.push(current)
    }
    current.events.push(event)
    current.to = event.time
    const kind = eventKind(event)
    current.running = !wireTime(event.finishedAt) && kind !== 'system' && kind !== 'user'
  }
  return groups
}
