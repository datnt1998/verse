"use client";

import { useEffect, useMemo, useRef, useState } from 'react';
import * as stylex from '@stylexjs/stylex';
import type { CompositionInputProps } from '@editor/types/export';
import type { TimelineItem } from '@editor/types/timeline';
import { createCompositionRenderer } from '@editor/features/export/utils/client-render-engine';
import type { CompositionPreviewRenderer } from '@editor/features/export/utils/client-render-engine';
import { useCompositionsStore } from '@editor/features/timeline/stores/compositions-store';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store';
import { useGizmoStore } from '@editor/features/preview/stores/gizmo-store';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { usePreviewBridgeStore } from '@editor/shared/state/preview-bridge';
import { getBestDomVideoElementForItem } from '@editor/runtime/composition-runtime/utils/dom-video-element-registry';
import { announce } from '@/editor/ui/shell/announcer';
import { needsCanvasPreview } from './canvas-preview-routing';
import { canvasPreviewStyles as styles } from './canvas-preview.stylex';

/**
 * Fingerprint of everything `createCompositionRenderer` snapshots at creation:
 * track/item membership, ordering, visibility, resolved sources, transitions,
 * and the composition constants. Per-item live edits (colour, transform, text)
 * are already read through `getLiveItemSnapshot`, so they leave the signature —
 * and the renderer — untouched instead of rebuilding the engine per edit.
 */
function rendererStructureSignature(composition: CompositionInputProps): string {
  return JSON.stringify([
    composition.fps,
    composition.width,
    composition.height,
    composition.backgroundColor,
    (composition.tracks ?? []).map(track => [
      track.id,
      track.visible !== false,
      (track.items ?? []).map((item: TimelineItem) => [item.id, item.type, 'src' in item ? item.src : undefined, 'audioSrc' in item ? item.audioSrc : undefined]),
    ]),
    composition.transitions ?? [],
  ]);
}

/** Freecut's composited overlay; the existing player remains the clock/audio source. */
export function CanvasPreview({ composition, quality }: { composition: CompositionInputProps; quality: number }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const compositions = useCompositionsStore(s => s.compositionById);
  const previews = useGizmoStore(s => s.preview);
  const [error, setError] = useState('');
  const enabled = useMemo(() => needsCanvasPreview(
    composition.tracks.filter(track => track.visible !== false).flatMap(track => track.items ?? []),
    compositions,
    previews,
  ), [composition.tracks, compositions, previews]);
  // The composition prop gets a new identity on every store edit. The renderer
  // must live across live-value edits, so keep the last instance whose
  // creation-time inputs (see rendererStructureSignature) still match.
  const [structural, setStructural] = useState(() => ({ signature: rendererStructureSignature(composition), composition }));
  if (structural.signature !== rendererStructureSignature(composition)) {
    setStructural({ signature: rendererStructureSignature(composition), composition });
  }
  const renderComposition = structural.composition;

  useEffect(() => {
    const display = canvas.current;
    if (!enabled || !display) return;
    const controller = new AbortController();
    let renderer: CompositionPreviewRenderer | null = null;
    let drawing = false;
    let pending = false;
    let scheduled = 0;
    const width = Math.max(1, Math.round((renderComposition.width ?? 1920) * quality));
    const height = Math.max(1, Math.round((renderComposition.height ?? 1080) * quality));
    const offscreen = new OffscreenCanvas(width, height);
    const context = offscreen.getContext('2d');
    const output = display.getContext('2d');
    const resolvedItems = new Map(renderComposition.tracks.flatMap(track => (track.items ?? []).map(item => [item.id, item] as const)));
    display.width = width;
    display.height = height;
    display.style.visibility = 'hidden';

    const reportError = (reason: unknown) => {
      if (controller.signal.aborted) return;
      const message = `Không dựng được xem trước: ${reason instanceof Error ? reason.message : 'Thử lại'}`;
      display.style.visibility = 'hidden';
      setError(message);
      announce(message, 'assertive');
    };
    const request = () => {
      if (controller.signal.aborted) return;
      pending = true;
      if (renderer && !drawing && !scheduled) scheduled = requestAnimationFrame(() => { scheduled = 0; void draw(); });
    };
    const draw = async () => {
      const active = renderer;
      if (!active || !context || !output || controller.signal.aborted) return;
      drawing = true;
      pending = false;
      const playback = usePlaybackStore.getState();
      const frame = playback.previewFrame ?? playback.currentFrame;
      try {
        await active.renderFrame(frame);
        if (!controller.signal.aborted && !active.wasLastRenderAborted()) {
          output.clearRect(0, 0, width, height);
          output.drawImage(offscreen, 0, 0);
          display.style.visibility = 'visible';
          setError('');
        }
      } catch (reason) { reportError(reason); }
      finally {
        drawing = false;
        if (pending && !controller.signal.aborted) request();
      }
    };
    const invalidate = () => { renderer?.invalidateFrameCache(); request(); };
    const unsubscribePlayback = usePlaybackStore.subscribe((state, previous) => {
      if (state.currentFrame !== previous.currentFrame || state.previewFrame !== previous.previewFrame) request();
    });
    const unsubscribeGizmo = useGizmoStore.subscribe(invalidate);
    const unsubscribeItems = useItemsStore.subscribe(invalidate);
    const unsubscribeKeyframes = useKeyframesStore.subscribe(invalidate);

    const initialize = async () => {
      try {
        if (!context || !output) throw new Error('Trình duyệt không hỗ trợ canvas');
        if (controller.signal.aborted) return;
        const active = await createCompositionRenderer(renderComposition, offscreen, context, {
          mode: 'preview',
          useProxyMedia: false,
          getPreviewTransformOverride: id => useGizmoStore.getState().preview?.[id]?.transform,
          getPreviewEffectsOverride: id => useGizmoStore.getState().preview?.[id]?.effects,
          getLiveKeyframes: id => useKeyframesStore.getState().keyframes.find(keyframes => keyframes.itemId === id),
          getLiveItemSnapshot: id => {
            const live = useItemsStore.getState().itemById[id];
            const resolved = resolvedItems.get(id);
            if (!live || !resolved) return live;
            if ((live.type === 'image' || live.type === 'video' || live.type === 'audio') && resolved.type === live.type) return { ...live, src: resolved.src };
            return live;
          },
          domVideoElementProvider: getBestDomVideoElementForItem,
        });
        if (controller.signal.aborted) { active.dispose(); return; }
        renderer = active;
        const playback = usePlaybackStore.getState();
        await active.preload({ priorityFrame: playback.previewFrame ?? playback.currentFrame, signal: controller.signal });
        if (controller.signal.aborted) return;
        await active.warmGpuPipeline();
        request();
      } catch (reason) { reportError(reason); }
    };
    void initialize();
    // Autosave thumbnails capture this canvas instead of re-rendering a frame
    // on the main thread. Hidden (never presented) previews report null so the
    // save falls back to a real render rather than storing a blank cover.
    const capture = async () => (display.style.visibility === 'visible' ? display : null);
    usePreviewBridgeStore.getState().setCaptureCanvasSource(capture);
    return () => {
      controller.abort();
      cancelAnimationFrame(scheduled);
      unsubscribePlayback(); unsubscribeGizmo(); unsubscribeItems(); unsubscribeKeyframes();
      const bridge = usePreviewBridgeStore.getState();
      if (bridge.captureCanvasSource === capture) bridge.setCaptureCanvasSource(null);
      renderer?.dispose();
      display.style.visibility = 'hidden';
    };
  }, [renderComposition, enabled, quality]);

  if (!enabled) return null;
  return <>
    <canvas ref={canvas} aria-hidden {...stylex.props(styles.canvas)} />
    {error && <div role="alert" {...stylex.props(styles.error)}>{error}</div>}
  </>;
}
