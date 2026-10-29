// Logic thuần của Não hệ thống (SysBrain.dc.html › renderVals): dòng nguồn, dòng đã quyết,
// danh sách tab Chờ duyệt giữ bài vừa quyết tại chỗ (có Hoàn tác) và thanh tỉ lệ số truyện vi phạm.
import type { Lesson, ViolationRow } from '../../api/types'
import { t } from '../../i18n/vi'

const copy = t.sysbrain

/** “25/10” theo giờ máy người xem. */
export function dayMonth(iso: string | undefined): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}`
}

/**
 * Dòng dưới thẻ đã quyết. `just` = vừa bấm ở tab Chờ duyệt (artboard: “Đã giữ · thành quy tắc …”);
 * ở tab Đã giữ / Đã bỏ là ngày quyết + cấp quy tắc.
 */
export function doneLine(lesson: Lesson, just: boolean): string {
  if (lesson.state === 'kept') {
    return just ? copy.keptNow(lesson.scope, lesson.packName) : copy.keptOn(dayMonth(lesson.decidedAt), lesson.scope, lesson.packName)
  }
  return just ? copy.droppedNow : copy.droppedOn(dayMonth(lesson.decidedAt))
}

/**
 * Tab Chờ duyệt: bài vừa giữ/bỏ vẫn đứng đúng chỗ cũ (kèm Hoàn tác) cho tới khi đổi tab.
 * `order` = id theo lần đầu thấy; `server` = danh sách chờ duyệt mới nhất; `local` = kết quả POST.
 */
export function pendingRows(order: string[], server: Lesson[], local: Record<string, Lesson>): Lesson[] {
  const byId = new Map(server.map((lesson) => [lesson.id, lesson]))
  const ids = [...order, ...server.map((lesson) => lesson.id).filter((id) => !order.includes(id))]
  const rows: Lesson[] = []
  for (const id of ids) {
    const fresh = byId.get(id)
    const mine = local[id]
    // Bài vừa quyết: giữ bản POST trả về. Bài còn chờ: bản máy chủ mới nhất (có thể đã sửa câu chữ).
    const row = mine && mine.state !== 'pending' ? mine : fresh ?? mine
    if (row) rows.push(row)
  }
  return rows
}

/** Thanh tỉ lệ: số truyện vi phạm / số truyện của gói (artboard: `Math.round(n / mx * 100) + '%'`). */
export function violationBar(row: ViolationRow, packStories: number): { label: string; width: string } {
  const width = packStories > 0 ? Math.round((Math.min(row.stories, packStories) / packStories) * 100) : 0
  return { label: `${row.stories}/${packStories}`, width: `${width}%` }
}

/**
 * Câu 2: tooltip chung của chuông và nhãn “· mọi gói” (artboard `v.tip`). Mục dùng chung
 * (`alertScope: 'all'`) ghi số truyện tính trên mọi gói; mục `Bộ …` ghi n/tổng truyện của gói.
 */
export function alertTip(row: ViolationRow): string {
  return row.alertScope === 'all'
    ? copy.alertTipAll(row.alertStories ?? 0)
    : copy.alertTipPack(row.stories, row.total)
}

/** Câu 2: “· mọi gói” chỉ đứng dưới mục dùng chung ĐANG bật chuông. */
export function showsScopeAll(row: ViolationRow): boolean {
  return row.alertScope === 'all' && row.alert
}
