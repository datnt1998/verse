import { describe, expect, it } from 'vitest'
import { readTemplateSharedAssets } from './templateAssets'

describe('pinned template shared assets', () => {
  it('separates reusable layers from generated slots, empty tracks and unreferenced library overlays', () => {
    const snapshot = {
      overlays: [{ id: 'unused-overlay', name: 'Không nằm trên timeline' }],
      project: { timeline: {
        tracks: [
          { id: 'scenes', name: 'Ảnh mẫu', templateSlot: 'scenes' },
          { id: 'voice', templateSlot: 'source-audio' },
          { id: 'captions', templateSlot: 'captions' },
          { id: 'snow', name: 'Tuyết nhẹ', kind: 'video' },
          { id: 'intro', name: 'Subscribe đầu', visible: false },
          { id: 'empty', name: 'Không có nội dung' },
        ],
        items: [
          { trackId: 'scenes', type: 'image', label: 'Không dùng ảnh mẫu này' },
          { trackId: 'voice', type: 'audio' },
          { trackId: 'captions', type: 'subtitle' },
          { trackId: 'snow', type: 'video', loop: true },
          { trackId: 'intro', type: 'shape' },
          { trackId: 'intro', type: 'text' },
          { trackId: 'orphan', type: 'image' },
        ],
      } },
    }
    expect(readTemplateSharedAssets(snapshot, 'snapshot')).toEqual([
      { id: 'snow', name: 'Tuyết nhẹ', kind: 'video', loop: true, visible: true },
      { id: 'intro', name: 'Subscribe đầu', kind: 'group', loop: false, visible: false },
    ])
  })

  it('keeps unknown inventory distinct from an empty template and does not expose a track ID as its name', () => {
    expect(readTemplateSharedAssets({}, 'snapshot')).toBeUndefined()
    expect(readTemplateSharedAssets({ project: { timeline: { tracks: [], items: [] } } }, 'snapshot')).toEqual([])
    expect(readTemplateSharedAssets({ project: { timeline: {
      tracks: [{ id: 'internal-id', kind: 'audio', muted: true }],
      items: [{ trackId: 'internal-id', type: 'audio' }],
    } } }, 'snapshot')).toEqual([{ id: 'internal-id', name: 'Lớp dùng chung', kind: 'audio', loop: false, visible: false }])
  })
})
