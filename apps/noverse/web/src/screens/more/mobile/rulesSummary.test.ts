import { describe, expect, it } from 'vitest'
import { rulesSub } from './rulesSummary'

describe('tóm tắt quy tắc ở trang Thêm', () => {
  it('nhãn dòng', () => {
    expect([rulesSub(undefined), rulesSub(0), rulesSub(9)]).toEqual(['Đang đếm…', 'Chưa có quy tắc nào', '9 quy tắc'])
  })
})
