import type { TimelineItem } from '@editor/types/timeline';
import type { ItemEffect } from '@editor/types/effects';
import type { SubComposition } from '@editor/features/timeline/stores/compositions-store';
import { hasCornerPin } from '@editor/runtime/composition-runtime/utils/corner-pin';

/** The DOM player does not composite GPU effects or non-normal blend modes. */
export function needsCanvasPreview(
  items: readonly TimelineItem[],
  compositions: Readonly<Record<string, SubComposition>>,
  previewEffects: Readonly<Record<string, { effects?: ItemEffect[] }>> | null = null,
  visited = new Set<string>(),
): boolean {
  return items.some(item => {
    if (item.type === 'audio') return false;
    if ((previewEffects?.[item.id]?.effects ?? item.effects)?.some(effect => effect.enabled && effect.effect.type === 'gpu-effect')) return true;
    if (item.blendMode && item.blendMode !== 'normal' && !(item.type === 'shape' && item.isMask)) return true;
    if (hasCornerPin(item.cornerPin)) return true;
    if (item.type !== 'composition' || visited.has(item.compositionId)) return false;
    const composition = compositions[item.compositionId];
    if (!composition) return false;
    visited.add(item.compositionId);
    return needsCanvasPreview(composition.items, compositions, previewEffects, visited);
  });
}
