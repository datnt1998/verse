// Kiểu wire của GET /api/activity (hợp đồng dữ liệu §11) — Việc nền & thông báo.
// Thao tác của một việc nền không qua endpoint chung: `objectRef.type` quyết
// endpoint (tool → /api/tools/{id}/install|cancel, setup_step → /api/setup/retry,
// storage_move → /api/storage/move/:taskId/cancel, rival → POST
// /api/rivals/{id}/scan với action = op, riêng retry → resume).
import { request } from './request'

export type TaskKind =
  | 'video' | 'identity' | 'channel_scan' | 'backdrop_scan' | 'thumbnail_preview'
  | 'template_preview' | 'editor_export' | 'setup' | 'cleanup' | 'logs' | 'storage_move' | 'storage_migrate'

export type TaskState = 'running' | 'paused' | 'waiting' | 'failed'

export type TaskAction = { label: string; href?: string; op?: 'cancel' | 'resume' | 'retry' }

export type TaskGroupProgress = { group: 'images' | 'editor' | 'sources' | 'exports' | string; done: number; total: number }

export type TaskProgress = { done: number; total: number; groups?: TaskGroupProgress[] } | null

export type ObjectRef = { type: string; id: string; label: string }

export type BackgroundTask = {
  id: string
  kind: TaskKind
  agent?: 'scout' | 'director' | 'artist' | 'qa'
  title: string
  objectRef: ObjectRef
  phase: string
  progress: TaskProgress
  etaSec: number | null
  state: TaskState
  action: TaskAction
}

export type AppNotification = {
  id: string
  kind: 'review_ready' | 'needs_attention' | 'ready_to_post' | 'task_done' | 'action_result'
  count: number
  text: string
  href: string
  actionLabel: string
  createdAt: string
  readAt: string | null
  objectRef?: ObjectRef
}

export type Activity = {
  running: BackgroundTask[]
  videoJobs: number
  queued: number
  unread: AppNotification[]
}

export const getActivity = (signal?: AbortSignal) => request<Activity>('/api/activity', { signal })
