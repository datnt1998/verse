import { useEffect, useMemo, useRef, useState } from 'react'
import type { ActivityEvent } from '../../../api/types'
import { useActivityHistory } from '../../../api/useActivityHistory'
import { useStoryEvents, type StoryEvents } from '../../../api/useStoryEvents'
import { useSession } from '../../../api/session'
import { useAction, useConnection } from '../../../data'
import { navigate, paths } from '../../../router'
import { Icon } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'
import { Skeleton } from '@noverse/ui'
import { useMobileToast } from '../../../mobile/primitives'
import { t } from '../../../i18n/vi'
import { activityText, userEventActor } from '../../../lib/activity'
import { agentRoleLabel, toolChipText } from '../../../lib/labels'
import { formatDuration, formatTimer } from '../../../lib/format'
import { eventToolCards, groupChapters, highlightedParts, initialLogFilter, isLogError, isMine, LOG_FILTERS, logAgent, logGroupKey, logQuery, logTime, matchesLog, mlogToolIcon, nextUnseenEvents, toolArgRows, toolDetailText, toolResultText, type LogFilter } from './logic'
import './mobile.css'
import { copyText } from '../../../lib/clipboard'
import { ReviewIssue } from '../../../ui/ReviewIssue'

const NOOP = () => {}
const AGENTS: Record<string, { label: string; color: string }> = {
  writer: { label: agentRoleLabel('writer'), color: '#DCC8F8' }, architect: { label: agentRoleLabel('architect'), color: '#C8D3F8' },
  editor: { label: agentRoleLabel('editor'), color: '#F8D9B0' }, arbiter: { label: agentRoleLabel('arbiter'), color: '#BDEFD9' },
}
const EMPTY: ActivityEvent[] = []
function Highlight({ text, query }: { text: string; query: string }) {
  return highlightedParts(text, query).map((part, i) => part.match ? <mark key={i}>{part.text}</mark> : part.text)
}
function Orb({ agent, live = false }: { agent: string; live?: boolean }) {
  return <span aria-hidden="true" className={`mlog-orb mlog-orb-${logAgent(agent)}${live ? ' breathing' : ''}`} />
}

/** Đồng hồ mỗi giây cho thẻ công cụ đang chạy (mục 9 — số đang chạy). */
function useNow(running: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!running) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [running])
  return now
}
/**
 * Ô thời lượng của thẻ công cụ: xong → “1,4 giây” (formatDuration); lỗi → “lỗi”;
 * đang chạy → đồng hồ `m:ss` tích mỗi giây trong ô cố định căn phải (07-copy.md › Kiểu chữ).
 */
function ToolDuration({ event, error, running, duration }: { event: ActivityEvent; error: boolean; running: boolean; duration: number }) {
  const now = useNow(running)
  const elapsed = running ? Math.max(0, now - Date.parse(event.time)) : 0
  const text = error ? 'lỗi' : running ? formatTimer(elapsed) : duration > 0 ? formatDuration(duration) : ''
  return <span className={error ? 'mlog-duration error' : 'mlog-duration'} role={running ? 'timer' : undefined} aria-label={running ? `Đã chạy ${Math.floor(elapsed / 1000)} giây` : undefined}>{text}</span>
}

/** Chi tiết công cụ mở ra (MLog.dc.html › .toolio): bảng nhãn·giá trị + một dòng kết quả; JSON thô chỉ qua nút chép. */
function ToolDetail({ event, query, onCopy, copying, pending }: { event: ActivityEvent; query: string; onCopy: () => void; copying: boolean; pending: boolean }) {
  const rows = toolArgRows(event)
  const out = toolResultText(event)
  const bad = isLogError(event)
  return <>
    {(rows.length > 0 || out) && <div className="mlog-io">
      {rows.map((row) => <div className="mlog-io-row" key={row.label}><span className="mlog-io-label">{row.label}</span><span className="mlog-io-value"><Highlight text={row.value} query={query} /></span></div>)}
      {!!out && <div className={`mlog-out${bad ? ' bad' : ''}`}><Icon name={bad ? 'warning-2' : 'tick-circle'} size={14} /><span><Highlight text={out} query={query} /></span></div>}
    </div>}
    <button type="button" className="mlog-copy mlog-chip" aria-busy={pending} disabled={copying} onClick={onCopy}><Icon name="copy" size={14} />{t.activity.copyTechnical}</button>
  </>
}

/** M6 uses the GAP-31 paged history and the existing worker live stream, never a second global SSE. */
export function MobileActivity({ project, story }: { project: string; story: string }) {
  const [filter, setFilter] = useState<LogFilter>(() => initialLogFilter(location.href))
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  useEffect(() => { const timer = setTimeout(() => setDebouncedQuery(query), 220); return () => clearTimeout(timer) }, [query])
  const live = useStoryEvents(project, story, NOOP)
  return <main className="mlog" aria-label="Nhật ký hoạt động">
    <header className="mlog-bar">
      <button type="button" className="mlog-icon" aria-label="Quay lại Viết" onClick={() => navigate(paths.workspace(project, story))}><Icon name="arrow-left-2" size={22} /></button>
      {searching ? <>
        <label className="kit-frame mlog-search"><Icon name="search-normal-1" size={17} /><input autoFocus type="search" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Tìm trong nhật ký" placeholder="Tìm sự kiện, công cụ, lỗi…" /></label>
        <button className="mlog-cancel" type="button" onClick={() => { setSearching(false); setQuery(''); setDebouncedQuery('') }}>Huỷ</button>
      </> : <>
        <div className="mlog-heading"><strong>Nhật ký hoạt động</strong><LiveSummary project={project} story={story} connected={live.connected} blocked={live.blocked !== null} events={live.events as ActivityEvent[]} /></div>
        <button type="button" className="mlog-icon mlog-find" aria-label="Tìm trong nhật ký" onClick={() => setSearching(true)}><Icon name="search-normal-1" size={21} /></button>
      </>}
    </header>
    <ActivityFilters project={project} story={story} filter={filter} setFilter={setFilter} query={debouncedQuery} live={live} />
    <ActivityFeed key={`${project}/${story}/${filter}/${debouncedQuery}`} project={project} story={story} filter={filter} query={debouncedQuery} live={live} />
  </main>
}
function LiveSummary({ project, story, connected, blocked, events }: { project: string; story: string; connected: boolean; blocked: boolean; events: ActivityEvent[] }) {
  const history = useActivityHistory(project, story, logQuery('all', ''))
  const head = history.data?.events[0]?.time
  const additional = new Set(events.filter((event) => head && Date.parse(event.time) > Date.parse(head) && !history.data?.events.some((row) => row.id === event.id)).map((event) => event.id)).size
  return <span className="mlog-subtitle" title={history.updatedAt ? `Cập nhật lúc ${new Date(history.updatedAt).toLocaleTimeString('vi-VN')}` : undefined}><i className={`mlog-live-dot${connected ? '' : ' disconnected'}`} aria-hidden="true" />{connected ? 'Đang ghi trực tiếp' : blocked ? 'Nhật ký đã lưu' : 'Đang kết nối lại'} · {(history.data?.counts.total ?? 0) + additional} sự kiện</span>
}

/** Giữ chip lọc ngoài feed có key theo truy vấn, để đổi lọc chỉ đổi màu, không dựng lại DOM. */
function ActivityFilters({ project, story, filter, setFilter, query, live }: {
  project: string; story: string; filter: LogFilter; setFilter: (filter: LogFilter) => void; query: string; live: StoryEvents
}) {
  const { user } = useSession()
  const overview = useActivityHistory(project, story, logQuery('all', query))
  const filters = useRef<HTMLDivElement>(null)
  useEffect(() => { filters.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest', inline: 'center' }) }, [filter])
  const overviewEvents = overview.data?.events ?? EMPTY
  const extras = (live.events as ActivityEvent[]).filter((event) => matchesLog(event, 'all', query, user?.username) && Date.parse(event.time) > Date.parse(overviewEvents[0]?.time ?? '') && !overviewEvents.some((item) => item.id === event.id))
  const errorCount = (overview.data?.counts?.errors ?? 0) + extras.filter((event) => matchesLog(event, 'error', '', user?.username)).length
  return <div className="mlog-filter-edge"><div className="mlog-filters" ref={filters} role="tablist" aria-label="Lọc nhật ký">{LOG_FILTERS.map((item) => <button type="button" role="tab" aria-selected={filter === item.id} className={`mlog-chip${filter === item.id ? ' on' : ''}`} key={item.id} onClick={() => setFilter(item.id)}>
    {AGENTS[item.id] && <Orb agent={item.id} />}{item.id === 'error' && <i className="mlog-error-dot" />}{item.label}{item.id === 'error' && <span>{errorCount}</span>}
  </button>)}</div></div>
}
function ActivityFeed({ project, story, filter, query, live }: {
  project: string; story: string; filter: LogFilter; query: string; live: StoryEvents
}) {
  const { user } = useSession()
  const incoming = useMemo(() => (live.events as ActivityEvent[]).filter((event) => matchesLog(event, filter, query, user?.username)), [live.events, filter, query, user?.username])
  const history = useActivityHistory(project, story, logQuery(filter, query), incoming)
  const overview = useActivityHistory(project, story, logQuery('all', query))
  const connection = useConnection()
  const { events: all, chapters, more, loadOlder } = history
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [newIds, setNewIds] = useState<Set<string>>(new Set())
  const [freshIds, setFreshIds] = useState<Set<string>>(new Set())
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    setSlow(false)
    if (!history.initialLoading) return
    const timer = setTimeout(() => setSlow(true), 8000)
    return () => clearTimeout(timer)
  }, [history.status])
  const scroll = useRef<HTMLDivElement>(null)
  const away = useRef(false)
  const known = useRef(new Set<string>())
  const pending = useRef(new Set<string>())
  const baseline = useRef<number | null>(null)
  const groups = useMemo(() => groupChapters(all, chapters, live.snapshot?.inProgressChapter || live.snapshot?.currentChapter || 0, live.snapshot?.isRunning ?? false), [all, chapters, live.snapshot])
  useEffect(() => {
    if (!history.data) return
    if (baseline.current === null) {
      baseline.current = Date.parse(history.data.events[0]?.time ?? new Date().toISOString())
      known.current = new Set(history.data.events.map((event) => event.id))
    }
    const previous = known.current
    const next = nextUnseenEvents(previous, pending.current, incoming, away.current, baseline.current)
    const fresh = incoming.filter((event) => !previous.has(event.id) && Date.parse(event.time) > baseline.current!).map((event) => event.id)
    known.current = next.known
    pending.current = next.pending
    setNewIds(next.pending)
    if (fresh.length) setFreshIds((ids) => new Set([...ids, ...fresh]))
  }, [incoming, history.data])
  useEffect(() => {
    if (!freshIds.size) return
    const timer = setTimeout(() => setFreshIds(new Set()), 550)
    return () => clearTimeout(timer)
  }, [freshIds])
  const showOlder = async () => {
    const page = await loadOlder.run()
    if (page) setExpanded((previous) => ({ ...previous, ...Object.fromEntries(page.events.map((event) => [logGroupKey(event), true])) }))
  }
  return <>
    <PanelRefreshBar resourceKeys={[['story', project, story, 'activity']]} updatedAt={history.updatedAt} />
    <StaleBanner error={history.error || overview.error || (!connection.writable || (!live.connected && !live.blocked) ? new Error('Mất kết nối · nhật ký sẽ tự cập nhật khi có mạng') : null)} onRetry={() => { connection.retry(); live.retryNow(); void history.refresh(); void overview.refresh() }} />
    <div className="mlog-scroll" ref={scroll} onScroll={(event) => {
      away.current = event.currentTarget.scrollTop >= 20
      if (!away.current && pending.current.size) { pending.current = new Set(); setNewIds(new Set()) }
    }}>
      {history.initialLoading && !all.length && <div className="mlog-loading" aria-label="Đang tải nhật ký">{[0, 1, 2, 3].map((i) => <div key={i}><Skeleton height={16} width="55%" /><Skeleton height={40} /></div>)}{slow && <small role="status">Đang chậm hơn bình thường…</small>}</div>}
      {history.status === 'error' && !all.length ? <div className="mlog-empty" role="alert"><Icon name="warning-2" size={28} /><strong>Chưa tải được nhật ký</strong><span>{history.error instanceof Error ? history.error.message : 'Vui lòng thử lại.'}</span><button type="button" className="mlog-chip" onClick={() => void history.refresh()}>Thử lại</button></div> : null}
      {history.status === 'ready' && !groups.length && <div className="mlog-empty"><div className="mlog-empty-icon"><Icon name="search-normal-1" size={24} /></div><strong>Không có sự kiện phù hợp</strong><span>Thử bỏ bộ lọc hoặc tìm từ khác.</span></div>}
      {groups.map((group) => {
        const open = expanded[group.key] ?? (group.live || filter !== 'all' || !!query || group === groups[0])
        const label = group.label
        return <section key={group.key} aria-label={label}>
          <button className="mlog-chapter" type="button" aria-expanded={open} onClick={() => setExpanded((previous) => ({ ...previous, [group.key]: !open }))}>
            {group.live ? <Orb agent={group.agent ?? 'writer'} live /> : <Icon name="book-1" size={15} style={{ color: 'var(--color-fg-4)' }} />}
            <span className="mlog-chapter-copy"><strong title={label}>{label}</strong><small className={group.live ? 'live' : ''}>{filter !== 'all' || query ? `${group.events.length} kết quả` : group.live ? group.work : t.activity.logSummary(group.score, group.count || group.events.length)}</small></span>
            <Icon name={open ? 'arrow-up-2' : 'arrow-down-2'} size={16} className="mlog-chevron" />
          </button>
          {open && <ol className="mlog-events">{group.events.map((event) => <EventRow key={event.id} event={event} query={query} fresh={freshIds.has(event.id)} total={live.snapshot?.totalChapters} />)}</ol>}
        </section>
      })}
      {more && <button type="button" className="mlog-older" aria-busy={loadOlder.pending} disabled={loadOlder.busy} onClick={() => void showOlder()}>{loadOlder.pending && <span className="mlog-spinner" />}{loadOlder.pending ? 'Đang tải các chương trước…' : 'Xem các chương trước'}</button>}
      {!!loadOlder.error && <div className="mlog-page-error" role="alert">{loadOlder.error instanceof Error ? loadOlder.error.message : 'Không tải được chương trước.'}<button type="button" onClick={() => void showOlder()}>Thử lại</button></div>}
    </div>
    {newIds.size > 0 && <button type="button" className="mlog-jump" aria-live="polite" onClick={() => { scroll.current?.scrollTo({ top: 0, behavior: 'smooth' }); away.current = false; pending.current = new Set(); setNewIds(new Set()) }}><Icon name="arrow-up-2" size={15} />{newIds.size} sự kiện mới</button>}
  </>
}
function EventRow({ event, query, fresh, total }: { event: ActivityEvent; query: string; fresh: boolean; total?: number }) {
  const tools = eventToolCards(event)
  const { user } = useSession()
  const error = isLogError(event)
  const mine = isMine(event, user?.username)
  const userEvent = isMine(event)
  const agent = error ? { label: 'Lỗi', color: '#FF9FA8' } : userEvent ? { label: userEventActor(event, user?.username), color: '#EDEDEF' } : AGENTS[logAgent(event.agent)] ?? { label: 'Hệ thống', color: '#A0A0AB' }
  const retry = event.retries?.filter((item) => item.ok).at(-1)
  return <li className={`mlog-event${fresh ? ' newin' : ''}${error ? ' error' : ''}`}>
    <span className="mlog-rail" aria-hidden="true" /><span className="mlog-node" aria-hidden="true">{error ? <span className="mlog-error-node"><Icon name="warning-2" size={13} /></span> : userEvent ? <span className="mlog-me">{(mine ? user?.name || user?.username : event.username)?.slice(0, 1).toLocaleUpperCase('vi') || '?'}</span> : AGENTS[logAgent(event.agent)] ? <Orb agent={event.agent} /> : <Icon name="info-circle" size={15} />}</span>
    <div className="mlog-event-body"><div className="mlog-event-meta"><strong style={{ color: agent.color }}>{agent.label}</strong><time dateTime={event.time}>{logTime(event.time)}</time></div>
      <div className="mlog-summary"><Highlight text={activityText(event, total)} query={query} /></div>
      {event.review && event.review.items.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 2 }}>
          {event.review.items.map((item) => <ReviewIssue key={item.id} item={item} variant="compact" />)}
        </div>
      )}
      {tools.map((tool, index) => <ToolCard key={`${tool.tool}/${index}`} event={tool} query={query} />)}
      {retry && <div className="mlog-retry"><Icon name="tick-circle" size={13} />Tự thử lại sau {retry.waitSec} giây · thành công</div>}
    </div>
  </li>
}

function ToolCard({ event, query }: { event: ActivityEvent; query: string }) {
  const [open, setOpen] = useState(false)
  const toast = useMobileToast()
  const error = isLogError(event)
  const running = event.status === 'running'
  const duration = event.durationMs ?? event.durationNs / 1e6
  const copy = useAction(async () => { await copyText(toolDetailText(event)); toast(t.activity.copiedTechnical) })
  const name = toolChipText({ tool: event.tool!, args: event.args })
  return <div><button type="button" className={`mlog-tool${open ? ' on' : ''}`} aria-expanded={open} onClick={() => setOpen(value => !value)}><Icon name={mlogToolIcon(event.tool!)} size={15} style={{ color: 'var(--color-accent)' }} /><span className="mlog-tool-name" title={name}><Highlight text={name} query={query} /></span>{running && <span className="mlog-spinner" aria-label="Đang chạy" />}<ToolDuration event={event} error={error} running={running} duration={duration} /><Icon name={open ? 'arrow-up-2' : 'arrow-down-2'} size={14} className="mlog-chevron" /></button>
    {open && <ToolDetail event={event} query={query} onCopy={() => void copy.run()} copying={copy.busy} pending={copy.pending} />}
  </div>
}
