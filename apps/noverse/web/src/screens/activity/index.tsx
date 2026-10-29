// ActivityScreen (#/p/:p/s/:s/activity) — 1:1 Activity.dc.html.
// Trang cuộn (ngoại lệ cùng Tuỳ chỉnh), max-width 920: lọc 7 loại + số, ẩn/hiện lệnh gọi
// công cụ, nhóm theo phiên (`HostEvent.sessionId`, G3), chi tiết kỹ thuật,
// “Thử lại ngay” cho lượt đang chờ backoff (G4), mở/chép thư mục nhật ký (G1).
import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  approveWithComments, canWriteStory, getOutline, getRevealPath, getStoryMetrics, getSystem, retryStory, revealFolder,
} from '../../api/client'
import { useSession } from '../../api/session'
import { useStoryEvents } from '../../api/useStoryEvents'
import { useActivityHistory } from '../../api/useActivityHistory'
import { setLocked } from '../../shell'
import { navigate, paths } from '../../router'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Banner } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { ActivityRow } from '../workspace/ActivityRow'
import { CommandResults, useCommandResults } from '../cmdresults'
import { eventKind, activityEmpty, canRetryNow, hasPendingRetry, pendingGateChapter, type EventKind } from '../workspace/events'
import { formatClock, formatShortClock } from '../workspace/format'
import { useNow } from '../workspace/hooks'
import { latestGateChapter } from '../workspace/GateCard'
import { groupSessions } from './groups'
import * as stylex from '@stylexjs/stylex'
import { s } from './empty.stylex'
import { metricsChapterOf } from './metrics'
import { MetricsLogRow } from './MetricsLogRow'
import { WorkspaceFeedback } from '../workspace/WorkspaceFeedback'
import { ScrollTopButton } from '@noverse/ui'
import { copyText } from '../../lib/clipboard'

const NOOP = () => {}

type Filter = 'all' | EventKind

const FILTERS: { id: Filter; icon: string }[] = [
  { id: 'all', icon: 'category' },
  { id: 'dispatch', icon: 'magicpen' },
  { id: 'review', icon: 'flag' },
  { id: 'tool', icon: 'code' },
  { id: 'decision', icon: 'shield-tick' },
  { id: 'system', icon: 'info-circle' },
  { id: 'user', icon: 'user' },
  { id: 'error', icon: 'danger' },
]

export function ActivityScreen({ project, story }: { project: string; story: string }) {
  const { user } = useSession()
  const { events: liveEvents, jobs, blocked, snapshot, loading: snapshotLoading, refreshing, updatedAt, error, refresh, connected, retryNow } = useStoryEvents(project, story, NOOP)
  const history = useActivityHistory(project, story, { limit: 500 }, liveEvents)
  const { events, more, loadOlder } = history
  const loading = snapshotLoading || history.initialLoading
  const end = useRef<HTMLDivElement>(null)
  const scroll = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (!more || loadOlder.busy || loadOlder.error || !end.current) return
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) void loadOlder.run()
    }, { root: scroll.current, rootMargin: '0px 0px 320px 0px' })
    observer.observe(end.current)
    return () => observer.disconnect()
  }, [more, loadOlder.busy, loadOlder.error, loadOlder.run])
  const commandResults = useCommandResults(project, story)
  const [filter, setFilter] = useState<Filter>('all')
  const [showTools, setShowTools] = useState(true)
  const connection = useConnection()
  const system = useResource(keys.system(), ({ signal }) => getSystem(signal))
  const canReveal = system.data?.canReveal ?? null
  const folderTarget = useMemo(() => ({ target: 'activity' as const, project, story }), [project, story])
  const folderPath = useResource([...keys.story(project, story), 'activity-path'], () => getRevealPath(folderTarget))
  const outline = useResource(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal))
  // D6 — số đo của mọi chương đã viết: dòng “Đo bằng máy · chương N” bám sau lần nộp chương tương ứng.
  const measured = useResource([...keys.story(project, story, 'chapters'), 'metrics'],
    ({ signal }) => getStoryMetrics(project, story, 'all', signal), { enabled: !!project && !!story })
  const metricsByChapter = useMemo(() => new Map((measured.data?.chapters ?? []).map((row) => [row.n, row])), [measured.data])
  const now = useNow(1000)
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const writable = canEdit && connection.writable && connected

  useEffect(() => {
    setLocked(blocked === 'locked')
    return () => setLocked(false)
  }, [blocked])


  const counts = useMemo(() => {
    const head = history.data?.events ?? []
    const additional = liveEvents.filter((event) => Date.parse(event.time) > (head[0] ? Date.parse(head[0].time) : -Infinity) && !head.some((row) => row.id === event.id)).length
    const map: Record<string, number> = { all: history.data ? history.data.counts.total + additional : events.length }
    for (const event of events) {
      const kind = eventKind(event)
      map[kind] = (map[kind] ?? 0) + 1
    }
    return map
  }, [events, history.data, liveEvents])

  const visible = useMemo(
    () => events.filter((event) => {
      const kind = eventKind(event)
      if (!showTools && kind === 'tool') return false
      return filter === 'all' || kind === filter
    }),
    [events, filter, showTools],
  )
  const groups = useMemo(() => groupSessions([...visible].reverse()).reverse(), [visible])
  const empty = activityEmpty(events.length, visible.length)
  const approveAction = useAction(() => approveWithComments(project, story, latestGateChapter(snapshot, outline.data ?? null)),
    { success: 'Đã duyệt chương', invalidate: [keys.story(project, story)] })
  const approve = () => { if (writable) void approveAction.run() }
  const retryAction = useAction(() => retryStory(project, story),
    { success: 'Đã yêu cầu thử lại', invalidate: [keys.story(project, story)] })
  const retry = () => { if (writable) void retryAction.run() }
  const folderAction = useAction(async () => {
    if (canReveal) {
      const { path } = await revealFolder(folderTarget)
      return path
    }
    if (!folderPath.data) throw new Error('Chưa tải được đường dẫn nhật ký')
    await copyText(folderPath.data.path)
    return folderPath.data.path
  }, { success: canReveal ? 'Đã mở thư mục nhật ký' : t.activity.copied })

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, position: 'relative', display: 'flex' }}>
    <main ref={scroll} className="scroll" style={{ flex: 1, minWidth: 0, minHeight: 0 }}>
      <div style={{ maxWidth: 920, margin: '0 auto', padding: '32px 24px 64px', display: 'flex', flexDirection: 'column', gap: 20 }}>
        <PanelRefreshBar resourceKeys={[keys.story(project, story), keys.system()]} updatedAt={history.updatedAt ?? updatedAt} style={{ minHeight: 2, height: 2, overflow: 'hidden' }} />
        <WorkspaceFeedback admin={user?.role === 'admin'}
          error={history.error || loadOlder.error || error || system.error || folderPath.error || (!connection.writable || !connected ? new Error('Không kết nối được máy chủ. Vui lòng thử lại.') : null)}
          onRetry={() => { retryNow(); connection.retry(); void refresh(); void history.refresh(); if (loadOlder.error) void loadOlder.run(); void system.refresh(); void folderPath.refresh() }} />
        {!canEdit && <Banner tone="info" title={t.locked.readonly}
          action={{ label: 'Mở Trình đọc', onClick: () => navigate(paths.reader(project, story)) }}>
          Bạn chỉ có quyền đọc truyện này — không gửi yêu cầu sửa được.
        </Banner>}
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'flex-end', justifyContent: 'space-between', gap: 16 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 }}>
            <h1 ref={headingRef} tabIndex={-1} style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-0.02em' }}>{t.activity.title}</h1>
            <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-3)' }}>{t.activity.sub}</p>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8 }}>
            {canReveal !== null && (
              <Button size="sm" icon={<Icon name={canReveal ? 'folder-open' : 'copy'} size={15} />}
                pending={folderAction.pending} success={folderAction.success} disabled={folderAction.busy || (!canReveal && !folderPath.data)}
                onClick={() => void folderAction.run()} style={{ height: 34, fontSize: 13 }}>
                {canReveal ? t.activity.openFolder : t.activity.copyPath}
              </Button>
            )}
            <button type="button" className="hov-ghost" aria-pressed={showTools} onClick={() => setShowTools((value) => !value)}
              style={{
                height: 34, padding: '0 12px 0 10px', display: 'flex', alignItems: 'center', gap: 8,
                borderRadius: 9, background: 'transparent', border: '1px solid var(--line-4)',
                color: 'var(--color-fg-2)', fontSize: 13,
              }}>
              <Icon name="code" size={16} />
              {showTools ? t.activity.hideTools : t.activity.showTools}
            </button>
          </div>
        </div>

        <Segmented label={t.activity.filters.all} value={filter} onChange={(next) => setFilter(next as Filter)}
          size={32} radius={999} thumbStyle={{ background: 'var(--color-surface-6)', boxShadow: 'none' }}
          style={{ flexWrap: 'wrap', gap: 6, padding: 0, background: 'transparent', border: 'none' }}
          options={FILTERS.map((item) => ({
            id: item.id,
            icon: <Icon name={item.icon} size={15} />,
            label: <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              {t.activity.filters[item.id]}
              <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{counts[item.id] ?? 0}</span>
            </span>,
            style: {
              flex: '0 0 auto', padding: '0 12px 0 10px', fontSize: 13, gap: 8,
              border: `1px solid ${filter === item.id ? 'var(--line-5)' : 'var(--line-3)'}`,
            },
          }))} />
        {(filter === 'all' || filter === 'system') && (
          <CommandResults ctl={commandResults} jobs={jobs} project={project} story={story} />
        )}

        {loading && (
          <div className="card" style={{ borderRadius: 18, padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
            {[0, 1, 2, 3, 4].map((row) => <Skeleton key={row} height={16} width={`${86 - row * 9}%`} />)}
          </div>
        )}

        {groups.map((group) => (
          <section key={`${group.id}-${group.from}`} className="card" style={{ borderRadius: 18, padding: '8px 8px 10px' }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px 12px', padding: '12px 12px 10px' }}>
              <h2 style={{ margin: 0, fontSize: 14, fontWeight: 600 }}>
                {t.activity.session(formatShortClock(group.from), group.running ? t.activity.groupRunning : formatShortClock(group.to))}
              </h2>
              <span style={{
                height: 22, padding: '0 8px', display: 'flex', alignItems: 'center', borderRadius: 6,
                fontSize: 11, fontWeight: 500, background: 'var(--color-surface-5)', color: 'var(--color-fg-3)',
              }}>{t.activity.groupTag(group.events.length)}</span>
              <span style={{ flex: 1 }} />
              <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
                {formatClock(group.from)} → {group.running ? t.activity.groupRunning : formatClock(group.to)}
              </span>
            </div>
            <ol style={{ listStyle: 'none', margin: 0, padding: 0 }}>
              {group.events.map((event, index) => {
                const chapter = pendingGateChapter(translateHost(event.summary))
                const actions: ReactNode[] = []
                if (writable && hasPendingRetry(event)) {
                  if (canRetryNow(event)) {
                    actions.push(
                      <Button key="retry" size="sm" icon={<Icon name="refresh-2" size={14} />}
                        onClick={retry} disabled={retryAction.busy} pending={retryAction.pending} success={retryAction.success}>{t.activity.retryNow}</Button>,
                    )
                  }
                  actions.push(
                    <Button key="model" size="sm" icon={<Icon name="cpu" size={14} />}
                      onClick={() => navigate(paths.settings('roles'))}>{t.activity.changeModel}</Button>,
                  )
                }
                if (writable && chapter !== null) {
                  actions.push(
                    <Button key="approve" size="sm" onClick={approve} disabled={approveAction.busy || outline.status !== 'ready'}
                      pending={approveAction.pending} success={approveAction.success}>{t.activity.approveChapter(chapter)}</Button>,
                  )
                }
                const metricsChapter = metricsChapterOf(event)
                const measuredRow = metricsChapter === null ? undefined : metricsByChapter.get(metricsChapter)
                return (
                  <Fragment key={event.id || `${group.id}-${index}`}>
                    <ActivityRow event={event} now={now} detail total={snapshot?.totalChapters}
                      action={actions.length > 0
                        ? <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>{actions}</div>
                        : undefined} />
                    {metricsChapter !== null && measuredRow
                      && <MetricsLogRow chapter={metricsChapter} metrics={measuredRow.metrics} tiredWord={measuredRow.tiredWord} time={event.time} />}
                  </Fragment>
                )
              })}
            </ol>
          </section>
        ))}
        {more && <div ref={end} aria-hidden="true" style={{ height: 1, flexShrink: 0 }} />}

        {!loading && empty === 'noActivity' && (
          <EmptyState icon="activity" title={t.empty.noActivity} hint={t.empty.noActivityHint} />
        )}
        {!loading && empty === 'filtered' && (
          <p {...stylex.props(s.filteredEmpty)}>{t.empty.events}</p>
        )}
      </div>
    </main>
      <ScrollTopButton target={scroll} focusTarget={headingRef} label={t.scrollTop.page} size="md" style={{ right: 24, bottom: 24 }} />
    </div>
  )
}
