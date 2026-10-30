// Kiểu wire của GET/POST /api/setup — màn Chuẩn bị máy (W1).
// Sáu bước cố định theo Onboard · step=setup; `codex-login` là bước duy nhất
// cần người (state needs_user), năm bước còn lại máy tự cài.
import { request } from './request'

export const SETUP_STEP_IDS = ['yt-dlp', 'ffmpeg', 'transcription', 'chrome', 'codex', 'codex-login'] as const
export type SetupStepId = (typeof SETUP_STEP_IDS)[number]

export type SetupStepState = 'done' | 'pending' | 'running' | 'failed' | 'needs_user'

export type SetupStepProgress = {
  phase: string
  /** Câu pha tiếng Việt server dịch sẵn — hiện thẳng, không tự dịch enum. */
  phaseLabel?: string
  detail?: string
  bytesDone: number
  bytesTotal: number | null
  pct: number | null
  startedAt: string
}

export type SetupStep = {
  id: SetupStepId
  state: SetupStepState
  progress: SetupStepProgress | null
  /** Lý do thô của bước lỗi. */
  error: string | null
  /** Dung lượng bước chiếm khi cài xong (0 = không đo được). */
  sizeBytes: number
  /** Công cụ có sẵn trên máy, không phải tải (Chrome hệ thống). */
  provided?: boolean
}

export type SetupStatus = {
  /** `idle` = máy sạch chưa khởi — chỉ trạng thái này được POST start. `login` =
   *  5 bước cài xong, chỉ còn đăng nhập ChatGPT. `failed` = lượt đã chạy xong
   *  có lỗi — không tự khởi lại, phải retry từng bước. */
  state: 'idle' | 'running' | 'login' | 'failed' | 'done'
  complete: boolean
  running: boolean
  steps: SetupStep[]
  disk: { freeBytes: number; requiredBytes: number; enough: boolean; approxBytes: number }
}

/** Đường dẫn là `/api/setup/state`: `/api/setup` thuộc luồng thiết lập tài
 *  khoản của platform/webapp ({needed, attemptsLeft}), không phải máy này.
 *  GET chỉ đọc trạng thái, không có hiệu ứng — lượt cài khởi bằng POST
 *  `/api/setup/start` (SetupScreen tự gọi khi desktop gặp máy `idle`). */
export const getSetup = (opts?: { signal?: AbortSignal }) =>
  request<SetupStatus>('/api/setup/state', { signal: opts?.signal })

/** Thăm dò lại trạng thái công cụ sau khi user sửa tay (ví dụ vừa gỡ lỗi). */
export const checkSetup = () => request<SetupStatus>('/api/setup/check', { body: {} })

/** Khởi lượt cài — chỉ có tác dụng từ `idle`; trạng thái khác trả snapshot
 *  hiện tại (202). Gọi lặp vô hại nhưng chỉ nên gọi khi `state === 'idle'`. */
export const startSetup = () => request<SetupStatus>('/api/setup/start', { body: {} })

/** Thử lại một bước lỗi. */
export const retrySetupStep = (id: SetupStepId) => request<SetupStatus>('/api/setup/retry', { body: { id } })
