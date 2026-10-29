import { describe, expect, it } from 'vitest'
import type { ChapterInfo, Snapshot } from '../../../api/types'
import { chapterReadable, chapterReadingState, nextChapterCaption, swipeChapter } from './navigation'

const chapters: ChapterInfo[] = [
  { chapter: 23, title: 'Dòng tên bị gạch', completed: true, words: 3102, hasDraft: false, volume: 1, arc: 1 },
  { chapter: 24, title: 'Tên tôi trên màn hình', completed: true, words: 3102, hasDraft: false, volume: 1, arc: 1 },
  { chapter: 25, title: 'Máy quay số ba', completed: false, words: 0, hasDraft: true, volume: 1, arc: 1 },
  { chapter: 26, title: 'Chưa viết', completed: false, words: 0, hasDraft: false, volume: 1, arc: 1 },
]

describe('M2 · changement de chapitre', () => {
  it('change dans le sens du geste seulement entre chapitres lisibles et conserve les vrais états', () => {
    expect(swipeChapter(chapters, 24, -90, 12)).toBe(25)
    expect(swipeChapter(chapters, 24, 90, 12)).toBe(23)
    expect(swipeChapter(chapters, 24, -50, 0)).toBeNull()
    expect(swipeChapter(chapters, 24, -90, 100)).toBeNull()
    expect(swipeChapter(chapters, 23, 90, 0)).toBeNull()
    expect(swipeChapter(chapters, 25, -90, 0)).toBeNull()
    expect(swipeChapter(chapters, 999, -90, 0)).toBeNull()
    expect(chapterReadable(chapters[2])).toBe(true)
    expect(chapterReadable(chapters[3])).toBe(false)
    const gate = { advanceMode: 'review', advancePermitChapter: 24, currentChapter: 25, completedCount: 24, isRunning: false, runtimeState: 'paused', phase: 'writing' } as Snapshot
    expect(chapterReadingState(chapters[1], gate)).toBe('waiting')
    expect(chapterReadingState(chapters[0], gate)).toBe('approved')
    expect(chapterReadingState(chapters[2], { ...gate, isRunning: true })).toBe('writing')
    expect(chapterReadingState(chapters[3], gate)).toBe('unwritten')
  })
})

describe('M2 · thẻ chương sau (MReader nextCh)', () => {
  it('chương đang viết hiện "đang viết N%" như artboard, không hiện số chữ', () => {
    expect(nextChapterCaption('writing', 1200, 48)).toBe('Chương sau · đang viết 48%')
  })
  it('các trạng thái khác giữ nhãn thật và số chữ nếu đã có', () => {
    expect(nextChapterCaption('waiting', 3102, 20)).toBe('Chương sau · Chờ duyệt · 3.102 chữ')
    expect(nextChapterCaption('approved', 0, 20)).toBe('Chương sau · Đã duyệt')
    expect(nextChapterCaption('unwritten', 0, 20)).toBe('Chương sau · Chưa viết')
  })
})
