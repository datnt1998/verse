// Video vừa mở từ danh sách Khám phá trên điện thoại. Route chỉ mang id; màn
// chi tiết đọc lại video đã có trong bộ nhớ để không phải tải lại cả trang đầu
// theo bộ lọc cũ. Liên kết trực tiếp không có bản nhớ thì màn chi tiết tự tìm
// trong trang đầu mặc định (xem MVideoScreen).
import type { Video } from '../../../api/rivals'

let remembered: Video | null = null

export function rememberVideo(video: Video) {
  remembered = video
}

export function recallVideo(id: string): Video | null {
  return remembered?.id === id ? remembered : null
}
