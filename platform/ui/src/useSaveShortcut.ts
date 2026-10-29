import { useEffect, useRef } from 'react'
export function useSaveShortcut(save: () => unknown, enabled = true) {
  const latest = useRef(save)
  latest.current = save
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
        event.preventDefault()
        if (enabled && !event.repeat) void latest.current()
      }
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [enabled])
}
