import { describe, expect, it } from 'vitest'
import { chapterLabel, formatDuration, formatNumber, formatTimer, formatTokens, formatUSD } from './format'

describe('formatDuration — 07-copy ví dụ', () => {
  it('mili giây → “N giây” vi-VN', () => {
    expect(formatDuration(137)).toBe('0,1 giây')
    expect(formatDuration(29)).toBe('0,03 giây')
    expect(formatDuration(15_600)).toBe('15,6 giây')
    expect(formatDuration(12_000)).toBe('12 giây')
    expect(formatDuration(99_400)).toBe('1 phút 39 giây')
    expect(formatDuration(60_000)).toBe('1 phút')
    expect(formatDuration(3_720_000)).toBe('1 giờ 2 phút')
    expect(formatDuration(3_600_000)).toBe('1 giờ')
  })

  it('không có thời lượng → chuỗi rỗng', () => {
    expect(formatDuration(0)).toBe('')
    expect(formatDuration(-5)).toBe('')
    expect(formatDuration(null)).toBe('')
    expect(formatDuration(undefined)).toBe('')
  })
})

describe('formatTimer — đồng hồ đếm lên', () => {
  it('m:ss dưới một giờ, h:mm:ss sau đó', () => {
    expect(formatTimer(49_000)).toBe('0:49')
    expect(formatTimer(725_000)).toBe('12:05')
    expect(formatTimer(3_723_000)).toBe('1:02:03')
    expect(formatTimer(0)).toBe('0:00')
    expect(formatTimer(null)).toBe('0:00')
  })
})

describe('formatTokens — 07-copy ví dụ', () => {
  it('dạng dài', () => {
    expect(formatTokens(3_500_000)).toBe('3,5 triệu token')
    expect(formatTokens(488_700)).toBe('488,7 nghìn token')
    expect(formatTokens(2_000_000)).toBe('2 triệu token')
    expect(formatTokens(870)).toBe('870')
    expect(formatTokens(0)).toBe('0')
    expect(formatTokens(null)).toBe('0')
  })

  it('dạng gọn thanh trạng thái: “4,0 tr vào · 87,0K ra”', () => {
    expect(formatTokens(4_000_000, 'short')).toBe('4,0 tr')
    expect(formatTokens(87_000, 'short')).toBe('87,0K')
    expect(formatTokens(500, 'short')).toBe('500')
  })
})

describe('formatNumber / formatUSD / chapterLabel', () => {
  it('nghìn chấm, thập phân phẩy', () => {
    expect(formatNumber(58_699)).toBe('58.699')
    expect(formatNumber(0)).toBe('0')
    expect(formatUSD(2.05)).toBe('$2,05')
    expect(formatUSD(2.64)).toBe('$2,64')
    expect(formatUSD(null)).toBe('$0,00')
  })

  it('Chương N — không viết tắt', () => {
    expect(chapterLabel(4)).toBe('Chương 4')
    expect(chapterLabel(25)).toBe('Chương 25')
  })
})
