// Client khu vực hiệu quả sau đăng — hợp đồng 07 §21, khớp máy chủ:
//   POST   /api/clone/:id/posted {youtubeUrl?}            → {youtubeId: string|null}
//   GET    /api/channels/:id/performance                  → Performance
//   POST   /api/channels/:id/insights/:insightId/apply    → {kind: 'rule'|'rotation', ruleId?}
//   GET    /api/youtube/data-key                          → {configured, keyHint?}
//   PUT    /api/youtube/data-key {key}                    → cùng hình
//   DELETE /api/youtube/data-key                          → 204
// Lỗi: ApiError.payload giữ {error, field?} (400 posted: field = "youtubeUrl").
import { z } from 'zod'
import { ApiError, mutate, request } from './request'

const stat = z.object({
  views: z.number(),
  likes: z.number().nullish(),
  comments: z.number().nullish(),
  readAt: z.string(),
})
export type PostStat = z.infer<typeof stat>

const posted = z.object({
  jobId: z.string(),
  youtubeId: z.string().nullish(),
  postedAt: z.string().nullish(),
  at24h: stat.nullish(),
  at7d: stat.nullish(),
  at28d: stat.nullish(),
  vsMedian7d: z.number().nullish(),
})
export type PostedStats = z.infer<typeof posted>

const insight = z.object({
  id: z.string(),
  channelId: z.string().optional(),
  dimension: z.enum(['template', 'title_pattern', 'length', 'post_hour']),
  text: z.string(),
  groups: z.array(z.object({ label: z.string(), n: z.number(), median7d: z.number() })).nullish().transform((v) => v ?? []),
  preliminary: z.boolean(),
  applied: z.boolean().optional(),
  apply: z.object({ op: z.enum(['add_rule', 'template.rotation']), params: z.record(z.string(), z.unknown()).nullish() }).nullish(),
})
export type Insight = z.infer<typeof insight>

const performance = z.object({
  posted: z.array(posted).nullish().transform((v) => v ?? []),
  insights: z.array(insight).nullish().transform((v) => v ?? []),
  needMore: z.number(),
  counted: z.number().default(0),
  median7d: z.number().nullish(),
  source: z.enum(['data_api', 'public_page']).nullish(),
})
export type Performance = z.infer<typeof performance>

export const getPerformance = async (channelId: string, signal?: AbortSignal) =>
  performance.parse(await request<unknown>(`/api/channels/${encodeURIComponent(channelId)}/performance`, { signal }))

const postedResult = z.object({ youtubeId: z.string().nullish() })
/** Đánh dấu một video đã đăng, kèm link YouTube tuỳ chọn (rỗng = không gửi). */
export const markPostedWithLink = async (jobId: string, youtubeUrl: string) =>
  postedResult.parse(await mutate('POST', `/api/clone/${encodeURIComponent(jobId)}/posted`, youtubeUrl.trim() ? { youtubeUrl: youtubeUrl.trim() } : {}))

const applyResult = z.object({ kind: z.enum(['rule', 'rotation']), ruleId: z.string().nullish() })
export const applyInsight = async (channelId: string, insightId: string) =>
  applyResult.parse(await mutate('POST', `/api/channels/${encodeURIComponent(channelId)}/insights/${encodeURIComponent(insightId)}/apply`, {}))

const dataKey = z.object({ configured: z.boolean(), keyHint: z.string().nullish() })
export type DataKeyState = z.infer<typeof dataKey>
export const getDataKey = async (signal?: AbortSignal) => dataKey.parse(await request<unknown>('/api/youtube/data-key', { signal }))
export const saveDataKey = async (key: string) => dataKey.parse(await mutate('PUT', '/api/youtube/data-key', { key: key.trim() }))
export const removeDataKey = () => mutate<void>('DELETE', '/api/youtube/data-key')

/** Trường `field` trong payload lỗi 400 (vd "youtubeUrl"). */
export function errorField(error: unknown): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const payload = error.payload
  if (!payload || typeof payload !== 'object' || !('field' in payload)) return undefined
  return typeof payload.field === 'string' ? payload.field : undefined
}
