import { cloneElement, isValidElement, useEffect, useId, useRef, useState, type HTMLAttributes, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Kbd } from './Button'
import { Icon } from './Icon'
import './kit.css'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

// ─── Lớp Esc (17-kit, chốt 25/10) ────────────────────────────────────────
// Esc bấm MỘT lần chỉ đóng lớp trên cùng: menu, popover, sheet, hộp thoại. Bản nháp đang gõ
// trong lớp đó được giữ nguyên, không hỏi. Chỉ khi không còn lớp nào mở thì Esc mới tới màn
// phía sau (Bàn viết: Tạm dừng).
// Lớp xếp theo thứ tự mở; listener ở pha capture trên `document` nên lớp trên cùng luôn thắng
// mọi handler Esc cũ (window / document bubble) của màn phía sau, và các handler đó không chạy.
type EscapeLayer = { id: symbol; close: () => void }
const escapeLayers: EscapeLayer[] = []

function onEscapeKey(event: KeyboardEvent) {
  if (event.key !== 'Escape' || event.defaultPrevented) return
  const top = escapeLayers[escapeLayers.length - 1]
  if (!top) return
  event.preventDefault()
  event.stopPropagation()
  top.close()
}

/** Đăng ký một lớp Esc đang mở; trả hàm gỡ. Lớp mở sau nằm trên. */
export function pushEscapeLayer(close: () => void): () => void {
  const layer: EscapeLayer = { id: Symbol('esc-layer'), close }
  escapeLayers.push(layer)
  if (escapeLayers.length === 1) window.addEventListener('keydown', onEscapeKey, true)
  return () => {
    const at = escapeLayers.findIndex((entry) => entry.id === layer.id)
    if (at >= 0) escapeLayers.splice(at, 1)
    if (!escapeLayers.length) window.removeEventListener('keydown', onEscapeKey, true)
  }
}

/** Đăng ký lớp khi `active`; Esc gọi bản `close` mới nhất. */
export function useEscapeLayer(active: boolean, close: () => void): void {
  const latest = useRef(close)
  latest.current = close
  useEffect(() => {
    if (!active) return
    return pushEscapeLayer(() => latest.current())
  }, [active])
}

const sx = stylex.create({
  tip: {
    position: 'fixed', zIndex: 200, backgroundColor: color['--color-surface-7'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'],
    paddingTop: 5, paddingBottom: 5, paddingInline: 8, borderRadius: 7,
    fontSize: 11.5, color: color['--color-fg'], whiteSpace: 'nowrap',
    boxShadow: kit['--kit-shadow-tt'], pointerEvents: 'none',
    display: 'flex', alignItems: 'center', gap: 6,
    transform: 'translateX(-50%)',
    animationName: { default: 'kit-tt-top', [NO_MOTION]: 'none' },
    animationDuration: '.12s', animationTimingFunction: 'ease', animationFillMode: 'both',
  },
  tipBottom: { animationName: { default: 'kit-tt-bottom', [NO_MOTION]: 'none' } },
  tipClosing: {
    animationName: { default: 'kit-tt-top-out', [NO_MOTION]: 'none' },
    animationDuration: '.12s', animationTimingFunction: 'ease', animationFillMode: 'both',
  },
  tipClosingBottom: { animationName: { default: 'kit-tt-bottom-out', [NO_MOTION]: 'none' } },
  tipAfterTop: {
    '::after': {
      content: "''", position: 'absolute', left: '50%', top: '100%', marginLeft: -4,
      borderWidth: 4, borderStyle: 'solid', borderColor: 'transparent',
      borderTopColor: color['--color-surface-7'],
    },
  },
  tipAfterBottom: {
    '::after': {
      content: "''", position: 'absolute', left: '50%', top: 'auto', bottom: '100%', marginLeft: -4,
      borderWidth: 4, borderStyle: 'solid', borderColor: 'transparent',
      borderTopColor: 'transparent', borderBottomColor: color['--color-surface-7'],
    },
  },
  anchor: { display: 'inline-flex' },
  anchorBlock: { display: 'flex', width: '100%' },
  popWrap: { position: 'relative', display: 'inline-flex' },
  popWrapOpen: { zIndex: 30 },
  popBtn: {
    width: 26, height: 26, borderRadius: '50%', borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent',
    color: color['--color-fg-3'], display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  pop: {
    position: 'absolute', top: 'calc(100% + 8px)', width: 280, padding: 14,
    display: 'flex', flexDirection: 'column', gap: 8,
  },
  popLeft: { left: -12, transformOrigin: 'top left' },
  popRight: { right: -12, transformOrigin: 'top right' },
})

/** `side` ép tooltip nằm trên/dưới; mặc định tự chọn (trên, nếu sát mép trên thì dưới). */
export function Tooltip({ children, content, kbd, block = false, asChild = false, side }: { children: ReactNode; content: ReactNode; kbd?: ReactNode; block?: boolean; asChild?: boolean; side?: 'top' | 'bottom' }) {
  const ref = useRef<HTMLSpanElement>(null)
  const anchor = useRef<HTMLElement | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [position, setPosition] = useState<{ x: number; y: number; side: 'top' | 'bottom' } | null>(null)
  const [closing, setClosing] = useState(false)
  const id = useId()
  // Artboard: hiện sau 0,4 s (transition-delay), rời là mờ đi trong .12 s rồi gỡ.
  const close = () => {
    clearTimeout(timer.current)
    if (!position) return
    setClosing(true)
    timer.current = setTimeout(() => { setPosition(null); setClosing(false) }, 120)
  }
  useEffect(() => () => clearTimeout(timer.current), [])
  const show = () => {
    clearTimeout(timer.current)
    setClosing(false)
    timer.current = setTimeout(() => {
      const r = (anchor.current ?? ref.current?.firstElementChild ?? ref.current)?.getBoundingClientRect()
      const placement = side ?? (r && r.top >= 44 ? 'top' : 'bottom')
      if (r) setPosition({ x: Math.min(window.innerWidth - 100, Math.max(100, r.left + r.width / 2)), y: placement === 'top' ? r.top - 8 : r.bottom + 8, side: placement })
    }, 400)
  }
  const tipSx = position && stylex.props(
    sx.tip,
    position.side === 'bottom' ? [sx.tipBottom, sx.tipAfterBottom] : sx.tipAfterTop,
    closing && [sx.tipClosing, position.side === 'bottom' && sx.tipClosingBottom],
  )
  const tip = position && tipSx && createPortal(<span id={id} role="tooltip" className={tipSx.className} style={{ ...tipSx.style, left: position.x, top: position.y, translate: position.side === 'top' ? '0 -100%' : undefined }}>{content}{kbd && <Kbd style={{ fontSize: 10 }}>{kbd}</Kbd>}</span>, document.body)
  if (asChild && isValidElement<HTMLAttributes<HTMLElement>>(children)) {
    const props = children.props
    return <>{cloneElement(children, {
      onPointerEnter: (event) => { props.onPointerEnter?.(event); anchor.current = event.currentTarget; show() },
      onPointerLeave: (event) => { props.onPointerLeave?.(event); close() },
      onFocus: (event) => { props.onFocus?.(event); anchor.current = event.currentTarget; show() },
      onBlur: (event) => { props.onBlur?.(event); close() },
      onKeyDown: (event) => { props.onKeyDown?.(event); if (event.key === 'Escape') close() },
      'aria-describedby': [props['aria-describedby'], position ? id : undefined].filter(Boolean).join(' ') || undefined,
    })}{tip}</>
  }
  const wrap = stylex.props(block ? sx.anchorBlock : sx.anchor)
  return <span ref={ref} className={wrap.className} style={wrap.style} onMouseEnter={show} onMouseLeave={close} onFocus={show} onBlur={close} onKeyDown={(e) => { if (e.key === 'Escape') close() }} aria-describedby={position ? id : undefined}>{children}{tip}</span>
}

/**
 * Popover giải thích: bấm để mở (không phải rê chuột), có thể chứa liên kết. Artboard: nút tròn 26 (icon info 16 --color-fg-3),
 * hộp 280 neo lệch trái -12, padding 14, các khối xếp dọc cách 8. `align="right"` neo mép phải khi nút sát cạnh phải màn hình.
 */
export function InfoPopover({ children, label = getKitLabels().overlays.info, trigger, align = 'left' }: { children: ReactNode; label?: string; trigger?: ReactNode; align?: 'left' | 'right' }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLSpanElement>(null)
  const id = useId()
  // Popover là một lớp: Esc đóng nó trước, không đóng hộp thoại bên dưới.
  useEscapeLayer(open, () => { setOpen(false); root.current?.querySelector('button')?.focus() })
  useEffect(() => {
    if (!open) return
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('pointerdown', outside)
    return () => { document.removeEventListener('pointerdown', outside) }
  }, [open])
  const wrap = stylex.props(sx.popWrap, open && sx.popWrapOpen)
  const pop = stylex.props(sx.pop, align === 'left' ? sx.popLeft : sx.popRight)
  return <span ref={root} className={wrap.className} style={wrap.style}><button type="button" className={`hov-ghost ${stylex.props(sx.popBtn).className}`} aria-label={label} aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(!open)}>{trigger ?? <Icon name="info-circle" size={16} />}</button>{open && <span id={id} role="dialog" aria-label={label} className={`kit-pop ${pop.className}`} style={pop.style}>{children}</span>}</span>
}
