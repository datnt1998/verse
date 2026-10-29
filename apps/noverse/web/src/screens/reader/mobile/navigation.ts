import type { ChapterInfo, Snapshot } from '../../../api/types'
import { isGate, type GateSnapshot } from '../../workspace/GateCard'

export type ReaderSnapshot = GateSnapshot & Pick<Snapshot, 'inProgressChapter' | 'flow' | 'bookTitle'>
export function readerSnapshot(snapshot: Snapshot | undefined): ReaderSnapshot | undefined {
  if (!snapshot) return undefined
  const { completedCount, currentChapter, runtimeState, phase, advanceMode, isRunning, advancePermitChapter, inProgressChapter, flow, bookTitle } = snapshot
  return { completedCount, currentChapter, runtimeState, phase, advanceMode, isRunning, advancePermitChapter, inProgressChapter, flow, bookTitle }
}
export function chapterReadable(chapter: ChapterInfo) {
  return chapter.completed || chapter.hasDraft || chapter.words > 0
}

export function swipeChapter(chapters: readonly ChapterInfo[], current: number, deltaX: number, deltaY: number) {
  if (Math.abs(deltaX) < 64 || Math.abs(deltaX) < Math.abs(deltaY) * 1.4) return null
  const index = chapters.findIndex((item) => item.chapter === current)
  if (index < 0) return null
  const next = chapters[index + (deltaX < 0 ? 1 : -1)]
  return next && chapterReadable(next) ? next.chapter : null
}

export function chapterReadingState(chapter: ChapterInfo, snapshot: ReaderSnapshot | undefined) {
  if (snapshot && isGate(snapshot) && chapter.chapter === snapshot.completedCount) return 'waiting' as const
  if (snapshot?.isRunning && (chapter.chapter === snapshot.inProgressChapter || chapter.chapter === snapshot.currentChapter)) return 'writing' as const
  return chapter.completed ? 'approved' as const : 'unwritten' as const
}

export type ChapterReadingState = ReturnType<typeof chapterReadingState>

const NEXT_CAPTION: Record<Exclude<ChapterReadingState, 'writing'>, string> = { waiting: 'Chờ duyệt', approved: 'Đã duyệt', unwritten: 'Chưa viết' }

/** Dòng nhỏ của thẻ "Chương sau" (MReader `nextCh`): đang viết thì kèm phần trăm, còn lại kèm số chữ đã có. */
export function nextChapterCaption(state: ChapterReadingState, words: number, percent: number): string {
  if (state === 'writing') return `Chương sau · đang viết ${percent}%`
  return `Chương sau · ${NEXT_CAPTION[state]}${words > 0 ? ` · ${words.toLocaleString('vi-VN')} chữ` : ''}`
}
