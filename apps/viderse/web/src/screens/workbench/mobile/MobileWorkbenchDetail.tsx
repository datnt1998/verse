// MMain › chi tiết: Nội dung đăng / Ảnh, với trợ lý, nguồn và nền trong sheet
// Thông tin. Giữ trình xem ảnh hiện có; không triển khai viewer điện thoại đợt 45.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Segmented, Sheet, Shimmer, toast } from '@noverse/ui'
import { ApiError } from '../../../api/request'
import { getScoreArtifact, jobAction, type CloneJobView, type JobImageList } from '../../../api/workbench'
import { getBackdrop, getMetadata, mutateBackdrop } from '../../../api/workbenchPanels'
import { useResource } from '../../../data/useResource'
import { subscribeEvents } from '../../../data/events'
import { useOnline } from '../../../data/status'
import { requestDeleteExport, requestMarkPosted } from '../../../components/performance'
import { Orb } from '../../../components/Orb'
import { DevelopEffect } from '../../../components'
import { ASSISTANTS, ASSISTANT_INK, assistantName } from '../../../components/agents'
import { progressState, progressText } from '../../../lib/progress-copy'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, routePath } from '../../../router'
import { finishedVideo, imageTime } from '../workbench-logic'
import { durationLabel, imageLabel, imageUrl } from './mworkbench-logic'
import { mw, mwDetail as d } from './mworkbench.stylex'
import { OverflowMenu } from '../OverflowMenu'
import { MobileMetaSheet } from './MobileMetaSheet'
import { MobileImageSheet } from './MobileImageSheet'
import { coverageAfterRemoval } from '../panels/panelLogic'

const c = screenWorkbench.mobile
const STATE: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang làm', done: 'Xong', stopped: 'Dừng', needs_you: 'Chờ bạn' }
const STEP_AGENT: Record<string, string> = {
  download: 'scout', backdrop: 'scout', transcript: 'scout',
  metadata: 'director', scenes: 'director',
  images: 'artist',
  build: 'qa', score: 'qa', export: 'qa',
}

export function MobileWorkbenchDetail({ job, images, imagesLoading, imagesError, jobChannel, onBack, onRefresh, onNotice }: {
  job: CloneJobView
  images: JobImageList | undefined
  imagesLoading: boolean
  imagesError: unknown
  jobChannel?: string
  onBack: () => void
  onRefresh: () => void
  onNotice: (text: string) => void
}) {
  const [moreOpen, setMoreOpen] = useState(false)
  const [infoOpen, setInfoOpen] = useState(false)
  const [tab, setTab] = useState<'content' | 'images'>('content')
  const [backdropBusy, setBackdropBusy] = useState(false)
  const [backdropError, setBackdropError] = useState('')
  const [metaOpen, setMetaOpen] = useState<'edit' | 'rewrite' | 'history' | null>(null)
  const [imageOpen, setImageOpen] = useState(false)
  const [imageId, setImageId] = useState<string | undefined>(undefined)
  const [busy, setBusy] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  // Lỗi tải/giải mã không đồng nghĩa tệp đã bị xoá.
  const [failedVideoUrl, setFailedVideoUrl] = useState<string | null>(null)
  const online = useOnline()
  const meta = useResource(['workbench', 'meta', job.id], ({ signal }) => getMetadata(job.id, signal))
  const backdrop = useResource(['workbench', 'backdrop', job.id, job.version], ({ signal }) => getBackdrop(job.id, signal), { enabled: infoOpen && !job.queueItem })
  const building = job.activeStep === 'build' || Boolean(job.steps?.some(step => step.step === 'build' && step.status === 'running'))
  const backdropLocked = !online || backdropBusy || building || Boolean(backdrop.data?.busy)
  const coverage = backdrop.data && coverageAfterRemoval(backdrop.data.coverage, 0)
  const changeBackdrop = async (op: 'remove' | 'reroll' | 'add', itemId?: string) => {
    if (!backdrop.data || backdropLocked) return
    setBackdropBusy(true); setBackdropError('')
    try {
      await mutateBackdrop(job.id, { op, revision: backdrop.data.revision, ...(itemId ? { itemId } : {}) })
      await backdrop.refresh(); onRefresh(); toast.ok('Đã cập nhật nền của video.')
    } catch { setBackdropError('Chưa đổi được nền. Vui lòng thử lại.') }
    finally { setBackdropBusy(false) }
  }
  const scoreArtifact = job.steps?.find((step) => step.step === 'score')?.output?.artifacts?.find((item) => item.path.endsWith('score.json'))
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: Boolean(scoreArtifact) && job.activeStep === 'score' })
  const video = finishedVideo(job)
  const playable = video && video.url !== failedVideoUrl ? video : null
  useEffect(() => { setFailedVideoUrl(null) }, [video?.url])
  const items = images?.items ?? []
  const cover = items.find((item) => item.kind === 'thumbnail' || item.kind === 'cover')
  const drawn = items.filter((item) => imageUrl(item)).length
  const missing = meta.error instanceof ApiError && meta.error.status === 404
  const data = meta.data?.metadata
  const edited = Boolean(data) && (data?.editedBy === 'owner' || Object.values(data?.edited ?? {}).some(Boolean))
  const status = meta.data?.status ?? (missing ? 'pending' : undefined)
  const statusLabel = status === 'writing' ? 'Đang viết…' : status === 'failed' ? 'Chưa viết được'
    : edited ? `Bạn đã sửa${data?.editedAt ? ` · ${new Date(data.editedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}` : ''}`
      : data ? 'Đạo diễn viết' : 'Chưa viết'
  const statusStyle = status === 'writing' ? d.statusWriting : status === 'failed' ? d.statusFailed : edited ? d.statusEdited : data ? d.statusAI : d.statusPending
  const markPosted = () => { setBusy(true); void requestMarkPosted(job, onRefresh).catch((err: unknown) => onNotice(err instanceof Error ? err.message : 'Không đánh dấu được')).finally(() => setBusy(false)) }
  // Tạm dừng / chạy tiếp / thử lại dùng đúng jobAction của bản máy tính:
  // chạy tiếp giữ ảnh đã vẽ và không tạo lượt mới, nên không tốn lại hạn mức ảnh.
  const action = async (kind: 'pause' | 'resume' | 'run') => {
    if (actionBusy) return
    setActionBusy(true); setActionError('')
    try { await jobAction(job.id, kind); onRefresh() }
    catch (err) { setActionError(err instanceof Error ? err.message : 'Không thực hiện được') }
    finally { setActionBusy(false) }
  }
  // Xoá riêng file MP4 của video đã đăng: giữ project, job, ảnh và video nguồn.
  const deleteMp4 = () => { void requestDeleteExport(job, onRefresh).catch((err: unknown) => onNotice(err instanceof Error ? err.message : 'Không xoá được')) }
  const hasExport = (job.exportBytes ?? 0) > 0
  const openImage = (id?: string) => {
    if (job.stageGroup === 'review') { navigate(routePath({ name: 'review', video: job.id, img: id })); return }
    setImageId(id); setImageOpen(true)
  }
  const copyPackage = () => {
    if (!meta.data?.metadata) return
    const body = meta.data.metadata
    void navigator.clipboard.writeText(`${body.title}\n\n${body.description}\n\n${body.tags.join(', ')}`)
      .then(() => onNotice(c.metaPackageCopied)).catch(() => onNotice(c.metaCopyFailed))
  }

  useEffect(() => { setActionError(''); setBackdropError(''); setInfoOpen(false); setTab('content'); setImageOpen(false); setMetaOpen(null) }, [job.id])
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated'], () => { void meta.refresh() }), [meta.refresh])

  return <div {...stylex.props(d.root)}>
    <header {...stylex.props(d.header)}>
      <button type="button" aria-label="Quay lại Bàn dựng" onClick={onBack} {...stylex.props(d.headerBtn)}><Icon name="arrow-r" size={20} style={{ transform: 'rotate(180deg)' }} /></button>
      <span {...stylex.props(d.headerText)}>
        <span {...stylex.props(d.headerTitle)} title={job.source.title}>{job.source.title}</span>
        <span {...stylex.props(d.headerMeta)}>{[jobChannel, durationLabel(job.source.durationSec), job.templateName ?? job.templateId].filter(Boolean).join(' · ')}</span>
      </span>
      {job.stageGroup === 'working' && <button type="button" aria-label={c.pause} title={!online ? 'Mất kết nối' : c.pause} disabled={!online || actionBusy} aria-busy={actionBusy || undefined} onClick={() => void action('pause')} {...stylex.props(d.headerBtn)}><Icon name="pause" size={20} /></button>}
      <button type="button" aria-label="Thông tin video" onClick={() => setInfoOpen(true)} {...stylex.props(d.headerBtn)}><Icon name="info" size={20} /></button>
      <button type="button" aria-label={c.more} onClick={() => setMoreOpen(true)} {...stylex.props(d.headerBtn)}><Icon name="more" size={20} /></button>
    </header>

    <main {...stylex.props(d.scroll)}>

      {job.queueItem && <div role="status" {...stylex.props(d.sectionCard)}>
        <span {...stylex.props(d.reviewBannerText)}><Icon name="clock" size={18} /><span>{c.queued} Hoạ sĩ bắt đầu vẽ khi video được nhận lượt, theo template của video này.</span></span>
        <a href="#/queue" {...stylex.props(mw.moreLink)}>{c.openQueue}<Icon name="arrow-r" size={14} /></a>
      </div>}

      {job.stageGroup === 'paused' && <div role="status" {...stylex.props(d.reviewBanner)}>
        <span {...stylex.props(d.reviewBannerText)}><Icon name="pause" size={18} xstyle={d.reviewFlag} /><span>Video đang tạm dừng. Phần đã làm được giữ nguyên; chạy tiếp không vẽ lại ảnh đã xong.</span></span>
        <Button size="lg" variant="primary" xstyle={d.reviewBtn} icon={<Icon name="play" size={18} />} disabled={!online || actionBusy} onClick={() => void action('resume')}>{c.resume}</Button>
      </div>}

      {job.stageGroup === 'review' && <div role="status" {...stylex.props(d.reviewBanner)}>
        <span {...stylex.props(d.reviewBannerText)}>
          <Icon name="flag" size={18} xstyle={d.reviewFlag} />
          <span>Hoạ sĩ vẽ xong {items.length || drawn} ảnh. Duyệt xong, Kiểm định tự dựng và xuất video.</span>
        </span>
        <Button size="lg" variant="primary" xstyle={d.reviewBtn} icon={<Icon name="gallery" size={18} />} onClick={() => navigate(routePath({ name: 'review', video: job.id }))}>
          Duyệt {items.length || drawn} ảnh
        </Button>
      </div>}

      {job.stageGroup === 'attention' && <div role="alert" {...stylex.props(d.reviewBanner)}>
        <span {...stylex.props(d.reviewBannerText)}><Icon name="danger" size={18} xstyle={[d.reviewFlag, d.attentionFlag]} /><span>{job.subline || 'Video cần bạn xử lý.'}</span></span>
        <Button size="lg" variant="primary" xstyle={d.reviewBtn} icon={<Icon name="refresh" size={18} />} disabled={!online || actionBusy} onClick={() => void action('run')}>{c.retryJob}</Button>
      </div>}

      {actionError && <p role="alert" {...stylex.props(d.drawEmpty)}>{actionError}</p>}

      {job.stageGroup === 'ready' && <div role="status" {...stylex.props(d.reviewBanner)}>
        <span {...stylex.props(d.reviewBannerText)}><Icon name="tick-b" size={18} xstyle={[d.reviewFlag, d.readyFlag]} /><span>{c.readyTitle}. Bạn có thể tải gói đăng hoặc đánh dấu đã đăng.</span></span>
        <div {...stylex.props(mw.actionRow)}>
          <Button size="lg" variant="primary" disabled={busy} onClick={markPosted}>{c.markPosted}</Button>
          <a href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`} {...stylex.props(d.headerBtn)} aria-label={c.download} style={{ width: 'auto', paddingInline: 14 }}>{c.download} ›</a>
        </div>
      </div>}
      <Segmented label="Chi tiết video" value={tab} onChange={value => setTab(value as 'content' | 'images')} fit="fill" size="md" buttonStyle={mw.touchSegment} tabs={{ id: 'mobile-video-tabs', panel: 'mobile-video-panel' }} options={[{ id: 'content', label: 'Nội dung đăng' }, { id: 'images', label: `Ảnh · ${items.length}` }]} />

      {tab === 'images' && <section id="mobile-video-panel" role="tabpanel" aria-labelledby="mobile-video-tabs-images" aria-label="Phòng vẽ trực tiếp" {...stylex.props(mw.section, d.tabPanel)}>
        <div {...stylex.props(d.drawHead)}>
          <span {...stylex.props(d.drawTitle)}><Orb of="artist" size={14} />Phòng vẽ</span>
          <span {...stylex.props(d.drawCount)}>{drawn} / {items.length} ảnh{job.stageGroup === 'review' ? ' · chờ bạn duyệt' : ''}</span>
        </div>
        {job.queueItem && <p {...stylex.props(d.drawEmpty)}>Ảnh xuất hiện ở đây khi video bắt đầu được làm.</p>}
        {imagesLoading && !images && <p role="status" {...stylex.props(d.drawEmpty)}>Đang tải ảnh…</p>}
        {imagesError != null && !images && <div role="alert" {...stylex.props(d.drawEmpty)}>{c.loadError} <Button size="lg" variant="quiet" onClick={onRefresh}>{c.retry}</Button></div>}
        {images && items.length === 0 && <p {...stylex.props(d.drawEmpty)}>{job.noImages ? c.noImages : c.imagesEmpty}</p>}
        {items.length > 0 && <div {...stylex.props(d.drawGrid)}>
          {(cover ? [cover, ...items.filter((item) => item !== cover)] : items).map((item, index) => {
            const src = imageUrl(item)
            const drawing = item.status === 'running'
            // Bản đang vẽ có thể là version mới rỗng; quét trên ảnh đã vẽ gần nhất.
            const drawingBase = drawing
              ? [...(item.versions ?? [])].filter((version) => version.originalUrl).sort((a, b) => b.version - a.version)[0]?.originalUrl
              : undefined
            const waiting = item.status === 'pending' || item.status === 'queued'
            const timing = imageTime(item)
            return <button key={item.id} type="button" onClick={() => openImage(item.id)}
              {...stylex.props(d.drawTile, (item.kind === 'thumbnail' || item.kind === 'cover') && d.drawCover)}>
              <span {...stylex.props(d.drawFrame)}>
                {src && !drawing && <img src={src} alt={imageLabel(item, index)} {...stylex.props(d.drawImg)} />}
                {drawing && <><DevelopEffect state="drawing" baseUrl={drawingBase} compact hideCard /><span role="status" {...stylex.props(d.drawDrawing)}><Orb of="artist" size={14} live /><Shimmer>{screenWorkbench.live.drawing}</Shimmer></span></>}
                {waiting && <span {...stylex.props(d.drawWait)}>Chờ lượt vẽ</span>}
                <span {...stylex.props(d.drawBadge)}><span {...stylex.props(d.drawBadgeKey)}>{index}</span>{imageLabel(item, index)}</span>
              </span>
              <span {...stylex.props(d.drawCapRow)}>
                <span {...stylex.props(d.drawCaption)}>{item.narration || imageLabel(item, index)}</span>
                {timing && <span {...stylex.props(d.drawTime)}>{timing}</span>}
              </span>
            </button>
          })}
        </div>}
        {items.length > 0 && <button type="button" aria-label="Mở ảnh lớn" onClick={() => openImage(items[0]?.id)} {...stylex.props(mw.moreLink)}>Xem cả {items.length} ảnh<Icon name="arrow-r" size={14} /></button>}
      </section>}

      {tab === 'content' && <section id="mobile-video-panel" role="tabpanel" aria-labelledby="mobile-video-tabs-content" {...stylex.props(d.sectionCard, d.tabPanel)}>
        <div {...stylex.props(d.sectionHead)}>
          <h2 id="pub-h" {...stylex.props(d.sectionTitle)}><Orb of="director" size={14} />{c.meta}</h2>
          <span {...stylex.props(d.statusPill, statusStyle)}>{statusLabel}</span>
        </div>

        {status === 'pending' && <span {...stylex.props(d.sectionNote)}>{c.metaPending} {c.metaPendingHint}</span>}
        {status === 'writing' && <div role="status" {...stylex.props(d.writeWriting)}>
          <Shimmer style={{ fontSize: 13, lineHeight: 1.5, color: 'var(--color-fg-3)' }}>{c.metaWriting}</Shimmer>
          <span aria-hidden="true" {...stylex.props(d.skeleton, d.skeletonShort)} />
          <span aria-hidden="true" {...stylex.props(d.skeleton, d.skeletonTall)} />
        </div>}
        {status === 'failed' && <div role="alert" {...stylex.props(d.writeFailed)}>
          <span><b>Chưa viết được.</b> {c.metaFailed} Video vẫn dựng tiếp.</span>
          <div {...stylex.props(mw.actionRow)}>
            <Button size="lg" variant="ghost" icon={<Icon name="refresh" size={15} />} onClick={() => setMetaOpen('rewrite')}>{c.retryJob}</Button>
            <a href="#/settings" {...stylex.props(mw.moreLink)}>Đăng nhập lại ở Cài đặt ›</a>
          </div>
        </div>}

        {data && <>
          <div {...stylex.props(d.fieldLabel)}>
            <span>{c.metaSheet.titleField} · <span style={{ fontVariantNumeric: 'tabular-nums' }}>{data.title.length}/100</span></span>
            {(data.edited?.title || edited) && <span style={{ color: 'var(--vv-st-review)' }}>· bạn đã sửa</span>}
            <span {...stylex.props(d.fieldGrow)} />
          </div>
          <button type="button" aria-label={c.metaSheet.titleField} onClick={() => setMetaOpen('edit')} {...stylex.props(d.titleBox)}>{data.title}</button>
          <span {...stylex.props(d.fieldLabel)}>{c.metaSheet.descriptionField} · <span style={{ fontVariantNumeric: 'tabular-nums' }}>{data.description.length}/5.000</span>{meta.data?.descriptionFooter ? ', có đoạn cuối của kênh' : ''}</span>
          <button type="button" aria-label={c.metaSheet.descriptionField} onClick={() => setMetaOpen('edit')} {...stylex.props(d.descBox)}>{data.description}</button>

          <span {...stylex.props(d.fieldLabel)}>{c.metaSheet.tagsField} · <span style={{ fontVariantNumeric: 'tabular-nums' }}>{data.tags.length}/30</span> · <Icon name="channel" size={12} /> là tag luôn thêm của kênh</span>
          <div {...stylex.props(d.tagRow)}>
            {data.tags.slice(0, 6).map((tag) => <span key={tag} {...stylex.props(d.tag)}>
              {meta.data?.alwaysTags?.includes(tag) && <Icon name="channel" size={12} xstyle={d.tagAlways} />}{tag}
            </span>)}
            {data.tags.length > 6 && <span {...stylex.props(d.tagMore)}>+{data.tags.length - 6}</span>}
          </div>

          <div {...stylex.props(d.metaActionGrid)}>
            <Button size="lg" variant="ghost" icon={<Icon name="magic" size={16} />} onClick={() => setMetaOpen('rewrite')}>{c.metaSheet.rewrite}</Button>
            <Button size="lg" variant="ghost" aria-label={c.metaSheet.history} icon={<Icon name="clock" size={16} />} onClick={() => setMetaOpen('history')} />
            <Button size="lg" variant="ghost" icon={<Icon name="copy" size={16} />} onClick={copyPackage}>{c.metaCopyPackage}</Button>
          </div>
          <span {...stylex.props(d.metaHint)}>Chạm vào ô để sửa; chỉ áp cho video này. Đoạn cuối và tag luôn thêm sửa ở Hồ sơ kênh.</span>
        </>}
        {!meta.data && !missing && meta.error == null && <p role="status" {...stylex.props(d.sectionNote)}>Đang tải nội dung đăng…</p>}
        {meta.error != null && !missing && <div role="alert" {...stylex.props(d.writeFailed)}>{c.loadError} <Button size="lg" variant="quiet" onClick={() => void meta.refresh()}>{c.retry}</Button></div>}
      </section>}

      <section aria-label="Tiến độ" {...stylex.props(d.sectionCard)}>
        <div {...stylex.props(d.sectionHead)}>
          <h2 {...stylex.props(d.sectionTitle)}><Icon name="activity" size={15} />Tiến độ</h2>
          <span style={{ fontSize: 12.5, color: 'var(--vv-fg-note)' }}>3 dòng gần nhất</span>
        </div>
        {job.steps?.length ? job.steps.slice(-3).reverse().map((step) => {
          const who = STEP_AGENT[step.step] ?? 'qa'
          const time = step.startedAt ? new Date(step.startedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : ''
          const what = progressText(step)
          const state = progressState(step.status)
          return <div key={step.step} {...stylex.props(d.logRow)}>
            <span {...stylex.props(d.logTime)}>{time}</span>
            <span {...stylex.props(d.logText)}><b style={{ color: ASSISTANT_INK[who as keyof typeof ASSISTANT_INK] }} {...stylex.props(d.logWho)}>{assistantName(who as (typeof ASSISTANTS)[number])}</b> <span {...stylex.props(d.logWhat)}>{what}{state ? ` · ${state}` : ''}</span></span>
          </div>
        }) : <p {...stylex.props(d.sectionNote)}>{c.logEmpty}</p>}
      </section>

      {job.stageGroup === 'attention' && job.activeStep === 'score' && <section aria-label="Kết quả kiểm định" {...stylex.props(d.sectionCard)}>
        <h2 {...stylex.props(d.sectionTitle)}><Orb of="qa" size={14} />Kết quả kiểm định</h2>
        {score.initialLoading ? <p role="status" {...stylex.props(d.sectionNote)}>Đang tải kết quả…</p> : score.error ? <div role="alert" {...stylex.props(d.writeFailed)}>Không tải được kết quả kiểm định. <Button size="lg" variant="quiet" onClick={() => void score.refresh()}>{c.retryJob}</Button></div> : <>
          {score.data != null && <span {...stylex.props(mw.tileValue)}>{score.data.total} điểm</span>}
          {score.data?.blockers?.length ? <ul>{score.data.blockers.map((item) => <li key={item.id}>{item.message}</li>)}</ul> : <p {...stylex.props(d.sectionNote)}>Không có lỗi chặn.</p>}
        </>}
      </section>}

      {(job.stageGroup === 'ready' || job.stageGroup === 'posted') && <section aria-label={c.readyTitle} {...stylex.props(d.videoCard)}>
        <h2 {...stylex.props(d.sectionTitle)}><Icon name="video-add" size={15} />{c.readyTitle}</h2>
        {playable ? <video controls preload="metadata" {...stylex.props(d.viewer)} src={playable.url} onError={() => setFailedVideoUrl(playable.url)} /> : <p {...stylex.props(d.sectionNote)}>{video ? screenWorkbench.videoPlaybackError : c.noVideoFile}</p>}
        {playable && <a {...stylex.props(mw.moreLink)} href={playable.url} target="_blank" rel="noreferrer">Mở file video<Icon name="arrow-r" size={14} /></a>}
        {job.stageGroup === 'posted' && hasExport && <Button size="lg" variant="ghost" icon={<Icon name="trash" size={15} />} onClick={deleteMp4}>{c.deleteMp4}</Button>}
      </section>}
    </main>

    <Sheet open={infoOpen} onClose={() => setInfoOpen(false)} title="Thông tin video" size="md" height="auto">
      <div {...stylex.props(d.infoBody)}>
      <div role="list" aria-label="Bốn trợ lý" {...stylex.props(d.agentRow)}>
        {ASSISTANTS.map((assistant) => {
          const agent = job.agentView?.find((item) => item.id === assistant)
          const running = agent?.state === 'running'
          const waiting = agent?.state === 'waiting'
          const state = running ? 'Đang làm' : agent?.state === 'needs_you' ? (job.stageGroup === 'review' ? 'Chờ bạn' : 'Chờ lượt') : agent ? STATE[agent.state] ?? agent.state : 'Chưa chạy'
          const ink = running ? ASSISTANT_INK[assistant] : agent?.state === 'needs_you' && job.stageGroup === 'review' ? 'var(--vv-st-review)' : agent?.state === 'done' ? 'var(--vv-st-ready)' : 'var(--vv-fg-note)'
          return <div key={assistant} role="listitem" {...stylex.props(d.agentTile, running && d.agentTileOn, waiting && d.agentTileDim)}>
            <Orb of={assistant} size={24} live={running} dim={waiting} />
            <span style={{ color: ASSISTANT_INK[assistant] }} {...stylex.props(d.agentName)}>{assistantName(assistant)}</span>
            <span style={{ color: ink }} {...stylex.props(d.agentState)}>{state}</span>
          </div>
        })}
      </div>
        <section aria-label="Nguồn" {...stylex.props(d.sectionCard)}>
          <h2 {...stylex.props(d.sectionTitle)}>Nguồn</h2>
          {job.source.thumbnailUrl && <img src={job.source.thumbnailUrl} alt="" {...stylex.props(d.viewer)} />}
          <strong {...stylex.props(d.sectionTitle)}>{job.source.title}</strong>
          <span {...stylex.props(d.sectionNote)}>{[job.source.channelTitle, durationLabel(job.source.durationSec), job.source.viewCount != null ? `${job.source.viewCount.toLocaleString('vi-VN')} lượt xem` : null, job.source.publishedAt ? new Date(job.source.publishedAt).toLocaleDateString('vi-VN') : null].filter(Boolean).join(' · ')}</span>
          <div {...stylex.props(mw.actionRow)}><a href={job.source.url} target="_blank" rel="noreferrer" {...stylex.props(mw.moreLink)}>Mở trên YouTube</a><a href="#/explore" {...stylex.props(mw.moreLink)}>Khám phá</a></div>
          {job.source.transcriptExcerpt && <><span {...stylex.props(d.sectionTitle)}>Đoạn đầu bản chép lời</span><p {...stylex.props(mw.promptText)}>{job.source.transcriptExcerpt}</p><Button size="lg" onClick={() => { void navigator.clipboard.writeText(job.source.transcriptExcerpt!).then(() => toast.ok('Đã sao chép'), () => toast.err('Không sao chép được. Kiểm tra quyền truy cập bộ nhớ tạm.')) }}>Sao chép</Button></>}
        </section>
        <section aria-label="Template" {...stylex.props(d.sectionCard)}>
          <h2 {...stylex.props(d.sectionTitle)}>Template</h2>
          <span>{job.templateName || 'Chưa xác định mẫu video'}</span>
          <a href={`#/templates/${encodeURIComponent(job.templateId)}`} {...stylex.props(mw.moreLink)}>Mở template</a>
        </section>
        <section aria-label="Video nền" {...stylex.props(d.sectionCard)}>
          <h2 {...stylex.props(d.sectionTitle)}>Video nền</h2>
          {!!(backdropError || backdrop.error) && <div role="alert" {...stylex.props(d.writeFailed)}>{backdropError || 'Không tải được nền của video.'}<Button size="lg" onClick={() => void backdrop.refresh()}>Thử lại</Button></div>}
          {backdrop.initialLoading && <p role="status" {...stylex.props(d.sectionNote)}>Đang tải nền…</p>}
          {backdrop.data ? <>
            <span {...stylex.props(d.sectionNote)}>{coverage?.total == null ? 'Độ phủ chưa xác định' : coverage.short ? `Nền sẽ lặp ${durationLabel(coverage.total - coverage.covered)}` : 'Phủ kín video'} · {backdrop.data.items.length + (backdrop.data.fixedItems?.length ?? 0)} clip</span>
            {coverage?.short && <Button size="lg" disabled={backdropLocked} onClick={() => void changeBackdrop('add')}>Thêm clip</Button>}
            {building && <p {...stylex.props(d.sectionNote)}>Kiểm định đang dựng video. Đổi nền được sau khi dựng xong.</p>}
            {backdrop.data.busy && <p role="status" {...stylex.props(d.sectionNote)}>Đang chuẩn bị nền. Đổi nền được sau khi hoàn tất.</p>}
            {backdrop.data.sources?.map(source => <span key={source.channelId} {...stylex.props(d.sectionNote)}>{source.title || source.handle} · {source.platform}</span>)}
            <Button size="lg" disabled={backdropLocked || !backdrop.data.randomPicks} onClick={() => void changeBackdrop('reroll')}>Đổi ngẫu nhiên tất cả</Button>
            {backdrop.data.items.map((clip, index) => <div key={clip.id} {...stylex.props(mw.card)}>
              <span>{clip.title || clip.channelTitle || clip.channelHandle}</span>
              <span {...stylex.props(d.sectionNote)}>{clip.source === 'owner' ? 'Bạn chọn' : 'Kênh'} · {durationLabel(clip.startSec)}–{durationLabel(clip.endSec)}</span>
              <div {...stylex.props(mw.actionRow)}><Button size="lg" disabled={backdropLocked || clip.source === 'owner'} aria-label={`Đổi ngẫu nhiên clip ${index + 1}`} onClick={() => void changeBackdrop('reroll', clip.id)}>Đổi ngẫu nhiên</Button><Button size="lg" disabled={backdropLocked} aria-label={`Bỏ clip ${index + 1}`} onClick={() => void changeBackdrop('remove', clip.id)}>Bỏ clip</Button></div>
            </div>)}
            {backdrop.data.fixedItems?.map((clip, index) => <div key={clip.id} {...stylex.props(mw.card)}><span>{clip.title} · Cố định · {durationLabel(clip.startSec)}–{durationLabel(clip.endSec)}</span><Button size="lg" disabled={backdropLocked} aria-label={`Bỏ clip cố định ${index + 1}`} onClick={() => void changeBackdrop('remove', clip.id)}>Bỏ clip</Button></div>)}
            <p {...stylex.props(d.sectionNote)}>Sau khi đổi, app chỉ tải phần thay đổi rồi dựng lại video. Dựng lại không bốc lại clip.</p>
          </> : !backdrop.initialLoading && !backdrop.error && <p {...stylex.props(d.sectionNote)}>{job.queueItem ? 'Nền được chuẩn bị khi video bắt đầu được làm.' : 'Chưa có clip nền cho video này.'}</p>}
          <a href="#/library?tab=backdrops" {...stylex.props(mw.moreLink)}>Mở Thư viện video nền</a>
        </section>
      </div>
    </Sheet>
    {moreOpen && <OverflowMenu mobile job={job} onClose={() => setMoreOpen(false)} onRefresh={onRefresh} onError={onNotice} />}
    <MobileMetaSheet open={metaOpen !== null} initialView={metaOpen ?? 'edit'} jobId={job.id} onClose={() => setMetaOpen(null)} onRefresh={() => { void meta.refresh(); onRefresh() }} />
    <MobileImageSheet open={imageOpen} jobId={job.id} initialId={imageId} images={images} loading={imagesLoading} error={imagesError}
      onClose={() => setImageOpen(false)} onRetry={onRefresh} />
  </div>
}
