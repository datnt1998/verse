// Phòng vẽ / Dựng & xuất / Thành phẩm của Bàn dựng PC (artboard Main › stage).
// Bảy trạng thái canonical dùng cùng dữ liệu thật của job: ảnh (§26), bước chạy,
// Độ hoàn thiện và file xuất. Không có số liệu hay ảnh mẫu nào được bịa.
import { useEffect, useMemo, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon, Shimmer, TextArea, toast } from '@noverse/ui'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { getJobImages, getScoreArtifact, redrawAll, type CloneJobView, type JobImage, type JobStep } from '../../api/workbench'
import {
  redrawImages, useImageVersion, useJobImages, withProgress,
  type ImageQuota, type ImageVersion,
} from '../../api/images'
import { useResource } from '../../data/useResource'
import { subscribeEvents } from '../../data/events'
import { useOnline } from '../../data/status'
import { ASSISTANT_INK, assistantName, DevelopEffect, Orb } from '../../components'
import { ImageViewer, type ImageViewerItem } from '../../components/image-viewer'
import { navigate, routePath, useRoute } from '../../router'
import { screenWorkbench } from '../../i18n/screens'
import { progressState, progressText } from '../../lib/progress-copy'
import { PART_LABELS, REF_ROLES, rawParams, rawSentText, versionMeta, versionNote } from '../review/imageView'
import { clockLabel, finishedVideo, imageTime, issueReason, jobStage, stepAgent, stepClock, stepDuration, stepPercent, QUALITY_MIN_SCORE, type FinishedVideo, type JobStage } from './workbench-logic'

type Props = {
  job: CloneJobView
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

export function LiveStudio({ job, onRefresh, onApproveAll, approveBusy, redrawAllOpen, onRedrawAllChange }: Props) {
  const stage = jobStage(job)
  const [logOpen, setLogOpen] = useState(false)
  const logAll = job.steps ?? []
  const logShow = logOpen ? logAll : logAll.slice(-2)
  const { route } = useRoute()
  const imageAddress = route.name === 'workbench' && route.video === job.id ? route.img : undefined
  const viewer = useMemo(() => imageAddress ? { jobId: job.id, imageId: imageAddress } : null, [job.id, imageAddress])
  const viewerPath = (img?: string) => routePath(route.name === 'workbench' ? { ...route, video: job.id, img } : { name: 'workbench', video: job.id, img })
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: stage !== 'queued' })
  // Phiên bản/prompt/ảnh tham chiếu thật (§26) + hạn mức ảnh của cùng endpoint.
  // Màn Duyệt dùng đúng hook này, nên hai màn không dựng bản sao dữ liệu. Chỉ
  // đọc khi thật sự mở viewer/xác nhận để không gọi trùng endpoint lúc xem lưới.
  const online = useOnline()
  const scoreArtifact = job.steps?.find(step => step.step === 'score')?.output?.artifacts?.find(item => item.path.endsWith('score.json'))
  const video = useMemo(() => finishedVideo(job), [job])
  const score = useResource(['workbench', 'score', job.id, scoreArtifact?.id || ''], ({ signal }) => getScoreArtifact(job.id, scoreArtifact!.id, signal), { enabled: !!scoreArtifact })
  // Hộp vẽ lại cả bộ gắn với đúng video đang mở: đổi video thì hộp cũ không còn hiệu lực.
  const [allOpen, setAllOpen] = useState<string | null>(null)
  // Lựa chọn theo ID ảnh, không theo chỉ số: polling/đổi job không thể trỏ nhầm ảnh.
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [feedback, setFeedback] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const allBusy = useRef(false)
  const [viewN, setViewN] = useState<number | null>(null)
  const [viewerFbOpen, setViewerFbOpen] = useState(false)
  const [viewerFeedback, setViewerFeedback] = useState('')
  const [viewerError, setViewerError] = useState('')
  const [viewerBusy, setViewerBusy] = useState(false)
  const viewerBusyRef = useRef(false)
  const [viewerSent, setViewerSent] = useState<{ imageId: string; n: number | null } | null>(null)
  const feedbackTarget = useRef<string | null>(null)
  const viewedJobRef = useRef(job.id)
  const viewedImageRef = useRef(viewer?.imageId)
  viewedJobRef.current = job.id
  viewedImageRef.current = viewer?.imageId
  const [usingVersion, setUsingVersion] = useState<number | null>(null)
  const live = useJobImages(viewer?.jobId === job.id || allOpen === job.id ? job.id : null)
  useEffect(() => {
    setAllOpen(null); setSelectedId(null)
    setFeedback(''); setError(''); setViewN(null)
    setViewerFbOpen(false); setViewerFeedback(''); setViewerError(''); setViewerSent(null); setUsingVersion(null)
  }, [job.id])
  useEffect(() => subscribeEvents(['clone.step.progress', 'clone.job.updated', 'image.progress'], () => { void images.refresh() }), [images.refresh])
  useEffect(() => {
    if (!redrawAllOpen) return
    // Chỉ một lớp nhận tương tác: mở xác nhận vẽ lại cả bộ thì đóng viewer.
    if (imageAddress) navigate(viewerPath(), true)
    setAllOpen(job.id); setError(''); setFeedback('')
  }, [redrawAllOpen])

  const items = useMemo(() => [...(images.data?.items ?? [])].sort((a, b) => Number(b.kind === 'thumbnail' || b.kind === 'cover') - Number(a.kind === 'thumbnail' || a.kind === 'cover')), [images.data])
  const coverFirst = items[0]?.kind === 'thumbnail' || items[0]?.kind === 'cover'
  const imageLabel = (item: JobImage, index: number) => item.kind === 'thumbnail' || item.kind === 'cover' ? copy.cover : copy.scene(index + (coverFirst ? 0 : 1))
  const legacyUrlOf = (item: JobImage) => (item.versions ?? []).find(version => version.version === (item.reviewVersion ?? item.currentVersion))?.originalUrl ?? (item.versions ?? []).at(-1)?.originalUrl
  const legacyUrlById = useMemo(() => new Map(items.map(item => [item.id, legacyUrlOf(item)] as const)), [items])
  // Ghép §26 (phiên bản/prompt/refs) với trường cũ của cùng payload (lời kể,
  // khung giờ, trạng thái đang vẽ). Không bịa phiên bản nào.
  const liveImages = useMemo<LiveImage[]>(() => items.map((item, index) => {
    const fromLive = live.data?.images.find(candidate => candidate.id === item.id)
    const shown = fromLive && live.progress[item.id] ? withProgress(fromLive, live.progress[item.id]) : fromLive
    return {
      id: item.id,
      kind: shown?.kind ?? (item.kind === 'thumbnail' || item.kind === 'cover' ? 'cover' : 'scene'),
      index: shown?.index ?? index + 1 - Number(coverFirst),
      label: shown?.label || imageLabel(item, index),
      status: item.status || shown?.status || 'review',
      usedVersion: shown?.usedVersion ?? item.currentVersion ?? item.reviewVersion ?? 1,
      refLimit: shown?.refLimit ?? 0,
      versions: shown?.versions ?? [],
      narration: item.narration,
      startSec: item.startSec,
      endSec: item.endSec,
    }
  }), [items, live.data, live.progress])

  const total = items.length
  const drawn = items.filter(item => !isDrawing(item.status) && item.status !== 'failed').length
  const drawingItems = items.filter(item => isDrawing(item.status))
  const failedItem = items.find(item => item.status === 'failed')
  const firstDrawing = items.findIndex(item => isDrawing(item.status))
  const drawing = drawingItems.length > 0
  const failedStep = job.steps?.find(step => step.status === 'failed')
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

  // Ảnh đang chọn phải còn trong bộ ảnh; nếu không, chọn ảnh còn lại và báo rõ.
  // Không giữ chỉ số cũ để R/vẽ lại trỏ nhầm sang ảnh khác.
  useEffect(() => {
    if (liveImages.some(image => image.id === selectedId)) return
    setSelectedId(liveImages[0]?.id ?? null)
  }, [liveImages, selectedId])

  // Viewer đang mở mà ảnh biến mất (polling/xoá): đóng viewer, trả lời bằng
  // toast, để lựa chọn rơi về ảnh còn lại thay vì thao tác nhầm ảnh đầu.
  useEffect(() => {
    if (!viewer || viewer.jobId !== job.id) return
    if (!live.data && !images.data) return
    if (liveImages.some(image => image.id === viewer.imageId)) return
    navigate(viewerPath(), true); setViewN(null)
    setViewerFbOpen(false); setViewerFeedback(''); setViewerError(''); setViewerSent(null)
    toast.err('Ảnh đã chọn không còn trong bộ ảnh. Đã chọn ảnh còn lại.')
  }, [viewer, liveImages, job.id, live.data, images.data])

  const viewerImage = viewer ? liveImages.find(image => image.id === viewer.imageId) : undefined
  const usedVersion = viewerImage?.versions.find(candidate => candidate.n === viewerImage.usedVersion)
  // Bản đã huỷ vẫn nằm trong lịch sử nhưng không còn là nguồn vẽ lại hợp lệ:
  // bỏ qua nó khi chọn mặc định hoặc khi lựa chọn cũ trỏ vào nó.
  const baseVersion = usedVersion && usedVersion.state !== 'cancelled'
    ? usedVersion
    : viewerImage?.versions.findLast(candidate => candidate.state !== 'cancelled')
  // Mặc định bám bản mới nhất đã vẽ xong, không bám bản đang vẽ (bản đang vẽ
  // chưa có ảnh): sau khi gửi góp ý, khung vẫn hiện ảnh đang dùng cho video.
  const latestReady = viewerImage?.versions.findLast(candidate => candidate.state === 'ready')
  const explicitVersion = viewerImage && viewN != null
    ? viewerImage.versions.find(candidate => candidate.n === viewN && candidate.state !== 'cancelled')
    : undefined
  const viewerVersion = viewerImage ? explicitVersion ?? latestReady ?? baseVersion : undefined
  const viewerDrawing = viewerImage?.versions.find(candidate => candidate.state === 'drawing')
  const quota = live.data?.quota

  const viewerItems = useMemo<ImageViewerItem[]>(() => liveImages.map((image, index) => {
    const currentVersion = image.id === viewerImage?.id ? viewerVersion : undefined
    const version = currentVersion ?? image.versions.find(candidate => candidate.n === image.usedVersion) ?? image.versions.at(-1)
    const url = currentVersion
      ? currentVersion.url ?? currentVersion.previewUrl
      : version?.url ?? version?.previewUrl ?? legacyUrlById.get(image.id)
    const tileDrawing = isDrawing(image.status) || image.versions.some(candidate => candidate.state === 'drawing')
    const failed = image.status === 'failed'
    return {
      id: image.id,
      label: image.label,
      badge: image.kind === 'cover' ? 'Bìa' : String(image.index),
      url,
      disabled: !url || failed || (tileDrawing && image.id !== viewerImage?.id),
    }
  }), [liveImages, viewerImage?.id, viewerVersion, legacyUrlById])
  useEffect(() => {
    if (!imageAddress || !images.data) return
    const target = liveImages.find(image => image.id === imageAddress)
    if (!target) return
    setSelectedId(imageAddress)
    if (stage === 'review') navigate(routePath({ name: 'review', video: job.id, img: imageAddress }), true)
  }, [imageAddress, images.data, liveImages, job.id, stage])
  useEffect(() => {
    setViewN(null)
    setViewerFbOpen(!!imageAddress && feedbackTarget.current === imageAddress)
    feedbackTarget.current = null
    setViewerFeedback(''); setViewerError(''); setViewerSent(null)
  }, [imageAddress])

  const closeViewer = () => {
    feedbackTarget.current = null
    navigate(viewerPath(), true); setViewN(null)
    setViewerFbOpen(false); setViewerFeedback(''); setViewerError(''); setViewerSent(null)
  }
  const openViewer = (imageId: string, withFeedback = false) => {
    const target = viewerItems.find(item => item.id === imageId)
    if (!target || target.disabled) return
    feedbackTarget.current = withFeedback ? imageId : null
    setAllOpen(null)
    setSelectedId(imageId); setViewN(null)
    setViewerFbOpen(withFeedback)
    setViewerFeedback(''); setViewerError(''); setViewerSent(null)
    navigate(viewerPath(imageId))
  }
  const openReview = (imageId: string) => navigate(routePath({ name: 'review', video: job.id, img: imageId }))
  const selectViewerImage = (imageId: string) => {
    setSelectedId(imageId); setViewN(null)
    setViewerFbOpen(false); setViewerFeedback(''); setViewerError(''); setViewerSent(null)
    navigate(viewerPath(imageId), true)
  }
  const redrawSelected = () => {
    if (!selectedId) return
    if (stage === 'review') { openReview(selectedId); return }
    if (stage === 'done') return
    openViewer(selectedId, true)
  }

  const useVersion = (n: number) => {
    if (!viewerImage || usingVersion !== null || !online) return
    const imageId = viewerImage.id
    setUsingVersion(n)
    void useImageVersion(job.id, imageId, n)
      .then(() => {
        if (viewedJobRef.current !== job.id) return
        if (viewedImageRef.current === imageId) setViewN(n)
        void live.reload(); void images.refresh(); onRefresh()
      })
      .catch(err => toast.err(err instanceof Error ? err.message : 'Chưa đổi được bản dùng cho video.'))
      .finally(() => setUsingVersion(null))
  }

  // Một lượt gửi tại một thời điểm; form và góp ý còn nguyên khi lỗi.
  const sendViewerRedraw = async () => {
    if (!viewerImage || viewerBusyRef.current || !online || viewerDrawing) return
    // Bản đã huỷ không phải nguồn hợp lệ: không gửi gì để máy chủ khỏi từ chối.
    if (viewerVersion?.state === 'cancelled') return
    // Chưa có hạn mức thật thì chưa gửi: nút gửi phải nói đúng chi phí trước khi tiêu ảnh.
    if (!quota || quota.left <= 0) return
    const imageId = viewerImage.id
    // Chốt bản đang xem lúc gửi: SSE đổi bản chọn trong lúc chờ không được đổi nguồn.
    const baseVersion = viewerVersion?.n
    viewerBusyRef.current = true; setViewerBusy(true); setViewerError('')
    try {
      const text = viewerFeedback.trim()
      const result = await redrawImages(job.id, { imageIds: [imageId], ...(baseVersion != null ? { baseVersion } : {}), ...(text ? { feedback: text } : {}) })
      const created = result.versions.find(entry => entry.imageId === imageId)
      const n = created?.n ?? null
      setViewerSent({ imageId, n })
      setViewerFbOpen(false); setViewerFeedback('')
      toast.ok(n != null ? `Đã gửi Hoạ sĩ · bản ${n}` : 'Đã gửi Hoạ sĩ.')
      void live.reload(); void images.refresh(); onRefresh()
    } catch (err) {
      setViewerError(err instanceof Error ? err.message : 'Chưa gửi được góp ý. Vui lòng thử lại.')
    } finally { viewerBusyRef.current = false; setViewerBusy(false) }
  }

  const closeRedraw = () => { setAllOpen(null); onRedrawAllChange?.(false) }
  const submitAll = async () => {
    if (!images.data?.manifest || allOpen !== job.id || allBusy.current) return
    if (!quota) return
    // Chốt đúng revision/count tại lúc gửi; server từ chối nếu bộ ảnh đã đổi.
    const revision = images.data.manifest.revision
    const count = items.length
    if (!count) return
    allBusy.current = true; setBusy(true); setError('')
    try {
      await redrawAll(job.id, revision, count, feedback)
      setAllOpen(null); onRedrawAllChange?.(false); setFeedback('')
      void images.refresh(); void live.reload(); onRefresh()
      toast.ok(`Đã gửi Hoạ sĩ vẽ lại ${count} ảnh.`)
    } catch (err) { setError(err instanceof Error ? err.message : 'Không vẽ lại được') } finally { allBusy.current = false; setBusy(false) }
  }

  // Phím của Phòng vẽ/duyệt tại chỗ: 1–9 chọn ảnh, R vẽ lại ảnh đang chọn, A duyệt cả video.
  // Viewer/hộp xác nhận đang mở thì không nhận phím nền.
  useEffect(() => {
    if (!['images', 'review', 'issue', 'paused'].includes(stage)) return
    const onKey = (event: KeyboardEvent) => {
      const node = event.target as HTMLElement | null
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || !node || allOpen === job.id || viewer) return
      if (node.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(node.tagName)) return
      if (/^[1-9]$/.test(event.key)) {
        const index = Number(event.key) - 1
        const target = items[index]
        if (target) { event.preventDefault(); setSelectedId(target.id) }
        return
      }
      if (event.key.toLowerCase() === 'r') { event.preventDefault(); redrawSelected(); return }
      if (event.key.toLowerCase() === 'a' && canApprove) { event.preventDefault(); onApproveAll?.() }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [stage, items, selectedId, canApprove, onApproveAll, allOpen, viewer, viewerItems])

  // R trong viewer mở ô góp ý của đúng ảnh; không chạy khi đang gõ.
  useEffect(() => {
    if (!viewer || !viewerImage || stage === 'done') return
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return
      const node = event.target as HTMLElement | null
      if (node?.isContentEditable || (node && ['INPUT', 'TEXTAREA', 'SELECT'].includes(node.tagName))) return
      if (event.key.toLowerCase() !== 'r') return
      event.preventDefault()
      if (!viewerDrawing && online && viewerVersion?.state !== 'cancelled') setViewerFbOpen(true)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [viewer, viewerImage, viewerVersion, stage, viewerDrawing, online])

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

      {logAll.length > 0 && <div {...stylex.props(styles.logBox)}>
        <ol aria-label="Nhật ký" {...stylex.props(styles.logList)}>{logShow.map(step => {
          const running = job.stageGroup === 'working' && job.status === 'running' && step.status === 'running'
          const failed = step.status === 'failed'
          const done = step.status === 'succeeded' || step.status === 'done'
          const agent = stepAgent(step.step)
          return <li key={step.step} {...stylex.props(styles.logRow)}>
            <span {...stylex.props(styles.logTime)}>{stepClock(step.startedAt)}</span>
            <span {...stylex.props(styles.logIcon)}>{running ? <span aria-label="Đang chạy" {...stylex.props(styles.spinner)} /> : <Icon name={failed ? 'danger' : done ? 'tick-b' : 'clock'} size={15} label={progressState(step.status) || undefined} xstyle={failed ? styles.warnIcon : done ? styles.okIcon : styles.waitIcon} />}</span>
            <span {...stylex.props(styles.logText)}>{agent && <b style={{ color: ASSISTANT_INK[agent] }}>{assistantName(agent)}</b>}{agent && ' · '}<span>{running ? <Shimmer>{progressText(step)}</Shimmer> : progressText(step)}</span></span>
            <span {...stylex.props(styles.logDuration)}>{stepDuration(step)}</span>
          </li>
        })}</ol>
        <button type="button" aria-expanded={logOpen} onClick={() => setLogOpen(!logOpen)} {...stylex.props(styles.logToggle)}><Icon name="activity" size={13} />{logOpen ? 'Thu gọn nhật ký' : logAll.length > 2 ? `Nhật ký · ${logAll.length} bước` : 'Nhật ký'}<Icon name="arrow-d" size={12} xstyle={logOpen ? styles.logRot : undefined} /></button>
      </div>}

      {(stage === 'images' || stage === 'review' || stage === 'issue' || stage === 'paused') && (images.error
        ? <p role="alert" {...stylex.props(styles.notice)}>Không tải được bộ ảnh. <Button variant="quiet" size="sm" onClick={() => void images.refresh()}>Thử lại</Button></p>
        : total === 0
          ? images.initialLoading
            ? <div role="status" {...stylex.props(styles.notice)}>Đang tải bộ ảnh…</div>
            : <SummaryStage job={job} />
          : <div role="listbox" aria-label="Ảnh của video" {...stylex.props(styles.grid)}>{items.map((item, index) => {
            const url = legacyUrlOf(item)
            const status = item.status
            const tileDrawing = isDrawing(status)
            const waiting = isWaiting(status)
            const failed = status === 'failed'
            const label = imageLabel(item, index)
            const timing = imageTime(item)
            const selected = item.id === selectedId
            return <div
              key={item.id}
              role="option"
              aria-selected={selected}
              aria-label={label}
              tabIndex={0}
              onClick={() => {
                setSelectedId(item.id)
                if (stage === 'review') openReview(item.id)
                else openViewer(item.id)
              }}
              onFocus={() => setSelectedId(item.id)}
              {...stylex.props(styles.tile, (item.kind === 'thumbnail' || item.kind === 'cover') && styles.coverTile, selected && styles.tileSelected)}
            >
              <div {...stylex.props(styles.preview, (item.kind === 'thumbnail' || item.kind === 'cover') && styles.coverPreview, tileDrawing && styles.previewDrawing, failed && styles.previewFailed)}>
                {url && <img src={url} alt={label} {...stylex.props(styles.image, tileDrawing && styles.imageDimmed)} />}
                {tileDrawing && <><DevelopEffect state="drawing" baseUrl={url} compact /><span role="status" {...stylex.props(styles.tileState)}><Orb of="artist" size={18} live /><Shimmer>{status === 'running' ? copy.drawing : copy.queued}</Shimmer></span></>}
                {waiting && !tileDrawing && <span role="status" {...stylex.props(styles.tileState, styles.tileWait)}>{copy.queued}</span>}
                {failed && <span role="alert" {...stylex.props(styles.tileState, styles.tileError)}><Icon name="danger" size={18} /><span>{failedStep ? progressText(failedStep) : copy.failed}</span></span>}
                <span {...stylex.props(styles.badge)}><span {...stylex.props(styles.badgeKey)}>{item.kind === 'thumbnail' || item.kind === 'cover' ? 'Bìa' : index + (coverFirst ? 0 : 1)}</span>{label}</span>
                {item.status === 'approved' && <span {...stylex.props(styles.approved)}><Icon name="tick-b" size={18} label={copy.approved} /></span>}
                {stage !== 'review' && url && !tileDrawing && !waiting && !failed && (
                  <Button variant="icon" size="sm" aria-label={copy.redraw(label)} tooltip="Vẽ lại (R)" xstyle={styles.tileAction} onClick={(event) => {
                    event.stopPropagation()
                    openViewer(item.id, true)
                  }}><Icon name="gallery-edit" size={15} /></Button>
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
      {stage === 'done' && <DoneStage job={job} video={video} score={score.data} items={items} onOpenImage={openViewer} />}

      {error && <p role="alert" {...stylex.props(styles.notice)}>{error}</p>}
      {activeStep?.error && stage !== 'issue' && <p role="alert" {...stylex.props(styles.notice)}>{progressText(activeStep)}</p>}
    </div>

    {allOpen === job.id && <Dialog
      title={`Vẽ lại cả ${items.length} ảnh?`}
      onClose={busy ? undefined : closeRedraw}
      width={540}
      footer={<>
        <Button size="sm" disabled={busy} onClick={closeRedraw}>Huỷ</Button>
        <Button variant="primary" size="sm" pending={busy} pendingLabel="Đang gửi…" disabled={busy || !images.data?.manifest || drawing || !items.length || !quota} onClick={() => void submitAll()}>Vẽ lại {items.length} ảnh</Button>
      </>}
    >
      <p>{job.source.title}</p>
      <p>Dùng {items.length} ảnh trong hạn mức hôm nay</p>
      {quota && <p>{quota.providers.map(entry => `${entry.label} còn ${entry.left}/${entry.total} hôm nay`).join(' · ')}</p>}
      {!quota && <p role="status">Chưa tải được hạn mức ảnh nên chưa gửi được.</p>}
      {images.data?.manifest && <p>Ảnh cảnh vẽ theo <b>{images.data.manifest.scenePrompt.name}</b>, ảnh bìa theo <b>{images.data.manifest.coverPrompt.name}</b>, theo template của kênh.</p>}
      <p>Ảnh cũ được giữ tới khi ảnh mới xong; ảnh mới cần bạn duyệt lại.</p>
      <TextArea label="Góp ý chung cho cả bộ (tuỳ chọn)" value={feedback} onChange={event => setFeedback(event.currentTarget.value)} showCount={false} maxLength={-1} />
      {error && <p role="alert">{error}</p>}
    </Dialog>}

    {viewer && viewer.jobId === job.id && viewerImage && <ImageViewer
      mode="view"
      modeLabel={stage === 'done' ? 'Đã xong · xem' : 'Đang làm · xem'}
      title={job.source.title}
      items={viewerItems}
      selectedId={viewerImage.id}
      onSelect={selectViewerImage}
      onClose={closeViewer}
      side={<ViewerSide
        image={viewerImage}
        version={viewerVersion}
        quota={quota}
        online={online}
        usingVersion={usingVersion}
        sent={viewerSent}
        feedbackOpen={viewerFbOpen}
        feedback={viewerFeedback}
        onFeedback={setViewerFeedback}
        sending={viewerBusy}
        error={viewerError}
        versionsError={live.error}
        versionsLoading={live.loading}
        readOnly={stage === 'done'}
        onRetryVersions={() => void live.reload()}
        onSelectVersion={setViewN}
        onUseVersion={useVersion}
      />}
      footer={stage === 'done'
        ? <span {...stylex.props(styles.viewerNote)}>Video đã xuất · chỉ xem ảnh</span>
        : <>
            <span {...stylex.props(styles.viewerNote)}>{viewerSent?.imageId === viewerImage.id
              ? `Đang vẽ ${viewerSent.n != null ? `bản ${viewerSent.n}` : 'bản mới'} · bản ${viewerImage.usedVersion} vẫn dùng cho video`
              : 'Video đang làm · ảnh vẽ lại cần bạn duyệt lại · ← → đổi ảnh · Esc đóng'}</span>
            <span {...stylex.props(styles.spacer)} />
            {!viewerFbOpen
              ? <Button size="sm" tooltip="Góp ý rồi vẽ lại (R)" kbd="R" disabled={!online || !!viewerDrawing || viewerVersion?.state === 'cancelled'} onClick={() => setViewerFbOpen(true)} icon={<Icon name="gallery-edit" size={15} />}>Góp ý &amp; vẽ lại</Button>
              : <>
                  <Button size="sm" variant="quiet" disabled={viewerBusy} onClick={() => { setViewerFbOpen(false); setViewerFeedback(''); setViewerError('') }}>Huỷ</Button>
                  <Button size="sm" variant="primary" pending={viewerBusy} pendingLabel="Đang gửi…" disabled={viewerBusy || !online || !!viewerDrawing || viewerVersion?.state === 'cancelled' || !quota || quota.left <= 0} onClick={() => void sendViewerRedraw()}>Vẽ lại · tốn 1 ảnh{quota ? ` (còn ${quota.left})` : ''}</Button>
                </>}
          </>}
    />}
  </section>
}


type LiveImage = {
  id: string
  kind: 'scene' | 'cover'
  index: number
  label: string
  status: string
  usedVersion: number
  refLimit: number
  versions: ImageVersion[]
  narration?: string
  startSec?: number
  endSec?: number
}

/** Khoảng thời gian thật của cảnh trong video (chỉ có khi ảnh gắn với cảnh). */
function sceneSpan(image: LiveImage): string {
  if (image.startSec == null || image.endSec == null) return ''
  const format = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
  return `${format(image.startSec)}–${format(image.endSec)}`
}

/**
 * Cột phải của chế độ Xem: phiên bản, chỉ dẫn đã gửi, ảnh tham chiếu và ô góp ý.
 * Mọi số đều đọc từ §26/hạn mức thật; không có phiên bản nào được tự dựng.
 */
function ViewerSide({
  image, version, quota, online, usingVersion, sent, feedbackOpen, feedback, onFeedback,
  sending, error, versionsError, versionsLoading, readOnly, onRetryVersions, onSelectVersion, onUseVersion,
}: {
  image: LiveImage
  version: ImageVersion | undefined
  quota: ImageQuota | undefined
  online: boolean
  usingVersion: number | null
  sent: { imageId: string; n: number | null } | null
  feedbackOpen: boolean
  feedback: string
  onFeedback: (value: string) => void
  sending: boolean
  error: string
  versionsError: boolean
  versionsLoading: boolean
  readOnly: boolean
  onRetryVersions: () => void
  onSelectVersion: (n: number) => void
  onUseVersion: (n: number) => void
}) {
  const [raw, setRaw] = useState(false)
  const [copied, setCopied] = useState<'idle' | 'done' | 'failed'>('idle')
  const copyTimer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(copyTimer.current), [])
  useEffect(() => { setRaw(false); setCopied('idle') }, [image.id, version?.n])
  const drawingVersion = image.versions.find(candidate => candidate.state === 'drawing')
  // Bản lỗi chỉ được báo khi nó vẫn là lần vẽ mới nhất chưa bị một bản vẽ xong
  // thay thế — không lấy bản lỗi cũ che bản mới đang hiển thị.
  const latestFailed = image.versions.findLast(candidate => candidate.state === 'failed')
  const failedVersion = drawingVersion
    ? undefined
    : version?.state === 'failed'
      ? version
      : latestFailed && !image.versions.some(candidate => candidate.n > latestFailed.n && candidate.state === 'ready')
        ? latestFailed
        : undefined
  const selectedN = version?.n ?? image.usedVersion
  const span = sceneSpan(image)
  // Prompt cấu trúc chỉ hiện phần người dùng kiểm được; ghi chú đã học vẫn còn
  // nguyên trong “Xem nguyên văn” (provenance bất biến).
  const shownParts = version?.parts.filter(part => part.kind !== 'learned') ?? []
  const copyPromptText = async () => {
    if (!version?.sentText) return
    try {
      await navigator.clipboard.writeText(version.sentText)
      setCopied('done')
      copyTimer.current = window.setTimeout(() => setCopied('idle'), 1600)
    } catch { setCopied('failed') }
  }
  return <div {...stylex.props(styles.viewerSide)}>
    <div {...stylex.props(styles.viewerSideHead)}>
      <span {...stylex.props(styles.viewerSideTitle)}>{image.label}</span>
      <span {...stylex.props(styles.viewerSideSub)}>{[span, image.kind === 'cover' ? 'Ảnh bìa của video' : 'Ảnh cảnh'].filter(Boolean).join(' · ')}</span>
    </div>

    <section {...stylex.props(styles.viewerSection)}>
      <div {...stylex.props(styles.viewerSectionHead)}>
        <span {...stylex.props(styles.viewerSectionTitle)}>Phiên bản</span>
        <span {...stylex.props(styles.viewerMeta, styles.num)}>{image.versions.length}</span>
      </div>
      <div role="group" aria-label="Phiên bản của ảnh" {...stylex.props(styles.viewerVersions)}>
        {image.versions.map(entry => {
          const previous = image.versions.find(candidate => candidate.n === entry.n - 1)
          const url = entry.url ?? entry.previewUrl ?? (entry.state === 'drawing' ? previous?.url : null)
          const on = entry.n === selectedN
          const cancelled = entry.state === 'cancelled'
          const note = `Bản ${entry.n} · đã huỷ · không dùng làm nguồn vẽ lại`
          return <button
            key={entry.n}
            type="button"
            disabled={cancelled}
            aria-pressed={on}
            aria-label={cancelled ? note : `Bản ${entry.n}${entry.feedback ? ` · theo góp ý “${entry.feedback}”` : ' · lần đầu'}${entry.n === image.usedVersion ? ' · đang dùng' : ''}`}
            title={cancelled ? note : `${versionNote(entry)}${entry.n === image.usedVersion ? ' · đang dùng' : ''}`}
            onClick={() => { if (!cancelled) onSelectVersion(entry.n) }}
            {...stylex.props(styles.viewerVer, on && styles.viewerVerOn, cancelled && styles.viewerVerCancelled)}
          >
            {url && <img alt="" src={url} {...stylex.props(styles.viewerVerImage)} />}
            <span {...stylex.props(styles.viewerVerN)}>{entry.n}</span>
            {entry.n === image.usedVersion && <span aria-hidden="true" {...stylex.props(styles.viewerVerUsed)} />}
          </button>
        })}
      </div>
      {image.versions.length === 0 && <span {...stylex.props(styles.viewerVerNote)}>
        {versionsLoading ? 'Đang tải phiên bản…' : versionsError ? <>Chưa tải được phiên bản của ảnh này. <Button variant="quiet" size="sm" onClick={onRetryVersions}>Thử lại</Button></> : 'Chưa có phiên bản nào.'}
      </span>}
      {image.versions.length > 0 && <span {...stylex.props(styles.viewerVerNote)}>
        {drawingVersion
          ? `Đang vẽ bản ${drawingVersion.n}. Bản ${image.usedVersion} vẫn dùng cho video tới khi bản mới xong.`
          : failedVersion
            ? `Bản ${failedVersion.n} chưa vẽ được. Bản ${image.usedVersion} vẫn dùng cho video.`
            : versionNote(version)}
      </span>}
      {!readOnly && version && version.n !== image.usedVersion && version.state === 'ready' && <Button
        size="sm"
        disabled={!online || usingVersion !== null}
        pending={usingVersion === version.n}
        onClick={() => onUseVersion(version.n)}
        xstyle={styles.viewerUseButton}
      >Dùng bản {version.n} cho video</Button>}
    </section>

    <section {...stylex.props(styles.viewerSection)}>
      <div {...stylex.props(styles.viewerSectionHead)}>
        <span {...stylex.props(styles.viewerSectionTitle)}>Prompt đã gửi</span>
      </div>
      {!version
        ? <span {...stylex.props(styles.viewerVerNote)}>{versionsLoading ? 'Đang tải chỉ dẫn đã gửi…' : versionsError ? 'Chưa tải được chỉ dẫn đã gửi.' : 'Chưa có chỉ dẫn đã gửi.'}</span>
        : raw
          ? <>
              <pre {...stylex.props(styles.viewerRaw)}>{rawSentText(version)}</pre>
              {rawParams(version) && <span {...stylex.props(styles.viewerVerNote)}>{rawParams(version)}</span>}
            </>
          : shownParts.length
            ? <div {...stylex.props(styles.viewerPrompt)}>
                {shownParts.map((part, index) => (
                  <div key={`${part.kind}-${index}`} {...stylex.props(styles.viewerPart)}>
                    <span {...stylex.props(styles.viewerPartLabel)}>{PART_LABELS[part.kind]}<span {...stylex.props(styles.viewerPartSource)}> · {part.source.label}</span></span>
                    <span {...stylex.props(styles.viewerPartText)}>{part.text}</span>
                  </div>
                ))}
              </div>
            : <pre {...stylex.props(styles.viewerRaw)}>{version.sentText}</pre>}
      <div {...stylex.props(styles.viewerSectionHead)}>
        <Button size="sm" disabled={!version?.sentText} onClick={() => void copyPromptText()}>
          <Icon name="copy" size={14} />{copied === 'done' ? 'Đã chép' : copied === 'failed' ? 'Không chép được' : 'Chép prompt'}
        </Button>
        <Button variant="quiet" size="sm" aria-pressed={raw} disabled={!version} onClick={() => setRaw(value => !value)}>
          {raw ? 'Xem theo phần' : 'Xem nguyên văn'}
        </Button>
      </div>
      {version && versionMeta(version) && <span {...stylex.props(styles.viewerMeta)}>{versionMeta(version)}</span>}
    </section>

    {!!version?.refs.length && <section {...stylex.props(styles.viewerSection)}>
      <div {...stylex.props(styles.viewerSectionHead)}>
        <span {...stylex.props(styles.viewerSectionTitle)}>Ảnh tham chiếu · {version.refs.length}/{image.refLimit}</span>
      </div>
      <div {...stylex.props(styles.viewerRefs)}>
        {version.refs.map(ref => <div key={ref.id} {...stylex.props(styles.viewerRefCard)}>
          <img alt="" src={ref.url} {...stylex.props(styles.viewerRefThumb)} />
          <span {...stylex.props(styles.viewerRefText)}>
            <b {...stylex.props(styles.viewerRefLabel)}>{REF_ROLES.find(option => option.role === ref.role)?.label ?? ref.role}</b>
            <span {...stylex.props(styles.viewerRefSource)} title={ref.source.label}>{ref.source.label}</span>
          </span>
        </div>)}
      </div>
    </section>}

    {feedbackOpen && <div {...stylex.props(styles.viewerFeedback)}>
      <TextArea
        label="Góp ý cho Hoạ sĩ"
        autoFocus
        value={feedback}
        onChange={event => onFeedback(event.currentTarget.value)}
        showCount={false}
        maxLength={-1}
      />
      <span {...stylex.props(styles.viewerFeedbackHint)}>
        Hoạ sĩ sửa phần Cảnh của chỉ dẫn theo góp ý rồi vẽ bản mới. Bản {image.usedVersion} vẫn dùng cho video tới khi bản mới xong; bản mới cần bạn duyệt lại.
      </span>
      {quota && <span {...stylex.props(styles.viewerFeedbackHint)}>
        {quota.left > 0 ? `Còn ${quota.left} ảnh trong hạn mức hôm nay.` : 'Hết hạn mức ảnh hôm nay.'}
      </span>}
      {!quota && <span role="status" {...stylex.props(styles.viewerFeedbackHint)}>Chưa tải được hạn mức ảnh nên chưa gửi được.</span>}
      {!online && <span role="alert" {...stylex.props(styles.viewerError)}>Mất kết nối — chưa gửi được góp ý.</span>}
      {error && <span role="alert" {...stylex.props(styles.viewerError)}>{error}</span>}
      {sending && <span role="status" {...stylex.props(styles.viewerFeedbackHint)}>Đang gửi góp ý cho Hoạ sĩ…</span>}
    </div>}

    {sent?.imageId === image.id && <div role="status" {...stylex.props(styles.viewerBanner)}>
      <b {...stylex.props(styles.viewerBannerTitle)}>Đã gửi Hoạ sĩ{sent.n != null ? ` · bản ${sent.n}` : ''}</b>
      <span {...stylex.props(styles.viewerBannerText)}>Bản mới sẽ hiện ở đây và ở Phòng vẽ khi vẽ xong; bản đang dùng vẫn giữ cho video.</span>
    </div>}

    {image.narration && <details {...stylex.props(styles.viewerNarration)}>
      <summary {...stylex.props(styles.viewerNarrationTitle)}><Icon name="doc" size={15} />Lời kể trong cảnh</summary>
      <p {...stylex.props(styles.viewerNarrationText)}>{image.narration}</p>
    </details>}
  </div>
}

/** Video không có bộ ảnh: ảnh nguồn 16:9 + các dòng sự kiện thật, không bịa ảnh cảnh. */
function SummaryStage({ job }: { job: CloneJobView }) {
  const active = job.steps?.find(step => step.status === 'failed' || step.status === 'running') ?? job.steps?.find(step => step.status === 'waiting')
  const facts = [
    active ? { icon: active.status === 'failed' ? 'danger' as const : 'clock' as const, text: progressText(active), sub: job.subline || 'Bước đang chạy của video' } : null,
    { icon: 'gallery' as const, text: job.noImages ? 'Video này không tạo ảnh' : 'Chưa có ảnh nào', sub: job.noImages ? 'Dựng từ video nguồn và clip nền' : 'Hoạ sĩ vẽ sau khi Đạo diễn chia cảnh' },
    active?.step === 'images' ? { icon: 'warning' as const, text: 'Ảnh đã xong vẫn được giữ', sub: 'Xử lý xong thì Hoạ sĩ vẽ tiếp' } : null,
  ].filter((fact): fact is { icon: 'danger' | 'clock' | 'gallery' | 'warning'; text: string; sub: string } => !!fact)
  return <div {...stylex.props(styles.summary)}>
    <div {...stylex.props(styles.summaryCover)}>
      {job.source.thumbnailUrl ? <img src={job.source.thumbnailUrl} alt="" {...stylex.props(styles.image)} /> : <span {...stylex.props(styles.muted)}>Chưa có ảnh nguồn</span>}
      <span {...stylex.props(styles.summaryChip)}>Video nguồn</span>
    </div>
    <div {...stylex.props(styles.facts)}>{facts.map((fact, index) => <div key={index} {...stylex.props(styles.fact)}><Icon name={fact.icon === 'warning' ? 'danger' : fact.icon} size={15} xstyle={fact.icon === 'danger' || fact.icon === 'warning' ? styles.warnIcon : undefined} /><span {...stylex.props(styles.factBody)}><b>{fact.text}</b><small>{fact.sub}</small></span></div>)}</div>
  </div>
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
function DoneStage({ job, video, score, items, onOpenImage }: { job: CloneJobView; video: FinishedVideo | null; score: { total: number } | null | undefined; items: JobImage[]; onOpenImage: (imageId: string) => void }) {
  const cover = items.find(item => item.kind === 'thumbnail' || item.kind === 'cover')
  // Lỗi tải/giải mã không chứng minh tệp đã bị xoá; removedAt mới xác nhận việc xoá.
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const player = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [mediaDuration, setMediaDuration] = useState<number | null>(null)
  useEffect(() => {
    setFailedUrl(null)
    setPlaying(false)
    setCurrentTime(0)
    setMediaDuration(null)
  }, [video?.url])
  const shown = video && video.url !== failedUrl ? video : null
  // Thời lượng thật của bản xuất khi máy chủ đã đo; job cũ chỉ có độ dài nguồn.
  const seconds = mediaDuration ?? shown?.durationSec ?? job.source.durationSec
  const size = shown ? shown.bytes ?? job.exportBytes : null
  return <div {...stylex.props(styles.done)}>
    <div {...stylex.props(styles.playerBox)}>
      {shown ? <>
        <video ref={player} preload="metadata" playsInline {...stylex.props(styles.video)} src={shown.url}
          poster={cover?.versions?.find(version => version.version === (cover.reviewVersion ?? cover.currentVersion))?.originalUrl}
          onLoadedMetadata={event => { const duration = event.currentTarget.duration; if (Number.isFinite(duration) && duration > 0) setMediaDuration(duration) }}
          onTimeUpdate={event => setCurrentTime(event.currentTarget.currentTime)}
          onPlay={() => setPlaying(true)} onPause={() => setPlaying(false)} onEnded={() => setPlaying(false)}
          onError={() => setFailedUrl(shown.url)} />
        <button type="button" aria-label={playing ? 'Tạm dừng video' : 'Phát video'} {...stylex.props(styles.playerToggle)}
          onClick={() => { const element = player.current; if (!element) return; if (element.paused) void element.play().catch(() => toast.err('Chưa phát được video. Thử lại.')); else element.pause() }}>
          <Icon name={playing ? 'pause' : 'play'} size={24} />
        </button>
        <div {...stylex.props(styles.playerBar)}>
          <span {...stylex.props(styles.num)}>{clockLabel(currentTime)} / {seconds != null ? clockLabel(seconds) : '—'}</span>
          <input type="range" aria-label="Vị trí phát video" min={0} max={seconds ?? 0} step={0.05}
            value={Math.min(currentTime, seconds ?? 0)} disabled={!seconds}
            onChange={event => { if (player.current) { const time = Number(event.currentTarget.value); player.current.currentTime = time; setCurrentTime(time) } }}
            {...stylex.props(styles.playerSeek)} />
          <span>{size != null ? `${(size / 1024 / 1024).toFixed(1)} MB` : 'MP4'}</span>
        </div>
      </> : <span {...stylex.props(styles.muted)}>{video ? screenWorkbench.videoPlaybackError : 'File video không còn trên máy.'}</span>}
    </div>
    <div {...stylex.props(styles.facts)}>
      {score && <div {...stylex.props(styles.fact)}><Icon name="tick-b" size={15} xstyle={styles.okIcon} /><span {...stylex.props(styles.factBody)}><b>Độ hoàn thiện {score.total} / 100</b><small>Kiểm định đã đo trên bản xuất</small></span></div>}
      {shown && <div {...stylex.props(styles.fact)}><Icon name="video-play" size={15} /><span {...stylex.props(styles.factBody)}><b>MP4{seconds != null ? ` ${clockLabel(seconds)}` : ''}{size != null ? ` · ${(size / 1024 / 1024).toFixed(1)} MB` : ''}</b><small>File xuất của video</small></span></div>}
      {cover && <button type="button" onClick={() => onOpenImage(cover.id)} aria-label="Xem ảnh bìa" title="Xem ảnh bìa" {...stylex.props(styles.fact, styles.factButton)}><Icon name="gallery" size={15} /><span {...stylex.props(styles.factBody)}><b>Ảnh bìa của video</b><small>Đi kèm gói đăng · bấm để xem ảnh</small></span></button>}
      <div {...stylex.props(styles.fact)}><Icon name="flag" size={15} xstyle={styles.reviewIcon} /><span {...stylex.props(styles.factBody)}><b>{job.stageGroup === 'posted' ? 'Đã đánh dấu đã đăng' : 'Đăng xong thì đánh dấu đã đăng'}</b><small>Video chuyển sang Đã đăng; file MP4 có thể xoá để lấy chỗ</small></span></div>
    </div>
  </div>
}

const styles = stylex.create({
  root: { flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden', border: '1px solid var(--line-1)', borderRadius: r['--vv-r-14'], backgroundImage: 'var(--card-bg)', boxShadow: 'var(--card-shadow)' },
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
  logBox: { flex: '0 0 auto', borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-0)', border: '1px solid var(--line-1)', paddingBlock: s['--vv-s-4'] },
  logList: { listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column' },
  logRow: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-3']} ${s['--vv-s-10']}`, borderRadius: r['--vv-r-7'] },
  logTime: { flex: '0 0 58px', paddingTop: 3, fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  logIcon: { flex: '0 0 auto', display: 'flex', marginTop: 2 },
  logText: { flex: 1, minWidth: 0, display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-4'], color: 'var(--vv-fg-note)', fontSize: 13, lineHeight: 1.45, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  logDuration: { flex: '0 0 auto', fontStyle: 'italic', fontSize: 12, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  logToggle: { width: '100%', height: 26, marginTop: 2, paddingInline: s['--vv-s-10'], borderWidth: 0, borderRadius: r['--vv-r-7'], backgroundColor: 'transparent', color: 'var(--color-fg-3)', fontFamily: 'inherit', fontSize: 12, display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], cursor: 'pointer' },
  logRot: { transform: 'rotate(180deg)' },
  kbd: { fontFamily: "'JetBrains Mono', monospace", fontSize: 11, color: 'var(--color-fg-3)', paddingBlock: 1, paddingInline: 5, borderRadius: r['--vv-r-4'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-0)' },
  grid: { flex: '0 1 auto', minHeight: 0, display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gridAutoRows: 'min-content', gap: `${s['--vv-s-12']} ${s['--vv-s-10']}`, paddingInlineEnd: s['--vv-s-4'] },
  tile: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], color: 'var(--color-fg-3)', fontSize: 12, cursor: 'pointer', borderRadius: r['--vv-r-10'], ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 } },
  coverTile: { gridColumn: 'span 2', gridRow: 'span 2' },
  tileSelected: { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 },
  preview: { position: 'relative', aspectRatio: '16 / 9', overflow: 'hidden', border: '1px solid var(--line-1)', borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-0)', display: 'grid', placeItems: 'center' },
  coverPreview: { flexGrow: 1 },
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
  summary: { flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'minmax(0, 1.6fr) minmax(0, 1fr)', gap: s['--vv-s-14'] },
  summaryCover: { position: 'relative', aspectRatio: '16 / 9', alignSelf: 'start', minHeight: 160, borderRadius: r['--vv-r-12'], overflow: 'hidden', border: '1px solid var(--line-1)', backgroundColor: 'var(--color-surface-0)', display: 'flex', alignItems: 'center', justifyContent: 'center' },
  summaryChip: { position: 'absolute', left: s['--vv-s-10'], top: s['--vv-s-10'], height: 20, paddingInline: s['--vv-s-7'], display: 'flex', alignItems: 'center', borderRadius: r['--vv-r-6'], backgroundColor: 'var(--vv-img-badge)', fontSize: 11.5, color: 'var(--color-fg)' },
  done: { flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'minmax(0, 1.6fr) minmax(0, 1fr)', gap: s['--vv-s-14'] },
  playerBox: { position: 'relative', minHeight: 0, aspectRatio: '16 / 9', alignSelf: 'start', borderRadius: r['--vv-r-12'], overflow: 'hidden', border: '1px solid var(--line-1)', backgroundColor: 'var(--color-surface-0)', display: 'flex', alignItems: 'center', justifyContent: 'center' },
  video: { display: 'block', width: '100%', height: '100%', objectFit: 'contain', backgroundColor: 'var(--color-surface-2)' },
  playerBar: { position: 'absolute', left: 0, right: 0, bottom: 0, padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], background: 'linear-gradient(0deg, rgba(11,11,13,.85), transparent)', fontSize: 12, color: 'var(--color-fg-2)' },
  playerToggle: { position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%, -50%)', width: s['--vv-s-56'], height: s['--vv-s-56'], padding: 0, borderRadius: r['--vv-r-pill'], border: '1px solid var(--line-5)', backgroundColor: 'var(--vv-img-badge)', color: 'var(--color-fg)', display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', ':hover': { backgroundColor: 'var(--color-surface-2)' }, ':focus-visible': { outline: '2px solid var(--color-fg)', outlineOffset: 2 } },
  playerSeek: { flex: 1, minWidth: 0, height: s['--vv-s-14'], margin: 0, accentColor: 'var(--color-fg)', cursor: 'pointer' },
  num: { fontVariantNumeric: 'tabular-nums', fontFamily: "'JetBrains Mono', monospace" },
  facts: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], minWidth: 0 },
  fact: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], padding: `${s['--vv-s-9']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-0)', border: '1px solid var(--line-1)', color: 'var(--color-fg-3)' },
  factBody: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, fontSize: 12.5, lineHeight: 1.45 },
  factButton: { width: '100%', textAlign: 'left', font: 'inherit', cursor: 'pointer', ':hover': { backgroundColor: 'var(--color-surface-1)' }, ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 } },
  reviewIcon: { color: 'var(--vv-st-review)' },

  // Cột phải Trình xem ảnh (chế độ Xem).
  viewerNote: { flex: '1 1 220px', minWidth: 0, fontSize: 12.5, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  viewerSide: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-16'], minWidth: 0 },
  viewerSideHead: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  viewerSideTitle: { fontSize: 14, fontWeight: 600 },
  viewerSideSub: { fontSize: 12.5, color: 'var(--vv-fg-note)' },
  viewerSection: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], minWidth: 0 },
  viewerSectionHead: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], flexWrap: 'wrap' },
  viewerSectionTitle: { flex: 1, fontSize: 11, fontWeight: 600, letterSpacing: '.07em', textTransform: 'uppercase', color: 'var(--vv-fg-note)' },
  viewerVersions: { display: 'flex', gap: s['--vv-s-6'], flexWrap: 'wrap' },
  viewerVer: { position: 'relative', width: 64, aspectRatio: '16 / 9', padding: 0, borderRadius: r['--vv-r-7'], overflow: 'hidden', border: '2px solid transparent', backgroundColor: 'var(--color-surface-1)', flex: '0 0 auto', cursor: 'pointer' },
  viewerVerOn: { borderColor: 'var(--vv-red-line)' },
  viewerVerCancelled: { opacity: 0.45, cursor: 'not-allowed' },
  viewerVerImage: { position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' },
  viewerVerN: { position: 'absolute', left: s['--vv-s-3'], bottom: s['--vv-s-2'], paddingInline: s['--vv-s-4'], borderRadius: r['--vv-r-4'], backgroundColor: 'var(--vv-glass)', color: 'var(--color-fg-2)', fontSize: 10, fontVariantNumeric: 'tabular-nums' },
  viewerVerUsed: { position: 'absolute', right: s['--vv-s-3'], bottom: s['--vv-s-3'], width: 7, height: 7, borderRadius: '50%', backgroundColor: 'var(--color-ok)', boxShadow: '0 0 0 2px var(--color-surface-0)' },
  viewerUseButton: { alignSelf: 'flex-start' },
  viewerVerNote: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5 },
  viewerPrompt: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  viewerPart: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  viewerPartLabel: { fontSize: 10.5, fontWeight: 600, letterSpacing: '.05em', textTransform: 'uppercase', color: 'var(--vv-fg-note)' },
  viewerPartSource: { textTransform: 'none', letterSpacing: 0, fontWeight: 400, color: 'var(--color-fg-4)' },
  viewerPartText: { fontSize: 12.5, color: 'var(--color-fg-2)', lineHeight: 1.55, whiteSpace: 'pre-wrap' },
  viewerRaw: { margin: 0, maxHeight: 180, overflow: 'auto', padding: s['--vv-s-10'], borderRadius: r['--vv-r-10'], border: '1px solid var(--line-1)', backgroundColor: 'var(--color-surface-0)', color: 'var(--color-fg-2)', fontSize: 12, lineHeight: 1.55, whiteSpace: 'pre-wrap' },
  viewerMeta: { fontSize: 12, color: 'var(--vv-fg-note)' },
  viewerRefs: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  viewerRefCard: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], padding: s['--vv-s-6'], borderRadius: r['--vv-r-10'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-1)' },
  viewerRefThumb: { width: 64, aspectRatio: '16 / 9', objectFit: 'cover', borderRadius: r['--vv-r-6'], flex: '0 0 auto', backgroundColor: 'var(--color-surface-3)' },
  viewerRefText: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], minWidth: 0 },
  viewerRefLabel: { fontSize: 12.5, fontWeight: 500 },
  viewerRefSource: { fontSize: 12, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  viewerFeedback: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: s['--vv-s-10'], borderRadius: r['--vv-r-12'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-0)' },
  viewerFeedbackHint: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5 },
  viewerError: { fontSize: 12.5, color: 'var(--vv-warn-ink)' },
  viewerBanner: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], padding: s['--vv-s-10'], borderRadius: r['--vv-r-12'], border: '1px solid var(--vv-ok-20)', backgroundColor: 'var(--vv-ok-6)' },
  viewerBannerTitle: { fontSize: 13, fontWeight: 600, color: 'var(--color-ok)' },
  viewerBannerText: { fontSize: 12, color: 'var(--color-fg-2)', lineHeight: 1.5 },
  viewerNarration: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  viewerNarrationTitle: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], fontSize: 12.5, color: 'var(--vv-fg-note)', cursor: 'pointer' },
  viewerNarrationText: { margin: 0, fontSize: 12.5, color: 'var(--color-fg-2)', lineHeight: 1.55 },
})
