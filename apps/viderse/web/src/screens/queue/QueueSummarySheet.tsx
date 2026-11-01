import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Sheet } from '@noverse/ui'
import { Orb } from '../../components'
import { Segmented } from '../../components/Segmented'
import type { Assistant } from '../../components/agents'
import { getActivity } from '../../api/activity'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { t } from '../../i18n/queue'
import { paths } from '../../router'
import { fmtEta, quotaName, waitLine } from './queueView'
import { useQueue } from './useQueue'
import { QueueAvatar, QueueBanner, QueueIdle } from './MobileQueue'
import { mobileQueueStyles as sx } from './queueMobile.stylex'
import { NotificationRow, useNotificationInbox, type NotificationInbox } from '../../shell/notificationList'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'

const tt = t.screens.queue
const m = tt.mobile
const shell = t.shell

/**
 * Sheet Việc nền của điện thoại (artboard MMain › screen=jobs|inbox).
 * Hai tab: Đang chạy (hàng chờ thật) và Thông báo (cùng dữ liệu với popover máy
 * tính). `initialTab` do MobileShell truyền: nút Việc nền có chấm vàng thì mở
 * thẳng tab Thông báo.
 */
export function QueueSummarySheet({ open, onClose, initialTab = 'running', unread }: {
  open: boolean
  onClose: () => void
  /** Tab mở đầu tiên; đọc lại mỗi lần mở vì thân sheet chỉ gắn khi `open`. */
  initialTab?: 'running' | 'inbox'
  /** Số chưa đọc dự phòng khi cache /api/activity chưa có (nguồn thật là activity.unread). */
  unread?: number
}) {
  return open ? <OpenQueueSummary onClose={onClose} initialTab={initialTab} unreadFallback={unread} /> : null
}

function OpenQueueSummary({ onClose, initialTab, unreadFallback }: { onClose: () => void; initialTab: 'running' | 'inbox'; unreadFallback?: number }) {
  const [tab, setTab] = useState<'running' | 'inbox'>(initialTab)
  const queue = useQueue()
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal))
  const inbox = useNotificationInbox(tab === 'inbox')
  const { view, items, online, busy, patch } = queue
  const time = view?.pausedAt ? fmtEta(view.pausedAt) : null
  const empty = view && view.running.length === 0 && items.length === 0
  const channels = view ? new Set([...items.map((item) => item.channelId), ...view.running.map((job) => job.channelId)]).size : 0
  const runningSub = !online ? m.offlineSub : !view ? t.common.loading : view.paused ? m.pausedSummary(time, items.length) : empty ? m.idleSub : m.summarySub(view.running.length, items.length, channels)
  const unread = activity.data?.unread.length ?? unreadFallback ?? 0
  const rows = inbox.rows
  const hasUnread = rows.some((n) => !n.readAt)
  const inboxSub = !inbox.history.data && !inbox.history.error ? t.common.loading : rows.length === 0 ? shell.jobsSheetInboxEmpty : hasUnread ? shell.jobsSheetInboxSub(rows.filter((n) => !n.readAt).length) : shell.jobsSheetInboxReadAll
  return <Sheet open onClose={onClose} label={tt.sheetTitleMobile} size="md" height="auto" xstyle={sx.sheet} bodyStyle={sx.sheetBody}>
    {/* Đầu sheet: tiêu đề + dòng phụ theo tab; nút Tạm dừng chỉ có ở tab Đang chạy. */}
    <div {...stylex.props(sx.summaryHead)}><span {...stylex.props(sx.grow, sx.stack)}><h2 {...stylex.props(sx.sheetTitle)}>{tt.sheetTitleMobile}</h2><span className={`vi-sub ${stylex.props(sx.sub).className}`}>{tab === 'inbox' ? inboxSub : runningSub}</span></span>{tab === 'running' && view && !empty && <button type="button" {...stylex.props(sx.button, sx.summaryPause)} disabled={busy || !online} aria-disabled={busy || !online} aria-pressed={view.paused} onClick={() => void patch({ paused: !view.paused })}><Icon name={view.paused ? 'play' : 'pause'} size={16} />{view.paused ? tt.resume : tt.pause}</button>}</div>
    <div {...stylex.props(inboxStyles.tabs)}>
      <Segmented size={40} radius={9} fit="fill" tabs={{ id: 'jobs-tabs', panel: 'jobs-panel' }} label={tt.title} value={tab} onChange={(id) => setTab(id as 'running' | 'inbox')}
        options={[
          { id: 'running', label: <TabLabel label={shell.jobsSheetTabs.running} count={view?.running.length ?? 0} tone="plain" /> },
          { id: 'inbox', label: <TabLabel label={shell.jobsSheetTabs.inbox} count={unread} tone="unread" /> },
        ]} />
    </div>
    <div id="jobs-panel" role="tabpanel" aria-labelledby={`jobs-tabs-${tab}`} {...stylex.props(sx.summaryBody)}>
      {tab === 'running' ? <>
        <QueueBanner queue={queue} />
        {!view && <span role="status">{queue.res.status === 'error' ? tt.fail.title : t.common.loading}</span>}
        {empty && <QueueIdle summary onNavigate={onClose} />}
        {view && !empty && <>
          {view.running.length > 0 && <section {...stylex.props(sx.group)}><h3 {...stylex.props(sx.section)}>{tt.runningNow}</h3>{view.running.map((job) => <a key={job.jobId} {...stylex.props(sx.summaryRun)} href={`#${paths.workbench({ video: job.jobId })}`} onClick={onClose}><Orb of={(['scout', 'director', 'artist', 'qa'].includes(job.agent) ? job.agent : 'scout') as Assistant} size={22} live={online && !view.paused} /><span {...stylex.props(sx.grow, sx.stack)}><span title={job.title} {...stylex.props(sx.ellipsis, sx.queuedTitle)}>{job.title}</span><span title={job.now} {...stylex.props(sx.ellipsis, sx.small)}>{job.now}</span></span><QueueAvatar channelId={job.channelId} queue={queue} size={26} /></a>)}</section>}
          {items.length > 0 && <section {...stylex.props(sx.group)}><h3 {...stylex.props(sx.section)}>{tt.nextRotation}</h3><div>{items.slice(0, 4).map((item, index) => <a key={item.id} {...stylex.props(sx.summaryRow)} href={`#${paths.queue({ item: item.id })}`} onClick={onClose}><span {...stylex.props(sx.number)}>{index + 1}</span><QueueAvatar channelId={item.channelId} queue={queue} size={26} /><span {...stylex.props(sx.grow, sx.stack)}><span title={item.title} {...stylex.props(sx.ellipsis, sx.queuedTitle)}>{item.title || item.source}</span><span title={waitLine(item, view.paused)} {...stylex.props(sx.ellipsis, sx.small)}>{waitLine(item, view.paused) || fmtEta(item.etaStart)}</span></span></a>)}</div></section>}
          <a {...stylex.props(sx.summaryAll)} href={`#${paths.queue()}`} onClick={onClose}>{m.openAll}<Icon name="arrow-r" size={14} /></a>
        </>}
        {view?.imageQuota.map((quota) => <div key={`${quota.provider}-${quota.role}`} {...stylex.props(sx.summaryQuota)}><Icon name="gallery" size={16} /><span {...stylex.props(sx.grow)}>{quotaName(quota)} {tt.sheetQuotaToday}</span><span>{quota.used}{quota.limit === null ? '' : `/${quota.limit}`}</span>{quota.resetsAt && <span {...stylex.props(sx.small)}>· {tt.rules.quotaReset(fmtEta(quota.resetsAt))}</span>}</div>)}
        {view && <div {...stylex.props(sx.row)}><span {...stylex.props(sx.grow, sx.sub)}>{tt.sheetRules(view.workers.total)}</span><a {...stylex.props(sx.link)} href={`#${paths.queue({ sheet: 'rules' })}`} onClick={onClose}>{tt.rulesTitle}</a></div>}
      </> : <InboxTab inbox={inbox} unread={hasUnread} onClose={onClose} />}
    </div>
  </Sheet>
}

/** Nhãn tab + huy hiệu số: Thông báo đếm chưa đọc nên dùng nền/chữ vàng. */
function TabLabel({ label, count, tone }: { label: string; count: number; tone: 'plain' | 'unread' }) {
  return <>{label}{count > 0 && <span {...stylex.props(inboxStyles.count, tone === 'unread' ? inboxStyles.countUnread : inboxStyles.countPlain)}>{count}</span>}</>
}

/** Tab Thông báo — cùng dòng, cùng câu, cùng thứ tự với popover máy tính. */
function InboxTab({ inbox, unread, onClose }: { inbox: NotificationInbox; unread: boolean; onClose: () => void }) {
  const { history, groups, rows, more, paging, pageError, loadMore, mark } = inbox
  if (history.error) return <div role="alert" {...stylex.props(inboxStyles.empty)}>Không tải được thông báo. <button type="button" {...stylex.props(inboxStyles.plain)} onClick={() => void history.refresh()}>{t.common.retry}</button></div>
  if (history.initialLoading) return <div role="status" {...stylex.props(inboxStyles.empty)}>{t.common.loading}</div>
  if (rows.length === 0) return (
    <div {...stylex.props(inboxStyles.emptyBody)}>
      <span aria-hidden="true" {...stylex.props(inboxStyles.emptyOrb)}><Icon name="notify" size={20} /></span>
      <span {...stylex.props(inboxStyles.emptyTitle)}>{shell.jobsSheetInboxEmpty}</span>
      <span {...stylex.props(inboxStyles.emptyHint)}>{shell.notificationsEmptyBody}</span>
      <a {...stylex.props(inboxStyles.settingsButton)} href={`#${paths.settings('notifications')}`} onClick={onClose}><Icon name="setting" size={16} />{shell.notificationSettings}</a>
    </div>
  )
  return <>
    <div {...stylex.props(inboxStyles.list)}>
      {groups.map((group) => group.rows.length ? <div key={group.label}>
        <div {...stylex.props(inboxStyles.group)}>
          <span {...stylex.props(inboxStyles.groupLabel)}>{group.label}</span>
          {unread && group === groups.find((item) => item.rows.length) && <button type="button" {...stylex.props(inboxStyles.markRead)} onClick={() => void mark('all')}>{shell.markAllRead}</button>}
        </div>
        {group.rows.map((n) => <NotificationRow key={n.id} n={n} variant="phone" onOpen={onClose} />)}
      </div> : null)}
      {more ? <div {...stylex.props(inboxStyles.moreRow)}><button type="button" {...stylex.props(sx.button)} disabled={paging} onClick={() => void loadMore()}>{paging ? t.common.loading : 'Xem thêm'}</button>{pageError && <span role="alert" {...stylex.props(sx.sub)}>{pageError}</span>}</div> : null}
      <a {...stylex.props(inboxStyles.settingsLink)} href={`#${paths.settings('notifications')}`} onClick={onClose}><Icon name="setting" size={16} />{shell.notificationSettings}</a>
    </div>
  </>
}

const inboxStyles = stylex.create({
  tabs: { padding: `0 ${s['--vv-s-16']} ${s['--vv-s-12']}` },
  count: { minWidth: 18, height: 18, padding: `0 ${s['--vv-s-5']}`, borderRadius: r['--vv-r-pill'], display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 11, fontWeight: 600, fontVariantNumeric: 'tabular-nums' },
  /* Huy hiệu vàng của tab Thông báo (artboard: nền vàng 16%, chữ --vv-st-review). */
  countUnread: { backgroundColor: 'var(--vv-badge-bg)', color: color['--color-warn'] },
  countPlain: { backgroundColor: line['--line-3'], color: color['--color-fg-2'] },
  list: { display: 'flex', flexDirection: 'column', margin: `0 calc(-1 * ${s['--vv-s-16']})` },
  group: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', minHeight: 32, padding: `0 ${s['--vv-s-16']}` },
  /* Nhãn nhóm 11.5px, chữ hoa, cùng nhịp .sec của artboard. */
  groupLabel: { fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--vv-fg-note)' },
  markRead: { minHeight: 44, padding: `0 ${s['--vv-s-2']}`, borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 13, cursor: 'pointer' },
  moreRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-10']} ${s['--vv-s-16']}` },
  /* Chân danh sách: lối vào Cài đặt thông báo (cùng đích với popover máy tính). */
  settingsLink: { minHeight: 44, display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], padding: `0 ${s['--vv-s-16']}`, color: color['--color-fg-2'], fontSize: 13, textDecoration: 'none', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  empty: { padding: `${s['--vv-s-24']} ${s['--vv-s-24']}`, textAlign: 'center', color: 'var(--vv-fg-note)', fontSize: 13, lineHeight: 1.6 },
  plain: { borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: color['--color-fg-3'], fontSize: 13, cursor: 'pointer' },
  emptyBody: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: s['--vv-s-8'], padding: `${s['--vv-s-28']} ${s['--vv-s-12']} ${s['--vv-s-20']}`, textAlign: 'center' },
  emptyOrb: { width: 44, height: 44, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], color: 'var(--vv-fg-note)' },
  emptyTitle: { fontSize: 15, fontWeight: 600 },
  emptyHint: { maxWidth: 300, fontSize: 13, color: 'var(--vv-fg-note)', lineHeight: 1.5 },
  settingsButton: { height: 44, marginTop: s['--vv-s-6'], padding: `0 ${s['--vv-s-16']}`, display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'], borderRadius: r['--vv-r-12'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], color: color['--color-fg'], fontSize: 13.5, textDecoration: 'none' },
})
