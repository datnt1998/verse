// Button/Kbd/Avatar theo 04-components.md + artboard inline styles.
import { forwardRef, type ButtonHTMLAttributes, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Tooltip } from './Overlays'
import { Spinner } from './feedback'
import './kit.css'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const W7 = line['--line-2'] // .07 trắng — hover mờ của ghost

const styles = stylex.create({
  base: {
    height: 36, paddingInline: 14, borderRadius: 10, fontSize: 13, fontWeight: 500,
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 7,
    whiteSpace: 'nowrap', borderWidth: 1, borderStyle: 'solid', borderColor: 'transparent',
    transition: { default: 'background .15s, box-shadow .2s, transform .08s, border-color .15s', [NO_MOTION]: 'none' },
    position: 'relative',
  },
  enabled: {
    transform: { ':active': 'scale(.98)' },
  },
  disabled: { opacity: .42, cursor: 'not-allowed' },
  primary: { backgroundColor: color['--color-fg'], color: color['--color-bg'], fontWeight: 600 },
  primaryHover: { backgroundColor: { default: color['--color-fg'], ':hover': 'white' } },
  ghost: { backgroundColor: 'transparent', color: color['--color-fg'], borderColor: kit['--kit-wash-12'] },
  quiet: { backgroundColor: 'transparent', color: color['--color-fg-3'] },
  ghostHover: { backgroundColor: { default: 'transparent', ':hover': W7 } },
  quietHover: { color: { default: color['--color-fg-3'], ':hover': color['--color-fg'] } },
  danger: { backgroundColor: kit['--kit-err-deep'], color: kit['--kit-err-deep-fg'], fontWeight: 600 },
  dangerHover: {
    backgroundColor: { default: kit['--kit-err-deep'], ':hover': kit['--kit-err-deep-hi'] },
    boxShadow: { default: 'none', ':hover': kit['--kit-ring-danger'] },
  },
  dsoft: { backgroundColor: kit['--kit-err-12'], color: kit['--kit-err-text'], borderColor: kit['--kit-errb-28'] },
  /* `.ghost.btn-warn` của artboard: nền trong, viền cam .45, chữ cam nhạt, hover
     nền cam .08 — dùng cho nút xoá nhóm/nhóm phụ (Settings › Lưu trữ). */
  warnGhost: { backgroundColor: 'transparent', color: kit['--kit-err-ghost-ink'], borderColor: kit['--kit-errb-45'] },
  warnGhostHover: { backgroundColor: { default: 'transparent', ':hover': kit['--kit-err-8'] } },
  dsoftHover: { backgroundColor: { default: kit['--kit-err-12'], ':hover': kit['--kit-err-20'] } },
  icon: {
    width: 36, height: 36, paddingInline: 0, borderWidth: 0, borderStyle: 'none',
    borderColor: 'currentColor',
    fontSize: 14, fontWeight: 400, backgroundColor: 'transparent', color: color['--color-fg-3'],
  },
  /* Nút chỉ có icon vẫn VUÔNG theo cỡ: 44 (lg) và 48 (xl) — vùng chạm điện thoại
     của artboard MKit/MExplore. Trước đây `icon` khoá 36×36 nên size bị bỏ qua. */
  iconSm: { width: 30, borderRadius: 8 },
  iconLg: { width: 44, borderRadius: 12 },
  iconXl: { width: 48, borderRadius: 13 },
  sm: { height: 30, paddingInline: 10, fontSize: 12, borderRadius: 8 },
  /* Cỡ lớn của mobile/CTA cuối thẻ: 44px, chữ 14, góc 12 (artboard .btn 44px). */
  lg: { height: kit['--kit-touch-min'], minHeight: kit['--kit-touch-min'], paddingInline: 16, fontSize: 14, borderRadius: 12 },
  /* Cỡ gọn của desktop trong Cài đặt: 32px, pad 12, góc 8, chữ 13 (artboard
     Settings `.btn`; Onboard dùng cùng chiều cao cho nút ghost). */
  compact: { height: 32, paddingInline: 12, borderRadius: 8, fontSize: 13, fontWeight: 400 },
  /* CTA chính của điện thoại: 48px, pad 16, góc 13, chữ 15/600, gap 8
     (artboard MChannel/MCreate/MCompVideo). */
  xl: { height: 48, paddingInline: 16, borderRadius: 13, fontSize: 15, fontWeight: 600, gap: 8 },
  /* CTA dính đáy của điện thoại chiếm trọn bề ngang (`.vv-btn--m-lg` artboard MKit). */
  block: { width: '100%' },
  cell: { display: 'inline-grid', alignItems: 'center', justifyItems: 'center', minWidth: 0, maxWidth: '100%' },
  slot: { gridArea: '1 / 1', display: 'inline-flex', alignItems: 'center', gap: 7, minWidth: 0, maxWidth: '100%' },
  slotCenter: { gridArea: '1 / 1', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 7, minWidth: 0, maxWidth: '100%' },
  hidden: { visibility: 'hidden' },
  tick: {
    color: color['--color-ok'],
    animationName: { default: 'kit-tick', [NO_MOTION]: 'none' },
    animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1.5)', animationFillMode: 'both',
  },
  kbd: {
    fontSize: 11, color: color['--color-fg-3'], paddingTop: 1, paddingBottom: 1,
    paddingInline: 5, borderRadius: 4, borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], backgroundColor: kit['--kit-surface-15'],
  },
})

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'ghost' | 'quiet' | 'icon' | 'icon-only' | 'dsoft' | 'danger' | 'warnGhost'
  size?: 'sm' | 'md' | 'compact' | 'lg' | 'xl'
  /** chiếm trọn bề ngang — dùng cho CTA 48px dính đáy (MKit `.vv-btn--m-lg`) */
  block?: boolean
  /** hiển thị phím tắt bên phải (vd 'Esc') */
  kbd?: ReactNode
  icon?: ReactNode
  active?: boolean
  pending?: boolean
  success?: boolean
  pendingLabel?: ReactNode
  successLabel?: ReactNode
  tooltip?: string
  /** phím tắt hiện trong tooltip (vd '⌘ ,') */
  tooltipKbd?: ReactNode
  /** App styles compose after the built-in variant and interaction styles. */
  xstyle?: stylex.StyleXStyles
}

/** Nút khoá bằng `aria-disabled` vẫn nhận focus (đọc được lý do qua `aria-describedby`)
 *  nhưng bấm không làm gì — kể cả submit form. */
const blockLocked = (event: { preventDefault: () => void }) => event.preventDefault()

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'ghost', size = 'md', block, kbd, icon, active, pending, success, pendingLabel, successLabel, tooltip, tooltipKbd, style, className, xstyle, children, disabled, title, ...rest }, ref,
) {
  const iconOnly = variant === 'icon' || variant === 'icon-only'
  // Spinner của nút đang chạy theo Kit: nút sáng → vòng đen mờ; đỏ → vòng trắng; ghost → theo chữ.
  const spin = variant === 'primary' ? { track: 'var(--kit-spin-dark)', top: 'var(--color-bg)' } : variant === 'dsoft' || variant === 'danger' ? { track: 'var(--kit-wash-25)', top: 'white' } : { track: undefined, top: 'currentColor' }
  const label = tooltip ?? title ?? rest['aria-label']
  const off = disabled || pending
  const locked = rest['aria-disabled'] === true || rest['aria-disabled'] === 'true'
  const kind = iconOnly ? 'icon' : variant
  const sx = stylex.props(
    styles.base,
    // `compact` đứng trước variant: nó mang fw 400 của `.btn`, còn `primary`/`danger`
    // phải giữ 600 của mình (`.btn.primary` của artboard). Đặt sau variant là đè mất 600.
    size === 'compact' && styles.compact,
    kind === 'primary' && styles.primary,
    kind === 'ghost' && styles.ghost,
    kind === 'quiet' && styles.quiet,
    kind === 'danger' && styles.danger,
    kind === 'dsoft' && styles.dsoft,
    kind === 'warnGhost' && styles.warnGhost,
    kind === 'icon' && styles.icon,
    size === 'sm' && styles.sm,
    size === 'lg' && styles.lg,
    size === 'xl' && styles.xl,
    iconOnly && size === 'sm' && styles.iconSm,
    iconOnly && size === 'lg' && styles.iconLg,
    iconOnly && size === 'xl' && styles.iconXl,
    block && styles.block,
    off || locked ? styles.disabled : [
      styles.enabled,
      kind === 'primary' && styles.primaryHover,
      (kind === 'ghost' || kind === 'icon') && styles.ghostHover,
      kind === 'quiet' && styles.quietHover,
      kind === 'danger' && styles.dangerHover,
      kind === 'dsoft' && styles.dsoftHover,
      kind === 'warnGhost' && styles.warnGhostHover,
    ],
    xstyle,
  )
  const button = <button ref={ref} type="button" {...rest} disabled={off} onClick={locked ? blockLocked : rest.onClick} aria-busy={pending || undefined}
    aria-label={rest['aria-label'] ?? (iconOnly ? label : undefined)}
    className={[sx.className, className].filter(Boolean).join(' ')}
    style={{ ...sx.style, ...(active ? { background: 'var(--color-surface-5)', color: 'var(--color-fg)' } : {}), ...style }}>
    <span {...stylex.props(styles.cell)}>
      <span {...stylex.props(styles.slot, (pending || success) && styles.hidden)}>{icon}{children}{kbd != null && <Kbd>{kbd}</Kbd>}</span>
      {(pending || success) && <span {...stylex.props(styles.slotCenter)}>{pending ? <Spinner size={13} color={spin.top} style={spin.track ? { borderTopColor: spin.top, borderColor: spin.track } : undefined} /> : <span {...stylex.props(styles.tick)}>✓</span>}{pending ? (iconOnly ? null : pendingLabel ?? children) : successLabel}</span>}
    </span>
  </button>
  return label && (iconOnly || tooltip || title) ? <Tooltip content={label} kbd={tooltipKbd} asChild>{button}</Tooltip> : button
})

export function Kbd({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  const sx = stylex.props(styles.kbd)
  return <span className={`mono num ${sx.className}`} style={{ ...sx.style, ...style }}>{children}</span>
}

const avatar = stylex.create({
  root: {
    borderRadius: '50%', flex: '0 0 auto',
    backgroundColor: color['--color-surface-6'],
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    fontWeight: 600, color: color['--color-fg'],
  },
  size: (size: string, fontSize: string) => ({ width: size, height: size, fontSize }),
})

/** Avatar chữ cái — nền trung tính, không dùng gradient agent */
export function Avatar({ name, size = 30, style, label }: { name: string; size?: number; style?: CSSProperties; label?: string }) {
  const initial = (name.trim()[0] ?? '?').toUpperCase()
  const sx = stylex.props(avatar.root, avatar.size(`${size}px`, `${Math.round(size * 0.4)}px`))
  return (
    <span aria-label={label} aria-hidden={label ? undefined : true} role={label ? 'img' : undefined}
      className={sx.className} style={{ ...sx.style, ...style }}>{initial}</span>
  )
}
