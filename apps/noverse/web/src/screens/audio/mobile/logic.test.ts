import { describe, expect, it } from 'vitest'
import type { AudioChapter } from '../../../api/types'
import { chapterEnded, formatMinutes, nextSleep, pendingNotice, playerQueue, sleepExpired, startSleep, stepChapter } from './logic'

const chapter = (number: number, done: number, total: number, rendering = false): AudioChapter => ({
  chapter: number, title: `Chương ${number}`, status: 'locked', issues: 0, durationSec: 100,
  lockedAt: null, segments: total, speakers: 1, changedSegments: 0,
  audio: { segmentsDone: done, segments: total, durationSec: 100, rendering, percent: done / (total || 1) },
})

describe('hàng đợi trình phát điện thoại', () => {
  it('chỉ xếp chương đủ giọng, không rỗng, không còn render, đúng thứ tự', () => {
    expect(playerQueue([chapter(4, 2, 2), chapter(2, 1, 2), chapter(1, 2, 2), chapter(3, 2, 2, true), chapter(5, 0, 0)]))
      .toEqual([1, 4])
  })
  it('tiến/lùi bỏ qua chương chưa sẵn sàng và không lặp ở hai đầu', () => {
    expect(stepChapter([1, 4, 6], 1, 1)).toBe(4)
    expect(stepChapter([1, 4, 6], 6, -1)).toBe(4)
    expect(stepChapter([1, 4, 6], 6, 1)).toBeNull()
    expect(stepChapter([1, 4, 6], 1, -1)).toBeNull()
    expect(stepChapter([], null, 1)).toBeNull()
    expect(stepChapter([1, 4], 2, 1)).toBe(4)
  })
  it('tự phát chương tiếp theo khi kết thúc, dừng ở cuối sách', () => {
    expect(chapterEnded([1, 4], 1, startSleep('off', 0), 0)).toEqual({ next: 4, sleep: false })
    expect(chapterEnded([1, 4], 4, startSleep('off', 0), 0)).toEqual({ next: null, sleep: false })
  })
})

describe('hẹn giờ tắt', () => {
  it('15/30 phút dùng mốc giờ thực, hết đúng hạn cả khi tab đã ngủ', () => {
    const timer = startSleep('15', 1000)
    expect(timer.deadline).toBe(901000)
    expect(sleepExpired(timer, 900999)).toBe(false)
    expect(sleepExpired(timer, 901000)).toBe(true)
    expect(sleepExpired(timer, 1901000)).toBe(true)
    expect(startSleep('30', 1000).deadline).toBe(1801000)
  })
  it('hết chương ngăn tự chuyển và chế độ tắt không tự dừng', () => {
    const timer = startSleep('chapter', 0)
    expect(sleepExpired(timer, 9999999)).toBe(false)
    expect(chapterEnded([1, 4], 1, timer, 100)).toEqual({ next: null, sleep: true })
    expect(sleepExpired(startSleep('off', 0), 9999999, true)).toBe(false)
    expect(chapterEnded([1, 4], 1, startSleep('15', 0), 900000)).toEqual({ next: null, sleep: true })
  })
  it('chuyển đúng vòng tắt → 15 → 30 → hết chương → tắt và đặt lại deadline', () => {
    let timer = startSleep('off', 0)
    timer = nextSleep(timer, 100)
    expect(timer).toEqual({ mode: '15', deadline: 900100 })
    timer = nextSleep(timer, 200)
    expect(timer).toEqual({ mode: '30', deadline: 1800200 })
    timer = nextSleep(timer, 300)
    expect(timer).toEqual({ mode: 'chapter', deadline: null })
    expect(nextSleep(timer, 400)).toEqual({ mode: 'off', deadline: null })
  })
})

describe('đồng hồ trình phát điện thoại (M3: m:ss, không đệm số 0 đầu phút)', () => {
  it('khớp bản mẫu: 312 s → 5:12, 800 − 312 → 8:08, 822 s → 13:42', () => {
    expect(formatMinutes(312)).toBe('5:12')
    expect(formatMinutes(800 - 312)).toBe('8:08')
    expect(formatMinutes(822)).toBe('13:42')
  })
  it('0 s, giá trị âm/NaN không vỡ và phút vượt 59 vẫn dồn thành phút', () => {
    expect(formatMinutes(0)).toBe('0:00')
    expect(formatMinutes(-4)).toBe('0:00')
    expect(formatMinutes(Number.NaN)).toBe('0:00')
    expect(formatMinutes(4503)).toBe('75:03')
    expect(formatMinutes(59.6)).toBe('1:00')
  })
})

describe('cảnh báo chương chưa xong trong sheet xuất', () => {
  const job = (state: 'queued' | 'rendering' | 'paused' | 'error' | 'done', etaSec: number) => ({ state, etaSec })
  it('đang tạo giọng: nêu số chương, ước lượng phút (cộng hàng đợi) và số chương đã xong', () => {
    expect(pendingNotice(3, 9, [job('rendering', 400), job('queued', 340), job('queued', 340)]))
      .toBe('Còn 3 chương đang tạo giọng (khoảng 18 phút). Xuất ngay sẽ chỉ gồm 9 chương đã xong.')
  })
  it('bỏ qua việc đã xong/lỗi/tạm dừng khi cộng; dưới một phút vẫn nói 1 phút', () => {
    expect(pendingNotice(1, 11, [job('done', 900), job('error', 900), job('paused', 900), job('rendering', 10)]))
      .toBe('Còn 1 chương đang tạo giọng (khoảng 1 phút). Xuất ngay sẽ chỉ gồm 11 chương đã xong.')
  })
  it('không có việc nào đang chạy: không hứa ước lượng', () => {
    expect(pendingNotice(3, 9, [])).toBe('Còn 3 chương chưa có giọng. Xuất ngay sẽ chỉ gồm 9 chương đã xong.')
    expect(pendingNotice(3, 9, [job('error', 60)])).toBe('Còn 3 chương chưa có giọng. Xuất ngay sẽ chỉ gồm 9 chương đã xong.')
  })
})
