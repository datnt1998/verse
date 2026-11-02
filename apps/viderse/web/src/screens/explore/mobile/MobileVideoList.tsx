import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import * as stylex from '@stylexjs/stylex'
import type { Video } from '../../../api/rivals'
import { mStyles } from './mexplore.stylex'

/** Giữ nguyên vùng cuộn của pane; chỉ dựng các thẻ quanh khung nhìn. */
export function MobileVideoList({ items, hasMore, needMore, children }: {
  items: Video[]
  hasMore: boolean
  needMore: () => void
  children: (video: Video) => ReactNode
}) {
  const host = useRef<HTMLDivElement>(null)
  const [scrollMargin, setScrollMargin] = useState(0)
  const getScrollElement = useCallback(() => {
    for (let node = host.current?.parentElement; node; node = node.parentElement) {
      if (/(auto|scroll)/.test(getComputedStyle(node).overflowY)) return node
    }
    return document.documentElement
  }, [])
  useLayoutEffect(() => {
    const element = host.current
    const scroller = getScrollElement()
    if (!element) return
    const measure = () => setScrollMargin(
      element.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop,
    )
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(scroller)
    if (element.parentElement) observer.observe(element.parentElement)
    return () => observer.disconnect()
  }, [getScrollElement])
  const getItemKey = useCallback((index: number) => items[index].id, [items])
  const estimateSize = useCallback((index: number) =>
    (host.current?.clientWidth || 358) * 9 / 16 + 90 + (items[index].usage?.length ? 44 : 0),
  [items])
  const list = useVirtualizer({
    count: items.length,
    getScrollElement,
    getItemKey,
    estimateSize,
    initialRect: { width: 390, height: 844 },
    overscan: 3,
    gap: 18,
    scrollMargin,
    useAnimationFrameWithResizeObserver: true,
  })
  const rows = list.getVirtualItems()
  const lastIndex = rows.at(-1)?.index ?? -1
  useEffect(() => {
    if (hasMore && lastIndex >= items.length - 4) needMore()
  }, [hasMore, lastIndex, items.length, needMore])
  return (
    <div ref={host} {...stylex.props(mStyles.list)} style={{ position: 'relative', height: list.getTotalSize() }}>
      {rows.map(row => (
        <div key={row.key} data-index={row.index} ref={list.measureElement}
          style={{ position: 'absolute', top: 0, left: 0, width: '100%', transform: `translateY(${row.start - scrollMargin}px)` }}>
          {children(items[row.index])}
        </div>
      ))}
    </div>
  )
}
