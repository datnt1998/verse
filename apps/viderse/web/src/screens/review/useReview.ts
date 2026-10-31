import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from '@noverse/ui'
import { getData, invalidate, setData } from '@noverse/ui/data'
import { ApiError, request } from '../../api/request'
import type { Status } from '../../api/status'
import { keys } from '../../data/keys'

export interface ReviewJob { id: string; channelId: string; stageGroup: string; subline: string; createdAt: string; source: { title: string; thumbnailUrl: string | null } }
export interface ReviewImage { id: string; kind: string; status: string; skipReason?: string; prompt: string; currentVersion: number | null; reviewVersion: number | null; versions: { version: number; cutout: unknown; originalUrl?: string; cutoutUrl?: string }[]; sceneId?: string; startSec?: number; endSec?: number; width?: number; height?: number; narration?: string; direction?: string; reference?: string; referenceUrl?: string }
export interface ReviewImages { manifest: { revision: number } | null; items: ReviewImage[]; approved: boolean; approvedToday?: number | null }
const base = (id: string) => `/api/clone/${encodeURIComponent(id)}/images`
export const imageURL = (job: string, image: ReviewImage, variant: 'full' | 'cutout' = 'full') => {
  const version = image.versions.find(v => v.version === (image.reviewVersion ?? image.currentVersion))
  return variant === 'cutout' ? version?.cutoutUrl || `${base(job)}/${encodeURIComponent(image.id)}?variant=cutout` : version?.originalUrl || ''
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
    try { const result = await request<ReviewImages>(base(id)); if (alive.current) { const since = redrawing.current.get(id); if (since !== undefined && (result.manifest?.revision ?? 0) > since && !result.items.some(i => i.status === 'running' || i.status === 'pending' || i.status === 'queued')) redrawing.current.delete(id); setImages(old => ({ ...old, [id]: result })); setImageErrors(old => ({ ...old, [id]: false })) } }
    catch { if (alive.current) setImageErrors(old => ({ ...old, [id]: true })) }
  }, [])
  useEffect(() => { alive.current = true; void loadJobs(); const timer = window.setInterval(() => { void loadJobs() }, 2500); return () => { alive.current = false; clearInterval(timer) } }, [loadJobs])
  const refreshImage = async (job: string, item: string) => {
    const data = await request<{ item: ReviewImage; revision: number; approved: boolean }>(`${base(job)}/${encodeURIComponent(item)}`)
    setImages(old => { const previous = old[job]; return previous ? { ...old, [job]: { ...previous, manifest: previous.manifest && { ...previous.manifest, revision: data.revision }, approved: data.approved, items: previous.items.map(i => i.id === item ? data.item : i) } } : old })
  }
  const mutate = async (job: string, item: ReviewImage | null, op: 'regenerate' | 'skip' | 'approveAll' | 'regenerateAll', feedback = '') => {
    const view = images[job]; const revision = view?.manifest?.revision
    if (!view || !revision || locks.current.has(job)) return false
    locks.current.add(job); setPending(old => ({ ...old, [job]: true }))
    const oldJobs = jobs; const oldView = view
    if (op === 'approveAll') {
      approving.current.add(job)
      setJobs(old => old?.filter(j => j.id !== job))
      const status = getData<Status>(keys.status())
      if (status) setData<Status>(keys.status(), { ...status, reviewCount: Math.max(0, status.reviewCount - 1) })
    } else {
      if (op !== 'skip') redrawing.current.set(job, revision)
      setImages(old => ({ ...old, [job]: { ...view, items: view.items.map(i => (item ? i.id === item.id : true) ? { ...i, status: op === 'skip' ? 'skipped' : 'running' } : i) } }))
    }
    try {
      const result = await request<{ manifest?: { revision: number }; revision?: number }>(item ? `${base(job)}/${encodeURIComponent(item.id)}` : base(job), { body: { op, expectedRevision: revision, ...(item && feedback ? { prompt: feedback } : {}), ...(op === 'regenerateAll' ? { confirmCount: view.items.length, ...(feedback ? { feedback } : {}) } : {}) } })
      const nextRevision = result.manifest?.revision ?? result.revision
      if (nextRevision) setImages(old => { const v = old[job]; return v ? { ...old, [job]: { ...v, manifest: { revision: nextRevision } } } : old })
      if (op === 'approveAll') { void invalidate(keys.status()); void invalidate(['review', 'counts']); window.dispatchEvent(new Event('activity:refresh')) }
      else void loadImages(job)
      return true
    } catch (err) {
      if (op === 'approveAll') { approving.current.delete(job); setJobs(oldJobs); const status = getData<Status>(keys.status()); if (status) setData<Status>(keys.status(), { ...status, reviewCount: status.reviewCount + 1 }); void invalidate(keys.status()) } else { redrawing.current.delete(job); setImages(old => ({ ...old, [job]: oldView })) }
      if (err instanceof ApiError && err.status === 409 && item) { try { await refreshImage(job, item.id) } catch { /* retain last known image */ } }
      toast.err(err instanceof Error ? err.message : 'Chưa thực hiện được. Vui lòng thử lại.')
      return false
    } finally { locks.current.delete(job); setPending(old => ({ ...old, [job]: false })) }
  }
  return { jobs, images, error, imageErrors, pending, loadJobs, loadImages, mutate }
}
