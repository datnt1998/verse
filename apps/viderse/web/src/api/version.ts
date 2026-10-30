// Kiểu wire của GET /api/version — thẻ phiên bản trong Cài đặt › Nhật ký.
// Kiểm tra bản mới là hành động tường minh (nút), không gọi mạng ở GET.
import { request } from './request'

export type AppVersion = {
  version: string
  /** null = đang dùng bản mới nhất (hoặc chưa kiểm tra được). */
  latest: string | null
  checkedAt: string | null
}

export const getVersion = (signal?: AbortSignal) => request<AppVersion>('/api/version', { signal })

/** Kiểm tra bản mới ngay (nút “Kiểm tra bản mới”). */
export const checkVersion = () => request<AppVersion>('/api/version/check', { body: {} })
