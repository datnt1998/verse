import { describe, expect, it } from 'vitest'
import type { Brain, BrainEntity } from '../../api/types'
import { mockBrainDetailOf, SAMPLE_STORY } from '../../../mock/brain'
import { charHeight, charWidth, edgeCurve, edgesAt, graphBounds, nodeRect, otherWidth, overlap, relsAt, type Rect } from '../brain/model'
import { createForceGraph, LabelCollisionIndex, placeForceLabel } from '../brain/forceGraph'

const sample = mockBrainDetailOf(SAMPLE_STORY)!
describe('Gói U — bộ lọc quan hệ dùng chung', () => {
  it('chương 4 không có cạnh Phùng Chấn–Nhược Vy; chương 5 mới có, và chi tiết khớp đồ thị', () => {
    for (const chapter of [4, 5]) {
      const graph = edgesAt(sample, chapter).filter(({ edge }) => edge.a === 'pc' || edge.b === 'pc')
      const detail = relsAt(sample, 'pc', chapter)
      expect(detail.map((row) => row.label)).toEqual(graph.map((row) => row.phase.label))
      expect(detail.some((row) => row.otherId === 'nv')).toBe(chapter === 5)
      expect(detail.some((row) => row.label === 'Ép xoá bản live')).toBe(true)
    }
  })
})

// 35 nút và nhiều cạnh quanh người kể: cùng bố cục/đặt nhãn cho màn đầy đủ và compact.
const entities: BrainEntity[] = Array.from({ length: 35 }, (_, index) => ({
  id: `x${index}`, type: index < 7 ? 'char' : 'obj', name: `Thực thể có tên dài ở vị trí ${index}`,
  role: index < 7 ? 'Vai trò người trong cuộc' : '', fromChapter: 1,
  desc: '', appears: [1, 2, 3, 4], aliases: [], timeline: [],
}))
const dense: Brain = { chapters: sample.chapters, entities, edges: entities.slice(1).map((entity) => ({
  a: 'x0', b: entity.id, phases: [{ fromChapter: 1, type: 'rel', label: 'Quan hệ có mô tả rất dài' }],
})) }
describe('Gói U — 35 nút không chồng chữ', () => {
  const boxes = entities.map((entity, index) => ({
    id: entity.id, w: entity.type === 'char' ? charWidth(index === 0, index === 0 ? 150 : 132) : otherWidth(136),
    h: entity.type === 'char' ? charHeight(index === 0) : 32,
  }))
  const graph = createForceGraph(entities, edgesAt(dense, 4), Object.fromEntries(boxes.map((box) => [box.id, box])))
  const positions = graph.positions
  const rects = boxes.map((box) => nodeRect(positions.get(box.id)!, box))
  // Vùng vẽ nới rộng theo số nút: thẻ phải nằm trong graphBounds, không bị ép vào khung cố định.
  const bounds = graphBounds(positions, Object.fromEntries(boxes.map((box) => [box.id, box])))
  it('thẻ tên không đè nhau và nằm trong đồ thị', () => {
    for (let i = 0; i < rects.length; i++) {
      expect(rects[i][0]).toBeGreaterThanOrEqual(bounds.x)
      expect(rects[i][2]).toBeLessThanOrEqual(bounds.x + bounds.w)
      for (let j = i + 1; j < rects.length; j++) expect(overlap(rects[i], rects[j])).toBe(0)
    }
  })
  it('nhãn cạnh né mọi thẻ và mọi nhãn đã đặt', () => {
    const placed: Rect[] = []
    const labelIndex = new LabelCollisionIndex(rects)
    for (const { edge, index } of edgesAt(dense, 4)) {
      const a = boxes.find((box) => box.id === edge.a)!
      const b = boxes.find((box) => box.id === edge.b)!
      const curve = edgeCurve(positions.get(edge.a)!, positions.get(edge.b)!, a, b, index)
      const spot = placeForceLabel(curve, 161, labelIndex, bounds)
      if (!spot) continue
      expect(rects.some((rect) => overlap(spot.rect, rect) > 0)).toBe(false)
      expect(placed.some((rect) => overlap(spot.rect, rect) > 0)).toBe(false)
      placed.push(spot.rect)
    }
    expect(placed.length).toBeGreaterThan(0)
  })
})
