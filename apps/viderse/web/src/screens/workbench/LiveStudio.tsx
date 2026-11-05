// Phòng vẽ / Dựng & xuất / Thành phẩm của Bàn dựng PC (artboard Main › stage).
// Bảy trạng thái canonical dùng cùng dữ liệu thật của job: ảnh (§26), bước chạy,
// Độ hoàn thiện và file xuất. Không có số liệu hay ảnh mẫu nào được bịa.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon, Shimmer, TextArea } from '@noverse/ui'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { getJobImages, getScoreArtifact, redrawAll, redrawImage, type CloneJobView, type JobImage, type JobStep } from '../../api/workbench'
import { useResource } from '../../data/useResource'
import { subscribeEvents } from '../../data/events'
import { DevelopEffect, Orb } from '../../components'
import { screenWorkbench } from '../../i18n/screens'
import { progressState, progressText } from '../../lib/progress-copy'
import { clockLabel, finishedVideo, imageTime, issueReason, jobStage, stepDuration, stepPercent, QUALITY_MIN_SCORE, type FinishedVideo, type JobStage } from './workbench-logic'

type Props = {
  job: CloneJobView
  onChangePrompt: () => void
  onRefresh: () => void
  /** Duyệt cả video & dựng — do Bàn dựng giữ khoá/đếm ảnh để header cũng dùng. */
  onApproveAll?: () => void
  approveBusy?: boolean
  /** Nút “Vẽ lại tất cả” ở header mở hộp thoại trong phần này. */
  redrawAllOpen?: boolean
  onRedrawAllChange?: (open: boolean) => void
}

const STEP_LABEL = screenWorkbench.progress.steps
const isDrawing = (status: string) => status === 'running' || status === 'pending' || status === 'queued'
const isWaiting = (status: string) => status === 'pending' || status === 'queued'

const copy = screenWorkbench.live
const agentOrb = { images: 'artist', review: 'artist', issue: 'artist', paused: 'artist', render: 'qa', quality: 'qa', done: 'qa', queued: 'qa' } as const

export function LiveStudio({ job, onChangePrompt, onRefresh, onApproveAll, approveBusy, redrawAllOpen, onRedrawAllChange }: Props) {
  const stage = jobStage(job)
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: stage !== 'queued' })
  const scoreArtifact = job.steps?.find(step => step.step === 'score')?.output?.artifacts?.find(item => item.path.endsWith('score.json'))
  const video = finishedVideo(job)
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: !!scoreArtifact })
  // Hộp vẽ lại gắn với đúng video đang mở: đổi video thì hộp cũ không còn hiệu lực.
  const [dialog, setDialog] = useState<{ jobId: string; imageId: string | 'all' } | null>(null)
  const [selectedTile, setSelectedTile] = useState(0)
  const [feedback, setFeedback] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { setSelectedTile(0); setDialog(null); setError('') }, [job.id])
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated', 'image.progress'], () => { void images.refresh() }), [images.refresh])
  useEffect(() => { if (redrawAllOpen) { setDialog({ jobId: job.id, imageId: 'all' }); setError(''); setFeedback('') } }, [redrawAllOpen])

  const items = images.data?.items || []
  const total = items.length
  const drawn = items.filter(item => !isDrawing(item.status) && item.status !== 'failed').length
  const drawingItems = items.filter(item => isDrawing(item.status))
  const failedItem = items.find(item => item.status === 'failed')
  const firstDrawing = items.findIndex(item => isDrawing(item.status))
  const drawing = drawingItems.length > 0
  const selected = Math.min(selectedTile, Math.max(0, total - 1))
  // `dialog` là 'all' hoặc đúng id một ảnh, gắn với video lúc mở. Danh sách có
  // thể làm mới trong lúc hộp mở, nên phải phân biệt "một ảnh" với "cả bộ"
  // trước khi gửi.
  const targetId = dialog && dialog.imageId !== 'all' && dialog.jobId === job.id ? dialog.imageId : undefined
  const singleTarget = targetId !== undefined
  const target = targetId !== undefined ? items.find(item => item.id === targetId) : undefined
  const targetLost = singleTarget && !!images.data && !target
  const targetIndex = target ? items.indexOf(target) : -1
  const failedStep = job.steps?.find(step => step.status === 'failed')
  const imageLabel = (item: JobImage, index: number) => item.kind === 'thumbnail' || item.kind === 'cover' ? copy.cover : copy.scene(index + 1)
  const pipeline = ['build', 'score', 'export'].map(key => job.steps?.find(step => step.step === key))
  const exportStep = pipeline[2]
  const completed = pipeline.filter(step => step && !['running', 'pending', 'queued'].includes(step.status)).length
  const percent = stage === 'done' ? 100
    : stage === 'render' ? (exportStep ? stepPercent(exportStep) : null) ?? Math.round(completed / 3 * 100)
      : stage === 'quality' ? Math.round(completed / 3 * 100)
        : total ? Math.round(drawn / total * 100) : 0
  const runStep = job.steps?.find(step => step.status === 'running')
  const [, setClockTick] = useState(0)
  useEffect(() => {
    if (!runStep) return
    const timer = window.setInterval(() => setClockTick(value => value + 1), 1000)
    return () => window.clearInterval(timer)
  }, [runStep])
  const canApprove = stage === 'review' && total > 0 && !drawing && !approveBusy
  const closeRedraw = () => { setDialog(null); onRedrawAllChange?.(false) }

  // Ảnh đã chọn không còn trong bộ ảnh (bộ ảnh/manifest đổi trong lúc hộp mở):
  // đóng hộp, chọn lại ảnh còn tồn tại và báo rõ. Không bao giờ biến ý định vẽ
  // một ảnh thành vẽ lại cả bộ — việc đó tốn hạn mức ảnh.
  useEffect(() => {
    if (!targetLost) return
    setDialog(null)
    setSelectedTile(0)
    setError('Ảnh đã chọn không còn trong bộ ảnh. Chưa vẽ lại ảnh nào — chọn ảnh khác rồi thử lại.')
  }, [targetLost])

  const submit = async () => {
    if (!images.data?.manifest || !dialog || dialog.jobId !== job.id) return
    if (singleTarget && !target) return
    setBusy(true); setError('')
    try {
      if (target) await redrawImage(job.id, target.id, images.data.manifest.revision, feedback)
      else await redrawAll(job.id, images.data.manifest.revision, items.length, feedback)
      closeRedraw(); setFeedback(''); void images.refresh(); onRefresh()
    } catch (err) { setError(err instanceof Error ? err.message : 'Không vẽ lại được') } finally { setBusy(false) }
  }

  const openRedraw = (imageId: string) => { setError(''); setFeedback(''); setDialog({ jobId: job.id, imageId }) }

  const redrawSelected = () => {
    const item = items[selected]
    if (!item || isDrawing(item.status) || stage === 'done') return
    openRedraw(item.id)
  }

  // Phím của Phòng vẽ/duyệt tại chỗ: 1–9 chọn ảnh, R vẽ lại ảnh đang chọn, A duyệt cả video.
  useEffect(() => {
    if (!['images', 'review', 'issue', 'paused'].includes(stage)) return
    const onKey = (event: KeyboardEvent) => {
      const node = event.target as HTMLElement | null
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || !node || dialog) return
      if (node.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(node.tagName)) return
      if (/^[1-9]$/.test(event.key)) {
        const index = Number(event.key) - 1
        if (index < items.length) { event.preventDefault(); setSelectedTile(index) }
        return
      }
      if (event.key.toLowerCase() === 'r') { event.preventDefault(); redrawSelected(); return }
      if (event.key.toLowerCase() === 'a' && canApprove) { event.preventDefault(); onApproveAll?.() }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [stage, items, selected, canApprove, onApproveAll, dialog])

  const meta = stageMeta(stage, { total, drawn, manifest: images.data?.manifest, job, drawingIndex: firstDrawing, runStep })
  const activeStep = job.steps?.find(step => step.status === 'running') ?? failedStep

  return <section {...stylex.props(styles.root)} aria-label={meta.title}>
    <div role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-label={meta.title} {...stylex.props(styles.progressTrack)}>
      <div {...stylex.props(styles.progressFill)} style={{ width: `${percent}%`, background: meta.gradient }} />
    </div>
    <div {...stylex.props(styles.body)}>
      <div {...stylex.props(styles.header)}>
        <span {...stylex.props(styles.orb)}><Orb of={agentOrb[stage]} size={16} live={drawing || stage === 'render'} /></span>
        <h2 {...stylex.props(styles.streamTitle)}>{meta.title}</h2>
        <span {...stylex.props(styles.meta)}>{meta.meta}</span>
        <span {...stylex.props(styles.spacer)} />
        {stage === 'review' && total > 1 && <span {...stylex.props(styles.keyHint)}><span {...stylex.props(styles.kbd)}>1–{total}</span>chọn ảnh<span {...stylex.props(styles.kbd)}>R</span>vẽ lại<span {...stylex.props(styles.kbd)}>A</span>duyệt tất cả</span>}
        {runStep && <span {...stylex.props(styles.run)}><Shimmer>{meta.runLabel}</Shimmer><span {...stylex.props(styles.elapsed)}>{stepDuration(runStep)}</span></span>}
      </div>

      {(stage === 'images' || stage === 'review' || stage === 'issue' || stage === 'paused') && (images.error
        ? <p role="alert" {...stylex.props(styles.notice)}>Không tải được bộ ảnh. <Button variant="quiet" size="sm" onClick={() => void images.refresh()}>Thử lại</Button></p>
        : total === 0
          ? <div role="status" {...stylex.props(styles.notice)}>{images.initialLoading ? 'Đang tải bộ ảnh…' : job.noImages ? 'Video này không tạo ảnh.' : job.subline || 'Chưa có ảnh. Hoạ sĩ vẽ sau khi Đạo diễn chia cảnh.'}</div>
          : <div role="listbox" aria-label="Ảnh của video" {...stylex.props(styles.grid)}>{items.map((item, index) => {
            const url = (item.versions ?? []).find(version => version.version === (item.reviewVersion ?? item.currentVersion))?.originalUrl ?? (item.versions ?? []).at(-1)?.originalUrl
            const status = item.status
            const tileDrawing = isDrawing(status)
            const waiting = isWaiting(status)
            const failed = status === 'failed'
            const label = imageLabel(item, index)
            const timing = imageTime(item)
            return <div
              key={item.id}
              role="option"
              aria-selected={selected === index}
              aria-label={label}
              tabIndex={0}
              onClick={() => setSelectedTile(index)}
              onFocus={() => setSelectedTile(index)}
              {...stylex.props(styles.tile, (item.kind === 'thumbnail' || item.kind === 'cover') && styles.coverTile, selected === index && styles.tileSelected)}
            >
              <div {...stylex.props(styles.preview, tileDrawing && styles.previewDrawing, failed && styles.previewFailed)}>
                {url && <img src={url} alt={label} {...stylex.props(styles.image, tileDrawing && styles.imageDimmed)} />}
                {tileDrawing && <><DevelopEffect state="drawing" baseUrl={url} compact /><span role="status" {...stylex.props(styles.tileState)}><Orb of="artist" size={18} live /><Shimmer>{status === 'running' ? copy.drawing : copy.queued}</Shimmer></span></>}
                {waiting && !tileDrawing && <span role="status" {...stylex.props(styles.tileState, styles.tileWait)}>{copy.queued}</span>}
                {failed && <span role="alert" {...stylex.props(styles.tileState, styles.tileError)}><Icon name="danger" size={18} /><span>{failedStep ? progressText(failedStep) : copy.failed}</span></span>}
                <span {...stylex.props(styles.badge)}><span {...stylex.props(styles.badgeKey)}>{index + 1}</span>{label}</span>
                {item.status === 'approved' && <span {...stylex.props(styles.approved)}><Icon name="tick-b" size={18} label={copy.approved} /></span>}
                {url && !tileDrawing && !waiting && !failed && (
                  <Button variant="icon" size="sm" aria-label={copy.redraw(label)} tooltip="Vẽ lại (R)" xstyle={styles.tileAction} onClick={(event) => { event.stopPropagation(); openRedraw(item.id) }}><Icon name="gallery-edit" size={15} /></Button>
                )}
              </div>
              <div {...stylex.props(styles.captionRow)}>
                <span {...stylex.props(styles.caption)} title={item.narration || label}>{item.narration || label}</span>
                {timing && <span {...stylex.props(styles.time)}>{timing}</span>}
              </div>
            </div>
          })}</div>)}
      {stage === 'issue' && <div role="alert" {...stylex.props(styles.issueLine)}><Icon name="danger" size={15} /><span>{issueReason(job)}</span>{failedItem && <span {...stylex.props(styles.issueHint)}>Ảnh đã xong vẫn được giữ; xử lý xong thì Hoạ sĩ vẽ tiếp.</span>}</div>}

      {stage === 'render' && <RenderSteps job={job} score={score.data} scoreLoading={score.initialLoading} />}
      {stage === 'quality' && <RenderSteps job={job} score={score.data} scoreLoading={score.initialLoading} />}
      {stage === 'done' && <DoneStage job={job} video={video} score={score.data} items={items} />}

      {error && <p role="alert" {...stylex.props(styles.notice)}>{error}</p>}
      {activeStep?.error && stage !== 'issue' && <p role="alert" {...stylex.props(styles.notice)}>{progressText(activeStep)}</p>}
    </div>

    {dialog && dialog.jobId === job.id && (dialog.imageId === 'all' || target) && <Dialog title={target ? copy.redrawConfirm(imageLabel(target, targetIndex)) : `Vẽ lại cả ${items.length} ảnh?`} onClose={closeRedraw} width={540} footer={<>
      <Button size="sm" onClick={closeRedraw}>Huỷ</Button>
      <Button variant="primary" size="sm" disabled={busy || !images.data?.manifest || drawing} onClick={() => void submit()}>{target ? copy.redraw(imageLabel(target, targetIndex)) : `Vẽ lại ${items.length} ảnh`}</Button>
    </>}>
      <p>{job.source.title}</p>
      <p>{target ? copy.oneImageCost : `Dùng ${items.length} ảnh trong hạn mức hôm nay`}</p>
      {images.data?.manifest && <p>Ảnh cảnh vẽ theo <b>{images.data.manifest.scenePrompt.name}</b>, ảnh bìa theo <b>{images.data.manifest.coverPrompt.name}</b>, theo chỉ dẫn của video này. <Button variant="quiet" size="sm" onClick={() => { closeRedraw(); onChangePrompt() }}>Đổi ở Bàn dựng</Button></p>}
      <p>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
      <TextArea label={target ? copy.feedbackOne : 'Góp ý chung cho cả bộ (tuỳ chọn)'} value={feedback} onChange={event => setFeedback(event.currentTarget.value)} showCount={false} maxLength={-1} />
      {error && <p role="alert">{error}</p>}
    </Dialog>}
  </section>
}


/** Dải trạng thái đầu thẻ Phòng vẽ: tên + câu thật theo ảnh/bước đang chạy. */
function stageMeta(stage: JobStage, input: {
  total: number
  drawn: number
  manifest: { scenePrompt: { name: string } } | null | undefined
  job: CloneJobView
  drawingIndex: number
  runStep: JobStep | undefined
}) {
  const { total, drawn, manifest, job, drawingIndex, runStep } = input
  const drawLine = total ? `Hoạ sĩ · xong ${drawn} / ${total} ảnh` : 'Hoạ sĩ · chưa có ảnh'
  const scene = drawingIndex >= 0 ? `cảnh ${drawingIndex + 1}` : 'cảnh tiếp theo'
  switch (stage) {
    case 'review': return { title: 'Bộ ảnh chờ duyệt', meta: total ? `${total} ảnh${manifest ? ` · prompt ${manifest.scenePrompt.name}` : ''}` : drawLine, runLabel: '', gradient: 'var(--vv-st-review)' }
    case 'render': return { title: 'Dựng & xuất', meta: `Kiểm định · bước ${Math.max(1, ['build', 'score', 'export'].indexOf(job.activeStep ?? '') + 1)} / 3`, runLabel: runStep ? (STEP_LABEL[runStep.step] ?? runStep.step) : 'Đang dựng', gradient: 'linear-gradient(90deg,#A6EDC1,#5DC1B0 60%,#3E73D6)' }
    case 'quality': return { title: 'Kiểm định dừng trước khi xuất', meta: `Kiểm định · chưa đạt (cần ≥ ${QUALITY_MIN_SCORE})`, runLabel: '', gradient: 'var(--vv-danger)' }
    case 'issue': return { title: 'Phòng vẽ', meta: `Hoạ sĩ · dừng ở ${scene}, xong ${drawn} / ${total || 0} ảnh`, runLabel: '', gradient: 'var(--vv-danger)' }
    case 'paused': return { title: 'Phòng vẽ', meta: `Hoạ sĩ · tạm dừng trước ${scene}, xong ${drawn} / ${total || 0} ảnh`, runLabel: '', gradient: 'var(--vv-text-hint)' }
    case 'done': return { title: 'Thành phẩm', meta: 'MP4 · ảnh bìa · tiêu đề, mô tả, tag', runLabel: '', gradient: 'var(--vv-st-ready)' }
    default: return { title: 'Phòng vẽ trực tiếp', meta: drawLine, runLabel: drawingIndex >= 0 ? `Đang vẽ cảnh ${drawingIndex + 1}` : 'Đang làm', gradient: 'var(--vv-grad-artist-progress)' }
  }
}

/** Các bước thật của bước dựng/xuất: trạng thái, thời lượng, tiến độ đơn vị và điểm số. */
function RenderSteps({ job, score, scoreLoading }: { job: CloneJobView; score: { total: number; blockers: { id: string; message: string }[] | null } | null | undefined; scoreLoading: boolean }) {
  const order = ['build', 'score', 'export']
  const steps = order.map(key => job.steps?.find(step => step.step === key)).filter((step): step is JobStep => !!step)
  const exportWaiting = jobStage(job) === 'quality' && !steps.some(step => step.step === 'export')
  if (!steps.length) return <p {...stylex.props(styles.notice)}>{job.subline}</p>
  return <ol {...stylex.props(styles.steps)}>{steps.map(step => {
    const percent = stepPercent(step)
    const running = step.status === 'running'
    const failed = step.status === 'failed'
    const done = step.status === 'succeeded' || step.status === 'done'
    const wait = step.status === 'pending' || step.status === 'queued'
    const summary = progressText(step)
    return <li key={step.step} {...stylex.props(styles.stepCard, running && styles.stepRunning, failed && styles.stepFailed)}>
      <div {...stylex.props(styles.stepHead)}>
        {running ? <span aria-label="Đang chạy" {...stylex.props(styles.spinner)} /> : <Icon name={failed ? 'danger' : wait ? 'clock' : 'tick-b'} size={15} label={progressState(step.status) || undefined} xstyle={failed ? styles.warnIcon : done ? styles.okIcon : styles.waitIcon} />}
        <b {...stylex.props(styles.stepLabel)}>{STEP_LABEL[step.step] ?? step.step}</b>
        <span {...stylex.props(styles.stepSummary)} title={summary}>{summary}</span>
        <span {...stylex.props(styles.stepDuration)}>{stepDuration(step)}</span>
      </div>
      {percent !== null && <div {...stylex.props(styles.unitRow)}><div role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-label={`Tiến độ ${STEP_LABEL[step.step] ?? step.step}`} {...stylex.props(styles.unitTrack)}><div {...stylex.props(styles.unitFill)} style={{ width: `${percent}%` }} /></div><span {...stylex.props(styles.unitCaption)}>{step.progress.unitsTotal ? `${step.progress.unitsDone.toLocaleString('vi-VN')} / ${step.progress.unitsTotal.toLocaleString('vi-VN')}` : `${percent}%`}{running && percent < 100 ? ' · đang chạy' : ''}</span></div>}
      {step.step === 'score' && <div {...stylex.props(styles.kvRow)}>
        {scoreLoading ? <span {...stylex.props(styles.kv)}>Đang tải điểm…</span> : score ? <>
          <span {...stylex.props(styles.kv)}>Độ hoàn thiện <b>{score.total} / 100</b></span>
          {score.total < QUALITY_MIN_SCORE && <span {...stylex.props(styles.kv, styles.kvWarn)}>Cần ≥ {QUALITY_MIN_SCORE} để tự xuất</span>}
          {score.blockers?.map(item => <span key={item.id} {...stylex.props(styles.kv, styles.kvWarn)} title={item.message}>{item.message}</span>)}
        </> : <span {...stylex.props(styles.kv)}>Chưa có kết quả Kiểm định</span>}
      </div>}
    </li>
  })}{exportWaiting && <li {...stylex.props(styles.stepCard)}>
    <div {...stylex.props(styles.stepHead)}><Icon name="clock" size={15} label="Chờ bạn" xstyle={styles.waitIcon} /><b {...stylex.props(styles.stepLabel)}>Xuất MP4</b><span {...stylex.props(styles.stepSummary)}>Chờ bạn sửa hoặc chọn Vẫn xuất</span></div>
  </li>}</ol>
}

/** Thành phẩm: file thật + điểm + ảnh bìa; nút tải/đánh dấu nằm ở header. */
function DoneStage({ job, video, score, items }: { job: CloneJobView; video: FinishedVideo | null; score: { total: number } | null | undefined; items: JobImage[] }) {
  const cover = items.find(item => item.kind === 'thumbnail' || item.kind === 'cover')
  const seconds = job.source.durationSec
  const size = job.exportBytes
  return <div {...stylex.props(styles.done)}>
    <div {...stylex.props(styles.playerBox)}>
      {video ? <video controls {...stylex.props(styles.video)} src={video.url} /> : <span {...stylex.props(styles.muted)}>File video không còn trên máy.</span>}
      <div {...stylex.props(styles.playerBar)}>
        <span {...stylex.props(styles.num)}>{seconds != null ? clockLabel(seconds) : 'MP4'}</span>
        <span {...stylex.props(styles.playerLine)} />
        <span>{[size != null ? `${(size / 1024 / 1024).toFixed(1)} MB` : null, video ? 'MP4' : null].filter(Boolean).join(' · ')}</span>
      </div>
    </div>
    <div {...stylex.props(styles.facts)}>
      {score && <div {...stylex.props(styles.fact)}><Icon name="tick-b" size={15} xstyle={styles.okIcon} /><span {...stylex.props(styles.factBody)}><b>Độ hoàn thiện {score.total} / 100</b><small>Kiểm định đã đo trên bản xuất</small></span></div>}
      {video && <div {...stylex.props(styles.fact)}><Icon name="video-play" size={15} /><span {...stylex.props(styles.factBody)}><b>MP4{seconds != null ? ` ${clockLabel(seconds)}` : ''}{size != null ? ` · ${(size / 1024 / 1024).toFixed(1)} MB` : ''}</b><small>File xuất của video</small></span></div>}
      {cover && <div {...stylex.props(styles.fact)}><Icon name="gallery" size={15} /><span {...stylex.props(styles.factBody)}><b>Ảnh bìa của video</b><small>Đi kèm gói đăng</small></span></div>}
      <div {...stylex.props(styles.fact)}><Icon name="flag" size={15} xstyle={styles.reviewIcon} /><span {...stylex.props(styles.factBody)}><b>{job.stageGroup === 'posted' ? 'Đã đánh dấu đã đăng' : 'Đăng xong thì đánh dấu đã đăng'}</b><small>Video chuyển sang Đã đăng; file MP4 có thể xoá để lấy chỗ</small></span></div>
    </div>
  </div>
}

const styles = stylex.create({
  root: { flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden', border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], background: 'var(--card-bg)', boxShadow: 'var(--card-shadow)' },
  progressTrack: { flex: '0 0 2px', height: 2, backgroundColor: 'var(--line-1)', overflow: 'hidden' },
  progressFill: { height: '100%', transition: 'width .3s ease' },
  body: { flex: 1, minHeight: 0, padding: `${s['--vv-s-10']} ${s['--vv-s-14']} ${s['--vv-s-12']} ${s['--vv-s-16']}`, display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'], overflowY: 'auto' },
  header: { flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: `${s['--vv-s-6']} ${s['--vv-s-10']}`, minHeight: 20 },
  orb: { display: 'flex', flexShrink: 0 },
  streamTitle: { margin: 0, fontSize: 13, fontWeight: 500, color: 'var(--color-fg)' },
  meta: { fontSize: 12, color: 'var(--vv-fg-note)' },
  spacer: { flex: 1 },
  run: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], fontSize: 12 },
  elapsed: { display: 'inline-block', minWidth: '5ch', textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: 'var(--vv-fg-note)' },
  keyHint: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], fontSize: 12, color: 'var(--vv-fg-note)' },
  kbd: { fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: 'var(--color-fg-3)', paddingBlock: 1, paddingInline: 5, borderRadius: r['--vv-r-4'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-0)' },
  grid: { flex: '0 1 auto', minHeight: 0, display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gridAutoRows: 'min-content', gap: `${s['--vv-s-12']} ${s['--vv-s-10']}`, paddingInlineEnd: s['--vv-s-4'] },
  tile: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], color: 'var(--color-fg-3)', fontSize: 12, cursor: 'pointer', borderRadius: r['--vv-r-10'], ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 } },
  coverTile: { gridColumn: 'span 2' },
  tileSelected: { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 },
  preview: { position: 'relative', aspectRatio: '16 / 9', overflow: 'hidden', border: '1px solid var(--line-1)', borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-0)', display: 'grid', placeItems: 'center' },
  previewDrawing: { backgroundImage: 'var(--vv-draw-tile-bg)', borderColor: 'var(--vv-draw-tile-line)' },
  previewFailed: { borderColor: 'var(--vv-artist-line)', backgroundColor: 'var(--vv-danger-8)' },
  image: { width: '100%', height: '100%', objectFit: 'cover' },
  imageDimmed: { opacity: .45 },
  tileState: { position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-8'], padding: s['--vv-s-10'], textAlign: 'center', fontSize: 12 },
  tileWait: { background: 'repeating-linear-gradient(135deg, #131315 0 10px, #161619 10px 20px)', color: 'var(--vv-fg-note)' },
  tileError: { color: 'var(--vv-warn-ink)' },
  badge: { position: 'absolute', left: s['--vv-s-8'], top: s['--vv-s-8'], height: 20, paddingInline: s['--vv-s-7'], display: 'flex', alignItems: 'center', gap: s['--vv-s-5'], borderRadius: r['--vv-r-6'], backgroundColor: 'var(--vv-img-badge)', fontSize: 11.5, color: 'var(--color-fg)' },
  badgeKey: { fontVariantNumeric: 'tabular-nums', color: 'var(--color-fg-3)' },
  approved: { position: 'absolute', right: s['--vv-s-8'], top: s['--vv-s-8'], color: 'var(--color-ok)' },
  tileAction: { position: 'absolute', right: s['--vv-s-8'], bottom: s['--vv-s-8'], backgroundColor: 'var(--vv-img-badge)' },
  captionRow: { display: 'flex', justifyContent: 'space-between', gap: s['--vv-s-8'], paddingInline: s['--vv-s-2'] },
  caption: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--vv-fg-note)' },
  time: { flexShrink: 0, fontVariantNumeric: 'tabular-nums' },
  notice: { margin: 0, padding: s['--vv-s-10'], borderRadius: r['--vv-r-10'], border: '1px solid var(--line-2)', color: 'var(--vv-fg-note)', fontSize: 13 },
  issueLine: { display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: s['--vv-s-8'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-12'], border: '1px solid var(--vv-danger-border)', backgroundColor: 'var(--vv-danger-8)', color: 'var(--vv-warn-ink)', fontSize: 13 },
  issueHint: { color: 'var(--color-fg-3)' },
  steps: { listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  stepCard: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-12'], backgroundColor: 'var(--color-surface-0)', border: '1px solid var(--line-1)' },
  stepRunning: { borderColor: 'var(--vv-st-ready-28)', backgroundColor: 'var(--vv-st-ready-bg)' },
  stepFailed: { borderColor: 'var(--vv-danger-border)', backgroundColor: 'var(--vv-danger-8)' },
  stepHead: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'] },
  spinner: { width: 14, height: 14, flexShrink: 0, borderRadius: '50%', border: '2px solid var(--line-3)', borderTopColor: 'var(--vv-st-ready)', animationName: { default: 'ai-spin', '@media (prefers-reduced-motion: reduce)': 'none' }, animationDuration: '.9s', animationIterationCount: 'infinite' },
  okIcon: { color: 'var(--color-ok)' },
  warnIcon: { color: 'var(--vv-danger)' },
  waitIcon: { color: 'var(--vv-fg-note)' },
  stepLabel: { fontSize: 13, fontWeight: 500 },
  stepSummary: { flex: 1, minWidth: 0, fontSize: 12.5, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  stepDuration: { fontSize: 12, color: 'var(--vv-fg-note)', fontStyle: 'italic', fontVariantNumeric: 'tabular-nums' },
  unitRow: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], paddingInlineStart: s['--vv-s-24'] },
  unitTrack: { height: 6, borderRadius: r['--vv-r-6'], backgroundColor: 'var(--color-surface-3)', overflow: 'hidden' },
  unitFill: { height: '100%', borderRadius: r['--vv-r-6'], background: 'linear-gradient(90deg,#A6EDC1,#5DC1B0)' },
  unitCaption: { display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  kvRow: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'], paddingInlineStart: s['--vv-s-24'] },
  kv: { padding: `${s['--vv-s-3']} ${s['--vv-s-8']}`, borderRadius: r['--vv-r-6'], backgroundColor: 'var(--color-surface-0)', border: '1px solid var(--line-1)', fontSize: 12, color: 'var(--vv-fg-note)' },
  kvWarn: { color: 'var(--vv-warn-ink)', borderColor: 'var(--vv-warn-bd)' },
  muted: { margin: 0, color: 'var(--vv-fg-note)', fontSize: 12.5, lineHeight: 1.55 },
  done: { flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'minmax(0, 1.6fr) minmax(0, 1fr)', gap: s['--vv-s-14'] },
  playerBox: { position: 'relative', minHeight: 160, borderRadius: r['--vv-r-12'], overflow: 'hidden', border: '1px solid var(--line-1)', backgroundColor: 'var(--color-surface-0)', display: 'flex', alignItems: 'center', justifyContent: 'center' },
  video: { display: 'block', width: '100%', maxHeight: 360, backgroundColor: 'var(--color-surface-2)' },
  playerBar: { position: 'absolute', left: 0, right: 0, bottom: 0, padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], background: 'linear-gradient(0deg, rgba(11,11,13,.85), transparent)', fontSize: 12, color: 'var(--color-fg-2)' },
  playerLine: { flex: 1, height: 3, borderRadius: r['--vv-r-3'], backgroundColor: 'var(--line-3)' },
  num: { fontVariantNumeric: 'tabular-nums', fontFamily: "'JetBrains Mono', monospace" },
  facts: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], minWidth: 0 },
  fact: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-9']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-0)', border: '1px solid var(--line-1)', color: 'var(--color-fg-3)' },
  factBody: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, fontSize: 12.5, lineHeight: 1.45 },
  reviewIcon: { color: 'var(--vv-st-review)' },
})
