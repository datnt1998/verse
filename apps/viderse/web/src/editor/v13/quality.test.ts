import { beforeEach, describe, expect, it } from 'vitest'
import { CommandExecutor } from '@editor/commands/executor'
import { registerAllCommands } from '@editor/commands/handlers'
import { captureProjectState, commitProjectState } from '@editor/commands/project-state'
import { createCommandFixture } from '@editor/commands/test-fixture'
import type { TimelineItem } from '@editor/types/timeline'
import { canExport, captionSignature, qualityRanges, rangeItem, type EditorQuality } from './quality'

const quality: EditorQuality = {
  total: 84, changedSinceMeasure: 0, items: [{ id: 'captionAlignment', label: 'Phụ đề khớp tiếng', status: 'fail', ranges: [
    { cueId: 'late', startSec: 30, endSec: 32, text: 'Phụ đề sau', offsetSec: -.6 },
    { cueId: 'first', startSec: 12, endSec: 14, text: 'Phụ đề đầu', offsetSec: .4 },
  ] }],
}
const caption: TimelineItem = { id: 'first', type: 'text', text: 'Phụ đề đầu', color: 'white', trackId: 'track-video', label: 'Phụ đề', from: 360, durationInFrames: 60, words: [{ text: 'Phụ', startFrame: 0, endFrame: 30 }, { text: 'đề', startFrame: 30, endFrame: 60 }] }

describe('V13 quality selection and export gate', () => {
  it('sorts measured failures so opening Chưa đạt selects the earliest range', () => {
    const ranges = qualityRanges(quality)
    expect(ranges.map((range) => range.cueId)).toEqual(['first', 'late'])
    expect(rangeItem(ranges[0], [caption], 30)?.id).toBe('first')
    expect(ranges[0].start * 30).toBe(360)
  })
  it('retains detailed cue metadata when flattened ranges also exist', () => {
    expect(qualityRanges({ ...quality, ranges: [{ start: 30, end: 32, checkId: 'captionAlignment', status: 'fail' }] })[0]).toMatchObject({ cueId: 'first', text: 'Phụ đề đầu', offsetSec: .4 })
  })
  it('gates an 84 score and allows only a fresh fully measured pass', () => {
    expect(canExport(quality)).toBe(false)
    const passed: EditorQuality = { ...quality, total: 93, items: [{ id: 'captionAlignment', label: 'Phụ đề', status: 'pass' }] }
    expect(canExport(passed)).toBe(true)
    expect(canExport({ ...passed, total: null })).toBe(false)
    expect(canExport({ ...passed, changedSinceMeasure: 1 })).toBe(false)
    expect(canExport({ ...passed, measuring: true })).toBe(false)
    expect(canExport({ ...passed, items: [{ ...passed.items[0], status: 'unknown' }] })).toBe(false)
    expect(canExport({ ...passed, items: [{ ...passed.items[0], stale: true }] })).toBe(false)
  })
})

describe('V13 alignment is one undoable engine edit', () => {
  beforeEach(() => {
    registerAllCommands()
    const fixture = createCommandFixture()
    commitProjectState({ ...fixture, items: [caption] })
  })
  it('alignment changes the timing signature, undo restores it, redo restores alignment', () => {
    const executor = new CommandExecutor()
    const range = qualityRanges(quality)[0]
    const original = captionSignature(range, captureProjectState().items, 30)
    const aligned = { ...caption, from: caption.from + 12 }
    expect(executor.executeCommands([{ type: 'project.patch', params: { patch: { items: [aligned] } } }]).ok).toBe(true)
    const matched = captionSignature(range, captureProjectState().items, 30)
    expect(matched).not.toBe(original)
    expect(executor.undo().ok).toBe(true)
    expect(captionSignature(range, captureProjectState().items, 30)).toBe(original)
    expect(executor.canUndo).toBe(false)
    expect(executor.redo().ok).toBe(true)
    expect(captionSignature(range, captureProjectState().items, 30)).toBe(matched)
  })
})
