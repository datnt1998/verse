import { useEffect, useState } from 'react'

const typing = (target: EventTarget | null) =>
  target instanceof HTMLElement && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))

/** `[` đóng/mở danh sách video, `]` đóng/mở bảng bên phải (artboard Main, hàng phím tắt). Không bắt phím khi đang gõ. */
export function usePanelKeys() {
  const [leftOpen, setLeftOpen] = useState(true)
  const [rightOpen, setRightOpen] = useState(true)
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || typing(event.target)) return
      if (event.key === '[') setLeftOpen((open) => !open)
      else if (event.key === ']') setRightOpen((open) => !open)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  return { leftOpen, rightOpen }
}
