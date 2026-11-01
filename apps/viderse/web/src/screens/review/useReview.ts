import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from '@noverse/ui'
import { getData, invalidate, setData } from '@noverse/ui/data'
import { ApiError, request } from '../../api/request'
import { readScoreSummary, type ScoreSummary } from '../../api/autonomy'
import type { Status } from '../../api/status'
import { keys } from '../../data/keys'

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

export function useReview() {
  const [jobs, setJobs] = useState<ReviewJob[]>()
  const [images, setImages] = useState<Record<string, ReviewImages>>({})
  const [error, setError] = useState(false)
  const [imageErrors, setImageErrors] = useState<Record<string, boolean>>({})
  const [pending, setPending] = useState<Record<string, boolean>>({})
  const locks = useRef(new Set<string>())
  const redrawing = useRef(new Map<string, number>())
  const approving = useRef(new Set<string>())
  const alive = useRef(true)
  const loadJobs = useCallback(async () => {
    try {
      const result = await request<{ jobs: ReviewJob[] }>('/api/clone?channel=all')
      if (alive.current) {
        for (const id of approving.current) if (!result.jobs.some(j => j.id === id && j.stageGroup === 'review')) approving.current.delete(id)
        setJobs(old => {
          const next = result.jobs.filter(j => j.stageGroup === 'review' && !approving.current.has(j.id))
          for (const j of old ?? []) if (redrawing.current.has(j.id) && !next.some(n => n.id === j.id)) next.push(j)
          return next.sort((a, b) => a.createdAt.localeCompare(b.createdAt))
        })
        setError(false)
      }
    }
    catch { if (alive.current) setError(true) }
  }, [])
  const loadImages = useCallback(async (id: string) => {
    try {
      const result = await request<ReviewImages & { autonomy?: unknown }>(base(id))
      if (alive.current) {
        const since = redrawing.current.get(id)
        if (since !== undefined && (result.manifest?.revision ?? 0) > since && !result.items.some(isDrawing)) redrawing.current.delete(id)
        setImages(old => ({ ...old, [id]: { ...result, summary: readReviewSummary(result.autonomy) } }))
        setImageErrors(old => ({ ...old, [id]: false }))
      }
    }
    catch { if (alive.current) setImageErrors(old => ({ ...old, [id]: true })) }
  }, [])
  useEffect(() => { alive.current = true; void loadJobs(); const timer = window.setInterval(() => { void loadJobs() }, 2500); return () => { alive.current = false; clearInterval(timer) } }, [loadJobs])
  const refreshImage = async (job: string, item: string) => {
    const data = await request<{ item: ReviewImage; revision: number; approved: boolean }>(`${base(job)}/${encodeURIComponent(item)}`)
    setImages(old => { const previous = old[job]; return previous ? { ...old, [job]: { ...previous, manifest: previous.manifest && { ...previous.manifest, revision: data.revision }, approved: data.approved, items: previous.items.map(i => i.id === item ? data.item : i) } } : old })
  }
  /**
   * Một cửa cho mọi thao tác duyệt: vẽ lại theo góp ý (`regenerate`), sửa prompt
   * (`editPrompt`), bỏ ảnh (`skip`), chọn bản đã vẽ (`approve` + `version`),
   * duyệt cả video (`approveAll`), vẽ lại cả bộ (`regenerateAll`).
   * `approve` không đổi trạng thái lạc quan: server chốt bản rồi tải lại ảnh.
   */
  const mutate = async (job: string, item: ReviewImage | null, op: 'regenerate' | 'editPrompt' | 'skip' | 'approve' | 'approveAll' | 'regenerateAll', feedback = '', version?: number) => {
    const view = images[job]; const revision = view?.manifest?.revision
    if (!view || !revision || locks.current.has(job)) return false
    locks.current.add(job); setPending(old => ({ ...old, [job]: true }))
    const oldJobs = jobs; const oldView = view
    if (op === 'approveAll') {
      approving.current.add(job)
      setJobs(old => old?.filter(j => j.id !== job))
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
      else void loadImages(job)
      return true
    } catch (err) {
      if (op === 'approveAll') { approving.current.delete(job); setJobs(oldJobs); const status = getData<Status>(keys.status()); if (status) setData<Status>(keys.status(), { ...status, reviewCount: status.reviewCount + 1 }); void invalidate(keys.status()) } else { redrawing.current.delete(job); if (op !== 'approve') setImages(old => ({ ...old, [job]: oldView })) }
      if (err instanceof ApiError && err.status === 409 && item) { try { await refreshImage(job, item.id) } catch { /* retain last known image */ } }
      toast.err(err instanceof Error ? err.message : 'Chưa thực hiện được. Vui lòng thử lại.')
      return false
    } finally { locks.current.delete(job); setPending(old => ({ ...old, [job]: false })) }
  }
  return { jobs, images, error, imageErrors, pending, loadJobs, loadImages, mutate }
}
