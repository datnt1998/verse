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
  /** GET /api/rivals/:id/videos — trang đầu của lưới theo bộ lọc + chuỗi tìm; trang sau qua cursor. */
  rivalVideos: (id: string, sort: string, long: number, within: number, q: string) =>
    ['rivals', id, 'videos', sort, String(long), String(within), q] as const satisfies Key,
  /** GET /api/rivals/:id/scan — thẻ Quét lịch sử ở cột phải. */
  rivalScan: (id: string) => ['rivals', id, 'scan'] as const satisfies Key,
  /** GET /api/rivals/:id/posting-slots — thẻ Lịch đăng theo múi giờ. */
  rivalPosting: (id: string, zone: string) => ['rivals', id, 'posting', zone] as const satisfies Key,
  /** GET /api/templates — lưới thẻ template của Thư viện. */
  templates: () => ['library', 'templates'] as const satisfies Key,
  /** GET /api/templates/:id — trang cài đặt template (kèm recipe + giá). */
  template: (id: string) => ['library', 'templates', id] as const satisfies Key,
  /** GET /api/image-prompts?kind&q — kho prompt ảnh theo bộ lọc + chuỗi tìm. */
  prompts: (kind: string, q: string) => ['library', 'prompts', kind, q] as const satisfies Key,
  /** GET /api/overlays — thư viện lớp phủ dùng chung. */
  overlays: () => ['library', 'overlays'] as const satisfies Key,
  /** GET /api/owner-channels — tên kênh của bạn (cột trái Thư viện + gắn template vào kênh). */
  ownerChannels: () => ['channels', 'refs'] as const satisfies Key,
  /** GET /api/owner-channels — hình đầy đủ cho cột trái/Bộ đổi kênh và danh sách. */
  channels: () => ['channels', 'list'] as const satisfies Key,
  /** GET /api/owner-channels/:id — một kênh cho trang Kênh (tab, nhận diện, nguồn). */
  channel: (id: string) => ['channels', 'one', id] as const satisfies Key,
  /** GET /api/rivals/:id/posting-slots gộp mọi đối thủ liên kết của kênh theo múi giờ. */
  channelSchedule: (id: string, zone: string) => ['channels', 'schedule', id, zone] as const satisfies Key,
  /** GET /api/backdrop/channels — cột nguồn của tab Video nền + hộp chọn nguồn. */
  backdropChannels: () => ['backdrop', 'channels'] as const satisfies Key,
  /** GET /api/backdrop/channels/:id — trang một kênh nền. */
  backdropChannel: (id: string) => ['backdrop', 'channels', id] as const satisfies Key,
  /** GET /api/backdrop/channels/:id/videos — trang đầu lưới theo bộ lọc kind; trang sau qua cursor tay. */
  backdropChannelVideos: (id: string, kinds: string) => ['backdrop', 'channels', id, 'videos', kinds] as const satisfies Key,
  /** GET /api/backdrop/videos — mọi video lẻ (kèm trạng thái tải). */
  backdropVideos: () => ['backdrop', 'videos'] as const satisfies Key,
  /** GET /api/templates/:id/backdrop/mix — ước lượng phần nền của một video 42 phút. */
  backdropMix: (templateId: string, lengthSec: number) => ['backdrop', 'mix', templateId, String(lengthSec)] as const satisfies Key,
  /** GET /api/queue — hàng chờ §3 (đang chạy + chờ lượt + quy tắc), làm mới
      bởi sự kiện clone.job.* / queue.updated và status. */
  queue: () => ['queue'] as const satisfies Key,
} as const

