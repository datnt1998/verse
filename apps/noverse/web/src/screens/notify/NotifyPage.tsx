import { useEffect, useRef, useState } from 'react'
import type { InboxNotification } from '../../api/types'
import { downloadJobFile, getNotifications, readNotifications } from '../../api/client'
import { helpTarget } from '../../commands/target'
import { useAction } from '../../data'
import { useToastBottom } from '../../mobile/toastBottom'
import { navigate, paths } from '../../router'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { groupNotifications, notificationMeta, notificationPath } from './logic'
import { NotifySettings } from './NotifySettings'
import { PermissionPanel } from './PermissionPanel'
import { NOTIFICATIONS_KEY, useNotificationInbox } from './useNotify'
import { useNotifyToast } from './useNotifyToast'
import './notify.css'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const VIEWS = ['inbox', 'prompt', 'system', 'denied', 'settings', 'lock', 'lockActions'] as const
type View = typeof VIEWS[number]

export function NotifyPage({ initialView, onBack }: { initialView?: string; onBack?: () => void }) {
  const [view, setView] = useState<View>(() => VIEWS.includes(initialView as View) ? initialView as View : 'inbox')
  const [granted, setGranted] = useState(() => 'Notification' in window && Notification.permission === 'granted')
  const notify = useNotifyToast()
  useEffect(() => setView(VIEWS.includes(initialView as View) ? initialView as View : 'inbox'), [initialView])
  useEffect(() => {
    const update = () => setGranted('Notification' in window && Notification.permission === 'granted')
    window.addEventListener('focus', update)
    return () => window.removeEventListener('focus', update)
  }, [])
  // Artboard: toast ở màn khoá nằm cao (bottom 760px); các màn khác theo mặc định của route.
  useToastBottom(view === 'lock' || view === 'lockActions' ? 760 : null, 1)
  const back = () => { if (view === 'settings' && initialView === 'settings') { if (onBack) onBack(); else window.history.back() } else if (view !== 'inbox') setView('inbox'); else if (onBack) onBack(); else window.history.back() }
  const enable = () => setView('Notification' in window && Notification.permission === 'denied' ? 'denied' : 'prompt')
  if (view === 'lock' || view === 'lockActions') return <LockPreview key={view} actions={view === 'lockActions'} onExit={() => setView('inbox')} />
  const preview = initialView === 'prompt' || initialView === 'system'
  return <section className="notify-page"><header className="notify-header"><button className="notify-tap" aria-label="Quay lại" onClick={back}><Icon name="arrow-left-2" size={22} /></button><strong>{view === 'inbox' ? 'Thông báo' : view === 'settings' ? 'Cài đặt thông báo' : ''}</strong>{view === 'inbox' && <InboxActions onSettings={() => setView('settings')} />}</header>{view === 'settings' ? <main className="notify-scroll"><NotifySettings /></main> : view === 'inbox' ? <Inbox showEnable={!granted} onEnable={enable} /> : <PermissionPanel denied={view === 'denied'} system={view === 'system'} forceIosHint={preview} onDenied={() => setView('denied')} onLater={() => setView('inbox')} onDone={() => { setGranted(true); setView('inbox'); notify('Đã bật thông báo trên máy này') }} />}</section>
}

function InboxActions({ onSettings }: { onSettings: () => void }) {
  const inbox = useNotificationInbox()
  const mark = useAction(readNotifications, { invalidate: [NOTIFICATIONS_KEY] })
  return <><button className="notify-read-all notify-tap" disabled={!inbox.data?.unread || mark.busy} onClick={() => void mark.run({ all: true })}>Đọc hết</button><button className="notify-header-settings notify-tap" aria-label="Cài đặt thông báo" onClick={onSettings}><Icon name="setting-2" size={21} /></button></>
}

function Inbox({ showEnable, onEnable }: { showEnable: boolean; onEnable: () => void }) {
  const inbox = useNotificationInbox()
  const [older, setOlder] = useState<InboxNotification[]>([])
  const [cursor, setCursor] = useState<string | undefined>()
  const [loaded, setLoaded] = useState(false)
  // Làm mới trang đầu khi SSE thay đổi để không giữ trạng thái đọc cũ trong các trang đã tải.
  const read = useAction(readNotifications, { invalidate: [NOTIFICATIONS_KEY] })
  const download = useAction(downloadJobFile, { success: 'Đã tải tệp' })
  const more = useAction(async () => {
    const response = await getNotifications(loaded ? cursor : inbox.data?.nextBefore)
    setOlder((items) => [...items, ...response.notifications])
    setCursor(response.nextBefore); setLoaded(true)
    return response
  })
  useEffect(() => { setOlder([]); setCursor(undefined); setLoaded(false) }, [inbox.updatedAt])
  const items = [...(inbox.data?.notifications ?? []), ...older].filter((item, index, all) => all.findIndex((other) => other.id === item.id) === index)
  const groups = groupNotifications(items)
  const open = (item: InboxNotification) => {
    if (!item.read) {
      setOlder((previous) => previous.map((old) => old.id === item.id ? { ...old, read: true } : old))
      void read.run({ ids: [item.id] })
    }
    const path = notificationPath(item)
    if (path.startsWith('/api/')) void download.run(path, item.title)
    else navigate(path)
  }
  return <main className="notify-scroll notify-inbox">{showEnable && <div className="notify-enable-card"><Icon name="notification" size={24} style={{ color: '#C9A6F5' }} /><span><b>Nhận thông báo trên điện thoại</b><small>Biết ngay khi chương chờ bạn duyệt</small></span><button className="notify-tap" onClick={onEnable}>Bật</button></div>}<PanelRefreshBar resourceKeys={[NOTIFICATIONS_KEY]} style={{ minHeight: 0 }} /><StaleBanner error={inbox.error} onRetry={inbox.refresh} />{!inbox.data && !inbox.error && <div className="notify-initial">{[1, 2, 3].map((key) => <Skeleton key={key} height={110} radius={16} />)}</div>}{inbox.data && !items.length && <div className="notify-empty"><Icon name="notification" size={40} /><h2>Chưa có thông báo</h2><p>Khi truyện cần bạn, thông báo sẽ xuất hiện ở đây.</p></div>}{groups.map((group) => <section key={group.label}><h3>{group.label}</h3><div className="notify-group">{group.items.map((item) => {
    const meta = notificationMeta[item.url.startsWith('/api/') ? 'download' : item.type] ?? notificationMeta.job
    return <button type="button" onClick={() => open(item)} className={`notify-inbox-row notify-press${!item.read ? ' is-unread' : ''}`} key={item.id}>{!item.read && <span className="notify-unread-dot" aria-label="Chưa đọc" />}<span className="notify-icon" style={{ color: meta.color, background: `${meta.color}1f` }}><Icon name={meta.icon} size={19} /></span><span className="notify-inbox-copy"><span className="notify-title-line"><b>{item.title}</b><time dateTime={item.createdAt}>{new Date(item.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</time></span><span>{item.body}</span><small>{meta.cta}{item.type === 'done' && item.chapter ? ` ${item.chapter}` : ''} <Icon name="arrow-right-2" size={12} /></small></span></button>
  })}</div></section>)}{(loaded ? cursor : inbox.data?.nextBefore) && <button className="notify-more" disabled={more.busy} onClick={() => void more.run()}>{more.pending ? 'Đang tải…' : 'Xem thông báo trước'}</button>}{inbox.data && <p className="notify-retention">Thông báo cũ hơn 30 ngày được tự xoá.</p>}</main>
}

/** Thông báo mẫu trên màn khoá (artboard `N.slice(0, 3)`): loại → đích sâu của truyện đang mở. */
const LOCK_ITEMS = [
  { type: 'gate', icon: 'flag-bulk', title: 'Chương 25 chờ bạn duyệt', body: 'Bản gốc trở về · Biên tập viên chấm 8,4', time: 'bây giờ' },
  { type: 'job', icon: 'headphone-bulk', title: 'Đã xuất sách nói Tập 1', body: 'tap-1.m4b · 9 chương · 118 MB', time: '34 phút trước' },
  { type: 'error', icon: 'danger', title: 'Truyện dừng do lỗi model', body: 'OpenRouter hết hạn mức · đã thử lại 3 lần', time: '3 giờ trước' },
] as const
const LOCK_ACTIONS = [
  ['Duyệt & viết tiếp', 'tick-circle', '#A6EDC1', 'Đã duyệt chương 25 · Người viết làm chương 26'],
  ['Nhờ viết lại…', 'edit-2', '#F4F2F8', 'Mở ô góp ý trong trình đọc'],
  ['Tắt loại thông báo này', 'close-circle-bulk', '#FF9FA8', 'Đã tắt “Chương chờ duyệt”'],
] as const
const LONG_PRESS_MS = 500

function lockTarget(type: typeof LOCK_ITEMS[number]['type']): string | null {
  const target = helpTarget()
  if (!target) return null
  if (type === 'gate') return `${paths.reader(target.project, target.story, 25)}?gate=1`
  if (type === 'job') return paths.audio(target.project, target.story, undefined, 'listen')
  return paths.workspace(target.project, target.story)
}

function LockPreview({ actions, onExit }: { actions: boolean; onExit: () => void }) {
  const [showActions, setShowActions] = useState(actions)
  const notify = useNotifyToast()
  const press = useRef<{ timer?: ReturnType<typeof setTimeout>; fired: boolean }>({ fired: false })
  useEffect(() => () => clearTimeout(press.current.timer), [])
  const startPress = (index: number) => {
    press.current.fired = false
    if (index) return
    clearTimeout(press.current.timer)
    press.current.timer = setTimeout(() => { press.current.fired = true; setShowActions(true) }, LONG_PRESS_MS)
  }
  const open = (type: typeof LOCK_ITEMS[number]['type']) => {
    if (press.current.fired) { press.current.fired = false; return }
    const path = lockTarget(type)
    if (path) navigate(path); else onExit()
  }
  return <section className="notify-page notify-lock-preview" aria-label="Màn hình khoá"><div className="notify-lock-date"><span>Thứ Bảy, 24 tháng 10</span><strong>14:32</strong></div><div className="notify-lock-items"><span className="notify-lock-hint">Chạm để mở đúng màn · nhấn giữ để thao tác nhanh</span>{LOCK_ITEMS.map((item, index) => <div className="notify-lock-item" key={item.title}><button type="button" className="notify-ln notify-tap" style={{ animationDelay: `${(index * 0.08).toFixed(2)}s` }} onClick={() => open(item.type)} onContextMenu={(event) => { event.preventDefault(); if (!index) setShowActions(true) }} onPointerDown={() => startPress(index)} onPointerUp={() => clearTimeout(press.current.timer)} onPointerLeave={() => clearTimeout(press.current.timer)} onPointerCancel={() => clearTimeout(press.current.timer)}><span className="notify-app-icon" aria-hidden="true"><Icon name={item.icon} size={19} /></span><span className="notify-ln-copy"><span><b>{item.title}</b><small>{item.time}</small></span><span>{item.body}</span></span></button>{!index && showActions && <div className="notify-lock-actions" role="menu" aria-label="Thao tác nhanh">{LOCK_ACTIONS.map(([label, icon, color, message]) => <button key={label} type="button" role="menuitem" style={{ color }} onClick={() => { setShowActions(false); notify(message) }}>{label}<Icon name={icon} size={18} /></button>)}</div>}</div>)}</div></section>
}
