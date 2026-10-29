import { useEffect, useSyncExternalStore, type ReactNode } from 'react'
import { useSession } from '../api/session'
import { cache } from '@noverse/ui/data'
import { events, startDataEvents } from './events'
import { toast } from '@noverse/ui'
import { setNextRoute } from '../api/storage'
import { navigate } from '../router'
import { accountTransition } from '../mobile/session'

const subscribeOnline = (listener: () => void) => {
  window.addEventListener('online', listener); window.addEventListener('offline', listener)
  return () => { window.removeEventListener('online', listener); window.removeEventListener('offline', listener) }
}
const getOnline = () => navigator.onLine
const getServerOnline = () => true
const retry = () => events.retryNow()
export function useConnection() {
  const status = useSyncExternalStore(events.onStatus, events.status, events.status)
  const online = useSyncExternalStore(subscribeOnline, getOnline, getServerOnline)
  return { ...status, online, writable: online && status.state === 'open', retry }
}

export function DataProvider({ children }: { children: ReactNode }) {
  const { user, session, refresh, setUser } = useSession()
  useEffect(() => {
    if (!user) return
    return startDataEvents(user.username)
  }, [user?.username])
  useEffect(() => events.subscribe(['account.changed', 'session.revoked'], (payload, meta) => {
    if (accountTransition.isActive()) return
    const data = payload as { username?: string; session?: string } | null
    if (!data || data.username !== user?.username) return
    if (meta.type === 'session.revoked' && data.session && data.session === session?.id) {
      if (!location.hash.startsWith('#/login')) setNextRoute(location.hash || '#/')
      events.stop()
      setUser(null)
      toast.err('Phiên đăng nhập đã bị thu hồi. Vui lòng đăng nhập lại.')
      navigate('/login', true)
      return
    }
    if (meta.type === 'account.changed') void refresh()
  }), [user?.username, session?.id])
  useEffect(() => {
    const focus = () => {
      if (document.visibilityState !== 'visible') return
      // iOS để luồng SSE chết trong nền mà đối tượng vẫn "open"; quay lại thì
      // nối ngay, đừng chờ backoff (tối đa 5 s thấy như mất dữ liệu).
      if (events.status().state !== 'open') events.retryNow()
      else void cache.revalidateActive(15000)
    }
    // hello nối lại làm mới cache; không tạo thêm một đợt fetch từ sự kiện online.
    const online = () => { events.retryNow() }
    window.addEventListener('focus', focus)
    window.addEventListener('online', online)
    document.addEventListener('visibilitychange', focus)
    return () => { window.removeEventListener('focus', focus); window.removeEventListener('online', online); document.removeEventListener('visibilitychange', focus) }
  }, [])
  return children
}
