// Khoá cache của Viderse. Khoá là mảng; `invalidate(prefix)` khớp mọi khoá bắt
// đầu bằng prefix (xem @noverse/ui/data), nên phần chung đứng trước phần chi tiết.
import type { Key } from '@noverse/ui/data'

export const keys = {
  /** GET /api/status — bộ đếm đầu trang (JobsPill, badge Duyệt, chấm Cài đặt). */
  status: () => ['status'] as const satisfies Key,
  /** GET /api/tools — hàng Công cụ trên máy (mỗi việc một trạng thái). */
  tools: () => ['tools'] as const satisfies Key,
  /** GET /api/setup — trạng thái Chuẩn bị máy (6 bước). */
  setup: () => ['setup'] as const satisfies Key,
  /** GET /api/storage — thư mục dự án + usage + lần dọn cuối (một request). */
  storage: () => ['storage'] as const satisfies Key,
  /** GET /api/storage/browse?path= — hộp chọn thư mục phía server. */
  storageBrowse: (path: string) => ['storage-browse', path] as const satisfies Key,
  /** GET /api/activity — việc nền & thông báo (§11). */
  activity: () => ['activity'] as const satisfies Key,
  /** GET /api/version — bản đang chạy và bản mới nhất (thẻ trong Nhật ký). */
  appVersion: () => ['version'] as const satisfies Key,
  /** GET /api/errors — lỗi gần đây 7 ngày (thẻ trong Nhật ký). */
  errors: () => ['errors'] as const satisfies Key,
  /** GET /api/ai/providers — danh sách nhà cung cấp (Kết nối AI). */
  aiProviders: () => ['ai', 'providers'] as const satisfies Key,
  /** GET /api/ai/providers/:id/models — danh mục model của một nhà cung cấp. */
  aiProviderModels: (id: string, q?: string, filter?: string, cursor?: string | null) =>
    ['ai', 'providers', id, 'models', filter ?? '', q ?? '', cursor ?? ''] as const satisfies Key,
  /** Toàn bộ model đã chọn của một provider (filter=selected qua mọi trang). */
  aiSelectedModels: (id: string) => ['ai', 'providers', id, 'selected'] as const satisfies Key,
  /** GET /api/ai/agents — model của bốn trợ lý (Trợ lý AI). */
  aiAgents: () => ['ai', 'agents'] as const satisfies Key,
  /** GET /api/platforms — thẻ từng nền tảng (Tài khoản nền tảng). */
  platforms: () => ['platforms'] as const satisfies Key,
  /** GET /api/platforms/youtube/login/profiles — hồ sơ trình duyệt của YouTube. */
  platformProfiles: () => ['platforms', 'profiles'] as const satisfies Key,
  /** GET /api/rivals — danh sách đối thủ + tổng video/video mới (cột trái Khám phá). */
  rivals: () => ['rivals'] as const satisfies Key,
  /** GET /api/rivals/:id/videos — trang đầu của lưới theo bộ lọc; trang sau qua cursor. */
  rivalVideos: (id: string, sort: string, long: number, within: number) =>
    ['rivals', id, 'videos', sort, String(long), String(within)] as const satisfies Key,
  /** GET /api/rivals/:id/scan — thẻ Quét lịch sử ở cột phải. */
  rivalScan: (id: string) => ['rivals', id, 'scan'] as const satisfies Key,
  /** GET /api/rivals/:id/posting-slots — thẻ Lịch đăng theo múi giờ. */
  rivalPosting: (id: string, zone: string) => ['rivals', id, 'posting', zone] as const satisfies Key,
} as const
