// Logic thuần của Trình đọc: nhóm chương theo tập/hồi, phục bút liên quan, xem trước
// khoảng chương xuất, chọn bản xuất gần nhất.
import type { ChapterFacts, ChapterInfo, ExportFile, ExportRow, ForeshadowEntry, Outline } from '../../api/types'

export type ChapterGroup = {
  volume: number
  title: string
  arcs: { arc: number; title: string; chapters: ChapterInfo[] }[]
}

/** Nhóm chương theo `volume` rồi `arc`; tiêu đề lấy từ đại cương khi có. */
export function groupChapters(chapters: ChapterInfo[], outline: Outline | null): ChapterGroup[] {
  const grouped = new Map<number, Map<number, ChapterInfo[]>>()
  for (const chapter of chapters) {
    const volume = chapter.volume || 1
    const arc = chapter.arc || 1
    let arcs = grouped.get(volume)
    if (!arcs) { arcs = new Map(); grouped.set(volume, arcs) }
    const list = arcs.get(arc)
    if (list) list.push(chapter)
    else arcs.set(arc, [chapter])
  }
  return [...grouped.entries()].map(([volume, arcs]) => {
    const volumeOutline = outline?.volumes.find((item) => item.index === volume)
    return {
      volume,
      title: volumeOutline?.title ?? `Tập ${volume}`,
      arcs: [...arcs.entries()].map(([arc, list]) => ({
        arc,
        title: volumeOutline?.arcs.find((item) => item.index === arc)?.title ?? `Hồi ${arc}`,
        chapters: list,
      })),
    }
  })
}

export function totalWords(chapters: ChapterInfo[]): number {
  return chapters.reduce((sum, chapter) => sum + chapter.words, 0)
}

/** Phục bút liên quan chương: được cập nhật trong facts, hoặc còn mở và đã gieo trước đó. */
export function relevantForeshadow(entries: ForeshadowEntry[], facts: ChapterFacts | null, chapter: number): ForeshadowEntry[] {
  const referenced = new Set((facts?.foreshadow_updates ?? []).map((update) => update.id))
  return entries.filter((entry) => {
    if (referenced.has(entry.id)) return true
    if (entry.status === 'resolved') return entry.resolved_at === chapter
    if (entry.status === 'abandoned') return entry.abandoned_at === chapter
    return entry.planted_at <= chapter
  })
}

/** Khoảng chương hợp lệ để xuất; null khi sai. */
export function normalizeRange(from: number, to: number, total: number): { from: number; to: number } | null {
  if (!Number.isInteger(from) || !Number.isInteger(to)) return null
  if (total < 1 || from < 1 || to < from || to > total) return null
  return { from, to }
}

export function latestExport(files: ExportFile[]): ExportFile | null {
  const book = files.filter((file) => file.format === 'txt' || file.format === 'epub')
  if (book.length === 0) return null
  return [...book].sort((a, b) => (b.updatedAt ?? '').localeCompare(a.updatedAt ?? ''))[0]
}

export function exportChapterCount(file: ExportFile, matrix: ExportRow[]): number | null {
  const row = matrix.find((item) => item.volume === file.volume)
  if (!row) return null
  return Math.max(0, row.to - row.from + 1)
}

/**
 * G10 — chỉ tải lại khi máy chủ trả `revision` khác lần trước.
 * Revision rỗng = máy chủ cũ chưa hỗ trợ G10 → không tải lại để tránh vòng hỏi 5 s.
 */
export function shouldReloadChapter(previous: string | null, next: string | null | undefined): boolean {
  if (!next) return false
  return previous !== next
}
