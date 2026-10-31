import type { TimelineItem } from '@editor/types/timeline';
import { expandSelectionWithLinkedItems } from '@editor/features/timeline/utils/linked-items';

interface ItemSelectionInput {
  items: TimelineItem[];
  currentIds: readonly string[];
  itemId: string;
  anchorId: string | null;
  append: boolean;
  range: boolean;
  linkedSelection: boolean;
}

/** Compute once per press so pointer-up cannot toggle a modifier selection twice. */
export function computeItemSelection({ items, currentIds, itemId, anchorId, append, range, linkedSelection }: ItemSelectionInput): string[] {
  const item = items.find((entry) => entry.id === itemId);
  if (!item) return [];
  let ids = [item.id];
  const anchor = items.find((entry) => entry.id === anchorId);
  if (range && anchor?.trackId === item.trackId) {
    const left = Math.min(anchor.from, item.from); const right = Math.max(anchor.from, item.from);
    ids = items.filter((entry) => entry.trackId === item.trackId && entry.from >= left && entry.from <= right).sort((a, b) => a.from - b.from || a.id.localeCompare(b.id)).map((entry) => entry.id);
  } else if (append) {
    const toggled = new Set(linkedSelection ? expandSelectionWithLinkedItems(items, [item.id]) : [item.id]);
    ids = currentIds.includes(item.id) ? currentIds.filter((id) => !toggled.has(id)) : [...currentIds, ...toggled];
  }
  if (linkedSelection) ids = expandSelectionWithLinkedItems(items, ids);
  // The inspector uses the first selected ID, so the clicked item is the primary.
  return ids.includes(item.id) ? [item.id, ...ids.filter((id) => id !== item.id)] : ids;
}

export function hasPointerDrag(startX: number, startY: number, endX: number, endY: number): boolean {
  return Math.abs(endX - startX) > 3 || Math.abs(endY - startY) > 3;
}
