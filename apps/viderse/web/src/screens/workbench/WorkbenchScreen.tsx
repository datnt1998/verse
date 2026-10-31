import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { getWorkbench, getWorkbenchJob, getJobImages, jobAction, getRecap, markRecapSeen } from '../../api/workbench'
import { getNotificationSettings } from '../../api/activity'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { useResource } from '../../data/useResource'
import { subscribeEvents } from '../../data/events'
import { navigate, routePath, type Route } from '../../router'
import { EmptyWorkbench, SystemLoading, SystemNotFound } from '../../components'
import { followAdmitted, groupJobs, scheduleScopeRestore } from './workbench-logic'
import { WorkbenchPanels } from './panels/WorkbenchPanels'
import { PromptMenu } from './PromptMenu'
import { LiveStudio } from './LiveStudio'
import { OverflowMenu } from './OverflowMenu'
import { Recap, shouldAutoOpenRecap } from './recap/Recap'

const SCOPE_KEY = 'viderse:workbench-scope'
const label: Record<string, string> = { attention: 'Cần xử lý', review: 'Chờ duyệt', paused: 'Tạm dừng', working: 'Đang làm', queued: 'Chờ lượt', ready: 'Sẵn sàng đăng', posted: 'Đã đăng' }

export function WorkbenchScreen({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  const scope = route.channel || 'all'
  const [recapOpen, setRecapOpen] = useState(false)
  const [draft, setDraft] = useState('')
  const [notice, setNotice] = useState('')
  // Menu mở gắn với video đang chọn: đổi video thì menu cũ tự đóng.
  const [openMenu, setOpenMenu] = useState<{ kind: 'prompt' | 'more'; id: string } | null>(null)
  const list = useResource(['workbench', 'list', scope], ({ signal }) => getWorkbench(scope, signal), { keepPrevious: true })
  const channels = useResource(['workbench', 'channels'], ({ signal }) => getChannelSummaries(signal))
  const selectedId = route.video || list.data?.defaultJobId || undefined
  const promptOpen = openMenu?.kind === 'prompt' && openMenu.id === selectedId
  const more = openMenu?.kind === 'more' && openMenu.id === selectedId
  const setPromptOpen = (open: boolean) => setOpenMenu(open && selectedId ? { kind: 'prompt', id: selectedId } : null)
  const setMore = (open: boolean) => setOpenMenu(open && selectedId ? { kind: 'more', id: selectedId } : null)
  const queuedJob = list.data?.jobs?.find(item => item.id === selectedId && item.stageGroup === 'queued')
  const images = useResource(['workbench', 'images', selectedId || ''], ({ signal }) => getJobImages(selectedId!, signal), { enabled: !!selectedId && !queuedJob })
  const detail = useResource(['workbench', 'job', selectedId || ''], ({ signal }) => getWorkbenchJob(selectedId!, signal), { enabled: !!selectedId && !queuedJob })
  const recap = useResource(['workbench', 'recap'], ({ signal }) => getRecap(signal))
  const recapSettings = useResource(['activity', 'settings'], ({ signal }) => getNotificationSettings(signal))
  const autoRecapChecked = useRef(false)
  const job = queuedJob || detail.data
  const followed = useRef<{ id: string; sourceKey: string } | null>(null)
  useEffect(() => {
    if (queuedJob) { followed.current = { id: queuedJob.id, sourceKey: queuedJob.sourceKey }; return }
    if (followed.current?.id !== selectedId) { followed.current = null; return }
    const next = followAdmitted(list.data?.jobs ?? [], followed.current)
    if (next) navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: next }), true)
  }, [queuedJob, selectedId, list.data, route.channel, route.scope])
  useEffect(() => {
    if (!recap.data || !recapSettings.data || autoRecapChecked.current) return
    autoRecapChecked.current = true
    if (shouldAutoOpenRecap(recap.data.from, recap.data.to, recap.data.hasNew, recapSettings.data.recapOnReturn)) setRecapOpen(true)
  }, [recap.data, recapSettings.data])
  useEffect(() => {
    const refresh = () => { void list.refresh(); void channels.refresh(); if (selectedId) void detail.refresh() }
    const unsubscribe = subscribeEvents(['clone.job.created', 'clone.job.updated', 'clone.job.deleted', 'clone.step.progress', 'queue.updated', 'channels'], refresh)
    return unsubscribe
  }, [list.refresh, channels.refresh, detail.refresh, selectedId])
  useEffect(() => {
    if (route.channel || route.scope === 'all') {
      try { localStorage.setItem(SCOPE_KEY, route.channel || 'all') } catch { /* storage unavailable */ }
    } else if (!route.video && location.hash === '#/') {
      try {
        const saved = localStorage.getItem(SCOPE_KEY)
        if (saved && saved !== 'all') scheduleScopeRestore(saved, () => location.hash, channel => navigate(routePath({ name: 'workbench', channel }), true))
      } catch { /* storage unavailable */ }
    }
  }, [route.channel, route.scope, route.video])
  const pickScope = (channel?: string) => {
    try { localStorage.setItem(SCOPE_KEY, channel || 'all') } catch { /* storage unavailable */ }
    navigate(routePath({ name: 'workbench', channel, video: selectedId }))
  }
  const refresh = () => { void list.refresh(); void detail.refresh(); void channels.refresh() }
  const closeRecap = () => {
    autoRecapChecked.current = true
    setRecapOpen(false)
    void markRecapSeen().then(() => recap.refresh()).catch(() => {})
  }
  const action = async (kind: 'pause' | 'resume' | 'cancel' | 'run') => {
    if (!job) return
    try { await jobAction(job.id, kind); refresh() } catch (err) { setNotice(err instanceof Error ? err.message : 'Không thực hiện được') }
  }
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !job || /INPUT|TEXTAREA|SELECT/.test((event.target as HTMLElement).tagName)) return
      if (promptOpen || more) { setPromptOpen(false); setMore(false); return }
      if (job.stageGroup === 'working') void action('pause')
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  })
  if (list.initialLoading) return <SystemLoading />
  if (list.error && !list.data) return <div {...stylex.props(styles.error)} role="alert">Không tải được Bàn dựng. <button onClick={() => void list.refresh()}>Thử lại</button></div>
  const jobs = list.data?.jobs || []
  const groups = groupJobs(jobs, scope === 'all')
  const channelRows = channels.data || []
  return <div {...stylex.props(styles.root)}>
    <aside aria-label="Video" {...stylex.props(styles.side)}>
      <div {...stylex.props(styles.sideTitle)}><span>Video</span><span>{jobs.length}</span></div>
      <button type="button" {...stylex.props(styles.button)} onClick={() => { autoRecapChecked.current = true; setRecapOpen(true); void recap.refresh() }}>Bản tin</button>
      {scope === 'all' && channelRows.length > 0 && <section><h2 {...stylex.props(styles.group)}>Kênh <span>{channelRows.length}</span></h2>{channelRows.map(c => <button key={c.channelId} type="button" {...stylex.props(styles.row)} onClick={() => pickScope(c.channelId)}><span {...stylex.props(styles.avatar)}>{c.initials}</span><span {...stylex.props(styles.rowText)}><strong title={c.name}>{c.name}</strong><small>{c.today.done}/{c.today.cap} · {c.waitingYou} chờ bạn · {c.ready} sẵn sàng đăng</small>{c.templateIssue && <small>{c.templateIssue.text}</small>}</span></button>)}</section>}
      {groups.map(group => <section key={group.key}><h2 {...stylex.props(styles.group)}>{group.label}<span>{group.key === 'queued' ? list.data?.queuedTotal : group.count}</span></h2>{group.items.map(item => <button key={item.id} type="button" {...stylex.props(styles.row, item.id === selectedId && styles.selected)} onClick={() => { if (recapOpen) closeRecap(); navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: item.id })) }}>{scope === 'all' && <span {...stylex.props(styles.avatar)}>{channelRows.find(c => c.channelId === item.channelId)?.initials}</span>}<span {...stylex.props(styles.rowText)}><strong title={item.source.title}>{item.source.title}</strong><small title={item.subline}>{item.subline}</small></span></button>)}{group.key === 'queued' && (list.data?.queuedTotal || 0) > group.items.length && <a href="#/queue" {...stylex.props(styles.link)}>Xem cả {list.data?.queuedTotal} trong Hàng chờ →</a>}{group.key === 'posted' && <a href={`#/channels/${encodeURIComponent(scope)}`} {...stylex.props(styles.link)}>Xem trang kênh →</a>}</section>)}
      <div {...stylex.props(styles.footer)}>{scope === 'all' ? <>Tự lấy video: bật ở {channelRows.filter(c => c.autoFetch).length}/{channelRows.length} kênh<br />Hàng chờ chung: {list.data?.queuedTotal || 0} video</> : <button type="button" {...stylex.props(styles.button)} onClick={() => pickScope()}>Mọi kênh</button>}</div>
    </aside>
    <main {...stylex.props(styles.center)}>
      {recapOpen ? <Recap data={recap.data} loading={recap.initialLoading} error={recap.error} waitingCount={groups.find(group => group.key === 'waiting')?.count || 0} readyCount={groups.find(group => group.key === 'ready')?.count || 0} channelNames={Object.fromEntries(channelRows.map(c => [c.channelId, c.name]))} onRetry={() => void recap.refresh()} onEnter={closeRecap} onSelectJob={id => { closeRecap(); navigate(routePath({ name: 'workbench', channel: route.channel, video: id })) }} /> : !selectedId ? <EmptyWorkbench /> : detail.initialLoading ? <SystemLoading /> : !job ? <SystemNotFound /> : <>
        <header {...stylex.props(styles.header, job.stageGroup === 'review' && styles.headerNotice, job.stageGroup === 'attention' && job.activeStep === 'score' && styles.headerNotice, (job.stageGroup === 'paused' || (job.stageGroup === 'attention' && job.activeStep !== 'score')) && styles.headerTall, promptOpen && styles.headerOpen, promptOpen && (job.stageGroup === 'review' || (job.stageGroup === 'attention' && job.activeStep === 'score')) && styles.headerNoticeOpen, promptOpen && (job.stageGroup === 'paused' || (job.stageGroup === 'attention' && job.activeStep !== 'score')) && styles.headerTallOpen)}><div {...stylex.props(styles.heading)}><h1 title={job.source.title} {...stylex.props(styles.title)}>{job.source.title}</h1>{job.stageGroup === 'working' && <button {...stylex.props(styles.button)} onClick={() => void action('pause')}>Tạm dừng <kbd>Esc</kbd></button>}{job.stageGroup === 'paused' && <button {...stylex.props(styles.button)} onClick={() => void action('resume')}>Chạy tiếp</button>}{!queuedJob && <div {...stylex.props(styles.relative)}><button {...stylex.props(styles.button)} aria-label="Thêm thao tác cho video" aria-expanded={more} onClick={() => setMore(!more)}>⋯</button>{more && <OverflowMenu job={job} onClose={() => setMore(false)} onRefresh={refresh} onError={setNotice} />}</div>}</div>
          <div {...stylex.props(styles.chips)}><span {...stylex.props(styles.chip)}>{label[job.stageGroup] || job.stageGroup}</span><span {...stylex.props(styles.chip)}>Nguồn <b>{job.source.channelTitle}</b></span><span {...stylex.props(styles.chip)}>Template <b>{job.templateId}</b></span><div><button {...stylex.props(styles.chip)} onClick={() => setPromptOpen(!promptOpen)} aria-expanded={promptOpen}>Ảnh <b>{images.data?.manifest?.scenePrompt.name}</b> {images.data?.manifest?.scenePrompt.overridden && 'riêng video này'} ▾</button>{promptOpen && <PromptMenu job={job} onClose={() => setPromptOpen(false)} onRefresh={refresh} />}</div></div>
          <div {...stylex.props(styles.headerSub)}>{job.subline}</div>
          {job.stageGroup === 'review' && <div {...stylex.props(styles.banner)}>Ảnh đã sẵn sàng để bạn duyệt. <a href={`#/review?video=${encodeURIComponent(job.id)}`}>Duyệt ảnh</a></div>}{job.stageGroup === 'paused' && <div {...stylex.props(styles.banner)}>Video đang tạm dừng. Bạn có thể góp ý trước khi chạy tiếp.</div>}{job.stageGroup === 'attention' && <div {...stylex.props(styles.banner)}>Video cần bạn xử lý. {job.subline}</div>}
        </header>
        <section {...stylex.props(styles.card, styles.log, ['review', 'attention'].includes(job.stageGroup) && styles.logBanner)} aria-label="Nhật ký tiến độ"><h2 {...stylex.props(styles.h2)}>Nhật ký tiến độ</h2>{job.steps?.map(step => <div key={step.step} {...stylex.props(styles.logRow)}><span>{step.startedAt ? new Date(step.startedAt).toLocaleTimeString('vi-VN') : ''}</span><span>{step.progress.phase || step.step}</span><span>{step.error?.message}</span></div>)}</section>
        {queuedJob ? <section {...stylex.props(styles.card, styles.recap)}>Video đang chờ lượt. <a href="#/queue">Mở Hàng chờ</a></section> : <LiveStudio job={job} onChangePrompt={() => setPromptOpen(true)} onRefresh={refresh} />}
        <form {...stylex.props(styles.composer)} onSubmit={event => { event.preventDefault(); setNotice('Sắp có') }}><input aria-label="Nhắn trợ lý" placeholder="Nhắn trợ lý…" value={draft} onChange={event => setDraft(event.target.value)} {...stylex.props(styles.input)} /><button {...stylex.props(styles.button)} type="submit">Gửi</button></form>{notice && <div role="status">{notice}</div>}
      </>}
    </main>
    {job && !queuedJob && !recapOpen && <aside {...stylex.props(styles.panels)}><WorkbenchPanels job={job} onRefresh={refresh} /></aside>}
  </div>
}

const styles = stylex.create({
  root: { display: 'flex', flex: '1 1 auto', minHeight: 0, minWidth: 0 },
  side: { flex: '0 0 248px', width: 248, minWidth: 0, overflowY: 'auto', padding: `${s['--vv-s-14']} ${s['--vv-s-10']}`, boxShadow: 'inset -1px 0 0 var(--line-1)', display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  sideTitle: { display: 'flex', justifyContent: 'space-between', padding: `0 ${s['--vv-s-8']}`, color: 'var(--color-fg-3)', fontSize: 13 },
  group: { display: 'flex', justifyContent: 'space-between', padding: `${s['--vv-s-10']} ${s['--vv-s-8']} ${s['--vv-s-4']}`, margin: 0, color: 'var(--color-fg-4)', fontSize: 11, textTransform: 'uppercase' },
  row: { width: '100%', display: 'flex', alignItems: 'center', gap: s['--vv-s-9'], padding: `${s['--vv-s-7']} ${s['--vv-s-8']}`, border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg)', textAlign: 'left', cursor: 'pointer' },
  selected: { backgroundColor: 'var(--color-surface-3)' },
  rowText: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], overflow: 'hidden', fontSize: 13 },
  avatar: { flexShrink: 0, width: 26, height: 26, display: 'grid', placeItems: 'center', borderRadius: '50%', backgroundColor: 'var(--color-surface-3)', fontSize: 10 },
  link: { display: 'block', padding: `${s['--vv-s-5']} ${s['--vv-s-8']}`, color: 'var(--color-fg-3)', fontSize: 12 },
  footer: { marginTop: 'auto', padding: `${s['--vv-s-10']} ${s['--vv-s-8']} 0`, borderTop: '1px solid var(--line-1)', color: 'var(--color-fg-4)', fontSize: 12, lineHeight: 1.8 },
  center: { position: 'relative', flex: '1 1 520px', minWidth: 0, minHeight: 0, overflow: 'hidden', padding: `${s['--vv-s-12']} ${s['--vv-s-18']} ${s['--vv-s-10']}`, display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'] },
  panels: { flex: '0 0 340px', width: 340, minHeight: 0, overflowY: 'auto', boxShadow: 'inset 1px 0 0 var(--line-1)' },
  header: { flex: '0 0 98px' },
  headerNotice: { flexBasis: 148 },
  headerTall: { flexBasis: 168 },
  headerOpen: { flexBasis: 106 },
  headerNoticeOpen: { flexBasis: 156 },
  headerTallOpen: { flexBasis: 176 },
  heading: { display: 'flex', gap: s['--vv-s-6'], alignItems: 'center' },
  title: { flex: 1, minWidth: 0, margin: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 17 },
  headerSub: { marginTop: s['--vv-s-6'], color: 'var(--color-fg-3)', fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  chips: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'], marginTop: s['--vv-s-8'] },
  chip: { border: '1px solid var(--line-1)', borderRadius: r['--vv-r-7'], backgroundColor: 'var(--color-surface-1)', color: 'var(--color-fg-3)', fontSize: 12, padding: `${s['--vv-s-5']} ${s['--vv-s-8']}` },
  button: { border: '1px solid var(--line-2)', borderRadius: r['--vv-r-8'], backgroundColor: 'var(--color-surface-2)', color: 'var(--color-fg)', padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, cursor: 'pointer', fontSize: 13 },
  banner: { marginTop: s['--vv-s-8'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, border: '1px solid var(--line-2)', borderRadius: r['--vv-r-12'], color: 'var(--color-fg-2)', fontSize: 13, display: 'flex', justifyContent: 'space-between' },
  card: { border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], backgroundColor: 'var(--color-surface-1)', padding: `${s['--vv-s-10']} ${s['--vv-s-14']}` },
  log: { flex: '0 0 168px', minHeight: 0, overflowY: 'auto' },
  logBanner: { flexBasis: 96 },
  h2: { margin: `0 0 ${s['--vv-s-8']}`, color: 'var(--color-fg-3)', fontSize: 13, fontWeight: 500 },
  logRow: { display: 'flex', gap: s['--vv-s-10'], padding: `${s['--vv-s-3']} 0`, color: 'var(--color-fg-3)', fontSize: 13 },
  composer: { boxSizing: 'border-box', flex: '0 0 81px', display: 'flex', gap: s['--vv-s-8'], padding: s['--vv-s-8'], borderRadius: r['--vv-r-14'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-2)' },
  input: { flex: 1, minWidth: 0, border: 0, outline: 'none', backgroundColor: 'transparent', color: 'var(--color-fg)', fontSize: 13 },
  relative: { position: 'relative' },
  recap: { flex: 1, overflowY: 'auto' },
  error: { padding: s['--vv-s-24'], color: 'var(--color-fg)' },
})
