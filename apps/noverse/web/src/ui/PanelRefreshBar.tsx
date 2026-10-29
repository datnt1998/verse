import { useCallback, useEffect, useRef, useState, useSyncExternalStore, type CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { cache, type Key } from '@noverse/ui/data'

const travel = stylex.keyframes({ from: { transform: 'translateX(-100%)' }, to: { transform: 'translateX(100%)' } })
const sx = stylex.create({
  root: { position: 'absolute', top: 0, left: 0, right: 0, height: 2, minHeight: 0, overflow: 'hidden', pointerEvents: 'none', zIndex: 3 },
  bar: {
    position: 'absolute', inset: 0,
    backgroundImage: 'linear-gradient(90deg, transparent, var(--color-info), transparent)',
    animationName: { default: travel, '@media (prefers-reduced-motion: reduce)': 'none' },
    animationDuration: '1.2s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
  },
})

/** Spec 16: background bookkeeping only renders this overlay, never the panel content. */
export function PanelRefreshBar({ resourceKeys, updatedAt, style }: { resourceKeys: readonly Key[]; updatedAt?: number | null; style?: CSSProperties }) {
  const snapshot = useCallback(() => resourceKeys.some((key) => cache.isRefreshing(key)), [resourceKeys])
  const background = useSyncExternalStore(cache.onRefreshingChange, snapshot, snapshot)
  const active = background
  const [visible, setVisible] = useState(false)
  const shownAt = useRef<number | null>(null)
  useEffect(() => {
    if (active) {
      if (shownAt.current !== null) return
      const timer = setTimeout(() => { shownAt.current = Date.now(); setVisible(true) }, 300)
      return () => clearTimeout(timer)
    }
    if (shownAt.current === null) return
    const timer = setTimeout(() => { shownAt.current = null; setVisible(false) }, Math.max(0, 400 - (Date.now() - shownAt.current)))
    return () => clearTimeout(timer)
  }, [active])
  const title = updatedAt != null
    ? `Cập nhật lúc ${new Date(updatedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}`
    : undefined
  const p = stylex.props(sx.root)
  return <div title={title} data-panel-refresh="" className={p.className} style={{ ...p.style, ...style, position: 'absolute', height: 2, minHeight: 0 }}>
    {visible && <div role="progressbar" aria-label={title ? `Đang cập nhật · ${title}` : 'Đang cập nhật'} {...stylex.props(sx.bar)} />}
  </div>
}
