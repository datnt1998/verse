// Lưới ảo hoá tự viết (Khám phá, Thư viện): đo số cột theo bề rộng cuộn (thẻ
// rộng tối thiểu + khe, cùng công thức auto-fill của artboard), đo chiều cao
// hàng từ hàng đầu tiên đã render, chỉ mount các hàng quanh khung nhìn +
// overscan. Kích thước lấy từ getBoundingClientRect — môi trường không layout
// (happy-dom) dùng fallback để test tính đúng.
import { useCallback, useLayoutEffect, useMemo, useRef, useState, type UIEvent } from 'react'

export const CARD_MIN_W = 210
export const GRID_GAP_X = 12
export const GRID_GAP_Y = 14
/** Phần không ảnh của một thẻ Khám phá: tiêu đề 2 dòng + meta + hàng nút + khe. */
const CARD_EXTRA_H = 112
const OVERSCAN_ROWS = 2

/** Kích thước thẻ của lưới — mặc định là thẻ video 16:9 của Khám phá. */
export type GridMetrics = {
  /** bề rộng thẻ tối thiểu */
  cardMinW?: number
  /** phần cao thêm ngoài ảnh 16:9 (chữ, nút, khe) */
  cardExtraH?: number
  gapX?: number
  gapY?: number
}

export type VirtualRange = { start: number; end: number }

const metricsOf = (metrics: GridMetrics = {}) => ({
  cardMinW: metrics.cardMinW ?? CARD_MIN_W,
  cardExtraH: metrics.cardExtraH ?? CARD_EXTRA_H,
  gapX: metrics.gapX ?? GRID_GAP_X,
  gapY: metrics.gapY ?? GRID_GAP_Y,
})

/** Số cột theo bề rộng khả dụng — cùng công thức auto-fill của CSS grid. */
export function colsForWidth(width: number, cardMinW = CARD_MIN_W, gapX = GRID_GAP_X): number {
  return Math.max(1, Math.floor((width + gapX) / (cardMinW + gapX)))
}

/** Chiều cao hàng ước tính khi chưa đo được DOM (test, lần render đầu). */
export function estimatedRowHeight(width: number, metrics: GridMetrics = {}): number {
  const { cardMinW, cardExtraH, gapX } = metricsOf(metrics)
  const cols = colsForWidth(width || cardMinW, cardMinW, gapX)
  const cardW = Math.max(cardMinW, (width - (cols - 1) * gapX) / cols)
  return cardW * (9 / 16) + cardExtraH
}

export function useVirtualGrid({ count, viewport, metrics }: {
  count: number
  /** Kích thước ép cho test {width,height}; bỏ qua khi đo DOM được. */
  viewport?: { width: number; height: number }
  /** Thẻ rộng/khác chiều cao thẻ video — mặc định theo thẻ Khám phá. */
  metrics?: GridMetrics
}) {
  const { cardMinW, gapX, gapY } = metricsOf(metrics)
  const ref = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState(() => viewport ?? { width: 0, height: 0 })
  const [rowHeight, setRowHeight] = useState(() => estimatedRowHeight(viewport?.width ?? 900, metrics))
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

  const cols = colsForWidth(size.width || cardMinW, cardMinW, gapX)
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
    const first = Math.max(0, Math.floor(scrollTop / (rowH + gapY)) - OVERSCAN_ROWS)
    const last = Math.min(rows, Math.ceil((scrollTop + (size.height || 800)) / (rowH + gapY)) + OVERSCAN_ROWS)
    return { start: first, end: Math.max(first, last) }
  }, [rows, scrollTop, size.height, rowH, gapY])

  /** Chỉ số item đầu/cuối cần render (cắt theo hàng, không nửa hàng). */
  const itemStart = range.start * cols
  const itemEnd = Math.min(count, range.end * cols)
  const padTop = range.start * (rowH + gapY)
  const padBottom = Math.max(0, (rows - range.end) * (rowH + gapY))

  return {
    ref, cols, rows, rowHeight: rowH, range, itemStart, itemEnd, padTop, padBottom, onScroll,
    /** Gọi khi dữ liệu đổi mà khung nhìn cần trượt đúng item (neo cuộn SSE). */
    scrollTo: (top: number) => { if (ref.current) ref.current.scrollTop = top },
    scrollTop,
    height: size.height,
  }
}
