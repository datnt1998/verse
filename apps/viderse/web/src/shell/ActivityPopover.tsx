import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { getNotificationHistory, markNotificationsRead, type Activity, type AppNotification, type BackgroundTask } from '../api/activity'
import { getQueue } from '../api/clone'
import { listChannels } from '../api/channels'
import { request } from '../api/request'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import { paths } from '../router'
import NotificationSettings from '../screens/settings/NotificationSettings'
import { notificationDetail } from './activityText'

const s = stylex.create({
  pop: { position: 'fixed', top: 'calc(var(--vv-s-48) + var(--vv-s-2))', right: 'var(--vv-s-16)', zIndex: 100, width: 448, maxWidth: 'calc(100vw - 24px)', maxHeight: 'calc(100vh - 66px)', display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: 'var(--vv-r-16)', backgroundColor: 'var(--kit-surface-15)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', boxShadow: 'var(--kit-shadow-pop)', color: 'var(--color-fg)' },
  head: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', height: 68, boxSizing: 'border-box', flexShrink: 0, padding: 'var(--vv-s-14) var(--vv-s-12) var(--vv-s-10) var(--vv-s-16)', borderBottom: '1px solid var(--line-1)', fontSize: 15, fontWeight: 600 },
  tabs: { display: 'flex', gap: 'var(--vv-s-4)', height: 40, boxSizing: 'border-box', padding: 'var(--vv-s-8) var(--vv-s-12) 0', borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--line-1)' },
  tab: { flex: 1, borderWidth: 0, borderBottomWidth: 2, borderBottomStyle: 'solid', borderBottomColor: 'transparent', padding: '0 var(--vv-s-10)', backgroundColor: 'transparent', color: 'var(--color-fg-3)', cursor: 'pointer' },
  active: { borderBottomColor: 'var(--color-fg)', color: 'var(--color-fg)' },
  scroll: { overflowY: 'auto', minHeight: 100 },
  group: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 'var(--vv-s-12) var(--vv-s-16) var(--vv-s-7)', fontSize: 11, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--color-fg-3)' },
  row: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16)' },
  content: { minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' },
  title: { fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5, overflowWrap: 'anywhere' },
  dot: { width: 7, height: 7, borderRadius: '50%', marginTop: 'var(--vv-s-6)', flexShrink: 0, backgroundColor: 'var(--color-warn)' },
  taskIcon: { width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', flexShrink: 0 },
  button: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', backgroundColor: 'transparent', color: 'var(--color-fg)', borderRadius: 'var(--vv-r-8)', padding: 'var(--vv-s-6) var(--vv-s-9)', fontSize: 12, cursor: 'pointer', textDecoration: 'none', whiteSpace: 'nowrap' },
  plain: { border: 0, backgroundColor: 'transparent', color: 'var(--color-fg-3)', cursor: 'pointer', fontSize: 12 },
  progress: { width: '100%', height: 4, accentColor: 'var(--color-accent)' },
  noteRow: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16)' },
  noteUnread: { backgroundColor: 'var(--vv-warn-bg)' },
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
  emptyBody: { padding: 'var(--vv-s-28) var(--vv-s-24) var(--vv-s-22)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-10)', textAlign: 'center' },
  emptyOrbs: { display: 'flex', gap: 'var(--vv-s-6)', opacity: 0.55 },
  emptyOrb: { width: 16, height: 16, borderRadius: '50%' },
  emptyTitle: { fontSize: 15, fontWeight: 600 },
  emptyDescription: { maxWidth: 320, fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.55 },
  emptyActions: { display: 'flex', gap: 'var(--vv-s-8)', marginTop: 'var(--vv-s-4)' },
  footer: { display: 'flex', justifyContent: 'space-between', padding: 'var(--vv-s-10) var(--vv-s-12)', borderTop: '1px solid var(--line-1)' },
})

function Task({ task, refresh }: { task: BackgroundTask; refresh: () => void }) {
  const [error, setError] = useState('')
  const act = async () => {
    try {
      const op = task.action.op
      if (!op) return
      const ref = task.objectRef
      const path = ref.type === 'tool' ? `/api/tools/${encodeURIComponent(ref.id)}/${op}` : ref.type === 'setup_step' ? '/api/setup/retry' : ref.type === 'storage_move' ? `/api/storage/move/${encodeURIComponent(task.id)}/cancel` : ref.type === 'rival' ? `/api/rivals/${encodeURIComponent(ref.id)}/scan` : ''
      if (!path) throw new Error('Không thể thực hiện thao tác này.')
      await request(path, { body: ref.type === 'rival' ? { action: op === 'retry' ? 'resume' : op } : {} })
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
        <span {...stylex.props(s.sub)}>{task.kind === 'video' ? 'Bước ' : ''}{progress.done.toLocaleString('vi-VN')} / {progress.total.toLocaleString('vi-VN')}{elapsed != null && Number.isFinite(elapsed) ? ` · đã chạy ${elapsed} phút` : ''}{task.etaSec != null ? ` · còn khoảng ${Math.max(1, Math.ceil(task.etaSec / 60))} phút` : ''}</span>
        {progress.groups?.map((group) => <span key={group.group} {...stylex.props(s.sub)}>{group.group}: {group.done.toLocaleString('vi-VN')} / {group.total.toLocaleString('vi-VN')}</span>)}
      </>}
      {error && <span role="alert" {...stylex.props(s.sub)}>{error}</span>}
    </div>
    {task.action.href ? <a {...stylex.props(s.button)} href={task.action.href}>{task.action.label}</a> : task.action.op ? <button {...stylex.props(s.button)} onClick={() => void act()}>{task.action.label}</button> : null}
  </div>
}

export function orderNotifications(notifications: AppNotification[]) {
  const priority: Record<AppNotification['kind'], number> = { review_ready: 0, needs_attention: 1, ready_to_post: 2, task_done: 3, action_result: 4 }
  return [...notifications].sort ( (a, b) => Number(!!a.readAt) - Number(!!b.readAt) || priority[a.kind] - priority[b.kind] || Date.parse(b.createdAt) - Date.parse(a.createdAt))
}
export function ActivityPopover({ activity, error, retry, close }: { activity?: Activity; error?: unknown; retry: () => void; close: () => void }) {
  const [tab, setTab] = useState<'running' | 'inbox' | 'settings'>('running')
  const [older, setOlder] = useState<AppNotification[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [pageError, setPageError] = useState('')
  const [paging, setPaging] = useState(false)
  const history = useResource(['activity', 'history'], ({ signal }) => getNotificationHistory(signal), { enabled: tab === 'inbox' || (tab === 'running' && !!activity && activity.running.length === 0 && activity.queued === 0) })
  const queue = useResource(['activity', 'queue'], ({ signal }) => getQueue(signal), { enabled: tab === 'running' && !!activity?.queued })
  const channels = useResource(['channels'], ({ signal }) => listChannels(signal), { enabled: tab === 'running' && !!activity?.queued })
  useEffect(() => { const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }; window.addEventListener('keydown', onKey); return () => window.removeEventListener('keydown', onKey) }, [close])
  const refresh = () => { setOlder([]); setCursor(null); invalidate(keys.activity()); void history.refresh() }
  const mark = async () => { await markNotificationsRead('all'); refresh() }
  const loadMore = async () => {
    const before = cursor ?? history.data?.nextCursor
    if (!before || paging) return
    setPaging(true)
    try {
      const page = await getNotificationHistory(undefined, before)
      setOlder((previous) => [...previous, ...page.notifications])
      setCursor(page.nextCursor ?? '')
      setPageError('')
    } catch { setPageError('Không tải được thông báo. Thử lại.') }
    finally { setPaging(false) }
  }
  const tasks = activity?.running ?? []
  const nextQueued = queue.data?.items[0]
  const nextChannel = channels.data?.channels.find((channel) => channel.id === nextQueued?.channelId)
  const rows = orderNotifications([...(history.data?.notifications ?? []), ...older])
  const today = new Date(); today.setHours(0, 0, 0, 0)
  const yesterday = today.getTime() - 86400000
  const groups = [ { label: 'Hôm nay', rows: rows.filter((n) => Date.parse(n.createdAt) >= today.getTime()) }, { label: 'Hôm qua', rows: rows.filter((n) => Date.parse(n.createdAt) >= yesterday && Date.parse(n.createdAt) < today.getTime()) }, { label: 'Trước đó', rows: rows.filter((n) => Date.parse(n.createdAt) < yesterday) } ]
  if (!activity) return <section {...stylex.props(s.pop)} aria-label="Việc nền và thông báo"><div {...stylex.props(s.head)}>Việc nền & thông báo</div><div role={error ? 'alert' : 'status'} {...stylex.props(s.empty)}>{error ? <>Không tải được việc nền. <button {...stylex.props(s.plain)} onClick={retry}>Thử lại</button></> : 'Đang tải…'}</div></section>
  return (
    <section {...stylex.props(s.pop)} role="dialog" aria-label="Việc nền">
      <div {...stylex.props(s.head)}>
        {tab === 'settings' && <button type="button" aria-label="Quay lại Việc nền" {...stylex.props(s.plain)} onClick={() => setTab('running')}><Icon name="arrow-r" size={16} /></button>}
        <div {...stylex.props(s.content)}>{tab === 'settings' ? 'Cài đặt thông báo' : 'Việc nền'}<span {...stylex.props(s.sub)}>{tab === 'settings' ? 'Chọn việc nào được báo và báo ở đâu' : tasks.length ? `Máy chạy ${activity.videoJobs} video · ${activity.queued} chờ lượt · ${tasks.filter((task) => task.kind !== 'video').length} việc khác` : `Máy rảnh · ${activity.queued} chờ lượt`}</span></div>
        {tab !== 'settings' && <button type="button" aria-label="Cài đặt thông báo" {...stylex.props(s.plain)} onClick={() => setTab('settings')}><Icon name="setting" size={16} /></button>}
        <button type="button" aria-label="Đóng (Esc)" {...stylex.props(s.plain)} onClick={close}><Icon name="close" size={16} /></button>
      </div>
      {tab !== 'settings' && <div role="tablist" aria-label="Việc nền" {...stylex.props(s.tabs)}>
        <button role="tab" aria-selected={tab === 'running'} {...stylex.props(s.tab, tab === 'running' && s.active)} onClick={() => setTab('running')}>Đang chạy · {tasks.length}</button>
        <button role="tab" aria-selected={tab === 'inbox'} {...stylex.props(s.tab, tab === 'inbox' && s.active)} onClick={() => setTab('inbox')}>Thông báo · {activity.unread.length}</button>
      </div>}
      <div {...stylex.props(s.scroll)}>
        {tab === 'settings' ? <NotificationSettings embedded /> : tab === 'running' ? tasks.length || activity.queued ? <>
          {(['video', 'other'] as const).map((group) => {
            const selected = tasks.filter((task) => group === 'video' ? task.kind === 'video' : task.kind !== 'video')
            return <div key={group}>{selected.length > 0 && <><div {...stylex.props(s.group)}>{group === 'video' ? 'Video' : 'Khác'} <span>{selected.length}</span></div>{selected.map((task) => <Task key={task.id} task={task} refresh={refresh} />)}</>}{group === 'video' && !!activity.queued && <div {...stylex.props(s.row)}><a href={`#${paths.queue()}`} {...stylex.props(s.button)}>{activity.queued} chờ lượt{nextQueued && <> · tiếp theo {nextChannel?.name || nextQueued.title}{nextQueued.etaStart && <> lúc {new Date(nextQueued.etaStart).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</>}</>}</a></div>}</div>
          })}
        </> : <>
          <div {...stylex.props(s.emptyBody)}>
            <span aria-hidden="true" {...stylex.props(s.emptyOrbs)}><span {...stylex.props(s.emptyOrb, s.scout)} /><span {...stylex.props(s.emptyOrb, s.director)} /><span {...stylex.props(s.emptyOrb, s.artist)} /><span {...stylex.props(s.emptyOrb, s.qa)} /></span>
            <span {...stylex.props(s.emptyTitle)}>Máy đang rảnh</span><span {...stylex.props(s.emptyDescription)}>Không có video nào đang chạy hay chờ lượt.</span>
            <div {...stylex.props(s.emptyActions)}><a href={`#${paths.new()}`} {...stylex.props(s.button)}>Làm video</a><a href="#/explore" {...stylex.props(s.button)}>Mở Khám phá</a></div>
          </div>
          {history.data?.notifications.find((n) => n.kind === 'ready_to_post' || n.kind === 'task_done') && <><div {...stylex.props(s.group)}>Xong gần nhất</div>{history.data.notifications.filter((n) => n.kind === 'ready_to_post' || n.kind === 'task_done').slice(0, 1).map((n) => <div key={n.id} {...stylex.props(s.row)}><span {...stylex.props(s.taskIcon)}><Icon name="tick" size={15} /></span><div {...stylex.props(s.content)}><span {...stylex.props(s.title)}>{n.text}</span><span {...stylex.props(s.sub)}>{new Date(n.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span></div>{n.actions.at(-1) && <a {...stylex.props(s.button)} href={n.actions.at(-1)!.href} onClick={close}>{n.actions.at(-1)!.label}</a>}</div>)}</>}
          {activity.nextScan && <div {...stylex.props(s.row)}><span {...stylex.props(s.taskIcon)}><Icon name="clock" size={15} /></span><div {...stylex.props(s.content)}><span {...stylex.props(s.title)}>Lượt quét tự động kế tiếp</span><span {...stylex.props(s.sub)}>{new Date(activity.nextScan.at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })} · {activity.nextScan.rivalChannels} kênh đối thủ</span></div></div>}
        </>
          : history.error ? <div role="alert" {...stylex.props(s.empty)}>Không tải được thông báo. <button {...stylex.props(s.plain)} onClick={() => void history.refresh()}>Thử lại</button></div>
          : history.initialLoading ? <div {...stylex.props(s.empty)}>Đang tải…</div>
          : rows.length ? groups.map((group) => group.rows.length ? <div key={group.label}>
            <div {...stylex.props(s.group)}>{group.label}{group.label === 'Hôm nay' && <button {...stylex.props(s.plain)} onClick={() => void mark()}>Đánh dấu đã đọc</button>}</div>
            {group.rows.map((n) => <div key={n.id} {...stylex.props(s.noteRow, !n.readAt && s.noteUnread)}>
              <span aria-label={n.readAt ? undefined : 'Chưa đọc'} {...stylex.props(s.dot, !!n.readAt && s.hiddenDot)} />
              <span {...stylex.props(s.noteTile, n.kind === 'review_ready' && s.noteReview, n.kind === 'needs_attention' && s.noteAttention)}><Icon name={n.kind === 'needs_attention' ? 'danger' : n.kind === 'ready_to_post' ? 'tick' : 'flag'} size={15} /></span>
              <div {...stylex.props(s.content)}><span {...stylex.props(s.title)}>{n.text}</span>{notificationDetail(n) && <span {...stylex.props(s.sub)}>{notificationDetail(n)}</span>}<span {...stylex.props(s.sub)}>{new Date(n.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span></div>
              {n.actions.at(-1) && <a {...stylex.props(s.button)} href={n.actions.at(-1)!.href} onClick={() => { if (!n.readAt) void markNotificationsRead([n.id]).then(refresh); close() }}>{n.actions.at(-1)!.label}</a>}
            </div>)}
          </div> : null) : <div {...stylex.props(s.empty)}>Chưa có thông báo.</div>}
        {tab === 'inbox' && (cursor ?? history.data?.nextCursor) ? <div {...stylex.props(s.row)}><button type="button" {...stylex.props(s.button)} disabled={paging} onClick={() => void loadMore()}>{paging ? 'Đang tải…' : 'Xem thêm'}</button>{pageError && <span role="alert" {...stylex.props(s.sub)}>{pageError}</span>}</div> : null}
      </div>
      {tab !== 'settings' && <footer {...stylex.props(s.footer)}><a {...stylex.props(s.button)} href={`#${paths.queue()}`} onClick={close}>Mở Hàng chờ</a><button type="button" {...stylex.props(s.plain)} onClick={() => setTab('settings')}>Cài đặt thông báo</button></footer>}
    </section>
  )
}
