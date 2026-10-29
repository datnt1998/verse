import { describe, expect, it } from 'vitest'
import { METRIC_NONE, metricStandard, metricValue, tiredWordLimit, tiredWordName, tiredWordValue, viNumber } from './metrics'

describe('D6 — chữ của một số đo', () => {
  it('số theo cách đọc Việt Nam', () => {
    expect(viNumber(34)).toBe('34')
    expect(viNumber(2.4)).toBe('2,4')
    expect(viNumber(0.8)).toBe('0,8')
  })
  it('số luôn đi cùng đơn vị, không có điểm số trần', () => {
    expect(metricValue({ value: 34, unit: '%' })).toBe('34%')
    expect(metricValue({ value: 38, unit: 'tiếng' })).toBe('38 tiếng')
    expect(metricValue({ value: 2.4, unit: '/1.000 tiếng' })).toBe('2,4 / 1.000 tiếng')
    expect(metricValue({ value: 0, unit: 'từ' })).toBe('0 từ')
  })
  it('số đo không áp cho chương (value null) hiện “—” như artboard', () => {
    expect(metricValue({ value: null, unit: '%' })).toBe(METRIC_NONE)
    expect(METRIC_NONE).toBe('—')
  })
  it('chuẩn đọc thành lời: “≤ N” → “tối đa N”, còn lại “chuẩn …”', () => {
    expect(metricStandard('≤ 40')).toBe('tối đa 40')
    expect(metricStandard('40–55%')).toBe('chuẩn 40–55%')
    expect(metricStandard('≤ 1,5')).toBe('tối đa 1,5')
    expect(metricStandard('0')).toBe('chuẩn 0')
  })
})

describe('D6 — dòng thêm “Từ mệt nhiều nhất”', () => {
  it('nhãn, mật độ và ngưỡng như hai artboard', () => {
    expect(tiredWordName('dường như')).toBe('Từ mệt nhiều nhất · “dường như”')
    expect(tiredWordValue({ per1000: 2.4 })).toBe('2,4 / 1.000')
    expect(tiredWordValue({ per1000: 3.1 }, true)).toBe('3,1 / 1.000 tiếng')
    expect(tiredWordLimit({ limit: 1.5 })).toBe('≤ 1,5')
  })
})
