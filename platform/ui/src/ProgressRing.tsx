import { useId, useSyncExternalStore, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from './tokens.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const spin = stylex.keyframes({ from: { transform: 'rotate(-90deg)' }, to: { transform: 'rotate(270deg)' } })
const styles = stylex.create({
  root: { position: 'relative', display: 'inline-flex', flexShrink: 0, alignItems: 'center', justifyContent: 'center' },
  svg: { position: 'absolute', inset: 0, display: 'block', overflow: 'visible', transform: 'rotate(-90deg)', pointerEvents: 'none' },
  spin: { animationName: { default: spin, [NO_MOTION]: 'none' }, animationDuration: '1.1s', animationTimingFunction: 'linear', animationIterationCount: 'infinite' },
  arc: { transition: { default: 'stroke-dasharray .5s cubic-bezier(.2,.9,.25,1), stroke .25s', [NO_MOTION]: 'none' } },
  center: { position: 'relative', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', lineHeight: 1.1 },
})

const motionQuery = () => typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : null
const reducedMotion = () => motionQuery()?.matches ?? false
const serverMotion = () => false
function subscribeMotion(changed: () => void) {
  const media = motionQuery()
  media?.addEventListener('change', changed)
  return () => media?.removeEventListener('change', changed)
}

type Props = {
  percent: number
  size: number
  stroke?: number
  color?: string
  gradient?: readonly [string, string, ...string[]]
  track?: string
  indeterminate?: boolean
  children?: ReactNode
  className?: string
  style?: CSSProperties
  label?: string
}

/** Shared ring geometry: percentages use normalized SVG paths, including empty and closed endpoints. */
export function ProgressRing({ percent, size, stroke = size >= 148 ? 7 : size >= 96 ? 7.5 : size >= 72 ? 5 : 2.5,
  color: arcColor = color['--color-accent'], gradient, track = line['--line-3'], indeterminate = false,
  children, className, style, label }: Props) {
  const id = useId()
  const reduce = useSyncExternalStore(subscribeMotion, reducedMotion, serverMotion)
  const value = indeterminate ? 25 : Number.isFinite(percent) ? Math.min(100, Math.max(0, percent)) : 0
  const radius = (size - stroke) / 2
  return <span className={[stylex.props(styles.root).className, className].filter(Boolean).join(' ')}
    role={label ? 'img' : undefined} aria-label={label} data-progress-ring="" style={{ width: size, height: size, ...style }}>
    <svg {...stylex.props(styles.svg, indeterminate && !reduce && styles.spin)} width={size} height={size} viewBox={`0 0 ${size} ${size}`} fill="none" aria-hidden="true">
      {gradient && <defs><linearGradient id={id} x1="0" y1="0" x2="1" y2="1">{gradient.map((stop, index) => <stop key={index} offset={`${index / (gradient.length - 1) * 100}%`} stopColor={stop} />)}</linearGradient></defs>}
      <circle cx={size / 2} cy={size / 2} r={radius} stroke={track} strokeWidth={stroke} />
      <circle {...stylex.props(styles.arc)} data-progress-arc="" cx={size / 2} cy={size / 2} r={radius} pathLength="100"
        stroke={gradient ? `url(#${id})` : arcColor} strokeWidth={stroke} strokeLinecap="round" strokeDasharray={`${value} 100`} visibility={value === 0 ? 'hidden' : undefined} />
    </svg>
    {children != null && <span {...stylex.props(styles.center)}>{children}</span>}
  </span>
}
