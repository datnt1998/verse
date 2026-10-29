// Données partagées du lecteur; le polling ne sert qu'en cas de panne SSE.
import { useEffect, useRef } from 'react'
import {
  ApiError, canWriteStory, getChapter, getChapterFacts, getChapterPlan, getChapterRevisions,
  getChapters, getExports, getForeshadow, getLatestReview, getOutline, getReviews, listStories,
} from '../../api/client'
import { useSession } from '../../api/session'
import type { ReviewEntry } from '../../api/types'
import { invalidate, keys, useConnection, useResource } from '../../data'
import { storyLabel } from '../../shell/labels'
import { shouldReloadChapter } from './logic'

export const REVISION_POLL_MS = 5000
const asApiError = (error: unknown) => error ? error instanceof ApiError ? error : new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.') : null
export type ReviewView = { entry: ReviewEntry; origin: 'chapter' | 'global' | 'latest' }
const missingAsNull = (error: unknown): null => { if (error instanceof ApiError && error.status === 404) return null; throw error }

export function useChapters(project: string, story: string) {
  const chapters = useResource(keys.story(project, story, 'chapters'), ({ signal }) => getChapters(project, story, signal))
  const outline = useResource(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal).catch(missingAsNull))
  const connection = useConnection()
  const previous = useRef<string | null>(null)
  useEffect(() => {
    const revision = chapters.data?.revision
    // Luồng gateway đã invalidated dữ liệu phụ; revision chỉ là lưới an toàn cho polling.
    if (connection.state !== 'open' && shouldReloadChapter(previous.current, revision)) {
      for (const part of ['chapter', 'plan', 'facts', 'revisions', 'reviews', 'foreshadow'] as const) void invalidate(keys.story(project, story, part))
    }
    previous.current = revision ?? null
  }, [project, story, chapters.data?.revision, connection.state])
  useEffect(() => {
    if (connection.state === 'open') return
    const poll = () => { if (document.visibilityState === 'visible' && navigator.onLine) void chapters.refresh() }
    const timer = setInterval(poll, REVISION_POLL_MS)
    return () => clearInterval(timer)
  }, [connection.state, chapters.refresh])
  return { chapters: chapters.data?.chapters ?? (chapters.status === 'error' ? [] : null), outline: outline.data ?? null, error: asApiError(chapters.error), reload: chapters.refresh, refreshing: chapters.refreshing || outline.refreshing, updatedAt: chapters.updatedAt }
}

export function useChapterContent(project: string, story: string, chapter: number | null, source: 'committed' | 'draft') {
  const resource = useResource(keys.story(project, story, 'chapter', chapter, source), ({ signal }) => getChapter(project, story, chapter!, source, signal), { enabled: chapter !== null, keepPrevious: true })
  // 404 xác nhận nguồn này chưa có chính văn; không dùng nội dung của chương/nguồn trước.
  const missing = resource.error instanceof ApiError && resource.error.status === 404
  return { content: chapter === null || missing ? null : resource.data ?? null, loading: chapter !== null && resource.initialLoading, error: asApiError(resource.error), reload: resource.refresh, refreshing: resource.refreshing, updatedAt: resource.updatedAt }
}

async function loadReview(project: string, story: string, chapter: number, signal: AbortSignal): Promise<ReviewView | null> {
  const result = await getReviews(project, story, chapter, signal).catch(missingAsNull)
  if (result?.chapter) return { entry: result.chapter, origin: 'chapter' }
  if (result?.global) return { entry: result.global, origin: 'global' }
  const latest = await getLatestReview(project, story, signal).catch(missingAsNull)
  return latest ? { entry: latest, origin: 'latest' } : null
}

export function useChapterExtras(project: string, story: string, chapter: number | null) {
  const enabled = chapter !== null
  const plan = useResource(keys.story(project, story, 'plan', chapter), ({ signal }) => getChapterPlan(project, story, chapter!, signal).catch(missingAsNull), { enabled, keepPrevious: true })
  const facts = useResource(keys.story(project, story, 'facts', chapter), ({ signal }) => getChapterFacts(project, story, chapter!, signal).catch(missingAsNull), { enabled, keepPrevious: true })
  const revisions = useResource(keys.story(project, story, 'revisions', chapter), ({ signal }) => getChapterRevisions(project, story, chapter!, signal).then((data) => data.revisions).catch((error: unknown) => { missingAsNull(error); return [] }), { enabled, keepPrevious: true })
  const review = useResource(keys.story(project, story, 'reviews', chapter), ({ signal }) => loadReview(project, story, chapter!, signal), { enabled, keepPrevious: true })
  const reload = () => Promise.all([plan.refresh(), facts.refresh(), revisions.refresh(), review.refresh()])
  return { plan: plan.data ?? null, facts: facts.data ?? null, revisions: revisions.data ?? (revisions.initialLoading ? null : []), review: review.data ?? (review.initialLoading ? undefined : null), error: asApiError(plan.error || facts.error || revisions.error || review.error), reload, refreshing: plan.refreshing || facts.refreshing || revisions.refreshing || review.refreshing, updatedAt: review.updatedAt }
}

export function useExports(project: string, story: string) {
  const resource = useResource(keys.story(project, story, 'exports'), ({ signal }) => getExports(project, story, signal))
  return { ...resource, data: resource.data ?? null, loading: resource.initialLoading, error: asApiError(resource.error), reload: resource.refresh }
}

export function useForeshadow(project: string, story: string) {
  const resource = useResource(keys.story(project, story, 'foreshadow'), ({ signal }) => getForeshadow(project, story, signal))
  return { entries: resource.data?.entries ?? null, error: asApiError(resource.error), reload: resource.refresh }
}

/** Tên truyện để hiện (story.json.title) — dùng dựng tên tệp xuất thân thiện. */
export function useStoryTitle(project: string, story: string): string {
  const resource = useResource(keys.project(project, 'stories'), () => listStories(project))
  const summary = resource.data?.stories.find((item) => item.story === story)
  return summary ? storyLabel(summary) : ''
}

export function useStoryAccess(project: string, story: string) {
  const { user } = useSession()
  const resource = useResource(keys.project(project, 'stories'), () => listStories(project))
  const summary = resource.data?.stories.find((item) => item.story === story)
  const locked = !!summary?.locked
  return { locked, writable: !!user && canWriteStory(user, `${project}/${story}`) && !locked, storyTitle: summary ? storyLabel(summary) : '' }
}
