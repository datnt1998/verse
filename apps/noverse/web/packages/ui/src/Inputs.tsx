import { forwardRef, useId, useImperativeHandle, useRef, useState, type CSSProperties, type InputHTMLAttributes, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, tint, kit } from './tokens.stylex'
import { FieldError, FieldLabel, inputFrame } from './Field'
import { Icon } from './Icon'
import './kit.css'
import { getKitLabels } from './labels'

const sx = stylex.create({
  col: { display: 'flex', flexDirection: 'column', gap: 5 },
  col6: { display: 'flex', flexDirection: 'column', gap: 6 },
  col8: { display: 'flex', flexDirection: 'column', gap: 8 },
  label: { fontSize: 11.5, color: kit['--kit-fg-icon'] },
  frame: {
    display: 'flex', alignItems: 'center', minHeight: 36,
    borderRadius: 10, backgroundColor: color['--color-surface-0'],
    borderWidth: 1, borderStyle: 'solid',
  },
  frameGapPlain: { gap: 8 },
  frameGapAdorn: { gap: 6 },
  framePadPlain: { paddingTop: 0, paddingRight: 12, paddingBottom: 0, paddingLeft: 12 },
  framePadClear: { paddingTop: 0, paddingRight: 6, paddingBottom: 0, paddingLeft: 12 },
  framePadAdorn: { paddingTop: 0, paddingRight: 10, paddingBottom: 0, paddingLeft: 10 },
  input: {
    flex: 1, minWidth: 0, width: '100%', height: 34, padding: 0, borderWidth: 0, borderStyle: 'none',
    backgroundColor: 'transparent', color: color['--color-fg'], outline: 'none',
  },
  inputPlain: { fontSize: 13 },
  inputAdorn: { fontSize: 12.5 },
  prefix: {
    display: 'inline-flex', alignItems: 'center', gap: 6, color: color['--color-fg-4'],
    fontSize: 12.5, flexShrink: 0,
  },
  suffix: {
    display: 'inline-flex', alignItems: 'center', color: color['--color-ok'], fontSize: 12,
    paddingTop: 2, paddingBottom: 2, paddingInline: 6, borderRadius: 5,
    backgroundColor: tint['--tint-ok'], flexShrink: 0,
  },
  clear: {
    width: 24, height: 24, padding: 0, borderWidth: 0, borderStyle: 'none', borderRadius: 6,
    backgroundColor: 'transparent', color: color['--color-fg-4'],
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
  },
  ghostHover: { backgroundColor: { default: 'transparent', ':hover': line['--line-2'] } },
  edit: {
    alignSelf: 'flex-start', padding: 0, borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent',
    color: color['--color-accent'], fontSize: 11.5,
  },
  message: { fontSize: 11.5, display: 'flex', gap: 6, alignItems: 'flex-start' },
  messageErr: { color: kit['--kit-err-soft'] },
  messageHint: { color: color['--color-fg-4'] },
  messageIcon: { marginTop: 2 },
  stepperBtn: {
    flex: '0 0 36px', height: '100%', borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent',
    color: color['--color-fg-2'], fontSize: 16, display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  stepperOff: { opacity: .4 },
  stepper: {
    display: 'flex', alignItems: 'center', gap: 0, width: 148, height: 36, borderRadius: 10,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], overflow: 'hidden',
  },
  dim: { opacity: .45 },
  stepperInput: {
    flex: 1, minWidth: 0, width: 0, height: '100%', padding: 0, borderWidth: 0, borderStyle: 'none',
    borderLeftWidth: 1, borderLeftStyle: 'solid', borderLeftColor: line['--line-3'],
    borderRightWidth: 1, borderRightStyle: 'solid', borderRightColor: line['--line-3'],
    backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'center',
    fontSize: 13, fontVariantNumeric: 'tabular-nums', outline: 'none',
  },
  hint: { fontSize: 11.5, color: color['--color-fg-4'] },
  sliderWrap: { position: 'relative', paddingTop: 22 },
  bubble: {
    position: 'absolute', top: 0, transform: 'translateX(-50%)',
    height: 18, paddingInline: 6, borderRadius: 5, backgroundColor: color['--color-surface-7'],
    fontSize: 10.5, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap', pointerEvents: 'none',
  },
  bubbleLeft: (left: string) => ({ left }),
  marks: { display: 'flex', justifyContent: 'space-between', color: kit['--kit-fg-off'], fontSize: 10.5 },
  sliderValue: { fontSize: 12, color: color['--color-accent'] },
})

export type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'defaultValue' | 'onChange' | 'prefix'> & {
  value?: string
  defaultValue?: string
  onChange?: (value: string) => void
  label?: ReactNode
  hint?: ReactNode
  error?: string
  prefix?: ReactNode
  suffix?: ReactNode
  clearable?: boolean
  onClear?: () => void
  validate?: (value: string) => string | undefined
  onEdit?: () => void
  editLabel?: string
  /** Vẽ sẵn trạng thái đang chọn (viền + vòng tím) — dùng cho bản trình diễn trạng thái. */
  focused?: boolean
}

export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(function TextField({
  value, defaultValue = '', onChange, label, hint, error, prefix, suffix, clearable,
  onClear, validate, onEdit, editLabel, focused, id, disabled, readOnly,
  style, className, 'aria-describedby': describedBy, 'aria-invalid': ariaInvalid, ...rest
}, ref) {
  const generatedId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  useImperativeHandle(ref, () => inputRef.current!)
  const inputId = id ?? generatedId
  const [draft, setDraft] = useState(defaultValue)
  const current = value ?? draft
  const message = error || validate?.(current)
  const invalid = !!message || ariaInvalid === true || ariaInvalid === 'true'
  const helpId = `${inputId}-help`
  const change = (next: string) => {
    if (value === undefined) setDraft(next)
    onChange?.(next)
  }
  const plain = !prefix && !suffix
  const frame = stylex.props(
    sx.frame,
    plain ? sx.frameGapPlain : sx.frameGapAdorn,
    plain ? (clearable ? sx.framePadClear : sx.framePadPlain) : sx.framePadAdorn,
    ...inputFrame({ invalid, focused, disabled }),
  )
  const input = stylex.props(sx.input, plain ? sx.inputPlain : sx.inputAdorn)
  const clear = stylex.props(sx.clear, sx.ghostHover)
  return (
    <div {...stylex.props(sx.col)}>
      {label && <label htmlFor={inputId} {...stylex.props(sx.label)}>{label}</label>}
      <div className={`kit-frame ${frame.className}`} data-invalid={invalid} data-focus={focused || undefined} data-disabled={disabled || undefined} style={frame.style}>
        {prefix && <span className={`mono ${stylex.props(sx.prefix).className}`}>{prefix}</span>}
        <input {...rest} ref={inputRef} id={inputId} disabled={disabled} readOnly={readOnly}
          className={[input.className, className, !plain && 'mono'].filter(Boolean).join(' ') || undefined} value={current} onChange={(event) => change(event.currentTarget.value)}
          aria-invalid={invalid || undefined}
          aria-describedby={[describedBy, (message || hint) && helpId].filter(Boolean).join(' ') || undefined}
          style={{ ...input.style, ...style }} />
        {suffix && <span className={`mono ${stylex.props(sx.suffix).className}`}>{suffix}</span>}
        {clearable && current && !disabled && !readOnly && <button type="button" aria-label={getKitLabels().inputs.clear} title={getKitLabels().inputs.clear}
          className={clear.className}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => { change(''); onClear?.(); inputRef.current?.focus() }}>
          <Icon name="add" size={14} style={{ transform: 'rotate(45deg)' }} />
        </button>}
      </div>
      {message ? <FieldMessage id={helpId} tone="err">{message}</FieldMessage> : hint ? <FieldMessage id={helpId}>{hint}</FieldMessage> : null}
      {(readOnly || disabled) && onEdit && <button type="button" onClick={onEdit}
        {...stylex.props(sx.edit)}>
        {editLabel}
      </button>}
    </div>
  )
})

/** Dòng chú thích dưới ô nhập của Kit: 11.5 px, icon 13 px (lỗi: warning-2 --kit-err-soft · gợi ý: info-circle --color-fg-4). */
function FieldMessage({ id, tone, children }: { id: string; tone?: 'err'; children: ReactNode }) {
  const p = stylex.props(sx.message, tone === 'err' ? sx.messageErr : sx.messageHint)
  return <span id={id} role={tone === 'err' ? 'alert' : undefined} className={p.className} style={p.style}>
    <Icon name={tone === 'err' ? 'warning-2' : 'info-circle'} size={13} {...stylex.props(sx.messageIcon)} />{children}
  </span>
}

export type NumberStepperProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'defaultValue' | 'onChange' | 'min' | 'max' | 'step'> & {
  value: number
  onChange: (value: number) => void
  min?: number
  max?: number
  step?: number
  label?: ReactNode
  hint?: ReactNode
}

export function NumberStepper({ value, onChange, min = 1, max = 24, step = 1, label, hint,
  id, disabled, readOnly, style, onBlur, onKeyDown, 'aria-describedby': describedBy, ...rest }: NumberStepperProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const helpId = `${inputId}-range`
  const [draft, setDraft] = useState<string | null>(null)
  const increment = step > 0 ? step : 1
  const bound = (next: number) => Math.min(max, Math.max(min, Number(next.toFixed(10))))
  const commit = () => {
    if (draft !== null && draft.trim() !== '' && Number.isFinite(Number(draft))) onChange(bound(Number(draft)))
    setDraft(null)
  }
  const downDisabled = disabled || readOnly || value <= min
  const upDisabled = disabled || readOnly || value >= max
  const box = stylex.props(sx.stepper, disabled && sx.dim)
  const input = stylex.props(sx.stepperInput)
  const down = stylex.props(sx.stepperBtn, sx.ghostHover, downDisabled && sx.stepperOff)
  const up = stylex.props(sx.stepperBtn, sx.ghostHover, upDisabled && sx.stepperOff)
  return <div {...stylex.props(sx.col6)}>
    {label && <FieldLabel htmlFor={inputId} size={10.5}>{label}</FieldLabel>}
    <div className={`kit-frame ${box.className}`} style={box.style}>
      <button type="button" aria-label={getKitLabels().inputs.decrease} title={getKitLabels().inputs.decrease} disabled={downDisabled}
        className={down.className}
        onClick={() => { setDraft(null); onChange(bound(value - increment)) }} style={down.style}>−</button>
      <input {...rest} className={['mono', 'num', input.className, rest.className].filter(Boolean).join(' ')} id={inputId} type="text" inputMode="decimal" role="spinbutton" value={draft ?? String(value)}
        disabled={disabled} readOnly={readOnly} aria-label={rest['aria-label'] ?? (typeof label === 'string' ? label : getKitLabels().inputs.value)}
        aria-valuemin={min} aria-valuemax={max} aria-valuenow={value}
        aria-describedby={[describedBy, helpId].filter(Boolean).join(' ')}
        onChange={(event) => setDraft(event.currentTarget.value)}
        onBlur={(event) => { commit(); onBlur?.(event) }}
        onKeyDown={(event) => {
          onKeyDown?.(event)
          if (event.defaultPrevented || disabled || readOnly) return
          if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
            event.preventDefault()
            const parsed = draft !== null && draft.trim() !== '' ? Number(draft) : value
            setDraft(null)
            onChange(bound((Number.isFinite(parsed) ? parsed : value) + (event.key === 'ArrowUp' ? increment : -increment)))
          } else if (event.key === 'Enter') { event.preventDefault(); commit() }
          else if (event.key === 'Escape') setDraft(null)
        }}
        style={{ ...input.style, ...style }} />
      <button type="button" aria-label={getKitLabels().inputs.increase} title={getKitLabels().inputs.increase} disabled={upDisabled}
        className={up.className}
        onClick={() => { setDraft(null); onChange(bound(value + increment)) }} style={up.style}>+</button>
    </div>
    <span id={helpId} {...stylex.props(sx.hint)}>{hint ?? getKitLabels().inputs.rangeHint(min, max)}</span>
  </div>
}

export type SliderTick = { value: number; label?: ReactNode }
export type SliderProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'defaultValue' | 'onChange' | 'min' | 'max' | 'step'> & {
  value: number
  onChange: (value: number) => void
  min?: number
  max?: number
  step?: number
  label?: ReactNode
  ticks?: SliderTick[]
  formatValue?: (value: number) => string
}

export function Slider({ value, onChange, min = .8, max = 1.5, step = .05, label, ticks,
  formatValue = (next) => `${Number(next.toFixed(2))}×`, id, disabled, style, className, ...rest }: SliderProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const percent = max > min ? Math.min(100, Math.max(0, (value - min) / (max - min) * 100)) : 0
  const marks: SliderTick[] = (ticks ?? [{ value: min }, { value: max }]).filter((tick) => tick.value >= min && tick.value <= max)
  const root = stylex.props(sx.col8, disabled && sx.dim)
  const bubble = stylex.props(sx.bubble, sx.bubbleLeft(`calc(${percent.toFixed(1)}% + ${(8 - percent * .16).toFixed(1)}px)`))
  return <div className={root.className} style={root.style}>
    {label && <FieldLabel htmlFor={inputId} size={10.5} style={{ alignItems: 'stretch' }} right={<span className={`mono num ${stylex.props(sx.sliderValue).className}`}>{formatValue(value)}</span>}>{label}</FieldLabel>}
    <div {...stylex.props(sx.sliderWrap)}>
      <span className={`mono num ${bubble.className}`} aria-hidden="true" style={bubble.style}>{formatValue(value)}</span>
      <input {...rest} id={inputId} className={['kit-range', className].filter(Boolean).join(' ')} type="range" value={value} min={min} max={max} step={step} disabled={disabled}
        aria-label={rest['aria-label'] ?? (typeof label === 'string' ? label : getKitLabels().inputs.value)}
        aria-valuetext={formatValue(value)} onChange={(event) => onChange(event.currentTarget.valueAsNumber)}
        style={{ ['--p' as string]: `${percent.toFixed(1)}%`, ...(disabled ? { cursor: 'not-allowed' } : {}), ...style }} />
    </div>
    <div aria-hidden="true" className={`mono ${stylex.props(sx.marks).className}`}>
      {marks.map((tick) => <span key={tick.value}>{tick.label ?? formatValue(tick.value)}</span>)}
    </div>
  </div>
}
