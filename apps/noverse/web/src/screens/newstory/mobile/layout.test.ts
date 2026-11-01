import { describe, expect, it } from 'vitest'
import { DRAFT_LABEL, draftCard, draftStage, draftStartable, quickChrome } from './layout'

describe('M11 · bàn phím mở thì ẩn tiêu đề và khối phụ, nút nằm trên bàn phím', () => {
  it('bàn phím đóng, chưa so truyện trước: đủ tiêu đề, orb, câu dẫn, khối phụ; nút sát đáy', () => {
    expect(quickChrome({ keyboard: 0, check: false })).toEqual({ heading: true, orbs: true, subtitle: true, extras: true, barOffset: 0 })
  })

  it('bàn phím mở: chỉ còn ô yêu cầu và nút, nút nhảy lên đúng chiều cao bàn phím', () => {
    expect(quickChrome({ keyboard: 336, check: false })).toEqual({ heading: false, orbs: false, subtitle: false, extras: false, barOffset: 336 })
  })

  it('đã so với truyện trước: giữ tiêu đề nhưng bỏ orb, câu dẫn và khối phụ', () => {
    expect(quickChrome({ keyboard: 0, check: true })).toEqual({ heading: true, orbs: false, subtitle: false, extras: false, barOffset: 0 })
  })

  it('số đo bàn phím lẻ không làm nút lệch nửa điểm ảnh', () => {
    expect(quickChrome({ keyboard: 291.6, check: true }).barOffset).toBe(292)
  })
})

describe('M11 · thẻ Bản nháp yêu cầu', () => {
  it('chưa có nháp: Chưa có, không có dòng đầu', () => {
    expect(draftStage({ draft: '  ', ready: false, busy: false, blocked: false })).toBe('none')
    expect(draftCard('none', '')).toEqual({ label: DRAFT_LABEL.none, tone: 'neutral', line: '' })
  })

  it('đang chờ AI: Đang cập nhật thắng cả bản nháp cũ đã sẵn sàng', () => {
    const stage = draftStage({ draft: 'Truyện 8 chương', ready: true, busy: true, blocked: false })
    expect(stage).toBe('updating')
    expect(draftCard(stage, 'Truyện 8 chương').tone).toBe('neutral')
  })

  it('còn mục Bắt sửa: nhãn đỏ và nút Bắt đầu không bấm được dù nháp đã đủ ý', () => {
    const stage = draftStage({ draft: 'Truyện 8 chương', ready: true, busy: false, blocked: true })
    expect(draftCard(stage, 'Truyện 8 chương')).toEqual({ label: 'Còn 1 mục Bắt sửa', tone: 'err', line: 'Truyện 8 chương' })
    expect(draftStartable(stage, true)).toBe(false)
  })

  it('sẵn sàng viết: xanh, mời bấm khi còn ghi được', () => {
    const stage = draftStage({ draft: 'Truyện 8 chương', ready: true, busy: false, blocked: false })
    expect(draftCard(stage, 'Truyện 8 chương').tone).toBe('ok')
    expect(draftStartable(stage, true)).toBe(true)
    expect(draftStartable(stage, false)).toBe(false)
  })

  it('đang làm rõ: dòng đầu lấy dòng có chữ đầu tiên, gộp khoảng trắng', () => {
    const stage = draftStage({ draft: '\n\n  Cô thợ may   ở khu chợ\nKết: mở tiệm riêng', ready: false, busy: false, blocked: false })
    expect(stage).toBe('clarifying')
    expect(draftCard(stage, '\n\n  Cô thợ may   ở khu chợ\nKết: mở tiệm riêng').line).toBe('Cô thợ may   ở khu chợ')
  })
})
