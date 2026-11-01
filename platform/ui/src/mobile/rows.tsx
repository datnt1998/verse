// Hàng và chỉ báo của điện thoại theo artboard Viderse MKit: ListRow (`.vv-row--m`),
// VideoRow, StatusPill (`.vv-status`), MeterBar (`.vv-progress`), InitialsAvatar
// (`.vv-avatar`), Stepper (`.vv-stepper--m`).
import type { CSSProperties, MouseEventHandler, ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius } from '../tokens.stylex'
import { Icon } from '../Icon'
import type { IconName } from '../iconNames'
import { getKitLabels } from '../labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const styles = stylex.create({
  row: {
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    width: '100%',
    minHeight: 56,
    paddingTop: 6,
    paddingBottom: 6,
    paddingInline: 10,
    borderWidth: 0,
    borderRadius: radius['--radius-tile'],
    backgroundColor: 'transparent',
    color: color['--color-fg'],
    fontSize: 13.5,
    textAlign: 'start',
    textDecoration: 'none',
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
    ':hover': { backgroundColor: kit['--kit-wash-4'] },
    ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: -2 },
  },
  rowStatic: { cursor: 'default', ':hover': { backgroundColor: 'transparent' } },
  rowActive: { backgroundColor: kit['--kit-wash-4'], boxShadow: `inset 0 0 0 1px ${line['--line-3']}` },
  rowDivider: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'], borderRadius: 0 },
  rowIcon: { flex: '0 0 auto', display: 'inline-flex', color: kit['--kit-fg-sub'] },
  rowText: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 1 },
  rowTitle: { minWidth: 0, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' },
  rowHint: { fontSize: 11.5, lineHeight: 1.5, color: kit['--kit-fg-sub'] },
  rowValue: { flex: '0 0 auto', fontSize: 13, color: kit['--kit-fg-sub'], fontVariantNumeric: 'tabular-nums' },
  rowChevron: { flex: '0 0 auto', display: 'inline-flex', color: kit['--kit-fg-icon'] },
  danger: { color: kit['--kit-err-ghost-ink'] },

  video: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    paddingTop: 8,
    paddingBottom: 8,
    borderBottomWidth: 1,
    borderBottomStyle: 'solid',
    borderBottomColor: line['--line-1'],
  },
  thumb: {
    flex: '0 0 auto',
    width: 80,
    height: 45,
    borderRadius: radius['--radius-control'],
    backgroundColor: color['--color-surface-5'],
    backgroundSize: 'cover',
    backgroundPosition: 'center',
    overflow: 'hidden',
  },
  videoText: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 },
  videoTitle: {
    fontSize: 13.5,
    fontWeight: 500,
    minWidth: 0,
    overflow: 'hidden',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
  },
  videoMeta: { minWidth: 0, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' },

  status: { display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12, whiteSpace: 'nowrap', lineHeight: 1.3 },
  dot: { flex: '0 0 auto', width: 7, height: 7, borderRadius: '50%', backgroundColor: 'currentColor' },
  spinner: {
    flex: '0 0 auto',
    width: 9,
    height: 9,
    borderRadius: '50%',
    borderWidth: 1.5,
    borderStyle: 'solid',
    borderColor: kit['--kit-spin-track'],
    borderTopColor: 'currentColor',
    animationName: { default: 'ai-spin', [NO_MOTION]: 'none' },
    animationDuration: '.9s',
    animationTimingFunction: 'linear',
    animationIterationCount: 'infinite',
  },

  meter: { display: 'flex', flexDirection: 'column', gap: 6, width: '100%' },
  meterHead: { display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 8, fontSize: 11.5, color: kit['--kit-fg-sub'] },
  track: { width: '100%', borderRadius: 999, backgroundColor: line['--line-2'], overflow: 'hidden' },
  fill: { display: 'block', height: '100%', borderRadius: 'inherit', transitionProperty: 'width', transitionDuration: '.3s' },

  avatar: {
    flex: '0 0 auto',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: '50%',
    fontWeight: 600,
    letterSpacing: '.02em',
    backgroundColor: color['--color-surface-6'],
    color: color['--color-fg'],
    boxShadow: `inset 0 0 0 1px ${line['--line-3']}`,
    overflow: 'hidden',
  },

  stepper: {
    display: 'inline-flex',
    alignItems: 'stretch',
    width: 160,
    height: 44,
    borderRadius: radius['--radius-row'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-4'],
    backgroundColor: color['--color-surface-0'],
    overflow: 'hidden',
  },
  stepButton: {
    flex: '0 0 auto',
    width: 44,
    borderWidth: 0,
    backgroundColor: 'transparent',
    color: color['--color-fg-2'],
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    cursor: 'pointer',
    ':hover': { backgroundColor: kit['--kit-wash-7'] },
    ':disabled': { opacity: .35, cursor: 'not-allowed' },
    ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: -2 },
  },
  stepValue: {
    flex: '1 1 auto',
    minWidth: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    borderLeftWidth: 1,
    borderRightWidth: 1,
    borderLeftStyle: 'solid',
    borderRightStyle: 'solid',
    borderLeftColor: line['--line-3'],
    borderRightColor: line['--line-3'],
    color: color['--color-fg'],
    fontFamily: 'var(--font-mono)',
    fontSize: 13,
    fontVariantNumeric: 'tabular-nums',
  },
})

export type ListRowProps = {
  icon?: IconName
  title: ReactNode
  hint?: ReactNode
  /** giá trị bên phải (số, chữ ngắn) */
  value?: ReactNode
  chevron?: boolean
  onClick?: MouseEventHandler<HTMLElement>
  href?: string
  /** hàng nguy hiểm: chữ cam */
  destructive?: boolean
  /** đang chọn/mở */
  active?: boolean
  /** kẻ trên (dùng trong danh sách dài, không dùng trong thẻ nhóm) */
  divider?: boolean
  /** nội dung thay cho `value` + chevron (công tắc, nút) */
  trailing?: ReactNode
  xstyle?: stylex.StyleXStyles
  children?: ReactNode
}

export function ListRow({
  icon, title, hint, value, chevron, onClick, href, destructive, active, divider, trailing, xstyle, children,
}: ListRowProps) {
  const interactive = Boolean(onClick || href)
  const sx = stylex.props(
    styles.row,
    !interactive && styles.rowStatic,
    active && styles.rowActive,
    divider && styles.rowDivider,
    destructive && styles.danger,
    xstyle,
  )
  const body = (
    <>
      {icon && <span {...stylex.props(styles.rowIcon)}><Icon name={icon} size={18} /></span>}
      <span {...stylex.props(styles.rowText)}>
        <span {...stylex.props(styles.rowTitle)}>{title}</span>
        {hint != null && <span {...stylex.props(styles.rowHint)}>{hint}</span>}
        {children}
      </span>
      {value != null && <span {...stylex.props(styles.rowValue)}>{value}</span>}
      {trailing}
      {chevron && <span {...stylex.props(styles.rowChevron)} aria-hidden="true"><Icon name="arrow-r" size={15} /></span>}
    </>
  )
  if (href) return <a href={href} className={`m-row ${sx.className ?? ''}`} style={sx.style} onClick={onClick}>{body}</a>
  if (onClick) return <button type="button" className={`m-row ${sx.className ?? ''}`} style={sx.style} onClick={onClick}>{body}</button>
  return <div className={`m-row ${sx.className ?? ''}`} style={sx.style}>{body}</div>
}

export type VideoRowProps = {
  title: ReactNode
  /** ảnh thu nhỏ 80×45 */
  thumb?: string
  thumbAlt?: string
  /** dòng trạng thái dưới tiêu đề (thường là StatusPill) */
  status?: ReactNode
  /** nút bên phải (Duyệt, Xem…) */
  action?: ReactNode
  onClick?: MouseEventHandler<HTMLElement>
  xstyle?: stylex.StyleXStyles
}

export function VideoRow({ title, thumb, thumbAlt, status, action, onClick, xstyle }: VideoRowProps) {
  const sx = stylex.props(styles.video, xstyle)
  const body = (
    <>
      <span {...stylex.props(styles.thumb)} style={thumb ? { backgroundImage: `url(${thumb})` } : undefined} role={thumb ? 'img' : undefined} aria-label={thumb ? thumbAlt ?? '' : undefined} />
      <span {...stylex.props(styles.videoText)}>
        <span {...stylex.props(styles.videoTitle)}>{title}</span>
        {status && <span {...stylex.props(styles.videoMeta)}>{status}</span>}
      </span>
      {action}
    </>
  )
  if (!onClick) return <div className={sx.className} style={sx.style}>{body}</div>
  return <button type="button" className={sx.className} style={sx.style} onClick={onClick}>{body}</button>
}

export type StatusTone = 'queued' | 'working' | 'review' | 'issue' | 'ready' | 'posted' | 'paused'

const TONE: Record<StatusTone, string> = {
  queued: kit['--kit-fg-icon'],
  paused: kit['--kit-fg-icon'],
  working: color['--color-info'],
  review: color['--color-warn'],
  issue: color['--color-err'],
  ready: color['--color-ok'],
  posted: kit['--kit-fg-sub'],
}

export type StatusPillProps = {
  status: StatusTone
  children?: ReactNode
  xstyle?: stylex.StyleXStyles
}

/** Chấm 7px + nhãn 12px (artboard `.vv-status`); `working` là vòng xoay 9px. */
export function StatusPill({ status, children, xstyle }: StatusPillProps) {
  const spin = status === 'working'
  return (
    <span className={stylex.props(styles.status, xstyle).className} style={{ ...stylex.props(styles.status, xstyle).style, color: TONE[status] }}>
      <span aria-hidden="true" {...stylex.props(spin ? styles.spinner : styles.dot)} />
      {children}
    </span>
  )
}

export type MeterTone = 'accent' | 'info' | 'ok' | 'err' | 'warn'

const METER: Record<MeterTone, string> = {
  accent: color['--color-accent'],
  info: color['--color-info'],
  ok: color['--color-ok'],
  err: color['--color-err'],
  warn: color['--color-warn'],
}

export type MeterBarProps = {
  /** 0…1 */
  value: number
  tone?: MeterTone
  /** chiều cao rãnh, mặc định 4 (artboard `.vv-progress`) */
  height?: number
  label?: ReactNode
  /** chữ bên phải nhãn (thường là phần trăm) */
  readout?: ReactNode
  xstyle?: stylex.StyleXStyles
}

export function MeterBar({ value, tone = 'accent', height = 4, label, readout, xstyle }: MeterBarProps) {
  const pct = Math.round(Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0)) * 100)
  return (
    <div className={stylex.props(styles.meter, xstyle).className} style={stylex.props(styles.meter, xstyle).style}>
      {(label != null || readout != null) && (
        <span {...stylex.props(styles.meterHead)}>
          <span>{label}</span>
          {readout != null && <span>{readout}</span>}
        </span>
      )}
      <div
        {...stylex.props(styles.track)}
        style={{ height }}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        aria-label={typeof label === 'string' ? label : undefined}
      >
        <span {...stylex.props(styles.fill)} style={{ width: `${pct}%`, backgroundColor: METER[tone] }} />
      </div>
    </div>
  )
}

export type InitialsAvatarProps = {
  /** tên đầy đủ — lấy chữ đầu của hai từ cuối cùng khi không truyền `initials` */
  name?: string
  initials?: string
  size?: number
  /** màu nền kênh (rgba) */
  bg?: string
  ring?: string
  fg?: string
  xstyle?: stylex.StyleXStyles
}

/** Lấy chữ đầu: hai từ đầu với tên tiếng Việt ("Kênh Thực Tế" → KT). */
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (!words.length) return ''
  if (words.length === 1) return words[0].slice(0, 2).toLocaleUpperCase('vi')
  return (words[0][0] + words[1][0]).toLocaleUpperCase('vi')
}

export function InitialsAvatar({ name, initials, size = 30, bg, ring, fg, xstyle }: InitialsAvatarProps) {
  const text = initials ?? initialsOf(name ?? '')
  const sx = stylex.props(styles.avatar, xstyle)
  const skin: CSSProperties = {
    width: size,
    height: size,
    fontSize: Math.round(size * 0.38),
    ...(bg ? { backgroundColor: bg } : null),
    ...(ring ? { boxShadow: `inset 0 0 0 1px ${ring}` } : null),
    ...(fg ? { color: fg } : null),
  }
  return <span className={sx.className} style={{ ...sx.style, ...skin }} aria-hidden={name ? undefined : true} title={name}>{text}</span>
}

export type StepperProps = {
  value: number
  onChange: (next: number) => void
  min?: number
  max?: number
  step?: number
  disabled?: boolean
  /** nhãn đọc cho trình đọc màn hình, ví dụ "Số video" */
  label?: string
  /** hiển thị thêm đơn vị cạnh số ("ảnh") */
  suffix?: string
  xstyle?: stylex.StyleXStyles
}

/** Bước số 44px của điện thoại (artboard `.vv-stepper--m`): luôn kẹp trong [min, max]. */
export function Stepper({ value, onChange, min = 0, max = 99, step = 1, disabled, label, suffix, xstyle }: StepperProps) {
  const t = getKitLabels()
  const clamp = (next: number) => Math.min(max, Math.max(min, next))
  const set = (next: number) => { const clamped = clamp(next); if (clamped !== value) onChange(clamped) }
  const atMin = value <= min
  const atMax = value >= max
  return (
    <span className={stylex.props(styles.stepper, xstyle).className} style={stylex.props(styles.stepper, xstyle).style} role="group" aria-label={label}>
      <button type="button" {...stylex.props(styles.stepButton)} aria-label={t.inputs.decrease} disabled={disabled || atMin} onClick={() => set(value - step)}>
        <Icon name="minus" size={16} />
      </button>
      <output {...stylex.props(styles.stepValue)} aria-label={label ?? t.inputs.value} aria-live="polite">
        {value}{suffix ? ` ${suffix}` : ''}
      </output>
      <button type="button" {...stylex.props(styles.stepButton)} aria-label={t.inputs.increase} disabled={disabled || atMax} onClick={() => set(value + step)}>
        <Icon name="add" size={16} />
      </button>
    </span>
  )
}
