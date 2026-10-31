import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { acceptExport, getJobImages, getScoreArtifact, jobBulkAction, redrawAll, type CloneJobView } from '../../api/workbench'
import { useResource } from '../../data/useResource'
import { subscribeEvents } from '../../data/events'

export function LiveStudio({ job, onChangePrompt, onRefresh }: { job: CloneJobView; onChangePrompt: () => void; onRefresh: () => void }) {
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: !['ready', 'posted'].includes(job.stageGroup) })
  const scoreArtifact = job.steps?.find(step => step.step === 'score')?.output?.artifacts?.find(item => item.path.endsWith('score.json'))
  const video = job.steps?.find(step => step.step === 'export')?.output?.artifacts?.find(item => item.contentType === 'video/mp4')
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: !!scoreArtifact && job.activeStep === 'score' })
  const [dialog, setDialog] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated'], () => { void images.refresh() }), [images.refresh])
  useEffect(() => {
    if (!dialog) return
    const close = (event: KeyboardEvent) => { if (event.key === 'Escape') { event.stopPropagation(); setDialog(false) } }
    document.addEventListener('keydown', close, true)
    return () => document.removeEventListener('keydown', close, true)
  }, [dialog])
  const items = images.data?.items || []
  const submit = async () => {
    if (!images.data?.manifest) return
    setBusy(true); setError('')
    try { await redrawAll(job.id, images.data.manifest.revision, items.length, feedback); setDialog(false); void images.refresh() } catch (err) { setError(err instanceof Error ? err.message : 'Không vẽ lại được') } finally { setBusy(false) }
  }
  if (job.stageGroup === 'ready' || job.stageGroup === 'posted') return <section {...stylex.props(styles.root)} aria-label="Trực tiếp">
    <h2 {...stylex.props(styles.title)}>Video đã dựng xong</h2>
    {video ? <video controls {...stylex.props(styles.video)} src={`/api/clone/${encodeURIComponent(job.id)}/artifacts/${encodeURIComponent(video.id)}`} /> : <p>File video không còn trên máy.</p>}
    <div {...stylex.props(styles.actions)}><a {...stylex.props(styles.button)} href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`}>Tải gói đăng</a>{job.stageGroup === 'ready' && <button {...stylex.props(styles.button)} onClick={() => { void jobBulkAction(job.id, 'posted').then(onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không đánh dấu được')) }}>Đánh dấu đã đăng</button>}{video && <button {...stylex.props(styles.button)} onClick={() => { if (window.confirm('Xoá file MP4 đã xuất?')) void jobBulkAction(job.id, 'deleteExport').then(onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không xoá được')) }}>Xoá MP4</button>}</div>{error && <p role="alert">{error}</p>}
  </section>
  if (job.activeStep === 'score' && job.stageGroup === 'attention') return <section {...stylex.props(styles.root)} aria-label="Trực tiếp"><h2>Video chưa đạt Độ hoàn thiện</h2>{score.initialLoading ? <p>Đang tải kết quả…</p> : score.error ? <p role="alert">Không tải được kết quả kiểm định.</p> : <><p>{score.data?.total} điểm</p><ul>{score.data?.blockers?.map(item => <li key={item.id}>{item.message}</li>)}</ul></>}<div {...stylex.props(styles.actions)}><a {...stylex.props(styles.button)} href={`#/editor/${encodeURIComponent(job.id)}`}>Sửa trong editor</a><button {...stylex.props(styles.button)} onClick={() => { void acceptExport(job.id).then(onRefresh).catch(err => setError(err instanceof Error ? err.message : 'Không xuất được')) }}>Vẫn xuất</button></div>{error && <p role="alert">{error}</p>}</section>
  if (job.stageGroup === 'working' && (job.activeStep === 'build' || job.activeStep === 'export')) return <section {...stylex.props(styles.root)} aria-label="Trực tiếp"><h2>Đang dựng video</h2><p>{job.subline}</p>{job.steps?.filter(step => step.status === 'running').map(step => <p key={step.step}>{step.progress.phase} {step.progress.unitsTotal ? `${step.progress.unitsDone}/${step.progress.unitsTotal}` : ''}</p>)}</section>
  return <section {...stylex.props(styles.root)} aria-label="Trực tiếp">
    <div {...stylex.props(styles.header)}><h2 {...stylex.props(styles.title)}>{job.agentView?.find(a => a.state === 'running')?.now || job.subline}</h2>{items.length > 0 && <button type="button" {...stylex.props(styles.button)} onClick={() => setDialog(true)}>Vẽ lại tất cả</button>}</div>
    {images.error ? <p role="alert">Không tải được bộ ảnh. <button onClick={() => void images.refresh()}>Thử lại</button></p> : <div {...stylex.props(styles.grid)}>{items.map((item, index) => {
      const image = item.versions?.find(v => v.version === (item.reviewVersion ?? item.currentVersion))?.originalUrl
      return <div key={item.id} {...stylex.props(styles.tile)}><div {...stylex.props(styles.preview)}>{image ? <img src={image} alt={item.kind === 'thumbnail' ? 'Ảnh bìa' : `Cảnh ${index + 1}`} {...stylex.props(styles.image)} /> : <span>{item.status === 'running' ? 'Hoạ sĩ đang vẽ…' : 'Chờ lượt vẽ'}</span>}</div><span>{item.kind === 'thumbnail' ? 'Ảnh bìa' : `Cảnh ${index + 1}`}</span></div>
    })}</div>}
    {dialog && <div {...stylex.props(styles.overlay)}><section role="dialog" aria-modal="true" aria-labelledby="redraw-title" {...stylex.props(styles.dialog)}><h2 id="redraw-title">Vẽ lại cả {items.length} ảnh?</h2><p>{job.source.title}</p><p>Dùng {items.length} ảnh trong hạn mức hôm nay</p>{images.data?.manifest && <p>Ảnh cảnh vẽ theo <b>{images.data.manifest.scenePrompt.name}</b>, ảnh bìa theo <b>{images.data.manifest.coverPrompt.name}</b>, theo prompt của video này. <button type="button" onClick={() => { setDialog(false); onChangePrompt() }}>Đổi ở Bàn dựng</button></p>}<p>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p><label>Góp ý chung cho cả bộ (tuỳ chọn)<textarea value={feedback} onChange={event => setFeedback(event.target.value)} {...stylex.props(styles.textarea)} /></label>{error && <p role="alert">{error}</p>}<div {...stylex.props(styles.actions)}><button type="button" {...stylex.props(styles.button)} onClick={() => setDialog(false)}>Huỷ</button><button type="button" {...stylex.props(styles.button)} disabled={busy || !images.data?.manifest} onClick={() => void submit()}>Vẽ lại {items.length} ảnh</button></div></section></div>}
  </section>
}
const styles = stylex.create({
  root: { flex: 1, minHeight: 0, overflowY: 'auto', padding: `${s['--vv-s-10']} ${s['--vv-s-14']}`, border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], backgroundColor: 'var(--color-surface-1)' },
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: s['--vv-s-8'] }, title: { margin: `0 0 ${s['--vv-s-8']}`, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' },
  video: { display: 'block', width: '100%', maxHeight: 360, backgroundColor: 'var(--color-surface-2)', borderRadius: r['--vv-r-10'] },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: s['--vv-s-10'] }, tile: { minWidth: 0, color: 'var(--color-fg-3)', fontSize: 12 }, preview: { aspectRatio: '16 / 9', overflow: 'hidden', borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-2)', display: 'grid', placeItems: 'center' }, image: { width: '100%', height: '100%', objectFit: 'cover' },
  button: { padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, border: '1px solid var(--line-2)', borderRadius: r['--vv-r-8'], color: 'var(--color-fg)', backgroundColor: 'var(--color-surface-2)', cursor: 'pointer' },
  overlay: { position: 'fixed', inset: 0, zIndex: 100, display: 'grid', placeItems: 'center', backgroundColor: 'var(--kit-scrim)' }, dialog: { width: 'min(540px, calc(100vw - 32px))', padding: s['--vv-s-20'], border: '1px solid var(--line-2)', borderRadius: r['--vv-r-16'], backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)' }, textarea: { display: 'block', width: '100%', minHeight: 80, marginTop: s['--vv-s-6'], backgroundColor: 'var(--color-surface-1)', color: 'var(--color-fg)', border: '1px solid var(--line-2)', borderRadius: r['--vv-r-8'] }, actions: { display: 'flex', justifyContent: 'flex-end', gap: s['--vv-s-8'], marginTop: s['--vv-s-14'] },
})
