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
  // Thẻ 300px (Cần bạn / Cần xử lý): tiêu đề video + một câu + đúng một nút việc, ở lại tới khi đóng.
  toastCard: {
    display: 'flex', flexDirection: 'column', gap: 8, width: 300, maxWidth: '100%',
    paddingTop: 12, paddingBottom: 12, paddingInline: 14,
    borderRadius: 14, backgroundColor: kit['--kit-surface-toast'],
    borderWidth: 1, borderStyle: 'solid',
    boxShadow: kit['--kit-shadow-toast'], fontSize: 13, color: color['--color-fg'],
  },
  activityCard: { borderRadius: 12, paddingBottom: 13 },
  activityIcon: { width: 28, height: 28, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 },
  activityIndented: { paddingLeft: 36 },
  activityActions: { justifyContent: 'flex-start', paddingLeft: 36 },
  cardHead: { display: 'flex', alignItems: 'flex-start', gap: 8 },
  cardTitle: { flex: '1 1 auto', minWidth: 0, fontSize: 13, fontWeight: 600, overflowWrap: 'anywhere' },
  cardText: { fontSize: 12, color: color['--color-fg-3'] },
  cardMeta: { fontSize: 11, color: color['--color-fg-3'] },
  cardActions: { display: 'flex', justifyContent: 'flex-end', gap: 6, marginTop: 2 },
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
  activityHost: { left: 'auto', right: 12, bottom: 'calc(16px + env(safe-area-inset-bottom, 0px))', transform: 'none', width: 300, alignItems: 'stretch', flexDirection: 'column-reverse' },
})

export type ToastItem = { id: number; key?: string; tone: Tone; text: string; /** nội dung dạng thẻ (toast.card) — mặc định không hết hạn */ card?: { title: string; text?: string; meta?: string; secondaryAction?: { label: string; onClick: () => void | Promise<void> } }; action?: { label: string; onClick: () => void | Promise<void> }; expiresAt: number; duration: number; undo?: boolean; leaving: boolean; /** đang dừng đếm ngược (rê chuột vào chồng toast) — `remaining` là thời gian còn lại */ paused?: boolean; remaining?: number }
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
    // Thẻ (toast.card) không có hẹn giờ — quay lại trạng thái chưa dừng.
    if (!Number.isFinite(item.expiresAt)) return { ...item, paused: false, remaining: undefined }
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
const push = (item: ToastItem) => {
  const previous = item.key === undefined ? undefined : items.find((notice) => notice.key === item.key)
  if (previous) { clearTimeout(timers.get(previous.id)); item.id = previous.id }
  items = previous ? items.map((notice) => notice.id === previous.id ? item : notice) : [...items, item]
  if (!held && Number.isFinite(item.duration)) timers.set(item.id, setTimeout(() => dismissToast(item.id), item.duration))
  publish()
  return item.id
}
export function toast(text: string, tone: Tone = 'neutral', action?: ToastItem['action'], options: ToastOptions = {}) {
  const duration = options.undo ? 5000 : options.duration ?? (typeof matchMedia !== 'undefined' && matchMedia('(max-width: 759px)').matches ? 2500 : 6000)
  const item: ToastItem = { id: ++seq, key: options.key, tone, text, action, expiresAt: Date.now() + duration, duration, undo: options.undo, leaving: false, paused: held || undefined, remaining: held ? duration : undefined }
  return push(item)
}
/** Thẻ thông báo mặc định ở lại tới khi đóng; có thể cung cấp hạn dùng và thao tác thứ hai. */
export type ToastCard = { title: string; text?: string; meta?: string; action: NonNullable<ToastItem['action']>; secondaryAction?: NonNullable<ToastItem['action']>; tone?: Tone; key?: string; duration?: number }
toast.card = (card: ToastCard) => {
  const duration = card.duration ?? Infinity
  const item: ToastItem = { id: ++seq, key: card.key, tone: card.tone ?? 'err', text: '', card: { title: card.title, text: card.text, meta: card.meta, secondaryAction: card.secondaryAction }, action: card.action, expiresAt: Date.now() + duration, duration, leaving: false, paused: held || undefined, remaining: held ? duration : undefined }
  return push(item)
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

function Notification({ item, front, held, activity }: { item: ToastItem; front: boolean; held: boolean; activity: boolean }) {
  const [now, setNow] = useState(Date.now)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 120)
    return () => clearInterval(timer)
  }, [item.expiresAt])
  const c = TONE[item.tone]
  const left = item.paused ? item.remaining ?? item.duration : Math.max(0, item.expiresAt - now)
  const wrap = stylex.props(sx.wrap, item.leaving && sx.wrapLeaving)
  const box = item.card
    ? stylex.props(sx.toastCard, activity && sx.activityCard, sx.toastTone(item.tone === 'neutral' ? line['--line-4'] : c.bd))
    : stylex.props(sx.toast, item.tone === 'ok' ? sx.toastOk : item.tone === 'neutral' ? sx.toastNeutral : sx.toastTone(c.bd))
  const card = stylex.props(sx.card, !activity && !front && (held ? sx.backHeld : sx.back))
  const ring = stylex.props(sx.countdown)
  const anim = stylex.props(item.leaving ? sx.out : sx.in)
  const urgent = item.tone === 'err' || (!!item.card && !activity)
  return <div className={['ui-toast-wrap', item.leaving && 'ui-toast-leaving', wrap.className].filter(Boolean).join(' ')} style={wrap.style} onMouseEnter={() => holdToasts(true)} onMouseLeave={() => holdToasts(false)}><div className={[item.leaving ? 'ui-toast-out' : 'ui-toast-in', anim.className].filter(Boolean).join(' ')} style={anim.style}><div className={[card.className, box.className].filter(Boolean).join(' ')} role={urgent ? 'alert' : 'status'} aria-live={urgent ? 'assertive' : 'polite'} style={{ ...card.style, ...box.style }}>
    {item.card ? <>
      <span {...stylex.props(sx.cardHead)}>{TONE_ICON[item.tone] && (activity ? <span {...stylex.props(sx.activityIcon)} style={{ color: c.fg, backgroundColor: c.bg }}><Icon name={item.tone === 'warn' ? 'flag' : item.tone === 'err' ? 'danger' : TONE_ICON[item.tone]!} size={17} /></span> : <Icon name={TONE_ICON[item.tone]!} size={16} style={{ color: c.fg, flexShrink: 0 }} />)}<span {...stylex.props(sx.cardTitle)}>{item.card.title}</span>
        <button type="button" className={`hov-ghost ${stylex.props(sx.close).className}`} aria-label={getKitLabels().toast.dismiss} disabled={item.leaving} onClick={() => dismissToast(item.id)}>
          <Icon name="add" size={14} style={{ transform: 'rotate(45deg)' }} />
        </button>
      </span>
      {item.card.text && <span {...stylex.props(sx.cardText, activity && sx.activityIndented)}>{item.card.text}</span>}
      <span {...stylex.props(sx.cardActions, activity && sx.activityActions)}><button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => {
        setBusy(true)
        void Promise.resolve().then(() => item.action!.onClick()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : getKitLabels().toast.actionFailed) })
      }}>{item.action!.label}</button>{item.card.secondaryAction && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => {
        setBusy(true)
        void Promise.resolve().then(() => item.card!.secondaryAction!.onClick()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : getKitLabels().toast.actionFailed) })
      }}>{item.card.secondaryAction.label}</button>}</span>
      {item.card.meta && <span {...stylex.props(sx.cardMeta, activity && sx.activityIndented)}>{item.card.meta}</span>}
    </> : <>
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
    </>}
  </div></div></div>
}
export function ToastHost({ style, activity = false }: { style?: CSSProperties; activity?: boolean }) {
  const notices = useSyncExternalStore(subscribe, snapshot, snapshot)
  const [mobile, setMobile] = useState(() => window.matchMedia('(max-width: 759px)').matches)
  useEffect(() => {
    const media = window.matchMedia('(max-width: 759px)')
    const change = () => setMobile(media.matches)
    media.addEventListener('change', change)
    return () => media.removeEventListener('change', change)
  }, [])
  useEffect(() => {
    if (!activity) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const latest = [...items].reverse().find((item) => !item.leaving)
        if (latest) dismissToast(latest.id)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activity])
  useEffect(() => {
    if (!activity) return
    const active = notices.filter((notice) => !notice.leaving)
    active.slice(0, -3).forEach((notice) => dismissToast(notice.id))
  }, [activity, notices])
  const held = notices.some((notice) => notice.paused)
  const host = stylex.props(sx.host(`var(--m-toast-bottom,calc(${mobile ? 96 : 18}px + env(safe-area-inset-bottom, 0px)))`), activity && !mobile && sx.activityHost)
  return <div aria-label={getKitLabels().toast.notifications} className={host.className} style={{ ...host.style, ...style }}>
    {(activity ? notices.slice(-3) : notices).map((item, index, visible) => <Notification key={item.id} item={item} held={held} front={index === visible.length - 1} activity={activity} />)}
  </div>
}
