// Hash router có kiểu — bản scaffold chỉ có hai màn trong app shell:
// Trang chủ `#/` và Tài khoản `#/account`. Hash không hợp lệ nào cũng rơi về
// Trang chủ (parseHash không bao giờ trả route ngoài union).
import { useEffect, useState } from 'react'

export type Route = { name: 'home' } | { name: 'account' }

export type Parsed = { route: Route; search: URLSearchParams }

const HOME: Route = { name: 'home' }

export function parseHash(hash: string): Parsed {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  const [pathPart, queryPart] = raw.split('?')
  const search = new URLSearchParams(queryPart ?? '')
  const seg = pathPart.split('/').filter(Boolean)
  if (seg.length === 1 && seg[0] === 'account') return { route: { name: 'account' }, search }
  return { route: HOME, search }
}

/** Đường dẫn tương ứng một route — dùng để nhận ra hash đã chuẩn hoá. */
export const routePath = (route: Route): string => (route.name === 'account' ? '/account' : '/')

export const paths = {
  home: () => '/',
  account: () => '/account',
}

/**
 * replaceState không bắn `hashchange` nên dispatch tay (giống Noverse);
 * gán location.hash thì trình duyệt tự bắn sự kiện.
 */
export function navigate(path: string, replace = false) {
  if (replace) {
    const next = new URL(location.href)
    next.hash = `#${path}`
    history.replaceState(null, '', next)
    dispatchEvent(new HashChangeEvent('hashchange'))
  } else {
    location.hash = `#${path}`
  }
}

export function useRoute(): Parsed {
  const [hash, setHash] = useState(location.hash)
  useEffect(() => {
    const change = () => setHash(location.hash)
    addEventListener('hashchange', change)
    return () => removeEventListener('hashchange', change)
  }, [])
  return parseHash(hash)
}
