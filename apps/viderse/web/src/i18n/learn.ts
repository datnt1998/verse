// Chữ còn sống của Đề xuất (V37): dòng trong nhóm Chờ bạn, ba loại việc.
// Quy tắc đã học, Vì sao?, Bản tin, "Đừng đề xuất kiểu này" và cài đặt theo loại đã bỏ.
import type { Assistant } from '../components/agents'
import { base } from './vi'

const agentNames: Record<Assistant, string> = { scout: 'Trinh sát', director: 'Đạo diễn', artist: 'Hoạ sĩ', qa: 'Kiểm định' }

export const learnCopy = {
  suggest: {
    who: (agent: Assistant) => `${agentNames[agent]} đề xuất`,
    dismiss: 'Bỏ qua',
    region: 'Đề xuất của trợ lý',
    loading: 'Đang tải đề xuất…',
    empty: 'Chưa có đề xuất. Trợ lý sẽ báo ở đây khi thấy việc đáng làm.',
    error: 'Không tải được đề xuất.',
    retry: 'Thử lại',
    offline: 'Mất kết nối · đề xuất cập nhật lại khi có kết nối',
    offlineAction: 'Mất kết nối máy chủ — chưa nhận được đề xuất này.',
    gone: 'Đề xuất này không còn nữa.',
    failed: 'Không thực hiện được đề xuất này.',
    confirm: {
      cost: (images: number) => `Tốn ${images} ảnh.`,
      body: 'Máy sẽ xếp việc này vào hàng chờ ngay sau khi bạn bấm.',
      cleanupTitle: 'Xoá MP4 của video đã đăng?',
      cleanupBody: 'Việc này không hoàn tác được.',
      cancel: 'Huỷ',
    },
  },
}

/** `t` của đề xuất: chữ khung app + chữ mảng đề xuất. */
export const t = { ...base, learn: learnCopy } as const
