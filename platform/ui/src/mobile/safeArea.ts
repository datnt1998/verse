// Vùng an toàn + nhịp điện thoại dùng chung cho mọi app của hệ (R-SA1…R-SA9).
//
// Một nguồn khai báo duy nhất nằm ở `platform/ui/src/kit.css` (`:root{--m-safe-*…}`).
// Tệp này chỉ giữ hằng số + logic thuần: mốc bề ngang, chế độ thanh tab, neo toast,
// đo bàn phím. Không đọc `env(safe-area-inset-*)` ở đây.
import { useEffect, useRef, useState } from 'react'

/** Noverse: điện thoại ≤ 640; 641–1099 là máy tính bảng. */
export const PHONE_MAX_NOVERSE = 640
/** Viderse: điện thoại ≤ 767 (chưa có artboard máy tính bảng). */
export const PHONE_MAX_VIDERSE = 767

/** Query bề ngang cho một mốc điện thoại — dùng chung với `--m-bp-*` trong CSS. */
export const phoneQuery = (max: number) => `(max-width: ${max}px)`

/**
 * Mốc điện thoại của app đang chạy, cho những chỗ trong kit không biết app nào gọi
 * (thời lượng toast, chồng thẻ việc nền). Mặc định = Noverse; Viderse gọi
 * `setPhoneMax(PHONE_MAX_VIDERSE)` ở `main.tsx`.
 */
let phoneMax = PHONE_MAX_NOVERSE
export const setPhoneMax = (max: number) => { phoneMax = max }
export const getPhoneMax = () => phoneMax

export type TabMode = 'default' | 'mini' | 'hide'

/** `hide` thắng `mini`: bàn phím mở / màn đẩy / chế độ chọn nhiều ẩn hẳn thanh tab. */
export function tabModeFor({ keyboard = 0, hidden = false, immersive = false, down = false }: {
  keyboard?: number; hidden?: boolean; immersive?: boolean; down?: boolean
}): TabMode {
  return keyboard > 0 || hidden || immersive ? 'hide' : down ? 'mini' : 'default'
}

export type ToastBottomEntry = { px: number; priority: number }

/** Ưu tiên cao thắng; cùng ưu tiên thì mục đăng ký sau thắng; không có mục nào → null (CSS mặc định). */
export function pickToastBottom(entries: ToastBottomEntry[]): number | null {
  let best: ToastBottomEntry | null = null
  for (const entry of entries) if (!best || entry.priority >= best.priority) best = entry
  return best ? best.px : null
}

// Toast nằm ngoài từng màn, nên mỗi màn đăng ký khoảng cách đáy qua `--m-toast-bottom`
// trên <html>. Mặc định theo route = ưu tiên 0, màn tự đặt = ưu tiên 1.
const entries = new Map<symbol, ToastBottomEntry>()

function applyToastBottom() {
  const px = pickToastBottom([...entries.values()])
  const root = document.documentElement.style
  if (px === null) root.removeProperty('--m-toast-bottom')
  else root.setProperty('--m-toast-bottom', `${px}px`)
}

/**
 * Neo toast theo màn. `px = null` = không đăng ký (dùng mặc định của kit: trên thanh tab 12px).
 * Ví dụ: màn có thanh CTA dính đáy cao 94px → `useToastBottom(94 + 12, 1)`.
 */
export function useToastBottom(px: number | null | undefined, priority = 0) {
  const id = useRef(Symbol('toastBottom'))
  useEffect(() => {
    const key = id.current
    if (px == null) { entries.delete(key); applyToastBottom(); return }
    entries.delete(key)
    entries.set(key, { px, priority })
    applyToastBottom()
    return () => { entries.delete(key); applyToastBottom() }
  }, [px, priority])
}

export type VisualViewportMetrics = {
  /** `window.innerHeight` — trên iOS là chiều cao layout viewport (không co theo bàn phím). */
  innerHeight: number
  /** `visualViewport.height` — phần thật sự nhìn thấy. */
  height: number
  /** `visualViewport.offsetTop` — đỉnh vùng nhìn thấy lệch khỏi đỉnh layout viewport. */
  offsetTop: number
  /** `visualViewport.scale` — > 1 khi người dùng chụm để thu phóng. */
  scale: number
  /** `document.activeElement` có phải ô nhập liệu có thể mở bàn phím ảo hay không. */
  editing: boolean
}

export type VisualViewportLayout = {
  /** Chiều cao bàn phím ảo (px), 0 khi đóng hoặc khi chỉ là thanh địa chỉ co giãn. */
  keyboard: number
  /**
   * Đáy shell trong toạ độ layout viewport — mốc cao hơn giữa `innerHeight` và đáy
   * vùng nhìn thấy (`offsetTop + height`), hoặc `null` khi không được dùng làm chiều
   * cao khung (bàn phím mở / đang thu phóng).
   */
  bottom: number | null
}

/**
 * Hình học khung nhìn thật — nguồn duy nhất cho chiều cao shell điện thoại.
 *
 * Chỉ coi là bàn phím khi đang focus một ô nhập liệu, chưa thu phóng và phần bị che
 * > 80px. Safari co giãn thanh địa chỉ cũng làm lệch `innerHeight`/`visualViewport`
 * nhưng đó không phải bàn phím: nếu phân loại bằng hình học thuần, nav bị ẩn oan khi
 * người dùng chỉ cuộn trang. Khi không có bàn phím và không thu phóng, đáy shell là
 * mốc cao hơn giữa layout viewport (`innerHeight`) và đáy vùng nhìn thấy
 * (`offsetTop + height`). iOS 26 / Safari có thể để lại `offsetTop > 0` và
 * `visualViewport.height` hụt so với `innerHeight` sau khi đóng bàn phím; các phần tử
 * neo vào `100dvh`/`position: fixed` khi đó bị đẩy lên khỏi đáy cho tới cú cuộn kế
 * tiếp. Neo shell/thanh tab vào mốc này khôi phục đáy thiết bị mà không cần cuộn.
 * Số đo rỗng hoặc đang thu phóng thì không suy ra chiều cao khung từ đây (giữ
 * `100dvh` và `--kb` như cũ).
 */
export function visualViewportLayout({ innerHeight, height, offsetTop, scale, editing }: VisualViewportMetrics): VisualViewportLayout {
  const pan = Math.max(0, offsetTop)
  const valid = height > 0 && innerHeight > 0
  const zoomed = Math.abs((scale || 1) - 1) > .01
  const occluded = innerHeight - height - pan
  const keyboard = valid && editing && !zoomed && occluded > 80 ? Math.round(occluded) : 0
  const usable = valid && !zoomed && keyboard === 0
  return { keyboard, bottom: usable ? Math.round(Math.max(innerHeight, pan + height)) : null }
}

const TEXT_INPUT_TYPES = /^(text|search|email|url|tel|password|number|date|datetime-local|month|time|week)$/

/**
 * Chỉ các ô nhập chữ mới có thể mở bàn phím ảo. Xét cả shadow root để composer trong
 * web component vẫn nhận đúng trạng thái.
 */
function isEditingElement(element: Element | null): boolean {
  let node: Element | null = element
  while (node) {
    if (node instanceof HTMLElement) {
      if (node.isContentEditable) return true
      if (node instanceof HTMLTextAreaElement) return !node.readOnly && !node.disabled
      if (node instanceof HTMLInputElement) return !node.readOnly && !node.disabled && TEXT_INPUT_TYPES.test(node.type)
    }
    node = node.shadowRoot?.activeElement ?? null
  }
  return false
}

/**
 * Bàn phím ảo + đáy khung nhìn thật. Ghi `--kb` và `--m-viewport-bottom` lên `<html>`:
 * - `--kb` cho CSS (chân trang, sheet, toast) né bàn phím;
 * - `--m-viewport-bottom` là mốc đáy shell; CSS dùng
 *   `max(100dvh, var(--m-viewport-bottom, 0px))`, sửa đúng trạng thái lệch còn lại sau
 *   khi Safari đóng bàn phím hoặc co giãn thanh địa chỉ.
 *
 * Nghe thêm `window.resize`, `orientationchange`, `pageshow`, `focusin/focusout` và
 * `visibilitychange`: iOS không phải lúc nào cũng phát `visualViewport` event ở nhịp
 * cuối của transition. `focusin/focusout` cũng là nguồn phân loại bàn phím: chỉ ô
 * nhập đang focus mới coi là bàn phím, còn thanh địa chỉ co giãn thì không. Sau
 * `focusout` chỉ đo lại một lần (hẹn giờ, không polling) vì Safari hoàn tất animation
 * đóng bàn phím sau sự kiện.
 */
export function useVisualViewportKeyboard(): number {
  const [keyboard, setKeyboard] = useState(0)
  useEffect(() => {
    const root = document.documentElement
    const viewport = window.visualViewport
    let frame = 0
    let settle = 0

    const measure = () => {
      frame = 0
      if (!viewport) {
        root.style.setProperty('--kb', '0px')
        root.style.removeProperty('--m-viewport-bottom')
        setKeyboard(0)
        return
      }
      const layout = visualViewportLayout({
        innerHeight: window.innerHeight,
        height: viewport.height,
        offsetTop: viewport.offsetTop,
        scale: viewport.scale,
        editing: isEditingElement(document.activeElement),
      })
      root.style.setProperty('--kb', `${layout.keyboard}px`)
      if (layout.bottom === null) root.style.removeProperty('--m-viewport-bottom')
      else root.style.setProperty('--m-viewport-bottom', `${layout.bottom}px`)
      setKeyboard(layout.keyboard)
    }
    const schedule = () => {
      if (frame) return
      if (typeof window.requestAnimationFrame === 'function') frame = window.requestAnimationFrame(measure)
      else measure()
    }
    /** Một nhịp đo lại sau khi mất focus; event cuối của Safari có thể tới trước animation. */
    const resettle = () => {
      schedule()
      window.clearTimeout(settle)
      settle = window.setTimeout(measure, 350)
    }
    const onVisibility = () => { if (document.visibilityState === 'visible') measure() }

    measure()
    viewport?.addEventListener('resize', schedule)
    viewport?.addEventListener('scroll', schedule)
    window.addEventListener('resize', schedule)
    window.addEventListener('orientationchange', schedule)
    window.addEventListener('pageshow', measure)
    document.addEventListener('focusin', schedule)
    document.addEventListener('focusout', resettle)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      if (frame && typeof window.cancelAnimationFrame === 'function') window.cancelAnimationFrame(frame)
      window.clearTimeout(settle)
      viewport?.removeEventListener('resize', schedule)
      viewport?.removeEventListener('scroll', schedule)
      window.removeEventListener('resize', schedule)
      window.removeEventListener('orientationchange', schedule)
      window.removeEventListener('pageshow', measure)
      document.removeEventListener('focusin', schedule)
      document.removeEventListener('focusout', resettle)
      document.removeEventListener('visibilitychange', onVisibility)
      root.style.removeProperty('--kb')
      root.style.removeProperty('--m-viewport-bottom')
    }
  }, [])
  return keyboard
}
