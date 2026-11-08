import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Button, Dialog, Icon, KitSelect, Sheet, toast } from '@noverse/ui'
import * as stylex from '@stylexjs/stylex'
import { color, font, kit, line, radius } from '@noverse/ui/tokens.stylex'
import { pipeline, space as s, typography } from '../../styles/viderse.stylex'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { listChannels } from '../../api/channels'
import { listTemplates } from '../../api/library'
import { enqueueClone, planClone, cloneTarget, type ClonePlan, type CloneEnqueueResult } from '../../api/clone'
import { navigate, paths } from '../../router'
import { useDebounced } from '../../lib/useDebounced'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { parseUrls, PLAN_DEBOUNCE_MS } from './plan'
import type { QuickCreateRequest } from './quickCreate'
import { fmtDuration, fmtScore, videoMeta } from '../explore/fmt'

const invalidLabel = (reason: string) => reason === 'not_youtube' ? 'Link không phải video YouTube' : reason === 'private' ? 'Video riêng tư hoặc không truy cập được' : 'Không đọc được video'

/** Mounted once per opening by AppShell, not a routed page. */
export function NewVideoDialog({ request, onClose }: { request: QuickCreateRequest; onClose: () => void }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const [text, setText] = useState((request.urls ?? []).join('\n'))
  const [channelId, setChannelId] = useState(request.channelId ?? '')
  const [templateId, setTemplateId] = useState('')
  const [plan, setPlan] = useState<ClonePlan | null>(null)
  const [planKey, setPlanKey] = useState('')
  const [phase, setPhase] = useState<'idle' | 'loading' | 'failed'>('idle')
  const [retry, setRetry] = useState(0)
  const [result, setResult] = useState<CloneEnqueueResult | null>(null)
  const [busy, setBusy] = useState(false)
  const submitting = useRef(false)
  const [open, setOpen] = useState(true)
  const [picker, setPicker] = useState<'channel' | 'template' | null>(null)
  const handoff = useRef<(() => void) | null>(null)
  const selected = !!request.fromSelection
  const debounced = useDebounced(text, PLAN_DEBOUNCE_MS)
  const urls = useMemo(() => parseUrls(text), [text])
  const channelList = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  useEffect(() => { if (!mobile && !open) onClose() }, [mobile, open, onClose])
  const templateList = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const channels = channelList.data?.channels ?? []
  const channel = channels.find((c) => c.id === channelId) ?? channels[0]
  const resolvedTemplate = templateId || channel?.defaultTemplateId || ''
  const key = JSON.stringify([channel?.id, urls, resolvedTemplate])
  useEffect(() => {
    const pending = parseUrls(debounced)
    if (!channel || !pending.length) { setPlan(null); setPhase('idle'); return }
    const ctrl = new AbortController()
    setPhase('loading')
    setPlanKey(JSON.stringify([channel.id, pending, resolvedTemplate]))
    planClone({ channelId: channel.id, urls: pending, ...(templateId ? { templateId } : {}) }, ctrl.signal)
      .then((next) => { if (!ctrl.signal.aborted) { setPlan(next); setPhase('idle') } })
      .catch(() => { if (!ctrl.signal.aborted) { setPlan(null); setPhase('failed') } })
    return () => ctrl.abort()
  }, [debounced, channel?.id, resolvedTemplate, templateId, retry])
  const current = text === debounced && phase === 'idle' && planKey === key ? plan : null
  const planningFailed = phase === 'failed' && text === debounced && planKey === key
  const planning = !!channel && !!urls.length && !current && !planningFailed
  const channelOptions = channels.map((c) => ({ value: c.id, label: c.name || 'Kênh chưa đặt tên', description: c.defaultTemplateName || 'Chưa có template mặc định' }))
  const templateOptions = [{ value: '', label: channel?.defaultTemplateName || 'Chưa đặt template', description: 'Mặc định kênh' }, ...(templateList.data?.items ?? []).filter((template) => template.id !== channel?.defaultTemplateId).map((template) => ({ value: template.id, label: template.name, description: 'Chỉ video này' }))]
  const changeChannel = (id: string) => { setChannelId(id); setTemplateId(''); setResult(null) }
  const close = () => { if (!submitting.current) setOpen(false) }
  const exited = () => {
    const next = handoff.current
    handoff.current = null
    if (next) next()
    else onClose()
  }
  const pick = (kind: 'channel' | 'template') => {
    handoff.current = () => { setPicker(kind); setOpen(true) }
    setOpen(false)
  }
  const finishPick = (value?: string) => {
    handoff.current = () => {
      if (value !== undefined) { if (picker === 'channel') changeChannel(value); else { setTemplateId(value); setResult(null) } }
      setPicker(null); setOpen(true)
    }
    setOpen(false)
  }
  const submit = async (event?: FormEvent) => {
    event?.preventDefault()
    if (submitting.current || !channel || !current?.accepted.length || !current.template.id) return
    submitting.current = true; setBusy(true)
    try {
      const next = await enqueueClone({ channelId: channel.id, urls, ...(templateId ? { templateId } : {}) })
      setResult(next)
      if (next.items.length) {
        toast(`Đã thêm ${next.items.length} video vào đầu hàng chờ của ${channel.name || 'kênh'}${templateId ? ` · ${current.template.name}` : ''}.`, 'ok', { label: 'Việc nền', onClick: () => navigate(paths.queue()) })
        const existing = next.duplicates?.find((item) => cloneTarget(item))
        if (existing) toast(`${next.duplicates?.length} video đã có cho kênh này.`, 'warn', { label: 'Mở ở Bàn dựng', onClick: () => navigate(cloneTarget(existing)!) })
        // Invalid lines remain editable; successful selections close back to their underlay.
        if (!next.invalid?.length) setOpen(false)
        else setRetry((v) => v + 1)
      } else if (!next.duplicates?.length && !next.invalid?.length) toast.err('Chưa thêm được video. Hãy thử lại.')
    } catch { toast.err('Không thêm được video. Nội dung của bạn vẫn được giữ lại để thử lại.') }
    finally { submitting.current = false; setBusy(false) }
  }
  const singlePreview = urls.length === 1 && current?.accepted.length === 1
  const subtitle = selected ? `Mang ${urls.length} video đã chọn ở Khám phá` : 'Dán link video nguồn trên YouTube. Kênh và template điền sẵn, đổi được cho riêng lần này.'
  const clear = () => { setText(''); setResult(null) }
  const paste = async () => {
    try { setText(await navigator.clipboard.readText()); setResult(null) }
    catch { toast.err('Không đọc được bộ nhớ tạm. Bạn có thể dán link vào ô nhập.') }
  }
  const planText = channelList.error ? 'Không tải được danh sách kênh. Hãy thử lại.'
    : channelList.initialLoading ? 'Đang tải danh sách kênh…'
    : !channel ? 'Tạo kênh để làm video từ nguồn YouTube.'
    : !urls.length ? 'Dán link video nguồn trên YouTube hoặc chọn video ở Khám phá để bắt đầu.'
    : planningFailed ? 'Không đọc được thông tin video. Hãy thử đọc lại.'
    : planning ? 'Đang đọc thông tin video…'
    : current?.accepted.length
      ? `${current.etaStart ? `Lên đầu hàng chờ · bắt đầu khoảng ${new Date(current.etaStart).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}, khi có luồng trống.` : 'Lên đầu hàng chờ, máy sẽ làm khi có luồng trống.'} Không tính vào nhịp tự lấy của kênh.`
      : current?.duplicates.length && !current.invalid.length ? 'Các video nguồn đã có cho kênh này. Mở ở Bàn dựng hoặc chọn kênh khác.'
      : current?.invalid.length && !current.duplicates.length ? 'Không có link video YouTube đọc được. Kiểm tra hoặc thay link nguồn.'
      : 'Các link nguồn đã có cho kênh này hoặc không đọc được. Kiểm tra từng video bên trên.'
  const header = <header {...stylex.props(sx.header, mobile && sx.mobileHeader)}>
    {mobile ? <Button type="button" size="lg" variant="quiet" aria-label="Đóng" disabled={busy} onClick={close}><Icon name="close" size={16} /></Button> : <span {...stylex.props(sx.headerIcon)}><Icon name="video-add" size={20} /></span>}
    <span {...stylex.props(sx.headerText)}><h2 {...stylex.props(sx.title)}>Làm video</h2><span {...stylex.props(sx.subtitle, mobile && sx.mobileSubtitle)}>{mobile && !selected ? 'Dán link video nguồn trên YouTube' : subtitle}</span></span>
    {!mobile && <Button type="button" variant="quiet" aria-label="Đóng" disabled={busy} onClick={close}><Icon name="close" size={16} /></Button>}
  </header>
  const options = <div {...stylex.props(sx.options, mobile && sx.mobileOptions)}>
    {mobile ? <>
      <button type="button" aria-label="Kênh" aria-haspopup="dialog" disabled={busy} {...stylex.props(sx.mobileOption)} onClick={() => pick('channel')}><Icon name="channel" size={18} /><span {...stylex.props(sx.optionLabel)}>Kênh</span><span {...stylex.props(sx.optionValue)}>{channel?.name || 'Chọn kênh'}</span><Icon name="arrow-d" size={16} /></button>
      <button type="button" aria-label="Template" aria-haspopup="dialog" disabled={busy} {...stylex.props(sx.mobileOption, sx.optionDivider)} onClick={() => pick('template')}><Icon name="layer" size={18} /><span {...stylex.props(sx.optionLabel)}>Template</span><span {...stylex.props(sx.optionValue)}>{templateId ? templateOptions.find((o) => o.value === templateId)?.label : current?.template.name || templateList.data?.items.find((item) => item.id === channel?.defaultTemplateId)?.name || 'Mặc định kênh'}</span><Icon name="arrow-d" size={16} /></button>
    </> : <>
      <KitSelect aria-label="Kênh" prefix={<span {...stylex.props(sx.optionLabel)}>Kênh</span>} value={channel?.id ?? ''} options={channelOptions} onChange={changeChannel} disabled={busy} xstyle={sx.channelOption} triggerStyle={sx.selectTrigger} />
      <span {...stylex.props(sx.templateOption)}><KitSelect aria-label="Template" prefix={<span {...stylex.props(sx.optionLabel)}>Template</span>} value={templateId} options={templateOptions} optionDescriptions onChange={(id) => { setTemplateId(id); setResult(null) }} disabled={busy} xstyle={sx.selectFill} triggerStyle={sx.selectTrigger} valueStyle={sx.templateValue} /><span aria-hidden="true" {...stylex.props(sx.templateTag)}>{templateId ? 'chỉ video này' : 'mặc định'}</span></span>
    </>}
  </div>
  const body = <div {...stylex.props(sx.body, mobile && sx.mobileBody)}>
    {!selected && <div {...stylex.props(sx.inputGroup)}><label htmlFor="quick-create-urls" {...stylex.props(sx.srOnly)}>Dán link video · mỗi dòng một link</label><div className={`kit-frame ${stylex.props(sx.field, mobile && sx.mobileField).className}`}>
      <textarea id="quick-create-urls" autoFocus rows={urls.length > 1 ? Math.min(urls.length, 4) : 1} value={text} disabled={busy} {...stylex.props(sx.input, mobile && sx.mobileInput)} placeholder="Dán link video nguồn YouTube · mỗi dòng một link" onChange={(event) => { setText(event.target.value); setResult(null) }} />
      {text ? <Button type="button" size={mobile ? 'lg' : 'md'} variant="quiet" aria-label="Xoá link" disabled={busy} onClick={clear}><Icon name="close" size={16} /></Button> : mobile ? <Button type="button" size="lg" disabled={busy} onClick={() => void paste()}>Dán</Button> : null}
    </div>{!urls.length && <a {...stylex.props(sx.exploreLink, mobile && sx.mobileExploreLink)} href={`#${paths.explore()}`} onClick={close}>Chưa có link? Chọn video nổi trội ở Khám phá<Icon name="arrow-r" size={16} /></a>}</div>}
    {!!urls.length && <div aria-live="polite" {...stylex.props(sx.lines, urls.length > 1 && sx.list)}>
      {!singlePreview && urls.length > 1 && <span {...stylex.props(sx.listHead)}>Đã chọn {urls.length} video{current ? ` · ${current.accepted.length} video mới · ${current.duplicates.length} đã có · ${current.invalid.length} link lỗi` : planningFailed ? ' · Chưa đọc được thông tin' : ' · Đang kiểm tra thông tin'}</span>}
      {urls.map((url, index) => {
        const accepted = current?.accepted.find((item) => item.url === url)
        const duplicate = current?.duplicates.find((item) => item.url === url)
        const invalid = current?.invalid.find((item) => item.url === url)
        const preview = accepted ?? duplicate
        if (urls.length === 1 && (invalid || duplicate)) return <div key={url} role={invalid ? 'alert' : 'status'} {...stylex.props(sx.warning)}>
          <Icon name="info-circle" size={20} />
          <div {...stylex.props(sx.grow)}>
            <b>{invalid ? 'Không đọc được link này' : `${channel?.name || 'Kênh này'} đã có video từ link này`}</b>
            <span>{invalid ? invalidLabel(invalid.reason) : duplicate?.title || url}</span>
            {duplicate && cloneTarget(duplicate) && <a {...stylex.props(sx.link)} href={`#${cloneTarget(duplicate)}`} onClick={close}>Mở ở Bàn dựng</a>}
          </div>
        </div>
        return <div key={`${index}-${url}`} {...stylex.props(sx.row, singlePreview && sx.singleRow, singlePreview && mobile && sx.mobileSingleRow)}>
          <PreviewThumbnail url={preview?.thumbnailUrl} single={singlePreview} mobile={mobile}>{singlePreview && preview && <>{preview.outlier != null && <span {...stylex.props(sx.score)}>{fmtScore(preview.outlier)}</span>}<span {...stylex.props(sx.duration)}>{fmtDuration(preview.durationSec)}</span></>}</PreviewThumbnail>
          <span {...stylex.props(sx.grow)}><b {...stylex.props(sx.previewTitle, singlePreview && sx.singleTitle)}>{preview?.title || url}</b><span {...stylex.props(sx.note, sx.ellipsis)}>{invalid ? invalidLabel(invalid.reason) : duplicate ? 'Video đã có cho kênh này' : preview ? `${preview.sourceChannelTitle || ''} · ${videoMeta(preview.viewCount ?? null, preview.publishedAt ?? null)}` : planningFailed ? 'Không tải được thông tin' : !channel ? 'Chọn kênh để đọc thông tin video' : 'Đang đọc thông tin video…'}</span>{preview?.hasCaptions && <span {...stylex.props(sx.caption)}><Icon name="subtitle" size={14} />Có phụ đề do kênh đăng</span>}{accepted?.rivalLinked && !mobile && <span {...stylex.props(sx.note)}>Nguồn đã liên kết với kênh</span>}</span>
          {duplicate && cloneTarget(duplicate) && <a {...stylex.props(sx.link)} href={`#${cloneTarget(duplicate)}`} onClick={close}>Mở ở Bàn dựng</a>}
          {!singlePreview && <Button type="button" size={mobile ? 'lg' : 'md'} variant="quiet" aria-label={`Bỏ ${preview?.title || url}`} disabled={busy} onClick={() => { setText(urls.filter((_, itemIndex) => itemIndex !== index).join('\n')); setResult(null) }}><Icon name="close" size={14} /></Button>}
        </div>
      })}
    </div>}
    {mobile && options}
    {mobile && templateId && <span {...stylex.props(sx.note)}>Chỉ video này · không đổi template mặc định kênh.</span>}
    {!!(channelList.error || templateList.error) && <Button size="lg" onClick={() => { void channelList.refresh(); void templateList.refresh() }}>Không tải được danh sách · Thử lại</Button>}
    {!channel && !channelList.initialLoading && <a {...stylex.props(sx.link)} href={`#${paths.channelNew()}`} onClick={close}>Tạo kênh để làm video</a>}
    {planningFailed && <Button size="lg" onClick={() => setRetry((value) => value + 1)}>Thử đọc lại thông tin</Button>}
    {result && <div role="status" {...stylex.props(sx.lines)}><b>Đã thêm {result.items.length} video</b>{result.items.map((item) => <a key={item.id} {...stylex.props(sx.link)} href={`#${cloneTarget({ jobId: item.jobId, queueItemId: item.id })}`} onClick={close}>Mở video vừa thêm ở Bàn dựng</a>)}{result.duplicates?.map((item, index) => cloneTarget(item) ? <a key={`${item.url}-${index}`} {...stylex.props(sx.link)} href={`#${cloneTarget(item)}`} onClick={close}>Đã có: {item.title || item.url} · Mở ở Bàn dựng</a> : <span key={`${item.url}-${index}`}>Đã có: {item.title || item.url}</span>)}{result.invalid?.map((item, index) => <span key={`${item.url}-${index}`}>{item.url} · {invalidLabel(item.reason)}</span>)}</div>}
  </div>
  const footer = <footer {...stylex.props(sx.footer, mobile && sx.mobileFooter)}>
    <span aria-live="polite" {...stylex.props(sx.plan)}><Icon name="clock" size={16} /><span>{planText}{current?.accepted.length ? <span {...stylex.props(sx.price)}>{current.priceUSD == null ? 'Chưa ước lượng được chi phí' : `Ước tính $${current.priceUSD.toFixed(2)} · ${current.imageCalls} lượt vẽ ảnh`}{!current.template.id && ' · Kênh chưa có template mặc định'}</span> : null}</span></span>
    <span {...stylex.props(sx.actions, mobile && sx.mobileActions)}>
      {!mobile && <><span {...stylex.props(sx.keyboardHint)}><kbd {...stylex.props(sx.kbd)}>Esc</kbd> đóng</span><Button type="button" disabled={busy} onClick={close}>Huỷ</Button></>}
      <Button type="submit" size={mobile ? 'lg' : 'md'} variant="primary" pending={busy} disabled={busy || !current?.accepted.length || !current.template.id} xstyle={mobile && sx.mobileSubmit}>{(current?.accepted.length ?? 0) > 1 ? `Làm ${current?.accepted.length} video` : 'Làm video'}</Button>
    </span>
  </footer>
  // Keep desktop listboxes inside the focus trap but outside the preview
  // scroller; neither that scroller nor the dialog shell may clip their menus.
  const form = <form id="quick-create" onSubmit={submit} {...stylex.props(sx.form, mobile && sx.mobileForm)}>{header}{body}{!mobile && <div {...stylex.props(sx.desktopOptions)}>{options}{templateId && <span {...stylex.props(sx.note)}>Chỉ video này · không đổi template mặc định kênh.</span>}</div>}{footer}</form>
  if (mobile) return <Sheet open={open} onClose={() => picker ? finishPick() : close()} onExited={exited} title={picker === 'channel' ? 'Chọn kênh' : picker === 'template' ? 'Template · chỉ video này' : undefined} label="Làm video" size="md" height={picker ? 'auto' : '100dvh'} grab={!!picker} xstyle={!picker && sx.fullSheet} bodyStyle={!picker && sx.sheetBody} homeIndicator>
    {picker ? <div {...stylex.props(sx.picker)}>{(picker === 'channel' ? channelOptions : templateOptions).map((option) => <Button key={option.value} size="lg" onClick={() => finishPick(option.value)}>{option.label}</Button>)}<Button size="lg" onClick={() => finishPick()}>Quay lại</Button></div> : form}
  </Sheet>
  return open ? <Dialog label="Làm video" width={pipeline.quickDialogWidth} onClose={close} padBody={false} style={{ alignSelf: 'flex-start', marginTop: `calc(${s['--vv-s-76']} - max(${s['--vv-s-16']}, var(--m-safe-top, 0px)))`, maxHeight: `calc(100dvh - ${s['--vv-s-76']} - ${s['--vv-s-20']})`, overflow: 'visible' }}>{form}</Dialog> : null
}

function PreviewThumbnail({ url, single, mobile, children }: { url?: string | null; single: boolean; mobile: boolean; children?: ReactNode }) {
  const [failedUrl, setFailedUrl] = useState<string>()
  return <span aria-hidden="true" {...stylex.props(sx.thumb, single && sx.singleThumb, single && mobile && sx.mobileThumb)}>
    {url && failedUrl !== url ? <img src={url} alt="" {...stylex.props(sx.thumbImage)} onError={() => setFailedUrl(url)} /> : <Icon name="gallery" size={20} />}
    {children}
  </span>
}

const sx = stylex.create({
  form: { display: 'flex', flexDirection: 'column', flex: '1 1 auto', minHeight: 0, maxHeight: 'inherit', boxSizing: 'border-box', padding: s['--vv-s-20'], gap: s['--vv-s-16'] },
  mobileForm: { flex: 1, padding: 0, gap: 0, overflow: 'hidden' },
  header: { display: 'flex', flexShrink: 0, gap: s['--vv-s-12'], alignItems: 'flex-start', minWidth: 0 },
  mobileHeader: { minHeight: pipeline.quickHeaderHeight, flexShrink: 0, alignItems: 'center', paddingInline: s['--vv-s-6'], gap: s['--vv-s-2'] },
  headerIcon: { width: s['--vv-s-42'], height: s['--vv-s-42'], display: 'grid', placeItems: 'center', flexShrink: 0, color: color['--color-accent'], backgroundColor: kit['--kit-acc-10'], borderRadius: radius['--radius-tile'] },
  headerText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  title: { margin: 0, fontSize: s['--vv-s-16'], fontWeight: 600 },
  subtitle: { color: color['--color-fg-3'], fontSize: typography['--vv-fs-ui'], lineHeight: 1.5 },
  mobileSubtitle: { fontSize: typography['--vv-fs-meta'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  body: { display: 'flex', flexDirection: 'column', flex: '1 1 auto', gap: s['--vv-s-16'], minWidth: 0, minHeight: 0, overflowY: 'auto' },
  mobileBody: { flex: 1, gap: s['--vv-s-14'], paddingTop: s['--vv-s-4'], paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-16'] },
  inputGroup: { display: 'flex', flexDirection: 'column', flexShrink: 0, gap: s['--vv-s-6'] },
  field: { display: 'flex', alignItems: 'flex-start', minHeight: s['--vv-s-42'], padding: s['--vv-s-4'], paddingLeft: s['--vv-s-12'], borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-3'], borderRadius: radius['--radius-tile'], backgroundColor: color['--color-surface-0'], ':focus-within': { outlineWidth: s['--vv-s-2'], outlineStyle: 'solid', outlineColor: color['--color-accent'], outlineOffset: `calc(-1 * ${s['--vv-s-2']})` } },
  mobileField: { borderRadius: radius['--radius-card'], paddingLeft: s['--vv-s-14'] },
  input: { flex: 1, width: '100%', minWidth: 0, paddingBlock: s['--vv-s-6'], paddingInline: 0, border: 0, outline: 'none', backgroundColor: 'transparent', color: color['--color-fg'], fontFamily: font['--font-sans'], fontSize: typography['--vv-fs-detail'], lineHeight: 1.6, resize: 'none' },
  mobileInput: { paddingBlock: s['--vv-s-11'], fontSize: typography['--vv-fs-ui'] },
  options: { display: 'flex', flexShrink: 0, gap: s['--vv-s-8'], minWidth: 0 },
  desktopOptions: { display: 'flex', flexDirection: 'column', flexShrink: 0, gap: s['--vv-s-8'], minWidth: 0 },
  channelOption: { flex: 1, minWidth: 0 },
  templateOption: { position: 'relative', flex: 1.5, minWidth: 0 },
  selectFill: { width: '100%', minWidth: 0 },
  selectTrigger: { backgroundColor: color['--color-surface-0'], fontSize: typography['--vv-fs-ui'] },
  templateValue: { marginRight: s['--vv-s-60'] },
  templateTag: { position: 'absolute', right: s['--vv-s-30'], top: '50%', transform: 'translateY(-50%)', pointerEvents: 'none', paddingInline: s['--vv-s-6'], paddingBlock: s['--vv-s-1'], color: color['--color-fg-3'], backgroundColor: kit['--kit-wash-4'], fontSize: typography['--vv-fs-small'], borderRadius: radius['--radius-chip'] },
  optionLabel: { color: color['--color-fg-3'], fontSize: typography['--vv-fs-meta'], flexShrink: 0 },
  mobileOptions: { flexDirection: 'column', gap: 0, borderRadius: radius['--radius-card'], backgroundColor: color['--color-surface-2'], borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], overflow: 'hidden', flexShrink: 0 },
  mobileOption: { display: 'flex', alignItems: 'center', gap: s['--vv-s-12'], width: '100%', minHeight: pipeline.quickSelectorHeight, paddingInline: s['--vv-s-14'], backgroundColor: 'transparent', color: color['--color-fg-3'], border: 0, font: 'inherit', textAlign: 'left', cursor: 'pointer', ':hover': { backgroundColor: kit['--kit-wash-4'] } },
  optionDivider: { borderTopWidth: s['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-2'] },
  optionValue: { flex: 1, minWidth: 0, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis', textAlign: 'right', fontSize: typography['--vv-fs-body'], fontWeight: 500, color: color['--color-fg'] },
  lines: { display: 'flex', flexDirection: 'column', flexShrink: 0, minWidth: 0 },
  list: { borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: radius['--radius-tile'], overflow: 'hidden', flexShrink: 0 },
  warning: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: s['--vv-s-12'], borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: kit['--kit-warn-20'], borderRadius: radius['--radius-tile'], backgroundColor: kit['--kit-warn-6'], color: color['--color-warn'], fontSize: typography['--vv-fs-ui'], lineHeight: 1.5 },
  listHead: { paddingBlock: s['--vv-s-8'], paddingInline: s['--vv-s-12'], backgroundColor: pipeline.quickPreviewWash, fontSize: typography['--vv-fs-detail'], color: color['--color-fg-3'] },
  row: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], paddingBlock: s['--vv-s-8'], paddingInline: s['--vv-s-12'], minWidth: 0, borderBottomWidth: s['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: line['--line-2'] },
  singleRow: { alignItems: 'flex-start', gap: s['--vv-s-12'], padding: s['--vv-s-10'], borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: radius['--radius-tile'], backgroundColor: pipeline.quickPreviewWash },
  mobileSingleRow: { padding: 0, borderWidth: 0, borderRadius: 0, backgroundColor: 'transparent' },
  grow: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  previewTitle: { fontSize: typography['--vv-fs-ui'], fontWeight: 500, lineHeight: 1.35, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  singleTitle: { fontSize: typography['--vv-fs-body'], fontWeight: 600, whiteSpace: 'normal', display: '-webkit-box', WebkitBoxOrient: 'vertical', WebkitLineClamp: 2, overflow: 'hidden' },
  ellipsis: { overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' },
  thumb: { width: s['--vv-s-64'], position: 'relative', display: 'grid', placeItems: 'center', flexShrink: 0, aspectRatio: '16/9', overflow: 'hidden', borderRadius: radius['--radius-chip'], backgroundColor: color['--color-surface-1'], color: color['--color-fg-4'] },
  singleThumb: { width: pipeline.quickPreviewWidth, borderRadius: radius['--radius-button'] },
  mobileThumb: { width: s['--vv-s-128'] },
  thumbImage: { position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' },
  score: { position: 'absolute', left: s['--vv-s-4'], bottom: s['--vv-s-4'], paddingInline: s['--vv-s-5'], backgroundColor: color['--color-surface-0'], color: color['--color-warn'], borderRadius: radius['--radius-chip'], fontSize: typography['--vv-fs-badge'], fontWeight: 600 },
  duration: { position: 'absolute', right: s['--vv-s-4'], bottom: s['--vv-s-4'], paddingInline: s['--vv-s-5'], backgroundColor: color['--color-surface-0'], color: color['--color-fg'], borderRadius: radius['--radius-chip'], fontSize: typography['--vv-fs-badge'], fontFamily: font['--font-sans'], fontVariantNumeric: 'tabular-nums' },
  note: { color: color['--color-fg-3'], fontSize: typography['--vv-fs-meta'], lineHeight: 1.5 },
  caption: { display: 'flex', alignItems: 'center', gap: s['--vv-s-5'], color: color['--color-ok'], fontSize: typography['--vv-fs-meta'] },
  link: { minHeight: s['--vv-s-44'], display: 'inline-flex', alignItems: 'center', color: color['--color-fg-2'], overflowWrap: 'anywhere' },
  exploreLink: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'], fontSize: typography['--vv-fs-detail'], color: color['--color-fg-3'] },
  mobileExploreLink: { minHeight: s['--vv-s-44'], fontSize: typography['--vv-fs-title-small'] },
  footer: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-16'], flexShrink: 0 },
  mobileFooter: { paddingTop: s['--vv-s-10'], paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-4'], gap: s['--vv-s-8'], borderTopWidth: s['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-2'], backgroundColor: 'var(--vv-composer-mobile-bg)' },
  plan: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-8'], color: color['--color-fg-3'], fontSize: typography['--vv-fs-detail'], lineHeight: 1.5 },
  price: { display: 'block', fontSize: typography['--vv-fs-small'] },
  actions: { display: 'flex', alignItems: 'center', justifyContent: 'flex-start', gap: s['--vv-s-8'] },
  mobileActions: { width: '100%' },
  mobileSubmit: { width: '100%' },
  keyboardHint: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-6'], color: color['--color-fg-3'], fontSize: typography['--vv-fs-small'] },
  kbd: { paddingBlock: s['--vv-s-1'], paddingInline: s['--vv-s-5'], borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: s['--vv-s-4'], fontFamily: font['--font-sans'], fontSize: typography['--vv-fs-group'] },
  fullSheet: { '--m-sheet-max-height': '100dvh', borderRadius: 0, backgroundColor: color['--color-bg'], paddingTop: 'var(--m-safe-top, 0px)' },
  sheetBody: { display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, overflow: 'hidden', padding: 0 },
  picker: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: s['--vv-s-16'] },
  srOnly: { position: 'absolute', width: s['--vv-s-1'], height: s['--vv-s-1'], overflow: 'hidden', clipPath: 'inset(50%)', whiteSpace: 'nowrap' },
})

