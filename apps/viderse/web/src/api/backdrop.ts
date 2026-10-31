// Client Video nền — khớp internal/web/backdrop.go.
//   POST   /api/sources/parse {input}      → 200 ParsedSource (channel|video|unsupported|invalid)
//   GET    /api/backdrop/channels          → BackdropChannel[]
//   POST   /api/backdrop/channels {input|url} → 201|200 BackdropChannel
//   GET    /api/backdrop/channels/:id      → BackdropChannel
//   DELETE /api/backdrop/channels/:id      → 204 (409 in_use kèm tên template)
//   POST   /api/backdrop/channels/:id/refresh → 200 BackdropChannel
//   GET    /api/backdrop/channels/:id/videos?kind&cursor&limit → BackdropVideoPage
//   GET    /api/backdrop/videos            → BackdropVideo[]
//   POST   /api/backdrop/videos {url,segment?} → 201 BackdropVideo (state downloading)
//   PATCH  /api/backdrop/videos/:id {segment} → 200 BackdropVideo (tải lại đoạn)
//   DELETE /api/backdrop/videos/:id        → 204 (409 in_use)
//   GET    /api/templates/:id/backdrop/mix?lengthSec → BackdropMix (404 chưa gắn kênh)
//   POST   /api/platforms/requests {host}  → 200 PlatformRequest (gộp theo host)
// Lỗi: ApiError.payload = {error,code,recovery?,retryAt?} — đọc qua backdropErrorInfo.
import { ApiError, mutate, request } from './request'

export type BackdropPlatform = 'twitch' | 'bilibili' | string
export type BackdropKind = 'clip' | 'highlight' | 'vod' | 'video'
export type BackdropVideoState = 'downloading' | 'ready' | 'failed'

/** Loại video một nền tảng cung cấp — khớp domain.PlatformKind + registry. */
export const PLATFORM_KINDS: Record<string, BackdropKind[]> = {
  twitch: ['clip', 'highlight', 'vod'],
  bilibili: ['video'],
}
export const backdropKindsOf = (platform: string): BackdropKind[] => PLATFORM_KINDS[platform] ?? ['video']

// ─── Nguồn parse ────────────────────────────────────────────────────────────

export type ParsedSource =
  | { kind: 'channel'; platform: string; id: string; handle: string; url: string }
  | { kind: 'video'; platform: string; id: string; url: string; title?: string; durationSec?: number }
  | { kind: 'unsupported'; host: string; label: string }
  | { kind: 'invalid'; message?: string; recovery?: string }

export function parseSource(input: string): Promise<ParsedSource> {
  return request<ParsedSource>('/api/sources/parse', { body: { input } })
}

// ─── Kênh nền ───────────────────────────────────────────────────────────────

export type BackdropTemplateUsage = {
  templateId: string
  name: string
  mode: 'random' | 'pick' | string
  kinds: string[]
  chosenCount: number
}

export type BackdropChannel = {
  id: string
  platform: BackdropPlatform
  handle: string
  url: string
  title: string | null
  avatarUrl: string | null
  addedAt: string
  /** null = đang đọc danh sách lần đầu */
  listedAt: string | null
  listError: { code: string; message: string } | null
  scanNextPage: number | null
  scanTotal: number
  scanStopKnown: boolean
  /** số video dùng được theo kind (không tính unavailable) */
  counts: Partial<Record<string, number>>
  pausedUntil: string | null
  pauseReason: string | null
  usedBy: BackdropTemplateUsage[]
  bilibili?: {
    budget: {
      short: { used: number; limit: number; availableAt: string }
      day: { used: number; limit: number; availableAt: string }
    }
    pausedUntil: string | null
    pauseReason: string | null
    nextPageAt: string | null
    pending: { channelId: string; nextPage: number | null; maxPages: number }[]
  }
}

export type ChannelVideoItem = {
  videoId: string
  kind: string
  title: string
  durationSec: number | null
  publishedAt: string | null
  thumbnailUrl: string | null
  url: string
  unavailable?: { reason: 'expired' | 'live' | 'unavailable' | 'login_required' | string; at: string }
  chosenBy: { templateId: string; name: string }[]
}

export type BackdropVideoPage = {
  items: ChannelVideoItem[]
  counts: Partial<Record<string, number>>
  total: number
  unavailable: number
  nextCursor?: string
}

export const listBackdropChannels = (signal?: AbortSignal) =>
  request<BackdropChannel[]>('/api/backdrop/channels', { signal })

export const getBackdropChannel = (id: string, signal?: AbortSignal) =>
  request<BackdropChannel>(`/api/backdrop/channels/${encodeURIComponent(id)}`, { signal })

export function addBackdropChannel(input: string): Promise<BackdropChannel> {
  return request<BackdropChannel>('/api/backdrop/channels', { body: { input } })
}

export function refreshBackdropChannel(id: string): Promise<BackdropChannel> {
  return mutate<BackdropChannel>('POST', `/api/backdrop/channels/${encodeURIComponent(id)}/refresh`, {})
}

export const removeBackdropChannel = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/backdrop/channels/${encodeURIComponent(id)}`)

/** Trang danh mục video của kênh (keyset; `cursor` rỗng = trang đầu). */
export function listChannelVideos(
  id: string,
  opts: { kinds?: string[]; cursor?: string; limit?: number } = {},
  signal?: AbortSignal,
): Promise<BackdropVideoPage> {
  const params = new URLSearchParams()
  if (opts.kinds?.length) params.set('kind', opts.kinds.join(','))
  if (opts.cursor) params.set('cursor', opts.cursor)
  if (opts.limit) params.set('limit', String(opts.limit))
  const s = params.toString()
  return request<BackdropVideoPage>(`/api/backdrop/channels/${encodeURIComponent(id)}/videos${s ? `?${s}` : ''}`, { signal })
}

// ─── Video lẻ ───────────────────────────────────────────────────────────────

export type BackdropSegment = { fromSec: number; toSec: number }

export type BackdropVideo = {
  id: string
  platform: BackdropPlatform
  url: string
  title: string
  durationSec: number | null
  segment: BackdropSegment
  file?: { path: string; height: number; bytes: number }
  state: BackdropVideoState
  /** 0..1 khi đang tải */
  progress?: number
  errorDetail?: { code: string; message: string }
  usedBy: { templateId: string; name: string }[]
  addedAt: string
  updatedAt: string
}

export const listBackdropVideos = (signal?: AbortSignal) =>
  request<BackdropVideo[]>('/api/backdrop/videos', { signal })

export function addBackdropVideo(url: string, segment?: BackdropSegment): Promise<BackdropVideo> {
  return request<BackdropVideo>('/api/backdrop/videos', { body: segment ? { url, segment } : { url } })
}

export function patchBackdropVideo(id: string, segment: BackdropSegment): Promise<BackdropVideo> {
  return mutate<BackdropVideo>('PATCH', `/api/backdrop/videos/${encodeURIComponent(id)}`, { segment })
}

export const removeBackdropVideo = (id: string): Promise<void> =>
  mutate<void>('DELETE', `/api/backdrop/videos/${encodeURIComponent(id)}`)

export const backdropVideoFileHref = (id: string) => `/api/backdrop/videos/${encodeURIComponent(id)}/file`

// ─── Ước lượng nền của template ─────────────────────────────────────────────

export type BackdropMix = {
  /** tổng giây phát của video lẻ ghim (đã chia tốc độ nền) */
  fixedSec: number
  perChannel: { channelId: string; clips: number; freshVideos: number; repeats: boolean }[]
  loopSec: number
  need: number
  capped: boolean
}

export function getBackdropMix(templateId: string, lengthSec = 0, signal?: AbortSignal): Promise<BackdropMix> {
  const s = lengthSec > 0 ? `?lengthSec=${lengthSec}` : ''
  return request<BackdropMix>(`/api/templates/${encodeURIComponent(templateId)}/backdrop/mix${s}`, { signal })
}

// ─── Yêu cầu nền tảng ───────────────────────────────────────────────────────

export type PlatformRequest = { host: string; label: string; count: number; firstAt: string; lastAt: string }

export function requestPlatformSupport(host: string): Promise<PlatformRequest> {
  return request<PlatformRequest>('/api/platforms/requests', { body: { host } })
}

// ─── Lỗi ────────────────────────────────────────────────────────────────────

export type BackdropErrorInfo = { code: string | null; recovery: string | null; retryAt: string | null }

/** Mã + gợi ý của envelope lỗi backdrop ({error,code,recovery?,retryAt?}). */
export function backdropErrorInfo(err: unknown): BackdropErrorInfo {
  if (!(err instanceof ApiError) || !err.payload || typeof err.payload !== 'object') {
    return { code: null, recovery: null, retryAt: null }
  }
  const p = err.payload as Record<string, unknown>
  return {
    code: typeof p.code === 'string' ? p.code : null,
    recovery: typeof p.recovery === 'string' ? p.recovery : null,
    retryAt: typeof p.retryAt === 'string' ? p.retryAt : null,
  }
}

export const isInUseError = (err: unknown): boolean =>
  err instanceof ApiError && err.status === 409
