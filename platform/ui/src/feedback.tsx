// Spinner/Shimmer/ProgressBar/Skeleton/EmptyState — chuyển động theo 03-tokens.md.
import type { CSSProperties, ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, tint } from './tokens.stylex'
import { Icon } from './Icon'
import './kit.css'

const spin = stylex.create({
  base: {
    display: 'inline-block', boxSizing: 'border-box', verticalAlign: 'middle',
    borderRadius: '50%', flex: '0 0 auto',
    borderWidth: 2, borderStyle: 'solid', borderColor: kit['--kit-spin-track'],
    animationName: 'ai-spin', animationDuration: '.9s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
  },
  size: (size: string, top: string) => ({ width: size, height: size, borderTopColor: top }),
})
export function Spinner({ size = 14, color, style }: { size?: number; color?: string; style?: CSSProperties }) {
  const sx = stylex.props(spin.base, spin.size(`${size}px`, color ?? 'var(--color-fg)'))
  return <span aria-hidden="true" className={sx.className} style={{ ...sx.style, ...style }} />
}

/** Chữ chạy sáng (ai-shimmer) — “Đang suy nghĩ”, tên tool đang chạy */
export function Shimmer({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <span className="shimmer" style={style}>{children}</span>
}

const bar = stylex.create({
  track: (height: string) => ({ height, borderRadius: height, backgroundColor: color['--color-surface-5'], overflow: 'hidden' }),
  fill: (width: string, height: string, fill: string) => ({ width, height: '100%', borderRadius: height }),
})

export function ProgressBar({ value, tone = 'neutral', height = 3, style }: {
  /** 0–100 */
  value: number; tone?: 'neutral' | 'writer' | 'info' | 'ok' | 'warn' | 'err'; height?: number; style?: CSSProperties
}) {
  const fill = tone === 'writer' ? 'var(--grad-writer-progress)'
    : tone === 'ok' ? 'var(--color-ok)'
    : tone === 'warn' ? 'var(--color-warn)'
    : tone === 'err' ? 'var(--color-err)'
    : tone === 'info' ? 'var(--color-info)'
    : 'var(--color-fg)'
  const outer = stylex.props(bar.track(`${height}px`))
  const inner = stylex.props(bar.fill(`${Math.min(100, Math.max(0, value))}%`, `${height}px`, fill))
  return (
    <div role="progressbar" aria-valuenow={Math.round(value)} aria-valuemin={0} aria-valuemax={100}
      className={outer.className} style={{ ...outer.style, ...style }}>
      <div className={inner.className} style={{ ...inner.style, background: fill }} />
    </div>
  )
}

const skel = stylex.create({
  base: (width: string, height: string, radius: string) => ({
    width, height, borderRadius: radius,
    backgroundImage: kit['--kit-grad-skel'],
    backgroundSize: '200% 100%',
    animationName: 'kit-skel', animationDuration: '1.4s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
  }),
})

/** Khối xương cá khi đang tải — Kit.dc.html `.skel`: --kit-grad-skel (40%), 1,4 s linear, bo 6 */
export function Skeleton({ width = '100%', height = 14, radius = 6, style }: {
  width?: number | string; height?: number | string; radius?: number; style?: CSSProperties
}) {
  const sx = stylex.props(skel.base(typeof width === 'number' ? `${width}px` : width, typeof height === 'number' ? `${height}px` : height, `${radius}px`))
  return <div aria-hidden="true" className={sx.className} style={{ ...sx.style, ...style }} />
}

const empty = stylex.create({
  // Kit.dc.html › “Đang tải & trống”: khung nét đứt, ô icon 40, tiêu đề 13,5/500.
  frame: {
    padding: 18, borderRadius: 12, borderWidth: 1, borderStyle: 'dashed', borderColor: kit['--kit-wash-12'],
    display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, textAlign: 'center',
  },
  // Khối trống cỡ lớn thay cả vùng danh sách (Explore/MExplore › trạng thái danh sách).
  frameLg: {
    padding: { default: '44px 16px', '@media (max-width: 767px)': '36px 16px' },
    gap: 10, borderRadius: { default: 14, '@media (max-width: 767px)': 16 }, borderWidth: 1.5,
  },
  iconChip: {
    width: 40, height: 40, borderRadius: 12, backgroundColor: tint['--tint-accent'], color: color['--color-accent'],
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  // Viderse đổi “đỏ phá huỷ” thành cam (--vv-danger*); Noverse không có biến đó → tông lỗi của Kit.
  iconChipDanger: {
    backgroundColor: 'var(--vv-danger-8, var(--kit-err-8))',
    color: 'var(--vv-danger, var(--color-err))',
  },
  titleFrame: { fontSize: 13.5, fontWeight: 500, color: color['--color-fg'] },
  titleFrameLg: { fontSize: { default: 14, '@media (max-width: 767px)': 15 } },
  hintFrame: { fontSize: 12, color: kit['--kit-fg-icon'] },
  hintFrameLg: {
    fontSize: { default: 12.5, '@media (max-width: 767px)': 13.5 },
    lineHeight: 1.55, maxWidth: 460,
  },
  // Trạng thái trống không có trong artboard của màn: giữ dáng cũ (icon trần, không khung).
  inline: {
    paddingTop: 28, paddingBottom: 28, paddingInline: 0, display: 'flex', flexDirection: 'column',
    alignItems: 'center', gap: 8, textAlign: 'center', color: color['--color-fg-4'],
  },
  titleInline: { fontSize: 13 },
  hintInline: { fontSize: 12 },
  actions: { display: 'flex', gap: 6 },
})

/**
 * `frame` = khối trống của Kit.dc.html (khung nét đứt + ô icon 40). `inline` = dáng cũ, dùng cho màn mà
 * artboard không vẽ khối trống riêng. Mặc định `inline` để không đổi hình của màn khác.
 */
export function EmptyState({ icon = 'magic-star', title, hint, action, style, variant = 'inline', size = 'md', tone = 'accent' }: {
  icon?: string; title: ReactNode; hint?: ReactNode; action?: ReactNode; style?: CSSProperties
  /** 'frame' = khối trống của Kit; 'inline' = icon + chữ, không khung (mặc định). */
  variant?: 'frame' | 'inline'
  /** `lg` = khối thay cả vùng danh sách (Explore/MExplore); chỉ áp cho `frame`. */
  size?: 'md' | 'lg'
  /** Tông ô icon; `danger` dùng màu lỗi của nền tảng. */
  tone?: 'accent' | 'danger'
}) {
  const framed = variant === 'frame'
  const large = framed && size === 'lg'
  const sx = stylex.props(framed ? empty.frame : empty.inline, large && empty.frameLg)
  const chip = stylex.props(empty.iconChip, tone === 'danger' && empty.iconChipDanger)
  return (
    <div className={sx.className} style={{ ...sx.style, ...style }}>
      {framed
        ? <span aria-hidden="true" className={chip.className} style={chip.style}><Icon name={icon} size={19} /></span>
        : <Icon name={icon} size={22} />}
      <span {...stylex.props(framed ? empty.titleFrame : empty.titleInline, large && empty.titleFrameLg)}>{title}</span>
      {hint && <span {...stylex.props(framed ? empty.hintFrame : empty.hintInline, large && empty.hintFrameLg)}>{hint}</span>}
      {action && <span {...stylex.props(empty.actions)}>{action}</span>}
    </div>
  )
}
