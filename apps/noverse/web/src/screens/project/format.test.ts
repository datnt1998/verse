// Số thứ tự thư mục truyện (11-api-contract.md §3 › POST stories) và các helper thuần khác của màn Dự án.
import { describe, expect, it } from 'vitest'
import type { AutoExport, StorySummary } from '../../api/types'
import {
  autoExportSentence, chapterStatusLabel, folderNumber, formatBytes, isRewriteFlow, nextStoryNumber, volumeLabel,
} from './format'

const stories = (...ids: string[]) => ids.map((story) => ({ story }) as StorySummary)

describe('folderNumber', () => {
  it('reads the numeric prefix of a story folder', () => {
    expect(folderNumber('01-tra-xanh')).toBe('01')
    expect(folderNumber('112-dai')).toBe('112')
  })
  it('returns null without an NN- prefix', () => {
    expect(folderNumber('tra-xanh')).toBeNull()
    expect(folderNumber('01')).toBeNull()
  })
})

describe('volumeLabel', () => {
  const book = 'Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc Cho Cả Nước Nghe'

  it('uses a real volume title', () => {
    expect(volumeLabel({ index: 2, title: 'Hậu trường sóng gió' }, book)).toBe('Hậu trường sóng gió')
  })
  it('falls back to Tập N when the volume repeats the book title', () => {
    expect(volumeLabel({ index: 1, title: book }, book)).toBe('Tập 1')
    expect(volumeLabel({ index: 3, title: `  ${book.toUpperCase()}  ` }, book)).toBe('Tập 3')
  })
  it('falls back to Tập N when the volume has no title or no book title is known', () => {
    expect(volumeLabel({ index: 2, title: '' }, book)).toBe('Tập 2')
    expect(volumeLabel({ index: 2 }, book)).toBe('Tập 2')
    expect(volumeLabel({ index: 2, title: book }, null)).toBe(book)
  })
})

describe('nextStoryNumber', () => {
  it('starts at 01 for an empty project', () => {
    expect(nextStoryNumber([])).toBe('01')
  })
  it('takes max + 1, not count + 1, so gaps are never reused', () => {
    expect(nextStoryNumber(stories('01-a', '04-b', '02-c'))).toBe('05')
  })
  it('ignores folders without a numeric prefix', () => {
    expect(nextStoryNumber(stories('ghi-chu', '02-c'))).toBe('03')
  })
  it('keeps growing past two digits', () => {
    expect(nextStoryNumber(stories('99-z'))).toBe('100')
  })
})

describe('formatBytes', () => {
  it('uses a decimal comma like the artboard', () => {
    expect(formatBytes(79_667)).toBe('77,8 KB')
    expect(formatBytes(1_258_291)).toBe('1,2 MB')
    expect(formatBytes(512)).toBe('512 B')
  })
})

describe('chapterStatusLabel', () => {
  it('uses the artboard rewrite wording while the story is being rewritten', () => {
    expect(chapterStatusLabel('done', true)).toBe('Đã viết lại')
    expect(chapterStatusLabel('rewriting', true)).toBe('Đang viết lại')
    expect(chapterStatusLabel('rewrite', true)).toBe('Chờ viết lại')
    expect(chapterStatusLabel('pending', true)).toBe('Chờ viết lại')
  })
  it('keeps plain writing wording otherwise', () => {
    expect(chapterStatusLabel('done', false)).toBe('Hoàn thành')
    expect(chapterStatusLabel('writing', false)).toBe('Đang viết')
    expect(chapterStatusLabel('pending', false)).toBe('Chưa viết')
  })
  it('detects the rewrite flow from chapter statuses', () => {
    expect(isRewriteFlow([{ status: 'done' }, { status: 'rewrite' }])).toBe(true)
    expect(isRewriteFlow([{ status: 'done' }, { status: 'pending' }])).toBe(false)
  })
})

describe('autoExportSentence', () => {
  const base: AutoExport = { onVolume: true, onBook: true, onChapter: false, formats: ['txt', 'epub'], pattern: '', onExists: 'replace' }
  it('says it in words, like the artboard, and notes that TTS is on demand', () => {
    expect(autoExportSentence(base)).toBe('Tự xuất văn bản và ebook mỗi khi xong một tập hoặc cả truyện. Bản đọc TTS xuất khi bạn cần.')
  })
  it('covers single triggers and chapter-by-chapter export', () => {
    expect(autoExportSentence({ ...base, onBook: false, formats: ['epub'] })).toBe('Tự xuất ebook mỗi khi xong một tập. Bản đọc TTS xuất khi bạn cần.')
    expect(autoExportSentence({ ...base, onVolume: false, onChapter: true })).toBe('Tự xuất văn bản và ebook sau mỗi chương và mỗi khi xong cả truyện. Bản đọc TTS xuất khi bạn cần.')
  })
  it('lists bản đọc TTS like any other format once it is turned on', () => {
    expect(autoExportSentence({ ...base, formats: ['txt', 'epub', 'narration'] }))
      .toBe('Tự xuất văn bản, ebook và bản đọc TTS mỗi khi xong một tập hoặc cả truyện.')
  })
  it('says so when switched off', () => {
    expect(autoExportSentence({ ...base, onVolume: false, onBook: false })).toBe('Xuất tự động đang tắt.')
    expect(autoExportSentence({ ...base, formats: [] })).toBe('Xuất tự động đang tắt.')
  })
})
