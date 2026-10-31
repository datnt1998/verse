import { describe, expect, it } from 'vitest'
import type { AppNotification } from '../api/activity'
import { orderNotifications } from './ActivityPopover'
import { isQuiet } from './ActivityBridge'

function note(id: string, kind: AppNotification['kind'], readAt: string | null = null): AppNotification {
  return { id, kind, readAt, count: 1, text: id, href: '#/', actionLabel: 'Xem', createdAt: '2026-11-01T12:00:00Z' }
}
describe('activity priorities', () => {
  it('places unread review, attention and ready before completed or read entries', () => {
    const rows = [note('read', 'review_ready', '2026-11-01T12:01:00Z'), note('done', 'task_done'), note('ready', 'ready_to_post'), note('issue', 'needs_attention'), note('review', 'review_ready')]
    expect(orderNotifications(rows).map((row) => row.id)).toEqual(['review', 'issue', 'ready', 'done', 'read'])
  })
  it('handles quiet hours crossing midnight', () => {
    const settings = { kinds: {} as never, badge: true, merge: true, recapOnReturn: true, doNotDisturb: { enabled: true, start: '23:00', end: '07:00' } }
    expect(isQuiet(settings, new Date(2026, 10, 1, 23, 30))).toBe(true)
    expect(isQuiet(settings, new Date(2026, 10, 2, 6, 59))).toBe(true)
    expect(isQuiet(settings, new Date(2026, 10, 2, 7, 0))).toBe(false)
  })
})
