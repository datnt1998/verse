// Trần tải lên đang có hiệu lực, lấy từ GET /api/upload-limits.
//
// Khi app phục vụ qua internet (PUBLIC_URL), Cloudflare chặn body > 100 MB
// bằng trang 413 HTML của nó — trước khi app kịp trả lời. Vì vậy máy chủ hạ
// trần xuống 90 MiB và SPA phải tự kiểm kích thước trước khi gửi: người dùng
// đọc được câu "Tải lên qua internet tối đa 90 MB" thay vì một trang lỗi lạ.
//
// Con số lấy từ máy chủ, không hard-code: trần đổi theo PUBLIC_URL và theo
// biến môi trường YTF_EDITOR_MAX_* của từng máy.
import { request } from './request'

export type UploadKind = 'video' | 'audio' | 'image' | 'overlay' | 'template'

export type UploadLimits = {
  /** Máy chủ phục vụ người khác qua PUBLIC_URL (có edge ở trước). */
  hosted: boolean
  /** Trần theo loại tệp, byte. */
  limits: Record<UploadKind, number>
  /** Câu từ chối dùng chung khi vượt trần công khai. */
  message: string
}

let pending: Promise<UploadLimits> | null = null

/** Trần tải lên; gọi một lần cho mỗi phiên trang (cache trong module). */
export function fetchUploadLimits(): Promise<UploadLimits> {
  pending ??= request<UploadLimits>('/api/upload-limits').catch((error: unknown) => {
    pending = null
    throw error
  })
  return pending
}

/** Chỉ dùng trong test: xoá cache để mỗi ca thử đọc lại từ máy chủ giả. */
export function resetUploadLimitsCache(): void {
  pending = null
}

/**
 * Câu từ chối khi `size` vượt trần của `kind`, hoặc null khi tệp nằm trong
 * trần. Chỉ áp dụng khi máy chủ chạy công khai: trên máy, trần thật là 20 GiB
 * và người dùng tự biết tệp của mình.
 */
export async function uploadTooLarge(kind: UploadKind, size: number): Promise<string | null> {
  const limits = await fetchUploadLimits().catch(() => null)
  if (!limits?.hosted) return null
  const max = limits.limits[kind]
  if (!max || size <= max) return null
  return limits.message
}

/** Loại trần áp cho một MIME của editor. */
export function uploadKindForMime(mime: string): UploadKind {
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  return 'image'
}
