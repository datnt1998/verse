import { createContext, useCallback, useContext, useEffect, useId, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Icon, Sheet, useEscapeLayer } from '@noverse/ui'
import { useMobileScroll } from './runtime'

const ToastContext = createContext<(text: string) => void>(() => {})
export const useMobileToast = () => useContext(ToastContext)
export function MobileFeedback({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<{ text: string; id: number } | null>(null)
  const notify = useCallback((text: string) => setMessage({ text, id: Date.now() }), [])
  useEffect(() => {
    if (!message) return
    const timer = setTimeout(() => setMessage(null), 2500)
    return () => clearTimeout(timer)
  }, [message])
  return <ToastContext.Provider value={notify}>{children}{message && <div className="m-toast" role="status" key={message.id}><Icon name="tick-circle" size={18} style={{ color: '#A6EDC1' }} /><span style={{ flex: '1 1 auto' }}>{message.text}</span></div>}</ToastContext.Provider>
}

export function MobilePage({ title, backTitle, onBack, actions, children, footer, manageScroll = true }: {
  title: ReactNode; backTitle?: string; onBack?: () => void; actions?: ReactNode; children: ReactNode; footer?: ReactNode; manageScroll?: boolean
}) {
  const scrollRef = useRef<HTMLElement>(null)
  useMobileScroll(scrollRef, { disabled: !manageScroll })
  return <section className="m-page"><header className="m-topbar">{onBack && <button className="m-back" onClick={onBack}><Icon name="arrow-left-2" size={22} />{backTitle ?? 'Quay lại'}</button>}<div className="m-title">{title}</div>{actions}</header><main ref={scrollRef} className="m-scroll">{children}</main>{footer && <div className="m-footer">{footer}</div>}</section>
}

export function BottomSheet({ open, onClose, title, label, children, height = 640, style, grab = true, role = 'dialog' }: {
  open: boolean; onClose: () => void; title?: string; label?: string; children: ReactNode; height?: number | string; style?: CSSProperties; grab?: boolean; role?: 'dialog' | 'alertdialog'
}) {
  return <Sheet open={open} onClose={onClose} title={title} label={label} height={height} style={style} grab={grab} role={role}>{children}</Sheet>
}

export function ActionSheet({ open, onClose, title, description, actions }: {
  open: boolean; onClose: () => void; title: string; description?: string; actions: { label: string; onSelect: () => void | Promise<void>; destructive?: boolean }[]
}) {
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  return <BottomSheet open={open} onClose={() => { if (!pending) onClose() }} height="auto" style={{ background: 'transparent', border: 0, boxShadow: 'none' }}><div className="m-action-group"><h2>{title}</h2>{description && <p>{description}</p>}{failure && <p className="m-error" role="alert">{failure}</p>}{actions.map((action) => <button key={action.label} disabled={pending} className={action.destructive ? 'm-danger-text' : ''} onClick={async () => { setPending(true); setFailure(''); try { await action.onSelect(); onClose() } catch (error) { setFailure(error instanceof Error ? error.message : 'Không thực hiện được. Vui lòng thử lại.') } finally { setPending(false) } }}>{action.label}</button>)}</div><button className="m-action-cancel" disabled={pending} onClick={onClose}>{pending ? 'Đang thực hiện…' : 'Huỷ'}</button></BottomSheet>
}

/** Hộp xác nhận kiểu iOS của MMore: một nhóm (tiêu đề + mô tả + một hành động) và nút Huỷ riêng, nền trong suốt, không có thanh kéo. */
export function ConfirmSheet({ open, onClose, title, description, action, cancel = 'Huỷ' }: {
  open: boolean; onClose: () => void; title: string; description: string; cancel?: string
  action: { label: string; destructive?: boolean; onSelect: () => void | Promise<void> }
}) {
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const id = useId()
  const run = async () => {
    setPending(true); setFailure('')
    try { await action.onSelect(); onClose() }
    catch (error) { setFailure(error instanceof Error ? error.message : 'Không thực hiện được. Vui lòng thử lại.') }
    finally { setPending(false) }
  }
  return <BottomSheet open={open} onClose={() => { if (!pending) onClose() }} height="auto" grab={false} role="alertdialog" label={title} style={{ background: 'transparent', border: 0, boxShadow: 'none', padding: '0 10px 30px', gap: 10 }}>
    <div className="m-confirm-group" aria-describedby={id}>
      <div className="m-confirm-head"><span>{title}</span><span id={id}>{description}</span>{failure && <span className="m-error" role="alert">{failure}</span>}</div>
      <button type="button" className={`m-confirm-act${action.destructive ? ' m-confirm-danger' : ''}`} disabled={pending} aria-busy={pending} onClick={() => { void run() }}>{action.label}</button>
    </div>
    <div className="m-confirm-group"><button type="button" className="m-confirm-act m-confirm-cancel" disabled={pending} onClick={onClose}>{cancel}</button></div>
  </BottomSheet>
}

export function PushPage({ title, previousTitle, onBack, children }: { title: string; previousTitle: string; onBack: () => void; children: ReactNode }) {
  return <div className="m-push"><MobilePage title={title} backTitle={previousTitle} onBack={onBack}>{children}</MobilePage></div>
}
