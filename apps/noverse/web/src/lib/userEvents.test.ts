import { describe, expect, it } from 'vitest'
import type { HostEvent } from '../api/types'
import { mergeLogEvents } from '../api/useActivityHistory'
import { eventView } from '../screens/workspace/events'
import { activityText, userEventActor } from './activity'
import { applyLiveToCache } from '../api/useStoryEvents'
import { cache } from '../data'

const event = (id: string, summary: string, extra: Partial<HostEvent> = {}): HostEvent => ({
  id, summary, category: 'USER', agent: 'user', kind: 'user', username: 'dat',
  time: '2026-10-27T10:00:00Z', finishedAt: '', failed: false, level: 'info', depth: 0,
  durationNs: 0, retryAt: '', sessionId: 's1', canRetryNow: false, ...extra,
})

describe('user action rows', () => {
  it('renders structured and legacy actions as Vietnamese without technical wrappers', () => {
    for (const summary of ['[Tiếp tục] Tiếp tục', '/input Tiếp tục', '[继续] Tiếp tục']) {
      const view = eventView(event(summary, summary), 0)
      expect(view.title).toBe('Tiếp tục')
      expect(view.icon).toBe('edit-2')
      expect(view.spin).toBe(false)
      expect(view.duration).toBe('')
    }
    expect(eventView(event('steer', '[Can thiệp] Đẩy tuyến tình cảm lên chương 4'), 0).title)
      .toBe('Can thiệp: Đẩy tuyến tình cảm lên chương 4')
    expect(eventView(event('next', '/next'), 0).title).toBe('Cho viết chương tiếp theo')
    for (const [command, input, expected] of [
      ['continue', 'Tiếp tục', 'Tiếp tục'],
      ['steer', 'Đổi hướng hồi ba', 'Can thiệp: Đổi hướng hồi ba'],
      ['pause', '', 'Tạm dừng'],
      ['abort', '', 'Tạm dừng'],
      ['export', '/export exports/truyen.epub', 'Xuất truyện ra văn bản, ebook hoặc bản đọc TTS'],
    ]) {
      const structured = event(command, input, { command, input })
      expect(activityText(structured)).toBe(expected)
      expect(eventView(structured, 0).title).toBe(expected)
      expect(userEventActor(structured, 'dat')).toBe('Bạn')
      expect(userEventActor(structured, 'reader')).toBe('dat')
      expect(structured.input).toBe(input)
    }
  })

  it('merges legacy host/audit pairs one-to-one, retaining username and distinct actions', () => {
    const rows = mergeLogEvents([
      event('audit-1', '/input Tiếp tục'),
      event('host-1', '[Tiếp tục] Tiếp tục', { username: 'machine' }),
      event('audit-2', '/input Tiếp tục', { time: '2026-10-27T10:00:01Z' }),
      event('host-2', '[Tiếp tục] Tiếp tục', { time: '2026-10-27T10:00:01Z', username: 'machine' }),
    ])
    expect(rows).toHaveLength(2)
    expect(rows.map((row) => row.username)).toEqual(['dat', 'dat'])
    expect(rows.map((row) => eventView(row, 0).title)).toEqual(['Tiếp tục', 'Tiếp tục'])
    expect(mergeLogEvents([event('a', '[Tiếp tục] Tiếp tục'), event('b', '[Tiếp tục] Tiếp tục')])).toHaveLength(2)
    expect(mergeLogEvents([event('a', '/input Tiếp tục'), event('b', '[Tiếp tục] Tiếp tục', { time: '2026-10-27T10:00:30Z' })])).toHaveLength(2)
    cache.clear()
    for (const row of [
      event('audit-1', '/input Tiếp tục'), event('host-1', '[Tiếp tục] Tiếp tục'),
      event('audit-2', '/input Tiếp tục'), event('host-2', '[Tiếp tục] Tiếp tục'),
    ]) applyLiveToCache('user-rows', 'story', { project: 'user-rows', story: 'story', kind: 'event', data: row })
    expect(cache.get<HostEvent[]>(['story', 'user-rows', 'story', 'events']).data).toHaveLength(2)
    cache.clear()
  })
})
