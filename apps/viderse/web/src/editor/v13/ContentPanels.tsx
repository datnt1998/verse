import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon } from '@noverse/ui'
import { z } from 'zod'
import { executeCommands, type SerializedCommand } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { mediaLibraryService } from '@editor/features/media-library/services/media-library-service'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { serverMediaToMetadata } from '@editor/storage/media-adapter'
import { getProjectRevision } from '@editor/storage/project-persistence'
import { request } from '../../api/request'
import { uploadKindForMime, uploadTooLarge } from '../../api/upload-limits'
import { subscribeEvents } from '../../data/events'
import type { MediaMetadata } from '@editor/types/storage'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import { insertMedia, addOverlay } from '../ui/library/library-actions'
import { contentStyles as sx } from './ContentPanels.stylex'

export type ContentPanel = 'scenes' | 'subs' | 'bg' | 'audio' | 'media'
export interface AudioMeasurement {
  evidence?: string
  measuredAt?: string
  stale?: boolean
}
export interface ContentPanelsProps {
  panel: ContentPanel
  onSelectItem?: (itemId: string) => void
  jobId?: string
  beforeMutation?: () => Promise<boolean>
  onProjectChanged?: () => void | Promise<void>
  measurement?: AudioMeasurement
}

export function editorCommand(commands: SerializedCommand[]): void {
  const result = executeCommands(commands)
  if (!result.ok) {
    const message = result.issues.map(issue => issue.message).filter(text => /[À-ỹ]/u.test(text)).join('; ')
    throw new Error(message || 'Không thể áp dụng thay đổi. Kiểm tra lại mục đang chọn.')
  }
  if (result.ok && result.results.some(value => value === false)) throw new Error('Chưa áp dụng được thay đổi. Kiểm tra thời lượng và các clip liền kề.')
}

export function panelError(error: unknown): string {
  return error instanceof Error && /[À-ỹ]/u.test(error.message)
    ? error.message : 'Không thực hiện được thay đổi. Hãy thử lại.'
}

export function matchesCloneEvent(data: unknown, jobId: string): boolean {
  if (!data || typeof data !== 'object') return true
  const event = data as Record<string, unknown>
  const reference = typeof event.jobId === 'string' ? event.jobId
    : typeof event.ref === 'string' ? event.ref : typeof event.id === 'string' ? event.id : undefined
  return !reference || reference === jobId
}

export function panelTime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  return `${Math.floor(total / 60).toString().padStart(2, '0')}:${(total % 60).toString().padStart(2, '0')}`
}

export function selectEditorItem(item: TimelineItem): void {
  useSelectionStore.getState().selectItems([item.id])
  const playback = usePlaybackStore.getState()
  playback.pause()
  playback.setCurrentFrame(item.from)
}

export function isCaption(item: TimelineItem): item is Extract<TimelineItem, { type: 'text' | 'subtitle' }> {
  return item.type === 'subtitle' || item.type === 'text' && (item.textRole === 'caption' || Boolean(item.words?.length))
}

export type BackendMutationProps = Pick<ContentPanelsProps, 'jobId' | 'beforeMutation' | 'onProjectChanged'>

const backdropItemSchema = z.object({
  id: z.string(), channelId: z.string(), videoId: z.string(), kind: z.string(),
  title: z.string(), startSec: z.number().nonnegative(), endSec: z.number().positive(),
  source: z.enum(['owner', 'random']), status: z.enum(['pending', 'ready', 'failed']),
  mediaId: z.string().nullable(), editorMediaId: z.string().nullable().optional(),
  videoDurationSec: z.number().nullable().optional(),
}).passthrough()
const backdropSchema = z.object({
  revision: z.string(), items: z.array(backdropItemSchema),
  randomPicks: z.boolean(), busy: z.boolean(),
  rebuilding: z.boolean(), projectRevision: z.string().nullable(),
}).passthrough()
const backdropResponseSchema = z.object({ backdrop: backdropSchema.nullable() }).passthrough()
type Backdrop = z.infer<typeof backdropSchema>
export type BackdropItem = z.infer<typeof backdropItemSchema>
export interface BackdropOperation {
  op: 'reroll' | 'swap' | 'remove'
  itemId?: string
  channelId?: string
  videoId?: string
  startSec?: number
  endSec?: number
}
export interface BackdropActions {
  backdrop: Backdrop | null
  error: string
  pending: boolean
  mutateBackdrop: (operation: BackdropOperation) => Promise<void>
}
const pendingBackdropBuilds = new Map<string, { projectId: string; revision: string }>()
const activeBackdropMutations = new Set<string>()

/** The manifest owns random/owner slots; never infer fixed clips from a filename. */
export function useBackdropActions({ jobId, beforeMutation, onProjectChanged }: BackendMutationProps): BackdropActions {
  const projectId = useProjectStore(state => state.currentProject?.id)
  const [backdrop, setBackdrop] = useState<Backdrop | null>(null)
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const [refresh, setRefresh] = useState(0)
  const acting = useRef(false)
  const contextKey = `${projectId ?? ''}:${jobId ?? ''}`
  const [loadedContext, setLoadedContext] = useState(contextKey)
  if (loadedContext !== contextKey) {
    setLoadedContext(contextKey)
    setBackdrop(null)
    setError('')
    setPending(false)
  }
  useEffect(() => {
    if (!jobId) return
    const controller = new AbortController()
    let loading = false
    let queued = false
    const load = async () => {
      if (loading) { queued = true; return }
      loading = true
      try {
        do {
          queued = false
          const response = backdropResponseSchema.parse(await request<unknown>(`/api/clone/${encodeURIComponent(jobId)}/backdrop`, { signal: controller.signal }))
          if (controller.signal.aborted) return
          setBackdrop(response.backdrop)
          const waiting = pendingBackdropBuilds.get(jobId)
          if (!waiting && !activeBackdropMutations.has(jobId)) setPending(false)
          if (waiting && response.backdrop && !response.backdrop.busy && !response.backdrop.rebuilding && response.backdrop.projectRevision && response.backdrop.items.every(item => item.status === 'ready')) {
            const stored = await getEditorStorage().loadProject(waiting.projectId)
            if (controller.signal.aborted) return
            if (stored.revision === response.backdrop.projectRevision && stored.revision !== waiting.revision && pendingBackdropBuilds.get(jobId) === waiting) {
              pendingBackdropBuilds.delete(jobId)
              setPending(false)
              if (useProjectStore.getState().currentProject?.id !== waiting.projectId) return
              if (useTimelineSettingsStore.getState().isDirty) setError('Nền đã dựng lại, nhưng video có thay đổi chưa lưu. Hãy xử lý bản sửa mới trước khi tải lại.')
              else await onProjectChanged?.()
            }
          }
          if (waiting && response.backdrop?.items.some(item => item.status === 'failed')) {
            pendingBackdropBuilds.delete(jobId)
            setPending(false)
            setError('Không dựng lại được clip nền. Hãy chọn đoạn khác hoặc thử lại.')
          }
        } while (queued && !controller.signal.aborted)
      } catch (error) { if (!controller.signal.aborted) setError(panelError(error)) }
      finally { loading = false }
    }
    void load()
    const unsubscribe = subscribeEvents(['clone.job.updated', 'clone.step.progress', 'reset'], data => {
      if (matchesCloneEvent(data, jobId)) void load()
    })
    return () => { controller.abort(); unsubscribe() }
  }, [jobId, projectId, refresh, onProjectChanged])

  const mutateBackdrop = async (operation: BackdropOperation) => {
    if (acting.current || pending || jobId && (activeBackdropMutations.has(jobId) || pendingBackdropBuilds.has(jobId))) return
    acting.current = true
    if (jobId) activeBackdropMutations.add(jobId)
    setPending(true); setError('')
    try {
      if (!jobId || !projectId || !backdrop) throw new Error('Chưa có nguồn nền cho video này.')
      if (!onProjectChanged) throw new Error('Không tải lại được bản dựng sau khi đổi nền.')
      if (beforeMutation && !await beforeMutation()) throw new Error('Chưa lưu được video. Hãy lưu trước khi đổi nền.')
      const revision = getProjectRevision(projectId)
      if (!revision) throw new Error('Chưa có phiên bản video. Hãy mở lại video trước khi đổi nền.')
      const accepted = z.object({ accepted: z.literal(true), revision: z.string() }).passthrough().parse(
        await request<unknown>(`/api/clone/${encodeURIComponent(jobId)}/backdrop`, { body: { ...operation, revision: backdrop.revision } }),
      )
      pendingBackdropBuilds.set(jobId, { projectId, revision })
      setBackdrop(current => current ? { ...current, revision: accepted.revision } : null)
      setRefresh(value => value + 1)
    } catch (error) { setPending(false); setError(panelError(error)) }
    finally { acting.current = false; if (jobId) activeBackdropMutations.delete(jobId) }
  }
  return { backdrop, error, pending: pending || Boolean(backdrop?.busy) || Boolean(backdrop?.rebuilding) || Boolean(jobId && pendingBackdropBuilds.has(jobId)), mutateBackdrop }
}

export function backdropItemFor(item: TimelineItem, backdrop: { items: BackdropItem[] } | null): BackdropItem | undefined {
  const backlink = (item as TimelineItem & { cloneBackdropId?: string }).cloneBackdropId
  const exact = backdrop?.items.find(entry => entry.id === (backlink ?? item.id))
  if (exact) return exact
  const matches = backdrop?.items.filter(entry => entry.editorMediaId && entry.editorMediaId === item.mediaId)
  return matches?.length === 1 ? matches[0] : undefined
}

export function MediaAsset({ mediaId, src, label = '' }: { mediaId?: string; src?: string; label?: string }) {
  const media = useMediaLibraryStore(state => mediaId ? state.mediaItems.find(entry => entry.id === mediaId) : undefined)
  const [resolved, setResolved] = useState<{ key: string; url: string; error: boolean } | null>(null)
  const key = `${mediaId ?? ''}:${media?.contentHash ?? ''}:${src ?? ''}`
  const image = media?.mimeType.startsWith('image/')
  useEffect(() => {
    let active = true
    const load = async () => {
      try {
        const thumbnail = media && !image ? await mediaLibraryService.getThumbnailBlobUrl(media.id, media.thumbnailId) : null
        const url = thumbnail || src || (media?.contentHash && !media.mimeType.startsWith('audio/') ? getEditorStorage().mediaUrl(media.id, media.contentHash) : '')
        if (active) setResolved({ key, url, error: false })
      } catch {
        if (active) setResolved({ key, url: '', error: true })
      }
    }
    void load()
    return () => { active = false }
  }, [key, media?.id, media?.thumbnailId, image, src, media?.contentHash])
  const url = resolved?.key === key ? resolved.url : image && media?.contentHash ? getEditorStorage().mediaUrl(media.id, media.contentHash) : src
  if (resolved?.key === key && resolved.error) return <span {...stylex.props(sx.meta)}>Không tải được ảnh</span>
  if (!url || media?.mimeType.startsWith('audio/')) return <Icon name={media?.mimeType.startsWith('audio/') ? 'volume' : 'gallery'} size={20} />
  if (media?.mimeType.startsWith('video/') && media.contentHash && url === getEditorStorage().mediaUrl(media.id, media.contentHash)) return <video {...stylex.props(sx.image)} src={url} muted preload="metadata" aria-label={label} onError={() => setResolved({ key, url: '', error: true })} />
  return <img {...stylex.props(sx.image)} src={url} alt={label} loading="lazy" onError={() => setResolved({ key, url: '', error: true })} />
}

/** Replaces one clip, never the source shared by the other clips using that media. */
export async function replaceItemMedia(itemId: string, media: MediaMetadata): Promise<void> {
  const state = useItemsStore.getState()
  const item = state.itemById[itemId]
  const projectId = useProjectStore.getState().currentProject?.id
  if (!item || !projectId) throw new Error('Mục đang chọn không còn trong video.')
  const expected = item.type === 'image' ? 'image/' : item.type === 'video' ? 'video/' : null
  if (!expected || !media.mimeType.startsWith(expected) || !media.contentHash) throw new Error('Chọn media đúng loại cho mục này.')
  const fps = useTimelineSettingsStore.getState().fps
  const sourceFps = media.fps || fps
  const sourceDuration = Math.round(media.duration * sourceFps)
  const requiredFrames = Math.ceil(item.durationInFrames / fps * sourceFps * (item.speed ?? 1))
  if (item.type === 'video' && sourceDuration < requiredFrames) throw new Error('Clip mới ngắn hơn đoạn đang thay. Hãy chọn clip dài hơn.')
  await getEditorStorage().linkMedia(projectId, media.id)
  if (useProjectStore.getState().currentProject?.id !== projectId || useItemsStore.getState().itemById[itemId] !== item) throw new Error('Video đã thay đổi. Hãy chọn lại mục cần thay.')
  editorCommand([{ type: 'item.update', params: { id: itemId, updates: {
    mediaId: media.id, src: getEditorStorage().mediaUrl(media.id, media.contentHash),
    sourceWidth: media.width, sourceHeight: media.height,
    ...(item.type === 'video' ? { sourceFps, sourceDuration, sourceStart: 0, sourceEnd: requiredFrames, embeddedAudioMuted: true, loop: false, isReversed: false } : {}),
  }, clearFields: ['thumbnailUrl', 'trimStart', 'trimEnd', 'reverseConformSrc', 'reverseConformPath', 'reverseConformKey', 'reverseConformPreviewSrc', 'reverseConformPreviewPath', 'reverseConformPreviewKey', 'reverseConformStatus', 'reverseConformLocalStart'] } }])
}

export function MediaPicker({ item, onClose }: { item: TimelineItem; onClose: () => void }) {
  const media = useMediaLibraryStore(state => state.mediaItems)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const choices = media.filter(entry => entry.mimeType.startsWith(item.type === 'image' ? 'image/' : 'video/'))
  const choose = async (entry: MediaMetadata) => {
    if (busy) return
    setBusy(true); setError('')
    try { await replaceItemMedia(item.id, entry); onClose() }
    catch (error) { setError(panelError(error)) }
    finally { setBusy(false) }
  }
  return <Dialog title={item.type === 'image' ? 'Thay ảnh' : 'Chọn đoạn khác'} onClose={busy ? undefined : onClose} width={560} footer={<Button onClick={onClose} disabled={busy}>Đóng</Button>}>
    {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
    <div {...stylex.props(sx.grid)}>{choices.map(entry => <button key={entry.id} type="button" disabled={busy} {...stylex.props(sx.mediaCard)} title={entry.fileName} onClick={() => { void choose(entry) }}><span {...stylex.props(sx.thumbnail, sx.mediaThumbnail)}><MediaAsset mediaId={entry.id} /></span><span {...stylex.props(sx.title)}>{entry.fileName}</span></button>)}</div>
    {!choices.length && <p {...stylex.props(sx.empty)}>Chưa có media phù hợp. Nhập media ở bảng Media rồi chọn lại.</p>}
  </Dialog>
}

function coveragePercent(items: TimelineItem[], duration: number): number {
  if (duration <= 0) return 0
  let end = 0; let covered = 0
  for (const item of items.toSorted((a, b) => a.from - b.from)) {
    const start = Math.max(end, item.from)
    const next = Math.min(duration, item.from + item.durationInFrames)
    if (next > start) covered += next - start
    end = Math.max(end, next)
  }
  return Math.min(100, covered / duration * 100)
}

function BackgroundList({ items, tracks, selectedId, select, actions }: {
  items: TimelineItem[]
  tracks: TimelineTrack[]
  selectedId?: string
  select: (item: TimelineItem) => void
  actions: BackdropActions
}) {
  const fps = useTimelineSettingsStore(state => state.fps)
  const allItems = useItemsStore(state => state.items)
  const duration = allItems.reduce((end, item) => Math.max(end, item.from + item.durationInFrames), 0)
  const coverage = coveragePercent(items, duration)
  const random = actions.backdrop?.items.filter(item => item.source === 'random').length
  const fixed = actions.backdrop?.items.filter(item => item.source === 'owner').length
  return <div {...stylex.props(sx.padded)}>
    <div {...stylex.props(sx.column)}>
      <div {...stylex.props(sx.heading)}>
        <b {...stylex.props(coverage >= 100 ? sx.ok : sx.warn)}>{coverage >= 100 ? 'Phủ kín video' : 'Nền chưa phủ kín video'}</b>
        <span {...stylex.props(sx.meta)}>{items.length} clip</span>
      </div>
      <div {...stylex.props(sx.coverage)}><div {...stylex.props(sx.coverageFill(coverage))} /></div>
      {actions.backdrop && <span {...stylex.props(sx.meta)}>{random} clip từ kênh · {fixed} clip cố định</span>}
    </div>
    <div {...stylex.props(sx.heading)}>
      <h3 {...stylex.props(sx.groupTitle)}>Clip của video này</h3>
      <Button size="sm" variant="quiet" title="Clip bạn tự chọn được giữ nguyên" disabled={!actions.backdrop?.randomPicks} pending={actions.pending} onClick={() => { void actions.mutateBackdrop({ op: 'reroll' }) }} icon={<Icon name="refresh" size={13} />}>Đổi ngẫu nhiên tất cả</Button>
    </div>
    {actions.error && <p role="alert" {...stylex.props(sx.error)}>{actions.error}</p>}
    <div role="list" {...stylex.props(sx.backgroundList)}>
      {items.map(item => {
        const source = backdropItemFor(item, actions.backdrop)
        return <div key={item.id} role="listitem" {...stylex.props(sx.backgroundRow, item.id === selectedId && sx.selected)}>
          <button type="button" {...stylex.props(sx.backgroundPick)} onClick={() => select(item)} aria-current={item.id === selectedId ? 'true' : undefined}>
            <span {...stylex.props(sx.thumbnail, sx.smallThumbnail)}><MediaAsset mediaId={item.mediaId} src={item.type === 'video' ? item.thumbnailUrl : undefined} /></span>
            <span {...stylex.props(sx.column)}>
              <span {...stylex.props(sx.title)} title={item.label}>{item.label}</span>
              <span {...stylex.props(sx.meta)}>{source ? source.source === 'owner' ? 'Cố định' : 'Từ kênh' : ''} · {panelTime(item.from / fps)} · {panelTime(item.durationInFrames / fps)}</span>
            </span>
          </button>
          <button type="button" {...stylex.props(sx.smallIcon)} title="Đổi ngẫu nhiên" aria-label={`Đổi ngẫu nhiên ${item.label}`} disabled={!source || !actions.backdrop?.randomPicks || actions.pending} onClick={() => { if (source) void actions.mutateBackdrop({ op: 'reroll', itemId: source.id }) }}><Icon name="refresh" size={14} /></button>
        </div>
      })}
    </div>
    {!items.length && <p {...stylex.props(sx.help)}>Chưa có clip nền trong video này.</p>}
    {tracks.some(track => track.templateSlot === 'background-video') && <p {...stylex.props(sx.help)}><Icon name="info-circle" size={13} />Đổi ở đây chỉ áp cho video này. Đổi cho mọi video: Template › Nền &amp; hiệu ứng.</p>}
  </div>
}

function MediaPanel({ onSelectItem }: Pick<ContentPanelsProps, 'onSelectItem'>) {
  const media = useMediaLibraryStore(state => state.mediaItems)
  const items = useItemsStore(state => state.items)
  const [kind, setKind] = useState<'image' | 'video' | 'overlay' | 'audio'>('image')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const picker = useRef<HTMLInputElement>(null)
  const [actionMount, setActionMount] = useState<HTMLElement | null>(null)
  useLayoutEffect(() => { setActionMount(document.getElementById('editor-panel-actions')) }, [])
  const matches = (entry: MediaMetadata, filter = kind) => filter === 'overlay' ? entry.overlay === true : entry.mimeType.startsWith(`${filter}/`) && !entry.overlay
  const importFiles = async (files: File[]) => {
    if (busy || !files.length) return
    const projectId = useProjectStore.getState().currentProject?.id
    if (!projectId) { setError('Chưa mở video để nhập media.'); return }
    setBusy(true); setError('')
    try {
      for (const file of files) {
        if (!/^(image|video|audio)\//u.test(file.type)) throw new Error('Chỉ nhập ảnh, video hoặc âm thanh.')
        const tooLarge = await uploadTooLarge(uploadKindForMime(file.type), file.size)
        if (tooLarge) { setError(tooLarge); return }
        const record = await getEditorStorage().importMedia(file, { fileName: file.name, mime: file.type, projectId })
        if (useProjectStore.getState().currentProject?.id !== projectId) throw new Error('Video đã thay đổi. Media đã nhập được giữ trong thư viện.')
        useMediaLibraryStore.getState().prependMediaItem(serverMediaToMetadata(record))
      }
    } catch (error) { setError(panelError(error)) }
    finally { setBusy(false) }
  }
  const add = async (entry: MediaMetadata) => {
    if (busy) return
    setBusy(true); setError('')
    try {
      if (entry.overlay) await addOverlay(entry)
      else { const id = await insertMedia(entry); onSelectItem?.(id) }
    } catch (error) { setError(panelError(error)) }
    finally { setBusy(false) }
  }
  const importButton = <Button size="sm" pending={busy} onClick={() => picker.current?.click()} icon={<Icon name="add" size={14} />}>Nhập media</Button>
  return <>
    <div {...stylex.props(sx.tabs)}>
      <div role="tablist" aria-label="Loại media" {...stylex.props(sx.tabBar)}>
        {([['image', 'Ảnh'], ['video', 'Nền'], ['overlay', 'Phủ'], ['audio', 'Âm']] as const).map(([value, label]) => <button key={value} type="button" role="tab" aria-selected={kind === value} {...stylex.props(sx.tab, kind === value && sx.activeTab)} onClick={() => setKind(value)}>{label} {media.filter(entry => matches(entry, value)).length}</button>)}
      </div>
    </div>
    {actionMount ? createPortal(importButton, actionMount) : <div {...stylex.props(sx.padded)}>{importButton}</div>}
    <input hidden ref={picker} type="file" accept="image/*,video/*,audio/*" multiple aria-label="Nhập media" onChange={event => {
      const files = Array.from(event.currentTarget.files ?? [])
      event.currentTarget.value = ''
      void importFiles(files)
    }} />
    {error && <div {...stylex.props(sx.padded)}><p role="alert" {...stylex.props(sx.error)}>{error}</p></div>}
    <div {...stylex.props(sx.grid)}>
      {media.filter(entry => matches(entry)).map(entry => <button key={entry.id} type="button" disabled={busy}
        {...stylex.props(sx.mediaCard)} title={entry.fileName} aria-label={`Thêm ${entry.fileName} vào video tại vị trí đang xem`}
        onClick={() => { void add(entry) }}>
        <span {...stylex.props(sx.thumbnail, sx.mediaThumbnail)}>
          <MediaAsset mediaId={entry.id} />
          {items.some(item => item.mediaId === entry.id) && <span {...stylex.props(sx.used)}><Icon name="tick-circle" size={13} /></span>}
        </span>
      </button>)}
    </div>
    {!media.some(entry => matches(entry)) && <p {...stylex.props(sx.empty)}>Chưa có media loại này.</p>}
  </>
}

function AudioPanel({ measurement }: { measurement?: AudioMeasurement }) {
  const tracks = useItemsStore(state => state.tracks)
  const items = useItemsStore(state => state.items)
  const [error, setError] = useState('')
  const target = tracks.find(track => track.templateSlot === 'source-audio')?.templateLoudnessLufs
  const measuredAt = measurement?.measuredAt ? new Date(measurement.measuredAt) : null
  const hasTime = measuredAt && Number.isFinite(measuredAt.getTime())
  return <div {...stylex.props(sx.padded)}>
    <div {...stylex.props(sx.audioCard)}>
      <span {...stylex.props(measurement?.stale ? sx.warn : sx.ok)}><Icon name="volume" size={16} /></span>
      <span {...stylex.props(sx.column)}>
        <b>{measurement?.evidence ? 'Âm lượng đã đo' : 'Chưa đo'}</b>
        {measurement?.evidence && <span>{measurement.evidence}</span>}
        <span {...stylex.props(sx.meta)}>
          {target !== undefined && `Mục tiêu ${target.toLocaleString('vi')} LUFS`}
          {hasTime && measuredAt && `${target !== undefined ? ' · ' : ''}đo lúc ${measuredAt.toLocaleTimeString('vi', { hour: '2-digit', minute: '2-digit' })}`}
          {measurement?.stale && ' · Cần đo lại'}
        </span>
      </span>
    </div>
    {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
    {tracks.filter(track => track.kind === 'audio' || items.some(item => item.trackId === track.id && (item.type === 'audio' || item.type === 'video'))).map(track => <label key={track.id} {...stylex.props(sx.volume)}>
      <span {...stylex.props(sx.heading)}><span>{track.name}</span><span {...stylex.props(sx.meta)}>{track.volume ?? 0} dB</span></span>
      <input {...stylex.props(sx.slider)} type="range" min={-60} max={12} step={0.5} value={track.volume ?? 0} aria-label={`Âm lượng ${track.name}`} onChange={event => {
        try {
          editorCommand([{ type: 'audio.setTrackVolume', params: { trackId: track.id, volume: Number(event.target.value) } }])
          setError('')
        } catch (error) { setError(panelError(error)) }
      }} />
      <span {...stylex.props(sx.help)}>{track.templateSlot === 'source-audio' ? 'Giọng kể của video.' : track.templateSlot === 'background-video' ? 'Tiếng của clip nền.' : 'Áp dụng cho các clip trên lớp âm thanh này.'}</span>
    </label>)}
  </div>
}

export function ContentPanels({ panel, onSelectItem, jobId, beforeMutation, onProjectChanged, measurement }: ContentPanelsProps) {
  const backdropActions = useBackdropActions({ jobId, beforeMutation, onProjectChanged })
  const items = useItemsStore(state => state.items)
  const tracks = useItemsStore(state => state.tracks)
  const selectedId = useSelectionStore(state => state.selectedItemIds[0])
  const fps = useTimelineSettingsStore(state => state.fps)
  const activeCueId = usePlaybackStore(state => {
    if (panel !== 'subs' || !selectedId) return undefined
    const item = items.find(entry => entry.id === selectedId)
    if (item?.type !== 'subtitle') return selectedId
    return item.cues.find(cue => state.currentFrame >= item.from + cue.startSeconds * fps && state.currentFrame < item.from + cue.endSeconds * fps)?.id
  })
  const [query, setQuery] = useState('')
  const select = (item: TimelineItem) => { selectEditorItem(item); onSelectItem?.(item.id) }
  const backgrounds = useMemo(() => items.filter(item => item.type === 'video' && tracks.some(track => track.id === item.trackId && track.templateSlot === 'background-video')).toSorted((a, b) => a.from - b.from), [items, tracks])
  const scenes = useMemo(() => items.filter((item): item is Extract<TimelineItem, { type: 'image' }> => item.type === 'image' && (!tracks.some(track => track.templateSlot === 'scenes') || tracks.some(track => track.id === item.trackId && track.templateSlot === 'scenes'))).toSorted((a, b) => a.from - b.from), [items, tracks])
  const captions = useMemo(() => items.filter(isCaption).flatMap<{ item: TimelineItem; id: string; cueId?: string; text: string; from: number }>(item => item.type === 'subtitle'
    ? item.cues.map(cue => ({ item, id: `${item.id}:${cue.id}`, cueId: cue.id, text: cue.text, from: item.from + Math.round(cue.startSeconds * fps) }))
    : [{ item, id: item.id, text: item.text, from: item.from }]).toSorted((a, b) => a.from - b.from), [items, fps])
  return <div {...stylex.props(sx.panel)} data-shortcut-scope="library">
    {panel === 'scenes' && <>
      <div role="list" {...stylex.props(sx.list)}>
        {scenes.map((item, index) => <button key={item.id} type="button" role="listitem"
          aria-current={item.id === selectedId ? 'true' : undefined}
          {...stylex.props(sx.row, item.id === selectedId && sx.selected)} onClick={() => select(item)}>
          <span {...stylex.props(sx.thumbnail)}>
            <MediaAsset mediaId={item.mediaId} src={item.src} />
            <span {...stylex.props(sx.badge)}>{index + 1}</span>
          </span>
          <span {...stylex.props(sx.column)}>
            <span {...stylex.props(sx.title)} title={item.label}>{item.label}</span>
            <span {...stylex.props(sx.meta)}>{panelTime(item.from / fps)}–{panelTime((item.from + item.durationInFrames) / fps)} · {panelTime(item.durationInFrames / fps)}</span>
          </span>
        </button>)}
      </div>
      {!scenes.length && <p {...stylex.props(sx.empty)}>Chưa có cảnh trong video này.</p>}
      <p {...stylex.props(sx.help, sx.sceneHelp)}><Icon name="info-circle" size={13} />Vẽ lại nhiều cảnh cùng lúc ở Duyệt ảnh. Ở đây sửa từng cảnh của video này.</p>
    </>}
    {panel === 'subs' && <>
      <div {...stylex.props(sx.padded, sx.subtitleSearch)}>
        <div {...stylex.props(sx.searchWrap)}>
          <span {...stylex.props(sx.searchIcon)}><Icon name="search" size={14} /></span>
          <input {...stylex.props(sx.input, sx.search)} aria-label="Tìm trong phụ đề" placeholder="Tìm trong phụ đề" value={query} onChange={event => setQuery(event.target.value)} />
        </div>
        <span {...stylex.props(sx.meta)}>{captions.length} đoạn phụ đề</span>
      </div>
      <div role="list" {...stylex.props(sx.list, sx.subtitleList)}>
        {captions.filter(caption => caption.text.toLocaleLowerCase('vi').includes(query.toLocaleLowerCase('vi'))).map(caption => {
          const current = caption.item.id === selectedId && (caption.cueId ? caption.cueId === activeCueId : true)
          return <button key={caption.id} type="button" role="listitem" aria-current={current ? 'true' : undefined}
            {...stylex.props(sx.row, sx.subRow, current && sx.selected)}
            onClick={() => { select(caption.item); usePlaybackStore.getState().setCurrentFrame(caption.from) }}>
            <span {...stylex.props(sx.time)}>{panelTime(caption.from / fps)}</span>
            <span {...stylex.props(sx.subText)}>{caption.text}</span>
          </button>
        })}
      </div>
      {!captions.length && <p {...stylex.props(sx.empty)}>Chưa có phụ đề trong video này.</p>}
    </>}
    {panel === 'bg' && <BackgroundList items={backgrounds} tracks={tracks} selectedId={selectedId} select={select} actions={backdropActions} />}
    {panel === 'audio' && <AudioPanel measurement={measurement} />}
    {panel === 'media' && <MediaPanel onSelectItem={onSelectItem} />}
  </div>
}
