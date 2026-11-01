// Kiểu wire của /api/storage — mục Lưu trữ (V22, hợp đồng §23 + hỏi đáp #10).
// Hình §23 giữ nguyên; `usage`/`clean` nằm gọn trong GET /api/storage (một
// request, server cache và invalidate qua sự kiện SSE `storage`).
import { request } from './request'

export type VolumeKind = 'internal' | 'external' | 'network' | 'cloud_sync'

/** `bus`: cổng nối của ổ ngoài (USB, Thunderbolt…), chỉ có khi máy chủ phân loại được.
 *  `freeBytes`: null khi ổ chứa thư mục dự án không đọc được (ổ ngoài đã rút) —
 *  UI ghi “chưa cắm · chưa đọc được chỗ trống”, không thay bằng số ổ trong máy (#79). */
export type StorageVolume = { name: string; kind: VolumeKind; fs: string; freeBytes: number | null; bus?: string }

export type StorageLocation = {
  path: string
  /** `path` với thư mục nhà rút thành `~` — dùng để hiển thị (artboard in `~/…`). */
  displayPath?: string
  volume: StorageVolume
  /** Assets dự án trong thư mục này (byte). */
  usedBytes: number
  /** false khi ổ bị rút / thư mục biến mất. */
  online: boolean
  /** RFC3339 — lúc ổ biến mất (“rút lúc 21:40”). */
  offlineSince?: string
  /** Thư mục cũ sau lần đổi chỗ gần nhất. `move` → bản sao chờ xoá (có
   *  `deleteAfter` khi server tự xoá); `new_only` → vẫn là nơi đọc dữ liệu cũ
   *  nên KHÔNG xoá được (`deleteAfter` vắng). */
  previous?: { path: string; bytes: number; deleteAfter?: string; mode?: 'move' | 'new_only' }
  /** Lần đổi thư mục gần nhất — phân biệt `done` (move) và `donenew` (new_only)
   *  sau khi tải lại trang. */
  lastSwitch?: { mode: 'move' | 'new_only'; at: string } | null
}

export type StorageSuggestion = { path: string; volume: StorageVolume }

export type StorageUsageGroup = { id: 'sources' | 'exports' | 'editor' | 'images' | 'other' | string; bytes: number }

/** Lần chuyển dở/hỏng — kể cả task bị reconcile sau khi app mở lại. */
export type StorageMoveFailure = {
  taskId: string
  /** Đích của lần chuyển dở (server giữ — client không cần suy từ task). */
  target: string
  /** Câu hiện lý do, vd “Chuyển không xong — dữ liệu ở thư mục cũ vẫn nguyên”. */
  phase: string
  at: string
}

export type StorageMoveState = {
  active: boolean
  taskId: string
  failed: StorageMoveFailure | null
}

export type StorageStatus = {
  projects: StorageLocation
  /** Trạng thái lượt chuyển: `active` ↔ đang chuyển; `failed` ↔ lần gần nhất dở. */
  move: StorageMoveState
  /** Dung lượng luôn ở máy này: cơ sở dữ liệu, mô hình, công cụ. */
  stayBytes: number
  suggestions: StorageSuggestion[]
  /** Breakdown của projects.usedBytes theo nhóm (system = stayBytes, không có trong đây). */
  usage: StorageUsageGroup[]
  clean: { lastRunAt: string | null }
}

export type StorageCheckKind = 'space' | 'write' | 'cloud' | 'fs' | 'same_volume'

export type StorageCheckItem = {
  kind: StorageCheckKind
  status: 'ok' | 'warn' | 'fail'
  text: string
}

export type StorageCheckResult = {
  ok: boolean
  /** true khi chọn gốc ổ — server tạo thư mục con “Viderse Projects”. */
  createSubdir: boolean
  checks: StorageCheckItem[]
  needBytes: number
  etaSec: number | null
}

export type StorageEntry = { name: string; path: string }

export type StorageBrowseResult = {
  /** Đường dẫn canonical đã resolve (`~` được bung). */
  path: string
  /** Nhảy nhanh: thư mục nhà + các ổ đang cắm (luôn có ở mọi response). */
  roots: StorageEntry[]
  dirs: StorageEntry[]
}

export type CleanupPreviewItem = {
  kind: string
  label: string
  count: number
  bytes: number
}

export type CleanupPreview = {
  previewId: string
  items: CleanupPreviewItem[]
  freedBytes: number
  stoppedProcesses: number
}

export type CleanupRunResult = { removed: number; freedBytes: number; stoppedProcesses: number }

export const getStorage = (signal?: AbortSignal) => request<StorageStatus>('/api/storage', { signal })

export const checkStoragePath = (path: string, mode: 'move' | 'new_only') =>
  request<StorageCheckResult>('/api/storage/check', { body: { path, mode } })

/** `done: true` = không có việc nền (new_only, hoặc đã ở đích) — `taskId` rỗng
 *  và không có sự kiện SSE nào để chờ; `done: false` thì theo dõi task qua
 *  `activity`/`storage` với đúng `taskId`. */
export const moveStorage = (path: string, mode: 'move' | 'new_only') =>
  request<{ taskId: string; done: boolean }>('/api/storage/move', { body: { path, mode } })

export const cancelStorageMove = (taskId: string) => request<{ ok: boolean }>(`/api/storage/move/${taskId}/cancel`, { body: {} })

/** Thử lại lần chuyển dở — server tự biết đích từ `move.failed.target`. */
export const retryStorageMove = () => request<{ taskId: string; done: boolean }>('/api/storage/move/retry', { body: {} })

/** Gác bản ghi lần chuyển dở — chỉ xoá ghi nhận, không đụng file. */
export const dismissStorageMove = () => request<{ ok: boolean }>('/api/storage/move/dismiss', { body: {} })

export const deletePreviousStorage = () => request<{ ok: boolean }>('/api/storage/previous/delete', { body: {} })

/** Ổ bị rút: video mới lưu lại vào thư mục trên máy này. */
export type ReclaimKind = 'src' | 'mp4'

export type ReclaimItem = {
  id: ReclaimKind
  title: string
  bytes: number
  count: number
  desc: string
  /** Nhãn nút xoá của server (“Xoá video gốc”…). */
  action?: string
}

/** Các nhóm lớn có thể xoá riêng — item vẫn trả khi count=0 (card hiện mờ). */
export const getStorageReclaim = (signal?: AbortSignal) =>
  request<{ items: ReclaimItem[] }>('/api/storage/reclaim', { signal })

/** Xoá một nhóm (xác nhận cam ở phía client); trả lại chính item sau khi xoá. */
export const reclaimStorage = (id: ReclaimKind) =>
  request<ReclaimItem>(`/api/storage/reclaim/${id}`, { body: {} })
export const useDefaultForNew = () => request<{ ok: boolean }>('/api/storage/use-default-for-new', { body: {} })

/** Thăm dò lại ngay sau khi cắm ổ (nút Thử lại của banner offline). */
export const recheckStorage = () => request<{ online: boolean }>('/api/storage/recheck', { body: {} })

/** Duyệt thư mục phía server (hộp chọn thay cho dialog hệ điều hành — hỏi đáp #10). */
export const browseStorage = (path?: string, signal?: AbortSignal) =>
  request<StorageBrowseResult>(`/api/storage/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`, { signal })

/** Mở thư mục dự án hiện tại trên máy đang chạy Viderse (Finder). */
// `open` không đọc body; vẫn gửi `{}` để gateway thấy content-type JSON.
export const openStorageInFinder = () => request<{ ok: boolean }>('/api/storage/open', { body: {} })

/** Xem trước những gì sẽ dọn — run cần đúng `previewId` của lần preview mới nhất. */
export const previewCleanup = () => request<CleanupPreview>('/api/storage/cleanup/preview', { body: {} })

export const runCleanup = (previewId: string) =>
  request<CleanupRunResult>('/api/storage/cleanup/run', { body: { previewId } })
