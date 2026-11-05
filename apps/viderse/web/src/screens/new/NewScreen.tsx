import { useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { Banner, Button, Icon, Segmented, Switch, toast } from '@noverse/ui'
import { ActionButton, LensLogo, Orb, type Assistant } from '../../components'
import { ResponsiveSelect } from '../../components/responsive-overlays'
import { useResource } from '../../data/useResource'
import { useStatus } from '../../data/status'
import { keys } from '../../data/keys'
import { listChannels } from '../../api/channels'
import { listTemplates } from '../../api/library'
import { listRivalVideos } from '../../api/rivals'
import { enqueueClone, planClone, type ClonePlan } from '../../api/clone'
import { t } from '../../i18n/newScreen'
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
  const scrollRef = useRef<HTMLElement>(null)
  const footerRef = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    if (!mobile || !scrollRef.current || !footerRef.current) return
    const scroll = scrollRef.current
    const footer = footerRef.current
    // Plan text wraps with width/data; its real height bounds the phone
    // scrollport above the sticky bar and keyboard.
    const measure = () => scroll.style.setProperty('--nv-footer-height', `${footer.getBoundingClientRect().height}px`)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(footer)
    return () => { observer.disconnect(); scroll.style.removeProperty('--nv-footer-height') }
  }, [mobile])
  const debounced = useDebounced(text, PLAN_DEBOUNCE_MS)
  const urls = useMemo(() => parseUrls(text), [text])
  const channelsRes = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const templatesRes = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const status = useStatus()
  const channels = channelsRes.data?.channels ?? []
  const channel = channels.find((c) => c.id === channelId) ?? channels[0]
  const multi = mode === 'multi'
  const inputPriority = priority ? 'pick' : pickPriority(urls)
  // Chọn chế độ là nguồn sự thật: "Một link" giữ link đầu, "Nhiều link" giữ
  // nguyên văn. Dán nhiều link khi đang ở chế độ một link thì tự chuyển sang
  // nhiều link — nút chọn và ô nhập không bao giờ lệch trạng thái.
  const pickMode = (next: 'single' | 'multi') => {
    if (next === mode) return
    setMode(next)
    setPlan(null)
    if (next === 'single') setText((value) => parseUrls(value)[0] ?? '')
  }
  const applyText = (next: string) => {
    setText(next)
    if (mode === 'single' && parseUrls(next).length > 1) setMode('multi')
  }
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
  // Kênh chưa đặt tên (để trống lúc tạo, nhận diện sẽ đặt) vẫn phải có nhãn — ô chọn cũ in ra chuỗi rỗng.
  const channelOptions = channels.map((c) => ({ value: c.id, label: c.name || t.channels.page.noName }))
  const templateOptions = [{ value: '', label: `${tt.defaultTemplate}${channel?.defaultTemplateName ? ` · ${channel.defaultTemplateName}` : ''}` }, ...(templatesRes.data?.items ?? []).map((tpl) => ({ value: tpl.id, label: tpl.name }))]
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
    text: tt.plan.mobileBatchTail(channel.perDay, channel.name, current.paceLeftToday),
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
    <main ref={scrollRef} className={`nv ${stylex.props(styles.root, state === 'empty' && styles.emptyRoot).className}`} data-input={state}>
      <div className={`nv-hero ${stylex.props(styles.hero).className}`}>
        <div className={`nv-orbs ${stylex.props(styles.orbs).className}`} aria-hidden="true">
          {/* Artboard NewVideo: ống kính xoay 9s, 4 orb trôi lên xuống 4px lệch nhịp 1,5s (MNew đứng yên). */}
          <Orb of="scout" size={mobile ? 18 : 34} className={stylex.props(styles.float, styles.float0).className} /><Orb of="director" size={mobile ? 18 : 34} className={stylex.props(styles.float, styles.float1).className} /><span className={`nv-lens ${stylex.props(styles.lens).className}`}><LensLogo size={76} spinning /></span><Orb of="artist" size={mobile ? 18 : 34} className={stylex.props(styles.float, styles.float2).className} /><Orb of="qa" size={mobile ? 18 : 34} className={stylex.props(styles.float, styles.float3).className} />
        </div>
        <h1 {...stylex.props(styles.heading)}>{tt.hero}</h1><p {...stylex.props(styles.lede)}>{tt.heroSub}</p>
      </div>
      <Segmented label={tt.modesLabel} value={mode} onChange={(id) => pickMode(id === 'multi' ? 'multi' : 'single')}
        size={mobile ? 'xl' : 'md'} fit={mobile ? 'fill' : 'content'} xstyle={styles.segmented}
        options={[
          { id: 'single', label: mobile ? tt.mobileSingle : tt.single, icon: <Icon name="link" size={mobile ? 16 : 15} /> },
          { id: 'multi', label: mobile ? tt.mobileMulti : tt.multi, icon: <Icon name="video-add" size={mobile ? 16 : 15} /> },
        ]} />
      <form className={`nv-card ${stylex.props(styles.liquidCard, styles.card, state === 'error' && styles.cardError).className}`} onSubmit={submit}>
        <label htmlFor="nv-url" {...stylex.props(textStyles.srOnly)}>{tt.textarea}</label>
        <div className={`nv-entry ${stylex.props(styles.entry, multi && styles.entryMulti).className}`}>
          {!multi && <Icon name="link" size={18} xstyle={styles.entryIcon} />}
          {multi ? <textarea {...stylex.props(styles.input, styles.multiInput)} id="nv-url" rows={4} value={text} aria-label={tt.textarea}
            placeholder={tt.multiPlaceholder} onChange={(e) => applyText(e.target.value)} />
            : <input {...stylex.props(styles.input)} id="nv-url" type="url" inputMode="url" value={text} aria-label={tt.textarea}
              placeholder={mobile ? tt.singlePlaceholder : tt.desktopPlaceholder}
              onChange={(e) => applyText(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void submit(e) } }} />}
          {text ? <Button type="button" variant="icon-only" size="lg" xstyle={[styles.clear, multi && styles.clearMulti]} aria-label={tt.clearInput} onClick={() => { applyText(''); setPlan(null) }}><Icon name="close" size={16} /></Button>
            : mobile && <Button type="button" variant="primary" size="lg" xstyle={styles.paste} onClick={async () => { try { applyText(await navigator.clipboard.readText()) } catch { document.getElementById('nv-url')?.focus() } }}>{tt.paste}</Button>}
        </div>
        {preview && !multi && <div className={`nv-preview ${stylex.props(styles.preview).className}`}>
          <div className={`nv-pimg ${stylex.props(styles.previewImage).className}`}>{preview.thumbnailUrl && <img {...stylex.props(styles.image)} src={preview.thumbnailUrl} alt="" />}<span className={`nv-score num ${stylex.props(styles.score, styles.previewScore, onlyDuplicate && styles.duplicateScore).className}`}>{fmtScore(preview.outlier ?? null)}</span><span className={`nv-dur mono ${stylex.props(styles.duration).className}`}>{fmtDuration(preview.durationSec)}</span></div>
          <div className={`nv-preview-body ${stylex.props(styles.previewBody).className}`}><b {...stylex.props(styles.previewTitle)}>{preview.title}</b><span className={`nv-meta ${stylex.props(styles.meta).className}`}>{[preview.sourceChannelTitle, videoMeta(preview.viewCount ?? null, preview.publishedAt ?? null)].filter(Boolean).join(' · ')}</span><div className={`nv-preview-badges ${stylex.props(styles.previewBadges).className}`}>{preview.hasCaptions && <span className={`nv-captions ${stylex.props(styles.captions).className}`}><Icon name="subtitle" size={13} />{t.screens.explore.hasSubs}</span>}{first?.rivalLinked && <span className={`nv-linked ${stylex.props(styles.linked).className}`}>{tt.linkedSource}</span>}</div></div>
        </div>}
        {onlyDuplicate && !multi && <div className={`nv-banner nv-banner--duplicate ${stylex.props(styles.banner, styles.bannerDuplicate).className}`} role="status"><Icon name="flag" size={16} /><span {...stylex.props(styles.bannerText, styles.duplicateText)}>{tt.dup.banner(duplicate?.title ?? duplicate?.url ?? '')}{tt.dup.bannerState[duplicate?.state ?? ''] ?? '.'}</span><a {...stylex.props(styles.duplicateLink)} href={`#${duplicate?.jobId ? paths.workbench({ video: duplicate.jobId }) : paths.queue()}`}>{tt.dup.open}</a></div>}
        {invalid && !multi && <div className={`nv-banner ${stylex.props(styles.banner).className}`} role="alert"><Icon name="danger" size={16} /><span {...stylex.props(styles.bannerText)}>{tt.invalid}</span></div>}
        {multi && current && <div className={`nv-summary ${stylex.props(styles.summary).className}`}><span className={`nv-chip nv-chip--ok ${stylex.props(styles.chip, styles.chipOk).className}`}><Icon xstyle={styles.summaryIcon} name="tick-b" size={15} />{tt.accepted(current.accepted.length)}</span>{current.duplicates.length > 0 && <span className={`nv-chip nv-chip--warn ${stylex.props(styles.chip, styles.chipWarn).className}`}><Icon xstyle={styles.summaryIcon} name="flag" size={15} />{tt.plan.skipDup(current.duplicates.length)}</span>}{current.invalid.length > 0 && <span className={`nv-chip ${stylex.props(styles.chip, styles.chipPlain).className}`}>{tt.plan.skipInvalid(current.invalid.length)}</span>}{line?.detail && <span className={`nv-chip ${stylex.props(styles.chip, styles.chipPlain).className}`}>{line.detail}</span>}</div>}
        <div className={`nv-options ${stylex.props(styles.options, state === 'empty' && styles.emptyHidden).className}`}>
          <ResponsiveSelect aria-label={tt.channel} value={channel?.id ?? ''} options={channelOptions} onChange={(id) => { setChannelId(id); setTemplateId('') }}
            icon={<Icon name="channel" size={mobile ? 17 : 14} />} prefix={<span {...stylex.props(styles.optionLabel)}>{tt.channel}</span>}
            chevronSize={mobile ? 15 : 12} xstyle={styles.optionSelect} triggerStyle={styles.optionTrigger} />
          <ResponsiveSelect aria-label={tt.template} value={templateId} options={templateOptions} onChange={setTemplateId}
            icon={<Icon name="layer" size={mobile ? 17 : 14} />} prefix={<span {...stylex.props(styles.optionLabel)}>{tt.template}</span>}
            chevronSize={mobile ? 15 : 12} xstyle={styles.optionSelect} triggerStyle={styles.optionTrigger} />
          {mobile && multi && <div className={`nv-option nv-priority-row ${stylex.props(styles.option, styles.priorityRow).className}`}><Icon name="arrow-up" size={17} xstyle={styles.priorityIcon} /><span {...stylex.props(styles.priorityText)}><span>{tt.priority(acceptedCount)}</span><span {...stylex.props(styles.prioritySubtitle)} title={tt.priorityHint}>{tt.priorityHint}</span></span><Switch size={[44, 26]} checked={priority} onChange={setPriority} aria-label={tt.priority(acceptedCount)} /></div>}
        </div>
        {!!templatesRes.error && <Banner tone="error" title={tt.templatesFailed} action={{ label: tt.retryTemplates, onClick: () => void templatesRes.refresh() }} />}
        <div ref={footerRef} className={`nv-plan ${stylex.props(styles.plan).className}`}>
          <span className={`nv-plantext ${stylex.props(styles.planText, state === 'empty' && styles.emptyHidden).className}`}><Icon xstyle={styles.noShrink} name="clock" size={15} /><span>{phase === 'failed' ? <button type="button" onClick={() => setRetry((v) => v + 1)}>{tt.plan.failed}</button> : phase === 'loading' ? tt.plan.fixing : line?.text || tt.empty.pickFirst}</span></span>
          {!mobile && multi && <span className={`nv-priority ${stylex.props(styles.priority).className}`}><Switch size={[26, 15]} checked={priority} onChange={setPriority} aria-label={tt.priority(acceptedCount)} />{tt.priority(acceptedCount)}</span>}
          <ActionButton type="submit" variant="primary" className="nv-submit" xstyle={[styles.submit, disabled && styles.submitLocked]} disabled={busy} lockedReason={disabled ? !channel ? tt.noChannel : current && !current.template.id ? tt.plan.blockedNoTemplate : tt.needLink : undefined}><span {...stylex.props(styles.submitLabel)}>{multi && current?.accepted.length ? tt.makeMany(current.accepted.length) : tt.makeVideo}<span className={`nv-key ${stylex.props(styles.key).className}`}>Enter</span></span></ActionButton>
        </div>
      </form>
      <section className={`nv-picks ${stylex.props(styles.picks, state !== 'empty' && styles.picksHidden).className}`}>
        <div className={`nv-pickhead ${stylex.props(styles.pickHead).className}`}><span {...stylex.props(styles.pickLabel)}>{mobile ? tt.mobileSuggestions : tt.suggestions}</span><a {...stylex.props(styles.pickLink)} href={`#${paths.explore()}`}>{tt.openExplore}<Icon name="arrow-r" size={13} /></a></div>
        <div className={`nv-ideas ${stylex.props(styles.ideas).className}`}>{(suggestions.data?.videos ?? []).map((v) => <button key={v.id} type="button" className={`nv-idea ${stylex.props(styles.idea).className}`} onClick={() => applyText(v.url)}><span className={`nv-ideaimg ${stylex.props(styles.ideaImage).className}`}><img {...stylex.props(styles.image)} src={v.thumbnailUrl} alt="" /><span className={`nv-score num ${stylex.props(styles.score).className}`}>{fmtScore(v.score)}</span><span className={`nv-dur mono ${stylex.props(styles.duration).className}`}>{fmtDuration(v.durationSec)}</span></span><b {...stylex.props(styles.ideaTitle)}>{v.title}</b><span {...stylex.props(styles.ideaMeta)}>{videoMeta(v.viewCount, v.publishedAt)}</span></button>)}</div>
      </section>
      <div className={`nv-workflow ${stylex.props(styles.workflow).className}`}><span {...stylex.props(styles.workflowLabel)}>{tt.whatNext}</span>{agents.map((a, i) => <span className={`nv-workchip ${stylex.props(styles.workChip).className}`} key={a}><Orb of={a} size={14} />{tt.steps[i]}</span>)}</div>
      <div className={`nv-alts ${stylex.props(styles.alts).className}`}>{tt.alts.map((a, i) => <a {...stylex.props(styles.alt)} key={a.title} href={`#${i === 0 ? paths.explore() : i === 1 ? channel ? paths.channel(channel.id) : paths.channelNew() : paths.library()}`}><Icon xstyle={styles.altIcon} name={i === 0 ? 'radar' : i === 1 ? 'refresh' : 'layer'} size={17} /><span {...stylex.props(styles.altText)}><b {...stylex.props(styles.altTitle)}>{a.title}</b><span {...stylex.props(styles.altBody)}>{a.body}</span></span></a>)}</div>
      {!mobile && <div className={`nv-shortcuts ${stylex.props(styles.shortcuts).className}`}><span {...stylex.props(styles.shortcut)}><kbd {...stylex.props(styles.shortcutKey)}>Tab</kbd>{tt.modeHint}</span><span {...stylex.props(styles.shortcut)}><kbd {...stylex.props(styles.shortcutKey)}>Enter</kbd>{tt.submitHint}</span></div>}
    </main>
  )
}

const MOBILE = '@media (max-width: 767px)'
const orbFloat = stylex.keyframes({ '0%, 100%': { transform: 'translateY(0)' }, '50%': { transform: 'translateY(-4px)' } })
const styles = stylex.create({
  liquidCard: { backgroundColor: "var(--card-bg)", borderWidth: "1px", borderStyle: "solid", borderColor: "var(--line-2)", boxShadow: 'var(--card-shadow)' },
  root: { flex: { default: 1, [MOBILE]: 'none' }, overflowY: 'auto', width: '100%', maxWidth: 820, margin: '0 auto', display: 'flex', flexDirection: 'column', alignItems: 'stretch', padding: { default: `${s['--vv-s-32']} ${s['--vv-s-20']} ${s['--vv-s-28']}`, [MOBILE]: `${s['--vv-s-4']} ${s['--vv-s-16']} ${s['--vv-s-16']}` }, gap: { default: s['--vv-s-22'], [MOBILE]: s['--vv-s-16'] }, minHeight: { default: null, [MOBILE]: 0 }, height: { default: null, [MOBILE]: 'max(0px, calc(100dvh - var(--m-top) - 52px - var(--nv-footer-height, 93.75px) - max(26px, var(--m-safe-bottom)) - var(--kb, 0px)))' }, marginBottom: { default: null, [MOBILE]: 'calc(var(--nv-footer-height, 93.75px) + max(26px, var(--m-safe-bottom)) + var(--kb, 0px))' }, lineHeight: { default: null, [MOBILE]: 1.5 } },
  emptyRoot: { '--nv-footer-height': pipeline.newEmptyFooterClearance },
  // Nhóm chọn chế độ không được co trong cột cuộn (kit-seg có overflow:auto nên min-size = 0).
  segmented: { flexShrink: 0, alignSelf: 'center', width: { default: null, [MOBILE]: '100%' }, margin: { default: null, [MOBILE]: 0 } },
  hero: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: { default: s['--vv-s-12'], [MOBILE]: s['--vv-s-10'] }, textAlign: 'center', paddingTop: { default: null, [MOBILE]: s['--vv-s-4'] } },
  orbs: { display: 'flex', alignItems: 'center', gap: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-6'] }, padding: { default: null, [MOBILE]: 0 } },
  lens: { display: { default: 'flex', [MOBILE]: 'none' }, margin: `0 ${s['--vv-s-6']}` },
  float: { animationName: { default: orbFloat, [MOBILE]: 'none' }, animationDuration: '6s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite' },
  float0: { animationDelay: '0s' },
  float1: { animationDelay: '-1.5s' },
  float2: { animationDelay: '-3s' },
  float3: { animationDelay: '-4.5s' },
  heading: { margin: 0, fontSize: { default: 30, [MOBILE]: 22 }, fontWeight: 600, letterSpacing: '-.02em', lineHeight: { default: 1.2, [MOBILE]: 1.5 } },
  lede: { margin: 0, maxWidth: 640, color: color['--color-fg-3'], fontSize: 14, textWrap: 'pretty', display: { default: null, [MOBILE]: 'none' } },
  card: { width: '100%', borderRadius: radius['--radius-dock'], padding: `${s['--vv-s-16']} ${s['--vv-s-16']} ${s['--vv-s-14']} ${s['--vv-s-18']}`, display: { default: 'flex', [MOBILE]: 'contents' }, flexDirection: 'column', gap: s['--vv-s-14'], backgroundColor: "var(--vv-clone-card-bg)", borderColor: line['--line-3'] },
  cardError: { borderColor: `color-mix(in srgb, ${color['--color-err']} 35%, transparent)` },
  entry: { display: 'flex', alignItems: 'center', gap: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-8'] }, color: 'var(--vv-fg-note)', width: { default: null, [MOBILE]: '100%' }, height: { default: null, [MOBILE]: 58 }, minHeight: { default: null, [MOBILE]: 58 }, padding: { default: null, [MOBILE]: `${s['--vv-s-4']} ${s['--vv-s-4']} ${s['--vv-s-4']} ${s['--vv-s-14']}` }, borderWidth: { default: null, [MOBILE]: "1px" }, borderStyle: { default: null, [MOBILE]: "solid" }, borderColor: { default: null, [MOBILE]: line['--line-2'] }, borderRadius: { default: null, [MOBILE]: radius['--radius-card'] }, backgroundColor: { default: null, [MOBILE]: "var(--vv-clone-card-bg)" }, backgroundImage: { default: null } },
  entryMulti: { height: { default: null, [MOBILE]: 'auto' }, minHeight: { default: null, [MOBILE]: 0 }, flexShrink: { default: null, [MOBILE]: 0 }, display: { default: null, [MOBILE]: 'block' } },
  entryIcon: { display: { default: null, [MOBILE]: 'none' } },
  input: { flex: 1, width: '100%', minWidth: 0, height: { default: 40, [MOBILE]: 48 }, resize: 'none', padding: `${s['--vv-s-1']} ${s['--vv-s-2']}`, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", outlineWidth: 0, outlineStyle: "none", outlineColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", color: { default: color['--color-fg'], "::placeholder": 'var(--vv-fg-note)' }, fontFamily: 'inherit', fontSize: 15, lineHeight: { default: 1.55, [MOBILE]: 1.5 } },
  multiInput: { height: { default: 100, [MOBILE]: 'auto' }, padding: { default: s['--vv-s-2'], [MOBILE]: `${s['--vv-s-10']} ${s['--vv-s-10']} ${s['--vv-s-10']} 0` }, lineHeight: { default: '24px', [MOBILE]: 1.6 }, fontSize: { default: 15, [MOBILE]: 14 } },
  // Nút xoá dùng Button icon-only của kit (44px trên điện thoại); chỉ đổi hình tròn + mực phụ.
  clear: { width: 44, borderRadius: '50%', color: 'var(--vv-fg-note)' },
  clearMulti: { display: 'none' },
  // Button primary cỡ lg của kit (44px) — không tự dựng nút Dán.
  paste: { flexShrink: 0 },
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
  optionLabel: { color: { default: 'var(--vv-fg-note)', [MOBILE]: color['--color-fg-3'] }, fontWeight: 400 },
  // NewVideo.dc.html › chip: 30px, viền 1px, chữ 12.5, giá trị đậm 500; MNew.dc.html › hàng 52px
  // trong thẻ, không viền, kẻ dưới 1px, nhãn bên trái, giá trị dồn phải.
  optionSelect: { width: { default: 'auto', [MOBILE]: '100%' } },
  optionTrigger: { height: 30, minHeight: { default: null, [MOBILE]: 52 }, paddingLeft: { default: s['--vv-s-8'], [MOBILE]: s['--vv-s-14'] }, paddingRight: { default: s['--vv-s-10'], [MOBILE]: s['--vv-s-14'] }, gap: { default: s['--vv-s-6'], [MOBILE]: s['--vv-s-12'] }, borderRadius: { default: radius['--radius-control'], [MOBILE]: 0 }, borderWidth: { default: "1px", [MOBILE]: 0 }, borderBottomWidth: { default: null, [MOBILE]: "1px" }, borderBottomStyle: { default: null, [MOBILE]: "solid" }, borderBottomColor: { default: null, [MOBILE]: line['--line-1'] }, backgroundColor: "transparent", color: color['--color-fg'], fontSize: { default: 12.5, [MOBILE]: 14 }, fontWeight: { default: 500, [MOBILE]: 400 }, whiteSpace: 'nowrap', maxWidth: { default: s['--vv-s-250'], [MOBILE]: 'none' }, textAlign: { default: null, [MOBILE]: 'right' } },
  plan: { display: 'flex', flexWrap: 'wrap', alignItems: { default: 'center', [MOBILE]: 'stretch' }, gap: { default: `${s['--vv-s-8']} ${s['--vv-s-12']}`, [MOBILE]: s['--vv-s-8'] }, marginTop: { default: `calc(-1 * ${s['--vv-s-2']})`, [MOBILE]: 0 }, position: { default: null, [MOBILE]: 'fixed' }, zIndex: { default: null, [MOBILE]: 20 }, left: { default: null, [MOBILE]: 0 }, right: { default: null, [MOBILE]: 0 }, bottom: { default: null, [MOBILE]: 'calc(max(26px, var(--m-safe-bottom)) + var(--kb))' }, width: { default: null, [MOBILE]: '100%' }, flexDirection: { default: null, [MOBILE]: 'column' }, padding: { default: null, [MOBILE]: `${s['--vv-s-10']} ${s['--vv-s-16']} ${s['--vv-s-4']}` }, borderTopWidth: { default: null, [MOBILE]: "1px" }, borderTopStyle: { default: null, [MOBILE]: "solid" }, borderTopColor: { default: null, [MOBILE]: line['--line-1'] }, backgroundColor: { default: null, [MOBILE]: `color-mix(in srgb, ${color['--color-bg']}, ${color['--color-surface-0']})` }, backgroundImage: { default: null } },
  planText: { flex: { default: '1 1 320px', [MOBILE]: '0 0 auto' }, minWidth: 0, display: 'flex', gap: s['--vv-s-8'], alignItems: { default: 'center', [MOBILE]: 'flex-start' }, fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5, padding: { default: null, [MOBILE]: `0 ${s['--vv-s-2']} ${s['--vv-s-4']}` } },
  noShrink: { flexShrink: 0 },
  submit: { flex: '0 0 auto', height: { default: 36, [MOBILE]: 48 }, minWidth: 0, paddingBlock: 0, paddingInlineStart: { default: s['--vv-s-14'], [MOBILE]: s['--vv-s-16'] }, paddingInlineEnd: { default: s['--vv-s-12'], [MOBILE]: s['--vv-s-16'] }, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", borderRadius: { default: radius['--radius-button'], [MOBILE]: pipeline.newSubmitRadius }, fontSize: { default: 13.5, [MOBILE]: 15 }, gap: s['--vv-s-8'], width: { default: null, [MOBILE]: '100%' }, lineHeight: { default: null, [MOBILE]: 1.5 } },
  submitLabel: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'] },
  submitLocked: { opacity: 1, backgroundColor: color['--color-surface-7'], color: 'var(--vv-fg-note)' },
  key: { fontFamily: font['--font-mono'], fontSize: 11, padding: `${s['--vv-s-1']} ${s['--vv-s-5']}`, borderWidth: "1px", borderStyle: "solid", borderColor: "var(--vv-kbd-line)", borderRadius: r['--vv-r-4'], backgroundColor: "var(--vv-kbd-bg)", lineHeight: 1.55, display: { default: null, [MOBILE]: 'none' } },
  priority: { height: 30, padding: `0 ${s['--vv-s-10']} 0 ${s['--vv-s-8']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-3'], borderRadius: radius['--radius-control'], backgroundColor: "transparent", backgroundImage: "none", color: color['--color-fg-3'], fontSize: 12.5 },
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
  priorityRow: { backgroundColor: "transparent", backgroundImage: "none", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", color: color['--color-fg'], fontSize: 14, textAlign: 'left', height: { default: null, [MOBILE]: 'auto' }, minHeight: { default: null, [MOBILE]: 52 }, padding: { default: null, [MOBILE]: `0 ${s['--vv-s-14']}` }, gap: { default: null, [MOBILE]: s['--vv-s-12'] } },
  priorityIcon: { flexShrink: 0, color: 'var(--vv-fg-note)' },
  priorityText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' },
  prioritySubtitle: { fontSize: 12, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
})


