import type { ExportFile } from '../../../api/types'

const inChapterName = (name: string, chapter: number) => new RegExp(`chương\\s*0*${chapter}\\b`, 'i').test(name)

/** Tệp của một chương có thể chỉ nằm trong `chapterFiles`, không có trong `files`. */
export function chapterExportFiles(chapterFiles: readonly ExportFile[], focusChapter?: number): ExportFile[] {
  if (focusChapter === undefined) return [...chapterFiles]
  return chapterFiles.filter((item) => item.chapter === undefined ? inChapterName(item.name, focusChapter) : item.chapter === focusChapter)
}

/** Phụ đề của đúng chương đang chọn — không lôi .srt của cả tập vào. */
export function chapterSubtitleFiles(files: readonly ExportFile[], scopedAudio: readonly ExportFile[], focusChapter?: number): ExportFile[] {
  const subtitles = files.filter((item) => item.name.endsWith('.srt'))
  if (focusChapter === undefined) return subtitles
  return subtitles.filter((item) => item.chapter === undefined
    ? inChapterName(item.name, focusChapter) ||
      scopedAudio.some((audio) => item.name.replace(/\.srt$/, '') === audio.name.replace(/\.[^.]+$/, ''))
    : item.chapter === focusChapter)
}
