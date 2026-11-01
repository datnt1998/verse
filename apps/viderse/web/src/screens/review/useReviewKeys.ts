import { useEffect, useRef } from 'react'

export interface ReviewKeyHandlers {
  nextJob: () => void
  previousJob: () => void
  nextImage: () => void
  previousImage: () => void
  redraw: () => void
  approve: () => void
  /** E — sửa prompt của ảnh đang xem. */
  editPrompt?: () => void
  /** V — so với bản trước. */
  compare?: () => void
  /** Esc — huỷ sửa / bỏ chọn / đóng hộp thoại. */
  escape?: () => void
  /** ⌘↵ — vẽ lại với prompt đã sửa (chạy cả khi đang gõ trong textarea). */
  submit?: () => void
}
const actions: Record<string, keyof ReviewKeyHandlers> = { j: 'nextJob', k: 'previousJob', arrowright: 'nextImage', arrowleft: 'previousImage', r: 'redraw', a: 'approve', e: 'editPrompt', v: 'compare' }

export function useReviewKeys(handlers: ReviewKeyHandlers, enabled = true) {
  const current = useRef(handlers)
  current.current = handlers
  useEffect(() => {
    if (!enabled) return
    const onKey = (event: KeyboardEvent) => {
      const target = event.target
      const typing = target instanceof HTMLElement && (target.isContentEditable || !!target.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]'))
      // ⌘↵ và Esc phải chạy cả khi con trỏ đang trong ô sửa prompt.
      if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
        if (!current.current.submit) return
        event.preventDefault()
        current.current.submit()
        return
      }
      if (event.key === 'Escape') {
        if (!current.current.escape) return
        event.preventDefault()
        current.current.escape()
        return
      }
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.isComposing || typing) return
      const action = actions[event.key.toLowerCase()]
      if (action && current.current[action]) { event.preventDefault(); current.current[action]!() }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [enabled])
}
