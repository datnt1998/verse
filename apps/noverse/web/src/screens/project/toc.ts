// Gói AG — mục lục cột trái của màn Dự án (Outline.dc.html › nav search, len 8/40/120).
// Tách phần thuần để test: bỏ dấu + khớp số, trạng thái tập (live/planned/unplanned/locked),
// một tập mở một lúc, nhớ tập đang mở theo truyện (localStorage), tải chương theo tập khi truyện >200 chương.
import type { Outline, OutlineChapter, StorySummary } from '../../api/types'
import { cache, keys, useResource } from '../../data'
import { getOutline } from '../../api/client'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { volumeLabel } from './format'

/** Truyện dài hơn ngưỡng này: sidebar không tải cả mục lục — kéo chương của một tập khi mở tập đó (?volume=). */
export const TOC_LAZY_CHAPTERS = 200

/** Truyện này có lazy mục lục không (>200 chương — §AG). */
export const tocLazy = (totalChapters: number) => totalChapters > TOC_LAZY_CHAPTERS

export type TocChapter = Pick<OutlineChapter, 'chapter' | 'title' | 'completed' | 'status'>

export type TocVolume = {
  index: number
  label: string
  /** dòng meta nhỏ bên phải tiêu đề tập */
  meta: string
  /** khoảng chương [from..to] của tập (0 khi chưa biết) */
  from: number
  to: number
  live: boolean
  locked: boolean
  /** hàng “Tập N+1” giả sau tập cuối: viết tiếp khi tập trước xong, còn không thì khoá */
  next: boolean
  /** số chương đã biết (arcs hoặc chapters) — hiển thị khi tập chưa tải */
  chapterCount: number
  chapters: TocChapter[]
  /** đang tải chương (chế độ lazy) */
  pending?: boolean
}

const read = <T>(key: string, fallback: T): T => {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch { return fallback }
}
const write = (key: string, value: unknown) => { try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* đầy */ } }

const tocKey = (project: string, story: string) => `ainovel.toc.${project}.${story}`
/** Tập đang mở của truyện — 0 = chưa mở tập nào (đã “Thu gọn mọi tập”). */
export const getTocOpen = (project: string, story: string) => read<number | null>(tocKey(project, story), null)
export const setTocOpen = (project: string, story: string, volume: number | null) => write(tocKey(project, story), volume ?? 0)

/** Bỏ dấu tiếng Việt để tìm không cần gõ dấu (NFD + đ→d). */
export const stripAccents = (text: string) =>
  text.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[đĐ]/g, (c) => (c === 'đ' ? 'd' : 'D'))

/** Khoảng chương của một tập từ `arcs[].chapters` (có cả khi chương chưa tải — chế độ lazy). */
export function volumeRange(volume: Outline['volumes'][number]): { from: number; to: number } | null {
  let from = 0, to = 0
  for (const arc of volume.arcs) for (const n of arc.chapters) { if (!from || n < from) from = n; if (n > to) to = n }
  return from > 0 ? { from, to } : null
}

/** Số chương đã biết của tập: theo arcs (khi chapters rỗng ở chế độ lazy) hoặc theo chapters đã tải. */
export function volumeChapterCount(volume: Outline['volumes'][number]): number {
  const arcs = volume.arcs.reduce((sum, arc) => sum + arc.chapters.length, 0)
  return Math.max(arcs, volume.chapters.length)
}

/** Tập chứa chương `n` (theo khoảng đã biết); 0 khi không thuộc tập nào. */
export function tocVolumeOf(volumes: readonly (Pick<TocVolume, 'index' | 'from' | 'to'> | Outline['volumes'][number])[], n: number): number {
  for (const volume of volumes) {
    const range = 'from' in volume ? (volume.from ? { from: volume.from, to: volume.to } : null) : volumeRange(volume)
    if (range && n >= range.from && n <= range.to) return volume.index
  }
  return 0
}

const LIVE_STATUS: Record<string, true> = { writing: true, rewriting: true, rewrite: true }

/**
 * Dựng dòng tập cho mục lục. `currentChapter` = chương đang viết (snapshot.inProgressChapter).
 * Một tập “khoá” = chưa có chương nào (arcs rỗng) — mờ, không bấm được (artboard `disabled`).
 */
export function buildTocVolumes(
  volumes: Outline['volumes'], bookTitle: string | null,
  currentChapter: number, nextVolumeTitle: string,
): TocVolume[] {
  const rows = volumes.map((volume): TocVolume => {
    const count = volumeChapterCount(volume)
    const chapters: TocChapter[] = volume.chapters.map((c) => ({ chapter: c.chapter, title: c.title, completed: c.completed, status: c.status }))
    const locked = count === 0
    const range = volumeRange(volume)
    const live = !locked && (chapters.some((c) => LIVE_STATUS[c.status] === true) || (!!range && currentChapter >= range.from && currentChapter <= range.to))
    const done = chapters.filter((c) => c.completed).length
    const untitled = count > 0 && chapters.length === count && chapters.every((c) => !c.title.trim())
    const meta = locked
      ? `mở khi Tập ${volume.index - 1} xong`
      : live
        ? `${done}/${count} xong`
        : untitled
          ? `${count} ch. · chưa dàn ý`
          : `${count} ch. · có dàn ý`
    return { index: volume.index, label: `${volumeLabel(volume, bookTitle)}${live ? ' · đang viết' : ''}`, meta, live, locked, next: false, chapterCount: count, from: range?.from ?? 0, to: range?.to ?? 0, chapters }
  })
  const last = volumes[volumes.length - 1]
  if (last) {
    const lastCount = volumeChapterCount(last)
    if (lastCount > 0) {
      // Tập cuối đã bung: thêm hàng “Tập N+1” — mở được (viết tiếp) khi Tập N đã xong hết,
      // khoá “mở khi Tập N xong” khi chưa (artboard len 8/40: Tập 2 · mở khi Tập 1 xong).
      const lastDone = last.chapters.length > 0 && last.chapters.every((c) => c.completed)
      rows.push({
        index: last.index + 1,
        label: `Tập ${last.index + 1}`,
        meta: lastDone ? 'viết tiếp' : `mở khi Tập ${last.index} xong`,
        live: false, locked: !lastDone, next: true, chapterCount: 0, from: 0, to: 0, chapters: [],
      })
    }
  }
  return rows
}

export type TocHit = { n: number; title: string; volume: number; volumeLabel: string }


/**
 * Tìm chương trong các tập đã tải: khớp số chương (gõ đúng số, kể cả “chương 41”)
 * hoặc tên không dấu chứa chuỗi không dấu. Lazy: số rơi vào khoảng của tập chưa tải
 * vẫn trả hàng gợi ý — useToc tự kéo chương tập đó (?volume=N) để điền tựa.
 */
export function tocHits(volumes: readonly TocVolume[], query: string): TocHit[] {
  const q = stripAccents(query.trim().toLowerCase())
  if (!q) return []
  const exact = q.match(/^(?:chuong\s*)?(\d+)$/)
  const hits: TocHit[] = []
  for (const volume of volumes) {
    const label = volume.label.replace(/ · đang viết$/, '')
    for (const c of volume.chapters) {
      const title = c.title || `Chương ${c.chapter}`
      if (exact ? c.chapter === Number(exact[1]) : stripAccents(title.toLowerCase()).includes(q)) {
        hits.push({ n: c.chapter, title, volume: volume.index, volumeLabel: label })
      }
    }
    if (exact && volume.chapters.length === 0 && volume.from && Number(exact[1]) >= volume.from && Number(exact[1]) <= volume.to) {
      hits.push({ n: Number(exact[1]), title: `Chương ${exact[1]}`, volume: volume.index, volumeLabel: label })
    }
  }
  hits.sort((a, b) => a.n - b.n)
  return hits.slice(0, 50)
}

/** Mặc định mở tập đang viết (live); không có thì tập đầu tiên mở được. */
export function defaultTocOpen(volumes: readonly Pick<TocVolume, 'index' | 'live' | 'locked'>[]): number | null {
  return volumes.find((v) => v.live && !v.locked)?.index ?? volumes.find((v) => !v.locked)?.index ?? null
}

/**
 * Tập mở đã lưu còn hợp lệ không; `0` = người dùng đã “Thu gọn mọi tập” (null),
 * `null` = chưa lưu gì → mở tập đang viết, còn không thì tập đầu mở được.
 */
export function savedTocOpen(volumes: readonly Pick<TocVolume, 'index' | 'live' | 'locked'>[], saved: number | null): number | null {
  if (saved === null) return defaultTocOpen(volumes)
  if (saved === 0) return null
  return volumes.some((v) => v.index === saved && !v.locked) ? saved : defaultTocOpen(volumes)
}

/** Gộp chương của các tập đã tải lazy vào danh sách tập header (tập có `chapters` thì thay, còn giữ nguyên). */
export function mergeVolumeChapters(base: Outline['volumes'], patch: Outline | null): Outline['volumes'] {
  if (!patch?.volumes.length) return base
  const loaded = new Map(patch.volumes.map((v) => [v.index, v]))
  return base.map((v) => {
    const next = loaded.get(v.index)
    return next && next.chapters.length > 0 ? { ...v, chapters: next.chapters } : v
  })
}

/** Trạng thái mục lục của truyện đang mở — dùng bởi cột trái. */
export function useToc(project: string, summary: StorySummary, bookTitle: string | null, outline: Outline | null, currentChapter: number, query: string) {
  const story = summary.story
  const lazy = tocLazy(summary.totalChapters)
  // Truyện >200 chương: chỉ lấy header các tập trước (?volume=0 = không kèm chương);
  // nếu màn giữa đã tải đủ outline thì dùng luôn, không gọi thêm.
  const headers = useResource<Outline>(
    keys.story(project, story, 'outline', 'vol', 0),
    ({ signal }) => getOutline(project, story, { signal, volume: 0 }),
    { enabled: lazy },
  )
  // Lazy: luôn đi qua header ?volume=0 — không dùng outline đầy đủ của màn giữa, để mục lục
  // thật sự kéo chương theo tập (?volume=N) thay vì nuốt luôn 240+ chương.
  const full = !lazy ? outline : (headers.data ?? null)
  const baseVolumes = full?.volumes ?? []

  // Chọn tập đang mở: từ localStorage (mặc định: tập đang viết). Một tập mở một lúc.
  const pick = useMemo(
    () => buildTocVolumes(baseVolumes, bookTitle, currentChapter, full?.nextVolumeTitle ?? ''),
    [baseVolumes, bookTitle, currentChapter, full],
  )
  const [openState, setOpenState] = useState<{ story: string; volume: number | null }>({ story: '', volume: null })
  const openVolume = openState.story === story ? openState.volume : savedTocOpen(pick, getTocOpen(project, story))
  const setOpenVolume = useCallback((volume: number | null) => {
    setOpenState({ story, volume })
    setTocOpen(project, story, volume)
  }, [project, story])

  // Chương của các tập đã từng mở — tích luỹ theo truyện, không chỉ tập vừa tải.
  const loadedRef = useRef<{ story: string; map: Map<number, OutlineChapter[]> }>({ story, map: new Map() })
  if (loadedRef.current.story !== story) loadedRef.current = { story, map: new Map() }

  // Lazy: tải chương của tập đang mở (chỉ khi tập đó chưa có chương nào).
  const needVolume = useMemo(() => {
    if (!lazy || openVolume === null) return 0
    const v = baseVolumes.find((item) => item.index === openVolume)
    // Đã từng tải (loadedRef) thì không kéo lại — header vẫn trả chapters rỗng.
    return v && volumeChapterCount(v) > 0 && v.chapters.length === 0 && !loadedRef.current.map.has(openVolume) ? openVolume : 0
  }, [lazy, openVolume, baseVolumes])
  const resource = useResource<Outline>(
    keys.story(project, story, 'outline', 'vol', needVolume),
    ({ signal }) => getOutline(project, story, { signal, volume: needVolume }),
    { enabled: needVolume > 0 },
  )
  for (const v of resource.data?.volumes ?? []) {
    if (v.chapters.length > 0) loadedRef.current.map.set(v.index, v.chapters)
  }
  const mergeLoaded = (base: Outline['volumes']) =>
    base.map((v) => {
      const chapters = loadedRef.current.map.get(v.index)
      return chapters ? { ...v, chapters } : v
    })

  // Vòng 1: tập đang mở đã gộp vào → dựng mục lục + hits để biết tìm kiếm có cần kéo thêm tập nào.
  const volumes1 = useMemo(
    () => buildTocVolumes(lazy ? mergeLoaded(baseVolumes) : baseVolumes, bookTitle, currentChapter, full?.nextVolumeTitle ?? ''),
    [lazy, baseVolumes, bookTitle, currentChapter, full, resource.data],
  )
  const hits1 = useMemo(() => tocHits(volumes1, query), [volumes1, query])
  // Lazy + tìm số chương: hàng gợi ý của tập chưa tải → kéo chương tập đó về để điền tựa.
  const searchNeed = useMemo(() => {
    if (!lazy) return 0
    for (const hit of hits1) {
      const v = volumes1.find((item) => item.index === hit.volume)
      if (v && v.chapters.length === 0 && !loadedRef.current.map.has(v.index)) return v.index
    }
    return 0
  }, [lazy, hits1, volumes1])
  const searchRes = useResource<Outline>(
    keys.story(project, story, 'outline', 'vol', searchNeed),
    ({ signal }) => getOutline(project, story, { signal, volume: searchNeed }),
    { enabled: searchNeed > 0 },
  )
  for (const v of searchRes.data?.volumes ?? []) {
    if (v.chapters.length > 0) loadedRef.current.map.set(v.index, v.chapters)
  }

  const merged = useMemo(() => {
    if (!lazy) return baseVolumes
    return mergeLoaded(baseVolumes)
  }, [lazy, baseVolumes, resource.data, searchRes.data])

  const volumes = useMemo(
    () => buildTocVolumes(merged, bookTitle, currentChapter, full?.nextVolumeTitle ?? ''),
    [merged, bookTitle, currentChapter, full],
  )
  const hits = useMemo(() => tocHits(volumes, query), [volumes, query])
  useEffect(() => {
    // Tập đã lưu mà giờ bị khoá (dàn ý đổi) → rơi về tập đang viết.
    if (openVolume !== null && volumes.length > 0 && !volumes.some((v) => v.index === openVolume && !v.locked)) {
      setOpenVolume(defaultTocOpen(volumes))
    }
  }, [volumes, openVolume, setOpenVolume])

  return {
    volumes, hits, openVolume, setOpenVolume, lazy,
    loadingHeaders: lazy && !full && headers.initialLoading,
    loadingVolume: needVolume > 0 && resource.initialLoading,
  }
}
