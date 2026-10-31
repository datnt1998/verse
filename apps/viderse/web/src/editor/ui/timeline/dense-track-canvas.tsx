"use client";
import { useEffect, useRef } from 'react';
import type { TimelineItem } from '@editor/types/timeline';

import { TYPE_COLOR } from './item-types';

/** Sub-pixel clips share a raster surface; roving focus still mounts the actual item button. */
export function DenseTrackCanvas({ items, selected, offset, width, height, scale }: { items: readonly TimelineItem[]; selected: ReadonlySet<string>; offset: number; width: number; height: number; scale: number }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current; if (!canvas) return;
    const ratio = window.devicePixelRatio || 1;
    canvas.width = Math.ceil(width * ratio); canvas.height = Math.ceil(height * ratio);
    const context = canvas.getContext('2d'); if (!context) return;
    context.scale(ratio, ratio);
    const computed = getComputedStyle(canvas);
    const colors: Record<string, string> = {};
    for (const key of ['video', 'audio', 'text', 'image', 'effect']) colors[key] = computed.getPropertyValue(`--ed-track-${key}`).trim();
    for (const item of items) {
      const left = item.from * scale - offset;
      const right = (item.from + item.durationInFrames) * scale - offset;
      if (right < 0 || left > width) continue;
      const x = Math.max(0, left); const clipWidth = Math.max(1, Math.min(width, right) - x);
      context.globalAlpha = 0.35;
      context.fillStyle = colors[TYPE_COLOR[item.type]];
      context.fillRect(x, 0, clipWidth, height);
      context.globalAlpha = 1;
      context.strokeStyle = selected.has(item.id) ? computed.getPropertyValue('--primary').trim() : colors[TYPE_COLOR[item.type]];
      context.lineWidth = selected.has(item.id) ? 2 : 1;
      context.strokeRect(x + 0.5, 0.5, Math.max(0, clipWidth - 1), height - 1);
    }
  }, [items, selected, offset, width, height, scale]);
  return <canvas ref={ref} aria-hidden className="pointer-events-none absolute top-1.5" style={{ left: offset, width, height }} />;
}
