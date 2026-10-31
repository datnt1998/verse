import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { TextItem } from '@editor/types/timeline'
import { WordCaptionContent } from './word-caption-content'

const item: TextItem = { id: 'caption', type: 'text', trackId: 'text', from: 0, durationInFrames: 90, label: 'Caption', text: 'Xin chào', color: '#ffffff', fontSize: 30, words: [{ text: 'Xin', startFrame: 0, endFrame: 30 }, { text: 'chào', startFrame: 30, endFrame: 60 }], captionPreset: 'karaoke' }

function word(container: HTMLElement, index: number) {
  return container.querySelector<HTMLElement>(`[data-caption-word="${index}"]`)!
}

describe('WordCaptionContent', () => {
  afterEach(cleanup)

  it('keeps the karaoke highlight clipped to the text while it advances frame by frame', () => {
    const { container, rerender } = render(<WordCaptionContent item={item} frame={5} fps={30} />)
    for (const frame of [10, 20, 29]) rerender(<WordCaptionContent item={item} frame={frame} fps={30} />)
    expect(word(container, 0).style.backgroundImage).toContain('linear-gradient')
    expect(word(container, 0).style.backgroundClip).toBe('text')
    expect(word(container, 0).style.color).toBe('transparent')
  })

  it('highlights spoken words as one whole word — never a mid-word sweep', () => {
    const { container } = render(<WordCaptionContent item={item} frame={15} fps={30} />)
    // Frame 15 sits inside word 0's span: it is fully lit (100%), word 1 not at all.
    expect(word(container, 0).style.backgroundImage).toContain('100%')
    expect(word(container, 1).style.backgroundImage).toBe('')
    expect(word(container, 1).style.color).not.toBe('transparent')
  })

  it('keeps earlier words lit once spoken, like the MP4 karaoke step', () => {
    const { container } = render(<WordCaptionContent item={item} frame={45} fps={30} />)
    expect(word(container, 0).style.backgroundImage).toContain('100%')
    expect(word(container, 1).style.backgroundImage).toContain('100%')
  })
})
