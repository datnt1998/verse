// Kiểu wire của GET /api/activity (hợp đồng dữ liệu §11) — Việc nền & thông báo.
// Thao tác của một việc nền không qua endpoint chung: `objectRef.type` quyết
// endpoint (tool → /api/tools/{id}/install|cancel, setup_step → /api/setup/retry,
// storage_move → /api/storage/move/:taskId/cancel, rival → POST
// /api/rivals/{id}/scan với action = op, riêng retry → resume).
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: tệp này nằm trên đường khởi
// động (ActivityBridge trong khung app) nên không được kéo cả thư viện schema.
import { bool, list, maybe, nullable, num, oneOf, record, str } from './guard'
import { mutate, request } from './request'
import type { SuggestionKind } from './suggestions'

export type TaskKind =
  | 'video' | 'identity' | 'channel_scan' | 'backdrop_scan' | 'thumbnail_preview'
  | 'template_preview' | 'image_trial' | 'editor_export' | 'setup' | 'cleanup' | 'logs' | 'storage_move' | 'storage_migrate' | 'assistant_plan'

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
  createdAt?: string
  phase: string
  progress: TaskProgress
  etaSec: number | null
  state: TaskState
  action: TaskAction
}

export type NotificationKind = 'review_ready' | 'needs_attention' | 'ready_to_post' | 'task_done' | 'action_result'

export type AppNotification = {
  id: string
  kind: NotificationKind
  count: number
  text: string
  actions: { label: string; href: string }[]
  createdAt: string
  readAt: string | null
  objectRef?: ObjectRef
  detail?: { channels?: { channelId: string; name: string; count: number }[]; cause?: string; recovery?: { label: string; href: string }; mp4?: { durationSec?: number; bytes?: number } }
  systemAllowed?: boolean
}

export type Activity = {
  running: BackgroundTask[]
  videoJobs: number
  queued: number
  badge?: number
  nextScan?: { at: string; rivalChannels: number } | null
  unread: AppNotification[]
}

const AGENTS = ['scout', 'director', 'artist', 'qa'] as const
const TASK_STATES = ['running', 'paused', 'waiting', 'failed'] as const
const TASK_OPS = ['cancel', 'resume', 'retry'] as const
const KINDS = ['review_ready', 'needs_attention', 'ready_to_post', 'task_done', 'action_result'] as const

const readObjectRef = (value: unknown, path: string): ObjectRef => {
  const ref = record(value, path)
  return { type: str(ref.type, `${path}.type`), id: str(ref.id, `${path}.id`), label: str(ref.label, `${path}.label`) }
}

const readTask = (value: unknown, path: string): BackgroundTask => {
  const task = record(value, path)
  const progress = task.progress == null ? null : (() => {
    const p = record(task.progress, `${path}.progress`)
    return {
      done: num(p.done, `${path}.progress.done`),
      total: num(p.total, `${path}.progress.total`),
      groups: maybe(p.groups, (groups, at) => list(groups, (entry, sub) => {
        const g = record(entry, sub)
        return { group: str(g.group, `${sub}.group`), done: num(g.done, `${sub}.done`), total: num(g.total, `${sub}.total`) }
      }, at), `${path}.progress.groups`),
    }
  })()
  const action = record(task.action, `${path}.action`)
  return {
    id: str(task.id, `${path}.id`),
    kind: str(task.kind, `${path}.kind`) as TaskKind,
    agent: maybe(task.agent, (value, at) => oneOf(value, AGENTS, at), `${path}.agent`),
    title: str(task.title, `${path}.title`),
    objectRef: readObjectRef(task.objectRef, `${path}.objectRef`),
    createdAt: maybe(task.createdAt, str, `${path}.createdAt`),
    phase: str(task.phase, `${path}.phase`),
    progress,
    etaSec: nullable(task.etaSec, num, `${path}.etaSec`),
    state: oneOf(task.state, TASK_STATES, `${path}.state`),
    action: {
      label: str(action.label, `${path}.action.label`),
      href: maybe(action.href, str, `${path}.action.href`),
      op: maybe(action.op, (value, at) => oneOf(value, TASK_OPS, at), `${path}.action.op`),
    },
  }
}

const readNotification = (value: unknown, path: string): AppNotification => {
  const item = record(value, path)
  const detail = maybe(item.detail, (value, at) => {
    const d = record(value, at)
    return {
      channels: maybe(d.channels, (channels, at2) => list(channels, (entry, sub) => {
        const c = record(entry, sub)
        return { channelId: str(c.channelId, `${sub}.channelId`), name: str(c.name, `${sub}.name`), count: num(c.count, `${sub}.count`) }
      }, at2), `${at}.channels`),
      cause: maybe(d.cause, str, `${at}.cause`),
      recovery: maybe(d.recovery, (value, at2) => {
        const r = record(value, at2)
        return { label: str(r.label, `${at2}.label`), href: str(r.href, `${at2}.href`) }
      }, `${at}.recovery`),
      mp4: maybe(d.mp4, (value, at2) => {
        const m = record(value, at2)
        return {
          durationSec: maybe(m.durationSec, num, `${at2}.durationSec`),
          bytes: maybe(m.bytes, num, `${at2}.bytes`),
        }
      }, `${at}.mp4`),
    }
  }, `${path}.detail`)
  return {
    id: str(item.id, `${path}.id`),
    kind: oneOf(item.kind, KINDS, `${path}.kind`),
    count: num(item.count, `${path}.count`),
    text: str(item.text, `${path}.text`),
    actions: list(item.actions, (entry, sub) => {
      const a = record(entry, sub)
      return { label: str(a.label, `${sub}.label`), href: str(a.href, `${sub}.href`) }
    }, `${path}.actions`),
    createdAt: str(item.createdAt, `${path}.createdAt`),
    readAt: nullable(item.readAt, str, `${path}.readAt`),
    objectRef: maybe(item.objectRef, readObjectRef, `${path}.objectRef`),
    detail,
    systemAllowed: maybe(item.systemAllowed, bool, `${path}.systemAllowed`),
  }
}

export const readActivity = (value: unknown): Activity => {
  const root = record(value, 'activity')
  return {
    running: list(root.running, readTask, 'activity.running'),
    videoJobs: num(root.videoJobs, 'activity.videoJobs'),
    queued: num(root.queued, 'activity.queued'),
    badge: maybe(root.badge, num, 'activity.badge'),
    nextScan: root.nextScan == null ? (root.nextScan === null ? null : undefined) : (() => {
      const scan = record(root.nextScan, 'activity.nextScan')
      return { at: str(scan.at, 'activity.nextScan.at'), rivalChannels: num(scan.rivalChannels, 'activity.nextScan.rivalChannels') }
    })(),
    unread: list(root.unread, readNotification, 'activity.unread'),
  }
}

export type SuggestionSettings = { off: { channelId: string | null; kind: SuggestionKind }[] }

export type NotificationSettings = {
  kinds: Record<NotificationKind, { inApp: boolean; system: boolean }>
  doNotDisturb: { enabled: boolean; start: string; end: string }
  badge: boolean
  merge: boolean
  recapOnReturn: boolean
  suggestions: SuggestionSettings
}

export const readNotificationSettings = (value: unknown): NotificationSettings => {
  const root = record(value, 'settings')
  const kinds = record(root.kinds, 'settings.kinds')
  const dnd = record(root.doNotDisturb, 'settings.doNotDisturb')
  const suggestions = maybe(root.suggestions, (value, at) => {
    const s = record(value, at)
    return {
      off: list(s.off ?? [], (entry, sub) => {
        const o = record(entry, sub)
        return { channelId: nullable(o.channelId, str, `${sub}.channelId`), kind: str(o.kind, `${sub}.kind`) as SuggestionKind }
      }, `${at}.off`),
    }
  }, 'settings.suggestions') ?? { off: [] }
  return {
    kinds: Object.fromEntries(KINDS.map((kind) => {
      const entry = record(kinds[kind], `settings.kinds.${kind}`)
      return [kind, { inApp: bool(entry.inApp, `settings.kinds.${kind}.inApp`), system: bool(entry.system, `settings.kinds.${kind}.system`) }]
    })) as NotificationSettings['kinds'],
    doNotDisturb: {
      enabled: bool(dnd.enabled, 'settings.doNotDisturb.enabled'),
      start: str(dnd.start, 'settings.doNotDisturb.start'),
      end: str(dnd.end, 'settings.doNotDisturb.end'),
    },
    badge: bool(root.badge, 'settings.badge'),
    merge: bool(root.merge, 'settings.merge'),
    recapOnReturn: bool(root.recapOnReturn, 'settings.recapOnReturn'),
    suggestions,
  }
}

export const getActivity = async (signal?: AbortSignal): Promise<Activity> =>
  readActivity(await request('/api/activity', { signal }))

export const getNotificationHistory = async (signal?: AbortSignal, before?: string) => {
  const root = record(await request(`/api/notifications${before ? `?before=${encodeURIComponent(before)}` : ''}`, { signal }), 'notifications')
  return {
    notifications: list(root.notifications, readNotification, 'notifications.notifications'),
    nextCursor: maybe(root.nextCursor, str, 'notifications.nextCursor'),
  }
}

export const markNotificationsRead = (ids: string[] | 'all') =>
  request<void>('/api/notifications/read', { body: ids === 'all' ? { all: true } : { ids } })

export const getNotificationSettings = async (signal?: AbortSignal): Promise<NotificationSettings> =>
  readNotificationSettings(await request('/api/notifications/settings', { signal }))

export const saveNotificationSettings = async (settings: NotificationSettings): Promise<NotificationSettings> =>
  readNotificationSettings(await mutate('PUT', '/api/notifications/settings', settings))
