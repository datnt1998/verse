import { useEffect, useRef } from 'react'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { getActivity, getNotificationSettings, type AppNotification, type NotificationSettings } from '../api/activity'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'

export function isQuiet(settings: NotificationSettings, now = new Date()) {
  if (!settings.doNotDisturb.enabled) return false
  const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
  const { start, end } = settings.doNotDisturb
  return start <= end ? time >= start && time < end : time >= start || time < end
}
function openNotification(n: AppNotification) {
  if (n.href) window.location.href = n.href
  window.focus()
}
export function ActivityBridge() {
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal))
  const settings = useResource(['activity', 'settings'], ({ signal }) => getNotificationSettings(signal))
  const seen = useRef<Map<string, number> | null>(null)
  useEffect(() => {
    const refresh = () => { invalidate(keys.activity()); invalidate(keys.status()) }
    window.addEventListener('activity:refresh', refresh)
    const timer = window.setInterval(() => { void activity.refresh() }, 3000)
    return () => { window.removeEventListener('activity:refresh', refresh); clearInterval(timer) }
  }, [activity.refresh])
  useEffect(() => {
    if (!activity.data || !settings.data) return
    const notifications = activity.data.unread ?? []
    const previous = seen.current
    seen.current = new Map(notifications.map((n) => [n.id, n.count]))
    if (!previous) return
    for (const n of notifications) {
      if (previous.get(n.id) === n.count) continue
      const config = settings.data.kinds[n.kind]
      if (!config || isQuiet(settings.data)) continue
      const action = { label: n.actionLabel || 'Xem', onClick: () => openNotification(n) }
      if (config.inApp) {
        if (n.kind === 'needs_attention' || n.kind === 'review_ready') toast.card({ key: `activity:${n.id}`, title: n.text, action, tone: n.kind === 'needs_attention' ? 'err' : 'warn' })
        else toast(n.text, 'ok', action, { key: `activity:${n.id}`, duration: 5000 })
      }
      if (!previous.has(n.id) && document.hidden && n.systemAllowed !== false && config.system && ['review_ready', 'needs_attention', 'ready_to_post'].includes(n.kind) && 'Notification' in window && Notification.permission === 'granted') {
        const notification = new Notification('Viderse', { body: n.text, tag: `activity:${n.id}`, icon: '/app-icon.svg' })
        notification.onclick = () => { openNotification(n); notification.close() }
      }
    }
  }, [activity.data, settings.data])
  useEffect(() => {
    if (!activity.data || !settings.data) return
    const badge = navigator as Navigator & { setAppBadge?: (count?: number) => Promise<void>; clearAppBadge?: () => Promise<void> }
    const count = activity.data.badge ?? 0
    if (settings.data.badge && count) void badge.setAppBadge?.(count).catch(() => {})
    else void badge.clearAppBadge?.().catch(() => {})
  }, [activity.data, settings.data])
  return null
}
