// Ảnh của cảnh trong Editor — xem bản, đọc prompt đã gửi, góp ý hoặc sửa phần
// Cảnh rồi vẽ lại tại chỗ (Gói V29 · artboard Editor › focus=scene).
//
// Bảng rộng 316px: ảnh 16:9, dải Bản, Prompt đã gửi (4 dòng, Xem đủ, Sửa phần
// Cảnh), ảnh tham chiếu thu nhỏ, rồi Góp ý + Vẽ lại / Thay ảnh. Bản đang vẽ dùng
// chung DevelopEffect; vẽ xong Editor tự thay ảnh vào timeline (scene-images.ts)
// và báo ở khung xem, nên ở đây chỉ hiện đúng bản và trạng thái.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon } from '@noverse/ui'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { DevelopEffect } from '../../components'
import { cancelImageVersion, redrawImages, type ImageProgress, type ImageQuota, type ImageVersion, type JobImage } from '../../api/images'
import { paths } from '../../router'
import { MediaAsset, MediaPicker, panelError, panelTime } from './ContentPanels'
import { drawingDetail, drawingStep, drawingVersionOf, needsReviewOf, useSceneSwapStore, useSceneVersion, versionOf } from './scene-images'
import { inspectorStyles as sx } from './Inspector.stylex'
import type { ImageItem } from '@editor/types/timeline'

export interface SceneImagePanelProps {
  item: ImageItem
  number: number
  jobId?: string
  image?: JobImage
  progress?: ImageProgress
  /** Hạn mức ảnh của nhà cung cấp — nút vẽ lại ghi giá thật. */
  quota?: ImageQuota
  /** Lưu video trước khi tốn một lượt vẽ. */
  beforeMutation?: () => Promise<boolean>
  onRedrawn?: () => Promise<void> | void
}

export function SceneImagePanel({ item, number, jobId, image, progress, quota, beforeMutation, onRedrawn }: SceneImagePanelProps) {
  const fps = useTimelineSettingsStore(state => state.fps)
  const [picked, setPicked] = useState<number | null>(null)
  const [more, setMore] = useState(false)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<string | null>(null)
  const [feedback, setFeedback] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [picker, setPicker] = useState(false)
  const notices = useSceneSwapStore(state => state.notices)
  const used = image?.usedVersion ?? 1
  const selected = picked ?? used
  const drawing = image ? drawingVersionOf(image) : undefined
  const version = image ? versionOf(image, selected) ?? versionOf(image, used) : undefined
  const sceneText = version?.parts.find(part => part.kind === 'scene')?.text ?? version?.sentText ?? ''
  const refs = version?.refs ?? []
  const changed = draft !== null && draft.trim() !== sceneText.trim()
  const justSwapped = notices.some(notice => notice.clipId === item.id)
  const review = image ? needsReviewOf(image) : false
  useEffect(() => {
    // Bản đang dùng đổi (vẽ xong, hoặc Dùng bản N) thì quay về xem đúng bản đó,
    // nhưng giữ nguyên prompt người dùng đang sửa dở — vẽ xong không được xoá.
    setPicked(null)
  }, [used, image?.id])
  const run = async (request: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true); setError('')
    try {
      if (beforeMutation && !await beforeMutation()) throw new Error('Chưa lưu được video. Hãy lưu trước khi vẽ lại ảnh.')
      await request()
      await onRedrawn?.()
    } catch (cause) { setError(panelError(cause)) }
    finally { setBusy(false) }
  }
  const redraw = (body: { feedback?: string; parts?: { scene: string } }) => {
    if (!jobId || !image) return
    void run(async () => {
      await redrawImages(jobId, { imageIds: [image.id], project: false, ...body })
      setFeedback(''); setEditing(false); setDraft(null)
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
      if (notice) useSceneSwapStore.getState().push(notice)
    })
  }
  const hint = !image
    ? 'Chưa có dữ liệu ảnh cho cảnh này. Thay ảnh để chọn ảnh khác trong Media.'
    : drawing
      ? 'Bạn chỉnh tiếp editor được. Vẽ xong, ảnh mới tự thay vào timeline; chuyển động, keyframe và thời lượng giữ nguyên.'
      : review
        ? `Ảnh mới chưa duyệt: Độ hoàn thiện kiểm lại ảnh này trước khi xuất. ⌘Z để quay lại bản ${Math.max(1, used - 1)}.`
        : `Vẽ lại tốn 1 ảnh${priceOf(quota)}. Ảnh mới thay vào timeline, giữ chuyển động và keyframe.`
  const status = !image
    ? null
    : review
      ? { text: 'Bản mới · cần duyệt lại', warn: true }
      : { text: 'Đã duyệt', warn: false }
  return <>
    <section {...stylex.props(sx.section, sx.spacious)}>
      <h2 {...stylex.props(sx.heading)}>
        <span {...stylex.props(sx.sceneIcon)}><Icon name="gallery" size={15} /></span>
        <span {...stylex.props(sx.headingText)} title={item.label}>Cảnh {number} · {item.label}</span>
      </h2>
      <span {...stylex.props(sx.meta)}>
        {panelTime(item.from / fps)}–{panelTime((item.from + item.durationInFrames) / fps)} · dài {panelTime(item.durationInFrames / fps)}
        {status && <> · <span {...stylex.props(status.warn ? sx.warn : sx.ok)}>{status.text}</span></>}
      </span>
      <div {...stylex.props(sx.ipBox, drawing ? sx.ipBoxDev : null)}>
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
      </div>
      {image && image.versions.length > 0 && <div {...stylex.props(sx.verStrip)}>
        <span {...stylex.props(sx.verLead)}>Bản</span>
        {image.versions.map(entry => <VersionTile
          key={entry.n}
          version={entry}
          used={entry.n === used}
          pressed={drawing ? entry.n === drawing.n : entry.n === selected}
          drawing={entry.n === drawing?.n}
          baseUrl={versionOf(image, used)?.url ?? undefined}
          onPick={() => setPicked(entry.n === used ? null : entry.n)}
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
              <label htmlFor="v13-scene-part" {...stylex.props(sx.srOnly)}>Phần Cảnh của prompt</label>
              <textarea
                id="v13-scene-part"
                rows={6}
                value={draft ?? sceneText}
                onChange={event => setDraft(event.currentTarget.value)}
                onKeyDown={event => { if (event.key === 'Escape') { setEditing(false); setDraft(null) } }}
                {...stylex.props(sx.psegArea, changed ? sx.psegAreaChanged : null)}
              />
              <span {...stylex.props(sx.help)}>4 phần còn lại (Phong cách, Tham chiếu, Ghi chú đã học, Tránh) giữ nguyên · <a href={`#${paths.review({ video: jobId })}`} {...stylex.props(sx.psegLink)}>sửa đủ ở Duyệt</a></span>
            </div>
            <div {...stylex.props(sx.editFooter)}>
              <Button disabled={busy} onClick={() => { setEditing(false); setDraft(null) }}>Huỷ</Button>
              <Button variant="primary" pending={busy} disabled={!changed || !!drawing} onClick={() => redraw({ parts: { scene: (draft ?? '').trim() } })} icon={<Icon name="magicpen" size={15} />}>Vẽ lại với prompt này</Button>
            </div>
          </>
        : <>
            <div {...stylex.props(sx.promptCard)}>
              <span {...stylex.props(sx.promptHead)}>
                <span {...stylex.props(sx.promptTitle)}>Prompt đã gửi</span>
                <Button size="sm" variant="quiet" onClick={() => setMore(value => !value)}>{more ? 'Thu gọn' : 'Xem đủ'}</Button>
                <Button size="sm" onClick={() => { setEditing(true); setDraft(sceneText) }} icon={<Icon name="edit" size={13} />}>Sửa</Button>
              </span>
              <span {...stylex.props(sx.promptText, more ? null : sx.promptClamp)}>{sceneText || 'Chưa có prompt đã gửi cho bản này.'}</span>
              <span {...stylex.props(sx.promptRefs)}>
                {refs.map(ref => <img key={ref.id} src={ref.url} alt="" title={`${ref.source.label} · ${ref.role}`} {...stylex.props(sx.refThumb)} />)}
                <span>{refs.length ? `${refs.length} ảnh tham chiếu` : 'Không dùng ảnh tham chiếu'}</span>
                <span {...stylex.props(sx.grow)} />
                <a href={`#${paths.review({ video: jobId })}`} {...stylex.props(sx.promptLink)}>Mở ở Duyệt</a>
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
                <Button variant="primary" pending={busy} disabled={!jobId} onClick={() => redraw(feedback.trim() ? { feedback: feedback.trim() } : {})} icon={<Icon name="magicpen" size={15} />}>Vẽ lại</Button>
                <Button disabled={busy} onClick={() => setPicker(true)} icon={<Icon name="gallery" size={15} />}>Thay ảnh</Button>
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
  const note = version.state === 'drawing'
    ? 'đang vẽ'
    : version.feedback
      ? `Theo góp ý “${version.feedback}”`
      : version.n === 1 ? 'Lần đầu · Hoạ sĩ viết từ lời kể' : `vẽ lúc ${startedLabel(version.startedAt)}`
  return <button
    type="button"
    aria-pressed={pressed}
    aria-label={`Bản ${version.n} · ${note}${used ? ' · đang dùng' : ''}`}
    title={`Bản ${version.n} · ${note}${used ? ' · đang dùng' : ''}`}
    onClick={onPick}
    {...stylex.props(sx.verTile, pressed ? sx.verTileOn : null)}
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
