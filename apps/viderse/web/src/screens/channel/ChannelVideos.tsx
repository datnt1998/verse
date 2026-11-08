import * as stylex from '@stylexjs/stylex'
import { EmptyState, Skeleton } from '@noverse/ui'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space, corners, typography } from '../../styles/viderse.stylex'
import type { ChannelJobs } from '../../api/channelVideos'
import type { Resource } from '../../data/useResource'
import { ActionButton } from '../../components'
import { t } from '../../i18n/channels'
import { paths } from '../../router'
import { fmtDate } from './fmt'

export function ChannelVideos({ channelId, resource }: { channelId: string; resource: Resource<ChannelJobs> }) {
  const tt = t.channels.videos
  if (resource.initialLoading || resource.placeholder) return <Skeleton height={80} />
  if (!resource.data) return <EmptyState variant="frame" icon="video-play" title={tt.loadFailed} hint={tt.retryHint}
    action={<ActionButton onClick={() => { void resource.refresh() }}>{tt.retry}</ActionButton>} />
  if (!resource.data.jobs.length) return <EmptyState variant="frame" icon="video-play" title={tt.empty} hint={tt.emptyHint} />
  return <section aria-label={tt.title}>
    {!!resource.error && <p role="status" {...stylex.props(sx.notice)}>{tt.stale}
      <ActionButton onClick={() => { void resource.refresh() }}>{tt.retry}</ActionButton>
    </p>}
    <ul {...stylex.props(sx.list)}>
      {resource.data.jobs.map((job) => <li key={job.id} {...stylex.props(sx.item)}>
        <a href={`#${paths.workbench({ channel: channelId, video: job.id })}`} {...stylex.props(sx.row)}>
          {job.source.thumbnailUrl
            ? <img src={job.source.thumbnailUrl} alt="" loading="lazy" {...stylex.props(sx.thumb)} />
            : <span aria-hidden="true" {...stylex.props(sx.thumb)} />}
          <div {...stylex.props(sx.details)}>
            <span {...stylex.props(sx.title)}>{job.source.title || tt.untitled}</span>
            <span {...stylex.props(sx.meta)}>{tt.states[job.stageGroup] ?? tt.otherState}</span>
            <time dateTime={job.updatedAt} {...stylex.props(sx.meta)}>{tt.updated(fmtDate(job.updatedAt))}</time>
          </div>
          <span aria-hidden="true" {...stylex.props(sx.arrow)}>›</span>
        </a>
      </li>)}
    </ul>
  </section>
}

const sx = stylex.create({
  list: { margin: 0, padding: 0, listStyleType: 'none', borderRadius: corners['--vv-r-12'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], overflow: 'hidden' },
  item: { borderBottomWidth: { default: space['--vv-s-1'], ':last-child': 0 }, borderBottomStyle: 'solid', borderBottomColor: line['--line-1'] },
  row: { display: 'flex', alignItems: 'center', gap: space['--vv-s-12'], padding: space['--vv-s-12'], minHeight: space['--vv-s-80'], color: color['--color-fg'], textDecorationLine: 'none', backgroundColor: { default: 'transparent', ':hover': 'var(--vv-hover-row)' } },
  thumb: { width: space['--vv-s-80'], aspectRatio: '16 / 9', flexShrink: 0, objectFit: 'cover', borderRadius: corners['--vv-r-6'], backgroundColor: color['--color-surface-5'] },
  details: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-4'], minWidth: 0, flex: 1 },
  title: { fontSize: typography['--vv-fs-body'], fontWeight: 600, overflow: 'hidden', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflowWrap: 'anywhere' },
  meta: { color: 'var(--vv-fg-note)', fontSize: typography['--vv-fs-meta'] },
  arrow: { color: 'var(--vv-fg-note)', fontSize: space['--vv-s-18'], flexShrink: 0 },
  notice: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: space['--vv-s-12'], fontSize: typography['--vv-fs-ui'], color: 'var(--vv-fg-note)' },
})
