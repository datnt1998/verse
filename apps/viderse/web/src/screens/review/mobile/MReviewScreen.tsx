// Artboard MReview — màn duyệt ảnh của điện thoại (≤767). Thay hẳn danh sách +
// chi tiết xếp chồng của bản desktop: một ảnh lớn, dải ảnh, dải phiên bản, prompt,
// và thanh hành động dính đáy (Vẽ lại · Bỏ ảnh này · Duyệt cả video & dựng).
// Mọi thao tác gọi đúng API duyệt ảnh đang có; không có nhánh giả lập.
import { useEffect, useMemo, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, InitialsAvatar, Segmented, Sheet, Spinner, TextArea, toast, useAsyncAction, useMobileScroll } from '@noverse/ui'
import { ActionButton } from '../../../components/ActionButton'
import { listChannels } from '../../../api/channels'
import { readImageScore } from '../../../api/autonomy'
import { redrawImages, useImageVersion, type RedrawBody } from '../../../api/images'
import { request } from '../../../api/request'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { useOnline } from '../../../data/status'
import { navigate, paths, type Route } from '../../../router'
import { DevelopEffect, Orb } from '../../../components'
import { LearnSlot } from '../../../components/learn/LearnSlot'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { isLow } from '../../../components/autonomy/scoreSummary'
import { screenReview } from '../../../i18n/screens'
import {
  imageURL, isBlocked, isDrawing, latestVersion, reviewLabel, useReview, usedVersion, versionURL,
  type ReviewImage,
} from '../useReview'
import { ms } from './mreview.stylex'

const copy = screenReview.mobile

const clock = (iso?: string) => {
  if (!iso) return ''
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

export function MReviewScreen({ route }: { route: Extract<Route, { name: 'review' }> }) {
  const review = useReview()
  const online = useOnline()
  const { busy: versionBusy, pending: versionPending, run: applyVersion } = useAsyncAction(async (targetJob: string, targetImage: string, n: number) => {
    try {
      await useImageVersion(targetJob, targetImage, n)
      await review.loadImages(targetJob, true)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Chưa đổi được phiên bản. Vui lòng thử lại.')
    }
  })
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const counts = useResource(['review', 'counts'], ({ signal }) => request<{ approvedToday: number | null }>('/api/clone/counts', { signal }))
  const address = route
  const jobId = address?.video
  const [versions, setVersions] = useState<Record<string, number>>({})
  const [imageFailed, setImageFailed] = useState(false)
  const [sheet, setSheet] = useState<'feedback' | 'prompt' | 'why' | null>(null)
  const [tab, setTab] = useState<'feed' | 'edit'>('feed')
  const [feedback, setFeedback] = useState('')
  const [draft, setDraft] = useState('')
  const [sheetBase, setSheetBase] = useState<{ jobId: string; imageId: string; version: number } | null>(null)
  const [reveal, setReveal] = useState<{ id: string; version: number } | null>(null)
  const [compare, setCompare] = useState(false)
  const [since, setSince] = useState<{ key: string; at: number } | null>(null)
  const [now, setNow] = useState(() => Date.now())
  const scrollRef = useRef<HTMLElement | null>(null)
  const itemRef = useRef<ReviewImage | undefined>(undefined)
  const seen = useRef<Record<string, number>>({})
  const redrawOwner = useRef<{ jobId?: string; imageId?: string; sheet: typeof sheet }>({ sheet: null })
  redrawOwner.current.sheet = sheet
  // Một cửa vẽ lại đúng một ảnh: giữ nguyên các phần prompt/ảnh tham chiếu của
  // bản đang xem qua `baseVersion`, chỉ gửi phần người dùng sửa.
  const { busy: redrawBusy, pending: redrawPending, run: applyRedraw } = useAsyncAction(async (targetJob: string, imageId: string, baseVersion: number | undefined, body: Omit<RedrawBody, 'imageIds' | 'baseVersion'>) => {
    try {
      await redrawImages(targetJob, { imageIds: [imageId], ...(baseVersion == null ? {} : { baseVersion }), ...body })
      await review.loadImages(targetJob, true)
      if (redrawOwner.current.jobId === targetJob && redrawOwner.current.imageId === imageId && redrawOwner.current.sheet === 'feedback') {
        setSheet(null); setFeedback(''); setDraft(''); setSheetBase(null)
      }
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Chưa gửi được yêu cầu vẽ lại. Vui lòng thử lại.')
    }
  })
  // Cuộn xuống thì thanh tab thu nhỏ đúng như artboard (tabScroll › down).
  useMobileScroll(scrollRef)

  const channelFilter = address?.channel
  const jobs = useMemo(() => (review.jobs ?? []).filter(j => !channelFilter || channelFilter === 'all' || j.channelId === channelFilter), [review.jobs, channelFilter])
  const job = jobId ? jobs.find(j => j.id === jobId) : jobs[0]
  redrawOwner.current.jobId = job?.id
  useEffect(() => {
    if (!review.jobs || review.error || !jobId || job) return
    toast.err('Video đã chọn không có trong danh sách chờ duyệt. Đã trở về danh sách hiện tại.')
    navigate(paths.review({ channel: channelFilter }), true)
  }, [review.jobs, review.error, jobId, job, channelFilter])
  const images = job ? review.images[job.id] : undefined
  const items = images?.items ?? []
  const requestedIndex = address?.img ? items.findIndex(candidate => candidate.id === address.img) : 0
  const selected = Math.max(0, requestedIndex)
  const item = requestedIndex < 0 ? undefined : items[selected]
  const setIndex = (at: number) => {
    const target = items[at]
    if (job && target) navigate(paths.review({ channel: channelFilter, video: job.id, img: target.id }))
  }
  useEffect(() => {
    if (!job || !images || !address?.img || requestedIndex >= 0) return
    toast.err('Ảnh đã chọn không còn trong bộ ảnh. Đã chuyển về ảnh còn lại.')
    navigate(paths.review({ channel: channelFilter, video: job.id, img: items[0]?.id }), true)
  }, [job?.id, images, address?.img, requestedIndex, channelFilter, items])
  itemRef.current = item
  redrawOwner.current.imageId = item?.id
  useEffect(() => {
    setSheet(null); setFeedback(''); setDraft(''); setSheetBase(null)
  }, [job?.id, item?.id])
  const channel = channels.data?.channels.find(c => c.id === job?.channelId)
  const summary = images?.summary ?? null
  const approved = items.filter(i => i.status === 'approved').length
  const busy = (!!job && !!review.pending[job.id]) || versionBusy || redrawBusy
  const locked = busy || !images || !items.length || items.some(isDrawing)
  const reason = !online ? copy.offline : !images ? copy.loadingImages : !items.length ? copy.noImages : copy.approveReason
  const scenesLeft = items.filter(i => i.kind === 'scene' && i.status !== 'skipped').length
  const neighbour = item
    ? [...items.slice(0, selected)].reverse().find(candidate => candidate.kind === 'scene' && candidate.status !== 'skipped')
      ?? items.find((candidate, i) => i > selected && candidate.kind === 'scene' && candidate.status !== 'skipped')
    : undefined
  const fallback = item && isBlocked(item) ? neighbour : undefined
  const shown = fallback ?? item
  const shownIndex = shown ? items.indexOf(shown) : 0
  const score = readImageScore(shown?.score)

  // Cùng GET có `JobImage[]` hiện đại; dữ liệu cũ có thể không mang `state`, nên
  // chỉ bản ghi rõ 'cancelled' mới bị loại khỏi lựa chọn mặc định/nguồn vẽ lại.
  const stateOf = (candidate: ReviewImage, n: number) => {
    const modern = images?.images?.find(entry => entry.id === candidate.id)?.versions.find(entry => entry.n === n)
    if (modern) return modern.state
    return (candidate.versions.find(entry => entry.version === n) as { state?: string } | undefined)?.state
  }
  const usable = (candidate: ReviewImage, n: number) => candidate.versions.some(entry => entry.version === n) && stateOf(candidate, n) !== 'cancelled'
  const latestUsable = (candidate: ReviewImage) =>
    candidate.versions.reduce((latest, entry) => entry.version > latest && usable(candidate, entry.version) ? entry.version : latest, 0)
  const chosen = (candidate: ReviewImage) => {
    const wanted = versions[`${job?.id}:${candidate.id}`]
    if (wanted !== undefined && candidate.versions.some(v => v.version === wanted) && usable(candidate, wanted)) return wanted
    const preferred = candidate.reviewVersion ?? candidate.currentVersion
    return preferred != null && usable(candidate, preferred) ? preferred : latestUsable(candidate)
  }
  const version = item ? chosen(item) : 0
  // Cùng GET có `JobImage[]` hiện đại: phần Cảnh của đúng bản đang xem là thứ
  // duy nhất được sửa trên điện thoại; các phần và ảnh tham chiếu còn lại do
  // `baseVersion` giữ nguyên ở máy chủ. Bản cũ không có phần thì không giả lập.
  const modernItem = item ? images?.images?.find(candidate => candidate.id === item.id) : undefined
  const editableSceneOf = (n: number) => {
    const candidate = modernItem?.versions.find(entry => entry.n === n)
    return candidate && !candidate.partsLegacy ? candidate.parts.find(part => part.kind === 'scene')?.text ?? null : null
  }
  const editableScene = item ? editableSceneOf(version) : null
  const sheetSourceVersion = sheetBase && sheetBase.jobId === job?.id && sheetBase.imageId === item?.id ? sheetBase.version : version
  const sheetSourceUsable = !!item && (sheetSourceVersion === 0 ? version === 0 : usable(item, sheetSourceVersion))
  const sheetScene = item ? editableSceneOf(sheetSourceVersion) : null
  const used = item ? usedVersion(item) : 0
  // Artboard › nút "So với bản N-1": bản trước đã có URL riêng nên so được thật.
  const shownChosen = shown ? chosen(shown) : 0
  const canCompare = !!shown && shownChosen > 1 && !isDrawing(shown)
  const comparing = compare && canCompare
  const shownVersionNumber = comparing ? shownChosen - 1 : shownChosen
  const stageURL = shown && job ? (versionURL(job.id, shown, shownVersionNumber) || imageURL(job.id, shown)) : ''
  const sceneTime = shown?.kind === 'thumbnail'
    ? shown.width && shown.height ? `${shown.width}×${shown.height}` : ''
    : shown?.startSec != null && shown.endSec != null
      ? `${Math.floor(shown.startSec / 60)}:${String(Math.floor(shown.startSec % 60)).padStart(2, '0')}–${Math.floor(shown.endSec / 60)}:${String(Math.floor(shown.endSec % 60)).padStart(2, '0')}`
      : ''
  const shownVersion = shown?.versions.find(v => v.version === (shown ? chosen(shown) : 0))
  const versionTime = clock(shownVersion?.createdAt)
  const metaLine = [shownVersion?.source, shown?.width && shown?.height ? `${shown.width}×${shown.height}` : '', versionTime].filter(Boolean).join(' · ')
  const sheetSubtitle = item ? (sheetSourceVersion > 0 ? copy.sheetSubtitle(reviewLabel(item, selected), sheetSourceVersion) : reviewLabel(item, selected)) : ''
  const sentPrompt = shownVersion?.prompt?.trim() || shown?.prompt?.trim() || ''
  const promptSubtitle = shown ? (shownChosen > 0 ? copy.sheetSubtitle(reviewLabel(shown, shownIndex), shownChosen) : reviewLabel(shown, shownIndex)) : ''
  const drawingKey = item && job && isDrawing(item) ? `${job.id}:${item.id}:${item.versions.length}` : null
  const elapsed = drawingKey && since?.key === drawingKey ? Math.max(0, Math.floor((now - since.at) / 1000)) : 0
  const lowIndex = summary ? items.findIndex(candidate => {
    const value = readImageScore(candidate.score)
    return !!value && isLow(value, summary.min) && !isDrawing(candidate)
  }) : -1
  const readyVersion = item
    ? item.versions.reduce((latest, entry) => (entry.originalUrl || entry.cutoutUrl) && usable(item, entry.version) ? Math.max(latest, entry.version) : latest, 0)
    : 0

  useEffect(() => { setImageFailed(false) }, [job?.id, item?.id, version])
  useEffect(() => { setCompare(false) }, [job?.id, item?.id, version])
  useEffect(() => {
    if (!item || !job) return
    const key = `${job.id}:${item.id}`
    setVersions(old => ({ ...old, [key]: old[key] ?? item.reviewVersion ?? item.currentVersion ?? latestVersion(item) }))
  }, [job?.id, item?.id, item?.reviewVersion, item?.currentVersion, item?.versions.length])
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
    const timer = window.setInterval(() => void review.loadImages(pollingId, true), 2500)
    return () => clearInterval(timer)
  }, [pollingId, hasDrawing, review.loadImages])
  // Bản mới vẽ xong: tự chọn bản mới nhất và chiếu một lượt sáng "MỚI" như artboard › reveal.
  useEffect(() => {
    const current = itemRef.current
    if (!current || !job) return
    const key = `${job.id}:${current.id}`
    const newest = readyVersion
    const previous = seen.current[key]
    if (isDrawing(current)) {
      seen.current[key] ??= newest
      return
    }
    seen.current[key] = newest
    if (previous === undefined || newest <= previous) {
      setReveal(null)
      return
    }
    setVersions(old => ({ ...old, [key]: newest }))
    setReveal({ id: current.id, version: newest })
    const timer = window.setTimeout(() => setReveal(null), 2600)
    return () => clearTimeout(timer)
  }, [job?.id, item?.id, readyVersion, item?.status])
  useEffect(() => {
    if (!drawingKey) return
    setSince({ key: drawingKey, at: Date.now() })
    setNow(Date.now())
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [drawingKey])

  const selectJob = (id: string) => { navigate(paths.review({ channel: channelFilter, video: id })); setFeedback(''); setDraft(''); setSheet(null) }
  const moveJob = (delta: number) => {
    if (!job) return
    const at = Math.max(0, Math.min(jobs.length - 1, jobs.findIndex(j => j.id === job.id) + delta))
    const next = jobs[at]
    if (next && next.id !== job.id) selectJob(next.id)
  }
  const moveImage = (delta: number) => { if (items.length) setIndex((selected + delta + items.length) % items.length) }
  const openFeedback = (next: 'feed' | 'edit') => {
    if (!item || !job || (next === 'edit' && !usable(item, version))) return
    const base = version
    setSheetBase({ jobId: job.id, imageId: item.id, version: base })
    setTab(next)
    if (next === 'edit') setDraft(editableSceneOf(base) ?? '')
    setSheet('feedback')
  }
  const redraw = () => {
    if (!job || !item || isDrawing(item) || busy || !online || !sheetBase || sheetBase.jobId !== job.id || sheetBase.imageId !== item.id) return
    // Chốt bản và câu chữ tại lúc gửi; chọn bản khác trong lúc chờ không đổi nguồn.
    const base = sheetBase.version
    if (base === 0 ? version !== 0 : !usable(item, base)) return
    const text = (tab === 'feed' ? feedback : draft).trim()
    if (tab === 'edit') {
      const scene = editableSceneOf(base)
      if (scene === null || !text || text === scene.trim()) return
      void applyRedraw(job.id, item.id, base, { parts: { scene: text } })
      return
    }
    void applyRedraw(job.id, item.id, base || undefined, text ? { feedback: text } : {})
  }
  const skip = () => {
    if (!job || !item || busy || !online || item.kind !== 'scene' || scenesLeft <= 1) return
    void review.mutate(job.id, item, 'skip')
  }
  const useVersion = () => {
    if (!job || !item || isDrawing(item) || busy || !online || version === used || !usable(item, version)) return
    void applyVersion(job.id, item.id, version)
  }
  const approve = () => {
    if (!job || !images || !items.length || locked || !online) return
    const at = jobs.findIndex(j => j.id === job.id)
    const next = jobs[at + 1] ?? jobs[at - 1]
    if (next) selectJob(next.id)
    void review.mutate(job.id, null, 'approveAll').then(ok => { if (!ok) selectJob(job.id) })
  }
  const copyPrompt = async () => {
    const text = sentPrompt
    if (!text) return
    try { await navigator.clipboard.writeText(text); toast.ok(copy.promptCopied) }
    catch { toast.err(copy.copyFailed) }
  }

  const titleRow = <header {...stylex.props(ms.titleRow)}>
    <h1 {...stylex.props(ms.title)}>{copy.title}</h1>
    {job && <span {...stylex.props(ms.count)}>{copy.videoCount(jobs.findIndex(j => j.id === job.id) + 1, jobs.length)}</span>}
    {job && <button type="button" aria-label={copy.nextVideo} title={copy.nextVideo} disabled={jobs.findIndex(j => j.id === job.id) >= jobs.length - 1} onClick={() => moveJob(1)} {...stylex.props(ms.nextJob)}><Icon name="arrow-r" size={18} /></button>}
  </header>

  if (!review.jobs) return <div {...stylex.props(ms.root)}>{titleRow}
    <div role={review.error ? 'alert' : 'status'} {...stylex.props(ms.center)}>
      {review.error ? <><strong {...stylex.props(ms.emptyTitle)}>{copy.loadError}</strong><Button variant="ghost" size="lg" onClick={() => void review.loadJobs()}>{copy.retry}</Button></> : <><Spinner size={18} /><span {...stylex.props(ms.loadingText)}>{copy.loading}</span></>}
    </div>
  </div>

  if (!job) return <div {...stylex.props(ms.root)}>{titleRow}
    <div role="status" {...stylex.props(ms.center)}>
      <span aria-hidden="true" {...stylex.props(ms.emptyOrbs)}><Orb of="artist" size={30} /><span {...stylex.props(ms.emptyQa)}><Orb of="qa" size={24} /></span></span>
      <strong {...stylex.props(ms.emptyTitle)}>{copy.emptyTitle}</strong>
      <span {...stylex.props(ms.emptyText)}>{counts.data?.approvedToday ? copy.emptyBuilding(counts.data.approvedToday) : screenReview.emptyHint}</span>
      <Button variant="ghost" size="lg" xstyle={ms.emptyAction} onClick={() => { window.location.hash = paths.workbench() }}>{copy.emptyAction}</Button>
    </div>
  </div>

  const drawing = item ? isDrawing(item) : false
  const failed = item?.status === 'failed' && !fallback
  const stageError = imageFailed || failed
  const nextVersion = images?.images?.find(candidate => candidate.id === item?.id)?.versions.find(candidate => candidate.state === 'drawing')?.n ?? (item ? latestVersion(item) + 1 : 1)
  const newest = item ? latestVersion(item) : 0

  return <div {...stylex.props(ms.root)}>
    {titleRow}
    <main ref={scrollRef} {...stylex.props(ms.scroll)}>
      <div {...stylex.props(ms.itemHead)}>
        <span {...stylex.props(ms.itemTitle)}>{job.source.title}</span>
        <span {...stylex.props(ms.metaRow)}>
          {channel && <InitialsAvatar name={channel.name} size={20} ring="var(--line-4)" />}
          {channel && <span {...stylex.props(ms.metaName)}>{channel.name}</span>}
          {channel && <span {...stylex.props(ms.metaSep)}>·</span>}
          <span {...stylex.props(ms.metaStrong)}>{copy.approvedCount(approved, items.length)}</span>
        </span>
      </div>

      <div {...stylex.props(ms.stage)}>
        {!images && <div role="status" {...stylex.props(ms.stageError)}>{review.imageErrors[job.id] ? <><span {...stylex.props(ms.stageErrorTitle)}>{copy.imageError}</span><Button variant="ghost" size="sm" onClick={() => void review.loadImages(job.id, true)}>{copy.retry}</Button></> : <><Spinner size={18} /><span {...stylex.props(ms.stageErrorText)}>{copy.loadingImages}</span></>}</div>}
        {images && !drawing && !stageError && stageURL && (reveal?.id === shown?.id
          ? <DevelopEffect key={`${shown?.id}:${shownVersionNumber}`} state="done"><img src={stageURL} alt={shown ? reviewLabel(shown, shownIndex) : ''} onError={() => setImageFailed(true)} {...stylex.props(ms.stageImage)} /></DevelopEffect>
          : <img src={stageURL} alt={shown ? reviewLabel(shown, shownIndex) : ''} onError={() => setImageFailed(true)} {...stylex.props(ms.stageImage)} />)}
        {images && !drawing && stageError && <div role="alert" {...stylex.props(ms.stageError)}>
          <span {...stylex.props(ms.stageErrorTitle)}>{failed ? copy.failedTitle : copy.imageError}</span>
          {failed && <span {...stylex.props(ms.stageErrorText)}>{copy.failedHint}</span>}
          <Button variant="ghost" size="sm" onClick={() => { setImageFailed(false); if (failed) openFeedback('feed'); else if (job) void review.loadImages(job.id, true) }}>{failed ? copy.redraw : copy.retry}</Button>
        </div>}
        {images && shown && <span {...stylex.props(ms.stageLabel)}>
          <span {...stylex.props(ms.stageKey)}>{shownIndex}</span>
          <span>{reviewLabel(shown, shownIndex)}{shownVersionNumber > 0 ? <> · <span {...stylex.props(ms.stageTag, (comparing || shownVersionNumber !== used) && ms.stageTagWarn)}>bản {shownVersionNumber}</span></> : null}</span>
        </span>}
        {images && item?.status === 'approved' && <span {...stylex.props(ms.stageApproved)}><Icon name="tick-b" size={12} />{copy.approvedBadge}</span>}
        {images && reveal?.id === shown?.id && <span {...stylex.props(ms.stageNew)}>{copy.newVersion}</span>}
        {images && drawing && <div {...stylex.props(ms.devWrap)}>
          <DevelopEffect state="drawing" baseUrl={stageURL} hideCard />
          <div role="status" aria-live="polite" {...stylex.props(ms.devOverlay)}>
            <span {...stylex.props(ms.devOverlayHead)}>
              <Orb of="artist" size={16} live />
              <span {...stylex.props(ms.devOverlayTitle)}>{copy.drawingTitle(nextVersion)}</span>
              <span {...stylex.props(ms.devOverlayTimer)}>{elapsed >= 3 ? `${Math.floor(elapsed / 60)}:${String(elapsed % 60).padStart(2, '0')} · ~40 giây` : '~40 giây'}</span>
            </span>
            <span {...stylex.props(ms.devOverlaySteps)}>
              <span {...stylex.props(ms.devOverlayStep, (item?.status === 'pending' || item?.status === 'running') && ms.devOverlayStepDone)}><span aria-hidden="true" {...stylex.props(ms.devDot, (item?.status === 'pending' || item?.status === 'running') && ms.devDotDone)} />{copy.drawingSent}</span>
              <span {...stylex.props(ms.devOverlayStep, item?.status === 'running' && ms.devOverlayStepOn)}><span aria-hidden="true" {...stylex.props(ms.devDot, item?.status === 'running' && ms.devDotOn)} />{copy.drawingRunning}</span>
              <span {...stylex.props(ms.devOverlayStep)}><span aria-hidden="true" {...stylex.props(ms.devDot)} />Tải về</span>
            </span>
          </div>
        </div>}
        {canCompare && <Button variant="ghost" size="sm" aria-pressed={comparing} aria-label={comparing ? copy.compareOn(shownVersionNumber) : copy.compareTo(shownChosen - 1)} xstyle={ms.compare} onClick={() => setCompare(old => !old)}><Icon name="eye" size={14} />{comparing ? copy.compareOn(shownVersionNumber) : copy.compareTo(shownChosen - 1)}</Button>}
        {items.length > 1 && <>
          <button type="button" aria-label={copy.prevImage} onClick={() => moveImage(-1)} {...stylex.props(ms.tap, ms.tapPrev)}><span aria-hidden="true" {...stylex.props(ms.tapArrow, ms.tapArrowPrev)}><Icon name="arrow-left-2" size={16} /></span></button>
          <button type="button" aria-label={copy.nextImage} onClick={() => moveImage(1)} {...stylex.props(ms.tap, ms.tapNext)}><span aria-hidden="true" {...stylex.props(ms.tapArrow, ms.tapArrowNext)}><Icon name="arrow-r" size={16} /></span></button>
        </>}
      </div>

      {fallback && <div role="status" {...stylex.props(ms.scoreBanner)}><span {...stylex.props(ms.scoreBannerIcon)}><Icon name="info" size={14} /></span><span>{copy.blocked}</span></div>}
      {summary && <div role="status" {...stylex.props(ms.scoreBanner)}>
        <Orb of="qa" size={16} />
        <span {...stylex.props(ms.scoreBannerText)}>{summary.text || copy.scoreFallback(summary.passed, summary.total)} · {summary.mode === 'score' ? copy.scoreAuto : copy.scoreAsk}</span>
        {lowIndex > 0 && <Button variant="ghost" size="sm" xstyle={ms.scoreBannerAction} onClick={() => setIndex(lowIndex)}>Xem {reviewLabel(items[lowIndex], lowIndex)}</Button>}
      </div>}

      {items.length > 0 && <div role="listbox" aria-label={copy.imagesAria} {...stylex.props(ms.strip)}>
        {items.map((candidate, i) => {
          const candidateScore = readImageScore(candidate.score)
          return <button key={candidate.id} type="button" role="option" aria-selected={selected === i} aria-label={reviewLabel(candidate, i)} onClick={() => { setIndex(i); setFeedback('') }} {...stylex.props(ms.tile, isBlocked(candidate) && ms.tileBlocked, selected === i && ms.tileOn)}>
            {imageURL(job.id, candidate) && <img src={imageURL(job.id, candidate)} alt="" {...stylex.props(ms.tileImage)} />}
            <span {...stylex.props(ms.tileKey)}>{i}</span>
            {isDrawing(candidate) ? <span {...stylex.props(ms.tilePending)}><Spinner size={11} /></span> : candidate.status === 'approved' ? <span {...stylex.props(ms.tileTick)}>✓</span> : isBlocked(candidate) ? <span {...stylex.props(ms.tileState)}>!</span> : null}
            {summary && candidateScore && !isDrawing(candidate) && <span {...stylex.props(ms.tileScore, isLow(candidateScore, summary.min) && ms.tileLow)}>{candidateScore.value}</span>}
            {reveal?.id === candidate.id && <span {...stylex.props(ms.tileNew)}>{copy.newVersion}</span>}
          </button>
        })}
      </div>}

      {shown && <div {...stylex.props(ms.caption)}>
        <div {...stylex.props(ms.captionHead)}>
          <span {...stylex.props(ms.captionTitle)}>{reviewLabel(shown, shownIndex)}</span>
          {sceneTime && <span {...stylex.props(ms.captionTime)}>{sceneTime}</span>}
        </div>
        <p {...stylex.props(ms.textBox)}>{shown.kind === 'thumbnail' ? copy.coverHint : shown.narration || copy.noNarration}</p>
      </div>}

      {summary && score && <div {...stylex.props(ms.scoreRow)}>
        <span>{copy.scoreLabel}</span>
        <span {...stylex.props(ms.scoreValue)}>{score.value}</span>
        <Button variant="ghost" size="sm" xstyle={ms.scoreWhy} onClick={() => setSheet('why')}>{copy.why}</Button>
      </div>}

      {item && item.versions.length > 1 && <div {...stylex.props(ms.versions)}>
        <div {...stylex.props(ms.verHead)}>
          <span {...stylex.props(ms.verTitle)}>{copy.versionsTitle}</span>
          {version > 0 && <Button variant="ghost" size="sm" xstyle={ms.useVersion} disabled={drawing || version === used || busy || !online || !usable(item, version)} pending={versionPending} onClick={useVersion}>{copy.useVersion(version)}</Button>}
        </div>
        <div {...stylex.props(ms.verRow)}>
          {item.versions.map(candidate => {
            if (drawing && candidate.version === nextVersion) return null
            const cancelled = !usable(item, candidate.version)
            return <button key={candidate.version} type="button" aria-pressed={version === candidate.version} disabled={cancelled} aria-label={cancelled ? `${copy.versionTile(candidate.version)} · đã huỷ · không dùng làm nguồn vẽ lại` : copy.versionTile(candidate.version)} title={cancelled ? `${copy.versionTile(candidate.version)} · đã huỷ · không dùng làm nguồn vẽ lại` : copy.versionCreated(clock(candidate.createdAt))} onClick={() => { if (!cancelled) setVersions(old => ({ ...old, [`${job.id}:${item.id}`]: candidate.version })) }} {...stylex.props(ms.verTile, version === candidate.version && ms.verTileOn, candidate.version === used && ms.verTileUsed)}>
            {(candidate.originalUrl || versionURL(job.id, item, candidate.version)) && <img src={candidate.originalUrl || versionURL(job.id, item, candidate.version)} alt="" {...stylex.props(ms.verImage)} />}
            <span {...stylex.props(ms.verNum)}>{candidate.version}</span>
            {candidate.version === used && <span {...stylex.props(ms.verUsed)} title={copy.versionUsed} />}
          </button>})}
          {drawing && <span {...stylex.props(ms.verTile, ms.verDev)}><span {...stylex.props(ms.verDevText)}>{copy.versionTile(nextVersion)}<br />{copy.drawingRunning}</span></span>}
        </div>
      </div>}

      {shown && <button type="button" aria-label={copy.promptAria} onClick={() => setSheet('prompt')} {...stylex.props(ms.promptRow)}>
        <Icon name="gallery-edit" size={16} />
        <span {...stylex.props(ms.promptLabel)}>{copy.promptTitle}</span>
        {shown.referenceUrl && <span {...stylex.props(ms.promptRefs)}><img src={shown.referenceUrl} alt={copy.promptReference} {...stylex.props(ms.promptRefImage)} /></span>}
        <Icon name="arrow-r" size={16} />
      </button>}

      {item && <LearnSlot ownerKey={`${job.channelId}:${job.id}:${item.id}`} learn={images?.learn} />}
      {metaLine && <span {...stylex.props(ms.meta)}>{metaLine}</span>}
    </main>

    <div {...stylex.props(ms.abar)}>
      <div {...stylex.props(ms.abarRow)}>
        <ActionButton variant="outline" size="lg" xstyle={ms.abarCell} disabled={!item || drawing || busy || !online} disabledReason={copy.approveReason} onClick={() => openFeedback('feed')}><Icon name="gallery-edit" size={17} />{copy.redraw}</ActionButton>
        <ActionButton variant="outline" size="lg" xstyle={ms.abarCell} disabled={!item || busy || !online || item.kind !== 'scene' || scenesLeft <= 1} disabledReason={copy.skipReason} onClick={skip}>{copy.skip}</ActionButton>
      </div>
      <ActionButton variant="primary" size="xl" block lockedReason={locked || !online ? reason : undefined} onClick={approve}><Icon name="tick-b" size={18} />{busy ? copy.busy : copy.approve}</ActionButton>
    </div>

    <Sheet open={sheet === 'feedback'} onClose={() => setSheet(null)} label={copy.feedbackTitle} size="md" height="auto">
      <div {...stylex.props(ms.sheetPad)}>
        {item && <div {...stylex.props(ms.sheetHero)}>
          <span aria-hidden="true" {...stylex.props(ms.sheetHeroThumb)}>
            {versionURL(job.id, item, sheetSourceVersion) && <img src={versionURL(job.id, item, sheetSourceVersion)} alt="" {...stylex.props(ms.sheetHeroImg)} />}
          </span>
          <span {...stylex.props(ms.sheetHeroText)}>
            <span {...stylex.props(ms.sheetHeroTitle)}>Vẽ lại {sheetSubtitle}</span>
            <span {...stylex.props(ms.sheetHeroCaption)}>{item.kind === 'thumbnail' ? copy.coverHint : item.narration || copy.noNarration}</span>
          </span>
        </div>}
        <Segmented label={copy.feedbackTitle} value={tab} onChange={next => { if (next === 'edit' && sheetScene === null) return; setTab(next as 'feed' | 'edit'); if (next === 'edit') setDraft(sheetScene ?? '') }} size="md" fit="fill" radius={7} tabs={{ id: 'review-feedback-tabs', panel: 'review-feedback-panel' }}
          options={[{ id: 'feed', label: copy.feedbackTab }, { id: 'edit', label: copy.editTab, disabled: sheetScene === null }]} />
        <div id="review-feedback-panel" role="tabpanel" aria-labelledby={`review-feedback-tabs-${tab}`}>
          {tab === 'feed' ? <>
            <TextArea aria-label={copy.feedbackTitle} rows={3} showCount={false} maxLength={-1} value={feedback} onChange={event => setFeedback(event.currentTarget.value)} placeholder={copy.feedbackPlaceholder} />
            <div {...stylex.props(ms.chipsRail)}>
              {copy.feedbackQuick.map(suggestion => <Button key={suggestion} variant="ghost" size="sm" xstyle={ms.chip} onClick={() => setFeedback(old => old.trim() ? `${old.trim()}, ${suggestion.toLowerCase()}` : suggestion)}>{suggestion}</Button>)}
            </div>
            <p {...stylex.props(ms.editHint)}>Hoạ sĩ sửa phần Cảnh của prompt theo góp ý rồi vẽ; xem lại thay đổi ở Prompt đã gửi.</p>
          </> : <>
            <label {...stylex.props(ms.fieldLabel)}>{copy.editLabel}
              <TextArea rows={6} showCount={false} maxLength={-1} value={draft} disabled={sheetScene === null} onChange={event => setDraft(event.currentTarget.value)} />
            </label>
            <p {...stylex.props(ms.editHint)}>{sheetScene === null ? 'Bản này chưa lưu prompt theo phần nên không sửa tay được; bạn vẫn có thể góp ý để vẽ lại.' : copy.editHint}</p>
          </>}
        </div>
        <div {...stylex.props(ms.sheetFoot)}>
          <Button variant="ghost" size="lg" xstyle={ms.sheetFootGrow} onClick={() => setSheet(null)}>{copy.cancel}</Button>
          <Button variant="primary" size="lg" xstyle={ms.sheetFootGrow} pending={redrawPending} disabled={!online || drawing || busy || !sheetSourceUsable || (tab === 'edit' && (sheetScene === null || !draft.trim() || draft.trim() === sheetScene.trim()))} tooltip={sheetSourceUsable ? undefined : 'Nguồn ảnh đã thay đổi; đóng và chọn lại để vẽ tiếp.'} onClick={redraw}>{tab === 'feed' ? copy.feedbackSend : copy.editSend}</Button>
        </div>
        <p {...stylex.props(ms.sheetNote)}>Dùng 1 ảnh trong hạn mức hôm nay. Ảnh cũ được giữ tới khi ảnh mới xong.</p>
      </div>
    </Sheet>

    <Sheet open={sheet === 'prompt'} onClose={() => setSheet(null)} title={copy.promptTitle} subtitle={promptSubtitle} size="md" height="auto">
      {shown && <div {...stylex.props(ms.sheetPad)}>
        <div {...stylex.props(ms.segments)}>
          {sentPrompt && <div {...stylex.props(ms.segment)}>
            <span {...stylex.props(ms.segmentHead)}>{copy.promptSegmentScene}<span {...stylex.props(ms.metaSep)}>·</span>{copy.promptSourceArtist}</span>
            <span {...stylex.props(ms.segmentText)}>{sentPrompt}</span>
          </div>}
          {shown.direction?.trim() && <div {...stylex.props(ms.segment)}>
            <span {...stylex.props(ms.segmentHead)}>{copy.promptSegmentDirection}<span {...stylex.props(ms.metaSep)}>·</span>{copy.promptSourceDirection}</span>
            <span {...stylex.props(ms.segmentText)}>{shown.direction.trim()}</span>
          </div>}
          {!sentPrompt && !shown.direction?.trim() && <p {...stylex.props(ms.sheetLead)}>{copy.promptEmpty}</p>}
        </div>
        {shown.referenceUrl && <div {...stylex.props(ms.segments)}>
          <span {...stylex.props(ms.segmentHead)}>{copy.promptReference}</span>
          <div {...stylex.props(ms.refRow)}><img src={shown.referenceUrl} alt={copy.promptReference} {...stylex.props(ms.refImage)} /></div>
        </div>}
        {metaLine && <span {...stylex.props(ms.meta)}>{metaLine}</span>}
        <div {...stylex.props(ms.sheetFoot)}>
          <Button variant="ghost" size="lg" xstyle={ms.sheetFootGrow} onClick={() => void copyPrompt()}>{copy.promptCopy}</Button>
          <Button variant="primary" size="lg" xstyle={ms.sheetFootGrow} disabled={editableScene === null} title={editableScene === null ? 'Bản này chưa lưu prompt theo phần nên chỉ xem được; góp ý vẫn vẽ lại được.' : undefined} onClick={() => { setSheet(null); openFeedback('edit') }}>{copy.promptEdit}</Button>
        </div>
      </div>}
    </Sheet>

    <Sheet open={sheet === 'why'} onClose={() => setSheet(null)} title={copy.whyTitle} size="md" height="auto">
      <div {...stylex.props(ms.whyBody)}>{score && <ReasonPopover reason={score.reason} label={copy.why} />}</div>
    </Sheet>
  </div>
}
