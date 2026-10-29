import { expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { events, startDataEvents, type EventHandler } from './events'
import { keys } from './keys'

it('coalesces exact invalidations and applies snapshot payloads without refetching them', async () => {
  vi.useFakeTimers()
  let receive!: EventHandler
  vi.spyOn(events, 'subscribe').mockImplementation((_types, handler) => { receive = handler; return () => {} })
  vi.spyOn(events, 'start').mockImplementation(() => {})
  vi.spyOn(events, 'stop').mockImplementation(() => {})
  vi.spyOn(events, 'setWatch').mockImplementation(() => {})
  const invalidate = vi.spyOn(cache, 'invalidate').mockResolvedValue(undefined)
  const stop = startDataEvents('dat')
  try {
    for (let i = 0; i < 8; i++) receive({ project: 'p', story: 's', parts: ['snapshot', 'audio/scripts'] }, { type: 'story.changed', id: String(i) })
    receive({ project: 'p', story: 's', kind: 'snapshot', data: { bookTitle: 'updated' } }, { type: 'live', id: '9' })
    expect(cache.get(keys.story('p', 's', 'snapshot')).data).toEqual({ bookTitle: 'updated' })
    await vi.advanceTimersByTimeAsync(249)
    expect(invalidate).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(invalidate.mock.calls).toEqual([
      [keys.story('p', 's', 'audio', 'script')],
      [keys.project('p', 'stories')],
    ])
  } finally {
    stop()
    vi.restoreAllMocks()
    vi.useRealTimers()
  }
})
