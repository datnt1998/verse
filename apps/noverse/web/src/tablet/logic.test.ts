import { expect, it } from 'vitest'
import type { ChapterInfo } from '../api/types'
import { storyMeta, tabletChapter } from './logic'

it('previews the last submitted chapter rather than the next live draft, respecting explicit picks and sparse unordered numbering', () => {
  const chapters: ChapterInfo[] = [
    { chapter: 25, title: 'Bản gốc trở về', completed: false, hasDraft: true, words: 3200, volume: 1, arc: 1 },
    { chapter: 8, title: 'Một chương cũ', completed: true, hasDraft: false, words: 2200, volume: 1, arc: 1 },
    { chapter: 24, title: 'Đèn tắt', completed: true, hasDraft: false, words: 3400, volume: 1, arc: 1 },
    { chapter: 30, title: 'Chưa viết', completed: false, hasDraft: false, words: 0, volume: 1, arc: 1 },
  ]
  expect(tabletChapter(chapters, null)).toBe(24)
  expect(tabletChapter(chapters, 8)).toBe(8)
  expect(tabletChapter(chapters, 30)).toBe(30)
  expect(tabletChapter(chapters.filter((chapter) => !chapter.completed), null)).toBe(25)
  expect(tabletChapter([], null)).toBeNull()
  expect(tabletChapter(null, null)).toBeNull()
})

it('formats the top-bar meta line without inventing a volume', () => {
  expect(storyMeta(1, 24, 30)).toBe('Tập 1 · 24/30 chương')
  expect(storyMeta(undefined, 3, 8)).toBe('3/8 chương')
})
