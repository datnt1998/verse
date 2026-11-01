import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon, ProgressBar, Shimmer, TextArea } from '@noverse/ui'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { acceptExport, getJobImages, getScoreArtifact, jobBulkAction, redrawAll, redrawImage, type CloneJobView } from '../../api/workbench'
import { useResource } from '../../data/useResource'
import { subscribeEvents } from '../../data/events'
import { requestMarkPosted } from '../../components/performance'
import { DevelopEffect, Orb } from '../../components'
import { screenWorkbench } from '../../i18n/screens'
import { editorProjectId } from './workbench-actions'
import { imageTime } from './workbench-logic'

export function LiveStudio({ job, onChangePrompt, onRefresh }: { job: CloneJobView; onChangePrompt: () => void; onRefresh: () => void }) {
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: !['ready', 'posted'].includes(job.stageGroup) })
  const scoreArtifact = job.steps?.find(step => step.step === 'score')?.output?.artifacts?.find(item => item.path.endsWith('score.json'))
  const video = job.steps?.find(step => step.step === 'export')?.output?.artifacts?.find(item => item.contentType === 'video/mp4')
  const projectId = editorProjectId(job)
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: !!scoreArtifact && job.activeStep === 'score' })
  const [dialog, setDialog] = useState<string | null>(null)
  const [feedback, setFeedback] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated'], () => { void images.refresh() }), [images.refresh])
  const items = images.data?.items || []
  const copy = screenWorkbench.live
  const done = items.filter(item => item.status === 'review' || item.status === 'approved').length
  const isDrawing = (status: string) => status === 'running' || status === 'pending' || status === 'queued'
  const drawing = items.some(item => isDrawing(item.status))
  const imageLabel = (index: number) => items[index]?.kind === 'thumbnail' ? copy.cover : copy.scene(index + 1)
  const targetIndex = items.findIndex(item => item.id === dialog)
  const target = targetIndex >= 0 ? items[targetIndex] : undefined
  const submit = async () => {
    if (!images.data?.manifest) return
    setBusy(true); setError('')
    try {
      if (target) await redrawImage(job.id, target.id, images.data.manifest.revision, feedback)
      else await redrawAll(job.id, images.data.manifest.revision, items.length, feedback)
      setDialog(null); setFeedback(''); void images.refresh(); onRefresh()
    } catch (err) { setError(err instanceof Error ? err.message : 'Không vẽ lại được') } finally { setBusy(false) }
  }
  if (job.stageGroup === 'ready' || job.stageGroup === 'posted') return <section {...stylex.props(styles.root)} aria-label="Trực tiếp">
    <h2 {...stylex.props(styles.title)}>Video đã dựng xong</h2>
    {video ? <video controls {...stylex.props(styles.video)} src={`/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(video.id)}`} /> : <p>File video không còn trên máy.</p>}
    <div {...stylex.props(styles.actions)}><a {...stylex.props(styles.button)} href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`}>Tải gói đăng</a>{job.stageGroup === 'ready' && <Button size="sm" onClick={() => { void requestMarkPosted(job, onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không đánh dấu được')) }}>Đánh dấu đã đăng</Button>}{video && job.stageGroup === 'posted' && <Button size="sm" onClick={() => { if (window.confirm('Xoá file MP4 đã xuất?')) void jobBulkAction(job.id, 'deleteExport').then(onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không xoá được')) }}>Xoá MP4</Button>}</div>{error && <p role="alert">{error}</p>}
  </section>
  if (job.activeStep === 'score' && job.stageGroup === 'attention') return <section {...stylex.props(styles.root)} aria-label="Trực tiếp"><h2>Video chưa đạt Độ hoàn thiện</h2>{score.initialLoading ? <p>Đang tải kết quả…</p> : score.error ? <p role="alert">Không tải được kết quả kiểm định.</p> : <><p>{score.data?.total} điểm</p><ul>{score.data?.blockers?.map(item => <li key={item.id}>{item.message}</li>)}</ul></>}<div {...stylex.props(styles.actions)}>{projectId ? <a {...stylex.props(styles.button)} href={`#/editor/${encodeURIComponent(projectId)}`}>Sửa trong editor</a> : <Button size="sm" disabled>Project chưa sẵn sàng</Button>}<Button size="sm" onClick={() => { void acceptExport(job.id).then(onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không xuất được')) }}>Vẫn xuất</Button></div>{error && <p role="alert">{error}</p>}</section>
  if (job.stageGroup === 'working' && (job.activeStep === 'build' || job.activeStep === 'export')) return <section {...stylex.props(styles.root)} aria-label="Trực tiếp"><h2>Đang dựng video</h2><p>{job.subline}</p>{job.steps?.filter(step => step.status === 'running').map(step => <p key={step.step}>{step.progress.phase} {step.progress.unitsTotal ? `${step.progress.unitsDone}/${step.progress.unitsTotal}` : ''}</p>)}</section>
  return <section {...stylex.props(styles.root)} aria-label="Trực tiếp">
    <div {...stylex.props(styles.progress)}><ProgressBar value={items.length ? done / items.length * 100 : 0} indeterminate={!items.length && images.initialLoading} tone="writer" height={2} label={copy.title} /></div>
    <div {...stylex.props(styles.header)}>
      <Orb of="artist" size={16} live={drawing} />
      <h2 {...stylex.props(styles.liveTitle)}>{copy.title}</h2>
      {items.length > 0 && <span {...stylex.props(styles.meta)}>{copy.progress(done, items.length)}</span>}
      <span {...stylex.props(styles.headerSpacer)} />
      {items.length > 0 && <Button size="sm" disabled={drawing} onClick={() => { setError(''); setFeedback(''); setDialog('all') }}>Vẽ lại tất cả</Button>}
    </div>
    {images.error ? <p role="alert">Không tải được bộ ảnh. <Button variant="quiet" size="sm" onClick={() => void images.refresh()}>Thử lại</Button></p> : <div {...stylex.props(styles.grid)}>{items.map((item, index) => {
      const image = item.versions?.find(v => v.version === (item.reviewVersion ?? item.currentVersion))?.originalUrl
      const drawing = isDrawing(item.status)
      const label = imageLabel(index)
      const timing = imageTime(item)
      return <div key={item.id} {...stylex.props(styles.tile)}>
        <div {...stylex.props(styles.preview, drawing && styles.previewDrawing)}>
          {drawing ? <><DevelopEffect state="drawing" baseUrl={image} compact /><span role="status" {...stylex.props(styles.drawingStatus)}><Orb of="artist" size={18} live /><Shimmer>{item.status === 'running' ? copy.drawing : copy.queued}</Shimmer></span></>
            : image ? <img src={image} alt={label} {...stylex.props(styles.image)} />
              : <span role={item.status === 'failed' ? 'alert' : 'status'}>{item.status === 'failed' ? copy.failed : copy.queued}</span>}
          <span {...stylex.props(styles.tileBadge)}><span {...stylex.props(styles.tileKey)}>{index + 1}</span>{label}</span>
          {item.status === 'approved' && <span {...stylex.props(styles.approved)}><Icon name="tick-circle" size={18} label={copy.approved} /></span>}
          {image && !drawing && <Button variant="ghost" size="sm" aria-label={copy.redraw(label)} xstyle={styles.tileAction} onClick={() => { setError(''); setFeedback(''); setDialog(item.id) }}><Icon name="gallery-edit" size={15} /></Button>}
        </div>
        <div {...stylex.props(styles.captionRow)}><span {...stylex.props(styles.caption)} title={item.narration || label}>{item.narration || label}</span>{timing && <span {...stylex.props(styles.time)}>{timing}</span>}</div>
      </div>
    })}</div>}
    {dialog && <Dialog title={target ? copy.redrawConfirm(imageLabel(targetIndex)) : `Vẽ lại cả ${items.length} ảnh?`} onClose={() => setDialog(null)} width={540} footer={<>
      <Button size="sm" onClick={() => setDialog(null)}>Huỷ</Button>
      <Button variant="primary" size="sm" disabled={busy || !images.data?.manifest || drawing} onClick={() => void submit()}>{target ? copy.redraw(imageLabel(targetIndex)) : `Vẽ lại ${items.length} ảnh`}</Button>
    </>}>
      <p>{job.source.title}</p>
      <p>{target ? copy.oneImageCost : `Dùng ${items.length} ảnh trong hạn mức hôm nay`}</p>
      {images.data?.manifest && <p>Ảnh cảnh vẽ theo <b>{images.data.manifest.scenePrompt.name}</b>, ảnh bìa theo <b>{images.data.manifest.coverPrompt.name}</b>, theo prompt của video này. <Button variant="quiet" size="sm" onClick={() => { setDialog(null); onChangePrompt() }}>Đổi ở Bàn dựng</Button></p>}
      <p>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
      <TextArea label={target ? copy.feedbackOne : 'Góp ý chung cho cả bộ (tuỳ chọn)'} value={feedback} onChange={event => setFeedback(event.currentTarget.value)} showCount={false} maxLength={-1} />
      {error && <p role="alert">{error}</p>}
    </Dialog>}
  </section>
}
const styles = stylex.create({
  root: { flex: 1, minHeight: 0, overflowY: 'auto', padding: `${s['--vv-s-10']} ${s['--vv-s-14']}`, border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], backgroundColor: 'var(--color-surface-1)' },
  progress: { margin: `calc(-1 * ${s['--vv-s-10']}) calc(-1 * ${s['--vv-s-14']}) 0`, overflow: 'hidden', '--grad-writer-progress': 'var(--vv-grad-artist-progress)' },
  header: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: `${s['--vv-s-6']} ${s['--vv-s-10']}`, marginBottom: s['--vv-s-10'], marginTop: s['--vv-s-10'] },
  headerSpacer: { flex: 1 },
  liveTitle: { margin: 0, fontSize: 13, fontWeight: 500, color: 'var(--color-fg)' },
  meta: { fontSize: 12, color: 'var(--color-fg-3)' },
  title: { margin: `0 0 ${s['--vv-s-8']}`, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' },
  video: { display: 'block', width: '100%', maxHeight: 360, backgroundColor: 'var(--color-surface-2)', borderRadius: r['--vv-r-10'] },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: `${s['--vv-s-12']} ${s['--vv-s-10']}` },
  tile: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], color: 'var(--color-fg-3)', fontSize: 12 },
  preview: { position: 'relative', aspectRatio: '16 / 9', overflow: 'hidden', border: '1px solid var(--line-1)', borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-2)', display: 'grid', placeItems: 'center' },
  previewDrawing: { backgroundImage: 'var(--vv-draw-tile-bg)', borderColor: 'var(--vv-draw-tile-line)' },
  drawingStatus: { position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-8'], fontSize: 12 },
  image: { width: '100%', height: '100%', objectFit: 'cover' },
  tileBadge: { position: 'absolute', left: s['--vv-s-8'], top: s['--vv-s-8'], height: 20, padding: `0 ${s['--vv-s-6']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-4'], borderRadius: r['--vv-r-6'], backgroundColor: 'var(--vv-glass)', fontSize: 11.5, color: 'var(--color-fg)' },
  tileKey: { fontVariantNumeric: 'tabular-nums', color: 'var(--color-fg-3)' },
  approved: { position: 'absolute', right: s['--vv-s-8'], top: s['--vv-s-8'], color: 'var(--color-ok)' },
  tileAction: { position: 'absolute', right: s['--vv-s-8'], bottom: s['--vv-s-8'] },
  caption: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', padding: `0 ${s['--vv-s-2']}` },
  captionRow: { display: 'flex', justifyContent: 'space-between', gap: s['--vv-s-8'] },
  time: { flexShrink: 0, fontVariantNumeric: 'tabular-nums' },
  button: { padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, border: '1px solid var(--line-2)', borderRadius: r['--vv-r-8'], color: 'var(--color-fg)', backgroundColor: 'var(--color-surface-2)', cursor: 'pointer' },
  actions: { display: 'flex', justifyContent: 'flex-end', gap: s['--vv-s-8'], marginTop: s['--vv-s-14'] },
})
