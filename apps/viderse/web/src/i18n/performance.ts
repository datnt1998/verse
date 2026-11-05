// Metadata đánh dấu đã đăng; không còn thu thập số liệu sau đăng.
import { base } from './vi'

export const performanceCopy = {
  mark: {
    title: 'Đánh dấu đã đăng?',
    body: 'Video chuyển sang Đã đăng. Bạn có thể lưu link YouTube để mở lại video đã đăng; bỏ trống nếu chưa có link.',
    label: 'Link YouTube (không bắt buộc)',
    placeholder: 'https://www.youtube.com/watch?v=…',
    ok: 'Đánh dấu đã đăng',
    cancel: 'Huỷ',
    pending: 'Đang lưu…',
    done: 'Đã đánh dấu đã đăng',
    badLink: 'Link này không phải video YouTube — dán link dạng youtube.com/watch?v=… hoặc youtu.be/…',
    failed: 'Không đánh dấu được — thử lại.',
    offline: 'Mất kết nối máy chủ — chưa đánh dấu được.',
  },
} as const

/** Chữ khung app + thao tác đánh dấu đã đăng. */
export const t = { ...base, performance: performanceCopy } as const
