import { describe, expect, it } from 'vitest'
import { audioExportOptions, type AudioExportForm } from './export'

const form: AudioExportForm = {
  format: 'm4b', chapterMarks: true, subtitles: true, whenReady: true,
  intro: false, chapterTitles: true, loudness: false, gap: true, gapSec: 3,
}

describe('tuỳ chọn xuất audio', () => {
  it('giữ đúng định dạng và mọi công tắc độc lập khi gửi API', () => {
    expect(audioExportOptions(form)).toEqual({
      format: 'm4b', chapterMarks: true, subtitles: true, whenReady: true,
      intro: false, chapterTitles: true, loudness: false, gapSec: 3,
    })
    expect(audioExportOptions({ ...form, format: 'mp3', chapterMarks: false, subtitles: false, whenReady: false, gap: false })).toEqual({
      format: 'mp3', chapterMarks: false, subtitles: false, whenReady: false,
      intro: false, chapterTitles: true, loudness: false, gapSec: 0,
    })
    expect(audioExportOptions({ ...form, format: 'wav' }).format).toBe('wav')
    expect(audioExportOptions({ ...form, chapters: [1, 3] }).chapters).toEqual([1, 3])
  })
})
