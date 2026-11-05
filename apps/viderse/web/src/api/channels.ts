// Client mảng Kênh của bạn — khớp internal/web/channels.go.
//   GET    /api/owner-channels                    → { channels: Channel[] }
//   POST   /api/owner-channels                    → 201 { channel, identityJobId }
//   GET    /api/owner-channels/:id                → Channel
//   PATCH  /api/owner-channels/:id                → Channel (null = xoá giá trị)
//   DELETE /api/owner-channels/:id                → 204
//   POST   /api/owner-channels/:id/sources        → Channel (idempotent)
//   DELETE /api/owner-channels/:id/sources/:rid   → { autoFetchPaused }
//   POST   /api/owner-channels/:id/branding       → 202 { channel, identityJobId }
//   GET    /api/owner-channels/:id/branding/:file → ảnh (?download=1 để tải về)
// Lỗi luôn có vỏ {error, code, recovery?, retryAt?}.
import { ApiError, mutate, request } from './request'

export type ChannelLanguage = 'en' | 'vi' | 'auto'
export type IdentityState = 'ready' | 'generating' | 'failed'
export type IdentityPart = 'all' | 'name' | 'avatar' | 'banner'
export type ChannelAgent = 'scout' | 'director' | 'artist' | 'qa'
export type AgentInstructions = Partial<Record<ChannelAgent, string>>

export type Branding = { brief: string; summary: string; look: string; avatar: string; banner: string }

export type ChannelSource = {
  rivalId: string
  title: string
  handle: string | null
  linkedAt: string
  /** đối thủ tham chiếu nhận diện (link sớm nhất) */
  reference: boolean
}


export type ChannelIdentity = {
  state: IdentityState
  /** step: bước hỏng — 'text' Đạo diễn, 'images' Hoạ sĩ; thiếu = không rõ. */
  error?: { code: string; message: string; step?: 'text' | 'images' }
  retryAt?: string
  startedAt?: string
  part?: IdentityPart
  jobId?: string
}

export type Channel = {
  id: string
  /** "" = chưa có tên (nhận diện sẽ đặt) */
  name: string
  youtubeUrl: string | null
  language: ChannelLanguage
  styleNote: string | null
  descriptionFooter: string | null
  alwaysTags: string[]
  autoFetch: boolean
  autoApprove: boolean
  /** 1..20 */
  perDay: number
  defaultTemplateId: string
  defaultTemplateName: string
  agentInstructions: AgentInstructions
  branding: Branding
  avatarUrl: string | null
  bannerUrl: string | null
  sources: ChannelSource[]
  identity: ChannelIdentity
  createdAt: string
  updatedAt: string
}

export type ChannelList = { channels: Channel[] }

export type CreateChannelInput = {
  name?: string
  youtubeUrl?: string
  /** bắt buộc ≥ 1 — server trả 400 khi rỗng */
  competitorIds: string[]
  autoFetch?: boolean
  autoApprove?: boolean
  perDay?: number
  /** Bỏ trống để tạo template chuẩn cùng kênh. */
  defaultTemplateId?: string
  language?: ChannelLanguage
}

export type ChannelPatch = {
  name?: string
  /** null = xoá link */
  youtubeUrl?: string | null
  language?: ChannelLanguage
  styleNote?: string | null
  descriptionFooter?: string | null
  alwaysTags?: string[]
  autoFetch?: boolean
  autoApprove?: boolean
  perDay?: number
  defaultTemplateId?: string
  /** Cập nhật các trợ lý có mặt trong bản vá, giữ nguyên các trợ lý khác. */
  agentInstructions?: AgentInstructions
  brief?: string
}


export type BrandingRequest = { part?: IdentityPart; brief?: string; direction?: string }

type FieldError = { field: string; message: string; recovery?: string }

/** Mã + phần bổ sung của envelope lỗi (fields chỉ có ở 400 của PATCH). */
export type ChannelErrorInfo = {
  code: string | null
  recovery: string | null
  retryAt: string | null
  fields: FieldError[]
}

const EMPTY: ChannelErrorInfo = { code: null, recovery: null, retryAt: null, fields: [] }

export function channelErrorInfo(err: unknown): ChannelErrorInfo {
  if (!(err instanceof ApiError) || !err.payload || typeof err.payload !== 'object') return EMPTY
  const body = err.payload as Record<string, unknown>
  const str = (key: string) => (typeof body[key] === 'string' ? (body[key] as string) : null)
  const fields = Array.isArray(body.fields)
    ? (body.fields as unknown[]).flatMap((item) => {
      if (!item || typeof item !== 'object') return []
      const f = item as Record<string, unknown>
      if (typeof f.field !== 'string' || typeof f.message !== 'string') return []
      return [{ field: f.field, message: f.message, recovery: typeof f.recovery === 'string' ? f.recovery : undefined }]
    })
    : []
  return { code: str('code'), recovery: str('recovery'), retryAt: str('retryAt'), fields }
}

/** Lỗi theo từng trường — hiện ngay dưới ô (PATCH 400). */
export function channelFieldErrors(err: unknown): Record<string, string> {
  const out: Record<string, string> = {}
  for (const field of channelErrorInfo(err).fields) out[field.field] = field.message
  return out
}

export const listChannels = (signal?: AbortSignal) =>
  request<ChannelList>('/api/owner-channels', { signal })

export const getChannel = (id: string, signal?: AbortSignal) =>
  request<Channel>(`/api/owner-channels/${encodeURIComponent(id)}`, { signal })

export const createChannel = (input: CreateChannelInput) =>
  request<{ channel: Channel; identityJobId: string | null }>('/api/owner-channels', { body: input })

export const patchChannel = (id: string, patch: ChannelPatch) =>
  mutate<Channel>('PATCH', `/api/owner-channels/${encodeURIComponent(id)}`, patch)

export const deleteChannel = (id: string) =>
  mutate<void>('DELETE', `/api/owner-channels/${encodeURIComponent(id)}`)

export const linkSource = (id: string, rivalId: string) =>
  mutate<Channel>('POST', `/api/owner-channels/${encodeURIComponent(id)}/sources`, { rivalId })

export const unlinkSource = (id: string, rivalId: string) =>
  mutate<{ autoFetchPaused: boolean }>(
    'DELETE',
    `/api/owner-channels/${encodeURIComponent(id)}/sources/${encodeURIComponent(rivalId)}`,
  )


export const generateBranding = (id: string, input: BrandingRequest = {}) =>
  mutate<{ channel: Channel; identityJobId: string }>(
    'POST',
    `/api/owner-channels/${encodeURIComponent(id)}/branding`,
    input,
  )

/** Đường tải avatar/ảnh bìa của kênh; `download` → Content-Disposition attachment. */
export function brandingHref(channelId: string, url: string | null, download = false): string | undefined {
  if (!url) return undefined
  return download ? `${url}${url.includes('?') ? '&' : '?'}download=1` : url
}
