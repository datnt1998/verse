// Lịch đăng (§12) — hiển thị PostingSchedule: nhãn độ đều của từng khung và
// tập ô trên bảng thứ × khối-2-giờ mà một khung bao phủ (bấm khung → làm nổi).
// Grid và slots đều đã ở múi giờ đang xem (server đổi múi), nên chỉ khớp thứ
// và khoảng giờ — không đổi múi ở client.
import { t } from '../../i18n/explore'
import type { PostingSchedule, PostingSlot } from '../../api/rivals'

/** "HH:MM" → phút trong ngày; giá trị sai trả -1. */
const minutesOf = (hhmm: string): number => {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm.trim())
  if (!m) return -1
  return Number(m[1]) * 60 + Number(m[2])
}

/** Câu độ đều của một khung — artboard: "Mỗi tuần · 6/6 tuần" · "Gần như mỗi
 *  tuần · 5/6" · "Mới · từ 3 tuần nay" · "Hai tuần một lần". */
export function slotRhythm(slot: PostingSlot): string {
  const r = t.screens.explore.postingRhythm
  switch (slot.rhythm) {
    case 'weekly': return r.weekly(slot.weeksHit, slot.weeksTotal)
    case 'most_weeks': return r.mostWeeks(slot.weeksHit, slot.weeksTotal)
    case 'new': return r.new(slot.sinceWeeks)
    case 'biweekly':
    default: return r.biweekly
  }
}

/** Ô (thứ, khối-2-giờ) thuộc khung: thứ nằm trong weekdays và khoảng giờ của
 *  ô giao với [start, end) — khung qua nửa đêm (vd 23:00–00:00) cắt hai đầu. */
export function slotCells(slot: PostingSlot): Set<string> {
  const cells = new Set<string>()
  const start = minutesOf(slot.start)
  const end = minutesOf(slot.end)
  if (start < 0 || end < 0) return cells
  for (const day of slot.weekdays) {
    if (day < 0 || day > 6) continue
    for (let block = 0; block < 12; block++) {
      const from = block * 120
      const covers = start < end ? start < from + 120 && from < end : start < from + 120 || from < end
      if (covers) cells.add(`${day}:${block}`)
    }
  }
  return cells
}

/** Số lớn nhất trong grid — mẫu số cho độ đậm của ô (0 khi chưa có dữ liệu). */
export const gridMax = (grid: number[][]): number =>
  grid.reduce((max, row) => Math.max(max, ...(row ?? [])), 0)
