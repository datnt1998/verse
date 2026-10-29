import { describe, expect, it } from 'vitest'
import type { Brain } from '../../api/types'
import { denseBrainFixture } from '../../../mock/brainScale'
import { createForceGraph, LabelCollisionIndex, neighboursOf, placeForceLabel } from './forceGraph'
import { edgesAt, graphBounds, nodeRect, overlap, edgeCurve, type Box, type Rect } from './model'

const sizesOf = (brain: Brain): Record<string, Box> =>
  Object.fromEntries(brain.entities.map((entity) => [entity.id, { w: entity.type === 'char' ? 210 : 183, h: entity.type === 'char' ? 54 : 32 }]))

describe('bố cục lực giữ thẻ của Não truyện', () => {
  it('cùng seed và thứ tự id cho vị trí xác định, không dùng hàng bốn cột', () => {
    const brain = denseBrainFixture()
    const a = createForceGraph(brain.entities, edgesAt(brain, 1), sizesOf(brain))
    const b = createForceGraph([...brain.entities].reverse(), edgesAt(brain, 1), sizesOf(brain))
    expect([...a.positions]).toEqual([...b.positions])
    expect(new Set([...a.positions.values()].map((point) => point.x)).size).toBeGreaterThan(4)
    a.simulation.stop(); b.simulation.stop()
  })

  it('125 hình chữ nhật không giao nhau sau warmup và sau settle', () => {
    const brain = denseBrainFixture()
    const sizes = sizesOf(brain)
    const graph = createForceGraph(brain.entities, edgesAt(brain, 1), sizes)
    for (const ticks of [0, 100]) {
      graph.simulation.tick(ticks)
      const rects = [...graph.positions].map(([id, point]) => nodeRect(point, sizes[id]))
      for (let i = 0; i < rects.length; i++) {
        for (let j = i + 1; j < rects.length; j++) expect(overlap(rects[i], rects[j])).toBe(0)
      }
    }
    graph.simulation.stop()
  })

  it('tập sáng chỉ chứa nút được rê và đúng hàng xóm, không lặp theo cạnh', () => {
    const brain = denseBrainFixture()
    const active = edgesAt(brain, 1)
    const id = brain.entities[1].id
    expect([...neighboursOf(active, id)].sort()).toEqual([brain.entities[0].id, id].sort())
    expect([...neighboursOf(active, 'không có')]).toEqual(['không có'])
  })

  it('đổi chương giữ vị trí nút cũ dưới 20 px và thêm nút mới gần hàng xóm', () => {
    const brain = denseBrainFixture()
    brain.entities.slice(30).forEach((entity) => { entity.fromChapter = 2 })
    const sizes = sizesOf(brain)
    const first = createForceGraph(brain.entities.filter((entity) => entity.fromChapter <= 1), edgesAt(brain, 1), sizes)
    const before = new Map([...first.positions].map(([id, point]) => [id, { x: point.x, y: point.y }]))
    const next = createForceGraph(brain.entities, edgesAt(brain, 2), sizes, before)
    next.simulation.tick(100)
    for (const [id, point] of before) {
      const after = next.positions.get(id)!
      expect(Math.hypot(after.x - point.x, after.y - point.y)).toBeLessThan(20)
    }
    first.simulation.stop(); next.simulation.stop()
  })

  it('vẽ seed vòng tròn ngay, để cả 300 tick cho các đợt không chặn paint', () => {
    const brain = denseBrainFixture()
    const sizes = sizesOf(brain)
    const graph = createForceGraph(brain.entities, edgesAt(brain, 1), sizes, new Map(), 0)
    expect(graph.warmupRemaining).toBe(300)
    expect(graph.simulation.alpha()).toBe(1)
    const rects = [...graph.positions].map(([id, point]) => nodeRect(point, sizes[id]))
    rects.forEach((rect, index) => rects.slice(index + 1).forEach((other) => expect(overlap(rect, other)).toBe(0)))
  })

  it('nhãn đặt tham lam né thẻ và nhãn trước; hết chỗ thì bỏ nhãn', () => {
    const brain = denseBrainFixture()
    const sizes = sizesOf(brain)
    const active = edgesAt(brain, 1)
    const graph = createForceGraph(brain.entities, active, sizes)
    const boxes = [...graph.positions].map(([id, point]) => nodeRect(point, sizes[id]))
    const bounds = graphBounds(graph.positions, sizes)
    const placed: Rect[] = []
    const labelIndex = new LabelCollisionIndex(boxes)
    for (const { edge, index } of active) {
      const curve = edgeCurve(graph.positions.get(edge.a)!, graph.positions.get(edge.b)!, sizes[edge.a], sizes[edge.b], index)
      const spot = placeForceLabel(curve, 110, labelIndex, bounds)
      if (!spot) continue
      expect(boxes.some((rect) => overlap(rect, spot.rect) > 0)).toBe(false)
      expect(placed.some((rect) => overlap(rect, spot.rect) > 0)).toBe(false)
      placed.push(spot.rect)
    }
    expect(placed.length).toBeGreaterThan(0)
    expect(placeForceLabel({ ax: 0, ay: 0, qx: 50, qy: 50, bx: 100, by: 100, d: '' }, 110, new LabelCollisionIndex([[-100, -100, 1000, 1000]]), bounds)).toBeNull()
  })
})
