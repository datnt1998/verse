// Danh sách video của một kênh (Kênh › Video): GET /api/clone?channel=<id> và
// thao tác hàng loạt POST /api/clone/bulk. Hợp đồng ở 07-data-contract §7:
// `bulkEligibility` do server tính, mỗi mục là danh sách jobId hợp lệ.
import { z } from 'zod'
import { cloneJobViewSchema } from './workbench'
import { mutate, request } from './request'

const ids = z.array(z.string()).nullable().transform((value) => value ?? [])

export const bulkEligibilitySchema = z.object({
  retry: ids,
  approve: ids,
  bundle: ids,
  markPosted: ids,
  cancel: ids,
  delete: ids,
})
export type BulkEligibility = z.infer<typeof bulkEligibilitySchema>

/** Job của danh sách kênh: view của Bàn dựng cộng các mốc giờ của server (`CloneJob`). */
const channelJobSchema = cloneJobViewSchema.extend({
  createdAt: z.string(),
  updatedAt: z.string(),
  postedAt: z.string().nullish(),
})
export type ChannelJob = z.infer<typeof channelJobSchema>

const listSchema = z.object({
  jobs: z.array(channelJobSchema).nullable().transform((value) => value ?? []),
  queuedTotal: z.number(),
  bulkEligibility: bulkEligibilitySchema,
}).passthrough()
export type ChannelJobs = z.infer<typeof listSchema>

export const getChannelJobs = async (channelId: string, signal?: AbortSignal) =>
  listSchema.parse(await request<unknown>(`/api/clone?channel=${encodeURIComponent(channelId)}`, { signal }))

export type BulkAction = 'retry' | 'cancel' | 'approve' | 'delete' | 'posted'

const bulkSchema = z.object({ results: z.array(z.object({ jobId: z.string(), ok: z.boolean(), error: z.string().optional() })) })
export type BulkRow = z.infer<typeof bulkSchema>['results'][number]

/** Một lần gọi tối đa 100 video (server từ chối quá số đó); lỗi từng video nằm trong `results`. */
export const BULK_MAX = 100
export const runBulk = async (action: BulkAction, jobIds: readonly string[]): Promise<BulkRow[]> =>
  bulkSchema.parse(await mutate('POST', '/api/clone/bulk', { action, jobIds })).results

/** Server gói tối đa 50 video mỗi tệp ZIP. */
export const BUNDLE_MAX = 50
export const bundleUrl = (jobIds: readonly string[]) =>
  `/api/clone/download?ids=${jobIds.map(encodeURIComponent).join(',')}`
