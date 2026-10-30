// Kiểu wire của GET /api/errors — thẻ “Lỗi gần đây” (7 ngày) trong Cài đặt ›
// Nhật ký: mỗi lỗi kèm trạng thái app đã tự xử lý (thử lại, đã chạy lại…).
// Chuỗi đã được server định dạng sẵn (time “23:12”|“Hôm qua”|“25/10”, vi).
import { request } from './request'

export type RecentError = {
  /** Định danh dòng lỗi phía server. */
  id: string
  time: string
  /** Tên đối tượng bị lỗi (video, kênh…). */
  what: string
  /** Câu lỗi đã hiểu được, không kèm mã kỹ thuật. */
  why: string
  /** Trạng thái xử lý (“Đã tự chạy lại lúc 22:10”…). */
  status: string
  /** 'warn' = còn chờ/chưa xong (chấm cam); 'ok'|'muted' = đã xử lý. */
  tone: 'ok' | 'warn' | 'muted'
  href?: string
  /** Nhãn link mở đối tượng (“Mở video”…). */
  link?: string
}

export type ErrorsFeed = { items: RecentError[]; count7d: number }

export const getErrors = (signal?: AbortSignal) => request<ErrorsFeed>('/api/errors', { signal })
