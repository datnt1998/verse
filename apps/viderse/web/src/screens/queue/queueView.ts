// View-model thuần của Hàng chờ: sort items, tính position float khi kéo
// thả, soạn nhãn ETA/lý do chưa chạy, phát hiện trạng thái "hết hạn mức".
import type { QueueImageQuota, QueueQueuedItem, QueueView } from '../../api/clone'
import { t } from '../../i18n/queue'

const tt = t.screens.queue

/** Thứ tự mục chờ: position float tăng dần (fallback giữ nguyên thứ tự API). */
export const sortItems = (items: readonly QueueQueuedItem[]): QueueQueuedItem[] =>
  [...items].sort((a, b) => (a.position ?? 0) - (b.position ?? 0))

/** Vị trí float mới khi thả `id` lên trước `beforeId` (null → lên đầu,
 *  không có beforeId và không lên đầu → xuống cuối). Số thực midpoint —
 *  server ghi đúng giá trị này vào clone_queue.position. */
export function movePosition(sorted: readonly QueueQueuedItem[], id: string, beforeId: string | null): number | null {
  const list = sorted.filter((i) => i.id !== id)
  if (list.length === 0) return null
  const pos = (i: QueueQueuedItem, idx: number) => i.position ?? idx + 1
  if (beforeId === null) return pos(list[0], 0) - 1
  const beforeIdx = list.findIndex((i) => i.id === beforeId)
  if (beforeIdx < 0) return pos(list[list.length - 1], list.length - 1) + 1
  const after = pos(list[beforeIdx], beforeIdx)
  const prev = beforeIdx > 0 ? pos(list[beforeIdx - 1], beforeIdx - 1) : after - 2
  return (prev + after) / 2
}

/** Id của mục ngay sau `id` trong danh sách — đích thả khi kéo xuống một nấc. */
export const nextId = (sorted: readonly QueueQueuedItem[], id: string): string | null => {
  const idx = sorted.findIndex((i) => i.id === id)
  return idx >= 0 && idx < sorted.length - 1 ? sorted[idx + 1].id : null
}

/** "00:04" / "Ngày mai 09:30" / "26/10 09:30" từ ISO; "—" khi chưa có ETA. */
export function fmtEta(iso: string | null, now = Date.now()): string {
  if (!iso) return '—'
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  const time = `${pad(at.getHours())}:${pad(at.getMinutes())}`
  const today = new Date(now)
  const sameDay = at.toDateString() === today.toDateString()
  if (sameDay) return time
  const tomorrow = new Date(now); tomorrow.setDate(today.getDate() + 1)
  if (at.toDateString() === tomorrow.toDateString()) return `${tt.etaTomorrow} ${time}`
  return `${pad(at.getDate())}/${pad(at.getMonth() + 1)} ${time}`
}

/** Lý do chưa chạy của một mục chờ — nhãn artboard theo waitReason của server. */
export function waitLine(item: QueueQueuedItem, paused: boolean): string {
  if (paused) return tt.reasons.queuePaused
  switch (item.waitReason) {
    case 'queue_paused': return tt.reasons.queuePaused
    case 'workers_busy': return tt.reasons.workersBusy
    case 'paused_channel': return tt.reasons.channelPaused
    case 'pace_full': return tt.reasons.paceFull
    case 'rotation': return tt.reasons.rotation
    default: return item.waitReason || (item.priority === 'pick' ? tt.reasons.pick : '')
  }
}

/** Nhãn nguồn của mục: "Ưu tiên · bạn chọn · <source>" | "<source>". */
export const sourceLabel = (item: QueueQueuedItem): string =>
  item.priority === 'pick' && item.source
    ? `${tt.priority} · ${item.source}`
    : item.source || tt.batch

/**
 * Trạng thái màn theo thứ tự: paused → empty → limited → running (map tweak artboard).
 * Hàng chờ rỗng đứng trước limited: không còn việc nào chờ thì "Máy đang rảnh" đúng
 * hơn "hết hạn mức", vì hạn mức chỉ chặn bước vẽ ảnh của video đang có.
 */
export function viewState(view: QueueView): 'running' | 'paused' | 'limited' | 'empty' {
  if (view.paused) return 'paused'
  if (view.running.length === 0 && view.items.length === 0) return 'empty'
  if (view.imageQuota.length > 0 && view.imageQuota.every((quota) => quota.limit !== null && quota.used >= quota.limit)) return 'limited'
  return 'running'
}

/** Chỉ báo hết hạn mức khi mọi nguồn đang cấu hình đều đã cạn. */
export function quotaLine(quota: readonly QueueImageQuota[], now = Date.now()): { title: string; detail: string } | null {
  if (quota.length === 0 || quota.some((item) => item.limit === null || item.used < item.limit)) return null
  const reset = quota.map((item) => item.resetsAt).filter((value): value is string => !!value).sort()[0]
  const sources = quota.map((item) => tt.banners.quotaSource(quotaName(item), quotaText(item), item.period, item.role === 'fallback')).join(', ')
  return {
    title: tt.banners.limitedTitle,
    detail: tt.banners.limitedBody(sources, reset ? fmtEta(reset, now) : null),
  }
}

/** Hạn mức theo từng nguồn cho cột Quy tắc chạy. */
export const quotaText = (q: QueueImageQuota): string =>
  q.limit === null
    ? `${q.used} ảnh hôm nay`
    : q.used >= q.limit
      ? tt.rules.quotaLimit
      : `${Math.max(0, q.limit - q.used)} ${tt.rules.quotaFree}`

/** Tên nguồn vẽ thân thiện: nhãn thẻ Kết nối AI từ server, sau đó tên loại nguồn. */
export const quotaName = (q: QueueImageQuota): string =>
  q.label || (tt.rules.quotaSourceNames as Record<string, string>)[q.provider] || q.provider
