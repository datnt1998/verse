import { describe, expect, it } from 'vitest'
import type { ActivityEvent } from '../../api/types'
import { metricsChapterOf } from './metrics'

const event = (over: Partial<ActivityEvent> = {}): ActivityEvent => ({
  id: 'ev-1', time: '2026-10-25T10:00:00Z', finishedAt: '2026-10-25T10:00:01Z', failed: false,
  category: 'TOOL', agent: 'writer', summary: 'commit_chapter(chương 3)', kind: '', level: 'info',
  depth: 1, durationNs: 1_000_000, retryAt: '', sessionId: 's', canRetryNow: false, chapter: 3,
  tool: 'commit_chapter', ...over,
})

describe('D6 — dòng “Đo bằng máy” bám sau lần nộp chương', () => {
  it('lần nộp chương thành công: đúng chương của sự kiện', () => {
    expect(metricsChapterOf(event())).toBe(3)
  })
  it('số chương lấy từ tham số công cụ trước `chapter` của phiên', () => {
    expect(metricsChapterOf(event({ args: '{"chapter":5,"hook_type":"crisis"}', chapter: 6 }))).toBe(5)
    expect(metricsChapterOf(event({ args: 'không phải json' }))).toBe(3)
  })
  it('không phải nộp chương, nộp lỗi, hoặc thiếu số chương: không gắn số đo', () => {
    expect(metricsChapterOf(event({ tool: 'read_chapter' }))).toBeNull()
    expect(metricsChapterOf(event({ failed: true }))).toBeNull()
    expect(metricsChapterOf(event({ level: 'error' }))).toBeNull()
    expect(metricsChapterOf(event({ chapter: 0 }))).toBeNull()
  })
})
