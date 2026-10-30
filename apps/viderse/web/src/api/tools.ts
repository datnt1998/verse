// Kiểu wire của /api/tools — mục Công cụ trên máy (V7).
// Server trả một view-model cho mỗi việc làm; tên việc là copy phía web (i18n),
// phiên bản và đường dẫn chỉ nằm trong `version`/`detail` (vùng “Chi tiết”).
import { request } from './request'
import type { SetupStepId } from './setup'

export type ToolId = SetupStepId

export type ToolState = 'ready' | 'update' | 'missing' | 'failed' | 'installing' | 'needs_login'

export type ToolProgress = {
  phase: string
  /** Câu pha tiếng Việt server dịch sẵn — hiện thẳng, không tự dịch enum. */
  phaseLabel?: string
  detail?: string
  bytesDone: number
  bytesTotal: number | null
  pct: number | null
  /** Lúc bắt đầu tải (ISO) — để tính ETA còn lại. */
  startedAt: string | null
}

export type ToolRow = {
  id: ToolId
  state: ToolState
  /** Câu mô tả dòng hai (đã dịch ở server), kể cả câu cập nhật/lỗi. */
  hint: string
  /** Phiên bản đang dùng — chỉ hiện trong “Chi tiết”. */
  version: string | null
  update: { current: string; latest: string } | null
  progress: ToolProgress | null
  /** Đường dẫn binary / lỗi thô — chỉ hiện trong “Chi tiết”. */
  detail: string | null
  retryAt: string | null
  /** Dung lượng bước chiếm khi cài xong (0 = không đo được). */
  sizeBytes: number
  /** Công cụ có sẵn trên máy, không phải tải (Chrome hệ thống). */
  provided?: boolean
}

export type ToolsStatus = {
  tools: ToolRow[]
  checkedAt: string
  /** “Thư mục dữ liệu” của vùng Nâng cao (thiếu = không rõ). */
  dataDir?: string
}

export const getTools = (signal?: AbortSignal) => request<ToolsStatus>('/api/tools', { signal })

/** Thăm dò lại mọi công cụ (nút “Kiểm tra lại”). */
export const checkTools = () => request<ToolsStatus>('/api/tools/check', { body: {} })

/** Cài / cập nhật / sửa một công cụ — chạy nền, theo dõi qua SSE + GET. */
export const installTool = (id: ToolId) => request<{ started: boolean; taskId: string }>(`/api/tools/${id}/install`, { body: {} })
