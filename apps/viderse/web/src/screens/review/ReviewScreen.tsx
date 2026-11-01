import { useEffect, useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Chip, Dialog, Icon, Kbd, Segmented, TextArea, TextField } from '@noverse/ui'
import { listChannels } from '../../api/channels'
import { request } from '../../api/request'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { Orb, SystemLoading } from '../../components'
import { ActionButton } from '../../components/ActionButton'
import { useOnline } from '../../data/status'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { paths } from '../../router'
import { imageURL, isBlocked, isDrawing, reviewLabel, useReview } from './useReview'
import { useReviewKeys } from './useReviewKeys'
import { ScoreBadge, ScoreBanner, ScorePanel } from '../../components/autonomy/ScoreParts'
import { isLow, summarizeScores } from '../../components/autonomy/scoreSummary'
import { scoreStyles } from '../../components/autonomy/score.stylex'
import { readChannelAutonomy, readImageScore } from '../../api/autonomy'
import { LearnSlot } from '../../components/learn/LearnSlot'
import { MReviewScreen } from './mobile/MReviewScreen'
import { sx } from './review.stylex'

const time = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`

/** ≤767: màn dọc riêng của artboard MReview; >767 giữ nguyên bố cục ba cột. */
export function ReviewScreen() {
  const phone = useMediaQuery('(max-width: 767px)')
  return phone ? <MReviewScreen /> : <DesktopReviewScreen />
}

function DesktopReviewScreen() {
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
  const autonomy = channel ? readChannelAutonomy(channel)?.autonomy : undefined
  const scores = autonomy ? summarizeScores(items.map((candidate, i) => ({ id: candidate.id, label: reviewLabel(candidate, i), drawing: isDrawing(candidate), score: candidate.score })), autonomy) : null
  const neighbour = [...items.slice(0, selected)].reverse().find(candidate => candidate.kind === 'scene' && candidate.status !== 'skipped') ?? items.find((candidate, i) => i > selected && candidate.kind === 'scene' && candidate.status !== 'skipped')
  const busy = !!job && !!review.pending[job.id]
  const locked = busy || items.some(isDrawing)
  const reason = !online ? 'Mất kết nối — thử lại khi mạng ổn định' : 'Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ'
  useEffect(() => { setImageFailed(false) }, [job?.id, item?.id, view])
  const selectJob = (id: string) => { setJobId(id); setIndex(0); setView('full'); setFeedback('') }
  useEffect(() => { if (job && !review.images[job.id] && !review.imageErrors[job.id]) void review.loadImages(job.id) }, [job, review.images, review.imageErrors, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    if (next && !review.images[next.id] && !review.imageErrors[next.id]) void review.loadImages(next.id)
  }, [job, jobs, review.images, review.imageErrors, review.loadImages])
  const pollingId = job?.id
  const hasDrawing = items.some(isDrawing)
  useEffect(() => {
    if (!pollingId || !hasDrawing) return
    const timer = window.setInterval(() => void review.loadImages(pollingId), 2500)
    return () => clearInterval(timer)
  }, [pollingId, hasDrawing, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    for (const image of next ? review.images[next.id]?.items ?? [] : []) { const url = imageURL(next!.id, image); if (url) { const preload = new Image(); preload.src = url } }
  }, [job, jobs, review.images])
  const redraw = () => { if (job && item && !isDrawing(item) && !busy && online) { void review.mutate(job.id, item, 'regenerate', feedback).then(ok => { if (ok) setFeedback('') }) } }
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
  const drawing = item ? isDrawing(item) : false
  return <div {...stylex.props(sx.root)}>
    <aside aria-label="Video chờ duyệt" {...stylex.props(sx.side)}>
      <div {...stylex.props(sx.group)}><span>Chờ bạn duyệt</span><span>{jobs.length}</span></div>
      {jobs.map(j => <button key={j.id} type="button" aria-pressed={j.id === job?.id} onClick={() => selectJob(j.id)} {...stylex.props(sx.row, j.id === job?.id && sx.selectedRow)}>{j.source.thumbnailUrl ? <img alt="" src={j.source.thumbnailUrl} {...stylex.props(sx.thumb)} /> : <span {...stylex.props(sx.thumb)} /> }<span {...stylex.props(sx.rowText)}><span title={j.source.title} {...stylex.props(sx.truncate)}>{j.source.title}</span><span {...stylex.props(sx.small, sx.truncate)}>{channels.data?.channels.find(c => c.id === j.channelId)?.name} · {j.subline}</span></span></button>)}
      {review.jobs && !jobs.length && <span {...stylex.props(sx.small)}>Không còn video nào chờ duyệt.</span>}
      {counts.data?.approvedToday != null && <div {...stylex.props(sx.footer)}><span>Duyệt xong hôm nay</span><span>{counts.data.approvedToday} video</span></div>}
    </aside>
    {!review.jobs ? <main {...stylex.props(sx.empty)}>{review.error ? <div role="alert"><span>Chưa tải được video chờ duyệt. </span><ActionButton size="sm" onClick={() => void review.loadJobs()}>Thử lại</ActionButton></div> : <SystemLoading label="Đang tải video chờ duyệt…" />}</main> : !job ? <main {...stylex.props(sx.empty)}><div role="status" {...stylex.props(sx.emptyInner)}><span aria-hidden="true" {...stylex.props(sx.emptyOrbs)}><Orb of="artist" size={30} /><span {...stylex.props(sx.emptyQa)}><Orb of="qa" size={24} /></span></span><strong {...stylex.props(sx.emptyTitle)}>Đã duyệt hết</strong><span {...stylex.props(sx.emptyText)}>Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.</span><a href={`#${paths.workbench()}`} {...stylex.props(sx.button, sx.emptyLink)}>Xem tiến độ trên Bàn dựng</a></div></main> : <>
      <main className={`vv-wide vv-wide--grid ${stylex.props(sx.main).className}`}>
        <div {...stylex.props(sx.heading)}><h1 title={job.source.title} {...stylex.props(sx.title)}>{job.source.title}</h1>{channel && <Chip size="sm" label={channel.name} />}<Chip size="sm" label="Đã duyệt" value={`${approved} / ${items.length}`} /><ActionButton size="sm" disabled={!items.length || busy || !online} disabledReason="Chờ ảnh đang vẽ xong" onClick={() => setAllOpen(true)}>Vẽ lại cả {items.length} ảnh</ActionButton><a href={`#${paths.workbench()}?video=${encodeURIComponent(job.id)}`} {...stylex.props(sx.button, sx.short)}>Mở Bàn dựng</a></div>
        {!images ? <div role="status" {...stylex.props(sx.empty)}>{review.imageErrors[job.id] ? <><span>Chưa tải được ảnh. </span><ActionButton size="sm" onClick={() => void review.loadImages(job.id)}>Thử lại</ActionButton></> : 'Đang tải ảnh…'}</div> : item ? <>
          <div {...stylex.props(sx.hero, view === 'cutout' && sx.checker)}>
            {!imageFailed && imageURL(job.id, isBlocked(item) && neighbour ? neighbour : item, view) && <img src={imageURL(job.id, isBlocked(item) && neighbour ? neighbour : item, view)} alt={isBlocked(item) && neighbour ? reviewLabel(neighbour, items.indexOf(neighbour)) : reviewLabel(item, selected)} onError={() => setImageFailed(true)} {...stylex.props(sx.heroImage, view === 'cutout' && sx.heroCutout)} />}
            {imageFailed && <div role="status" {...stylex.props(sx.overlay)}>{view === 'cutout' ? 'Chưa có bản tách nền cho ảnh này.' : 'Chưa tải được ảnh.'}</div>}
            {drawing && <div role="status" {...stylex.props(sx.overlay)}><Orb of="artist" size={28} live /><span>Hoạ sĩ đang vẽ lại theo góp ý của bạn…</span><span {...stylex.props(sx.small)}>thường mất 1–2 phút · bạn duyệt tiếp ảnh khác được</span></div>}
            {isBlocked(item) && <div role="status" {...stylex.props(sx.overlay, sx.blockedOverlay)}><div {...stylex.props(sx.blockedCard)}><strong>Bị chặn{neighbour ? ` · dùng ảnh ${reviewLabel(neighbour, items.indexOf(neighbour)).toLocaleLowerCase('vi')}` : ''}</strong><span>Bộ lọc nội dung đã chặn ảnh này. Video dùng ảnh cảnh bên cạnh để tiếp tục.</span></div></div>}
            <span {...stylex.props(sx.heroLabel)}>{selected} · {reviewLabel(item, selected)}{item.kind === 'thumbnail' && item.width && item.height ? ` · ${item.width}×${item.height}` : item.startSec != null && item.endSec != null ? ` · ${time(item.startSec)}–${time(item.endSec)}` : ''}</span>
            <Button variant="ghost" aria-label="Ảnh trước" onClick={() => moveImage(-1)} xstyle={[sx.nav, sx.prev]}><Icon name="arrow-left-2" size={16} /></Button>
            <Button variant="ghost" aria-label="Ảnh sau" onClick={() => moveImage(1)} xstyle={[sx.nav, sx.next]}><Icon name="arrow-r" size={16} /></Button>
            {item.kind === 'scene' && !drawing && !isBlocked(item) && <Segmented label="Lớp ảnh" value={view} onChange={next => setView(next as 'full' | 'cutout')} size="sm" fit="content" xstyle={sx.layers} options={[{ id: 'full', label: 'Ảnh gốc' }, { id: 'cutout', label: 'Tách nền' }]} />}
          </div>
          {scores && autonomy && <ScoreBanner summary={scores} autonomy={autonomy} channelName={channel?.name ?? ''} onJump={(id) => { const at = items.findIndex(candidate => candidate.id === id); if (at >= 0) { setIndex(at); setView('full') } }} />}
          <div role="listbox" aria-label="Ảnh của video" {...stylex.props(sx.strip)}>{items.map((img, i) => <button key={img.id} type="button" role="option" aria-selected={selected === i} aria-label={reviewLabel(img, i)} onClick={() => { setIndex(i); setView('full') }} {...stylex.props(sx.tile, isBlocked(img) && sx.tileBlocked, scores && !isDrawing(img) && isLow(readImageScore(img.score), scores.min) && scoreStyles.tileLow, selected === i && sx.tileSelected)}>{imageURL(job.id, img) && <img src={imageURL(job.id, img)} alt="" {...stylex.props(sx.tileImage)} />}<span {...stylex.props(sx.tileLabel)}>{i}</span>{scores && !isDrawing(img) && readImageScore(img.score) && <ScoreBadge corner value={readImageScore(img.score)?.value ?? 0} low={isLow(readImageScore(img.score), scores.min)} />}<span {...stylex.props(sx.tileState)}>{isBlocked(img) ? '!' : isDrawing(img) ? '●' : img.status === 'approved' ? '✓' : ''}</span></button>)}</div>
          <div {...stylex.props(sx.composer)}><TextField aria-label="Góp ý để vẽ lại ảnh này" placeholder="Góp ý để vẽ lại, ví dụ: thêm tấm biển “Mall closes at 10PM”, ánh đèn xanh hơn" prefix={<Icon name="gallery-edit" size={16} />} value={feedback} onChange={setFeedback} xstyle={sx.composerField} /><ActionButton kbd="R" disabled={drawing || busy || !online} disabledReason="Chờ ảnh đang vẽ xong" onClick={redraw}>Vẽ lại</ActionButton><ActionButton disabled={item.kind !== 'scene' || busy || !online} disabledReason="Cảnh này dùng ảnh của cảnh bên cạnh" onClick={() => void review.mutate(job.id, item, 'skip')}>Bỏ ảnh này</ActionButton><ActionButton variant="primary" kbd="A" lockedReason={locked || !online ? reason : undefined} onClick={approve}>{busy ? 'Đang gửi…' : 'Duyệt cả video & dựng'}</ActionButton></div>
          <LearnSlot learn={images?.learn} />
          <div {...stylex.props(sx.hints)}><span {...stylex.props(sx.keyHint)}><Kbd>← →</Kbd> ảnh</span><span {...stylex.props(sx.keyHint)}><Kbd>J K</Kbd> video trước / sau</span><span {...stylex.props(sx.keyHint)}><Kbd>R</Kbd> vẽ lại</span><span {...stylex.props(sx.keyHint)}><Kbd>A</Kbd> duyệt cả video</span><span {...stylex.props(sx.hintSpacer)} /><span>Duyệt xong, video tự chuyển sang dựng và mở video kế tiếp</span></div>
        </> : <div role="status">Chưa có ảnh để duyệt.</div>}
      </main>
      {item && <aside aria-label="Chi tiết ảnh" {...stylex.props(sx.rail)}><div {...stylex.props(sx.section)}><strong>{reviewLabel(item, selected)}</strong><span {...stylex.props(sx.small)}>{item.direction}</span></div>{scores && <ScorePanel imageId={item.id} rawScore={item.score} min={scores.min} />}<section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Lời kể trong cảnh</h2><p {...stylex.props(sx.textBox)}>{item.narration || 'Ảnh bìa không gắn với một đoạn lời kể.'}</p></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Khung đối thủ làm mẫu</h2><div {...stylex.props(sx.referenceRow)}>{item.referenceUrl && <img alt="Khung đối thủ làm mẫu" src={item.referenceUrl} {...stylex.props(sx.referenceImage)} />}<span {...stylex.props(sx.small)}>Giữ bố cục và góc máy; thay mặt người, bối cảnh và mọi chữ, logo.</span></div></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Mô tả Hoạ sĩ đã dùng</h2><p {...stylex.props(sx.textBox)}>{item.prompt}</p></section><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.sectionTitle)}>Lần vẽ</h2><Chip size="sm" label={`Lần ${item.versions.length}${drawing ? ' · đang vẽ' : isBlocked(item) ? ' · bị chặn' : ''}`} /></section></aside>}
    </>}
    {/* Thân hộp này tự đệm 18 px nên tắt đệm thân của kit. */}
    {allOpen && job && <Dialog title={`Vẽ lại cả ${items.length} ảnh?`} onClose={() => setAllOpen(false)} width={540} padBody={false} footer={<><ActionButton onClick={() => setAllOpen(false)}>Huỷ</ActionButton><Button variant="primary" onClick={() => { setAllOpen(false); void review.mutate(job.id, null, 'regenerateAll', allFeedback); setAllFeedback('') }}>Vẽ lại {items.length} ảnh</Button></>}>
      <div {...stylex.props(sx.dialogBody)}>
        <div {...stylex.props(sx.dialogMeta)}><span>{job.source.title}</span><span {...stylex.props(sx.small)}>{channel?.name}</span></div>
        <div {...stylex.props(sx.strip)}>{items.map((img, i) => <span key={img.id} {...stylex.props(sx.tile)}>{imageURL(job.id, img) && <img src={imageURL(job.id, img)} alt={reviewLabel(img, i)} {...stylex.props(sx.tileImage)} />}</span>)}</div>
        <p>Dùng {items.length} ảnh trong hạn mức ChatGPT hôm nay</p>
        <p {...stylex.props(sx.small)}>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
        <TextArea label="Góp ý chung cho cả bộ (tuỳ chọn)" rows={2} showCount={false} maxLength={-1} value={allFeedback} onChange={event => setAllFeedback(event.currentTarget.value)} placeholder="ví dụ: tông lạnh hơn, ít người hơn, ánh đèn xanh" />
      </div>
    </Dialog>}
  </div>
}
