"use client";

import { useEffect } from 'react';
import { toast } from '@/editor/ui/toast';
import { getCommandExecutor, executeCommands, captureProjectState } from '@editor/commands';
import { startMediaMetadataSync } from '@editor/features/media-library/services/media-metadata-sync';
import { createMediaSourceSync } from '@editor/features/media-library/services/media-source-sync';
import { mediaLibraryService } from '@editor/features/media-library/services/media-library-service';
import { getMedia } from '@editor/infrastructure/storage';
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store';
import { useProjectStore } from '@editor/features/projects/stores/project-store';
import { getEditorStorage } from '@editor/storage/storage-backend';
import { MEDIA_SOURCE_CHANNEL, MEDIA_SOURCE_EVENT, visitSourceItems } from '@editor/storage/media-source-refresh';
import { observeMediaContentHash } from '@editor/infrastructure/storage/workspace-fs/cache-identity';
import { blobUrlManager } from '@editor/infrastructure/browser/blob-url-manager';
import { invalidatePredecodedSource } from '@editor/features/preview/utils/decoder-prewarm';
import { invalidateDecodedMediaAudio } from '@editor/runtime/composition-runtime/utils/audio-decode-cache';
import { gifFrameCache } from '@editor/features/timeline/services/gif-frame-cache';
import { proxyService } from '@editor/features/media-library/services/proxy-service';
import { announce } from './announcer';

export function MediaMetadataSync() {
  useEffect(() => startMediaMetadataSync(getCommandExecutor(), (_mediaId, error) => {
    console.error('media metadata save failed', error);
    const message = 'Chưa lưu được thông tin của file vừa thêm. Hãy thử thêm lại file đó.';
    toast.error(message); announce(message, 'assertive');
  }), []);
  useEffect(() => {
    let dispose: (() => void) | undefined;
    let stopped = false;
    const fail = (error: unknown) => { console.error('media source sync failed', error); toast.error('Chưa cập nhật được file vừa thay đổi. Hãy tải lại trang nếu hình không đổi.'); };
    void getEditorStorage().workspaceId().then(workspaceId => {
      if (stopped) return;
      const sync = createMediaSourceSync({
        workspaceId,
        current: id => useMediaLibraryStore.getState().mediaById[id],
        fetchMedia: getMedia,
        invalidate(id, oldUrl, hash) {
          const previous = useMediaLibraryStore.getState().mediaById[id];
          const urls = new Set<string>();
          if (oldUrl) urls.add(oldUrl);
          visitSourceItems(captureProjectState(), item => { if (item.mediaId === id) { if (item.src) urls.add(item.src); if (item.audioSrc) urls.add(item.audioSrc); } });
          for (const src of urls) invalidatePredecodedSource(src);
          if (previous?.contentHash) observeMediaContentHash(id, previous.contentHash);
          observeMediaContentHash(id, hash);
          blobUrlManager.invalidate(id);
          mediaLibraryService.clearThumbnailCache(id);
          gifFrameCache.clearMemory(id);
          invalidateDecodedMediaAudio(id);
          proxyService.clearMemory(id);
          proxyService.setProxyKey(id, `${id}@${hash}`);
        },
        apply(media, src) {
          const result = executeCommands([{ type: 'media.refreshSource', params: { media, src } }]);
          if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
        },
      });
      const channel = typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel(MEDIA_SOURCE_CHANNEL);
      const message = (event: MessageEvent) => { void sync.handle(event.data).catch(fail); };
      const local = (event: Event) => { void sync.handle((event as CustomEvent).detail).catch(fail); };
      let refreshing = false;
      const refresh = async () => {
        const projectId = useProjectStore.getState().currentProject?.id;
        if (!projectId || refreshing || document.visibilityState === 'hidden') return;
        refreshing = true;
        try {
          const records = await getEditorStorage().listMedia(projectId);
          await Promise.all(records.map(record => sync.handle({ workspaceId, mediaId: record.id, contentHash: record.contentHash })));
        } finally { refreshing = false; }
      };
      const focus = () => { void refresh().catch(fail); };
      channel?.addEventListener('message', message);
      window.addEventListener(MEDIA_SOURCE_EVENT, local);
      window.addEventListener('focus', focus);
      document.addEventListener('visibilitychange', focus);
      dispose = () => { channel?.close(); window.removeEventListener(MEDIA_SOURCE_EVENT, local); window.removeEventListener('focus', focus); document.removeEventListener('visibilitychange', focus); };
    }).catch(fail);
    return () => { stopped = true; dispose?.(); };
  }, []);
  return null;
}
