import type { CaptionWord } from '@editor/types/timeline'

/** Clip a local word track to [start,end), rebasing its retained words to zero. */
export function trimCaptionWords(words: readonly CaptionWord[], start: number, end: number): CaptionWord[] {
  return words.flatMap((word) => {
    const left = Math.max(start, word.startFrame)
    const right = Math.min(end, word.endFrame)
    return right > left ? [{ ...word, startFrame: left - start, endFrame: right - start }] : []
  })
}

export function splitCaptionWords(words: readonly CaptionWord[], at: number, duration: number): [CaptionWord[], CaptionWord[]] {
  return [trimCaptionWords(words, 0, at), trimCaptionWords(words, at, duration)]
}

/** Timings are post-speed: doubling playback speed halves each boundary. */
export function scaleCaptionWords(words: readonly CaptionWord[], oldSpeed: number, newSpeed: number): CaptionWord[] {
  if (!(oldSpeed > 0 && newSpeed > 0)) throw new Error('Tốc độ phải lớn hơn 0.')
  const ratio = oldSpeed / newSpeed
  return words.map((word) => ({
    ...word,
    startFrame: Math.round(word.startFrame * ratio),
    endFrame: Math.max(Math.round(word.startFrame * ratio) + 1, Math.round(word.endFrame * ratio)),
  }))
}

export function activeCaptionWordIndex(words: readonly CaptionWord[], frame: number): number {
  return words.findIndex((word) => frame >= word.startFrame && frame < word.endFrame)
}
