import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { voiceShape } from '@editor/test/waveform-fixture'
import { WaveformVisualizerContent } from './waveform-visualizer-content'
import { ShapeContent } from './shape-content'
import { SequenceContext } from '@editor/runtime/composition-runtime/deps/player'
import { VideoConfigContext } from '@editor/runtime/player/video-config'

afterEach(cleanup)
describe('preview voice waveform', () => {
  it('changes SVG geometry at speech/seek while never creating a second audio element', () => {
    const item = voiceShape()
    const { container, rerender } = render(<WaveformVisualizerContent item={item} frame={15} fps={30} width={128} height={64} />)
    const silence = container.querySelector('path')!.getAttribute('d')
    rerender(<WaveformVisualizerContent item={item} frame={45} fps={30} width={128} height={64} />)
    const speech = container.querySelector('path')!.getAttribute('d')
    expect(speech).not.toBe(silence)
    rerender(<WaveformVisualizerContent item={item} frame={90} fps={60} width={128} height={64} />)
    expect(container.querySelector('path')!.getAttribute('d')).toBe(speech)
    expect(container.querySelector('audio')).toBeNull()
  })
  it('routes ShapeContent sequence-local time into waveform and keeps ordinary rectangles unchanged', () => {
    const item = voiceShape({ from: 20 })
    const { container, rerender } = render(<VideoConfigContext.Provider value={{ id: 'voice', fps: 30, width: 128, height: 64, durationInFrames: 120 }}><SequenceContext.Provider value={{ from: 20, parentFrom: 0, localFrame: 45, durationInFrames: 120 }}><ShapeContent item={item} /></SequenceContext.Provider></VideoConfigContext.Provider>)
    expect(container.querySelector('svg')).toHaveAttribute('aria-label', 'Sóng giọng')
    expect(container.querySelector('svg')).toHaveAttribute('data-waveform-frame', '65')
    rerender(<ShapeContent item={{ ...item, waveformVisualizer: undefined }} />)
    expect(container.querySelector('svg')).not.toHaveAttribute('aria-label', 'Sóng giọng')
  })
  it('shows explicit missing-data status, never a filled rectangle fallback', () => {
    const item = voiceShape(); delete item.waveformVisualizer!.envelopeBase64
    const { container, getByRole } = render(<WaveformVisualizerContent item={item} frame={45} fps={30} width={128} height={64} />)
    expect(getByRole('status')).toHaveTextContent('Thiếu dữ liệu sóng giọng')
    expect(container.querySelector('svg')).toBeNull()
  })
})
