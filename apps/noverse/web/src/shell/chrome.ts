// Đo chiều cao chrome của shell (header + tab bar đáy) để màn con dựng cột đúng viewport.
import { useEffect, useState } from 'react'

/** Chiều cao header và tab bar đang render (px); 0 khi không có (desktop). */
export function useShellChrome(): { header: number; tabbar: number } {
  const [chrome, setChrome] = useState({ header: 0, tabbar: 0 })
  useEffect(() => {
    const measure = () => {
      const header = document.querySelector('header')?.getBoundingClientRect().height ?? 0
      const tabbar = [...document.querySelectorAll('nav[aria-label]')]
        .reduce((tallest, element) => Math.max(tallest, element.getBoundingClientRect().height), 0)
      setChrome((previous) => (previous.header === header && previous.tabbar === tabbar ? previous : { header, tabbar }))
    }
    measure()
    addEventListener('resize', measure)
    return () => removeEventListener('resize', measure)
  }, [])
  return chrome
}
