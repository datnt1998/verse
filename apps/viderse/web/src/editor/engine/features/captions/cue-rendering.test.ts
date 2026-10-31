import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { createNodeSession } from '@editor/commands/node-session'
import { captureProjectState } from '@editor/commands/project-state'
import { registerCaptionCommands } from '@editor/commands/handlers/caption'
import type { SubtitleSegmentItem, TextItem, TimelineItem } from '@editor/types/timeline'
import type { ItemRenderContext } from '@editor/features/export/utils/canvas-item-renderer/types'
import { renderWordCaptionItem } from '@editor/features/export/utils/canvas-item-renderer/word-caption'
import { WordCaptionContent } from '@editor/runtime/composition-runtime/components/word-caption-content'
import { SubtitleSegmentContent } from '@editor/runtime/composition-runtime/components/subtitle-segment-content'
import { parseCaptionTiming } from './parsers'

let previewFrame = 0
vi.mock('@editor/runtime/composition-runtime/deps/player', () => ({ useSequenceContext: () => ({ localFrame: previewFrame }) }))
vi.mock('@editor/runtime/composition-runtime/hooks/use-player-compat', () => ({ useVideoConfig: () => ({ fps: 30 }) }))
vi.mock('@editor/runtime/composition-runtime/components/text-content', () => ({ TextContent: ({ item }: { item: TextItem }) => createElement(WordCaptionContent, { item, frame: previewFrame, fps: 30 }) }))
registerCaptionCommands()

// First two cue windows from the long-video production fixture.
const vtt = 'WEBVTT\n\n1\n00:00:02.000 --> 00:00:06.000\nSynthetic <00:00:03.000>pattern <00:00:04.000>motion <00:00:05.000>check\n\n2\n00:00:08.000 --> 00:00:12.000\nSynthetic <00:00:09.000>pattern <00:00:10.000>motion <00:00:11.000>check\n'
const parsed = parseCaptionTiming(vtt, 'vtt', 30)

function importCaption(): TimelineItem {
  const item: TextItem = { id: 'caption', type: 'text', trackId: 'text', from: 0, durationInFrames: 360, label: 'captions.vtt', text: parsed.words.map((word) => word.text).join(' '), color: '#ffffff', fontSize: 30, captionPreset: 'karaoke' }
  const session = createNodeSession({ ...captureProjectState(), items: [item], tracks: [], keyframes: [] })
  try {
    expect(session.executor.executeCommands([{ type: 'caption.importTimed', params: { itemId: item.id, data: parsed } }]).ok).toBe(true)
    return session.getState().items[0]!
  } finally { session.dispose() }
}

function canvasWords(item: TextItem | SubtitleSegmentItem, frame: number, renderMode: 'preview' | 'export') {
  const fills: { text: string; color: string }[] = []
  const context = { save: vi.fn(), restore: vi.fn(), beginPath: vi.fn(), roundRect: vi.fn(), fill: vi.fn(), clip: vi.fn(), translate: vi.fn(), scale: vi.fn(), strokeText: vi.fn(), rect: vi.fn(), globalAlpha: 1, fillStyle: '', font: '', measureText: (text: string) => ({ width: text.length * 10, fontBoundingBoxAscent: 24, fontBoundingBoxDescent: 6 }), fillText(this: { fillStyle: string }, text: string) { fills.push({ text, color: this.fillStyle }) } } as unknown as OffscreenCanvasRenderingContext2D
  const rctx = { canvasSettings: { width: 1920, height: 1080, fps: 30 }, renderMode, textMeasureCache: { measure: (_ctx: OffscreenCanvasRenderingContext2D, text: string) => text.length * 10 } } as unknown as ItemRenderContext
  renderWordCaptionItem(context, item, { x: 0, y: 400, width: 1600, height: 160 }, frame, rctx)
  return fills
}

describe('imported timed caption cue rendering', () => {
  it('retains imported cue windows in the canonical subtitle item instead of flattening text', () => {
    const item = importCaption()
    expect(item.type).toBe('subtitle')
    expect((item as SubtitleSegmentItem).cues).toEqual(parsed.cues)
    expect(item.id).toBe('caption')
    expect((item as SubtitleSegmentItem).captionPreset).toBe('karaoke')
  })
  it('shows only active cue words in Canvas preview/export, resets highlights and hides gaps', () => {
    const item = importCaption() as TextItem | SubtitleSegmentItem
    for (const mode of ['preview', 'export'] as const) {
      expect(canvasWords(item, 0, mode)).toEqual([])
      expect(canvasWords(item, 180, mode)).toEqual([])
      expect(canvasWords(item, 210, mode)).toEqual([])
      expect(canvasWords(item, 360, mode)).toEqual([])
      for (const frame of [105, 285]) {
        const fills = canvasWords(item, frame, mode)
        // One page, each word painted once: spoken (incl. active) yellow, upcoming in the text colour.
        expect(fills.map((fill) => fill.text)).toEqual(['Synthetic', 'pattern', 'motion', 'check'])
        expect(fills.filter((fill) => fill.color === '#ffffff').map((fill) => fill.text)).toEqual(['motion', 'check'])
        expect(fills.filter((fill) => fill.color === '#facc15').map((fill) => fill.text)).toEqual(['Synthetic', 'pattern'])
      }
    }
  })
  it('uses the same cue windows for DOM playback, including silent intervals', () => {
    const item = importCaption() as TextItem | SubtitleSegmentItem
    for (const frame of [0, 105, 180, 210, 285, 360]) {
      previewFrame = frame
      const html = renderToStaticMarkup(item.type === 'subtitle' ? createElement(SubtitleSegmentContent, { item }) : createElement(WordCaptionContent, { item, frame, fps: 30 }))
      expect((html.match(/data-caption-word=/g) ?? []).length).toBe(frame === 105 || frame === 285 ? 4 : 0)
      if (frame === 105 || frame === 285) expect((html.match(/data-active="true"/g) ?? []).length).toBe(1)
    }
  })
})
