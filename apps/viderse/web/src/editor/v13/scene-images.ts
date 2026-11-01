// Ảnh của một cảnh trong Editor — vẽ lại tại chỗ (Gói V29 · 07-data-contract §26).
//
// Máy chủ giữ mọi bản của một ảnh và đúng văn bản đã gửi. Editor chỉ cần ba việc:
// đọc bản đang dùng để hiện ảnh, nghe `image.progress` để biết bản nào đang vẽ, và
// khi vẽ xong thì đổi ĐÚNG media của clip đó bằng một lệnh hoàn tác được.
//
// Vì sao chỉ một lệnh `item.update` trên vài trường media: clip giữ nguyên vị trí,
// thời lượng, chuyển động và keyframe, và mọi thao tác đang dở của người dùng
// (kéo phụ đề, sửa chữ, chỉnh transform) không bị ghi đè. Editor không tải lại dự
// án ở đây — dự án trong bộ nhớ đang có bản sửa chưa lưu (CAS), tải lại là mất.
//
// `POST …/use { n, project: false }` chỉ đổi bản đang dùng ở máy chủ và trả về
// thay đổi cần áp cho clip; máy chủ không ghi project.json khi `project: false`.
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { create } from 'zustand'
import { getCommandExecutor, withCommandMergeKey } from '@editor/commands'
import type { HistoryEntry } from '@editor/commands/history'
import type { SerializedCommand } from '@editor/commands/types'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import type { ImageItem } from '@editor/types/timeline'
import {
  subscribeImageProgress,
  useImageVersion,
  useJobImages,
  usedVersionOf,
  withProgress,
  type ImageProgress,
  type ImageQuota,
  type ImageUseResult,
  type ImageVersion,
  type JobImage,
} from '../../api/images'
import { applyItemMedia, editorCommand, panelError } from './ContentPanels'

/** Mã clip của một ảnh cảnh trong project editor (`<projectId>-scene-<sceneId>`). */
export function clipIdFor(projectId: string, image: Pick<JobImage, 'id' | 'kind'>): string | null {
  return image.kind === 'scene' ? `${projectId}-scene-${image.id}` : null
}

/** Ảnh của clip đang chọn: mã clip là nguồn duy nhất, không đoán theo khung giờ. */
export function sceneImageFor(images: readonly JobImage[], item: ImageItem, projectId: string | undefined): JobImage | undefined {
  if (!projectId) return undefined
  return images.find(image => clipIdFor(projectId, image) === item.id)
}

export function versionOf(image: JobImage, n: number): ImageVersion | undefined {
  return image.versions.find(version => version.n === n)
}

/** Ảnh mới của một bản chưa được duyệt lại (`needsReview` là cờ của ảnh, không phải `status`). */
export function needsReviewOf(image: JobImage): boolean {
  return image.needsReview === true
}

/** Bản đang vẽ của ảnh, nếu có. */
export function drawingVersionOf(image: JobImage): ImageVersion | undefined {
  return image.versions.find(version => version.state === 'drawing')
}

/** “0:21” — đồng hồ của thẻ đang vẽ (artboard Editor › ipTimer). */
export function elapsedLabel(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`
}

/** “0:21 · thường khoảng 40 giây” khi bản trước đã đo được thời gian vẽ. */
export function drawingDetail(progress: ImageProgress | undefined, image: JobImage): string {
  const elapsed = elapsedLabel(progress?.elapsedSec ?? 0)
  const previous = [...image.versions].reverse().find(version => version.state === 'ready' && version.durationSec)
  return previous?.durationSec ? `${elapsed} · thường khoảng ${Math.round(previous.durationSec)} giây` : elapsed
}

/** Bước đang làm của DevelopEffect: 0 Gửi prompt · 1 Đang vẽ · 2 Tải về. */
export function drawingStep(progress: ImageProgress | undefined): number {
  if (progress?.phase === 'receiving') return 2
  if (progress?.phase === 'drawing') return 1
  return 0
}

// ── Thông báo “đã thay ảnh vào timeline” + Hoàn tác ──────────────────────────

export interface SceneSwapNotice {
  id: string
  jobId: string
  imageId: string
  clipId: string
  /** “Cảnh 5” */
  label: string
  n: number
  mediaId: string
  previousMediaId: string
  previousVersion: number
  /** Exact discrete swap, not whichever edit happens to be newest when clicked. */
  historyEntry: HistoryEntry
  /** Only fields changed by the media swap; never a whole-clip snapshot. */
  restoreMedia: SerializedCommand
}

interface SceneSwapState {
  notices: SceneSwapNotice[]
  error: string
  push: (notice: SceneSwapNotice) => void
  dismiss: (id: string) => void
  clear: () => void
  setError: (message: string) => void
}

export const useSceneSwapStore = create<SceneSwapState>()((set) => ({
  notices: [],
  error: '',
  push: (notice) => set(state => ({ notices: [...state.notices.filter(entry => entry.clipId !== notice.clipId), notice], error: '' })),
  dismiss: (id) => set(state => ({ notices: state.notices.filter(entry => entry.id !== id) })),
  clear: () => set({ notices: [], error: '' }),
  setError: (error) => set({ error }),
}))

/** Số cảnh của clip trên track cảnh (1-based), `null` khi clip không còn. */
export function sceneNumber(clipId: string): number | null {
  const state = useItemsStore.getState()
  const item = state.itemById[clipId]
  if (!item) return null
  const scenes = state.items.filter(entry => entry.trackId === item.trackId && entry.type === 'image').toSorted((a, b) => a.from - b.from)
  const index = scenes.findIndex(entry => entry.id === item.id)
  return index < 0 ? null : index + 1
}

/** Nhãn “Cảnh 5” của clip trên track cảnh. */
export function sceneLabel(clipId: string): string {
  const number = sceneNumber(clipId)
  return number ? `Cảnh ${number}` : ''
}

// ── Đổi bản đang dùng: áp cho clip + đồng bộ ngược khi ⌘Z ───────────────────

interface SceneVersionTrace {
  jobId: string
  imageId: string
  clipId: string
  /** mediaId → số bản, để biết ⌘Z/Làm lại đang trỏ về bản nào. */
  byMedia: Map<string, number>
  /** Bản máy chủ đang dùng theo lần đồng bộ gần nhất. */
  synced: number
}

const traces = new Map<string, SceneVersionTrace>()

export function resetSceneVersionTraces(): void {
  traces.clear()
}

/**
 * Ghi lại cặp media ↔ bản của một clip. Hoàn tác đổi `mediaId` của clip, nên so
 * media đang dùng với bảng này là biết phải báo máy chủ bản nào.
 */
export function traceSceneVersion(input: { jobId: string; imageId: string; clipId: string; entries: { mediaId: string | null | undefined; n: number | null | undefined }[]; synced: number }): void {
  const previous = traces.get(input.clipId)
  const trace: SceneVersionTrace = { jobId: input.jobId, imageId: input.imageId, clipId: input.clipId, byMedia: new Map(previous?.byMedia), synced: input.synced }
  for (const entry of input.entries) if (entry.mediaId && typeof entry.n === 'number') trace.byMedia.set(entry.mediaId, entry.n)
  traces.set(input.clipId, trace)
}

/**
 * Báo máy chủ bản đang dùng sau khi ⌘Z/Làm lại đổi media của clip. Chỉ gửi khi
 * bản đổi thật; đọc lại danh sách sau khi máy chủ nhận để nhãn đang dùng khớp ảnh.
 */
export async function syncSceneVersionUse(): Promise<void> {
  const updates: Promise<unknown>[] = []
  for (const trace of traces.values()) {
    const item = useItemsStore.getState().itemById[trace.clipId]
    const n = item?.mediaId ? trace.byMedia.get(item.mediaId) : undefined
    if (n === undefined || n === trace.synced) continue
    trace.synced = n
    updates.push(useImageVersion(trace.jobId, trace.imageId, n, { project: false }).catch(cause => {
      if (trace.synced === n) trace.synced = -1
      throw cause
    }))
  }
  await Promise.all(updates)
}

/** Bỏ thông báo của clip đã rời khỏi bản vừa thay (người dùng tự hoàn tác). */
export function dismissStaleNotices(): void {
  const { notices, dismiss } = useSceneSwapStore.getState()
  for (const notice of notices) {
    const item = useItemsStore.getState().itemById[notice.clipId]
    if (!item || item.mediaId !== notice.mediaId) dismiss(notice.id)
  }
}

/** Undo the swap itself without undoing edits made after its toast appeared. */
export function undoSceneSwap(notice: SceneSwapNotice): void {
  const store = useSceneSwapStore.getState()
  const item = useItemsStore.getState().itemById[notice.clipId]
  const executor = getCommandExecutor()
  const undo = executor.history.stacks(executor.contextKey).undo
  if (!item || item.mediaId !== notice.mediaId || !undo.includes(notice.historyEntry)) {
    store.dismiss(notice.id)
    return
  }
  try {
    if (undo.at(-1) === notice.historyEntry) {
      const result = executor.undo()
      if (!result.ok) throw new Error('Chưa hoàn tác được ảnh. Hãy thử lại.')
    } else {
      // A compensating media-only command preserves later edits and is itself undoable.
      withCommandMergeKey(null, () => editorCommand([notice.restoreMedia]))
    }
    store.dismiss(notice.id)
  } catch (cause) {
    store.setError(panelError(cause))
  }
}

/**
 * Vẽ xong một bản: máy chủ đã đổi `usedVersion`; Editor đổi media của clip bằng
 * đúng một lệnh hoàn tác được rồi trả về thông báo để hiện “Hoàn tác ⌘Z”.
 * Ảnh bìa không có clip nên chỉ đổi bản đang dùng.
 */
export async function bindFinishedVersion(jobId: string, imageId: string, n: number): Promise<SceneSwapNotice | null> {
  const projectId = useProjectStore.getState().currentProject?.id
  if (!projectId) return null
  return applyUseResult(jobId, imageId, await useImageVersion(jobId, imageId, n, { project: false }))
}

/** “Dùng bản N”: đổi bản đang dùng của video rồi áp đúng media đó cho clip. */
export async function useSceneVersion(jobId: string, imageId: string, n: number): Promise<SceneSwapNotice | null> {
  return applyUseResult(jobId, imageId, await useImageVersion(jobId, imageId, n, { project: false }))
}

async function applyUseResult(jobId: string, imageId: string, result: ImageUseResult): Promise<SceneSwapNotice | null> {
  if (!result.clipId || !result.mediaId) return null
  const item = useItemsStore.getState().itemById[result.clipId]
  if (!item || item.type !== 'image') return null
  const previousMediaId = item.mediaId ?? result.previousMediaId ?? ''
  const previousVersion = traces.get(result.clipId)?.byMedia.get(previousMediaId) ?? result.previousVersion ?? 0
  traceSceneVersion({ jobId, imageId, clipId: result.clipId, synced: result.usedVersion, entries: [{ mediaId: result.mediaId, n: result.usedVersion }, { mediaId: previousMediaId, n: previousVersion }] })
  if (item.mediaId === result.mediaId) return null
  await useMediaLibraryStore.getState().loadMediaItems()
  const media = useMediaLibraryStore.getState().mediaItems.find(entry => entry.id === result.mediaId)
  if (!media) throw new Error('Chưa tải được ảnh mới về máy. Hãy thử lại sau khi kết nối ổn định.')
  // Picking versions is discrete, not an Inspector drag: every swap has its own undo.
  const executor = getCommandExecutor()
  const undo = executor.history.stacks(executor.contextKey).undo
  const previousEntry = undo.at(-1)
  const before = useItemsStore.getState().itemById[item.id]
  withCommandMergeKey(null, () => applyItemMedia(item.id, media))
  const historyEntry = undo.at(-1)
  if (!historyEntry || historyEntry === previousEntry) return null
  const after = useItemsStore.getState().itemById[item.id]
  const updates: Record<string, unknown> = {}
  const clearFields: string[] = []
  // Derive the inverse from this synchronous media-only command, including fields
  // it cleared. Do not retain or replay the history's whole-item inverse patch.
  for (const key of new Set([...Object.keys(before), ...Object.keys(after)])) {
    const previous = Reflect.get(before, key)
    if (previous === Reflect.get(after, key)) continue
    if (previous === undefined) clearFields.push(key)
    else updates[key] = previous
  }
  const restoreMedia: SerializedCommand = { type: 'item.update', params: { id: item.id, updates, clearFields } }
  return { id: `${result.clipId}:${result.usedVersion}`, jobId, imageId, clipId: result.clipId, label: sceneLabel(result.clipId), n: result.usedVersion, mediaId: result.mediaId, previousMediaId, previousVersion, historyEntry, restoreMedia }
}

export interface SceneRedraws {
  images: JobImage[]
  progress: Record<string, ImageProgress>
  /** Hạn mức ảnh còn lại — nút vẽ lại luôn ghi giá. */
  quota?: ImageQuota
  error: boolean
  reload: () => Promise<void>
}

/**
 * Nghe tiến độ vẽ của cả video ở tầng Editor (một chỗ, không phụ thuộc cảnh đang
 * chọn): vẽ xong thì thay ảnh vào timeline và báo; ⌘Z/Làm lại thì đồng bộ bản
 * đang dùng về máy chủ.
 */
export function useSceneRedraws(jobId: string | undefined): SceneRedraws {
  const jobImages = useJobImages(jobId ?? null)
  const handled = useRef(new Set<string>())
  const reload = jobImages.reload
  const reloadRef = useRef(reload)
  reloadRef.current = reload
  useEffect(() => {
    handled.current = new Set()
    if (!jobId) return
    return subscribeImageProgress(event => {
      if (event.jobId !== jobId || event.phase !== 'done') return
      const key = `${event.imageId}:${event.n}`
      if (handled.current.has(key)) return
      handled.current.add(key)
      void bindFinishedVersion(jobId, event.imageId, event.n)
        .then(notice => { if (notice) useSceneSwapStore.getState().push(notice) })
        .catch((cause: unknown) => {
          handled.current.delete(key)
          useSceneSwapStore.getState().setError(cause instanceof Error && /[À-ỹ]/u.test(cause.message) ? cause.message : 'Đã vẽ xong nhưng chưa thay được ảnh vào timeline. Hãy thử lại.')
        })
      void reloadRef.current()
    })
  }, [jobId])
  useEffect(() => getCommandExecutor().on('commit', event => {
    // Targeted toast undo is an ordinary media-only execute. Observe its version
    // change too; unrelated execute commits need no server synchronization/reload.
    if (event.kind === 'execute' && !event.after.items.some(item =>
      traces.get(item.id)?.byMedia.has(item.mediaId ?? '')
      && event.before.items.find(previous => previous.id === item.id)?.mediaId !== item.mediaId,
    )) return
    void syncSceneVersionUse().then(() => reloadRef.current()).catch((cause: unknown) => {
      useSceneSwapStore.getState().setError(cause instanceof Error ? cause.message : 'Chưa đồng bộ được bản ảnh đã hoàn tác. Hãy thử lại.')
    })
    dismissStaleNotices()
  }), [])
  const reloadImages = useCallback(async () => { await reloadRef.current() }, [])
  // Tiến độ SSE ghép ngay vào danh sách: bản đang vẽ lộ ra trong lúc chờ lượt
  // đọc lại, nếu không bảng cảnh và khung xem không có trạng thái đang vẽ nào.
  const images = useMemo(
    () => (jobImages.data?.images ?? []).map(image => withProgress(image, jobImages.progress[image.id])),
    [jobImages.data, jobImages.progress],
  )
  useEffect(() => {
    if (!jobId) return
    const projectId = useProjectStore.getState().currentProject?.id
    if (!projectId) return
    for (const image of images) {
      const clipId = clipIdFor(projectId, image)
      if (!clipId) continue
      const item = useItemsStore.getState().itemById[clipId]
      if (!item || item.type !== 'image' || image.usedVersion == null || traces.has(clipId)) continue
      traceSceneVersion({ jobId, imageId: image.id, clipId, synced: image.usedVersion, entries: [{ mediaId: item.mediaId, n: image.usedVersion }] })
    }
  }, [jobId, images])
  return { images, progress: jobImages.progress, quota: jobImages.data?.quota, error: jobImages.error, reload: reloadImages }
}
