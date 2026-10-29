import type { AudioTimelineChapter } from '../../api/types'
import { t } from '../../i18n/vi'

/** A zero-second legacy value is not a generated chapter duration. */
export const timelineSeconds = (chapter: AudioTimelineChapter) => chapter.sec !== null && chapter.sec > 0 ? chapter.sec : chapter.estSec
export const timelineMode = (averageWidth: number): 'full' | 'number' | 'bar' => averageWidth >= 72 ? 'full' : averageWidth >= 28 ? 'number' : 'bar'

export function timelineIndex(key: string, index: number, count: number): number | null {
 const next = key === 'ArrowRight' ? index + 1 : key === 'ArrowLeft' ? index - 1 : key === 'Home' ? 0 : key === 'End' ? count - 1 : null
 return next === null || count === 0 ? null : Math.max(0, Math.min(count - 1, next))
}

const hoursMinutes = (seconds: number) => t.audioTimeline.duration(Math.floor(seconds / 3600), String(Math.floor((seconds % 3600) / 60)).padStart(2, '0'))
export function timelineTotal(chapters: AudioTimelineChapter[], madeSec: number, estTotalSec = chapters.reduce((sum, chapter) => sum + timelineSeconds(chapter), 0)): string {
 const done = chapters.filter(chapter => chapter.state === 'done').length
 return t.audioTimeline.total(hoursMinutes(madeSec), done === chapters.length ? null : hoursMinutes(estTotalSec), done, chapters.length)
}

export function timelineTicks(chapters: AudioTimelineChapter[]): { index: number; active: boolean }[] {
 const count = chapters.length
 const step = count <= 60 ? 5 : count <= 150 ? 10 : 25
 const active = chapters.findIndex(chapter => chapter.state === 'rendering')
 return chapters.flatMap((_, index) => {
  const endpoint = index === 0 || index === count - 1
  const visible = endpoint || index === active || (index + 1) % step === 0
  return visible && (endpoint || active < 0 || index === active || Math.abs(index - active) > 1) ? [{ index, active: index === active }] : []
 })
}

/** Pixel centres include the actual gaps, so labels line up with unequal cells. */
export function timelineCentres(chapters: AudioTimelineChapter[], width: number, gap: number): number[] {
 const total = chapters.reduce((sum, chapter) => sum + timelineSeconds(chapter), 0)
 const usable = Math.max(0, width - gap * Math.max(0, chapters.length - 1))
 let cursor = 0
 return chapters.map(chapter => {
  const cell = total > 0 ? timelineSeconds(chapter) / total * usable : 0
  const centre = cursor + cell / 2
  cursor += cell + gap
  return centre
 })
}
