// Ảnh của cảnh trong Editor — xem bản, đọc prompt đã gửi, góp ý hoặc sửa phần
// Cảnh rồi vẽ lại tại chỗ (Gói V29 · artboard Editor › focus=scene).
//
// Bảng rộng 316px: ảnh 16:9, dải Bản, Prompt đã gửi (4 dòng, Xem đủ, Sửa phần
// Cảnh), ảnh tham chiếu thu nhỏ, rồi Góp ý + Vẽ lại / Thay ảnh. Bản đang vẽ dùng
// chung DevelopEffect; vẽ xong Editor tự thay ảnh vào timeline (scene-images.ts)
// và báo ở khung xem, nên ở đây chỉ hiện đúng bản và trạng thái.
import { useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, confirm, toast } from '@noverse/ui'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { ImageViewer } from '../../components/image-viewer/ImageViewer'
import { DevelopEffect } from '../../components'
import { cancelImageVersion, redrawImages, type ImageProgress, type ImageQuota, type ImageVersion, type JobImage } from '../../api/images'
import { paths } from '../../router'
import { MediaAsset, MediaPicker, panelError, panelTime } from './ContentPanels'
import { clipIdFor, drawingDetail, drawingStep, drawingVersionOf, needsReviewOf, useSceneSwapStore, useSceneVersion, versionOf } from './scene-images'
import { inspectorStyles as sx } from './Inspector.stylex'
import type { ImageItem } from '@editor/types/timeline'

export interface SceneImagePanelProps {
  item: ImageItem
  number: number
  jobId?: string
  image?: JobImage
  images?: readonly JobImage[]
  imageProgress?: Record<string, ImageProgress>
  progress?: ImageProgress
  /** Hạn mức ảnh của nhà cung cấp — nút vẽ lại ghi giá thật. */
  quota?: ImageQuota
  /** Lưu video trước khi tốn một lượt vẽ. */
  beforeMutation?: () => Promise<boolean>
  onRedrawn?: () => Promise<void> | void
}

type SceneDraft = { imageId: string; feedback: string; draft: string | null; editing: boolean; version: number | null }

/** Bản đã huỷ vẫn nằm trong lịch sử nhưng không còn là nguồn vẽ lại hợp lệ. */
const selectableVersion = (version: ImageVersion | undefined): ImageVersion | undefined =>
  version && version.state !== 'cancelled' ? version : undefined

export function SceneImagePanel(props: SceneImagePanelProps) {
  const [viewing, setViewing] = useState<string | null>(null)
  const [pickedVersion, setPickedVersion] = useState<number | null>(null)
  const [replacement, setReplacement] = useState<ImageItem | null>(null)
  const [resume, setResume] = useState<SceneDraft | null>(null)
  const projectId = useProjectStore(state => state.currentProject?.id)
  const itemById = useItemsStore(state => state.itemById)
  const images = props.images ?? (props.image ? [props.image] : [])
  const target = images.find(image => image.id === viewing)
  const clipId = target && projectId ? clipIdFor(projectId, target) : null
  const clip = (clipId ? itemById[clipId] : undefined) ?? (viewing === props.image?.id || viewing === props.item.id ? props.item : undefined)
  const targetClip = clip?.type === 'image' ? clip : undefined
  const usedSource = target ? versionOf(target, target.usedVersion) : undefined
  const sourceVersion = target
    ? selectableVersion(pickedVersion != null ? versionOf(target, pickedVersion) : undefined)
      ?? selectableVersion(usedSource)
      ?? target.versions.findLast(version => version.state !== 'cancelled')
    : undefined
  useEffect(() => {
    if (!viewing || targetClip) return
    setViewing(null)
    toast.err('Cảnh đã chọn không còn trong video. Chưa thay đổi ảnh nào.')
  }, [viewing, targetClip])
  const options = images.length ? images.map(image => {
    const mapped = projectId ? clipIdFor(projectId, image) : null
    const ready = versionOf(image, image.usedVersion)
    return { id: image.id, label: image.label, badge: image.kind === 'cover' ? 'Bìa' : String(image.index), url: image.id === viewing ? sourceVersion?.url : ready?.url, disabled: (!(mapped && itemById[mapped]?.type === 'image') && image.id !== props.image?.id) || !ready?.url || !!drawingVersionOf(image) }
  }) : [{ id: props.item.id, label: props.item.label, url: props.item.src }]
  const closeViewer = () => { setResume(null); setPickedVersion(null); setViewing(null) }
  const confirmRedraw = async (draft: SceneDraft, request: () => Promise<unknown>) => {
    const returnTo = viewing
    setResume(returnTo ? draft : null)
    flushSync(() => setViewing(null))
    const accepted = await confirm({
      title: 'Vẽ lại ảnh của cảnh này?',
      body: <p>Tốn 1 ảnh{priceOf(props.quota)}. Bản đang dùng được giữ tới khi ảnh mới xong; video chỉ thay ảnh của cảnh đã chọn.</p>,
      confirmLabel: 'Vẽ lại',
      onConfirm: async () => { await request() },
    })
    if (accepted) { setResume(null); toast.ok('Đã gửi Hoạ sĩ · vẽ lại 1 ảnh') }
    else if (returnTo) { setPickedVersion(draft.version); setViewing(returnTo) }
    return accepted
  }
  return <>
    <SceneImageDetails {...props} initialDraft={resume} confirmRedraw={confirmRedraw} onPreview={version => { setPickedVersion(version); setViewing(props.image?.id ?? props.item.id) }} />
    {viewing && targetClip && <ImageViewer mode="view-replace" title={target?.label ?? props.item.label} items={options} selectedId={viewing} onSelect={id => { setResume(null); setPickedVersion(null); setViewing(id) }} onClose={closeViewer}
      side={<SceneImageDetails key={viewing} {...props} item={targetClip} image={target} progress={target ? props.imageProgress?.[target.id] : undefined} initialDraft={resume} confirmRedraw={confirmRedraw} hideHero viewVersion={pickedVersion} onViewVersion={setPickedVersion}
        hideReplace />}
      footer={<><span>Chỉ thay ảnh của cảnh này · giữ chuyển động, thời lượng và điểm mốc</span><Button variant="primary" onClick={() => { flushSync(closeViewer); setReplacement(targetClip) }}>Thay ảnh</Button></>}
    />}
    {replacement && <MediaPicker item={replacement} onClose={() => setReplacement(null)} />}
  </>
}

function SceneImageDetails({ item, number, jobId, image, progress, quota, beforeMutation, onRedrawn, onPreview, hideHero, viewVersion, onViewVersion, hideReplace, initialDraft, confirmRedraw }: SceneImagePanelProps & {
  onPreview?: (version: number) => void
  hideHero?: boolean
  viewVersion?: number | null
  onViewVersion?: (version: number | null) => void
  hideReplace?: boolean
  initialDraft: SceneDraft | null
  confirmRedraw: (draft: SceneDraft, request: () => Promise<unknown>) => Promise<boolean>
}) {
  const fps = useTimelineSettingsStore(state => state.fps)
  const [picked, setPicked] = useState<number | null>(null)
  const [more, setMore] = useState(false)
  const [editing, setEditing] = useState(initialDraft && initialDraft.imageId === image?.id ? initialDraft.editing : false)
  const [draft, setDraft] = useState<string | null>(initialDraft && initialDraft.imageId === image?.id ? initialDraft.draft : null)
  const [editSource, setEditSource] = useState<number | null>(initialDraft && initialDraft.imageId === image?.id ? initialDraft.version : null)
  const [feedback, setFeedback] = useState(initialDraft && initialDraft.imageId === image?.id ? initialDraft.feedback : '')
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)
  const [error, setError] = useState('')
  const [picker, setPicker] = useState(false)
  const notices = useSceneSwapStore(state => state.notices)
  const used = image?.usedVersion ?? 1
  const usedEntry = image ? versionOf(image, used) : undefined
  const explicit = editing && editSource != null ? editSource : onViewVersion ? viewVersion : picked
  // Lựa chọn cũ trỏ vào bản đã huỷ (hoặc bản đã biến mất) thì rơi về bản đang
  // dùng, rồi tới bản còn dùng được mới nhất — không giữ nguồn bị từ chối.
  const selectedEntry = image
    ? selectableVersion(explicit != null ? versionOf(image, explicit) : undefined)
      ?? selectableVersion(usedEntry)
      ?? image.versions.findLast(entry => entry.state !== 'cancelled')
    : undefined
  const selected = selectedEntry?.n ?? used
  const drawing = image ? drawingVersionOf(image) : undefined
  const version = selectedEntry ?? selectableVersion(usedEntry)
  const sceneText = version?.parts.find(part => part.kind === 'scene')?.text ?? version?.sentText ?? ''
  const editableScene = version && !version.partsLegacy ? version.parts.find(part => part.kind === 'scene')?.text ?? null : null
  const pinnedEdit = image && editing && editSource != null ? selectableVersion(versionOf(image, editSource)) : undefined
  // Bản nguồn của bản nháp không còn dùng được (đã huỷ/biến mất) thì không tự
  // lấy bản khác để gửi kèm bản nháp: giữ nháp và khoá gửi cho tới khi chọn lại.
  const editSourceStale = editing && editSource != null && !pinnedEdit
  const editVersion = editing && editSource != null ? pinnedEdit : version
  // So bản nháp với đúng bản nguồn lúc bắt đầu sửa, không phải bản SSE vừa chọn.
  const editSceneText = editVersion?.parts.find(part => part.kind === 'scene')?.text ?? editVersion?.sentText ?? ''
  const refs = version?.refs ?? []
  const changed = draft !== null && draft.trim() !== editSceneText.trim()
  const justSwapped = notices.some(notice => notice.clipId === item.id)
  const review = image ? needsReviewOf(image) : false
  useEffect(() => {
    // Bản đang dùng đổi (vẽ xong, hoặc Dùng bản N) thì quay về xem đúng bản đó,
    // nhưng giữ nguyên prompt người dùng đang sửa dở — vẽ xong không được xoá.
    setPicked(null)
  }, [used, image?.id])
  const run = async (request: () => Promise<unknown>) => {
    if (pending.current) return
    pending.current = true
    setBusy(true); setError('')
    try {
      if (beforeMutation && !await beforeMutation()) throw new Error('Chưa lưu được video. Hãy lưu trước khi vẽ lại ảnh.')
      await request()
      await onRedrawn?.()
    } catch (cause) { setError(panelError(cause)) }
    finally { pending.current = false; setBusy(false) }
  }
  const redraw = (body: { feedback?: string; parts?: { scene: string } }) => {
    if (!jobId || !image) return
    // Chốt bản nguồn tại lúc bấm gửi: bản đã chốt khi bắt đầu sửa prompt, hoặc
    // bản đang xem với góp ý. SSE đổi bản dùng trong lúc chờ không đổi nguồn.
    const source = editing ? editVersion : version
    // Bản đã huỷ không phải nguồn hợp lệ: không mở xác nhận hay gửi gì.
    if (source?.state === 'cancelled') return
    // Bản nháp đang sửa mất nguồn thì không gửi kèm một bản khác.
    if (editing && editSourceStale) return
    const baseVersion = source?.n ?? null
    void run(async () => {
      const accepted = await confirmRedraw({ imageId: image.id, feedback, draft, editing, version: baseVersion }, () => redrawImages(jobId, { imageIds: [image.id], project: false, ...(baseVersion != null ? { baseVersion } : {}), ...body }))
      if (accepted) { setFeedback(''); setEditing(false); setDraft(null); setEditSource(null) }
    })
  }
  const cancel = () => {
    if (!jobId || !image || !drawing) return
    void run(async () => { await cancelImageVersion(jobId, image.id, drawing.n) })
  }
  const usePicked = () => {
    if (!jobId || !image || selected === used) return
    void run(async () => {
      const notice = await useSceneVersion(jobId, image.id, selected)
      setPicked(null); onViewVersion?.(null)
      if (notice) useSceneSwapStore.getState().push(notice)
    })
  }
  const hint = !image
    ? 'Chưa có dữ liệu ảnh cho cảnh này. Thay ảnh để chọn ảnh khác trong Media.'
    : drawing
      ? 'Bạn chỉnh tiếp trong Biên tập được. Vẽ xong, ảnh mới tự thay vào dòng thời gian; chuyển động, điểm mốc và thời lượng giữ nguyên.'
      : review
        ? `Ảnh mới chưa duyệt: Độ hoàn thiện kiểm lại ảnh này trước khi xuất. ⌘Z để quay lại bản ${Math.max(1, used - 1)}.`
        : `Vẽ lại tốn 1 ảnh${priceOf(quota)}. Ảnh mới thay vào dòng thời gian, giữ chuyển động và điểm mốc.`
  const status = !image
    ? null
    : review
      ? { text: 'Bản mới · cần duyệt lại', warn: true }
      : { text: 'Đã duyệt', warn: false }
  return <>
    <section {...stylex.props(sx.section, sx.spacious)}>
      <h2 {...stylex.props(sx.heading)}>
        <span {...stylex.props(sx.sceneIcon)}><Icon name="gallery" size={15} /></span>
        <span {...stylex.props(sx.headingText)} title={item.label}>{image?.label ?? `Cảnh ${number}`} · {item.label}</span>
      </h2>
      <span {...stylex.props(sx.meta)}>
        {panelTime(item.from / fps)}–{panelTime((item.from + item.durationInFrames) / fps)} · dài {panelTime(item.durationInFrames / fps)}
        {status && <> · <span {...stylex.props(status.warn ? sx.warn : sx.ok)}>{status.text}</span></>}
      </span>
      {!hideHero && <div {...stylex.props(sx.ipBox, drawing ? sx.ipBoxDev : null)}>
        {image && drawing
          ? <DevelopEffect
              state="drawing"
              variant="inline"
              baseUrl={versionOf(image, used)?.url ?? item.src}
              title={`Hoạ sĩ đang vẽ bản ${drawing.n}`}
              detail={drawingDetail(progress, image)}
              step={drawingStep(progress)}
            />
          : version?.url
            ? justSwapped
              ? <DevelopEffect state="done"><img src={version.url} alt={`Cảnh ${number} · bản ${version.n}`} {...stylex.props(sx.ipMedia)} /></DevelopEffect>
              : <img src={version.url} alt={`Cảnh ${number} · bản ${version.n}`} {...stylex.props(sx.ipMedia)} />
            : <MediaAsset mediaId={item.mediaId} src={item.src} label={item.label} />}
        {image && <span {...stylex.props(sx.ipBadge, selected !== used && !drawing ? sx.ipBadgeWarn : null)}>
          {drawing ? `Bản ${used} · đang vẽ bản ${drawing.n}` : `Bản ${selected}${selected === used ? ' · đang dùng' : ' · chưa dùng'}`}
        </span>}
        {!drawing && onPreview && (version?.url || item.src) && <button type="button" aria-label={`Xem lớn ${item.label}`} onClick={() => onPreview(selected)} {...stylex.props(viewerStyles.open)} />}
      </div>}
      {image && image.versions.length > 0 && <div {...stylex.props(sx.verStrip)}>
        <span {...stylex.props(sx.verLead)}>Bản</span>
        {image.versions.map(entry => <VersionTile
          key={entry.n}
          version={entry}
          used={entry.n === used}
          pressed={drawing ? entry.n === drawing.n : entry.n === selected}
          drawing={entry.n === drawing?.n}
          baseUrl={versionOf(image, used)?.url ?? undefined}
          onPick={() => { const next = entry.n === used ? null : entry.n; setEditing(false); setDraft(null); setEditSource(null); setPicked(next); onViewVersion?.(next) }}
        />)}
        <span {...stylex.props(sx.grow)} />
        {selected !== used && !drawing && <Button size="sm" pending={busy} onClick={usePicked}>Dùng bản {selected}</Button>}
      </div>}
      {image && (editing
        ? <>
            <div {...stylex.props(sx.pseg)}>
              <span {...stylex.props(sx.psegHead)}>
                Cảnh<span {...stylex.props(sx.psegSrc)}>· Hoạ sĩ viết từ lời kể</span>
                <span {...stylex.props(sx.grow)} />
                {changed && <span {...stylex.props(sx.psegChanged)}>Đã sửa</span>}
              </span>
              <label htmlFor="v13-scene-part" {...stylex.props(sx.srOnly)}>Phần Cảnh của chỉ dẫn</label>
              <textarea
                id="v13-scene-part"
                rows={6}
                value={draft ?? sceneText}
                onChange={event => setDraft(event.currentTarget.value)}
                onKeyDown={event => { if (event.key === 'Escape') { setEditing(false); setDraft(null); setEditSource(null) } }}
                {...stylex.props(sx.psegArea, changed ? sx.psegAreaChanged : null)}
              />
              <span {...stylex.props(sx.help)}>Phong cách, Tham chiếu và Tránh giữ nguyên · <a href={`#${paths.review({ video: jobId, img: image.id })}`} {...stylex.props(sx.psegLink)}>sửa đủ ở Duyệt</a></span>
            </div>
            <div {...stylex.props(sx.editFooter)}>
              <Button disabled={busy} onClick={() => { setEditing(false); setDraft(null); setEditSource(null) }}>Huỷ</Button>
              <Button variant="primary" pending={busy} aria-disabled={!changed || !!drawing || editSourceStale} title={drawing ? 'Đang vẽ một bản khác; chờ xong hoặc huỷ lần vẽ.' : editSourceStale ? 'Bản đang sửa không còn dùng được; huỷ để chọn bản khác.' : !changed ? 'Sửa phần Cảnh trước khi vẽ lại.' : 'Vẽ lại bằng chỉ dẫn vừa sửa'} onClick={() => { if (changed && !drawing && !editSourceStale) redraw({ parts: { scene: (draft ?? '').trim() } }) }} icon={<Icon name="magicpen" size={15} />}>Vẽ lại với chỉ dẫn này</Button>
            </div>
          </>
        : <>
            <div {...stylex.props(sx.promptCard)}>
              <span {...stylex.props(sx.promptHead)}>
                <span {...stylex.props(sx.promptTitle)}>Prompt đã gửi</span>
                <Button size="sm" variant="quiet" onClick={() => setMore(value => !value)}>{more ? 'Thu gọn' : 'Xem đủ'}</Button>
                <Button size="sm" disabled={editableScene == null} onClick={() => { if (editableScene == null) return; setEditing(true); setEditSource(selected); setDraft(editableScene) }} icon={<Icon name="edit" size={13} />}>Sửa</Button>
              </span>
              <span {...stylex.props(sx.promptText, more ? null : sx.promptClamp)}>{sceneText || 'Chưa có chỉ dẫn nào được gửi cho bản này.'}</span>
              <span {...stylex.props(sx.promptRefs)}>
                {refs.map(ref => <img key={ref.id} src={ref.url} alt="" title={`${ref.source.label} · ${ref.role}`} {...stylex.props(sx.refThumb)} />)}
                <span>{refs.length ? `${refs.length} ảnh tham chiếu` : 'Không dùng ảnh tham chiếu'}</span>
                <span {...stylex.props(sx.grow)} />
                <a href={`#${paths.review({ video: jobId, img: image.id })}`} {...stylex.props(sx.promptLink)}>Mở ở Duyệt</a>
              </span>
            </div>
            {!drawing && <div {...stylex.props(sx.compose)}>
              <label htmlFor="v13-scene-feedback" {...stylex.props(sx.srOnly)}>Góp ý để vẽ lại cảnh {number}</label>
              <input
                id="v13-scene-feedback"
                type="text"
                placeholder="Góp ý để vẽ lại, ví dụ: thêm bóng người cạnh cửa sổ"
                value={feedback}
                onChange={event => setFeedback(event.currentTarget.value)}
                onKeyDown={event => { if (event.key === 'Enter' && feedback.trim()) redraw({ feedback: feedback.trim() }) }}
                {...stylex.props(sx.composeField)}
              />
              <div {...stylex.props(sx.twoColumns)}>
                <Button variant="primary" pending={busy} aria-disabled={!jobId} title={!jobId ? 'Video chưa liên kết với lượt làm video nên chưa vẽ lại được.' : 'Vẽ lại ảnh cảnh này, giữ chuyển động và điểm mốc'} onClick={() => { if (jobId) redraw(feedback.trim() ? { feedback: feedback.trim() } : {}) }} icon={<Icon name="magicpen" size={15} />}>Vẽ lại</Button>
                {!hideReplace && <Button disabled={busy} onClick={() => setPicker(true)} icon={<Icon name="gallery" size={15} />}>Thay ảnh</Button>}
              </div>
            </div>}
          </>)}
      {image && drawing && <Button disabled={busy} onClick={cancel}>Huỷ lần vẽ</Button>}
      <span {...stylex.props(sx.help)}>{hint}</span>
      {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
    </section>
    {picker && <MediaPicker item={item} onClose={() => setPicker(false)} />}
  </>
}

function VersionTile({ version, used, pressed, drawing, baseUrl, onPick }: {
  version: ImageVersion
  used: boolean
  pressed: boolean
  drawing: boolean
  /** Ảnh của bản đang dùng — nền mờ dưới hiệu ứng khi bản này đang vẽ. */
  baseUrl?: string
  onPick: () => void
}) {
  const cancelled = version.state === 'cancelled'
  const note = cancelled
    ? 'đã huỷ · không dùng làm nguồn vẽ lại'
    : version.state === 'drawing'
      ? 'đang vẽ'
      : version.feedback
        ? `Theo góp ý “${version.feedback}”`
        : version.n === 1 ? 'Lần đầu · Hoạ sĩ viết từ lời kể' : `vẽ lúc ${startedLabel(version.startedAt)}`
  const label = `Bản ${version.n} · ${note}${used ? ' · đang dùng' : ''}`
  return <button
    type="button"
    disabled={cancelled}
    aria-pressed={pressed}
    aria-label={label}
    title={label}
    onClick={() => { if (!cancelled) onPick() }}
    {...stylex.props(sx.verTile, pressed ? sx.verTileOn : null)}
    style={cancelled ? { opacity: 0.45, cursor: 'not-allowed' } : undefined}
  >
    {version.url && version.state === 'ready' && <img src={version.url} alt="" {...stylex.props(sx.ipMedia)} />}
    {drawing && <DevelopEffect state="drawing" compact baseUrl={baseUrl} />}
    <span {...stylex.props(sx.verNum)}>{version.n}</span>
    {used && <span aria-hidden="true" {...stylex.props(sx.verUsed)} />}
  </button>
}

/** “(ChatGPT còn 7/50 hôm nay)” — giá của một lượt vẽ, khi máy chủ đã biết hạn mức. */
function priceOf(quota: ImageQuota | undefined): string {
  const provider = quota?.providers.find(entry => entry.left > 0) ?? quota?.providers[0]
  if (!provider) return ''
  return ` (${provider.label} còn ${provider.left}/${provider.total} hôm nay)`
}

/** “23:58” — giờ vẽ của một bản, như nhãn trong artboard. */
function startedLabel(startedAt: string): string {
  const date = new Date(startedAt)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleTimeString('vi', { hour: '2-digit', minute: '2-digit' })
}

const viewerStyles = stylex.create({
  open: {
    position: 'absolute', inset: 0, padding: 0, borderWidth: 0, backgroundColor: 'transparent', cursor: 'zoom-in',
    ':focus-visible': { outlineWidth: 'var(--vv-s-2)', outlineStyle: 'solid', outlineColor: 'var(--vv-text)', outlineOffset: 'calc(-1 * var(--vv-s-2))' },
  },
})
