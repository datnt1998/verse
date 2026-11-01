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
import { navigate, paths, type Route } from '../../router'
import { useDebounced } from '../../lib/useDebounced'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { fmtDuration, fmtScore, videoMeta } from '../explore/fmt'
import { PLAN_DEBOUNCE_MS, parseUrls, pickPriority, planLine } from './plan'
import * as stylex from '@stylexjs/stylex'
import { text as textStyles } from '../../styles/shared.stylex'
import { color, line, radius, font } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r, pipeline } from '../../styles/viderse.stylex'

const tt = t.screens.new
const agents: Assistant[] = ['scout', 'director', 'artist', 'qa']

export function NewScreen({ route }: { route: Extract<Route, { name: 'new' }> }) {
  const [text, setText] = useState('')
  const [mode, setMode] = useState<'single' | 'multi'>('single')
  const [channelId, setChannelId] = useState(route.channel ?? '')
  const [templateId, setTemplateId] = useState('')
  const [priority, setPriority] = useState(false)
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
    planClone({ channelId: channel.id, urls: pending, priority: priority ? 'pick' : pickPriority(pending), ...(templateId ? { templateId } : {}) }, ctrl.signal)
      .then((result) => { if (!ctrl.signal.aborted) { setPlan(result); setPlanKey(JSON.stringify([channel.id, pending, priority ? 'pick' : pickPriority(pending), templateId])); setPhase('idle') } })
      .catch(() => { if (!ctrl.signal.aborted) { setPlan(null); setPhase('failed') } })
    return () => ctrl.abort()
  }, [debounced, channel?.id, priority, templateId, retry])

  // A result belongs only to the exact current input, never the previous paste.
  const current = text === debounced && phase === 'idle' && planKey === JSON.stringify([channel?.id, urls, inputPriority, templateId]) ? plan : null
  const duplicate = current?.duplicates[0]
  const onlyDuplicate = !!current && current.accepted.length === 0 && current.duplicates.length > 0
  const invalid = !!current && current.invalid.length > 0
  const disabled = !channel || !current || phase !== 'idle' || busy || current.accepted.length === 0 || !current.template.id
  const first = current?.accepted[0]
  const preview = first ?? (onlyDuplicate ? duplicate : undefined)
  const plannedLine = planLine(current, { urls, running: status.data?.running ?? 0, channelName: channel?.name ?? null })
  const acceptedCount = current?.accepted.length ?? 0
  const prioritizedBatch = multi && priority && acceptedCount > 0 && !!current?.template.id && current.position === 'front'
  const line = prioritizedBatch ? {
    text: tt.plan.priorityHead(acceptedCount, current.etaStart ? new Date(current.etaStart).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : null),
    detail: tt.plan.priorityReady(acceptedCount),
  } : mobile && multi && acceptedCount > 0 && current?.template.id && current.position === 'back' && channel ? {
    text: tt.plan.mobileBatchTail(channel.pacePerDay, channel.name, current.paceLeftToday),
    detail: tt.plan.mobileBatchDetail(current.paceLeftToday, acceptedCount),
  } : plannedLine
  const state = urls.length === 0 ? 'empty' : multi ? 'multi' : onlyDuplicate ? 'duplicate' : invalid ? 'error' : 'pasted'

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (disabled || !channel || !current) return
    setBusy(true)
    try {
      await enqueueClone({ channelId: channel.id, urls: current.accepted.map((a) => a.url), priority: inputPriority, ...(templateId ? { templateId } : {}) })
      toast(tt.toasts.made, 'ok', { label: tt.toasts.openQueue, onClick: () => navigate(paths.queue()) })
      setText(''); setPlan(null)
    } catch { toast.err(tt.toasts.failed) }
    finally { setBusy(false) }
  }

  return (
    <main className={`nv ${stylex.props(styles.root, state === 'empty' && styles.emptyRoot).className}`} data-input={state}>
      <div className={`nv-hero ${stylex.props(styles.hero).className}`}>
        <div className={`nv-orbs ${stylex.props(styles.orbs).className}`} aria-hidden="true">
          <Orb of="scout" size={mobile ? 18 : 34} /><Orb of="director" size={mobile ? 18 : 34} /><span className={`nv-lens ${stylex.props(styles.lens).className}`}><LensLogo size={76} /></span><Orb of="artist" size={mobile ? 18 : 34} /><Orb of="qa" size={mobile ? 18 : 34} />
        </div>
        <h1 {...stylex.props(styles.heading)}>{tt.hero}</h1><p {...stylex.props(styles.lede)}>{tt.heroSub}</p>
      </div>
      <div className={`nv-modes ${stylex.props(styles.modes).className}`} role="radiogroup" aria-label={tt.modesLabel}>
        <button {...stylex.props(styles.mode, !multi && styles.modeChecked)} type="button" role="radio" aria-checked={!multi} onClick={() => setMode('single')}><Icon name="link" size={mobile ? 16 : 15} />{mobile ? tt.mobileSingle : tt.single}</button>
        <button {...stylex.props(styles.mode, multi && styles.modeChecked)} type="button" role="radio" aria-checked={multi} onClick={() => setMode('multi')}><Icon name="video-add" size={mobile ? 16 : 15} />{mobile ? tt.mobileMulti : tt.multi}</button>
      </div>
      <form className={`nv-card ${stylex.props(styles.liquidCard, styles.card, state === 'error' && styles.cardError).className}`} onSubmit={submit}>
        <label htmlFor="nv-url" {...stylex.props(textStyles.srOnly)}>{tt.textarea}</label>
        <div className={`nv-entry ${stylex.props(styles.entry, multi && styles.entryMulti).className}`}>
          {!multi && <Icon name="link" size={18} xstyle={styles.entryIcon} />}
          {multi ? <textarea {...stylex.props(styles.input, styles.multiInput)} id="nv-url" rows={4} value={text} aria-label={tt.textarea}
            placeholder={tt.multiPlaceholder} onChange={(e) => setText(e.target.value)} />
            : <input {...stylex.props(styles.input)} id="nv-url" type="url" inputMode="url" value={text} aria-label={tt.textarea}
              placeholder={mobile ? tt.singlePlaceholder : tt.desktopPlaceholder}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void submit(e) } }} />}
          {text ? <button type="button" className={`nv-clear ${stylex.props(styles.clear, multi && styles.clearMulti).className}`} aria-label={tt.clearInput} onClick={() => { setText(''); setPlan(null) }}><Icon name="close" size={16} /></button>
            : mobile && <button type="button" className={`nv-paste ${stylex.props(styles.paste).className}`} onClick={async () => { try { setText(await navigator.clipboard.readText()) } catch { document.getElementById('nv-url')?.focus() } }}>{tt.paste}</button>}
        </div>
        {preview && !multi && <div className={`nv-preview ${stylex.props(styles.preview).className}`}>
          <div className={`nv-pimg ${stylex.props(styles.previewImage).className}`}>{preview.thumbnailUrl && <img {...stylex.props(styles.image)} src={preview.thumbnailUrl} alt="" />}<span className={`nv-score num ${stylex.props(styles.score, styles.previewScore, onlyDuplicate && styles.duplicateScore).className}`}>{fmtScore(preview.outlier ?? null)}</span><span className={`nv-dur mono ${stylex.props(styles.duration).className}`}>{fmtDuration(preview.durationSec)}</span></div>
          <div className={`nv-preview-body ${stylex.props(styles.previewBody).className}`}><b {...stylex.props(styles.previewTitle)}>{preview.title}</b><span className={`nv-meta ${stylex.props(styles.meta).className}`}>{[preview.sourceChannelTitle, videoMeta(preview.viewCount ?? null, preview.publishedAt ?? null)].filter(Boolean).join(' · ')}</span><div className={`nv-preview-badges ${stylex.props(styles.previewBadges).className}`}>{preview.hasCaptions && <span className={`nv-captions ${stylex.props(styles.captions).className}`}><Icon name="subtitle" size={13} />{t.screens.explore.hasSubs}</span>}{first?.rivalLinked && <span className={`nv-linked ${stylex.props(styles.linked).className}`}>{tt.linkedSource}</span>}</div></div>
        </div>}
        {onlyDuplicate && !multi && <div className={`nv-banner nv-banner--duplicate ${stylex.props(styles.banner, styles.bannerDuplicate).className}`} role="status"><Icon name="flag" size={16} /><span {...stylex.props(styles.bannerText, styles.duplicateText)}>{tt.duplicateTitle(duplicate?.title ?? duplicate?.url ?? '')}</span><a {...stylex.props(styles.duplicateLink)} href={`#${duplicate?.jobId ? paths.workbench({ video: duplicate.jobId }) : paths.queue()}`}>{tt.dup.open}</a></div>}
        {invalid && !multi && <div className={`nv-banner ${stylex.props(styles.banner).className}`} role="alert"><Icon name="danger" size={16} /><span {...stylex.props(styles.bannerText)}>{tt.invalid}</span></div>}
        {multi && current && <div className={`nv-summary ${stylex.props(styles.summary).className}`}><span className={`nv-chip nv-chip--ok ${stylex.props(styles.chip, styles.chipOk).className}`}><Icon xstyle={styles.summaryIcon} name="tick-b" size={15} />{tt.accepted(current.accepted.length)}</span>{current.duplicates.length > 0 && <span className={`nv-chip nv-chip--warn ${stylex.props(styles.chip, styles.chipWarn).className}`}><Icon xstyle={styles.summaryIcon} name="flag" size={15} />{tt.plan.skipDup(current.duplicates.length)}</span>}{current.invalid.length > 0 && <span className={`nv-chip ${stylex.props(styles.chip, styles.chipPlain).className}`}>{tt.plan.skipInvalid(current.invalid.length)}</span>}{line?.detail && <span className={`nv-chip ${stylex.props(styles.chip, styles.chipPlain).className}`}>{line.detail}</span>}</div>}
        <div className={`nv-options ${stylex.props(styles.options, state === 'empty' && styles.emptyHidden).className}`}>
          <label className={`nv-option ${stylex.props(styles.option).className}`}><Icon name="channel" size={mobile ? 17 : 14} /><span {...stylex.props(styles.optionLabel)}>{tt.channel}</span><select {...stylex.props(styles.select)} aria-label={tt.channel} value={channel?.id ?? ''} onChange={(e) => { setChannelId(e.target.value); setTemplateId('') }}>{channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
          <label className={`nv-option ${stylex.props(styles.option).className}`}><Icon name="layer" size={mobile ? 17 : 14} /><span {...stylex.props(styles.optionLabel)}>{tt.template}</span><select {...stylex.props(styles.select)} aria-label={tt.template} value={templateId} onChange={(e) => setTemplateId(e.target.value)}><option value="">{tt.rotation}{current?.template?.name ? ` · ${current.template.name}` : ''}</option>{channel?.templates.map((tpl) => <option key={tpl.id} value={tpl.id}>{tpl.name}</option>)}</select></label>
          {mobile && multi && <button className={`nv-option nv-priority-row ${stylex.props(styles.option, styles.priorityRow).className}`} type="button" role="switch" aria-label={tt.priority(acceptedCount)} aria-checked={priority} onClick={() => setPriority((value) => !value)}><Icon name="arrow-up" size={17} xstyle={styles.priorityIcon} /><span {...stylex.props(styles.priorityText)}><span>{tt.priority(acceptedCount)}</span><span {...stylex.props(styles.prioritySubtitle)} title={tt.priorityHint}>{tt.priorityHint}</span></span><span className={`nv-toggle ${stylex.props(styles.toggle, priority && styles.toggleOn).className}`} aria-hidden="true"><span {...stylex.props(styles.toggleKnob)} /></span></button>}
        </div>
        <div className={`nv-plan ${stylex.props(styles.plan).className}`}>
          <span className={`nv-plantext ${stylex.props(styles.planText, state === 'empty' && styles.emptyHidden).className}`}><Icon xstyle={styles.noShrink} name="clock" size={15} /><span>{phase === 'failed' ? <button type="button" onClick={() => setRetry((v) => v + 1)}>{tt.plan.failed}</button> : phase === 'loading' ? tt.plan.fixing : line?.text || tt.empty.pickFirst}</span></span>
          {!mobile && multi && <button className={`nv-priority ${stylex.props(styles.priority).className}`} type="button" role="switch" aria-checked={priority} onClick={() => setPriority((p) => !p)}><span className={`nv-priority-track ${stylex.props(styles.priorityTrack, priority && styles.priorityTrackOn).className}`} aria-hidden="true" />{tt.priority(acceptedCount)}</button>}
          <ActionButton type="submit" variant="primary" className="nv-submit" xstyle={[styles.submit, disabled && styles.submitLocked]} disabled={busy} lockedReason={disabled ? !channel ? tt.noChannel : current && !current.template.id ? tt.plan.blockedNoTemplate : tt.needLink : undefined}><span {...stylex.props(styles.submitLabel)}>{multi && current?.accepted.length ? tt.makeMany(current.accepted.length) : tt.makeVideo}<span className={`nv-key ${stylex.props(styles.key).className}`}>Enter</span></span></ActionButton>
        </div>
      </form>
      <section className={`nv-picks ${stylex.props(styles.picks, state !== 'empty' && styles.picksHidden).className}`}>
        <div className={`nv-pickhead ${stylex.props(styles.pickHead).className}`}><span {...stylex.props(styles.pickLabel)}>{mobile ? tt.mobileSuggestions : tt.suggestions}</span><a {...stylex.props(styles.pickLink)} href={`#${paths.explore()}`}>{tt.openExplore}<Icon name="arrow-r" size={13} /></a></div>
        <div className={`nv-ideas ${stylex.props(styles.ideas).className}`}>{(suggestions.data?.videos ?? []).map((v) => <button key={v.id} type="button" className={`nv-idea ${stylex.props(styles.idea).className}`} onClick={() => setText(v.url)}><span className={`nv-ideaimg ${stylex.props(styles.ideaImage).className}`}><img {...stylex.props(styles.image)} src={v.thumbnailUrl} alt="" /><span className={`nv-score num ${stylex.props(styles.score).className}`}>{fmtScore(v.score)}</span><span className={`nv-dur mono ${stylex.props(styles.duration).className}`}>{fmtDuration(v.durationSec)}</span></span><b {...stylex.props(styles.ideaTitle)}>{v.title}</b><span {...stylex.props(styles.ideaMeta)}>{videoMeta(v.viewCount, v.publishedAt)}</span></button>)}</div>
      </section>
      <div className={`nv-workflow ${stylex.props(styles.workflow).className}`}><span {...stylex.props(styles.workflowLabel)}>{tt.whatNext}</span>{agents.map((a, i) => <span className={`nv-workchip ${stylex.props(styles.workChip).className}`} key={a}><Orb of={a} size={14} />{tt.steps[i]}</span>)}</div>
      <div className={`nv-alts ${stylex.props(styles.alts).className}`}>{tt.alts.map((a, i) => <a {...stylex.props(styles.alt)} key={a.title} href={`#${i === 0 ? paths.explore() : i === 1 ? channel ? paths.channel(channel.id) : paths.channelNew() : paths.library()}`}><Icon xstyle={styles.altIcon} name={i === 0 ? 'radar' : i === 1 ? 'refresh' : 'layer'} size={17} /><span {...stylex.props(styles.altText)}><b {...stylex.props(styles.altTitle)}>{a.title}</b><span {...stylex.props(styles.altBody)}>{a.body}</span></span></a>)}</div>
      {!mobile && <div className={`nv-shortcuts ${stylex.props(styles.shortcuts).className}`}><span {...stylex.props(styles.shortcut)}><kbd {...stylex.props(styles.shortcutKey)}>Tab</kbd>{tt.modeHint}</span><span {...stylex.props(styles.shortcut)}><kbd {...stylex.props(styles.shortcutKey)}>Enter</kbd>{tt.submitHint}</span></div>}
    </main>
  )
}

const MOBILE = '@media (max-width: 767px)'
const styles = stylex.create({
  liquidCard: { backgroundColor: "var(--card-bg)", borderWidth: "1px", borderStyle: "solid", borderColor: "var(--line-2)", boxShadow: 'var(--card-shadow)' },
  root: { flex: 1, overflowY: 'auto', width: '100%', maxWidth: 820, margin: '0 auto', display: 'flex', flexDirection: 'column', alignItems: 'stretch', padding: { default: `${s['--vv-s-32']} ${s['--vv-s-20']} ${s['--vv-s-28']}`, [MOBILE]: `${s['--vv-s-4']} ${s['--vv-s-16']} ${s['--vv-s-16']}` }, gap: { default: s['--vv-s-22'], [MOBILE]: s['--vv-s-16'] }, minHeight: { default: null, [MOBILE]: 0 }, marginBottom: { default: null, [MOBILE]: pipeline.newFooterClearance }, lineHeight: { default: null, [MOBILE]: 1.5 } },
  emptyRoot: { marginBottom: { default: null, [MOBILE]: pipeline.newEmptyFooterClearance } },
  hero: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: { default: s['--vv-s-12'], [MOBILE]: s['--vv-s-10'] }, textAlign: 'center', paddingTop: { default: null, [MOBILE]: s['--vv-s-4'] } },
  orbs: { display: 'flex', alignItems: 'center', gap: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-6'] }, padding: { default: null, [MOBILE]: 0 } },
  lens: { display: { default: 'flex', [MOBILE]: 'none' }, margin: `0 ${s['--vv-s-6']}` },
  heading: { margin: 0, fontSize: { default: 30, [MOBILE]: 22 }, fontWeight: 600, letterSpacing: '-.02em', lineHeight: { default: 1.2, [MOBILE]: 1.5 } },
  lede: { margin: 0, maxWidth: 640, color: color['--color-fg-3'], fontSize: 14, textWrap: 'pretty', display: { default: null, [MOBILE]: 'none' } },
  modes: { alignSelf: 'center', display: 'flex', gap: s['--vv-s-2'], padding: s['--vv-s-3'], borderRadius: { default: radius['--radius-row'], [MOBILE]: radius['--radius-tile'] }, backgroundColor: color['--color-surface-1'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-1'], width: { default: null, [MOBILE]: '100%' }, margin: { default: null, [MOBILE]: 0 } },
  mode: { height: { default: 34, [MOBILE]: 40 }, padding: `0 ${s['--vv-s-16']}`, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-8'], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", borderRadius: { default: r['--vv-r-7'], [MOBILE]: radius['--radius-button'] }, backgroundColor: "transparent", color: color['--color-fg-3'], fontSize: { default: 13.5, [MOBILE]: 14 }, cursor: 'pointer', flex: { default: null, [MOBILE]: 1 } },
  modeChecked: { backgroundColor: color['--color-surface-6'], color: color['--color-fg'], fontWeight: 500 },
  card: { width: '100%', borderRadius: radius['--radius-dock'], padding: `${s['--vv-s-16']} ${s['--vv-s-16']} ${s['--vv-s-14']} ${s['--vv-s-18']}`, display: { default: 'flex', [MOBILE]: 'contents' }, flexDirection: 'column', gap: s['--vv-s-14'], backgroundColor: "var(--vv-clone-card-bg)", borderColor: line['--line-3'] },
  cardError: { borderColor: `color-mix(in srgb, ${color['--color-err']} 35%, transparent)` },
  entry: { display: 'flex', alignItems: 'center', gap: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-8'] }, color: 'var(--vv-fg-note)', width: { default: null, [MOBILE]: '100%' }, height: { default: null, [MOBILE]: 58 }, minHeight: { default: null, [MOBILE]: 58 }, padding: { default: null, [MOBILE]: `${s['--vv-s-4']} ${s['--vv-s-4']} ${s['--vv-s-4']} ${s['--vv-s-14']}` }, borderWidth: { default: null, [MOBILE]: "1px" }, borderStyle: { default: null, [MOBILE]: "solid" }, borderColor: { default: null, [MOBILE]: line['--line-2'] }, borderRadius: { default: null, [MOBILE]: radius['--radius-card'] }, backgroundColor: { default: null, [MOBILE]: "var(--vv-clone-card-bg)" }, backgroundImage: { default: null } },
  entryMulti: { height: { default: null, [MOBILE]: 'auto' }, minHeight: { default: null, [MOBILE]: 0 }, display: { default: null, [MOBILE]: 'block' } },
  entryIcon: { display: { default: null, [MOBILE]: 'none' } },
  input: { flex: 1, width: '100%', minWidth: 0, height: { default: 40, [MOBILE]: 48 }, resize: 'none', padding: `${s['--vv-s-1']} ${s['--vv-s-2']}`, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", outlineWidth: 0, outlineStyle: "none", outlineColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", color: { default: color['--color-fg'], "::placeholder": 'var(--vv-fg-note)' }, fontFamily: 'inherit', fontSize: 15, lineHeight: { default: 1.55, [MOBILE]: 1.5 } },
  multiInput: { height: { default: 100, [MOBILE]: 'auto' }, padding: { default: s['--vv-s-2'], [MOBILE]: `${s['--vv-s-10']} ${s['--vv-s-10']} ${s['--vv-s-10']} 0` }, lineHeight: { default: '24px', [MOBILE]: 1.6 }, fontSize: { default: 15, [MOBILE]: 14 } },
  clear: { backgroundColor: "transparent", backgroundImage: "none", borderWidth: 0, borderStyle: "none", borderColor: "currentColor", borderRadius: radius['--radius-control'], color: 'var(--vv-fg-note)', cursor: 'pointer', width: { default: 30, [MOBILE]: 44 }, height: { default: 30, [MOBILE]: 44 }, display: 'grid', placeItems: 'center' },
  clearMulti: { display: 'none' },
  paste: { height: 40, padding: `0 ${s['--vv-s-14']}`, borderRadius: r['--vv-r-11'], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: color['--color-fg'], color: color['--color-bg'], fontSize: 14, fontWeight: 600 },
  preview: { display: 'flex', gap: { default: s['--vv-s-14'], [MOBILE]: s['--vv-s-12'] }, padding: { default: s['--vv-s-10'], [MOBILE]: 0 }, borderRadius: { default: radius['--radius-tile'], [MOBILE]: 0 }, backgroundColor: { default: color['--color-surface-0'], [MOBILE]: "transparent" }, borderWidth: { default: "1px", [MOBILE]: 0 }, borderStyle: { default: "solid", [MOBILE]: "none" }, borderColor: { default: line['--line-1'], [MOBILE]: "currentColor" }, width: { default: null, [MOBILE]: '100%' }, alignItems: { default: null, [MOBILE]: 'flex-start' } },
  previewImage: { position: 'relative', width: { default: 168, [MOBILE]: 132 }, aspectRatio: '16/9', borderRadius: { default: radius['--radius-button'], [MOBILE]: radius['--radius-row'] }, backgroundColor: color['--color-surface-3'], flexShrink: 0, overflow: 'hidden' },
  image: { display: 'block', width: '100%', height: '100%', objectFit: 'cover' },
  score: { position: 'absolute', left: s['--vv-s-6'], bottom: s['--vv-s-5'], height: 18, padding: `0 ${s['--vv-s-6']}`, display: 'flex', alignItems: 'center', borderRadius: r['--vv-r-5'], color: color['--color-bg'], backgroundColor: color['--color-warn'], fontSize: 11, fontWeight: 600 },
  previewScore: { backgroundColor: `color-mix(in srgb, ${color['--color-bg']} 82%, transparent)`, color: 'var(--vv-note-ink)' },
  duplicateScore: { backgroundColor: color['--color-warn'], color: color['--color-bg'] },
  duration: { position: 'absolute', right: s['--vv-s-6'], bottom: s['--vv-s-5'], padding: `0 ${s['--vv-s-5']}`, borderRadius: r['--vv-r-4'], backgroundColor: "var(--vv-thumb-veil)", fontSize: 10.5 },
  previewBody: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: { default: s['--vv-s-6'], [MOBILE]: s['--vv-s-4'] } },
  previewTitle: { fontSize: { default: 14.5, [MOBILE]: 14 }, fontWeight: { default: 500, [MOBILE]: 600 }, lineHeight: { default: 1.4, [MOBILE]: 1.35 }, display: { default: null, [MOBILE]: '-webkit-box' }, WebkitLineClamp: { default: null, [MOBILE]: 2 }, WebkitBoxOrient: { default: null, [MOBILE]: 'vertical' }, overflow: { default: null, [MOBILE]: 'hidden' } },
  meta: { fontSize: { default: 12.5, [MOBILE]: 12 }, color: 'var(--vv-fg-note)', whiteSpace: { default: null, [MOBILE]: 'nowrap' }, overflow: { default: null, [MOBILE]: 'hidden' }, textOverflow: { default: null, [MOBILE]: 'ellipsis' } },
  captions: { alignSelf: 'flex-start', height: { default: 22, [MOBILE]: 'auto' }, padding: { default: `0 ${s['--vv-s-8']}`, [MOBILE]: 0 }, borderRadius: radius['--radius-chip'], backgroundColor: { default: `color-mix(in srgb, ${color['--color-ok']} 8%, transparent)`, [MOBILE]: "transparent" }, backgroundImage: { [MOBILE]: "none" }, display: 'flex', alignItems: 'center', gap: s['--vv-s-5'], color: color['--color-ok'], fontSize: { default: 11.5, [MOBILE]: 12 }, lineHeight: { default: null, [MOBILE]: 1.5 } },
  previewBadges: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], flexWrap: 'wrap', margin: { default: null, [MOBILE]: 0 } },
  linked: { height: 22, display: { default: 'flex', [MOBILE]: 'none' }, alignItems: 'center', padding: `0 ${s['--vv-s-8']}`, borderRadius: radius['--radius-chip'], backgroundColor: "var(--vv-wash-5)", fontSize: 11.5, color: color['--color-fg-2'] },
  options: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: { default: s['--vv-s-8'], [MOBILE]: 0 }, flexDirection: { default: null, [MOBILE]: 'column' }, width: { default: null, [MOBILE]: '100%' }, borderWidth: { default: null, [MOBILE]: "1px" }, borderStyle: { default: null, [MOBILE]: "solid" }, borderColor: { default: null, [MOBILE]: line['--line-2'] }, borderRadius: { default: null, [MOBILE]: radius['--radius-card'] }, backgroundColor: { default: null, [MOBILE]: "transparent" }, backgroundImage: { default: null } },
  emptyHidden: { display: { default: 'flex', [MOBILE]: 'none' } },
  option: { display: 'flex', alignItems: 'center', gap: { default: s['--vv-s-6'], [MOBILE]: s['--vv-s-12'] }, height: 30, borderWidth: { default: "1px", [MOBILE]: 0 }, borderStyle: { default: "solid", [MOBILE]: "none" }, borderColor: { default: `color-mix(in srgb, ${line['--line-3']}, ${line['--line-4']})`, [MOBILE]: "currentColor" }, borderBottomWidth: { default: null, [MOBILE]: "1px" }, borderBottomStyle: { default: null, [MOBILE]: "solid" }, borderBottomColor: { default: null, [MOBILE]: line['--line-2'] }, borderRadius: { default: radius['--radius-control'], [MOBILE]: 0 }, padding: { default: `0 ${s['--vv-s-10']} 0 ${s['--vv-s-8']}`, [MOBILE]: `0 ${s['--vv-s-14']}` }, fontSize: { default: 12.5, [MOBILE]: 14 }, color: color['--color-fg-3'], width: { default: null, [MOBILE]: '100%' }, minHeight: { default: null, [MOBILE]: 52 } },
  optionLabel: { display: 'inline', color: { default: null, [MOBILE]: color['--color-fg-3'] } },
  select: { maxWidth: { default: 250, [MOBILE]: 'none' }, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: "transparent", color: color['--color-fg'], fontFamily: "inherit", fontSize: { default: "inherit", [MOBILE]: 14 }, fontWeight: { default: 500, [MOBILE]: 400 }, lineHeight: "inherit", fontStyle: "inherit", fontVariant: "inherit", fontStretch: "inherit", flex: { default: null, [MOBILE]: 1 }, textAlign: { default: null, [MOBILE]: 'right' }, minWidth: { default: null, [MOBILE]: 0 } },
  plan: { display: 'flex', flexWrap: 'wrap', alignItems: { default: 'center', [MOBILE]: 'stretch' }, gap: { default: `${s['--vv-s-8']} ${s['--vv-s-12']}`, [MOBILE]: s['--vv-s-8'] }, marginTop: { default: `calc(-1 * ${s['--vv-s-2']})`, [MOBILE]: 0 }, position: { default: null, [MOBILE]: 'fixed' }, zIndex: { default: null, [MOBILE]: 20 }, left: { default: null, [MOBILE]: 0 }, right: { default: null, [MOBILE]: 0 }, bottom: { default: null, [MOBILE]: 'env(safe-area-inset-bottom)' }, width: { default: null, [MOBILE]: '100%' }, flexDirection: { default: null, [MOBILE]: 'column' }, padding: { default: null, [MOBILE]: `${s['--vv-s-10']} ${s['--vv-s-16']} ${s['--vv-s-4']}` }, borderTopWidth: { default: null, [MOBILE]: "1px" }, borderTopStyle: { default: null, [MOBILE]: "solid" }, borderTopColor: { default: null, [MOBILE]: line['--line-1'] }, backgroundColor: { default: null, [MOBILE]: `color-mix(in srgb, ${color['--color-bg']}, ${color['--color-surface-0']})` }, backgroundImage: { default: null } },
  planText: { flex: { default: '1 1 320px', [MOBILE]: '0 0 auto' }, minWidth: 0, display: 'flex', gap: s['--vv-s-8'], alignItems: { default: 'center', [MOBILE]: 'flex-start' }, fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5, padding: { default: null, [MOBILE]: `0 ${s['--vv-s-2']} ${s['--vv-s-4']}` } },
  noShrink: { flexShrink: 0 },
  submit: { flex: '0 0 auto', height: { default: 36, [MOBILE]: 48 }, minWidth: 0, paddingBlock: 0, paddingInlineStart: { default: s['--vv-s-14'], [MOBILE]: s['--vv-s-16'] }, paddingInlineEnd: { default: s['--vv-s-12'], [MOBILE]: s['--vv-s-16'] }, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", borderRadius: { default: radius['--radius-button'], [MOBILE]: pipeline.newSubmitRadius }, fontSize: { default: 13.5, [MOBILE]: 15 }, gap: s['--vv-s-8'], width: { default: null, [MOBILE]: '100%' }, lineHeight: { default: null, [MOBILE]: 1.5 } },
  submitLabel: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'] },
  submitLocked: { opacity: 1, backgroundColor: color['--color-surface-7'], color: 'var(--vv-fg-note)' },
  key: { fontFamily: font['--font-mono'], fontSize: 11, padding: `${s['--vv-s-1']} ${s['--vv-s-5']}`, borderWidth: "1px", borderStyle: "solid", borderColor: "var(--vv-kbd-line)", borderRadius: r['--vv-r-4'], backgroundColor: "var(--vv-kbd-bg)", lineHeight: 1.55, display: { default: null, [MOBILE]: 'none' } },
  priority: { height: 30, padding: `0 ${s['--vv-s-10']} 0 ${s['--vv-s-8']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-3'], borderRadius: radius['--radius-control'], backgroundColor: "transparent", backgroundImage: "none", color: color['--color-fg-3'], fontSize: 12.5, cursor: 'pointer' },
  priorityTrack: { width: { default: 26, "::after": 11 }, height: { default: 15, "::after": 11 }, padding: s['--vv-s-2'], display: 'flex', borderRadius: { default: r['--vv-r-pill'], "::after": '50%' }, backgroundColor: { default: color['--color-surface-7'], "::after": color['--color-fg'] }, content: { default: null, "::after": "''" } },
  priorityTrackOn: { justifyContent: 'flex-end', backgroundColor: 'var(--vv-red-button)' },
  banner: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-10'], borderWidth: "1px", borderStyle: "solid", borderColor: `color-mix(in srgb, ${color['--color-err']} 30%, transparent)`, borderRadius: radius['--radius-tile'], backgroundColor: `color-mix(in srgb, ${color['--color-err']} 6%, transparent)`, padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, color: color['--color-err'], fontSize: 13, lineHeight: 1.55 },
  bannerText: { flex: '1 1 280px', minWidth: 0, color: 'var(--vv-warn-ink)' },
  bannerDuplicate: { borderColor: `color-mix(in srgb, ${color['--color-warn']} 30%, transparent)`, backgroundColor: `color-mix(in srgb, ${color['--color-warn']} 7%, transparent)`, color: color['--color-warn'] },
  duplicateText: { color: 'var(--vv-note-ink)' },
  duplicateLink: { height: 30, padding: `0 ${s['--vv-s-10']}`, display: 'flex', alignItems: 'center', borderRadius: radius['--radius-control'], borderWidth: "1px", borderStyle: "solid", borderColor: `color-mix(in srgb, ${line['--line-3']}, ${line['--line-5']})`, color: color['--color-fg'], fontSize: 12.5, whiteSpace: 'nowrap' },
  summary: { display: 'flex', flexWrap: { default: 'wrap', [MOBILE]: 'nowrap' }, gap: s['--vv-s-6'], fontSize: { default: 12.5, [MOBILE]: 13 }, flexDirection: { default: null, [MOBILE]: 'column' } },
  chip: { height: { default: 24, [MOBILE]: 'auto' }, padding: { default: `0 ${s['--vv-s-8']}`, [MOBILE]: 0 }, display: 'flex', alignItems: 'center', borderRadius: { default: radius['--radius-chip'], [MOBILE]: 0 }, gap: { default: null, [MOBILE]: s['--vv-s-8'] } },
  chipOk: { color: color['--color-ok'], backgroundColor: { default: `color-mix(in srgb, ${color['--color-ok']} 8%, transparent)`, [MOBILE]: "transparent" }, backgroundImage: { [MOBILE]: "none" } },
  chipWarn: { color: color['--color-warn'], backgroundColor: { default: `color-mix(in srgb, ${color['--color-warn']} 8%, transparent)`, [MOBILE]: "transparent" }, backgroundImage: { [MOBILE]: "none" } },
  chipPlain: { color: { default: null, [MOBILE]: 'var(--vv-fg-note)' }, lineHeight: { default: null, [MOBILE]: 1.5 }, backgroundColor: { default: null, [MOBILE]: "transparent" }, backgroundImage: { default: null, [MOBILE]: "none" } },
  summaryIcon: { display: { default: 'none', [MOBILE]: 'inline-flex' } },
  picks: { width: '100%', display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'], margin: { default: null, [MOBILE]: 0 } },
  picksHidden: { display: { default: 'flex', [MOBILE]: 'none' } },
  pickHead: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: { default: `0 ${s['--vv-s-4']}`, [MOBILE]: 0 }, color: color['--color-fg-3'] },
  pickLabel: { flex: 1, fontSize: 13 },
  pickLink: { display: { default: 'flex', [MOBILE]: 'none' }, alignItems: 'center', gap: s['--vv-s-4'], color: color['--color-fg-3'], fontSize: 12.5, textDecoration: 'none', whiteSpace: 'nowrap' },
  ideas: { display: { default: 'grid', [MOBILE]: 'flex' }, gridTemplateColumns: 'repeat(auto-fit, minmax(min(220px, 100%), 1fr))', gap: s['--vv-s-10'], flexDirection: { default: null, [MOBILE]: 'column' } },
  idea: { minWidth: 0, borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-2'], borderRadius: r['--vv-r-14'], backgroundColor: color['--color-card'], color: color['--color-fg'], padding: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-8'] }, display: { default: 'flex', [MOBILE]: 'grid' }, flexDirection: 'column', gap: s['--vv-s-8'], columnGap: { default: null, [MOBILE]: s['--vv-s-12'] }, rowGap: { default: null, [MOBILE]: s['--vv-s-3'] }, textAlign: 'left', cursor: 'pointer', whiteSpace: 'normal', gridTemplateColumns: { default: null, [MOBILE]: '104px 1fr' } },
  ideaImage: { position: 'relative', width: '100%', aspectRatio: '16/9', borderRadius: radius['--radius-button'], overflow: 'hidden', backgroundColor: color['--color-surface-2'], gridRow: { default: null, [MOBILE]: 'span 2' }, height: { default: null, [MOBILE]: 58 } },
  ideaTitle: { fontSize: 13.5, lineHeight: { default: 1.4, [MOBILE]: 1.35 }, fontWeight: 500, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' },
  ideaMeta: { fontSize: 12, color: 'var(--vv-fg-note)' },
  workflow: { width: '100%', display: { default: 'flex', [MOBILE]: 'none' }, flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', gap: `${s['--vv-s-8']} ${s['--vv-s-10']}` },
  workflowLabel: { fontSize: 12.5, color: 'var(--vv-fg-note)' },
  workChip: { display: 'flex', alignItems: 'center', gap: s['--vv-s-7'], height: 28, padding: `0 ${s['--vv-s-10']} 0 ${s['--vv-s-6']}`, borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-3'], borderRadius: r['--vv-r-pill'], color: color['--color-fg-2'], fontSize: 12.5 },
  alts: { width: '100%', display: { default: 'grid', [MOBILE]: 'none' }, gridTemplateColumns: 'repeat(auto-fit, minmax(min(220px, 100%), 1fr))', gap: s['--vv-s-10'] },
  alt: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-12']} ${s['--vv-s-14']}`, color: color['--color-fg'], textDecoration: 'none', borderWidth: "1px", borderStyle: "dashed", borderColor: `color-mix(in srgb, ${line['--line-3']}, ${line['--line-5']})`, borderRadius: r['--vv-r-14'] },
  altText: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  altIcon: { marginTop: s['--vv-s-2'], color: color['--color-fg-3'], flexShrink: 0 },
  altTitle: { fontSize: 13.5, fontWeight: 500 },
  altBody: { fontSize: 12, lineHeight: 1.45, color: 'var(--vv-fg-note)' },
  shortcuts: { display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: `${s['--vv-s-4']} ${s['--vv-s-16']}`, fontSize: 12, color: 'var(--vv-fg-note)' },
  shortcut: { display: 'flex', gap: s['--vv-s-6'], alignItems: 'center' },
  shortcutKey: { padding: `${s['--vv-s-1']} ${s['--vv-s-5']}`, borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-4'], borderRadius: r['--vv-r-4'], backgroundColor: "var(--vv-wash-5)", fontFamily: font['--font-mono'], fontSize: "11px", fontWeight: "normal", lineHeight: "1.55", fontStyle: "normal", fontVariant: "normal", fontStretch: "normal" },
  priorityRow: { backgroundColor: "transparent", backgroundImage: "none", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", color: color['--color-fg'], fontSize: 14, textAlign: 'left', cursor: 'pointer' },
  priorityIcon: { flexShrink: 0, color: 'var(--vv-fg-note)' },
  priorityText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' },
  prioritySubtitle: { fontSize: 12, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  toggle: { flex: '0 0 auto', width: 44, height: 26, borderRadius: r['--vv-r-pill'], padding: s['--vv-s-3'], backgroundColor: color['--color-fg-5'], display: 'flex', justifyContent: 'flex-start' },
  toggleOn: { backgroundColor: 'var(--vv-red-button)', justifyContent: 'flex-end' },
  toggleKnob: { width: 20, height: 20, borderRadius: '50%', backgroundColor: 'var(--vv-red-ink)' },
})


