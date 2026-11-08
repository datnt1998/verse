import { expect, it } from 'vitest'
import type { MaskVertex } from '@editor/types/masks'
import { Children, isValidElement, type ReactNode } from 'react'
import { getCommandExecutor } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { makeTimelineTrack, resetTimelineCompositionTestState } from '@editor/features/timeline/test-helpers'
import type { ShapeItem } from '@editor/types/timeline'
import { ShapePathSection, subdividePath } from './shape-path'

const vertices: MaskVertex[] = [
  { position: [0, 0], inHandle: [-0.1, 0.2], outHandle: [0.3, 0.6] },
  { position: [1, 1], inHandle: [-0.2, 0.1], outHandle: [0.2, -0.4] },
  { position: [0.5, 0.2], inHandle: [0.1, 0.3], outHandle: [-0.2, -0.3] },
]
function point(start: MaskVertex, end: MaskVertex, t: number) {
  return start.position.map((p, axis) => (1-t)**3*p + 3*(1-t)**2*t*(p+start.outHandle[axis]!) + 3*(1-t)*t*t*(end.position[axis]!+end.inHandle[axis]!) + t**3*end.position[axis]!)
}
for (const closed of [false, true]) it(`preserves the cubic outline when adding a point to a ${closed ? 'closed' : 'open'} path`, () => {
  const original = structuredClone(vertices)
  const split = subdividePath(vertices, closed)
  const from = closed ? 2 : 1
  const end = closed ? 0 : 2
  for (const t of [0, 0.125, 0.3, 0.5, 0.75, 0.9, 1]) {
    const expected = point(vertices[from]!, vertices[end]!, t)
    const actual = t <= 0.5 ? point(split[from]!, split[from+1]!, t*2) : point(split[from+1]!, split[closed ? 0 : from+2]!, t*2-1)
    expect(actual[0]).toBeCloseTo(expected[0]!, 12)
    expect(actual[1]).toBeCloseTo(expected[1]!, 12)
  }
  expect(vertices).toEqual(original)
  expect(split[closed ? 1 : 0]).toEqual(vertices[closed ? 1 : 0])
})

it('keeps rapid point addition and deletion as separate undoable edits', () => {
  resetTimelineCompositionTestState()
  const executor = getCommandExecutor()
  executor.clearHistory()
  useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'path-track', name: 'Đường', kind: 'video', order: 0 })])
  useItemsStore.getState().setItems([{ id: 'path', type: 'shape', trackId: 'path-track', label: 'Đường', shapeType: 'path', fillColor: '#ffaa00', pathVertices: structuredClone(vertices), from: 0, durationInFrames: 60 }])
  const item = () => useItemsStore.getState().itemById.path as ShapeItem
  const initial = structuredClone(item().pathVertices)
  const click = (label: string) => {
    let action: (() => void) | undefined
    function visit(node: ReactNode) {
      Children.forEach(node, child => {
        if (!isValidElement<{ label?: string; onClick?: () => void; children?: ReactNode }>(child)) return
        if (child.props.label === label) action = child.props.onClick
        visit(child.props.children)
      })
    }
    visit(ShapePathSection({ item: item() }))
    if (!action) throw new Error('Missing path action')
    action()
  }
  click('Thêm điểm đường vẽ')
  const added = structuredClone(item().pathVertices)
  click('Xoá điểm 4')
  expect(item().pathVertices).toHaveLength(3)
  executor.undo()
  expect(item().pathVertices).toEqual(added)
  executor.undo()
  expect(item().pathVertices).toEqual(initial)
  executor.redo()
  expect(item().pathVertices).toEqual(added)
})
