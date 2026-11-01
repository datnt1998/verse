// Danh sách của Kênh › Video: thanh lọc, Chọn nhiều, Tải gói đã xong, thẻ Trợ lý
// nhận ra (Đã đăng / teaser ở Tất cả), bảng, trạng thái tải/lỗi/rỗng.
// `perf` chỉ có khi bộ lọc là Tất cả hoặc Đã đăng (usePerformance chỉ gọi ở đó).
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Skeleton } from '@noverse/ui'
import type { ChannelJob, ChannelJobs } from '../../api/channelVideos'
import type { Performance } from '../../api/performance'
import { InsightsTeaser, PostedInsights, postedDetail, usePerformance } from '../../components/performance'
import { statusColor } from '../../components/StatusLabel'
import type { Resource } from '../../data/useResource'
import { t } from '../../i18n/vi'
import { VideoRow } from './VideoRow'
import { FILTER_IDS, countByFilter, filterJobs, rowAction, type FilterId, type Selection } from './videosLogic'
import { videoStyles as sx } from './videos.stylex'

const tt = t.channels.videos.list
const PAGE = 30

const DOT: Record<FilterId, string> = {
  all: 'var(--color-fg)',
  working: statusColor('working'),
  waiting: statusColor('review'),
  queued: statusColor('queued'),
  ready: statusColor('ready'),
  posted: statusColor('posted'),
}

export type ListProps = {
  channelId: string
  jobs: Resource<ChannelJobs>
  filter: FilterId
  onFilter: (filter: FilterId) => void
  selection: Selection
  onToggleSelect: () => void
  onPick: (id: string, shift: boolean) => void
  templateNames: ReadonlyMap<string, string>
  etaById: ReadonlyMap<string, string | null>
  onBundleDone: (ids: readonly string[]) => void
  onRefresh: () => void
  onError: (message: string) => void
}

type BodyProps = ListProps & { perf: Performance | undefined }

export function VideosList(props: ListProps) {
  return props.filter === 'all' || props.filter === 'posted' ? <WithPerformance {...props} /> : <ListBody {...props} perf={undefined} />
}

function WithPerformance(props: ListProps) {
  const perf = usePerformance(props.channelId)
  return <ListBody {...props} perf={perf.data} />
}

function ListBody({ channelId, jobs, filter, onFilter, selection, onToggleSelect, onPick, templateNames, etaById, onBundleDone, onRefresh, onError, perf }: BodyProps) {
  const data = jobs.data
  const list = data?.jobs
  const counts = countByFilter(list ?? [])
  const rows = list ? filterJobs(list, filter) : []
  const [limit, setLimit] = useState(PAGE)
  const sentinel = useRef<HTMLDivElement>(null)
  useEffect(() => setLimit(PAGE), [filter, channelId])
  const shown = rows.slice(0, limit)
  const hasMore = rows.length > shown.length
  useEffect(() => {
    const node = sentinel.current
    if (!node || !hasMore || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => { if (entries.some((entry) => entry.isIntersecting)) setLimit((value) => value + PAGE) })
    observer.observe(node)
    return () => observer.disconnect()
  }, [hasMore, shown.length])

  const bundleIds = new Set(data?.bulkEligibility.bundle ?? [])
  const now = Date.now()
  const selecting = selection.active

  return (
    <section aria-label={tt.region} {...stylex.props(sx.section)}>
      <div {...stylex.props(sx.toolbar)}>
        <div role="radiogroup" aria-label={tt.filterLabel} {...stylex.props(sx.filters)}>
          {FILTER_IDS.map((id) => (
            <button key={id} type="button" role="radio" aria-checked={filter === id} onClick={() => onFilter(id)} {...stylex.props(sx.chip, filter === id && sx.chipOn)}>
              <span aria-hidden="true" {...stylex.props(sx.chipDot(DOT[id]))} />
              {tt.filters[id]}
              {list && <span className="num" {...stylex.props(sx.chipCount)}>{counts[id]}</span>}
            </button>
          ))}
        </div>
        <span {...stylex.props(sx.spacer)} />
        <button type="button" aria-pressed={selecting} onClick={onToggleSelect} {...stylex.props(sx.toolBtn, selecting && sx.toolBtnOn)}>
          <Icon name="tick-b" size={15} xstyle={[sx.toolIcon, selecting && sx.toolIconOn]} />
          {selecting ? tt.exitSelect : tt.select}
        </button>
        {!selecting && (
          <button
            type="button"
            aria-disabled={bundleIds.size === 0}
            title={bundleIds.size === 0 ? tt.bundleNone : undefined}
            onClick={() => { if (bundleIds.size > 0) onBundleDone(data!.bulkEligibility.bundle) }}
            {...stylex.props(sx.toolBtn, bundleIds.size === 0 && sx.toolBtnLocked)}
          >
            <Icon name="download" size={15} />
            {tt.bundleDone}
          </button>
        )}
      </div>

      {filter === 'all' && !selecting && <InsightsTeaser channelId={channelId} onOpen={() => onFilter('posted')} />}
      {filter === 'posted' && <PostedInsights channelId={channelId} />}

      <div {...stylex.props(sx.card)}>
        <div role="table" aria-label={tt.table} aria-busy={jobs.initialLoading || undefined} {...stylex.props(sx.table)}>
          <div role="row" {...stylex.props(sx.grid, sx.head)}>
            <span role="columnheader">{tt.colVideo}</span>
            <span role="columnheader">{tt.colStatus}</span>
            <span role="columnheader" {...stylex.props(sx.template)}>{tt.colTemplate}</span>
            <span role="columnheader" {...stylex.props(sx.right)}>{tt.colNext}</span>
          </div>
          {data === undefined && !jobs.error && Array.from({ length: 6 }, (_, index) => (
            <div key={index} role="row" {...stylex.props(sx.grid, sx.skelRow)}>
              <span {...stylex.props(sx.skelVideo)}><Skeleton width={80} height={45} radius={7} /><span {...stylex.props(sx.skelText)}><Skeleton width={220} height={13} /><Skeleton width={150} height={11} /></span></span>
              <Skeleton width={90} height={13} />
              <span {...stylex.props(sx.template)}><Skeleton width={80} height={12} /></span>
              <span />
            </div>
          ))}
          {data === undefined && !!jobs.error && (
            <div role="alert" {...stylex.props(sx.empty)}>
              <Icon name="video-play" size={22} xstyle={sx.emptyIcon} />
              <span {...stylex.props(sx.emptyText)}>{tt.errorTitle}</span>
              <span {...stylex.props(sx.sub)}>{tt.errorHint}</span>
              <button type="button" onClick={() => void jobs.refresh()} {...stylex.props(sx.toolBtn, sx.emptyBtn)}>{t.common.retry}</button>
            </div>
          )}
          {shown.map((job: ChannelJob) => (
            <VideoRow
              key={job.id}
              job={job}
              now={now}
              selecting={selecting}
              chosen={selection.picked.has(job.id)}
              templateName={templateNames.get(job.templateId) ?? job.templateId}
              etaStart={etaById.get(job.id) ?? null}
              postedLine={job.stageGroup === 'posted' ? postedDetail(perf, job.id) : null}
              action={rowAction(job, bundleIds)}
              onPick={onPick}
              onRefresh={onRefresh}
              onError={onError}
            />
          ))}
          {data !== undefined && rows.length === 0 && (
            <div {...stylex.props(sx.empty)}>
              <Icon name="video-play" size={22} xstyle={sx.emptyIcon} />
              <span {...stylex.props(sx.emptyText)}>{tt.emptyFilter}</span>
              <button type="button" onClick={() => onFilter('all')} {...stylex.props(sx.toolBtn, sx.emptyBtn)}>{tt.showAll}</button>
            </div>
          )}
        </div>
      </div>
      {hasMore && <div ref={sentinel} />}
      {data !== undefined && rows.length > 0 && hasMore && <span {...stylex.props(sx.footnote)}>{tt.windowed(shown.length, rows.length)}</span>}
    </section>
  )
}
