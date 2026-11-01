// Danh sách video của một kênh (Kênh › Video): GET /api/clone?channel=<id> và
// thao tác hàng loạt POST /api/clone/bulk. Hợp đồng ở 07-data-contract §7:
// `bulkEligibility` do server tính, mỗi mục là danh sách jobId hợp lệ.
import { bool, list, num, record, str } from './guard'
import { mutate, request } from './request'
import { readCloneJob, type CloneJobView } from './workbench'

export type BulkEligibility = {
  retry: string[]
  approve: string[]
  bundle: string[]
  markPosted: string[]
  cancel: string[]
  delete: string[]
}

const ids = (value: unknown, path: string) => (value == null ? [] : list(value, str, path))

const readEligibility = (value: unknown, path: string): BulkEligibility => {
  const o = record(value, path)
  return {
    retry: ids(o.retry, `${path}.retry`),
    approve: ids(o.approve, `${path}.approve`),
    bundle: ids(o.bundle, `${path}.bundle`),
    markPosted: ids(o.markPosted, `${path}.markPosted`),
    cancel: ids(o.cancel, `${path}.cancel`),
    delete: ids(o.delete, `${path}.delete`),
  }
}

/** Job của danh sách kênh: view của Bàn dựng cộng các mốc giờ của server (`CloneJob`). */
export type ChannelJob = CloneJobView & { createdAt: string; updatedAt: string; postedAt?: string | null }

const readChannelJob = (value: unknown, path: string): ChannelJob => {
  const o = record(value, path)
  return {
    ...readCloneJob(value, path),
    createdAt: str(o.createdAt, `${path}.createdAt`),
    updatedAt: str(o.updatedAt, `${path}.updatedAt`),
    ...(o.postedAt == null ? {} : { postedAt: str(o.postedAt, `${path}.postedAt`) }),
  }
}

export type ChannelJobs = { jobs: ChannelJob[]; queuedTotal: number; bulkEligibility: BulkEligibility }

const readChannelJobs = (value: unknown): ChannelJobs => {
  const o = record(value, 'channelJobs')
  return {
    jobs: o.jobs == null ? [] : list(o.jobs, readChannelJob, 'channelJobs.jobs'),
    queuedTotal: num(o.queuedTotal, 'channelJobs.queuedTotal'),
    bulkEligibility: readEligibility(o.bulkEligibility, 'channelJobs.bulkEligibility'),
  }
}

export const getChannelJobs = async (channelId: string, signal?: AbortSignal) =>
  readChannelJobs(await request<unknown>(`/api/clone?channel=${encodeURIComponent(channelId)}`, { signal }))

export type BulkAction = 'retry' | 'cancel' | 'approve' | 'delete' | 'posted'

export type BulkRow = { jobId: string; ok: boolean; error?: string }

const readBulkRows = (value: unknown): BulkRow[] => {
  const o = record(value, 'bulk')
  return list(o.results, (entry, at) => {
    const row = record(entry, at)
    return {
      jobId: str(row.jobId, `${at}.jobId`),
      ok: bool(row.ok, `${at}.ok`),
      ...(row.error == null ? {} : { error: str(row.error, `${at}.error`) }),
    }
  }, 'bulk.results')
}

/** Một lần gọi tối đa 100 video (server từ chối quá số đó); lỗi từng video nằm trong `results`. */
export const BULK_MAX = 100
export const runBulk = async (action: BulkAction, jobIds: readonly string[]): Promise<BulkRow[]> =>
  readBulkRows(await mutate('POST', '/api/clone/bulk', { action, jobIds }))

/** Server gói tối đa 50 video mỗi tệp ZIP. */
export const BUNDLE_MAX = 50
export const bundleUrl = (jobIds: readonly string[]) =>
  `/api/clone/download?ids=${jobIds.map(encodeURIComponent).join(',')}`
