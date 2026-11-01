// Nhịp điện thoại dùng chung: bàn phím ảo, chế độ thanh tab, cờ cuộn.
//
// Provider này KHÔNG biết phiên đăng nhập, mạng hay outbox — app bọc nó ở ngoài và
// tự vẽ băng mất kết nối / sheet hết phiên. App nào cũng có thể gọi `useTabMode`,
// `useMobileScroll`, `MobilePaneScope` mà không cần tự dựng lại state.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode, RefObject } from 'react'
import { tabModeFor, useVisualViewportKeyboard } from './safeArea'
import type { TabMode } from './safeArea'

type ScrollState = { down: boolean; immersive: boolean }

export type MobileRuntime = {
  /** Chiều cao bàn phím ảo (px), 0 khi đóng. */
  keyboard: number
  tabMode: TabMode
  down: boolean
  setScroll: (down: boolean, immersive: boolean) => void
}

type RuntimeActions = Pick<MobileRuntime, 'setScroll'> & { setMode: (id: symbol, mode: TabMode | null) => void }

const RuntimeContext = createContext<MobileRuntime>({ keyboard: 0, tabMode: 'default', down: false, setScroll: () => {} })
const RuntimeActionsContext = createContext<RuntimeActions>({ setScroll: () => {}, setMode: () => {} })

export const useMobileRuntime = () => useContext(RuntimeContext)

/**
 * Nhận cờ "đang hỏng" nhưng chỉ bật sau `ms` liên tục — để không nháy băng mất kết nối
 * cho những lần rớt ngắn (iOS nền → hiện lại).
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

export function MobileRuntimeProvider({ children, hidden = false }: { children: ReactNode; hidden?: boolean }) {
  const keyboard = useVisualViewportKeyboard()
  const [scroll, updateScroll] = useState<ScrollState>({ down: false, immersive: false })
  const [modes, setModes] = useState<ReadonlyMap<symbol, TabMode>>(() => new Map())

  const setMode = useCallback((id: symbol, mode: TabMode | null) => setModes((previous) => {
    if (mode === null ? !previous.has(id) : previous.get(id) === mode) return previous
    const next = new Map(previous)
    if (mode === null) next.delete(id)
    else next.set(id, mode)
    return next
  }), [])
  const setScroll = useCallback((down: boolean, immersive: boolean) => updateScroll((previous) => previous.down === down && previous.immersive === immersive ? previous : { down, immersive }), [])

  const requested = [...modes.values()]
  const tabMode = tabModeFor({
    keyboard,
    hidden: hidden || requested.includes('hide'),
    immersive: scroll.immersive,
    down: scroll.down || requested.includes('mini'),
  })
  const actions = useMemo(() => ({ setScroll, setMode }), [setScroll, setMode])
  const value = useMemo(() => ({ keyboard, tabMode, down: scroll.down, setScroll }), [keyboard, tabMode, scroll.down, setScroll])

  useEffect(() => { document.documentElement.dataset.tabMode = tabMode }, [tabMode])
  return <RuntimeActionsContext.Provider value={actions}><RuntimeContext.Provider value={value}>{children}</RuntimeContext.Provider></RuntimeActionsContext.Provider>
}

/**
 * Màn (hoặc thành phần) xin một chế độ thanh tab. `hide` thắng `mini`; bàn phím mở
 * luôn ẩn. Chỉ pane đang hiển thị mới có hiệu lực (xem `MobilePaneScope`).
 */
export function useTabMode(mode?: TabMode) {
  const { setMode } = useContext(RuntimeActionsContext)
  const paneActive = useMobilePaneActive()
  const id = useRef(Symbol('tabMode'))
  const effective = paneActive ? (mode ?? null) : null
  useEffect(() => { setMode(id.current, effective); return () => setMode(id.current, null) }, [effective, setMode])
}

const PaneActive = createContext(true)

/** Pane đang hiển thị? Hook tab-mode/cuộn của pane ẩn không được đẩy state toàn cục. */
export const useMobilePaneActive = () => useContext(PaneActive)

/** Phạm vi pane: chỉ khi pane đang hiển thị các hook (useMobileScroll, useTabMode) mới ảnh hưởng thanh tab. */
export function MobilePaneScope({ active, children }: { active: boolean; children: ReactNode }) {
  return <PaneActive.Provider value={active}>{children}</PaneActive.Provider>
}

export function useMobileScroll(ref: RefObject<HTMLElement | null>, options: { immersive?: boolean; disabled?: boolean; onScroll?: (element: HTMLElement) => void } = {}) {
  const { setScroll } = useContext(RuntimeActionsContext)
  const paneActive = useMobilePaneActive()
  const disabled = Boolean(options.disabled) || !paneActive
  const [state, update] = useState<ScrollState>({ down: false, immersive: false })
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
