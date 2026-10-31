// Kiểu wire của GET /api/activity (hợp đồng dữ liệu §11) — Việc nền & thông báo.
// Thao tác của một việc nền không qua endpoint chung: `objectRef.type` quyết
// endpoint (tool → /api/tools/{id}/install|cancel, setup_step → /api/setup/retry,
// storage_move → /api/storage/move/:taskId/cancel, rival → POST
// /api/rivals/{id}/scan với action = op, riêng retry → resume).
import { z } from 'zod'
import { mutate, request } from './request'

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
  systemAllowed?: boolean
}

export type Activity = {
  running: BackgroundTask[]
  videoJobs: number
  queued: number
  badge?: number
  unread: AppNotification[]
}

export const getActivity = async (signal?: AbortSignal): Promise<Activity> =>
  activitySchema.parse(await request('/api/activity', { signal })) as Activity

const kinds = ['review_ready', 'needs_attention', 'ready_to_post', 'task_done', 'action_result'] as const
const kindSettings = z.object({ inApp: z.boolean(), system: z.boolean() })
export const notificationSettingsSchema = z.object({
  kinds: z.object(Object.fromEntries(kinds.map((kind) => [kind, kindSettings])) as Record<typeof kinds[number], typeof kindSettings>),
  doNotDisturb: z.object({ enabled: z.boolean(), start: z.string(), end: z.string() }),
  badge: z.boolean(),
  merge: z.boolean(),
  recapOnReturn: z.boolean(),
})
export type NotificationSettings = z.infer<typeof notificationSettingsSchema>
const notificationSchema = z.object({
  id: z.string(), kind: z.enum(kinds), count: z.number(), text: z.string(), href: z.string(),
  actionLabel: z.string(), createdAt: z.string(), readAt: z.string().nullable(),
  objectRef: z.object({ type: z.string(), id: z.string(), label: z.string() }).optional(),
  systemAllowed: z.boolean().optional(),
})
const activitySchema = z.object({
  running: z.array(z.object({
    id: z.string(), kind: z.string(), agent: z.enum(['scout', 'director', 'artist', 'qa']).optional(),
    title: z.string(), objectRef: z.object({ type: z.string(), id: z.string(), label: z.string() }),
    phase: z.string(), progress: z.object({ done: z.number(), total: z.number(), groups: z.array(z.object({ group: z.string(), done: z.number(), total: z.number() })).optional() }).nullable(),
    etaSec: z.number().nullable(), state: z.enum(['running', 'paused', 'waiting', 'failed']),
    action: z.object({ label: z.string(), href: z.string().optional(), op: z.enum(['cancel', 'resume', 'retry']).optional() }),
  })),
  videoJobs: z.number(), queued: z.number(), badge: z.number().optional(), unread: z.array(notificationSchema),
})
export const getNotificationHistory = async (signal?: AbortSignal) =>
  z.object({ notifications: z.array(notificationSchema) }).parse(await request('/api/notifications', { signal }))
export const markNotificationsRead = (ids: string[] | 'all') =>
  request<void>('/api/notifications/read', { body: ids === 'all' ? { all: true } : { ids } })
export const getNotificationSettings = async (signal?: AbortSignal) =>
  notificationSettingsSchema.parse(await request('/api/notifications/settings', { signal }))
export const saveNotificationSettings = async (settings: NotificationSettings) =>
  notificationSettingsSchema.parse(await mutate('PUT', '/api/notifications/settings', settings))
