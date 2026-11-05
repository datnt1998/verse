// Duyệt ảnh — duyệt cả video bằng phím, mỗi ảnh có phiên bản, ảnh tham chiếu và
// prompt đã gửi; góp ý hoặc sửa prompt rồi vẽ lại đúng ảnh (artboard Review, E1).
// Vẽ lại đi qua §26 (`api/images`), duyệt/bỏ ảnh vẫn dùng thao tác cũ của màn.
import { useEffect, useMemo, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Chip, Dialog, Icon, Kbd, Segmented, TextArea, TextField, confirm, toast } from '@noverse/ui'
import { imageFailureCopy } from '../../i18n/errorsCopy'
import { listProviders } from '../../api/ai'
import { listChannels } from '../../api/channels'
import { request } from '../../api/request'
import { ApiError } from '../../api/request'
import {
  cancelImageVersion, redrawImages, tooManyRefs, uploadRefImage, useImageVersion, useJobImages,
  type ImageRef, type PromptPartKind, type RedrawBody,
} from '../../api/images'
import { redrawAll } from '../../api/workbench'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { Orb, SystemLoading, DevelopEffect } from '../../components'
import { ActionButton } from '../../components/ActionButton'
import { useOnline } from '../../data/status'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { navigate, paths, type Route } from '../../router'
import { imageURL, versionURL, isBlocked as blocked, isDrawing as drawing, reviewLabel as label, useReview } from './useReview'
import { useReviewKeys } from './useReviewKeys'
import { ScoreBadge, ScoreBanner, ScorePanel } from '../../components/autonomy/ScoreParts'
import { isLow, summarizeScores } from '../../components/autonomy/scoreSummary'
import { scoreStyles } from '../../components/autonomy/score.stylex'
import { APPROVE_DEFAULT, readImageScore } from '../../api/autonomy'
import { MReviewScreen } from './mobile/MReviewScreen'
import { ImageRail, type RefCandidate } from './ImageRail'
import { toReviewItems, defaultVersion, stageWidthVar, providerName, resetAtLabel, type ReviewItemView } from './imageView'
import { useImageEditing } from './useImageEditing'
import { ImageViewer } from '../../components/image-viewer/ImageViewer'
import { sx } from './review.stylex'

const clock = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`

/** Ảnh tham chiếu gửi kèm: ảnh đã lưu gửi lại bằng id, ảnh mới gửi bằng nguồn. */
function toRedrawRefs(refs: ImageRef[]) {
  return refs.map((ref) => {
    if (ref.source.type === 'upload') return { upload: ref.source.imageId ?? ref.id, role: ref.role }
    if (ref.source.type === 'rival_frame' && ref.source.videoId != null && ref.source.atSec != null) {
      return { rivalFrame: { videoId: ref.source.videoId, atSec: ref.source.atSec }, role: ref.role }
    }
    if (ref.source.type === 'video_image' && ref.source.imageId) return { videoImageId: ref.source.imageId, role: ref.role }
    return { id: ref.id, role: ref.role }
  })
}

/** ≤767: màn dọc riêng của artboard MReview; >767 giữ nguyên bố cục ba cột. */
export function ReviewScreen({ route }: { route: Extract<Route, { name: 'review' }> }) {
  const phone = useMediaQuery('(max-width: 767px)')
  return phone ? <MReviewScreen route={route} /> : <DesktopReviewScreen route={route} />
}

function DesktopReviewScreen({ route }: { route: Extract<Route, { name: 'review' }> }) {
  const review = useReview()
  const online = useOnline()
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const counts = useResource(['review', 'counts'], ({ signal }) => request<{ approvedToday: number | null }>('/api/clone/counts', { signal }))
  const address = route
  const jobId = address?.video
  const [view, setView] = useState<'full' | 'cutout'>('full')
  const [imageFailed, setImageFailed] = useState(false)
  const [feedback, setFeedback] = useState('')
  const [allOpen, setAllOpen] = useState<{ jobId: string; revision: number; count: number } | null>(null)
  const [allBusy, setAllBusy] = useState(false)
  const [allError, setAllError] = useState('')
  const [allFeedback, setAllFeedback] = useState('')
  const [viewN, setViewN] = useState<number | null>(null)
  const [cmp, setCmp] = useState(false)
  const [selection, setSelection] = useState<string[]>([])
  const [batchFeedback, setBatchFeedback] = useState('')
  const [reveal, setReveal] = useState<{ imageId: string; n: number } | null>(null)
  const [uploading, setUploading] = useState(false)
  const busyRef = useRef(false)
  const seenReady = useRef<Record<string, number>>({})
  const channelFilter = address?.channel
  const jobs = useMemo(() => (review.jobs ?? []).filter(j => !channelFilter || channelFilter === 'all' || j.channelId === channelFilter), [review.jobs, channelFilter])
  const job = jobId ? jobs.find(j => j.id === jobId) : jobs[0]
  useEffect(() => {
    if (!review.jobs || review.error || !jobId || job || review.pending[jobId]) return
    toast.err('Video đã chọn không có trong danh sách chờ duyệt. Đã trở về danh sách hiện tại.')
    navigate(paths.review({ channel: channelFilter }), true)
  }, [review.jobs, review.error, review.pending, jobId, job, channelFilter])
  const images = job ? review.images[job.id] : undefined
  useEffect(() => { setAllOpen(null); setAllFeedback(''); setAllError('') }, [job?.id])
  const legacyItems = images?.items ?? []
  const live = useJobImages(job?.id ?? null)
  const items = useMemo(() => toReviewItems(legacyItems, live.data?.images, live.progress), [legacyItems, live.data, live.progress])
  const requestedIndex = address?.img ? items.findIndex(candidate => candidate.id === address.img) : 0
  const selected = Math.max(0, requestedIndex)
  const item = requestedIndex < 0 ? undefined : items[selected]
  const legacy = legacyItems.find(candidate => candidate.id === item?.id)
  const viewedJobRef = useRef(job?.id)
  const viewedImageRef = useRef(item?.id)
  viewedJobRef.current = job?.id
  viewedImageRef.current = item?.id
  const setIndex = (at: number) => {
    const target = items[at]
    if (job && target) navigate(paths.review({ channel: channelFilter, video: job.id, img: target.id }))
  }
  useEffect(() => {
    if (!job || !images || !address?.img || requestedIndex >= 0) return
    toast.err('Ảnh đã chọn không còn trong bộ ảnh. Đã chuyển về ảnh còn lại.')
    navigate(paths.review({ channel: channelFilter, video: job.id, img: items[0]?.id }), true)
  }, [job?.id, images, address?.img, requestedIndex, channelFilter, items])
  const channel = channels.data?.channels.find(c => c.id === job?.channelId)
  const scores = channel ? summarizeScores(legacyItems.map((candidate, i) => ({ id: candidate.id, label: label(candidate, i), drawing: drawing(candidate), score: candidate.score })), channel.autoApprove) : null
  const neighbour = [...legacyItems.slice(0, selected)].reverse().find(candidate => candidate.kind === 'scene' && candidate.status !== 'skipped') ?? legacyItems.find((candidate, i) => i > selected && candidate.kind === 'scene' && candidate.status !== 'skipped')
  const pipelineBusy = !!job && !!review.pending[job.id]

  const usedVersion = item?.versions.find(version => version.n === item.usedVersion && version.state !== 'cancelled') ?? defaultVersion(item?.versions)
  const drawingVersion = item?.versions.find(version => version.state === 'drawing')
  const latestVersion = defaultVersion(item?.versions)
  const requestedVersionN = viewN ?? latestVersion?.n ?? item?.usedVersion ?? 1
  const version = item?.versions.find(candidate => candidate.n === requestedVersionN && candidate.state !== 'cancelled') ?? latestVersion ?? usedVersion
  const versionN = version?.n ?? requestedVersionN
  const previousVersion = item?.versions.find(candidate => candidate.n === versionN - 1)
  const isDrawing = !!drawingVersion
  const failedVersion = version?.state === 'failed' ? version : undefined
  const isFailed = !isDrawing && !!failedVersion
  // Sửa/góp ý bám đúng bản đang hiện: thẻ lỗi giữ prompt + ảnh tham chiếu của
  // bản lỗi, còn lại giữ bản đang xem (không phải bản đang dùng của video).
  const editingBase = isFailed ? failedVersion : version
  const editing = useImageEditing(job && item ? `${job.id}:${item.id}` : undefined, editingBase)
  const legacyParts = !editing.base || !!editing.base.partsLegacy || !editing.base.parts.length
  const liveEditSource = editing.base && item?.versions.find(candidate => candidate.n === editing.base!.n)
  const editSourceStale = editing.mode !== 'view' && (editing.base
    ? !liveEditSource || (liveEditSource.state !== 'ready' && liveEditSource.state !== 'failed')
    : !!version)
  const editBlocked = isDrawing || pipelineBusy || !online || editSourceStale
  const refTarget = useRef({ jobId: job?.id, imageId: item?.id, source: editing.base?.n, blocked: editBlocked })
  refTarget.current = { jobId: job?.id, imageId: item?.id, source: editing.base?.n, blocked: editBlocked }
  const batch = selection.length > 0
  const readyVersion = item?.versions.reduce((latest, candidate) => candidate.state === 'ready' ? Math.max(latest, candidate.n) : latest, 0) ?? 0
  const justRevealed = !!reveal && reveal.imageId === item?.id && reveal.n === versionN && !cmp
  const shownVersion = cmp && previousVersion ? previousVersion : version
  const shownImage = view === 'cutout' && job && item
    ? versionURL(job.id, legacy ?? { id: item.id, versions: [] }, shownVersion?.n ?? versionN, 'cutout')
    : shownVersion?.url ?? shownVersion?.previewUrl ?? (cmp ? undefined : usedVersion?.url) ?? item?.legacyUrl ?? (legacy ? imageURL(job!.id, legacy, view) : undefined)
  const frameTime = item?.startSec != null && item.endSec != null ? `${clock(item.startSec)}–${clock(item.endSec)}` : item?.kind === 'cover' && usedVersion?.width ? `${usedVersion.width}×${usedVersion.height}` : ''
  const versionTag = isDrawing ? `đang vẽ bản ${drawingVersion!.n}` : !version ? 'chưa có bản vẽ dùng được' : cmp && previousVersion ? `bản ${previousVersion.n} (so sánh)` : `bản ${versionN}${versionN === item?.usedVersion ? '' : ' · chưa dùng'}`
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
    if (editing.mode === 'view' || !editing.base) return
    setViewN(editing.base.n)
    setCmp(false)
  }, [editing.mode, editing.base?.n])
  useEffect(() => {
    if (!job || !item) return
    const key = `${job.id}:${item.id}`
    const previous = seenReady.current[key]
    if (isDrawing) {
      seenReady.current[key] ??= readyVersion
      return
    }
    seenReady.current[key] = readyVersion
    if (previous === undefined || readyVersion <= previous) {
      setReveal(null)
      return
    }
    if (editing.mode !== 'view') return
    setViewN(readyVersion)
    setCmp(false)
    setReveal({ imageId: item.id, n: readyVersion })
    const timer = window.setTimeout(() => setReveal(null), 2600)
    return () => window.clearTimeout(timer)
  }, [job?.id, item?.id, isDrawing, readyVersion, editing.mode])

  const selectJob = (id: string) => { navigate(paths.review({ channel: channelFilter, video: id })); setView('full'); setFeedback(''); setSelection([]) }
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
    const timer = window.setInterval(() => void review.loadImages(pollingId, true), 2500)
    return () => clearInterval(timer)
  }, [pollingId, hasDrawing, review.loadImages])
  useEffect(() => {
    if (!job) return
    const next = jobs[jobs.findIndex(j => j.id === job.id) + 1]
    for (const image of next ? review.images[next.id]?.items ?? [] : []) { const url = imageURL(next!.id, image); if (url) { const preload = new Image(); preload.src = url } }
  }, [job, jobs, review.images])

  const refresh = () => { if (job) { void live.reload(); void review.loadImages(job.id, true) } }
  const closeAll = () => { if (!allBusy && !busyRef.current) setAllOpen(null) }
  const submitAll = async () => {
    if (!job || !allOpen || allOpen.jobId !== job.id || busyRef.current || !online) return
    busyRef.current = true
    setAllBusy(true)
    setAllError('')
    try {
      await redrawAll(allOpen.jobId, allOpen.revision, allOpen.count, allFeedback)
      setAllOpen(null)
      setAllFeedback('')
      toast.ok(`Đã gửi Hoạ sĩ · vẽ lại ${allOpen.count} ảnh`)
      void review.loadImages(allOpen.jobId, true)
      void live.reload()
    } catch (error) {
      setAllError(error instanceof Error ? error.message : 'Chưa gửi được yêu cầu vẽ lại. Vui lòng thử lại.')
    } finally {
      busyRef.current = false
      setAllBusy(false)
    }
  }

  const runRedraw = async (body: Omit<RedrawBody, 'imageIds'> & { imageIds?: string[] }) => {
    if (!job || !online || busyRef.current) return false
    busyRef.current = true
    try {
      const imageIds = body.imageIds ?? (item ? [item.id] : [])
      if (!imageIds.length) return false
      const accepted = await confirm({
        title: `Vẽ lại ${imageIds.length} ảnh?`,
        body: <p>Tốn {imageIds.length} ảnh trong hạn mức hôm nay{live.data?.quota ? ` · ${live.data.quota.providers.map(provider => `${provider.label} còn ${provider.left}/${provider.total}`).join(' · ')}` : ''}. Ảnh cũ được giữ tới khi bản mới xong; bản mới cần bạn duyệt lại.</p>,
        confirmLabel: 'Vẽ lại',
        onConfirm: async () => {
          try {
            await redrawImages(job.id, { ...body, imageIds })
          } catch (error) {
            const limit = tooManyRefs(error)
            if (limit !== null) throw new Error(`Nhà cung cấp nhận tối đa ${limit} ảnh tham chiếu.`)
            throw error
          }
        },
      })
      if (accepted) {
        setFeedback('')
        refresh()
        toast.ok(`Đã gửi Hoạ sĩ · vẽ lại ${imageIds.length} ảnh`)
      }
      return accepted
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Chưa vẽ lại được. Vui lòng thử lại.')
      return false
    } finally { busyRef.current = false }
  }

  /** Góp ý → Hoạ sĩ sửa phần Cảnh của đúng bản nguồn đang sửa rồi vẽ (giữ như cũ, nay có phiên bản). */
  const redraw = () => {
    const source = editing.base
    if (!item || editBlocked) return
    // Không còn bản dùng được (ví dụ chỉ có bản đã huỷ): vẽ mới theo chính sách
    // thường của máy chủ, không gửi baseVersion và không sửa prompt của bản huỷ.
    const baseVersion = source && source.state !== 'cancelled' ? source.n : undefined
    void runRedraw({ ...(baseVersion !== undefined ? { baseVersion } : {}), ...(feedback ? { feedback } : {}) })
  }

  /** Vẽ lại với prompt đã sửa: chỉ gửi phần đã đổi + ảnh tham chiếu nếu đổi. */
  const submitEdit = () => {
    const source = editing.base
    if (!item || editBlocked || editing.changedCount === 0) return
    const body: Omit<RedrawBody, 'imageIds'> = source ? { baseVersion: source.n } : {}
    if (Object.keys(editing.changed).length) body.parts = editing.changed as Partial<Record<PromptPartKind, string>>
    const baseRefs = source?.refs ?? []
    const refsChanged = editing.changedCount - Object.keys(editing.changed).length > 0
    if (refsChanged || baseRefs.length !== editing.refs.length) body.refs = toRedrawRefs(editing.refs)
    void runRedraw(body).then(accepted => { if (accepted) editing.reset() })
  }

  /** R khi đang có nháp sửa prompt phải gửi nháp đó, không vẽ lại bằng góp ý rồi bỏ nháp. */
  const redrawShortcut = () => {
    if (editing.mode === 'edit' || (editing.mode === 'raw' && editing.changedCount > 0)) { submitEdit(); return }
    redraw()
  }

  /** Thẻ lỗi: vẽ bằng Grok đúng bản lỗi; máy chủ giữ nguyên prompt + ảnh tham chiếu đã lưu. */
  const retryWithGrok = () => {
    if (!item || !failedVersion || editBlocked) return
    void runRedraw({ provider: 'grok', baseVersion: failedVersion.n })
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
      .then(() => { if (viewedJobRef.current === job.id && viewedImageRef.current === item.id) setViewN(n); refresh() })
      .catch((error: unknown) => toast.err(error instanceof Error ? error.message : 'Chưa đổi được bản dùng cho video.'))
  }

  const upload = (file: File) => {
    if (refTarget.current.blocked) return
    const target = refTarget.current
    setUploading(true)
    void uploadRefImage(file)
      .then((id) => {
        const current = refTarget.current
        if (current.blocked || current.jobId !== target.jobId || current.imageId !== target.imageId || current.source !== target.source) {
          toast('Đã tải ảnh lên nhưng lựa chọn đã thay đổi; hãy thêm ảnh vào bản đang xem.')
          return
        }
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
    if (allOpen) closeAll()
  }
  useReviewKeys({
    nextJob: () => moveJob(1), previousJob: () => moveJob(-1), nextImage: () => moveImage(1), previousImage: () => moveImage(-1),
    redraw: redrawShortcut, approve, editPrompt: () => { if (item && !isDrawing && !pipelineBusy && online && !legacyParts && version?.state !== 'cancelled') editing.setMode('edit') }, compare: toggleCompare,
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
    void runRedraw({ imageIds: selection, ...(batchFeedback ? { feedback: batchFeedback } : {}) }).then(accepted => {
      if (accepted) { setSelection([]); setBatchFeedback('') }
    })
  }

  const approved = legacyItems.filter(i => i.status === 'approved').length
  const usedImage = usedVersion?.url ?? (legacy && job ? imageURL(job.id, legacy, view) : undefined)
  const editTitle = editing.changedCount
    ? `Đã sửa ${editing.changedCount} chỗ trong prompt ${item?.label.toLocaleLowerCase('vi') ?? ''}`
    : `Sửa prompt ${item?.label.toLocaleLowerCase('vi') ?? ''}`
  const quotaLine = live.data?.quota
    ? `${live.data.quota.providers.map(entry => `${entry.label} còn ${entry.left}/${entry.total}`).join(' · ')}`
    : ''
  const detail = drawingVersion
    ? item?.progress
      ? item.progress.phase === 'sending'
        ? `${clock(item.progress.elapsedSec)} · đang gửi chỉ dẫn và ${usedVersion?.refs.length ?? 0} ảnh tham chiếu`
        : item.progress.phase === 'receiving'
          ? `${clock(item.progress.elapsedSec)} · đang nhận ảnh, sắp xong`
          : `${clock(item.progress.elapsedSec)} · thường khoảng 40 giây`
      : '0:01 · đang gửi chỉ dẫn'
    : undefined
  const step = !item?.progress ? 1 : item.progress.phase === 'sending' ? 0 : item.progress.phase === 'receiving' ? 2 : 1
  return <div {...stylex.props(sx.root)}>
    <aside aria-label="Video chờ duyệt" {...stylex.props(sx.side)}>
      <div {...stylex.props(sx.group)}><span>Chờ bạn duyệt</span><span>{jobs.length}</span></div>
      {jobs.map(j => <button key={j.id} type="button" aria-pressed={j.id === job?.id} onClick={() => selectJob(j.id)} {...stylex.props(sx.row, j.id === job?.id && sx.selectedRow)}>{j.source.thumbnailUrl ? <img alt="" src={j.source.thumbnailUrl} {...stylex.props(sx.thumb)} /> : <span {...stylex.props(sx.thumb)} /> }<span {...stylex.props(sx.rowText)}><span title={j.source.title} {...stylex.props(sx.truncate)}>{j.source.title}</span><span {...stylex.props(sx.small, sx.truncate)}>{channels.data?.channels.find(c => c.id === j.channelId)?.name} · {j.subline}</span></span></button>)}
      {review.jobs && !jobs.length && <span {...stylex.props(sx.small)}>Không còn video nào chờ duyệt.</span>}
      {counts.data?.approvedToday != null && <div {...stylex.props(sx.footer)}><span>Duyệt xong hôm nay</span><span>{counts.data.approvedToday} video</span></div>}
    </aside>
    {!review.jobs ? <main {...stylex.props(sx.empty)}>{review.error ? <div role="alert"><span>Chưa tải được video chờ duyệt. </span><ActionButton size="sm" onClick={() => void review.loadJobs()}>Thử lại</ActionButton></div> : <SystemLoading label="Đang tải video chờ duyệt…" />}</main> : !job ? <main {...stylex.props(sx.empty)}><div role="status" {...stylex.props(sx.emptyInner)}><span aria-hidden="true" {...stylex.props(sx.emptyOrbs)}><Orb of="artist" size={30} /><span {...stylex.props(sx.emptyQa)}><Orb of="qa" size={24} /></span></span><strong {...stylex.props(sx.emptyTitle)}>Đã duyệt hết</strong><span {...stylex.props(sx.emptyText)}>{review.building > 0 ? `Kiểm định đang dựng ${review.building} video vừa duyệt. Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.` : 'Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.'}</span><a href={`#${paths.workbench()}`} {...stylex.props(sx.emptyCta)}>Xem tiến độ trên Bàn dựng</a></div></main> : <>
      <ImageViewer inline mode="review" stage={<main className={`vv-wide vv-wide--grid ${stylex.props(sx.main).className}`}>
        <div {...stylex.props(sx.heading)}>
          <h1 title={job.source.title} {...stylex.props(sx.title)}>{job.source.title}</h1>
          {channel && <Chip size="sm" label={channel.name} />}
          <Chip size="sm" label="Đã duyệt" value={`${approved} / ${items.length}`} />
          <ActionButton size="sm" disabled={!items.length || !images?.manifest || hasDrawing || pipelineBusy || allBusy || !online} disabledReason="Chờ ảnh đang vẽ xong" onClick={() => { if (images?.manifest) { setAllError(''); setAllOpen({ jobId: job.id, revision: images.manifest.revision, count: items.length }) } }}>Vẽ lại cả {items.length} ảnh</ActionButton>
          <a href={`#${paths.workbench({ video: job.id })}`} {...stylex.props(sx.link)}>Mở Bàn dựng</a>
        </div>
        {!items.length ? <div role="status" {...stylex.props(sx.empty)}>{review.imageErrors[job.id] ? <><span>Chưa tải được ảnh. </span><Button type="button" onClick={() => void review.loadImages(job.id, true)}>Thử lại</Button></> : 'Đang tải ảnh…'}</div> : item ? <>
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
                      {failedVersion!.error ? imageFailureCopy[failedVersion!.error.reason] : imageFailureCopy.provider}
                      {failedVersion!.error?.reason === 'quota' ? ` (${resetAtLabel(failedVersion!.error.retryAt)})` : ''}
                      {' '}Bản {item.usedVersion} vẫn được giữ; chỉ dẫn bạn sửa vẫn còn.
                    </span>
                    <div {...stylex.props(sx.failedActions)}>
                      <Button variant="primary" size="sm" type="button" disabled={editBlocked} onClick={retryWithGrok} >
                        Vẽ bằng {providerName('grok', providers.data)}{live.data?.quota?.fallback ? ` · còn ${live.data.quota.fallback.left} ảnh` : ''}</Button>
                      <Button size="sm" type="button" disabled={editBlocked || legacyParts} onClick={() => editing.setMode('edit')} >Sửa prompt</Button>
                    </div>
                  </div>
                </>
              ) : justRevealed && shownImage ? (
                <DevelopEffect key={shownImage} state="done"><img alt={item.label} src={shownImage} onError={() => setImageFailed(true)} {...stylex.props(sx.stageImage, view === 'cutout' && sx.heroCutout)} /></DevelopEffect>
              ) : (
                !blockedNow && shownImage && !imageFailed && <img alt={cmp && previousVersion ? `${item.label} bản ${previousVersion.n}` : item.label} src={shownImage} onError={() => setImageFailed(true)} {...stylex.props(sx.stageImage, view === 'cutout' && sx.heroCutout)} />
              )}
              {blockedNow && legacy && neighbour && <img alt="" src={imageURL(job.id, neighbour, view)} {...stylex.props(sx.stageImage, sx.dimmed)} />}
              {blockedNow && <div role="status" {...stylex.props(sx.blockedCard)}><strong {...stylex.props(sx.blockedTitle)}><Icon name="danger" size={16} />Bị chặn{neighbour ? ` · dùng ảnh ${label(neighbour, legacyItems.indexOf(neighbour)).toLocaleLowerCase('vi')}` : ''}</strong><span {...stylex.props(sx.blockedText)}>Bộ lọc nội dung đã chặn ảnh này. Video dùng ảnh cảnh bên cạnh để tiếp tục.</span></div>}
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
          {scores && channel && <ScoreBanner summary={scores} autoApprove={channel.autoApprove} channelName={channel.name} onJump={(id) => { const at = items.findIndex(candidate => candidate.id === id); if (at >= 0) { setIndex(at); setView('full') } }} />}
          <div role="listbox" aria-label="Ảnh của video" {...stylex.props(sx.strip)}>
            {items.map((img, i) => {
              const imgDrawing = img.versions.some(candidate => candidate.state === 'drawing')
              const imgUrl = img.versions.find(candidate => candidate.n === img.usedVersion)?.url ?? img.legacyUrl ?? img.versions.at(-1)?.url
              const isNew = reveal?.imageId === img.id && reveal.n === img.versions.at(-1)?.n && img.versions.at(-1)?.state === 'ready'
              const checked = selection.includes(img.id)
              const imgScore = scores ? readImageScore(img.score) : null
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
                  <span {...stylex.props(sx.tileLabel)}>{img.kind === 'cover' ? 0 : img.index}</span>
                  {imgDrawing && <DevelopEffect state="drawing" baseUrl={imgUrl ?? undefined} compact hideCard />}
                  {isNew && <span {...stylex.props(sx.tileNew)}>MỚI</span>}
                  {scores && imgScore && !imgDrawing && !batch && !checked && <ScoreBadge value={imgScore.value} low={isLow(imgScore, scores.min)} corner />}
                  {img.status === 'approved' && <span {...stylex.props(sx.tileApproved)}><Icon name="tick-b" size={14} label="Đã duyệt" /></span>}
                  {(batch || checked) && <span role="checkbox" aria-checked={checked} aria-label={`Chọn ${img.label}`} {...stylex.props(sx.selbox, checked && sx.selboxOn)}>{checked && <Icon name="tick" size={11} />}</span>}
                </button>
              )
            })}
          </div>
          {batch ? (
            <div {...stylex.props(sx.bar)}>
              <span {...stylex.props(sx.countChip)}>{selection.length} ảnh đã chọn</span>
              <label htmlFor="redraw-batch" {...stylex.props(sx.hiddenLabel)}>Góp ý chung cho các ảnh đã chọn</label>
              <TextField id="redraw-batch" aria-label="Góp ý chung cho các ảnh đã chọn" placeholder="Góp ý chung, ví dụ: không có chữ trong ảnh, tông lạnh hơn" value={batchFeedback} onChange={setBatchFeedback} xstyle={sx.barInput} frameStyle={sx.barInputFrame} inputStyle={sx.barInputText} />
              <Button type="button" onClick={() => { setSelection([]); setBatchFeedback('') }} >Bỏ chọn</Button>
              <Button variant="primary" type="button" disabled={!online || pipelineBusy} onClick={redrawBatch} ><Icon name="refresh" size={15} />Vẽ lại {selection.length} ảnh · tốn {selection.length} ảnh</Button>
            </div>
          ) : editing.mode === 'edit' ? (
            <div {...stylex.props(sx.bar, sx.barEdit)}>
              <span {...stylex.props(sx.barIcon, sx.barEditIcon)}><Icon name="edit" size={16} /></span>
              <span {...stylex.props(sx.barText)}>
                <span {...stylex.props(sx.barTitle)}>{editTitle}</span>
                <span title={`Chỉ áp cho ảnh này · tốn 1 ảnh${quotaLine ? `, ${quotaLine} hôm nay` : ''}`} {...stylex.props(sx.barSub)}>Chỉ áp cho ảnh này · tốn 1 ảnh{quotaLine ? `, ${quotaLine} hôm nay` : ''}</span>
              </span>
              <Button onClick={editing.reset} kbd="Esc">Huỷ sửa</Button>
              <Button variant="primary" onClick={submitEdit} aria-disabled={editing.changedCount === 0 || editBlocked} tooltip={editSourceStale ? 'Bản nguồn đã thay đổi; huỷ sửa để chọn lại. Bản nháp vẫn được giữ.' : undefined} icon={<Icon name="magicpen" size={15} />} kbd="⌘↵">Vẽ lại với prompt này</Button>
            </div>
          ) : (
            <div {...stylex.props(sx.bar)}>
              <span {...stylex.props(sx.barIcon)}><Icon name="gallery-edit" size={18} /></span>
              <label htmlFor="redraw" {...stylex.props(sx.hiddenLabel)}>Góp ý để vẽ lại ảnh này</label>
              <TextField id="redraw" aria-label="Góp ý để vẽ lại ảnh này" placeholder="Góp ý để vẽ lại, ví dụ: thêm biển “Đóng cửa lúc 22 giờ”, ánh đèn xanh hơn" value={feedback} onChange={setFeedback} xstyle={sx.barInput} frameStyle={sx.barInputFrame} inputStyle={sx.barInputText} />
              <Button variant="quiet" onClick={() => editing.setMode('edit')} tooltip={legacyParts ? 'Bản cũ chưa lưu prompt theo phần nên không sửa được; bạn vẫn có thể góp ý để vẽ lại.' : 'Sửa thẳng prompt đã gửi'} disabled={isDrawing || !online || legacyParts} icon={<Icon name="edit" size={15} />}>Sửa prompt</Button>
              <Button disabled={isDrawing || pipelineBusy || !online} tooltip={`Hoạ sĩ sửa prompt theo góp ý rồi vẽ bản mới · tốn 1 ảnh${quotaLine ? ` · ${quotaLine} hôm nay` : ''}`} onClick={redraw} icon={<Icon name="refresh" size={15} />} kbd="R">Vẽ lại</Button>
              <Button variant="quiet" disabled={item.kind !== 'scene' || pipelineBusy || !online} tooltip="Cảnh này dùng ảnh của cảnh bên cạnh" onClick={() => { if (job && legacy) void review.mutate(job.id, legacy, 'skip') }}>Bỏ ảnh</Button>
              <ActionButton variant="primary" lockedReason={hasDrawing || pipelineBusy || !online ? 'Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ' : undefined} onClick={approve} icon={<Icon name="tick-b" size={15} />} kbd="A">Duyệt cả video &amp; dựng</ActionButton>
            </div>
          )}
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
      </main>}
      side={item && (batch ? (
        <aside aria-label="Ảnh đã chọn" {...stylex.props(sx.rail, sx.railWide)}>
          <div {...stylex.props(sx.railHead)}>
            <span {...stylex.props(sx.railTitle)}>{selection.length} ảnh đã chọn</span>
            <span {...stylex.props(sx.railSub)}>Mỗi ảnh giữ chỉ dẫn riêng. Góp ý chung được Hoạ sĩ ghép vào phần Cảnh của từng chỉ dẫn rồi vẽ lần lượt.</span>
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
          selected={editing.mode === 'view' ? versionN : editing.base?.n ?? versionN}
          onSelectVersion={(n) => { editing.reset(); setViewN(n); setCmp(false) }}
          onUseVersion={useVersion}
          edit={editing}
          online={online}
          mutationBlocked={isDrawing || pipelineBusy || editSourceStale}
          candidates={candidates}
          onUpload={upload}
          uploading={uploading}
          score={scores || shownVersion?.scoreIssue ? <ScorePanel imageId={`${item.id}:${shownVersion?.n}`} rawScore={shownVersion?.score} min={scores?.min ?? APPROVE_DEFAULT} issue={shownVersion?.scoreIssue} /> : undefined}
        />
      ))} />
    </>}
    {allOpen && job && allOpen.jobId === job.id && <Dialog
      label={`Vẽ lại cả ${allOpen.count} ảnh?`}
      onClose={closeAll}
      width={540}
      padBody={false}
      footer={<><span {...stylex.props(sx.dialogEsc)}><Kbd>Esc</Kbd> đóng</span><Button disabled={allBusy} onClick={closeAll}>Huỷ</Button><Button variant="primary" pending={allBusy} disabled={allBusy || !online} onClick={() => void submitAll()}><Icon name="gallery-edit" size={15} />Vẽ lại {allOpen.count} ảnh</Button></>}
    >
      <div {...stylex.props(sx.dialogBody)}>
        <div {...stylex.props(sx.dialogHead)}>
          <span {...stylex.props(sx.dialogIcon)}><Icon name="gallery-edit" size={20} /></span>
          <div {...stylex.props(sx.dialogHeadText)}>
            <h2 {...stylex.props(sx.dialogTitle)}>Vẽ lại cả {allOpen.count} ảnh?</h2>
            <span {...stylex.props(sx.dialogSub)}>{job.source.title}{channel?.name ? ` · ${channel.name}` : ''}</span>
          </div>
          <Button variant="icon" size="sm" aria-label="Đóng (Esc)" onClick={closeAll} xstyle={sx.dialogClose}><Icon name="close" size={16} /></Button>
        </div>
        <div {...stylex.props(sx.dialogThumbs)}>{items.map(img => <span key={img.id} {...stylex.props(sx.dialogThumb)}>{(img.legacyUrl ?? img.versions.at(-1)?.url) && <img alt={img.label} src={img.legacyUrl ?? img.versions.at(-1)?.url ?? ''} {...stylex.props(sx.dialogThumbImage)} />}</span>)}</div>
        <div {...stylex.props(sx.dialogQuota)}>
          <Icon name="gallery" size={16} />
          <span>Dùng <b {...stylex.props(sx.dialogStrong)}>{allOpen.count} ảnh</b> trong hạn mức {usedVersion?.provider ? providerName(usedVersion.provider, providers.data) : 'AI'} hôm nay{quotaLine ? ` · ${quotaLine}` : ''}</span>
        </div>
        {images?.manifest?.scenePrompt?.name && images.manifest.coverPrompt?.name && (
          <div {...stylex.props(sx.dialogStyle)}>
            <Icon name="magicpen" size={16} {...stylex.props(sx.dialogStyleIcon)} />
            <span {...stylex.props(sx.dialogStyleText)}>Ảnh cảnh vẽ theo <b {...stylex.props(sx.dialogStrong)}>{images.manifest.scenePrompt.name}</b>, ảnh bìa theo <b {...stylex.props(sx.dialogStrong)}>{images.manifest.coverPrompt.name}</b>, theo prompt của video này.</span>
            <a href={`#${paths.workbench({ video: job.id })}`} {...stylex.props(sx.dialogLink)}>Đổi ở Bàn dựng</a>
          </div>
        )}
        <p {...stylex.props(sx.dialogInfo)}>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
        <label htmlFor="redraw-all-feedback" {...stylex.props(sx.dialogFieldLabel)}>Góp ý chung cho cả bộ <span {...stylex.props(sx.dialogOptional)}>(tuỳ chọn)</span></label>
        <TextArea id="redraw-all-feedback" placeholder="Ví dụ: tông lạnh hơn, ít người hơn" rows={2} showCount={false} maxLength={-1} disabled={allBusy} value={allFeedback} onChange={event => setAllFeedback(event.currentTarget.value)} />
        {allError && <p role="alert">{allError}</p>}
      </div>
    </Dialog>}
  </div>
}
