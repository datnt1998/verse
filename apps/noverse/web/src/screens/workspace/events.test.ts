// Nhật ký trống: 07-copy chỉ dành “Không có sự kiện nào thuộc loại này” cho trường hợp
// bộ lọc giấu hết; truyện chưa có sự kiện nào phải hiện câu “Chưa có hoạt động nào”.
// G13: lỗi/kết quả `POST …/command` hiện nguyên văn trong Nhật ký, không lặp khi host
// cũng phát sự kiện cùng nội dung.
import { describe, expect, it } from 'vitest'
import type { HostEvent } from '../../api/types'
import { activityEmpty, canRetryNow, commandEvent, eventKind, eventView, hasPendingRetry, mergeCommandLog } from './events'

describe('activityEmpty', () => {
  it('chưa có sự kiện nào → noActivity', () => {
    expect(activityEmpty(0, 0)).toBe('noActivity')
  })

  it('có sự kiện nhưng bộ lọc/ẩn công cụ giấu hết → filtered', () => {
    expect(activityEmpty(5, 0)).toBe('filtered')
  })

  it('còn dòng để hiện → none', () => {
    expect(activityEmpty(5, 5)).toBe('none')
    expect(activityEmpty(5, 2)).toBe('none')
  })
})

const hostEvent = (id: string, summary: string): HostEvent => ({
  id, time: '2026-10-25T10:00:00.000Z', finishedAt: '2026-10-25T10:00:00.000Z', failed: false,
  category: 'SYSTEM', agent: 'host', summary, kind: '', level: 'success',
  depth: 0, durationNs: 0, retryAt: '', sessionId: 'sess-1', canRetryNow: false,
})

describe('commandEvent', () => {
  it('kết quả thành công → dòng SYSTEM, level success', () => {
    const event = commandEvent(3, { message: 'Đã xuất 8 chương ra xuat-ban/a.epub' }, Date.parse('2026-10-25T10:00:00.000Z'))
    expect(event).toMatchObject({
      id: 'cmd-3', summary: 'Đã xuất 8 chương ra xuat-ban/a.epub', category: 'SYSTEM',
      level: 'success', failed: false, time: '2026-10-25T10:00:00.000Z',
    })
  })

  it('lỗi server → dòng ERROR, level error, giữ nguyên văn', () => {
    const event = commandEvent(4, { error: 'Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /next' })
    expect(event).toMatchObject({
      summary: 'Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /next',
      category: 'ERROR', level: 'error', failed: true,
    })
  })

  it('không có message/error → không thêm dòng', () => {
    expect(commandEvent(5, {})).toBeNull()
    expect(commandEvent(6, { message: '   ' })).toBeNull()
  })
})

describe('mergeCommandLog', () => {
  const events = [hostEvent('ev-1', 'Đã chuyển chế độ sang nghiệm thu từng chương')]

  it('host đã phát sự kiện cùng nội dung → không lặp dòng cục bộ', () => {
    const log = [commandEvent(1, { message: 'Đã chuyển chế độ sang nghiệm thu từng chương' })!]
    expect(mergeCommandLog(events, log)).toEqual(events)
  })

  it('lỗi không có sự kiện host → nối vào cuối Nhật ký', () => {
    const failure = commandEvent(2, { error: 'Cách dùng: /review on|off' })!
    const merged = mergeCommandLog(events, [failure])
    expect(merged).toHaveLength(2)
    expect(merged[1]).toBe(failure)
    expect(merged[0]).toBe(events[0])
  })

  it('không có dòng cục bộ → trả nguyên mảng cũ', () => {
    expect(mergeCommandLog(events, [])).toBe(events)
  })
})

describe('hasPendingRetry / canRetryNow', () => {
  const retryAt = '2026-10-25T10:00:07.000Z'

  // Real `serve` wire: Go zero time.Time marshals to 0001-01-01 for every event (E2EWORK-3).
  it('Go zero retryAt/finishedAt are unset: no retry chip, running tool still spins', () => {
    const zero = '0001-01-01T00:00:00Z'
    const done = { ...hostEvent('wire', 'Đã xong'), retryAt: zero, canRetryNow: true }
    expect(hasPendingRetry(done)).toBe(false)
    expect(canRetryNow(done)).toBe(false)
    expect(eventView(done, Date.parse('2026-10-25T10:00:00Z')).retryIn).toBe(0)
    const running = { ...hostEvent('tool', 'novel_context'), category: 'TOOL', depth: 1, finishedAt: zero, retryAt: zero }
    expect(eventView(running, 0).spin).toBe(true)
  })

  it('chỉ sự kiện có retryAt mới hiện đếm ngược', () => {
    expect(hasPendingRetry(hostEvent('ev-1', 'Đang chạy'))).toBe(false)
    expect(hasPendingRetry({ ...hostEvent('ev-2', 'Thử lại (2/7)'), retryAt })).toBe(true)
  })

  it('nút “Thử lại ngay” chỉ hiện khi canRetryNow=true', () => {
    expect(canRetryNow({ ...hostEvent('ev-3', 'Thử lại (2/7)'), retryAt, canRetryNow: true })).toBe(true)
    // agentcore retry giữa luồng: POST …/retry trả 409 → chỉ đếm ngược, không có nút
    expect(canRetryNow({ ...hostEvent('ev-4', 'Thử lại (3/7)'), retryAt, canRetryNow: false })).toBe(false)
    expect(canRetryNow({ ...hostEvent('ev-5', 'Đã xong'), retryAt: '', canRetryNow: true })).toBe(false)
  })
})

describe('eventKind error filter', () => {
  it('includes failed tool and agent calls despite their category', () => {
    expect(eventKind({ ...hostEvent('tool', 'Lỗi công cụ'), category: 'TOOL', failed: true })).toBe('error')
    expect(eventKind({ ...hostEvent('agent', 'Lỗi trợ lý'), category: 'DISPATCH', level: 'error' })).toBe('error')
  })
})

// D4/D9: dòng Biên tập viên mang category REVIEW vào bộ lọc “Thẩm duyệt”, không lẫn “Trợ lý”.
describe('eventKind thẩm duyệt', () => {
  it('category REVIEW/review xếp vào nhóm Thẩm duyệt, kể cả khi agent là editor', () => {
    expect(eventKind({ ...hostEvent('review', 'Thẩm chương 3'), category: 'REVIEW', agent: 'editor' })).toBe('review')
    expect(eventKind({ ...hostEvent('review-2', 'Chấm lại chương 3'), category: 'review', agent: 'editor' })).toBe('review')
  })
  it('editor phát sự kiện không phải REVIEW vẫn là Trợ lý; lỗi vẫn thắng', () => {
    expect(eventKind({ ...hostEvent('dispatch', 'Nhận việc'), category: 'DISPATCH', agent: 'editor' })).toBe('dispatch')
    expect(eventKind({ ...hostEvent('review-failed', 'Thẩm lỗi'), category: 'REVIEW', agent: 'editor', failed: true })).toBe('error')
  })
})
