// Tab Video của trang Kênh (artboard Channel › tab=videos): danh sách video của
// kênh với bộ lọc, Chọn nhiều + thanh thao tác ở dock, hoặc thẻ “kênh chưa có
// video” khi kênh chưa có job nào. Dữ liệu `jobs` do ChannelScreen đọc (số đầu
// kênh và số trên tab dùng chung); hàng chờ chỉ để lấy giờ bắt đầu dự kiến.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { toast } from '@noverse/ui'
import { listTemplates } from '../../api/library'
import { getQueue } from '../../api/clone'
import type { Channel } from '../../api/channels'
import type { ChannelJobs } from '../../api/channelVideos'
import type { Rival } from '../../api/rivals'
import { keys } from '../../data/keys'
import { useResource, type Resource } from '../../data/useResource'
import { VideoBulkBar } from './VideoBulkBar'
import { VideosFresh } from './VideosFresh'
import { VideosList } from './VideosList'
import { confirmBulk, downloadBundle, runBulkAction } from './videoBulk'
import { NO_SELECTION, bulkCounts, eligibleSelected, pruneSelection, toggleSelection, filterJobs, type FilterId, type Selection } from './videosLogic'

export function VideosTab({ channel, rival, jobs, dockSlot, onSelecting }: {
  channel: Channel
  /** đối thủ tham chiếu (link sớm nhất) — nguồn của thẻ "Video mới nhất" */
  rival: Rival | undefined
  jobs: Resource<ChannelJobs>
  /** chỗ trong dock của ChannelScreen để đặt thanh Chọn nhiều */
  dockSlot: HTMLElement | null
  onSelecting: (active: boolean) => void
}) {
  const [filter, setFilter] = useState<FilterId>('all')
  const [selection, setSelection] = useState<Selection>(NO_SELECTION)
  const [busy, setBusy] = useState(false)
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal), { enabled: (jobs.data?.queuedTotal ?? 0) > 0 })

  const list = jobs.data?.jobs
  const eligibility = jobs.data?.bulkEligibility
  const templateNames = useMemo(() => new Map((templates.data?.items ?? []).map((item) => [item.id, item.name])), [templates.data])
  const etaById = useMemo(() => new Map((queue.data?.items ?? []).map((item) => [item.id, item.etaStart])), [queue.data])
  const order = useMemo(() => (list ? filterJobs(list, filter).map((job) => job.id) : []), [list, filter])

  const selecting = selection.active
  useEffect(() => { onSelecting(selecting) }, [selecting, onSelecting])
  useEffect(() => () => onSelecting(false), [onSelecting])
  // Video bị xoá/ẩn khỏi danh sách thì bỏ khỏi lựa chọn.
  useEffect(() => { if (list) setSelection((state) => pruneSelection(state, list.map((job) => job.id))) }, [list])
  // Esc thoát chọn (không động tới Esc đã được menu/hộp thoại xử lý).
  useEffect(() => {
    if (!selecting) return
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape' && !event.defaultPrevented) setSelection(NO_SELECTION) }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selecting])

  const toggleMode = useCallback(() => setSelection((state) => (state.active ? NO_SELECTION : { ...state, active: true })), [])
  const pick = useCallback((id: string, shift: boolean) => setSelection((state) => toggleSelection(state, id, shift, order)), [order])
  const refresh = useCallback(() => { void jobs.refresh() }, [jobs])
  const report = useCallback((message: string) => toast.err(message), [])

  if (list && list.length === 0 && !jobs.error) return <VideosFresh channel={channel} rival={rival} />

  const counts = eligibility ? bulkCounts(selection.picked, eligibility) : undefined
  const run = async (action: 'retry' | 'approve' | 'posted', ids: readonly string[]) => {
    if (busy || ids.length === 0) return
    setBusy(true)
    try {
      const outcome = await runBulkAction(action, ids)
      const done = new Set(outcome.ok)
      setSelection((state) => ({ ...state, picked: new Set([...state.picked].filter((id) => !done.has(id))) }))
    } finally { setBusy(false) }
  }
  const confirmRun = async (action: 'cancel' | 'delete', ids: readonly string[]) => {
    if (busy || ids.length === 0) return
    const outcome = await confirmBulk(action, ids)
    if (!outcome) return
    const done = new Set(outcome.ok)
    setSelection((state) => ({ ...state, picked: new Set([...state.picked].filter((id) => !done.has(id))) }))
  }

  return (
    <>
      <VideosList
        channelId={channel.id}
        jobs={jobs}
        filter={filter}
        onFilter={setFilter}
        selection={selection}
        onToggleSelect={toggleMode}
        onPick={pick}
        templateNames={templateNames}
        etaById={etaById}
        onBundleDone={downloadBundle}
        onRefresh={refresh}
        onError={report}
      />
      {selecting && dockSlot && eligibility && counts && createPortal(
        <VideoBulkBar
          count={selection.picked.size}
          counts={counts}
          busy={busy}
          onRetry={() => void run('retry', eligibleSelected(selection.picked, eligibility.retry))}
          onApprove={() => void run('approve', eligibleSelected(selection.picked, eligibility.approve))}
          onBundle={() => downloadBundle(eligibleSelected(selection.picked, eligibility.bundle))}
          onMarkPosted={() => void run('posted', eligibleSelected(selection.picked, eligibility.markPosted))}
          onCancel={() => void confirmRun('cancel', eligibleSelected(selection.picked, eligibility.cancel))}
          onRemove={() => void confirmRun('delete', eligibleSelected(selection.picked, eligibility.delete))}
          onExit={() => setSelection(NO_SELECTION)}
        />,
        dockSlot,
      )}
    </>
  )
}
