// Lưới ảo hoá tự viết (Khám phá): đo số cột theo bề rộng cuộn (card tối thiểu
// ~210px + khe 12px, theo grid auto-fill của artboard), đo chiều cao hàng từ
// hàng đầu tiên đã render, chỉ mount các hàng quanh khung nhìn + overscan.
// 20.000 video vẫn ≤ ~120 thẻ DOM. Kích thước lấy từ getBoundingClientRect —
// môi trường không layout (happy-dom) dùng fallback để test tính đúng.
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type UIEvent } from 'react'

export const CARD_MIN_W = 210
export const GRID_GAP_X = 12
export const GRID_GAP_Y = 14
/** Phần không ảnh của một thẻ: tiêu đề 2 dòng + meta + hàng nút + các khe. */
const CARD_EXTRA_H = 112
const FALLBACK_ROW_H = 230
const OVERSCAN_ROWS = 2

export type VirtualRange = { start: number; end: number }

/** Số cột theo bề rộng khả dụng — cùng công thức auto-fill của CSS grid. */
export const colsForWidth = (width: number): number =>
  Math.max(1, Math.floor((width + GRID_GAP_X) / (CARD_MIN_W + GRID_GAP_X)))

/** Chiều cao hàng ước tính khi chưa đo được DOM (test, lần render đầu). */
export const estimatedRowHeight = (width: number): number => {
  const cols = colsForWidth(width || CARD_MIN_W)
  const cardW = Math.max(CARD_MIN_W, (width - (cols - 1) * GRID_GAP_X) / cols)
  return cardW * (9 / 16) + CARD_EXTRA_H
}

export function useVirtualGrid({ count, viewport }: {
  count: number
  /** Kích thước ép cho test {width,height}; bỏ qua khi đo DOM được. */
  viewport?: { width: number; height: number }
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState(() => viewport ?? { width: 0, height: 0 })
  const [rowHeight, setRowHeight] = useState(() => estimatedRowHeight(viewport?.width ?? 900))
  const [scrollTop, setScrollTop] = useState(0)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el || viewport) return
    const measure = () => {
      const rect = el.getBoundingClientRect()
      if (rect.width > 0 || rect.height > 0) setSize({ width: rect.width, height: rect.height })
    }
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [viewport])

  const cols = colsForWidth(size.width || CARD_MIN_W)
  const rows = Math.ceil(count / cols)

  // Đo hàng đầu tiên thật (aspect-ratio ảnh + phần chữ) — lần đầu sau khi có item.
  const measureRow = useCallback(() => {
    const row = ref.current?.querySelector<HTMLElement>('[data-vrow]')
    const h = row?.getBoundingClientRect().height
    if (h && h > 40) setRowHeight((old) => Math.abs(old - h) > 0.5 ? h : old)
  }, [])
  useLayoutEffect(measureRow, [measureRow, count, cols])
  const onScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    setScrollTop(event.currentTarget.scrollTop)
  }, [])

  const rowH = rowHeight
  const range: VirtualRange = useMemo(() => {
    if (rows === 0) return { start: 0, end: 0 }
    const first = Math.max(0, Math.floor(scrollTop / (rowH + GRID_GAP_Y)) - OVERSCAN_ROWS)
    const last = Math.min(rows, Math.ceil((scrollTop + (size.height || 800)) / (rowH + GRID_GAP_Y)) + OVERSCAN_ROWS)
    return { start: first, end: Math.max(first, last) }
  }, [rows, scrollTop, size.height, rowH])

  /** Chỉ số item đầu/cuối cần render (cắt theo hàng, không nửa hàng). */
  const itemStart = range.start * cols
  const itemEnd = Math.min(count, range.end * cols)
  const padTop = range.start * (rowH + GRID_GAP_Y)
  const padBottom = Math.max(0, (rows - range.end) * (rowH + GRID_GAP_Y))

  return {
    ref, cols, rows, rowHeight: rowH, range, itemStart, itemEnd, padTop, padBottom, onScroll,
    /** Gọi khi dữ liệu đổi mà khung nhìn cần trượt đúng item (neo cuộn SSE). */
    scrollTo: (top: number) => { if (ref.current) ref.current.scrollTop = top },
    scrollTop,
    height: size.height,
  }
}
