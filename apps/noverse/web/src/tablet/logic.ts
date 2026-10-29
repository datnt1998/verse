import type { ChapterInfo } from '../api/types'

/** Review the last submitted chapter, never the next live draft, unless explicitly picked. */
export function tabletChapter(chapters: readonly ChapterInfo[] | null, requested: number | null): number | null {
  if (requested !== null) return requested
  let completed: number | null = null
  let readable: number | null = null
  for (const chapter of chapters ?? []) {
    if (chapter.completed) completed = Math.max(completed ?? 0, chapter.chapter)
    if (chapter.completed || chapter.hasDraft || chapter.words > 0) readable = Math.max(readable ?? 0, chapter.chapter)
  }
  return completed ?? readable
}

/** Dòng phụ của thanh trên (“Tập 1 · 24/30 chương”): tập lấy từ dàn ý; chưa có dàn ý thì chỉ hiện tiến độ, không bịa tập. */
export function storyMeta(volume: number | undefined, done: number, total: number): string {
  return `${volume ? `Tập ${volume} · ` : ''}${done}/${total} chương`
}

