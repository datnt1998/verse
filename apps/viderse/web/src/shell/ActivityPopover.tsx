import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { getNotificationHistory, markNotificationsRead, type Activity, type AppNotification, type BackgroundTask } from '../api/activity'
import { request } from '../api/request'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import { paths } from '../router'

const s = stylex.create({
  pop: { position: 'fixed', top: 'calc(var(--vv-s-48) + var(--vv-s-2))', right: 'var(--vv-s-16)', zIndex: 100, width: 448, maxWidth: 'calc(100vw - 24px)', maxHeight: 'calc(100vh - 65px)', display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: 'var(--vv-r-14)', backgroundColor: 'var(--color-surface-4)', border: '1px solid var(--line-2)', boxShadow: 'var(--kit-shadow-pop)', color: 'var(--color-fg)' },
  head: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-14) var(--vv-s-12) var(--vv-s-10) var(--vv-s-16)', borderBottom: '1px solid var(--line-1)', fontSize: 15, fontWeight: 600 },
  tabs: { display: 'flex', gap: 'var(--vv-s-4)', margin: '0 var(--vv-s-12) var(--vv-s-8)', padding: 'var(--vv-s-3)', borderRadius: 'var(--vv-r-9)', backgroundColor: 'var(--color-surface-1)' },
  tab: { flex: 1, border: 0, borderRadius: 'var(--vv-r-7)', padding: 'var(--vv-s-8) var(--vv-s-10)', backgroundColor: 'transparent', color: 'var(--color-fg-3)', cursor: 'pointer' },
  active: { backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg)' },
  scroll: { overflowY: 'auto', minHeight: 100 },
  group: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 'var(--vv-s-10) var(--vv-s-16) var(--vv-s-4)', fontSize: 11, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--color-fg-3)' },
  row: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-12) var(--vv-s-16)', borderTop: '1px solid var(--line-1)' },
  content: { minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' },
  title: { fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5, overflowWrap: 'anywhere' },
  dot: { width: 7, height: 7, borderRadius: '50%', marginTop: 'var(--vv-s-6)', flexShrink: 0, backgroundColor: 'var(--color-warn)' },
  taskIcon: { width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', flexShrink: 0 },
  button: { border: '1px solid var(--line-2)', backgroundColor: 'transparent', color: 'var(--color-fg)', borderRadius: 'var(--vv-r-8)', padding: 'var(--vv-s-6) var(--vv-s-9)', fontSize: 12, cursor: 'pointer', textDecoration: 'none', whiteSpace: 'nowrap' },
  plain: { border: 0, backgroundColor: 'transparent', color: 'var(--color-fg-3)', cursor: 'pointer', fontSize: 12 },
  progress: { width: '100%', height: 4, accentColor: 'var(--color-accent)' },
  empty: { padding: 'var(--vv-s-28) var(--vv-s-24)', textAlign: 'center', color: 'var(--color-fg-3)', fontSize: 13, lineHeight: 1.6 },
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
  return <div {...stylex.props(s.row)}><span {...stylex.props(s.taskIcon)}><Icon name="activity" size={16} /></span><div {...stylex.props(s.content)}><span {...stylex.props(s.title)} title={task.objectRef.label}>{task.title} · {task.objectRef.label}</span><span {...stylex.props(s.sub)}>{task.phase}</span>{task.progress && task.progress.total > 0 && <><progress {...stylex.props(s.progress)} value={task.progress.done} max={task.progress.total} /><span {...stylex.props(s.sub)}>{task.progress.done} / {task.progress.total}{task.etaSec != null ? ` · còn khoảng ${Math.ceil(task.etaSec / 60)} phút` : ''}</span></>}{error && <span role="alert" {...stylex.props(s.sub)}>{error}</span>}</div>{task.action.href ? <a {...stylex.props(s.button)} href={task.action.href}>{task.action.label}</a> : task.action.op ? <button {...stylex.props(s.button)} onClick={() => void act()}>{task.action.label}</button> : null}</div>
}

export function orderNotifications(notifications: AppNotification[]) {
  const priority: Record<AppNotification['kind'], number> = { review_ready: 0, needs_attention: 1, ready_to_post: 2, task_done: 3, action_result: 4 }
  return [...notifications].sort ( (a, b) => Number(!!a.readAt) - Number(!!b.readAt) || priority[a.kind] - priority[b.kind] || Date.parse(b.createdAt) - Date.parse(a.createdAt))
}
export function ActivityPopover({ activity, error, retry, close }: { activity?: Activity; error?: unknown; retry: () => void; close: () => void }) {
  const [tab, setTab] = useState<'running' | 'inbox'>('running')
  const [older, setOlder] = useState<AppNotification[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [pageError, setPageError] = useState('')
  const [paging, setPaging] = useState(false)
  const history = useResource(['activity', 'history'], ({ signal }) => getNotificationHistory(signal), { enabled: tab === 'inbox' })
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
  const rows = orderNotifications([...(history.data?.notifications ?? []), ...older])
  const today = new Date(); today.setHours(0, 0, 0, 0)
  const yesterday = today.getTime() - 86400000
  const groups = [ { label: 'Hôm nay', rows: rows.filter((n) => Date.parse(n.createdAt) >= today.getTime()) }, { label: 'Hôm qua', rows: rows.filter((n) => Date.parse(n.createdAt) >= yesterday && Date.parse(n.createdAt) < today.getTime()) }, { label: 'Trước đó', rows: rows.filter((n) => Date.parse(n.createdAt) < yesterday) } ]
  if (!activity) return <section {...stylex.props(s.pop)} aria-label="Việc nền và thông báo"><div {...stylex.props(s.head)}>Việc nền & thông báo</div><div role={error ? 'alert' : 'status'} {...stylex.props(s.empty)}>{error ? <>Không tải được việc nền. <button {...stylex.props(s.plain)} onClick={retry}>Thử lại</button></> : 'Đang tải…'}</div></section>
  return (
    <section {...stylex.props(s.pop)} role="dialog" aria-label="Việc nền">
      <div {...stylex.props(s.head)}>
        <div {...stylex.props(s.content)}>Việc nền<span {...stylex.props(s.sub)}>{tasks.length ? `Máy chạy ${activity.videoJobs} video · ${activity.queued} chờ lượt · ${tasks.filter((task) => task.kind !== 'video').length} việc khác` : `Máy rảnh · ${activity.queued} chờ lượt`}</span></div>
        <a href="#/settings?section=notifications" aria-label="Cài đặt thông báo" {...stylex.props(s.plain)} onClick={close}><Icon name="setting" size={16} /></a>
        <button type="button" aria-label="Đóng (Esc)" {...stylex.props(s.plain)} onClick={close}><Icon name="close" size={16} /></button>
      </div>
      <div role="tablist" aria-label="Việc nền" {...stylex.props(s.tabs)}>
        <button role="tab" aria-selected={tab === 'running'} {...stylex.props(s.tab, tab === 'running' && s.active)} onClick={() => setTab('running')}>Đang chạy · {tasks.length}</button>
        <button role="tab" aria-selected={tab === 'inbox'} {...stylex.props(s.tab, tab === 'inbox' && s.active)} onClick={() => setTab('inbox')}>Thông báo · {activity.unread.length}</button>
      </div>
      <div {...stylex.props(s.scroll)}>
        {tab === 'running' ? tasks.length ? <>
          {(['video', 'other'] as const).map((group) => {
            const selected = tasks.filter((task) => group === 'video' ? task.kind === 'video' : task.kind !== 'video')
            return selected.length ? <div key={group}><div {...stylex.props(s.group)}>{group === 'video' ? 'Video' : 'Khác'} <span>{selected.length}</span></div>{selected.map((task) => <Task key={task.id} task={task} refresh={refresh} />)}</div> : null
          })}
          {!!activity.queued && <div {...stylex.props(s.row)}><a href={`#${paths.queue()}`} {...stylex.props(s.button)}>{activity.queued} chờ lượt</a></div>}
        </> : <div {...stylex.props(s.empty)}>Máy đang rảnh<br />Không có video nào đang chạy hay chờ lượt.<div><a href={`#${paths.new()}`} {...stylex.props(s.button)}>Làm video</a> <a href="#/explore" {...stylex.props(s.button)}>Mở Khám phá</a></div></div>
          : history.error ? <div role="alert" {...stylex.props(s.empty)}>Không tải được thông báo. <button {...stylex.props(s.plain)} onClick={() => void history.refresh()}>Thử lại</button></div>
          : history.initialLoading ? <div {...stylex.props(s.empty)}>Đang tải…</div>
          : rows.length ? groups.map((group) => group.rows.length ? <div key={group.label}>
            <div {...stylex.props(s.group)}>{group.label}{group.label === 'Hôm nay' && <button {...stylex.props(s.plain)} onClick={() => void mark()}>Đánh dấu đã đọc</button>}</div>
            {group.rows.map((n) => <div key={n.id} {...stylex.props(s.row)}>
              {!n.readAt && <span aria-label="Chưa đọc" {...stylex.props(s.dot)} />}
              <span {...stylex.props(s.taskIcon)}><Icon name={n.kind === 'needs_attention' ? 'danger' : 'flag'} size={15} /></span>
              <div {...stylex.props(s.content)}><span {...stylex.props(s.title)}>{n.text}</span><span {...stylex.props(s.sub)}>{new Date(n.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span></div>
              {n.href && <a {...stylex.props(s.button)} href={n.href} onClick={() => { if (!n.readAt) void markNotificationsRead([n.id]).then(refresh); close() }}>{n.actionLabel || 'Xem'}</a>}
            </div>)}
          </div> : null) : <div {...stylex.props(s.empty)}>Chưa có thông báo.</div>}
        {tab === 'inbox' && (cursor ?? history.data?.nextCursor) ? <div {...stylex.props(s.row)}><button type="button" {...stylex.props(s.button)} disabled={paging} onClick={() => void loadMore()}>{paging ? 'Đang tải…' : 'Xem thêm'}</button>{pageError && <span role="alert" {...stylex.props(s.sub)}>{pageError}</span>}</div> : null}
      </div>
      <footer {...stylex.props(s.footer)}><a {...stylex.props(s.button)} href={`#${paths.queue()}`} onClick={close}>Mở Hàng chờ</a><a {...stylex.props(s.button)} href="#/settings?section=notifications" onClick={close}>Cài đặt thông báo</a></footer>
    </section>
  )
}
