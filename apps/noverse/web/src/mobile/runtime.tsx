import { createContext, useCallback, useContext, useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { useSession } from '../api/session'
import { Icon, MobileRuntimeProvider as KitMobileRuntimeProvider, useMobileRuntime as useKitMobileRuntime, useSustained, type MobileRuntime } from '@noverse/ui'
import { cache, useConnection } from '../data'
import { useLibrary } from '../api/library'
import { MobileAuth } from '../screens/auth/mobile'
import { latestDraft, registerAccountSession, rememberAccount, setDraftAccount } from './accounts'
import { drainOutbox, setOutboxAccount } from './outbox'
import { sessionGate } from './session'
import { mobileBackoff } from './logic'
import { useMobileToast } from './primitives'

type WorkerConnection = { connected: boolean; reconnectAt: number | null; retryNow: () => void; blocked: unknown }

/**
 * Noverse bọc provider của kit (bàn phím, chế độ thanh tab, cờ cuộn) và thêm phần riêng:
 * mất kết nối, đếm ngược thử lại, sheet hết phiên, luồng worker. Hook tab-mode/cuộn dùng
 * thẳng bản kit — xem `export { … } from '@noverse/ui'` cuối tệp.
 */
type Runtime = MobileRuntime & { offline: boolean; /** Giây còn lại tới lần thử kết nối kế tiếp. */ seconds: number; retryNow: () => void; setWorker: (connection: WorkerConnection | null) => void }
const WorkerContext = createContext<(connection: WorkerConnection | null) => void>(() => {})
const RuntimeContext = createContext<Runtime>({ keyboard: 0, tabMode: 'default', down: false, offline: false, seconds: 0, retryNow: () => {}, setScroll: () => {}, setWorker: () => {} })
export const useMobileRuntime = () => useContext(RuntimeContext)

export function MobileRuntimeProvider({ children }: { children: ReactNode }) {
  const { user, setUser, refresh } = useSession()
  const library = useLibrary()
  const connection = useConnection()
  const expired = useSyncExternalStore(sessionGate.subscribe, sessionGate.isOpen, sessionGate.isOpen)
  const kit = useKitMobileRuntime()
  const [worker, setWorker] = useState<WorkerConnection | null>(null)
  const toast = useMobileToast()
  const [now, setNow] = useState(Date.now())
  const [networkRetry, setNetworkRetry] = useState({ attempt: 0, at: Date.now() + mobileBackoff(0) })
  useEffect(() => {
    if (!expired) return
    const previous = document.activeElement as HTMLElement | null
    const roots = document.querySelectorAll<HTMLElement>('.m-app')
    roots.forEach((root) => { root.inert = true })
    const focusable = () => document.querySelectorAll<HTMLElement>('.m-login-sheet button:not(:disabled), .m-login-sheet input, .m-login-sheet a[href]')
    focusable()[0]?.focus()
    const trap = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); return }
      if (event.key !== 'Tab') return
      const nodes = focusable()
      if (!nodes.length) { event.preventDefault(); return }
      const first = nodes[0], last = nodes[nodes.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', trap)
    return () => { document.removeEventListener('keydown', trap); roots.forEach((root) => { root.inert = false }); previous?.focus() }
  }, [expired])
  useEffect(() => {
    setDraftAccount(user?.username ?? ''); setOutboxAccount(user?.username ?? '')
    if (user && navigator.onLine) void drainOutbox().catch(() => {})
  }, [user?.username])
  useEffect(() => {
    if (user && connection.online && connection.state === 'open') void drainOutbox().catch(() => {})
  }, [user?.username, connection.online, connection.state])
  useEffect(() => registerAccountSession(async (next) => {
    setUser(next); library.reload(); if (next) await refresh()
  }), [setUser, library.reload, refresh])
  useEffect(() => {
    const online = () => { setNetworkRetry({ attempt: 0, at: Date.now() + mobileBackoff(0) }); void drainOutbox().catch(() => {}) }
    const offline = () => setNetworkRetry({ attempt: 0, at: Date.now() + mobileBackoff(0) })
    window.addEventListener('online', online); window.addEventListener('offline', offline)
    return () => { window.removeEventListener('online', online); window.removeEventListener('offline', offline) }
  }, [])
  // Rớt một nhịp (iOS khoá luồng khi chuyển nền, đổi mạng giữa chừng) không nên
  // làm cả màn tối đi: chỉ báo mất kết nối khi đã hỏng liên tục ≥ 3 s.
  const flap = !connection.online || connection.state === 'retrying' || Boolean(worker && !worker.connected && worker.reconnectAt !== null && !worker.blocked)
  const offline = useSustained(flap, 3000)
  useEffect(() => {
    if (!offline) return
    const timer = setInterval(() => setNow(Date.now()), 250)
    return () => clearInterval(timer)
  }, [offline])
  useEffect(() => {
    if (connection.online || now < networkRetry.at) return
    connection.retry(); worker?.retryNow()
    setNetworkRetry((previous) => ({ attempt: previous.attempt + 1, at: Date.now() + mobileBackoff(previous.attempt + 1) }))
  }, [now, networkRetry.at, connection.online])
  const reconnectAt = !connection.online ? networkRetry.at : worker?.reconnectAt ?? connection.reconnectAt
  const seconds = Math.max(0, Math.ceil(((reconnectAt ?? Date.now() + 1000) - Date.now()) / 1000))
  const retryNow = useCallback(() => { connection.retry(); worker?.retryNow(); void cache.revalidateActive(0) }, [connection.retry, worker?.retryNow])
  const value = useMemo(() => ({ ...kit, offline, seconds, retryNow, setWorker }), [kit, offline, seconds, retryNow])
  return <KitMobileRuntimeProvider hidden={expired}><RuntimeContext.Provider value={value}><WorkerContext.Provider value={setWorker}>
    {children}
    {offline && <div className="m-offline" role="status"><span>Mất kết nối · thử lại sau {seconds} s</span><button onClick={retryNow}>Thử ngay</button></div>}
    {expired && createPortal(<div className="m-overlay m-login-sheet"><div className="m-scrim" /><section className="m-sheet" role="alertdialog" aria-modal="true" aria-labelledby="m-expired-title"><div className="m-drag" aria-hidden="true"><span /></div><div className="m-expired-head"><span className="m-expired-icon"><Icon name="lock" size={21} /></span><span className="m-expired-copy"><span id="m-expired-title">Phiên đăng nhập đã hết</span><span>Sau 30 ngày phải đăng nhập lại</span></span></div>{latestDraft() && <div className="m-login-draft"><span>Góp ý đang gõ dở vẫn được giữ</span><span>“{latestDraft()}”</span></div>}<div className="m-sheet-body"><MobileAuth compact initialUsername={user?.username} onAuthenticated={(next) => { rememberAccount(next); sessionGate.complete(); library.reload(); toast('Đã đăng nhập lại · góp ý vẫn còn trong ô chat'); void cache.revalidateActive(0) }} /></div></section></div>, document.body)}
  </WorkerContext.Provider></RuntimeContext.Provider></KitMobileRuntimeProvider>
}
/** Màn báo kết nối worker của nó để provider tính đồng hồ thử lại. */
export function useStoryConnection(connection: WorkerConnection) {
  const setWorker = useContext(WorkerContext)
  useEffect(() => { setWorker(connection); return () => setWorker(null) }, [connection.connected, connection.reconnectAt, connection.retryNow, connection.blocked, setWorker])
}

// Hook dùng chung nằm ở kit; Noverse re-export để các màn hiện có giữ nguyên đường import.
export { useTabMode, useMobileScroll, useSustained, MobilePaneScope, useMobilePaneActive } from '@noverse/ui'
export type { TabMode } from '@noverse/ui'
