import { createContext, useCallback, useContext, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { isNeedsConfig } from '../../../api/needsConfig'
import { NeedsConfigHint } from '../../../mobile/needsConfig'
import { Icon } from '@noverse/ui'

export function Tick({ size = 24 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true"><path className="mc-draw" d="M6 12.5l4 4 8-9" stroke="#A6EDC1" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" /></svg>
}
export function Orb({ live = false, size = 26, writer = false }: { live?: boolean; size?: number; writer?: boolean }) {
  return <span aria-hidden="true" className={`mc-orb ${writer ? 'mc-writer' : ''} ${live ? 'mc-live' : ''}`} style={{ width: size, height: size }} />
}
export function Button({ children, onClick, disabled, primary = false, className = '' }: { children: ReactNode; onClick?: () => void; disabled?: boolean; primary?: boolean; className?: string }) {
  return <button type="button" className={`mc-button ${primary ? 'mc-primary' : ''} ${className}`} onClick={onClick} disabled={disabled}>{children}</button>
}
export function Actions({ children, stack = false }: { children: ReactNode; stack?: boolean }) { return <div className={`mc-actions ${stack ? 'mc-actions-stack' : ''}`}>{children}</div> }
/** `split` = nhãn có số đếm ở mép phải (“XEM TRƯỚC CÁCH CHIA … 24”). */
export function Label({ children, split = false }: { children: ReactNode; split?: boolean }) { return <h2 className={`mc-label ${split ? 'mc-label-split' : ''}`}>{children}</h2> }
export function Loading({ text = 'Đang tải kết quả…' }: { text?: string }) {
  return <div className="mc-loading" role="status" aria-label="Đang phân tích"><span className="mc-loading-label"><span className="mc-spinner" aria-hidden="true" />{text}</span><div className="mc-stats">{[0, 1, 2, 3].map((key) => <div key={key} className="mc-skeleton" style={{ height: 76 }} />)}</div>{[0, 1, 2].map((key) => <div key={key} className="mc-skeleton" style={{ height: 72 }} />)}</div>
}
export function Failure({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  if (!error) return null
  return <div className="mc-failure" role="alert"><Icon name="warning-2" size={18} /><span>{error instanceof Error ? error.message : 'Không tải được kết quả. Vui lòng thử lại.'}<NeedsConfigHint error={error} /></span>{onRetry && !isNeedsConfig(error) && <button onClick={onRetry}>Thử lại</button>}</div>
}

/** Vòng tròn 24px đầu dòng điều kiện / bước: tick lục, dấu đóng đỏ, vòng quay hoặc nét đứt. */
export function Chk({ state }: { state: 'done' | 'bad' | 'now' | 'todo' }) {
  if (state === 'now') return <span className="mc-check"><span className="mc-spinner" aria-hidden="true" /></span>
  if (state === 'todo') return <span className="mc-check mc-pending" />
  return <span className={`mc-check ${state === 'done' ? 'mc-good' : 'mc-bad'}`}><Icon name={state === 'done' ? 'tick-circle' : 'close-circle-bulk'} size={state === 'done' ? 14 : 13} /></span>
}

// ─── Toast của MCmd: có liên kết “Xem”, nằm cao 112px so với đáy ─────────────

type Notify = (text: string, link?: (() => void) | null) => void
const ToastContext = createContext<Notify>(() => {})
export const useCmdToast = () => useContext(ToastContext)

export function CmdToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<{ id: number; text: string; link: (() => void) | null } | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const seq = useRef(0)
  const notify = useCallback<Notify>((text, link = null) => {
    clearTimeout(timer.current)
    const id = ++seq.current
    setToast({ id, text, link })
    timer.current = setTimeout(() => setToast((current) => current?.id === id ? null : current), 2800)
  }, [])
  useEffect(() => () => clearTimeout(timer.current), [])
  return <ToastContext.Provider value={notify}>{children}{toast && <div className="mc-overlay"><div className="mc-toast" role="status" key={toast.id}><Icon name="tick-circle" size={18} style={{ color: '#A6EDC1' }} /><span>{toast.text}</span>{toast.link && <button type="button" onClick={toast.link}>Xem</button>}</div></div>}</ToastContext.Provider>
}

// ─── Sheet & action sheet theo MCmd (grab 38×5, tiêu đề nằm trong nội dung) ─

/** Giữ phần tử gắn thêm 260ms sau khi đóng để chạy hoạt cảnh thoát. */
function useExit(open: boolean) {
  const [mounted, setMounted] = useState(open)
  useEffect(() => {
    if (open) { setMounted(true); return }
    const timer = setTimeout(() => setMounted(false), 260)
    return () => clearTimeout(timer)
  }, [open])
  return mounted
}

function useDialogFocus(mounted: boolean, onClose: () => void, ref: React.RefObject<HTMLElement | null>) {
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  useEffect(() => {
    if (!mounted) return
    const previous = document.activeElement as HTMLElement | null
    ref.current?.focus()
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); closeRef.current(); return }
      if (event.key !== 'Tab') return
      const focusable = ref.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), a[href]')
      if (!focusable?.length) { event.preventDefault(); return }
      const first = focusable[0], last = focusable[focusable.length - 1]
      if (event.shiftKey && (document.activeElement === first || document.activeElement === ref.current)) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && (document.activeElement === last || document.activeElement === ref.current)) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', key)
    return () => { document.removeEventListener('keydown', key); previous?.focus() }
  }, [mounted, ref])
}

export function CmdSheet({ open, onClose, title, lead, children }: { open: boolean; onClose: () => void; title: string; lead?: string; children: ReactNode }) {
  const mounted = useExit(open)
  const ref = useRef<HTMLElement>(null)
  const titleId = useId()
  useDialogFocus(mounted, onClose, ref)
  if (!mounted) return null
  return createPortal(<div className="mc-overlay"><div className={`mc-scrim ${open ? '' : 'mc-out'}`} aria-hidden="true" onClick={onClose} /><section ref={ref} tabIndex={-1} className={`mc-sheet ${open ? '' : 'mc-out'}`} role="dialog" aria-modal="true" aria-labelledby={titleId}><div className="mc-grab" aria-hidden="true" /><div className="mc-sheet-head"><span id={titleId} className="mc-sheet-title">{title}</span>{lead && <span className="mc-sheet-lead">{lead}</span>}</div>{children}</section></div>, document.body)
}

export function CmdActionSheet({ open, onClose, title, description, action, cancel = 'Huỷ' }: { open: boolean; onClose: () => void; title: string; description: string; action: { label: string; onSelect: () => void }; cancel?: string }) {
  const mounted = useExit(open)
  const ref = useRef<HTMLDivElement>(null)
  const titleId = useId()
  const descId = useId()
  useDialogFocus(mounted, onClose, ref)
  if (!mounted) return null
  return createPortal(<div className="mc-overlay"><div className={`mc-scrim ${open ? '' : 'mc-out'}`} aria-hidden="true" onClick={onClose} /><div ref={ref} tabIndex={-1} className={`mc-asheet ${open ? '' : 'mc-out'}`} role="alertdialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={descId}>
    <div className="mc-agrp"><div className="mc-agrp-head"><span id={titleId}>{title}</span><span id={descId}>{description}</span></div><button type="button" className="mc-press mc-agrp-action" onClick={action.onSelect}>{action.label}</button></div>
    <div className="mc-agrp"><button type="button" className="mc-press" onClick={onClose}>{cancel}</button></div>
  </div></div>, document.body)
}

export function Lamp() {
  return <Icon name="lamp-on" size={15} className="mc-lamp" />
}
