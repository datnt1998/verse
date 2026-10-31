import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { SubtitleSegmentItem, TextItem } from '@editor/types/timeline'
import { WordCaptionContent } from './word-caption-content'
import { SubtitleSegmentContent } from './subtitle-segment-content'

let previewFrame = 0
vi.mock('@editor/runtime/composition-runtime/deps/player', () => ({ useSequenceContext: () => ({ localFrame: previewFrame }) }))
vi.mock('@editor/runtime/composition-runtime/hooks/use-player-compat', () => ({ useVideoConfig: () => ({ fps: 30 }) }))
vi.mock('@editor/runtime/composition-runtime/components/text-content', () => ({ TextContent: ({ item }: { item: TextItem }) => createElement(WordCaptionContent, { item, frame: previewFrame, fps: 30 }) }))

// Stored rolling-window data: cue B starts inside cue A's window and its words
// cross the clipped boundary — the shape test projects carry on disk.
function overlappingItem(): SubtitleSegmentItem {
  return {
    id: 'subs', type: 'subtitle', trackId: 'cap', from: 0, durationInFrames: 180, label: 'Phụ đề',
    source: { type: 'subtitle-import', fileName: 'captions.json', format: 'vtt', importedAt: 0 },
    cues: [
      { id: 'a', text: 'một hai ba', startSeconds: 0.5, endSeconds: 2.5 },
      { id: 'b', text: 'bốn năm', startSeconds: 1.5, endSeconds: 3.5 },
    ],
    words: [
      { text: 'một', startFrame: 15, endFrame: 35 },
      { text: 'hai', startFrame: 35, endFrame: 55 },
      { text: 'ba', startFrame: 55, endFrame: 75 },
      { text: 'bốn', startFrame: 60, endFrame: 90 },
      { text: 'năm', startFrame: 90, endFrame: 105 },
    ],
    captionPreset: 'karaoke',
    color: '#ffffff',
    fontSize: 30,
  }
}

function shown(html: string) {
  return [...html.matchAll(/data-caption-word="\d+"[^>]*>([^<]*)</g)].map((match) => match[1])
}

describe('SubtitleSegmentContent', () => {
  it('shows only the active page when stored cues overlap', () => {
    const item = overlappingItem()
    // Cue A is clipped to [15,45) and owns the words starting inside it ('một','hai');
    // cue B is clipped to [45,105) and owns the words starting inside it ('ba','bốn','năm').
    // A page starts at its first word: cue B's window opens at 45 but 'ba' is first spoken at 55.
    // That 10-frame (333 ms) blank is shorter than a pause, so cue A's page stays up instead of flashing off.
    for (const frame of [20, 44, 45, 54]) {
      previewFrame = frame
      const html = renderToStaticMarkup(createElement(SubtitleSegmentContent, { item }))
      expect(shown(html)).toEqual(['một', 'hai'])
    }
    for (const frame of [55, 70, 104]) {
      previewFrame = frame
      const html = renderToStaticMarkup(createElement(SubtitleSegmentContent, { item }))
      expect(shown(html)).toEqual(['ba', 'bốn', 'năm'])
    }
    // No page before the first cue or after the last.
    for (const frame of [0, 14, 105, 179]) {
      previewFrame = frame
      expect(renderToStaticMarkup(createElement(SubtitleSegmentContent, { item }))).not.toContain('data-caption-word')
    }
  })

  it('shows one page of a split long phrase at a time', () => {
    const text = 'one two three four five six seven eight nine ten'
    const words = text.split(' ').map((token, index) => ({ text: token, startFrame: index * 10, endFrame: index * 10 + 10 }))
    const item: SubtitleSegmentItem = {
      id: 'subs', type: 'subtitle', trackId: 'cap', from: 0, durationInFrames: 150, label: 'Phụ đề',
      source: { type: 'subtitle-import', fileName: 'captions.json', format: 'vtt', importedAt: 0 },
      cues: [{ id: 'a', text, startSeconds: 0, endSeconds: 4 }],
      words,
      captionPreset: 'karaoke',
      color: '#ffffff',
      fontSize: 30,
    }
    previewFrame = 85
    const html = renderToStaticMarkup(createElement(SubtitleSegmentContent, { item }))
    const texts = shown(html)
    expect(texts.join(' ').length).toBeLessThanOrEqual(42)
    expect(texts.length).toBeLessThanOrEqual(7)
    expect(texts).toContain('nine')
  })
})
