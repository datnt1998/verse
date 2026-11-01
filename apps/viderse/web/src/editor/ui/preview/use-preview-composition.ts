"use client";

import { useEffect, useMemo, useState, type RefObject } from 'react';
import { toast } from '@/editor/ui/toast';
import type { PlayerRef } from '@editor/runtime/player/Player';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store';
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store';
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store';
import { useProjectStore } from '@editor/features/projects/stores/project-store';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { resolveMediaUrl } from '@editor/features/media-library/utils/media-resolver';

/**
 * The composition the preview canvas draws, built from the loaded project's stores. Shared by the editor's
 * preview panel and the read-only template player so both resolve media and size the video the same way.
 */
export function usePreviewComposition() {
  const items = useItemsStore(s => s.items); const tracks = useItemsStore(s => s.tracks);
  const transitions = useTransitionsStore(s => s.transitions); const keyframes = useKeyframesStore(s => s.keyframes);
  const fps = useTimelineSettingsStore(s => s.fps); const project = useProjectStore(s => s.currentProject);
  const width = project?.metadata.width ?? 1920; const height = project?.metadata.height ?? 1080;
  const backgroundColor = project?.metadata.backgroundColor ?? '#000000';
  const duration = Math.max(1, ...items.map(i => i.from + i.durationInFrames));
  const inputTracks = useMemo(() => tracks.map(track => ({ ...track, items: items.filter(i => i.trackId === track.id) })), [tracks, items]);
  // Sources are resolved once per mediaId: editing a clip (colour, text) rebuilds the resolved tracks
  // from the same item objects and does not make the network or the Player redo work on every edit.
  const mediaIds = useMemo(() => {
    const ids: string[] = [];
    for (const item of items) {
      if (item.mediaId && (item.type === 'video' || item.type === 'audio' || item.type === 'image' || item.type === 'lottie') && !ids.includes(item.mediaId)) ids.push(item.mediaId);
    }
    return ids;
  }, [items]);
  const [sources, setSources] = useState<Record<string, string>>({});
  useEffect(() => {
    const missing = mediaIds.filter(id => !(id in sources));
    if (!missing.length) return;
    const controller = new AbortController();
    void Promise.all(missing.map(async id => [id, await resolveMediaUrl(id)] as const)).then(entries => {
      if (!controller.signal.aborted) setSources(current => ({ ...current, ...Object.fromEntries(entries.filter(([, url]) => url)) }));
    }).catch(error => { if (!controller.signal.aborted) toast.error(`Không đọc được media: ${error instanceof Error ? error.message : 'Thử thay file'}`); });
    return () => controller.abort();
  }, [mediaIds, sources]);
  const resolved = useMemo(() => inputTracks.map(track => ({
    ...track,
    items: track.items.map(item => {
      const url = item.mediaId ? sources[item.mediaId] : undefined;
      if (!url) return item;
      return item.type === 'video' ? { ...item, src: url, audioSrc: url } : { ...item, src: url };
    }),
  })), [inputTracks, sources]);
  const composition = useMemo(() => ({ fps, width, height, durationInFrames: duration, tracks: resolved, transitions, keyframes, backgroundColor }), [fps, width, height, duration, resolved, transitions, keyframes, backgroundColor]);
  return { items, fps, width, height, duration, resolved, transitions, keyframes, backgroundColor, composition };
}

/** Keeps a Player's frame, rate and play state equal to the playback store, which is the single transport. */
export function usePlayerStoreSync(player: RefObject<PlayerRef | null>) {
  useEffect(() => {
    const sync = () => {
      const state = usePlaybackStore.getState(); const ref = player.current; if (!ref) return;
      const frame = state.previewFrame ?? state.currentFrame;
      if (ref.getCurrentFrame() !== frame) ref.seekTo(frame);
      if (ref.getPlaybackRate() !== state.playbackRate) ref.setPlaybackRate(state.playbackRate);
      if (ref.isPlaying() !== state.isPlaying) { if (state.isPlaying) ref.play(); else ref.pause(); }
    };
    sync();
    return usePlaybackStore.subscribe(sync);
  }, [player]);
}
