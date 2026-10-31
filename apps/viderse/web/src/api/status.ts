// Kiểu wire của GET /api/status (data-contract §5) — bộ đếm cho đầu trang.
import { request } from './request'

export type AttentionKind = 'tool_update' | 'login_expired' | 'quota_low' | 'tool_failed' | 'storage_down'

export type Attention = { kind: AttentionKind; text: string; href: string }

export type Status = {
  running: number
  queued: number
  paused: boolean
  reviewCount: number
  attention: Attention[]
  storageDown?: { label: string; since: string }
}

export const status = (signal?: AbortSignal) => request<Status>('/api/status', { signal })
