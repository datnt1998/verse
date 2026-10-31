import { describe, expect, it } from 'vitest'
import { keysForEvent } from '../../../data/eventKeys'

describe('foundation invalidation', () => {
  it('refreshes file-backed premise and world rules when foundation files change', () => {
    const keys = keysForEvent({ type: 'story.changed', data: { project: 'p', story: 's', parts: ['snapshot', 'outline', 'brain'] } })
    expect(keys).toContainEqual(['story', 'p', 's', 'premise'])
    expect(keys).toContainEqual(['story', 'p', 's', 'world-rules'])
  })
})
