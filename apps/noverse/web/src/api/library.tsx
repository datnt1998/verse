// Dữ liệu thư viện chia sẻ cho shell: /api/library + /api/projects/:p (cache theo p).
// Wave-2 screens dùng lại hook này thay vì gọi API trùng lặp.
import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { getLibrary, getProject } from './client'
import type { LibraryResponse, ProjectDetail, StorySummary } from './types'
import { useSession } from './session'
import { events, subscribeEvents } from '../data/events'

export type LibraryState = {
  library: LibraryResponse | null
  loading: boolean
  error: unknown
  reload: () => void
  /** chi tiết dự án (stories/ideas/assets) — tải lười theo p, cache */
  projectDetail: (p: string) => ProjectDetail | null
  /** truyện trong cache chi tiết */
  storyOf: (p: string, s: string) => StorySummary | null
  ensureProject: (p: string) => void
}

const LibraryContext = createContext<LibraryState>({
  library: null, loading: true, error: null,
  reload: () => {}, projectDetail: () => null, storyOf: () => null, ensureProject: () => {},
})

export function LibraryProvider({ children }: { children: ReactNode }) {
  const { user } = useSession()
  const [library, setLibrary] = useState<LibraryResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [tick, setTick] = useState(0)
  const details = useRef(new Map<string, ProjectDetail>())
  const pending = useRef(new Set<string>())
  const focusRefreshAt = useRef(0)
  const [detailsVersion, force] = useState(0)

  useEffect(() => {
    if (!user) { setLibrary(null); setLoading(false); details.current.clear(); return }
    let alive = true
    setLoading(true)
    getLibrary()
      .then((data) => { if (alive) { setLibrary(data); setError(null) } })
      .catch((e) => { if (alive) setError(e) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [user, tick])

  // This provider is the desktop project list, outside the shared resource
  // cache, so the event→key invalidation done by DataProvider never reaches
  // it. Without this a tab that stays open keeps the project set it fetched
  // at mount: a project created/deleted by this or another device — or a
  // missing event after reconnect — stays invisible until a full reload.
  useEffect(() => {
    if (!user) return
    focusRefreshAt.current = 0
    let timer: number | undefined
    // One mutation often publishes project.changed and library.changed as
    // separate frames; coalesce them into one list fetch (same 250 ms window
    // the shared cache uses for event invalidations).
    const schedule = () => {
      if (timer) return
      timer = window.setTimeout(() => { timer = undefined; setTick((v) => v + 1) }, 250)
    }
    const offLibrary = subscribeEvents(['library.changed', 'project.changed'], schedule)
    const offReconnect = events.onReconnect(schedule)
    // Same 15 s budget as the shared cache's focus revalidation.
    const onVisible = () => {
      if (document.visibilityState !== 'visible') return
      const now = Date.now()
      if (now - focusRefreshAt.current < 15000) return
      focusRefreshAt.current = now
      schedule()
    }
    window.addEventListener('focus', onVisible)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearTimeout(timer)
      offLibrary()
      offReconnect()
      window.removeEventListener('focus', onVisible)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [user])

  const ensureProject = (p: string) => {
    if (!p || details.current.has(p) || pending.current.has(p)) return
    pending.current.add(p)
    getProject(p)
      .then((detail) => { details.current.set(p, detail); force((v) => v + 1) })
      .catch(() => { /* lỗi để screen tự xử lý */ })
      .finally(() => pending.current.delete(p))
  }

  const value = useMemo<LibraryState>(() => ({
    library, loading, error,
    reload: () => { details.current.clear(); setTick((v) => v + 1) },
    projectDetail: (p) => details.current.get(p) ?? null,
    storyOf: (p, s) => details.current.get(p)?.stories.find((st) => st.story === s) ?? null,
    ensureProject,
  }), [library, loading, error, detailsVersion])
  return <LibraryContext.Provider value={value}>{children}</LibraryContext.Provider>
}

export const useLibrary = () => useContext(LibraryContext)
