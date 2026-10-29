import { useEffect } from 'react'
import { getNotifications, getNotifyPrefs, getPushSubscriptions } from '../../api/client'
import { invalidate, useResource } from '../../data'

export const NOTIFICATIONS_KEY = ['notifications'] as const
export const NOTIFY_PREFS_KEY = ['notify-prefs'] as const
export const PUSH_DEVICES_KEY = ['push-subscriptions'] as const
let inboxReaders = 0
const receive = (event: MessageEvent) => { if (event.data?.type === 'ainovel.notification') void invalidate(NOTIFICATIONS_KEY) }
function retainNotificationMessages() {
  if (!('serviceWorker' in navigator)) return
  if (inboxReaders++ === 0) navigator.serviceWorker.addEventListener('message', receive)
  return () => { if (--inboxReaders === 0) navigator.serviceWorker.removeEventListener('message', receive) }
}

export function useNotificationInbox() {
  useEffect(retainNotificationMessages, [])
  return useResource(NOTIFICATIONS_KEY, ({ signal }) => getNotifications(undefined, signal))
}

export function useNotifyPrefs() {
  return useResource(NOTIFY_PREFS_KEY, ({ signal }) => getNotifyPrefs(signal))
}

export function usePushDevices() {
  return useResource(PUSH_DEVICES_KEY, ({ signal }) => getPushSubscriptions(signal))
}
