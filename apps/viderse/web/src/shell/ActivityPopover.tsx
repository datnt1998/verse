import { Suspense, lazy, useEffect, useId, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Tabs } from '@noverse/ui'
import type { Activity, BackgroundTask } from '../api/activity'
import { getQueue } from '../api/clone'
import { listChannels } from '../api/channels'
import { request } from '../api/request'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import { paths } from '../router'
import { NotificationRow, useNotificationInbox } from './notificationList'

// Tab Cài đặt thông báo chỉ mở khi bấm: nạp mã riêng, không nằm trong chunk đầu.
const NotificationSettings = lazy(() => import('../screens/settings/NotificationSettings'))

const s = stylex.create({
  pop: { position: 'fixed', top: 'calc(var(--vv-s-48) + var(--vv-s-2))', right: 'var(--vv-s-16)', zIndex: 100, width: 448, maxWidth: 'calc(100vw - 24px)', maxHeight: 'calc(100vh - 66px)', display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: 'var(--vv-r-16)', backgroundColor: 'var(--kit-surface-15)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', boxShadow: 'var(--kit-shadow-pop)', color: 'var(--color-fg)' },
  head: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', height: 68, boxSizing: 'border-box', flexShrink: 0, padding: 'var(--vv-s-14) var(--vv-s-12) var(--vv-s-10) var(--vv-s-16)', borderBottom: '1px solid var(--line-1)', fontSize: 15, fontWeight: 600 },
  headButton: { width: 30, height: 30, flexShrink: 0 },
  tabs: { display: 'flex', gap: 'var(--vv-s-2)', height: 40, boxSizing: 'border-box', padding: 'var(--vv-s-8) var(--vv-s-12) 0' },
  tabCount: { minWidth: 18, height: 18, padding: '0 var(--vv-s-5)', borderRadius: 'var(--vv-r-pill)', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', fontSize: 11, fontWeight: 600 },
  activeCount: { backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)' },
  scroll: { overflowY: 'auto', minHeight: 100, paddingBottom: 'var(--vv-s-6)' },
  group: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 'var(--vv-s-12) var(--vv-s-16) var(--vv-s-6)', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--color-fg-3)' },
  markRead: { height: 26 },
  row: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16)' },
  content: { minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' },
  title: { fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.55, overflowWrap: 'anywhere' },
  taskStep: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  dot: { width: 7, height: 7, borderRadius: '50%', marginTop: 'var(--vv-s-6)', flexShrink: 0, backgroundColor: 'var(--color-warn)' },
  taskIcon: { width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', flexShrink: 0 },
  button: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', backgroundColor: 'transparent', color: 'var(--color-fg)', borderRadius: 'var(--vv-r-8)', padding: 'var(--vv-s-6) var(--vv-s-9)', fontSize: 12, cursor: 'pointer', textDecoration: 'none', whiteSpace: 'nowrap' },
  progress: { width: '100%', height: 4, accentColor: 'var(--color-accent)' },
  noteRow: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16) var(--vv-s-10) var(--vv-s-12)' },
  noteContent: { gap: 'var(--vv-s-2)' },
  noteTitle: { lineHeight: 1.4 },
  noteDetail: { lineHeight: 1.45, color: 'var(--vv-fg-note)' },
  noteTime: { fontSize: 11.5, lineHeight: 1.55, color: 'var(--vv-fg-note)' },
  noteDot: { marginTop: 'var(--vv-s-12)' },
  noteAction: { height: 28, boxSizing: 'border-box', padding: '0 var(--vv-s-10)', display: 'flex', alignItems: 'center', fontSize: 12.5 },
  footerAction: { display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-6)', fontSize: 12.5 },
  noteUnread: { backgroundColor: 'var(--vv-notification-unread)' },
  noteTile: { width: 28, height: 28, borderRadius: 'var(--vv-r-8)', display: 'grid', placeItems: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', flexShrink: 0 },
  noteReview: { backgroundColor: 'var(--vv-cap-warn-bg)', color: 'var(--vv-cap-warn-ink)' },
  noteAttention: { backgroundColor: 'var(--vv-warn-bg)', color: 'var(--vv-warn-ink)' },
  hiddenDot: { visibility: 'hidden' },
  taskTitle: { display: 'flex', alignItems: 'baseline', gap: 'var(--vv-s-8)', minWidth: 0, whiteSpace: 'nowrap' },
  taskLabel: { overflow: 'hidden', textOverflow: 'ellipsis', color: 'var(--color-fg-2)', fontSize: 12.5 },
  orb: { width: 22, height: 22, borderRadius: '50%', margin: 'var(--vv-s-3)', flexShrink: 0 },
  scout: { backgroundImage: 'var(--vv-orb-scout)' },
  director: { backgroundImage: 'var(--vv-orb-director)' },
  artist: { backgroundImage: 'var(--vv-orb-artist)' },
  qa: { backgroundImage: 'var(--vv-orb-qa)' },
  segments: { display: 'flex', width: '100%', gap: 'var(--vv-s-3)' },
  segment: { flex: 1, height: 4, borderRadius: 'var(--vv-r-2)', backgroundColor: 'var(--color-surface-6)' },
  segmentScout: { backgroundColor: 'var(--vv-scout-ink)' },
  segmentDirector: { backgroundColor: 'var(--vv-director-ink)' },
  segmentArtist: { backgroundColor: 'var(--vv-artist-ink)' },
  segmentQa: { backgroundColor: 'var(--vv-qa-ink)' },
  empty: { padding: 'var(--vv-s-28) var(--vv-s-24)', textAlign: 'center', color: 'var(--color-fg-3)', fontSize: 13, lineHeight: 1.6 },
  emptyBody: { boxSizing: 'border-box', padding: 'var(--vv-s-28) var(--vv-s-24) var(--vv-s-22)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-10)', textAlign: 'center' },
  emptyOrbs: { display: 'flex', gap: 'var(--vv-s-6)', opacity: 0.55 },
  emptyOrb: { width: 16, height: 16, borderRadius: '50%' },
  emptyTitle: { fontSize: 15, fontWeight: 600 },
  emptyDescription: { maxWidth: 320, fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.55 },
  emptyActions: { display: 'flex', gap: 'var(--vv-s-8)', marginTop: 'var(--vv-s-4)' },
  footer: { display: 'flex', justifyContent: 'space-between', height: 53, boxSizing: 'border-box', flexShrink: 0, padding: 'var(--vv-s-10) var(--vv-s-12)', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)' },
})

export function taskOperation(task: BackgroundTask) {
  const op = task.action.op
  const ref = task.objectRef
  if (!op) return null
  if (ref.type === 'tool' && (op === 'cancel' || op === 'retry')) return { path: `/api/tools/${encodeURIComponent(ref.id)}/${op === 'retry' ? 'install' : 'cancel'}`, body: {} }
  if (ref.type === 'setup_step' && op === 'retry') return { path: '/api/setup/retry', body: { id: ref.id } }
  if (ref.type === 'storage_move' && op === 'cancel') return { path: `/api/storage/move/${encodeURIComponent(task.id)}/cancel`, body: {} }
  if (ref.type === 'rival') return { path: `/api/rivals/${encodeURIComponent(ref.id)}/scan`, body: { action: op === 'retry' ? 'resume' : op } }
  return null
}

function Task({ task, refresh }: { task: BackgroundTask; refresh: () => void }) {
  const [error, setError] = useState('')
  const act = async () => {
    try {
      const operation = taskOperation(task)
      if (!operation) throw new Error('Không thể thực hiện thao tác này.')
      await request(operation.path, { body: operation.body })
      refresh()
    } catch (e) { setError(e instanceof Error ? e.message : 'Không thực hiện được.') }
  }
  const orb = task.agent === 'scout' ? s.scout : task.agent === 'director' ? s.director : task.agent === 'artist' ? s.artist : task.agent === 'qa' ? s.qa : null
  const progress = task.progress
  const elapsed = task.createdAt ? Math.max(0, Math.floor((Date.now() - Date.parse(task.createdAt)) / 60000)) : null
  const segments = task.kind === 'video' && progress && progress.total > 0 && progress.total <= 30
  return <div {...stylex.props(s.row)}>
    {orb ? <span aria-hidden="true" {...stylex.props(s.orb, orb)} /> : <span {...stylex.props(s.taskIcon)}><Icon name="activity" size={16} /></span>}
    <div {...stylex.props(s.content)}>
      <div {...stylex.props(s.taskTitle)}><span {...stylex.props(s.title)}>{task.title}</span><span {...stylex.props(s.taskLabel)} title={task.objectRef.label}>{task.objectRef.label}</span></div>
      <span {...stylex.props(s.sub)}>{task.phase}</span>
      {progress && progress.total > 0 && <>
        {segments ? <span aria-hidden="true" {...stylex.props(s.segments)}>{Array.from({ length: progress.total }, (_, index) => <span key={index} {...stylex.props(s.segment, index < progress.done && (task.agent === 'scout' ? s.segmentScout : task.agent === 'director' ? s.segmentDirector : task.agent === 'artist' ? s.segmentArtist : s.segmentQa))} />)}</span> : <progress {...stylex.props(s.progress)} value={progress.done} max={progress.total} />}
        <span {...stylex.props(s.sub, s.taskStep)}>{task.kind === 'video' ? 'Bước ' : ''}{progress.done.toLocaleString('vi-VN')} / {progress.total.toLocaleString('vi-VN')}{elapsed != null && Number.isFinite(elapsed) ? ` · đã chạy ${elapsed} phút` : ''}{task.etaSec != null ? ` · còn khoảng ${Math.max(1, Math.ceil(task.etaSec / 60))} phút` : ''}</span>
        {progress.groups?.map((group) => <span key={group.group} {...stylex.props(s.sub)}>{group.group}: {group.done.toLocaleString('vi-VN')} / {group.total.toLocaleString('vi-VN')}</span>)}
      </>}
      {error && <span role="alert" {...stylex.props(s.sub)}>{error}</span>}
    </div>
    {task.action.href ? <a {...stylex.props(s.button)} href={task.action.href}>{task.action.label}</a> : task.action.op ? <Button size="sm" variant="ghost" onClick={() => void act()}>{task.action.label}</Button> : null}
  </div>
}

export function ActivityPopover({ activity, error, retry, close }: { activity?: Activity; error?: unknown; retry: () => void; close: () => void }) {
  const [tab, setTab] = useState<'running' | 'inbox' | 'settings'>('running')
  const tabsId = useId()
  const panelId = `${tabsId}-panel`
  const inbox = useNotificationInbox(tab === 'inbox' || (tab === 'running' && !!activity && activity.running.length === 0 && activity.queued === 0))
  const { history, rows, groups, more, paging, pageError, loadMore, mark, refresh } = inbox
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal), { enabled: tab === 'running' && !!activity?.queued })
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal), { enabled: tab === 'running' && !!activity?.queued })
  useEffect(() => { const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }; window.addEventListener('keydown', onKey); return () => window.removeEventListener('keydown', onKey) }, [close])
  const tasks = activity?.running ?? []
  const nextQueued = queue.data?.items[0]
  const nextChannel = channels.data?.channels.find((channel) => channel.id === nextQueued?.channelId)
  if (!activity) return <section {...stylex.props(s.pop)} aria-label="Việc nền và thông báo"><div {...stylex.props(s.head)}>Việc nền & thông báo</div><div role={error ? 'alert' : 'status'} {...stylex.props(s.empty)}>{error ? <>Không tải được việc nền. <Button size="sm" variant="quiet" onClick={retry}>Thử lại</Button></> : 'Đang tải…'}</div></section>
  return (
    <section {...stylex.props(s.pop)} role="dialog" aria-label="Việc nền">
      <div {...stylex.props(s.head)}>
        {tab === 'settings' && <Button variant="icon" size="sm" aria-label="Quay lại Việc nền" xstyle={s.headButton} onClick={() => setTab('running')}><Icon name="arrow-left-2" size={16} /></Button>}
        <div {...stylex.props(s.content)}>{tab === 'settings' ? 'Cài đặt thông báo' : 'Việc nền'}<span {...stylex.props(s.sub)}>{tab === 'settings' ? 'Chọn việc nào được báo và báo ở đâu' : tasks.length ? `Máy chạy ${activity.videoJobs} video · ${activity.queued} chờ lượt · ${tasks.filter((task) => task.kind !== 'video').length} việc khác` : `Máy rảnh · ${activity.queued} chờ lượt`}</span></div>
        {tab !== 'settings' && <Button variant="icon" size="sm" aria-label="Cài đặt thông báo" xstyle={s.headButton} onClick={() => setTab('settings')}><Icon name="setting" size={16} /></Button>}
        <Button variant="icon" size="sm" aria-label="Đóng (Esc)" xstyle={s.headButton} onClick={close}><Icon name="close" size={16} /></Button>
      </div>
      {tab !== 'settings' && <div {...stylex.props(s.tabs)}><Tabs label="Việc nền" value={tab} onChange={(id) => setTab(id as 'running' | 'inbox')} variant="bar" tabs={{ id: tabsId, panel: panelId }} items={[
        { id: 'running', label: 'Đang chạy', dot: <span {...stylex.props(s.tabCount, tab === 'running' && s.activeCount)}>{tasks.length}</span> },
        { id: 'inbox', label: 'Thông báo', dot: <span {...stylex.props(s.tabCount, tab === 'inbox' && s.activeCount)}>{activity.unread.length}</span> },
      ]} /></div>}
      <div id={panelId} role={tab === 'settings' ? undefined : 'tabpanel'} aria-labelledby={tab === 'settings' ? undefined : `${tabsId}-${tab}`} {...stylex.props(s.scroll)}>
        {tab === 'settings' ? <Suspense fallback={null}><NotificationSettings embedded /></Suspense> : tab === 'running' ? tasks.length || activity.queued ? <>
          {(['video', 'other'] as const).map((group) => {
            const selected = tasks.filter((task) => group === 'video' ? task.kind === 'video' : task.kind !== 'video')
            return <div key={group}>{selected.length > 0 && <><div {...stylex.props(s.group)}>{group === 'video' ? 'Video' : 'Khác'} <span>{selected.length}</span></div>{selected.map((task) => <Task key={task.id} task={task} refresh={refresh} />)}</>}{group === 'video' && !!activity.queued && <div {...stylex.props(s.row)}><a href={`#${paths.queue()}`} {...stylex.props(s.button)}>{activity.queued} chờ lượt{nextQueued && <> · tiếp theo {nextChannel?.name || nextQueued.title}{nextQueued.etaStart && <> lúc {new Date(nextQueued.etaStart).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</>}</>}</a></div>}</div>
          })}
        </> : <>
          <div {...stylex.props(s.emptyBody)}>
            <span aria-hidden="true" {...stylex.props(s.emptyOrbs)}><span {...stylex.props(s.emptyOrb, s.scout)} /><span {...stylex.props(s.emptyOrb, s.director)} /><span {...stylex.props(s.emptyOrb, s.artist)} /><span {...stylex.props(s.emptyOrb, s.qa)} /></span>
            <span {...stylex.props(s.emptyTitle)}>Máy đang rảnh</span><span {...stylex.props(s.emptyDescription)}>Không có video nào đang chạy hay chờ lượt.{activity.nextScan && <> Tự lấy video kiểm tra đối thủ lần tới lúc {new Date(activity.nextScan.at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}.</>}</span>
            <div {...stylex.props(s.emptyActions)}><a href={`#${paths.new()}`} {...stylex.props(s.button, s.noteAction)}>Làm video</a><a href="#/explore" {...stylex.props(s.button, s.noteAction)}>Mở Khám phá</a></div>
          </div>
          {history.data?.notifications.find((n) => n.kind === 'ready_to_post' || n.kind === 'task_done') && <><div {...stylex.props(s.group)}>Xong gần nhất</div>{history.data.notifications.filter((n) => n.kind === 'ready_to_post' || n.kind === 'task_done').slice(0, 1).map((n) => <NotificationRow key={n.id} n={n} showDot={false} onOpen={close} />)}</>}
        </>
          : history.error ? <div role="alert" {...stylex.props(s.empty)}>Không tải được thông báo. <Button size="sm" variant="quiet" onClick={() => void history.refresh()}>Thử lại</Button></div>
          : history.initialLoading ? <div {...stylex.props(s.empty)}>Đang tải…</div>
          : rows.length ? groups.map((group) => group.rows.length ? <div key={group.label}>
            <div {...stylex.props(s.group)}>{group.label}{group.label === 'Hôm nay' && <Button size="sm" variant="quiet" xstyle={s.markRead} onClick={() => void mark('all')}>Đánh dấu đã đọc</Button>}</div>
            {group.rows.map((n) => <NotificationRow key={n.id} n={n} onOpen={close} />)}
          </div> : null) : <div {...stylex.props(s.empty)}>Chưa có thông báo.</div>}
        {tab === 'inbox' && more ? <div {...stylex.props(s.row)}><Button size="sm" variant="ghost" disabled={paging} onClick={() => void loadMore()}>{paging ? 'Đang tải…' : 'Xem thêm'}</Button>{pageError && <span role="alert" {...stylex.props(s.sub)}>{pageError}</span>}</div> : null}
      </div>
      {tab !== 'settings' && <footer {...stylex.props(s.footer)}><a {...stylex.props(s.button, s.footerAction)} href={`#${paths.queue()}`} onClick={close}><Icon name="video-play" size={15} />Mở Hàng chờ</a><Button size="sm" variant="ghost" xstyle={s.footerAction} onClick={() => setTab('settings')}><Icon name="setting" size={15} />Cài đặt thông báo</Button></footer>}
    </section>
  )
}
