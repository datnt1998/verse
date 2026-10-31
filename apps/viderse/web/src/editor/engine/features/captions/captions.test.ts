import { describe, expect, it } from 'vitest'
import fixture from './fixtures/timed-words.json'
import { parseCaptionTiming } from './parsers'
import { activeCaptionWordIndex, scaleCaptionWords, splitCaptionWords, trimCaptionWords } from './word-timing'
import { renderAnimatedCaption } from '@editor/donor/openreel/text/caption-animation-renderer'

const parsed = parseCaptionTiming(JSON.stringify(fixture), 'json3', 30)

describe('word captions', () => {
  it('preserves json3 event and per-word offsets with exact active intervals', () => {
    expect(parsed.estimated).toBe(false)
    expect(parsed.words.slice(0, 3)).toEqual([{ text: 'Một', startFrame: 30, endFrame: 45 }, { text: 'câu', startFrame: 45, endFrame: 60 }, { text: 'chuyện', startFrame: 60, endFrame: 90 }])
    expect(activeCaptionWordIndex(parsed.words, 44)).toBe(0)
    expect(activeCaptionWordIndex(parsed.words, 45)).toBe(1)
    expect(activeCaptionWordIndex(parsed.words, 90)).toBe(-1)
  })
  it('cuts crossing words and rebases split and trim boundaries', () => {
    const [left, right] = splitCaptionWords(parsed.words, 50, 180)
    expect(left.at(-1)).toEqual({ text: 'câu', startFrame: 45, endFrame: 50 })
    expect(right[0]).toEqual({ text: 'câu', startFrame: 0, endFrame: 10 })
    expect(trimCaptionWords(parsed.words, 35, 65)).toEqual([{ text: 'Một', startFrame: 0, endFrame: 10 }, { text: 'câu', startFrame: 10, endFrame: 25 }, { text: 'chuyện', startFrame: 25, endFrame: 30 }])
  })
  it('rescales word timings for speed changes', () => {
    expect(scaleCaptionWords(parsed.words, 1, 2)[0]).toEqual({ text: 'Một', startFrame: 15, endFrame: 23 })
  })
  it('preserves inline VTT stamps and estimates ordinary captions by character count', () => {
    const vtt = parseCaptionTiming('WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nMột <00:00:01.500>câu <00:00:02.000>chuyện\n', 'vtt', 30)
    expect(vtt.words).toEqual(parsed.words.slice(0, 3))
    expect(vtt.estimated).toBe(false)
    const srt = parseCaptionTiming('1\n00:00:00,000 --> 00:00:03,000\nA BBB\n', 'srt', 30)
    expect(srt.words).toEqual([{ text: 'A', startFrame: 0, endFrame: 23 }, { text: 'BBB', startFrame: 23, endFrame: 90 }])
    expect(srt.estimated).toBe(true)
  })
  it('estimates json3 without offsets without dropping text', () => {
    const data = parseCaptionTiming(JSON.stringify({ events: [{ tStartMs: 0, dDurationMs: 1000, segs: [{ utf8: 'a ' }, { utf8: 'b' }] }] }), 'json3', 30)
    expect(data.words.map((word) => word.text)).toEqual(['a', 'b'])
    expect(data.estimated).toBe(true)
  })
  it('emits consistent renderer states for karaoke, pop, fade and all donor presets', () => {
    const state = renderAnimatedCaption(parsed.words, 52.5, 'karaoke', 30)
    expect(state[1]?.active).toBe(true)
    // Whole-word step: the active word is fully lit, spoken words stay lit, upcoming words are not.
    expect(state[1]?.highlightProgress).toBe(1)
    expect(state[0]?.highlightProgress).toBe(1)
    expect(state[2]?.highlightProgress).toBe(0)
    // Never a partial sweep: every word is 0 or 1 at every (fractional) frame.
    for (let frame = 28; frame <= 92; frame += 0.5) {
      for (const word of renderAnimatedCaption(parsed.words, frame, 'karaoke', 30)) expect([0, 1]).toContain(word.highlightProgress)
    }
    expect(renderAnimatedCaption(parsed.words, 44.5, 'karaoke', 30)[1]?.highlightProgress).toBe(0)
    expect(renderAnimatedCaption(parsed.words, 45, 'karaoke', 30)[1]?.highlightProgress).toBe(1)
    expect(renderAnimatedCaption(parsed.words, 35, 'fade')[0]?.opacity).toBeCloseTo(5 / 9)
    expect(renderAnimatedCaption(parsed.words, 20, 'pop')[0]?.visible).toBe(false)
    for (const preset of ['none', 'word-highlight', 'word-by-word', 'karaoke', 'typewriter', 'bounce', 'pop', 'fade'] as const) {
      for (const word of renderAnimatedCaption(parsed.words, 52, preset)) expect(Number.isFinite(word.scale + word.opacity + word.offsetY)).toBe(true)
    }
  })
})
