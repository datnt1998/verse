// Sheet — panel trượt từ đáy (mobile bottom sheet của Kit, 17-kit).
// Lớp CSS .m-overlay/.m-scrim/.m-sheet nằm trong kit.css; app giữ nguyên tên lớp cho CSS bổ sung.
import { useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { useEscapeLayer } from './Overlays'
import { getKitLabels } from './labels'

let scrollLocks = 0
let originalOverflow = ''

/** Kéo xuống quá 30% chiều cao thì đóng (nhịp của app di động). */
export function sheetCloseThreshold(distance: number, height: number): boolean {
  return height > 0 && distance > height * 0.3
}

export type SheetProps = {
  open: boolean
  onClose: () => void
  title?: string
  label?: string
  children: ReactNode
  height?: number | string
  style?: CSSProperties
  xstyle?: stylex.StyleXStyles
  bodyStyle?: stylex.StyleXStyles
  /** thanh kéo đóng (mặc định có) */
  grab?: boolean
  role?: 'dialog' | 'alertdialog'
  /** `md` = tấm theo artboard Viderse (góc 22, nền --color-surface-3, tiêu đề 18px,
   *  grabber 36×4, max-height 86%). Mặc định `lg` = tấm Noverse hiện tại. */
  size?: 'lg' | 'md'
  /** Câu phụ dưới tiêu đề (12.5px, --kit-fg-sub). */
  subtitle?: ReactNode
  /** Vạch home indicator 134×5 ở đáy (artboard điện thoại). */
  homeIndicator?: boolean
}

export function Sheet({ open, onClose, title, label, children, height = 640, style, xstyle, bodyStyle, grab = true, role = 'dialog', size = 'lg', subtitle, homeIndicator }: SheetProps) {
  const [mounted, setMounted] = useState(open)
  const [distance, setDistance] = useState(0)
  const ref = useRef<HTMLElement>(null)
  const start = useRef<number | null>(null)
  const titleId = useId()
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  const t = getKitLabels()
  useEffect(() => {
    if (open) { setMounted(true); setDistance(0); return }
    const timer = setTimeout(() => setMounted(false), 260)
    return () => clearTimeout(timer)
  }, [open])
  useEffect(() => { ref.current?.querySelector('.m-sheet-body')?.scrollTo({ top: 0 }) }, [title, mounted])
  // Sheet là một lớp: Esc đóng sheet trên cùng, không đóng hộp thoại bên dưới (17-kit, chốt 25/10).
  useEscapeLayer(mounted, () => closeRef.current())
  useEffect(() => {
    if (!mounted) return
    const previous = document.activeElement as HTMLElement | null
    if (scrollLocks++ === 0) originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    ref.current?.focus()
    const key = (event: KeyboardEvent) => {
      const sheets = document.querySelectorAll('.m-sheet')
      if (sheets[sheets.length - 1] !== ref.current) return
      if (event.key !== 'Tab') return
      const focusable = ref.current?.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), textarea:not(:disabled), select:not(:disabled), [tabindex="0"]')
      if (!focusable?.length) { event.preventDefault(); return }
      const first = focusable[0], last = focusable[focusable.length - 1]
      if (event.shiftKey && (document.activeElement === first || document.activeElement === ref.current)) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && (document.activeElement === last || document.activeElement === ref.current)) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', key)
    return () => {
      if (--scrollLocks === 0) { document.body.style.overflow = originalOverflow; previous?.focus() }
      document.removeEventListener('keydown', key)
    }
  }, [mounted])
  if (!mounted) return null
  return createPortal(<div className={`m-overlay${!open ? ' m-out' : ''}`}>
    <div className="m-scrim" onClick={onClose} />
    <section ref={ref} className={`m-sheet${size === 'md' ? ' m-sheet--md' : ''} ${stylex.props(xstyle).className ?? ''}`} role={role} aria-modal="true" aria-labelledby={title ? titleId : undefined} aria-label={title ? undefined : label ?? t.sheet.defaultLabel} tabIndex={-1}
      style={{ ...stylex.props(xstyle).style, height, ...(distance ? { transform: `translateY(${distance}px)`, animation: open ? 'none' : undefined } : {}), ...style }}>
      {grab && <div className="m-drag" aria-label={t.sheet.grabHandle}
        onPointerDown={(event) => { start.current = event.clientY; event.currentTarget.setPointerCapture(event.pointerId) }}
        onPointerMove={(event) => { if (start.current !== null) setDistance(Math.max(0, event.clientY - start.current)) }}
        onPointerUp={(event) => {
          const dy = start.current === null ? 0 : Math.max(0, event.clientY - start.current)
          start.current = null
          if (sheetCloseThreshold(dy, ref.current?.getBoundingClientRect().height ?? 0)) onClose()
          else setDistance(0)
        }}
        onPointerCancel={() => { start.current = null; setDistance(0) }}><span /></div>}
      {title && <div className="m-sheet-title"><h2 id={titleId}>{title}</h2>{subtitle != null && <span className="m-sheet-sub">{subtitle}</span>}</div>}
      <div className={`m-sheet-body ${stylex.props(bodyStyle).className ?? ''}`} style={stylex.props(bodyStyle).style}>{children}</div>
      {homeIndicator && <div className="m-sheet-home" aria-hidden="true"><span /></div>}
    </section>
  </div>, document.body)
}
