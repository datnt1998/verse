import { describe, expect, it } from 'vitest'
import type { Insight, Performance } from '../../api/performance'
import { barWidths, insightRatio, isYoutubeLink, postedDetail, teaserEvidence } from './performance-logic'

const insight = (patch: Partial<Insight>): Insight => ({ id: 'i', dimension: 'template', text: '', groups: [], preliminary: true, ...patch })
const stat = (views: number) => ({ views, readAt: '2026-10-30T00:00:00Z' })

describe('thanh so sánh nhóm', () => {
  it('thanh dài nhất vẽ 90%, thanh kia tỉ lệ (artboard: 8,1 N → 90%, 4,5 N → 50%)', () => {
    expect(barWidths([8100, 4500])).toEqual([90, 50])
    expect(barWidths([0, 0])).toEqual([0, 0])
  })
  it('hệ số lấy nhóm đầu chia nhóm sau, bỏ qua khi mẫu số 0', () => {
    expect(insightRatio({ groups: [{ label: 'a', n: 3, median7d: 8100 }, { label: 'b', n: 4, median7d: 4500 }] })).toBeCloseTo(1.8)
    expect(insightRatio({ groups: [{ label: 'a', n: 3, median7d: 1 }, { label: 'b', n: 3, median7d: 0 }] })).toBeUndefined()
  })
})

describe('dòng bằng chứng của thẻ mời xem', () => {
  it('ghép các nhận xét bằng “ · ” và thêm Sơ bộ khi còn sơ bộ', () => {
    const title = insight({ dimension: 'title_pattern', groups: [{ label: 'Câu hỏi', n: 3, median7d: 8100 }, { label: 'Không phải câu hỏi', n: 4, median7d: 4500 }] })
    const template = insight({ dimension: 'template', groups: [{ label: 'Mẫu chuẩn', n: 4, median7d: 6900 }, { label: 'Cận cảnh', n: 3, median7d: 4900 }] })
    expect(teaserEvidence([title, template])).toBe('Tiêu đề dạng câu hỏi ×1,8 · Mẫu chuẩn ×1,4 so với Cận cảnh · Sơ bộ')
  })
})

describe('dòng phụ hàng Đã đăng', () => {
  const perf = (row: object) => ({ posted: [{ jobId: 'j', ...row }], insights: [], needMore: 0, counted: 0 }) as Performance
  it('có số 7 ngày thì kèm ×N so với trung vị; mới có 24 giờ thì chỉ số 24 giờ', () => {
    expect(postedDetail(perf({ at7d: stat(6200), vsMedian7d: 1.4 }), 'j')).toBe('6,2 N / 7 ngày · ×1,4 so với trung vị')
    expect(postedDetail(perf({ at24h: stat(6200), vsMedian7d: 1.4 }), 'j')).toBe('6,2 N / 24 giờ')
  })
  it('chưa đọc được số nào hoặc không có hàng → null', () => {
    expect(postedDetail(perf({}), 'j')).toBeNull()
    expect(postedDetail(perf({ at7d: stat(1) }), 'x')).toBeNull()
    expect(postedDetail(undefined, 'j')).toBeNull()
  })
})

describe('link YouTube tuỳ chọn', () => {
  it('để trống là hợp lệ; chấp nhận watch, shorts, youtu.be; từ chối nơi khác', () => {
    expect(isYoutubeLink('  ')).toBe(true)
    expect(isYoutubeLink('https://www.youtube.com/watch?v=abc123')).toBe(true)
    expect(isYoutubeLink('youtube.com/shorts/abc123')).toBe(true)
    expect(isYoutubeLink('https://youtu.be/abc123')).toBe(true)
    expect(isYoutubeLink('https://youtube.com/watch')).toBe(false)
    expect(isYoutubeLink('https://example.com/watch?v=abc123')).toBe(false)
    expect(isYoutubeLink('không phải link')).toBe(false)
  })
})
