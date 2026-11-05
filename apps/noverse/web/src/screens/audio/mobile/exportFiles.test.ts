import { describe, expect, it } from 'vitest'
import type { ExportFile } from '../../../api/types'
import { chapterExportFiles, chapterSubtitleFiles } from './exportFiles'

const audio = (chapter: number, name: string): ExportFile => ({
  path: `audio/${chapter}-${name}`, name, format: 'audio', size: 1024, updatedAt: null, volume: 1, chapter,
})
const subtitle = (name: string): ExportFile => ({
  path: `audio/${name}`, name, format: 'other', size: 256, updatedAt: null, volume: 1,
})

const chapters = [
  audio(1, 'Truyện · Tập 1 · Chương 01 · Mở đầu.mp3'),
  audio(2, 'Truyện · Tập 1 · Chương 02 · Gặp gỡ.mp3'),
  audio(3, 'Truyện · Tập 1 · Chương 03 · Rời đi.mp3'),
]

describe('chọn tệp xuất của một chương', () => {
  it('giữ tệp chương chỉ có trong chapterFiles, không rơi về files', () => {
    expect(chapterExportFiles(chapters, 2)).toEqual([chapters[1]])
  })

  it('không có chương đang chọn thì giữ toàn bộ chapterFiles', () => {
    expect(chapterExportFiles(chapters)).toEqual(chapters)
  })

  it('chương thiếu không lấy nhầm chương có cùng tiền tố số', () => {
    const files = [audio(10, 'Chương 10.mp3'), audio(11, 'Chương 11.mp3')]
    expect(chapterExportFiles(files, 1)).toEqual([])
    expect(chapterExportFiles(chapters, 4)).toEqual([])
  })

  it('số chương trong metadata có thẩm quyền hơn tên tệp cũ', () => {
    const renamed = audio(2, 'Chương 01.mp3')
    expect(chapterExportFiles([renamed], 1)).toEqual([])
    expect(chapterExportFiles([renamed], 2)).toEqual([renamed])
  })

  it('tệp cũ không có metadata vẫn khớp số chương đầy đủ', () => {
    const legacy = { ...chapters[0], chapter: undefined }
    const other = { ...audio(10, 'Chương 10.mp3'), chapter: undefined }
    expect(chapterExportFiles([legacy, other], 1)).toEqual([legacy])
  })

  it('phụ đề chỉ lấy của chương đang chọn', () => {
    const files = [
      ...chapters,
      subtitle('Truyện · Tập 1 · Chương 01 · Mở đầu.srt'),
      subtitle('Truyện · Tập 1 · Chương 02 · Gặp gỡ.srt'),
    ]
    const scoped = chapterExportFiles(chapters, 2)
    expect(chapterSubtitleFiles(files, scoped, 2)).toEqual([files[4]])
  })

  it('không lấy phụ đề tên gần giống hay metadata thuộc chương khác', () => {
    const selected = audio(1, 'Mở đầu.mp3')
    const exact = subtitle('Mở đầu.srt')
    const similar = subtitle('Mở đầu khác.srt')
    const other = { ...subtitle('Chương 01.srt'), chapter: 2 }
    expect(chapterSubtitleFiles([exact, similar, other], [selected], 1)).toEqual([exact])
  })

  it('không có chương đang chọn thì giữ phụ đề cả tập', () => {
    const files = [subtitle('a.srt'), subtitle('b.srt')]
    expect(chapterSubtitleFiles(files, [], undefined)).toEqual(files)
  })
})
