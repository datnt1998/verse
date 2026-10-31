import { useEffect, useRef } from 'react'

export interface ReviewKeyHandlers {
  nextJob: () => void
  previousJob: () => void
  nextImage: () => void
  previousImage: () => void
  redraw: () => void
  approve: () => void
}
const actions: Record<string, keyof ReviewKeyHandlers> = { j: 'nextJob', k: 'previousJob', arrowright: 'nextImage', arrowleft: 'previousImage', r: 'redraw', a: 'approve' }

export function useReviewKeys(handlers: ReviewKeyHandlers, enabled = true) {
  const current = useRef(handlers)
  current.current = handlers
  useEffect(() => {
    if (!enabled) return
    const onKey = (event: KeyboardEvent) => {
      const target = event.target
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.isComposing ||
        target instanceof HTMLElement && (target.isContentEditable || !!target.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]'))) return
      const action = actions[event.key.toLowerCase()]
      if (action) { event.preventDefault(); current.current[action]() }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [enabled])
}
