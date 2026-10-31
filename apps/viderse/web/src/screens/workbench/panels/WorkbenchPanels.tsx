import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { useEscapeLayer } from '@noverse/ui'
import { getChannel, type Channel } from '../../../api/channels'
import { Orb } from '../../../components/Orb'
import type { Assistant } from '../../../components/agents'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { coverageAfterRemoval, metadataCounts, togglePart, type MetadataPart } from './panelLogic'
import type { CloneJobView } from '../../../api/workbench'

import { getMetadata, mutateMetadata, getHistory, getBackdrop, mutateBackdrop, type Metadata, type MetaResponse, type History, type BackdropView } from '../../../api/workbenchPanels'
const labels: Record<MetadataPart, string> = { title: 'Tiêu đề', description: 'Mô tả', tags: 'Tag' }
const agentNames: Record<Assistant, [string, string]> = { scout: ['Trinh sát', 'Tìm và tải video'], director: ['Đạo diễn', 'Viết nội dung và chia cảnh'], artist: ['Hoạ sĩ', 'Vẽ ảnh cho video'], qa: ['Kiểm định', 'Dựng và kiểm tra video'] }
const agentStates: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang làm', done: 'Xong', stopped: 'Dừng', needs_you: 'Cần bạn' }
const fmt = (value: number) => `${Math.floor(value / 60)}:${String(Math.floor(value % 60)).padStart(2, '0')}`

export function WorkbenchPanels({ job, onRefresh }: { job: CloneJobView; onRefresh: () => void }) {
  const [tab, setTab] = useState<'agents' | 'content' | 'source' | 'backdrop'>('agents')
  const [meta, setMeta] = useState<MetaResponse | null>(null)
  const [channel, setChannel] = useState<Channel | null>(null)
  const [backdrop, setBackdrop] = useState<BackdropView | null>(null)
  const [history, setHistory] = useState<History[] | null>(null)
  const [rewrite, setRewrite] = useState(false)
  const [parts, setParts] = useState<MetadataPart[]>(['title'])
  const [direction, setDirection] = useState('')
  const [why, setWhy] = useState(false)
  const [whyDescription, setWhyDescription] = useState(false)
  const [whyTags, setWhyTags] = useState(false)
  const reasonAnchor = useRef<HTMLDivElement>(null)
  useEscapeLayer(why || whyDescription || whyTags, () => { setWhy(false); setWhyDescription(false); setWhyTags(false) })
  useEffect(() => {
    if (!why) return
    const dismiss = (event: PointerEvent) => { if (!reasonAnchor.current?.contains(event.target as Node)) setWhy(false) }
    document.addEventListener('pointerdown', dismiss)
    return () => document.removeEventListener('pointerdown', dismiss)
  }, [why])
  const [error, setError] = useState('')
  const [metaError, setMetaError] = useState('')
  const [backdropError, setBackdropError] = useState('')
  const [pending, setBusy] = useState(false)
  const [offline, setOffline] = useState(!navigator.onLine)
  const [loadingBackdrop, setLoadingBackdrop] = useState(true)
  const [retry, setRetry] = useState(0)
  const dirty = useRef(false)
  const lastJob = useRef(job.id)
  const busy = pending || offline
  useEffect(() => {
    const update = () => setOffline(!navigator.onLine)
    window.addEventListener('online', update); window.addEventListener('offline', update)
    return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update) }
  }, [])
  const [tag, setTag] = useState('')
  useEffect(() => {
    const controller = new AbortController()
    if (lastJob.current !== job.id) { lastJob.current = job.id; dirty.current = false; setMeta(null); setBackdrop(null); setHistory(null) }
    setError(''); setMetaError(''); setBackdropError(''); setLoadingBackdrop(true)
    getMetadata(job.id, controller.signal).then(value => setMeta(current => dirty.current && current ? { ...value, metadata: current.metadata } : value)).catch(e => { if (!controller.signal.aborted) setMetaError(e.message) })
    getBackdrop(job.id, controller.signal).then(setBackdrop).catch(e => { if (!controller.signal.aborted) setBackdropError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoadingBackdrop(false) })
    getChannel(job.channelId, controller.signal).then(setChannel).catch(() => { if (!controller.signal.aborted) setChannel(null) })
    return () => controller.abort()
  }, [job.id, job.channelId, job.version, retry])
  async function run(fn: () => Promise<void>) { setBusy(true); setError(''); try { await fn() } catch (e) { setError(e instanceof Error ? e.message : 'Không thực hiện được. Vui lòng thử lại.') } finally { setBusy(false) } }
  async function save(patch: Partial<Metadata>) { await run(async () => { const result = await mutateMetadata(job.id, '', { ...patch, expectedRevision: meta?.revision }); dirty.current = false; setMeta(result); onRefresh() }) }
  async function loadHistory() { if (history) { setHistory(null); return } await run(async () => { setHistory(await getHistory(job.id)) }) }
  async function editBackdrop(op: 'remove' | 'reroll' | 'add', itemId?: string) {
    if (!backdrop || backdrop.busy || job.activeStep === 'build' || job.steps?.some(step => step.step === 'build' && step.status === 'running')) return
    const old = backdrop
    if (op === 'remove' && itemId) {
      const item = old.items.find(x => x.id === itemId)
      const fixed = old.fixedItems?.find(x => x.id === itemId)
      if (item) setBackdrop({ ...old, items: old.items.filter(x => x.id !== itemId), readyCount: old.readyCount - (item.status === 'ready' ? 1 : 0), coverage: { ...old.coverage, channelSec: Math.max(0, old.coverage.channelSec - (item.status === 'ready' ? (item.endSec - item.startSec) / (old.playbackSpeed || 1) : 0)) } })
      if (fixed) setBackdrop({ ...old, fixedItems: (old.fixedItems ?? []).filter(x => x.id !== itemId), coverage: { ...old.coverage, fixedSec: Math.max(0, old.coverage.fixedSec - fixed.playbackSec) } })
    }
    await run(async () => { try { await mutateBackdrop(job.id, { op, revision: old.revision, ...(itemId ? { itemId } : {}) }); setBackdrop(await getBackdrop(job.id)); onRefresh() } catch (e) { setBackdrop(old); throw e } })
  }
  const data = meta?.metadata
  const footer = meta?.descriptionFooter ?? channel?.descriptionFooter ?? ''
  const footerSuffix = footer ? `\n\n${footer}` : ''
  const editableDescription = data?.description.endsWith(footerSuffix) && footerSuffix ? data.description.slice(0, -footerSuffix.length) : data?.description ?? ''
  const counts = data && metadataCounts(data.title, data.description, data.tags)
  const building = job.activeStep === 'build' || Boolean(job.steps?.some(step => step.step === 'build' && step.status === 'running'))
  const locked = Boolean(backdrop?.busy || building)
  const coverage = backdrop && coverageAfterRemoval(backdrop.coverage, 0)
  const panelError = error || (tab === 'content' ? metaError : tab === 'backdrop' ? backdropError : '')
  return <aside {...stylex.props(s.root)} aria-label="Bảng bên phải">
    <div {...stylex.props(s.tabs)} role="tablist" aria-label="Bảng bên phải">{([['agents','Trợ lý'], ['content','Nội dung'], ['source','Nguồn'], ['backdrop','Nền']] as const).map(([id, label]) => <button key={id} type="button" role="tab" aria-selected={tab === id} {...stylex.props(s.tab, tab === id && s.tabSelected)} onClick={() => setTab(id)}>{label}</button>)}</div>
    <div {...stylex.props(s.panel, tab === 'agents' && s.panelAgents)} role="tabpanel">
      {panelError && <div role="alert" {...stylex.props(s.notice)}>{panelError} <button {...stylex.props(s.link)} onClick={() => setRetry(value => value + 1)}>Thử lại ngay</button></div>}
      {tab === 'agents' && (['scout','director','artist','qa'] as Assistant[]).map(id => { const agent = job.agentView?.find(a => a.id === id); const state = agent?.state ?? 'waiting'; return <div {...stylex.props(s.card)} key={id}><div {...stylex.props(s.row)}><Orb of={id} size={26} live={state === 'running'} dim={state === 'waiting'} /><div {...stylex.props(s.grow)}><strong>{agentNames[id][0]}</strong><small {...stylex.props(s.muted)}>{agentNames[id][1]}</small></div><span {...stylex.props(s.badge)}>{agentStates[state] ?? state}</span></div>{agent?.now && <div {...stylex.props(s.inset)}>{agent.now}</div>}{agent?.metric && <div {...stylex.props(s.between, s.muted)}><span>{agent.metric.label}</span><span>{agent.metric.value}</span></div>}</div> })}
      {tab === 'source' && <>
        <div {...stylex.props(s.thumbnail)}>{job.source.thumbnailUrl && <img {...stylex.props(s.image)} src={job.source.thumbnailUrl} alt="" />}{job.source.outlier != null && <span {...stylex.props(s.scoreBadge)}>×{job.source.outlier.toLocaleString('vi-VN', { maximumFractionDigits: 1 })}</span>}{job.source.durationSec != null && <span {...stylex.props(s.durationBadge)}>{fmt(job.source.durationSec)}</span>}</div>
        <div {...stylex.props(s.field)}><strong>{job.source.title}</strong><span {...stylex.props(s.muted)}>{[job.source.channelTitle, job.source.viewCount != null ? `${job.source.viewCount.toLocaleString('vi-VN')} lượt xem` : null, job.source.publishedAt ? new Date(job.source.publishedAt).toLocaleDateString('vi-VN') : null].filter(Boolean).join(' · ')}</span></div>
        {(job.source.outlier != null || job.source.hasCaptions != null || job.source.chapterCount != null) && <div {...stylex.props(s.sourceStats)}>{job.source.outlier != null && <div {...stylex.props(s.inset)}><small {...stylex.props(s.muted)}>Điểm nổi</small><br /><strong {...stylex.props(s.warn)}>×{job.source.outlier.toLocaleString('vi-VN', { maximumFractionDigits: 1 })}</strong></div>}{job.source.hasCaptions != null && <div {...stylex.props(s.inset)}><small {...stylex.props(s.muted)}>Phụ đề</small><br /><strong>{job.source.hasCaptions ? job.source.captionLanguage || 'Có' : 'Không'}</strong></div>}{job.source.chapterCount != null && <div {...stylex.props(s.inset)}><small {...stylex.props(s.muted)}>Chương</small><br /><strong>{job.source.chapterCount}</strong></div>}</div>}
        <div {...stylex.props(s.row)}><a {...stylex.props(s.button, s.sourceLink)} href={job.source.url} target="_blank" rel="noreferrer">Mở trên YouTube</a><a {...stylex.props(s.button, s.sourceLink)} href="#/explore">Khám phá</a></div>
        {job.source.transcriptExcerpt && <div {...stylex.props(s.field)}><strong>Đoạn đầu bản chép lời</strong><div {...stylex.props(s.inset)}>{job.source.transcriptExcerpt}</div></div>}
      </>}
      {tab === 'content' && <><div {...stylex.props(s.notice)}>Đạo diễn viết theo tiêu đề, mô tả, tag của video đối thủ và <a href={`#/channels/${encodeURIComponent(job.channelId)}`}>quy tắc viết của kênh</a>. Sửa tự do; chỉ áp cho video này.</div><div {...stylex.props(s.row)}><button {...stylex.props(s.button)} disabled={!data || busy} onClick={() => setRewrite(!rewrite)}>Viết lại</button><button {...stylex.props(s.button)} disabled={!data || busy} onClick={loadHistory} aria-expanded={!!history}>Lịch sử</button><span {...stylex.props(s.muted)}>{meta?.status === 'pending' ? 'Chưa viết' : meta?.status === 'writing' ? 'Đang viết…' : meta?.status === 'failed' ? 'Chưa viết được' : data?.edited && Object.values(data.edited).some(Boolean) ? 'Bạn đã sửa' : 'Đạo diễn viết'}</span></div>
      {!meta && !panelError && <div role="status">Đang tải…</div>}{meta?.status === 'pending' && <div {...stylex.props(s.card)}>Chưa viết tiêu đề, mô tả, tag. Đạo diễn viết ngay khi có lời thoại của video gốc, thường vài phút sau khi tải xong. Không tốn ảnh.</div>}{meta?.status === 'writing' && <div {...stylex.props(s.card)} role="status">Đạo diễn đang viết tiêu đề, mô tả, tag…</div>}{meta?.status === 'failed' && <div {...stylex.props(s.notice)} role="alert">Chưa viết được tiêu đề, mô tả, tag. Video vẫn dựng tiếp; chỉ phần này chờ. <button {...stylex.props(s.button)} disabled={busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'rewrite', { parts: ['title','description','tags'], expectedRevision: meta.revision })); onRefresh() })}>Thử lại</button> <a href="#/settings">Đăng nhập lại</a></div>}
      {history && <div {...stylex.props(s.card)} role="group" aria-label="Lịch sử nội dung">{history.map(h => <div key={h.at} {...stylex.props(s.between)}><span>{h.by === 'owner' ? 'Bạn sửa' : 'Đạo diễn viết'} · {new Date(h.at).toLocaleString('vi-VN')}<br />{h.title}</span><button {...stylex.props(s.button)} disabled={busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'restore', { at: h.at, expectedRevision: meta?.revision })); setHistory(null); onRefresh() })}>Dùng lại</button></div>)}<small {...stylex.props(s.muted)}>Giữ 10 bản gần nhất. Dùng lại một bản cũ thì bản đang dùng cũng vào lịch sử.</small></div>}
      {data && <>{rewrite && <div {...stylex.props(s.card)} role="group" aria-label="Viết lại"><div {...stylex.props(s.row)}>{(Object.keys(labels) as MetadataPart[]).map(part => <button key={part} {...stylex.props(s.button, parts.includes(part) && s.tabSelected)} aria-pressed={parts.includes(part)} onClick={() => setParts(togglePart(parts, part))}>{labels[part]}</button>)}</div><input {...stylex.props(s.input)} value={direction} onChange={e => setDirection(e.target.value)} placeholder="Góp ý, ví dụ: tiêu đề ngắn hơn" /><small {...stylex.props(s.muted)}>Không tốn ảnh. Phần bạn đã sửa tay bị thay; bản cũ vào Lịch sử.</small><button {...stylex.props(s.button)} disabled={!parts.length || busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'rewrite', { parts, direction, expectedRevision: meta?.revision })); setRewrite(false); onRefresh() })}>Viết lại {parts.length} phần</button></div>}
      <div ref={reasonAnchor} {...stylex.props(s.reasonAnchor)}><label {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Tiêu đề {data.edited?.title && <small {...stylex.props(s.warn)}>bạn đã sửa</small>}</strong>{data.reason?.title && <button {...stylex.props(s.link)} aria-expanded={why} aria-haspopup="dialog" onClick={e => { e.preventDefault(); setWhy(!why) }}>Vì sao?</button>}<button {...stylex.props(s.link)} onClick={e => { e.preventDefault(); void navigator.clipboard.writeText(data.title) }}>Sao chép</button></span><textarea {...stylex.props(s.input)} value={data.title} readOnly={offline} maxLength={100} onChange={e => { dirty.current = true; setMeta({ ...meta!, metadata: { ...data, title: e.target.value } }) }} onBlur={() => { if (dirty.current) void save({ title: data.title }) }} /><small {...stylex.props(s.muted)}>{counts?.title} / 100 ký tự</small></label>{why && data.reason?.title && <div {...stylex.props(s.reasonPopup)}><ReasonPopover reason={data.reason.title} /></div>}</div>
      <label {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Mô tả {data.edited?.description && <small {...stylex.props(s.warn)}>bạn đã sửa</small>}</strong><button {...stylex.props(s.link)} onClick={e => { e.preventDefault(); void navigator.clipboard.writeText(data.description) }}>Sao chép</button></span><textarea {...stylex.props(s.input, s.description)} value={editableDescription} readOnly={offline} onChange={e => { dirty.current = true; setMeta({ ...meta!, metadata: { ...data, description: e.target.value + footerSuffix } }) }} onBlur={() => { if (dirty.current) void save({ description: data.description }) }} />{footer && <div {...stylex.props(s.inset)}>Đoạn cuối của kênh · tự thêm<br />{footer}</div>}<small {...stylex.props(s.muted)}>{counts?.description} / 5.000 ký tự, tính cả đoạn cuối</small></label>
      {data.reason?.description && <><button {...stylex.props(s.link)} aria-expanded={whyDescription} onClick={() => setWhyDescription(!whyDescription)}>Vì sao mô tả này?</button>{whyDescription && <ReasonPopover reason={data.reason.description} />}</>}
      <div {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Tag {data.edited?.tags && <small {...stylex.props(s.warn)}>bạn đã sửa</small>}</strong><button {...stylex.props(s.link)} onClick={() => void navigator.clipboard.writeText(data.tags.join(', '))}>Sao chép</button></span><div {...stylex.props(s.wrap)}>{data.tags.map((item, index) => <span {...stylex.props(s.badge)} key={`${item}-${index}`}>{item}{!(meta?.alwaysTags ?? channel?.alwaysTags ?? []).includes(item) && <button {...stylex.props(s.link)} disabled={busy} aria-label={`Bỏ tag ${item}`} onClick={() => save({ tags: data.tags.filter((_, i) => i !== index) })}> ×</button>}</span>)}</div><input {...stylex.props(s.input)} value={tag} disabled={busy} placeholder="Thêm tag" onChange={e => setTag(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && tag.trim()) { e.preventDefault(); void save({ tags: [...data.tags, tag.trim()] }); setTag('') } }} /><small {...stylex.props(s.muted)}>{counts?.tags} / 30 tag · {counts?.tagCharacters} / 500 ký tự · tag của kênh là tag luôn thêm</small></div><button {...stylex.props(s.button)} onClick={() => void navigator.clipboard.writeText([data.title, data.description, data.tags.join(', ')].filter(Boolean).join('\n\n'))}>Sao chép cả gói đăng</button></>}
      {data?.reason?.tags && <><button {...stylex.props(s.link)} aria-expanded={whyTags} onClick={() => setWhyTags(!whyTags)}>Vì sao những tag này?</button>{whyTags && <ReasonPopover reason={data.reason.tags} />}</>}
      </>}
      {tab === 'backdrop' && (loadingBackdrop && !backdrop ? <div role="status" {...stylex.props(s.card)}>Đang tải…</div> : !backdrop ? !panelError && <div {...stylex.props(s.card)}>Chưa có clip nền cho video này.</div> : <>
        <div {...stylex.props(s.card)}>
          <div {...stylex.props(s.between)}><strong {...stylex.props(coverage?.short ? s.warn : s.ok)}>{coverage?.total == null ? 'Độ phủ chưa xác định' : coverage.short ? `Nền sẽ lặp ${fmt(coverage.total - coverage.covered)}` : 'Phủ kín video'}</strong><span>{backdrop.items.length + (backdrop.fixedItems?.length ?? 0)} clip</span></div>
          <progress max={coverage?.total ?? 1} value={Math.min(coverage?.covered ?? 0, coverage?.total ?? 1)} {...stylex.props(s.progress, coverage?.short && s.progressWarn)} />
          <small {...stylex.props(s.muted)}>Phủ {fmt(coverage?.covered ?? 0)}{coverage?.total != null ? ` / ${fmt(coverage.total)}` : ''}</small>
          {coverage?.short && <button {...stylex.props(s.button)} disabled={locked || busy} onClick={() => editBackdrop('add')}>Thêm clip</button>}
        </div>
        {locked && <div {...stylex.props(s.notice)} role="note">{building ? 'Kiểm định đang dựng video. Đổi nền được sau khi dựng xong.' : 'Đang chuẩn bị nền. Đổi nền được sau khi hoàn tất.'}</div>}
        <div {...stylex.props(s.between)}><strong>Lấy từ kênh nền</strong><a href="#/library?tab=backdrops">Thư viện</a></div>
        <div {...stylex.props(s.wrap)}>{backdrop.sources?.map(source => <span key={source.channelId} {...stylex.props(s.badge)}>{source.title || source.handle} · {source.platform}</span>)}{Boolean(backdrop.fixedItems?.length) && <span {...stylex.props(s.badge)}>{backdrop.fixedItems.length} clip cố định</span>}</div>
        <div {...stylex.props(s.between)}><strong>Clip của video này</strong><button {...stylex.props(s.link)} disabled={locked || busy || !backdrop.randomPicks} onClick={() => editBackdrop('reroll')}>Đổi ngẫu nhiên tất cả</button></div>
        {backdrop.items.map((item, index) => <div key={item.id} {...stylex.props(s.row)}>{item.thumbnailUrl && <img {...stylex.props(s.clip)} src={item.thumbnailUrl} alt="" />}<div {...stylex.props(s.grow)}><span>{item.title || item.channelTitle || item.channelHandle}</span><small {...stylex.props(s.muted)}>{item.source === 'owner' ? 'Bạn chọn' : 'Kênh'} · {fmt(item.startSec)}–{fmt(item.endSec)}</small></div><button {...stylex.props(s.link)} disabled={locked || busy || item.source === 'owner'} aria-label={`Đổi ngẫu nhiên clip ${index + 1}`} onClick={() => editBackdrop('reroll', item.id)}>Đổi</button><button {...stylex.props(s.link)} disabled={locked || busy} aria-label={`Bỏ clip ${index + 1}`} onClick={() => editBackdrop('remove', item.id)}>Bỏ</button></div>)}
        {backdrop.fixedItems?.map((item, index) => <div key={item.id} {...stylex.props(s.row)}><div {...stylex.props(s.grow)}><span>{item.title}</span><small {...stylex.props(s.muted)}>Cố định · {fmt(item.startSec)}–{fmt(item.endSec)}</small></div><button {...stylex.props(s.link)} disabled={locked || busy} aria-label={`Bỏ clip cố định ${index + 1}`} onClick={() => editBackdrop('remove', item.id)}>Bỏ</button></div>)}
        <small {...stylex.props(s.muted)}>Sau khi đổi, app chỉ tải phần thay đổi rồi dựng lại video. Dựng lại không bốc lại clip.</small>
      </>)}
    </div>
  </aside>
}
const s = stylex.create({
  root: { height: '100%', minHeight: 0, display: 'flex', flexDirection: 'column', borderLeftWidth: '1px', borderLeftStyle: 'solid', borderLeftColor: line['--line-1'] },
  tabs: { margin: 'var(--vv-s-12) var(--vv-s-12) 0', display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 'var(--vv-s-2)', padding: 'var(--vv-s-3)', borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-1'], borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-1'] },
  tab: { height: '30px', borderRadius: 'var(--vv-r-7)', borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg-3'], fontSize: '13px', cursor: 'pointer' },
  tabSelected: { backgroundColor: color['--color-surface-6'], color: color['--color-fg'] },
  panel: { flex: 1, minHeight: 0, overflowY: 'auto', padding: 'var(--vv-s-12)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-12)', fontSize: '13px' },
  panelAgents: { gap: 'var(--vv-s-10)' },
  card: { padding: 'var(--vv-s-12)', borderRadius: 'var(--vv-r-14)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-card'], display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)' },
  row: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)' }, between: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 'var(--vv-s-8)' }, grow: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden', textOverflow: 'ellipsis' },
  muted: { color: 'var(--vv-fg-note)', fontSize: '12px', lineHeight: 1.5 }, warn: { color: color['--color-warn'] }, ok: { color: color['--color-ok'] },
  badge: { padding: 'var(--vv-s-4) var(--vv-s-8)', borderRadius: 'var(--vv-r-6)', backgroundColor: color['--color-surface-4'], fontSize: '11.5px' }, inset: { padding: 'var(--vv-s-8) var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', backgroundColor: color['--color-surface-0'], fontSize: '12px', whiteSpace: 'pre-wrap' },
  notice: { padding: 'var(--vv-s-10)', borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-1'], borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-2'], lineHeight: 1.5 },
  button: { padding: 'var(--vv-s-6) var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], cursor: 'pointer', fontSize: '12.5px', textAlign: 'center' },
  link: { padding: 0, borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg-3'], cursor: 'pointer', fontSize: '12px' },
  field: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }, input: { width: '100%', padding: 'var(--vv-s-10) var(--vv-s-12)', borderRadius: 'var(--vv-r-10)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: color['--color-surface-0'], color: color['--color-fg'], fontFamily: 'inherit', fontSize: '13px', resize: 'vertical' }, description: { minHeight: '96px' },
  wrap: { display: 'flex', flexWrap: 'wrap', gap: 'var(--vv-s-6)' }, thumbnail: { position: 'relative', aspectRatio: '16 / 9', borderRadius: 'var(--vv-r-12)', overflow: 'hidden', backgroundColor: color['--color-surface-0'] }, image: { width: '100%', height: '100%', objectFit: 'cover' }, clip: { width: '56px', height: '32px', objectFit: 'cover', borderRadius: 'var(--vv-r-5)' }, progress: { width: '100%', height: '6px', accentColor: color['--color-ok'] }, progressWarn: { accentColor: color['--color-warn'] },
  reasonAnchor: { position: 'relative' }, reasonPopup: { position: 'absolute', top: '100%', left: 0, width: '320px', maxWidth: '100%', zIndex: 4 },
  sourceStats: { display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 'var(--vv-s-6)' }, sourceLink: { flex: 1 }, scoreBadge: { position: 'absolute', left: 'var(--vv-s-8)', bottom: 'var(--vv-s-8)', padding: 'var(--vv-s-2) var(--vv-s-6)', borderRadius: 'var(--vv-r-6)', backgroundColor: color['--color-warn'], color: color['--color-surface-0'], fontWeight: 600 }, durationBadge: { position: 'absolute', right: 'var(--vv-s-8)', bottom: 'var(--vv-s-8)', padding: 'var(--vv-s-2) var(--vv-s-6)', borderRadius: 'var(--vv-r-5)', backgroundColor: color['--color-surface-0'], fontSize: '11px' },
})
