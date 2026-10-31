import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { TimelineItem } from '@editor/types/timeline'
import { useItemsStore } from './items-store'
import { useTimelineStore } from './timeline-store-facade'

function textItem(id: string, label: string): TimelineItem {
  return {
    id,
    type: 'text',
    trackId: 'track-1',
    from: 0,
    durationInFrames: 30,
    label,
    text: label,
    color: '#ffffff',
  } as TimelineItem
}

describe('useTimelineStore selector', () => {
  beforeEach(() => {
    useItemsStore.getState().setItems([textItem('a', 'Alpha'), textItem('b', 'Beta')])
  })

  it('re-evaluates when the selector closes over a different item without a store change', () => {
    const { result, rerender } = renderHook(
      ({ id }) => useTimelineStore((state) => state.items.find((item) => item.id === id)?.label),
      { initialProps: { id: 'a' } },
    )
    expect(result.current).toBe('Alpha')

    rerender({ id: 'b' })
    expect(result.current).toBe('Beta')
  })

  it('keeps an undefined selection cached instead of treating it as missing', () => {
    let calls = 0
    const selector = (state: { items: TimelineItem[] }) => {
      calls += 1
      return state.items.find((item) => item.id === 'missing')
    }
    const { result, rerender } = renderHook(() => useTimelineStore(selector))
    const callsAfterMount = calls
    rerender()
    expect(result.current).toBeUndefined()
    expect(calls).toBe(callsAfterMount)
  })
})
