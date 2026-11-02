import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from '@noverse/ui'
import { cache, getData, invalidate, setData } from '@noverse/ui/data'
import { ApiError, request } from '../../api/request'
import { readScoreSummary, type ScoreSummary } from '../../api/autonomy'
import type { Status } from '../../api/status'
import { events, subscribeEvents } from '../../data/events'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'

import type { Suggestion } from '../../api/suggestions'
export interface ReviewJob { id: string; channelId: string; stageGroup: string; subline: string; createdAt: string; source: { title: string; thumbnailUrl: string | null } }
export interface ReviewVersion { version: number; cutout: unknown; originalUrl?: string; cutoutUrl?: string; source?: string; width?: number; height?: number; createdAt?: string; prompt?: string; promptVersion?: number; score?: unknown }
export interface ReviewImage { id: string; kind: string; status: string; skipReason?: string; prompt: string; promptVersion?: number; queuePosition?: number; currentVersion: number | null; reviewVersion: number | null; versions: ReviewVersion[]; sceneId?: string; startSec?: number; endSec?: number; width?: number; height?: number; narration?: string; direction?: string; reference?: string; referenceUrl?: string; score?: unknown }
export interface ReviewImages { manifest: { revision: number } | null; items: ReviewImage[]; approved: boolean; approvedToday?: number | null; learn?: Suggestion | null; summary?: ReviewSummary | null }
/** Tóm tắt điểm của video + câu server viết (artboard MReview › băng điểm). */
export type ReviewSummary = NonNullable<ScoreSummary> & { text: string }
const readReviewSummary = (value: unknown): ReviewSummary | null => {
  const summary = readScoreSummary(value)
  if (!summary) return null
  const text = (value as { summary?: unknown } | null)?.summary
  return { ...summary, text: typeof text === 'string' ? text : '' }
}
/** Artboard MReview: trạng thái ảnh đang được Hoạ sĩ vẽ (chưa có bản mới). */
export const isDrawing = (item: ReviewImage) => item.status === 'running' || item.status === 'pending' || item.status === 'queued'
/** Ảnh bị bộ lọc chặn hoặc Hoạ sĩ từ chối vẽ — màn hình dùng ảnh cảnh bên cạnh. */
export const isBlocked = (item: ReviewImage) => item.status === 'failed' || item.status === 'blocked' || item.status === 'skipped' && item.skipReason === 'refused'
/** Nhãn ảnh dùng chung cho danh sách, ảnh lớn, phiên bản và sheet prompt. */
export const reviewLabel = (item: ReviewImage, index: number) => item.kind === 'thumbnail' ? 'Ảnh bìa' : `Cảnh ${item.sceneId?.match(/\d+/)?.[0] || index + 1}`
/** Bản mới nhất của ảnh — bản đang xem khi chưa chọn bản khác. */
export const latestVersion = (item: ReviewImage) => item.versions.reduce((max, version) => Math.max(max, version.version), 0)
/** Bản đang được video dùng: bản đã chốt, không thì bản mới nhất. */
export const usedVersion = (item: ReviewImage) => item.currentVersion ?? latestVersion(item)
const base = (id: string) => `/api/clone/${encodeURIComponent(id)}/images`
export const imageURL = (job: string, image: ReviewImage, variant: 'full' | 'cutout' = 'full') => {
  const version = image.versions.find(v => v.version === (image.reviewVersion ?? image.currentVersion))
  return variant === 'cutout' ? version?.cutoutUrl || `${base(job)}/${encodeURIComponent(image.id)}?variant=cutout` : version?.originalUrl || ''
}
/** URL của một phiên bản cụ thể (artboard MReview › dải Phiên bản). */
export const versionURL = (job: string, image: ReviewImage, version: number, variant: 'full' | 'cutout' = 'full') => {
  const found = image.versions.find(v => v.version === version)
  return variant === 'cutout'
    ? found?.cutoutUrl || `${base(job)}/${encodeURIComponent(image.id)}?variant=cutout&version=${version}`
    : found?.originalUrl || ''
}

/**
 * Dữ liệu màn Duyệt đọc qua cache dùng chung: quay lại tab trong hạn này
 * không gọi lại API. Sự kiện `clone.job.*` (bảng `data/eventKeys.ts`) và lần
 * nối lại luồng là đường làm mới thật; màn không còn vòng poll nền nào.
 */
export const REVIEW_STALE_MS = 30_000

type ReviewJobs = { jobs: ReviewJob[] }
/** Tiến độ vẽ SSE `image.progress` — đủ để biết ảnh nào vừa xong/lỗi. */
type ImageProgress = { jobId: string; imageId: string; phase: string }
const readImageProgress = (value: unknown): ImageProgress | null => {
  if (!value || typeof value !== 'object') return null
  const { jobId, imageId, phase } = value as Record<string, unknown>
  return typeof jobId === 'string' && typeof imageId === 'string' && typeof phase === 'string'
    ? { jobId, imageId, phase }
    : null
}

/** Thao tác duyệt gửi lên `POST /api/clone/:id/images`. */
export type ReviewOp = 'regenerate' | 'editPrompt' | 'skip' | 'approve' | 'approveAll' | 'regenerateAll'
/** Hợp đồng `useReview` cho hai bản màn hình (máy tính + MReview). */
export interface ReviewState {
  jobs: ReviewJob[] | undefined
  images: Record<string, ReviewImages>
  error: boolean
  imageErrors: Record<string, boolean>
  pending: Record<string, boolean>
  /** Thử lại danh sách video chờ duyệt (nút "Thử lại" khi lỗi). */
  loadJobs: () => Promise<unknown>
  /** Đọc lại ảnh một video — lần đầu mở hoặc sau khi vẽ/duyệt. */
  loadImages: (id: string, force?: boolean) => Promise<void>
  mutate: (job: string, item: ReviewImage | null, op: ReviewOp, feedback?: string, version?: number) => Promise<boolean>
}

export function useReview(): ReviewState {
  const jobsResource = useResource<ReviewJobs>(
    keys.reviewJobs(),
    ({ signal }) => request<ReviewJobs>('/api/clone?channel=all', { signal }),
    { staleTime: REVIEW_STALE_MS },
  )
  const [images, setImages] = useState<Record<string, ReviewImages>>({})
  const [imageErrors, setImageErrors] = useState<Record<string, boolean>>({})
  const [pending, setPending] = useState<Record<string, boolean>>({})
  /** Video vừa `approveAll` lạc quan — ẩn tới khi server rời nhóm duyệt. */
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set())
  const locks = useRef(new Set<string>())
  const redrawing = useRef(new Map<string, number>())
  const requested = useRef(new Set<string>())
  const alive = useRef(true)
  const serverJobs = jobsResource.data?.jobs
  const previousJobs = useRef<ReviewJob[] | undefined>(undefined)
  const jobs = useMemo(() => {
    if (!serverJobs) return undefined
    const next = serverJobs.filter(job => job.stageGroup === 'review' && !hidden.has(job.id))
    // Server có thể đưa video sang nhóm khác trong lúc vẽ lại: giữ nó tới khi
    // `loadImages` thấy bản mới rồi xoá `redrawing`.
    for (const job of previousJobs.current ?? []) {
      if (redrawing.current.has(job.id) && !next.some(item => item.id === job.id)) next.push(job)
    }
    return next.sort((a, b) => a.createdAt.localeCompare(b.createdAt))
  }, [serverJobs, hidden])
  useEffect(() => { previousJobs.current = jobs }, [jobs])
  // Server đã đưa video ra khỏi nhóm duyệt → thôi ẩn lạc quan (duyệt xong).
  useEffect(() => {
    if (!serverJobs) return
    const still = new Set(serverJobs.filter(job => job.stageGroup === 'review').map(job => job.id))
    setHidden(old => {
      if (![...old].some(id => !still.has(id))) return old
      const next = new Set(old)
      for (const id of next) if (!still.has(id)) next.delete(id)
      return next
    })
  }, [serverJobs])

  const refreshJobs = jobsResource.refresh
  const loadJobs = useCallback(() => refreshJobs(), [refreshJobs])
  const loadImages = useCallback(async (id: string, force = false) => {
    requested.current.add(id)
    const key = keys.reviewImages(id)
    try {
      await cache.fetch(key, async ({ signal }) => {
        const result = await request<ReviewImages & { autonomy?: unknown }>(base(id), { signal })
        return { ...result, summary: readReviewSummary(result.autonomy) }
      }, { staleTime: force ? 0 : 120_000 })
      const result = cache.get<ReviewImages>(key)
      if (result.error) throw result.error
      if (!result.data || !alive.current) return
      const view = { ...result.data, summary: result.data.summary ?? readReviewSummary((result.data as ReviewImages & { autonomy?: unknown }).autonomy) }
      const since = redrawing.current.get(id)
      if (since !== undefined && (view.manifest?.revision ?? 0) > since && !view.items.some(isDrawing)) redrawing.current.delete(id)
      setImages(old => old[id] === view ? old : { ...old, [id]: view })
      setImageErrors(old => old[id] === false ? old : { ...old, [id]: false })
    }
    catch { if (alive.current) setImageErrors(old => ({ ...old, [id]: true })) }
  }, [])
  // Tab ẩn giữ kết quả tải trước trong cache; khi hiện lại, khôi phục ảnh mà
  // không GET lần nữa nếu còn mới, trừ khi danh sách/sự kiện đã làm dữ liệu cũ.
  // Tiến độ vẽ ghép ngay vào ảnh đang hiện và đọc lại khi xong/lỗi; job đổi (duyệt
  // ở thiết bị khác) đọc lại ảnh của video đang mở; nối lại luồng đọc lại mọi
  // ảnh đã mở. Danh sách video do cache + bảng sự kiện lo, kể cả lúc tab ẩn.
  useEffect(() => {
    alive.current = true
    const jobs = cache.get(keys.reviewJobs())
    const catchingUp = jobs.refreshing || jobs.status === 'loading' || jobs.updatedAt === null || Date.now() - jobs.updatedAt >= REVIEW_STALE_MS
    for (const id of requested.current) void loadImages(id, catchingUp)
    const stopProgress = subscribeEvents('image.progress', data => {
      const progress = readImageProgress(data)
      if (!progress || !requested.current.has(progress.jobId)) return
      if (progress.phase === 'done' || progress.phase === 'failed') { void loadImages(progress.jobId, true); return }
      setImages(old => {
        const view = old[progress.jobId]
        if (!view || !view.items.some(item => item.id === progress.imageId && (item.status === 'review' || item.status === 'pending'))) return old
        return { ...old, [progress.jobId]: { ...view, items: view.items.map(item => item.id === progress.imageId && (item.status === 'review' || item.status === 'pending') ? { ...item, status: 'running' } : item) } }
      })
    })
    const stopJobs = subscribeEvents('clone.job.updated', data => {
      const id = (data as { jobId?: unknown } | null)?.jobId
      if (typeof id === 'string' && requested.current.has(id)) void loadImages(id, true)
    })
    const stopReconnect = events.onReconnect(() => { for (const id of requested.current) void loadImages(id, true) })
    return () => { stopProgress(); stopJobs(); stopReconnect(); alive.current = false }
  }, [loadImages])
  const refreshImage = async (job: string, item: string) => {
    const data = await request<{ item: ReviewImage; revision: number; approved: boolean }>(`${base(job)}/${encodeURIComponent(item)}`)
    const key = keys.reviewImages(job)
    const previous = getData<ReviewImages>(key)
    if (!previous) return
    const view = { ...previous, summary: previous.summary ?? readReviewSummary((previous as ReviewImages & { autonomy?: unknown }).autonomy), manifest: previous.manifest && { ...previous.manifest, revision: data.revision }, approved: data.approved, items: previous.items.map(i => i.id === item ? data.item : i) }
    setData(key, view)
    setImages(old => ({ ...old, [job]: view }))
  }
  /**
   * Một cửa cho mọi thao tác duyệt: vẽ lại theo góp ý (`regenerate`), sửa prompt
   * (`editPrompt`), bỏ ảnh (`skip`), chọn bản đã vẽ (`approve` + `version`),
   * duyệt cả video (`approveAll`), vẽ lại cả bộ (`regenerateAll`).
   * `approve` không đổi trạng thái lạc quan: server chốt bản rồi tải lại ảnh.
   */
  const mutate = async (job: string, item: ReviewImage | null, op: ReviewOp, feedback = '', version?: number) => {
    const view = images[job]; const revision = view?.manifest?.revision
    if (!view || !revision || locks.current.has(job)) return false
    locks.current.add(job); setPending(old => ({ ...old, [job]: true }))
    const oldView = view
    if (op === 'approveAll') {
      setHidden(old => new Set(old).add(job))
      const status = getData<Status>(keys.status())
      if (status) setData<Status>(keys.status(), { ...status, reviewCount: Math.max(0, status.reviewCount - 1) })
    } else if (op !== 'approve') {
      if (op !== 'skip') redrawing.current.set(job, revision)
      setImages(old => ({ ...old, [job]: { ...view, items: view.items.map(i => (item ? i.id === item.id : true) ? { ...i, status: op === 'skip' ? 'skipped' : 'running' } : i) } }))
    }
    try {
      const result = await request<{ manifest?: { revision: number }; revision?: number; learn?: Suggestion | null; generation?: { learn?: Suggestion | null } | null }>(item ? `${base(job)}/${encodeURIComponent(item.id)}` : base(job), { body: { op, expectedRevision: revision, ...(item && feedback ? { prompt: feedback } : {}), ...(version !== undefined ? { version } : {}), ...(op === 'regenerateAll' ? { confirmCount: view.items.length, ...(feedback ? { feedback } : {}) } : {}) } })
      const nextRevision = result.manifest?.revision ?? result.revision
      const learn = result.generation?.learn ?? result.learn
      if (nextRevision || learn) setImages(old => { const v = old[job]; return v ? { ...old, [job]: { ...v, ...(nextRevision ? { manifest: { revision: nextRevision } } : {}), ...(learn ? { learn } : {}) } } : old })
      if (op === 'approveAll') { void invalidate(keys.status()); void invalidate(['review', 'counts']); window.dispatchEvent(new Event('activity:refresh')) }
      else void loadImages(job, true)
      return true
    } catch (err) {
      if (op === 'approveAll') { setHidden(old => { if (!old.has(job)) return old; const next = new Set(old); next.delete(job); return next }); const status = getData<Status>(keys.status()); if (status) setData<Status>(keys.status(), { ...status, reviewCount: status.reviewCount + 1 }); void invalidate(keys.status()) } else { redrawing.current.delete(job); if (op !== 'approve') setImages(old => ({ ...old, [job]: oldView })) }
      // Duyệt bị từ chối (409 revision cũ / ảnh đang vẽ / cảnh đổi): đọc lại ảnh
      // để `manifest.revision` và trạng thái khớp server, nếu không lần bấm sau
      // lặp lại đúng lỗi cũ. Ảnh đơn vẫn chỉ đọc lại đúng ảnh đó (409).
      if (op === 'approveAll') { if (alive.current) await loadImages(job, true) }
      else if (item && err instanceof ApiError && err.status === 409) { try { await refreshImage(job, item.id) } catch { /* retain last known image */ } }
      toast.err(err instanceof Error ? err.message : 'Chưa thực hiện được. Vui lòng thử lại.')
      return false
    } finally { locks.current.delete(job); setPending(old => ({ ...old, [job]: false })) }
  }
  return { jobs, images, error: !!jobsResource.error, imageErrors, pending, loadJobs, loadImages, mutate }
}
