// Trang chi tiết video của Bàn dựng điện thoại (artboard MMain › detail): thanh
// tiêu đề 52px, bốn trợ lý một hàng, "Ảnh của video", Phòng vẽ, Nội dung đăng,
// Nhật ký và ô nhắn dính đáy. Không thanh thứ hai; tab ẩn nhờ useTabMode('hide').
import { Suspense, lazy, useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Shimmer, useEscapeLayer } from '@noverse/ui'
import { ApiError } from '../../../api/request'
import { getScoreArtifact, jobAction, type CloneJobView, type JobImageList } from '../../../api/workbench'
import { getMetadata } from '../../../api/workbenchPanels'
import { useResource } from '../../../data/useResource'
import { subscribeEvents } from '../../../data/events'
import { useOnline } from '../../../data/status'
import { requestDeleteExport, requestMarkPosted } from '../../../components/performance'
import { Orb } from '../../../components/Orb'
import { DevelopEffect } from '../../../components'
import { ASSISTANTS, ASSISTANT_INK, assistantName } from '../../../components/agents'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { workbenchPlaceholder } from '../../../components/assistant/placeholders'
import { progressState, progressText } from '../../../lib/progress-copy'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, routePath } from '../../../router'
import { imageTime } from '../workbench-logic'
import { durationLabel, imageLabel, imageUrl } from './mworkbench-logic'
import { mw, mwDetail as d } from './mworkbench.stylex'
import { OverflowMenu } from '../OverflowMenu'
import { PromptMenu } from '../PromptMenu'
import { MobileMetaSheet } from './MobileMetaSheet'
import { MobileImageSheet } from './MobileImageSheet'
const LearnSlot = lazy(() => import('../../../components/learn/LearnSlot').then((module) => ({ default: module.LearnSlot })))

const c = screenWorkbench.mobile
const STATE: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang làm', done: 'Xong', stopped: 'Dừng', needs_you: 'Chờ bạn' }
const STEP_AGENT: Record<string, string> = {
  download: 'scout', backdrop: 'scout', transcript: 'scout',
  metadata: 'director', scenes: 'director',
  images: 'artist',
  build: 'qa', score: 'qa', export: 'qa',
}

export function MobileWorkbenchDetail({ job, images, imagesLoading, imagesError, jobChannel, onBack, onRefresh, onPlan, onNotice, planOpen }: {
  job: CloneJobView
  images: JobImageList | undefined
  imagesLoading: boolean
  imagesError: unknown
  jobChannel?: string
  onBack: () => void
  onRefresh: () => void
  onPlan: (text: string) => void
  onNotice: (text: string) => void
  /** Sheet Kế hoạch đang mở: khoá ô nhập để không gửi trùng cùng một câu. */
  planOpen: boolean
}) {
  const [promptOpen, setPromptOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [metaOpen, setMetaOpen] = useState<'edit' | 'rewrite' | 'history' | null>(null)
  const [imageOpen, setImageOpen] = useState(false)
  const [whyOpen, setWhyOpen] = useState(false)
  const [imageId, setImageId] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  const online = useOnline()
  const draftInput = useRef<HTMLInputElement>(null)
  const sendLock = useRef(false)
  const wasPlanOpen = useRef(planOpen)
  const meta = useResource(['workbench', 'meta', job.id], ({ signal }) => getMetadata(job.id, signal))
  const scoreArtifact = job.steps?.find((step) => step.step === 'score')?.output?.artifacts?.find((item) => item.path.endsWith('score.json'))
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: Boolean(scoreArtifact) && job.activeStep === 'score' })
  const video = job.steps?.find((step) => step.step === 'export')?.output?.artifacts?.find((item) => item.contentType === 'video/mp4')
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
  // Câu đã gửi vẫn nằm trong ô: đóng sheet Kế hoạch thì người dùng sửa lại chính
  // câu đó, không mất bản nháp. Chỉ chặn gửi trùng khi sheet còn mở.
  const send = () => {
    const text = draft.trim()
    if (!text || planOpen || sendLock.current) return
    sendLock.current = true
    onPlan(text)
  }
  useEffect(() => {
    if (!planOpen) sendLock.current = false
    if (wasPlanOpen.current && !planOpen) draftInput.current?.focus()
    wasPlanOpen.current = planOpen
  }, [planOpen])
  const openImage = (id?: string) => { setImageId(id); setImageOpen(true) }
  const copyPackage = () => {
    if (!meta.data?.metadata) return
    const body = meta.data.metadata
    void navigator.clipboard.writeText(`${body.title}\n\n${body.description}\n\n${body.tags.join(', ')}`)
      .then(() => onNotice(c.metaPackageCopied)).catch(() => onNotice(c.metaCopyFailed))
  }

  useEscapeLayer(whyOpen && !metaOpen, () => setWhyOpen(false))
  useEffect(() => { setActionError('') }, [job.id])
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated'], () => { void meta.refresh() }), [meta.refresh])

  return <div {...stylex.props(d.root)}>
    <header {...stylex.props(d.header)}>
      <button type="button" aria-label="Quay lại Bàn dựng" onClick={onBack} {...stylex.props(d.headerBtn)}><Icon name="arrow-r" size={20} style={{ transform: 'rotate(180deg)' }} /></button>
      <span {...stylex.props(d.headerText)}>
        <span {...stylex.props(d.headerTitle)} title={job.source.title}>{job.source.title}</span>
        <span {...stylex.props(d.headerMeta)}>{[jobChannel, durationLabel(job.source.durationSec), job.templateName ?? job.templateId].filter(Boolean).join(' · ')}</span>
      </span>
      {job.stageGroup === 'working' && <button type="button" aria-label={c.pause} title={!online ? 'Mất kết nối' : c.pause} disabled={!online || actionBusy} aria-busy={actionBusy || undefined} onClick={() => void action('pause')} {...stylex.props(d.headerBtn)}><Icon name="pause" size={20} /></button>}
      <button type="button" aria-label={c.more} onClick={() => setMoreOpen(true)} {...stylex.props(d.headerBtn)}><Icon name="more" size={20} /></button>
    </header>

    <main {...stylex.props(d.scroll)}>
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

      {job.queueItem && <div role="status" {...stylex.props(d.sectionCard)}>
        <span {...stylex.props(d.reviewBannerText)}><Icon name="clock" size={18} /><span>{c.queued} Hoạ sĩ bắt đầu vẽ khi video được nhận lượt; chỉ dẫn cảnh và bìa đã chọn áp dụng lúc bắt đầu.</span></span>
        <a href="#/queue" {...stylex.props(mw.moreLink)}>{c.openQueue}<Icon name="arrow-r" size={14} /></a>
      </div>}

      <button type="button" aria-haspopup="dialog" onClick={() => setPromptOpen(true)} {...stylex.props(d.promptRow)}>
        <span {...stylex.props(d.promptIcon)}><Icon name="magicpen" size={18} /></span>
        <span {...stylex.props(d.promptText)}>
          <span {...stylex.props(d.promptTitle)}>{c.images}</span>
          <span {...stylex.props(d.promptSub)}>
            {images?.manifest
              ? `Cảnh ${images.manifest.scenePrompt.name} · Bìa ${images.manifest.coverPrompt.name}`
              : 'Chọn chỉ dẫn cảnh và ảnh bìa cho video này'}
          </span>
        </span>
        {(images?.manifest?.scenePrompt.overridden || images?.manifest?.coverPrompt.overridden) && <span {...stylex.props(d.promptBadge)}>Riêng video này</span>}
        <Icon name="arrow-r" size={15} xstyle={d.promptChevron} />
      </button>

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

      {!job.queueItem && <section aria-label="Phòng vẽ trực tiếp" {...stylex.props(mw.section)}>
        <div {...stylex.props(d.drawHead)}>
          <span {...stylex.props(d.drawTitle)}><Orb of="artist" size={14} />Phòng vẽ</span>
          <span {...stylex.props(d.drawCount)}>{drawn} / {items.length} ảnh{job.stageGroup === 'review' ? ' · chờ bạn duyệt' : ''}</span>
        </div>
        {imagesLoading && !images && <p role="status" {...stylex.props(d.drawEmpty)}>Đang tải ảnh…</p>}
        {imagesError != null && !images && <div role="alert" {...stylex.props(d.drawEmpty)}>{c.loadError} <Button size="sm" variant="quiet" onClick={onRefresh}>{c.retry}</Button></div>}
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

      <section aria-labelledby="pub-h" {...stylex.props(d.sectionCard)}>
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
            <Button size="sm" variant="ghost" icon={<Icon name="refresh" size={15} />} onClick={() => setMetaOpen('rewrite')}>{c.retryJob}</Button>
            <a href="#/settings" {...stylex.props(mw.moreLink)}>Đăng nhập lại ở Cài đặt ›</a>
          </div>
        </div>}

        {data && <>
          <div {...stylex.props(d.fieldLabel)}>
            <span>{c.metaSheet.titleField} · <span style={{ fontVariantNumeric: 'tabular-nums' }}>{data.title.length}/100</span></span>
            {(data.edited?.title || edited) && <span style={{ color: 'var(--vv-st-review)' }}>· bạn đã sửa</span>}
            <span {...stylex.props(d.fieldGrow)} />
            {data.reason?.title && !edited && <button type="button" aria-expanded={whyOpen} onClick={() => setWhyOpen(!whyOpen)} {...stylex.props(d.whyBtn)}><Icon name="info" size={13} />{c.metaWhy}</button>}
          </div>
          <button type="button" aria-label={c.metaSheet.titleField} onClick={() => setMetaOpen('edit')} {...stylex.props(d.titleBox)}>{data.title}</button>
          {whyOpen && data.reason?.title && <div {...stylex.props(mw.field)}>
            <ReasonPopover reason={data.reason.title} label={c.metaWhy} />
            <Button variant="quiet" size="sm" onClick={() => setWhyOpen(false)}>{c.metaWhyClose}</Button>
          </div>}
          {!metaOpen && <Suspense fallback={null}><LearnSlot ownerKey={`${job.channelId}:${job.id}`} learn={meta.data?.learn} /></Suspense>}

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
            <Button size="sm" variant="ghost" icon={<Icon name="magic" size={16} />} onClick={() => setMetaOpen('rewrite')}>{c.metaSheet.rewrite}</Button>
            <Button size="sm" variant="ghost" aria-label={c.metaSheet.history} icon={<Icon name="clock" size={16} />} onClick={() => setMetaOpen('history')} />
            <Button size="sm" variant="ghost" icon={<Icon name="copy" size={16} />} onClick={copyPackage}>{c.metaCopyPackage}</Button>
          </div>
          <span {...stylex.props(d.metaHint)}>Chạm vào ô để sửa; chỉ áp cho video này. Đoạn cuối và tag luôn thêm sửa ở Hồ sơ kênh.</span>
        </>}
        {!meta.data && !missing && meta.error == null && <p role="status" {...stylex.props(d.sectionNote)}>Đang tải nội dung đăng…</p>}
        {meta.error != null && !missing && <div role="alert" {...stylex.props(d.writeFailed)}>{c.loadError} <Button size="sm" variant="quiet" onClick={() => void meta.refresh()}>{c.retry}</Button></div>}
      </section>

      <section aria-label="Nhật ký tiến độ" {...stylex.props(d.sectionCard)}>
        <div {...stylex.props(d.sectionHead)}>
          <h2 {...stylex.props(d.sectionTitle)}><Icon name="activity" size={15} />{c.log}</h2>
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
        {score.initialLoading ? <p role="status" {...stylex.props(d.sectionNote)}>Đang tải kết quả…</p> : score.error ? <div role="alert" {...stylex.props(d.writeFailed)}>Không tải được kết quả kiểm định. <Button size="sm" variant="quiet" onClick={() => void score.refresh()}>{c.retryJob}</Button></div> : <>
          {score.data != null && <span {...stylex.props(mw.tileValue)}>{score.data.total} điểm</span>}
          {score.data?.blockers?.length ? <ul>{score.data.blockers.map((item) => <li key={item.id}>{item.message}</li>)}</ul> : <p {...stylex.props(d.sectionNote)}>Không có lỗi chặn.</p>}
        </>}
      </section>}

      {(job.stageGroup === 'ready' || job.stageGroup === 'posted') && <section aria-label={c.readyTitle} {...stylex.props(d.videoCard)}>
        <h2 {...stylex.props(d.sectionTitle)}><Icon name="video-add" size={15} />{c.readyTitle}</h2>
        {video ? <video controls {...stylex.props(d.viewer)} src={`/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(video.id)}`} /> : <p {...stylex.props(d.sectionNote)}>{c.noVideoFile}</p>}
        {video && <a {...stylex.props(mw.moreLink)} href={`/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(video.id)}`} target="_blank" rel="noreferrer">Mở file video<Icon name="arrow-r" size={14} /></a>}
        {job.stageGroup === 'posted' && hasExport && <Button size="sm" variant="ghost" icon={<Icon name="trash" size={15} />} onClick={deleteMp4}>{c.deleteMp4}</Button>}
      </section>}
    </main>

    <form onSubmit={(event) => { event.preventDefault(); send() }} aria-busy={planOpen || undefined} {...stylex.props(d.composerBar, d.composerSafe)}>
      <div {...stylex.props(d.composerCard)}>
        <label htmlFor="mmsg" {...stylex.props(mw.srOnly)}>{c.composerDetail}</label>
        <input ref={draftInput} id="mmsg" value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={workbenchPlaceholder(job)} aria-label={c.composerDetail}
          disabled={planOpen} autoComplete="off" enterKeyHint="send" autoCapitalize="sentences" {...stylex.props(d.composerInput)} />
        <Button type="submit" variant="primary" size="lg" aria-label={c.send} disabled={!draft.trim() || planOpen} aria-busy={planOpen || undefined} xstyle={d.composerSend} icon={<Icon name="arrow-up" size={18} />} />
      </div>
    </form>

    {promptOpen && <PromptMenu mobile job={job} onClose={() => setPromptOpen(false)} onRefresh={onRefresh} />}
    {moreOpen && <OverflowMenu mobile job={job} onClose={() => setMoreOpen(false)} onRefresh={onRefresh} onError={onNotice} />}
    <MobileMetaSheet open={metaOpen !== null} initialView={metaOpen ?? 'edit'} jobId={job.id} onClose={() => setMetaOpen(null)} onRefresh={() => { void meta.refresh(); onRefresh() }} />
    <MobileImageSheet open={imageOpen} jobId={job.id} initialId={imageId} images={images} loading={imagesLoading} error={imagesError}
      onClose={() => setImageOpen(false)} onChangePrompt={() => { setImageOpen(false); setPromptOpen(true) }} onRetry={onRefresh} />
  </div>
}
