import { describe, expect, it } from 'vitest'
import { budgetPercent, stepBudget, storySetChangedKeys, storySetConfig, storySetValues, usd } from './storySetLogic'

const effective = { style: 'trinh thám', advance_mode: 'auto' as const, budget: { book_usd: 12, warn_ratio: 0.8 } }

describe('Tuỳ chỉnh truyện', () => {
  it('giá trị hiển thị lấy bản nháp, rơi về cấu hình đang có hiệu lực', () => {
    expect(storySetValues(effective, {})).toEqual({ advance: 'auto', budget: 12 })
    expect(storySetValues(effective, { advance: 'review', budget: 15 })).toEqual({ advance: 'review', budget: 15 })
  })

  it('chỉ đếm khoá khác giá trị hiện tại; chọn lại giá trị cũ không là thay đổi', () => {
    expect(storySetChangedKeys(effective, { advance: 'auto', budget: 12 })).toEqual([])
    expect(storySetChangedKeys(effective, { budget: 13 })).toEqual(['budget'])
  })

  it('áp dụng ghi đúng khoá đã đổi và giữ phần còn lại của cấp', () => {
    const target = { style: 'romance', roles: { writer: { provider: 'a', model: 'b' } }, budget: { warn_ratio: 0.5 } }
    expect(storySetConfig(target, effective, { advance: 'review', budget: 20 })).toEqual({
      style: target.style, roles: target.roles, advance_mode: 'review', budget: { warn_ratio: 0.5, book_usd: 20 },
    })
  })

  it('nút ± bước 1, không xuống dưới 1; phần trăm không quá 100', () => {
    expect([stepBudget(12, 1), stepBudget(1, -1), stepBudget(2.5, -1)]).toEqual([13, 1, 1.5])
    expect([budgetPercent(3.2, 10), budgetPercent(30, 10), budgetPercent(3, 0)]).toEqual(['32%', '100%', '0%'])
    expect([usd(12), usd(3.2)]).toEqual(['$12', '$3,20'])
  })
})
