// Bàn dựng trên điện thoại (artboard MMain): màn cha giữ dữ liệu/khoá cache
// chung với bản máy tính, chọn danh sách ↔ chi tiết ↔ Bản tin, ẩn thanh tab khi
// mở trang con và khôi phục vị trí cuộn khi quay lại.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, useTabMode, toast } from '@noverse/ui'
import { getChannelSummaries } from '../../../api/workbenchChannels'
import { getJobImages, getRecap, getWorkbench, getWorkbenchJob, markRecapSeen, type CloneJobView, type JobImageList } from '../../../api/workbench'
import { getNotificationSettings } from '../../../api/activity'
import type { AssistantScope } from '../../../api/assistant'
import { useResource } from '../../../data/useResource'
import { subscribeEvents } from '../../../data/events'
import { navigate, routePath, type Route } from '../../../router'
import { followAdmitted } from '../workbench-logic'
import { shouldAutoOpenRecap } from '../recap/recap-logic'
import { mw } from './mworkbench.stylex'
import { MobileWorkbenchList } from './MobileWorkbenchList'
import { MobileWorkbenchDetail } from './MobileWorkbenchDetail'
import { MobileRecap } from './MobileRecap'
import { MobilePlanSheet } from './MobilePlanSheet'

const SCOPE_KEY = 'viderse:workbench-scope'

export function MobileWorkbench({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  const scope = route.channel || 'all'
  const selectedId = route.video
  const list = useResource(['workbench', 'list', scope], ({ signal }) => getWorkbench(scope, signal), { keepPrevious: true })
  const channels = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const queuedJob = list.data?.jobs?.find((item) => item.id === selectedId && item.stageGroup === 'queued')
  const detail = useResource(['workbench', 'job', selectedId || ''], ({ signal }) => getWorkbenchJob(selectedId!, signal), { enabled: Boolean(selectedId) && !queuedJob })
  const images = useResource(['workbench', 'images', selectedId || ''], ({ signal }) => getJobImages(selectedId!, signal), {
    enabled: Boolean(selectedId) && !queuedJob && (detail.data ? !detail.data.queueItem : false),
  })
  const recap = useResource(['workbench', 'recap'], ({ signal }) => getRecap(signal))
  const recapSettings = useResource(['activity', 'settings'], ({ signal }) => getNotificationSettings(signal))
  const [recapOpen, setRecapOpen] = useState(false)
  const [plan, setPlan] = useState<{ text: string; scope: AssistantScope } | null>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const savedListScroll = useRef(new Map<string, number>())
  const restoredView = useRef('')
  const autoRecapChecked = useRef(false)
  const followed = useRef<{ id: string; sourceKey: string } | null>(null)
  const job = queuedJob || detail.data

  useTabMode(selectedId || recapOpen ? 'hide' : undefined)

  const scrollParent = (): HTMLElement | null => {
    let node = rootRef.current?.parentElement ?? null
    while (node) {
      const overflow = getComputedStyle(node).overflowY
      if (overflow === 'auto' || overflow === 'scroll') return node
      node = node.parentElement
    }
    return null
  }
  const saveListScroll = () => {
    if (!selectedId && !recapOpen) savedListScroll.current.set(scope, scrollParent()?.scrollTop ?? 0)
  }
  const refresh = () => {
    void list.refresh(); void channels.refresh(); void recap.refresh()
    if (selectedId) { void detail.refresh(); void images.refresh() }
  }
  const openJob = (id: string) => {
    saveListScroll()
    navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: id }))
  }
  const closeJob = () => {
    navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope }))
  }
  const openRecap = () => {
    autoRecapChecked.current = true
    saveListScroll()
    setRecapOpen(true)
    void recap.refresh()
  }
  const closeRecap = () => {
    autoRecapChecked.current = true
    setRecapOpen(false)
    void markRecapSeen().then(() => recap.refresh()).catch(() => {})
  }
  const pickScope = (channel?: string) => {
    saveListScroll()
    try { localStorage.setItem(SCOPE_KEY, channel || 'all') } catch { /* storage unavailable */ }
    navigate(routePath({ name: 'workbench', channel, scope: route.scope }))
  }

  useLayoutEffect(() => {
    const view = `${scope}:${selectedId ?? 'list'}:${recapOpen}`
    if (restoredView.current === view || (!selectedId && !recapOpen && list.placeholder)) return
    const parent = scrollParent()
    if (!parent) return
    if (!recapOpen) parent.scrollTop = selectedId ? 0 : savedListScroll.current.get(scope) ?? 0
    restoredView.current = view
  }, [scope, selectedId, recapOpen, list.placeholder])

  useEffect(() => {
    if (selectedId || recapOpen || list.placeholder) return
    const parent = scrollParent()
    if (!parent) return
    const save = () => savedListScroll.current.set(scope, parent.scrollTop)
    parent.addEventListener('scroll', save, { passive: true })
    return () => parent.removeEventListener('scroll', save)
  }, [scope, selectedId, recapOpen, list.placeholder])

  useEffect(() => {
    if (queuedJob) { followed.current = { id: queuedJob.id, sourceKey: queuedJob.sourceKey }; return }
    if (followed.current?.id !== selectedId) { followed.current = null; return }
    const next = followAdmitted(list.data?.jobs ?? [], followed.current)
    if (next) navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: next }), true)
  }, [queuedJob, selectedId, list.data, route.channel, route.scope])

  useEffect(() => {
    if (!recap.data || !recapSettings.data || autoRecapChecked.current) return
    autoRecapChecked.current = true
    // Mở thẳng một video thì không tự nhảy sang Bản tin.
    if (selectedId) return
    if (shouldAutoOpenRecap(recap.data.from, recap.data.to, recap.data.hasNew, recapSettings.data.recapOnReturn)) {
      saveListScroll()
      setRecapOpen(true)
    }
  }, [recap.data, recapSettings.data, selectedId])

  useEffect(() => subscribeEvents(['clone.job.created', 'clone.job.updated', 'clone.job.deleted', 'clone.step.progress', 'queue.updated', 'channels'], refresh), [list.refresh, channels.refresh, detail.refresh, images.refresh, recap.refresh, selectedId])

  useEffect(() => {
    if (route.channel || route.scope === 'all') {
      try { localStorage.setItem(SCOPE_KEY, route.channel || 'all') } catch { /* storage unavailable */ }
    }
  }, [route.channel, route.scope])

  const channelName = channels.data?.find((item) => item.channelId === scope)?.name
  const jobChannel = job ? channels.data?.find((item) => item.channelId === job.channelId)?.name : undefined
  const openPlan = (text: string, planScope: AssistantScope) => setPlan({ text, scope: planScope })

  return <div ref={rootRef}>
    {recapOpen
      ? <MobileRecap data={recap.data} loading={recap.initialLoading} error={recap.error} channels={channels.data ?? []}
        onRetry={() => void recap.refresh()} onClose={closeRecap} onSelectJob={(id) => { setRecapOpen(false); openJob(id) }}
        onPlan={(text) => openPlan(text, 'all')} />
      : selectedId
        ? <DetailOrState job={job} loading={detail.initialLoading && !queuedJob} error={detail.error} images={images.data} imagesLoading={images.initialLoading} imagesError={images.error}
          jobChannel={jobChannel} onBack={closeJob} onRefresh={refresh}
          onPlan={(text) => openPlan(text, { jobId: selectedId })} onNotice={(text) => toast.err(text)} />
        : <MobileWorkbenchList scope={scope} jobs={list.data?.jobs ?? []} queuedTotal={list.data?.queuedTotal ?? 0}
          channels={channels.data ?? []} channelsLoading={channels.initialLoading} listLoading={list.initialLoading}
          listError={list.error} recap={recap.data} recapLoading={recap.initialLoading} recapError={recap.error}
          onRetry={() => { void list.refresh(); void channels.refresh() }} onPickScope={pickScope} onOpenJob={openJob} onOpenRecap={openRecap} />}
    <MobilePlanSheet open={plan !== null} text={plan?.text ?? ''} scope={plan?.scope ?? 'all'} onClose={() => setPlan(null)} onSettled={refresh} />
  </div>
}

function DetailOrState({ job, loading, error, onBack, ...rest }: {
  job: CloneJobView | undefined
  loading: boolean
  error: unknown
  images: JobImageList | undefined
  imagesLoading: boolean
  imagesError: unknown
  jobChannel?: string
  onBack: () => void
  onRefresh: () => void
  onPlan: (text: string) => void
  onNotice: (text: string) => void
}) {
  if (loading) return <div role="status" {...stylex.props(mw.state)}>Đang mở video…</div>
  if (!job) return <div role="alert" {...stylex.props(mw.state)}>
    <span {...stylex.props(mw.stateTitle)}>Không tìm thấy video này.</span>
    {error instanceof Error ? error.message : ''}
    <div {...stylex.props(mw.emptyActions)}><Button variant="quiet" size="sm" onClick={onBack}>‹ Bàn dựng</Button></div>
  </div>
  return <MobileWorkbenchDetail {...rest} job={job} onBack={onBack} />
}
