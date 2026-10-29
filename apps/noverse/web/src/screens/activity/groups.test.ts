// G3 (19-gap-closure.md): nhóm Nhật ký theo phiên engine; dữ liệu cũ (không có sessionId)
// vẫn nhóm bằng khoảng lặng > 10 phút.
import { describe, expect, it } from 'vitest'
import type { HostEvent } from '../../api/types'
import { groupSessions, SESSION_GAP_MS } from './groups'

const T0 = Date.parse('2026-10-25T10:00:00.000Z')

const event = (patch: Partial<HostEvent> & { id: string; minutes: number }): HostEvent => {
  const { minutes, ...rest } = patch
  const at = new Date(T0 + minutes * 60_000).toISOString()
  return {
    time: at, finishedAt: at, failed: false, category: 'SYSTEM', agent: 'host',
    summary: rest.id, kind: '', level: 'info', depth: 0, durationNs: 0, retryAt: '', sessionId: '', canRetryNow: false,
    ...rest,
  }
}

describe('groupSessions', () => {
  it('sessionId đổi → phiên mới, bỏ hẳn mốc 10 phút', () => {
    const groups = groupSessions([
      event({ id: 'a1', minutes: 0, sessionId: 'sess-1' }),
      event({ id: 'a2', minutes: 1, sessionId: 'sess-1' }),
      // chỉ cách phiên trước 1 phút nhưng là phiên mới → vẫn tách
      event({ id: 'b1', minutes: 2, sessionId: 'sess-2' }),
      event({ id: 'b2', minutes: 120, sessionId: 'sess-2' }),
    ])
    expect(groups.map((group) => group.id)).toEqual(['sess-1', 'sess-2'])
    expect(groups[0].events.map((item) => item.id)).toEqual(['a1', 'a2'])
    // cùng sessionId, cách nhau 2 tiếng → vẫn một phiên
    expect(groups[1].events.map((item) => item.id)).toEqual(['b1', 'b2'])
  })

  it('không có sessionId → tách khi khoảng lặng > 10 phút', () => {
    const groups = groupSessions([
      event({ id: 'a1', minutes: 0 }),
      event({ id: 'a2', minutes: 5 }),
      event({ id: 'b1', minutes: 5 + SESSION_GAP_MS / 60_000 + 1 }),
    ])
    expect(groups).toHaveLength(2)
    expect(groups[0].events.map((item) => item.id)).toEqual(['a1', 'a2'])
    expect(groups[1].events.map((item) => item.id)).toEqual(['b1'])
    expect(groups[0].sessionId).toBe('')
    expect(groups[1].id).toBe('session-1')
  })

  it('dữ liệu cũ rồi có sessionId → phiên mới bắt đầu từ sự kiện có sessionId', () => {
    const groups = groupSessions([
      event({ id: 'old', minutes: 0 }),
      event({ id: 'new', minutes: 1, sessionId: 'sess-9' }),
    ])
    expect(groups.map((group) => group.sessionId)).toEqual(['', 'sess-9'])
  })

  it('running = sự kiện cuối còn chưa xong', () => {
    const groups = groupSessions([
      event({ id: 'a1', minutes: 0, sessionId: 's', finishedAt: '2026-10-25T10:00:00.000Z' }),
      event({ id: 'a2', minutes: 1, sessionId: 's', category: 'TOOL', finishedAt: '' }),
    ])
    expect(groups[0].running).toBe(true)
    const done = groupSessions([event({ id: 'b1', minutes: 0, sessionId: 's' })])
    expect(done[0].running).toBe(false)
  })

  it('mảng rỗng → không nhóm nào', () => {
    expect(groupSessions([])).toEqual([])
  })
})
