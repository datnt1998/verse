import { describe, expect, it } from 'vitest'
import { attentionTone, strongestTone } from './attentionTone'

describe('tông chấm cảnh báo theo loại', () => {
  it('map từng loại đã chốt; loại lạ mặc định cam', () => {
    expect(attentionTone('tool_failed')).toBe('err')
    expect(attentionTone('login_expired')).toBe('err')
    expect(attentionTone('tool_update')).toBe('info')
    expect(attentionTone('quota_low')).toBe('warn')
    expect(attentionTone('something_new')).toBe('err')
  })

  it('một chấm gộp nhiều việc lấy tông khẩn nhất', () => {
    expect(strongestTone(['tool_update', 'quota_low'])).toBe('warn')
    expect(strongestTone(['tool_update', 'quota_low', 'login_expired'])).toBe('err')
    expect(strongestTone([])).toBeUndefined()
  })
})
