import { describe, expect, it } from 'vitest'
import type { ReviewEntry, ReviewItem } from '../../api/types'
import { gateReviewItems, gateReviewMeta, gateRoundNote } from './GateCard'

const error: ReviewItem = {
  id: 'e1', level: 'error', ref: 'Master §7', refName: 'Đầu chương và cuối chương',
  text: 'Chương kết ở trạng thái an toàn.', quote: 'Tôi tắt đèn.', fix: 'Kết bằng tin nhắn từ số lạ.',
}
const warning: ReviewItem = {
  id: 'w1', level: 'warning', ref: 'Master §13', refName: 'Văn phong, thoại, hài',
  text: 'Thoại chiếm 34%.', quote: 'Tôi kể lại…', fix: 'Chuyển thành đối thoại.',
}
const warning2: ReviewItem = { ...warning, id: 'w2', ref: 'Bộ A §4.3', refName: 'Trà xanh và phản diện' }

const entry = (over: Partial<ReviewEntry> = {}): ReviewEntry => ({
  chapter: 3, scope: 'chapter', issues: [], items: [warning, warning2], round: 2,
  pack: { id: 'nu-tan-bao-thu', ten: 'Nữ tần báo thù' }, blocking: false, verdict: 'accept', summary: '', ...over,
}) as ReviewEntry

describe('dòng phụ “Thẩm duyệt chương N”', () => {
  it('lần chấm sạch: “Đạt · còn 2 cảnh báo · chấm theo gói Nữ tần báo thù, lần 2”', () => {
    expect(gateReviewMeta(entry())).toBe('Đạt · còn 2 cảnh báo · chấm theo gói Nữ tần báo thù, lần 2')
  })
  it('còn Bắt sửa thì nêu số mục bắt sửa thay cho số cảnh báo', () => {
    expect(gateReviewMeta(entry({ items: [error, warning], verdict: 'polish', blocking: true })))
      .toBe('Cần gọt giũa · còn 1 mục bắt sửa · chấm theo gói Nữ tần báo thù, lần 2')
  })
  it('thiếu gói/lần chấm thì bỏ phần đó; không có thẩm duyệt thì rỗng', () => {
    expect(gateReviewMeta(entry({ pack: null, round: undefined }))).toBe('Đạt · còn 2 cảnh báo')
    expect(gateReviewMeta(entry({ items: [] }))).toBe('Đạt · chấm theo gói Nữ tần báo thù, lần 2')
    expect(gateReviewMeta(null)).toBe('')
  })
})

describe('mục của cửa duyệt', () => {
  it('chia đúng Bắt sửa / Cảnh báo', () => {
    const split = gateReviewItems(entry({ items: [error, warning, warning2] }))
    expect(split.errors.map((item) => item.id)).toEqual(['e1'])
    expect(split.warnings.map((item) => item.id)).toEqual(['w1', 'w2'])
    expect(gateReviewItems(null)).toEqual({ errors: [], warnings: [] })
  })
})

describe('dải xanh lần chấm trước', () => {
  it('chỉ hiện khi đã chấm lại và không còn mục bắt sửa', () => {
    expect(gateRoundNote(entry())).toBe('Lần chấm 1 có mục bắt sửa. Người viết đã sửa, chương 3 giờ đạt.')
  })
  it('dùng previous của lần chấm trước: câu đúng artboard', () => {
    const withPrior = entry({
      previous: [{ round: 1, verdict: 'polish', rule_aware: false, items: [{ ...error, text: 'chương kết ở trạng thái an toàn' }] }],
    })
    expect(gateRoundNote(withPrior))
      .toBe('Lần chấm 1 có 1 mục bắt sửa (Master §7 · chương kết ở trạng thái an toàn). Người viết đã sửa, chương 3 giờ đạt.')
  })
  it('previous nhiều mục bắt sửa nối bằng “; ”, lần trước sạch thì không có dải xanh', () => {
    const many = entry({
      previous: [{ round: 1, verdict: 'polish', rule_aware: false, items: [{ ...error, text: 'a' }, { ...error, id: 'e2', ref: 'Bộ A §4.3', text: 'b' }] }],
    })
    expect(gateRoundNote(many)).toBe('Lần chấm 1 có 2 mục bắt sửa (Master §7 · a; Bộ A §4.3 · b). Người viết đã sửa, chương 3 giờ đạt.')
    expect(gateRoundNote(entry({ previous: [{ round: 1, verdict: 'accept', rule_aware: false, items: [warning] }] }))).toBe('')
    expect(gateRoundNote(entry({ round: 3, previous: [{ round: 1, verdict: 'polish', rule_aware: false, items: [error] }] })))
      .toBe('Lần chấm 2 có mục bắt sửa. Người viết đã sửa, chương 3 giờ đạt.')
  })
  it('lần chấm đầu hoặc còn bắt sửa thì không có dải xanh', () => {
    expect(gateRoundNote(entry({ round: 1 }))).toBe('')
    expect(gateRoundNote(entry({ round: undefined }))).toBe('')
    expect(gateRoundNote(entry({ items: [error] }))).toBe('')
    expect(gateRoundNote(null)).toBe('')
  })
})
