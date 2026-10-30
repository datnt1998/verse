// Client khu vực đối thủ (Khám phá) — khớp internal/web/rivals.go.
//   GET    /api/rivals                       → { rivals, totalVideos, newOutliers }
//   POST   /api/rivals { input }             → 201 Rival · lỗi {error,code,recovery?,retryAt?,rival?}
//   DELETE /api/rivals/:id                   → 204
//   GET    /api/rivals/:id/videos?sort&long&within&q&cursor&limit → { videos, total, nextCursor }
//   GET    /api/rivals/:id/scan              → ScanStatus
//   POST   /api/rivals/:id/scan { action }   → ScanStatus
//   PATCH  /api/rivals/:id/scan {historyCap} → ScanStatus
//   GET    /api/rivals/:id/posting-slots?zone=vn|ny → PostingSchedule
// `:id = "all"` chỉ hợp lệ ở /videos — quét và lịch đăng là của từng kênh.
import { mutate, request, ApiError } from './request'

export type RivalScan = { state: HistoryState; discoveryRunning: boolean }

export type Rival = {
  id: string
  handle: string | null
  title: string
  avatarUrl: string | null
  url: string
  subscriberCount: number | null
  /** số video đã lập chỉ mục */
  videoCount: number
  /** mức thường — trung vị lượt xem của 150 video mới nhất */
  medianViews: number | null
  /** số video có điểm ≥ 3 */
  outliers: number
  /** video nổi trội mới (điểm ≥ 3, phát hiện ≤ 7 ngày) — nhãn "N mới" ở cột trái */
  newOutliers: number
  addedAt: string
  scannedAt: string | null
  scan: RivalScan
}

export type RivalList = { rivals: Rival[]; totalVideos: number; newOutliers: number }

export type VideoSort = 'outlier' | 'newest' | 'views'

export type Video = {
  id: string
  channelId: string
  title: string
  durationSec: number | null
  viewCount: number | null
  /** uploadedAt nếu đã đọc giờ chính xác, không thì giờ ước lượng */
  publishedAt: string | null
  /** publishedAt là giờ đăng chính xác (đã đọc) hay ước lượng */
  exactTime: boolean
  score: number | null
  /** phụ đề do kênh đăng; null = chưa đọc */
  hasCaptions: boolean | null
  thumbnailUrl: string
  url: string
}

export type VideoPage = { videos: Video[]; total: number; nextCursor: string | null }

export type VideoQuery = {
  sort?: VideoSort
  /** 1 = chỉ video ≥ 8 phút */
  long?: 0 | 1
  /** 0|7|30|90 ngày gần đây; 0 = mọi thời gian */
  within?: 0 | 7 | 30 | 90
  q?: string
  cursor?: string | null
  limit?: number
}

export type HistoryState = 'idle' | 'queued' | 'running' | 'backoff' | 'paused' | 'capped' | 'complete' | 'cancelled' | 'error'

export type HistoryStatus = {
  state: HistoryState
  jobId: string | null
  scanned: number
  added: number
  pages: number
  reachedEnd: boolean
  startedAt: string | null
  updatedAt: string | null
  /** backoff: lúc tự thử lại */
  nextAttemptAt: string | null
  errorCode: string | null
  error: string | null
  recovery: string | null
}

export type DiscoveryStatus = { checkedAt: string | null; caughtUp: boolean; running: boolean; nextAt: string | null }

export type ScanStatus = {
  channelId: string
  historyCap: number
  indexedVideos: number
  rescanHours: number
  history: HistoryStatus
  discovery: DiscoveryStatus
}

export type ScanAction = 'start' | 'pause' | 'resume' | 'cancel' | 'check'

export type PostingZone = 'vn' | 'ny'

export type PostingRhythm = 'weekly' | 'most_weeks' | 'biweekly' | 'new'

export type PostingSlot = {
  /** 0 = T2 … 6 = CN theo múi giờ đang xem */
  weekdays: number[]
  /** "HH:MM" */
  start: string
  end: string
  count: number
  weeksHit: number
  weeksTotal: number
  /** rhythm 'new': số tuần gần nhất khung xuất hiện */
  sinceWeeks: number
  rhythm: PostingRhythm
  isTop: boolean
}

export type PostingSchedule = {
  channelId: string
  zone: PostingZone
  sample: number
  /** số video trong mẫu đã có giờ đăng chính xác */
  total: number
  /** số video trong mẫu chưa đọc giờ đăng */
  pending: number
  /** việc đọc giờ đăng đang tạm nghỉ tới */
  pausedUntil: string | null
  /** số video mỗi tuần trong mẫu; null khi < 2 video */
  perWeek: number | null
  weeksTotal: number
  offSlot: number
  /** thứ × khối 2 giờ (0 = 00:00–02:00) */
  grid: number[][]
  slots: PostingSlot[]
}

/** Mã lỗi của POST /api/rivals — hiện ngay dưới ô nhập. */
export type AddRivalCode = 'invalid_input' | 'not_found' | 'exists' | 'rate_limited' | 'blocked' | 'unexpected_page'

/** code + phần bổ sung của envelope lỗi ({error,code,recovery?,retryAt?,rival?}). */
export function apiErrorInfo(err: unknown): { code: string | null; retryAt: string | null; recovery: string | null; rival: Rival | null } {
  const empty = { code: null, retryAt: null, recovery: null, rival: null }
  if (!(err instanceof ApiError) || !err.payload || typeof err.payload !== 'object') return empty
  const p = err.payload as Record<string, unknown>
  return {
    code: typeof p.code === 'string' ? p.code : null,
    retryAt: typeof p.retryAt === 'string' ? p.retryAt : null,
    recovery: typeof p.recovery === 'string' ? p.recovery : null,
    rival: p.rival && typeof p.rival === 'object' ? (p.rival as Rival) : null,
  }
}

export const listRivals = (signal?: AbortSignal) => request<RivalList>('/api/rivals', { signal })

export function addRival(input: string): Promise<Rival> {
  return request<Rival>('/api/rivals', { body: { input } })
}

export function removeRival(id: string): Promise<void> {
  return mutate<void>('DELETE', `/api/rivals/${encodeURIComponent(id)}`)
}

export function listRivalVideos(id: string, query: VideoQuery = {}, signal?: AbortSignal): Promise<VideoPage> {
  const params = new URLSearchParams()
  if (query.sort) params.set('sort', query.sort)
  if (query.long) params.set('long', '1')
  if (query.within) params.set('within', String(query.within))
  if (query.q) params.set('q', query.q)
  if (query.cursor) params.set('cursor', query.cursor)
  if (query.limit) params.set('limit', String(query.limit))
  const s = params.toString()
  return request<VideoPage>(`/api/rivals/${encodeURIComponent(id)}/videos${s ? `?${s}` : ''}`, { signal })
}

export const getRivalScan = (id: string, signal?: AbortSignal) =>
  request<ScanStatus>(`/api/rivals/${encodeURIComponent(id)}/scan`, { signal })

export function controlRivalScan(id: string, action: ScanAction): Promise<ScanStatus> {
  return request<ScanStatus>(`/api/rivals/${encodeURIComponent(id)}/scan`, { body: { action } })
}

export function setRivalScanCap(id: string, historyCap: number): Promise<ScanStatus> {
  return mutate<ScanStatus>('PATCH', `/api/rivals/${encodeURIComponent(id)}/scan`, { historyCap })
}

export const getPostingSlots = (id: string, zone: PostingZone, signal?: AbortSignal) =>
  request<PostingSchedule>(`/api/rivals/${encodeURIComponent(id)}/posting-slots?zone=${zone}`, { signal })
