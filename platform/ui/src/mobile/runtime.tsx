// Runtime mobile của kit — nhịp cuộn, bàn phím, chế độ thanh tab.
// TẠM THỜI: nhánh safe-area/tab-bar sở hữu tệp này (runtime.tsx); bản của nhánh đó
// là bản chuẩn. Bản ở đây giữ đúng hợp đồng xuất (useMobileRuntime/useTabMode/
// useMobileScroll/useSustained) để component mobile của kit chạy được độc lập.
import {
  createContext, useCallback, useContext, useEffect, useMemo, useRef, useState,
  type ReactNode, type RefObject,
} from 'react'

export type TabMode = 'default' | 'mini' | 'hide'

export type MobileRuntimeValue = {
  /** chiều cao bàn phím ảo (px), 0 khi đóng */
  keyboard: number
  /** chế độ thanh tab đang áp dụng */
  tabMode: TabMode
  /** người dùng đang cuộn xuống (thanh tab thu nhỏ) */
  down: boolean
  /** báo hướng cuộn cho runtime */
  setScroll: (down: boolean, immersive?: boolean) => void
}

const FALLBACK: MobileRuntimeValue = { keyboard: 0, tabMode: 'default', down: false, setScroll: () => {} }

const RuntimeContext = createContext<MobileRuntimeValue>(FALLBACK)

/** Chế độ thanh tab do từng màn hình yêu cầu (màn con ẩn thanh tab). */
const TabModeContext = createContext<(mode: TabMode | null) => void>(() => {})

/** Bàn phím ảo: đọc visualViewport, trả về chiều cao bị bàn phím chiếm. */
export function useVisualViewportKeyboard(): number {
  const [keyboard, setKeyboard] = useState(0)
  useEffect(() => {
    const view = window.visualViewport
    if (!view) return
    const read = () => setKeyboard(Math.max(0, Math.round(window.innerHeight - view.height - view.offsetTop)))
    read()
    view.addEventListener('resize', read)
    view.addEventListener('scroll', read)
    return () => {
      view.removeEventListener('resize', read)
      view.removeEventListener('scroll', read)
    }
  }, [])
  return keyboard
}

/** Đặt biến `--kb` trên <html> để CSS dùng chung (bàn phím ảo). */
export function useKeyboardVar(): number {
  const keyboard = useVisualViewportKeyboard()
  useEffect(() => {
    document.documentElement.style.setProperty('--kb', `${keyboard}px`)
  }, [keyboard])
  return keyboard
}

export function MobileRuntimeProvider({ children, hidden }: { children: ReactNode; hidden?: boolean }) {
  const keyboard = useKeyboardVar()
  const [down, setDown] = useState(false)
  const [requested, setRequested] = useState<TabMode | null>(null)
  const setScroll = useCallback((next: boolean) => setDown(next), [])
  const requestTabMode = useCallback((mode: TabMode | null) => setRequested(mode), [])
  const tabMode: TabMode = hidden ? 'hide' : requested ?? (keyboard > 0 || down ? 'mini' : 'default')
  const value = useMemo<MobileRuntimeValue>(() => ({ keyboard, tabMode, down, setScroll }), [keyboard, tabMode, down, setScroll])
  return (
    <RuntimeContext.Provider value={value}>
      <TabModeContext.Provider value={requestTabMode}>{children}</TabModeContext.Provider>
    </RuntimeContext.Provider>
  )
}

export const useMobileRuntime = (): MobileRuntimeValue => useContext(RuntimeContext)

/**
 * Yêu cầu chế độ thanh tab cho màn hình đang render. Không truyền tham số thì chỉ đọc.
 * `hide` = màn con (không có thanh tab), `mini` = thanh tab thu nhỏ.
 */
export function useTabMode(mode?: TabMode): TabMode {
  const request = useContext(TabModeContext)
  const { tabMode } = useMobileRuntime()
  useEffect(() => {
    if (!mode) return
    request(mode)
    return () => request(null)
  }, [mode, request])
  return mode ?? tabMode
}

/** Cuộn xuống → thu nhỏ thanh tab; cuộn lên → hiện lại. */
export function useMobileScroll(
  ref: RefObject<HTMLElement | null>,
  options: { disabled?: boolean } = {},
): { down: boolean; immersive: boolean; reveal: () => void } {
  const { setScroll } = useMobileRuntime()
  const [down, setDown] = useState(false)
  const [immersive, setImmersive] = useState(false)
  const last = useRef(0)
  const disabled = options.disabled ?? false
  useEffect(() => {
    const el = ref.current
    if (!el || disabled) return
    const onScroll = () => {
      const top = el.scrollTop
      const delta = top - last.current
      if (Math.abs(delta) < 8) return
      const next = delta > 0 && top > 24
      last.current = top
      setDown(next)
      setImmersive(next && delta > 40)
      setScroll(next, next && delta > 40)
    }
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [ref, disabled, setScroll])
  const reveal = useCallback(() => {
    setDown(false)
    setImmersive(false)
    setScroll(false, false)
  }, [setScroll])
  return { down, immersive, reveal }
}

/** true khi `value` đã giữ nguyên trạng thái thật trong `ms` — tránh nháy banner. */
export function useSustained(value: boolean, ms = 3000): boolean {
  const [settled, setSettled] = useState(false)
  useEffect(() => {
    if (!value) {
      setSettled(false)
      return
    }
    const timer = setTimeout(() => setSettled(true), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return settled
}
