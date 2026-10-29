// Điều khiển Kit — motion theo `motion:controls v1` (17-kit.md › Chuyển động của điều khiển):
// quy tắc chuyển động nằm trong kit.css và bám [role=switch|radio|checkbox][aria-checked],
// nên đổi trạng thái từ chuột, phím hay SSE đều ra cùng chuyển động; re-render giữ nguyên
// aria-checked không chạy lại animation (DOM node được giữ, animation gắn theo thuộc tính).
import { useLayoutEffect, useRef, type ButtonHTMLAttributes, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react'
import { Spinner } from './feedback'
import { Icon } from './Icon'
import './kit.css'
import { getKitLabels } from './labels'

/** Phím mũi tên/Home/End trong role=radiogroup: click() mục kế tiếp nên ra cùng chuyển động như chuột. */
export function radioGroupKeys(event: KeyboardEvent<HTMLElement>) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
  const group = event.currentTarget.closest('[role=radiogroup]')
  if (!group) return
  const items = Array.from(group.querySelectorAll<HTMLElement>('[role=radio]:not(:disabled)'))
  const index = items.indexOf(event.currentTarget as HTMLElement)
  if (index < 0 || !items.length) return
  event.preventDefault()
  const next = event.key === 'Home' ? items[0]
    : event.key === 'End' ? items[items.length - 1]
    : items[(index + (event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]
  next?.focus()
  next?.click()
}

/** Vòng radio của Kit (.rd): mặc định 18 px viền 1,5 px · size=lg 22 px viền 2 px, chấm 10 (MCmd .rdot). */
export function RadioDot({ checked, style, size = 'md' }: { checked: boolean; style?: CSSProperties; size?: 'md' | 'lg' | number }) {
  const d = typeof size === 'number' ? size : size === 'lg' ? 22 : 18
  const lg = d >= 20
  const dot = lg ? 10 : 8
  return (
    <span aria-hidden="true" className="kit-rdot" data-checked={checked} style={{
      width: d, height: d, boxSizing: 'border-box', borderRadius: '50%', flex: '0 0 auto',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      border: `${lg ? 2 : 1.5}px solid ${checked ? 'var(--color-accent)' : lg ? 'var(--kit-wash-22)' : 'var(--kit-wash-28)'}`,
      transition: 'border-color .2s ease', ...style,
    }}>
      <span style={{ width: dot, height: dot, borderRadius: '50%', background: 'var(--color-accent)', transform: checked ? undefined : 'scale(0)' }} />
    </span>
  )
}

/** Kích thước rãnh công tắc theo artboard: sm 36×20 (Onboard/AudioExport) · md 36×22 (Kit) · lg 38×22 (Settings) · tuple [w,h] tuỳ ý (51×31 mobile .msw). */
export type SwitchSize = 'sm' | 'md' | 'lg' | readonly [number, number]
const SWITCH_TRACK: Record<'sm' | 'md' | 'lg', readonly [number, number]> = { sm: [36, 20], md: [36, 22], lg: [38, 22] }

export type SwitchProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> & {
  checked: boolean
  onChange: (on: boolean) => void
  label?: string
  pending?: boolean
  loading?: boolean
  size?: SwitchSize
  /** 'accent' = rãnh tím #9A63E3 · 'mono' = rãnh sáng khi bật + nút tối (Onboard, MiniSwitch audio). */
  tone?: 'accent' | 'mono'
  trackStyle?: CSSProperties
  thumbStyle?: CSSProperties
}

export function Switch({ checked, onChange, label, disabled, pending = false, loading = false, size = 'md', tone = 'accent', trackStyle, thumbStyle, style, onClick, ...rest }: SwitchProps) {
  const busy = pending || loading
  const settled = useRef(checked)
  useLayoutEffect(() => { if (!busy) settled.current = checked }, [busy, checked])
  const displayed = busy ? settled.current : checked
  const [w, h] = typeof size === 'string' ? SWITCH_TRACK[size] : size
  const pad = size === 'md' ? 3 : 2
  const knob = h - pad * 2
  const tx = displayed ? w - pad * 2 - knob : 0
  const trackBg = tone === 'mono'
    ? (displayed ? 'var(--color-fg)' : 'var(--color-surface-7)')
    : (displayed ? 'var(--kit-switch-on)' : 'var(--color-surface-7)')
  const knobBg = tone === 'mono'
    ? (displayed ? 'var(--color-bg)' : 'var(--color-fg-4)')
    : size === 'lg' || h === 31 ? 'var(--kit-white)' : 'var(--color-fg)'
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 12, flexShrink: 0 }}>
      {/* Vùng bấm 48×44; lề âm giữ ô bố cục đúng rãnh như artboard. */}
      <button {...rest} type="button" role="switch" className={['kit-switch', rest.className].filter(Boolean).join(' ')} aria-checked={displayed} aria-label={rest['aria-label'] ?? label}
        aria-busy={busy || undefined} disabled={disabled || busy}
        onClick={(event) => { onClick?.(event); if (!event.defaultPrevented) onChange(!checked) }} style={{
          width: Math.max(48, w + 12), height: Math.max(44, h), margin: `${-(Math.max(44, h) - h) / 2}px -6px`, padding: 0, border: 'none', background: 'transparent',
          display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
          opacity: disabled ? .4 : 1, cursor: disabled || busy ? 'not-allowed' : 'pointer', ...style,
        }}>
        <span className="kit-switch-track" aria-hidden="true" style={{
          width: w, height: h, borderRadius: 999, position: 'relative', display: 'block', flexShrink: 0,
          background: trackBg, ...trackStyle,
        }}>
          <span className="kit-switch-thumb" style={{
            position: 'absolute', top: pad, left: pad, width: knob, height: knob, borderRadius: '50%',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            background: busy ? 'transparent' : knobBg, boxShadow: busy ? 'none' : h === 31 ? 'var(--kit-shadow-mobile-thumb)' : 'var(--kit-shadow-thumb)',
            transform: `translateX(${tx}px) scaleX(var(--kit-sx, 1))`,
            ...thumbStyle,
          }}>
            {busy && <span role="status" aria-label={getKitLabels().controls.busyChanging}><Spinner size={12} /></span>}
          </span>
        </span>
      </button>
    </span>
  )
}

export type CheckboxProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> & {
  checked: boolean | 'mixed'
  onChange: (on: boolean) => void
  label?: ReactNode
  indeterminate?: boolean
  /** 'box' = ô 18 px (mặc định) · 'chip' = viên thuốc không ô, dấu ✓ icon (Onboard › Định dạng, thư viện). */
  variant?: 'box' | 'chip'
  /** 'accent' = ô tím ✓ đậm (mặc định) · 'mono' = ô sáng #EDEDEF ✓ đậm (NewProject, Auth, Account). */
  tone?: 'accent' | 'mono'
  size?: 16 | 18
  showMark?: boolean
}

export function Checkbox({ checked, onChange, label, disabled, indeterminate, variant = 'box', tone = 'accent', size = 18, showMark = true, style, onClick, ...rest }: CheckboxProps) {
  const mixed = checked === 'mixed' || !!indeterminate
  const selected = mixed || checked === true
  if (variant === 'chip') {
    return (
      <button {...rest} type="button" role="checkbox" aria-checked={selected} disabled={disabled}
        onClick={(event) => { onClick?.(event); if (!event.defaultPrevented) onChange(!checked) }} style={{
          display: 'inline-flex', alignItems: 'center', gap: 6, padding: '5px 10px',
          borderRadius: 999, border: `1px solid ${selected ? 'var(--kit-acc-45)' : 'var(--line-3)'}`,
          background: selected ? 'var(--tint-accent)' : 'transparent',
          color: disabled ? 'var(--kit-fg-off)' : 'var(--color-fg)', fontSize: 12.5,
          cursor: disabled ? 'not-allowed' : 'pointer', ...style,
        }}>
        {selected && showMark && <Icon name="tick-circle-bulk" size={13} className="kit-cb-tick" />}
        {label}
      </button>
    )
  }
  const boxOn = tone === 'mono' ? 'var(--color-fg)' : 'var(--color-accent)'
  return (
    <button {...rest} type="button" role="checkbox" aria-checked={mixed ? 'mixed' : checked} disabled={disabled}
      onClick={(event) => { onClick?.(event); if (!event.defaultPrevented) onChange(mixed || !checked) }} style={{
        display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', margin: '-6px 0',
        border: 'none', background: 'transparent', color: disabled ? 'var(--kit-fg-off)' : 'var(--color-fg)', fontSize: 13, textAlign: 'left', opacity: 1, ...style,
      }}>
      {/* kit-cb-box nảy khi chọn; ✓ vẽ bằng stroke-dashoffset, mờ đi khi bỏ chọn (giữ trong DOM). */}
      <span aria-hidden="true" className="kit-cb-box" style={{
        width: size, height: size, borderRadius: 5, flex: '0 0 auto', boxSizing: 'border-box',
        position: 'relative', display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        background: selected ? boxOn : 'transparent',
        border: `1.5px solid ${selected ? boxOn : 'var(--kit-wash-28)'}`,
        opacity: disabled && selected ? .4 : 1,
      }}>
        <span style={{ position: 'absolute', width: 8, height: 2, borderRadius: 1, background: 'var(--color-bg)', opacity: mixed ? 1 : 0 }} />
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" style={{ position: 'absolute', opacity: !mixed && checked === true ? 1 : 0 }}>
          <path d="M5 12.5l4.5 4.5L19 7.5" stroke="var(--color-bg)" strokeWidth="3.2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
      {label}
    </button>
  )
}

export type RadioProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange' | 'children'> & {
  checked: boolean
  onChange: (checked: boolean) => void
  label?: ReactNode
  description?: ReactNode
  /** 'dot' = RadioDot 18 px (mặc định) · 'tick' = icon tick-circle 20 px (hàng chọn của mobile). */
  mark?: 'dot' | 'tick'
  /** 'start' = dấu bên trái (mặc định) · 'end' = bên phải sau nhãn. */
  markPosition?: 'start' | 'end'
  dotSize?: number
  /** Nội dung trái tự do (vd avatar 42 px ở đổi tài khoản) — đặt trước mark 'start'. */
  leading?: ReactNode
  /** Nội dung phải tự do (vd cỡ ngữ cảnh model, chữ "Đăng nhập") — đặt sau nhãn, trước mark 'end'. */
  trailing?: ReactNode
  /** children thay cột nhãn/phụ khi hàng cần bố cục riêng (avatar + tên/vai trò 15.5 px). */
  children?: ReactNode
  /** Gắn data-name để nhóm hình thức; radiogroup bố trí ở ngoài. */
  name?: string
}

export function Radio({ checked, onChange, label, description, disabled, mark = 'dot', markPosition = 'start', dotSize, leading, trailing, name, children, style, onClick, onKeyDown, ...rest }: RadioProps) {
  const markEl = mark === 'tick'
    ? (checked ? <Icon name="tick-circle" size={20} className="kit-radio-tick" style={{ color: 'var(--color-accent)', flex: '0 0 auto' }} /> : null)
    : <RadioDot checked={checked} size={dotSize} />
  return (
    <button {...rest} type="button" role="radio" aria-checked={checked} tabIndex={rest.tabIndex ?? (checked ? 0 : -1)} data-name={name} disabled={disabled}
      onClick={(event) => { onClick?.(event); if (!event.defaultPrevented) onChange(true) }}
      onKeyDown={(event) => { onKeyDown?.(event); if (!event.defaultPrevented) radioGroupKeys(event) }} style={{
        display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', margin: '-6px 0',
        border: 'none', background: 'transparent', color: disabled ? 'var(--kit-fg-off)' : 'var(--color-fg)',
        fontSize: 13, textAlign: 'left', cursor: disabled ? 'not-allowed' : 'pointer', ...style,
      }}>
      {leading}
      {markPosition === 'start' && markEl}
      {children ?? (
        <span style={{ display: 'flex', flexDirection: 'column', flex: '1 1 auto', minWidth: 0 }}>
          <span>{label}</span>
          {description && <span style={{ fontSize: 11.5, color: 'var(--kit-fg-icon)' }}>{description}</span>}
        </span>
      )}
      {trailing}
      {markPosition === 'end' && markEl}
    </button>
  )
}

export type RadioCardProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange' | 'children'> & {
  checked: boolean
  onChange: (checked: boolean) => void
  label?: ReactNode
  description?: ReactNode
  /** 'md' = thẻ Kit 10 px bo 10 · 'lg' = .rcard của MCmd: rdot 22 px, pad 14, bo 16, nhãn 15/500, phụ 13. */
  size?: 'md' | 'lg'
  mark?: 'dot' | 'none'
  dotSize?: number
  /** children thay cột nhãn/phụ (thẻ riêng: provider, ngôn ngữ, chế độ); dấu radio vẫn giữ bên trái. */
  children?: ReactNode
}

export function RadioCard({ checked, onChange, label, description, disabled, id, size = 'md', mark = 'dot', dotSize, children, style, className, onClick, onKeyDown, ...rest }: RadioCardProps) {
  const lg = size === 'lg'
  return (
    <button {...rest} id={id} type="button" role="radio" aria-checked={checked} tabIndex={rest.tabIndex ?? (checked ? 0 : -1)} disabled={disabled}
      className={['kit-rcard', className].filter(Boolean).join(' ')}
      onClick={(event) => { onClick?.(event); if (!event.defaultPrevented) onChange(true) }}
      onKeyDown={(event) => { onKeyDown?.(event); if (!event.defaultPrevented) radioGroupKeys(event) }} style={{
        display: 'flex', gap: lg ? 12 : 10, alignItems: 'flex-start', padding: lg ? 14 : '8px 10px', borderRadius: lg ? 16 : 10,
        border: `1px solid ${checked ? lg ? 'var(--kit-acc-50)' : 'var(--kit-acc-40)' : lg ? 'var(--line-3)' : 'var(--line-2)'}`,
        background: checked ? lg ? 'var(--kit-acc-7)' : 'var(--kit-acc-6)' : lg ? 'var(--color-card)' : 'transparent', color: 'var(--color-fg)',
        textAlign: 'left', cursor: disabled ? 'not-allowed' : 'pointer', opacity: disabled ? .4 : 1, ...style,
      }}>
      {mark === 'dot' && <RadioDot checked={checked} size={dotSize ?? size} style={{ marginTop: 1 }} />}
      {children ?? (
        <span style={{ display: 'flex', flexDirection: 'column', flex: '1 1 auto', minWidth: 0, lineHeight: lg ? 1.45 : undefined }}>
          <span style={{ fontSize: lg ? 15 : 13, fontWeight: lg ? 500 : undefined }}>{label}</span>
          {description && <span style={{ fontSize: lg ? 13 : 11.5, color: 'var(--kit-fg-icon)' }}>{description}</span>}
        </span>
      )}
    </button>
  )
}
