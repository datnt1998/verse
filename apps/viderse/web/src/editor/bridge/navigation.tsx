import { createContext, useContext, useEffect, useMemo, useState, type AnchorHTMLAttributes, type ImgHTMLAttributes } from 'react'
import { navigate, useRoute } from '../../router'

/** The editor shares the application's hash location, never Next's pathname. */
export function hashHref(href: string): string {
  if (!href.startsWith('/') || href.startsWith('//')) return href
  return `#${href}`
}
const Pending = createContext({ pending: false })
export function useLinkStatus() { return useContext(Pending) }
export default function Link({ href, children, onClick, prefetch: _prefetch, ...props }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string; prefetch?: boolean }) {
  const [pending, setPending] = useState(false)
  useEffect(() => {
    const done = () => setPending(false)
    window.addEventListener('hashchange', done)
    return () => window.removeEventListener('hashchange', done)
  }, [])
  return <a {...props} href={hashHref(href)} onClick={(event) => {
    onClick?.(event)
    if (!event.defaultPrevented && event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey && hashHref(href) !== location.hash && !props.target) setPending(true)
  }}><Pending value={{ pending }}>{children}</Pending></a>
}
export function useRouter() {
  return useMemo(() => ({
    push: (href: string) => { if (href.startsWith('#') || href.startsWith('/')) navigate(href.replace(/^#/, '')); else location.assign(href) },
    replace: (href: string) => { if (href.startsWith('#') || href.startsWith('/')) navigate(href.replace(/^#/, ''), true); else location.replace(href) },
    refresh: () => location.reload(),
    back: () => history.back(),
  }), [])
}
export function useSearchParams() { return useRoute().search }
export function useParams<T extends Record<string, string> = Record<string, string>>(): T {
  const { route } = useRoute()
  const result = route.name === 'editor' ? { projectId: route.projectId } : route.name === 'render' || route.name === 'measure' ? { requestId: route.requestId } : {}
  return result as T
}
/** Native media: the server, not a Next image optimizer, owns project bytes. */
export function Image({ fill, unoptimized: _unoptimized, priority, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean; unoptimized?: boolean; priority?: boolean }) {
  return <img {...props} loading={priority ? 'eager' : props.loading} width={fill ? undefined : props.width} height={fill ? undefined : props.height} />
}
