// Một hàng của bảng Kênh › Video (artboard Channel `.row`): ô chọn khi Chọn nhiều,
// thumbnail 80×45 + thời lượng, tên (ellipsis + tooltip), dòng phụ, trạng thái +
// chi tiết, template, nút “Việc tiếp” theo trạng thái và menu ⋯.
import { memo, type MouseEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { ActionMenu, Icon, type ActionMenuItem } from '@noverse/ui'
import type { ChannelJob } from '../../api/channelVideos'
import { statusColor, statusName } from '../../components/StatusLabel'
import { requestMarkPosted } from '../../components/performance'
import { t } from '../../i18n/channels'
import { navigate, routePath } from '../../router'
import { editorProjectId } from '../workbench/workbench-actions'
import { removeVideo, restartVideo, stopVideo } from '../workbench/jobActions'
import { fmtDuration } from '../explore/fmt'
import { downloadBundle } from './videoBulk'
import { sourceLine, statusDetail, statusOf, type RowAction } from './videosLogic'
import { videoStyles as sx } from './videos.stylex'

const tt = t.channels.videos.list

export type VideoRowProps = {
  job: ChannelJob
  now: number
  selecting: boolean
  chosen: boolean
  templateName: string
  /** giờ bắt đầu dự kiến của video Chờ lượt (từ hàng chờ) */
  etaStart: string | null
  /** dòng số liệu 7 ngày của video Đã đăng (`postedDetail`), null nếu chưa có */
  postedLine: string | null
  action: RowAction | null
  onPick: (id: string, shift: boolean) => void
  onRefresh: () => void
  onError: (message: string) => void
}

const openJob = (job: ChannelJob) => navigate(routePath({ name: 'workbench', channel: job.channelId, video: job.id }))
const workbenchHref = (job: ChannelJob) => `#${routePath({ name: 'workbench', channel: job.channelId, video: job.id })}`
const reviewHref = (job: ChannelJob) => `#${routePath({ name: 'review', channel: job.channelId, video: job.id })}`

function menuItems(job: ChannelJob, props: Pick<VideoRowProps, 'onRefresh' | 'onError'>): ActionMenuItem[] {
  const m = tt.menu
  const report = (error: unknown) => props.onError(error instanceof Error ? error.message : t.channels.videos.bulk.failed)
  const project = editorProjectId(job)
  const items: ActionMenuItem[] = [{ id: 'open', label: m.open, onSelect: () => openJob(job) }]
  if (project) items.push({ id: 'editor', label: m.editor, onSelect: () => navigate(`/editor/${encodeURIComponent(project)}`) })
  if (job.stageGroup === 'ready') {
    items.push({ id: 'pack', label: m.pack, onSelect: () => downloadBundle([job.id]) })
    items.push({ id: 'posted', label: m.markPosted, onSelect: () => { void requestMarkPosted(job, props.onRefresh).catch(report) } })
  }
  if (!job.supersededAt) items.push({ id: 'restart', label: m.restart, onSelect: () => restartVideo(job, { onRefresh: props.onRefresh, onError: report }) })
  if (job.stageGroup === 'working' || job.stageGroup === 'paused') items.push({ id: 'stop', label: m.stop, onSelect: () => stopVideo(job, { onRefresh: props.onRefresh, onError: report }) })
  if (/^https?:\/\//i.test(job.source.url)) items.push({ id: 'source', label: m.source, onSelect: () => window.open(job.source.url, '_blank', 'noreferrer') })
  items.push({ id: 'remove', label: m.remove, danger: true, lockedReason: job.stageGroup === 'working' ? m.removeLocked : undefined, onSelect: () => removeVideo(job, { onRefresh: props.onRefresh, onError: report, afterRemove: props.onRefresh }) })
  return items
}

export const VideoRow = memo(function VideoRow(props: VideoRowProps) {
  const { job, now, selecting, chosen, templateName, etaStart, postedLine, action, onPick } = props
  const status = statusOf(job)
  const title = job.source.title
  const detail = statusDetail(job, { now, etaStart, posted: postedLine })
  const sub = sourceLine(job, now)

  const click = (event: MouseEvent<HTMLDivElement>) => {
    if ((event.target as HTMLElement).closest('a, button, [role="menu"]')) return
    if (selecting) onPick(job.id, event.shiftKey)
    else openJob(job)
  }

  return (
    <div role="row" onClick={click} {...stylex.props(sx.grid, sx.row, selecting && sx.rowSelecting, selecting && chosen && sx.rowChosen)}>
      {selecting && (
        <button
          type="button"
          role="checkbox"
          aria-checked={chosen}
          aria-label={tt.pick(title)}
          onClick={(event) => { event.stopPropagation(); onPick(job.id, event.shiftKey) }}
          {...stylex.props(sx.check)}
        >
          <span aria-hidden="true" {...stylex.props(sx.box, chosen && sx.boxOn)}>{chosen ? '✓' : ''}</span>
        </button>
      )}
      <a role="cell" href={workbenchHref(job)} {...stylex.props(sx.video)}>
        <span aria-hidden="true" {...stylex.props(sx.thumb)}>
          {job.source.thumbnailUrl ? <img src={job.source.thumbnailUrl} alt="" loading="lazy" {...stylex.props(sx.thumbImg)} /> : null}
          {job.source.durationSec != null && <span className={`num mono ${stylex.props(sx.duration).className}`}>{fmtDuration(job.source.durationSec)}</span>}
        </span>
        <span {...stylex.props(sx.titles)}>
          <span title={title} {...stylex.props(sx.title)}>{title}</span>
          <span title={sub} {...stylex.props(sx.sub)}>{sub}</span>
        </span>
      </a>
      <span role="cell" {...stylex.props(sx.status)}>
        <span {...stylex.props(sx.statusRow, sx.tone(statusColor(status)))}>
          <span aria-hidden="true" {...stylex.props(status === 'working' ? sx.spinner : sx.statusDot)} />
          {statusName(status)}
        </span>
        {detail && <span title={detail} {...stylex.props(sx.statusDetail)}>{detail}</span>}
      </span>
      <span role="cell" title={templateName} {...stylex.props(sx.tpl, sx.template)}>{templateName}</span>
      <span role="cell" {...stylex.props(sx.actions)}>
        {action === 'pack'
          ? <button type="button" onClick={() => downloadBundle([job.id])} {...stylex.props(sx.act)}>{tt.pack}</button>
          : action === 'review'
            ? <a href={reviewHref(job)} {...stylex.props(sx.act, sx.actReview)}>{tt.review}</a>
            : action === 'fix'
              ? <a href={workbenchHref(job)} {...stylex.props(sx.act, sx.actFix)}>{tt.fix}</a>
              : action === 'open'
                ? <a href={workbenchHref(job)} {...stylex.props(sx.act)}>{tt.open}</a>
                : null}
        <ActionMenu
          alwaysVisible
          label={tt.moreFor(title)}
          items={menuItems(job, props)}
          width={248}
          triggerStyle={sx.more}
          trigger={<Icon name="more" size={15} />}
        />
      </span>
    </div>
  )
})
