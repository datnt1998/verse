// Gộp khung đăng lặp lại của mọi đối thủ liên kết cho khối "Lịch đăng" ở trang
// Kênh. Server đã đổi múi giờ trước khi trả slots; ở đây chỉ gộp theo
// đúng (tập thứ · giờ bắt đầu · giờ kết thúc) — không đổi múi, không đoán thêm.
// Hàm thuần để test riêng (test/merge không phụ thuộc React).
import type { PostingSchedule } from '../../api/rivals'

/** Một lượt đọc posting-slots của một đối thủ (lỗi mạng → schedule null). */
export type ScheduleInput = {
  rivalId: string
  rivalTitle: string
  schedule: PostingSchedule | null
}

export type MergedSlot = {
  /** khoá gộp: "thứ|bắt đầu|kết thúc" — dùng cho React key và test */
  key: string
  /** các thứ trong tuần, tăng dần (0 = T2 … 6 = CN) */
  weekdays: number[]
  start: string
  end: string
  /** tổng số video của khung trong mẫu của từng đối thủ */
  count: number
  /** các đối thủ có khung này — giữ thứ tự xuất hiện, không trùng */
  rivals: { id: string; title: string }[]
}

const minutesOf = (hhmm: string): number => {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm.trim())
  if (!m) return -1
  return Number(m[1]) * 60 + Number(m[2])
}

/** Gộp các khung trùng thứ + giờ trên mọi đối thủ; khung dày video nhất đứng trước
 *  (đúng thứ tự rail của artboard), hoà thì theo thứ rồi giờ. */
export function mergePostingSchedules(inputs: ScheduleInput[]): MergedSlot[] {
  const merged = new Map<string, MergedSlot>()
  for (const input of inputs) {
    for (const slot of input.schedule?.slots ?? []) {
      const weekdays = [...slot.weekdays].sort((a, b) => a - b)
      const key = `${weekdays.join(',')}|${slot.start}|${slot.end}`
      const found = merged.get(key)
      if (found) {
        found.count += slot.count
        if (!found.rivals.some((rival) => rival.id === input.rivalId)) {
          found.rivals.push({ id: input.rivalId, title: input.rivalTitle })
        }
        continue
      }
      merged.set(key, {
        key,
        weekdays,
        start: slot.start,
        end: slot.end,
        count: slot.count,
        rivals: [{ id: input.rivalId, title: input.rivalTitle }],
      })
    }
  }
  return [...merged.values()].sort((a, b) =>
    b.count - a.count
    || (a.weekdays[0] ?? 7) - (b.weekdays[0] ?? 7)
    || minutesOf(a.start) - minutesOf(b.start)
    // Cùng số video + cùng ngày bắt đầu + cùng giờ: tập thứ ngắn hơn đứng trước.
    || a.weekdays.length - b.weekdays.length
    || a.key.localeCompare(b.key),
  )
}

/** Số video trong mẫu đã đọc được của mọi đối thủ + số đối thủ có dữ liệu. */
export function scheduleTotals(inputs: ScheduleInput[]): { videos: number; rivals: number } {
  let videos = 0
  let rivals = 0
  for (const input of inputs) {
    if (!input.schedule) continue
    videos += input.schedule.sample
    if (input.schedule.sample > 0) rivals += 1
  }
  return { videos, rivals }
}
