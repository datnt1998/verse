import { createContext, useCallback, useContext, useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Icon, Sheet, MobilePage as KitMobilePage, ActionSheet as KitActionSheet, ConfirmSheet as KitConfirmSheet } from '@noverse/ui'
import { useMobileScroll } from './runtime'
import { StorageBanner, StoragePill } from '../screens/settings/storage/indicators'

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

/** Khung một màn hình điện thoại — dựng bằng `MobilePage` của kit.
 *  Noverse vẫn tự theo dõi cuộn (đặt `data-tab-mode` trên `.m-app`) nên kit chỉ dựng khung;
 *  CSS `.m-page/.m-topbar/.m-title/.m-scroll/.m-footer` của app đè lên lớp kit nên
 *  từng điểm ảnh giữ nguyên. */
export function MobilePage({ title, backTitle, onBack, actions, children, footer, manageScroll = true }: {
  title: ReactNode; backTitle?: string; onBack?: () => void; actions?: ReactNode; children: ReactNode; footer?: ReactNode; manageScroll?: boolean
}) {
  const scrollRef = useRef<HTMLElement>(null)
  useMobileScroll(scrollRef, { disabled: !manageScroll })
  return <KitMobilePage
    title={title}
    backTitle={onBack ? backTitle ?? 'Quay lại' : undefined}
    onBack={onBack}
    headerExtra={<StoragePill mobile />}
    actions={actions}
    banner={<StorageBanner mobile />}
    footer={footer}
    manageScroll={false}
    scrollRef={scrollRef}
  >{children}</KitMobilePage>
}

export function BottomSheet({ open, onClose, title, label, children, height = 640, style, grab = true, role = 'dialog' }: {
  open: boolean; onClose: () => void; title?: string; label?: string; children: ReactNode; height?: number | string; style?: CSSProperties; grab?: boolean; role?: 'dialog' | 'alertdialog'
}) {
  return <Sheet open={open} onClose={onClose} title={title} label={label} height={height} style={style} grab={grab} role={role}>{children}</Sheet>
}

/** Tấm thao tác kiểu iOS của MMore — bản kit (`layout="grouped"`), CSS nhóm vẫn ở app. */
export function ActionSheet({ open, onClose, title, description, actions }: {
  open: boolean; onClose: () => void; title: string; description?: string; actions: { label: string; onSelect: () => void | Promise<void>; destructive?: boolean }[]
}) {
  return <KitActionSheet
    open={open}
    onClose={onClose}
    title={title}
    description={description}
    layout="grouped"
    actions={actions.map((action) => ({ label: action.label, destructive: action.destructive, onSelect: action.onSelect }))}
  />
}

/** Hộp xác nhận kiểu iOS của MMore — bản kit (`layout="grouped"`, `alertdialog`). */
export function ConfirmSheet({ open, onClose, title, description, action, cancel = 'Huỷ' }: {
  open: boolean; onClose: () => void; title: string; description: string; cancel?: string
  action: { label: string; destructive?: boolean; onSelect: () => void | Promise<void> }
}) {
  return <KitConfirmSheet open={open} onClose={onClose} title={title} description={description} action={action} cancel={cancel} layout="grouped" />
}

export function PushPage({ title, previousTitle, onBack, children }: { title: string; previousTitle: string; onBack: () => void; children: ReactNode }) {
  return <div className="m-push"><MobilePage title={title} backTitle={previousTitle} onBack={onBack}>{children}</MobilePage></div>
}
