import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, ListRow, Segmented, Sheet, toast } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { AdminReadOnly } from '../../components/AdminReadOnly'
import { assistantName, type Assistant } from '../../components/agents'
import type { QueueImageQuota, QueueQueuedItem } from '../../api/clone'
import { t } from '../../i18n/queue'
import { navigate, paths, useRoute } from '../../router'
import { channelTint } from '../../lib/channelTint'
import { fmtDuration } from '../explore/fmt'
import { fmtEta, quotaLine, quotaName, viewState, waitLine } from './queueView'
import type { QueueController } from './useQueue'
import { mobileQueueStyles as sx } from './queueMobile.stylex'

const tt = t.screens.queue
const m = tt.mobile
const STEPS = m.steps
const agentOf = (value: string): Assistant => value === 'director' || value === 'artist' || value === 'qa' ? value : 'scout'
export function QueueThumbnail({ url }: { url?: string | null }) {
  return <span {...stylex.props(sx.thumb)} aria-hidden="true">{url && <img {...stylex.props(sx.image)} src={url} alt="" />}</span>
}
export function QueueAvatar({ channelId, queue, size = 20 }: { channelId: string; queue: QueueController; size?: number }) {
  const channel = queue.channels.get(channelId)
  const tint = channelTint(channelId)
  return <ChannelAvatar name={channel?.name || channelId} size={size} color={{ bg: tint, ring: tint }} />
}
export function QueueBanner({ queue }: { queue: QueueController }) {
  const { view, online, res } = queue
  if (!view) return null
  const quota = quotaLine(view.imageQuota)
  const warning = !online || (!view.paused && !!quota)
  const text = !online ? m.offline(res.updatedAt !== null ? fmtEta(new Date(res.updatedAt).toISOString()) : '—')
    : view.paused ? m.paused(view.pausedAt ? fmtEta(view.pausedAt) : null)
      : quota ? `${quota.title} ${quota.detail}` : null
  if (!text) return null
  return <div role="status" {...stylex.props(sx.banner, warning && sx.warningBanner)}><span {...stylex.props(sx.bannerIcon)}><Icon name={warning ? 'danger' : 'pause'} size={16} /></span><span>{text}</span></div>
}
export function QueueIdle({ summary = false, onNavigate }: { summary?: boolean; onNavigate?: () => void }) {
  return <div {...stylex.props(sx.idle)}><span {...stylex.props(sx.idleOrb)} aria-hidden="true"><span {...stylex.props(sx.idleDot)} /></span><span {...stylex.props(sx.idleTitle)}>{m.idle}</span><span {...stylex.props(sx.idleHint)}>{summary ? m.idleSummary : m.idleHint}</span><a href={`#${paths.new()}`} onClick={onNavigate} {...stylex.props(sx.link)}><Icon name="add" size={16} />{m.newVideo}</a></div>
}
export function QueueStepper({ label, value, max, disabled, unit, onChange }: { label: string; value: number; max: number; disabled: boolean; unit?: (value: number) => string; onChange: (value: number) => void }) {
  return <div role="group" aria-label={label} {...stylex.props(sx.stepper)}>
    <Button variant="icon" size="lg" aria-label={`${label} giảm`} disabled={disabled || value <= 1} onClick={() => onChange(value - 1)}><Icon name="minus" size={16} /></Button>
    <span aria-live="polite" {...stylex.props(sx.stepValue)}>{unit ? unit(value) : value}</span>
    <Button variant="icon" size="lg" aria-label={`${label} tăng`} disabled={disabled || value >= max} onClick={() => onChange(value + 1)}><Icon name="add" size={16} /></Button>
  </div>
}
export function QueueRulesSheet({ open, onClose, queue }: { open: boolean; onClose: () => void; queue: QueueController }) {
  const { view, busy, online, patch } = queue
  if (!view) return null
  const disabled = busy || !online
  return <Sheet open={open} onClose={onClose} label={tt.rulesTitle} size="md" height="auto" xstyle={sx.sheet} bodyStyle={sx.sheetBody}>
    <div {...stylex.props(sx.sheetHeading)}><h2 {...stylex.props(sx.sheetTitle)}>{tt.rulesTitle}</h2><span {...stylex.props(sx.sub)}>{m.rulesSubtitle}</span></div>
    <AdminReadOnly><div {...stylex.props(sx.rulesBody)}>
      <div {...stylex.props(sx.rule)}><span {...stylex.props(sx.grow, sx.stack)}><span {...stylex.props(sx.ruleTitle)}>{tt.rules.lanes}</span><span {...stylex.props(sx.sub)}>{m.workerBounds}</span></span><QueueStepper label={tt.rules.lanesTitle} value={view.workers.total} max={6} disabled={disabled} onChange={(workers) => void patch({ workers })} /></div>
      <p {...stylex.props(sx.ruleNote)}>{tt.workerGuidance}</p><div {...stylex.props(sx.divider)} />
      <div {...stylex.props(sx.rule)}><span {...stylex.props(sx.grow, sx.stack)}><span {...stylex.props(sx.ruleTitle)}>{tt.rules.imageSlots}</span><span {...stylex.props(sx.sub)}>{m.imageBounds}</span></span><QueueStepper label={tt.rules.imageSlotsTitle} value={view.imageWorkers} max={8} disabled={disabled} onChange={(imageWorkers) => void patch({ imageWorkers })} /></div><div {...stylex.props(sx.divider)} />
      <div {...stylex.props(sx.rule)}><span {...stylex.props(sx.grow, sx.stack)}><span {...stylex.props(sx.ruleTitle)}>{tt.rules.rescan}</span><span {...stylex.props(sx.sub)}>{m.rescanBounds}</span></span><QueueStepper label={tt.rules.rescanTitle} value={view.rescanHours} max={24} unit={tt.rules.rescanUnit} disabled={disabled} onChange={(rescanHours) => void patch({ rescanHours })} /></div>
      <p {...stylex.props(sx.ruleFooter)}>{m.rulesNote}</p>
    </div></AdminReadOnly><div {...stylex.props(sx.sheetFooter)}><Button variant="primary" size="xl" block onClick={onClose}>{m.done}</Button></div>
  </Sheet>
}
function Quota({ quota }: { quota: QueueImageQuota }) {
  const limited = quota.limit !== null && quota.used >= quota.limit
  return <div {...stylex.props(sx.quotaRow)}><div {...stylex.props(sx.row)}><span>{quotaName(quota)}</span><span {...stylex.props(sx.small)}>{quota.role === 'main' ? tt.quotaMain : tt.quotaFallback}</span><span {...stylex.props(sx.grow)} /><span {...stylex.props(limited && sx.warning)}>{m.quotaValue(quota.used, quota.limit)}</span></div><div role="img" aria-label={`${quotaName(quota)}: ${m.quotaValue(quota.used, quota.limit)}`} {...stylex.props(sx.bar)}><span {...stylex.props(sx.fill(quota.limit ? Math.min(100, quota.used / quota.limit * 100) : 0), limited && sx.warningFill)} /></div>{quota.resetsAt && <span {...stylex.props(sx.small)}>{tt.rules.quotaReset(fmtEta(quota.resetsAt))}</span>}</div>
}
export function MobileQueue({ queue }: { queue: QueueController }) {
  const { view, items, busy, online, patch, res } = queue
  const { route } = useRoute()
  const [filter, setFilter] = useState('all')
  const [rulesOpen, setRulesOpen] = useState(route.name === 'queue' && route.sheet === 'rules')
  const [selected, setSelected] = useState<QueueQueuedItem | null>(null)
  const [removed, setRemoved] = useState<string[]>([])
  const focused = useRef<string | null>(null)
  const itemId = route.name === 'queue' ? route.item : undefined
  useEffect(() => { if (route.name === 'queue' && route.sheet === 'rules') setRulesOpen(true) }, [route.name, route.name === 'queue' ? route.sheet : undefined])
  useEffect(() => {
    if (!itemId || focused.current === itemId || !items.some((item) => item.id === itemId)) return
    setFilter('all')
    const frame = requestAnimationFrame(() => {
      const element = document.getElementById(`queue-item-${itemId}`)
      if (!element) return
      element.scrollIntoView?.({ block: 'center' }); element.focus(); focused.current = itemId
    })
    return () => cancelAnimationFrame(frame)
  }, [itemId, items])
  const closeRules = () => { setRulesOpen(false); if (route.name === 'queue' && route.sheet) navigate(paths.queue({ item: route.item }), true) }
  if (!view) return <main {...stylex.props(sx.content)}><h1 {...stylex.props(sx.heading)}>{tt.title}</h1><span role="status">{res.status === 'error' ? tt.fail.title : t.common.loading}</span>{res.status === 'error' && <Button size="lg" onClick={() => void res.refresh().catch(() => toast.err(tt.fail.title))}>{tt.fail.retry}</Button>}</main>
  const state = viewState(view)
  const empty = state === 'empty'
  const shown = filter === 'all' ? items : items.filter((item) => item.channelId === filter)
  const filters = [...new Set(items.map((item) => item.channelId))]
  const remove = async (item: QueueQueuedItem) => {
    setSelected(null)
    if (await patch({ id: item.id, action: 'remove' })) setRemoved((ids) => [...ids, item.id])
  }
  const undo = async () => {
    for (const id of removed) {
      if (await patch({ id, action: 'restore' })) setRemoved((ids) => ids.filter((value) => value !== id))
      else break
    }
  }
  return <div className={`mq ${stylex.props(sx.screen).className}`} data-state={state}>
    <header {...stylex.props(sx.header)}>
      <a href={`#${paths.workbench()}`} aria-label={m.back} {...stylex.props(sx.back)}><span {...stylex.props(sx.backIcon)}><Icon name="arrow-r" size={18} /></span></a>
      <span {...stylex.props(sx.grow, sx.stack)}>
        <h1 {...stylex.props(sx.heading)}>{tt.title}</h1>
        <span {...stylex.props(sx.subLine)}>
          {!online ? <span aria-hidden="true" {...stylex.props(sx.statusDot)} /> : view.paused ? <Icon name="pause" size={13} xstyle={sx.statusPause} /> : <span aria-hidden="true" {...stylex.props(sx.statusDot, empty ? sx.statusDotMuted : state === 'limited' && sx.statusDotWarn)} />}
          <span {...stylex.props(sx.ellipsis)}>{!online ? m.offlineSub : empty ? m.idle : view.paused ? m.pausedSub(items.length) : tt.sheetSub(view.running.length, items.length)}</span>
        </span>
      </span>
      {!empty && <Button size="lg" disabled={busy || !online} aria-pressed={view.paused} onClick={() => void patch({ paused: !view.paused })} icon={<Icon name={view.paused ? 'play' : 'pause'} size={16} />}>{view.paused ? tt.resume : tt.pause}</Button>}
    </header>
    <main {...stylex.props(sx.content)}><QueueBanner queue={queue} />{empty && <QueueIdle />}
      {!empty && <><section {...stylex.props(sx.group)}><h2 {...stylex.props(sx.section)}>{tt.runningNow}<span {...stylex.props(sx.count)}>{view.running.length}/{view.workers.total} {tt.lanes}</span></h2>{view.running.map((job) => {
        const agent = agentOf(job.agent), step = Math.min(8, Math.max(0, job.stepIndex)), limited = state === 'limited' && step === 5
        return <a key={job.jobId} className={`mq-run ${stylex.props(sx.card, limited && sx.warningBorder).className}`} href={`#${paths.workbench({ video: job.jobId })}`}>
          <span {...stylex.props(sx.row)}>
            <QueueThumbnail url={job.thumbnailUrl} />
            <span {...stylex.props(sx.grow, sx.stack)}>
              <span title={job.title} {...stylex.props(sx.title)}>{job.title}</span>
              <span title={job.now} {...stylex.props(sx.sub, sx.ellipsis, limited && sx.warning)}>
                <span {...stylex.props(sx[agent])}>{assistantName(agent)}</span> · {job.now}
              </span>
            </span>
            <span {...stylex.props(sx.small)}>{job.etaMinutes !== null ? m.eta(job.etaMinutes) : ''}</span>
          </span>
          <span role="img" aria-label={m.step(step, STEPS[step])} {...stylex.props(sx.segments)}>
            {STEPS.map((label, index) => <span key={label} {...stylex.props(sx.segment, index < step && sx.segmentDone, index === step && (agent === 'scout' ? sx.segmentScout : agent === 'director' ? sx.segmentDirector : agent === 'artist' ? sx.segmentArtist : sx.segmentQa), index === step && limited && sx.warningFill)} />)}
          </span>
          <span {...stylex.props(sx.meta)}>
            <QueueAvatar queue={queue} channelId={job.channelId} size={18} />
            <span {...stylex.props(sx.grow, sx.ellipsis)}>{job.templateName || queue.channels.get(job.channelId)?.name} · {fmtDuration(job.durationSec)}</span>
            <span>{m.step(step, STEPS[step])}</span>
          </span>
        </a>
      })}<span {...stylex.props(sx.hint)}>{state === 'paused' ? m.pausedHint : state === 'limited' ? m.limitedHint : m.runHint}</span></section>
      <section {...stylex.props(sx.group)}>
        <h2 {...stylex.props(sx.section)}>{tt.nextRotation}<span {...stylex.props(sx.count)}>{items.length}</span></h2>
        <Segmented label={tt.filterAria} value={filter} onChange={setFilter} size="lg" fit="content"
          options={['all', ...filters].map((id) => ({
            id, label: <>{id === 'all' ? tt.filterAll : queue.channels.get(id)?.name || id} <span {...stylex.props(sx.small)}>{id === 'all' ? items.length : items.filter((item) => item.channelId === id).length}</span></>,
          }))} />
        <div>
          {shown.map((item) => <div key={item.id} id={`queue-item-${item.id}`} tabIndex={-1} className={`mq-row ${stylex.props(sx.queueRow, item.id === itemId && sx.highlighted).className}`}>
            <span {...stylex.props(sx.number)}>{items.indexOf(item) + 1}</span>
            <QueueThumbnail url={item.thumbnailUrl} />
            <span {...stylex.props(sx.grow, sx.stack, sx.queuedStack)}>
              <span title={item.title} {...stylex.props(sx.ellipsis, sx.queuedTitle)}>{item.title || item.source}</span>
              <span {...stylex.props(sx.meta)}>
                <QueueAvatar queue={queue} channelId={item.channelId} size={16} />
                <span {...stylex.props(sx.ellipsis)}>{item.priority === 'pick' ? `${tt.priority} · ` : ''}{item.templateName} · {fmtDuration(item.durationSec)}</span>
              </span>
              <span title={waitLine(item, view.paused)} {...stylex.props(sx.small, sx.ellipsis)}>
                {!view.paused && item.etaStart ? `${fmtEta(item.etaStart)} · ` : ''}{waitLine(item, view.paused)}
              </span>
            </span>
            <Button variant="icon" size="lg" aria-label={m.rowAction(item.title)} aria-haspopup="dialog" onClick={() => setSelected(item)}><Icon name="more" size={18} /></Button>
          </div>)}
          {shown.length === 0 && <span {...stylex.props(sx.hint)}>{tt.noRows}</span>}
        </div>
      </section></>}
      {removed.length > 0 && <div role="status" {...stylex.props(sx.undoBanner)}><span {...stylex.props(sx.grow)}>{m.removed(removed.length)}</span><Button variant="quiet" size="lg" disabled={busy || !online} onClick={() => void undo()}>{t.common.undo}</Button></div>}
      <ListRow
        aria-label={tt.rulesTitle}
        aria-haspopup="dialog"
        icon="setting"
        title={tt.rulesTitle}
        hint={m.rulesSummary(view.workers.total, view.imageWorkers, view.rescanHours)}
        chevron
        onClick={() => setRulesOpen(true)}
        xstyle={[sx.card, sx.row, sx.rulesLink]}
      />
      {view.imageQuota.length > 0 && <section {...stylex.props(sx.card, sx.panel, state === 'limited' && sx.warningBorder)}><div {...stylex.props(sx.row)}><h2 {...stylex.props(sx.panelTitle)}>{tt.rules.quota}</h2><a {...stylex.props(sx.link, sx.panelLink)} href={`#${paths.settings('agents')}`}>{tt.quotaSettings}</a></div>{view.imageQuota.map((quota) => <Quota key={`${quota.provider}-${quota.role}`} quota={quota} />)}</section>}
      <section {...stylex.props(sx.card, sx.panel)}><div {...stylex.props(sx.row)}><h2 {...stylex.props(sx.panelTitle)}>{tt.rules.pace}</h2>{view.pace[0] && <a {...stylex.props(sx.link, sx.panelLink)} href={`#${paths.channel(view.pace[0].channelId, 'sources')}`}>{tt.paceEdit}</a>}</div>{view.pace.map((pace) => <div key={pace.channelId} {...stylex.props(sx.quotaRow)}><a {...stylex.props(sx.row)} href={`#${paths.channel(pace.channelId, 'sources')}`}><QueueAvatar queue={queue} channelId={pace.channelId} /><span {...stylex.props(sx.grow, sx.ellipsis)}>{queue.channels.get(pace.channelId)?.name || pace.channelId}</span><span>{pace.used}/{pace.limit}</span></a><div {...stylex.props(sx.bar)} role="img" aria-label={`${queue.channels.get(pace.channelId)?.name}: ${pace.used}/${pace.limit}`}><span {...stylex.props(sx.fill(pace.limit ? Math.min(100, pace.used / pace.limit * 100) : 0))} /></div></div>)}<span {...stylex.props(sx.small)}>{tt.paceNote}</span></section>
      {view.doneToday.length > 0 && <section {...stylex.props(sx.group)}><h2 {...stylex.props(sx.section)}>{tt.doneHeading(view.doneToday.length)}</h2>{view.doneToday.map((job) => <a key={job.jobId} {...stylex.props(sx.doneRow)} href={`#${paths.workbench({ video: job.jobId })}`}><span aria-hidden="true" {...stylex.props(sx.doneDot, job.state === 'review' && sx.reviewDot)} /><span title={job.title} {...stylex.props(sx.grow, sx.ellipsis)}>{job.title}</span><span {...stylex.props(sx.small)}>{job.state === 'review' ? tt.status.review : fmtEta(job.at)}</span></a>)}</section>}
    </main>
    <QueueRulesSheet open={rulesOpen} onClose={closeRules} queue={queue} />
    <Sheet open={selected !== null} onClose={() => setSelected(null)} label={m.rowTitle} size="md" height="auto" xstyle={sx.sheet} bodyStyle={sx.sheetBody}>
      {selected && <>
        <div {...stylex.props(sx.rowHeading)}>
          <QueueThumbnail url={selected.thumbnailUrl} />
          <span {...stylex.props(sx.grow, sx.stack)}>
            <span {...stylex.props(sx.rowSheetTitle)}>{selected.title}</span>
            <span {...stylex.props(sx.sub, sx.ellipsis)}>{m.rowSub(queue.channels.get(selected.channelId)?.name || selected.channelId, items.findIndex((item) => item.id === selected.id) + 1)}</span>
          </span>
        </div>
        <div {...stylex.props(sx.actions)}>
          <Button size="xl" block disabled={busy || !online || items[0]?.id === selected.id} aria-describedby="queue-front-reason" onClick={() => { const id = selected.id; setSelected(null); void patch({ id, action: 'front' }) }} icon={<Icon name="arrow-up" size={20} />}>
            {m.front}
          </Button>
          <span id="queue-front-reason" {...stylex.props(sx.sub)}>{items[0]?.id === selected.id ? m.alreadyFront : m.frontHint}</span>
          {selected.sourceUrl && <a href={selected.sourceUrl} target="_blank" rel="noreferrer" onClick={() => setSelected(null)} {...stylex.props(sx.link)}>
            <Icon name="link" size={20} />{m.source}
          </a>}
          <Button variant="dsoft" size="xl" block disabled={busy || !online} title={m.removeHint} onClick={() => void remove(selected)} icon={<Icon name="trash" size={20} />}>
            {tt.removeItem}
          </Button>
          <span {...stylex.props(sx.sub)}>{m.removeHint}</span>
        </div>
      </>}
    </Sheet>
  </div>
}
