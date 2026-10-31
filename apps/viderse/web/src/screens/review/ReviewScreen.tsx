import { useEffect, useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { listChannels } from '../../api/channels'
import { request } from '../../api/request'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { Orb, SystemLoading } from '../../components'
import { useOnline } from '../../data/status'
import { paths } from '../../router'
import { imageURL, useReview, type ReviewImage } from './useReview'
import { useReviewKeys } from './useReviewKeys'
import { sx } from './review.stylex'

const drawing = (item: ReviewImage) => item.status === 'running' || item.status === 'pending' || item.status === 'queued'
const blocked = (item: ReviewImage) => item.status === 'failed' || item.status === 'blocked' || item.status === 'skipped' && item.skipReason === 'refused'
const label = (item: ReviewImage, index: number) => item.kind === 'thumbnail' ? 'Ảnh bìa' : `Cảnh ${item.sceneId?.match(/\d+/)?.[0] || index + 1}`
const time = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`

export function ReviewScreen() {
  const review = useReview()
  const online = useOnline()
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const counts = useResource(['review', 'counts'], ({ signal }) => request<{ approvedToday: number | null }>('/api/clone/counts', { signal }))
  const [jobId, setJobId] = useState<string | null>(() => new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('video'))
  const [index, setIndex] = useState(0)
  const [view, setView] = useState<'full' | 'cutout'>('full')
  const [imageFailed, setImageFailed] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [allOpen, setAllOpen] = useState(false)
  const [allFeedback, setAllFeedback] = useState('')
  const channelFilter = new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('channel')
  const jobs = useMemo(() => (review.jobs ?? []).filter(j => !channelFilter || channelFilter === 'all' || j.channelId === channelFilter), [review.jobs, channelFilter])
  const job = jobs.find(j => j.id === jobId) ?? jobs[0]
  const images = job ? review.images[job.id] : undefined
  const items = images?.items ?? []
  const selected = Math.min(index, Math.max(0, items.length - 1))
  const item = items[selected]
  const channel = channels.data?.channels.find(c => c.id === job?.channelId)
  const neighbour = [...items.slice(0, selected)].reverse().find(candidate => candidate.kind === 'scene' && candidate.status !== 'skipped') ?? items.find((candidate, i) => i > selected && candidate.kind === 'scene' && candidate.status !== 'skipped')
  const busy = !!job && !!review.pending[job.id]
  const locked = busy || items.some(drawing)
  const reason = 'Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ'
  useEffect(() => { setImageFailed(false) }, [job?.id, item?.id, view])
  const selectJob = (id: string) => { setJobId(id); setIndex(0); setView('full'); setFeedback('') }
  useEffect(() => { if (job && !review.images[job.id] && !review.imageErrors[job.id]) void review.loadImages(job.id) }, [job, review.images, review.imageErrors, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    if (next && !review.images[next.id] && !review.imageErrors[next.id]) void review.loadImages(next.id)
  }, [job, jobs, review.images, review.imageErrors, review.loadImages])
  useEffect(() => {
    if (!job || !images || !items.some(drawing)) return
    const timer = window.setInterval(() => void review.loadImages(job.id), 2500)
    return () => clearInterval(timer)
  }, [job, images, items, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    for (const image of next ? review.images[next.id]?.items ?? [] : []) { const url = imageURL(next!.id, image); if (url) { const preload = new Image(); preload.src = url } }
  }, [job, jobs, review.images])
  const redraw = () => { if (job && item && !drawing(item) && !busy && online) { void review.mutate(job.id, item, 'regenerate', feedback).then(ok => { if (ok) setFeedback('') }) } }
  const approve = () => {
    if (!job || !images || !items.length || locked || !online) return
    const at = jobs.findIndex(j => j.id === job.id)
    const next = jobs[at + 1] ?? jobs[at - 1]
    if (next) selectJob(next.id)
    void review.mutate(job.id, null, 'approveAll').then(ok => { if (!ok) selectJob(job.id) })
  }
  const moveJob = (delta: number) => { if (!job) return; const next = jobs[Math.max(0, Math.min(jobs.length - 1, jobs.findIndex(j => j.id === job.id) + delta))]; if (next) selectJob(next.id) }
  const moveImage = (delta: number) => { if (items.length) { setIndex((selected + delta + items.length) % items.length); setView('full') } }
  useReviewKeys({ nextJob: () => moveJob(1), previousJob: () => moveJob(-1), nextImage: () => moveImage(1), previousImage: () => moveImage(-1), redraw, approve }, !allOpen)
  useEffect(() => { if (!allOpen) return; const close = (event: KeyboardEvent) => { if (event.key === 'Escape') setAllOpen(false) }; document.addEventListener('keydown', close); return () => document.removeEventListener('keydown', close) }, [allOpen])
  const approved = items.filter(i => i.status === 'approved').length
  return <div {...stylex.props(sx.root)}>
    <aside aria-label="Video chờ duyệt" {...stylex.props(sx.side)}>
      <div {...stylex.props(sx.group)}><span>Chờ bạn duyệt</span><span>{jobs.length}</span></div>
      {jobs.map(j => <button key={j.id} type="button" aria-pressed={j.id === job?.id} onClick={() => selectJob(j.id)} {...stylex.props(sx.row, j.id === job?.id && sx.selectedRow)}>{j.source.thumbnailUrl ? <img alt="" src={j.source.thumbnailUrl} {...stylex.props(sx.thumb)} /> : <span {...stylex.props(sx.thumb)} /> }<span {...stylex.props(sx.rowText)}><span title={j.source.title} {...stylex.props(sx.truncate)}>{j.source.title}</span><span {...stylex.props(sx.small, sx.truncate)}>{channels.data?.channels.find(c => c.id === j.channelId)?.name} · {j.subline}</span></span></button>)}
      {review.jobs && !jobs.length && <span {...stylex.props(sx.small)}>Không còn video nào chờ duyệt.</span>}
      {counts.data?.approvedToday != null && <div {...stylex.props(sx.footer)}><span>Duyệt xong hôm nay</span><span>{counts.data.approvedToday} video</span></div>}
    </aside>
    {!review.jobs ? <main {...stylex.props(sx.empty)}>{review.error ? <div role="alert"><span>Chưa tải được video chờ duyệt. </span><button type="button" {...stylex.props(sx.button)} onClick={() => void review.loadJobs()}>Thử lại</button></div> : <SystemLoading label="Đang tải video chờ duyệt…" />}</main> : !job ? <main {...stylex.props(sx.empty)}><div role="status" {...stylex.props(sx.emptyInner)}><Orb of="artist" size={30} /><strong>Đã duyệt hết</strong><span {...stylex.props(sx.small)}>Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.</span><a href={`#${paths.workbench()}`} {...stylex.props(sx.button)}>Xem tiến độ trên Bàn dựng</a></div></main> : <>
      <main {...stylex.props(sx.main)}>
        <div {...stylex.props(sx.heading)}><h1 title={job.source.title} {...stylex.props(sx.title)}>{job.source.title}</h1><span {...stylex.props(sx.chip)}>{channel?.name}</span><span {...stylex.props(sx.chip)}>Đã duyệt <b>{approved} / {items.length}</b></span><button type="button" {...stylex.props(sx.button, sx.short)} disabled={!items.length || busy || !online} onClick={() => setAllOpen(true)}>Vẽ lại cả {items.length} ảnh</button><a href={`#${paths.workbench()}?video=${encodeURIComponent(job.id)}`} {...stylex.props(sx.button, sx.short)}>Mở Bàn dựng</a></div>
        {!images ? <div role="status" {...stylex.props(sx.empty)}>{review.imageErrors[job.id] ? <><span>Chưa tải được ảnh. </span><button type="button" {...stylex.props(sx.button)} onClick={() => void review.loadImages(job.id)}>Thử lại</button></> : 'Đang tải ảnh…'}</div> : item ? <>
          <div {...stylex.props(sx.hero, view === 'cutout' && sx.checker)}>
            {!imageFailed && imageURL(job.id, item, view) && <img src={imageURL(job.id, item, view)} alt={label(item, selected)} onError={() => setImageFailed(true)} {...stylex.props(sx.heroImage, view === 'cutout' && sx.heroCutout)} />}
            {imageFailed && <div role="status" {...stylex.props(sx.overlay)}>{view === 'cutout' ? 'Chưa có bản tách nền cho ảnh này.' : 'Chưa tải được ảnh.'}</div>}
            {drawing(item) && <div role="status" {...stylex.props(sx.overlay)}><Orb of="artist" size={28} live /><span>Hoạ sĩ đang vẽ lại theo góp ý của bạn…</span><span {...stylex.props(sx.small)}>thường mất 1–2 phút · bạn duyệt tiếp ảnh khác được</span></div>}
            {blocked(item) && <div role="status" {...stylex.props(sx.overlay)}><div {...stylex.props(sx.blockedCard)}><strong>Bị chặn{neighbour ? ` · dùng ảnh ${label(neighbour, items.indexOf(neighbour)).toLocaleLowerCase('vi')}` : ''}</strong><span>Bộ lọc nội dung đã chặn ảnh này. Video dùng ảnh cảnh bên cạnh để tiếp tục.</span></div></div>}
            <span {...stylex.props(sx.heroLabel)}>{selected} · {label(item, selected)}{item.kind === 'thumbnail' && item.width && item.height ? ` · ${item.width}×${item.height}` : item.startSec != null && item.endSec != null ? ` · ${time(item.startSec)}–${time(item.endSec)}` : ''}</span>
            <button type="button" aria-label="Ảnh trước" onClick={() => moveImage(-1)} {...stylex.props(sx.nav, sx.prev)}>‹</button><button type="button" aria-label="Ảnh sau" onClick={() => moveImage(1)} {...stylex.props(sx.nav, sx.next)}>›</button>
            {item.kind === 'scene' && !drawing(item) && !blocked(item) && <div role="radiogroup" aria-label="Lớp ảnh" {...stylex.props(sx.layers)}>{(['full', 'cutout'] as const).map(v => <button key={v} type="button" role="radio" aria-checked={view === v} onClick={() => setView(v)} {...stylex.props(sx.layer, view === v && sx.layerOn)}>{v === 'full' ? 'Ảnh gốc' : 'Tách nền'}</button>)}</div>}
          </div>
          <div role="listbox" aria-label="Ảnh của video" {...stylex.props(sx.strip)}>{items.map((img, i) => <button key={img.id} type="button" role="option" aria-selected={selected === i} aria-label={label(img, i)} onClick={() => { setIndex(i); setView('full') }} {...stylex.props(sx.tile, blocked(img) && sx.tileBlocked, selected === i && sx.tileSelected)}>{imageURL(job.id, img) && <img src={imageURL(job.id, img)} alt="" {...stylex.props(sx.tileImage)} />}<span {...stylex.props(sx.tileLabel)}>{i}</span><span {...stylex.props(sx.tileState)}>{blocked(img) ? '!' : drawing(img) ? '●' : img.status === 'approved' ? '✓' : ''}</span></button>)}</div>
          <div {...stylex.props(sx.composer)}><Icon name="gallery-edit" size={18} /><input aria-label="Góp ý để vẽ lại ảnh này" placeholder="Góp ý để vẽ lại, ví dụ: thêm tấm biển “Mall closes at 10PM”, ánh đèn xanh hơn" value={feedback} onChange={e => setFeedback(e.target.value)} {...stylex.props(sx.input)} /><button type="button" disabled={drawing(item) || busy || !online} onClick={redraw} {...stylex.props(sx.button)}>Vẽ lại　R</button><button type="button" disabled={item.kind !== 'scene' || busy || !online} title="Cảnh này dùng ảnh của cảnh bên cạnh" onClick={() => void review.mutate(job.id, item, 'skip')} {...stylex.props(sx.button)}>Bỏ ảnh này</button><button type="button" aria-disabled={locked || !online} title={locked ? reason : 'Duyệt mọi ảnh của video này rồi dựng'} onClick={approve} {...stylex.props(sx.button, sx.primary, (locked || !online) && sx.disabled)}>Duyệt cả video &amp; dựng　A</button></div>
          <div {...stylex.props(sx.hints)}><span>← → ảnh</span><span>J K video trước / sau</span><span>R vẽ lại</span><span>A duyệt cả video</span><span>Duyệt xong, video tự chuyển sang dựng và mở video kế tiếp</span></div>
        </> : <div role="status">Chưa có ảnh để duyệt.</div>}
      </main>
      {item && <aside aria-label="Chi tiết ảnh" {...stylex.props(sx.rail)}><div {...stylex.props(sx.section)}><strong>{label(item, selected)}</strong><span {...stylex.props(sx.small)}>{item.direction}</span></div><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Lời kể trong cảnh</h2><p {...stylex.props(sx.textBox)}>{item.narration || 'Ảnh bìa không gắn với một đoạn lời kể.'}</p></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Khung đối thủ làm mẫu</h2>{item.referenceUrl && <img alt="Khung đối thủ làm mẫu" src={item.referenceUrl} {...stylex.props(sx.thumb)} />}<span {...stylex.props(sx.small)}>Giữ bố cục và góc máy; thay mặt người, bối cảnh và mọi chữ, logo.</span></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Mô tả Hoạ sĩ đã dùng</h2><p {...stylex.props(sx.textBox)}>{item.prompt}</p></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Lần vẽ</h2><span {...stylex.props(sx.chip)}>Lần {item.versions.length}{drawing(item) ? ' · đang vẽ' : blocked(item) ? ' · bị chặn' : ''}</span></section></aside>}
    </>}
    {allOpen && job && <div {...stylex.props(sx.backdrop)}><section role="dialog" aria-modal="true" aria-labelledby="review-redraw-title" {...stylex.props(sx.dialog)}><h2 id="review-redraw-title">Vẽ lại cả {items.length} ảnh?</h2><span {...stylex.props(sx.small)}>{job.source.title} · {channel?.name}</span><div {...stylex.props(sx.strip)}>{items.map((img, i) => <span key={img.id} {...stylex.props(sx.tile)}>{imageURL(job.id, img) && <img src={imageURL(job.id, img)} alt={label(img, i)} {...stylex.props(sx.tileImage)} />}</span>)}</div><p>Dùng {items.length} ảnh trong hạn mức ChatGPT hôm nay</p><p {...stylex.props(sx.small)}>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p><label>Góp ý chung cho cả bộ (tuỳ chọn)<textarea rows={2} value={allFeedback} onChange={e => setAllFeedback(e.target.value)} placeholder="ví dụ: tông lạnh hơn, ít người hơn, ánh đèn xanh" {...stylex.props(sx.textarea)} /></label><div {...stylex.props(sx.dialogActions)}><button type="button" {...stylex.props(sx.button)} onClick={() => setAllOpen(false)}>Huỷ</button><button type="button" {...stylex.props(sx.button, sx.primary)} onClick={() => { setAllOpen(false); void review.mutate(job.id, null, 'regenerateAll', allFeedback); setAllFeedback('') }}>Vẽ lại {items.length} ảnh</button></div></section></div>}
  </div>
}
