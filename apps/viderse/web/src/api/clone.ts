// Wire DTO của internal/domain/clone-request.go và clone_queue.go.
import { invalidate } from '@noverse/ui/data'
import { keys } from '../data/keys'
import { mutate, request } from './request'
import { paths } from '../router'

/** request body chung: đặt kênh đích (bắt buộc) + danh sách URL + tuỳ chọn. */
export type CloneInput = { channelId: string; urls: string[]; templateId?: string }

export type ClonePlanAccepted = {
  url: string
  videoId: string
  title: string
  durationSec: number | null
  outlier: number | null
  hasCaptions: boolean
  /** §4 mở rộng — thông tin nguồn để vẽ phần xem trước. */
  thumbnailUrl?: string | null
  sourceChannelTitle?: string | null
  viewCount?: number | null
  publishedAt?: string | null
  sourceChannelId?: string | null
  rivalLinked?: boolean
  /** số ảnh trả phí dự kiến / số lần gọi / giá (null khi chưa ước lượng) */
  imageEstimate?: number | null
  imageCalls?: number | null
  priceUSD?: number | null
}

/** jobId = null khi URL chỉ mới xếp hàng (queue item), chưa tạo job. */
export type ClonePlanDuplicate = {
  url: string
  jobId: string | null
  queueItemId?: string
  state: string
  title?: string
  thumbnailUrl?: string | null
  durationSec?: number | null
  outlier?: number | null
  hasCaptions?: boolean | null
  sourceChannelTitle?: string | null
  viewCount?: number | null
  publishedAt?: string | null
}
export type ClonePlanInvalid = { url: string; reason: 'unreadable' | 'private' | 'not_youtube' | string }
export type ClonePlanTemplate = { id: string; name: string }

export type ClonePlan = {
  accepted: ClonePlanAccepted[]
  duplicates: ClonePlanDuplicate[]
  invalid: ClonePlanInvalid[]
  /** Manual additions enter the front of the pending queue. */
  position: string
  etaStart: string | null
  paceLeftToday: number
  /** số video (tính từ đầu danh sách) sẽ qua ngày mai */
  overflowDays: number
  template: ClonePlanTemplate
  imageCalls: number
  priceUSD: number | null
}

/** Canonical POST /api/jobs result, including mixed-input feedback. */
export type CloneEnqueueResult = {
  items: { id: string; jobId: string | null }[]
  duplicates?: ClonePlanDuplicate[]
  invalid?: ClonePlanInvalid[]
}

export const planClone = (body: CloneInput, signal?: AbortSignal) =>
  request<ClonePlan>('/api/clone/plan', { body, signal })
export const enqueueClone = async (body: CloneInput, signal?: AbortSignal) => {
  const result = await request<CloneEnqueueResult>('/api/jobs', { body, signal })
  void invalidate(keys.queue())
  void invalidate(keys.status())
  return result
}

export const cloneTarget = (item: { jobId: string | null; queueItemId?: string }) =>
  item.jobId ? paths.workbench({ video: item.jobId }) : item.queueItemId ? paths.workbench({ queueItem: item.queueItemId }) : null

export type QueueItemTarget = { id: string; channelId: string; jobId: string | null; state: string }
export const getQueueItem = (id: string, signal?: AbortSignal) =>
  request<QueueItemTarget>(`/api/queue/items/${encodeURIComponent(id)}`, { signal })

// ── Hàng chờ (§3 · internal/domain/clone_queue.go) ───────────────────────────

export type QueueRunningItem = {
  jobId: string
  channelId: string
  title: string
  /** trợ lý đang đảm nhiệm step này (scout|director|artist|qa) */
  agent: string
  /** mô tả việc đang làm — chuỗi server soạn sẵn ("Đang tải video gốc · 63%") */
  now: string
  /** 0–8 theo CloneSteps */
  stepIndex: number
  etaMinutes: number | null
  /** mở rộng cho thẻ (xem GAP) */
  thumbnailUrl?: string | null
  templateName?: string | null
  durationSec?: number | null
  startedAt?: string | null
  /** pausedAt của chính job — note "Làm nốt bước này" */
  pausedAt?: string | null
}

export type QueueSkipReason = 'pace_full' | 'paused_channel' | 'workers_busy' | string
export type QueueSkippedChannel = { channelId: string; reason: QueueSkipReason; until?: string | null }
export type QueueRotation = { order: string[]; next: string | null; skipped: QueueSkippedChannel[] }

/** waitReason do server soạn: queue_paused | workers_busy | rotation | paused_channel | pace_full */
export type QueueWaitReason = 'queue_paused' | 'workers_busy' | 'rotation' | 'paused_channel' | 'pace_full' | ''

export type QueueQueuedItem = {
  /** id của queue item — đích của PATCH move/remove */
  id: string
  channelId: string
  title: string
  /** tên kênh đối thủ nguồn */
  source: string
  templateName: string
  durationSec: number | null
  /** 'pick' = video bạn chọn, 'batch' = video hàng loạt */
  priority: 'pick' | 'batch'
  etaStart: string | null
  waitReason: QueueWaitReason
  position: number
  thumbnailUrl?: string | null
  sourceUrl?: string | null
}

/** Nhịp hôm nay của một kênh (chỉ đếm job batch). */
export type QueuePace = { channelId: string; used: number; limit: number }

export type QueueImageQuota = {
  provider: 'gpt' | 'gemini' | 'local' | string
  /** Tên hiển thị của thẻ Kết nối AI (server gửi); rỗng → suy từ provider. */
  label?: string
  used: number
  limit: number | null
  period: 'day' | 'week' | string
  resetsAt: string | null
  role: 'main' | 'fallback' | string
}

export type QueueDoneToday = { jobId: string; title: string; at: string; state: 'ready' | 'review' | string }
export type QueueWorkers = { used: number; total: number }

export type QueueView = {
  paused: boolean
  /** thời điểm bấm Tạm dừng gần nhất (clone.queuePausedAt — xem GAP) */
  pausedAt?: string | null
  workers: QueueWorkers
  imageWorkers: number
  rescanHours: number
  running: QueueRunningItem[]
  rotation: QueueRotation
  items: QueueQueuedItem[]
  pace: QueuePace[]
  imageQuota: QueueImageQuota[]
  doneToday: QueueDoneToday[]
  /** Receipt returned only after an immediate, reversible removal. */
  removed?: { id: string; position: number; priority: 'pick' | 'batch' }
}

export const getQueue = (signal?: AbortSignal) =>
  request<QueueView>('/api/queue', { signal })

/** PATCH /api/queue (domain.QueuePatch, flat): pause/resume, Quy tắc chạy,
 *  move theo `position` float midpoint, action "front"|"remove". */
export type QueuePatch =
  | { paused: boolean }
  | { workers: number }
  | { id: string; position: number }
  | { id: string; action: 'front' | 'remove' | 'restore' }

export const patchQueue = async (body: QueuePatch) => {
  const view = await mutate<QueueView>('PATCH', '/api/queue', body)
  void invalidate(keys.queue())
  void invalidate(keys.status())
  return view
}
