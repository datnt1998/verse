// @vitest-environment node

import type { TimelineItem } from '@editor/types/timeline'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useTimelineItemOverlayStore } from '@editor/features/timeline/stores/timeline-item-overlay-store'
import { applySilencePreviewOverlays } from './silence-removal-preview'
// Filler-word preview needs transcription (dropped, ML); silence overlays keep coverage.

function audioItem(id: string, mediaId: string): TimelineItem {
  return {
    id,
    type: 'audio',
    name: id,
    trackId: 'track-1',
    from: 30,
    durationInFrames: 90,
    mediaId,
    sourceStart: 0,
    sourceDuration: 90,
    sourceFps: 30,
  } as unknown as TimelineItem
}

beforeEach(() => {
  useTimelineSettingsStore.setState({ fps: 30 })
  useItemsStore.getState().setItems([audioItem('item-1', 'media-1')])
  useTimelineItemOverlayStore.setState({ overlaysByItemId: {} })
})

describe('removal preview overlays', () => {
  it('preserves silence preview overlay labels, tone, ranges, and summary', () => {
    const summary = applySilencePreviewOverlays(['item-1'], {
      'media-1': [{ start: 1.5, end: 2.5 }],
    })

    expect(summary.rangeCount).toBe(1)
    expect(summary.totalSeconds).toBeCloseTo(1)
    expect(useTimelineItemOverlayStore.getState().overlaysByItemId['item-1']).toEqual([
      {
        id: 'silence-removal-preview',
        label: '1 silent range',
        tone: 'error',
        ranges: [{ startRatio: 0.5, endRatio: 0.8333333333333334 }],
      },
    ])
  })

  it('counts a linked audio/video range once in the summary', () => {
    const audio = audioItem('audio-1', 'media-1')
    const video = {
      ...audio,
      id: 'video-1',
      type: 'video' as const,
      linkedGroupId: 'group-1',
      originId: 'origin-1',
    } as TimelineItem
    useItemsStore
      .getState()
      .setItems([
        { ...audio, linkedGroupId: 'group-1', originId: 'origin-1' } as TimelineItem,
        video,
      ])

    const summary = applySilencePreviewOverlays(['audio-1', 'video-1'], {
      'media-1': [{ start: 1.5, end: 2.5 }],
    })

    expect(summary.rangeCount).toBe(1)
    expect(summary.totalSeconds).toBeCloseTo(1)
  })

})
