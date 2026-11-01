// Danh sách Thông báo dùng chung: popover máy tính (ActivityPopover) và tab
// Thông báo của sheet Việc nền trên điện thoại (QueueSummarySheet).
// Một nguồn dữ liệu (GET /api/notifications), một cách sắp và một cách vẽ dòng,
// nên hai nơi không thể lệch câu chữ hay thứ tự (08-dev-prompt V32).
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { getNotificationHistory, markNotificationsRead, type AppNotification } from '../api/activity'
import { keys } from '../data/keys'
import { useResource, type Resource } from '../data/useResource'
import { notificationDetail } from './activityText'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../styles/viderse.stylex'

/** Chưa đọc trước, rồi theo loại việc, rồi mới nhất trước. */
export function orderNotifications(notifications: AppNotification[]) {
  const priority: Record<AppNotification['kind'], number> = { review_ready: 0, needs_attention: 1, ready_to_post: 2, task_done: 3, action_result: 4 }
  return [...notifications].sort((a, b) => Number(!!a.readAt) - Number(!!b.readAt) || priority[a.kind] - priority[b.kind] || Date.parse(b.createdAt) - Date.parse(a.createdAt))
}

/** Nhóm Hôm nay · Hôm qua · Trước đó — cùng mốc cho cả hai nơi. */
export function groupNotifications(rows: AppNotification[], now = new Date()) {
  const today = new Date(now); today.setHours(0, 0, 0, 0)
  const yesterday = today.getTime() - 86400000
  return [
    { label: 'Hôm nay', rows: rows.filter((n) => Date.parse(n.createdAt) >= today.getTime()) },
    { label: 'Hôm qua', rows: rows.filter((n) => Date.parse(n.createdAt) >= yesterday && Date.parse(n.createdAt) < today.getTime()) },
    { label: 'Trước đó', rows: rows.filter((n) => Date.parse(n.createdAt) < yesterday) },
  ]
}

export type NotificationInbox = {
  history: Resource<Awaited<ReturnType<typeof getNotificationHistory>>>
  rows: AppNotification[]
  groups: { label: string; rows: AppNotification[] }[]
  /** Còn trang cũ để "Xem thêm" không. */
  more: string | undefined
  paging: boolean
  pageError: string
  loadMore: () => Promise<void>
  mark: (ids: string[] | 'all') => Promise<void>
  refresh: () => void
}

/** Trang đầu + các trang "Xem thêm", đánh dấu đã đọc, làm mới sau mỗi thao tác. */
export function useNotificationInbox(enabled: boolean): NotificationInbox {
  const history = useResource(['activity', 'history'], ({ signal }) => getNotificationHistory(signal), { enabled })
  const [older, setOlder] = useState<AppNotification[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [pageError, setPageError] = useState('')
  const [paging, setPaging] = useState(false)
  const rows = orderNotifications([...(history.data?.notifications ?? []), ...older])
  const more = cursor ?? history.data?.nextCursor
  // Tin đã đọc rồi thì không lấy lại từ trang trước (server đã đổi readAt).
  const refresh = () => { setOlder([]); setCursor(null); invalidate(keys.activity()); void history.refresh() }
  const mark = async (ids: string[] | 'all') => { await markNotificationsRead(ids); refresh() }
  const loadMore = async () => {
    if (!more || paging) return
    setPaging(true)
    try {
      const page = await getNotificationHistory(undefined, more)
      setOlder((previous) => [...previous, ...page.notifications])
      setCursor(page.nextCursor ?? '')
      setPageError('')
    } catch { setPageError('Không tải được thông báo. Thử lại.') }
    finally { setPaging(false) }
  }
  return { history, rows, groups: groupNotifications(rows), more, paging, pageError, loadMore, mark, refresh }
}

/** Icon + hai màu của ô icon theo loại thông báo (artboard Activity · MMain). */
export function notificationTone(kind: AppNotification['kind']) {
  switch (kind) {
    case 'review_ready': return { icon: 'flag', tile: noteStyles.review }
    case 'needs_attention': return { icon: 'danger', tile: noteStyles.attention }
    case 'ready_to_post': return { icon: 'tick-b', tile: noteStyles.ready }
    case 'task_done': return { icon: 'video-add', tile: noteStyles.done }
    default: return { icon: 'gallery', tile: noteStyles.plain }
  }
}

const timeLabel = (iso: string) => new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })

/**
 * Một dòng thông báo.
 * - `desktop` (popover máy tính): chấm vàng bên trái, ô icon vuông 28, nút hành động bên phải.
 * - `phone` (sheet Việc nền): ô icon tròn 34, chấm vàng nằm trên mép ô icon, cả dòng là liên kết.
 */
export function NotificationRow({ n, variant = 'desktop', showDot = true, onOpen }: {
  n: AppNotification
  variant?: 'desktop' | 'phone'
  /** `false` cho khối "Xong gần nhất": giữ chỗ chấm nhưng không hiện. */
  showDot?: boolean
  onOpen?: () => void
}) {
  const unread = !n.readAt
  const detail = notificationDetail(n)
  const action = n.actions.at(-1)
  const tone = notificationTone(n.kind)
  const merged = n.detail?.channels?.length && n.count > 1 ? ` · gộp ${n.count} thông báo` : ''
  const detailLine = detail ? <span title={detail} className={`vv-note__detail ${stylex.props(noteStyles.detail).className}`}>{detail}{merged}</span> : null
  const time = <span title={new Date(n.createdAt).toLocaleString('vi-VN')} className={`num ${stylex.props(noteStyles.time).className}`}>{timeLabel(n.createdAt)}</span>
  if (variant === 'phone') return (
    <a className={`vv-note ${stylex.props(noteStyles.phoneRow).className}`} href={action?.href} onClick={() => { if (unread) void markNotificationsRead([n.id]).then(() => invalidate(keys.activity())); onOpen?.() }}>
      <span aria-hidden="true" className={stylex.props(noteStyles.phoneTile, tone.tile).className}>
        <Icon name={tone.icon} size={17} />
        {showDot && unread && <span aria-hidden="true" className={`vv-note__dot ${stylex.props(noteStyles.phoneDot).className}`} />}
      </span>
      <span className={stylex.props(noteStyles.text, noteStyles.phoneText).className}>
        <span data-unread={unread ? 'true' : 'false'} className={`vv-note__title ${stylex.props(noteStyles.phoneTitle, unread ? noteStyles.phoneUnread : noteStyles.phoneRead).className}`}>{n.text}</span>
        {detailLine}
      </span>
      {time}
    </a>
  )
  return (
    <div className={`vv-note ${stylex.props(noteStyles.row, unread && noteStyles.rowUnread).className}`}>
      <span aria-label={showDot && unread ? 'Chưa đọc' : undefined} aria-hidden={showDot && unread ? undefined : true} className={stylex.props(noteStyles.desktopDot, noteStyles.desktopDotGap, (!showDot || !unread) && noteStyles.hiddenDot).className} />
      <span aria-hidden="true" className={stylex.props(noteStyles.tile, noteStyles.desktopTile, tone.tile).className}><Icon name={tone.icon} size={15} /></span>
      <span className={stylex.props(noteStyles.text).className}>
        <span data-unread={unread ? 'true' : 'false'} className={`vv-note__title ${stylex.props(noteStyles.title, noteStyles.titleLine, unread ? noteStyles.desktopUnread : noteStyles.desktopRead).className}`}>{n.text}</span>
        {detailLine}
        {time}
      </span>
      {action && <a className={stylex.props(noteStyles.action).className} href={action.href} onClick={() => { if (unread) void markNotificationsRead([n.id]).then(() => invalidate(keys.activity())); onOpen?.() }}>{action.label}</a>}
    </div>
  )
}

const noteStyles = stylex.create({
  row: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-10']} ${s['--vv-s-16']}` },
  rowUnread: { backgroundColor: 'var(--vv-notification-unread)' },
  text: { minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  titleLine: { lineHeight: 1.4 },
  title: { fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' },
  desktopUnread: { color: color['--color-fg'] },
  desktopRead: { fontWeight: 400, color: color['--color-fg-2'] },
  detail: { fontSize: 12, color: 'var(--vv-fg-note)', lineHeight: 1.45, overflowWrap: 'anywhere' },
  time: { fontSize: 11.5, lineHeight: 1.55, color: 'var(--vv-fg-note)' },
  desktopDot: { width: 7, height: 7, borderRadius: '50%', flexShrink: 0, backgroundColor: color['--color-warn'] },
  desktopDotGap: { marginTop: s['--vv-s-6'] },
  hiddenDot: { visibility: 'hidden' },
  tile: { width: 28, height: 28, borderRadius: r['--vv-r-8'], display: 'grid', placeItems: 'center', flexShrink: 0 },
  desktopTile: { backgroundColor: color['--color-surface-6'], color: color['--color-fg-3'] },
  review: { backgroundColor: 'var(--vv-cap-warn-bg)', color: 'var(--vv-cap-warn-ink)' },
  attention: { backgroundColor: 'var(--vv-err-12)', color: 'var(--vv-warn-ink)' },
  ready: { backgroundColor: 'var(--vv-ok-10)', color: color['--color-ok'] },
  done: { backgroundColor: 'var(--vv-qa-tint)', color: 'var(--vv-qa-ink)' },
  plain: { backgroundColor: color['--color-surface-6'], color: color['--color-fg-3'] },
  action: { borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: 'transparent', color: color['--color-fg'], borderRadius: r['--vv-r-8'], padding: `${s['--vv-s-6']} ${s['--vv-s-9']}`, fontSize: 12, cursor: 'pointer', textDecoration: 'none', whiteSpace: 'nowrap' },

  // Điện thoại — artboard MMain › sheet Việc nền › tab Thông báo.
  phoneRow: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-12'], padding: `${s['--vv-s-12']} ${s['--vv-s-16']}`, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--vv-wash-5)', color: color['--color-fg'], textDecoration: 'none' },
  phoneText: { gap: s['--vv-s-2'] },
  phoneTitle: { fontSize: 14, lineHeight: 1.4, overflowWrap: 'anywhere' },
  phoneUnread: { fontWeight: 600, color: color['--color-fg'] },
  phoneRead: { fontWeight: 400, color: color['--color-fg-2'] },
  phoneDetail: { fontSize: 12.5, lineHeight: 1.45 },
  phoneTime: { fontSize: 12, paddingTop: s['--vv-s-2'], padding: 0, flex: '0 0 auto' },
  phoneTile: { position: 'relative', width: 34, height: 34, borderRadius: '50%', display: 'grid', placeItems: 'center', flexShrink: 0 },
  /* 10px, tâm trên mép tròn góc trên phải; viền 2px màu nền. */
  phoneDot: { position: 'absolute', top: -1, right: -1, width: 10, height: 10, borderRadius: '50%', backgroundColor: color['--color-warn'], boxShadow: `0 0 0 2px ${color['--color-bg']}`, zIndex: 1, pointerEvents: 'none' },
})
