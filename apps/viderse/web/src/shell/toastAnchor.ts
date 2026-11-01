// Neo vị trí viên toast (V31 · r35): `[data-toast-anchor]` là thanh đáy màn
// (ô Nhắn trợ lý của AssistantComposer). Viên nằm giữa cột nội dung, trên
// thanh đó 12px, qua hai biến --vv-toast-left/--vv-toast-bottom trên <html>;
// không có neo thì host dùng fallback giữa đáy màn (Editor, các màn cổng).
import { useEffect } from 'react'

const SELECTOR = '[data-toast-anchor]'

export function ToastAnchor() {
  useEffect(() => {
    const root = document.documentElement
    let anchor: Element | null = null
    let resize: ResizeObserver | null = null
    const update = () => {
      if (!anchor || !anchor.isConnected) { root.style.removeProperty('--vv-toast-left'); root.style.removeProperty('--vv-toast-bottom'); return }
      const rect = anchor.getBoundingClientRect()
      if (!rect.width) { root.style.removeProperty('--vv-toast-left'); root.style.removeProperty('--vv-toast-bottom'); return }
      root.style.setProperty('--vv-toast-left', `${Math.round(rect.left + rect.width / 2)}px`)
      root.style.setProperty('--vv-toast-bottom', `${Math.max(0, Math.round(window.innerHeight - rect.top + 12))}px`)
    }
    const watch = (next: Element | null) => {
      if (next === anchor) return
      resize?.disconnect()
      anchor = next
      if (anchor && typeof ResizeObserver !== 'undefined') { resize = new ResizeObserver(update); resize.observe(anchor) }
      update()
    }
    watch(document.querySelector(SELECTOR))
    const mutations = new MutationObserver(() => watch(document.querySelector(SELECTOR)))
    mutations.observe(document.body, { childList: true, subtree: true })
    window.addEventListener('resize', update)
    document.addEventListener('scroll', update, true)
    return () => {
      mutations.disconnect()
      resize?.disconnect()
      window.removeEventListener('resize', update)
      document.removeEventListener('scroll', update, true)
      root.style.removeProperty('--vv-toast-left')
      root.style.removeProperty('--vv-toast-bottom')
    }
  }, [])
  return null
}
