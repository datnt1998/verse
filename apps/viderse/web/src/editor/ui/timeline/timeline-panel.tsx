"use client";

import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react';
import { defaultRangeExtractor, useVirtualizer } from '@tanstack/react-virtual';
import { ChevronRight, Eye, EyeOff, Flag, GripHorizontal, GripVertical, Headphones, Link, Lock, Magnet, Maximize, Minus, MousePointer2, Plus, Scissors, Trash2, Unlock, Volume2, VolumeX, PanelTop, MoveHorizontal } from 'lucide-react';
import { toast } from '@/editor/ui/toast';
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline';
import type { Transition } from '@editor/types/transition';
import { useTimelineStore } from '@editor/features/timeline/stores/timeline-store';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { flushTrackHeightOverrides, setTrackHeightOverride } from '@editor/features/timeline/utils/track-heights';
import { clampTrackHeight } from '@editor/features/timeline/utils/track-resize';
import * as actions from '@editor/features/timeline/stores/timeline-actions';
import { executeCommands, getCommandExecutor } from '@editor/commands';
import { useZoomStore } from '@editor/features/timeline/stores/zoom-store';
import { useCompositionNavigationStore } from '@editor/features/timeline/stores/composition-navigation-store';
import { useSelectionStore } from '@editor/shared/state/selection';
import { useEditorStore } from '@editor/shared/state/editor';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store';
import { getMediaDragData, clearMediaDragData } from '@editor/features/media-library/utils/drag-data-cache';
import { expandSelectionWithLinkedItems } from '@editor/features/timeline/utils/linked-items';
import { clampTrimAmount, clampToAdjacentItems } from '@editor/features/timeline/utils/trim-utils';
import { clampRippleTrimDeltaToPreserveEditState, clampRollingTrimDeltaToPreserveEditState } from '@editor/features/timeline/utils/trim-edit-constraints';
import { ZOOM_MAX, ZOOM_MIN } from '@editor/features/timeline/constants';
import { EditorIconButton, NumberField } from '@/editor/ui/controls';
import { useShortcutAction, type ShortcutAction } from '@/editor/ui/shortcuts';
import { announce } from '@/editor/ui/shell/announcer';
import { insertMedia, insertText, applyLibraryDrop, destination, LIBRARY_DRAG_MIME, type LibraryDrop } from '@/editor/ui/library/library-actions';
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuSub, ContextMenuSubTrigger, ContextMenuSubContent, ContextMenuRadioGroup, ContextMenuRadioItem } from '@/components/ui/context-menu';
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '@/components/ui/alert-dialog';
import { Slider } from '@/components/ui/slider';
import { clampFrame, fitZoomLevel, indexItems, isDenseWindow, marqueeItems, moveGroup, reorderTrackOrder, resizedTrackHeight, rulerKeyFrame, rulerStep, snapFrame, snapGroupDelta, timecode, visibleItems } from './geometry';
import { TimelineClip, type TimelineClipApi } from './timeline-clip';
import { previewTimelineItems } from './preview';
import { MarkerListDialog } from './marker-list-dialog';
import { DenseTrackCanvas } from './dense-track-canvas';
import { computeItemSelection, hasPointerDrag } from './selection';
import { TYPE_COLOR, TYPE_ICON, TYPE_LABEL } from './item-types';
import { nextTrackName } from './track-names';
import { ownsPointerEvent } from './pointer-ownership';
import { TEMPLATE_SLOT_LABELS } from '@editor/features/editing-templates/channel-template';
import { useProjectStore } from '@editor/features/projects/stores/project-store';
import { useTemplateStatus } from '@/editor/ui/template/template-status';
import { TIMELINE_HEADER as HEADER, timelineStyles as styles } from './styles';
import './timeline.css';

interface Gesture { kind: 'move'|'start'|'end'|'fade-in'|'fade-out'|'marquee'|'scrub'|'in'|'out'|'marker'|'height'; x: number; y: number; frame: number; item?: TimelineItem; ids?: string[]; trackId?: string; markerId?: string; alt?: boolean; delta: number; endX: number; endY: number }
function ShortcutRegistration({ action }: { action: ShortcutAction }) { useShortcutAction(action); return null; }
function Playhead({ scale }: { scale: number }) {
  const frame = usePlaybackStore((state) => state.currentFrame);
  return <div className={styles.playhead} style={{ left: HEADER + frame * scale }} />;
}
function Ruler({ fps, end, scale, startFrame, endFrame, onStart }: { fps: number; end: number; scale: number; startFrame: number; endFrame: number; onStart: (event: ReactPointerEvent<HTMLElement>) => void }) {
  const frame = usePlaybackStore((state) => state.currentFrame);
  const step = rulerStep(fps, scale);
  const ticks = [];
  for (let tick = Math.floor(startFrame / step) * step; tick <= endFrame; tick += step) ticks.push(tick);
  return <div role="slider" aria-label="Vị trí phát" aria-valuemin={0} aria-valuemax={Math.max(0, end - 1)} aria-valuenow={Math.min(frame, Math.max(0, end - 1))} aria-valuetext={timecode(frame, fps)} tabIndex={0} className="absolute inset-y-0" style={{ left: HEADER, right: 0 }} onPointerDown={onStart} onKeyDown={(event) => {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const next = rulerKeyFrame(event.key, usePlaybackStore.getState().currentFrame, fps, end, event.shiftKey);
    if (next === null) return;
    event.preventDefault(); event.stopPropagation();
    usePlaybackStore.getState().setCurrentFrame(next); announce(`Vị trí phát ${timecode(next, fps)}`);
  }}>{ticks.map((tick) => <span key={tick} className={styles.tick} style={{ left: tick * scale }}>{timecode(tick, fps).slice(0, 8)}</span>)}</div>;
}

export function TimelinePanel() {
  // Field by field, not the whole facade state: it also changes with the dirty flag, which flips on every
  // first edit and every autosave and would re-render every timeline row.
  const timeline = {
    items: useTimelineStore((state) => state.items),
    tracks: useTimelineStore((state) => state.tracks),
    transitions: useTimelineStore((state) => state.transitions),
    keyframes: useTimelineStore((state) => state.keyframes),
    markers: useTimelineStore((state) => state.markers),
    fps: useTimelineStore((state) => state.fps),
    inPoint: useTimelineStore((state) => state.inPoint),
    outPoint: useTimelineStore((state) => state.outPoint),
    snapEnabled: useTimelineStore((state) => state.snapEnabled),
  };
  const selectedIds = useSelectionStore((state) => state.selectedItemIds);
  const activeTool = useSelectionStore((state) => state.activeTool);
  const activeTrackId = useSelectionStore((state) => state.activeTrackId);
  const zoom = useZoomStore((state) => state.level);
  const pixelsPerSecond = useZoomStore((state) => state.pixelsPerSecond);
  const breadcrumbs = useCompositionNavigationStore((state) => state.breadcrumbs);
  const linkedSelection = useEditorStore((state) => state.linkedSelectionEnabled);
  const scrollRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLElement>(null);
  const itemRefs = useRef(new Map<string, HTMLButtonElement>());
  const headerRefs = useRef(new Map<string, HTMLButtonElement>());
  const [viewport, setViewportState] = useState({ width: 1000, left: 0, top: 0 });
  // Same numbers keep the same object, so a resize/scroll that changes nothing never re-renders the timeline.
  const syncViewport = useCallback((element: HTMLElement) => setViewportState((current) => (current.width === element.clientWidth && current.left === element.scrollLeft && current.top === element.scrollTop ? current : { width: element.clientWidth, left: element.scrollLeft, top: element.scrollTop })), []);
  const [focusedId, setFocusedId] = useState<string | null>(null);
  const [renameTrack, setRenameTrack] = useState<string | null>(null);
  const [removeTrackId, setRemoveTrackId] = useState<string | null>(null);
  const [markerListOpen, setMarkerListOpen] = useState(false);
  const [openMenuItemId, setOpenMenuItemId] = useState<string | null>(null);
  const [trimMode, setTrimMode] = useState<'ripple'|'rolling'>('ripple');
  const [gesture, setGesture] = useState<Gesture | null>(null);
  const gestureRef = useRef<Gesture | null>(null);
  const pendingItemClick = useRef<{ itemId: string; ids: string[] } | null>(null);
  const [guide, setGuide] = useState<number | null>(null);
  const displayIn = gesture?.kind === 'in' ? clampFrame(gesture.frame + gesture.delta) : timeline.inPoint;
  const displayOut = gesture?.kind === 'out' ? clampFrame(gesture.frame + gesture.delta) : timeline.outPoint;
  const [keyboardMarquee, setKeyboardMarquee] = useState<{ start: number; end: number; firstTrack: number; lastTrack: number } | null>(null);
  const clipboard = useRef<TimelineItem[]>([]);
  const suppressClick = useRef(false);
  const pointerFrame = useRef(0);
  const [coarsePointer, setCoarsePointer] = useState(false);
  const minimumTrackHeight = coarsePointer ? 104 : 72;
  const rulerHeight = coarsePointer ? 48 : 32;
  useEffect(() => {
    const query = window.matchMedia('(pointer: coarse)');
    const changed = () => setCoarsePointer(query.matches);
    changed(); query.addEventListener('change', changed);
    return () => query.removeEventListener('change', changed);
  }, []);
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const tracks = useMemo(() => [...timeline.tracks].sort((a, b) => a.order - b.order), [timeline.tracks]);
  const indexes = useMemo(() => indexItems(timeline.items), [timeline.items]);
  const keyframesById = useMemo(() => Object.fromEntries(timeline.keyframes.map((keys) => [keys.itemId, keys])), [timeline.keyframes]);
  const transitionsByTrack = useMemo(() => {
    const result = new Map<string, Transition[]>();
    for (const transition of timeline.transitions) {
      const list = result.get(transition.trackId) ?? [];
      list.push(transition); result.set(transition.trackId, list);
    }
    return result;
  }, [timeline.transitions]);
  const trackRows = useMemo(() => {
    let start = rulerHeight;
    return tracks.map((track, index) => {
      const size = Math.max(minimumTrackHeight, track.height);
      const row = { index, start, size, end: start + size };
      start += size;
      return row;
    });
  }, [tracks, minimumTrackHeight, rulerHeight]);
  const displayItems = useMemo(() => {
    if (!gesture?.item) return timeline.items;
    if (gesture.kind === 'move') {
      const items = timeline.items.filter((item) => gesture.ids?.includes(item.id) && !tracks.find((track) => track.id === item.trackId)?.locked);
      const row = trackRows.find((entry) => gesture.endY >= entry.start && gesture.endY < entry.end);
      const target = row ? tracks[row.index] : undefined;
      const offset = target && !target.locked ? tracks.findIndex((track) => track.id === target.id) - tracks.findIndex((track) => track.id === gesture.item!.trackId) : 0;
      const updates = moveGroup(items, gesture.delta).map((update) => {
        const index = tracks.findIndex((track) => track.id === update.trackId);
        const destination = tracks[index + offset];
        return destination && !destination.locked && (destination.kind ?? 'video') === (tracks[index].kind ?? 'video') ? { ...update, trackId: destination.id } : update;
      });
      return previewTimelineItems(timeline.items, { kind: 'move', updates }, timeline.fps);
    }
    if (gesture.kind === 'start' || gesture.kind === 'end') return previewTimelineItems(timeline.items, { kind: 'trim', itemId: gesture.item.id, side: gesture.kind, delta: gesture.delta, mode: activeTool === 'trim-edit' ? trimMode : 'normal', linkedSelection }, timeline.fps);
    return timeline.items;
  }, [gesture, timeline.items, timeline.fps, tracks, trackRows, activeTool, trimMode, linkedSelection]);
  const displayIndexes = useMemo(() => displayItems === timeline.items ? indexes : indexItems(displayItems), [displayItems, timeline.items, indexes]);
  const itemById = useMemo(() => new Map(displayItems.map((item) => [item.id, item])), [displayItems]);
  const displayEnd = useMemo(() => Math.max(0, ...displayItems.map((item) => item.from + item.durationInFrames)), [displayItems]);
  const end = useMemo(() => Math.max(timeline.fps * 10, ...timeline.items.map((item) => item.from + item.durationInFrames)), [timeline.items, timeline.fps]);
  const fitLevel = fitZoomLevel(viewport.width - HEADER - 32, end, timeline.fps, ZOOM_MAX);
  const minimumZoom = Math.min(ZOOM_MIN, fitLevel);
  const scale = pixelsPerSecond / timeline.fps;
  const width = Math.max(viewport.width, HEADER + (Math.max(end, displayEnd) + timeline.fps * 5) * scale);
  // TanStack Virtual deliberately retains its own live measurement channel.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualTracks = useVirtualizer({
    count: tracks.length, getScrollElement: () => scrollRef.current,
    estimateSize: (index) => trackRows[index].size, overscan: 3, paddingStart: rulerHeight,
    getItemKey: (index) => tracks[index].id,
    rangeExtractor: (range) => {
      const indices = defaultRangeExtractor(range);
      const focusedTrack = timeline.items.find((item) => item.id === focusedId)?.trackId;
      for (const id of [focusedTrack, activeTrackId]) {
        const index = tracks.findIndex((track) => track.id === id);
        if (index >= 0 && !indices.includes(index)) indices.push(index);
      }
      return indices.sort((a, b) => a - b);
    },
  });
  const firstFrame = Math.max(0, (viewport.left - 100) / scale);
  const lastFrame = (viewport.left + viewport.width - HEADER + 100) / scale;
  const focusItem = useCallback((id: string) => {
    const item = useTimelineStore.getState().items.find((entry) => entry.id === id); if (!item) return;
    setFocusedId(id);
    const container = scrollRef.current; if (!container) return;
    const x = item.from * scale;
    if (x < container.scrollLeft || x + Math.min(item.durationInFrames * scale, 100) > container.scrollLeft + container.clientWidth - HEADER) container.scrollLeft = Math.max(0, x - 40);
    const trackIndex = tracks.findIndex((track) => track.id === item.trackId);
    virtualTracks.scrollToIndex(trackIndex, { align: 'auto' });
    container.focus({ preventScroll: true });
    requestAnimationFrame(() => itemRefs.current.get(id)?.focus({ preventScroll: true }));
  }, [scale, tracks, virtualTracks]);
  const commitItemSelection = useCallback((item: TimelineItem, ids: string[], append = false) => {
    const selection = useSelectionStore.getState();
    selection.setActiveTrack(item.trackId);
    useSelectionStore.getState().selectItems(ids);
    setFocusedId(item.id);
    announce(append ? `Đã đổi vùng chọn, ${ids.length} item` : `${TYPE_LABEL[item.type]} ${item.label}, ${timecode(item.from, timeline.fps)} đến ${timecode(item.from + item.durationInFrames, timeline.fps)}, ${(item.durationInFrames / timeline.fps).toFixed(1)} giây`);
  }, [timeline.fps]);
  const select = useCallback((item: TimelineItem, append = false, range = false) => {
    const ids = computeItemSelection({ items: timeline.items, currentIds: selectedIds, itemId: item.id, anchorId: focusedId, append, range, linkedSelection });
    commitItemSelection(item, ids, append);
  }, [timeline.items, selectedIds, focusedId, linkedSelection, commitItemSelection]);
  const focusItemTrack = (id: string) => {
    const selection = useSelectionStore.getState();
    if (selection.activeTrackId === id) return;
    const ids = selection.selectedItemIds;
    selection.setActiveTrack(id);
    if (ids.length) useSelectionStore.getState().selectItems(ids);
  };
  // Read stores at call time, not render-time snapshots: copy/split/delete triggered from memoised
  // clips and context menus must see the current items and selection.
  const editable = () => {
    const state = useTimelineStore.getState();
    const selected = useSelectionStore.getState().selectedItemIds;
    const ids = new Set(linkedSelection ? expandSelectionWithLinkedItems(state.items, selected) : selected);
    const items = state.items.filter((item) => ids.has(item.id));
    if (items.some((item) => tracks.find((track) => track.id === item.trackId)?.locked)) { toast.error('Track đang khoá. Mở khoá track rồi thử lại.'); return []; }
    return items;
  };
  const chosen = () => useTimelineStore.getState().items.find((item) => item.id === focusedId) ?? editable()[0];
  const currentFrame = () => usePlaybackStore.getState().currentFrame;
  const templateProjectId = useProjectStore((state) => state.currentProject?.id ?? '');
  const isTemplate = useTemplateStatus(templateProjectId).status?.template ?? false;
  /** Slot roles go through `track.update` so the engine validates kind and uniqueness; `null` clears the role. */
  const setTrackRole = (track: TimelineTrack, value: string) => {
    const slot = value === 'none' ? null : value;
    const result = executeCommands([{ type: 'track.update', params: { id: track.id, updates: { templateSlot: slot } } }]);
    if (!result.ok) {
      const message = result.issues.find((issue) => /[À-ỹ]/u.test(issue.message))?.message ?? 'Không gán được vai trò template cho track này.';
      toast.error(message); announce(message, 'assertive'); return;
    }
    announce(slot ? `Track ${track.name}: vai trò ${TEMPLATE_SLOT_LABELS[slot as keyof typeof TEMPLATE_SLOT_LABELS]}` : `Đã bỏ vai trò template của track ${track.name}`);
  };
  const updateTrack = (id: string, update: Partial<TimelineTrack>) => actions.setTracks(useTimelineStore.getState().tracks.map((track) => track.id === id ? { ...track, ...update } : track));
  const resizeTrack = (id: string, height: number, commit = true) => {
    setTrackHeightOverride(id, clampTrackHeight(height));
    if (commit) flushTrackHeightOverrides();
    const state = useItemsStore.getState();
    state.setTracks(state.tracks);
  };
  const focusTrack = (id?: string) => {
    if (!id) { scrollRef.current?.focus(); return; }
    useSelectionStore.getState().setActiveTrack(id);
    const currentTracks = useTimelineStore.getState().tracks;
    const target = currentTracks.find((track) => track.id === id); if (!target) return;
    const index = currentTracks.reduce((count, track) => count + Number(track.order < target.order), 0);
    virtualTracks.scrollToIndex(index, { align: 'auto' });
    requestAnimationFrame(() => headerRefs.current.get(id)?.focus({ preventScroll: true }));
  };
  const reorderTrack = (id: string, direction: number) => {
    const ordered = reorderTrackOrder(useTimelineStore.getState().tracks, id, direction);
    if (!ordered) return;
    actions.setTracks(ordered);
    const track = ordered.find((entry) => entry.id === id);
    focusTrack(id); announce(`Đã dời track ${track?.name} đến vị trí ${(track?.order ?? 0) + 1}`);
  };
  const addTrack = (kind: 'audio'|'video', index = tracks.length) => {
    const id = crypto.randomUUID(); const next = [...tracks];
    next.splice(index, 0, { id, name: nextTrackName(tracks, kind), kind, height: 64, locked: false, visible: true, muted: false, solo: false, order: index, items: [] });
    actions.setTracks(next.map((track, order) => ({ ...track, order }))); requestAnimationFrame(() => focusTrack(id)); announce(`Đã thêm track ${kind === 'audio' ? 'âm thanh' : 'video'}`);
  };
  const removeTrackNow = (id: string) => {
    const track = tracks.find((entry) => entry.id === id); if (!track) return;
    if (track.locked) { toast.error('Track đang khoá. Mở khoá track rồi thử lại.'); return; }
    const result = executeCommands([{ type: 'track.remove', params: { ids: [id] } }]);
    if (!result.ok) { toast.error('Không xoá được track. Hãy thử lại.'); return; }
    const next = tracks.find((entry) => entry.id !== id);
    if (next) focusTrack(next.id); else scrollRef.current?.focus();
    setRemoveTrackId(null); announce('Đã xoá track');
    toast.success(`Đã xoá track ${track.name}`, { action: { label: 'Hoàn tác', onClick: () => { getCommandExecutor().undo(); } }, undo: true });
  };
  /** An empty track goes straight away (with an undo toast); one holding clips asks first. */
  const requestRemoveTrack = (id: string) => {
    if (timeline.items.some((item) => item.trackId === id)) setRemoveTrackId(id); else removeTrackNow(id);
  };
  const deleteSelection = (ripple = false) => {
    const transitionId = useSelectionStore.getState().selectedTransitionId;
    const transition = timeline.transitions.find((entry) => entry.id === transitionId);
    if (transition) {
      if (tracks.find((track) => track.id === transition.trackId)?.locked) { toast.error('Track đang khoá. Mở khoá track rồi thử lại.'); return; }
      actions.removeTransition(transition.id); useSelectionStore.getState().selectTransition(null);
      focusItem(transition.leftClipId); announce('Đã xoá transition'); return;
    }
    if (!selectedIds.length && activeTrackId && headerRefs.current.get(activeTrackId) === document.activeElement) { requestRemoveTrack(activeTrackId); return; }
    if (!selectedIds.length) { toast('Chọn một clip trên timeline để xoá.'); return; }
    const items = editable(); if (!items.length) return;
    const anchor = items[0]; const rest = (indexes.get(anchor.trackId)?.items ?? []).filter((item) => !selectedSet.has(item.id));
    const neighbor = rest.find((item) => item.from >= anchor.from) ?? rest.at(-1);
    if (ripple) actions.rippleDeleteItems(items.map((item) => item.id)); else actions.removeItems(items.map((item) => item.id));
    useSelectionStore.getState().clearItemSelection();
    if (neighbor) focusItem(neighbor.id); else { setFocusedId(null); focusTrack(anchor.trackId); }
    announce(ripple ? `Đã xoá và dồn ${(items.reduce((sum, item) => sum + item.durationInFrames, 0) / timeline.fps).toFixed(1)} giây` : `Đã xoá ${anchor.label}`);
  };
  const duplicate = (delta?: number) => {
    const items = editable(); if (!items.length) return;
    const first = Math.min(...items.map((item) => item.from));
    const amount = delta ?? Math.max(...items.map((item) => item.from + item.durationInFrames)) - first;
    const copies = actions.duplicateItems(items.map((item) => item.id), moveGroup(items, amount).map(({ from, trackId }) => ({ from, trackId })));
    useSelectionStore.getState().selectItems(copies.map((item) => item.id)); if (copies[0]) focusItem(copies[0].id); announce(`Đã nhân bản ${copies.length} item`);
  };
  const split = (at = currentFrame(), target = chosen()) => {
    if (!target || tracks.find((track) => track.id === target.trackId)?.locked) return;
    const result = actions.splitItem(target.id, at);
    if (result) { useSelectionStore.getState().selectItems([result.rightItem.id]); focusItem(result.rightItem.id); announce(`Đã tách clip tại ${timecode(at, timeline.fps)}`); }
  };
  const boundedTrimDelta = (item: TimelineItem, side: 'start'|'end', requested: number) => {
    const siblings = timeline.items.filter((candidate) => candidate.id === item.id || (linkedSelection && item.linkedGroupId && candidate.linkedGroupId === item.linkedGroupId));
    let delta = Math.round(requested);
    for (const sibling of siblings) {
      delta = clampTrimAmount(sibling, side, delta, timeline.fps).clampedAmount;
      if (side === 'start') delta = Math.max(-sibling.from, delta);
    }
    if (activeTool === 'trim-edit' && trimMode === 'rolling') {
      const neighbor = (indexes.get(item.trackId)?.items ?? []).find((entry) => entry.id !== item.id && (side === 'start' ? entry.from + entry.durationInFrames === item.from : entry.from === item.from + item.durationInFrames));
      if (!neighbor || tracks.find((track) => track.id === neighbor.trackId)?.locked) return 0;
      delta = clampTrimAmount(neighbor, side === 'start' ? 'end' : 'start', delta, timeline.fps).clampedAmount;
      return clampRollingTrimDeltaToPreserveEditState(item, side, delta, neighbor, timeline.items, timeline.transitions, keyframesById, timeline.fps);
    }
    if (activeTool === 'trim-edit') return clampRippleTrimDeltaToPreserveEditState(item, side, delta, timeline.items, timeline.transitions, keyframesById, timeline.fps);
    const transitionLinked = new Set(timeline.transitions.filter((transition) => transition.leftClipId === item.id || transition.rightClipId === item.id).flatMap((transition) => [transition.leftClipId, transition.rightClipId]));
    return clampToAdjacentItems(item, side, delta, timeline.items, transitionLinked);
  };
  const trim = (item: TimelineItem, side: 'start'|'end', delta: number) => {
    if (tracks.find((track) => track.id === item.trackId)?.locked) return;
    delta = boundedTrimDelta(item, side, delta);
    if (!delta) return;
    if (activeTool === 'trim-edit' && trimMode === 'rolling') {
      const neighbors = indexes.get(item.trackId)?.items ?? [];
      const left = side === 'start' ? neighbors.find((entry) => entry.from + entry.durationInFrames === item.from) : item;
      const right = side === 'end' ? neighbors.find((entry) => entry.from === item.from + item.durationInFrames) : item;
      if (!left || !right) { announce('Cần hai clip liền kề để rolling trim'); return; }
      actions.rollingTrimItems(left.id, right.id, delta); announce(`Đã rolling edit ${delta} khung`);
    } else if (activeTool === 'trim-edit') { actions.rippleTrimItem(item.id, side, delta); announce(`Đã trim ${side === 'start' ? 'đầu' : 'đuôi'} ${delta} khung`); }
    else { if (side === 'start') actions.trimItemStart(item.id, delta); else actions.trimItemEnd(item.id, delta); announce(`Đã trim ${side === 'start' ? 'đầu' : 'đuôi'} ${delta} khung`); }
  };
  const nudge = (delta: number, trackDelta = 0) => {
    const items = editable(); if (!items.length) return;
    const updates = moveGroup(items, delta).map((update) => {
      const position = tracks.findIndex((track) => track.id === update.trackId); const target = tracks[position + trackDelta];
      return target && !target.locked && (target.kind ?? 'video') === (tracks[position].kind ?? 'video') ? { ...update, trackId: target.id } : update;
    });
    actions.moveItems(updates); const first = updates[0]; announce(`Đã dời đến ${timecode(first.from, timeline.fps)}, track ${tracks.find((track) => track.id === first.trackId)?.name}`);
  };
  const navigate = (direction: number, trackDirection = 0) => {
    const headerFocused = headerRefs.current.get(activeTrackId ?? '') === document.activeElement;
    const item = headerFocused ? undefined : chosen();
    const sourceTrackId = item?.trackId ?? activeTrackId ?? tracks[0]?.id;
    const position = tracks.findIndex((track) => track.id === sourceTrackId);
    const entries = indexes.get(tracks[position + trackDirection]?.id ?? sourceTrackId ?? '')?.items ?? [];
    const frame = item?.from ?? currentFrame();
    const target = trackDirection ? entries.find((entry) => entry.from <= frame && entry.from + entry.durationInFrames > frame) ?? entries[0]
      : item ? entries[entries.findIndex((entry) => entry.id === item.id) + direction]
      : direction > 0 ? entries.find((entry) => entry.from >= frame) ?? entries[0] : [...entries].reverse().find((entry) => entry.from <= frame) ?? entries.at(-1);
    if (target) { select(target); focusItem(target.id); }
  };
  const zoomTo = (level: number, anchorX = (viewport.width - HEADER) / 2) => {
    const container = scrollRef.current; if (!container) return;
    const anchoredFrame = (container.scrollLeft + anchorX) / scale;
    const next = Math.max(minimumZoom, Math.min(ZOOM_MAX, level)); useZoomStore.getState().setZoomLevelSynchronized(next);
    requestAnimationFrame(() => { container.scrollLeft = Math.max(0, anchoredFrame * next * 100 / timeline.fps - anchorX); });
  };
  const fit = () => {
    useZoomStore.getState().setZoomLevelSynchronized(fitLevel);
    if (scrollRef.current) scrollRef.current.scrollLeft = 0;
    announce(`Timeline vừa khung, ${(fitLevel * 100).toFixed(fitLevel < 0.01 ? 1 : 0)} phần trăm`);
  };
  const addMarker = () => {
    const existing = new Set(timeline.markers.map((marker) => marker.id));
    const frame = currentFrame(); actions.addMarker(frame);
    const marker = useTimelineStore.getState().markers.find((entry) => !existing.has(entry.id));
    if (marker) {
      useSelectionStore.getState().selectMarker(marker.id);
      const element = scrollRef.current;
      if (element && (frame * scale < element.scrollLeft || frame * scale > element.scrollLeft + element.clientWidth - HEADER)) element.scrollLeft = Math.max(0, frame * scale - (element.clientWidth - HEADER) / 2);
      requestAnimationFrame(() => panelRef.current?.querySelector<HTMLButtonElement>(`[data-marker-id="${marker.id}"]`)?.focus());
    }
    announce(`Đã thêm marker tại ${timecode(frame, timeline.fps)}`);
  };
  const jumpMarker = (direction: number) => {
    const markers = [...timeline.markers].sort((a, b) => a.frame - b.frame);
    const marker = direction > 0 ? markers.find((entry) => entry.frame > currentFrame()) : markers.reverse().find((entry) => entry.frame < currentFrame());
    if (marker) { usePlaybackStore.getState().setCurrentFrame(marker.frame); useSelectionStore.getState().selectMarker(marker.id); scrollRef.current?.querySelector<HTMLElement>('[role="slider"]')?.focus(); announce(`Marker ${marker.label || marker.id.slice(0, 4)}, ${timecode(marker.frame, timeline.fps)}`); }
  };
  const cancel = () => {
    const active = gestureRef.current;
    pendingItemClick.current = null; suppressClick.current = true;
    gestureRef.current = null; setGesture(null); setGuide(null);
    if (active?.item) { focusItem(active.item.id); announce('Đã huỷ trim'); }
    else { setKeyboardMarquee(null); useSelectionStore.getState().clearSelection(); scrollRef.current?.focus(); announce('Bỏ chọn'); }
  };
  const registrations: ShortcutAction[] = [
    { id: 'select', label: 'Công cụ chọn', run: () => useSelectionStore.getState().setActiveTool('select') },
    { id: 'razor', label: 'Dao cắt', run: () => useSelectionStore.getState().setActiveTool('razor') },
    { id: 'trim', label: 'Công cụ trim', run: () => { useSelectionStore.getState().setActiveTool('trim-edit'); itemRefs.current.get(focusedId ?? '')?.parentElement?.querySelector<HTMLButtonElement>('[data-trim="end"]')?.focus(); } },
    { id: 'split', label: 'Tách tại playhead', run: () => split() },
    { id: 'split-pointer', label: 'Tách tại con trỏ', bindings: ['shift+KeyC'], run: () => split(pointerFrame.current) },
    { id: 'snap', label: 'Bật hoặc tắt snap', run: () => { actions.toggleSnap(); setGuide(null); announce(timeline.snapEnabled ? 'Đã tắt snap' : 'Đã bật snap'); } },
    { id: 'delete', label: 'Xoá đã chọn', run: () => deleteSelection() }, { id: 'ripple-delete', label: 'Xoá và dồn', run: () => deleteSelection(true) },
    { id: 'duplicate', label: 'Nhân bản đã chọn', run: () => duplicate() },
    { id: 'deselect', label: 'Bỏ chọn', run: cancel },
    { id: 'select-all', label: 'Chọn item trên track', run: () => { const ids = (indexes.get(activeTrackId ?? '')?.items ?? []).map((item) => item.id); useSelectionStore.getState().selectItems(ids); announce(`Đã chọn ${ids.length} item`); } },
    { id: 'item-previous', label: 'Item trước', run: () => navigate(-1) }, { id: 'item-next', label: 'Item sau', run: () => navigate(1) },
    { id: 'item-track-up', label: 'Item track trên', bindings: ['alt+ArrowUp'], run: () => navigate(0, -1) }, { id: 'item-track-down', label: 'Item track dưới', bindings: ['alt+ArrowDown'], run: () => navigate(0, 1) },
    { id: 'nudge-left', label: 'Dời trái một khung', run: () => nudge(-1) }, { id: 'nudge-right', label: 'Dời phải một khung', run: () => nudge(1) },
    { id: 'nudge-second-left', label: 'Dời trái một giây', run: () => nudge(-timeline.fps) }, { id: 'nudge-second-right', label: 'Dời phải một giây', run: () => nudge(timeline.fps) },
    { id: 'track-up', label: 'Nâng item một track', run: () => nudge(0, -1) }, { id: 'track-down', label: 'Hạ item một track', run: () => nudge(0, 1) },
    { id: 'ripple-head', label: 'Trim đầu đến playhead', run: () => { const item = chosen(); if (item && !tracks.find((track) => track.id === item.trackId)?.locked) { actions.rippleTrimItem(item.id, 'start', currentFrame() - item.from); announce(`Đã trim đầu đến ${timecode(currentFrame(), timeline.fps)}`); } } },
    { id: 'ripple-tail', label: 'Trim đuôi đến playhead', run: () => { const item = chosen(); if (item && !tracks.find((track) => track.id === item.trackId)?.locked) { actions.rippleTrimItem(item.id, 'end', currentFrame() - item.from - item.durationInFrames); announce(`Đã trim đuôi đến ${timecode(currentFrame(), timeline.fps)}`); } } },
    { id: 'link', label: 'Liên kết audio và video', run: () => { if (actions.linkItems(editable().map((item) => item.id))) announce('Đã liên kết audio và video'); else announce('Chọn audio và video đồng bộ để liên kết'); } },
    { id: 'unlink', label: 'Tách liên kết', run: () => { actions.unlinkItems(editable().map((item) => item.id)); announce('Đã tách liên kết'); } },
    { id: 'linked-selection', label: 'Chọn theo liên kết', run: () => { useEditorStore.getState().toggleLinkedSelectionEnabled(); announce(linkedSelection ? 'Đã tắt chọn theo liên kết' : 'Đã bật chọn theo liên kết'); } },
    { id: 'zoom-in', label: 'Phóng to timeline', run: () => zoomTo(zoom * 1.25) }, { id: 'zoom-out', label: 'Thu nhỏ timeline', run: () => zoomTo(zoom / 1.25) },
    { id: 'zoom-fit', label: 'Vừa timeline', run: fit }, { id: 'zoom-reset', label: 'Zoom 100%', run: () => zoomTo(1) },
    { id: 'set-in', label: 'Đặt điểm vào', run: () => { actions.setInPoint(currentFrame()); announce(`Điểm vào ${timecode(currentFrame(), timeline.fps)}`); } },
    { id: 'set-out', label: 'Đặt điểm ra', run: () => { actions.setOutPoint(currentFrame()); announce(`Điểm ra ${timecode(currentFrame(), timeline.fps)}`); } }, { id: 'clear-in-out', label: 'Xoá vùng vào ra', run: () => actions.clearInOutPoints() },
    { id: 'marker', label: 'Thêm marker', run: addMarker },
    { id: 'marker-list', label: 'Danh sách marker', bindings: ['alt+KeyM'], run: () => setMarkerListOpen(true) },
    { id: 'delete-marker', label: 'Xoá marker', run: () => {
      const markerId = useSelectionStore.getState().selectedMarkerId;
      const marker = timeline.markers.find((entry) => entry.id === markerId) ?? timeline.markers.find((entry) => entry.frame === currentFrame());
      if (marker) { actions.removeMarker(marker.id); useSelectionStore.getState().selectMarker(null); scrollRef.current?.querySelector<HTMLElement>('[role="slider"]')?.focus(); announce('Đã xoá marker'); }
    } },
    { id: 'marker-previous', label: 'Marker trước', run: () => jumpMarker(-1) }, { id: 'marker-next', label: 'Marker sau', run: () => jumpMarker(1) },
    ...([-1, 1] as const).map((direction) => ({
      id: direction < 0 ? 'keyframe-previous' : 'keyframe-next', label: direction < 0 ? 'Keyframe trước' : 'Keyframe sau',
      run: () => {
        const item = chosen(); if (!item) return;
        const keys = keyframesById[item.id];
        const frames = [...(keys?.properties.flatMap((property) => property.keyframes.map((key) => item.from + key.frame)) ?? []), ...(keys?.vectorProperties?.flatMap((property) => property.keyframes.map((key) => item.from + key.frame)) ?? [])].sort((a, b) => a - b);
        const frame = direction > 0 ? frames.find((value) => value > currentFrame()) : frames.reverse().find((value) => value < currentFrame());
        if (frame !== undefined) { usePlaybackStore.getState().setCurrentFrame(frame); announce(`Keyframe tại ${timecode(frame, timeline.fps)}`); }
      },
    })),
    { id: 'track-focus-up', label: 'Track trước', bindings: ['alt+PageUp'], run: () => focusTrack(tracks[Math.max(0, tracks.findIndex((track) => track.id === activeTrackId) - 1)]?.id) },
    { id: 'track-focus-down', label: 'Track sau', bindings: ['alt+PageDown'], run: () => focusTrack(tracks[Math.min(tracks.length - 1, tracks.findIndex((track) => track.id === activeTrackId) + 1)]?.id) },
    { id: 'track-order-up', label: 'Dời track lên', bindings: ['ctrl+shift+PageUp'], run: () => activeTrackId && reorderTrack(activeTrackId, -1) }, { id: 'track-order-down', label: 'Dời track xuống', bindings: ['ctrl+shift+PageDown'], run: () => activeTrackId && reorderTrack(activeTrackId, 1) },
    ...(['lock', 'hide', 'solo'] as const).map((kind, index) => ({
      id: `track-${kind}`, label: kind === 'lock' ? 'Khoá track' : kind === 'hide' ? 'Ẩn hoặc tắt tiếng track' : 'Solo track',
      bindings: [`ctrl+shift+Key${['K', 'M', 'O'][index]}`],
      run: () => {
        const track = tracks.find((entry) => entry.id === activeTrackId); if (!track) return;
        updateTrack(track.id, kind === 'lock' ? { locked: !track.locked } : kind === 'solo' ? { solo: !track.solo } : track.kind === 'audio' ? { muted: !track.muted } : { visible: !track.visible });
        const verb = kind === 'lock' ? track.locked ? 'Đã mở khoá' : 'Đã khoá'
          : kind === 'solo' ? track.solo ? 'Đã tắt solo' : 'Solo'
          : track.kind === 'audio' ? track.muted ? 'Đã bật tiếng' : 'Đã tắt tiếng' : track.visible ? 'Đã ẩn' : 'Đã hiện';
        announce(`${verb} track ${track.name}`);
      },
    })),
    { id: 'rename-track', label: 'Đổi tên track', bindings: ['F2'], run: () => setRenameTrack(activeTrackId) },
    { id: 'context-menu', label: 'Mở menu ngữ cảnh', bindings: ['shift+F10', 'ContextMenu'], run: () => { const target = document.activeElement; if (target instanceof HTMLElement) { const rect = target.getBoundingClientRect(); target.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: rect.x + 20, clientY: rect.y + 20 })); } } },
    { id: 'marquee', label: 'Chọn vùng timeline', bindings: ['shift+KeyA'], run: () => { const track = Math.max(0, tracks.findIndex((entry) => entry.id === activeTrackId)); setKeyboardMarquee({ start: currentFrame(), end: currentFrame(), firstTrack: track, lastTrack: track }); scrollRef.current?.focus(); announce('Chọn vùng bằng mũi tên, Enter chốt'); } },
    { id: 'copy', label: 'Sao chép item', run: () => { clipboard.current = editable(); announce(`Đã sao chép ${clipboard.current.length} item`); } },
    { id: 'cut', label: 'Cắt item', run: () => { clipboard.current = editable(); deleteSelection(); } },
    { id: 'paste', label: 'Dán item', run: () => {
      if (!clipboard.current.length) return;
      const first = Math.min(...clipboard.current.map((item) => item.from));
      const groups = new Map<string, string>();
      const items = clipboard.current.map((item) => {
        if (item.linkedGroupId && !groups.has(item.linkedGroupId)) groups.set(item.linkedGroupId, crypto.randomUUID());
        return { ...item, id: crypto.randomUUID(), from: Math.max(0, currentFrame() + item.from - first), linkedGroupId: item.linkedGroupId ? groups.get(item.linkedGroupId) : undefined };
      });
      if (items.some((item) => !tracks.some((track) => track.id === item.trackId && !track.locked))) { announce('Track đích đã bị xoá hoặc đang khoá'); return; }
      actions.addItems(items); useSelectionStore.getState().selectItems(items.map((item) => item.id)); focusItem(items[0].id); announce(`Đã dán ${items.length} item`);
    } },
    ...([-1, 1] as const).map((direction) => ({
      id: direction < 0 ? 'snap-previous' : 'snap-next', label: direction < 0 ? 'Điểm snap trước' : 'Điểm snap sau',
      bindings: [direction < 0 ? 'ctrl+shift+BracketLeft' : 'ctrl+shift+BracketRight'],
      run: () => {
        const targets = [0, end, ...timeline.markers.map((marker) => marker.frame), ...timeline.items.flatMap((item) => [item.from, item.from + item.durationInFrames])].sort((a, b) => a - b);
        const next = direction > 0 ? targets.find((frame) => frame > currentFrame()) : targets.reverse().find((frame) => frame < currentFrame());
        if (next !== undefined) { usePlaybackStore.getState().setCurrentFrame(next); announce(`Vị trí phát ${timecode(next, timeline.fps)}`); }
      },
    })),
  ].map((action) => ({ ...action, scope: 'timeline' as const }));

  const onWheel = useEffectEvent((event: WheelEvent, element: HTMLElement) => {
    if (event.metaKey || event.ctrlKey) { event.preventDefault(); zoomTo(zoom * Math.exp(-event.deltaY * 0.002), event.clientX - element.getBoundingClientRect().left - HEADER); }
    else if (event.shiftKey) { event.preventDefault(); element.scrollLeft += event.deltaY || event.deltaX; }
  });
  // Subscribed once: a fresh ResizeObserver per render fires its first callback immediately and kept the panel re-rendering forever.
  useEffect(() => {
    const element = scrollRef.current; if (!element) return;
    const observer = new ResizeObserver(() => syncViewport(element)); observer.observe(element);
    const wheel = (event: WheelEvent) => onWheel(event, element);
    element.addEventListener('wheel', wheel, { passive: false });
    return () => { observer.disconnect(); element.removeEventListener('wheel', wheel); };
  }, [syncViewport]);
  useEffect(() => { virtualTracks.measure(); }, [tracks, minimumTrackHeight, virtualTracks]);

  const point = (event: {clientX: number; clientY: number}) => {
    const element = scrollRef.current!; const rect = element.getBoundingClientRect();
    return { x: event.clientX - rect.left + element.scrollLeft - HEADER, y: event.clientY - rect.top + element.scrollTop, frame: clampFrame((event.clientX - rect.left + element.scrollLeft - HEADER) / scale) };
  };
  const begin = (event: ReactPointerEvent<HTMLElement>, kind: Gesture['kind'], item?: TimelineItem, trackId?: string, markerId?: string) => {
    if (event.button !== 0) return;
    event.preventDefault(); event.stopPropagation();
    suppressClick.current = false;
    pendingItemClick.current = item && kind === 'move' ? {
      itemId: item.id,
      ids: computeItemSelection({ items: timeline.items, currentIds: useSelectionStore.getState().selectedItemIds, itemId: item.id, anchorId: focusedId, append: event.metaKey, range: event.shiftKey, linkedSelection }),
    } : null;
    if (item && tracks.find((track) => track.id === item.trackId)?.locked) {
      select(item, event.metaKey, event.shiftKey); event.currentTarget.focus(); return;
    }
    const captureTarget = item && kind === 'move' ? event.currentTarget : scrollRef.current;
    captureTarget?.setPointerCapture(event.pointerId);
    const position = point(event);
    let ids = selectedIds;
    if (item) {
      event.currentTarget.focus({ preventScroll: true });
      if (kind !== 'move' && (!selectedSet.has(item.id) || event.metaKey || event.shiftKey)) select(item, event.metaKey, event.shiftKey);
      if (pendingItemClick.current?.itemId === item.id && (!selectedSet.has(item.id) || event.metaKey || event.shiftKey)) commitItemSelection(item, pendingItemClick.current.ids, event.metaKey);
      ids = useSelectionStore.getState().selectedItemIds;
      if (linkedSelection) {
        ids = expandSelectionWithLinkedItems(timeline.items, ids);
        if (ids.includes(item.id)) ids = [item.id, ...ids.filter((id) => id !== item.id)];
        useSelectionStore.getState().selectItems(ids);
      }
      const affected = kind === 'move' ? ids : linkedSelection ? expandSelectionWithLinkedItems(timeline.items, [item.id]) : [item.id];
      if (timeline.items.some((entry) => affected.includes(entry.id) && tracks.find((track) => track.id === entry.trackId)?.locked)) { announce('Vùng chọn có track đang khoá'); return; }
    }
    if (kind === 'move' && item && activeTool === 'razor') { pendingItemClick.current = null; suppressClick.current = true; split(position.frame, item); return; }
    const baseFrame = kind === 'in' ? timeline.inPoint ?? position.frame : kind === 'out' ? timeline.outPoint ?? position.frame : kind === 'marker' ? timeline.markers.find((marker) => marker.id === markerId)?.frame ?? position.frame : position.frame;
    if (kind === 'marker' && markerId) { useSelectionStore.getState().selectMarker(markerId); usePlaybackStore.getState().setCurrentFrame(baseFrame); }
    if (trackId && !item) useSelectionStore.getState().setActiveTrack(trackId);
    if (event.currentTarget.tabIndex >= 0) event.currentTarget.focus({ preventScroll: true }); else scrollRef.current?.focus({ preventScroll: true });
    const next: Gesture = { kind, ...position, frame: baseFrame, item, ids, trackId, markerId, alt: event.altKey, delta: 0, endX: position.x, endY: position.y };
    gestureRef.current = next; setGesture(next);
    if (kind === 'scrub') usePlaybackStore.getState().setCurrentFrame(Math.min(end, position.frame));
  };
  const move = (event: ReactPointerEvent<HTMLElement>) => {
    const active = gestureRef.current; if (!active) { pointerFrame.current = point(event).frame; return; }
    const position = point(event); pointerFrame.current = position.frame; let delta = Math.round((position.x - active.x) / scale);
    if (active.kind === 'move') {
      if (!hasPointerDrag(active.x, active.y, position.x, position.y)) {
        gestureRef.current = { ...active, delta: 0, endX: position.x, endY: position.y };
        return;
      }
      scrollRef.current?.setPointerCapture(event.pointerId);
    }
    let nextGuide: number | null = null;
    const excluded = new Set(active.kind === 'move' ? active.ids : active.item ? linkedSelection ? expandSelectionWithLinkedItems(timeline.items, [active.item.id]) : [active.item.id] : []);
    if (timeline.snapEnabled && !['fade-in', 'fade-out', 'height', 'marquee'].includes(active.kind)) {
      const targets = [currentFrame(), ...timeline.markers.filter((marker) => marker.id !== active.markerId).map((marker) => marker.frame), ...timeline.items.filter((item) => !excluded.has(item.id)).flatMap((item) => [item.from, item.from + item.durationInFrames])];
      if (active.kind === 'move') {
        const items = timeline.items.filter((item) => excluded.has(item.id) && !tracks.find((track) => track.id === item.trackId)?.locked);
        const snapped = snapGroupDelta(items, delta, targets, scale);
        delta = snapped.delta; nextGuide = snapped.guide;
      } else {
        const source = active.item ? active.item.from + (active.kind === 'end' ? active.item.durationInFrames : 0) : active.frame;
        const snapped = snapFrame(source + delta, targets, scale);
        delta = snapped.frame - source; nextGuide = snapped.guide;
      }
    }
    if (active.kind === 'move') {
      const items = timeline.items.filter((item) => excluded.has(item.id));
      if (items.length) delta = Math.max(-Math.min(...items.map((item) => item.from)), delta);
    }
    if (active.item && (active.kind === 'start' || active.kind === 'end')) {
      const bounded = boundedTrimDelta(active.item, active.kind, delta);
      if (bounded !== delta) nextGuide = null;
      delta = bounded;
    }
    const next = { ...active, delta, endX: position.x, endY: position.y }; gestureRef.current = next; setGesture(next); setGuide(nextGuide);
    if (active.kind === 'scrub') usePlaybackStore.getState().setCurrentFrame(clampFrame(active.frame + delta, end));
    const element = scrollRef.current!; const rect = element.getBoundingClientRect();
    if (event.clientX > rect.right - 24) element.scrollLeft += 12; else if (event.clientX < rect.left + HEADER + 24 && active.kind !== 'height') element.scrollLeft -= 12;
  };
  const finish = () => {
    const active = gestureRef.current; if (!active) return;
    gestureRef.current = null; setGesture(null); setGuide(null);
    const dragged = hasPointerDrag(active.x, active.y, active.endX, active.endY);
    suppressClick.current = dragged || active.kind !== 'move';
    if (dragged) pendingItemClick.current = null;
    if (active.kind === 'move' && active.item && dragged) {
      const items = timeline.items.filter((item) => active.ids?.includes(item.id) && !tracks.find((track) => track.id === item.trackId)?.locked);
      const targetRow = trackRows.find((row) => active.endY >= row.start && active.endY <= row.end);
      const target = targetRow ? tracks[targetRow.index] : undefined;
      const trackOffset = target && !target.locked ? tracks.findIndex((track) => track.id === target.id) - tracks.findIndex((track) => track.id === active.item!.trackId) : 0;
      const updates = moveGroup(items, active.delta).map((update) => { const index = tracks.findIndex((track) => track.id === update.trackId); const destination = tracks[index + trackOffset]; return destination && !destination.locked && (destination.kind ?? 'video') === (tracks[index].kind ?? 'video') ? { ...update, trackId: destination.id } : update; });
      if (active.alt) { const copies = actions.duplicateItems(items.map((item) => item.id), updates); useSelectionStore.getState().selectItems(copies.map((item) => item.id)); if (copies[0]) focusItem(copies[0].id); announce(`Đã nhân bản ${copies.length} item`); }
      else { actions.moveItems(updates); focusItem(active.item.id); announce(`Đã dời đến ${timecode(updates[0]?.from ?? 0, timeline.fps)}`); }
    } else if ((active.kind === 'start' || active.kind === 'end') && active.item && active.delta) trim(active.item, active.kind, active.delta);
    else if ((active.kind === 'fade-in' || active.kind === 'fade-out') && active.item) {
      const audio = active.item.type === 'audio'; const key = audio ? active.kind === 'fade-in' ? 'audioFadeIn' : 'audioFadeOut' : active.kind === 'fade-in' ? 'fadeIn' : 'fadeOut';
      const value = Math.max(0, Math.min(active.item.durationInFrames / timeline.fps, (active.item[key] ?? 0) + active.delta / timeline.fps * (active.kind === 'fade-in' ? 1 : -1)));
      actions.updateItem(active.item.id, { [key]: value }); announce(`Fade ${value.toFixed(1)} giây`);
    } else if (active.kind === 'in') actions.setInPoint(active.frame + active.delta);
    else if (active.kind === 'out') actions.setOutPoint(active.frame + active.delta);
    else if (active.kind === 'marker' && active.markerId) { const frame = clampFrame(active.frame + active.delta); if (active.delta) actions.updateMarker(active.markerId, { frame }); usePlaybackStore.getState().setCurrentFrame(frame); announce(`Marker tại ${timecode(frame, timeline.fps)}`); }
    else if (active.kind === 'height' && active.trackId) { const track = tracks.find((entry) => entry.id === active.trackId); if (track) resizeTrack(track.id, resizedTrackHeight(track.height, active.endY - active.y, minimumTrackHeight)); }
    else if (active.kind === 'marquee') {
      const rows = trackRows.filter((row) => row.end >= Math.min(active.y, active.endY) && row.start <= Math.max(active.y, active.endY));
      const ids = marqueeItems(timeline.items, active.frame, active.frame + active.delta, new Set(rows.map((row) => tracks[row.index].id)));
      useSelectionStore.getState().selectItems(ids); if (ids[0]) focusItem(ids[0]); announce(`Đã chọn ${ids.length} item`);
    } else if (active.kind === 'scrub') announce(`Vị trí phát ${timecode(currentFrame(), timeline.fps)}`);
  };

  // Live implementations for the memoised TimelineClip: the api ref is stable so clips do not
  // re-render on closure changes, while handlers call the panel's latest functions.
  const clipApi = useRef<TimelineClipApi | null>(null);
  useLayoutEffect(() => {
    clipApi.current = {
      begin: (event, kind, item) => begin(event, kind, item),
      click: (item, event) => {
        if (!suppressClick.current) {
          const pending = pendingItemClick.current;
          if (event.detail > 0 && pending?.itemId === item.id) commitItemSelection(item, pending.ids, event.metaKey);
          else select(item, event.metaKey, event.shiftKey);
        }
        pendingItemClick.current = null; suppressClick.current = false;
      },
      selectOnContextMenu: (item) => { if (!useSelectionStore.getState().selectedItemIds.includes(item.id)) select(item); },
      focus: (item) => { setFocusedId(item.id); focusItemTrack(item.trackId); },
      doubleClick: (item) => { if (item.type === 'composition') useCompositionNavigationStore.getState().enterComposition(item.compositionId, item.label, item.id); },
      itemKeyDown: (item, event) => {
        if (event.key === 'Enter') { event.preventDefault(); select(item, event.metaKey); }
        if (event.key === 'Tab') {
          const entries = useTimelineStore.getState().items.filter((entry) => entry.trackId === item.trackId).sort((a, b) => a.from - b.from || a.id.localeCompare(b.id));
          const next = entries[entries.findIndex((entry) => entry.id === item.id) + (event.shiftKey ? -1 : 1)];
          if (next) { event.preventDefault(); focusItem(next.id); }
        }
      },
      trimKey: (item, side, event) => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
        event.preventDefault(); event.stopPropagation();
        trim(item, side, (event.key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? timeline.fps : 1));
      },
      fadeKey: (item, side, event) => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
        event.preventDefault(); event.stopPropagation();
        const key = item.type === 'audio' ? side === 'fade-in' ? 'audioFadeIn' : 'audioFadeOut' : side === 'fade-in' ? 'fadeIn' : 'fadeOut';
        const value = Math.max(0, Math.min(item.durationInFrames / timeline.fps, (item[key] ?? 0) + (event.key === 'ArrowLeft' ? -1 : 1) / timeline.fps * (event.shiftKey ? timeline.fps : 1)));
        actions.updateItem(item.id, { [key]: value }); announce(`Fade ${value.toFixed(1)} giây`);
      },
      menuOpenChange: (itemId, open) => setOpenMenuItemId((current) => (open ? itemId : current === itemId ? null : current)),
      registerRef: (itemId, element) => { if (element) itemRefs.current.set(itemId, element); else itemRefs.current.delete(itemId); },
    };
  });

  const itemMenu = (item: TimelineItem) => <ContextMenuContent className="w-56" onCloseAutoFocus={(event) => { event.preventDefault(); focusItem(useSelectionStore.getState().selectedItemIds[0] ?? item.id); }}>
    <ContextMenuItem disabled={tracks.find((track) => track.id === item.trackId)?.locked} onSelect={() => split(currentFrame(), item)}>Tách tại playhead <span className="ml-auto text-muted-foreground">S</span></ContextMenuItem>
    <ContextMenuItem onSelect={() => duplicate()}>Nhân bản <span className="ml-auto text-muted-foreground">⇧D</span></ContextMenuItem>
    <ContextMenuItem onSelect={() => { if (actions.linkItems(useSelectionStore.getState().selectedItemIds)) announce('Đã liên kết audio và video'); }}>Liên kết audio và video</ContextMenuItem>
    <ContextMenuItem disabled={!item.linkedGroupId} onSelect={() => { actions.unlinkItems(useSelectionStore.getState().selectedItemIds); announce('Đã tách liên kết'); }}>Tách liên kết</ContextMenuItem>
    {item.type === 'composition' && <ContextMenuItem onSelect={() => { useCompositionNavigationStore.getState().enterComposition(item.compositionId, item.label, item.id); announce('Đã mở composition'); }}>Vào composition</ContextMenuItem>}
    <ContextMenuSeparator />
    <div className="px-2 py-1"><NumberField label="Vị trí (khung)" value={item.from} min={0} step={1} disabled={tracks.find((track) => track.id === item.trackId)?.locked} onChange={(from) => actions.moveItem(item.id, from)} /></div>
    <div className="px-2 py-1"><NumberField label="Thời lượng (khung)" value={item.durationInFrames} min={1} step={1} disabled={tracks.find((track) => track.id === item.trackId)?.locked} onChange={(duration) => { const current = useTimelineStore.getState().items.find((entry) => entry.id === item.id); if (current) trim(current, 'end', duration - current.durationInFrames); }} /></div>
    <div className="px-2 py-1"><NumberField label="Fade vào (giây)" value={item.type === 'audio' ? item.audioFadeIn ?? 0 : item.fadeIn ?? 0} min={0} max={item.durationInFrames / timeline.fps} step={0.1} disabled={tracks.find((track) => track.id === item.trackId)?.locked} onChange={(value) => actions.updateItem(item.id, item.type === 'audio' ? { audioFadeIn: value } : { fadeIn: value })} /></div>
    <div className="px-2 py-1"><NumberField label="Fade ra (giây)" value={item.type === 'audio' ? item.audioFadeOut ?? 0 : item.fadeOut ?? 0} min={0} max={item.durationInFrames / timeline.fps} step={0.1} disabled={tracks.find((track) => track.id === item.trackId)?.locked} onChange={(value) => actions.updateItem(item.id, item.type === 'audio' ? { audioFadeOut: value } : { fadeOut: value })} /></div>
    <ContextMenuSeparator /><ContextMenuItem onSelect={() => deleteSelection()}>Xoá <span className="ml-auto text-muted-foreground">Delete</span></ContextMenuItem><ContextMenuItem onSelect={() => deleteSelection(true)}>Xoá và dồn <span className="ml-auto text-muted-foreground">⇧Delete</span></ContextMenuItem>
  </ContextMenuContent>;

  return <section ref={panelRef} className={styles.panel} style={{ '--timeline-ruler-height': `${rulerHeight}px` } as CSSProperties} aria-label="Timeline" data-shortcut-scope="timeline">
    {registrations.map((action) => <ShortcutRegistration key={action.id} action={action} />)}
    <div className={styles.toolbar} role="toolbar" aria-label="Công cụ timeline">
      <EditorIconButton icon={MousePointer2} label="Công cụ chọn" shortcut="V" aria-pressed={activeTool === 'select'} onClick={() => useSelectionStore.getState().setActiveTool('select')} />
      <EditorIconButton icon={Scissors} label="Dao cắt" shortcut="C" aria-pressed={activeTool === 'razor'} onClick={() => useSelectionStore.getState().setActiveTool('razor')} />
      <EditorIconButton icon={MoveHorizontal} label="Trim ripple" shortcut="T" aria-pressed={activeTool === 'trim-edit' && trimMode === 'ripple'} onClick={() => { setTrimMode('ripple'); useSelectionStore.getState().setActiveTool('trim-edit'); }} />
      <EditorIconButton icon={PanelTop} label="Trim rolling" aria-pressed={activeTool === 'trim-edit' && trimMode === 'rolling'} onClick={() => { setTrimMode('rolling'); useSelectionStore.getState().setActiveTool('trim-edit'); }} />
      <span className="mx-1 h-5 border-l border-border" />
      <EditorIconButton icon={Magnet} label="Bật hoặc tắt snap" shortcut="N" aria-pressed={timeline.snapEnabled} onClick={actions.toggleSnap} />
      <EditorIconButton icon={Link} label="Chọn theo liên kết" shortcut="⇧L" aria-pressed={linkedSelection} onClick={() => useEditorStore.getState().toggleLinkedSelectionEnabled()} />
      <EditorIconButton icon={Flag} label="Thêm marker; menu mở danh sách marker" shortcut="M / ⌥M" onClick={addMarker} onContextMenu={(event) => { event.preventDefault(); setMarkerListOpen(true); }} />
      <ContextMenu><ContextMenuTrigger asChild><span><EditorIconButton icon={Plus} label="Thêm track video; menu để chọn âm thanh" onClick={() => addTrack('video')} /></span></ContextMenuTrigger><ContextMenuContent><ContextMenuItem onSelect={() => addTrack('video')}>Thêm track video</ContextMenuItem><ContextMenuItem onSelect={() => addTrack('audio')}>Thêm track âm thanh</ContextMenuItem></ContextMenuContent></ContextMenu>
      <EditorIconButton icon={Trash2} label={selectedIds.length ? 'Xoá clip đã chọn' : 'Xoá clip (chọn một clip trên timeline trước)'} shortcut="Delete" aria-disabled={!selectedIds.length} onClick={() => deleteSelection()} />
      <div className="ml-auto flex shrink-0 items-center gap-1">
        <EditorIconButton icon={Minus} label="Thu nhỏ timeline" shortcut="−" onClick={() => zoomTo(zoom / 1.25)} />
        <Slider aria-label="Thu phóng timeline" min={Math.log2(minimumZoom)} max={Math.log2(ZOOM_MAX)} step={0.001} value={[Math.log2(zoom)]} onValueChange={(value) => zoomTo(2 ** value[0])} onValueCommit={(value) => announce(`Thu phóng ${Math.round(2 ** value[0] * 100)} phần trăm`)} className="w-24" />
        <span className="w-10 text-right text-[10px] tabular-nums text-muted-foreground">{(zoom * 100).toFixed(zoom < 0.01 ? 1 : 0)}%</span>
        <EditorIconButton icon={Plus} label="Phóng to timeline" shortcut="+" onClick={() => zoomTo(zoom * 1.25)} />
        <EditorIconButton icon={Maximize} label="Vừa toàn bộ timeline" shortcut="\\" onClick={fit} />
      </div>
    </div>
    <MarkerListDialog open={markerListOpen} onOpenChange={setMarkerListOpen} />
    {breadcrumbs.length > 1 && <nav aria-label="Composition lồng nhau" className="flex h-7 shrink-0 items-center gap-1 border-b border-border px-2">{breadcrumbs.map((crumb, index) => <span key={crumb.compositionId ?? 'root'} className="flex items-center"><button className="h-7 rounded px-2 hover:bg-secondary" onClick={() => { useCompositionNavigationStore.getState().navigateTo(index); announce(`Đã mở ${crumb.label}`); }}>{index === 0 ? 'Timeline chính' : crumb.label}</button>{index < breadcrumbs.length - 1 && <ChevronRight className="size-3" aria-hidden />}</span>)}</nav>}
    <div ref={scrollRef} className={styles.scroll} role="region" tabIndex={0} aria-label="Vùng track timeline" onScroll={(event) => syncViewport(event.currentTarget)} onPointerMove={move} onPointerUp={finish} onPointerCancel={() => { pendingItemClick.current = null; suppressClick.current = true; gestureRef.current = null; setGesture(null); setGuide(null); }} onKeyDown={(event) => {
      if (!keyboardMarquee) return;
      if (event.key.startsWith('Arrow')) { event.preventDefault(); event.stopPropagation(); setKeyboardMarquee({ ...keyboardMarquee, end: Math.max(0, keyboardMarquee.end + (event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : 0) * (event.shiftKey ? timeline.fps : 1)), lastTrack: Math.max(0, Math.min(tracks.length - 1, keyboardMarquee.lastTrack + (event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0))) }); }
      else if (event.key === 'Enter') { event.preventDefault(); const ids = marqueeItems(timeline.items, keyboardMarquee.start, keyboardMarquee.end, new Set(tracks.slice(Math.min(keyboardMarquee.firstTrack, keyboardMarquee.lastTrack), Math.max(keyboardMarquee.firstTrack, keyboardMarquee.lastTrack) + 1).map((track) => track.id))); useSelectionStore.getState().selectItems(ids); setKeyboardMarquee(null); if (ids[0]) focusItem(ids[0]); announce(`Đã chọn ${ids.length} item`); }
    }} onDragOver={(event) => { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; }} onDrop={async (event) => {
      event.preventDefault(); const position = point(event); const row = trackRows.find((entry) => position.y >= entry.start && position.y < entry.end); const track = row ? tracks[row.index] : undefined;
      if (track?.locked) { announce('Track đang khoá'); return; }
      try {
        const special = event.dataTransfer.getData(LIBRARY_DRAG_MIME);
        if (special) { const data = JSON.parse(special) as LibraryDrop; const item = (indexes.get(track?.id ?? '')?.items ?? []).find((entry) => position.frame >= entry.from && position.frame < entry.from + entry.durationInFrames); applyLibraryDrop(data, item?.id); return; }
        const data = getMediaDragData(); if (!data) return;
        if (data.type === 'media-item' || data.type === 'media-items') {
          const ids = data.items?.map((item) => item.mediaId) ?? (data.mediaId ? [data.mediaId] : []); let frame = position.frame;
          for (const id of ids) { const media = useMediaLibraryStore.getState().mediaById[id]; if (media) { const itemId = await insertMedia(media, frame, track?.id); const item = useTimelineStore.getState().items.find((entry) => entry.id === itemId); if (item) frame += item.durationInFrames; } }
        } else if (data.type === 'timeline-template' && data.itemType === 'text') insertText({ label: data.label, textStylePresetId: data.textStylePresetId }, position.frame, track?.id);
        else if (data.type === 'composition') {
          const { track: target, tracks: plannedTracks, isNew } = destination('video', position.frame, data.durationInFrames, track?.kind !== 'audio' ? track?.id : undefined);
          const item = { id: crypto.randomUUID(), type: 'composition' as const, compositionId: data.compositionId, compositionWidth: data.width, compositionHeight: data.height, label: data.name, from: position.frame, durationInFrames: data.durationInFrames, trackId: target.id };
          if (isNew) actions.addItemOnNewTrack(item, plannedTracks);
          else actions.addItem(item);
        }
      } catch (error) { toast.error(error instanceof Error ? error.message : 'Không thêm được vào timeline'); } finally { clearMediaDragData(); }
    }}>
      <div className={styles.surface} style={{ width, height: Math.max(160, virtualTracks.getTotalSize()) }}>
        <div className={styles.ruler}><Ruler fps={timeline.fps} end={end} scale={scale} startFrame={firstFrame} endFrame={lastFrame} onStart={(event) => begin(event, 'scrub')} /><div className={styles.rulerCorner}><span className="tabular-nums">{timeline.fps} fps</span></div>
          {displayIn !== null && displayOut !== null && <div className={styles.range} style={{ left: HEADER + displayIn * scale, width: Math.max(0, displayOut - displayIn) * scale }} />}
          {(['in', 'out'] as const).map((side) => { const frame = side === 'in' ? displayIn : displayOut; return frame !== null && <button key={side} data-hit-area data-tooltip={side === 'in' ? 'Điểm vào (I)' : 'Điểm ra (O)'} aria-label={side === 'in' ? 'Điểm vào' : 'Điểm ra'} className={styles.rangeHandle} style={{ left: HEADER + frame * scale - 14 }} onPointerDown={(event) => begin(event, side)} onKeyDown={(event) => { if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); event.stopPropagation(); const next = frame + (event.key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? timeline.fps : 1); if (side === 'in') actions.setInPoint(next); else actions.setOutPoint(next); } }}>{side === 'in' ? 'I' : 'O'}</button>; })}
          {timeline.markers.filter((marker) => marker.frame >= firstFrame && marker.frame <= lastFrame).map((marker, index) => {
            const frame = gesture?.kind === 'marker' && gesture.markerId === marker.id ? clampFrame(gesture.frame + gesture.delta) : marker.frame;
            return <button key={marker.id} data-hit-area data-tooltip={`${marker.label || `Marker ${index + 1}`} · ${timecode(frame, timeline.fps)}`} data-marker-id={marker.id} className={styles.marker} style={{ left: HEADER + frame * scale - 14 }} aria-label={`${marker.label || `Marker ${index + 1}`}, ${timecode(frame, timeline.fps)}`} onDoubleClick={() => setMarkerListOpen(true)} onPointerDown={(event) => begin(event, 'marker', undefined, undefined, marker.id)} onKeyDown={(event) => { if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); event.stopPropagation(); actions.updateMarker(marker.id, { frame: clampFrame(marker.frame + (event.key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? timeline.fps : 1)) }); } }}><Flag className="mx-auto size-3" aria-hidden /></button>;
          })}
        </div>
        {virtualTracks.getVirtualItems().map((row) => {
          const track = tracks[row.index]; const entries = visibleItems(displayIndexes.get(track.id), firstFrame, lastFrame, focusedId); const kind = track.kind === 'audio' ? 'audio' : indexes.get(track.id)?.items[0]?.type ?? 'video'; const Icon = TYPE_ICON[kind]; const color = `var(--ed-track-${TYPE_COLOR[kind]})`;
          const dense = isDenseWindow(entries, scale, coarsePointer ? 44 : 24);
          const buttonEntries = dense ? entries.filter((item) => item.durationInFrames * scale >= (coarsePointer ? 44 : 24) || item.id === focusedId || (!focusedId && timeline.items[0]?.id === item.id)) : entries;
          return <div key={track.id} role="group" aria-label={`${track.name} · ${track.kind === 'audio' ? 'Âm thanh' : 'Video'}`} data-track-kind={track.kind} className={styles.track} style={{ top: row.start, height: row.size }}>
            <ContextMenu><ContextMenuTrigger asChild><div className={styles.header} style={{ color }} draggable onDragStart={(event) => { event.dataTransfer.setData('application/x-ytf-track', track.id); }} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { const id = event.dataTransfer.getData('application/x-ytf-track'); if (!id) return; event.preventDefault(); event.stopPropagation(); reorderTrack(id, row.index - tracks.findIndex((entry) => entry.id === id)); }}>
              <div className={styles.trackName}><GripVertical className="size-3 shrink-0 text-muted-foreground" aria-hidden /><Icon className="size-3 shrink-0" aria-hidden />{renameTrack === track.id ? <input autoFocus aria-label={`Tên track ${track.name}`} className="h-7 min-w-0 w-full rounded border border-ring bg-background px-1 text-foreground" defaultValue={track.name} onBlur={(event) => { if (event.target.value.trim()) updateTrack(track.id, { name: event.target.value.trim() }); setRenameTrack(null); focusTrack(track.id); }} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); if (event.key === 'Escape') { event.currentTarget.value = track.name; event.currentTarget.blur(); } }} /> : <button ref={(element) => { if (element) headerRefs.current.set(track.id, element); else headerRefs.current.delete(track.id); }} data-hit-area className="focus-visible:outline-2 focus-visible:outline-ring" onFocus={() => useSelectionStore.getState().setActiveTrack(track.id)} onClick={() => focusTrack(track.id)} onDoubleClick={() => setRenameTrack(track.id)} aria-label={`Track ${track.name}, ${track.kind === 'audio' ? 'âm thanh' : 'video'}${track.templateSlot ? `, vai trò template ${TEMPLATE_SLOT_LABELS[track.templateSlot]}` : ''}`}>{track.name} · {track.kind === 'audio' ? 'Âm thanh' : TYPE_LABEL[kind]}</button>}{track.templateSlot && <span data-tooltip={`Vai trò template: ${TEMPLATE_SLOT_LABELS[track.templateSlot]}`} className="max-w-14 shrink-0 truncate rounded bg-primary/15 px-1 text-[10px] font-medium leading-4 text-primary">{TEMPLATE_SLOT_LABELS[track.templateSlot]}</span>}</div>
              <div className={styles.trackButtons}><EditorIconButton icon={track.locked ? Lock : Unlock} label={`Khoá track ${track.name}`} shortcut="⌃⇧K" aria-pressed={track.locked} onClick={() => { updateTrack(track.id, { locked: !track.locked }); announce(`${track.locked ? 'Đã mở khoá' : 'Đã khoá'} track ${track.name}`); }} /><EditorIconButton icon={track.kind === 'audio' ? track.muted ? VolumeX : Volume2 : track.visible ? Eye : EyeOff} label={track.kind === 'audio' ? `Tắt tiếng ${track.name}` : `Ẩn track ${track.name}`} shortcut="⌃⇧M" aria-pressed={track.kind === 'audio' ? track.muted : !track.visible} onClick={() => { updateTrack(track.id, track.kind === 'audio' ? { muted: !track.muted } : { visible: !track.visible }); announce(`Đã đổi ${track.kind === 'audio' ? 'tắt tiếng' : 'ẩn'} track ${track.name}`); }} /><EditorIconButton icon={Headphones} label={`Solo track ${track.name}`} shortcut="⌃⇧O" aria-pressed={track.solo} onClick={() => { updateTrack(track.id, { solo: !track.solo }); announce(`${track.solo ? 'Đã tắt solo' : 'Solo'} track ${track.name}`); }} /></div>
              <EditorIconButton icon={GripHorizontal} label={`Đổi chiều cao track ${track.name}`} className="absolute bottom-0.5 right-1 cursor-ns-resize" onPointerDown={(event) => begin(event, 'height', undefined, track.id)} onKeyDown={(event) => {
                if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                  event.preventDefault(); event.stopPropagation();
                  const height = resizedTrackHeight(track.height, (event.key === 'ArrowUp' ? -1 : 1) * (event.shiftKey ? 32 : 8), minimumTrackHeight);
                  resizeTrack(track.id, height); announce(`Track ${track.name}: ${height} pixel`);
                }
              }} />
            </div></ContextMenuTrigger><ContextMenuContent onCloseAutoFocus={(event) => { event.preventDefault(); focusTrack(track.id); }}>
              <ContextMenuItem onSelect={() => setRenameTrack(track.id)}>Đổi tên</ContextMenuItem>
              <ContextMenuItem onSelect={() => reorderTrack(track.id, -1)}>Đưa lên</ContextMenuItem>
              <ContextMenuItem onSelect={() => reorderTrack(track.id, 1)}>Đưa xuống</ContextMenuItem>
              {isTemplate && <><ContextMenuSeparator /><ContextMenuSub><ContextMenuSubTrigger>Vai trò template</ContextMenuSubTrigger><ContextMenuSubContent><ContextMenuRadioGroup value={track.templateSlot ?? 'none'} onValueChange={(value) => setTrackRole(track, value)}>
                {(Object.keys(TEMPLATE_SLOT_LABELS) as (keyof typeof TEMPLATE_SLOT_LABELS)[]).map((slot) => <ContextMenuRadioItem key={slot} value={slot} disabled={slot === 'source-audio' ? track.kind !== 'audio' : track.kind === 'audio'} className="min-h-7 [@media(pointer:coarse)]:min-h-11">{TEMPLATE_SLOT_LABELS[slot]}</ContextMenuRadioItem>)}
                <ContextMenuRadioItem value="none" className="min-h-7 [@media(pointer:coarse)]:min-h-11">Không</ContextMenuRadioItem>
              </ContextMenuRadioGroup></ContextMenuSubContent></ContextMenuSub></>}
              <ContextMenuSeparator />
              <ContextMenuItem onSelect={() => addTrack('video', row.index + 1)}>Thêm track video dưới</ContextMenuItem>
              <ContextMenuItem onSelect={() => addTrack('audio', row.index + 1)}>Thêm track âm thanh dưới</ContextMenuItem>
              <div className="px-2 py-1"><NumberField label="Chiều cao track" value={Math.max(minimumTrackHeight, track.height)} min={minimumTrackHeight} max={160} unit="px" onChange={(height) => resizeTrack(track.id, height, false)} onCommit={() => flushTrackHeightOverrides()} /></div>
              <ContextMenuItem onSelect={() => requestRemoveTrack(track.id)}>Xoá track</ContextMenuItem>
            </ContextMenuContent></ContextMenu>
            <div className={styles.lane} onPointerDown={(event) => {
              if (!ownsPointerEvent(event)) return;
              const frame = point(event).frame;
              const hit = dense ? entries.find((item) => item.from <= frame && item.from + item.durationInFrames > frame) : undefined;
              begin(event, hit ? 'move' : event.shiftKey ? 'scrub' : 'marquee', hit, track.id);
            }} onContextMenu={(event) => {
              if (!dense || (event.target instanceof Element && event.target.closest('.ytf-timeline-clip'))) return;
              event.preventDefault();
              const frame = point(event).frame;
              const item = entries.find((entry) => entry.from <= frame && entry.from + entry.durationInFrames > frame); if (!item) return;
              select(item); focusItem(item.id);
              requestAnimationFrame(() => {
                const button = itemRefs.current.get(item.id); if (!button) return;
                const rect = button.getBoundingClientRect();
                button.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: rect.x + 12, clientY: rect.y + 12 }));
              });
            }}>
              {dense && <DenseTrackCanvas items={entries} selected={selectedSet} offset={viewport.left} width={Math.max(1, viewport.width - HEADER)} height={row.size - 12} scale={scale} />}
              {buttonEntries.map((item) => {
                const active = gesture?.item?.id === item.id;
                const duration = item.durationInFrames;
                let fadeIn = (item.type === 'audio' ? item.audioFadeIn : item.fadeIn) ?? 0;
                let fadeOut = (item.type === 'audio' ? item.audioFadeOut : item.fadeOut) ?? 0;
                if (active && gesture.kind === 'fade-in') fadeIn = Math.max(0, Math.min(duration / timeline.fps, fadeIn + gesture.delta / timeline.fps));
                if (active && gesture.kind === 'fade-out') fadeOut = Math.max(0, Math.min(duration / timeline.fps, fadeOut - gesture.delta / timeline.fps));
                return <TimelineClip key={item.id} item={item} keys={keyframesById[item.id]} menu={openMenuItemId === item.id ? itemMenu(item) : null} api={clipApi}
                  selected={selectedSet.has(item.id)} focused={focusedId === item.id}
                  tabbable={focusedId === item.id || (!focusedId && timeline.items[0]?.id === item.id)}
                  trackLocked={track.locked} denseMinWidth={dense ? (coarsePointer ? 44 : 28) : undefined}
                  scale={scale} pixelsPerSecond={pixelsPerSecond} fps={timeline.fps}
                  viewportLeft={viewport.left} viewportWidth={viewport.width - HEADER}
                  fadeIn={fadeIn} fadeOut={fadeOut} />;
              })}
              {(transitionsByTrack.get(track.id) ?? []).map((transition) => {
                const left = itemById.get(transition.leftClipId); if (!left) return null;
                const cut = left.from + left.durationInFrames;
                if (cut < firstFrame || cut > lastFrame) return null;
                return <button key={transition.id} data-hit-area data-tooltip={`Transition ${transition.presentation}`} className={styles.transition} aria-label={`Transition ${transition.presentation}`} style={{ left: cut * scale - 14 }} onPointerDown={(event) => event.stopPropagation()} onClick={() => { useSelectionStore.getState().selectTransition(transition.id); announce(`Đã chọn transition ${transition.presentation}`); }}><MoveHorizontal className="mx-auto size-3" aria-hidden /></button>;
              })}
            </div>
          </div>;
        })}
        {!timeline.items.length && <p className="pointer-events-none absolute left-[204px] top-16 text-xs text-muted-foreground">Nhập media rồi thêm vào timeline để bắt đầu.</p>}
        <Playhead scale={scale} />{guide !== null && <div className={styles.guide} style={{ left: HEADER + guide * scale }} />}
        {gesture?.kind === 'marquee' && <div className={styles.marquee} style={{ left: HEADER + Math.min(gesture.x, gesture.endX), top: Math.min(gesture.y, gesture.endY), width: Math.abs(gesture.endX - gesture.x), height: Math.abs(gesture.endY - gesture.y) }} />}
        {keyboardMarquee && <div className={styles.marquee} style={{ left: HEADER + Math.min(keyboardMarquee.start, keyboardMarquee.end) * scale, width: Math.max(2, Math.abs(keyboardMarquee.end - keyboardMarquee.start) * scale), top: trackRows[Math.min(keyboardMarquee.firstTrack, keyboardMarquee.lastTrack)]?.start ?? 32, height: tracks.slice(Math.min(keyboardMarquee.firstTrack, keyboardMarquee.lastTrack), Math.max(keyboardMarquee.firstTrack, keyboardMarquee.lastTrack) + 1).reduce((sum, track) => sum + Math.max(minimumTrackHeight, track.height), 0) }} />}
      </div>
    </div>
    <div className={styles.footer}><span>{selectedIds.length ? `Đã chọn ${selectedIds.length} clip` : `${timeline.items.length} clip · ${tracks.length} track`}{gesture && ` · ${gesture.delta > 0 ? '+' : ''}${gesture.delta} khung`}</span><span className="tabular-nums">{timecode(end, timeline.fps)}</span></div>
    <AlertDialog open={removeTrackId !== null} onOpenChange={(open) => { if (!open) setRemoveTrackId(null); }}>
      <AlertDialogContent><AlertDialogHeader>
        <AlertDialogTitle>Xoá track {tracks.find((track) => track.id === removeTrackId)?.name}?</AlertDialogTitle>
        <AlertDialogDescription>Track này có {timeline.items.filter((item) => item.trackId === removeTrackId).length} clip, các clip đó cũng sẽ bị xoá. Bạn có thể hoàn tác ngay sau đó.</AlertDialogDescription>
      </AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Huỷ</AlertDialogCancel>
        <AlertDialogAction className="bg-primary text-background" onClick={() => { if (removeTrackId) removeTrackNow(removeTrackId); }}>Xoá track</AlertDialogAction>
      </AlertDialogFooter></AlertDialogContent>
    </AlertDialog>
  </section>;
}
