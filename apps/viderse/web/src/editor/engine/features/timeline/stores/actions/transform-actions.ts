/**
 * Transform Actions - Item transform operations with undo/redo support.
 *
 * Mutation bodies live in commands/handlers/item.ts (item.updateTransform,
 * item.commitMaskEdit, item.resetTransform, item.updateTransforms,
 * item.updateTransformsMap, item.applyBentoLayout); these entry points keep their
 * public signatures and translate arguments into JSON-only command params.
 */

import '@editor/commands'
import type { TransformProperties } from '@editor/types/transform'
import type { MaskVertex } from '@editor/types/masks'
import type { LayoutConfig } from '../../utils/bento-layout'
import type {
  BatchTransformCommandOptions,
  TransformCommandOptions,
  TransformHistoryOperation,
} from '../../types'
import type { AutoKeyframeOperation } from '@editor/features/timeline/deps/keyframes'
import { runItemCommand, splitItemUpdates } from '@editor/commands/handlers/item'
import type { ItemCommandResult } from '@editor/commands/handlers/item'

function inferTransformOperation(keys: ReadonlySet<string>): TransformHistoryOperation {
  const hasPosition = keys.has('x') || keys.has('y')
  const hasSize = keys.has('width') || keys.has('height')
  const hasRotation = keys.has('rotation')
  const hasOpacity = keys.has('opacity')
  const hasCornerRadius = keys.has('cornerRadius')
  const hasOther = [...keys].some(
    (key) =>
      key !== 'x' &&
      key !== 'y' &&
      key !== 'width' &&
      key !== 'height' &&
      key !== 'rotation' &&
      key !== 'opacity' &&
      key !== 'cornerRadius',
  )

  if (hasPosition && !hasSize && !hasRotation && !hasOpacity && !hasCornerRadius && !hasOther) {
    return 'move'
  }
  if (hasSize && !hasPosition && !hasRotation && !hasOpacity && !hasCornerRadius && !hasOther) {
    return 'resize'
  }
  if (hasRotation && !hasPosition && !hasSize && !hasOpacity && !hasCornerRadius && !hasOther) {
    return 'rotate'
  }
  if (hasOpacity && !hasPosition && !hasSize && !hasRotation && !hasCornerRadius && !hasOther) {
    return 'opacity'
  }
  if (hasCornerRadius && !hasPosition && !hasSize && !hasRotation && !hasOpacity && !hasOther) {
    return 'corner_radius'
  }
  return 'transform'
}

interface MaskEditCommit {
  pathVertices?: MaskVertex[]
  transform?: Partial<TransformProperties>
  autoKeyframeOperations?: AutoKeyframeOperation[]
}

export function updateItemTransform(
  id: string,
  transform: Partial<TransformProperties>,
  options?: TransformCommandOptions,
): ItemCommandResult<void> {
  const transformKeys = new Set(Object.keys(transform))
  const autoKeyframeOperations = options?.autoKeyframeOperations ?? []
  if (transformKeys.size === 0 && autoKeyframeOperations.length === 0) return { ok: true, result: undefined }
  return runItemCommand<void>('item.updateTransform', {
    id,
    transform,
    operation: options?.operation ?? inferTransformOperation(transformKeys),
    autoKeyframeOperations,
  })
}

export function commitMaskEdit(
  id: string,
  commit: MaskEditCommit,
  options?: TransformCommandOptions,
): void {
  const transform = commit.transform ?? {}
  const transformKeys = new Set(Object.keys(transform))
  const autoKeyframeOperations = commit.autoKeyframeOperations ?? []

  if (!commit.pathVertices && transformKeys.size === 0 && autoKeyframeOperations.length === 0) {
    return
  }

  runItemCommand('item.commitMaskEdit', {
    id,
    pathVertices: commit.pathVertices,
    transform,
    operation:
      options?.operation ??
      (commit.pathVertices ? 'transform' : inferTransformOperation(transformKeys)),
    autoKeyframeOperations,
  })
}

export function resetItemTransform(id: string): void {
  runItemCommand('item.resetTransform', { id })
}

export function updateItemsTransform(
  ids: string[],
  transform: Partial<TransformProperties>,
  options?: TransformCommandOptions,
): void {
  if (ids.length === 0) return
  runItemCommand('item.updateTransforms', {
    ids,
    transform,
    operation: options?.operation ?? inferTransformOperation(new Set(Object.keys(transform))),
  })
}

export function updateItemsTransformMap(
  transformsMap: Map<string, Partial<TransformProperties>>,
  options?: BatchTransformCommandOptions,
): void {
  const itemUpdates = options?.itemUpdates
  const autoKeyframeOperations = options?.autoKeyframeOperations ?? []
  if (transformsMap.size === 0 && autoKeyframeOperations.length === 0 && (itemUpdates?.size ?? 0) === 0) {
    return
  }
  const unionKeys = new Set<string>()
  for (const transform of transformsMap.values()) {
    for (const key of Object.keys(transform)) unionKeys.add(key)
  }
  runItemCommand('item.updateTransformsMap', {
    entries: [...transformsMap].map(([id, transform]) => ({ id, transform })),
    itemUpdates: itemUpdates
      ? [...itemUpdates].map(([id, updates]) => ({ id, ...splitItemUpdates(updates) }))
      : undefined,
    operation: options?.operation ?? inferTransformOperation(unionKeys),
    autoKeyframeOperations,
  })
}

export function applyBentoLayout(
  itemIds: string[],
  canvasWidth: number,
  canvasHeight: number,
  config?: LayoutConfig,
  /** Pre-ordered chains from dialog drag-swap; skips buildTransitionChains when provided */
  orderedChains?: string[][],
): void {
  if (itemIds.length < 2) return
  runItemCommand('item.applyBentoLayout', {
    itemIds,
    canvasWidth,
    canvasHeight,
    config,
    orderedChains,
  })
}
