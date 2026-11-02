/**
 * Adapter exports for media-library dependencies.
 * Export modules should import media resolution helpers from here.
 */

import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { importMediaLibraryService } from '@editor/features/media-library/services/media-library-service-loader'
import type { MediaMetadata } from '@editor/types/storage'

export {
  resolveMediaUrl,
  resolveMediaUrls,
  resolveProxyUrl,
} from '@editor/features/media-library/utils/media-resolver'

export function getMediaAudioCodecById(mediaId: string | undefined): string | undefined {
  if (!mediaId) return undefined

  const media = useMediaLibraryStore.getState().mediaById[mediaId]
  if (!media) return undefined

  if (media.mimeType.startsWith('video/')) {
    return media.audioCodec
  }
  if (media.mimeType.startsWith('audio/')) {
    return media.codec
  }
  return undefined
}

export type MediaAudioPresence = 'present' | 'absent' | 'unknown'

/**
 * Whether a media item's own container carries an audio track.
 *
 * Only `absent` is a confirmed negative — a probe saw the streams and found no
 * audio track. Missing metadata or an unprobed record stays `unknown`, so the
 * mixer still attempts a decode and a corrupt source produces a real failure
 * instead of silently disappearing from the mix.
 */
export function getMediaAudioPresenceById(mediaId: string | undefined): MediaAudioPresence {
  if (!mediaId) return 'unknown'

  const media = useMediaLibraryStore.getState().mediaById[mediaId]
  if (!media) return 'unknown'
  if (media.audioTrackPresent === false) return 'absent'
  if (media.audioTrackPresent === true) return 'present'
  if (media.mimeType.startsWith('audio/')) return 'present'
  // A codec without the explicit presence flag is still positive evidence.
  if (media.mimeType.startsWith('video/') && media.audioCodec) return 'present'
  return 'unknown'
}

export function getMediaMetadataById(mediaId: string | undefined): MediaMetadata | undefined {
  if (!mediaId) return undefined
  return useMediaLibraryStore.getState().mediaById[mediaId]
}

export function useMediaMetadataById(mediaId: string | undefined): MediaMetadata | undefined {
  return useMediaLibraryStore((state) => (mediaId ? state.mediaById[mediaId] : undefined))
}

export async function getMediaFileById(mediaId: string): Promise<Blob | null> {
  const { mediaLibraryService } = await importMediaLibraryService()
  return mediaLibraryService.getMediaFile(mediaId)
}

export function useBrokenMediaIds(): string[] {
  return useMediaLibraryStore((state) => state.brokenMediaIds)
}
