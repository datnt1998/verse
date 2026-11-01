// Client khu vực hiệu quả sau đăng — hợp đồng 07 §21, khớp máy chủ:
//   POST   /api/clone/:id/posted {youtubeUrl?}            → {youtubeId: string|null}
//   GET    /api/channels/:id/performance                  → Performance
//   POST   /api/channels/:id/insights/:insightId/apply    → {kind: 'rule'|'rotation', ruleId?}
//   GET    /api/youtube/data-key                          → {configured, keyHint?}
//   PUT    /api/youtube/data-key {key}                    → cùng hình
//   DELETE /api/youtube/data-key                          → 204
// Lỗi: ApiError.payload giữ {error, field?} (400 posted: field = "youtubeUrl").
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: thẻ hiệu quả nằm trong chunk
// của màn đầu (Bàn dựng).
import { bool, list, nullish, num, oneOf, record, str } from './guard'
import { ApiError, mutate, request } from './request'

export type PostStat = { views: number; likes?: number | null; comments?: number | null; readAt: string }

const readStat = (value: unknown, path: string): PostStat => {
  const o = record(value, path)
  return {
    views: num(o.views, `${path}.views`),
    ...(o.likes === undefined ? {} : { likes: nullish(o.likes, num, `${path}.likes`) }),
    ...(o.comments === undefined ? {} : { comments: nullish(o.comments, num, `${path}.comments`) }),
    readAt: str(o.readAt, `${path}.readAt`),
  }
}

export type PostedStats = {
  jobId: string
  youtubeId?: string | null
  postedAt?: string | null
  at24h?: PostStat | null
  at7d?: PostStat | null
  at28d?: PostStat | null
  vsMedian7d?: number | null
}

export type Insight = {
  id: string
  channelId?: string
  dimension: 'template' | 'title_pattern' | 'length' | 'post_hour'
  text: string
  groups: { label: string; n: number; median7d: number }[]
  preliminary: boolean
  applied?: boolean
  apply?: { op: 'add_rule' | 'template.rotation'; params?: Record<string, unknown> | null } | null
}

const readInsight = (value: unknown, path: string): Insight => {
  const o = record(value, path)
  return {
    id: str(o.id, `${path}.id`),
    ...(o.channelId == null ? {} : { channelId: str(o.channelId, `${path}.channelId`) }),
    dimension: oneOf(o.dimension, ['template', 'title_pattern', 'length', 'post_hour'] as const, `${path}.dimension`),
    text: str(o.text, `${path}.text`),
    groups: o.groups == null ? [] : list(o.groups, (entry, at) => {
      const group = record(entry, at)
      return { label: str(group.label, `${at}.label`), n: num(group.n, `${at}.n`), median7d: num(group.median7d, `${at}.median7d`) }
    }, `${path}.groups`),
    preliminary: bool(o.preliminary, `${path}.preliminary`),
    ...(o.applied == null ? {} : { applied: bool(o.applied, `${path}.applied`) }),
    ...(o.apply === undefined ? {} : {
      apply: nullish(o.apply, (entry, at) => {
        const apply = record(entry, at)
        return {
          op: oneOf(apply.op, ['add_rule', 'template.rotation'] as const, `${at}.op`),
          ...(apply.params === undefined ? {} : { params: nullish(apply.params, record, `${at}.params`) }),
        }
      }, `${path}.apply`),
    }),
  }
}

export type Performance = {
  posted: PostedStats[]
  insights: Insight[]
  needMore: number
  counted: number
  median7d?: number | null
  source?: 'data_api' | 'public_page' | null
}

export const readPerformance = (value: unknown): Performance => {
  const o = record(value, 'performance')
  return {
    posted: o.posted == null ? [] : list(o.posted, (entry, at) => {
      const row = record(entry, at)
      return {
        jobId: str(row.jobId, `${at}.jobId`),
        ...(row.youtubeId === undefined ? {} : { youtubeId: nullish(row.youtubeId, str, `${at}.youtubeId`) }),
        ...(row.postedAt === undefined ? {} : { postedAt: nullish(row.postedAt, str, `${at}.postedAt`) }),
        ...(row.at24h === undefined ? {} : { at24h: nullish(row.at24h, readStat, `${at}.at24h`) }),
        ...(row.at7d === undefined ? {} : { at7d: nullish(row.at7d, readStat, `${at}.at7d`) }),
        ...(row.at28d === undefined ? {} : { at28d: nullish(row.at28d, readStat, `${at}.at28d`) }),
        ...(row.vsMedian7d === undefined ? {} : { vsMedian7d: nullish(row.vsMedian7d, num, `${at}.vsMedian7d`) }),
      }
    }, 'performance.posted'),
    insights: o.insights == null ? [] : list(o.insights, readInsight, 'performance.insights'),
    needMore: num(o.needMore, 'performance.needMore'),
    counted: o.counted == null ? 0 : num(o.counted, 'performance.counted'),
    ...(o.median7d === undefined ? {} : { median7d: nullish(o.median7d, num, 'performance.median7d') }),
    ...(o.source === undefined ? {} : { source: nullish(o.source, (entry, at) => oneOf(entry, ['data_api', 'public_page'] as const, at), 'performance.source') }),
  }
}

export const getPerformance = async (channelId: string, signal?: AbortSignal) =>
  readPerformance(await request<unknown>(`/api/channels/${encodeURIComponent(channelId)}/performance`, { signal }))

/** Đánh dấu một video đã đăng, kèm link YouTube tuỳ chọn (rỗng = không gửi). */
export const markPostedWithLink = async (jobId: string, youtubeUrl: string) => {
  const o = record(await mutate<unknown>('POST', `/api/clone/${encodeURIComponent(jobId)}/posted`, youtubeUrl.trim() ? { youtubeUrl: youtubeUrl.trim() } : {}), 'posted')
  return { youtubeId: nullish(o.youtubeId, str, 'posted.youtubeId') }
}

export const applyInsight = async (channelId: string, insightId: string) => {
  const o = record(await mutate<unknown>('POST', `/api/channels/${encodeURIComponent(channelId)}/insights/${encodeURIComponent(insightId)}/apply`, {}), 'apply')
  return {
    kind: oneOf(o.kind, ['rule', 'rotation'] as const, 'apply.kind'),
    ruleId: nullish(o.ruleId, str, 'apply.ruleId'),
  }
}

export type DataKeyState = { configured: boolean; keyHint?: string | null }

const readDataKey = (value: unknown): DataKeyState => {
  const o = record(value, 'dataKey')
  return { configured: bool(o.configured, 'dataKey.configured'), ...(o.keyHint === undefined ? {} : { keyHint: nullish(o.keyHint, str, 'dataKey.keyHint') }) }
}

export const getDataKey = async (signal?: AbortSignal) => readDataKey(await request<unknown>('/api/youtube/data-key', { signal }))
export const saveDataKey = async (key: string) => readDataKey(await mutate<unknown>('PUT', '/api/youtube/data-key', { key: key.trim() }))
export const removeDataKey = () => mutate<void>('DELETE', '/api/youtube/data-key')

/** Trường `field` trong payload lỗi 400 (vd "youtubeUrl"). */
export function errorField(error: unknown): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const payload = error.payload
  if (!payload || typeof payload !== 'object' || !('field' in payload)) return undefined
  return typeof payload.field === 'string' ? payload.field : undefined
}
