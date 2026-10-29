// D6 — nối số đo với dòng Nhật ký. Host không phát sự kiện “Đo bằng máy” riêng: số đo của chương
// nằm ở `GET …/metrics`, nên dòng số đo bám sau lần nộp chương của đúng chương đó.
import type { HostEvent } from '../../api/types'

/** Chương trong tham số công cụ (`commit_chapter` nhận `{"chapter":N}`) — chuẩn hơn `chapter` của sự kiện. */
function chapterFromArgs(args?: string): number | null {
  if (!args) return null
  try {
    const parsed: unknown = JSON.parse(args)
    const chapter = parsed && typeof parsed === 'object' && 'chapter' in parsed ? Number(parsed.chapter) : Number.NaN
    return Number.isFinite(chapter) && chapter > 0 ? chapter : null
  } catch {
    return null
  }
}

/**
 * Chương cần hiện dòng “Đo bằng máy · chương N” sau sự kiện này; null = không gắn.
 * Chỉ lần nộp chương thành công mới có số đo (chương đã viết); sự kiện lỗi hoặc thiếu số chương → null.
 * Số chương lấy từ tham số công cụ trước, vì `chapter` của sự kiện là chương của phiên (có thể đã sang chương sau).
 */
export function metricsChapterOf(event: HostEvent & { args?: string; chapter?: number; tool?: string }): number | null {
  if (event.tool !== 'commit_chapter' || event.failed || event.level === 'error') return null
  const chapter = chapterFromArgs(event.args) ?? event.chapter ?? 0
  return chapter > 0 ? chapter : null
}
