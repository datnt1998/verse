import type { TimelineItem } from '@editor/types/timeline';
import { applyTrimEndPreview, applyTrimStartPreview, type PreviewItemUpdate } from '@editor/features/timeline/utils/item-edit-preview';

export type TimelinePreviewEdit =
  | { kind: 'move'; updates: ReadonlyArray<{ id: string; from: number; trackId: string }> }
  | { kind: 'trim'; itemId: string; side: 'start'|'end'; delta: number; mode: 'normal'|'ripple'|'rolling'; linkedSelection: boolean };

/** Temporary render values only: canonical edits remain command-owned. */
export function previewTimelineItems(items: readonly TimelineItem[], edit: TimelinePreviewEdit | null, fps: number): readonly TimelineItem[] {
  if (!edit) return items;
  if (edit.kind === 'move') {
    const updates = new Map(edit.updates.map((update) => [update.id, update]));
    return items.map((item) => { const update = updates.get(item.id); return update ? { ...item, from: update.from, trackId: update.trackId } : item; });
  }
  const anchor = items.find((item) => item.id === edit.itemId);
  if (!anchor) return items;
  const targets = items.filter((item) => item.id === anchor.id || (edit.linkedSelection && anchor.linkedGroupId && item.linkedGroupId === anchor.linkedGroupId));
  const updates = new Map<string, PreviewItemUpdate>();
  for (const target of targets) {
    const update = edit.side === 'start' ? applyTrimStartPreview(target, edit.delta, fps) : applyTrimEndPreview(target, edit.delta, fps);
    if (edit.mode === 'ripple' && edit.side === 'start') update.from = target.from;
    updates.set(target.id, update);
    if (edit.mode === 'ripple') {
      const shift = edit.side === 'start' ? -edit.delta : edit.delta;
      for (const item of items) {
        if (item.trackId === target.trackId && item.id !== target.id && item.from >= target.from + target.durationInFrames) updates.set(item.id, { id: item.id, from: item.from + shift });
      }
    } else if (edit.mode === 'rolling') {
      const neighbor = items.find((item) => item.trackId === target.trackId && item.id !== target.id && (edit.side === 'start' ? item.from + item.durationInFrames === target.from : item.from === target.from + target.durationInFrames));
      if (neighbor) updates.set(neighbor.id, edit.side === 'start' ? applyTrimEndPreview(neighbor, edit.delta, fps) : applyTrimStartPreview(neighbor, edit.delta, fps));
    }
  }
  return items.map((item) => { const update = updates.get(item.id); return update ? { ...item, ...update } : item; });
}
