// Theo dõi media query (như useMediaQuery của Noverse) — trả true khi khớp,
// cập nhật khi bề ngang đổi. Shell dùng để chuyển giữa khung điện thoại / desktop.
import { useEffect, useState } from 'react'

export function useMediaQuery(query: string): boolean {
  const [match, setMatch] = useState(() => typeof matchMedia !== 'undefined' && matchMedia(query).matches)
  useEffect(() => {
    const mql = matchMedia(query)
    const update = () => setMatch(mql.matches)
    update()
    mql.addEventListener('change', update)
    return () => mql.removeEventListener('change', update)
  }, [query])
  return match
}
