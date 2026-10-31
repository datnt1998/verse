import type { MediaMetadata } from '@editor/types/storage';
import type { TimelineItem, TimelineTrack, TextItem } from '@editor/types/timeline';
import type { TransitionPresentation } from '@editor/types/transition';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { useSelectionStore } from '@editor/shared/state/selection';
import { addEffect, addTransition } from '@editor/features/timeline/stores/timeline-actions';
import { executeCommands } from '@editor/commands';
import { resolveMediaUrl } from '@editor/features/media-library/utils/media-resolver';
import { getMediaType } from '@editor/features/media-library/utils/validation';
import { getGpuEffectDefaultParams } from '@editor/infrastructure/gpu-effects/registry';
import { buildOverlayCommands } from '@editor/features/editing-templates/overlay-commands';
import { announce } from '@/editor/ui/shell/announcer';
import { nextTrackName } from '@/editor/ui/timeline/track-names';

export const LIBRARY_DRAG_MIME = 'application/x-ytf-library';
export type LibraryDrop = { type: 'effect'; effectId: string } | { type: 'transition'; presentation: TransitionPresentation };

export function requestMediaImport(): void {
  window.dispatchEvent(new Event('ytf-import-media'));
}

/**
 * Plans where a new item goes: an existing compatible free track, or a new one.
 * A new video track is inserted first (order 0): the composition paints tracks by descending
 * order (order 0 is the top layer), so a track appended at the end would sit under the image
 * layers and the dropped video would be invisible in both the timeline and the preview.
 */
export function destination(kind: 'audio' | 'video', frame: number, duration: number, trackId?: string) {
  const state = useItemsStore.getState();
  // Use the track under the cursor only if it is free at the drop point; otherwise the clip would overlap its neighbour.
  const fits = (t: TimelineTrack) => !t.locked && (t.kind ?? 'video') === kind
    && !state.items.some(i => i.trackId === t.id && i.from < frame + duration && i.from + i.durationInFrames > frame);
  const track = state.tracks.find(t => t.id === trackId && fits(t)) ?? state.tracks.find(t => !t.isGroup && fits(t));
  if (track) return { track, tracks: state.tracks, isNew: false };
  const created: TimelineTrack = { id: crypto.randomUUID(), name: nextTrackName(state.tracks, kind), kind, height: 56, locked: false, visible: true, muted: false, solo: false, order: kind === 'audio' ? state.tracks.length : 0, items: [] };
  const tracks = kind === 'audio'
    ? [...state.tracks, created]
    : [created, ...state.tracks.map(t => ({ ...t, order: t.order + 1 }))];
  return { track: created, tracks, isNew: true };
}

export async function insertMedia(media: MediaMetadata, frame = usePlaybackStore.getState().currentFrame, trackId?: string): Promise<string> {
  const fps = useTimelineSettingsStore.getState().fps;
  const type = getMediaType(media.mimeType);
  if (type !== 'image' && type !== 'audio' && type !== 'video') throw new Error('Loại media chưa được hỗ trợ');
  const duration = Math.max(1, Math.round((type === 'image' ? 5 : media.duration) * fps));
  const src = await resolveMediaUrl(media.id);
  if (!src) throw new Error('Không tìm thấy file. Hãy thay file.');
  const { track, tracks, isNew } = destination(type === 'audio' ? 'audio' : 'video', frame, duration, trackId);
  const item: TimelineItem = { id: crypto.randomUUID(), type, mediaId: media.id, label: media.fileName, from: frame, durationInFrames: duration, trackId: track.id, src, sourceWidth: media.width, sourceHeight: media.height, sourceFps: media.fps || fps, sourceDuration: Math.round(media.duration * (media.fps || fps)), sourceStart: 0, ...(type === 'video' ? { embeddedAudioMuted: !media.audioCodec } : {}) };
  const result = executeCommands([isNew
    ? { type: 'item.addOnNewTracks', params: { items: [item], tracks } }
    : item.type === 'video' && media.audioCodec ? { type: 'item.addWithLinkedAudio', params: { video: item } }
    : { type: 'item.add', params: { item } }]);
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
  useSelectionStore.getState().selectItems([item.id]);
  announce(`Đã thêm ${media.fileName} vào timeline`);
  return item.id;
}

export function insertText(overrides: Partial<TextItem> = {}, frame = usePlaybackStore.getState().currentFrame, trackId?: string): string {
  const fps = useTimelineSettingsStore.getState().fps;
  const { track, tracks, isNew } = destination('video', frame, overrides.durationInFrames ?? fps * 5, trackId);
  const item: TextItem = { id: crypto.randomUUID(), trackId: track.id, from: frame, durationInFrames: fps * 5, label: 'Chữ mới', type: 'text', text: 'Chữ mới', color: '#ffffff', fontFamily: 'Geist', fontSize: 64, ...overrides };
  const result = executeCommands([isNew
    ? { type: 'item.addOnNewTracks', params: { items: [item], tracks } }
    : { type: 'item.add', params: { item } }]);
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
  useSelectionStore.getState().selectItems([item.id]);
  announce(`Đã thêm ${item.label}`);
  return item.id;
}

export function applyLibraryDrop(data: LibraryDrop, itemId = useSelectionStore.getState().selectedItemIds[0]): void {
  const items = useItemsStore.getState().items;
  const item = items.find(i => i.id === itemId);
  if (!item || item.type === 'audio') throw new Error('Chọn một clip hình trên timeline');
  if (data.type === 'effect') {
    addEffect(item.id, { type: 'gpu-effect', gpuEffectType: data.effectId, params: getGpuEffectDefaultParams(data.effectId) });
    announce('Đã thêm hiệu ứng');
  } else {
    const next = items.find(i => i.trackId === item.trackId && i.from === item.from + item.durationInFrames);
    if (!next || !addTransition(item.id, next.id, 'crossfade', undefined, data.presentation)) throw new Error('Cần hai clip liền kề với đủ khung nguồn ở điểm cắt');
    announce('Đã thêm chuyển cảnh');
  }
}

/**
 * One-click overlay insert on a fresh top video track (see `buildOverlayCommands`):
 * the same command a template's overlay uses, so there is one convention.
 */
export async function addOverlay(media: MediaMetadata): Promise<void> {
  const fps = useTimelineSettingsStore.getState().fps;
  const state = useItemsStore.getState();
  const src = await resolveMediaUrl(media.id);
  if (!src) throw new Error('Không tìm thấy file. Hãy thay file.');
  const { commands, itemId } = buildOverlayCommands({ media, src, fps, tracks: state.tracks, items: state.items });
  const result = executeCommands(commands);
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
  useSelectionStore.getState().selectItems([itemId]);
  announce(`Đã thêm lớp phủ ${media.fileName} vào video`);
}
