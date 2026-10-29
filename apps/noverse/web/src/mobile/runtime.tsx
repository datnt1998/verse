import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { useSession } from '../api/session'
import { Icon } from '@noverse/ui'
import { cache, useConnection } from '../data'
import { useLibrary } from '../api/library'
import { MobileAuth } from '../screens/auth/mobile'
import { latestDraft, registerAccountSession, rememberAccount, setDraftAccount } from './accounts'
import { drainOutbox, setOutboxAccount } from './outbox'
import { sessionGate } from './session'
import { mobileBackoff, tabModeFor, type TabMode } from './logic'
import { useMobileToast } from './primitives'

/** Pane giữ-alive: `false` khi màn đang mounted nhưng ẩn (display:none) — hook không được đẩy trạng thái lên runtime. */
const PaneActive = createContext(true)
type WorkerConnection = { connected: boolean; reconnectAt: number | null; retryNow: () => void; blocked: unknown }

type Runtime = { keyboard: number; tabMode: TabMode; down: boolean; offline: boolean; /** Giây còn lại tới lần thử kết nối kế tiếp. */ seconds: number; retryNow: () => void; setScroll: (down: boolean, immersive: boolean) => void; setMode: (id: symbol, mode: TabMode | null) => void; setWorker: (connection: WorkerConnection | null) => void }
const RuntimeContext = createContext<Runtime>({ keyboard: 0, tabMode: 'default', down: false, offline: false, seconds: 0, retryNow: () => {}, setScroll: () => {}, setMode: () => {}, setWorker: () => {} })
type RuntimeActions = Pick<Runtime, 'setScroll' | 'setMode' | 'setWorker'>
const RuntimeActionsContext = createContext<RuntimeActions>({ setScroll: () => {}, setMode: () => {}, setWorker: () => {} })
export const useMobileRuntime = () => useContext(RuntimeContext)

/**
 * Nhận cờ "đang hỏng" nhưng chỉ bật sau `ms` liên tục — dùng để không nháy
 * băng mất kết nối/toàn màn tối cho những lần rớt ngắn (iOS nền → hiện lại).
 */
export function useSustained(value: boolean, ms: number): boolean {
  const [sustained, setSustained] = useState(false)
  useEffect(() => {
    if (!value) { setSustained(false); return }
    const timer = setTimeout(() => setSustained(true), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return sustained
}

export function MobileRuntimeProvider({ children }: { children: ReactNode }) {
  const { user, setUser, refresh } = useSession()
  const library = useLibrary()
  const connection = useConnection()
  const expired = useSyncExternalStore(sessionGate.subscribe, sessionGate.isOpen, sessionGate.isOpen)
  const [keyboard, setKeyboard] = useState(0)
  const [scroll, updateScroll] = useState({ down: false, immersive: false })
  const [modes, setModes] = useState(new Map<symbol, TabMode>())
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
    const viewport = window.visualViewport
    const measure = () => {
      const delta = viewport ? window.innerHeight - viewport.height - viewport.offsetTop : 0
      const px = delta > 80 ? Math.round(delta) : 0
      document.documentElement.style.setProperty('--kb', `${px}px`)
      setKeyboard(px)
    }
    measure(); viewport?.addEventListener('resize', measure); viewport?.addEventListener('scroll', measure)
    return () => { viewport?.removeEventListener('resize', measure); viewport?.removeEventListener('scroll', measure); document.documentElement.style.removeProperty('--kb') }
  }, [])
  useEffect(() => {
    const online = () => { setNetworkRetry({ attempt: 0, at: Date.now() + mobileBackoff(0) }); void drainOutbox().catch(() => {}) }
    const offline = () => setNetworkRetry({ attempt: 0, at: Date.now() + mobileBackoff(0) })
    window.addEventListener('online', online); window.addEventListener('offline', offline)
    return () => { window.removeEventListener('online', online); window.removeEventListener('offline', offline) }
  }, [])
  // Rớt một nhịp (iOS khoá luồng khi chuyển nền, Wi-Fi chuyển cell) không nên
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
  const setMode = useCallback((id: symbol, mode: TabMode | null) => setModes((previous) => {
    if (mode === null ? !previous.has(id) : previous.get(id) === mode) return previous
    const next = new Map(previous)
    if (mode === null) next.delete(id); else next.set(id, mode)
    return next
  }), [])
  const setScroll = useCallback((down: boolean, immersive: boolean) => updateScroll((previous) => previous.down === down && previous.immersive === immersive ? previous : { down, immersive }), [])
  const hidden = expired || [...modes.values()].includes('hide')
  const tabMode = tabModeFor({ keyboard, hidden, immersive: scroll.immersive, down: scroll.down || [...modes.values()].includes('mini') })
  const reconnectAt = !connection.online ? networkRetry.at : worker?.reconnectAt ?? connection.reconnectAt
  const seconds = Math.max(0, Math.ceil(((reconnectAt ?? Date.now() + 1000) - Date.now()) / 1000))
  const retryNow = useCallback(() => { connection.retry(); worker?.retryNow(); void cache.revalidateActive(0) }, [connection.retry, worker?.retryNow])
  const actions = useMemo(() => ({ setScroll, setMode, setWorker }), [setScroll, setMode])
  const value = useMemo(() => ({ keyboard, tabMode, down: scroll.down, offline, seconds, retryNow, ...actions }), [keyboard, tabMode, scroll.down, offline, seconds, retryNow, actions])
  return <RuntimeActionsContext.Provider value={actions}><RuntimeContext.Provider value={value}>
    {children}
    {offline && <div className="m-offline" role="status"><span>Mất kết nối · thử lại sau {seconds} s</span><button onClick={retryNow}>Thử ngay</button></div>}
    {expired && createPortal(<div className="m-overlay m-login-sheet"><div className="m-scrim" /><section className="m-sheet" role="alertdialog" aria-modal="true" aria-labelledby="m-expired-title"><div className="m-drag" aria-hidden="true"><span /></div><div className="m-expired-head"><span className="m-expired-icon"><Icon name="lock" size={21} /></span><span className="m-expired-copy"><span id="m-expired-title">Phiên đăng nhập đã hết</span><span>Sau 30 ngày phải đăng nhập lại</span></span></div>{latestDraft() && <div className="m-login-draft"><span>Góp ý đang gõ dở vẫn được giữ</span><span>“{latestDraft()}”</span></div>}<div className="m-sheet-body"><MobileAuth compact initialUsername={user?.username} onAuthenticated={(next) => { rememberAccount(next); sessionGate.complete(); library.reload(); toast('Đã đăng nhập lại · góp ý vẫn còn trong ô chat'); void cache.revalidateActive(0) }} /></div></section></div>, document.body)}
  </RuntimeContext.Provider></RuntimeActionsContext.Provider>
}
export function useTabMode(mode?: TabMode) {
  const { setMode } = useContext(RuntimeActionsContext)
  const paneActive = useContext(PaneActive)
  const id = useRef(Symbol('tabMode'))
  const effective = paneActive ? (mode ?? null) : null
  useEffect(() => { setMode(id.current, effective); return () => setMode(id.current, null) }, [effective, setMode])
}

/** Phạm vi pane: chỉ khi pane đang hiển thị các hook (useMobileScroll, useTabMode) mới ảnh hưởng thanh tab. */
export function MobilePaneScope({ active, children }: { active: boolean; children: ReactNode }) {
  return <PaneActive.Provider value={active}>{children}</PaneActive.Provider>
}

export function useStoryConnection(connection: WorkerConnection) {
  const { setWorker } = useContext(RuntimeActionsContext)
  useEffect(() => { setWorker(connection); return () => setWorker(null) }, [connection.connected, connection.reconnectAt, connection.retryNow, connection.blocked, setWorker])
}

export function useMobileScroll(ref: RefObject<HTMLElement | null>, options: { immersive?: boolean; disabled?: boolean; onScroll?: (element: HTMLElement) => void } = {}) {
  const { setScroll } = useContext(RuntimeActionsContext)
  const paneActive = useContext(PaneActive)
  const disabled = Boolean(options.disabled) || !paneActive
  const [state, update] = useState({ down: false, immersive: false })
  const callbacks = useRef(options); callbacks.current = options
  const last = useRef(0)
  useEffect(() => {
    const element = ref.current
    if (!element || disabled) return
    last.current = element.scrollTop
    const scroll = () => {
      const y = element.scrollTop
      if (y === last.current) return
      // Collapsing chrome/chips can clamp scrollTop; that is not an upward gesture.
      if (y < last.current && last.current > element.scrollHeight - element.clientHeight) {
        last.current = y
        callbacks.current.onScroll?.(element)
        return
      }
      const down = y > last.current && y > 40
      const immersive = Boolean(callbacks.current.immersive && down)
      last.current = y
      update((previous) => previous.down === down && previous.immersive === immersive ? previous : { down, immersive }); setScroll(down, immersive)
      callbacks.current.onScroll?.(element)
    }
    element.addEventListener('scroll', scroll, { passive: true })
    // Pane ẩn → hiện lại: khôi phục cờ "đang cuộn xuống" của chính pane. Đặt ở khung hình kế tiếp
    // để thắng nhịp nhả (false) của pane vừa ẩn, bất kể thứ tự effect giữa hai pane.
    const frame = requestAnimationFrame(() => { if (state.down || state.immersive) setScroll(state.down, state.immersive) })
    return () => { cancelAnimationFrame(frame); element.removeEventListener('scroll', scroll); setScroll(false, false) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ref, setScroll, disabled])
  const reveal = useCallback(() => { update((previous) => !previous.down && !previous.immersive ? previous : { down: false, immersive: false }); setScroll(false, false) }, [setScroll])
  return { ...state, reveal }
}
