// Khung đăng → tập ô trên bảng thứ × khối-2-giờ + câu độ đều. Bấm một khung
// trong UI sẽ tô đúng các ô này (set "day:block").
import { describe, expect, it } from 'vitest'
import { gridMax, slotCells, slotRhythm } from './posting'
import type { PostingSlot } from '../../api/rivals'

const slot = (patch: Partial<PostingSlot> = {}): PostingSlot => ({
  weekdays: [0], start: '19:00', end: '20:00', count: 14, weeksHit: 6, weeksTotal: 6,
  sinceWeeks: 0, rhythm: 'weekly', isTop: true, ...patch,
})

describe('slotCells — ô được bấm sáng', () => {
  it('khung 19:00–20:00 T2 chỉ phủ khối 18:00–20:00 của thứ 2', () => {
    expect([...slotCells(slot())].sort()).toEqual(['0:9'])
  })

  it('khung 20:00–22:00 hai thứ → hai ô (mỗi thứ một khối 2h trùng khít)', () => {
    const cells = slotCells(slot({ weekdays: [3, 5], start: '20:00', end: '22:00' }))
    expect([...cells].sort()).toEqual(['3:10', '5:10'])
  })

  it('khoảng lẻ phút phủ đúng các ô giao nhau (18:30–20:30 → khối 9 và 10)', () => {
    const cells = slotCells(slot({ start: '18:30', end: '20:30' }))
    expect([...cells].sort()).toEqual(['0:10', '0:9'])
  })

  it('khung qua nửa đêm 23:00–01:00 phủ khối 11 và khối 0', () => {
    const cells = slotCells(slot({ start: '23:00', end: '01:00' }))
    expect([...cells].sort()).toEqual(['0:0', '0:11'])
  })

  it('giờ sai định dạng → rỗng', () => {
    expect(slotCells(slot({ start: 'sai', end: '20:00' })).size).toBe(0)
  })
})

describe('slotRhythm — câu độ đều', () => {
  it('weekly: "Mỗi tuần · 6/6 tuần"', () => {
    expect(slotRhythm(slot())).toBe('Mỗi tuần · 6/6 tuần')
  })
  it('most_weeks: "Gần như mỗi tuần · 5/6"', () => {
    expect(slotRhythm(slot({ rhythm: 'most_weeks', weeksHit: 5 }))).toBe('Gần như mỗi tuần · 5/6')
  })
  it('biweekly', () => {
    expect(slotRhythm(slot({ rhythm: 'biweekly' }))).toBe('Hai tuần một lần')
  })
  it('new: "Mới · từ 3 tuần nay"', () => {
    expect(slotRhythm(slot({ rhythm: 'new', sinceWeeks: 3 }))).toBe('Mới · từ 3 tuần nay')
  })
})

describe('gridMax', () => {
  it('lớn nhất của grid, 0 khi trống', () => {
    expect(gridMax([[1, 9], [4]])).toBe(9)
    expect(gridMax([])).toBe(0)
  })
})
