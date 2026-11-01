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

/**
 * Bàn phím ảo: `delta = innerHeight - visualViewport.height - offsetTop`, chỉ tính khi
 * > 80px (ngưỡng để không nhầm với thanh địa chỉ co giãn của Safari). Ghi `--kb` lên
 * `<html>` để CSS (thanh dính đáy, sheet, toast) né bàn phím.
 */
export function useVisualViewportKeyboard(): number {
  const [keyboard, setKeyboard] = useState(0)
  useEffect(() => {
    const viewport = window.visualViewport
    const measure = () => {
      const delta = viewport ? window.innerHeight - viewport.height - viewport.offsetTop : 0
      const px = delta > 80 ? Math.round(delta) : 0
      document.documentElement.style.setProperty('--kb', `${px}px`)
      setKeyboard(px)
    }
    measure()
    viewport?.addEventListener('resize', measure)
    viewport?.addEventListener('scroll', measure)
    return () => {
      viewport?.removeEventListener('resize', measure)
      viewport?.removeEventListener('scroll', measure)
      document.documentElement.style.removeProperty('--kb')
    }
  }, [])
  return keyboard
}
