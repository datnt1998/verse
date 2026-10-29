import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { createRoot } from 'react-dom/client'
import * as stylex from '@stylexjs/stylex'
import { color, line, surface, kit } from './tokens.stylex'
import { Button } from './Button'
import { Checkbox, RadioDot } from './controls'
import { AgentOrb, type AgentKind } from './AgentOrb'
import { Icon } from './Icon'
import { useEscapeLayer } from './Overlays'
import { toast } from './Toast'
import { useAsyncAction } from './useAsyncAction'
import { getKitLabels } from './labels'
import './kit.css'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const MOBILE = '@media (max-width: 760px)'

const sx = stylex.create({
  scrim: {
    position: 'fixed', inset: 0, backgroundColor: kit['--kit-scrim'], backdropFilter: 'blur(3px)',
    display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20, zIndex: 100,
    animationName: { default: 'kit-fade', [NO_MOTION]: 'none' },
    animationDuration: '.2s', animationTimingFunction: 'ease-out', animationFillMode: 'both',
  },
  scrimInline: { position: 'absolute', zIndex: 1 },
  scrimClosing: {
    animationName: { default: 'kit-fade-out', [NO_MOTION]: 'none' },
    animationDuration: '.2s', animationTimingFunction: 'ease-in', animationFillMode: 'both',
  },
  confirm: {
    width: 460, maxWidth: '100%', padding: 20, display: 'flex', flexDirection: 'column', gap: 16,
    backgroundColor: color['--color-surface-3'], borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], borderRadius: 18,
    boxShadow: surface['--dialog-shadow'],
    animationName: { default: 'kit-dialog', [NO_MOTION]: 'none' },
    animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.25,1.08)', animationFillMode: 'both',
    maxHeight: { default: 'none', [MOBILE]: 'calc(100dvh - 40px)' },
    overflow: { default: 'visible', [MOBILE]: 'auto' },
  },
  confirmClosing: {
    animationName: { default: 'kit-dialog-out', [NO_MOTION]: 'none' },
    animationDuration: '.18s', animationTimingFunction: 'cubic-bezier(.5,0,.75,0)', animationFillMode: 'both',
  },
  row: { display: 'flex', gap: 14, alignItems: 'flex-start' },
  iconChip: {
    width: 42, height: 42, borderRadius: 12, display: 'flex', alignItems: 'center',
    justifyContent: 'center', flex: '0 0 auto',
  },
  iconChipLook: (bg: string, fg: string) => ({ backgroundColor: bg, color: fg }),
  orbWrap: {
    width: 42, height: 42, borderRadius: '50%', backgroundImage: kit['--kit-orb-glow'],
    display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
  },
  leadCol: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  title: { margin: 0, fontSize: 16, fontWeight: 600, lineHeight: 1.4 },
  body: { fontSize: 13.5, color: kit['--kit-fg-body'] },
  bodyText: { margin: 0, fontSize: 13.5, color: kit['--kit-fg-body'] },
  choiceList: { display: 'flex', flexDirection: 'column', gap: 6 },
  choice: {
    display: 'flex', gap: 12, alignItems: 'flex-start', paddingTop: 10, paddingBottom: 10,
    paddingInline: 12, borderRadius: 11, borderWidth: 1, borderStyle: 'solid',
    color: color['--color-fg'], textAlign: 'left',
  },
  choiceOn: { borderColor: kit['--kit-acc-40'], backgroundColor: kit['--kit-acc-6'] },
  choiceOff: { borderColor: line['--line-3'], backgroundColor: 'transparent' },
  choiceCol: { display: 'flex', flexDirection: 'column' },
  choiceLabel: { fontSize: 13, fontWeight: 500 },
  choiceDesc: { fontSize: 12, color: kit['--kit-fg-icon'] },
  compare: { display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 },
  compareCell: {
    paddingTop: 10, paddingBottom: 10, paddingInline: 12, borderRadius: 10,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid',
    display: 'flex', flexDirection: 'column', gap: 2,
  },
  compareAfter: { borderColor: kit['--kit-ok-20'] },
  compareBefore: { borderColor: line['--line-1'] },
  compareLabel: { fontSize: 10.5 },
  compareLabelAfter: { color: color['--color-ok'] },
  compareSum: { fontSize: 12.5 },
  compareMeta: { fontSize: 11.5, color: color['--color-fg-4'] },
  consequences: {
    margin: 0, paddingTop: 10, paddingBottom: 10, paddingLeft: 30, paddingRight: 12,
    borderRadius: 11, backgroundColor: kit['--kit-err-5'],
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-errb-16'],
    display: 'flex', flexDirection: 'column', gap: 5, fontSize: 12.5,
    color: kit['--kit-fg-note'], listStyle: 'disc',
  },
  phraseCol: { display: 'flex', flexDirection: 'column', gap: 6 },
  shaking: {
    animationName: { default: 'kit-shake', [NO_MOTION]: 'none' },
    animationDuration: '.38s', animationTimingFunction: 'cubic-bezier(.36,.07,.19,.97)',
  },
  phraseLabel: { fontSize: 12.5, color: kit['--kit-fg-body'] },
  phraseChip: {
    color: color['--color-fg'], backgroundColor: line['--line-1'],
    paddingTop: 1, paddingBottom: 1, paddingInline: 5, borderRadius: 4,
  },
  typedWrap: { position: 'relative' },
  typed: {
    height: 36, paddingTop: 0, paddingBottom: 0, paddingLeft: 12, paddingRight: 36,
    borderRadius: 10, backgroundColor: color['--color-surface-0'],
    borderWidth: 1, borderStyle: 'solid', color: color['--color-fg'],
    fontSize: 13, width: '100%', outline: 'none',
    transition: { default: 'border-color .15s, box-shadow .15s', [NO_MOTION]: 'none' },
  },
  typedOk: {
    borderColor: { default: line['--line-4'], ':hover': kit['--kit-wash-18'], ':focus': kit['--kit-acc-55'] },
    boxShadow: { default: 'none', ':focus': kit['--kit-ring-acc'] },
  },
  typedBad: {
    borderColor: { default: kit['--kit-errb-55'], ':hover': kit['--kit-errb-55'], ':focus': kit['--kit-errb-55'] },
    boxShadow: { default: kit['--kit-ring-err'], ':focus': kit['--kit-ring-err'] },
  },
  tick: {
    position: 'absolute', right: 10, top: 10, color: color['--color-ok'],
    animationName: { default: 'kit-tick', [NO_MOTION]: 'none' },
    animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1.5)', animationFillMode: 'both',
  },
  hint: { fontSize: 11.5 },
  hintOk: { color: color['--color-ok'] },
  hintIdle: { color: color['--color-fg-4'] },
  error: {
    display: 'flex', gap: 10, alignItems: 'flex-start', paddingTop: 10, paddingBottom: 10,
    paddingInline: 12, borderRadius: 10, backgroundColor: kit['--kit-err-8'],
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-errb-25'],
    fontSize: 12.5, color: kit['--kit-err-text'],
  },
  errorIcon: { marginTop: 2 },
  note: { display: 'flex', gap: 8, alignItems: 'flex-start', fontSize: 12.5, color: color['--color-fg-3'] },
  footer: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  remember: { color: color['--color-fg-3'], fontSize: 12.5 },
  spacer: { flex: 1 },
  holdFill: (width: string, transition: string) => ({
    position: 'absolute', left: 0, top: 0, bottom: 0, width,
    backgroundColor: kit['--kit-wash-16'], pointerEvents: 'none', transition,
  }),
  holdLabel: { position: 'relative' },
  holdIcon: { position: 'relative' },
})

export type ConfirmTone = 'normal' | 'warn' | 'dsoft' | 'danger'
export type ConfirmChoice = { value: string; label: string; description?: string }
export type ConfirmCompare = { label: string; summary: ReactNode; meta?: ReactNode }
export type ConfirmDialogProps = {
  title: string; body: ReactNode; tone?: ConfirmTone; icon?: ReactNode; agent?: boolean | AgentKind
  confirmLabel: string; cancelLabel?: string; pendingLabel?: string
  choices?: ConfirmChoice[]; defaultChoice?: string; compare?: ConfirmCompare[]; consequences?: string[]
  phrase?: string; hold?: boolean
  remember?: { user: string; kind: string }
  /** Khi đặt: hành động phá hoại không thể kích hoạt bằng click, giữ hay phím — lý do hiện trong hộp. */
  disabledReason?: string
  secondary?: { label: string; onConfirm: () => void | Promise<void>; danger?: boolean; successMessage?: string }
  onConfirm: (choice: string) => void | Promise<void>; onClose: (accepted?: boolean) => void
  /** chuỗi cố định, hoặc hàm theo lựa chọn radio; trả undefined → không hiện toast */
  successMessage?: string | ((choice: string) => string | undefined); undo?: () => void; inline?: boolean
}
export const HOLD_DURATION = 1200
export const matchesConfirmation = (input: string, phrase: string) => input.trim() === phrase
/** Màu & icon mặc định của ô icon 42 px theo tone (chip/ic trong TONE của Kit.dc.html). */
const CHIP: Record<ConfirmTone, { bg: string; fg: string; icon: string }> = {
  normal: { bg: 'var(--kit-acc-12)', fg: 'var(--color-accent)', icon: 'info-circle' },
  warn: { bg: 'var(--kit-warn-12)', fg: 'var(--color-warn)', icon: 'warning-2' },
  dsoft: { bg: 'var(--kit-err-10)', fg: 'var(--kit-err-bright)', icon: 'danger' },
  danger: { bg: 'var(--kit-err-14)', fg: 'var(--kit-err-strong)', icon: 'danger' },
}
const rememberKey = (user: string, kind: string) => `ainovel.confirm.${encodeURIComponent(user)}.${encodeURIComponent(kind)}`
export function scheduleConfirmationHold(onComplete: () => void) {
  const timer = globalThis.setTimeout(onComplete, HOLD_DURATION)
  return () => globalThis.clearTimeout(timer)
}
export function isConfirmationRemembered(user: string, kind: string) {
  try { return localStorage.getItem(rememberKey(user, kind)) === '1' } catch { return false }
}

export function ConfirmDialog({ title, body, tone = 'normal', icon, agent, confirmLabel, cancelLabel, pendingLabel, choices, defaultChoice, compare, consequences, phrase, hold, disabledReason, remember, secondary, onConfirm, onClose, successMessage, undo, inline }: ConfirmDialogProps) {
  const L = getKitLabels()
  const cancelText = cancelLabel ?? L.cancel
  const pendingText = pendingLabel ?? L.confirm.pendingGeneric
  const id = useId()
  const dialog = useRef<HTMLDivElement>(null)
  const cancel = useRef<HTMLButtonElement>(null)
  const primary = useRef<HTMLButtonElement>(null)
  const mounted = useRef(true)
  const busyRef = useRef(false)
  const dismissed = useRef(false)
  const shakeTimer = useRef<number | undefined>(undefined)
  const closeTimer = useRef<number | undefined>(undefined)
  const cancelHold = useRef<(() => void) | undefined>(undefined)
  const [typed, setTyped] = useState('')
  const [choice, setChoice] = useState(defaultChoice ?? choices?.[0]?.value ?? '')
  const [dontAsk, setDontAsk] = useState(false)
  const [error, setError] = useState('')
  const [closing, setClosing] = useState(false)
  const [shaking, setShaking] = useState(false)
  const [holding, setHolding] = useState(false)
  const [holdLeft, setHoldLeft] = useState(String(HOLD_DURATION / 1000))
  useEffect(() => {
    if (!holding) return
    const started = Date.now()
    setHoldLeft(String(HOLD_DURATION / 1000))
    const timer = window.setInterval(() => setHoldLeft(String(Math.max(0, Math.ceil((HOLD_DURATION - (Date.now() - started)) / 100) / 10))), 40)
    return () => window.clearInterval(timer)
  }, [holding])
  const { pending, busy, run: execute } = useAsyncAction(async (action: () => void | Promise<void>) => {
    try { await action() }
    catch (cause) {
      if (mounted.current && !dismissed.current) setError(cause instanceof Error ? cause.message : L.confirm.actionFailed)
      throw cause
    }
    return true
  })
  const dangerous = tone === 'danger' || tone === 'dsoft'
  const chip = CHIP[tone]
  const matched = !phrase || matchesConfirmation(typed, phrase)
  const stopHold = useCallback(() => { cancelHold.current?.(); cancelHold.current = undefined; setHolding(false) }, [])
  const close = useCallback((accepted = false) => {
    dismissed.current = true
    stopHold(); setClosing(true)
    window.clearTimeout(closeTimer.current)
    closeTimer.current = window.setTimeout(() => onClose(accepted), 180)
  }, [onClose, stopHold])
  // Esc = Huỷ, và chỉ lớp trên cùng nhận (hộp "gõ/giữ để xác nhận": Esc = Huỷ — 17-kit).
  useEscapeLayer(!inline, () => close())
  useEffect(() => {
    mounted.current = true
    const previous = document.activeElement as HTMLElement | null
    const root = document.getElementById('root')
    const wasInert = root?.hasAttribute('inert')
    if (!inline && root) root.setAttribute('inert', '')
    if (!inline) (dangerous ? cancel.current : primary.current)?.focus()
    return () => {
      mounted.current = false
      cancelHold.current?.(); window.clearTimeout(closeTimer.current); window.clearTimeout(shakeTimer.current)
      if (!inline && root && !wasInert) root.removeAttribute('inert')
      previous?.focus()
    }
  }, [dangerous, inline])
  useEffect(() => {
    const release = () => stopHold()
    window.addEventListener('pointerup', release); window.addEventListener('pointercancel', release); window.addEventListener('keyup', release); window.addEventListener('blur', release)
    return () => { window.removeEventListener('pointerup', release); window.removeEventListener('pointercancel', release); window.removeEventListener('keyup', release); window.removeEventListener('blur', release) }
  }, [stopHold])
  const shake = () => { setShaking(true); window.clearTimeout(shakeTimer.current); shakeTimer.current = window.setTimeout(() => setShaking(false), 380) }
  const run = async (action = () => onConfirm(choice), message?: string | null) => {
    if (busyRef.current || closing) return
    if (!matched || disabledReason) { if (!matched) shake(); return }
    busyRef.current = true; setError(''); stopHold()
    try {
      const success = await execute(action)
      if (!success || !mounted.current || dismissed.current) return
      if (dontAsk && remember) { try { localStorage.setItem(rememberKey(remember.user, remember.kind), '1') } catch { /* preference unavailable */ } }
      const text = message !== undefined ? message : typeof successMessage === 'function' ? successMessage(choice) : successMessage
      if (text) toast(text, 'ok', undo ? { label: L.confirm.undo, onClick: undo } : undefined)
      close(true)
    } finally { busyRef.current = false }
  }
  const startHold = () => {
    if (busyRef.current || closing || cancelHold.current !== undefined || disabledReason) return
    setHolding(true)
    cancelHold.current = scheduleConfirmationHold(() => { cancelHold.current = undefined; void run() })
  }
  const scrim = stylex.props(sx.scrim, closing && sx.scrimClosing, inline && sx.scrimInline)
  const dlg = stylex.props(sx.confirm, closing && sx.confirmClosing)
  const content = <div className={scrim.className} style={scrim.style}>
    <div ref={dialog} role="alertdialog" aria-modal={!inline} aria-labelledby={`${id}-title`} aria-describedby={`${id}-body`} className={dlg.className} style={dlg.style} onKeyDown={(event) => {
      if (event.key === 'Enter' && !hold && !disabledReason) {
        if (phrase && !matched && event.target instanceof HTMLInputElement) { event.preventDefault(); shake(); return }
        if (dangerous && event.target === primary.current) { event.preventDefault(); return }
        if (!dangerous && event.target !== cancel.current && event.target instanceof HTMLElement && event.target.getAttribute('role') !== 'radio' && event.target !== dialog.current?.querySelector('[data-secondary]')) { event.preventDefault(); void run() }
      }
      if (event.key === 'Tab') {
        const focusable = Array.from(dialog.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), [href], [tabindex="0"]') ?? [])
        const first = focusable[0]; const last = focusable[focusable.length - 1]
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
      }
    }}>
      <div {...stylex.props(sx.row)}>
        {agent
          ? <span {...stylex.props(sx.orbWrap)}><AgentOrb agent={agent === true ? 'writer' : agent} size={24} /></span>
          : <span {...stylex.props(sx.iconChip, sx.iconChipLook(chip.bg, chip.fg))}>{icon ?? <Icon name={chip.icon} size={20} />}</span>}
        <div {...stylex.props(sx.leadCol)}>
          <h3 id={`${id}-title`} {...stylex.props(sx.title)}>{title}</h3>
          {typeof body === 'string' ? <p id={`${id}-body`} {...stylex.props(sx.bodyText)}>{body}</p> : <div id={`${id}-body`} {...stylex.props(sx.body)}>{body}</div>}
        </div>
      </div>
      {choices && <div role="radiogroup" aria-labelledby={`${id}-title`} {...stylex.props(sx.choiceList)}>{choices.map((item, index) => <button type="button" key={item.value} role="radio" aria-checked={choice === item.value} tabIndex={choice === item.value ? 0 : -1} onKeyDown={(e) => { if (['ArrowDown', 'ArrowRight', 'ArrowUp', 'ArrowLeft'].includes(e.key)) { e.preventDefault(); const next = choices[(index + (['ArrowDown', 'ArrowRight'].includes(e.key) ? 1 : choices.length - 1)) % choices.length]; setChoice(next.value); (e.currentTarget.parentElement?.children[choices.indexOf(next)] as HTMLElement)?.focus() } }} onClick={() => setChoice(item.value)} {...stylex.props(sx.choice, choice === item.value ? sx.choiceOn : sx.choiceOff)}><RadioDot checked={choice === item.value} style={{ marginTop: 2 }} /><span {...stylex.props(sx.choiceCol)}><span {...stylex.props(sx.choiceLabel)}>{item.label}</span>{item.description && <span {...stylex.props(sx.choiceDesc)}>{item.description}</span>}</span></button>)}</div>}
      {compare && <div {...stylex.props(sx.compare)}>{compare.map((item, index) => <div key={item.label} {...stylex.props(sx.compareCell, index ? sx.compareAfter : sx.compareBefore)}><span className="lbl" {...stylex.props(sx.compareLabel, index === 1 && sx.compareLabelAfter)} style={{ fontSize: 10.5 }}>{item.label}</span><span className={`num ${stylex.props(sx.compareSum).className}`} style={stylex.props(sx.compareSum).style}>{item.summary}</span><span {...stylex.props(sx.compareMeta)}>{item.meta}</span></div>)}</div>}
      {consequences && <ul {...stylex.props(sx.consequences)}>{consequences.map((item) => <li key={item}>{item}</li>)}</ul>}
      {phrase && <div {...stylex.props(sx.phraseCol, shaking && sx.shaking)}><label htmlFor={`${id}-typed`} {...stylex.props(sx.phraseLabel)}>{L.confirm.typeToConfirmPre} <span className="mono" style={{ color: 'var(--color-fg)', background: 'var(--line-1)', padding: '1px 5px', borderRadius: 4 }}>{phrase}</span> {L.confirm.typeToConfirmPost}</label><div {...stylex.props(sx.typedWrap)}><input id={`${id}-typed`} className={`mono kit-frame ${stylex.props(sx.typed, typed.length > 0 && !phrase.startsWith(typed) ? sx.typedBad : sx.typedOk).className}`} value={typed} autoComplete="off" spellCheck={false} placeholder={phrase} aria-invalid={typed.length > 0 && !phrase.startsWith(typed)} onChange={(e) => setTyped(e.target.value)} />{matched && <Icon name="tick-circle" size={16} {...stylex.props(sx.tick)} />}</div><span id={`${id}-typed-hint`} {...stylex.props(sx.hint, matched ? sx.hintOk : sx.hintIdle)}>{matched ? L.confirm.typedHintOk : L.confirm.typedHintIdle}</span></div>}
      {disabledReason && <div {...stylex.props(sx.note)}><Icon name="warning-2" size={15} style={{ color: 'var(--color-warn)', flexShrink: 0 }} /><span>{disabledReason}</span></div>}
      {error && <div role="alert" {...stylex.props(sx.error)}><Icon name="warning-2" size={15} {...stylex.props(sx.errorIcon)} /><span>{error}</span></div>}
      <div {...stylex.props(sx.footer)}>{remember && <Checkbox checked={dontAsk} onChange={setDontAsk} label={L.confirm.dontAskAgain} style={{ minHeight: 0, gap: 8, color: 'var(--color-fg-3)', fontSize: 12.5, whiteSpace: 'nowrap' }} />}<span {...stylex.props(sx.spacer)} /><Button ref={cancel} onClick={() => close()}>{cancelText}</Button>{secondary && <Button data-secondary variant={secondary.danger ? 'dsoft' : 'ghost'} disabled={busy} onClick={() => void run(secondary.onConfirm, secondary.successMessage ?? null)}>{secondary.label}</Button>}
      <Button ref={primary} variant={hold ? 'dsoft' : tone === 'danger' ? 'danger' : tone === 'dsoft' ? 'dsoft' : 'primary'} disabled={!matched || !!disabledReason || closing || busy} pending={pending} pendingLabel={pendingText} aria-label={hold ? L.confirm.holdToConfirm : undefined} icon={hold ? <Icon name="logout" size={15} {...stylex.props(sx.holdIcon)} /> : undefined} onPointerDown={hold ? (e) => { if (e.button === 0) startHold() } : undefined} onPointerLeave={hold ? stopHold : undefined} onKeyDown={hold ? (e) => { if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); if (!e.repeat) startHold() } } : undefined} onClick={hold ? (e) => e.preventDefault() : () => void run()} style={hold ? { overflow: 'hidden' } : undefined}>{hold && <i aria-hidden="true" {...stylex.props(sx.holdFill(holding ? '100%' : '0%', holding ? `width ${HOLD_DURATION}ms linear` : 'width .25s ease-out'))} />}<span {...stylex.props(sx.holdLabel)}>{error ? L.confirm.retry : hold ? (holding ? L.confirm.holdMore(holdLeft) : confirmLabel) : confirmLabel}</span></Button></div>
    </div>
  </div>
  return inline ? content : createPortal(content, document.body)
}

export type ConfirmOptions = Omit<ConfirmDialogProps, 'onClose'>
export function confirm(options: ConfirmOptions): Promise<boolean> {
  if (options.remember && isConfirmationRemembered(options.remember.user, options.remember.kind)) return Promise.resolve().then(() => options.onConfirm(options.defaultChoice ?? options.choices?.[0]?.value ?? '')).then(() => true)
  return new Promise((resolve) => {
    const container = document.createElement('div'); document.body.append(container)
    const root = createRoot(container)
    root.render(<ConfirmDialog {...options} onClose={(accepted) => { root.unmount(); container.remove(); resolve(accepted ?? false) }} />)
  })
}
