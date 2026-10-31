import type { CaptionWord, SubtitleSegmentCue } from '@editor/types/timeline'
import { parseSrt, parseVtt } from '@editor/shared/utils/subtitles'

export interface ParsedCaptionTiming {
  words: CaptionWord[]
  estimated: boolean
  cues: SubtitleSegmentCue[]
  durationInFrames: number
}

function allocateWords(text: string, start: number, end: number): CaptionWord[] {
  const tokens = text.replace(/<[^>]+>/g, '').replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').trim().split(/\s+/).filter(Boolean)
  const total = tokens.reduce((sum, token) => sum + [...token].length, 0)
  let consumed = 0
  return tokens.flatMap((token) => {
    const left = Math.round(start + (end - start) * consumed / total)
    consumed += [...token].length
    const right = Math.round(start + (end - start) * consumed / total)
    return right > left ? [{ text: token, startFrame: left, endFrame: right }] : []
  })
}

function timestampSeconds(stamp: string): number {
  return stamp.replace(',', '.').split(':').reduce((sum, part) => sum * 60 + Number(part), 0)
}

/** The json3 event shape mirrors src/server/videos/parse.ts, preserving seg.tOffsetMs. */
export function parseCaptionTiming(raw: string, format: 'srt' | 'vtt' | 'json3', fps: number): ParsedCaptionTiming {
  if (!(Number.isFinite(fps) && fps > 0)) throw new Error('Tốc độ khung hình không hợp lệ.')
  const result: ParsedCaptionTiming = { words: [], estimated: false, cues: [], durationInFrames: 0 }
  if (format === 'json3') {
    const data = JSON.parse(raw) as { events?: { tStartMs?: number; dDurationMs?: number; segs?: { utf8?: string; tOffsetMs?: number }[] }[] }
    for (const [index, event] of (data.events ?? []).entries()) {
      const text = (event.segs ?? []).map((seg) => seg.utf8 ?? '').join('').replace(/\s+/g, ' ').trim()
      if (!text) continue
      const startMs = event.tStartMs ?? 0
      const nextStart = data.events?.[index + 1]?.tStartMs
      const endMs = startMs + (event.dDurationMs ?? (nextStart !== undefined ? nextStart - startMs : 2000))
      const segments = (event.segs ?? []).filter((seg) => seg.utf8?.trim())
      if (segments.some((seg) => seg.tOffsetMs === undefined)) {
        result.estimated = true
        result.words.push(...allocateWords(text, Math.round(startMs * fps / 1000), Math.round(endMs * fps / 1000)))
      } else {
        for (const [segIndex, seg] of segments.entries()) {
          const left = startMs + (seg.tOffsetMs ?? 0)
          const right = Math.min(endMs, segIndex + 1 < segments.length ? startMs + (segments[segIndex + 1]?.tOffsetMs ?? 0) : endMs)
          if ((seg.utf8?.trim().split(/\s+/).length ?? 0) > 1) result.estimated = true
          result.words.push(...allocateWords(seg.utf8 ?? '', Math.round(left * fps / 1000), Math.round(right * fps / 1000)))
        }
      }
      result.cues.push({ id: `cue-${index}`, text, startSeconds: startMs / 1000, endSeconds: endMs / 1000 })
    }
  } else {
    const parsed = format === 'srt' ? parseSrt(raw) : parseVtt(raw)
    for (const [index, cue] of parsed.cues.entries()) {
      result.cues.push({ id: `cue-${index}`, text: cue.text.replace(/<[^>]+>/g, ''), startSeconds: cue.startSeconds, endSeconds: cue.endSeconds })
    }
    // Parse original VTT bodies: the existing parser deliberately strips inline timestamps.
    if (format === 'vtt' && /<\d{2}:\d{2}(?::\d{2})?\.\d{3}>/.test(raw)) {
      for (const block of raw.replace(/\r\n?/g, '\n').split(/\n\s*\n/)) {
        const lines = block.split('\n')
        const at = lines.findIndex((line) => line.includes('-->'))
        if (at < 0) continue
        const range = lines[at]?.match(/([\d:.]+)\s*-->\s*([\d:.]+)/)
        if (!range) continue
        const end = timestampSeconds(range[2]!)
        let cursor = timestampSeconds(range[1]!)
        let bodyAt = 0
        const body = lines.slice(at + 1).join(' ')
        const stamps = [...body.matchAll(/<(\d{2}:\d{2}(?::\d{2})?\.\d{3})>/g)]
        if (!stamps.length) result.estimated = true
        for (const stamp of stamps) {
          const next = Math.max(cursor, Math.min(end, timestampSeconds(stamp[1]!)))
          const text = body.slice(bodyAt, stamp.index)
          if (text.trim()) {
            const words = allocateWords(text, Math.round(cursor * fps), Math.round(next * fps))
            if (words.length > 1) result.estimated = true
            result.words.push(...words)
          }
          cursor = next
          bodyAt = (stamp.index ?? 0) + stamp[0].length
        }
        const remaining = allocateWords(body.slice(bodyAt), Math.round(cursor * fps), Math.round(end * fps))
        if (remaining.length > 1) result.estimated = true
        result.words.push(...remaining)
      }
    } else {
      result.estimated = true
      for (const cue of result.cues) result.words.push(...allocateWords(cue.text, Math.round(cue.startSeconds * fps), Math.round(cue.endSeconds * fps)))
    }
  }
  result.words.sort((a, b) => a.startFrame - b.startFrame)
  result.durationInFrames = Math.max(0, ...result.cues.map((cue) => Math.round(cue.endSeconds * fps)), ...result.words.map((word) => word.endFrame))
  return result
}
