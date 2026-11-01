import { describe, expect, it } from 'vitest'
import type { AppNotification, BackgroundTask } from '../api/activity'
import { taskOperation } from './ActivityPopover'
import { groupNotifications, orderNotifications } from './notificationList'
import { isQuiet } from './ActivityBridge'

function note(id: string, kind: AppNotification['kind'], readAt: string | null = null): AppNotification {
  return { id, kind, readAt, count: 1, text: id, actions: [{ href: '#/', label: 'Xem' }], createdAt: '2026-11-01T12:00:00Z' }
}
describe('activity priorities', () => {
  it('places unread review, attention and ready before completed or read entries', () => {
    const rows = [note('read', 'review_ready', '2026-11-01T12:01:00Z'), note('done', 'task_done'), note('ready', 'ready_to_post'), note('issue', 'needs_attention'), note('review', 'review_ready')]
    expect(orderNotifications(rows).map((row) => row.id)).toEqual(['review', 'issue', 'ready', 'done', 'read'])
  })
  it('groups the same rows into today, yesterday and older for both surfaces', () => {
    const now = new Date(2026, 10, 1, 12, 0)
    const daysAgo = (days: number) => new Date(2026, 10, 1 - days, 9, 0).toISOString()
    const rows = [note('today', 'review_ready'), note('yesterday', 'review_ready'), note('older', 'review_ready')]
    rows[0].createdAt = daysAgo(0); rows[1].createdAt = daysAgo(1); rows[2].createdAt = daysAgo(3)
    const groups = groupNotifications(rows, now)
    expect(groups.map((group) => [group.label, group.rows.map((row) => row.id)])).toEqual([['Hôm nay', ['today']], ['Hôm qua', ['yesterday']], ['Trước đó', ['older']]])
  })
  it('handles quiet hours crossing midnight', () => {
    const settings = { kinds: {} as never, badge: true, merge: true, recapOnReturn: true, suggestions: { off: [] }, doNotDisturb: { enabled: true, start: '23:00', end: '07:00' } }
    expect(isQuiet(settings, new Date(2026, 10, 1, 23, 30))).toBe(true)
    expect(isQuiet(settings, new Date(2026, 10, 2, 6, 59))).toBe(true)
    expect(isQuiet(settings, new Date(2026, 10, 2, 7, 0))).toBe(false)
  })
})

it('routes real task operations to their existing endpoints and request bodies', () => {
  const task: BackgroundTask = { id: 'task', kind: 'setup', title: 'Cài', objectRef: { type: 'tool', id: 'chrome', label: 'Chrome' }, phase: '', progress: null, etaSec: null, state: 'failed', action: { label: 'Thử lại', op: 'retry' } }
  expect(taskOperation(task)).toEqual({ path: '/api/tools/chrome/install', body: {} })
  expect(taskOperation({ ...task, objectRef: { type: 'setup_step', id: 'chrome', label: 'Chrome' } })).toEqual({ path: '/api/setup/retry', body: { id: 'chrome' } })
  expect(taskOperation({ ...task, objectRef: { type: 'storage_move', id: 'other', label: 'Ổ' }, action: { label: 'Huỷ', op: 'cancel' } })).toEqual({ path: '/api/storage/move/task/cancel', body: {} })
  expect(taskOperation({ ...task, objectRef: { type: 'rival', id: 'UC/test', label: 'Kênh' } })).toEqual({ path: '/api/rivals/UC%2Ftest/scan', body: { action: 'resume' } })
})
