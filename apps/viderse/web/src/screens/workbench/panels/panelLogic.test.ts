import { describe, expect, it } from 'vitest'
import { coverageAfterRemoval, metadataCounts, togglePart } from './panelLogic'

describe('panel logic', () => {
  it('counts editable text and always-added footer and tags', () => {
    expect(metadataCounts('abc', 'hello', ['a', 'bb'], 'bye')).toEqual({ title: 3, description: 8, tags: 2, tagCharacters: 4 })
  })
  it('counts Unicode code points the same way as the metadata API', () => {
    expect(metadataCounts('🙂a', '🙂', ['🙂'], '')).toEqual({ title: 2, description: 1, tags: 1, tagCharacters: 1 })
  })
  it('toggles only the requested rewrite part', () => {
    expect(togglePart(['title'], 'description')).toEqual(['title', 'description'])
    expect(togglePart(['title', 'tags'], 'title')).toEqual(['tags'])
  })
  it('immediately reduces coverage and reports a shortfall', () => {
    expect(coverageAfterRemoval({ fixedSec: 20, channelSec: 90, videoSec: 100 }, 30)).toEqual({ covered: 80, total: 100, short: true })
  })
})
