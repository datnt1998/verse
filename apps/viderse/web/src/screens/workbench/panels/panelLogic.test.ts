import { describe, expect, it } from 'vitest'
import { coverageAfterRemoval, metadataCounts, metadataDescriptionBody, metadataDescriptionWithFooter, togglePart } from './panelLogic'

describe('panel logic', () => {
  it('counts editable text and always-added footer and tags', () => {
    expect(metadataCounts('abc', 'hello', ['a', 'bb'], 'bye')).toEqual({ title: 3, description: 8, tags: 2, tagCharacters: 4 })
  })
  it('counts Unicode code points the same way as the metadata API', () => {
    expect(metadataCounts('🙂a', '🙂', ['🙂'], '')).toEqual({ title: 2, description: 1, tags: 1, tagCharacters: 1 })
  })
  it('keeps the locked channel footer exactly once while editing or pasting a package', () => {
    expect(metadataDescriptionBody('Nội dung\n\nTheo dõi kênh', 'Theo dõi kênh')).toBe('Nội dung')
    expect(metadataDescriptionWithFooter('Đã sửa', 'Theo dõi kênh')).toBe('Đã sửa\n\nTheo dõi kênh')
    expect(metadataDescriptionWithFooter('Đã sửa\n\nTheo dõi kênh', 'Theo dõi kênh')).toBe('Đã sửa\n\nTheo dõi kênh')
    expect(metadataDescriptionWithFooter('Theo dõi kênh là nội dung', null)).toBe('Theo dõi kênh là nội dung')
  })
  it('toggles only the requested rewrite part', () => {
    expect(togglePart(['title'], 'description')).toEqual(['title', 'description'])
    expect(togglePart(['title', 'tags'], 'title')).toEqual(['tags'])
  })
  it('immediately reduces coverage and reports a shortfall', () => {
    expect(coverageAfterRemoval({ fixedSec: 20, channelSec: 90, videoSec: 100 }, 30)).toEqual({ covered: 80, total: 100, short: true })
  })
})
