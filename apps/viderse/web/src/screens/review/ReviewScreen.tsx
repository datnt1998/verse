// Duyệt ảnh — duyệt cả video bằng phím, mỗi ảnh có phiên bản, ảnh tham chiếu và
// prompt đã gửi; góp ý hoặc sửa prompt rồi vẽ lại đúng ảnh (artboard Review, E1).
// Vẽ lại đi qua §26 (`api/images`), duyệt/bỏ ảnh vẫn dùng thao tác cũ của màn.
import { useEffect, useMemo, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Chip, Dialog, Icon, Kbd, Segmented, TextArea, TextField, toast } from '@noverse/ui'
import { listProviders } from '../../api/ai'
import { listChannels } from '../../api/channels'
import { request } from '../../api/request'
import { ApiError } from '../../api/request'
import {
  cancelImageVersion, redrawImages, tooManyRefs, uploadRefImage, useImageVersion, useJobImages,
  type ImageRef, type PromptPartKind, type RedrawBody,
} from '../../api/images'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { Orb, SystemLoading, DevelopEffect } from '../../components'
import { ActionButton } from '../../components/ActionButton'
import { useOnline } from '../../data/status'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { paths } from '../../router'
import { imageURL, isBlocked as blocked, isDrawing as drawing, reviewLabel as label, useReview } from './useReview'
import { useReviewKeys } from './useReviewKeys'
import { ScoreBadge, ScoreBanner, ScorePanel } from '../../components/autonomy/ScoreParts'
import { isLow, summarizeScores } from '../../components/autonomy/scoreSummary'
import { scoreStyles } from '../../components/autonomy/score.stylex'
import { readChannelAutonomy, readImageScore } from '../../api/autonomy'
import { LearnSlot } from '../../components/learn/LearnSlot'
import { MReviewScreen } from './mobile/MReviewScreen'
import { ImageRail, type RefCandidate } from './ImageRail'
import { toReviewItems, stageWidthVar, providerName, resetAtLabel, type ReviewItemView } from './imageView'
import { useImageEditing } from './useImageEditing'
import { sx } from './review.stylex'

const clock = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`

/** Ảnh tham chiếu gửi kèm: ảnh đã lưu gửi lại bằng id, ảnh mới gửi bằng nguồn. */
function toRedrawRefs(refs: ImageRef[]) {
  return refs.map((ref) => {
    if (ref.source.type === 'upload') return { upload: ref.id, role: ref.role }
    if (ref.source.type === 'rival_frame' && ref.source.videoId != null && ref.source.atSec != null) {
      return { rivalFrame: { videoId: ref.source.videoId, atSec: ref.source.atSec }, role: ref.role }
    }
    if (ref.source.type === 'video_image' && ref.source.imageId) return { videoImageId: ref.source.imageId, role: ref.role }
    return { id: ref.id, role: ref.role }
  })
}

/** ≤767: màn dọc riêng của artboard MReview; >767 giữ nguyên bố cục ba cột. */
export function ReviewScreen() {
  const phone = useMediaQuery('(max-width: 767px)')
  return phone ? <MReviewScreen /> : <DesktopReviewScreen />
}

function DesktopReviewScreen() {
  const review = useReview()
  const online = useOnline()
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const counts = useResource(['review', 'counts'], ({ signal }) => request<{ approvedToday: number | null }>('/api/clone/counts', { signal }))
  const [jobId, setJobId] = useState<string | null>(() => new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('video'))
  const [index, setIndex] = useState(0)
  const [view, setView] = useState<'full' | 'cutout'>('full')
  const [imageFailed, setImageFailed] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [allOpen, setAllOpen] = useState(false)
  const [allFeedback, setAllFeedback] = useState('')
  const [viewN, setViewN] = useState<number | null>(null)
  const [cmp, setCmp] = useState(false)
  const [selection, setSelection] = useState<string[]>([])
  const [batchFeedback, setBatchFeedback] = useState('')
  const [reveal, setReveal] = useState<{ imageId: string; n: number } | null>(null)
  const [uploading, setUploading] = useState(false)
  const busyRef = useRef(false)
  const channelFilter = new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('channel')
  const jobs = useMemo(() => (review.jobs ?? []).filter(j => !channelFilter || channelFilter === 'all' || j.channelId === channelFilter), [review.jobs, channelFilter])
  const job = jobs.find(j => j.id === jobId) ?? jobs[0]
  const images = job ? review.images[job.id] : undefined
  const legacyItems = images?.items ?? []
  const live = useJobImages(job?.id ?? null)
  const items = useMemo(() => toReviewItems(legacyItems, live.data?.images, live.progress), [legacyItems, live.data, live.progress])
  const selected = Math.min(index, Math.max(0, items.length - 1))
  const item = items[selected]
  const legacy = legacyItems[selected]
  const channel = channels.data?.channels.find(c => c.id === job?.channelId)
  const autonomy = channel ? readChannelAutonomy(channel)?.autonomy : undefined
  const scores = autonomy ? summarizeScores(legacyItems.map((candidate, i) => ({ id: candidate.id, label: label(candidate, i), drawing: drawing(candidate), score: candidate.score })), autonomy) : null
  const neighbour = [...legacyItems.slice(0, selected)].reverse().find(candidate => candidate.kind === 'scene' && candidate.status !== 'skipped') ?? legacyItems.find((candidate, i) => i > selected && candidate.kind === 'scene' && candidate.status !== 'skipped')
  const pipelineBusy = !!job && !!review.pending[job.id]

  const usedVersion = item?.versions.find(version => version.n === item.usedVersion) ?? item?.versions.at(-1)
  const drawingVersion = item?.versions.find(version => version.state === 'drawing')
  const failedVersion = item?.versions.find(version => version.state === 'failed')
  const versionN = viewN ?? item?.versions.at(-1)?.n ?? item?.usedVersion ?? 1
  const version = item?.versions.find(candidate => candidate.n === versionN) ?? usedVersion
  const previousVersion = item?.versions.find(candidate => candidate.n === versionN - 1)
  const editing = useImageEditing(item?.id, failedVersion ?? usedVersion)
  const batch = selection.length > 0
  const isDrawing = !!drawingVersion
  const isFailed = !isDrawing && !!failedVersion
  const justRevealed = !!reveal && reveal.imageId === item?.id && reveal.n === versionN && !cmp
  const shownVersion = cmp && previousVersion ? previousVersion : version
  const shownImage = shownVersion?.url ?? shownVersion?.previewUrl ?? (cmp ? undefined : usedVersion?.url) ?? item?.legacyUrl ?? (legacy ? imageURL(job!.id, legacy, view) : undefined)
  const frameTime = item?.startSec != null && item.endSec != null ? `${clock(item.startSec)}–${clock(item.endSec)}` : item?.kind === 'cover' && usedVersion?.width ? `${usedVersion.width}×${usedVersion.height}` : ''
  const versionTag = isDrawing ? `đang vẽ bản ${drawingVersion!.n}` : cmp && previousVersion ? `bản ${previousVersion.n} (so sánh)` : `bản ${versionN}${versionN === item?.usedVersion ? '' : ' · chưa dùng'}`
  const blockedNow = !!legacy && blocked(legacy)

  const candidates = useMemo(() => {
    const frames: RefCandidate[] = []
    for (const entry of items) {
      for (const candidate of entry.versions) {
        for (const ref of candidate.refs) {
          if (ref.source.type !== 'rival_frame' || ref.source.videoId == null || ref.source.atSec == null) continue
          const id = `${ref.source.videoId}|${ref.source.atSec}`
          if (!frames.some(frame => frame.id === id)) frames.push({ id, url: ref.url, tag: clock(ref.source.atSec), label: ref.source.label, kind: 'frame' })
        }
      }
    }
    const shots = items
      .filter(entry => entry.id !== item?.id)
      .map(entry => ({ id: entry.id, url: entry.legacyUrl ?? entry.versions.at(-1)?.url ?? '', tag: entry.label, label: entry.label, kind: 'image' as const }))
      .filter(entry => entry.url)
    return { images: shots, frames }
  }, [items, item?.id])

  useEffect(() => { setImageFailed(false) }, [job?.id, item?.id, view, shownImage])
  useEffect(() => { setViewN(null); setCmp(false); setFeedback('') }, [item?.id, job?.id])
  useEffect(() => {
    const progress = item?.progress
    if (progress?.phase === 'done') setReveal({ imageId: progress.imageId, n: progress.n })
  }, [item?.progress])

  const selectJob = (id: string) => { setJobId(id); setIndex(0); setView('full'); setFeedback(''); setSelection([]) }
  useEffect(() => { if (job && !review.images[job.id] && !review.imageErrors[job.id]) void review.loadImages(job.id) }, [job, review.images, review.imageErrors, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    if (next && !review.images[next.id] && !review.imageErrors[next.id]) void review.loadImages(next.id)
  }, [job, jobs, review.images, review.imageErrors, review.loadImages])
  const pollingId = job?.id
  const hasDrawing = legacyItems.some(drawing) || items.some(candidate => candidate.versions.some(version => version.state === 'drawing'))
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

  const refresh = () => { if (job) { void live.reload(); void review.loadImages(job.id) } }

  const runRedraw = async (body: Omit<RedrawBody, 'imageIds'> & { imageIds?: string[] }) => {
    if (!job || !online || busyRef.current) return
    busyRef.current = true
    try {
      await redrawImages(job.id, { imageIds: body.imageIds ?? (item ? [item.id] : []), ...body })
      setFeedback('')
      refresh()
    } catch (error) {
      const limit = tooManyRefs(error)
      if (limit !== null) toast.err(`${providerName(usedVersion?.provider ?? 'chatgpt', providers.data)} nhận tối đa ${limit} ảnh tham chiếu.`)
      else toast.err(error instanceof ApiError ? error.message : 'Chưa vẽ lại được. Vui lòng thử lại.')
    } finally { busyRef.current = false }
  }

  /** Góp ý → Hoạ sĩ sửa phần Cảnh của đúng ảnh rồi vẽ (giữ như cũ, nay có phiên bản). */
  const redraw = () => { if (!item || isDrawing) return; void runRedraw(feedback ? { feedback } : {}) }

  /** Vẽ lại với prompt đã sửa: chỉ gửi phần đã đổi + ảnh tham chiếu nếu đổi. */
  const submitEdit = () => {
    if (!item || editing.changedCount === 0) return
    const body: Omit<RedrawBody, 'imageIds'> = {}
    if (Object.keys(editing.changed).length) body.parts = editing.changed as Partial<Record<PromptPartKind, string>>
    const baseRefs = (failedVersion ?? usedVersion)?.refs ?? []
    const refsChanged = editing.changedCount - Object.keys(editing.changed).length > 0
    if (refsChanged || baseRefs.length !== editing.refs.length) body.refs = toRedrawRefs(editing.refs)
    void runRedraw(body)
    editing.reset()
  }

  /** Thẻ lỗi: vẽ bằng Grok đúng prompt đã sửa (phần + ảnh tham chiếu của bản lỗi). */
  const retryWithGrok = () => {
    if (!item || !failedVersion) return
    const parts: Partial<Record<PromptPartKind, string>> = {}
    for (const part of failedVersion.parts) if (part.kind !== 'refs') parts[part.kind] = part.text
    void runRedraw({ provider: 'grok', ...(Object.keys(parts).length ? { parts } : {}), ...(failedVersion.refs.length ? { refs: toRedrawRefs(failedVersion.refs) } : {}) })
  }

  const cancelDrawing = () => {
    if (!job || !item || !drawingVersion) return
    void cancelImageVersion(job.id, item.id, drawingVersion.n)
      .then((result) => { toast(`Đã huỷ lần vẽ bản ${result.n}. Bản ${result.usedVersion} vẫn dùng cho video.`); refresh() })
      .catch((error: unknown) => toast.err(error instanceof Error ? error.message : 'Chưa huỷ được lần vẽ.'))
  }

  const useVersion = (n: number) => {
    if (!job || !item) return
    void useImageVersion(job.id, item.id, n)
      .then(() => { setViewN(n); refresh() })
      .catch((error: unknown) => toast.err(error instanceof Error ? error.message : 'Chưa đổi được bản dùng cho video.'))
  }

  const upload = (file: File) => {
    setUploading(true)
    void uploadRefImage(file)
      .then((id) => {
        editing.setMode('edit')
        editing.addRef({ id, role: 'character', url: URL.createObjectURL(file), source: { type: 'upload', label: file.name } })
      })
      .catch((error: unknown) => toast.err(error instanceof Error ? error.message : 'Chưa tải được ảnh lên.'))
      .finally(() => setUploading(false))
  }
  const approve = () => {
    if (!job || !images || !legacyItems.length || isDrawing || pipelineBusy || !online) return
    const at = jobs.findIndex(j => j.id === job.id)
    const next = jobs[at + 1] ?? jobs[at - 1]
    if (next) selectJob(next.id)
    void review.mutate(job.id, null, 'approveAll').then(ok => { if (!ok) selectJob(job.id) })
  }
  const moveJob = (delta: number) => { if (!job) return; const next = jobs[Math.max(0, Math.min(jobs.length - 1, jobs.findIndex(j => j.id === job.id) + delta))]; if (next) selectJob(next.id) }
  const moveImage = (delta: number) => { if (items.length) { setIndex((selected + delta + items.length) % items.length); setView('full') } }
  const toggleCompare = () => { if (versionN > 1 && !isDrawing && !isFailed) setCmp(on => !on) }
  const escape = () => {
    if (editing.mode === 'edit') { editing.reset(); return }
    if (selection.length) { setSelection([]); return }
    if (allOpen) setAllOpen(false)
  }
  useReviewKeys({
    nextJob: () => moveJob(1), previousJob: () => moveJob(-1), nextImage: () => moveImage(1), previousImage: () => moveImage(-1),
    redraw, approve, editPrompt: () => { if (item && !isDrawing) editing.setMode('edit') }, compare: toggleCompare,
    escape, submit: submitEdit,
  }, !allOpen)

  const pickImage = (at: number, shift: boolean) => {
    if (shift) {
      setSelection(old => old.includes(items[at].id) ? old.filter(id => id !== items[at].id) : [...old, items[at].id])
      return
    }
    setIndex(at); setView('full'); setCmp(false)
  }

  const redrawBatch = () => {
    if (!selection.length) return
    void runRedraw({ imageIds: selection, ...(batchFeedback ? { feedback: batchFeedback } : {}) }).then(() => {
      setSelection([]); setBatchFeedback('')
    })
  }

  const approved = legacyItems.filter(i => i.status === 'approved').length
  const usedImage = usedVersion?.url ?? (legacy && job ? imageURL(job.id, legacy, view) : undefined)
  const editTitle = editing.changedCount
    ? `Đã sửa ${editing.changedCount} chỗ trong prompt ${item?.label.toLocaleLowerCase('vi') ?? ''}`
    : `Sửa prompt ${item?.label.toLocaleLowerCase('vi') ?? ''}`
  const quotaLine = live.data?.quota
    ? `${live.data.quota.providers.map(entry => `${entry.label} còn ${entry.left}/${entry.total} hôm nay`).join(' · ')}`
    : ''
  const detail = drawingVersion
    ? item?.progress
      ? item.progress.phase === 'sending'
        ? `${clock(item.progress.elapsedSec)} · đang gửi prompt và ${usedVersion?.refs.length ?? 0} ảnh tham chiếu`
        : item.progress.phase === 'receiving'
          ? `${clock(item.progress.elapsedSec)} · đang nhận ảnh, sắp xong`
          : `${clock(item.progress.elapsedSec)} · thường khoảng 40 giây`
      : '0:01 · đang gửi prompt'
    : undefined
  const step = !item?.progress ? 1 : item.progress.phase === 'sending' ? 0 : item.progress.phase === 'receiving' ? 2 : 1
  return <div {...stylex.props(sx.root)}>
    <aside aria-label="Video chờ duyệt" {...stylex.props(sx.side)}>
      <div {...stylex.props(sx.group)}><span>Chờ bạn duyệt</span><span>{jobs.length}</span></div>
      {jobs.map(j => <button key={j.id} type="button" aria-pressed={j.id === job?.id} onClick={() => selectJob(j.id)} {...stylex.props(sx.row, j.id === job?.id && sx.selectedRow)}>{j.source.thumbnailUrl ? <img alt="" src={j.source.thumbnailUrl} {...stylex.props(sx.thumb)} /> : <span {...stylex.props(sx.thumb)} /> }<span {...stylex.props(sx.rowText)}><span title={j.source.title} {...stylex.props(sx.truncate)}>{j.source.title}</span><span {...stylex.props(sx.small, sx.truncate)}>{channels.data?.channels.find(c => c.id === j.channelId)?.name} · {j.subline}</span></span></button>)}
      {review.jobs && !jobs.length && <span {...stylex.props(sx.small)}>Không còn video nào chờ duyệt.</span>}
      {counts.data?.approvedToday != null && <div {...stylex.props(sx.footer)}><span>Duyệt xong hôm nay</span><span>{counts.data.approvedToday} video</span></div>}
    </aside>
    {!review.jobs ? <main {...stylex.props(sx.empty)}>{review.error ? <div role="alert"><span>Chưa tải được video chờ duyệt. </span><ActionButton size="sm" onClick={() => void review.loadJobs()}>Thử lại</ActionButton></div> : <SystemLoading label="Đang tải video chờ duyệt…" />}</main> : !job ? <main {...stylex.props(sx.empty)}><div role="status" {...stylex.props(sx.emptyInner)}><span aria-hidden="true" {...stylex.props(sx.emptyOrbs)}><Orb of="artist" size={30} /><span {...stylex.props(sx.emptyQa)}><Orb of="qa" size={24} /></span></span><strong {...stylex.props(sx.emptyTitle)}>Đã duyệt hết</strong><span {...stylex.props(sx.emptyText)}>Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.</span><a href={`#${paths.workbench()}`} {...stylex.props(sx.link, sx.emptyLink)} >Xem tiến độ trên Bàn dựng</a></div></main> : <>
      <main className={`vv-wide vv-wide--grid ${stylex.props(sx.main).className}`}>
        <div {...stylex.props(sx.heading)}>
          <h1 title={job.source.title} {...stylex.props(sx.title)}>{job.source.title}</h1>
          {channel && <Chip size="sm" label={channel.name} />}
          <Chip size="sm" label="Đã duyệt" value={`${approved} / ${items.length}`} />
          <ActionButton size="sm" disabled={!items.length || pipelineBusy || !online} disabledReason="Chờ ảnh đang vẽ xong" onClick={() => setAllOpen(true)}>Vẽ lại cả {items.length} ảnh</ActionButton>
          <a href={`#${paths.workbench()}?video=${encodeURIComponent(job.id)}`} {...stylex.props(sx.link)}>Mở Bàn dựng</a>
        </div>
        {!items.length ? <div role="status" {...stylex.props(sx.empty)}>{review.imageErrors[job.id] ? <><span>Chưa tải được ảnh. </span><Button type="button" onClick={() => void review.loadImages(job.id)}>Thử lại</Button></> : 'Đang tải ảnh…'}</div> : item ? <>
          <div {...stylex.props(sx.stageWrap)}>
            <div {...stylex.props(sx.stageBox, sx.stageSize(stageWidthVar(version)), isDrawing && sx.stageDrawing, view === 'cutout' && sx.checker)}>
              {isDrawing ? (
                <DevelopEffect
                  state="drawing"
                  baseUrl={usedImage}
                  previewUrl={drawingVersion?.previewUrl ?? null}
                  title={`Hoạ sĩ đang vẽ bản ${drawingVersion!.n}`}
                  detail={detail}
                  step={step}
                  onCancel={cancelDrawing}
                />
              ) : isFailed ? (
                <>
                  {usedImage && <img alt="" src={usedImage} {...stylex.props(sx.stageImage, sx.dimmed)} />}
                  <div role="alert" {...stylex.props(sx.failedCard)}>
                    <span {...stylex.props(sx.failedTitle)}><Icon name="danger" size={16} />Chưa vẽ được bản {failedVersion!.n}</span>
                    <span {...stylex.props(sx.failedText)}>
                      {failedVersion!.error?.message || 'Nhà cung cấp chưa vẽ được ảnh này.'}
                      {failedVersion!.error?.reason === 'quota' ? ` (${resetAtLabel(failedVersion!.error.retryAt)})` : ''}
                      {' '}Bản {item.usedVersion} vẫn được giữ; prompt bạn sửa vẫn còn.
                    </span>
                    <div {...stylex.props(sx.failedActions)}>
                      <Button variant="primary" size="sm" type="button" disabled={!online} onClick={retryWithGrok} >
                        Vẽ bằng {providerName('grok', providers.data)}{live.data?.quota?.fallback ? ` · còn ${live.data.quota.fallback.left} ảnh` : ''}</Button>
                      <Button size="sm" type="button" onClick={() => editing.setMode('edit')} >Sửa prompt</Button>
                    </div>
                  </div>
                </>
              ) : justRevealed && shownImage ? (
                <DevelopEffect state="done"><img alt={item.label} src={shownImage} onError={() => setImageFailed(true)} {...stylex.props(sx.stageImage, view === 'cutout' && sx.heroCutout)} /></DevelopEffect>
              ) : (
                !blockedNow && shownImage && !imageFailed && <img alt={cmp && previousVersion ? `${item.label} bản ${previousVersion.n}` : item.label} src={shownImage} onError={() => setImageFailed(true)} {...stylex.props(sx.stageImage, view === 'cutout' && sx.heroCutout)} />
              )}
              {blockedNow && legacy && neighbour && <img alt="" src={imageURL(job.id, neighbour, view)} {...stylex.props(sx.stageImage, sx.dimmed)} />}
              {imageFailed && !isDrawing && <div role="status" {...stylex.props(sx.empty)}>{view === 'cutout' ? 'Chưa có bản tách nền cho ảnh này.' : 'Chưa tải được ảnh.'}</div>}
              <span {...stylex.props(sx.badge)}>
                <span {...stylex.props(sx.badgeKey)}>{item.index}</span>{item.label}
                {frameTime && <span {...stylex.props(sx.badgeVersion)}>· {frameTime}</span>}
                <span {...stylex.props(versionTag.includes('chưa dùng') ? sx.badgeVersionUnused : sx.badgeVersion)}>· {versionTag}</span>
              </span>
              {legacy?.status === 'approved' && <span {...stylex.props(sx.approved)}><Icon name="tick-b" size={15} />Đã duyệt</span>}
              {versionN > 1 && !isDrawing && !isFailed && (
                <Button size="sm" aria-pressed={cmp} onClick={toggleCompare} tooltip="Phím V" xstyle={sx.compare} icon={<Icon name="eye" size={14} />}>
                  {cmp && previousVersion ? `Đang xem bản ${previousVersion.n}` : `So với bản ${previousVersion?.n ?? versionN - 1}`}
                </Button>
              )}
              <Button variant="icon" aria-label="Ảnh trước" onClick={() => moveImage(-1)} xstyle={[sx.nav, sx.prev]} icon={<Icon name="arrow-r" size={16} xstyle={sx.prevIcon} />} />
              <Button variant="icon" aria-label="Ảnh sau" onClick={() => moveImage(1)} xstyle={[sx.nav, sx.next]} icon={<Icon name="arrow-r" size={16} />} />
              {item.kind === 'scene' && !isDrawing && !blockedNow && (
                <Segmented label="Lớp ảnh" value={view} onChange={next => setView(next as 'full' | 'cutout')} size="sm" fit="content" xstyle={sx.layers} options={[{ id: 'full', label: 'Ảnh gốc', icon: <Icon name="gallery" size={14} /> }, { id: 'cutout', label: 'Tách nền', icon: <Icon name="layer" size={14} /> }]} />
              )}
            </div>
          </div>
          {scores && autonomy && <ScoreBanner summary={scores} autonomy={autonomy} channelName={channel?.name ?? ''} onJump={(id) => { const at = items.findIndex(candidate => candidate.id === id); if (at >= 0) { setIndex(at); setView('full') } }} />}
          <div role="listbox" aria-label="Ảnh của video" {...stylex.props(sx.strip)}>
            {items.map((img, i) => {
              const imgDrawing = img.versions.some(candidate => candidate.state === 'drawing')
              const imgUrl = img.versions.find(candidate => candidate.n === img.usedVersion)?.url ?? img.legacyUrl ?? img.versions.at(-1)?.url
              const isNew = reveal?.imageId === img.id && reveal.n === img.versions.at(-1)?.n && img.versions.at(-1)?.state === 'ready'
              const checked = selection.includes(img.id)
              return (
                <button
                  key={img.id}
                  type="button"
                  role="option"
                  aria-selected={selected === i}
                  aria-label={img.label}
                  title="Shift + bấm để chọn nhiều ảnh"
                  onClick={(event) => pickImage(i, event.shiftKey)}
                  {...stylex.props(sx.tile, blockedNow && selected === i && sx.tileBlocked, scores && !imgDrawing && isLow(readImageScore(img.score), scores.min) && scoreStyles.tileLow, selected === i && sx.tileSelected, checked && sx.tileChecked)}
                >
                  {imgUrl && <img src={imgUrl} alt="" {...stylex.props(sx.tileImage)} />}
                  <span {...stylex.props(sx.tileLabel)}>{img.index}</span>
                  {imgDrawing && <DevelopEffect state="drawing" compact hideCard />}
                  {isNew && <span {...stylex.props(sx.tileNew)}>MỚI</span>}
                  {legacyItems[i]?.status === 'approved' && <span {...stylex.props(sx.tileApproved)}><Icon name="tick-b" size={14} /></span>}
                  {(batch || checked) && <span role="checkbox" aria-checked={checked} aria-label={`Chọn ${img.label}`} {...stylex.props(sx.selbox, checked && sx.selboxOn)}>{checked && <Icon name="tick" size={11} />}</span>}
                </button>
              )
            })}
          </div>
          {batch ? (
            <div {...stylex.props(sx.bar)}>
              <span {...stylex.props(sx.countChip)}>{selection.length} ảnh đã chọn</span>
              <label htmlFor="redraw-batch" {...stylex.props(sx.hiddenLabel)}>Góp ý chung cho các ảnh đã chọn</label>
              <TextField id="redraw-batch" aria-label="Góp ý chung cho các ảnh đã chọn" placeholder="Góp ý chung, ví dụ: không có chữ trong ảnh, tông lạnh hơn" value={batchFeedback} onChange={setBatchFeedback} xstyle={sx.barInput} />
              <Button type="button" onClick={() => { setSelection([]); setBatchFeedback('') }} >Bỏ chọn</Button>
              <Button variant="primary" type="button" disabled={!online || pipelineBusy} onClick={redrawBatch} ><Icon name="refresh" size={15} />Vẽ lại {selection.length} ảnh · tốn {selection.length} ảnh</Button>
            </div>
          ) : editing.mode === 'edit' ? (
            <div {...stylex.props(sx.bar, sx.barEdit)}>
              <span {...stylex.props(sx.barIcon)}><Icon name="edit" size={16} /></span>
              <span {...stylex.props(sx.barText)}>
                <span {...stylex.props(sx.barTitle)}>{editTitle}</span>
                <span {...stylex.props(sx.barSub)}>Chỉ áp cho ảnh này · tốn 1 ảnh{quotaLine ? `, ${quotaLine}` : ''}</span>
              </span>
              <Button onClick={editing.reset} kbd="Esc">Huỷ sửa</Button>
              <Button variant="primary" onClick={submitEdit} aria-disabled={editing.changedCount === 0 || !online} icon={<Icon name="magicpen" size={15} />} kbd="⌘↵">Vẽ lại với prompt này</Button>
            </div>
          ) : (
            <div {...stylex.props(sx.bar)}>
              <span {...stylex.props(sx.barIcon)}><Icon name="gallery-edit" size={18} /></span>
              <label htmlFor="redraw" {...stylex.props(sx.hiddenLabel)}>Góp ý để vẽ lại ảnh này</label>
              <TextField id="redraw" aria-label="Góp ý để vẽ lại ảnh này" placeholder="Góp ý để vẽ lại, ví dụ: thêm tấm biển “Mall closes at 10PM”, ánh đèn xanh hơn" value={feedback} onChange={setFeedback} xstyle={sx.barInput} />
              <Button onClick={() => editing.setMode('edit')} tooltip="Sửa thẳng prompt đã gửi" disabled={isDrawing || !online} icon={<Icon name="edit" size={15} />}>Sửa prompt</Button>
              <Button disabled={isDrawing || pipelineBusy || !online} tooltip={`Hoạ sĩ sửa prompt theo góp ý rồi vẽ bản mới · tốn 1 ảnh${quotaLine ? ` · ${quotaLine}` : ''}`} onClick={redraw} icon={<Icon name="refresh" size={15} />} kbd="R">Vẽ lại</Button>
              <Button disabled={item.kind !== 'scene' || pipelineBusy || !online} tooltip="Cảnh này dùng ảnh của cảnh bên cạnh" onClick={() => { if (job && legacy) void review.mutate(job.id, legacy, 'skip') }}>Bỏ ảnh</Button>
              <ActionButton variant="primary" lockedReason={hasDrawing || pipelineBusy || !online ? 'Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ' : undefined} onClick={approve} icon={<Icon name="tick-b" size={15} />} kbd="A">Duyệt cả video &amp; dựng</ActionButton>
            </div>
          )}
          <LearnSlot learn={images?.learn} />
          <div {...stylex.props(sx.hints)}>
            <span {...stylex.props(sx.keyHint)}><Kbd>← →</Kbd>ảnh</span>
            <span {...stylex.props(sx.keyHint)}><Kbd>J K</Kbd>video trước / sau</span>
            <span {...stylex.props(sx.keyHint)}><Kbd>R</Kbd>vẽ lại</span>
            <span {...stylex.props(sx.keyHint)}><Kbd>E</Kbd>sửa prompt</span>
            <span {...stylex.props(sx.keyHint)}><Kbd>V</Kbd>so sánh</span>
            <span {...stylex.props(sx.keyHint)}><Kbd>A</Kbd>duyệt cả video</span>
            <span {...stylex.props(sx.hintSpacer)} />
            <span title="Duyệt xong, video tự chuyển sang dựng và mở video kế tiếp">Duyệt xong tự sang video kế</span>
          </div>
        </> : <div role="status">Chưa có ảnh để duyệt.</div>}
      </main>
      {item && (batch ? (
        <aside aria-label="Ảnh đã chọn" {...stylex.props(sx.rail, sx.railWide)}>
          <div {...stylex.props(sx.railHead)}>
            <span {...stylex.props(sx.railTitle)}>{selection.length} ảnh đã chọn</span>
            <span {...stylex.props(sx.railSub)}>Mỗi ảnh giữ prompt riêng. Góp ý chung được Hoạ sĩ ghép vào phần Cảnh của từng prompt rồi vẽ lần lượt.</span>
          </div>
          {items.filter(entry => selection.includes(entry.id)).map(entry => (
            <div key={entry.id} {...stylex.props(sx.refCard)}>
              {(entry.versions.find(candidate => candidate.n === entry.usedVersion)?.url ?? entry.legacyUrl) && <img alt="" src={entry.versions.find(candidate => candidate.n === entry.usedVersion)?.url ?? entry.legacyUrl} {...stylex.props(sx.refThumb)} />}
              <span {...stylex.props(sx.refText)}>
                <b {...stylex.props(sx.refRole)}>{entry.label}</b>
                <span {...stylex.props(sx.refSource)} title={entry.versions.find(candidate => candidate.n === entry.usedVersion)?.parts.find(part => part.kind === 'scene')?.text}>{entry.versions.find(candidate => candidate.n === entry.usedVersion)?.parts.find(part => part.kind === 'scene')?.text ?? ''}</span>
              </span>
            </div>
          ))}
          <span {...stylex.props(sx.verNote)}>Shift + bấm ảnh để thêm, bớt · Esc bỏ chọn</span>
        </aside>
      ) : (
        <ImageRail
          item={item}
          quota={live.data?.quota}
          providers={providers.data}
          mode={editing.mode}
          onMode={editing.setMode}
          selected={versionN}
          onSelectVersion={(n) => { setViewN(n); setCmp(false) }}
          onUseVersion={useVersion}
          edit={editing}
          online={online}
          candidates={candidates}
          onUpload={upload}
          uploading={uploading}
          score={scores ? <ScorePanel imageId={item.id} rawScore={item.score} min={scores.min} /> : undefined}
        />
      ))}
    </>}
    {allOpen && job && <Dialog title={`Vẽ lại cả ${items.length} ảnh?`} onClose={() => setAllOpen(false)} width={540} padBody={false} footer={<><Button onClick={() => setAllOpen(false)}>Huỷ</Button><Button variant="primary" onClick={() => { setAllOpen(false); if (job && images) void review.mutate(job.id, null, 'regenerateAll', allFeedback) }}>Vẽ lại {items.length} ảnh</Button></>}>
      <div {...stylex.props(sx.dialogBody)}>
        <div {...stylex.props(sx.dialogMeta)}><span>{job.source.title}</span><span {...stylex.props(sx.small)}>{channel?.name}</span></div>
        <div {...stylex.props(sx.strip)}>{items.map(img => <span key={img.id} {...stylex.props(sx.tile)}>{(img.legacyUrl ?? img.versions.at(-1)?.url) && <img alt={img.label} src={img.legacyUrl ?? img.versions.at(-1)?.url ?? ''} {...stylex.props(sx.tileImage)} />}</span>)}</div>
        <p>Dùng {items.length} ảnh trong hạn mức {providerName(usedVersion?.provider ?? 'chatgpt', providers.data)} hôm nay</p>
        <p {...stylex.props(sx.small)}>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
        <TextArea label="Góp ý chung cho cả bộ" rows={3} showCount={false} maxLength={-1} value={allFeedback} onChange={event => setAllFeedback(event.currentTarget.value)} />
      </div>
    </Dialog>}
  </div>
}
