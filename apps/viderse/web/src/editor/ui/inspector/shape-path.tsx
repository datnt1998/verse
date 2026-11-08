import { Plus, Trash2 } from 'lucide-react'
import { withCommandMergeKey } from '@editor/commands'
import type { MaskVertex } from '@editor/types/masks'
import type { ShapeItem } from '@editor/types/timeline'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { updateItem } from '@editor/features/timeline/stores/timeline-actions'
import { EditorIconButton, NumberField, PropertyRow, SectionHeader } from '../controls'

const coordinates = [
  ['position', 'Điểm'], ['inHandle', 'Tay nắm vào'], ['outHandle', 'Tay nắm ra'],
] as const
const mid = (a: readonly number[], b: readonly number[]): [number, number] => [(a[0]! + b[0]!) / 2, (a[1]! + b[1]!) / 2]
const sub = (a: readonly number[], b: readonly number[]): [number, number] => [a[0]! - b[0]!, a[1]! - b[1]!]

/** Split the last cubic in half without changing its rendered outline. */
export function subdividePath(vertices: readonly MaskVertex[], closed: boolean): MaskVertex[] {
  if (vertices.length < 2) return [...vertices]
  const from = closed ? vertices.length - 1 : vertices.length - 2
  const to = (from + 1) % vertices.length
  const start = vertices[from]!, end = vertices[to]!
  const controlA = start.position.map((v, axis) => v + start.outHandle[axis]!)
  const controlB = end.position.map((v, axis) => v + end.inHandle[axis]!)
  const a = mid(start.position, controlA), b = mid(controlA, controlB), c = mid(controlB, end.position)
  const d = mid(a, b), e = mid(b, c), position = mid(d, e)
  const result = vertices.map((vertex, index) => index === from
    ? { ...vertex, outHandle: sub(a, start.position), tangentMode: 'continuous' as const }
    : index === to ? { ...vertex, inHandle: sub(c, end.position), tangentMode: 'continuous' as const } : vertex)
  result.splice(from + 1, 0, { position, inHandle: sub(d, position), outHandle: sub(e, position), tangentMode: 'continuous' })
  return result
}

export function ShapePathSection({ item }: { item: ShapeItem }) {
  if (item.shapeType !== 'path' || item.waveformVisualizer) return null
  const vertices = item.pathVertices ?? []
  const current = () => {
    const latest = useItemsStore.getState().itemById[item.id]
    return latest?.type === 'shape' && latest.shapeType === 'path' ? latest : null
  }
  const coordinate = (index: number, property: typeof coordinates[number][0], axis: 0 | 1, value: number) => {
    const latest = current()
    if (!latest?.pathVertices?.[index]) return
    const next = latest.pathVertices.map((vertex, at) => at !== index ? vertex : {
      ...vertex, [property]: vertex[property].map((v, position) => position === axis ? value / 100 : v),
      ...(property === 'position' ? {} : { tangentMode: 'broken' as const }),
    })
    updateItem(item.id, { pathVertices: next as MaskVertex[] })
  }
  return <SectionHeader title="Điểm đường vẽ" defaultOpen={false}>
    <p className="text-xs text-muted-foreground">Vị trí tính theo khung hình dạng; tay nắm là độ lệch từ điểm. Thêm điểm giữ nguyên đường cong.</p>
    {vertices.map((_, index) => <SectionHeader key={index} title={`Điểm ${index + 1}`} defaultOpen={false}>
      {coordinates.flatMap(([property, label]) => ([0, 1] as const).map(axis => <PropertyRow key={`${property}-${axis}`} label={`${label} ${axis === 0 ? 'X' : 'Y'}`}>
        <NumberField label={`Điểm ${index + 1}: ${label.toLocaleLowerCase('vi')} ${axis === 0 ? 'X' : 'Y'}`} value={vertices[index]![property][axis] * 100} unit="%" step={0.1} onChange={value => coordinate(index, property, axis, value)} />
      </PropertyRow>))}
      <EditorIconButton icon={Trash2} label={`Xoá điểm ${index + 1}`} disabled={vertices.length <= 2} onClick={() => {
        const latest = current()
        if (latest?.pathVertices && latest.pathVertices.length > 2) withCommandMergeKey(null, () => updateItem(item.id, { pathVertices: latest.pathVertices!.filter((_, at) => at !== index) }))
      }} />
    </SectionHeader>)}
    <EditorIconButton icon={Plus} label="Thêm điểm đường vẽ" disabled={vertices.length < 2} onClick={() => {
      const latest = current()
      if (latest?.pathVertices) withCommandMergeKey(null, () => updateItem(item.id, { pathVertices: subdividePath(latest.pathVertices!, latest.pathClosed !== false) }))
    }} />
  </SectionHeader>
}
