// Native field interfaces stay compatible; TextArea adds a counter and ⌘ Enter.
import { forwardRef, useId, useState, type CSSProperties, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Icon } from './Icon'
import { Kbd } from './Button'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const L4 = line['--line-4']
const ACC55 = kit['--kit-acc-55']
const ERRB55 = kit['--kit-errb-55']

const sx = stylex.create({
  labelRow: { display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' },
  field: {
    height: 36, paddingInline: 10, borderRadius: 9, width: '100%',
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid',
    color: color['--color-fg'], fontSize: 13,
  },
  fieldOk: { borderColor: L4 },
  fieldErr: { borderColor: color['--color-err'] },
  padded: { paddingRight: 44 },
  area: { height: 'auto', paddingTop: 10, paddingBottom: 10, paddingInline: 12, borderRadius: 10, lineHeight: 1.55, resize: 'none' },
  wrap: { position: 'relative' },
  framed: { display: 'flex', alignItems: 'center', gap: 8, paddingRight: 4 },
  plain: { flexGrow: 1, minWidth: 0, width: 0, padding: 0, borderWidth: 0, borderRadius: 0, backgroundColor: 'transparent', outline: 'none', boxShadow: 'none', color: color['--color-fg'] },
  eye: {
    position: 'absolute', right: 4, top: 4, width: 32, height: 28, borderRadius: 8,
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: color['--color-fg-3'],
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  col: { display: 'flex', flexDirection: 'column', gap: 6 },
  counterRow: { display: 'flex', justifyContent: 'flex-end' },
  counter: { fontSize: 11 },
  counterWarn: { color: color['--color-warn'] },
  counterNormal: { color: color['--color-fg-4'] },
  hint: { fontSize: 11.5, color: color['--color-fg-4'] },
  err: { display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: color['--color-err'] },
  /* Khung ô nhập (kit-input-frame): hover → viền sáng hơn; focus trong → viền tím + vòng; lỗi → đỏ; tắt → mờ. */
  frame: { transition: { default: 'border-color .15s, box-shadow .15s', [NO_MOTION]: 'none' } },
  frameNormal: {
    borderColor: { default: L4, ':hover': kit['--kit-wash-18'], ':focus-within': ACC55 },
    boxShadow: { default: 'none', ':focus-within': kit['--kit-ring-acc'] },
  },
  frameInvalid: {
    borderColor: { default: ERRB55, ':hover': ERRB55, ':focus-within': ERRB55 },
    boxShadow: { default: kit['--kit-ring-err'], ':focus-within': kit['--kit-ring-err'] },
  },
  frameFocus: {
    borderColor: { default: ACC55, ':hover': ACC55, ':focus-within': ACC55 },
    boxShadow: { default: kit['--kit-ring-acc'], ':focus-within': kit['--kit-ring-acc'] },
  },
  frameDisabled: {
    opacity: .45,
    borderColor: { default: L4, ':focus-within': ACC55 },
    boxShadow: { default: 'none', ':focus-within': kit['--kit-ring-acc'] },
  },
})

/** Khung trạng thái dùng chung cho TextField/TextArea (thay .kit-input-frame). */
export function inputFrame(opts: { invalid?: boolean; focused?: boolean; disabled?: boolean }) {
  return [sx.frame, opts.invalid ? sx.frameInvalid : opts.focused ? sx.frameFocus : opts.disabled ? sx.frameDisabled : sx.frameNormal]
}

export function FieldLabel({ htmlFor, children, right, style, size }: { htmlFor?: string; children: ReactNode; right?: ReactNode; style?: CSSProperties; size?: number }) {
  const p = stylex.props(sx.labelRow)
  return (
    <div className={p.className} style={{ ...p.style, ...style }}>
      <label htmlFor={htmlFor} className="lbl" style={size ? { fontSize: size } : undefined}>{children}</label>
      {right}
    </div>
  )
}

export const Field = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }>(
  function Field({ invalid, style, className, ...rest }, ref) {
    const p = stylex.props(sx.field, invalid ? sx.fieldErr : sx.fieldOk)
    return <input ref={ref} aria-invalid={invalid || undefined} className={[p.className, className].filter(Boolean).join(' ')} style={{ ...p.style, ...style }} {...rest} />
  },
)

export const PasswordField = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean; toggleStyle?: CSSProperties; frameStyle?: CSSProperties; toggleIcon?: ReactNode }>(
  function PasswordField({ invalid, style, className, toggleStyle, frameStyle, toggleIcon, ...rest }, ref) {
    const [show, setShow] = useState(false)
    const p = stylex.props(sx.field, invalid ? sx.fieldErr : sx.fieldOk, sx.padded)
    return (
      <div {...stylex.props(sx.wrap, frameStyle && sx.field, frameStyle && sx.framed, frameStyle && inputFrame({ invalid, disabled: rest.disabled }))} style={frameStyle}>
        <input ref={ref} type={show ? 'text' : 'password'} aria-invalid={invalid || undefined}
          className={[(frameStyle ? stylex.props(sx.plain).className : p.className), className].filter(Boolean).join(' ')} style={{ ...p.style, ...style, ...(frameStyle ? { height: '100%', padding: 0, borderRadius: 0 } : {}) }} {...rest} />
        <button type="button" onClick={() => setShow((v) => !v)}
          aria-label={show ? getKitLabels().inputs.hidePassword : getKitLabels().inputs.showPassword}
          className={stylex.props(sx.eye).className} style={toggleStyle}>
          {toggleIcon ?? <Icon name={show ? 'eye' : 'eye-slash'} size={17} />}
        </button>
      </div>
    )
  },
)

export type TextAreaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  invalid?: boolean
  label?: ReactNode
  error?: string
  showCount?: boolean
  onSubmitValue?: (value: string) => void
  /** Bỏ khung trạng thái của ô (khi ô nằm trong một khung khác đã vẽ vòng focus). */
  bare?: boolean
}

export const TextArea = forwardRef<HTMLTextAreaElement, TextAreaProps>(
  function TextArea({ invalid, label, error, showCount = true, onSubmitValue, id, value, defaultValue,
    maxLength, onChange, onKeyDown, style, className, disabled, readOnly, bare = false,
    'aria-describedby': describedBy, 'aria-invalid': ariaInvalid, ...rest }, ref) {
    const generatedId = useId()
    const inputId = id ?? generatedId
    const counterId = `${inputId}-counter`
    const errorId = `${inputId}-error`
    const shortcutId = `${inputId}-shortcut`
    const [draft, setDraft] = useState(String(defaultValue ?? ''))
    const length = String(value ?? draft).length
    const limit = maxLength ?? 500
    const isInvalid = invalid || !!error || ariaInvalid === true || ariaInvalid === 'true'
    const count = stylex.props(sx.counter, length > limit * .9 ? sx.counterWarn : sx.counterNormal)
    const counter = showCount && <span id={counterId} className={`num ${count.className}`} style={count.style}>{length}/{limit}</span>
    const area = stylex.props(sx.field, sx.area, ...(bare ? [] : inputFrame({ invalid: isInvalid, disabled })), !bare && disabled && sx.frameDisabled)
    return (
      <div {...stylex.props(sx.col)}>
        {label ? <FieldLabel htmlFor={inputId} right={counter} size={10.5} style={{ alignItems: 'stretch' }}>{label}</FieldLabel>
          : counter && <div {...stylex.props(sx.counterRow)}>{counter}</div>}
        <textarea {...rest} ref={ref} id={inputId} value={value} defaultValue={defaultValue}
          disabled={disabled} readOnly={readOnly} maxLength={maxLength}
          className={['kit-frame', area.className, className].filter(Boolean).join(' ')}
          data-invalid={isInvalid || undefined} aria-invalid={isInvalid || ariaInvalid}
          aria-describedby={[describedBy, showCount && counterId, error && errorId, onSubmitValue && shortcutId].filter(Boolean).join(' ') || undefined}
          onChange={(event) => { if (value === undefined) setDraft(event.currentTarget.value); onChange?.(event) }}
          onKeyDown={(event) => {
            onKeyDown?.(event)
            if (!event.defaultPrevented && !event.nativeEvent.isComposing && event.key === 'Enter'
              && (event.metaKey || event.ctrlKey) && !event.shiftKey && !event.altKey
              && onSubmitValue && !disabled && !readOnly) {
              event.preventDefault()
              onSubmitValue(event.currentTarget.value)
            }
          }}
          style={{ ...area.style, ...(disabled ? { opacity: .45 } : {}), ...style }} />
        {error && <FieldError id={errorId}>{error}</FieldError>}
        {onSubmitValue && <span id={shortcutId} {...stylex.props(sx.hint)}>
          <Kbd>⌘</Kbd> <Kbd>Enter</Kbd> gửi · <Kbd>Shift</Kbd> <Kbd>Enter</Kbd> xuống dòng
        </span>}
      </div>
    )
  },
)

export function FieldError({ children, id, showIcon = true, style }: { children: ReactNode; id?: string; showIcon?: boolean; style?: CSSProperties }) {
  return (
    <span id={id} role="alert" {...stylex.props(sx.err)} style={style}>
      {showIcon && <Icon name="warning-2" size={14} />}
      {children}
    </span>
  )
}
