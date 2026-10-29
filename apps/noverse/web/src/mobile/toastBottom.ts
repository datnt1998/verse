import { useEffect, useRef } from 'react'
import { pickToastBottom, type ToastBottomEntry } from './logic'

// Toast nằm ngoài từng màn (MobileFeedback), nên mỗi màn đăng ký khoảng cách đáy qua `--m-toast-bottom`.
// Ưu tiên cao thắng; cùng ưu tiên thì đăng ký sau thắng. Mặc định theo route = ưu tiên 0, màn tự đặt = ưu tiên 1.
const entries = new Map<symbol, ToastBottomEntry>()

function apply() {
  const px = pickToastBottom([...entries.values()])
  const root = document.documentElement.style
  if (px === null) root.removeProperty('--m-toast-bottom'); else root.setProperty('--m-toast-bottom', `${px}px`)
}

export function useToastBottom(px: number | null | undefined, priority = 0) {
  const id = useRef(Symbol('toastBottom'))
  useEffect(() => {
    const key = id.current
    if (px == null) { entries.delete(key); apply(); return }
    entries.delete(key)
    entries.set(key, { px, priority })
    apply()
    return () => { entries.delete(key); apply() }
  }, [px, priority])
}
