import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { proxyService } from '@editor/features/media-library/services/proxy-service'
import { getSharedProxyKey } from '@editor/features/media-library/utils/proxy-key'
import { getMedia, mediaSourceUrl } from '@editor/features/media-library/deps/storage'
import { blobUrlManager } from '@editor/infrastructure/browser/blob-url-manager'
import { registerKeyframeIndex } from '@editor/shared/utils/keyframe-index-registry'
import type { TimelineTrack } from '@editor/types/timeline'
import { createLogger } from '@editor/shared/logging/logger'

const logger = createLogger('MediaResolver')

/**
 * Resolves a mediaId to the URL of its source bytes for use in Composition Player.
 *
 * The URL is the server's immutable `mediaUrl(id, contentHash)`: the browser range-requests
 * and caches it, nothing is read into JS memory, and replacing the source bytes changes the
 * hash and therefore the URL.
 *
 * @param mediaId - The ID of the media in the media library
 * @returns URL for the media, or empty string if not found
 */
export async function resolveMediaUrl(mediaId: string): Promise<string> {
  try {
    // The library store already holds the hash for every loaded media; only fall back to a
    // server read for media it has not hydrated yet.
    const loaded = useMediaLibraryStore.getState().mediaById[mediaId]
    const media = loaded?.contentHash === undefined ? await getMedia(mediaId) : loaded

    if (!media) {
      logger.warn(`Media not found: ${mediaId}`)
      return '' // Fallback: empty string (Composition will skip)
    }

    const url = mediaSourceUrl(media)

    // Register keyframe index for adaptive seek backtracking
    if (media.keyframeTimestamps && media.keyframeTimestamps.length > 0) {
      registerKeyframeIndex(url, media.keyframeTimestamps)
    }

    // Resolved successfully — clear any stale broken flag so the clip stops showing the
    // offline state without needing a reload.
    useMediaLibraryStore.getState().markMediaHealthy(mediaId)

    return url
  } catch (error) {
    logger.error(`Failed to resolve media ${mediaId}:`, error)
    return '' // Fallback: empty string
  }
}

/**
 * Resolves a proxy URL for a media item if available.
 * Returns null if no proxy exists (caller should fall back to full-res).
 */
export function resolveProxyUrl(mediaId: string): string | null {
  const media = useMediaLibraryStore.getState().mediaById[mediaId]
  if (media) {
    const proxyKey = getSharedProxyKey(media)
    // Safety net for legacy state restores where the mapping may be missing.
    if (proxyService.getProxyKey(mediaId) !== proxyKey) {
      proxyService.setProxyKey(mediaId, proxyKey)
    }
    return proxyService.getProxyBlobUrl(mediaId, proxyKey)
  }

  return proxyService.getProxyBlobUrl(mediaId)
}

/**
 * Resolves all media URLs in timeline tracks
 * Creates a deep clone of tracks with resolved blob URLs
 *
 * @param tracks - Timeline tracks with media items
 * @param options.useProxy - If true, prefer proxy URLs for video items (default: true)
 * @returns Tracks with resolved blob URLs in item.src
 */
export async function resolveMediaUrls(
  tracks: TimelineTrack[],
  options?: { useProxy?: boolean; signal?: AbortSignal },
): Promise<TimelineTrack[]> {
  const useProxy = options?.useProxy ?? true
  const signal = options?.signal

  // Deep clone tracks to avoid mutating original
  const resolvedTracks: TimelineTrack[] = structuredClone(tracks)

  // Resolve all media URLs in parallel
  const resolutionPromises: Promise<void>[] = []

  for (const track of resolvedTracks) {
    for (const item of track.items) {
      // Only resolve media items with mediaId
      if (
        item.mediaId &&
        (item.type === 'video' ||
          item.type === 'audio' ||
          item.type === 'image' ||
          item.type === 'lottie')
      ) {
        const promise = resolveMediaUrl(item.mediaId).then((blobUrl) => {
          // For video items in preview mode, prefer proxy URL if available
          if (useProxy && item.type === 'video') {
            const proxyUrl = resolveProxyUrl(item.mediaId!)
            item.src = proxyUrl || blobUrl
            item.audioSrc = blobUrl
          } else {
            item.src = blobUrl
            if (item.type === 'video') {
              item.audioSrc = blobUrl
            }
          }
        })
        resolutionPromises.push(promise)
      }
    }
  }

  // Wait for all resolutions to complete
  await Promise.all(resolutionPromises)

  // Check if aborted after resolution
  if (signal?.aborted) {
    throw new DOMException('Media resolution aborted', 'AbortError')
  }

  return resolvedTracks
}

/**
 * Releases every blob URL held by the centralized manager.
 * Call this on component unmount to prevent memory leaks. Source media URLs are plain
 * server URLs and own no memory.
 */
export function cleanupBlobUrls(): void {
  blobUrlManager.releaseAll()
}
