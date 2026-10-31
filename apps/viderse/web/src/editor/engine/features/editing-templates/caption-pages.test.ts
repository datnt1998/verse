import { describe, expect, it } from 'vitest'
import { activeCaptionPage, buildCaptionPages, effectiveCaptionPreset, resolveCaptionPages, type CaptionPageSource } from './caption-pages'

const fps = 30

function cue(startSeconds: number, endSeconds: number, text: string, id = `cue-${startSeconds}`) {
  return { id, text, startSeconds, endSeconds }
}

describe('buildCaptionPages', () => {
  it('clips overlapping cue ends so one page shows at a time', () => {
    // Stored rolling-window shape: cue 2 starts before cue 1 ends.
    const item: CaptionPageSource = {
      cues: [cue(0.08, 4.08, 'một hai', 'a'), cue(2.24, 7.44, 'ba bốn', 'b')],
      words: [
        { text: 'một', startFrame: 2, endFrame: 40 },
        { text: 'hai', startFrame: 40, endFrame: 67 },
        { text: 'ba', startFrame: 68, endFrame: 150 },
        { text: 'bốn', startFrame: 150, endFrame: 223 },
      ],
      durationInFrames: 300,
    }
    const { pages } = buildCaptionPages(item, fps)
    // Cue 2's first word starts one frame after cue 1 is clipped: that one-frame blank would only flash, so page 1 holds until page 2.
    expect(pages.map((page) => [page.startFrame, page.endFrame])).toEqual([
      [2, 68],
      [68, Math.round(7.44 * fps)],
    ])
    for (let frame = 0; frame < 300; frame++) {
      const hits = pages.filter((page) => frame >= page.startFrame && frame < page.endFrame)
      expect(hits.length).toBeLessThanOrEqual(1)
    }
    expect(pages[1]!.words.map((word) => word.text)).toEqual(['ba', 'bốn'])
  })

  it('splits a multi-word piece into single words by character length and carries the source flag', () => {
    const piece = { text: 'Abigail ship', startFrame: 0, endFrame: 60 }
    const build = (estimated: boolean | undefined) =>
      buildCaptionPages({ cues: [cue(0, 2, 'Abigail ship', 'a')], words: [{ ...piece, estimated }], durationInFrames: 90 }, fps)
    const measured = build(undefined)
    const words = measured.pages[0]!.words
    expect(words.map((word) => word.text)).toEqual(['Abigail', 'ship'])
    // 7 chars of 11 → 'Abigail' spans 0..38, 'ship' 38..60.
    expect(words.map((word) => [word.startFrame, word.endFrame])).toEqual([[0, 38], [38, 60]])
    // The split adds no estimate of its own: the piece was not flagged, so the page stays karaoke-capable.
    expect(measured.pages[0]!.estimated).toBe(false)
    expect(measured.degraded).toBe(false)
    const flagged = build(true)
    expect(flagged.pages[0]!.words.every((word) => word.estimated === true)).toBe(true)
    expect(flagged.pages[0]!.estimated).toBe(true)
  })

  it('clips a piece to its page window without calling it estimated', () => {
    const { pages, degraded } = buildCaptionPages(
      {
        cues: [cue(0, 4, 'một hai', 'a'), cue(1, 3, 'ba', 'b')],
        words: [{ text: 'một hai', startFrame: 0, endFrame: 120 }, { text: 'ba', startFrame: 30, endFrame: 90 }],
        durationInFrames: 120,
      },
      fps,
    )
    expect(pages[0]!.words.map((word) => [word.text, word.startFrame, word.endFrame])).toEqual([['một', 0, 15], ['hai', 15, 30]])
    expect(pages[0]!.estimated).toBe(false)
    expect(degraded).toBe(false)
  })

  it('keeps measured single-word pieces verbatim and unestimated', () => {
    const { pages, degraded } = buildCaptionPages(
      {
        cues: [cue(0, 2, 'một hai', 'a')],
        words: [{ text: 'một', startFrame: 0, endFrame: 30 }, { text: 'hai', startFrame: 30, endFrame: 60 }],
        durationInFrames: 90,
      },
      fps,
    )
    expect(pages[0]!.words.map((word) => [word.startFrame, word.endFrame])).toEqual([[0, 30], [30, 60]])
    expect(pages[0]!.estimated).toBe(false)
    expect(degraded).toBe(false)
  })

  it('splits pages over 7 words or 42 chars into tiled windows', () => {
    const text = 'one two three four five six seven eight nine ten'
    const words = text.split(' ').map((token, index) => ({ text: token, startFrame: index * 10, endFrame: index * 10 + 10 }))
    const { pages } = buildCaptionPages({ cues: [cue(0, 4, text, 'a')], words, durationInFrames: 120 }, fps)
    expect(pages.length).toBeGreaterThan(1)
    for (const page of pages) {
      expect(page.words.length).toBeLessThanOrEqual(7)
      expect(page.text.length).toBeLessThanOrEqual(42)
    }
    // Windows tile the cue: page N ends where page N+1 begins, first at cue start, last at cue end.
    expect(pages[0]!.startFrame).toBe(0)
    for (const [index, page] of pages.slice(0, -1).entries()) expect(page.endFrame).toBe(pages[index + 1]!.startFrame)
    expect(pages.at(-1)!.endFrame).toBe(120)
    // Every stored word lands on exactly one page.
    expect(pages.flatMap((page) => page.words.map((word) => word.text))).toEqual(text.split(' '))
  })

  it('splits a char-heavy page even under the word limit', () => {
    const text = 'bây giờ chúng ta cùng nhìn kỹ hơn một chút'
    const words = text.split(' ').map((token, index) => ({ text: token, startFrame: index * 10, endFrame: index * 10 + 10 }))
    const { pages } = buildCaptionPages({ cues: [cue(0, 4, text, 'a')], words, durationInFrames: 120 }, fps)
    expect(pages.length).toBeGreaterThan(1)
    for (const page of pages) expect(page.text.length).toBeLessThanOrEqual(42)
  })

  it('synthesises estimated words for a plain wordless cue but keeps marked-up text static', () => {
    const { pages } = buildCaptionPages({ cues: [cue(0, 2, 'một hai', 'a')], words: [], durationInFrames: 90 }, fps)
    expect(pages[0]!.words.map((word) => word.text)).toEqual(['một', 'hai'])
    expect(pages[0]!.estimated).toBe(true)

    const marked = buildCaptionPages({ cues: [cue(0, 2, '<i>leaning</i> text', 'a')], words: [], durationInFrames: 90 }, fps)
    expect(marked.pages[0]!.words).toEqual([])
    expect(marked.pages[0]!.text).toBe('<i>leaning</i> text')
  })

  it('drops word pieces that start outside every cue window', () => {
    const { pages, degraded } = buildCaptionPages(
      {
        cues: [cue(0, 1, 'một', 'a'), cue(2, 3, 'hai', 'b')],
        words: [{ text: 'một', startFrame: 0, endFrame: 30 }, { text: 'stray', startFrame: 40, endFrame: 50 }, { text: 'hai', startFrame: 60, endFrame: 90 }],
        durationInFrames: 120,
      },
      fps,
    )
    expect(pages.flatMap((page) => page.words.map((word) => word.text))).toEqual(['một', 'hai'])
    expect(degraded).toBe(true)
  })

  it('breaks at a 400 ms measured pause, leaves silence empty, but not at 200 ms', () => {
    const build = (gap: number, estimated = false) => buildCaptionPages({
      cues: [cue(0, 3, 'one two')],
      words: [{ text: 'one', startFrame: 0, endFrame: 30, estimated }, { text: 'two', startFrame: 30 + gap, endFrame: 70, estimated }],
      durationInFrames: 90,
    }, fps).pages
    expect(build(12).map(p => [p.startFrame, p.endFrame])).toEqual([[0, 30], [42, 90]])
    expect(activeCaptionPage(build(12), 36)).toBeNull()
    expect(build(6)).toHaveLength(1)
    expect(build(12, true)).toHaveLength(1)
    expect(build(12, true)[0]!.estimated).toBe(true)
  })
})

describe('resolveCaptionPages', () => {
  const item: CaptionPageSource = {
    cues: [cue(0, 2, 'một hai', 'a')],
    words: [{ text: 'một', startFrame: 0, endFrame: 30 }, { text: 'hai', startFrame: 30, endFrame: 60 }],
    durationInFrames: 90,
  }

  it('memoises per stored-data identity', () => {
    const first = resolveCaptionPages(item, fps)
    expect(resolveCaptionPages({ ...item }, fps)).toBe(first)
    expect(resolveCaptionPages(item, 60)).not.toBe(first)
    const words = [...(item.words ?? [])]
    expect(resolveCaptionPages({ ...item, words }, fps)).not.toBe(first)
  })

  it('activeCaptionPage binary-searches the tiling windows', () => {
    const pages = resolveCaptionPages(item, fps)
    expect(activeCaptionPage(pages, 0)).toBe(pages[0])
    expect(activeCaptionPage(pages, 59)).toBe(pages[0])
    expect(activeCaptionPage(pages, 60)).toBeNull()
  })
})

describe('effectiveCaptionPreset', () => {
  const cloneSource = { type: 'subtitle-import', fileName: 'captions.json' }

  it('honours a legacy captions.json pop preset in preview and export despite estimated timing', () => {
    const item: CaptionPageSource = { source: cloneSource, captionPreset: 'pop', words: [{ text: 'a', startFrame: 0, endFrame: 10 }], durationInFrames: 30 }
    expect(effectiveCaptionPreset(item, { estimated: false })).toBe('pop')
    expect(effectiveCaptionPreset(item, { estimated: true })).toBe('pop')
    expect(effectiveCaptionPreset({ ...item, wordsEstimated: true }, { estimated: false })).toBe('pop')
  })

  it('demotes karaoke to fade on estimated timing but keeps other presets', () => {
    const item: CaptionPageSource = { captionPreset: 'karaoke', words: [{ text: 'a', startFrame: 0, endFrame: 10 }], durationInFrames: 30 }
    expect(effectiveCaptionPreset(item, { estimated: true })).toBe('fade')
    expect(effectiveCaptionPreset({ ...item, captionPreset: 'pop' }, { estimated: true })).toBe('pop')
    expect(effectiveCaptionPreset({ ...item, captionPreset: 'karaoke' }, { estimated: false })).toBe('karaoke')
  })
})
