import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import type { ReviewItem } from '../api/types'
import { reviewEvent3Items } from '../../mock/data'
import {
  ReviewIssue, reviewBlocking, reviewClock, reviewFixPrefill, reviewIssueCounts, reviewIssueFrame, reviewItemsOf, reviewKeptLine, reviewLevel,
} from './ReviewIssue'

const error: ReviewItem = {
  id: 'e1', level: 'error', ref: 'Master §7', refName: 'Đầu chương và cuối chương',
  text: 'Chương kết ở trạng thái an toàn.', quote: 'Tôi tắt đèn, kéo chăn lên.', fix: 'Kết bằng tin nhắn từ số lạ.',
}
/** Lỗi lint xưng hô mức Bắt sửa (22-brief-2510.md › câu 13) — chặn duyệt như mọi mục khác. */
const addressError: ReviewItem = {
  id: 'ch3-xh-direction', level: 'error', ref: 'Xưng hô', refName: 'Tri Hạ → Nhược Vy',
  text: 'Tri Hạ gọi Nhược Vy là “em”.', quote: 'Em về trước đi, chuyện này để chị lo.', fix: 'Đổi “em” thành “cô”.',
}
const warning: ReviewItem = {
  id: 'w1', level: 'warning', ref: 'Master §13', refName: 'Văn phong, thoại, hài',
  text: 'Thoại chiếm 34%.', quote: 'Tôi kể lại cho anh nghe chuyện buổi thử vai…', fix: 'Chuyển đoạn kể thành đối thoại.',
}
const warning2: ReviewItem = { ...warning, id: 'w2', ref: 'Bộ A §4.3', refName: 'Trà xanh và phản diện', quote: '', fix: '' }

describe('mức của mục thẩm duyệt', () => {
  it('máy chủ đã chuẩn hoá: error là Bắt sửa, warning là Cảnh báo', () => {
    expect(reviewLevel({ level: 'error' })).toBe('error')
    expect(reviewLevel({ level: 'warning' })).toBe('warning')
  })
  it('đếm mục và chỉ chặn duyệt khi có Bắt sửa', () => {
    expect(reviewIssueCounts([error, warning, warning2])).toEqual({ all: 3, errors: 1, warnings: 2 })
    expect(reviewIssueCounts([])).toEqual({ all: 0, errors: 0, warnings: 0 })
    expect(reviewBlocking([warning, warning2])).toBe(false)
    expect(reviewBlocking([warning, error])).toBe(true)
    expect(reviewItemsOf(null)).toEqual([])
    expect(reviewItemsOf(undefined)).toEqual([])
  })
  it('lỗi lint Xưng hô mức Bắt sửa chặn duyệt y như mọi mục Bắt sửa (câu 13)', () => {
    expect(reviewBlocking([addressError, warning])).toBe(true)
    expect(reviewIssueCounts([addressError, warning])).toEqual({ all: 2, errors: 1, warnings: 1 })
  })
  it('chỉ dẫn “Yêu cầu sửa”', () => {
    expect(reviewFixPrefill(error)).toBe('sửa theo Master §7: kết bằng tin nhắn từ số lạ.')
  })
  it('khung thẻ theo màn: thẻ Trình đọc bo 14, khối lồng bo 10 nền surface-0', () => {
    expect(reviewIssueFrame('error', 'card')).toMatchObject({ borderRadius: 14, padding: '12px 14px', border: '1px solid var(--kit-errf-22)' })
    expect(reviewIssueFrame('warning', 'card').border).toBe('1px solid var(--line-2)')
    expect(reviewIssueFrame('warning', 'inset')).toMatchObject({ borderRadius: 10, padding: '8px 12px', background: 'var(--color-surface-0)' })
  })
})

describe('ví dụ lint xưng hô trong Nhật ký (câu 11, 13)', () => {
  it('lỗi lint thật thay “Chưa rõ ai nói”: một Bắt sửa + một Cảnh báo cùng nguồn Xưng hô', () => {
    const address = reviewEvent3Items.filter((item) => item.ref === 'Xưng hô')
    expect(address.map((item) => item.level)).toEqual(['error', 'warning'])
    expect(address[0]?.refName).toBe('Tri Hạ → Nhược Vy')
    expect(address[1]?.refName).toBe('Bảng xưng hô của truyện')
    expect(address[1]?.text).toBe('Kỳ Dương gọi Tri Hạ là “cậu”, bảng xưng hô ghi “chị” từ chương 2.')
    expect(reviewEvent3Items.some((item) => item.refName.includes('Chưa rõ ai nói'))).toBe(false)
    expect(reviewBlocking(reviewEvent3Items)).toBe(true)
    expect(reviewIssueCounts(reviewEvent3Items)).toEqual({ all: 5, errors: 2, warnings: 3 })
  })
})

describe('giờ và lý do giữ nguyên (GAP-47)', () => {
  const at = new Date(2026, 9, 25, 21, 58).toISOString()
  it('giờ:phút của mốc RFC3339; mốc hỏng hoặc thiếu thì bỏ trống', () => {
    expect(reviewClock(at)).toBe('21:58')
    expect(reviewClock('')).toBe('')
    expect(reviewClock(null)).toBe('')
    expect(reviewClock('không phải giờ')).toBe('')
  })
  it('dòng đã giữ: “Lý do: … · Dat, 21:58”; thiếu giờ thì chỉ còn tên', () => {
    expect(reviewKeptLine({ state: 'kept', reason: 'Cố ý, hợp giọng truyện', by: 'Dat', at }))
      .toBe('Lý do: Cố ý, hợp giọng truyện · Dat, 21:58')
    expect(reviewKeptLine({ state: 'kept', reason: 'Sẽ xử lý ở chương sau', by: 'Dat', at: '' }))
      .toBe('Lý do: Sẽ xử lý ở chương sau · Dat')
    expect(reviewKeptLine({ state: 'kept', reason: 'Biên tập viên hiểu sai', by: '', at: '' }))
      .toBe('Lý do: Biên tập viên hiểu sai')
  })
})

describe('thẻ thẩm duyệt', () => {
  it('bản đầy đủ: nhãn, số mục luật, tên mục, câu, đoạn trích, gợi ý, trạng thái và hai hành động', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={warning} variant="full" onKeep={() => {}} onRequestFix={() => {}} />)
    expect(html).toContain('Cảnh báo')
    expect(html).toContain('Master §13')
    expect(html).toContain('Văn phong, thoại, hài')
    expect(html).toContain('Thoại chiếm 34%.')
    expect(html).toContain('“Tôi kể lại cho anh nghe chuyện buổi thử vai…”')
    expect(html).toContain('Gợi ý:')
    expect(html).toContain('Tuỳ bạn')
    expect(html).toContain('>Giữ nguyên<')
    expect(html).toContain('>Yêu cầu sửa<')
    expect(html).not.toContain('var(--kit-errf-22)')
  })
  it('mục Bắt sửa chỉ báo đã vào hàng chờ sửa, không có nút', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={error} variant="full" onKeep={() => {}} onRequestFix={() => {}} />)
    expect(html).toContain('Bắt sửa')
    expect(html).toContain('Đã vào hàng chờ sửa')
    expect(html).toContain('var(--kit-errf-22)')
    expect(html).not.toContain('>Giữ nguyên<')
    expect(html).not.toContain('>Yêu cầu sửa<')
  })
  it('lỗi lint Xưng hô mức Bắt sửa hiện như mọi mục Bắt sửa, không có nút giữ (câu 13)', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={addressError} variant="full" onKeep={() => {}} onRequestFix={() => {}} />)
    expect(html).toContain('Bắt sửa')
    expect(html).toContain('Xưng hô')
    expect(html).toContain('Tri Hạ → Nhược Vy')
    expect(html).toContain('Đã vào hàng chờ sửa')
    expect(html).not.toContain('>Giữ nguyên<')
  })
  it('bản gate bỏ đoạn trích và hành động, giữ gợi ý', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={warning} variant="gate" onKeep={() => {}} onRequestFix={() => {}} />)
    expect(html).toContain('Gợi ý:')
    expect(html).not.toContain('“Tôi kể lại')
    expect(html).not.toContain('>Yêu cầu sửa<')
    expect(html).toContain('var(--color-surface-0)')
  })
  it('bản log giữ đoạn trích, bỏ gợi ý và hành động', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={warning} variant="log" onRequestFix={() => {}} />)
    expect(html).toContain('“Tôi kể lại')
    expect(html).not.toContain('Gợi ý:')
    expect(html).not.toContain('>Yêu cầu sửa<')
  })
  it('bản compact chỉ còn nhãn và câu', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={warning} variant="compact" />)
    expect(html).toContain('Master §13')
    expect(html).toContain('Thoại chiếm 34%.')
    expect(html).not.toContain('“Tôi kể lại')
    expect(html).not.toContain('Gợi ý:')
    expect(html).not.toContain('Tuỳ bạn')
  })
  it('đã giữ: hiện “✓ Lý do: … · Dat, 21:58” và “Bỏ giữ”, ẩn nút giữ/yêu cầu sửa', () => {
    const at = new Date(2026, 9, 25, 21, 58).toISOString()
    const html = renderToStaticMarkup(<ReviewIssue item={warning} variant="full"
      decision={{ state: 'kept', reason: 'Cố ý, hợp giọng truyện', by: 'Dat', at }}
      onKeep={() => {}} onUnkeep={() => {}} onRequestFix={() => {}} />)
    expect(html).toContain('Giữ nguyên')
    expect(html).toContain('✓')
    expect(html).toContain('Lý do: Cố ý, hợp giọng truyện · Dat, 21:58')
    expect(html).toContain('Bỏ giữ')
    expect(html).not.toContain('>Yêu cầu sửa<')
    expect(html).toContain('var(--color-ok)')
  })
  it('mục không có số mục luật vẫn hiện câu và đoạn trích', () => {
    const html = renderToStaticMarkup(<ReviewIssue item={{ ...warning, ref: '', refName: '' }} variant="full" />)
    expect(html).toContain('Thoại chiếm 34%.')
    expect(html).not.toContain('Master §13')
  })
})
