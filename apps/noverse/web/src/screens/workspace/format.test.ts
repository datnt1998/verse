// Gói Q: `formatClock` chạy cho mỗi dòng nhật ký và mỗi giây (`useNow`) — trước đây mỗi lần gọi
// lại dựng một `Intl.DateTimeFormat` mới, đo được ~116 ms mỗi lần mở Bàn viết. Test này giữ cho
// formatter chỉ được dựng một lần.
import { describe, expect, it, vi } from 'vitest'
import { formatClock, formatShortClock } from './format'

describe('formatClock — không dựng Intl.DateTimeFormat mỗi lần gọi', () => {
  it('40 mốc giờ khác nhau không gọi toLocaleTimeString (thủ phạm dựng Intl mỗi lần)', () => {
    const spy = vi.spyOn(Date.prototype, 'toLocaleTimeString')
    for (let i = 0; i < 40; i++) formatClock(`2026-10-2${i % 9}T1${i % 10}:0${i % 6}:0${i % 10}Z`)
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })

  it('nhớ kết quả theo chuỗi ISO và vẫn đúng định dạng giờ:phút:giây', () => {
    expect(formatClock('2026-10-26T03:04:05Z')).toMatch(/^\d{2}:\d{2}:\d{2}$/)
    expect(formatShortClock('2026-10-26T03:04:05Z')).toMatch(/^\d{2}:\d{2}$/)
    expect(formatClock(null)).toBe('—')
    expect(formatClock('không-phải-ngày')).toBe('—')
  })
})
