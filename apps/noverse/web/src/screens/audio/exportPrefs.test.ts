import { describe, expect, it } from 'vitest'
import { audioExportOptions, exportVisibility, defaultExportPrefs, exportPrefsOf } from './export'

describe('tuỳ chọn xuất dùng chung', () => {
  it('chỉ hiện mốc chương khi cả tập M4B và cách nhận khi xuất lẻ', () => {
    expect(exportVisibility(false, 'm4b')).toEqual({ intro: true, gap: true, chapterMarks: true, pack: false })
    for (const format of ['mp3', 'wav'] as const) expect(exportVisibility(false, format).chapterMarks).toBe(false)
    for (const format of ['mp3', 'm4b', 'wav'] as const) expect(exportVisibility(true, format)).toEqual({ intro: false, gap: false, chapterMarks: false, pack: true })
  })
  it('giữ sở thích nhưng bỏ các tuỳ chọn không áp dụng khi xuất lẻ', () => {
    const form = { ...defaultExportPrefs, gap: true, whenReady: false, split: true, chapters: [2, 4], pack: 'zip' as const }
    expect(audioExportOptions(form)).toMatchObject({ intro: false, gapSec: 0, chapterMarks: false, split: true, pack: 'zip', chapters: [2, 4], subtitles: true })
    expect(exportPrefsOf(form)).toEqual({ ...defaultExportPrefs, pack: 'zip' })
  })
})
