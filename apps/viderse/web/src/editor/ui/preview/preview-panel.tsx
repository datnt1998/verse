"use client";

import { useEffect, useRef, useState, type PointerEvent } from 'react';
import { ChevronLeft, ChevronRight, SkipBack, SkipForward, Play, Pause, Repeat, Volume2, VolumeX, Scan, Move, Maximize2, RotateCw, Film } from 'lucide-react';
import { HeadlessPlayer as Player, type PlayerRef } from '@editor/runtime/player/Player';
import { MainComposition } from '@editor/runtime/composition-runtime/compositions/main-composition';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store';
import { useProjectStore } from '@editor/features/projects/stores/project-store';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { useSelectionStore } from '@editor/shared/state/selection';
import { useGizmoStore } from '@editor/features/preview/stores/gizmo-store';
import { resolveItemTransformAtFrame } from '@editor/runtime/composition-runtime/utils/frame-scene';
import { updateItemTransform } from '@editor/features/timeline/stores/timeline-actions';
import type { ResolvedTransform } from '@editor/types/transform';
import { EditorIconButton, EditorSelect } from '@/editor/ui/controls';
import { useShortcutAction, type ShortcutScope } from '@/editor/ui/shortcuts';
import { announce } from '@/editor/ui/shell/announcer';
import { toast } from '@/editor/ui/toast';

import { frameTimecode } from './timecode';
import { withCommandMergeKey } from '@editor/commands';
import { gizmoScaleFactor } from './gizmo-geometry';
import { Button } from '@/components/ui/button';
import { Slider } from '@/components/ui/slider';
import { requestMediaImport } from '@/editor/ui/library/library-actions';
import { CanvasPreview } from './canvas-preview';
import { usePlayerStoreSync, usePreviewComposition } from './use-preview-composition';

function CanvasDescription({ fps }: { fps: number }) {
  const name = useProjectStore(s => s.currentProject?.name);
  const frame = usePlaybackStore(s => s.currentFrame);
  const selected = useSelectionStore(s => s.selectedItemIds);
  const items = useItemsStore(s => s.items);
  return <p id="preview-canvas-description" className="sr-only">{name ?? 'Project'}, {frameTimecode(frame, fps)}. {items.find(item => item.id === selected[0])?.label ?? 'Không chọn clip'}. Dùng Thuộc tính để nhập vị trí, tỉ lệ và góc xoay.</p>;
}

function TransportShortcuts({ scope, duration, fps }: { scope: ShortcutScope; duration: number; fps: number }) {
  const held = useRef(new Set<string>());
  const jump = (delta: number) => { const state = usePlaybackStore.getState(); state.pause(); const frame = Math.max(0, Math.min(duration - 1, state.currentFrame + delta)); state.setCurrentFrame(frame); announce(frameTimecode(frame, fps)); };
  const shuttle = (direction: number) => { const state = usePlaybackStore.getState(); if (held.current.has('KeyK')) { state.setPlaybackRate(direction * .25); state.play(); } else if (direction > 0) state.shuttleForward(); else state.shuttleReverse(); };
  useShortcutAction({ id: 'play', label: 'Phát / tạm dừng', scope, run: e => { if (e.repeat) return; held.current.add(e.code); const state = usePlaybackStore.getState(); if (e.code === 'KeyK') { if (held.current.has('KeyJ') || held.current.has('KeyL')) { state.setPlaybackRate(held.current.has('KeyJ') ? -.25 : .25); state.play(); } else state.pause(); } else state.togglePlayPause(); }, onRelease: e => { held.current.delete(e.code); if (e.code === 'KeyK' && (held.current.has('KeyJ') || held.current.has('KeyL'))) usePlaybackStore.getState().pause(); } });
  useShortcutAction({ id: 'reverse', label: 'Phát ngược', scope, run: e => { if (!e.repeat) { held.current.add(e.code); shuttle(-1); } }, onRelease: e => { held.current.delete(e.code); if (held.current.has('KeyK')) usePlaybackStore.getState().pause(); } });
  useShortcutAction({ id: 'forward', label: 'Phát tới', scope, run: e => { if (!e.repeat) { held.current.add(e.code); shuttle(1); } }, onRelease: e => { held.current.delete(e.code); if (held.current.has('KeyK')) usePlaybackStore.getState().pause(); } });
  useShortcutAction({ id: 'frame-back', label: 'Lùi một khung', scope, run: () => jump(-1) });
  useShortcutAction({ id: 'frame-forward', label: 'Tiến một khung', scope, run: () => jump(1) });
  useShortcutAction({ id: 'second-back', label: 'Lùi một giây', scope, run: () => jump(-fps) });
  useShortcutAction({ id: 'second-forward', label: 'Tiến một giây', scope, run: () => jump(fps) });
  useShortcutAction({ id: 'start', label: 'Về đầu', scope, run: () => jump(-duration) });
  useShortcutAction({ id: 'end', label: 'Về cuối', scope, run: () => jump(duration) });
  return null;
}

function Transport({ duration, fps }: { duration: number; fps: number }) {
  const currentFrame = usePlaybackStore(s => s.currentFrame);
  const isPlaying = usePlaybackStore(s => s.isPlaying);
  const loop = usePlaybackStore(s => s.loop);
  const muted = usePlaybackStore(s => s.muted);
  const volume = usePlaybackStore(s => s.volume);
  const seek = (frame: number) => { const state = usePlaybackStore.getState(); state.pause(); state.setCurrentFrame(Math.max(0, Math.min(duration - 1, frame))); announce(frameTimecode(frame, fps)); };
  return <div className="flex min-h-9 min-w-0 flex-wrap items-center justify-center gap-1 border-t border-border px-2 py-1">
    <EditorIconButton icon={SkipBack} label="Về đầu" shortcut="Home" onClick={() => seek(0)} />
    <EditorIconButton icon={ChevronLeft} label="Lùi một khung" shortcut="←" onClick={() => seek(currentFrame - 1)} />
    <EditorIconButton icon={isPlaying ? Pause : Play} label={isPlaying ? 'Tạm dừng' : 'Phát'} shortcut="Space" onClick={() => usePlaybackStore.getState().togglePlayPause()} />
    <EditorIconButton icon={ChevronRight} label="Tiến một khung" shortcut="→" onClick={() => seek(currentFrame + 1)} />
    <EditorIconButton icon={SkipForward} label="Về cuối" shortcut="End" onClick={() => seek(duration - 1)} />
    <span role="timer" className="px-1 text-[10px] tabular-nums" aria-label={`Thời gian ${frameTimecode(currentFrame, fps)}`}>{frameTimecode(currentFrame, fps)} <span className="text-muted-foreground">/ {frameTimecode(duration, fps)}</span></span>
    <EditorIconButton icon={Repeat} label="Lặp lại" aria-pressed={loop} onClick={() => usePlaybackStore.getState().toggleLoop()} />
    <EditorIconButton icon={muted ? VolumeX : Volume2} label={muted ? 'Bật tiếng màn hình' : 'Tắt tiếng màn hình'} aria-pressed={muted} onClick={() => usePlaybackStore.getState().toggleMute()} />
    <Slider aria-label="Âm lượng màn hình" min={0} max={1} step={0.05} value={[volume]} onValueChange={values => usePlaybackStore.getState().setVolume(values[0])} className="w-14" />
  </div>;
}

function TransformGizmo({ width, height, fps }: { width: number; height: number; fps: number }) {
  const items = useItemsStore(s => s.items);
  const ids = useSelectionStore(s => s.selectedItemIds);
  const frame = usePlaybackStore(s => s.currentFrame);
  const playing = usePlaybackStore(s => s.isPlaying);
  const keyframes = useKeyframesStore(s => s.keyframes);
  const preview = useGizmoStore(s => s.preview);
  const surface = useRef<HTMLDivElement>(null);
  const drag = useRef<{ itemId: string; mode: 'move' | 'scale' | 'rotate'; x: number; y: number; initial: ResolvedTransform; current: ResolvedTransform } | null>(null);
  const item = items.find(i => i.id === ids[0] && i.type !== 'audio' && i.type !== 'adjustment' && frame >= i.from && frame < i.from + i.durationInFrames);
  const transform = item ? resolveItemTransformAtFrame(item, { canvas: { width, height, fps }, frame, keyframes: keyframes.find(k => k.itemId === item.id), previewTransform: preview?.[item.id]?.transform }) : null;
  const start = (event: PointerEvent<HTMLButtonElement>, mode: 'move' | 'scale' | 'rotate') => { if (!item || !transform || playing) return; event.preventDefault(); event.currentTarget.setPointerCapture(event.pointerId); drag.current = { itemId: item.id, mode, x: event.clientX, y: event.clientY, initial: transform, current: transform }; };
  const move = (event: PointerEvent<HTMLButtonElement>) => {
    const d = drag.current; const rect = surface.current?.getBoundingClientRect(); if (!d || !rect) return;
    const dx = (event.clientX - d.x) * width / rect.width; const dy = (event.clientY - d.y) * height / rect.height;
    const initial = d.initial;
    let current = { ...initial };
    if (d.mode === 'move') current = { ...current, x: initial.x + dx, y: initial.y + dy };
    if (d.mode === 'scale') { const ratio = gizmoScaleFactor(initial.width, initial.height, initial.rotation, dx, dy); current = { ...current, width: initial.width * ratio, height: initial.height * ratio, anchorX: initial.anchorX * ratio, anchorY: initial.anchorY * ratio }; }
    if (d.mode === 'rotate') { const cx = rect.left + (width / 2 + initial.x) * rect.width / width; const cy = rect.top + (height / 2 + initial.y) * rect.height / height; const angle = Math.atan2(event.clientY - cy, event.clientX - cx) - Math.atan2(d.y - cy, d.x - cx); current.rotation = initial.rotation + angle * 180 / Math.PI; if (event.shiftKey) current.rotation = Math.round(current.rotation / 15) * 15; }
    d.current = current; useGizmoStore.getState().setTransformPreview({ [d.itemId]: current });
  };
  const finish = (cancel: boolean) => {
    const d = drag.current;
    if (!d) return;
    if (!cancel) {
      const result = withCommandMergeKey(null, () => updateItemTransform(d.itemId, d.current));
      if (result.ok) announce('Đã đổi biến đổi clip');
      else {
        const message = 'Không áp dụng được biến đổi clip';
        toast.error(message);
        announce(message, 'assertive');
      }
    }
    useGizmoStore.getState().replaceItemPreview(d.itemId, null);
    drag.current = null;
  };
  const nudge = (dx: number, dy: number) => {
    if (!item || !transform) return;
    const result = withCommandMergeKey(null, () => updateItemTransform(item.id, { x: transform.x + dx, y: transform.y + dy }));
    if (result.ok) announce(`Vị trí X ${Math.round(transform.x + dx)}, Y ${Math.round(transform.y + dy)}`);
    else {
      const message = 'Không áp dụng được vị trí clip';
      toast.error(message);
      announce(message, 'assertive');
    }
  };
  useShortcutAction({ id: 'gizmo-left', label: 'Dời hình sang trái', scope: 'preview', bindings: ['ctrl+ArrowLeft', 'ctrl+shift+ArrowLeft'], enabled: !!item, run: e => nudge(e.shiftKey ? -10 : -1, 0) });
  useShortcutAction({ id: 'gizmo-right', label: 'Dời hình sang phải', scope: 'preview', bindings: ['ctrl+ArrowRight', 'ctrl+shift+ArrowRight'], enabled: !!item, run: e => nudge(e.shiftKey ? 10 : 1, 0) });
  useShortcutAction({ id: 'gizmo-up', label: 'Dời hình lên', scope: 'preview', bindings: ['ctrl+ArrowUp', 'ctrl+shift+ArrowUp'], enabled: !!item, run: e => nudge(0, e.shiftKey ? -10 : -1) });
  useShortcutAction({ id: 'gizmo-down', label: 'Dời hình xuống', scope: 'preview', bindings: ['ctrl+ArrowDown', 'ctrl+shift+ArrowDown'], enabled: !!item, run: e => nudge(0, e.shiftKey ? 10 : 1) });
  useShortcutAction({ id: 'cancel-gizmo', label: 'Huỷ kéo biến đổi', scope: 'preview', bindings: ['Escape'], run: () => finish(true) });
  return <div ref={surface} role="group" className="pointer-events-none absolute inset-0" aria-label="Biến đổi clip; có thể nhập vị trí, tỉ lệ và xoay trong Thuộc tính">{item && transform && !playing && <div className="absolute border-2 border-primary" style={{ left: `${50 + (transform.x - transform.width / 2) / width * 100}%`, top: `${50 + (transform.y - transform.height / 2) / height * 100}%`, width: `${transform.width / width * 100}%`, height: `${transform.height / height * 100}%`, transform: `rotate(${transform.rotation}deg)`, transformOrigin: `${transform.anchorX / transform.width * 100}% ${transform.anchorY / transform.height * 100}%` }}>
    {(['move', 'scale', 'rotate'] as const).map(mode => <EditorIconButton key={mode} icon={mode === 'move' ? Move : mode === 'scale' ? Maximize2 : RotateCw} label={mode === 'move' ? 'Di chuyển clip' : mode === 'scale' ? 'Đổi tỉ lệ clip' : 'Xoay clip'} shortcut={mode === 'move' ? 'Ctrl+mũi tên; Shift: 10px' : 'Nhập số trong Thuộc tính'} className={`pointer-events-auto absolute border border-ring bg-background/80 ${mode === 'move' ? 'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 cursor-move' : mode === 'scale' ? '-bottom-3.5 -right-3.5 cursor-nwse-resize' : '-top-9 left-1/2 -translate-x-1/2 cursor-grab rounded-full'}`} onPointerDown={e => start(e, mode)} onPointerMove={move} onPointerUp={() => finish(false)} onPointerCancel={() => finish(true)} />)}
  </div>}</div>;
}

export function PreviewPanel() {
  const { items, fps, width, height, duration, resolved, transitions, keyframes, backgroundColor, composition } = usePreviewComposition();
  const quality = usePlaybackStore(s => s.previewQuality); const loop = usePlaybackStore(s => s.loop);
  const player = useRef<PlayerRef>(null); const stage = useRef<HTMLDivElement>(null);
  const [safe, setSafe] = useState(false); const [zoom, setZoom] = useState('fit');
  const [size, setSize] = useState({ width: 0, height: 0 });
  useEffect(() => { const node = stage.current; if (!node) return; const observer = new ResizeObserver(([entry]) => setSize({ width: entry.contentRect.width, height: entry.contentRect.height })); observer.observe(node); return () => observer.disconnect(); }, []);
  usePlayerStoreSync(player);
  const fit = Math.min((size.width - 32) / width, (size.height - 32) / height); const scale = zoom === 'fit' ? Math.max(0, fit) : Number(zoom) / 100;
  return <section aria-label="Xem trước" data-shortcut-scope="preview" className="flex h-full min-h-0 min-w-0 flex-col bg-background text-xs">
    <TransportShortcuts scope="preview" duration={duration} fps={fps} /><TransportShortcuts scope="timeline" duration={duration} fps={fps} />
    <header className="flex min-h-8 flex-wrap items-center justify-between gap-1 border-b border-border px-2"><span>Xem trước</span><div className="flex items-center gap-1"><EditorSelect label="Thu phóng xem trước" value={zoom} onValueChange={setZoom} className="w-auto" options={[{ value: 'fit', label: 'Vừa khung' }, ...[25, 50, 100, 150, 200].map(v => ({ value: String(v), label: `${v}%` }))]} /><EditorSelect label="Chất lượng xem trước" value={String(quality)} onValueChange={value => usePlaybackStore.getState().setPreviewQuality(Number(value) as 1 | .5 | .25)} className="w-auto" options={[{ value: '1', label: '1:1' }, { value: '0.5', label: '½' }, { value: '0.25', label: '¼' }]} /><EditorIconButton icon={Scan} label="Khung an toàn" aria-pressed={safe} onClick={() => setSafe(!safe)} /></div></header>
    <CanvasDescription fps={fps} />
    <div ref={stage} className="relative flex min-h-0 flex-1 items-center justify-center overflow-auto p-4" tabIndex={0} role="region" aria-label="Canvas xem trước" aria-describedby="preview-canvas-description">
      <div className="relative shrink-0 border border-border bg-background" style={{ width: width * scale, height: height * scale }}>
        <Player ref={player} fps={fps} durationInFrames={duration} width={Math.round(width * quality)} height={Math.round(height * quality)} loop={loop} style={{ width: '100%', height: '100%' }} onFrameChange={frame => usePlaybackStore.getState().setCurrentFrame(frame)} onPlayStateChange={playing => { const state = usePlaybackStore.getState(); if (playing !== state.isPlaying) { if (playing) state.play(); else state.pause(); } }}><MainComposition fps={fps} tracks={resolved} transitions={transitions} keyframes={keyframes} width={width} height={height} backgroundColor={backgroundColor} liveItemTransformSource={useItemsStore} /></Player>
        {!items.length && <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background p-4 text-center"><Film aria-hidden className="mb-1 size-8 text-muted-foreground" /><h2 className="text-sm font-semibold">Bắt đầu dựng video</h2><p className="text-xs text-muted-foreground">Nhập media, rồi thêm vào timeline.</p><Button variant="outline" size="sm" onClick={requestMediaImport}>Nhập media</Button></div>}
        <CanvasPreview composition={composition} quality={quality} />
        {safe && <div aria-hidden className="pointer-events-none absolute inset-[10%] border border-dashed border-ring"><div className="absolute inset-[5%] border border-dashed border-ring/60" /></div>}
        <TransformGizmo width={width} height={height} fps={fps} />
      </div>
    </div>
    <Transport duration={duration} fps={fps} />
  </section>;
}
