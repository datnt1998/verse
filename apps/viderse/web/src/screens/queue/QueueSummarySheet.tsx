import * as stylex from '@stylexjs/stylex'
import { Icon, Sheet } from '@noverse/ui'
import { Orb } from '../../components'
import type { Assistant } from '../../components/agents'
import { vi as t } from '../../i18n/vi'
import { paths } from '../../router'
import { fmtEta, quotaName, waitLine } from './queueView'
import { useQueue } from './useQueue'
import { QueueAvatar, QueueBanner, QueueIdle } from './MobileQueue'
import { mobileQueueStyles as sx } from './queueMobile.stylex'

const tt = t.screens.queue
const m = tt.mobile

/** MMain jobs sheet. All counts, reasons and mutations share the live queue cache. */
export function QueueSummarySheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  return open ? <OpenQueueSummary onClose={onClose} /> : null
}
function OpenQueueSummary({ onClose }: { onClose: () => void }) {
  const queue = useQueue()
  const { view, items, online, busy, patch, res } = queue
  const time = view?.pausedAt ? fmtEta(view.pausedAt) : null
  const empty = view && view.running.length === 0 && items.length === 0
  const channels = view ? new Set([...items.map((item) => item.channelId), ...view.running.map((job) => job.channelId)]).size : 0
  const sub = !online ? m.offlineSub : !view ? t.common.loading : view.paused ? m.pausedSummary(time, items.length) : empty ? m.idleSub : m.summarySub(view.running.length, items.length, channels)
  return <Sheet open onClose={onClose} label={tt.sheetTitle} size="md" height="auto" xstyle={sx.sheet} bodyStyle={sx.sheetBody}>
    <div {...stylex.props(sx.summaryHead)}><span {...stylex.props(sx.grow, sx.stack)}><h2 {...stylex.props(sx.sheetTitle)}>{tt.title}</h2><span {...stylex.props(sx.sub)}>{sub}</span></span>{view && !empty && <button type="button" {...stylex.props(sx.button, sx.summaryPause)} disabled={busy || !online} aria-disabled={busy || !online} aria-pressed={view.paused} onClick={() => void patch({ paused: !view.paused })}><Icon name={view.paused ? 'play' : 'pause'} size={16} />{view.paused ? tt.resume : tt.pause}</button>}</div>
    <div {...stylex.props(sx.summaryBody)}>
      <QueueBanner queue={queue} />
      {!view && <span role="status">{res.status === 'error' ? tt.fail.title : t.common.loading}</span>}
      {empty && <QueueIdle summary onNavigate={onClose} />}
      {view && !empty && <>
        {view.running.length > 0 && <section {...stylex.props(sx.group)}><h3 {...stylex.props(sx.section)}>{tt.runningNow}</h3>{view.running.map((job) => <a key={job.jobId} {...stylex.props(sx.summaryRun)} href={`#${paths.workbench({ video: job.jobId })}`} onClick={onClose}><Orb of={(['scout', 'director', 'artist', 'qa'].includes(job.agent) ? job.agent : 'scout') as Assistant} size={22} live={online && !view.paused} /><span {...stylex.props(sx.grow, sx.stack)}><span title={job.title} {...stylex.props(sx.ellipsis, sx.queuedTitle)}>{job.title}</span><span title={job.now} {...stylex.props(sx.ellipsis, sx.small)}>{job.now}</span></span><QueueAvatar channelId={job.channelId} queue={queue} size={26} /></a>)}</section>}
        {items.length > 0 && <section {...stylex.props(sx.group)}><h3 {...stylex.props(sx.section)}>{tt.nextRotation}</h3><div>{items.slice(0, 4).map((item, index) => <a key={item.id} {...stylex.props(sx.summaryRow)} href={`#${paths.queue({ item: item.id })}`} onClick={onClose}><span {...stylex.props(sx.number)}>{index + 1}</span><QueueAvatar channelId={item.channelId} queue={queue} size={26} /><span {...stylex.props(sx.grow, sx.stack)}><span title={item.title} {...stylex.props(sx.ellipsis, sx.queuedTitle)}>{item.title || item.source}</span><span title={waitLine(item, view.paused)} {...stylex.props(sx.ellipsis, sx.small)}>{waitLine(item, view.paused) || fmtEta(item.etaStart)}</span></span></a>)}</div></section>}
        <a {...stylex.props(sx.summaryAll)} href={`#${paths.queue()}`} onClick={onClose}>{m.openAll}<Icon name="arrow-r" size={14} /></a>
      </>}
      {view?.imageQuota.map((quota) => <div key={`${quota.provider}-${quota.role}`} {...stylex.props(sx.summaryQuota)}><Icon name="gallery" size={16} /><span {...stylex.props(sx.grow)}>{quotaName(quota)} {tt.sheetQuotaToday}</span><span>{quota.used}{quota.limit === null ? '' : `/${quota.limit}`}</span>{quota.resetsAt && <span {...stylex.props(sx.small)}>· {tt.rules.quotaReset(fmtEta(quota.resetsAt))}</span>}</div>)}
      {view && <div {...stylex.props(sx.row)}><span {...stylex.props(sx.grow, sx.sub)}>{tt.sheetRules(view.workers.total)}</span><a {...stylex.props(sx.link)} href={`#${paths.queue({ sheet: 'rules' })}`} onClick={onClose}>{tt.rulesTitle}</a></div>}
    </div>
  </Sheet>
}
