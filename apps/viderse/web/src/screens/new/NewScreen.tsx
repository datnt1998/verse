import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Icon, toast } from '@noverse/ui'
import { ActionButton, LensLogo, Orb, type Assistant } from '../../components'
import { useResource } from '../../data/useResource'
import { useStatus } from '../../data/status'
import { keys } from '../../data/keys'
import { listChannels } from '../../api/channels'
import { listRivalVideos } from '../../api/rivals'
import { enqueueClone, planClone, type ClonePlan } from '../../api/clone'
import { vi as t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { useDebounced } from '../../lib/useDebounced'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { fmtDuration, fmtScore, videoMeta } from '../explore/fmt'
import { PLAN_DEBOUNCE_MS, parseUrls, pickPriority, planLine } from './plan'
import './new.css'

const tt = t.screens.new
const agents: Assistant[] = ['scout', 'director', 'artist', 'qa']

export function NewScreen() {
  const [text, setText] = useState('')
  const [mode, setMode] = useState<'single' | 'multi'>('single')
  const [channelId, setChannelId] = useState('')
  const [templateId, setTemplateId] = useState('')
  const [priority, setPriority] = useState(false)
  const [noImages, setNoImages] = useState(false)
  const [plan, setPlan] = useState<ClonePlan | null>(null)
  const [planKey, setPlanKey] = useState('')
  const [phase, setPhase] = useState<'idle' | 'loading' | 'failed'>('idle')
  const [retry, setRetry] = useState(0)
  const [busy, setBusy] = useState(false)
  const mobile = useMediaQuery('(max-width: 767px)')
  const debounced = useDebounced(text, PLAN_DEBOUNCE_MS)
  const urls = useMemo(() => parseUrls(text), [text])
  const channelsRes = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const status = useStatus()
  const channels = channelsRes.data?.channels ?? []
  const channel = channels.find((c) => c.id === channelId) ?? channels[0]
  const multi = mode === 'multi' || urls.length > 1
  const inputPriority = priority ? 'pick' : pickPriority(urls)
  const suggestions = useResource(
    ['new', 'picks', channel?.id ?? '', retry],
    ({ signal }) => listRivalVideos(channel?.sources[0]?.rivalId ?? 'all', { sort: 'outlier', limit: 3, notMadeFor: channel?.id }, signal),
    { enabled: !!channel },
  )

  useEffect(() => {
    const pending = parseUrls(debounced)
    if (!channel || pending.length === 0) { setPlan(null); setPhase('idle'); return }
    const ctrl = new AbortController()
    setPhase('loading')
    planClone({ channelId: channel.id, urls: pending, priority: priority ? 'pick' : pickPriority(pending), noImages, ...(templateId ? { templateId } : {}) }, ctrl.signal)
      .then((result) => { if (!ctrl.signal.aborted) { setPlan(result); setPlanKey(JSON.stringify([channel.id, pending, priority ? 'pick' : pickPriority(pending), templateId, noImages])); setPhase('idle') } })
      .catch(() => { if (!ctrl.signal.aborted) { setPlan(null); setPhase('failed') } })
    return () => ctrl.abort()
  }, [debounced, channel?.id, priority, noImages, templateId, retry])

  // A result belongs only to the exact current input, never the previous paste.
  const current = text === debounced && phase === 'idle' && planKey === JSON.stringify([channel?.id, urls, inputPriority, templateId, noImages]) ? plan : null
  const duplicate = current?.duplicates[0]
  const onlyDuplicate = !!current && current.accepted.length === 0 && current.duplicates.length > 0
  const invalid = !!current && current.invalid.length > 0
  const disabled = !channel || !current || phase !== 'idle' || busy || current.accepted.length === 0 || !current.template.id
  const first = current?.accepted[0]
  const preview = first ?? (onlyDuplicate ? duplicate : undefined)
  const line = planLine(current, { urls, running: status.data?.running ?? 0, channelName: channel?.name ?? null })
  const state = urls.length === 0 ? 'empty' : multi ? 'multi' : onlyDuplicate ? 'duplicate' : invalid ? 'error' : 'pasted'

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (disabled || !channel || !current) return
    setBusy(true)
    try {
      await enqueueClone({ channelId: channel.id, urls: current.accepted.map((a) => a.url), priority: inputPriority, noImages, ...(templateId ? { templateId } : {}) })
      toast(tt.toasts.made, 'ok', { label: tt.toasts.openQueue, onClick: () => navigate(paths.queue()) })
      setText(''); setPlan(null)
    } catch { toast.err(tt.toasts.failed) }
    finally { setBusy(false) }
  }

  return (
    <main className="nv" data-input={state}>
      <div className="nv-hero">
        <div className="nv-orbs" aria-hidden="true">
          <Orb of="scout" size={34} /><Orb of="director" size={34} /><span className="nv-lens"><LensLogo size={76} /></span><Orb of="artist" size={34} /><Orb of="qa" size={34} />
        </div>
        <h1>{tt.hero}</h1><p>{tt.heroSub}</p>
      </div>
      <div className="nv-modes" role="radiogroup" aria-label={tt.modesLabel}>
        <button type="button" role="radio" aria-checked={!multi} onClick={() => setMode('single')}><Icon name="link" size={mobile ? 16 : 15} />{mobile ? tt.mobileSingle : tt.single}</button>
        <button type="button" role="radio" aria-checked={multi} onClick={() => setMode('multi')}><Icon name="video-add" size={mobile ? 16 : 15} />{mobile ? tt.mobileMulti : tt.multi}</button>
      </div>
      <form className="nv-card card" onSubmit={submit}>
        <label htmlFor="nv-url" className="sr-only">{tt.textarea}</label>
        <div className="nv-entry">
          {!multi && <Icon name="link" size={18} />}
          {multi ? <textarea id="nv-url" rows={4} value={text} aria-label={tt.textarea}
            placeholder={tt.multiPlaceholder} onChange={(e) => setText(e.target.value)} />
            : <input id="nv-url" type="url" inputMode="url" value={text} aria-label={tt.textarea}
              placeholder={mobile ? tt.singlePlaceholder : tt.desktopPlaceholder}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void submit(e) } }} />}
          {text ? <button type="button" className="nv-clear" aria-label={tt.clearInput} onClick={() => { setText(''); setPlan(null) }}><Icon name="close" size={16} /></button>
            : mobile && <button type="button" className="nv-paste" onClick={async () => { try { setText(await navigator.clipboard.readText()) } catch { document.getElementById('nv-url')?.focus() } }}>{tt.paste}</button>}
        </div>
        {preview && !multi && <div className="nv-preview">
          <div className="nv-pimg">{preview.thumbnailUrl && <img src={preview.thumbnailUrl} alt="" />}<span className="nv-score num">{fmtScore(preview.outlier ?? null)}</span><span className="nv-dur mono">{fmtDuration(preview.durationSec)}</span></div>
          <div className="nv-preview-body"><b>{preview.title}</b><span className="nv-meta">{[preview.sourceChannelTitle, videoMeta(preview.viewCount ?? null, preview.publishedAt ?? null)].filter(Boolean).join(' · ')}</span><div className="nv-preview-badges">{preview.hasCaptions && <span className="nv-captions"><Icon name="subtitle" size={13} />{t.screens.explore.hasSubs}</span>}{first?.rivalLinked && <span className="nv-linked">{tt.linkedSource}</span>}</div></div>
        </div>}
        {onlyDuplicate && !multi && <div className="nv-banner nv-banner--duplicate" role="status"><Icon name="flag" size={16} /><span>{tt.duplicateTitle(duplicate?.title ?? duplicate?.url ?? '')}</span><a href={`#${duplicate?.jobId ? paths.workbench({ video: duplicate.jobId }) : paths.queue()}`}>{tt.dup.open}</a></div>}
        {invalid && !multi && <div className="nv-banner" role="alert"><Icon name="danger" size={16} /><span>{tt.invalid}</span></div>}
        {multi && current && <div className="nv-summary"><span className="nv-chip nv-chip--ok"><Icon name="tick-b" size={15} />{tt.accepted(current.accepted.length)}</span>{current.duplicates.length > 0 && <span className="nv-chip nv-chip--warn"><Icon name="flag" size={15} />{tt.plan.skipDup(current.duplicates.length)}</span>}{current.invalid.length > 0 && <span className="nv-chip">{tt.plan.skipInvalid(current.invalid.length)}</span>}{line?.detail && <span className="nv-chip">{line.detail}</span>}</div>}
        <div className="nv-options">
          <label className="nv-option"><Icon name="channel" size={mobile ? 17 : 14} /><span>{tt.channel}</span><select aria-label={tt.channel} value={channel?.id ?? ''} onChange={(e) => { setChannelId(e.target.value); setTemplateId('') }}>{channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
          <label className="nv-option"><Icon name="layer" size={mobile ? 17 : 14} /><span>{tt.template}</span><select aria-label={tt.template} value={templateId} onChange={(e) => setTemplateId(e.target.value)}><option value="">{tt.rotation}{current?.template?.name ? ` · ${current.template.name}` : ''}</option>{channel?.templates.map((tpl) => <option key={tpl.id} value={tpl.id}>{tpl.name}</option>)}</select></label>
          <label className="nv-noimages"><input type="checkbox" checked={noImages} onChange={(e) => setNoImages(e.target.checked)} />{tt.noImages}</label>
          {mobile && multi && <button className="nv-option nv-priority-row" type="button" role="switch" aria-checked={priority} onClick={() => setPriority((value) => !value)}><span>{tt.priorityTurn}</span><span className="nv-toggle" /></button>}
        </div>
        <div className="nv-plan">
          <span className="nv-plantext"><Icon name="clock" size={15} /><span>{phase === 'failed' ? <button type="button" onClick={() => setRetry((v) => v + 1)}>{tt.plan.failed}</button> : phase === 'loading' ? tt.plan.fixing : line?.text || tt.empty.pickFirst}</span></span>
          {!mobile && multi && <button className="nv-priority" type="button" role="switch" aria-checked={priority} onClick={() => setPriority((p) => !p)}><span className="nv-priority-track" aria-hidden="true" />{tt.priority(current?.accepted.length ?? urls.length)}</button>}
          <ActionButton type="submit" variant="primary" className="nv-submit" disabled={busy} lockedReason={disabled ? !channel ? tt.noChannel : current && !current.template.id ? tt.plan.blockedNoTemplate : tt.needLink : undefined}>{multi && current?.accepted.length ? tt.makeMany(current.accepted.length) : tt.makeVideo}<span className="nv-key">Enter</span></ActionButton>
        </div>
      </form>
      <section className="nv-picks">
        <div className="nv-pickhead"><span>{mobile ? tt.mobileSuggestions : tt.suggestions}</span><a href={`#${paths.explore()}`}>{tt.openExplore}<Icon name="arrow-r" size={13} /></a></div>
        <div className="nv-ideas">{(suggestions.data?.videos ?? []).map((v) => <button key={v.id} type="button" className="nv-idea" onClick={() => setText(v.url)}><span className="nv-ideaimg"><img src={v.thumbnailUrl} alt="" /><span className="nv-score num">{fmtScore(v.score)}</span><span className="nv-dur mono">{fmtDuration(v.durationSec)}</span></span><b>{v.title}</b><span>{videoMeta(v.viewCount, v.publishedAt)}</span></button>)}</div>
      </section>
      <div className="nv-workflow"><span>{tt.whatNext}</span>{agents.map((a, i) => <span className="nv-workchip" key={a}><Orb of={a} size={14} />{tt.steps[i]}</span>)}</div>
      <div className="nv-alts">{tt.alts.map((a, i) => <a key={a.title} href={`#${i === 0 ? paths.explore() : i === 1 ? channel ? paths.channel(channel.id) : paths.channelNew() : paths.library()}`}><Icon name={i === 0 ? 'radar' : i === 1 ? 'refresh' : 'layer'} size={17} /><span><b>{a.title}</b><span>{a.body}</span></span></a>)}</div>
      {!mobile && <div className="nv-shortcuts"><span><kbd>Tab</kbd>{tt.modeHint}</span><span><kbd>Enter</kbd>{tt.submitHint}</span></div>}
    </main>
  )
}
