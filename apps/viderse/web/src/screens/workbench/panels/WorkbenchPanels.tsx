// Bảng bên phải Bàn dựng V37: đúng hai tab Nội dung đăng · Thông tin (mặc định Thông tin).
// Thông tin gom Bốn trợ lý + Nguồn + Template + Nền; Nội dung đăng giữ nguyên sửa/chép/
// viết lại/lịch sử/tag thật. Không còn Vì sao?, Lưu thành quy tắc hay tab riêng.
import { useEffect, useId, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { typography, visualRoles } from '../../../styles/viderse.stylex'
import { Button, Icon, Tabs, toast } from '@noverse/ui'
import { getChannel, type Channel } from '../../../api/channels'
import { Orb } from '../../../components/Orb'
import type { Assistant } from '../../../components/agents'
import { coverageAfterRemoval, metadataCounts, metadataDescriptionBody, metadataDescriptionWithFooter, togglePart, usesBackdrop, type MetadataPart } from './panelLogic'
import type { CloneJobView } from '../../../api/workbench'
import { getMetadata, mutateMetadata, getHistory, getBackdrop, mutateBackdrop, type Metadata, type MetaResponse, type History, type BackdropView } from '../../../api/workbenchPanels'
import { TemplateAssetList } from './TemplateAssetList'

const labels: Record<MetadataPart, string> = { title: 'Tiêu đề', description: 'Mô tả', tags: 'Tag' }
const agentNames: Record<Assistant, [string, string]> = { scout: ['Trinh sát', 'Tải nguồn, phụ đề, bốc clip nền'], director: ['Đạo diễn', 'Chia cảnh, viết tiêu đề, tag'], artist: ['Hoạ sĩ', 'Vẽ ảnh cảnh và ảnh bìa'], qa: ['Kiểm định', 'Dựng, đo, xuất MP4'] }
const agentStates: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang chạy', done: 'Xong', stopped: 'Dừng', needs_you: 'Chờ bạn' }
const fmt = (value: number) => `${Math.floor(value / 60)}:${String(Math.floor(value % 60)).padStart(2, '0')}`
const historyTime = (at: string) => new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
const historyParts = (parts: MetadataPart[], tags: number) => {
  const text = (parts.length ? parts : Object.keys(labels) as MetadataPart[]).map(part => part === 'tags' ? `${tags} tag` : labels[part].toLowerCase()).join(', ')
  return text.charAt(0).toUpperCase() + text.slice(1)
}

type PanelTab = 'content' | 'info'
const panelTabs: { id: PanelTab; label: string }[] = [
  { id: 'content', label: 'Nội dung đăng' },
  { id: 'info', label: 'Thông tin' },
]

export function WorkbenchPanels({ job, onRefresh }: { job: CloneJobView; onRefresh: () => void }) {
  const [tab, setTab] = useState<PanelTab>('info')
  const tabsId = useId()
  const panelId = `${tabsId}-panel`
  const [meta, setMeta] = useState<MetaResponse | null>(null)
  const [channel, setChannel] = useState<Channel | null>(null)
  const [backdrop, setBackdrop] = useState<BackdropView | null>(null)
  const [history, setHistory] = useState<History[] | null>(null)
  const [rewrite, setRewrite] = useState(false)
  const [parts, setParts] = useState<MetadataPart[]>(['title'])
  const [direction, setDirection] = useState('')
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
  async function copy(text: string) {
    try { await navigator.clipboard.writeText(text); toast.ok('Đã sao chép') }
    catch { toast.err('Không sao chép được. Kiểm tra quyền truy cập bộ nhớ tạm của trình duyệt.') }
  }
  async function save(patch: Partial<Metadata>) { await run(async () => { const result = await mutateMetadata(job.id, '', { ...patch, expectedRevision: meta?.revision }); dirty.current = false; setMeta(result); onRefresh() }) }
  async function loadHistory() { if (history) { setHistory(null); return } setRewrite(false); await run(async () => { setHistory(await getHistory(job.id)) }) }
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
  const editableDescription = metadataDescriptionBody(data?.description ?? '', footer)
  const counts = data && metadataCounts(data.title, data.description, data.tags)
  const building = job.activeStep === 'build' || Boolean(job.steps?.some(step => step.step === 'build' && step.status === 'running'))
  const locked = Boolean(backdrop?.busy || building)
  const coverage = backdrop && coverageAfterRemoval(backdrop.coverage, 0)
  const panelError = error || (tab === 'content' ? metaError : tab === 'info' ? backdropError : '')
  const metaFailure = job.steps?.find(step => step.step === 'metadata')?.error?.message
  const metaEdited = Boolean(data?.editedBy === 'owner' || (data?.edited && Object.values(data.edited).some(Boolean)))
  const metaStatusLabel = meta?.status === 'pending' ? 'Chưa viết' : meta?.status === 'writing' ? 'Đang viết…' : meta?.status === 'failed' ? 'Chưa viết được' : metaEdited ? 'Bạn đã sửa' : meta ? 'Đạo diễn viết' : ''
  const addTag = () => {
    const value = tag.trim()
    if (!value || !data) return
    if (data.tags.includes(value)) { setTag(''); return }
    setTag('')
    void save({ tags: [...data.tags, value] })
  }
  return <aside {...stylex.props(s.root)} aria-label="Bảng bên phải">
    <div {...stylex.props(s.tabs)}><Tabs items={panelTabs} value={tab} onChange={id => { const next = panelTabs.find(item => item.id === id); if (next) setTab(next.id) }} label="Bảng bên phải" tabs={{ id: tabsId, panel: panelId }} /></div>
    <div {...stylex.props(s.panel)} role="tabpanel" id={panelId} aria-labelledby={`${tabsId}-${tab}`} tabIndex={0}>
      {panelError && <div role="alert" {...stylex.props(s.notice)}>{panelError} <Button variant="quiet" size="sm" xstyle={s.link} onClick={() => setRetry(value => value + 1)}>Thử lại ngay</Button></div>}
      {tab === 'info' && <>
        <div {...stylex.props(s.section, s.agentSection)}><span {...stylex.props(s.sectionTitle)}>Bốn trợ lý</span>
          {(['artist', 'qa', 'director', 'scout'] as Assistant[]).map(id => {
            const agent = job.agentView?.find(a => a.id === id)
            const state = agent?.state ?? 'waiting'
            return <div {...stylex.props(s.agentCard)} key={id}>
              <div {...stylex.props(s.row, s.agentRow)}><Orb of={id} size={22} live={state === 'running'} dim={state === 'waiting'} /><div {...stylex.props(s.grow)}><strong>{agentNames[id][0]}</strong><small {...stylex.props(s.agentRole)} title={agent?.now ?? agentNames[id][1]}>{agent?.now ?? agentNames[id][1]}</small></div><span {...stylex.props(s.badge, state === 'running' && s.badgeRunning, state === 'needs_you' && s.badgeNeeds, state === 'stopped' && s.badgeStopped, state === 'done' && s.badgeDone)}>{agentStates[state] ?? state}</span></div>
            </div>
          })}
        </div>
        <div {...stylex.props(s.section)}><span {...stylex.props(s.sectionTitle)}>Nguồn</span>
          <div {...stylex.props(s.thumbnail)}>{job.source.thumbnailUrl && <img {...stylex.props(s.image)} src={job.source.thumbnailUrl} alt="" />}{job.source.outlier != null && <span {...stylex.props(s.scoreBadge)}>×{job.source.outlier.toLocaleString('vi-VN', { maximumFractionDigits: 1 })}</span>}{job.source.durationSec != null && <span {...stylex.props(s.durationBadge)}>{fmt(job.source.durationSec)}</span>}</div>
          <div {...stylex.props(s.field)}><strong>{job.source.title}</strong><span {...stylex.props(s.muted)}>{[job.source.channelTitle, job.source.viewCount != null ? `${job.source.viewCount.toLocaleString('vi-VN')} lượt xem` : null, job.source.publishedAt ? new Date(job.source.publishedAt).toLocaleDateString('vi-VN') : null].filter(Boolean).join(' · ')}</span></div>
          <div {...stylex.props(s.row)}><a {...stylex.props(s.button, s.sourceLink)} href={job.source.url} target="_blank" rel="noreferrer">Mở trên YouTube</a><a {...stylex.props(s.button, s.sourceLink)} href="#/explore">Khám phá</a></div>
          {job.source.transcriptExcerpt && <div {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Đoạn đầu bản chép lời</strong><Button variant="quiet" size="sm" xstyle={s.link} onClick={() => void copy(job.source.transcriptExcerpt ?? '')}>Sao chép</Button></span><div {...stylex.props(s.inset)}><span {...stylex.props(s.mono, s.muted)}>00:00</span>{'  '}{job.source.transcriptExcerpt}</div></div>}
        </div>
        <div {...stylex.props(s.section)}><span {...stylex.props(s.sectionTitle)}>Template</span>
          <div {...stylex.props(s.templateRow)}><Icon name="layer" size={16} xstyle={s.mutedIcon} /><div {...stylex.props(s.grow)}><span>{job.templateName ?? job.templateId}</span><small {...stylex.props(s.muted)}>Template của video này</small></div><a href={`#/templates/${encodeURIComponent(job.templateId)}`} {...stylex.props(s.quietLink)}>Mở</a></div>
          <TemplateAssetList assets={job.templateSharedAssets} />
        </div>
        <div {...stylex.props(s.section)}><span {...stylex.props(s.sectionTitle)}>Nền</span>
          {loadingBackdrop && !backdrop ? <div role="status" {...stylex.props(s.card)}>Đang tải…</div> : !backdrop || !usesBackdrop(backdrop) ? !panelError && <div {...stylex.props(s.card)}>{backdrop ? 'Template này không dùng clip nền.' : 'Chưa có clip nền cho video này.'} <a href="#/library?tab=backdrops" {...stylex.props(s.inlineLink)}>Mở Thư viện</a></div> : <>
            <div {...stylex.props(s.card, s.coverageCard)}>
              <div {...stylex.props(s.between, s.coverageHeading)}><span {...stylex.props(s.row)}><span aria-hidden="true" {...stylex.props(s.coverageDot, coverage?.short ? s.warn : s.ok)} /><strong {...stylex.props(coverage?.short ? s.warn : s.ok)}>{coverage?.total == null ? 'Độ phủ chưa xác định' : coverage.short ? `Nền sẽ lặp ${fmt(coverage.total - coverage.covered)}` : 'Phủ kín video'}</strong></span><span {...stylex.props(s.muted)}>{backdrop.items.length + (backdrop.fixedItems?.length ?? 0)} clip</span></div>
              <div role="progressbar" aria-label="Độ phủ nền video" aria-valuemin={0} aria-valuemax={coverage?.total ?? 1} aria-valuenow={Math.min(coverage?.covered ?? 0, coverage?.total ?? 1)} {...stylex.props(s.progress)}>
                <span style={{ width: `${coverage?.total ? Math.min(coverage.covered / coverage.total, 1) * 100 : 0}%` }} {...stylex.props(s.progressFill, coverage?.short && s.progressWarn)} />
              </div>
              <small {...stylex.props(s.muted)}>Phủ {fmt(coverage?.covered ?? 0)}{coverage?.total != null ? ` / ${fmt(coverage.total)}` : ''}</small>
              {coverage?.short && <Button size="sm" xstyle={s.coverageButton} disabled={locked || busy} onClick={() => editBackdrop('add')}>Thêm clip</Button>}
            </div>
            {locked && <div {...stylex.props(s.notice)} role="note">{building ? 'Kiểm định đang dựng video. Đổi nền được sau khi dựng xong.' : 'Đang chuẩn bị nền. Đổi nền được sau khi hoàn tất.'}</div>}
            <div {...stylex.props(s.between)}><strong>Lấy từ kênh nền</strong><a href="#/library?tab=backdrops" {...stylex.props(s.inlineLink)}>Thư viện</a></div>
            <div {...stylex.props(s.wrap)}>{backdrop.sources?.map(source => <span key={source.channelId} {...stylex.props(s.badge)}>{source.title || source.handle} · {source.platform}</span>)}{Boolean(backdrop.fixedItems?.length) && <span {...stylex.props(s.badge)}>{backdrop.fixedItems.length} clip cố định</span>}</div>
            <div {...stylex.props(s.between)}><strong>Clip của video này</strong><Button variant="quiet" size="sm" xstyle={s.link} disabled={locked || busy || !backdrop.randomPicks} onClick={() => editBackdrop('reroll')}>Đổi ngẫu nhiên tất cả</Button></div>
            {backdrop.items.map((item, index) => <div key={item.id} {...stylex.props(s.row)}>{item.thumbnailUrl && <img {...stylex.props(s.clip)} src={item.thumbnailUrl} alt="" />}<div {...stylex.props(s.grow)}><span>{item.title || item.channelTitle || item.channelHandle}</span><small {...stylex.props(s.muted, s.mono)}>{item.source === 'owner' ? 'Bạn chọn' : 'Kênh'} · {fmt(item.startSec)}–{fmt(item.endSec)}</small></div><Button variant="icon" size="sm" aria-label={`Đổi ngẫu nhiên clip ${index + 1}`} tooltip="Đổi ngẫu nhiên" disabled={locked || busy || item.source === 'owner'} onClick={() => editBackdrop('reroll', item.id)}><Icon name="refresh" size={15} /></Button><Button variant="icon" size="sm" aria-label={`Bỏ clip ${index + 1}`} tooltip="Bỏ clip" disabled={locked || busy} onClick={() => editBackdrop('remove', item.id)}><Icon name="trash" size={15} /></Button></div>)}
            {backdrop.fixedItems?.map((item, index) => <div key={item.id} {...stylex.props(s.row)}><div {...stylex.props(s.grow)}><span>{item.title}</span><small {...stylex.props(s.muted, s.mono)}>Cố định · {fmt(item.startSec)}–{fmt(item.endSec)}</small></div><Button variant="icon" size="sm" aria-label={`Bỏ clip cố định ${index + 1}`} tooltip="Bỏ clip" disabled={locked || busy} onClick={() => editBackdrop('remove', item.id)}><Icon name="trash" size={15} /></Button></div>)}
            <small {...stylex.props(s.muted)}>Sau khi đổi, app chỉ tải phần thay đổi rồi dựng lại video. Dựng lại không bốc lại clip.</small>
            <a href="#/library?tab=backdrops" {...stylex.props(s.button)}>Xem cả {backdrop.items.length + (backdrop.fixedItems?.length ?? 0)} clip</a>
          </>}
        </div>
      </>}
      {tab === 'content' && <>
        <div {...stylex.props(s.notice, s.metadataNotice)}><Orb of="director" size={12} /><span>Đạo diễn viết theo tiêu đề, mô tả, tag của video đối thủ và <a href={`#/channels/${encodeURIComponent(job.channelId)}`} {...stylex.props(s.inlineLink)}>quy tắc viết của kênh</a>. Sửa tự do; chỉ áp cho video này.</span></div>
        <div {...stylex.props(s.row)}><Button size="sm" disabled={!data || busy} onClick={() => { setHistory(null); setRewrite(!rewrite) }} aria-expanded={rewrite} icon={<Icon name="refresh" size={14} />}>Viết lại</Button><Button size="sm" disabled={!data || busy} onClick={loadHistory} aria-expanded={!!history} icon={<Icon name="clock" size={14} />}>Lịch sử</Button>{metaStatusLabel && <span {...stylex.props(s.metaStatus, meta?.status === 'writing' && s.metaStatusWriting, metaEdited && s.metaStatusEdited)}>{metaStatusLabel}</span>}</div>
        {!meta && !panelError && <div role="status">Đang tải…</div>}
        {meta?.status === 'pending' && <div {...stylex.props(s.pendingCard)}><span aria-hidden="true" {...stylex.props(s.pendingIcon)}>◷</span><span {...stylex.props(s.pendingText)}><span {...stylex.props(s.pendingTitle)}>Chưa viết tiêu đề, mô tả, tag</span><span {...stylex.props(s.pendingDescription)}>Đạo diễn viết ngay khi có lời thoại của video gốc, thường vài phút sau khi tải xong. Không tốn ảnh.</span></span></div>}
        {meta?.status === 'writing' && <div {...stylex.props(s.card, s.writingCard)} role="status"><span {...stylex.props(s.row)}><Orb of="director" size={14} live /><span>Đạo diễn đang viết tiêu đề, mô tả, tag…</span></span><span {...stylex.props(s.skeleton, s.skeletonTitle)} /><span {...stylex.props(s.skeleton, s.skeletonDescription)} /><span {...stylex.props(s.skeleton, s.skeletonTags)} /><span {...stylex.props(s.writingCaption)}>Khoảng 20 giây · bạn sửa được ngay khi xong</span></div>}
        {meta?.status === 'failed' && <div {...stylex.props(s.failedCard)} role="alert"><span {...stylex.props(s.failedText)}><b>Chưa viết được tiêu đề, mô tả, tag.</b> {metaFailure ? `${metaFailure} ` : ''}Video vẫn dựng tiếp; chỉ phần này chờ.</span><span {...stylex.props(s.wrap)}><Button size="sm" disabled={busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'rewrite', { parts: ['title','description','tags'], expectedRevision: meta.revision })); onRefresh() })}>Thử lại</Button><a {...stylex.props(s.settingsLink)} href="#/settings">Đăng nhập lại ở Cài đặt ›</a></span></div>}
        {history && <div {...stylex.props(s.card, s.historyCard)} role="group" aria-label="Lịch sử nội dung">
          {data && <div {...stylex.props(s.historyRow, s.historyFirst)}><span {...stylex.props(s.historyDetails)}><span {...stylex.props(s.historyWho)}>{data.editedBy === 'owner' ? 'Bạn sửa' : 'Đạo diễn viết'}{data.editedAt ? ` · ${historyTime(data.editedAt)}` : ''}</span><span {...stylex.props(s.historyTitle)}>{data.title}</span><span {...stylex.props(s.historyWhat)}>{historyParts((Object.keys(labels) as MetadataPart[]).filter(part => data.edited?.[part]), data.tags.length)}</span></span><span {...stylex.props(s.historyCurrent)}>Đang dùng</span></div>}
          {history.map(h => <div key={h.at} {...stylex.props(s.historyRow, s.historyBorder)}><span {...stylex.props(s.historyDetails)}><span {...stylex.props(s.historyWho)}>{h.by === 'owner' ? 'Bạn sửa' : 'Đạo diễn viết'} · {historyTime(h.at)}</span><span {...stylex.props(s.historyTitle)}>{h.title}</span><span {...stylex.props(s.historyWhat)}>{historyParts(h.parts, h.tags.length)}</span></span><Button size="sm" variant="quiet" xstyle={s.link} disabled={busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'restore', { at: h.at, expectedRevision: meta?.revision })); setHistory(null); onRefresh() })}>Dùng lại</Button></div>)}
          <span {...stylex.props(s.historyFooter)}>Giữ 10 bản gần nhất. Dùng lại một bản cũ thì bản đang dùng cũng vào lịch sử.</span>
        </div>}
        {data && <>
          {rewrite && <div {...stylex.props(s.card, s.rewriteCard)} role="group" aria-label="Viết lại">
            <div {...stylex.props(s.wrap)} role="group" aria-label="Phần viết lại">{(Object.keys(labels) as MetadataPart[]).map(part => <Button key={part} size="sm" xstyle={s.rewriteChip} active={parts.includes(part)} aria-pressed={parts.includes(part)} onClick={() => setParts(togglePart(parts, part))}>{labels[part]}</Button>)}</div>
            <input {...stylex.props(s.input, s.rewriteInput)} aria-label="Góp ý cho Đạo diễn" value={direction} onChange={e => setDirection(e.target.value)} placeholder="Góp ý, ví dụ: tiêu đề ngắn hơn, giữ từ khoá room 214" />
            <div {...stylex.props(s.wrap)}>{['Ngắn hơn', 'Gợi tò mò hơn', 'Đúng nội dung hơn'].map(text => <Button key={text} size="sm" xstyle={s.rewriteChip} onClick={() => setDirection(text)}>{text}</Button>)}</div>
            <div {...stylex.props(s.row)}><small {...stylex.props(s.muted, s.grow, s.rewriteNote)}>Không tốn ảnh. Phần bạn đã sửa tay bị thay; bản cũ vào Lịch sử.</small><Button variant="primary" size="sm" xstyle={s.rewriteSubmit} disabled={!parts.length || busy} onClick={() => run(async () => { setMeta(await mutateMetadata(job.id, 'rewrite', { parts, direction, expectedRevision: meta?.revision })); setRewrite(false); onRefresh() })}>Viết lại {parts.length} phần</Button></div>
          </div>}
          <div {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Tiêu đề {data.edited?.title && <small {...stylex.props(s.warn)}>bạn đã sửa</small>}</strong><Button variant="quiet" size="sm" xstyle={s.link} onClick={() => void copy(data.title)}>Sao chép</Button></span><textarea aria-label="Tiêu đề" {...stylex.props(s.input)} value={data.title} readOnly={offline} maxLength={100} onChange={e => { dirty.current = true; setMeta({ ...meta!, metadata: { ...data, title: e.target.value } }) }} onBlur={() => { if (dirty.current) void save({ title: data.title }) }} />{counts && <span {...stylex.props(s.count)}>{counts.title} / 100 ký tự</span>}</div>
          <div {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Mô tả {data.edited?.description && <small {...stylex.props(s.warn)}>bạn đã sửa</small>}</strong><Button variant="quiet" size="sm" xstyle={s.link} onClick={() => void copy(data.description)}>Sao chép</Button></span><textarea aria-label="Mô tả" {...stylex.props(s.input, s.description)} value={editableDescription} readOnly={offline} onChange={e => { dirty.current = true; setMeta({ ...meta!, metadata: { ...data, description: metadataDescriptionWithFooter(e.target.value, footer) } }) }} onBlur={() => { if (dirty.current) void save({ description: data.description }) }} />{footer && <div {...stylex.props(s.inset)}>Đoạn cuối của kênh · tự thêm<br />{footer}</div>}{counts && <span {...stylex.props(s.count)}>{counts.description} / 5.000 ký tự, tính cả đoạn cuối</span>}</div>
          <div {...stylex.props(s.field)}><span {...stylex.props(s.between)}><strong>Tag <span {...stylex.props(s.num, s.muted)}>{data.tags.length}</span></strong><Button variant="quiet" size="sm" xstyle={s.link} onClick={() => void copy(data.tags.join(', '))}>Sao chép</Button></span><div {...stylex.props(s.wrap)}>{data.tags.map((item, index) => <span {...stylex.props(s.badge, s.tag)} key={`${item}-${index}`}>{item}{!(meta?.alwaysTags ?? channel?.alwaysTags ?? []).includes(item) && <Button variant="quiet" size="sm" xstyle={s.tagRemove} disabled={busy} aria-label={`Bỏ tag ${item}`} onClick={() => save({ tags: data.tags.filter((_, i) => i !== index) })}><Icon name="close" size={11} /></Button>}</span>)}</div><input {...stylex.props(s.input)} value={tag} disabled={busy} placeholder="Thêm tag" aria-label="Thêm tag" onChange={e => setTag(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addTag() } }} />{counts && <span {...stylex.props(s.count)}>{counts.tags} / 30 tag · {counts.tagCharacters} / 500 ký tự</span>}</div>
          <Button size="sm" xstyle={s.copyAll} onClick={() => void copy(`${data.title}\n\n${data.description}\n\n${data.tags.join(', ')}`)}><Icon name="copy" size={15} />Sao chép cả gói đăng</Button>
        </>}
      </>}
    </div>
  </aside>
}
const s = stylex.create({
  root: { height: '100%', minHeight: 0, display: 'flex', flexDirection: 'column', borderLeftWidth: '1px', borderLeftStyle: 'solid', borderLeftColor: line['--line-1'] },
  tabs: { margin: 'var(--vv-s-12) var(--vv-s-12) 0' },
  section: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' },
  agentSection: { gap: 'var(--vv-s-6)' },
  sectionTitle: { fontSize: typography['--vv-fs-ui'], fontWeight: 500, color: color['--color-fg-3'] },
  panel: { flex: 1, minHeight: 0, overflowY: 'auto', padding: 'var(--vv-s-12)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)', fontSize: '13px' },
  agentCard: { padding: 'var(--vv-s-8) var(--vv-s-10)', borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'] },
  agentRow: { gap: 'var(--vv-s-10)' }, agentRole: { fontSize: '12px', color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  templateRow: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-8) var(--vv-s-10)', borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'] },
  quietLink: { height: '28px', paddingInline: 'var(--vv-s-10)', display: 'inline-flex', alignItems: 'center', borderRadius: 'var(--vv-r-8)', color: color['--color-fg-3'], fontSize: '12.5px' },
  coverageCard: { gap: 'var(--vv-s-8)', borderRadius: 'var(--vv-r-12)', fontSize: '14px' }, coverageHeading: { fontSize: '13px' }, coverageButton: { alignSelf: 'flex-start' },
  metadataNotice: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-8)', padding: 'var(--vv-s-8) var(--vv-s-10)', fontSize: '12.5px', color: visualRoles.directorNoticeInk, backgroundColor: visualRoles.directorNoticeFill, borderColor: visualRoles.directorNoticeLine },
  card: { padding: 'var(--vv-s-12)', borderRadius: 'var(--vv-r-14)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-card'], display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)' },
  row: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)' }, between: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 'var(--vv-s-8)' }, grow: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden', textOverflow: 'ellipsis' },
  muted: { color: 'var(--vv-fg-note)', fontSize: '12px', lineHeight: 1.5 }, mutedIcon: { color: 'var(--vv-fg-note)' }, warn: { color: color['--color-warn'] }, ok: { color: color['--color-ok'] }, num: { fontSize: '12px', fontWeight: 400 },
  badge: { padding: 'var(--vv-s-4) var(--vv-s-8)', borderRadius: 'var(--vv-r-6)', backgroundColor: color['--color-surface-4'], fontSize: '11.5px' }, inset: { padding: 'var(--vv-s-8) var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', backgroundColor: color['--color-surface-0'], fontSize: '12px', whiteSpace: 'pre-wrap' },
  badgeRunning: { backgroundColor: 'var(--vv-st-review-bg)', border: '1px solid var(--vv-st-review-border)', color: 'var(--vv-st-review)' },
  badgeNeeds: { backgroundColor: 'var(--vv-st-review-bg)', border: '1px solid var(--vv-st-review-border)', color: 'var(--vv-st-review)' },
  badgeStopped: { backgroundColor: 'var(--vv-danger-8)', border: '1px solid var(--vv-danger-border)', color: 'var(--vv-warn-ink)' },
  badgeDone: { backgroundColor: 'var(--vv-st-ready-bg)', border: '1px solid var(--vv-st-ready-28)', color: 'var(--vv-st-ready)' },
  metaStatus: { marginLeft: 'auto', padding: 'var(--vv-s-3) var(--vv-s-8)', borderRadius: 'var(--vv-r-6)', backgroundColor: color['--color-surface-4'], fontSize: '11.5px', color: color['--color-fg-3'], whiteSpace: 'nowrap' },
  metaStatusWriting: { color: 'var(--vv-st-review)', backgroundColor: 'var(--vv-st-review-bg)' },
  metaStatusEdited: { color: 'var(--vv-warn-ink)', backgroundColor: 'var(--vv-warn-bg)' },
  coverageDot: { width: 7, height: 7, borderRadius: '50%', backgroundColor: 'currentColor', flexShrink: 0 },
  mono: { fontFamily: "'JetBrains Mono', monospace", fontVariantNumeric: 'tabular-nums' },
  rewriteCard: { padding: 'var(--vv-s-10)', borderRadius: 'var(--vv-r-12)' }, rewriteChip: { height: '26px', padding: '0 var(--vv-s-9)', borderRadius: 'var(--vv-r-pill)' }, rewriteInput: { height: '34px', padding: '0 var(--vv-s-10)', borderRadius: 'var(--vv-r-9)' }, rewriteSubmit: { paddingInline: 'var(--vv-s-12)' },
  rewriteNote: { fontSize: '11.5px', lineHeight: 1.45 },
  pendingCard: { padding: 'var(--vv-s-14)', borderRadius: 'var(--vv-r-12)', borderWidth: '1px', borderStyle: 'dashed', borderColor: line['--line-4'], display: 'flex', gap: 'var(--vv-s-10)', alignItems: 'flex-start' },
  pendingIcon: { width: '16px', height: '16px', flexShrink: 0, marginTop: 'var(--vv-s-2)', color: 'var(--vv-fg-note)' }, pendingText: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }, pendingTitle: { fontSize: '13.5px' }, pendingDescription: { fontSize: '12.5px', color: 'var(--vv-fg-note)', lineHeight: 1.5 },
  writingCard: { padding: 'var(--vv-s-14)', borderRadius: 'var(--vv-r-12)', fontSize: '13px' }, skeleton: { display: 'block', backgroundColor: color['--color-surface-4'], borderRadius: 'var(--vv-r-10)' }, skeletonTitle: { height: '38px' }, skeletonDescription: { height: '96px' }, skeletonTags: { height: '26px', width: '70%', borderRadius: 'var(--vv-r-6)' }, writingCaption: { fontSize: '12px', color: 'var(--vv-fg-note)' },
  failedCard: { padding: 'var(--vv-s-12)', borderRadius: 'var(--vv-r-12)', borderWidth: '1px', borderStyle: 'solid', borderColor: 'var(--vv-warn-bd)', backgroundColor: 'var(--vv-warn-bg)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' }, failedText: { fontSize: '13px', color: 'var(--vv-warn-ink)', lineHeight: 1.55 }, settingsLink: { height: '30px', padding: '0 var(--vv-s-10)', display: 'flex', alignItems: 'center', fontSize: '12.5px', color: color['--color-fg-3'] },
  historyCard: { padding: 'var(--vv-s-6) var(--vv-s-10)', borderRadius: 'var(--vv-r-12)', gap: 0 }, historyRow: { padding: 'var(--vv-s-8) 0', display: 'flex', gap: 'var(--vv-s-10)', alignItems: 'flex-start' }, historyBorder: { borderTopWidth: '1px', borderTopStyle: 'solid', borderTopColor: line['--line-2'] }, historyDetails: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }, historyWho: { fontSize: '12px', color: 'var(--vv-fg-note)' }, historyTitle: { fontSize: '13px', lineHeight: 1.45 }, historyCurrent: { flexShrink: 0, height: '22px', padding: '0 var(--vv-s-7)', display: 'flex', alignItems: 'center', borderRadius: 'var(--vv-r-6)', color: color['--color-ok'], backgroundColor: color['--color-surface-4'], fontSize: '11px' },
  historyWhat: { fontSize: '11.5px', color: 'var(--vv-fg-note)' }, historyFirst: { borderTopWidth: '1px', borderTopStyle: 'solid', borderTopColor: 'transparent' },
  historyFooter: { paddingTop: 'var(--vv-s-6)', paddingBottom: 'var(--vv-s-4)', fontSize: '11.5px', color: 'var(--vv-fg-note)' },
  notice: { padding: 'var(--vv-s-10)', borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-1'], borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-2'], lineHeight: 1.5 },
  button: { padding: 'var(--vv-s-6) var(--vv-s-10)', borderRadius: 'var(--vv-r-8)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], cursor: 'pointer', fontSize: '12.5px', textAlign: 'center' },
  link: { height: 'auto', padding: 0, borderWidth: 0, alignSelf: 'flex-start' },
  inlineLink: { color: color['--color-fg'], textDecorationLine: 'underline', textUnderlineOffset: 2 },
  field: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }, input: { width: '100%', padding: 'var(--vv-s-10) var(--vv-s-12)', borderRadius: 'var(--vv-r-10)', borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: color['--color-surface-0'], color: color['--color-fg'], fontFamily: 'inherit', fontSize: '13px', resize: 'vertical' }, description: { minHeight: '96px' },
  count: { fontSize: '11.5px', color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  wrap: { display: 'flex', flexWrap: 'wrap', gap: 'var(--vv-s-6)' }, thumbnail: { position: 'relative', aspectRatio: '16 / 9', borderRadius: 'var(--vv-r-12)', overflow: 'hidden', backgroundColor: color['--color-surface-0'], fontSize: '14px' }, image: { width: '100%', height: '100%', objectFit: 'cover' }, clip: { width: '56px', height: '32px', objectFit: 'cover', borderRadius: 'var(--vv-r-5)' }, progress: { width: '100%', height: 'var(--vv-s-6)', borderRadius: 'var(--vv-r-pill)', backgroundColor: color['--color-surface-6'], overflow: 'hidden' }, progressFill: { display: 'block', height: '100%', borderRadius: 'inherit', backgroundColor: color['--color-ok'] }, progressWarn: { backgroundColor: color['--color-warn'] },
  sourceLink: { flex: 1 }, scoreBadge: { position: 'absolute', left: 'var(--vv-s-8)', bottom: 'var(--vv-s-8)', padding: 'var(--vv-s-2) var(--vv-s-6)', borderRadius: 'var(--vv-r-6)', backgroundColor: color['--color-warn'], color: color['--color-surface-0'], fontWeight: 600 }, durationBadge: { position: 'absolute', right: 'var(--vv-s-8)', bottom: 'var(--vv-s-8)', padding: 'var(--vv-s-2) var(--vv-s-6)', borderRadius: 'var(--vv-r-5)', backgroundColor: color['--color-surface-0'], fontSize: '11px' },
  tag: { display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-2)', height: '26px', paddingInlineStart: 'var(--vv-s-8)', paddingInlineEnd: 'var(--vv-s-4)' },
  tagRemove: { height: '18px', minWidth: '18px', padding: 0 },
  copyAll: { alignSelf: 'stretch' },
})
