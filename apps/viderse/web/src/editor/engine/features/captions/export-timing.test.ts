import { expect, it } from 'vitest'
import { convertTimelineToComposition } from '@editor/features/export/utils/timeline-to-composition'
import type { TextItem, TimelineTrack } from '@editor/types/timeline'

it('rebases word timings when export in/out markers cut through a caption', () => {
  const item: TextItem = { id: 'caption', type: 'text', trackId: 'captions', from: 0, durationInFrames: 90, label: 'Caption', text: 'Xin chào', color: '#fff', words: [{ text: 'Xin', startFrame: 0, endFrame: 30 }, { text: 'chào', startFrame: 30, endFrame: 90 }] }
  const track: TimelineTrack = { id: 'captions', name: 'Phụ đề', height: 64, order: 0, locked: false, visible: true, muted: false, solo: false, items: [item] }
  const composition = convertTimelineToComposition([track], [item], [], 30, 640, 360, 15, 60, [])
  const exported = composition.tracks?.[0]?.items?.[0] as TextItem
  expect(exported.words).toEqual([{ text: 'Xin', startFrame: 0, endFrame: 15 }, { text: 'chào', startFrame: 15, endFrame: 45 }])
  expect(item.words?.[0]?.startFrame).toBe(0)
})
