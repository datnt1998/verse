// Trang chi tiết video của Bàn dựng điện thoại (artboard MMain › detail): một
// trang điện thoại đầy đủ, không thanh thứ hai — thanh ứng dụng của shell vẫn ở
// trên, thanh tab ẩn nhờ useTabMode('hide') ở màn cha. Ảnh, prompt, meta, agent
// và thao tác đều đọc/ghi API thật.
import { Suspense, lazy, useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, MeterBar, Shimmer, StatusPill, useEscapeLayer } from '@noverse/ui'
import { ApiError } from '../../../api/request'
import { getScoreArtifact, jobAction, jobBulkAction, type CloneJobView, type JobImageList } from '../../../api/workbench'
import { getMetadata } from '../../../api/workbenchPanels'
import { useResource } from '../../../data/useResource'
import { subscribeEvents } from '../../../data/events'
import { requestMarkPosted } from '../../../components/performance'
import { Orb } from '../../../components/Orb'
import { DevelopEffect } from '../../../components'
import { ASSISTANTS, assistantName } from '../../../components/agents'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { workbenchPlaceholder } from '../../../components/assistant/placeholders'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, routePath } from '../../../router'
import { imageTime } from '../workbench-logic'
import { durationLabel, imageLabel, imageUrl, jobStatusLine, runningProgress, stageToneOf } from './mworkbench-logic'
import { mw } from './mworkbench.stylex'
import { OverflowMenu } from '../OverflowMenu'
import { PromptMenu } from '../PromptMenu'
import { MobileMetaSheet } from './MobileMetaSheet'
import { MobileImageSheet } from './MobileImageSheet'
const LearnSlot = lazy(() => import('../../../components/learn/LearnSlot').then((module) => ({ default: module.LearnSlot })))

const c = screenWorkbench.mobile
const STATE: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang chạy', done: 'Xong', stopped: 'Dừng', needs_you: 'Chờ bạn' }
const stage: Record<string, string> = { attention: 'Cần xử lý', review: 'Chờ duyệt', paused: 'Tạm dừng', working: 'Đang làm', queued: 'Chờ lượt', ready: 'Sẵn sàng đăng', posted: 'Đã đăng' }

export function MobileWorkbenchDetail({ job, images, imagesLoading, imagesError, jobChannel, onBack, onRefresh, onPlan, onNotice }: {
  job: CloneJobView
  images: JobImageList | undefined
  imagesLoading: boolean
  imagesError: unknown
  jobChannel?: string
  onBack: () => void
  onRefresh: () => void
  onPlan: (text: string) => void
  onNotice: (text: string) => void
}) {
  const [promptOpen, setPromptOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [metaOpen, setMetaOpen] = useState<'edit' | 'rewrite' | 'history' | null>(null)
  const [imageOpen, setImageOpen] = useState(false)
  const [whyOpen, setWhyOpen] = useState(false)
  const [imageId, setImageId] = useState<string | undefined>(undefined)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const meta = useResource(['workbench', 'meta', job.id], ({ signal }) => getMetadata(job.id, signal))
  const scoreArtifact = job.steps?.find((step) => step.step === 'score')?.output?.artifacts?.find((item) => item.path.endsWith('score.json'))
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: Boolean(scoreArtifact) && job.activeStep === 'score' })
  const video = job.steps?.find((step) => step.step === 'export')?.output?.artifacts?.find((item) => item.contentType === 'video/mp4')
  const progress = runningProgress(job)
  const items = images?.items ?? []
  useEscapeLayer(whyOpen && !metaOpen, () => setWhyOpen(false))

  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated'], () => { void meta.refresh() }), [meta.refresh])

  const action = async (kind: 'pause' | 'resume' | 'run' | 'cancel') => {
    setBusy(true); setNotice('')
    try { await jobAction(job.id, kind); onRefresh() } catch (err) { setNotice(err instanceof Error ? err.message : 'Không thực hiện được') } finally { setBusy(false) }
  }
  const markPosted = () => { setBusy(true); void requestMarkPosted(job, onRefresh).catch((err: unknown) => onNotice(err instanceof Error ? err.message : 'Không đánh dấu được')).finally(() => setBusy(false)) }
  const deleteExport = () => { if (!window.confirm(c.deleteMp4Confirm)) return; setBusy(true); void jobBulkAction(job.id, 'deleteExport').then(onRefresh).catch((err: unknown) => onNotice(err instanceof Error ? err.message : 'Không xoá được')).finally(() => setBusy(false)) }
  const send = () => { const text = draft.trim(); if (!text) return; onPlan(text) }

  return <div {...stylex.props(mw.detailRoot)}>
    <div {...stylex.props(mw.backRow)}>
      <Button size="sm" variant="quiet" onClick={onBack} aria-label={`Quay lại ${c.back}`}>‹ {c.back}</Button>
    </div>

    <div>
      <h1 {...stylex.props(mw.detailTitle)}>{job.source.title}</h1>
      <div {...stylex.props(mw.detailMeta)}>
        {jobChannel && <span>{jobChannel}</span>}
        {jobChannel && durationLabel(job.source.durationSec) && <span {...stylex.props(mw.dot)}>·</span>}
        {durationLabel(job.source.durationSec) && <span>{durationLabel(job.source.durationSec)}</span>}
        <span {...stylex.props(mw.dot)}>·</span>
        <span>{job.templateName ?? job.templateId}</span>
      </div>
    </div>

    <div {...stylex.props(mw.chips)}>
      <span {...stylex.props(mw.chip)}><StatusPill status={stageToneOf(job.stageGroup)}>{stage[job.stageGroup] ?? job.stageGroup}</StatusPill></span>
      {job.agentView?.find((agent) => agent.state === 'running')?.now && <span {...stylex.props(mw.chip)}>{jobStatusLine(job)}</span>}
    </div>

    <div {...stylex.props(mw.actionRow)}>
      {job.stageGroup === 'working' && <Button size="lg" variant="ghost" disabled={busy} onClick={() => void action('pause')}>{c.pause}</Button>}
      {job.stageGroup === 'paused' && <Button size="lg" variant="primary" disabled={busy} onClick={() => void action('resume')}>{c.resume}</Button>}
      {job.stageGroup === 'attention' && <Button size="lg" variant="primary" disabled={busy} onClick={() => void action('run')}>{c.retryJob}</Button>}
      {job.stageGroup === 'review' && <Button size="lg" variant="primary" onClick={() => navigate(routePath({ name: 'review', video: job.id }))}>{c.reviewImages}</Button>}
      {job.stageGroup === 'ready' && <Button size="lg" variant="primary" onClick={markPosted} disabled={busy}>{c.markPosted}</Button>}
      {job.stageGroup === 'ready' && <a {...stylex.props(mw.moreLink)} href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`}>{c.download} ›</a>}
      {job.stageGroup === 'posted' && video && <Button size="lg" variant="ghost" disabled={busy} onClick={deleteExport}>{c.deleteMp4}</Button>}
      <Button size="lg" variant="ghost" aria-label={c.more} onClick={() => setMoreOpen(true)}>⋯</Button>
    </div>

    {job.stageGroup === 'review' && <div {...stylex.props(mw.banner)}><span>Ảnh đã sẵn sàng để bạn duyệt.</span><a {...stylex.props(mw.moreLink)} href={`#/review?video=${encodeURIComponent(job.id)}`}>{c.reviewImages} ›</a></div>}
    {job.stageGroup === 'paused' && <div {...stylex.props(mw.banner)}>Video đang tạm dừng. Bạn có thể góp ý trước khi chạy tiếp.</div>}
    {job.stageGroup === 'attention' && <div {...stylex.props(mw.banner)}>Video cần bạn xử lý. {job.subline}</div>}
    {job.queueItem && <div {...stylex.props(mw.banner)}><span>{c.queued}</span><a {...stylex.props(mw.moreLink)} href="#/queue">{c.openQueue} ›</a></div>}

    {progress && <div {...stylex.props(mw.card)}>
      <div {...stylex.props(mw.cardHead)}><strong {...stylex.props(mw.cardTitle)}>{job.agentView?.find((agent) => agent.state === 'running')?.now ?? job.subline}</strong><span {...stylex.props(mw.cardHint)}>{progress.done}/{progress.total}</span></div>
      <MeterBar value={progress.pct} tone="info" />
    </div>}

    <section aria-label={c.agents} {...stylex.props(mw.card)}>
      <div {...stylex.props(mw.cardHead)}><h2 {...stylex.props(mw.cardTitle)}>{c.agents}</h2></div>
      {job.agentView?.length ? <div {...stylex.props(mw.agentGrid)}>{ASSISTANTS.map((assistant) => {
        const agent = job.agentView?.find((item) => item.id === assistant)
        return <div key={assistant} {...stylex.props(mw.agentTile)}>
          <div {...stylex.props(mw.agentTop)}>
            <Orb of={assistant} size={18} live={agent?.state === 'running'} />
            <span {...stylex.props(mw.agentName)}>{assistantName(assistant)}</span>
          </div>
          <span {...stylex.props(mw.agentState)}>{agent ? STATE[agent.state] ?? agent.state : 'Chưa chạy'}</span>
          {agent?.now && <span {...stylex.props(mw.agentNow)}>{agent.now}</span>}
          {agent?.metric && <span {...stylex.props(mw.agentMetric)}>{agent.metric.label}: {agent.metric.value}</span>}
        </div>
      })}</div> : <p {...stylex.props(mw.notice)}>{c.agentsEmpty}</p>}
    </section>

    {job.stageGroup === 'attention' && job.activeStep === 'score' && <section aria-label="Kết quả kiểm định" {...stylex.props(mw.card)}>
      <h2 {...stylex.props(mw.cardTitle)}>Kết quả kiểm định</h2>
      {score.initialLoading ? <p role="status" {...stylex.props(mw.notice)}>Đang tải kết quả…</p> : score.error ? <div role="alert" {...stylex.props(mw.error)}>Không tải được kết quả kiểm định. <Button size="sm" variant="quiet" onClick={() => void score.refresh()}>{c.retryJob}</Button></div> : <>
        <span {...stylex.props(mw.tileValue)}>{score.data?.total} điểm</span>
        {score.data?.blockers?.length ? <ul>{score.data.blockers.map((item) => <li key={item.id}>{item.message}</li>)}</ul> : <p {...stylex.props(mw.notice)}>Không có lỗi chặn.</p>}
      </>}
    </section>}

    <section aria-label={c.images} {...stylex.props(mw.card)}>
      <div {...stylex.props(mw.cardHead)}>
        <h2 {...stylex.props(mw.cardTitle)}>{c.images}</h2>
        <Button size="sm" variant="ghost" onClick={() => { setImageId(items[0]?.id); setImageOpen(true) }}>{c.changePrompt}</Button>
      </div>
      {images?.manifest && <p {...stylex.props(mw.promptLine)}>
        {c.promptScene(images.manifest.scenePrompt.name)}{images.manifest.scenePrompt.overridden && <span {...stylex.props(mw.promptTag)}> · {c.promptOverride}</span>}<br />
        {c.promptCover(images.manifest.coverPrompt.name)}{images.manifest.coverPrompt.overridden && <span {...stylex.props(mw.promptTag)}> · {c.promptOverride}</span>}
      </p>}
      {imagesLoading && !images && <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>}
      {imagesError != null && !images && <div role="alert" {...stylex.props(mw.error)}>{c.loadError} <Button size="sm" variant="quiet" onClick={onRefresh}>{c.retry}</Button></div>}
      {images && items.length === 0 && <p {...stylex.props(mw.notice)}>{job.noImages ? c.noImages : c.imagesEmpty}</p>}
      {items.length > 0 && <div {...stylex.props(mw.imageGrid)}>{items.slice(0, 6).map((item, index) => {
        const src = imageUrl(item)
        const drawing = item.status === 'running' || item.status === 'pending' || item.status === 'queued'
        const timing = imageTime(item)
        return <button key={item.id} type="button" {...stylex.props(mw.imageTile, (item.kind === 'thumbnail' || item.kind === 'cover') && mw.imageCover)} onClick={() => { setImageId(item.id); setImageOpen(true) }}>
          <span {...stylex.props(mw.imageFrame)}>
            {drawing ? <><DevelopEffect state="drawing" baseUrl={src} compact /><span role="status" {...stylex.props(mw.imageDrawingStatus)}><Orb of="artist" size={14} live /><Shimmer>{item.status === 'running' ? screenWorkbench.live.drawing : screenWorkbench.live.queued}</Shimmer></span></> : src ? <img {...stylex.props(mw.image)} src={src} alt={imageLabel(item, index)} /> : screenWorkbench.live.queued}
            <span {...stylex.props(mw.imageBadge)}>{imageLabel(item, index)}</span>
          </span>
          <span {...stylex.props(mw.imageCapRow)}><span {...stylex.props(mw.imageCap)}>{item.narration || imageLabel(item, index)}</span>{timing && <span {...stylex.props(mw.imageTime)}>{timing}</span>}</span>
        </button>
      })}</div>}
      {items.length > 6 && <Button size="sm" variant="quiet" onClick={() => { setImageId(items[0]?.id); setImageOpen(true) }}>Xem cả {items.length} ảnh ›</Button>}
      <div {...stylex.props(mw.actionRow)}>
        <Button size="sm" variant="ghost" onClick={() => setPromptOpen(true)}>{c.changePrompt}</Button>
        <a {...stylex.props(mw.moreLink)} href={`#/review?video=${encodeURIComponent(job.id)}`}>{c.reviewImages} ›</a>
      </div>
    </section>

    <section aria-label={c.meta} {...stylex.props(mw.card)}>
      <div {...stylex.props(mw.cardHead)}>
        <h2 {...stylex.props(mw.cardTitle)}>{c.meta}</h2>
        <Button size="sm" variant="ghost" onClick={() => setMetaOpen('edit')} disabled={!meta.data}>{c.metaEdit}</Button>
      </div>
      {!meta.data && !meta.error && <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>}
      {meta.error != null && !meta.data && !(meta.error instanceof ApiError && meta.error.status === 404) && <div role="alert" {...stylex.props(mw.error)}>{c.loadError} <Button size="sm" variant="quiet" onClick={() => void meta.refresh()}>{c.retry}</Button></div>}
      {meta.error != null && meta.error instanceof ApiError && meta.error.status === 404 && !meta.data && <p {...stylex.props(mw.notice)}>{c.metaEmpty}</p>}
      {meta.data?.status === 'pending' && <><p {...stylex.props(mw.notice)}>{c.metaPending}</p><p {...stylex.props(mw.cardHint)}>{c.metaPendingHint}</p></>}
      {meta.data?.status === 'writing' && <p role="status" {...stylex.props(mw.notice)}>{c.metaWriting}</p>}
      {meta.data?.status === 'failed' && <div role="alert" {...stylex.props(mw.error)}>{c.metaFailed} <Button size="sm" variant="quiet" onClick={() => setMetaOpen('rewrite')}>{c.retryJob}</Button></div>}
      {meta.data?.metadata && <>
        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldLabel)}>{c.metaSheet.titleField} · {meta.data.metadata.title.length}/100</span>
          <textarea readOnly rows={2} aria-label={c.metaSheet.titleField} value={meta.data.metadata.title} {...stylex.props(mw.input)} onClick={() => setMetaOpen('edit')} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); setMetaOpen('edit') } }} />
        </div>
        <div {...stylex.props(mw.metaActions)}>
          <span {...stylex.props(mw.cardHint)}>{meta.data.metadata.editedBy === 'owner' || Object.values(meta.data.metadata.edited ?? {}).some(Boolean) ? c.metaEdited : c.metaAI}{meta.data.metadata.editedAt ? ` · ${new Date(meta.data.metadata.editedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}` : ''}</span>
          {meta.data.metadata.reason?.title && <Button variant="quiet" size="sm" aria-expanded={whyOpen} onClick={() => setWhyOpen(!whyOpen)}>{c.metaWhy}</Button>}
        </div>
        {whyOpen && meta.data.metadata.reason?.title && <div {...stylex.props(mw.field)}>
          <ReasonPopover reason={meta.data.metadata.reason.title} />
          <Button variant="quiet" size="sm" onClick={() => setWhyOpen(false)}>{c.metaWhyClose}</Button>
        </div>}
        {!metaOpen && <Suspense fallback={null}><LearnSlot ownerKey={`${job.channelId}:${job.id}`} learn={meta.data.learn} /></Suspense>}
        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldLabel)}>{c.metaSheet.descriptionField} · {meta.data.metadata.description.length}/5.000</span>
          <textarea readOnly rows={3} aria-label={c.metaSheet.descriptionField} value={meta.data.metadata.description} {...stylex.props(mw.input)} onClick={() => setMetaOpen('edit')} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); setMetaOpen('edit') } }} />
        </div>
        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldLabel)}>{c.metaSheet.tagsField} · {meta.data.metadata.tags.length}/30</span>
          <div {...stylex.props(mw.tagRow)}>{meta.data.metadata.tags.map((tag, index) => <span key={`${tag}-${index}`} {...stylex.props(mw.tag)}>{meta.data?.alwaysTags?.includes(tag) && <span aria-label="Tag luôn thêm của kênh">⌂ </span>}{tag}</span>)}</div>
        </div>
        <div {...stylex.props(mw.metaActions)}>
          <Button variant="ghost" size="sm" onClick={() => setMetaOpen('rewrite')}>{c.metaSheet.rewrite}</Button>
          <Button variant="ghost" size="sm" onClick={() => setMetaOpen('history')}>{c.metaSheet.history}</Button>
          <Button variant="ghost" size="sm" onClick={() => {
            const data = meta.data?.metadata
            if (!data) return
            void navigator.clipboard.writeText(`${data.title}\n\n${data.description}\n\n${data.tags.join(', ')}`).then(() => setNotice(c.metaPackageCopied)).catch(() => setNotice(c.metaCopyFailed))
          }}>{c.metaCopyPackage}</Button>
        </div>
      </>}
    </section>

    <section aria-label={c.log} {...stylex.props(mw.card)}>
      <h2 {...stylex.props(mw.cardTitle)}>{c.log}</h2>
      {job.steps?.length ? job.steps.map((step) => <div key={step.step} {...stylex.props(mw.runRow)}>
        <span>{step.startedAt ? new Date(step.startedAt).toLocaleTimeString('vi-VN') : ''}</span>
        <span>{c.progressSteps[step.progress.phase] || step.progress.phase || c.progressSteps[step.step] || c.progressUnknown}</span>
        <span {...stylex.props(mw.runState)}>{step.error?.message || c.progressStates[step.status] || c.progressUnknown}</span>
      </div>) : <p {...stylex.props(mw.notice)}>{c.logEmpty}</p>}
    </section>

    {job.stageGroup === 'ready' || job.stageGroup === 'posted' ? <section aria-label={c.readyTitle} {...stylex.props(mw.card)}>
      <h2 {...stylex.props(mw.cardTitle)}>{c.readyTitle}</h2>
      {video ? <video controls {...stylex.props(mw.viewer)} src={`/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(video.id)}`} /> : <p {...stylex.props(mw.notice)}>{c.noVideoFile}</p>}
    </section> : null}

    <form {...stylex.props(mw.composerFoot)} onSubmit={(event) => { event.preventDefault(); send() }}>
      <input {...stylex.props(mw.composerInput)} aria-label={c.composerDetail} value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={workbenchPlaceholder(job)} />
      <Button type="submit" size="lg" variant="primary" disabled={!draft.trim()}>{c.send}</Button>
    </form>
    {notice && <p role="status" {...stylex.props(mw.notice)}>{notice}</p>}

    {promptOpen && <PromptMenu mobile job={job} onClose={() => setPromptOpen(false)} onRefresh={onRefresh} />}
    {moreOpen && <OverflowMenu mobile job={job} onClose={() => setMoreOpen(false)} onRefresh={onRefresh} onError={setNotice} />}
    <MobileMetaSheet open={metaOpen !== null} initialView={metaOpen ?? 'edit'} jobId={job.id} onClose={() => setMetaOpen(null)} onRefresh={() => { void meta.refresh(); onRefresh() }} />
    <MobileImageSheet open={imageOpen} jobId={job.id} initialId={imageId} images={images} loading={imagesLoading} error={imagesError}
      onClose={() => setImageOpen(false)} onChangePrompt={() => { setImageOpen(false); setPromptOpen(true) }} onRetry={onRefresh} />
  </div>
}
