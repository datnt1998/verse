import { Suspense, lazy, useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, type IconName } from '@noverse/ui'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { acceptExport, getWorkbench, getWorkbenchJob, getJobImages, getScoreArtifact, jobAction, type CloneJobView } from '../../api/workbench'
import { getJobImages as getFullJobImages, redrawImages } from '../../api/images'
import { BUNDLE_MAX, bundleUrl } from '../../api/channelVideos'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { mutate } from '../../api/request'
import { useResource } from '../../data/useResource'
import { useOnline } from '../../data/status'
import { subscribeEvents } from '../../data/events'
import { navigate, routePath, type Route } from '../../router'
import { ChannelAvatar, EmptyWorkbench, Orb, SystemLoading, SystemNotFound } from '../../components'
import { ActionButton } from '../../components/ActionButton'
import { SuggestionRow } from '../../components/suggestions/SuggestionRow'
import { useSuggestions } from '../../components/suggestions/useSuggestions'
import { clockLabel, followAdmitted, groupJobs, issueReason, jobStage, scheduleScopeRestore, stepPercent, QUALITY_MIN_SCORE, type JobStage } from './workbench-logic'
import { LiveStudio } from './LiveStudio'
import { usePanelKeys } from './usePanelKeys'
import { requestDeleteExport, requestMarkPosted } from '../../components/performance'
import { editorProjectId } from './workbench-actions'

// Menu ⋯ chỉ mở khi bấm: chunk riêng, không nằm trong lần tải đầu của Bàn dựng.
const OverflowMenu = lazy(() => import('./OverflowMenu').then((m) => ({ default: m.OverflowMenu })))
// Chỉ máy tính mới dùng: tách khỏi lần tải đầu để điện thoại không kéo 27KB gzip.
const WorkbenchPanels = lazy(() => import('./panels/WorkbenchPanels').then((m) => ({ default: m.WorkbenchPanels })))

const SCOPE_KEY = 'viderse:workbench-scope'
const label: Record<string, string> = { attention: 'Cần xử lý', review: 'Chờ duyệt', paused: 'Tạm dừng', working: 'Đang làm', queued: 'Chờ lượt', ready: 'Sẵn sàng đăng', posted: 'Đã đăng' }

export function WorkbenchDesktop({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  const scope = route.channel || 'all'
  const [notice, setNotice] = useState('')
  const [redrawAll, setRedrawAll] = useState(false)
  const [approveBusy, setApproveBusy] = useState(false)
  const [grokBusy, setGrokBusy] = useState(false)
  const [grokError, setGrokError] = useState('')
  // Chọn nhiều: chỉ video Sẵn sàng đăng mới vào gói; dòng đề xuất không chọn được.
  const [selectMode, setSelectMode] = useState(false)
  const [picked, setPicked] = useState<Record<string, true>>({})
  const list = useResource(['workbench', 'list', scope], ({ signal }) => getWorkbench(scope, signal), { keepPrevious: true })
  // Cùng khoá với khung app (AppShell) và TodayRow: một endpoint thì một khoá,
  // nếu không màn đầu gọi /api/channels/summary hai lần cho cùng dữ liệu.
  const channels = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const selectedId = route.video || list.data?.defaultJobId || undefined
  const queuedJob = list.data?.jobs?.find(item => item.id === selectedId && item.stageGroup === 'queued')
  const images = useResource(['workbench', 'images', selectedId || ''], ({ signal }) => getJobImages(selectedId!, signal), { enabled: !!selectedId && !queuedJob })
  const detail = useResource(['workbench', 'job', selectedId || ''], ({ signal }) => getWorkbenchJob(selectedId!, signal), { enabled: !!selectedId && !queuedJob })
  const suggestions = useSuggestions(scope)
  const suggestionItems = suggestions.list.data?.suggestions.slice(0, 3) ?? []
  const { leftOpen, rightOpen } = usePanelKeys()
  const online = useOnline()
  const job = queuedJob || detail.data
  const scoreArtifact = job?.steps?.find(step => step.step === 'score')?.output?.artifacts?.find(item => item.path.endsWith('score.json'))
  // Cùng khoá với LiveStudio/panel: mở trang đã có điểm thì không gọi thêm.
  const score = useResource(['workbench', 'score', selectedId || '', scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(selectedId!, scoreArtifact!.id, signal), { enabled: !!scoreArtifact && !!selectedId })
  const followed = useRef<{ id: string; sourceKey: string } | null>(null)
  useEffect(() => {
    if (queuedJob) { followed.current = { id: queuedJob.id, sourceKey: queuedJob.sourceKey }; return }
    if (followed.current?.id !== selectedId) { followed.current = null; return }
    const next = followAdmitted(list.data?.jobs ?? [], followed.current)
    if (next) navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: next }), true)
  }, [queuedJob, selectedId, list.data, route.channel, route.scope])
  useEffect(() => {
    const refresh = () => { void list.refresh(); void channels.refresh(); if (selectedId) void detail.refresh() }
    const unsubscribe = subscribeEvents(['clone.job.created', 'clone.job.updated', 'clone.job.deleted', 'clone.step.progress', 'queue.updated', 'channels'], refresh)
    return unsubscribe
  }, [list.refresh, channels.refresh, detail.refresh, selectedId])
  useEffect(() => { setGrokError(''); setRedrawAll(false) }, [selectedId])
  // Phạm vi đổi thì lựa chọn cũ không còn thuộc danh sách đang xem.
  useEffect(() => { setPicked({}); setSelectMode(false) }, [scope])
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
    navigate(routePath({ name: 'workbench', channel }))
  }
  const refresh = () => { void list.refresh(); void detail.refresh(); void channels.refresh() }
  const action = async (kind: 'pause' | 'resume' | 'cancel' | 'run') => {
    if (!job) return
    setNotice('')
    try { await jobAction(job.id, kind); refresh() } catch (err) { setNotice(err instanceof Error ? err.message : 'Không thực hiện được') }
  }
  /** Duyệt cả video & dựng: đúng endpoint owner-approval của màn Duyệt, chốt revision thật. */
  const approveAll = async () => {
    if (!job || approveBusy) return
    const view = images.data
    if (!view?.manifest || !view.items?.length) { setNotice('Chưa tải được bộ ảnh để duyệt.'); return }
    setApproveBusy(true); setNotice('')
    try {
      await mutate('POST', `/api/clone/${encodeURIComponent(job.id)}/images`, { op: 'approveAll', expectedRevision: view.manifest.revision })
      window.dispatchEvent(new Event('activity:refresh'))
      refresh()
    } catch (err) {
      setNotice(err instanceof Error ? err.message : 'Chưa duyệt được video.')
    } finally { setApproveBusy(false) }
  }
  const accept = async () => {
    if (!job) return
    setNotice('')
    try { await acceptExport(job.id); refresh() } catch (err) { setNotice(err instanceof Error ? err.message : 'Không xuất được') }
  }
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !job || /INPUT|TEXTAREA|SELECT/.test((event.target as HTMLElement).tagName)) return
      if (selectMode) { setSelectMode(false); setPicked({}); return }
      if (job.stageGroup === 'working') void action('pause')
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  })
  if (list.initialLoading) return <SystemLoading />
  if (list.error && !list.data) return <div {...stylex.props(styles.error)} role="alert">Không tải được Bàn dựng. <Button variant="quiet" size="sm" onClick={() => void list.refresh()}>Thử lại</Button></div>
  const jobs = list.data?.jobs || []
  const groups = groupJobs(jobs, scope === 'all')
  const channelRows = channels.data || []
  const stage = job ? jobStage(job) : 'images'
  const items = images.data?.items || []
  const drawingIndex = items.findIndex(item => item.status === 'running' || item.status === 'pending' || item.status === 'queued')
  const drawing = drawingIndex >= 0
  const allDrawn = items.length > 0 && !drawing && !items.some(item => item.status === 'failed')
  const failedStep = job?.steps?.find(step => step.status === 'failed')
  const failedItem = items.find(item => item.status === 'failed')
  // Giữ prompt/refs của lần lỗi mới nhất qua bản nguồn, không gửi lại chúng
  // thành phần sửa tay hoặc dựng lại định danh ảnh tải lên.
  const retryWithGrok = async () => {
    if (!job || !failedItem || grokBusy) return
    setGrokBusy(true); setGrokError('')
    try {
      const view = await getFullJobImages(job.id)
      const image = view.images.find(candidate => candidate.id === failedItem.id)
      const failedVersion = image?.versions.findLast(version => version.state === 'failed')
      if (!image || !failedVersion) { setGrokError('Không đọc được bản lỗi của ảnh này.'); return }
      await redrawImages(job.id, {
        imageIds: [image.id],
        provider: 'grok',
        baseVersion: failedVersion.n,
      })
      refresh(); void images.refresh()
    } catch (err) {
      setGrokError(err instanceof Error ? err.message : 'Chưa vẽ tiếp được bằng Grok.')
    } finally { setGrokBusy(false) }
  }
  const dayDone = scope === 'all' ? channelRows.reduce((sum, row) => sum + row.today.done, 0) : channelRows.find(row => row.channelId === scope)?.today.done
  const dayCap = scope === 'all' ? channelRows.reduce((sum, row) => sum + row.today.cap, 0) : channelRows.find(row => row.channelId === scope)?.today.cap
  const jobChannel = job ? channelRows.find(row => row.channelId === job.channelId) : undefined
  const status = job ? statusLine(stage, job, drawingIndex) : { text: '', tone: 'active' as const, icon: 'clock' as const, spin: false }
  const pickedIds = Object.keys(picked)
  const readyPicked = pickedIds.filter(id => jobs.find(item => item.id === id)?.stageGroup === 'ready')
  const tooManyReady = readyPicked.length > BUNDLE_MAX
  const waitingGroup = groups.some(group => group.key === 'waiting')
  const renderSuggestions = () => suggestions.list.error && !suggestions.list.data
    ? <div role="alert" {...stylex.props(styles.suggestRowNotice)}>Không tải được đề xuất. <Button variant="quiet" size="sm" onClick={() => void suggestions.list.refresh()}>Thử lại</Button></div>
    : suggestions.list.initialLoading && !suggestions.list.data
      ? <div role="status" {...stylex.props(styles.suggestRowNotice)}>Đang tải đề xuất…</div>
      : suggestionItems.length === 0 ? null
      : <div role="list" aria-label="Đề xuất của trợ lý" {...stylex.props(styles.suggestions)}>
        {suggestionItems.map(item => <div role="listitem" key={item.id}><SuggestionRow suggestion={item} busy={suggestions.busy === item.id} offline={!suggestions.online}
          notice={suggestions.notice?.id === item.id ? suggestions.notice.text : undefined}
          onAccept={() => void suggestions.accept(item)} onDismiss={() => void suggestions.dismiss(item)} /></div>)}
      </div>
  return <div {...stylex.props(styles.root)}>
    {leftOpen && <aside aria-label={scope === 'all' ? 'Video mọi kênh' : 'Video của kênh'} {...stylex.props(styles.side)}>
      <div {...stylex.props(styles.sideTitle)}>
        <h2 {...stylex.props(styles.sideHeading)}><Icon name="video-play" size={16} />{scope === 'all' ? 'Video mọi kênh' : 'Video của kênh'}</h2>
        <Button variant="quiet" size="compact" aria-pressed={selectMode} title="Chọn nhiều video để tải gói" xstyle={styles.selectButton}
          onClick={() => { setSelectMode(!selectMode); setPicked({}) }}>{selectMode ? 'Xong' : 'Chọn'}</Button>
      </div>
      <div {...stylex.props(styles.progressRow)}><span {...stylex.props(styles.dayTrack)}><span {...stylex.props(styles.dayFill)} style={{ width: `${dayCap ? Math.min(100, Math.round((dayDone ?? 0) / dayCap * 100)) : 0}%` }} /></span><span {...stylex.props(styles.sideCount)} title={`Hôm nay bắt đầu ${dayDone ?? 0} trên tổng nhịp ${dayCap ?? 0} video`}>{dayDone ?? 0} / {dayCap ?? 0} hôm nay</span></div>
      {scope === 'all' && channelRows.length > 0 && <section {...stylex.props(styles.section)}><h2 {...stylex.props(styles.group)}>Kênh <span>{channelRows.length}</span></h2>{channelRows.map(c => <button key={c.channelId} type="button" {...stylex.props(styles.channelRow)} onClick={() => pickScope(c.channelId)}>
        <ChannelAvatar name={c.name} initials={c.initials} src={c.avatarUrl ?? undefined} color={{ bg: c.color, ring: c.color }} size={26} />
        <span {...stylex.props(styles.channelText)}>
          <span {...stylex.props(styles.channelName)}><strong title={c.name}>{c.name}</strong>{c.templateIssue && <Icon name="danger" size={13} xstyle={styles.warnIcon} label={c.templateIssue.text} />}<span {...stylex.props(styles.channelToday)}>{c.today.done}/{c.today.cap}</span></span>
          <span aria-hidden="true" {...stylex.props(styles.channelTrack)}><span {...stylex.props(styles.channelFill)} style={{ width: `${c.today.cap ? Math.min(100, Math.round(c.today.done / c.today.cap * 100)) : 0}%` }} /></span>
          <span {...stylex.props(styles.channelFacts)}><span {...stylex.props(c.waitingYou ? styles.waitFact : styles.note)}>{c.waitingYou} chờ bạn</span><span {...stylex.props(c.ready ? styles.readyFact : styles.note)}>{c.ready} sẵn sàng đăng</span></span>
        </span>
      </button>)}</section>}
      {!waitingGroup && (suggestionItems.length > 0 || suggestions.list.error || suggestions.list.initialLoading) && <section {...stylex.props(styles.section)}>
        <h2 {...stylex.props(styles.group)}>Chờ bạn<span>0</span></h2>
        {renderSuggestions()}
      </section>}
      {groups.map(group => <section key={group.key} {...stylex.props(styles.section)}>
        <h2 {...stylex.props(styles.group)}>{group.label}<span>{group.key === 'queued' ? list.data?.queuedTotal : group.count}</span></h2>
        <ol {...stylex.props(styles.rows)}>{group.items.map(item => {
          const itemStage = jobStage(item)
          const current = item.id === selectedId
          const checked = !!picked[item.id]
          return <li key={item.id}><button type="button" role={selectMode ? 'checkbox' : undefined} aria-current={current || undefined} aria-checked={selectMode ? checked : undefined} title={item.source.title} {...stylex.props(styles.row, current && styles.selected)}
            onClick={() => {
              if (selectMode) { setPicked(prev => { const next = { ...prev }; if (next[item.id]) delete next[item.id]; else next[item.id] = true; return next }); return }
              navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: item.id }))
            }}>
            {selectMode && <span aria-hidden="true" {...stylex.props(styles.check, checked && styles.checkOn)}>{checked && <Icon name="tick" size={11} />}</span>}
            {scope === 'all' && <ChannelAvatar name={channelRows.find(c => c.channelId === item.channelId)?.name ?? item.source.channelTitle ?? 'Kênh'} initials={channelRows.find(c => c.channelId === item.channelId)?.initials} src={channelRows.find(c => c.channelId === item.channelId)?.avatarUrl ?? undefined} color={channelRows.find(c => c.channelId === item.channelId) ? { bg: channelRows.find(c => c.channelId === item.channelId)!.color, ring: channelRows.find(c => c.channelId === item.channelId)!.color } : undefined} size={20} />}
            <span {...stylex.props(styles.rowText)}><span {...stylex.props(styles.rowTitle, current && styles.rowTitleCurrent)}>{item.source.title}</span><span {...stylex.props(styles.rowHint)}>{item.subline}</span></span>
            {itemStage === 'review' ? <Icon name="flag" size={15} xstyle={styles.reviewIcon} label={label[item.stageGroup]} />
              : itemStage === 'issue' || itemStage === 'quality' ? <Icon name="danger" size={15} xstyle={styles.warnIcon} label="Cần xử lý" />
                : itemStage === 'paused' ? <Icon name="pause" size={15} xstyle={styles.noteIcon} label="Tạm dừng" />
                  : itemStage === 'render' || itemStage === 'images' ? (current ? <Orb of={itemStage === 'render' ? 'qa' : 'artist'} size={12} live /> : <span {...stylex.props(styles.rowSpinner)} aria-label="Đang làm" />)
                    : itemStage === 'done' ? <Icon name="tick-b" size={15} xstyle={styles.okIcon} label={item.stageGroup === 'posted' ? 'Đã đăng' : 'Sẵn sàng'} />
                      : <Icon name="clock" size={15} xstyle={styles.noteIcon} label="Chờ lượt" />}
          </button></li>
        })}</ol>
        {group.key === 'waiting' && renderSuggestions()}
        {group.key === 'queued' && group.count > group.items.length && <a href="#/queue" {...stylex.props(styles.moreLink)}>Xem cả {group.count} trong Hàng chờ<Icon name="arrow-r" size={12} /></a>}
        {group.key === 'posted' && scope !== 'all' && <a href={routePath({ name: 'channel', channelId: scope })} {...stylex.props(styles.moreLink)}>Xem ở trang kênh<Icon name="arrow-r" size={12} /></a>}
      </section>)}
      {selectMode && <div role="region" aria-label="Video đã chọn" {...stylex.props(styles.selectBar)}>
        <span {...stylex.props(styles.selectText)}>{pickedIds.length ? `${pickedIds.length} video đã chọn${readyPicked.length ? ` · ${readyPicked.length} sẵn sàng đăng` : ' · chưa có video sẵn sàng đăng'}` : 'Chọn video Sẵn sàng đăng để tải gói một lần.'}</span>
        {tooManyReady && <span role="status" {...stylex.props(styles.selectText)}>Mỗi lần tải tối đa {BUNDLE_MAX} video sẵn sàng đăng. Bỏ chọn bớt để tiếp tục.</span>}
        <div {...stylex.props(styles.selectActions)}>
          {readyPicked.length > 0 && !tooManyReady
            ? <a href={bundleUrl(readyPicked)} download onClick={() => { setPicked({}); setSelectMode(false) }} {...stylex.props(styles.bundleLink)}><Icon name="download" size={14} />Tải gói {readyPicked.length} video</a>
            : <Button variant="primary" size="compact" disabled title={tooManyReady ? `Mỗi lần tải tối đa ${BUNDLE_MAX} video sẵn sàng đăng` : 'Chọn ít nhất một video Sẵn sàng đăng'} xstyle={styles.bundleDisabled}><Icon name="download" size={14} />Tải gói</Button>}
          <Button variant="ghost" size="compact" onClick={() => { setSelectMode(false); setPicked({}) }}>Xong</Button>
        </div>
      </div>}
      <div {...stylex.props(styles.footer)}>
        <div {...stylex.props(styles.footRow)}><span>Tự lấy video</span><span {...stylex.props(styles.footValue)}>{scope === 'all' ? `Bật ở ${channelRows.filter(c => c.autoFetch).length}/${channelRows.length} kênh` : channelRows.find(c => c.channelId === scope)?.autoFetch ? `Bật · ${channelRows.find(c => c.channelId === scope)?.today.cap ?? 0} mỗi ngày` : 'Chỉ đề xuất'}</span></div>
        {scope === 'all' && <div {...stylex.props(styles.footRow)}><span>Hàng chờ chung</span><span {...stylex.props(styles.footValue)}>{list.data?.queuedTotal || 0} video</span></div>}
      </div>
    </aside>}
    <main className={`vv-wide ${stylex.props(styles.center).className}`}>
      {!selectedId ? <EmptyWorkbench /> : detail.initialLoading ? <SystemLoading /> : !job ? <SystemNotFound /> : <>
        <header {...stylex.props(styles.header)}>
          <div {...stylex.props(styles.heading)}>
            {(scope === 'all' || job.channelId !== scope) && <ChannelAvatar name={jobChannel?.name ?? job.source.channelTitle ?? 'Kênh'} initials={jobChannel?.initials} src={jobChannel?.avatarUrl ?? undefined} color={jobChannel ? { bg: jobChannel.color, ring: jobChannel.color } : undefined} size={26} />}
            <h1 title={job.source.title} {...stylex.props(styles.title)}>{job.source.title}</h1>
            <div {...stylex.props(styles.actions)}>
              {stage === 'images' && job.stageGroup === 'working' && <Button size="compact" variant="ghost" kbd="Esc" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => void action('pause')}><Icon name="pause" size={15} />Tạm dừng</Button>}
              {stage === 'render' && <Button size="compact" variant="ghost" kbd="Esc" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => void action('pause')}><Icon name="pause" size={15} />Tạm dừng</Button>}
              {stage === 'paused' && <Button size="compact" variant="primary" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => void action('resume')}><Icon name="play" size={15} />Chạy tiếp</Button>}
              {stage === 'issue' && <Button size="compact" variant="ghost" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => void action('run')}><Icon name="refresh" size={15} />Thử lại</Button>}
              {stage === 'review' && <>
                <Button size="compact" variant="ghost" disabled={!allDrawn || !online} title={!online ? 'Mất kết nối' : !allDrawn ? 'Chờ Hoạ sĩ vẽ xong ảnh đang vẽ' : undefined} onClick={() => { setNotice(''); setRedrawAll(true) }}><Icon name="refresh" size={15} />Vẽ lại tất cả</Button>
                <Button size="compact" variant="primary" disabled={!allDrawn || approveBusy || !online} title={!online ? 'Mất kết nối' : !allDrawn ? 'Chờ Hoạ sĩ vẽ xong ảnh đang vẽ' : undefined} onClick={() => void approveAll()}><Icon name="tick-b" size={15} />Duyệt &amp; dựng</Button>
              </>}
              {stage === 'quality' && <>
                <Button size="compact" variant="ghost" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => void accept()}><Icon name="export" size={15} />Vẫn xuất</Button>
                {editorProjectId(job) ? <Button size="compact" variant="primary" onClick={() => navigate(routePath({ name: 'editor', projectId: editorProjectId(job)! }))}><Icon name="edit" size={15} />Sửa trong Biên tập</Button> : <Button size="compact" variant="primary" disabled title="Chưa mở được project"><Icon name="edit" size={15} />Sửa trong Biên tập</Button>}
              </>}
              {stage === 'done' && <>
                {job.stageGroup === 'ready' && <Button size="compact" variant="ghost" disabled={!online} title={!online ? 'Mất kết nối' : undefined} onClick={() => { setNotice(''); void requestMarkPosted(job, refresh).catch(err => setNotice(err instanceof Error ? err.message : 'Không đánh dấu được')) }}><Icon name="tick-b" size={15} />Đánh dấu đã đăng</Button>}
                {job.stageGroup === 'posted' && (job.exportBytes ?? 0) > 0 && <Button size="compact" variant="ghost" disabled={!online} title={!online ? 'Mất kết nối' : 'Xoá file MP4, giữ project và ảnh'} onClick={() => { setNotice(''); void requestDeleteExport(job, refresh).catch(err => setNotice(err instanceof Error ? err.message : 'Không xoá được')) }}><Icon name="trash" size={15} />Xoá MP4</Button>}
                <a href={bundleUrl([job.id])} {...stylex.props(styles.primaryLink)}><Icon name="download" size={15} />Tải gói đăng</a>
              </>}
              {!queuedJob && <Suspense fallback={null}><OverflowMenu key={job.id} job={job} onRefresh={refresh} onError={setNotice} /></Suspense>}
            </div>
          </div>
          <div {...stylex.props(styles.chips)}>
            <span {...stylex.props(styles.statusChip, (status.tone === 'review' || status.tone === 'active') && styles.statusReview, status.tone === 'warn' && styles.statusWarn, status.tone === 'ready' && styles.statusReady, status.tone === 'muted' && styles.statusMuted)}>
              {status.spin ? <span aria-hidden="true" {...stylex.props(styles.statusSpin)} /> : <Icon name={status.icon} size={14} />}
              <b>{status.text}</b>
            </span>
            <span {...stylex.props(styles.chip)}><Icon name="radar" size={14} />Nguồn <b>{job.source.channelTitle ?? '—'}</b>{job.source.outlier != null && <b {...stylex.props(styles.outlier)}>×{job.source.outlier.toLocaleString('vi-VN', { maximumFractionDigits: 1 })}</b>}</span>
            <span {...stylex.props(styles.chip)}><Icon name="layer" size={14} />Template <b>{job.templateName ?? job.templateId}</b></span>
            {job.source.durationSec != null && <span {...stylex.props(styles.chip)}><Icon name="timer" size={14} />Độ dài <b {...stylex.props(styles.num)}>{clockLabel(job.source.durationSec)}</b></span>}
          </div>
          {stage === 'review' && <div role="status" {...stylex.props(styles.banner, styles.bannerReview)}><Icon name="flag" size={15} xstyle={styles.reviewIcon} /><span {...stylex.props(styles.bannerText)}>Đã vẽ xong {items.length} ảnh. Vẽ lại ảnh nào chưa ưng, hoặc duyệt để Kiểm định dựng và xuất.</span></div>}
          {stage === 'paused' && <div role="status" {...stylex.props(styles.banner)}><Icon name="pause" size={15} xstyle={styles.noteIcon} /><span {...stylex.props(styles.bannerText)}>Video đang tạm dừng. Phần đã làm được giữ; video khác vẫn chạy. Bạn có thể góp ý trước khi chạy tiếp.</span></div>}
          {stage === 'issue' && <div role="alert" {...stylex.props(styles.banner, styles.bannerWarn)}><Icon name="danger" size={15} xstyle={styles.warnIcon} /><span {...stylex.props(styles.bannerText)}>{issueReason(job)}</span>{failedStep?.step === 'images' && failedItem && <ActionButton variant="outline" size="compact" disabled={!online || grokBusy} disabledReason={!online ? 'Mất kết nối' : undefined} onClick={() => void retryWithGrok()}><Icon name="magicpen" size={15} />{grokBusy ? 'Đang đọc bản lỗi…' : 'Vẽ tiếp bằng Grok'}</ActionButton>}{grokError && <span role="alert" {...stylex.props(styles.bannerError)}>{grokError}</span>}<a href="#/settings?section=connect" {...stylex.props(styles.bannerLink)}>Đăng nhập lại ở Cài đặt</a></div>}
          {stage === 'quality' && <div role="alert" {...stylex.props(styles.banner, styles.bannerWarn)}><Icon name="danger" size={15} xstyle={styles.warnIcon} /><span {...stylex.props(styles.bannerText)}>{score.data ? `Độ hoàn thiện ${score.data.total}/100, cần từ ${QUALITY_MIN_SCORE} để tự xuất.` : `Video chưa đạt Độ hoàn thiện, cần từ ${QUALITY_MIN_SCORE}/100 để tự xuất.`}{score.data?.blockers?.length ? ` ${score.data.blockers.map(blocker => blocker.message).join(' · ')}.` : ''} Sửa rồi bấm Thử lại, hoặc chọn Vẫn xuất.</span></div>}
        </header>
        {queuedJob ? <section {...stylex.props(styles.card, styles.queued)}>Video đang chờ lượt. <a href="#/queue">Mở Hàng chờ</a></section> : <LiveStudio job={job} onRefresh={refresh} onApproveAll={() => void approveAll()} approveBusy={approveBusy} redrawAllOpen={redrawAll} onRedrawAllChange={setRedrawAll} />}
        {notice && <div role="status" {...stylex.props(styles.notice)}>{notice}</div>}
      </>}
    </main>
    {job && !queuedJob && <aside hidden={!rightOpen} {...stylex.props(styles.panels)}><Suspense fallback={null}><WorkbenchPanels job={job} onRefresh={refresh} /></Suspense></aside>}
  </div>
}

/** Nhãn trạng thái của chip đầu video — mọi chữ đều suy từ dữ liệu thật. */
function statusLine(stage: JobStage, job: CloneJobView, drawingIndex: number): { text: string; tone: 'active' | 'review' | 'warn' | 'ready' | 'muted'; icon: IconName; spin: boolean } {
  switch (stage) {
    case 'review': return { text: 'Chờ bạn duyệt', tone: 'review', icon: 'flag', spin: false }
    case 'render': {
      const exportStep = job.steps?.find(step => step.step === 'export')
      const percent = exportStep ? stepPercent(exportStep) : null
      return { text: percent != null ? `Đang xuất video ${percent}%` : exportStep ? 'Đang xuất video' : 'Đang dựng video', tone: 'ready', icon: 'export', spin: true }
    }
    case 'issue': return { text: 'Cần xử lý', tone: 'warn', icon: 'danger', spin: false }
    case 'quality': return { text: 'Chưa đạt độ hoàn thiện', tone: 'warn', icon: 'danger', spin: false }
    case 'done': return job.stageGroup === 'posted'
      ? { text: 'Đã đăng', tone: 'muted', icon: 'tick-b', spin: false }
      : { text: 'Sẵn sàng đăng', tone: 'ready', icon: 'tick-b', spin: false }
    case 'paused': return { text: 'Tạm dừng', tone: 'muted', icon: 'pause', spin: false }
    case 'queued': return { text: 'Chờ lượt', tone: 'muted', icon: 'clock', spin: false }
    default: return { text: drawingIndex >= 0 ? `Đang vẽ cảnh ${drawingIndex + 1}` : 'Đang làm', tone: 'active', icon: 'gallery', spin: true }
  }
}

const styles = stylex.create({
  root: { display: 'flex', flex: '1 1 auto', minHeight: 0, minWidth: 0, '@media (max-width: 1100px)': { flexWrap: 'wrap', alignItems: 'flex-start' } },
  side: { flex: '0 0 248px', width: 248, minWidth: 0, overflowY: 'auto', padding: `${s['--vv-s-14']} ${s['--vv-s-10']}`, boxShadow: 'inset -1px 0 0 var(--line-1)', display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], '@media (max-width: 1100px)': { flex: '1 1 100%', width: 'auto', maxWidth: 'none', order: 2, overflow: 'visible', boxShadow: 'none', borderTop: '1px solid var(--line-1)' } },
  sideTitle: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: s['--vv-s-8'], padding: `0 ${s['--vv-s-2']} 0 ${s['--vv-s-8']}` },
  sideHeading: { margin: 0, display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], color: 'var(--color-fg-3)', fontSize: 13, fontWeight: 500 },
  selectButton: { height: 26, paddingInline: s['--vv-s-8'] },
  sideCount: { fontSize: 11.5, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' },
  progressRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], paddingInline: s['--vv-s-8'] },
  dayTrack: { flex: 1, display: 'block', height: 3, borderRadius: 3, backgroundColor: 'var(--color-surface-3)', overflow: 'hidden' },
  dayFill: { display: 'block', height: '100%', borderRadius: 3, backgroundColor: 'var(--color-fg)' },
  section: { display: 'flex', flexDirection: 'column', gap: 1 },
  group: { display: 'flex', justifyContent: 'space-between', padding: `${s['--vv-s-10']} ${s['--vv-s-8']} ${s['--vv-s-4']}`, margin: 0, color: 'var(--vv-fg-note)', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase' },
  rows: { listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 1 },
  row: { width: '100%', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-7']} ${s['--vv-s-8']}`, border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg)', textAlign: 'left', cursor: 'pointer', transition: 'background .15s', ':hover': { backgroundColor: 'var(--vv-hover-row)' }, ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: -2 } },
  selected: { backgroundColor: 'var(--color-surface-4)' },
  check: { flex: '0 0 auto', width: 14, height: 14, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', borderRadius: 4, border: '1px solid var(--line-3)', color: 'var(--color-bg)' },
  checkOn: { backgroundColor: 'var(--color-fg)', borderColor: 'var(--color-fg)' },
  rowText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 1, overflow: 'hidden' },
  rowTitle: { fontSize: 13, fontWeight: 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--color-fg-2)' },
  rowTitleCurrent: { fontWeight: 500, color: 'var(--color-fg)' },
  rowHint: { fontSize: 11.5, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  rowSpinner: { width: 12, height: 12, flexShrink: 0, borderRadius: '50%', border: '2px solid var(--line-3)', borderTopColor: 'var(--color-fg)', animationName: { default: 'ai-spin', '@media (prefers-reduced-motion: reduce)': 'none' }, animationDuration: '.9s', animationIterationCount: 'infinite' },
  channelRow: { width: '100%', display: 'flex', alignItems: 'center', gap: s['--vv-s-9'], padding: `${s['--vv-s-7']} ${s['--vv-s-8']}`, border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg)', textAlign: 'left', cursor: 'pointer', ':hover': { backgroundColor: 'var(--vv-hover-row)' }, ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: -2 } },
  channelText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  channelName: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], fontSize: 13, minWidth: 0 },
  channelToday: { marginLeft: 'auto', fontSize: 11.5, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  channelTrack: { display: 'block', height: 3, borderRadius: 3, backgroundColor: 'var(--color-surface-3)', overflow: 'hidden' },
  channelFill: { display: 'block', height: '100%', borderRadius: 3, backgroundColor: 'var(--vv-fg-note)' },
  channelFacts: { display: 'flex', gap: s['--vv-s-8'], fontSize: 11.5, whiteSpace: 'nowrap', overflow: 'hidden' },
  waitFact: { color: 'var(--vv-st-review)', fontVariantNumeric: 'tabular-nums' },
  readyFact: { color: 'var(--vv-st-ready)', fontVariantNumeric: 'tabular-nums' },
  note: { color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  suggestions: { display: 'flex', flexDirection: 'column', gap: 1 },
  suggestRowNotice: { padding: `${s['--vv-s-7']} ${s['--vv-s-8']}`, fontSize: 12, color: 'var(--vv-fg-note)' },
  moreLink: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], padding: `${s['--vv-s-5']} ${s['--vv-s-8']}`, borderRadius: r['--vv-r-8'], fontSize: 12, color: 'var(--color-fg-3)', ':hover': { backgroundColor: 'var(--vv-hover-row)' } },
  selectBar: { position: 'sticky', bottom: 0, zIndex: 2, display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: s['--vv-s-10'], borderRadius: r['--vv-r-12'], backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--line-2)' },
  selectText: { fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.45 },
  selectActions: { display: 'flex', gap: s['--vv-s-6'] },
  bundleLink: { flex: 1, height: 32, paddingInline: s['--vv-s-12'], display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-7'], borderRadius: r['--vv-r-8'], backgroundColor: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600 },
  bundleDisabled: { flex: 1 },
  footer: { marginTop: 'auto', padding: `${s['--vv-s-10']} ${s['--vv-s-8']} 0`, borderTop: '1px solid var(--line-1)', display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], fontSize: 12 },
  footRow: { display: 'flex', justifyContent: 'space-between', gap: s['--vv-s-8'], color: 'var(--vv-fg-note)' },
  footValue: { color: 'var(--color-fg-2)', whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' },
  center: { position: 'relative', flex: '1 1 520px', minWidth: 0, minHeight: 0, overflow: 'hidden', padding: `${s['--vv-s-12']} ${s['--vv-s-18']} ${s['--vv-s-10']}`, display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'], '@media (max-width: 1100px)': { flex: '1 1 100%', order: 1, minHeight: '70vh', overflow: 'visible' } },
  panels: { flex: '0 0 340px', width: 340, minHeight: 0, overflowY: 'auto', boxShadow: 'inset 1px 0 0 var(--line-1)', '@media (max-width: 1100px)': { flex: '1 1 100%', width: 'auto', maxWidth: 'none', maxHeight: '70vh', order: 3, boxShadow: 'none', borderTop: '1px solid var(--line-1)' } },
  header: { flex: '0 0 auto', display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  heading: { display: 'flex', gap: s['--vv-s-10'], alignItems: 'center', minWidth: 0 },
  title: { flex: 1, minWidth: 0, margin: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 17, fontWeight: 600, lineHeight: 1.35, letterSpacing: '-.015em' },
  actions: { flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], flexWrap: 'wrap' },
  primaryLink: { height: 32, paddingInline: s['--vv-s-12'], display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-7'], borderRadius: r['--vv-r-8'], backgroundColor: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600 },
  chips: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-6'] },
  statusChip: { height: 26, paddingInline: s['--vv-s-9'], display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-6'], borderRadius: r['--vv-r-7'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-1)', color: 'var(--vv-fg-note)', fontSize: 12, whiteSpace: 'nowrap' },
  statusReview: { backgroundColor: 'var(--vv-st-review-bg)', borderColor: 'var(--vv-st-review-border)', color: 'var(--vv-st-review)' },
  statusWarn: { backgroundColor: 'var(--vv-danger-8)', borderColor: 'var(--vv-danger-border)', color: 'var(--vv-warn-ink)' },
  statusReady: { backgroundColor: 'var(--vv-st-ready-bg)', borderColor: 'var(--vv-st-ready-28)', color: 'var(--vv-st-ready)' },
  statusMuted: { backgroundColor: 'var(--vv-wash-5)', color: 'var(--color-fg-3)' },
  statusSpin: { width: 11, height: 11, flexShrink: 0, borderRadius: '50%', border: '2px solid var(--line-3)', borderTopColor: 'currentColor', animationName: { default: 'ai-spin', '@media (prefers-reduced-motion: reduce)': 'none' }, animationDuration: '.9s', animationIterationCount: 'infinite' },
  chip: { height: 26, paddingInline: s['--vv-s-7'], display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-6'], borderRadius: r['--vv-r-7'], backgroundColor: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap' },
  outlier: { color: 'var(--vv-st-review)' },
  banner: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-12'], border: '1px solid var(--line-2)', backgroundImage: 'var(--card-bg)', color: 'var(--color-fg-2)', fontSize: 13, lineHeight: 1.5 },
  bannerReview: { borderColor: 'var(--vv-st-review-border)' },
  bannerWarn: { borderColor: 'var(--vv-danger-border)', backgroundImage: 'none', backgroundColor: 'var(--vv-danger-8)', color: 'var(--vv-warn-ink)' },
  bannerText: { flex: '1 1 300px', minWidth: 0 },
  bannerLink: { color: 'var(--color-fg)', textDecorationLine: 'underline', textUnderlineOffset: 3, whiteSpace: 'nowrap' },
  bannerError: { flexBasis: '100%', color: 'var(--vv-warn-ink)', fontSize: 12 },
  card: { border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], backgroundImage: 'var(--card-bg)', boxShadow: 'var(--card-shadow)', padding: `${s['--vv-s-8']} ${s['--vv-s-6']} ${s['--vv-s-4']}` },
  queued: { flex: 1, overflowY: 'auto' },
  notice: { padding: `${s['--vv-s-8']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-10'], border: '1px solid var(--line-2)', color: 'var(--color-fg-2)', fontSize: 12.5 },
  error: { padding: s['--vv-s-24'], color: 'var(--color-fg)' },
  reviewIcon: { color: 'var(--vv-st-review)' },
  warnIcon: { color: 'var(--vv-danger)' },
  okIcon: { color: 'var(--color-ok)' },
  noteIcon: { color: 'var(--vv-fg-note)' },
  num: { fontVariantNumeric: 'tabular-nums' },
})
