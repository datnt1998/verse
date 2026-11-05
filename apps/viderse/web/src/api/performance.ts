// Metadata đánh dấu video đã đăng; không thu thập hiệu quả sau đăng.
import { nullish, record, str } from './guard'
import { ApiError, mutate } from './request'

/** Đánh dấu một video đã đăng, kèm link YouTube tuỳ chọn (rỗng = không gửi). */
export const markPostedWithLink = async (jobId: string, youtubeUrl: string) => {
  const o = record(await mutate<unknown>('POST', `/api/clone/${encodeURIComponent(jobId)}/posted`, youtubeUrl.trim() ? { youtubeUrl: youtubeUrl.trim() } : {}), 'posted')
  return { youtubeId: nullish(o.youtubeId, str, 'posted.youtubeId') }
}


/** Trường `field` trong payload lỗi 400 (vd "youtubeUrl"). */
export function errorField(error: unknown): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const payload = error.payload
  if (!payload || typeof payload !== 'object' || !('field' in payload)) return undefined
  return typeof payload.field === 'string' ? payload.field : undefined
}
