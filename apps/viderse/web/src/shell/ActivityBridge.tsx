import { useEffect, useRef } from 'react'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { getActivity, getNotificationSettings, type AppNotification, type NotificationSettings } from '../api/activity'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import { notificationSub, notificationTitle } from './activityText'

export function isQuiet(settings: NotificationSettings, now = new Date()) {
  if (!settings.doNotDisturb.enabled) return false
  const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
  const { start, end } = settings.doNotDisturb
  return start <= end ? time >= start && time < end : time >= start || time < end
}
function openNotification(href?: string) {
  if (href) window.location.href = href
  window.focus()
}
async function showSystemNotification(n: AppNotification) {
  const href = n.actions.at(-1)?.href
  const options = { body: n.text, tag: `activity:${n.id}`, icon: '/icons/icon-192.png', data: { href } }
  if ('serviceWorker' in navigator) {
    try {
      const registration = await navigator.serviceWorker.ready
      await registration.showNotification('Viderse', options)
      return
    } catch { /* Fall back to the window notification API. */ }
  }
  const notification = new Notification('Viderse', options)
  notification.onclick = () => { openNotification(href); notification.close() }
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
      const first = n.actions[0]
      const second = n.actions[1]
      const action = first ? { label: first.label, onClick: () => openNotification(first.href) } : undefined
      const secondaryAction = second ? { label: second.label, onClick: () => openNotification(second.href) } : undefined
      if (config.inApp) {
        if (action) {
          // Copy Toast r35: thẻ việc nền — tiêu đề là việc, tên video ở câu phụ;
          // Cần xử lý ở lại tới khi đóng, thẻ khác 6 giây. Viên = kết quả thao tác.
          toast.card({ key: `activity:${n.id}`, title: notificationTitle(n), text: notificationSub(n) || undefined, action, secondaryAction, tone: n.kind === 'needs_attention' ? 'err' : n.kind === 'review_ready' ? 'warn' : 'ok', duration: n.kind === 'needs_attention' ? undefined : 6000 })
        } else toast(n.text, n.kind === 'needs_attention' ? 'err' : n.kind === 'review_ready' ? 'warn' : 'ok', undefined, { key: `activity:${n.id}`, duration: 6000 })
      }
      if (!previous.has(n.id) && document.hidden && n.systemAllowed !== false && config.system && ['review_ready', 'needs_attention', 'ready_to_post'].includes(n.kind) && 'Notification' in window && Notification.permission === 'granted') {
        void showSystemNotification(n)
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
