"use client";

import { memo, useMemo, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type MutableRefObject, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react';
import { Link, Repeat } from 'lucide-react';
import type { ItemKeyframes } from '@editor/types/keyframe';
import type { TimelineItem } from '@editor/types/timeline';
import { ContextMenu, ContextMenuTrigger } from '@/components/ui/context-menu';
import { timecode } from './geometry';
import { ItemVisual } from './item-visual';
import { TYPE_COLOR, TYPE_ICON, TYPE_LABEL } from './item-types';
import { timelineStyles as styles } from './styles';

type ClipGestureKind = 'move' | 'start' | 'end' | 'fade-in' | 'fade-out';

/**
 * Clip callbacks are live implementations from TimelinePanel. The panel publishes them to a ref
 * in a layout effect and the clip reads `api.current` only inside event handlers, so the api
 * reference stays stable while handlers always see fresh store and gesture state.
 */
export interface TimelineClipApi {
  begin(event: ReactPointerEvent<HTMLElement>, kind: ClipGestureKind, item: TimelineItem): void;
  click(item: TimelineItem, event: { metaKey: boolean; shiftKey: boolean; detail: number }): void;
  selectOnContextMenu(item: TimelineItem): void;
  focus(item: TimelineItem): void;
  doubleClick(item: TimelineItem): void;
  itemKeyDown(item: TimelineItem, event: ReactKeyboardEvent<HTMLButtonElement>): void;
  trimKey(item: TimelineItem, side: 'start' | 'end', event: ReactKeyboardEvent<HTMLButtonElement>): void;
  fadeKey(item: TimelineItem, side: 'fade-in' | 'fade-out', event: ReactKeyboardEvent<HTMLButtonElement>): void;
  menuOpenChange(itemId: string, open: boolean): void;
  registerRef(itemId: string, element: HTMLButtonElement | null): void;
}

export interface TimelineClipProps {
  item: TimelineItem;
  /** Keyframes of the clip; the reference is stable until its keyframes-store entry changes. */
  keys?: ItemKeyframes;
  /** Context menu content; the panel builds it only for the clip whose menu is open, otherwise null. */
  menu: ReactNode;
  api: MutableRefObject<TimelineClipApi | null>;
  selected: boolean;
  focused: boolean;
  /** Single tab stop of the lane: the focused clip, or the first item of the timeline. */
  tabbable: boolean;
  trackLocked: boolean;
  denseMinWidth?: number;
  scale: number;
  pixelsPerSecond: number;
  fps: number;
  viewportLeft: number;
  viewportWidth: number;
  fadeIn: number;
  fadeOut: number;
}

/** Fields the clip draws or reads in handlers; colour, text and effects do not affect the timeline. */
const DRAWN_ITEM_FIELDS = [
  'id', 'trackId', 'type', 'label', 'from', 'durationInFrames', 'linkedGroupId', 'compositionId',
  'mediaId', 'src', 'speed', 'sourceStart', 'sourceFps', 'sourceDuration', 'sourceWidth', 'sourceHeight', 'trimStart',
  'fadeIn', 'fadeOut', 'audioFadeIn', 'audioFadeOut', 'loop',
] as const;

function sameDrawnItem(a: TimelineItem, b: TimelineItem): boolean {
  // Union members carry type-specific fields (src, sourceWidth…), so index access needs the
  // record view; the field list above is exactly what the clip draws.
  const first = a as unknown as Record<string, unknown>, second = b as unknown as Record<string, unknown>;
  return DRAWN_ITEM_FIELDS.every((field) => first[field] === second[field]);
}

export const TimelineClip = memo(function TimelineClip({
  item, keys, menu, api, selected, focused, tabbable, trackLocked,
  denseMinWidth, scale, pixelsPerSecond, fps, viewportLeft, viewportWidth, fadeIn, fadeOut,
}: TimelineClipProps) {
  const Icon = TYPE_ICON[item.type];
  const clipWidth = Math.max(4, item.durationInFrames * scale);
  const offset = Math.max(0, viewportLeft - item.from * scale);
  const visibleWidth = Math.min(clipWidth - offset, viewportWidth + 100);
  const keyFrames = useMemo(
    () => [...(keys?.properties.flatMap((property) => property.keyframes.map((key) => key.frame)) ?? []), ...(keys?.vectorProperties?.flatMap((property) => property.keyframes.map((key) => key.frame)) ?? [])],
    [keys],
  );
  return <ContextMenu onOpenChange={(open) => api.current?.menuOpenChange(item.id, open)}>
    <ContextMenuTrigger asChild>
      <div className={styles.clip} data-selected={selected} style={{ '--track': `var(--ed-track-${TYPE_COLOR[item.type]})`, left: item.from * scale, width: clipWidth, minWidth: denseMinWidth, zIndex: focused ? 8 : undefined } as CSSProperties} onContextMenu={() => api.current?.selectOnContextMenu(item)}>
        <button ref={(element) => api.current?.registerRef(item.id, element)} data-item-id={item.id} data-item-type={item.type} className={styles.clipButton}
          tabIndex={tabbable ? 0 : -1}
          aria-pressed={selected} aria-disabled={trackLocked}
          aria-label={`${TYPE_LABEL[item.type]} ${item.label}, ${timecode(item.from, fps)} đến ${timecode(item.from + item.durationInFrames, fps)}, ${(item.durationInFrames / fps).toFixed(1)} giây${item.type === 'video' && item.loop === true ? ', lặp' : ''}`}
          onFocus={() => api.current?.focus(item)}
          onPointerDown={(event) => api.current?.begin(event, 'move', item)}
          onClick={(event) => api.current?.click(item, event)}
          onDoubleClick={() => api.current?.doubleClick(item)}
          onKeyDown={(event) => api.current?.itemKeyDown(item, event)}>
          <ItemVisual item={item} fps={fps} width={Math.max(0, visibleWidth)} offset={offset} pixelsPerFrame={scale} />
          <span className={styles.label} style={{ left: Math.max(8, offset + 8), maxWidth: Math.max(0, visibleWidth - 16) }}><Icon className="size-3 shrink-0" aria-hidden />{item.linkedGroupId && <Link className="size-3 shrink-0" aria-hidden />}{item.type === 'video' && item.loop === true && <Repeat className="size-3 shrink-0" aria-hidden />}<span className="truncate">{item.label}</span></span>
          {fadeIn > 0 && <span aria-hidden className="pointer-events-none absolute inset-y-0 left-0 bg-background/40" style={{ width: Math.min(clipWidth, fadeIn * pixelsPerSecond), clipPath: 'polygon(0 0,100% 0,0 100%)' }} />}
          {fadeOut > 0 && <span aria-hidden className="pointer-events-none absolute inset-y-0 right-0 bg-background/40" style={{ width: Math.min(clipWidth, fadeOut * pixelsPerSecond), clipPath: 'polygon(0 0,100% 0,100% 100%)' }} />}
          {[...new Set(keyFrames)].filter((frame) => frame >= 0 && frame <= item.durationInFrames && frame * scale >= offset && frame * scale <= offset + visibleWidth).map((frame) => <span key={frame} className={styles.keyframe} style={{ left: frame * scale }} />)}
        </button>
        {clipWidth >= 56 && (['start', 'end'] as const).map((side) => <button key={side} data-hit-area data-tooltip={`Trim ${side === 'start' ? 'đầu' : 'đuôi'}; ←/→ đổi một khung`} data-trim={side} aria-label={`Trim ${side === 'start' ? 'đầu' : 'đuôi'} ${item.label}`} disabled={trackLocked} className={`${styles.handle} ${side === 'start' ? styles.start : styles.end}`} onPointerDown={(event) => api.current?.begin(event, side, item)} onKeyDown={(event) => api.current?.trimKey(item, side, event)} />)}
        {clipWidth >= 112 && (['fade-in', 'fade-out'] as const).map((side) => { const label = side === 'fade-in' ? 'Fade vào' : 'Fade ra'; return <button key={side} data-hit-area data-tooltip={label} aria-label={`${label} ${item.label}`} disabled={trackLocked} className={styles.fade} style={{ left: side === 'fade-in' ? Math.min(clipWidth / 2 - 28, fadeIn * pixelsPerSecond) : Math.max(clipWidth / 2, clipWidth - fadeOut * pixelsPerSecond - 28) }} onPointerDown={(event) => api.current?.begin(event, side, item)} onKeyDown={(event) => api.current?.fadeKey(item, side, event)} />; })}
      </div>
    </ContextMenuTrigger>
    {menu}
  </ContextMenu>;
}, (prev, next) =>
  prev.selected === next.selected && prev.focused === next.focused && prev.tabbable === next.tabbable &&
  prev.trackLocked === next.trackLocked && prev.denseMinWidth === next.denseMinWidth &&
  prev.scale === next.scale && prev.pixelsPerSecond === next.pixelsPerSecond && prev.fps === next.fps &&
  prev.viewportLeft === next.viewportLeft && prev.viewportWidth === next.viewportWidth &&
  prev.fadeIn === next.fadeIn && prev.fadeOut === next.fadeOut &&
  prev.keys === next.keys && prev.menu === next.menu && prev.api === next.api &&
  sameDrawnItem(prev.item, next.item),
);
