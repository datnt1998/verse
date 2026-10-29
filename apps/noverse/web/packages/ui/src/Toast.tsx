// Une seule hôte pour les notifications, y compris avant le montage du shell.
import { useEffect, useState, useSyncExternalStore, type CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Icon } from './Icon'
import { TONE, type Tone } from './Chip'
import './Toast.css'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const sx = stylex.create({
  wrap: { maxWidth: '100%', maxHeight: 240, paddingTop: 8, pointerEvents: 'auto' },
  wrapLeaving: {
    overflow: 'hidden',
    animationName: { default: 'ui-toast-collapse', [NO_MOTION]: 'none' },
    animationDuration: '.32s', animationTimingFunction: 'cubic-bezier(.4,0,.2,1)', animationDelay: '.2s', animationFillMode: 'both',
  },
  in: {
    animationName: { default: 'ui-toast-in', [NO_MOTION]: 'none' },
    animationDuration: '.4s', animationTimingFunction: 'cubic-bezier(.2,.9,.25,1.12)', animationFillMode: 'both',
  },
  out: {
    animationName: { default: 'ui-toast-out', [NO_MOTION]: 'none' },
    animationDuration: '.42s', animationTimingFunction: 'cubic-bezier(.5,0,.7,.2)', animationFillMode: 'both',
    transformOrigin: 'calc(100% - 40px) 100%',
  },
  card: {
    transformOrigin: '50% 100%',
    transition: { default: 'transform .4s cubic-bezier(.2,.9,.25,1), opacity .4s, filter .4s', [NO_MOTION]: 'none' },
  },
  back: { transform: 'translateY(4px) scale(.955)', opacity: .6, filter: 'saturate(.7)' },
  backHeld: { transform: 'none', opacity: 1, filter: 'none' },
  toast: {
    display: 'flex', alignItems: 'center', gap: 10, minHeight: 40,
    paddingTop: 0, paddingBottom: 0, paddingLeft: 14, paddingRight: 8,
    maxWidth: '100%', borderRadius: 999, backgroundColor: kit['--kit-surface-toast'],
    borderWidth: 1, borderStyle: 'solid',
    boxShadow: kit['--kit-shadow-toast'], fontSize: 13, color: color['--color-fg'],
  },
  toastOk: { borderColor: kit['--kit-ok-28'] },
  toastNeutral: { borderColor: line['--line-4'] },
  toastTone: (border: string) => ({ borderColor: border }),
  text: { minWidth: 0, paddingTop: 8, paddingBottom: 8, paddingInline: 0 },
  action: {
    flexShrink: 0, height: 28, paddingInline: 10, borderRadius: 8,
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-12'],
    backgroundColor: 'transparent', color: color['--color-fg'],
    fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap',
  },
  close: {
    position: 'relative', flexShrink: 0, width: 28, height: 28, borderRadius: '50%',
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: kit['--kit-fg-icon'],
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  ring: { position: 'absolute', inset: 0, transform: 'rotate(-90deg)' },
  countdown: { transition: 'stroke-dashoffset .16s linear, opacity .3s ease' },
  host: (bottom: string) => ({
    position: 'fixed', left: '50%', bottom, transform: 'translateX(-50%)', zIndex: 1000,
    display: 'flex', flexDirection: 'column', alignItems: 'center',
    width: 'min(640px, calc(100% - 28px))', pointerEvents: 'none',
  }),
})

export type ToastItem = { id: number; key?: string; tone: Tone; text: string; action?: { label: string; onClick: () => void | Promise<void> }; expiresAt: number; duration: number; undo?: boolean; leaving: boolean; /** đang dừng đếm ngược (rê chuột vào chồng toast) — `remaining` là thời gian còn lại */ paused?: boolean; remaining?: number }
export type ToastOptions = { key?: string; duration?: number; undo?: boolean }
let seq = 0
let items: ToastItem[] = []
const listeners = new Set<() => void>()
const timers = new Map<number, ReturnType<typeof setTimeout>>()
const publish = () => listeners.forEach((listener) => listener())
let held = false
/** Rê chuột vào chồng toast: dừng đếm ngược của tất cả, rời chuột thì chạy tiếp (14-jobs-dock §1.4). */
export function holdToasts(hold: boolean) {
  if (hold === held) return
  held = hold
  items = items.map((item) => {
    if (item.leaving) return item
    if (hold) { clearTimeout(timers.get(item.id)); return { ...item, paused: true, remaining: Math.max(0, item.expiresAt - Date.now()) } }
    const remaining = item.remaining ?? item.duration
    timers.set(item.id, setTimeout(() => dismissToast(item.id), remaining))
    return { ...item, paused: false, remaining: undefined, expiresAt: Date.now() + remaining }
  })
  publish()
}
export function dismissToast(id: number) {
  if (!items.some((item) => item.id === id && !item.leaving)) return
  clearTimeout(timers.get(id))
  items = items.map((item) => item.id === id ? { ...item, leaving: true } : item)
  publish()
  const delay = typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 520
  timers.set(id, setTimeout(() => {
    timers.delete(id)
    items = items.filter((item) => item.id !== id)
    publish()
  }, delay))
}
export function toast(text: string, tone: Tone = 'neutral', action?: ToastItem['action'], options: ToastOptions = {}) {
  const duration = options.undo ? 5000 : options.duration ?? (typeof matchMedia !== 'undefined' && matchMedia('(max-width: 759px)').matches ? 2500 : 6000)
  const previous = options.key === undefined ? undefined : items.find((item) => item.key === options.key)
  const item: ToastItem = { id: previous?.id ?? ++seq, key: options.key, tone, text, action, expiresAt: Date.now() + duration, duration, undo: options.undo, leaving: false, paused: held || undefined, remaining: held ? duration : undefined }
  if (previous) clearTimeout(timers.get(previous.id))
  items = previous ? items.map((notice) => notice.id === previous.id ? item : notice) : [...items, item]
  if (!held) timers.set(item.id, setTimeout(() => dismissToast(item.id), duration))
  publish()
  return item.id
}
toast.ok = (text: string) => toast(text, 'ok')
toast.warn = (text: string) => toast(text, 'warn')
toast.err = (text: string) => toast(text, 'err')
toast.info = (text: string) => toast(text, 'info')
const subscribe = (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener) } }
const snapshot = () => items
// Biểu tượng theo artboard: Kit (xong = tick-circle mint), Jobs (lỗi = warning-2, cần bạn/thông tin = info-circle).
const TONE_ICON: Partial<Record<Tone, string>> = { ok: 'tick-circle', warn: 'info-circle', err: 'warning-2', info: 'info-circle', accent: 'magic-star' }
const CIRCUMFERENCE = 2 * Math.PI * 12

function Notification({ item, front, held }: { item: ToastItem; front: boolean; held: boolean }) {
  const [now, setNow] = useState(Date.now)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 120)
    return () => clearInterval(timer)
  }, [item.expiresAt])
  const c = TONE[item.tone]
  const left = item.paused ? item.remaining ?? item.duration : Math.max(0, item.expiresAt - now)
  const wrap = stylex.props(sx.wrap, item.leaving && sx.wrapLeaving)
  const box = stylex.props(sx.toast, item.tone === 'ok' ? sx.toastOk : item.tone === 'neutral' ? sx.toastNeutral : sx.toastTone(c.bd))
  const card = stylex.props(sx.card, !front && (held ? sx.backHeld : sx.back))
  const ring = stylex.props(sx.countdown)
  const anim = stylex.props(item.leaving ? sx.out : sx.in)
  return <div className={['ui-toast-wrap', item.leaving && 'ui-toast-leaving', wrap.className].filter(Boolean).join(' ')} style={wrap.style} onMouseEnter={() => holdToasts(true)} onMouseLeave={() => holdToasts(false)}><div className={[item.leaving ? 'ui-toast-out' : 'ui-toast-in', anim.className].filter(Boolean).join(' ')} style={anim.style}><div className={[card.className, box.className].filter(Boolean).join(' ')} role={item.tone === 'err' ? 'alert' : 'status'} aria-live={item.tone === 'err' ? 'assertive' : 'polite'} style={{ ...card.style, ...box.style }}>
    {TONE_ICON[item.tone] && <Icon name={TONE_ICON[item.tone]!} size={16} style={{ color: c.fg, flexShrink: 0 }} />}
    <span {...stylex.props(sx.text)}>{item.text}</span>
    {item.action && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => {
      setBusy(true)
      void Promise.resolve().then(() => item.action!.onClick()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : getKitLabels().toast.actionFailed) })
    }}>
      {item.action.label}{item.undo && <span aria-hidden="true"> · {Math.ceil(left / 1000)}s</span>}
    </button>}
    <button type="button" className={`hov-ghost ui-toast-close ${stylex.props(sx.close).className}`} aria-label={getKitLabels().toast.dismiss} title={item.paused ? getKitLabels().toast.pausedAutoDismiss : getKitLabels().toast.autoDismissAfter(Math.ceil(left / 1000))} disabled={item.leaving} onClick={() => dismissToast(item.id)}>
      <svg aria-hidden="true" width="28" height="28" viewBox="0 0 28 28" {...stylex.props(sx.ring)}><circle cx="14" cy="14" r="12" fill="none" stroke="var(--line-2)" strokeWidth="1.6" /><circle className={`ui-toast-countdown ${ring.className}`} cx="14" cy="14" r="12" fill="none" stroke={c.fg} strokeWidth="1.6" strokeLinecap="round" strokeDasharray={CIRCUMFERENCE} strokeDashoffset={CIRCUMFERENCE * (1 - Math.max(0, Math.min(1, left / item.duration)))} style={{ opacity: item.paused ? 0.3 : 1 }} /></svg>
      <Icon name="add" size={14} style={{ position: 'relative', transform: 'rotate(45deg)' }} />
    </button>
  </div></div></div>
}
export function ToastHost({ style }: { style?: CSSProperties }) {
  const notices = useSyncExternalStore(subscribe, snapshot, snapshot)
  const [mobile, setMobile] = useState(() => window.matchMedia('(max-width: 759px)').matches)
  useEffect(() => {
    const media = window.matchMedia('(max-width: 759px)')
    const change = () => setMobile(media.matches)
    media.addEventListener('change', change)
    return () => media.removeEventListener('change', change)
  }, [])
  const held = notices.some((notice) => notice.paused)
  const host = stylex.props(sx.host(`var(--m-toast-bottom,calc(${mobile ? 96 : 18}px + env(safe-area-inset-bottom, 0px)))`))
  return <div aria-label={getKitLabels().toast.notifications} className={host.className} style={{ ...host.style, ...style }}>
    {notices.map((item, index) => <Notification key={item.id} item={item} held={held} front={index === notices.length - 1} />)}
  </div>
}
