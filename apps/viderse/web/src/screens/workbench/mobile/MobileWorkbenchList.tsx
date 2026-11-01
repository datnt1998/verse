// Danh sách Bàn dựng điện thoại (artboard MMain › screen=list): một cột H1 +
// tiến độ ngày, thẻ Bản tin, đề xuất ngang, dải kênh và video thật theo nhóm.
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, MeterBar, StatusPill } from '@noverse/ui'
import type { CloneJobView, RecapData } from '../../../api/workbench'
import type { ChannelSummary } from '../../../api/workbenchChannels'
import { ChannelAvatar } from '../../../components/ChannelAvatar'
import { SuggestionCard } from '../../../components/suggestions/SuggestionCard'
import { useSuggestions } from '../../../components/suggestions/useSuggestions'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, paths } from '../../../router'
import { groupJobs } from '../workbench-logic'
import { dayProgress, durationLabel, groupLabel, jobStatusLine, recapWindow, runningProgress, scopeName, stageToneOf } from './mworkbench-logic'
import { mw } from './mworkbench.stylex'

const c = screenWorkbench.mobile

export function MobileWorkbenchList({ scope, jobs, queuedTotal, channels, channelsLoading, listLoading, listError, recap, recapLoading, recapError, onRetry, onPickScope, onOpenJob, onOpenRecap }: {
  scope: string
  jobs: CloneJobView[]
  queuedTotal: number
  channels: ChannelSummary[]
  channelsLoading: boolean
  listLoading: boolean
  listError: unknown
  recap: RecapData | undefined
  recapLoading: boolean
  recapError: unknown
  onRetry: () => void
  onPickScope: (channel?: string) => void
  onOpenJob: (id: string) => void
  onOpenRecap: () => void
}) {
  const progress = dayProgress(channels, scope)
  const allProgress = scope === 'all' ? progress : dayProgress(channels, 'all')
  const groups = groupJobs(jobs, scope === 'all')
  const suggestions = useSuggestions(scope)
  const ready = channels.reduce((sum, channel) => sum + channel.ready, 0)
  const waiting = channels.reduce((sum, channel) => sum + channel.waitingYou, 0)
  const channel = channels.find((item) => item.channelId === scope)

  if (listLoading) return <div role="status" {...stylex.props(mw.state)}>Đang tải Bàn dựng…</div>
  if (listError != null && jobs.length === 0) return <div role="alert" {...stylex.props(mw.state)}>
    <span {...stylex.props(mw.stateTitle)}>{c.loadError}</span>
    <Button size="lg" variant="ghost" onClick={onRetry}>{c.retry}</Button>
  </div>

  return <div {...stylex.props(mw.listRoot)}>
    <div>
      <div {...stylex.props(mw.headRow)}>
        <h1 {...stylex.props(mw.h1)}>Bàn dựng</h1>
        <Button size="sm" variant="quiet" onClick={onOpenRecap}>{c.recapCta}</Button>
      </div>
      <div {...stylex.props(mw.dayBlock)}>
        <span {...stylex.props(mw.dayLabel)}>{channelsLoading ? 'Đang tải tiến độ…' : <><b {...stylex.props(mw.dayCount)}>{progress.done}/{progress.cap}</b> video hôm nay</>}</span>
        <MeterBar xstyle={mw.dayMeter} value={progress.pct} tone={progress.cap > 0 && progress.done >= progress.cap ? 'ok' : 'accent'} />
        <span {...stylex.props(mw.daySub)}>{channelsLoading ? scopeName(channels, scope) : progress.autoLine}</span>
      </div>
    </div>

    <button type="button" {...stylex.props(mw.recapCard)} onClick={onOpenRecap}>
      <span {...stylex.props(mw.recapTop)}>
        <span {...stylex.props(mw.recapTitle)}>{c.recapTitle}</span>
        <span {...stylex.props(mw.recapWhen)}>{recap ? recapWindow(recap.from, recap.to) : ''}</span>
      </span>
      <span {...stylex.props(mw.recapSummary)}>{recapLoading ? 'Đang tải Bản tin…' : recap ? recap.summary || c.recapLead(ready, waiting) : recapError ? 'Không tải được Bản tin.' : c.recapEmpty}</span>
      <span {...stylex.props(mw.recapCta)}>{c.recapCta} ›</span>
    </button>

    <section aria-label={c.suggestions} {...stylex.props(mw.section)}>
      <div {...stylex.props(mw.sectionHead)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{c.suggestions}</h2>
        <span {...stylex.props(mw.sectionCount)}>{suggestions.list.data?.total ?? 0}</span>
      </div>
      {suggestions.list.initialLoading ? <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>
        : suggestions.list.error != null && !suggestions.list.data ? <div role="alert" {...stylex.props(mw.notice)}>{c.suggestionsError} <Button size="sm" variant="quiet" onClick={() => void suggestions.list.refresh()}>{c.retry}</Button></div>
        : !suggestions.list.data?.suggestions.length ? <p {...stylex.props(mw.notice)}>{c.suggestionsEmpty}</p>
        : <div {...stylex.props(mw.rail)}>
          {suggestions.list.data.suggestions.slice(0, 5).map((suggestion) => <div key={suggestion.id} {...stylex.props(mw.railItem, mw.suggestCard)}>
            <SuggestionCard suggestion={suggestion} busy={suggestions.busy === suggestion.id} offline={!suggestions.online}
              notice={suggestions.notice?.id === suggestion.id ? suggestions.notice.text : undefined}
              onAccept={() => void suggestions.accept(suggestion)} onDismiss={() => void suggestions.dismiss(suggestion)} />
          </div>)}
        </div>}
    </section>

    <section aria-label={c.channels} {...stylex.props(mw.section)}>
      <div {...stylex.props(mw.sectionHead)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{c.channels}</h2>
        <span {...stylex.props(mw.sectionCount)}>{channels.length}</span>
      </div>
      <div {...stylex.props(mw.rail)}>
        <button type="button" {...stylex.props(mw.channelCard, scope === 'all' && mw.channelCardOn)} onClick={() => onPickScope()} aria-pressed={scope === 'all'}>
          <span {...stylex.props(mw.channelTop)}>
            <span {...stylex.props(mw.allStack)} aria-hidden="true">
              {channels.slice(0, 3).map((item, index) => <ChannelAvatar key={item.channelId} name={item.name} initials={item.initials.slice(0, 1)} size={26} stacked ringWidth={2}
                color={{ bg: item.color, ring: item.color }} className={index === 0 ? stylex.props(mw.allStackFirst).className : stylex.props(mw.allStackItem).className} />)}
              {channels.length === 0 && <ChannelAvatar name={c.allChannels} initials="∅" size={26} />}
            </span>
            <span {...stylex.props(mw.channelName)}>{c.allChannels}</span>
          </span>
          <span {...stylex.props(mw.channelMeta)}>{channelsLoading ? 'Đang tải…' : allProgress.label}</span>
        </button>
        {channels.map((item) => <button key={item.channelId} type="button" aria-pressed={scope === item.channelId}
          {...stylex.props(mw.channelCard, scope === item.channelId && mw.channelCardOn)} onClick={() => onPickScope(item.channelId)}>
          <span {...stylex.props(mw.channelTop)}>
            <ChannelAvatar name={item.name} initials={item.initials} src={item.avatarUrl ?? undefined} size={26} color={{ bg: item.color, ring: item.color }} />
            <span {...stylex.props(mw.channelToday)}>{item.today.done}/{item.today.cap}</span>
            {item.templateIssue && <span title={item.templateIssue.text}><Icon name="danger" size={15} label={item.templateIssue.text} xstyle={mw.channelWarn} /></span>}
          </span>
          <span title={item.name} {...stylex.props(mw.channelName)}>{item.name}</span>
          <span {...stylex.props(mw.channelBar)}><span {...stylex.props(mw.channelBarFill(Math.min(1, item.today.cap > 0 ? item.today.done / item.today.cap : 0) * 100))} /></span>
          <span {...stylex.props(mw.channelMeta)}>{item.waitingYou} chờ bạn</span>
        </button>)}
      </div>
    </section>

    {jobs.length === 0 && <div {...stylex.props(mw.state)}>
      <span {...stylex.props(mw.stateTitle)}>{c.emptyTitle}</span>
      {screenWorkbench.emptyHint}
      <div {...stylex.props(mw.emptyActions)}><Button size="lg" variant="primary" onClick={() => navigate(paths.new(channel?.channelId))}>{c.emptyAction}</Button></div>
    </div>}

    {groups.map((group) => <section key={group.key} aria-label={groupLabel[group.key]} {...stylex.props(mw.section)}>
      <div {...stylex.props(mw.sectionHead)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{groupLabel[group.key]}</h2>
        <span {...stylex.props(mw.sectionCount)}>{group.count}</span>
      </div>
      <div {...stylex.props(mw.jobList)}>
        {group.items.map((job) => <JobRow key={job.id} job={job} scope={scope} channel={channels.find((item) => item.channelId === job.channelId)} onOpen={() => onOpenJob(job.id)} />)}
      </div>
      {group.key === 'queued' && queuedTotal > group.items.length && <a {...stylex.props(mw.moreLink)} href="#/queue">{c.queueAll(queuedTotal)} ›</a>}
      {group.key === 'posted' && scope !== 'all' && <a {...stylex.props(mw.moreLink)} href={`#/channels/${encodeURIComponent(scope)}`}>{c.channelPage} ›</a>}
    </section>)}

    <div {...stylex.props(mw.actionRow)}>
      {queuedTotal > 0 && <a {...stylex.props(mw.moreLink)} href="#/queue">{c.queueShared(queuedTotal)} ›</a>}
      {scope !== 'all' && <a {...stylex.props(mw.moreLink)} href={`#/channels/${encodeURIComponent(scope)}`}>{c.channelPage} ›</a>}
    </div>
  </div>
}

function JobRow({ job, scope, channel, onOpen }: { job: CloneJobView; scope: string; channel?: ChannelSummary; onOpen: () => void }) {
  const progress = runningProgress(job)
  const agent = job.agentView?.find((item) => item.state === 'running')
  return <div {...stylex.props(mw.jobWrap)}>
    <button type="button" {...stylex.props(mw.jobMain)} onClick={onOpen}>
      <span {...stylex.props(mw.jobThumb)} aria-hidden="true">
        {job.source.thumbnailUrl && <img src={job.source.thumbnailUrl} alt="" loading="lazy" {...stylex.props(mw.jobThumbImg)} />}
      </span>
      <span {...stylex.props(mw.jobText)}>
        <span title={job.source.title} {...stylex.props(mw.jobTitle)}>{job.source.title}</span>
        <span {...stylex.props(mw.jobStatus)}>
          <StatusPill status={stageToneOf(job.stageGroup)}>{jobStatusLine(job)}</StatusPill>
          {scope === 'all' && channel && <span>· {channel.name}</span>}
          {durationLabel(job.source.durationSec) && <span>· {durationLabel(job.source.durationSec)}</span>}
        </span>
        {progress && <MeterBar value={progress.pct} tone="info" readout={`${progress.done}/${progress.total}`} />}
        {progress && agent?.now && <span {...stylex.props(mw.jobStatus)}>{agent.now}</span>}
      </span>
    </button>
    {job.stageGroup === 'ready'
      ? <a href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`} aria-label={c.download} {...stylex.props(mw.jobAction)}>↓</a>
      : <span aria-hidden="true" {...stylex.props(mw.jobAction)}>›</span>}
  </div>
}
