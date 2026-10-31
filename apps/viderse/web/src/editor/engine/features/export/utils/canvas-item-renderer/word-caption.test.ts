import { describe, expect, it, vi } from 'vitest'
import type { TextItem } from '@editor/types/timeline'
import type { ItemRenderContext } from './types'
import { renderWordCaptionItem } from './word-caption'

const item: TextItem = { id: 'caption', type: 'text', trackId: 'text', from: 100, durationInFrames: 90, label: 'Caption', text: 'Xin chào', color: '#ffffff', fontSize: 30, words: [{ text: 'Xin', startFrame: 0, endFrame: 30 }, { text: 'chào', startFrame: 30, endFrame: 60 }], captionPreset: 'karaoke' }

function draw(frame: number, renderMode: 'preview' | 'export') {
  const fills: { text: string; color: string }[] = []
  const context = {
    save: vi.fn(), restore: vi.fn(), beginPath: vi.fn(), roundRect: vi.fn(), fill: vi.fn(), clip: vi.fn(), translate: vi.fn(), scale: vi.fn(), strokeText: vi.fn(), rect: vi.fn(),
    globalAlpha: 1, fillStyle: '', font: '', textAlign: 'left', textBaseline: 'alphabetic',
    measureText: (text: string) => ({ width: text.length * 10, fontBoundingBoxAscent: 24, fontBoundingBoxDescent: 6 }),
    fillText(this: { fillStyle: string }, text: string) { fills.push({ text, color: this.fillStyle }) },
  } as unknown as OffscreenCanvasRenderingContext2D
  const rctx = { canvasSettings: { width: 640, height: 360, fps: 30 }, renderMode, textMeasureCache: { measure: (_ctx: OffscreenCanvasRenderingContext2D, text: string) => text.length * 10 } } as unknown as ItemRenderContext
  renderWordCaptionItem(context, item, { x: 0, y: 80, width: 500, height: 100 }, frame, rctx)
  return { fills, context }
}

describe('word caption canvas painting', () => {
  it('paints spoken words fully highlighted — never a partial clip — identically for preview/export', () => {
    // Frame 145 = 45 into the item: word 0 done, word 1 active → both lit, each as ONE fill.
    const preview = draw(145, 'preview')
    expect(preview.fills).toEqual(draw(145, 'export').fills)
    expect(preview.fills).toEqual([
      { text: 'Xin', color: '#facc15' },
      { text: 'chào', color: '#facc15' },
    ])
    // Whole-word step: no clip rectangle is ever drawn for the highlight.
    expect(preview.context.clip).not.toHaveBeenCalled()
    expect(preview.context.rect).not.toHaveBeenCalled()
  })

  it('paints upcoming words in the text colour and nothing between pages', () => {
    const midWord = draw(115, 'preview')
    expect(midWord.fills).toEqual([
      { text: 'Xin', color: '#facc15' },
      { text: 'chào', color: '#ffffff' },
    ])
    expect(draw(95, 'preview').fills).toEqual([])
  })
})
