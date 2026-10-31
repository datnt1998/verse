import { describe, expect, it } from 'vitest'
import { createNodeSession } from '../node-session'
import { captureProjectState } from '../project-state'
import { createSeededIdFactory } from '../id-factory'
import { registerCaptionCommands } from './caption'
import type { TextItem } from '@editor/types/timeline'

registerCaptionCommands()

describe('caption commands', () => {
  it('imports words, edits, validates and undoes the canonical data', () => {
    const item: TextItem = { id: 'text', type: 'text', trackId: 'track', from: 0, durationInFrames: 90, label: 'Caption', text: '', color: '#fff' }
    const session = createNodeSession({ ...captureProjectState(), items: [item], tracks: [], keyframes: [] }, createSeededIdFactory('caption'))
    try {
      expect(session.executor.executeCommands([{ type: 'caption.importTimed', params: { itemId: 'text', data: { words: [{ text: 'Xin', startFrame: 0, endFrame: 30 }, { text: 'chào', startFrame: 30, endFrame: 60 }], cues: [], estimated: false, durationInFrames: 60 } } }, { type: 'caption.setPreset', params: { itemId: 'text', preset: 'karaoke' } }]).ok).toBe(true)
      expect((session.getState().items[0] as TextItem).captionPreset).toBe('karaoke')
      expect(session.executor.executeCommands([{ type: 'caption.updateWord', params: { itemId: 'text', index: 0, word: { endFrame: 100 } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'caption.updateWord', params: { itemId: 'text', index: 0, word: { text: 'Chào' } } }]).ok).toBe(true)
      session.executor.undo()
      expect((session.getState().items[0] as TextItem).words?.[0]?.text).toBe('Xin')
      session.executor.undo()
      expect((session.getState().items[0] as TextItem).words).toBeUndefined()
    } finally { session.dispose() }
  })
  it('replays every caption command from JSON with identical canonical state and redo', () => {
    const item: TextItem = { id: 'text', type: 'text', trackId: 'track', from: 0, durationInFrames: 90, label: 'Caption', text: '', color: '#fff' }
    const initial = { ...captureProjectState(), items: [item], tracks: [], keyframes: [] }
    const commands = [
      { type: 'caption.setWords', params: { itemId: 'text', words: [{ text: 'Một', startFrame: 0, endFrame: 30 }, { text: 'hai', startFrame: 30, endFrame: 60 }], estimated: true } },
      { type: 'caption.setPreset', params: { itemId: 'text', preset: 'pop' } },
      { type: 'caption.updateWord', params: { itemId: 'text', index: 1, word: { endFrame: 70 } } },
      { type: 'caption.importTimed', params: { itemId: 'text', data: { words: [{ text: 'Ba', startFrame: 0, endFrame: 45 }], cues: [{ id: 'c1', text: 'Ba', startSeconds: 0, endSeconds: 1.5 }], estimated: false, durationInFrames: 45 } } },
    ]
    const live = createNodeSession(initial, createSeededIdFactory('live'))
    const replayed = createNodeSession(initial, createSeededIdFactory('replayed'))
    try {
      const result = live.executor.executeCommands(commands)
      expect(result.ok).toBe(true)
      if (result.ok !== true) throw new Error('command batch must succeed')
      const roundTrip = JSON.parse(JSON.stringify(result.commands))
      expect(replayed.executor.executeCommands(roundTrip).ok).toBe(true)
      expect(replayed.getState().items).toEqual(live.getState().items)
      live.executor.undo()
      live.executor.redo()
      expect(live.getState().items).toEqual(replayed.getState().items)
      expect((live.getState().items[0] as TextItem).captionPreset).toBe('pop')
    } finally { live.dispose(); replayed.dispose() }
  })
  it('preserves per-word estimate flags through import and canonical replay', () => {
    const item: TextItem = { id: 'text', type: 'text', trackId: 'track', from: 0, durationInFrames: 90, label: 'Caption', text: '', color: '#fff' }
    const session = createNodeSession({ ...captureProjectState(), items: [item], tracks: [], keyframes: [] }, createSeededIdFactory('mixed'))
    try {
      const words = [{ text: 'one', startFrame: 0, endFrame: 30, estimated: false }, { text: 'two', startFrame: 30, endFrame: 60, estimated: true }]
      const result = session.executor.executeCommands([{ type: 'caption.importTimed', params: { itemId: 'text', data: { words, cues: [], estimated: false, durationInFrames: 90 } } }])
      expect(result.ok).toBe(true)
      expect((session.getState().items[0] as TextItem).words).toEqual(words)
      expect((session.getState().items[0] as TextItem).wordsEstimated).toBe(false)
    } finally { session.dispose() }
  })
})
