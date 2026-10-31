import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { ApiError, abortStory, getChapter, getLiveHistory, getOutline, getReviews, nextChapter, postCommand, resumeStory, sendInput } from '../../../api/client'
import type { ExportConflict } from '../../../api/types'
import { useSession } from '../../../api/session'
import { useStoryEvents } from '../../../api/useStoryEvents'
import { invalidate, keys, useAction, useResource } from '../../../data'
import { translateHost } from '../../../i18n/hostStrings'
import { toolLabel, saneChapter } from '../../../lib/labels'
import { activityText, deduplicateUserEvents, isInternalEvent, isUserEvent, userEventActor } from '../../../lib/activity'
import { latestRunProse, mergeRunEvents } from '../../../lib/runHistoryMerge'
import { StreamMarkdown } from '../../../lib/StreamMarkdown'
import { enumLabel } from '../../../i18n/enums'
import { navigate, paths } from '../../../router'
import { ActionSheet, useMobileToast } from '../../../mobile/primitives'
import { useMobileRuntime, useMobileScroll, useStoryConnection } from '../../../mobile/runtime'
import { setDraft } from '../../../mobile/accounts'
import { enqueueInput, useInputOutbox } from '../../../mobile/outbox'
import { useNotificationInbox } from '../../notify/useNotify'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { ProgressRing, ThinkingOrb } from '@noverse/ui'
import { useActiveJobs } from '../../../shell/jobs/useJobs'
import { isIndeterminate, kindIcon, ringColor } from '../../../shell/jobs/logic'
import { useStoryName } from '../../../shell/useNames'
import { formatShortClock } from '../format'
import { latestGateChapter } from '../GateCard'
import { CommandSheet } from './CommandSheet'
import { JobsSheet } from './JobsSheet'
import { PauseSheet } from './PauseSheet'
import { agentStatus, hasMobileResult, resultHref, retryText, reviewScore, WRITING_STEPS, writingPercent, workState, workspaceActions, type AgentRole } from './logic'
import { withExportFlag } from '../../cmdresults/logic'
import { idleModel, PendingQuestionCard } from '../IdleCard'
import './workspace.css'
import { WorkspaceComposer } from './WorkspaceComposer'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const NOOP = () => {}
const ROLES = [
  { key: 'architect', label: 'Kiến trúc sư' }, { key: 'writer', label: 'Người viết' },
  { key: 'editor', label: 'Biên tập viên' }, { key: 'arbiter', label: 'Trọng tài' },
]
const STEPS = WRITING_STEPS
/** Dải trợ lý của MWork: Người viết đứng đầu, rồi Kiến trúc sư · Biên tập viên · Trọng tài. */
const STRIP = ['writer', 'architect', 'editor', 'arbiter'].map((key) => ROLES.find((role) => role.key === key)!)
const AGENT_RGB: Record<string, string> = { writer: '154,99,227', architect: '124,147,243', editor: '236,149,72', arbiter: '93,193,176' }
const moreHref = (page?: string) => `${paths.settings('mobile')}${page ? `?page=${page}` : ''}`
const logHref = (project: string, story: string) => paths.activity(project, story)

/** Trạng thái thẻ trợ lý gọn (tablet, MTablet.dc.html): dữ liệu thật khi đang chạy, còn lại dùng chữ của artboard. */
function tabletAgentStatus(key: string, state: string | null, score: string | number | null | undefined, stale: boolean, agentState?: string): string {
  if (stale) return 'Dữ liệu cũ'
  if (state === 'gate') return key === 'editor' ? (score ? `Chấm ${score}` : 'Chờ bản nháp') : key === 'arbiter' ? 'Chờ bạn' : 'Chờ lệnh'
  if (agentState && ['running', 'working', 'thinking', 'tool'].includes(agentState)) return enumLabel('agentState', agentState)
  return key === 'editor' ? 'Chờ bản nháp' : key === 'arbiter' ? 'Chờ' : 'Chờ lệnh'
}

export function MobileWorkspace(props: { project: string; story: string; tablet?: boolean }) {
  return props.project && props.story ? <StoryWorkspace {...props} /> : <EmptyWorkspace />
}

function EmptyWorkspace() {
  const { data } = useNotificationInbox()
  return <section className="m-page mw-workspace"><header className="m-topbar"><button type="button" className="mw-story-link" onClick={() => navigate(moreHref('projects'))}><span className="mw-brand-orbs" aria-hidden="true"><span className="m-orb architect" /><span className="m-orb writer" /><span className="m-orb editor" /></span><span className="mw-story-copy"><strong>Noverse</strong><small>Chưa mở truyện nào</small></span><Icon name="arrow-down-2" size={15} /></button><button type="button" className="m-icon-button mw-bell" onClick={() => navigate('/notifications')} aria-label="Thông báo"><Icon name="notification" size={21} />{!!data?.unread && <i />}</button></header><main className="m-scroll"><div className="mw-empty"><div className="mw-empty-orbs" aria-hidden="true">{ROLES.map((role) => <span key={role.key} className={`m-orb ${role.key}`} />)}</div><h1>Chưa có truyện nào</h1><p>Tạo truyện mới hoặc nhập truyện có sẵn trên máy tính. Khi truyện bắt đầu, bạn theo dõi và duyệt ngay tại đây.</p><div className="mw-empty-actions"><button type="button" className="m-button" onClick={() => navigate(moreHref('projects'))}>Mở truyện có sẵn</button><button type="button" className="m-button secondary" onClick={() => navigate('/notifications')}>Báo tôi khi có truyện mới</button></div></div></main></section>
}

function StoryWorkspace({ project, story, tablet = false }: { project: string; story: string; tablet?: boolean }) {
  const { user } = useSession()
  const live = useStoryEvents(project, story, NOOP)
  const { snapshot, connected, blocked } = live
  const events = useMemo(() => deduplicateUserEvents(live.events.filter((event) => !isInternalEvent(event))), [live.events])
  useStoryConnection(live)
  const background = useActiveJobs()
  const inbox = useNotificationInbox()
  const notify = useMobileToast()
  const { keyboard, offline, seconds, retryNow } = useMobileRuntime()
  const scrollRef = useRef<HTMLElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const { down } = useMobileScroll(scrollRef)
  const [sheet, setSheet] = useState<'cmd' | 'jobs' | 'pause' | null>(null)
  const setText = useCallback((text: string) => setDraft(`${project}/${story}`, text), [project, story])
  const queued = useInputOutbox(project, story)
  const [initialCommand, setInitialCommand] = useState('')
  const [pauseRequested, setPauseRequested] = useState(false)
  const [conflict, setConflict] = useState<{ line: string; exists: ExportConflict } | null>(null)
  const actions = workspaceActions(user, `${project}/${story}`, tablet)
  const permitted = actions.write && !blocked
  // Rớt ngắn (<3 s) không được làm tối cả màn: runtime.tsx đã giữ cờ offline 3 s
  // (useSustained) và `offline` đã gồm cả luồng chung lẫn luồng worker, nên chỉ
  // dùng nó là đủ; nhãn “cập nhật lúc …” vẫn cho biết dữ liệu đang cũ.
  const stale = offline
  const writable = permitted && !stale
  const exportable = actions.export && !blocked && !stale
  const state = snapshot ? workState(snapshot, events) : null
  const running = state === 'running'
  const history = useResource(keys.story(project, story, 'live-history'), ({ signal }) => getLiveHistory(project, story, { limit: 20 }, signal), { enabled: running })
  const prose = useMemo(() => latestRunProse(mergeRunEvents(history.data?.rounds ?? [], live.events)), [history.data, live.events])
  const gate = state === 'gate'
  const outline = useResource(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal))
  const total = snapshot?.totalChapters
  const chapter = saneChapter(gate ? latestGateChapter(snapshot, outline.data ?? null) : snapshot?.inProgressChapter || snapshot?.currentChapter || 0, total)
    ?? (snapshot?.completedCount ?? 0)
  const volume = outline.data?.volumes.find((item) => item.chapters.some((item) => item.chapter === chapter))?.index
  const storyTitle = useStoryName(project, story, snapshot?.bookTitle)
  const volumeLabel = volume ? `Tập ${volume}` : /Tập \d+/.exec(snapshot?.currentVolumeArc ?? '')?.[0] || '…'
  const excerptResource = useResource(keys.story(project, story, 'chapter', chapter, 'draft'), ({ signal }) => getChapter(project, story, chapter, 'draft', signal), { enabled: gate && chapter > 0 })
  const reviews = useResource(keys.story(project, story, 'reviews', chapter, 'raw'), ({ signal }) => getReviews(project, story, chapter, signal), { enabled: gate && chapter > 0 })
  const excerpt = excerptResource.data
  const score = reviewScore(reviews.data?.chapter?.dimensions)
  const step = STEPS[snapshot?.flow ?? ''] ?? { number: 1, label: snapshot ? enumLabel('phase', snapshot.phase) : 'Đang kết nối' }
  const title = snapshot?.outline?.find((item) => item.chapter === chapter)?.title
  useEffect(() => { if (!running) setPauseRequested(false) }, [running])
  const send = useAction(async (message: string) => {
    if (stale) await enqueueInput({ project, story, text: message })
    else { await sendInput(project, story, message); notify('Đã gửi góp ý · Trọng tài sẽ xét') }
    setText('')
  }, { error: 'Không gửi được góp ý', invalidate: [keys.story(project, story, 'snapshot')] })
  const next = useAction(async () => { await nextChapter(project, story); notify(`Đã duyệt chương ${chapter} · đang viết chương ${chapter + 1}`) }, { error: 'Không duyệt được chương', invalidate: [keys.story(project, story)] })
  const resume = useAction(async () => { await resumeStory(project, story); notify('Đã chạy tiếp') }, { error: 'Không tiếp tục được', invalidate: [keys.story(project, story)] })
  const begin = useAction(async () => { await resumeStory(project, story); notify(`Đã chạy /next · viết chương ${(snapshot?.completedCount ?? 0) + 1}`) }, { error: 'Không bắt đầu được', invalidate: [keys.story(project, story)] })
  const retry = useAction(async () => { await resumeStory(project, story); notify('Đang thử lại') }, { error: 'Không thử lại được', invalidate: [keys.story(project, story)] })
  const pause = useAction(async (afterStep: boolean) => {
    await abortStory(project, story, afterStep)
    if (afterStep) setPauseRequested(true)
    notify(afterStep ? `Sẽ tạm dừng sau bước ${step.label}` : 'Đã dừng Người viết')
    return true
  }, { error: 'Không tạm dừng được', invalidate: [keys.story(project, story, 'snapshot')] })
  const openCommand = useCallback((line = '') => {
    if (tablet && actions.write) { setText(line || '/'); inputRef.current?.focus() }
    else { inputRef.current?.blur(); setInitialCommand(line); setSheet('cmd') }
  }, [tablet, actions.write, setText])
  const runCommand = async (line: string) => {
    if (!writable && !(/^\/export(?:\s|$)/.test(line) && exportable)) throw new Error(stale ? 'Đang mất kết nối. Vui lòng thử lại khi có mạng.' : 'Bạn chỉ có quyền đọc truyện này.')
    if (hasMobileResult(line)) { setText(''); navigate(resultHref(project, story, line)); return }
    if (/^\/help(?:\s|$)/.test(line)) { navigate(`${moreHref('guide')}${line.trim().split(/\s+/)[1] ? `&cmd=${encodeURIComponent(line.trim().split(/\s+/)[1])}` : ''}`); return }
    if (/^\/model(?:\s|$)/.test(line)) { navigate(moreHref('model')); return }
    try {
      const response = await postCommand(project, story, line)
      await invalidate(keys.story(project, story))
      setText('')
      notify(`Đã chạy ${line}`)
      if (response.result?.kind === 'job') toast('Đã bắt đầu việc nền', 'ok', { label: 'Xem', onClick: () => setSheet('jobs') })
    } catch (reason) {
      const payload = reason instanceof ApiError ? reason.payload as { result?: { exists?: ExportConflict } } | undefined : undefined
      const exists = payload?.result?.exists
      if (exists) { setConflict({ line, exists }); return }
      throw reason
    }
  }
  const command = useAction(runCommand, { error: 'Không chạy được lệnh' })
  const submit = (message: string) => {
    const value = message.trim()
    if (!value || !permitted || send.busy || command.busy) return
    if (value.startsWith('/')) { if (tablet) { if (writable) void command.run(value) } else openCommand(value) }
    else void send.run(value)
  }
  const focusNote = () => { inputRef.current?.focus() }
  const latestError = [...events].reverse().find((event) => event.failed || event.level === 'error')
  const recentReview = [...events].reverse().find((event) => !event.failed && (event.kind.includes('review') || event.summary.includes('duyệt')))
  const ringJob = background.find((job) => job.kind === 'export') ?? background[0]
  const progress = Math.round((ringJob?.progress ?? 0) * 100)
  const percent = writingPercent(snapshot?.flow)
  const waitingChapter = saneChapter(snapshot?.currentChapter ?? 0, total) || (snapshot?.completedCount ?? 0) + 1
  const updatedClock = live.updatedAt ? formatShortClock(new Date(live.updatedAt).toISOString()) : ''
  const pendingQuestion = snapshot?.pendingAsk ? idleModel(snapshot) : null
  const content = <>
    {blocked && <div className="m-error mw-pad" role="alert">{blocked === 'forbidden' ? 'Bạn không có quyền mở truyện này.' : 'Truyện đang mở trên máy khác. Bạn có thể quay lại sau.'}</div>}
    <PanelRefreshBar resourceKeys={[keys.story(project, story, 'snapshot'), outline.key, excerptResource.key, reviews.key]} updatedAt={live.updatedAt} />
    {live.error != null && connected && <div className="m-error" role="alert">Không tải lại được · <button className="m-chip" onClick={() => void live.refresh()}>Thử lại</button></div>}
    {!snapshot && !blocked && <section className="m-card mw-skeleton" role="status"><span /> <span /> <span /><p className="m-muted">Đang tải truyện…</p>{live.error != null && <button className="m-chip" onClick={() => void live.refresh()}>Không tải được · Thử lại</button>}</section>}
    {!tablet && <div className={`m-hscroll m-agents${stale ? ' mw-dim' : ''}`} role="list" aria-label="Trợ lý">{STRIP.map((role) => {
      const agent = snapshot?.agents?.find((item) => item.name.toLowerCase().includes(role.key))
      const working = !!agent && ['running', 'working', 'thinking', 'tool'].includes(agent.state)
      const active = running && !stale && (role.key === 'writer' || working)
      const task = agent?.tool ? toolLabel(agent.tool) : step.label
      const text = agentStatus(role.key as AgentRole, { state, stale, active, task, nextChapter: waitingChapter, queued: queued.length > 0 })
      return <div className={`m-agent${active ? ' active' : ''}`} role="listitem" aria-label={active ? `${role.label} · ${text} · đang chạy` : `${role.label} · ${text}`} key={role.key} style={{ '--rgb': AGENT_RGB[role.key], '--ag': 'var(--rgb)' } as CSSProperties}>
        <span className="mw-agent-orb" aria-hidden="true">
          {active && <span className="mw-agent-halo" />}
          <span className={`m-orb ${role.key} mw-agent-sphere`}>
            {active && <span className="mw-agent-highlight" />}
          </span>
        </span>
        <span className="mw-agent-copy"><strong title={role.label}>{role.label}</strong><small title={text} className={active ? 'mw-agent-action-live' : role.key === 'writer' ? (state === 'error' ? 'mw-st-error' : 'mw-st-writer') : undefined}>{text}</small></span>
      </div>
    })}</div>}
    {snapshot && pendingQuestion?.phase === 'wait' && <div className="mw-pad"><PendingQuestionCard model={pendingQuestion} snapshot={snapshot} mobile
      disabled={!writable || send.busy} onReply={(reply) => void send.run(reply)}
      onRead={(n) => navigate(paths.reader(project, story, n))} /></div>}
    {snapshot && !pendingQuestion && <section key={state} className={`m-card mw-hero mw-${state}${stale ? ' mw-dim' : ''}`} title={live.updatedAt ? `Cập nhật lúc ${formatShortClock(new Date(live.updatedAt).toISOString())}` : undefined}>
      {state === 'running' && <>
        <div className="mw-between"><span className="mw-label">Đang viết</span><span className="mw-spacer" />{stale && updatedClock && <span className="mw-updated">cập nhật lúc {updatedClock}</span>}<span className="num mw-percent">{percent}%</span></div>
        {tablet && <span className={`m-orb writer mw-large-orb${!stale ? ' live' : ''}`} aria-hidden="true" />}
        <div className="mw-titles"><h1>{tablet ? `Người viết · chương ${chapter}` : `Chương ${chapter}${title ? ` · ${title}` : ''}`}</h1><div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><p className="m-muted" style={{ flex: '1 1 auto', minWidth: 0 }}>{tablet ? `${step.label} · bước ${step.number}/5` : `Bước ${step.number}/5 · ${step.label}`}</p><button type="button" onClick={() => navigate(`${paths.workspace(project, story)}?view=run`)} style={{ flex: '0 0 auto', fontSize: 13.5, color: 'var(--color-accent)', whiteSpace: 'nowrap', border: 0, padding: 0, background: 'transparent' }}>Luồng viết ›</button></div></div>
        <div className="m-progress"><span style={{ width: `${percent}%` }} /></div>
        <div className={`m-stream${prose ? '' : ' mw-stream-empty'}`} aria-live="off">{prose ? <StreamMarkdown text={prose} /> : <>Người viết đang chuẩn bị những dòng tiếp theo… <ThinkingOrb size={22} label="Người viết đang viết" /></>}</div>
        <div className="mw-actions">{permitted && <button className="m-chip" disabled={!writable || pauseRequested || snapshot.runtimeState === 'pausing'} onClick={() => setSheet('pause')}><Icon name="pause" size={14} />{pauseRequested || snapshot.runtimeState === 'pausing' ? 'Đã hẹn tạm dừng' : 'Tạm dừng'}</button>}<button type="button" className="m-chip" onClick={() => navigate(paths.reader(project, story, chapter || undefined))}><Icon name="book-1" size={14} />Xem bản nháp</button></div>
      </>}
      {state === 'gate' && <>
        {tablet && <><span className="m-orb writer mw-large-orb" aria-hidden="true" /><div className="mw-between"><span>Người viết · chương {chapter}</span><span className="num">100%</span></div><div className="m-progress"><span style={{ width: '100%' }} /></div></>}
        <div className="mw-card-heading"><span className="mw-hero-icon"><Icon name="book-1" size={20} /></span><span className="mw-ct"><h1>Chương {chapter} chờ bạn duyệt</h1>{tablet ? <small className="mw-gate-sub">Đã tạm dừng theo chế độ duyệt từng chương</small> : <small>{excerpt?.words ? `${excerpt.words.toLocaleString('vi')} chữ` : 'Đọc bản nháp trước khi viết tiếp'}{score ? ` · Biên tập chấm ${score}` : ''}</small>}</span></div>
        {excerpt?.text.trim() && <p className="mw-excerpt">“{excerpt.text.slice(0, 180)}{excerpt.text.length > 180 ? '…' : ''}”</p>}
        {excerptResource.initialLoading && <p className="m-muted" role="status">Đang tải bản nháp…</p>}
        {(excerptResource.refreshing || reviews.refreshing) && <span className="m-muted" role="status">Đang cập nhật bản nháp…</span>}
        {(excerptResource.error != null || reviews.error != null) && <div className="m-error" role="alert">Không tải được {excerptResource.error ? 'bản nháp' : 'điểm duyệt'}.<button className="m-chip" onClick={() => { void excerptResource.refresh(); void reviews.refresh() }}>Thử lại</button></div>}
        {tablet ? <p className="m-muted">Chương {chapter} xong — đang mở bên phải để bạn đọc.{permitted ? ` Duyệt thì Người viết làm tiếp chương ${chapter + 1}.` : ''}</p> : <div className="m-inline-actions" style={permitted ? undefined : { gridTemplateColumns: '1fr' }}><button type="button" className="m-button secondary" onClick={() => navigate(paths.reader(project, story, chapter))}>Đọc chương {chapter}</button>{permitted && <button className="m-button" disabled={!writable || next.busy} aria-busy={next.pending} onClick={() => void next.run()}>{next.pending ? 'Đang duyệt…' : next.success ? 'Đã duyệt ✓' : 'Duyệt & viết tiếp'}</button>}</div>}
      </>}
      {state === 'idle' && <><div className="mw-card-heading"><span className="mw-hero-icon"><Icon name="clock" size={20} /></span><span className="mw-ct"><h1>Trợ lý đang chờ bạn</h1><small>{snapshot.completedCount ? `Chương ${snapshot.completedCount} đã duyệt${recentReview ? ` lúc ${formatShortClock(recentReview.time)}` : ''}` : 'Sẵn sàng viết chương đầu tiên'}</small></span></div>{permitted && <div className="m-inline-actions"><button className="m-button secondary" disabled={!writable} onClick={() => navigate(resultHref(project, story, '/cocreate'))}>Bàn hướng đi</button><button className="m-button" disabled={!writable || begin.busy} aria-busy={begin.pending} onClick={() => void begin.run()}>{begin.pending ? 'Đang bắt đầu…' : `Viết chương ${snapshot.completedCount + 1}`}</button></div>}</>}
      {state === 'paused' && <><div className="mw-card-heading"><span className="m-orb writer" aria-hidden="true" /><span className="mw-ct"><h1>Đã tạm dừng</h1><small>Chương {chapter} · dừng sau bước {step.label}</small></span></div><div className="m-progress"><span style={{ width: `${percent}%`, background: '#5E5E68' }} /></div>{permitted && <div className="m-inline-actions"><button className="m-button secondary" onClick={focusNote}>Góp ý trước</button><button className="m-button" disabled={!writable || resume.busy} aria-busy={resume.pending} onClick={() => void resume.run()}>{resume.pending ? 'Đang tiếp tục…' : <><Icon name="play" size={16} />Chạy tiếp</>}</button></div>}</>}
      {state === 'done' && <><div className="mw-done-orbs" aria-hidden="true">{ROLES.map((role) => <span className={`m-orb ${role.key}`} key={role.key} />)}</div><h1>Truyện đã hoàn thành</h1><p className="m-muted num">{snapshot.completedCount.toLocaleString('vi')} chương · {snapshot.totalWordCount.toLocaleString('vi')} chữ{events.length > 0 && ` · xong lúc ${formatShortClock(events[events.length - 1].time)}`}</p><div className="m-inline-actions">{actions.export && <button className="m-button" disabled={!exportable} onClick={() => openCommand(`/export exports/${story}.epub`)}><Icon name="export-square" size={16} />Xuất EPUB</button>}<button type="button" className="m-button secondary" onClick={() => navigate(paths.audio(project, story, undefined, 'listen'))}><Icon name="headphone" size={16} />Nghe</button></div>{permitted && <button className="mw-text-link" disabled={!writable} onClick={() => navigate(resultHref(project, story, '/reopen'))}>Viết tiếp tập mới ›</button>}</>}
      {state === 'error' && <><div className="mw-card-heading"><span className="mw-hero-icon"><Icon name="warning-2" size={20} /></span><span className="mw-ct"><h1>Người viết dừng lại</h1><small>Chương {chapter} · bước {step.number}/5</small></span></div><p>{latestError ? translateHost(latestError.summary).replace(/[.。]?\s*$/, '.') : 'Trợ lý gặp lỗi khi viết. Bạn có thể đổi model hoặc thử lại.'} Phần đã viết được giữ nguyên.</p>{permitted && <div className="m-inline-actions"><button type="button" className="m-button secondary" onClick={() => navigate(moreHref('model'))}>Đổi model</button><button className="m-button" disabled={!writable || retry.busy} aria-busy={retry.pending} onClick={() => void retry.run()}>{retry.pending ? 'Đang thử lại…' : <><Icon name="refresh-2" size={16} />Thử lại</>}</button></div>}</>}
    </section>}
    {tablet && <div className="mw-compact-agents">{ROLES.filter((role) => role.key !== 'writer').map((role) => { const agent = snapshot?.agents?.find((item) => item.name.toLowerCase().includes(role.key)); return <div key={role.key} className="m-agent"><span className={`m-orb ${role.key}`} /><span><strong>{role.key === 'editor' ? 'Biên tập viên' : role.label}</strong><small>{tabletAgentStatus(role.key, state, score, stale, agent?.state)}</small></span></div> })}</div>}
    {queued.map((item) => <div className="mw-queued" role="status" key={item.id}><Icon name="clock" size={17} /><span><small>Chờ gửi · sẽ tự gửi khi có mạng</small><p>“{item.text}”</p></span></div>)}
    <div className={`m-section-label mw-recent-heading${stale ? ' mw-dim' : ''}`}><span>{tablet ? 'Vừa xảy ra' : 'Vừa xong'}</span><button type="button" className="mw-recent-link" onClick={() => navigate(logHref(project, story))}>{tablet ? 'Nhật ký' : 'Xem tất cả'}</button></div>
    <div className={`m-group${stale ? ' mw-dim' : ''}`}>{events.slice(-5).reverse().map((event) => {
      const fromUser = isUserEvent(event)
      return <div className="m-row mw-event" key={event.id}>{fromUser ? <Icon name="edit-2" size={16} style={{ color: 'var(--color-user)' }} /> : <span className={`m-orb ${event.agent}`} aria-hidden="true" />}<span className="m-row-copy"><span>{activityText(event, total)}</span><small>{fromUser ? userEventActor(event, user?.username) : event.failed ? 'Chưa hoàn tất' : event.finishedAt ? 'Đã hoàn tất' : stale ? 'Chưa cập nhật' : 'Đang thực hiện'}</small></span><time className="m-muted">{formatShortClock(event.time)}</time></div>
    })}{!events.length && <p className="m-muted mw-pad">Các hoạt động của trợ lý sẽ hiện ở đây.</p>}</div>
  </>
  const composer = <WorkspaceComposer project={project} story={story} tablet={tablet} inputRef={inputRef} keyboard={keyboard} down={down} actions={actions} permitted={permitted} writable={writable} exportable={exportable} stale={stale} running={running} pauseRequested={pauseRequested} snapshot={snapshot} send={send} command={command} openCommand={openCommand} onPause={() => setSheet('pause')} onSubmit={submit} onCommand={(line) => { void command.run(line) }} />
  return <><section className={`m-page mw-workspace${tablet ? ' mw-tablet' : ''}${stale ? ' mw-offline' : ''}`}>
    {!tablet && <header className="m-topbar"><button type="button" className="mw-story-link" onClick={() => navigate(moreHref('projects'))}><span className="mw-brand-orbs" aria-hidden="true"><span className="m-orb architect" /><span className="m-orb writer" /><span className="m-orb editor" /></span><span className="mw-story-copy"><strong>{storyTitle}</strong><small>{state === 'done' ? `Hoàn thành · ${snapshot?.completedCount}/${snapshot?.totalChapters} chương` : `${volumeLabel} · ${snapshot?.completedCount ?? '…'}/${snapshot?.totalChapters ?? '…'} chương`}</small></span><Icon name="arrow-down-2" size={15} /></button>{state !== 'done' && background.length > 0 && <button className="mw-job-button" aria-label={`Việc chạy nền: ${background.length} việc`} onClick={() => setSheet('jobs')}><ProgressRing className="mw-job-ring" size={28} stroke={2.5} percent={progress} color={ringJob ? ringColor(ringJob) : undefined} track="var(--line-4)" indeterminate={ringJob && isIndeterminate(ringJob) && ringJob.status === 'running'}><Icon name={ringJob ? kindIcon(ringJob) : 'export-square'} size={13} /></ProgressRing><span className="num">{background.length}</span></button>}<button type="button" className="m-icon-button" onClick={() => navigate(`${paths.story(project, story)}?tab=info`)} aria-label="Truyện"><Icon name="document-text" size={21} /></button><button type="button" className="m-icon-button mw-bell" onClick={() => navigate('/notifications')} aria-label="Thông báo"><Icon name="notification" size={21} />{!!inbox.data?.unread && <i aria-label="Có thông báo chưa đọc" />}</button></header>}
    {!tablet && offline && <div className="mw-netbar" role="status"><span className="mw-net-spinner" aria-hidden="true" /><span className="mw-net-copy"><span>Mất kết nối</span><span className="num">{retryText(seconds)}</span></span><button type="button" onClick={retryNow}>Thử ngay</button></div>}
    <main className="m-scroll mw-scroll" ref={scrollRef}>{content}</main>{composer}
  </section>
  <CommandSheet open={sheet === 'cmd'} onClose={() => setSheet(null)} initialLine={initialCommand} story={story} snapshot={snapshot} writable={writable || exportable} readOnly={!actions.write} onRun={runCommand} />
  <JobsSheet open={sheet === 'jobs'} onClose={() => setSheet(null)} logHref={logHref(project, story)} />
  <PauseSheet open={sheet === 'pause'} onClose={() => setSheet(null)} chapter={chapter} step={step.label} onPause={async (afterStep) => { if (!await pause.run(afterStep)) throw new Error('Không tạm dừng được') }} />
  <ActionSheet open={!!conflict} onClose={() => setConflict(null)} title="Tệp xuất đã tồn tại" description={conflict ? `${conflict.exists.path} đã có trên máy chủ. Bạn muốn thay bản cũ hay giữ cả hai?` : ''} actions={(['replace', 'keep'] as const).map((mode) => ({ label: mode === 'replace' ? 'Thay bản cũ' : 'Giữ cả hai', destructive: mode === 'replace', onSelect: async () => { if (conflict) await runCommand(withExportFlag(conflict.line, mode)) } }))} />
  </>
}
