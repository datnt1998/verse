// Bàn dựng trên điện thoại (artboard MMain): màn cha giữ dữ liệu/khoá cache
// chung với bản máy tính, chọn danh sách ↔ chi tiết. V37 chỉ giữ phần đang dùng:
// bỏ Bản tin, sheet Kế hoạch, ô nhắn và mọi lời gọi recap/plan; bố cục điện thoại
// đầy đủ vẫn thuộc đợt 44.
import { Suspense, lazy, useEffect, useLayoutEffect, useRef } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, KeepAlive, useTabMode, toast } from '@noverse/ui'
import { getChannelSummaries } from '../../../api/workbenchChannels'
import { getJobImages, getWorkbench, getWorkbenchJob, type CloneJobView, type JobImageList } from '../../../api/workbench'
import { useResource } from '../../../data/useResource'
import { subscribeEvents } from '../../../data/events'
import { navigate, routePath, type Route } from '../../../router'
import { followAdmitted } from '../workbench-logic'
import { mw } from './mworkbench.stylex'
import { MobileWorkbenchList } from './MobileWorkbenchList'

const MobileWorkbenchDetail = lazy(() => import('./MobileWorkbenchDetail').then((module) => ({ default: module.MobileWorkbenchDetail })))

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
  const rootRef = useRef<HTMLDivElement>(null)
  const savedListScroll = useRef(new Map<string, number>())
  const restoredView = useRef('')
  const followed = useRef<{ id: string; sourceKey: string } | null>(null)
  const job = queuedJob || detail.data

  useTabMode(selectedId ? 'hide' : undefined)

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
    if (!selectedId) savedListScroll.current.set(scope, scrollParent()?.scrollTop ?? 0)
  }
  const refresh = () => {
    void list.refresh(); void channels.refresh()
    if (selectedId) { void detail.refresh(); void images.refresh() }
  }
  const openJob = (id: string) => {
    saveListScroll()
    navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: id }))
  }
  const closeJob = () => {
    navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope }))
  }
  const pickScope = (channel?: string) => {
    saveListScroll()
    try { localStorage.setItem(SCOPE_KEY, channel || 'all') } catch { /* storage unavailable */ }
    navigate(routePath({ name: 'workbench', channel, scope: route.scope }))
  }

  useLayoutEffect(() => {
    const view = `${scope}:${selectedId ?? 'list'}`
    if (restoredView.current === view || (!selectedId && (list.placeholder || !list.data))) return
    const parent = scrollParent()
    if (!parent) return
    parent.scrollTop = selectedId ? 0 : savedListScroll.current.get(scope) ?? 0
    restoredView.current = view
  }, [scope, selectedId, list.placeholder, list.data])

  useEffect(() => {
    if (selectedId || list.placeholder) return
    const parent = scrollParent()
    if (!parent) return
    const save = () => savedListScroll.current.set(scope, parent.scrollTop)
    parent.addEventListener('scroll', save, { passive: true })
    return () => parent.removeEventListener('scroll', save)
  }, [scope, selectedId, list.placeholder])

  useEffect(() => {
    if (queuedJob) { followed.current = { id: queuedJob.id, sourceKey: queuedJob.sourceKey }; return }
    if (followed.current?.id !== selectedId) { followed.current = null; return }
    const next = followAdmitted(list.data?.jobs ?? [], followed.current)
    if (next) navigate(routePath({ name: 'workbench', channel: route.channel, scope: route.scope, video: next }), true)
  }, [queuedJob, selectedId, list.data, route.channel, route.scope])

  useEffect(() => subscribeEvents(['clone.job.created', 'clone.job.updated', 'clone.job.deleted', 'clone.step.progress', 'queue.updated', 'channels'], refresh), [list.refresh, channels.refresh, detail.refresh, images.refresh, selectedId])

  useEffect(() => {
    if (route.channel || route.scope === 'all') {
      try { localStorage.setItem(SCOPE_KEY, route.channel || 'all') } catch { /* storage unavailable */ }
    }
  }, [route.channel, route.scope])

  const channelName = channels.data?.find((item) => item.channelId === scope)?.name
  const jobChannel = job ? channels.data?.find((item) => item.channelId === job.channelId)?.name : undefined

  return <div ref={rootRef} style={{ display: 'contents' }}>
    {/* Giữ DOM danh sách khi mở video: vị trí cuộn khôi phục không bị kẹp vào
        chiều cao tạm thời lúc danh sách mount lại. */}
    <KeepAlive active={!selectedId}>
      <MobileWorkbenchList scope={scope} jobs={list.data?.jobs ?? []} queuedTotal={list.data?.queuedTotal ?? 0}
        channels={channels.data ?? []} channelsLoading={channels.initialLoading} listLoading={list.initialLoading}
        listError={list.error} onRetry={() => { void list.refresh(); void channels.refresh() }} onPickScope={pickScope} onOpenJob={openJob} />
    </KeepAlive>
    <Suspense fallback={<div role="status" {...stylex.props(mw.state)}>Đang mở…</div>}>
      {selectedId
        ? <DetailOrState job={job} loading={detail.initialLoading && !queuedJob} error={detail.error} images={images.data} imagesLoading={images.initialLoading} imagesError={images.error}
          jobChannel={jobChannel} channelName={channelName} onBack={closeJob} onRefresh={refresh} onNotice={(text) => toast.err(text)} />
        : null}
    </Suspense>
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
  channelName?: string
  onBack: () => void
  onRefresh: () => void
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
