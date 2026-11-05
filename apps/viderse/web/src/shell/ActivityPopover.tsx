import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Segmented } from '@noverse/ui'
import type { Activity, BackgroundTask } from '../api/activity'
import { getQueue } from '../api/clone'
import { listChannels } from '../api/channels'
import { request } from '../api/request'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import { paths } from '../router'
import { NotificationRow, useNotificationInbox } from './notificationList'

// Tab Cài đặt thông báo mở đúng route #/settings?section=notifications —
// cùng đích với sheet Việc nền trên điện thoại.

const s = stylex.create({
  pop: { position: 'fixed', top: 'calc(var(--vv-s-48) + var(--vv-s-2))', right: 'var(--vv-s-16)', zIndex: 100, width: 448, maxWidth: 'calc(100vw - 24px)', maxHeight: 'calc(100vh - 66px)', display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: 'var(--vv-r-16)', backgroundColor: 'var(--kit-surface-15)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-4)', boxShadow: 'var(--kit-shadow-pop)', color: 'var(--color-fg)', outline: 'none' },
  /* Mũi nhọn 12px của artboard Activity: nền và viền trùng popover, ngay dưới viên Việc nền. */
  caret: { position: 'fixed', top: 44, width: 12, height: 12, transform: 'rotate(45deg)', backgroundColor: 'var(--kit-surface-15)', borderLeftWidth: 1, borderTopWidth: 1, borderLeftStyle: 'solid', borderTopStyle: 'solid', borderLeftColor: 'var(--line-4)', borderTopColor: 'var(--line-4)', zIndex: 101 },
  head: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', height: 68, boxSizing: 'border-box', flexShrink: 0, padding: 'var(--vv-s-14) var(--vv-s-12) var(--vv-s-10) var(--vv-s-16)', borderBottom: '1px solid var(--line-1)', fontSize: 15, fontWeight: 600 },
  headText: { gap: 'var(--vv-s-1)' },
  headButton: { width: 30, height: 30, flexShrink: 0 },
  headLink: { width: 30, height: 30, flexShrink: 0, display: 'grid', placeItems: 'center', borderRadius: 'var(--vv-r-8)', color: 'var(--color-fg)', textDecoration: 'none' },
  /* Nhóm chọn của popover là `.vv-seg` của Kit, không phải thanh tab giữa trang. */
  seg: { alignSelf: 'flex-start' },
  segButton: { fontSize: 13, gap: 'var(--vv-s-7)' },
  tabCount: { minWidth: 18, height: 18, padding: '0 var(--vv-s-5)', borderRadius: 'var(--vv-r-pill)', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', fontSize: 11, fontWeight: 600 },
  activeCount: { backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)' },
  scroll: { overflowY: 'auto', minHeight: 100, paddingBottom: 'var(--vv-s-6)' },
  group: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 'var(--vv-s-12) var(--vv-s-16) var(--vv-s-6)', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--color-fg-3)' },
  markRead: { height: 26 },
  row: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16)' },
  taskDivider: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--kit-wash-5)' },
  content: { minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' },
  title: { fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.55, overflowWrap: 'anywhere' },
  taskStep: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  dot: { width: 7, height: 7, borderRadius: '50%', marginTop: 'var(--vv-s-6)', flexShrink: 0, backgroundColor: 'var(--color-warn)' },
  taskIcon: { width: 28, height: 28, borderRadius: '50%', display: 'grid', placeItems: 'center', backgroundColor: 'var(--color-surface-6)', color: 'var(--color-fg-3)', flexShrink: 0 },
  /* `.mini ghost` của artboard Activity: nút 28px cạnh dòng việc / khối rảnh. */
  mini: { height: 28, boxSizing: 'border-box', flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 'var(--vv-s-6)', padding: '0 var(--vv-s-10)', borderRadius: 'var(--vv-r-7)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--kit-wash-12)', backgroundColor: 'transparent', color: 'var(--color-fg)', fontSize: 12.5, whiteSpace: 'nowrap', cursor: 'pointer', textDecoration: 'none' },
  /* Nút 32px ở chân popover (`vv-btn--sm --h:32`). */
  sm32: { height: 32, boxSizing: 'border-box', flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 'var(--vv-s-6)', padding: '0 var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--kit-wash-12)', backgroundColor: 'transparent', color: 'var(--color-fg)', fontSize: 12, whiteSpace: 'nowrap', cursor: 'pointer', textDecoration: 'none' },
  /* Dòng “N chờ lượt” của artboard: khung nét đứt 10px, icon đồng hồ, mũi tên phải. */
  queueLink: { margin: 'var(--vv-s-2) var(--vv-s-12) var(--vv-s-4)', padding: 'var(--vv-s-9) var(--vv-s-10)', borderRadius: 'var(--vv-r-10)', borderWidth: 1, borderStyle: 'dashed', borderColor: 'var(--kit-wash-12)', display: 'flex', alignItems: 'center', gap: 'var(--vv-s-10)', fontSize: 12.5, color: 'var(--color-fg-2)', textDecoration: 'none' },
  queueText: { flex: 1, minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  queueIcon: { color: 'var(--color-fg-3)', flexShrink: 0 },
  progress: { width: '100%', height: 4, accentColor: 'var(--color-accent)' },
  noteRow: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-10) var(--vv-s-16) var(--vv-s-10) var(--vv-s-12)' },
  noteContent: { gap: 'var(--vv-s-2)' },
  noteTitle: { lineHeight: 1.4 },
  noteDetail: { lineHeight: 1.45, color: 'var(--vv-fg-note)' },
  noteTime: { fontSize: 11.5, lineHeight: 1.55, color: 'var(--vv-fg-note)' },
  noteDot: { marginTop: 'var(--vv-s-12)' },
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

function Task({ task, refresh, close, first = false }: { task: BackgroundTask; refresh: () => void; close?: () => void; first?: boolean }) {
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
  return <div {...stylex.props(s.row, !first && s.taskDivider)}>
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
    {task.action.href ? <a {...stylex.props(s.mini)} href={task.action.href} onClick={close}>{task.action.label}</a> : task.action.op ? <Button size="sm" variant="ghost" xstyle={s.mini} onClick={() => void act()}>{task.action.label}</Button> : null}
  </div>
}

export function ActivityPopover({ activity, error, retry, close, anchorX, anchor }: { activity?: Activity; error?: unknown; retry: () => void; close: () => void; /** tâm viên Việc nền (px) — mũi nhọn chỉ vào đúng nút */ anchorX?: number; /** nút mở popover: bấm ra ngoài thì đóng, Esc/Đóng trả focus về đây */ anchor?: HTMLElement | null }) {
  const [tab, setTab] = useState<'running' | 'inbox'>('running')
  const pop = useRef<HTMLElement>(null)
  const tabsId = useId()
  const panelId = `${tabsId}-panel`
  const inbox = useNotificationInbox(tab === 'inbox' || (tab === 'running' && !!activity && activity.running.length === 0 && activity.queued === 0))
  const { history, rows, groups, more, paging, pageError, loadMore, mark, refresh } = inbox
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal), { enabled: tab === 'running' && !!activity?.queued })
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal), { enabled: tab === 'running' && !!activity?.queued })
  // anchor là khung bọc viên Việc nền; focus phải vào chính <a>/<button> bên trong.
  const focusTrigger = () => {
    const el = anchor?.querySelector<HTMLElement>('a[data-jobs], button[data-jobs]') ?? anchor?.querySelector<HTMLElement>('a, button')
    el?.focus()
  }
  useEffect(() => {
    const dismiss = () => { focusTrigger(); close() }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') dismiss() }
    const outside = (event: PointerEvent) => {
      const target = event.target as Node
      if (pop.current?.contains(target) || anchor?.contains(target)) return
      close()
    }
    window.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', outside)
    return () => { window.removeEventListener('keydown', onKey); document.removeEventListener('pointerdown', outside) }
  }, [anchor, close])
  const tasks = activity?.running ?? []
  const nextQueued = queue.data?.items[0]
  const nextChannel = channels.data?.channels.find((channel) => channel.id === nextQueued?.channelId)
  /* Mũi nhọn chỉ vào tâm viên Việc nền; kẹp trong lòng popover để không tràn mép. */
  const viewport = typeof window === 'undefined' ? 1440 : window.innerWidth
  const popWidth = Math.min(448, viewport - 24)
  const rightEdge = viewport - 16
  const caretLeft = Math.max(rightEdge - popWidth + 14, Math.min((anchorX ?? rightEdge - 178) - 6, rightEdge - 26))
  useEffect(() => { pop.current?.focus({ preventScroll: true }) }, [])
  const frame = (body: ReactNode) => {
    const caret = stylex.props(s.caret)
    return createPortal(<><span aria-hidden="true" className={caret.className} style={{ ...caret.style, left: caretLeft }} />{body}</>, document.body)
  }
  if (!activity) return frame(<section ref={pop} tabIndex={-1} {...stylex.props(s.pop)} aria-label="Việc nền và thông báo"><div {...stylex.props(s.head)}>Việc nền & thông báo</div><div role={error ? 'alert' : 'status'} {...stylex.props(s.empty)}>{error ? <>Không tải được việc nền. <Button size="sm" variant="quiet" onClick={retry}>Thử lại</Button></> : 'Đang tải…'}</div></section>)
  return frame(
    <section ref={pop} tabIndex={-1} {...stylex.props(s.pop)} role="dialog" aria-label="Việc nền">
      <div {...stylex.props(s.head)}>
        <div {...stylex.props(s.content, s.headText)}>{'Việc nền'}<span {...stylex.props(s.sub)}>{tasks.length ? `Máy chạy ${activity.videoJobs} video · ${activity.queued} chờ lượt · ${tasks.filter((task) => task.kind !== 'video').length} việc khác` : `Máy rảnh · ${activity.queued} chờ lượt`}</span></div>
        <a href={`#${paths.settings('notifications')}`} aria-label="Cài đặt thông báo" title="Cài đặt thông báo" {...stylex.props(s.headLink)} onClick={close}><Icon name="setting" size={16} /></a>
        <Button variant="icon" size="sm" aria-label="Đóng (Esc)" xstyle={s.headButton} onClick={() => { focusTrigger(); close() }}><Icon name="close" size={16} /></Button>
      </div>
      <Segmented label="Việc nền" value={tab} onChange={(id) => setTab(id as 'running' | 'inbox')} size={32} radius={8} fit="content" tabs={{ id: tabsId, panel: panelId }} xstyle={s.seg} buttonStyle={s.segButton} options={[
        { id: 'running', label: <>{'Đang chạy'}<span {...stylex.props(s.tabCount, tab === 'running' && s.activeCount)}>{tasks.length}</span></> },
        { id: 'inbox', label: <>{'Thông báo'}<span {...stylex.props(s.tabCount, tab === 'inbox' && s.activeCount)}>{activity.unread.length}</span></> },
      ]} />
      <div id={panelId} role="tabpanel" aria-labelledby={`${tabsId}-${tab}`} {...stylex.props(s.scroll)}>
        {tab === 'running' ? tasks.length || activity.queued ? <>
          {(['video', 'other'] as const).map((group) => {
            const selected = tasks.filter((task) => group === 'video' ? task.kind === 'video' : task.kind !== 'video')
            return <div key={group}>{selected.length > 0 && <><div {...stylex.props(s.group)}>{group === 'video' ? 'Video' : 'Khác'} <span>{selected.length}</span></div>{selected.map((task, index) => <Task key={task.id} task={task} refresh={refresh} close={close} first={index === 0} />)}</>}{group === 'video' && !!activity.queued && <a href={`#${paths.queue()}`} onClick={close} {...stylex.props(s.queueLink)}><Icon name="clock" size={15} xstyle={s.queueIcon} /><span {...stylex.props(s.queueText)}><b className="num" style={{ color: 'var(--color-fg)' }}>{activity.queued}</b> chờ lượt{nextQueued && <> · tiếp theo {nextChannel?.name || nextQueued.title}{nextQueued.etaStart && <> lúc {new Date(nextQueued.etaStart).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</>}</>}</span><Icon name="arrow-right-2" size={14} xstyle={s.queueIcon} /></a>}</div>
          })}
        </> : <>
          <div {...stylex.props(s.emptyBody)}>
            <span aria-hidden="true" {...stylex.props(s.emptyOrbs)}><span {...stylex.props(s.emptyOrb, s.scout)} /><span {...stylex.props(s.emptyOrb, s.director)} /><span {...stylex.props(s.emptyOrb, s.artist)} /><span {...stylex.props(s.emptyOrb, s.qa)} /></span>
            <span {...stylex.props(s.emptyTitle)}>Máy đang rảnh</span><span {...stylex.props(s.emptyDescription)}>Không có video nào đang chạy hay chờ lượt.{activity.nextScan && <> Tự lấy video kiểm tra đối thủ lần tới lúc {new Date(activity.nextScan.at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}.</>}</span>
            <div {...stylex.props(s.emptyActions)}><a href={`#${paths.new()}`} onClick={close} {...stylex.props(s.mini)}>Làm video</a><a href="#/explore" onClick={close} {...stylex.props(s.mini)}>Mở Khám phá</a></div>
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
      <footer {...stylex.props(s.footer)}><a {...stylex.props(s.sm32)} href={`#${paths.queue()}`} onClick={close}><Icon name="clock" size={15} />Mở Hàng chờ</a><a {...stylex.props(s.sm32)} href={`#${paths.settings('notifications')}`} onClick={close}><Icon name="notify" size={15} />Cài đặt thông báo</a></footer>
    </section>
  )
}
