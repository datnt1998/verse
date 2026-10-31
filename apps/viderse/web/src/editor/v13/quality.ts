import type { TimelineItem } from '@editor/types/timeline'
import { request } from '../../api/request'

export type QualityRange = {
  start: number; end: number; checkId: string; status: string
  cueId?: string; itemId?: string; text?: string; offsetSec?: number
}
export type QualityItem = {
  id: string; label: string; status: 'pass' | 'fail' | 'unknown'; evidence?: string; stale?: boolean
  ranges?: { cueId: string; startSec: number; endSec: number; text: string; offsetSec: number; itemId?: string }[]
}
export type EditorQuality = {
  total: number | null; score?: { total: number } | null; measuredAt?: string; items: QualityItem[]
  changedSinceMeasure: number; ranges?: QualityRange[]; measuring?: boolean
  jobId?: string; channelName?: string; templateName?: string; checks?: QualityItem[]
}
export function qualityRanges(quality: EditorQuality): QualityRange[] {
  const detailed = quality.items.flatMap((item) => (item.ranges ?? []).map((range) => ({
    start: range.startSec, end: range.endSec, checkId: item.id, status: item.status,
    cueId: range.cueId, itemId: range.itemId, text: range.text, offsetSec: range.offsetSec,
  })))
  return (detailed.length ? detailed : quality.ranges ?? []).slice().sort((a, b) => a.start - b.start)
}
export function canExport(quality: EditorQuality): boolean {
  const score = quality.total ?? quality.score?.total
  return score != null && score >= 90 && quality.changedSinceMeasure === 0 && !quality.measuring && quality.items.length > 0 && quality.items.every((item) => item.status === 'pass' && !item.stale)
}
/** Match the concrete caption, not whichever clip happens to be selected. */
export function rangeItem(range: QualityRange, items: readonly TimelineItem[], fps: number): TimelineItem | undefined {
  return items.find((item) => item.id === range.itemId || item.id === range.cueId)
    ?? items.find((item) => item.type === 'subtitle' && item.cues.some((cue) => cue.id === range.cueId))
    ?? items.find((item) => (item.type === 'text' || item.type === 'subtitle') && item.from <= Math.round(range.start * fps) && item.from + item.durationInFrames > Math.round(range.start * fps))
}
export function captionSignature(range: QualityRange, items: readonly TimelineItem[], fps: number): string {
  const item = rangeItem(range, items, fps)
  if (!item || (item.type !== 'text' && item.type !== 'subtitle')) return ''
  const cue = item.type === 'subtitle' ? item.cues.find((entry) => entry.id === range.cueId) : undefined
  const words = item.words?.filter((word) => (item.from + word.endFrame) / fps >= range.start - 1 && (item.from + word.startFrame) / fps <= range.end + 1)
  return JSON.stringify({ from: item.from, cue, words, text: item.type === 'text' ? item.text : undefined })
}
export async function fetchQuality(projectId: string, signal?: AbortSignal): Promise<EditorQuality> {
  const quality = await request<EditorQuality>(`/api/editor/${encodeURIComponent(projectId)}/quality`, { signal })
  return { ...quality, items: quality.items ?? quality.checks ?? [], total: quality.total ?? quality.score?.total ?? null, changedSinceMeasure: quality.changedSinceMeasure ?? 0 }
}
