// Danh sách Bàn dựng điện thoại (artboard MMain › screen=list): một cột H1 +
// tiến độ ngày, đề xuất dạng dòng, dải kênh (chỉ ở Mọi kênh) và video thật chia
// đúng nhóm Chờ bạn / Đang làm / Sẵn sàng đăng / Chờ lượt / Đã đăng. Bản tin và
// thẻ Kế hoạch đã bỏ ở V37; bố cục điện thoại đầy đủ thuộc đợt 44.
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Shimmer } from '@noverse/ui'
import type { CloneJobView } from '../../../api/workbench'
import type { ChannelSummary } from '../../../api/workbenchChannels'
import { ChannelAvatar } from '../../../components/ChannelAvatar'
import { Orb } from '../../../components/Orb'
import { SuggestionRow } from '../../../components/suggestions/SuggestionRow'
import { useSuggestions } from '../../../components/suggestions/useSuggestions'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, paths, routePath } from '../../../router'
import { groupJobs } from '../workbench-logic'
import { dayProgress, durationLabel, groupLabel, jobStatusLine, runningProgress, scopeName } from './mworkbench-logic'
import { mw, mwList as l } from './mworkbench.stylex'

const c = screenWorkbench.mobile

export function MobileWorkbenchList({ scope, jobs, queuedTotal, channels, channelsLoading, listLoading, listError, onRetry, onPickScope, onOpenJob }: {
  scope: string
  jobs: CloneJobView[]
  queuedTotal: number
  channels: ChannelSummary[]
  channelsLoading: boolean
  listLoading: boolean
  listError: unknown
  onRetry: () => void
  onPickScope: (channel?: string) => void
  onOpenJob: (id: string) => void
}) {
  const progress = dayProgress(channels, scope)
  const groups = groupJobs(jobs, scope === 'all')
  const suggestions = useSuggestions(scope)
  const channel = channels.find((item) => item.channelId === scope)
  const waiting = groups.find(group => group.key === 'waiting')
  const suggestionRows = suggestions.list.data?.suggestions.slice(0, 3) ?? []

  if (listLoading) return <div role="status" {...stylex.props(mw.state)}>Đang tải Bàn dựng…</div>
  if (listError != null && jobs.length === 0) return <div role="alert" {...stylex.props(mw.state)}>
    <span {...stylex.props(mw.stateTitle)}>{c.loadError}</span>
    <Button size="lg" variant="ghost" onClick={onRetry}>{c.retry}</Button>
  </div>

  return <div {...stylex.props(mw.listRoot)}>
    <div {...stylex.props(l.headBlock)}>
      <h1 {...stylex.props(l.h1)}>Bàn dựng</h1>
      <div {...stylex.props(l.dayRow)}>
        <span {...stylex.props(l.dayLabel)}>{channelsLoading ? 'Đang tải tiến độ…' : <><b {...stylex.props(l.dayBold)}>{progress.done}</b>/{progress.cap} video hôm nay</>}</span>
        <span {...stylex.props(l.dayTrack)} aria-hidden="true"><span {...stylex.props(l.dayFill(progress.pct * 100))} /></span>
        <span {...stylex.props(l.dayAuto)}>{channelsLoading ? scopeName(channels, scope) : progress.autoLine}</span>
      </div>
    </div>

    {(waiting || suggestionRows.length > 0 || suggestions.list.initialLoading || suggestions.list.error != null) && <section aria-label="Chờ bạn" {...stylex.props(l.groupSection)}>
      <div {...stylex.props(l.groupHead)}>
        <h2 {...stylex.props(l.groupTitle)}>Chờ bạn</h2>
        <span {...stylex.props(l.groupCount)}>{(waiting?.count ?? 0) + suggestionRows.length}</span>
      </div>
      {waiting && <div role="list" aria-label="Video chờ bạn" {...stylex.props(mw.jobList)}>
        {waiting.items.map(job => <GroupRow key={job.id} group="waiting" job={job} channel={channels.find(item => item.channelId === job.channelId)} onOpen={() => onOpenJob(job.id)} />)}
      </div>}
      {suggestions.list.initialLoading ? <p role="status" {...stylex.props(mw.notice)}>Đang tải đề xuất…</p>
        : suggestions.list.error != null && !suggestions.list.data ? <div role="alert" {...stylex.props(mw.notice)}>{c.suggestionsError} <Button size="sm" variant="quiet" onClick={() => void suggestions.list.refresh()}>{c.retry}</Button></div>
        : suggestionRows.map(suggestion => <SuggestionRow key={suggestion.id} suggestion={suggestion} busy={suggestions.busy === suggestion.id} offline={!suggestions.online}
          notice={suggestions.notice?.id === suggestion.id ? suggestions.notice.text : undefined}
          onAccept={() => void suggestions.accept(suggestion)} onDismiss={() => void suggestions.dismiss(suggestion)} />)}
    </section>}

    {scope === 'all' && <section aria-label={c.channels} {...stylex.props(mw.section)}>
      <div {...stylex.props(mw.sectionHead)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{c.channels}</h2>
        <span {...stylex.props(mw.sectionCount)}>{channels.length}</span>
      </div>
      <div {...stylex.props(mw.rail)}>
        {channels.map((item) => <button key={item.channelId} type="button" aria-pressed={scope === item.channelId}
          aria-label={`${item.name} · hôm nay ${item.today.done}/${item.today.cap}${item.waitingYou ? ` · ${item.waitingYou} chờ bạn` : ''}${item.ready ? ` · ${item.ready} sẵn sàng đăng` : ''}${item.templateIssue ? ` · ${item.templateIssue.text}` : ''}`}
          {...stylex.props(mw.channelCard)} onClick={() => onPickScope(item.channelId)}>
          <span {...stylex.props(l.channelTop)}>
            <ChannelAvatar name={item.name} initials={item.initials} src={item.avatarUrl ?? undefined} size={26} color={{ bg: item.color, ring: item.color }} />
            <span {...stylex.props(l.channelToday)}>{item.today.done}/{item.today.cap} hôm nay</span>
            {item.templateIssue && <span title={item.templateIssue.text}><Icon name="danger" size={15} label={item.templateIssue.text} xstyle={mw.channelWarn} /></span>}
          </span>
          <span title={item.name} {...stylex.props(mw.channelName)}>{item.name}</span>
          <span aria-hidden="true" {...stylex.props(l.channelBarTrack)}><span {...stylex.props(l.channelBarFill(Math.min(1, item.today.cap > 0 ? item.today.done / item.today.cap : 0) * 100))} /></span>
          <span {...stylex.props(l.channelFacts)}>
            {item.waitingYou > 0 && <span {...stylex.props(l.factWaiting)}>{item.waitingYou} chờ bạn</span>}
            {item.ready > 0 && <span {...stylex.props(l.factReady)}>{item.ready} sẵn sàng đăng</span>}
            {!item.waitingYou && !item.ready && <span {...stylex.props(l.factNone)}>Không có việc chờ bạn</span>}
          </span>
        </button>)}
        {channels.length === 0 && <span {...stylex.props(mw.notice)}>{channelsLoading ? 'Đang tải…' : 'Chưa có kênh nào.'}</span>}
      </div>
    </section>}

    {jobs.length === 0 && <div {...stylex.props(mw.state)}>
      <span {...stylex.props(mw.stateTitle)}>{c.emptyTitle}</span>
      {screenWorkbench.emptyHint}
      <div {...stylex.props(mw.emptyActions)}><Button size="lg" variant="primary" onClick={() => navigate(paths.new(channel?.channelId))}>{c.emptyAction}</Button></div>
    </div>}

    {groups.filter(group => group.key !== 'waiting').map((group) => <section key={group.key} aria-label={groupLabel[group.key]} {...stylex.props(l.groupSection)}>
      <div {...stylex.props(l.groupHead)}>
        <h2 {...stylex.props(l.groupTitle)}>{groupLabel[group.key]}</h2>
        <span {...stylex.props(l.groupCount)}>{group.count}</span>
      </div>
      {group.key === 'working'
        ? <div role="list" aria-label={groupLabel[group.key]}>
          {group.items.map((job) => <div role="listitem" key={job.id}><WorkingCard job={job} channel={channels.find((item) => item.channelId === job.channelId)} scope={scope} onOpen={() => onOpenJob(job.id)} /></div>)}
        </div>
        : <div role="list" aria-label={groupLabel[group.key]} {...stylex.props(mw.jobList)}>
          {group.items.map((job) => <GroupRow key={job.id} group={group.key} job={job} channel={channels.find((item) => item.channelId === job.channelId)} onOpen={() => onOpenJob(job.id)} />)}
        </div>}
      {group.key === 'queued' && queuedTotal > group.items.length && <a {...stylex.props(l.fullButton)} href="#/queue">{c.queueAll(queuedTotal)}<Icon name="arrow-r" size={14} /></a>}
      {group.key === 'posted' && scope !== 'all' && <a {...stylex.props(l.quietLink)} href={`#/channels/${encodeURIComponent(scope)}`}>{c.channelPage}<Icon name="arrow-r" size={14} /></a>}
    </section>)}

    {scope === 'all' && queuedTotal > 0 && !groups.some((group) => group.key === 'queued') && <a {...stylex.props(l.fullButton)} href="#/queue">{c.queueAll(queuedTotal)}<Icon name="arrow-r" size={14} /></a>}
  </div>
}

function GroupRow({ group, job, channel, onOpen }: { group: string; job: CloneJobView; channel?: ChannelSummary; onOpen: () => void }) {
  const review = job.stageGroup === 'review'
  const issue = job.stageGroup === 'attention'
  const status = issue ? 'Cần xử lý' : review ? 'Chờ duyệt' : job.stageGroup === 'paused' ? 'Tạm dừng' : jobStatusLine(job)
  const statusStyle = issue ? l.rowStatusIssue : review ? l.rowStatusReview : job.stageGroup === 'ready' ? l.rowStatusReady : undefined

  if (group === 'queued') {
    return <div role="listitem" {...stylex.props(l.row)}>
      <button type="button" {...stylex.props(l.rowMain)} onClick={onOpen}>
        <span {...stylex.props(l.queueChannel)}>
          <ChannelAvatar name={channel?.name || `Kênh ${job.channelId}`} initials={channel?.initials || undefined} src={channel?.avatarUrl ?? undefined} size={20} color={channel ? { bg: channel.color, ring: channel.color } : undefined} />
        </span>
        <span {...stylex.props(l.rowText)}>
          <span title={job.source.title} {...stylex.props(l.rowTitle)}>{job.source.title}</span>
          <span {...stylex.props(l.rowStatus)}>{job.subline}</span>
        </span>
      </button>
      <Icon name="clock" size={16} xstyle={l.rowClock} />
    </div>
  }

  return <div role="listitem" {...stylex.props(l.row)}>
    <button type="button" {...stylex.props(l.rowMain)} onClick={onOpen}>
      <span aria-hidden="true" {...stylex.props(l.rowThumb)}>
        {job.source.thumbnailUrl ? <img src={job.source.thumbnailUrl} alt="" loading="lazy" {...stylex.props(l.rowThumbImg)} /> : null}
        <ChannelAvatar name={channel?.name || `Kênh ${job.channelId}`} initials={channel?.initials || undefined} src={channel?.avatarUrl ?? undefined} size={20} color={channel ? { bg: channel.color, ring: channel.color } : undefined} style={{ position: 'absolute', left: -4, bottom: -4, boxShadow: '0 0 0 2px var(--color-bg)' }} />
      </span>
      <span {...stylex.props(l.rowText)}>
        <span title={job.source.title} {...stylex.props(l.rowTitle, group === 'posted' && l.rowTitleMuted)}>{job.source.title}</span>
        <span {...stylex.props(l.rowStatus, statusStyle)}>{group === 'posted' ? job.subline : status}</span>
      </span>
    </button>
    {group === 'waiting' && (review
      ? <button type="button" {...stylex.props(l.rowAction, l.rowActionReview)} onClick={() => navigate(routePath({ name: 'review', video: job.id }))}>Duyệt</button>
      : <button type="button" {...stylex.props(l.rowAction, issue && l.rowActionIssue)} onClick={onOpen}>{issue ? 'Xử lý' : 'Mở'}</button>)}
    {group === 'ready' && <a href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`} aria-label={c.download} {...stylex.props(l.rowIconAction)}><Icon name="download" size={18} /></a>}
  </div>
}

function WorkingCard({ job, channel, scope, onOpen }: { job: CloneJobView; channel?: ChannelSummary; scope: string; onOpen: () => void }) {
  const progress = runningProgress(job)
  const agent = job.agentView?.find((item) => item.state === 'running')
  const orb = agent?.id === 'artist' ? 'artist' : agent?.id === 'director' ? 'director' : agent?.id === 'qa' ? 'qa' : 'scout'
  const ink = orb === 'artist' ? 'var(--vv-artist-ink)' : orb === 'director' ? 'var(--vv-director-ink)' : orb === 'qa' ? 'var(--vv-qa-ink)' : 'var(--vv-scout-ink)'
  const running = job.stageGroup === 'working' && job.status === 'running'
  const statusText = agent?.now ?? jobStatusLine(job)
  return <button type="button" {...stylex.props(l.workCard)} onClick={onOpen}>
    <span {...stylex.props(l.workTop)}>
      <span aria-hidden="true" {...stylex.props(l.workThumb)}>
        {job.source.thumbnailUrl ? <img src={job.source.thumbnailUrl} alt="" loading="lazy" {...stylex.props(l.rowThumbImg)} /> : null}
        {durationLabel(job.source.durationSec) && <span {...stylex.props(l.workDur)}>{durationLabel(job.source.durationSec)}</span>}
        <ChannelAvatar name={channel?.name || `Kênh ${job.channelId}`} initials={channel?.initials || undefined} src={channel?.avatarUrl ?? undefined} size={20} color={channel ? { bg: channel.color, ring: channel.color } : undefined} style={{ position: 'absolute', left: -4, bottom: -4, boxShadow: '0 0 0 2px var(--color-bg)' }} />
      </span>
      <span {...stylex.props(l.workText)}>
        <span {...stylex.props(l.workTitle)}>{job.source.title}</span>
        <span {...stylex.props(l.workSrc)}>{scope === 'all' && channel ? `${channel.name} · ` : ''}{job.source.channelTitle ?? job.templateName ?? ''}</span>
      </span>
    </span>
    <span {...stylex.props(l.workStepRow)}>
      <Orb of={orb} size={18} live={running} />
      <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, color: 'var(--color-fg-2)' }}>{running ? <Shimmer>{statusText}</Shimmer> : statusText}</span>
      {progress && <span {...stylex.props(l.workPct)}>{Math.round(progress.pct * 100)}%</span>}
    </span>
    {progress && <span aria-hidden="true" {...stylex.props(l.workTrack)}><span {...stylex.props(l.workFill(progress.pct * 100, ink))} /></span>}
  </button>
}
