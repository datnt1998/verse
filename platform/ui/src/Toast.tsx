// Une seule hôte pour les notifications, y compris avant le montage du shell.
import { useEffect, useRef, useState, useSyncExternalStore, type CSSProperties, type FocusEvent, type TouchEvent } from 'react'
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
})

export type ToastAgent = 'scout' | 'director' | 'artist' | 'qa'
export type ToastItem = { id: number; key?: string; tone: Tone; text: string; /** nội dung dạng thẻ (toast.card) — mặc định không hết hạn */ card?: { title: string; text?: string; meta?: string; secondaryAction?: { label: string; onClick: () => void | Promise<void> }; agent?: ToastAgent }; action?: { label: string; onClick: () => void | Promise<void> }; expiresAt: number; duration: number; undo?: boolean; leaving: boolean; /** đang dừng đếm ngược (rê chuột vào chồng toast) — `remaining` là thời gian còn lại */ paused?: boolean; remaining?: number }
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
export type ToastCard = { title: string; text?: string; meta?: string; action: NonNullable<ToastItem['action']>; secondaryAction?: NonNullable<ToastItem['action']>; tone?: Tone; key?: string; duration?: number; /** chỉ nhánh activity: thẻ trợ lý (orb + tick mint) */ agent?: ToastAgent }
toast.card = (card: ToastCard) => {
  const duration = card.duration ?? Infinity
  const item: ToastItem = { id: ++seq, key: card.key, tone: card.tone ?? 'err', text: '', card: { title: card.title, text: card.text, meta: card.meta, secondaryAction: card.secondaryAction, agent: card.agent }, action: card.action, expiresAt: Date.now() + duration, duration, leaving: false, paused: held || undefined, remaining: held ? duration : undefined }
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
  const box = item.card
    ? stylex.props(sx.toastCard, sx.toastTone(item.tone === 'neutral' ? line['--line-4'] : c.bd))
    : stylex.props(sx.toast, item.tone === 'ok' ? sx.toastOk : item.tone === 'neutral' ? sx.toastNeutral : sx.toastTone(c.bd))
  const card = stylex.props(sx.card, !front && (held ? sx.backHeld : sx.back))
  const ring = stylex.props(sx.countdown)
  const anim = stylex.props(item.leaving ? sx.out : sx.in)
  const urgent = item.tone === 'err' || !!item.card
  return <div className={['ui-toast-wrap', item.leaving && 'ui-toast-leaving', wrap.className].filter(Boolean).join(' ')} style={wrap.style} onMouseEnter={() => holdToasts(true)} onMouseLeave={() => holdToasts(false)}><div className={[item.leaving ? 'ui-toast-out' : 'ui-toast-in', anim.className].filter(Boolean).join(' ')} style={anim.style}><div className={[card.className, box.className].filter(Boolean).join(' ')} role={urgent ? 'alert' : 'status'} aria-live={urgent ? 'assertive' : 'polite'} style={{ ...card.style, ...box.style }}>
    {item.card ? <>
      <span {...stylex.props(sx.cardHead)}>{TONE_ICON[item.tone] && <Icon name={TONE_ICON[item.tone]!} size={16} style={{ color: c.fg, flexShrink: 0 }} />}<span {...stylex.props(sx.cardTitle)}>{item.card.title}</span>
        <button type="button" className={`hov-ghost ${stylex.props(sx.close).className}`} aria-label={getKitLabels().toast.dismiss} disabled={item.leaving} onClick={() => dismissToast(item.id)}>
          <Icon name="add" size={14} style={{ transform: 'rotate(45deg)' }} />
        </button>
      </span>
      {item.card.text && <span {...stylex.props(sx.cardText)}>{item.card.text}</span>}
      <span {...stylex.props(sx.cardActions)}><button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => {
        setBusy(true)
        void Promise.resolve().then(() => item.action!.onClick()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : getKitLabels().toast.actionFailed) })
      }}>{item.action!.label}</button>{item.card.secondaryAction && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => {
        setBusy(true)
        void Promise.resolve().then(() => item.card!.secondaryAction!.onClick()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : getKitLabels().toast.actionFailed) })
      }}>{item.card.secondaryAction.label}</button>}</span>
      {item.card.meta && <span {...stylex.props(sx.cardMeta)}>{item.card.meta}</span>}
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

/* ─── Nhánh activity (Viderse, r35) ──────────────────────────────────────────
   Hai vùng cố định: viên kết quả thao tác giữa đáy cột nội dung (neo theo
   `[data-toast-anchor]` qua --vv-toast-left/--vv-toast-bottom, không neo thì
   giữa đáy màn) và chồng thẻ việc nền góc phải (máy tính) / rộng màn (điện
   thoại). Kính mờ `--vv-toast-glass`, đếm ngược = vòng mảnh quanh nút đóng,
   không dòng “Tự đóng sau…”. Noverse không dùng nhánh này — mọi --vv-* có
   fallback về token kit. */
const ACT_MOBILE = '@media (max-width: 759px)'
const ax = stylex.create({
  pillHost: (bottom: string) => ({
    position: 'fixed', left: 'var(--vv-toast-left, 50%)', bottom,
    transform: 'translateX(-50%)', zIndex: 1001,
    display: 'flex', flexDirection: 'column', alignItems: 'center',
    width: 'min(640px, calc(100% - 24px))', pointerEvents: 'none',
  }),
  pill: {
    display: 'inline-flex', alignItems: 'center', gap: 10, minHeight: 40,
    maxWidth: 'min(640px, calc(100vw - 28px))',
    paddingTop: 0, paddingBottom: 0, paddingLeft: 14, paddingRight: 6,
    borderRadius: 999, backgroundColor: 'var(--vv-toast, var(--kit-surface-toast))',
    backgroundImage: 'var(--vv-toast-glass, none)', backdropFilter: 'blur(18px) saturate(150%)',
    borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-toast-border, var(--kit-wash-12))',
    boxShadow: 'var(--vv-toast-shadow, var(--kit-shadow-toast))',
    fontSize: 13, color: color['--color-fg'], whiteSpace: 'nowrap',
  },
  pillTone: (border: string) => ({ borderColor: border }),
  pillCount: { fontVariantNumeric: 'tabular-nums', color: color['--color-fg-3'], fontWeight: 400 },
  closeActivity: { color: 'var(--vv-text-hint, var(--kit-fg-icon))' },
  stack: {
    position: 'fixed', zIndex: 1000, display: 'flex', flexDirection: 'column-reverse', gap: 8,
    left: { default: 'auto', [ACT_MOBILE]: 12 },
    right: 12,
    bottom: { default: 'calc(16px + env(safe-area-inset-bottom, 0px))', [ACT_MOBILE]: 'calc(80px + env(safe-area-inset-bottom, 0px))' },
    width: { default: 328, [ACT_MOBILE]: 'auto' },
    maxWidth: 'calc(100vw - 24px)',
  },
  tcard: {
    position: 'relative', overflow: 'hidden', display: 'flex', gap: 12,
    paddingTop: 12, paddingBottom: 13, paddingInline: 12,
    borderRadius: 18, backgroundColor: 'var(--vv-toast, var(--kit-surface-toast))',
    backgroundImage: 'var(--vv-toast-glass, none)', backdropFilter: 'blur(18px) saturate(150%)',
    borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-toast-border, var(--kit-wash-12))',
    boxShadow: 'var(--vv-toast-shadow, var(--kit-shadow-toast))',
    color: color['--color-fg'], textAlign: 'left', whiteSpace: 'normal',
  },
  tcardErr: { borderColor: 'var(--vv-danger-border, var(--kit-errf-30))' },
  tcardTint: {
    position: 'absolute', inset: 0, pointerEvents: 'none',
    backgroundImage: 'radial-gradient(120% 90% at 0% 0%, var(--vv-tcard-tint), transparent 55%)',
  },
  ticon: { position: 'relative', width: 34, height: 34, borderRadius: '50%', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  torb: { width: 22, height: 22, borderRadius: '50%' },
  tbadge: {
    position: 'absolute', right: -2, bottom: -2, width: 15, height: 15, borderRadius: '50%',
    backgroundColor: 'var(--vv-st-ready, var(--color-ok))',
    boxShadow: '0 0 0 2px var(--vv-toast, var(--kit-surface-toast))',
    color: color['--color-bg'], display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  tbody: { position: 'relative', flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 },
  ttitle: { fontSize: 13.5, fontWeight: 600, lineHeight: 1.4, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', paddingRight: 28 },
  ttext: { fontSize: 12, lineHeight: 1.45, color: 'var(--vv-text-3, var(--color-fg-3))', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' },
  tact: { display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 },
  primary: { backgroundColor: 'var(--vv-primary, var(--color-fg))', borderColor: 'transparent', color: 'var(--vv-primary-fg, var(--color-bg))', fontWeight: 600 },
  tclose: { position: 'absolute', top: 9, right: 9, color: 'var(--vv-text-hint, var(--kit-fg-icon))' },
  actIn: {
    animationName: { default: 'vv-tIn', [NO_MOTION]: 'none' },
    animationDuration: '.5s', animationTimingFunction: 'cubic-bezier(.18,.95,.25,1.15)', animationFillMode: 'both',
  },
  actOut: {
    animationName: { default: 'vv-tOut', [NO_MOTION]: 'none' },
    animationDuration: '.42s', animationTimingFunction: 'cubic-bezier(.5,0,.7,.2)', animationFillMode: 'both',
  },
})
// Icon và màu loại theo artboard Activity · view=toasts (tick-b · flag · danger · tick-circle).
const ACT_PILL_ICON: Partial<Record<Tone, string>> = { ok: 'tick-b', warn: 'flag', err: 'danger', info: 'info-circle', accent: 'magic-star' }
const ACT_CARD_ICON: Partial<Record<Tone, string>> = { ok: 'tick-circle', warn: 'flag', err: 'danger', info: 'info-circle', accent: 'magic-star' }
// Vệt loang (--vv-*-tint) và màu icon/vòng (--vv-*-arc / thang trạng thái) của r35.
const ACT_TINT: Record<Tone, { tint: string; fg: string; border?: string }> = {
  neutral: { tint: 'transparent', fg: 'var(--vv-text-2, var(--color-fg-2))' },
  accent: { tint: 'var(--vv-director-tint, var(--tint-accent))', fg: 'var(--vv-director-arc, var(--color-accent-soft))' },
  ok: { tint: 'var(--vv-st-ready-bg, var(--tint-ok))', fg: 'var(--vv-st-ready, var(--color-ok))' },
  warn: { tint: 'var(--vv-st-review-bg, var(--tint-warn))', fg: 'var(--vv-st-review, var(--color-warn))', border: 'var(--vv-st-review-border, var(--kit-warn-30))' },
  err: { tint: 'var(--vv-danger-soft, var(--tint-err))', fg: 'var(--vv-danger, var(--color-err))', border: 'var(--vv-danger-border, var(--kit-errf-30))' },
  info: { tint: 'var(--vv-info-12, var(--tint-info))', fg: 'var(--vv-scout-arc, var(--color-info))' },
}
const ACT_AGENT: Record<ToastAgent, { tint: string; fg: string; orb: string }> = {
  scout: { tint: 'var(--vv-scout-tint, var(--tint-info))', fg: 'var(--vv-scout-arc, var(--color-info))', orb: 'var(--vv-orb-scout)' },
  director: { tint: 'var(--vv-director-tint, var(--tint-accent))', fg: 'var(--vv-director-arc, var(--color-accent-soft))', orb: 'var(--vv-orb-director)' },
  artist: { tint: 'var(--vv-artist-tint, var(--tint-warn))', fg: 'var(--vv-artist-arc, var(--color-warn))', orb: 'var(--vv-orb-artist)' },
  qa: { tint: 'var(--vv-qa-tint-strong, var(--tint-ok))', fg: 'var(--vv-qa-arc, var(--color-ok))', orb: 'var(--vv-orb-qa)' },
}

function ActivityNotification({ item, front }: { item: ToastItem; front: boolean }) {
  const [now, setNow] = useState(Date.now)
  const [busy, setBusy] = useState(false)
  const touchY = useRef<number | null>(null)
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 120)
    return () => clearInterval(timer)
  }, [item.expiresAt])
  const agent = item.card?.agent
  const tint = agent ? ACT_AGENT[agent].tint : ACT_TINT[item.tone].tint
  const fg = agent ? ACT_AGENT[agent].fg : ACT_TINT[item.tone].fg
  const finite = Number.isFinite(item.duration)
  const left = item.paused ? item.remaining ?? item.duration : Math.max(0, item.expiresAt - now)
  const labels = getKitLabels().toast
  const closeTitle = !finite ? labels.staysUntilDismissed : item.paused ? labels.pausedAutoDismiss : labels.autoDismissAfter(Math.ceil(left / 1000))
  const run = (fn: () => void | Promise<void>) => {
    setBusy(true)
    void Promise.resolve().then(() => fn()).then(() => dismissToast(item.id), (error: unknown) => { setBusy(false); toast.err(error instanceof Error ? error.message : labels.actionFailed) })
  }
  const wrap = stylex.props(sx.wrap, item.leaving && sx.wrapLeaving)
  const anim = stylex.props(item.leaving ? ax.actOut : ax.actIn)
  const ring = stylex.props(sx.countdown)
  const ringSvg = finite && <svg aria-hidden="true" width="28" height="28" viewBox="0 0 28 28" {...stylex.props(sx.ring)}><circle cx="14" cy="14" r="12" fill="none" stroke="var(--vv-toast-ring, var(--line-2))" strokeWidth="1.6" /><circle className={`ui-toast-countdown ${ring.className}`} cx="14" cy="14" r="12" fill="none" stroke={fg} strokeWidth="1.6" strokeLinecap="round" strokeDasharray={CIRCUMFERENCE} strokeDashoffset={CIRCUMFERENCE * (1 - Math.max(0, Math.min(1, left / item.duration)))} style={{ opacity: item.paused ? 0.3 : 1 }} /></svg>
  const closeButton = (closeStyle: { className?: string; style?: CSSProperties }) => (
    <button type="button" className={`hov-ghost ui-toast-close ${closeStyle.className}`} style={closeStyle.style} aria-label={labels.dismiss} title={closeTitle} disabled={item.leaving} onClick={() => dismissToast(item.id)}>
      {ringSvg}
      <Icon name="add" size={14} style={{ position: 'relative', transform: 'rotate(45deg)' }} />
    </button>
  )
  const urgent = item.tone === 'err'
  const swipe = {
    onTouchStart: (event: TouchEvent<HTMLDivElement>) => { touchY.current = event.touches[0]?.clientY ?? null },
    onTouchEnd: (event: TouchEvent<HTMLDivElement>) => {
      const from = touchY.current
      touchY.current = null
      const to = event.changedTouches[0]?.clientY
      if (from != null && to != null && to - from > 48) dismissToast(item.id)
    },
  }
  if (!item.card) {
    // Viên kết quả thao tác: icon · câu một dòng · tối đa một nút · đóng có vòng.
    const pill = stylex.props(ax.pill, ax.pillTone(item.tone === 'ok' ? 'var(--vv-st-ready-28, var(--kit-ok-30))' : item.tone === 'warn' ? 'var(--vv-st-review-border, var(--kit-warn-30))' : item.tone === 'err' ? 'var(--vv-danger-border, var(--kit-errf-30))' : 'var(--vv-toast-border, var(--kit-wash-12))'))
    return <div className={['ui-toast-wrap', item.leaving && 'ui-toast-leaving', wrap.className].filter(Boolean).join(' ')} style={wrap.style}>
      <div className={['ui-toast-pill', item.leaving ? 'ui-toast-out' : 'ui-toast-in', anim.className].filter(Boolean).join(' ')} style={anim.style}>
        <div className={['ui-toast-glass', pill.className].filter(Boolean).join(' ')} role={urgent ? 'alert' : 'status'} aria-live={urgent ? 'assertive' : 'polite'} style={pill.style}>
          {ACT_PILL_ICON[item.tone] && <Icon name={ACT_PILL_ICON[item.tone]!} size={16} style={{ color: fg, flexShrink: 0 }} />}
          <span {...stylex.props(sx.text)}>{item.text}</span>
          {item.action && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${stylex.props(sx.action).className}`} onClick={() => run(item.action!.onClick)}>
            {item.action.label}{item.undo && <span {...stylex.props(ax.pillCount)}> · {Math.ceil(left / 1000)}s</span>}
          </button>}
          {closeButton(stylex.props(sx.close, ax.closeActivity))}
        </div>
      </div>
    </div>
  }
  // Thẻ việc nền: icon tròn 34 (hoặc orb trợ lý + tick mint) · tiêu đề 1 dòng ·
  // câu phụ ≤ 2 dòng · nút 28 · đóng góc trên phải. Thẻ lùi sau thu nhỏ/mờ.
  const card = stylex.props(ax.tcard, sx.card, item.tone === 'err' && ax.tcardErr, !front && (item.paused ? sx.backHeld : sx.back))
  const primaryStyle = stylex.props(sx.action, ax.primary)
  const ghostStyle = stylex.props(sx.action)
  return <div className={['ui-toast-wrap', item.leaving && 'ui-toast-leaving', wrap.className].filter(Boolean).join(' ')} style={wrap.style} {...swipe}>
    <div className={['ui-toast-card', item.leaving ? 'ui-toast-out' : 'ui-toast-in', anim.className].filter(Boolean).join(' ')} style={anim.style}>
      <div className={['ui-toast-glass', card.className].filter(Boolean).join(' ')} role={urgent ? 'alert' : 'status'} aria-live={urgent ? 'assertive' : 'polite'} style={{ ...card.style, '--vv-tcard-tint': tint } as CSSProperties}>
        <span {...stylex.props(ax.tcardTint)} aria-hidden="true" />
        <span {...stylex.props(ax.ticon)} style={{ color: fg, backgroundColor: tint }} aria-hidden="true">
          {agent ? <>
            <span {...stylex.props(ax.torb)} style={{ backgroundImage: ACT_AGENT[agent].orb }} />
            <span {...stylex.props(ax.tbadge)}><Icon name="tick-b" size={10} /></span>
          </> : ACT_CARD_ICON[item.tone] && <Icon name={ACT_CARD_ICON[item.tone]!} size={18} />}
        </span>
        <span {...stylex.props(ax.tbody)}>
          <span {...stylex.props(ax.ttitle)}>{item.card.title}</span>
          {item.card.text && <span {...stylex.props(ax.ttext)}>{item.card.text}</span>}
          <span {...stylex.props(ax.tact)}>
            {item.action && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={front ? `hov-ghost ${primaryStyle.className}` : `hov-ghost ${ghostStyle.className}`} style={front ? primaryStyle.style : ghostStyle.style} onClick={() => run(item.action!.onClick)}>{item.action.label}</button>}
            {item.card.secondaryAction && <button type="button" disabled={busy || item.leaving} aria-busy={busy} className={`hov-ghost ${ghostStyle.className}`} style={ghostStyle.style} onClick={() => run(item.card!.secondaryAction!.onClick)}>{item.card.secondaryAction.label}</button>}
          </span>
        </span>
        {closeButton(stylex.props(sx.close, ax.tclose))}
      </div>
    </div>
  </div>
}

/** Chồng toast của Viderse: viên giữa đáy + thẻ góc phải (tối đa 2 · điện thoại 1). */
export function ActivityToastHost({ style }: { style?: CSSProperties }) {
  const notices = useSyncExternalStore(subscribe, snapshot, snapshot)
  const [mobile, setMobile] = useState(() => window.matchMedia('(max-width: 759px)').matches)
  useEffect(() => {
    const media = window.matchMedia('(max-width: 759px)')
    const change = () => setMobile(media.matches)
    media.addEventListener('change', change)
    return () => media.removeEventListener('change', change)
  }, [])
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        const latest = [...items].reverse().find((item) => !item.leaving)
        if (latest) dismissToast(latest.id)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  const cap = mobile ? 1 : 2
  const pills = notices.filter((notice) => !notice.card)
  const cards = notices.filter((notice) => !!notice.card && !notice.leaving)
  // Thẻ thứ ba đẩy đi thẻ cũ nhất không phải Cần xử lý (thẻ đó ở lại tới khi đóng).
  const droppedCards = cards.filter((card) => card.tone !== 'err').slice(0, Math.max(0, cards.length - cap))
  const shown = cards.filter((card) => !droppedCards.includes(card))
  useEffect(() => {
    for (const pill of pills.slice(0, -1)) if (!pill.leaving) dismissToast(pill.id)
    for (const card of droppedCards) dismissToast(card.id)
  })
  const visiblePills = pills.filter((pill, index) => pill.leaving || index === pills.length - 1)
  const renderCards = [...shown, ...notices.filter((notice) => !!notice.card && notice.leaving)].reverse()
  const blur = (event: FocusEvent<HTMLDivElement>) => { if (!event.currentTarget.contains(event.relatedTarget)) holdToasts(false) }
  const hold = { onMouseEnter: () => holdToasts(true), onMouseLeave: () => holdToasts(false), onFocus: () => holdToasts(true), onBlur: blur }
  const pillHost = stylex.props(ax.pillHost(`var(--vv-toast-bottom, calc(${mobile ? 80 : 18}px + env(safe-area-inset-bottom, 0px)))`))
  const stack = stylex.props(ax.stack)
  return <>
    <div className={pillHost.className} style={{ ...pillHost.style, ...style }} {...hold}>
      {visiblePills.map((item) => <ActivityNotification key={item.id} item={item} front />)}
    </div>
    <div role="region" aria-label={getKitLabels().toast.notifications} className={stack.className} style={stack.style} {...hold}>
      {renderCards.map((item, index) => <ActivityNotification key={item.id} item={item} front={index === 0} />)}
    </div>
  </>
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
  if (activity) return <ActivityToastHost style={style} />
  const held = notices.some((notice) => notice.paused)
  const host = stylex.props(sx.host(`var(--m-toast-bottom,calc(${mobile ? 96 : 18}px + env(safe-area-inset-bottom, 0px)))`))
  return <div aria-label={getKitLabels().toast.notifications} className={host.className} style={{ ...host.style, ...style }}>
    {notices.map((item, index, visible) => <Notification key={item.id} item={item} held={held} front={index === visible.length - 1} />)}
  </div>
}
