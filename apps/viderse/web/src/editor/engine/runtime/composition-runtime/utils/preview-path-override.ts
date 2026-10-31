import type { MaskVertex } from '@editor/types/masks'
import type { ShapeItem, TimelineItem } from '@editor/types/timeline'

export type PreviewPathVerticesOverride = (itemId: string) => MaskVertex[] | undefined

export function applyPreviewPathVerticesToShape<TShape extends ShapeItem>(
  shape: TShape,
  getPreviewPathVertices?: PreviewPathVerticesOverride,
): TShape {
  if (shape.shapeType !== 'path') {
    return shape
  }

  const previewVertices = getPreviewPathVertices?.(shape.id)
  if (!previewVertices || previewVertices === shape.pathVertices) {
    return shape
  }

  return {
    ...shape,
    pathVertices: previewVertices,
  }
}

export function applyPreviewPathVerticesToItem<TItem extends TimelineItem>(
  item: TItem,
  getPreviewPathVertices?: PreviewPathVerticesOverride,
): TItem {
  if (item.type !== 'shape') {
    return item
  }

  return applyPreviewPathVerticesToShape(item, getPreviewPathVertices) as TItem
}
