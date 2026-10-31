// Test thuần cho khối “Lịch đăng của đối thủ” ở trang Kênh: gộp khung trùng
// thứ + giờ trên nhiều đối thủ, giữ tập thứ/khung khác nhau riêng, sắp theo thứ
// rồi giờ, cộng số video + đếm đối thủ có dữ liệu. Không dựng React.
import { describe, expect, it } from 'vitest'
import type { PostingSchedule, PostingSlot } from '../../api/rivals'
import { mergePostingSchedules, scheduleTotals, type ScheduleInput } from './schedule'

const slot = (patch: Partial<PostingSlot> = {}): PostingSlot => ({
  weekdays: [3], start: '20:00', end: '22:00', count: 10,
  weeksHit: 6, weeksTotal: 6, sinceWeeks: 0, rhythm: 'weekly', isTop: false,
  ...patch,
})

const schedule = (channelId: string, slots: PostingSlot[], patch: Partial<PostingSchedule> = {}): PostingSchedule => ({
  channelId, zone: 'vn', sample: 30, total: 30, pending: 0, pausedUntil: null,
  perWeek: 2.3, weeksTotal: 6, offSlot: 0,
  grid: Array.from({ length: 7 }, () => Array.from({ length: 12 }, () => 1)),
  slots,
  ...patch,
})

const input = (rivalId: string, rivalTitle: string, slots: PostingSlot[], patch: Partial<PostingSchedule> = {}): ScheduleInput =>
  ({ rivalId, rivalTitle, schedule: schedule(rivalId, slots, patch) })

describe('gộp lịch đăng đối thủ', () => {
  it('khung trùng thứ + giờ ở hai đối thủ thành một dòng: cộng video, gộp đối thủ', () => {
    const rows = mergePostingSchedules([
      input('r1', 'Một Phim', [slot({ count: 12, isTop: true })]),
      input('r2', 'Chuyện Lạ', [slot({ count: 8 })]),
    ])
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({
      weekdays: [3], start: '20:00', end: '22:00', count: 20,
      rivals: [{ id: 'r1', title: 'Một Phim' }, { id: 'r2', title: 'Chuyện Lạ' }],
    })
  })

  it('khác thứ hoặc khác giờ thì ở dòng riêng, sắp theo thứ rồi giờ', () => {
    const rows = mergePostingSchedules([
      input('r1', 'A', [slot({ weekdays: [5], start: '21:00', end: '23:00' })]),
      input('r2', 'B', [slot({ weekdays: [3], start: '21:00', end: '23:00' })]),
      input('r2', 'B', [slot({ weekdays: [3], start: '08:00', end: '10:00' })]),
    ])
    expect(rows.map((row) => `${row.weekdays.join(',')} ${row.start}`)).toEqual([
      '3 08:00',
      '3 21:00',
      '5 21:00',
    ])
  })

  it('hai khung cùng giờ nhưng tập thứ khác nhau không bị gộp; thứ được sắp tăng dần', () => {
    const rows = mergePostingSchedules([
      input('r1', 'A', [slot({ weekdays: [5, 3] })]),
      input('r2', 'B', [slot({ weekdays: [3] })]),
    ])
    expect(rows).toHaveLength(2)
    expect(rows[0].weekdays).toEqual([3])
    expect(rows[1].weekdays).toEqual([3, 5])
  })

  it('khung dày video nhất đứng đầu (rail của artboard), hoà mới theo thứ rồi giờ', () => {
    const rows = mergePostingSchedules([
      input('r1', 'A', [slot({ weekdays: [6], start: '19:00', end: '20:00', count: 6 })]),
      input('r1', 'A', [slot({ weekdays: [1], start: '10:00', end: '11:00', count: 9 })]),
      input('r1', 'A', [slot({ weekdays: [4], start: '20:00', end: '21:00', count: 5 })]),
      input('r2', 'B', [slot({ weekdays: [1], start: '10:00', end: '11:00', count: 8 })]),
    ])
    expect(rows.map((row) => `${row.weekdays.join(',')}:${row.count}`)).toEqual(['1:17', '6:6', '4:5'])
  })

  it('bỏ qua đối thủ chưa đọc được lịch; đếm video và đối thủ có dữ liệu', () => {
    const inputs: ScheduleInput[] = [
      input('r1', 'A', [slot()], { sample: 30 }),
      input('r2', 'B', [], { sample: 0 }),
      { rivalId: 'r3', rivalTitle: 'C', schedule: null },
    ]
    expect(mergePostingSchedules(inputs)).toHaveLength(1)
    expect(scheduleTotals(inputs)).toEqual({ videos: 30, rivals: 1 })
  })

  it('không sửa slot của input', () => {
    const original = slot({ count: 5 })
    mergePostingSchedules([
      input('r1', 'A', [original]),
      input('r2', 'B', [slot({ count: 7 })]),
    ])
    expect(original.count).toBe(5)
  })
})
