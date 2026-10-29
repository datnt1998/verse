// Khoá cache của Viderse. Khoá là mảng; `invalidate(prefix)` khớp mọi khoá bắt
// đầu bằng prefix (xem @noverse/ui/data), nên phần chung đứng trước phần chi tiết.
import type { Key } from '@noverse/ui/data'

export const keys = {
  /** GET /api/status — bộ đếm đầu trang (JobsPill, badge Duyệt, chấm Cài đặt). */
  status: () => ['status'] as const satisfies Key,
} as const
