// Chip/Badge/Card — 04-components.md + artboard.
import type { CSSProperties, ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, tint, kit } from './tokens.stylex'

export type Tone = 'neutral' | 'accent' | 'ok' | 'warn' | 'err' | 'info'

/** Chuỗi CSS (biến) theo tone — Toast dùng cho icon & viền inline. */
export const TONE: Record<Tone, { bg: string; bd: string; fg: string }> = {
  neutral: { bg: 'var(--kit-surface-15)', bd: 'var(--line-2)', fg: 'var(--color-fg-4)' },
  accent: { bg: 'var(--tint-accent)', bd: 'var(--kit-acc-30)', fg: 'var(--color-accent-soft)' },
  ok: { bg: 'var(--tint-ok)', bd: 'var(--kit-ok-30)', fg: 'var(--color-ok)' },
  warn: { bg: 'var(--tint-warn)', bd: 'var(--kit-warn-30)', fg: 'var(--color-warn)' },
  err: { bg: 'var(--tint-err)', bd: 'var(--kit-errf-30)', fg: 'var(--color-err)' },
  info: { bg: 'var(--tint-info)', bd: 'var(--kit-info-20)', fg: 'var(--color-info)' },
}

const chip = stylex.create({
  base: {
    display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12, whiteSpace: 'nowrap',
    borderWidth: 1, borderStyle: 'solid',
  },
  sm: { height: 26, borderRadius: 7 },
  smIcon: { paddingInlineStart: 7, paddingInlineEnd: 9 },
  smPlain: { paddingInline: 9 },
  md: {
    height: 28, paddingInline: 10, borderRadius: 8,
    backgroundColor: color['--color-surface-1'], borderColor: line['--line-4'], color: color['--color-fg-2'],
  },
  neutral: { backgroundColor: kit['--kit-surface-15'], borderColor: line['--line-2'], color: color['--color-fg-4'] },
  accent: { backgroundColor: tint['--tint-accent'], borderColor: kit['--kit-acc-30'], color: color['--color-accent-soft'] },
  ok: { backgroundColor: tint['--tint-ok'], borderColor: kit['--kit-ok-30'], color: color['--color-ok'] },
  warn: { backgroundColor: tint['--tint-warn'], borderColor: kit['--kit-warn-30'], color: color['--color-warn'] },
  err: { backgroundColor: tint['--tint-err'], borderColor: kit['--kit-errf-30'], color: color['--color-err'] },
  info: { backgroundColor: tint['--tint-info'], borderColor: kit['--kit-info-30'], color: color['--color-info'] },
  mdIcon: { display: 'inline-flex', color: kit['--kit-fg-icon'] },
  value: { fontWeight: 500 },
  mdValue: { color: color['--color-fg'] },
  smValue: { color: color['--color-fg'] },
})

export function Chip({ icon, label, value, tone = 'neutral', valueColor, style, size = 'sm' }: {
  icon?: ReactNode; label: ReactNode; value?: ReactNode
  tone?: Tone; valueColor?: string; style?: CSSProperties
  /** 'sm' = chip tiêu đề 26 px; 'md' = chip khoá-giá trị của Kit (28 px, bo 8, nền surface-1, icon --kit-fg-icon). */
  size?: 'sm' | 'md'
}) {
  const md = size === 'md'
  const sx = stylex.props(chip.base, md ? chip.md : [chip.sm, icon ? chip.smIcon : chip.smPlain, chip[tone]])
  const v = stylex.props(chip.value, md ? chip.mdValue : chip.smValue)
  return (
    <span className={sx.className} style={{ ...sx.style, ...style }}>
      {md && icon ? <span {...stylex.props(chip.mdIcon)}>{icon}</span> : icon}
      {label}
      {value !== undefined && value !== null && <b className={v.className} style={{ ...v.style, ...(valueColor ? { color: valueColor } : {}) }}>{value}</b>}
    </span>
  )
}

export type BadgeTone = Tone | 'running'
/** Huy hiệu của Kit: nền, chữ và chấm 6 px riêng cho từng trạng thái (Kit.dc.html › badges). */
const badge = stylex.create({
  base: {
    height: 22, paddingInline: 8, display: 'inline-flex', alignItems: 'center', gap: 6,
    borderRadius: 6, fontSize: 11.5, whiteSpace: 'nowrap',
  },
  running: { backgroundColor: kit['--kit-run-14'], color: kit['--kit-badge-run'] },
  ok: { backgroundColor: kit['--kit-ok-12'], color: color['--color-ok'] },
  warn: { backgroundColor: kit['--kit-warn-12'], color: color['--color-warn'] },
  err: { backgroundColor: kit['--kit-err-12'], color: kit['--kit-err-soft'] },
  neutral: { backgroundColor: line['--line-1'], color: color['--color-fg-3'] },
  accent: { backgroundColor: kit['--kit-acc-12'], color: kit['--kit-badge-run'] },
  info: { backgroundColor: kit['--kit-info-12'], color: color['--color-info'] },
  dot: { width: 6, height: 6, borderRadius: '50%' },
  dotRunning: { backgroundColor: kit['--kit-switch-on'] },
  dotOk: { backgroundColor: color['--color-ok'] },
  dotWarn: { backgroundColor: color['--color-warn'] },
  dotErr: { backgroundColor: kit['--kit-err-strong'] },
  dotNeutral: { backgroundColor: kit['--kit-fg-off'] },
  dotAccent: { backgroundColor: color['--color-accent'] },
  dotInfo: { backgroundColor: kit['--kit-info-dot'] },
})

const DOT: Record<BadgeTone, keyof typeof badge> = {
  running: 'dotRunning', ok: 'dotOk', warn: 'dotWarn', err: 'dotErr',
  neutral: 'dotNeutral', accent: 'dotAccent', info: 'dotInfo',
}

export function Badge({ tone = 'neutral', children, style, dot = false, dotColor, bg, fg, title }: {
  tone?: BadgeTone; children: ReactNode; style?: CSSProperties; dot?: boolean; dotColor?: string; bg?: string; fg?: string
  /** Nhãn bị cắt “…” ở lề hẹp → tooltip giữ nguyên chữ đầy đủ. */
  title?: string
}) {
  const sx = stylex.props(badge.base, badge[tone])
  const d = stylex.props(badge.dot, badge[DOT[tone]])
  return (
    <span className={sx.className} title={title} style={{ ...sx.style, ...(bg ? { background: bg } : {}), ...(fg ? { color: fg } : {}), ...style }}>
      {dot && <span aria-hidden="true" className={d.className} style={{ ...d.style, ...(dotColor ? { background: dotColor } : {}) }} />}{children}
    </span>
  )
}

export function Card({ children, padding = 12, radius = 16, style, className, role, label }: {
  children: ReactNode; padding?: CSSProperties['padding']; radius?: number
  style?: CSSProperties; className?: string; role?: string; label?: string
}) {
  return (
    <div className={`card${className ? ` ${className}` : ''}`} role={role} aria-label={label}
      style={{ borderRadius: radius, padding, ...style }}>{children}</div>
  )
}

const cardTitle = stylex.create({
  root: {
    margin: 0, display: 'flex', alignItems: 'center', gap: 8,
    fontSize: 13, fontWeight: 500, color: color['--color-fg-3'],
  },
  label: { flex: 1, minWidth: 0 },
})

export function CardTitle({ icon, children, right, style }: { icon?: ReactNode; children: ReactNode; right?: ReactNode; style?: CSSProperties }) {
  const sx = stylex.props(cardTitle.root)
  return (
    <h3 className={sx.className} style={{ ...sx.style, ...style }}>
      {icon}
      <span {...stylex.props(cardTitle.label)}>{children}</span>
      {right}
    </h3>
  )
}
