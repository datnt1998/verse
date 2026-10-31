export interface FrameItem { id: string; from: number; durationInFrames: number; trackId: string }
export interface ItemWindow<T extends FrameItem> { items: T[]; maxEnds: number[] }

export function clampFrame(frame: number, maximum = Number.MAX_SAFE_INTEGER): number {
  return Math.max(0, Math.min(maximum, Math.round(frame)));
}
export function timecode(frame: number, fps: number): string {
  const safe = clampFrame(frame);
  const seconds = Math.floor(safe / fps);
  return `${String(Math.floor(seconds / 3600)).padStart(2, '0')}:${String(Math.floor(seconds / 60) % 60).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}:${String(Math.floor(safe % fps)).padStart(2, '0')}`;
}
export function snapFrame(frame: number, targets: readonly number[], pixelsPerFrame: number, thresholdPixels = 8): { frame: number; guide: number | null } {
  let result = clampFrame(frame);
  let distance = thresholdPixels / Math.max(0.0001, pixelsPerFrame);
  let guide: number | null = null;
  for (const target of targets) {
    const nextDistance = Math.abs(target - frame);
    if (nextDistance <= distance && target >= 0) { result = target; guide = target; distance = nextDistance; }
  }
  return { frame: clampFrame(result), guide };
}
export function snapGroupDelta(items: readonly FrameItem[], delta: number, targets: readonly number[], pixelsPerFrame: number): { delta: number; guide: number | null } {
  if (!items.length) return { delta: 0, guide: null };
  const minimum = Math.min(...items.map((item) => item.from));
  let result = Math.max(-minimum, Math.round(delta));
  const baseDelta = result;
  let distance = Infinity;
  let guide: number | null = null;
  for (const item of items) {
    for (const edge of [item.from, item.from + item.durationInFrames]) {
      const snapped = snapFrame(edge + baseDelta, targets, pixelsPerFrame);
      const candidate = snapped.frame - edge;
      const difference = Math.abs(candidate - delta);
      if (snapped.guide !== null && candidate >= -minimum && difference < distance) {
        result = candidate; guide = snapped.guide; distance = difference;
      }
    }
  }
  return { delta: result, guide };
}
export function indexItems<T extends FrameItem>(items: readonly T[]): Map<string, ItemWindow<T>> {
  const tracks = new Map<string, ItemWindow<T>>();
  for (const item of items) {
    let index = tracks.get(item.trackId);
    if (!index) { index = { items: [], maxEnds: [] }; tracks.set(item.trackId, index); }
    index.items.push(item);
  }
  for (const index of tracks.values()) {
    index.items.sort((a, b) => a.from - b.from || a.id.localeCompare(b.id));
    let end = 0;
    index.maxEnds = index.items.map((item) => { end = Math.max(end, item.from + item.durationInFrames); return end; });
  }
  return tracks;
}
export function visibleItems<T extends FrameItem>(index: ItemWindow<T> | undefined, start: number, end: number, pinnedId?: string | null): T[] {
  if (!index) return [];
  let low = 0; let high = index.items.length;
  while (low < high) { const middle = (low + high) >>> 1; if (index.maxEnds[middle] < start) low = middle + 1; else high = middle; }
  const result: T[] = [];
  for (let i = low; i < index.items.length && index.items[i].from <= end; i++) {
    const item = index.items[i];
    if (item.from + item.durationInFrames >= start) result.push(item);
  }
  if (pinnedId && !result.some((item) => item.id === pinnedId)) { const pinned = index.items.find((item) => item.id === pinnedId); if (pinned) result.push(pinned); }
  return result;
}
export function rulerStep(fps: number, pixelsPerFrame: number): number {
  const steps = [1, 5, 10, fps, fps * 2, fps * 5, fps * 10, fps * 30, fps * 60, fps * 120, fps * 300];
  return steps.find((step) => step * pixelsPerFrame >= 80) ?? fps * 600;
}
export function moveGroup<T extends FrameItem>(items: readonly T[], delta: number): Array<{id: string; from: number; trackId: string}> {
  const minimum = items.reduce((value, item) => Math.min(value, item.from), Infinity);
  const safeDelta = Math.max(-minimum, Math.round(delta));
  return items.map((item) => ({ id: item.id, from: item.from + safeDelta, trackId: item.trackId }));
}
export function marqueeItems<T extends FrameItem>(items: readonly T[], start: number, end: number, trackIds: ReadonlySet<string>): string[] {
  const left = Math.min(start, end); const right = Math.max(start, end);
  return items.filter((item) => trackIds.has(item.trackId) && item.from <= right && item.from + item.durationInFrames >= left).map((item) => item.id);
}

export function fitZoomLevel(availableWidth: number, durationInFrames: number, fps: number, maximum = 2): number {
  const seconds = Math.max(10, durationInFrames / fps);
  return Math.min(maximum, Math.max(0.000001, availableWidth / (seconds * 100)));
}
export function isDenseWindow(items: readonly FrameItem[], pixelsPerFrame: number, minimumPixels = 24): boolean {
  let narrow = 0;
  for (const item of items) if (item.durationInFrames * pixelsPerFrame < minimumPixels) narrow++;
  return narrow > 0;
}

export function rulerKeyFrame(key: string, frame: number, fps: number, end: number, shift = false): number | null {
  const maximum = Math.max(0, end - 1);
  if (key === 'Home') return 0;
  if (key === 'End') return maximum;
  const delta = key === 'ArrowLeft' ? -(shift ? fps : 1) : key === 'ArrowRight' ? shift ? fps : 1 : key === 'PageUp' ? -fps : key === 'PageDown' ? fps : null;
  return delta === null ? null : clampFrame(frame + delta, maximum);
}

export function resizedTrackHeight(current: number, delta: number, minimum = 72): number {
  return Math.max(minimum, Math.min(160, Math.max(minimum, current) + delta));
}

export function reorderTrackOrder<T extends { id: string; order: number }>(tracks: readonly T[], id: string, direction: number): T[] | null {
  const ordered = [...tracks].sort((left, right) => left.order - right.order);
  const position = ordered.findIndex((track) => track.id === id);
  if (position < 0) return null;
  const next = Math.max(0, Math.min(ordered.length - 1, position + direction));
  if (next === position) return null;
  const [track] = ordered.splice(position, 1);
  ordered.splice(next, 0, track);
  return ordered.map((entry, order) => ({ ...entry, order }));
}
