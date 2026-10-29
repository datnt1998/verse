import type { AudioExportOptions, AudioExportPrefs } from '../../api/types'

export type AudioExportForm = Omit<AudioExportOptions, 'gapSec'> & { gap: boolean; gapSec: number }
export const defaultExportPrefs: AudioExportPrefs = {
  format: 'mp3', intro: true, chapterTitles: true, loudness: true,
  gapSec: 2, chapterMarks: true, subtitles: true, pack: 'files',
}

export function exportVisibility(split: boolean, format: AudioExportPrefs['format']) {
  return { intro: !split, gap: !split, chapterMarks: !split && format === 'm4b', pack: split }
}

export function exportPrefsOf(form: AudioExportForm): AudioExportPrefs {
  return { format: form.format, intro: form.intro, chapterTitles: form.chapterTitles,
    loudness: form.loudness, gapSec: form.gap ? form.gapSec : 0,
    chapterMarks: form.chapterMarks, subtitles: form.subtitles, pack: form.pack ?? 'files' }
}

export function audioExportOptions(form: AudioExportForm): AudioExportOptions {
  const visible = exportVisibility(form.split === true, form.format)
  return {
    format: form.format, chapterTitles: form.chapterTitles, loudness: form.loudness, subtitles: form.subtitles,
    chapterMarks: visible.chapterMarks && form.chapterMarks,
    whenReady: !form.split && form.whenReady,
    ...(form.chapters && (form.split || form.chapters.length > 0) ? { chapters: form.chapters } : {}),
    ...(form.split ? { split: true, pack: form.pack ?? 'files' } : {}),
    ...(form.trial ? { trial: true } : {}),
    ...(form.onExists ? { onExists: form.onExists } : {}),
    intro: visible.intro && form.intro,
    gapSec: visible.gap && form.gap ? form.gapSec : 0,
  }
}

