import type { MediaMetadata } from '@editor/types/storage'
import { createLogger, createOperationId } from '@editor/shared/logging/logger'
import {
  deleteMedia as deleteMediaDB,
  deleteThumbnailsByMediaId,
  importMediaSource,
  saveThumbnail as saveThumbnailDB,
  updateMedia as updateMediaDB,
  type MediaMetadataUpdate,
} from '@editor/features/media-library/deps/storage'

const logger = createLogger('MediaAssetHelpers')

function getImageDimensionsFromBitmap(bitmap: ImageBitmap): { width: number; height: number } {
  return {
    width: bitmap.width,
    height: bitmap.height,
  }
}

export function getThumbnailDimensions(
  width: number,
  height: number,
  maxSize: number,
): { width: number; height: number } {
  let w = Number(width)
  if (!Number.isFinite(w)) w = 1
  let h = Number(height)
  if (!Number.isFinite(h)) h = 1
  let m = Number(maxSize)
  if (!Number.isFinite(m)) m = 1

  const safeWidth = Math.max(1, Math.round(w))
  const safeHeight = Math.max(1, Math.round(h))
  const safeMax = Math.max(1, Math.round(m))

  if (safeWidth >= safeHeight) {
    return {
      width: safeMax,
      height: Math.max(1, Math.floor((safeMax * safeHeight) / safeWidth)),
    }
  }

  return {
    width: Math.max(1, Math.floor((safeMax * safeWidth) / safeHeight)),
    height: safeMax,
  }
}

export async function getGeneratedImageDimensions(
  file: File,
): Promise<{ width: number; height: number }> {
  if (typeof createImageBitmap === 'function') {
    let bitmap: ImageBitmap | undefined
    try {
      bitmap = await createImageBitmap(file)
      const dimensions = getImageDimensionsFromBitmap(bitmap)
      return dimensions
    } catch {
      // Fall through to Image-based fallback
    } finally {
      bitmap?.close()
    }
  }

  return new Promise((resolve, reject) => {
    const image = new Image()
    const url = URL.createObjectURL(file)

    image.onload = () => {
      URL.revokeObjectURL(url)
      resolve({
        width: image.naturalWidth,
        height: image.naturalHeight,
      })
    }

    image.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('Failed to load generated image'))
    }

    image.src = url
  })
}

/**
 * Persist a media thumbnail (a disposable cache) and return its change-marker id, or
 * `undefined` when the dimensions are unusable and nothing was written.
 */
export async function saveMediaThumbnail(
  mediaId: string,
  blob: Blob,
  width: number | undefined,
  height: number | undefined,
): Promise<string | undefined> {
  const rawWidth = Number(width)
  const rawHeight = Number(height)
  if (!(Number.isFinite(rawWidth) && rawWidth > 0 && Number.isFinite(rawHeight) && rawHeight > 0)) {
    return undefined
  }
  const thumbnailId = crypto.randomUUID()
  await saveThumbnailDB({
    id: thumbnailId,
    mediaId,
    blob,
    timestamp: 0,
    width: Math.max(1, Math.floor(Math.abs(rawWidth))),
    height: Math.max(1, Math.floor(Math.abs(rawHeight))),
  })
  return thumbnailId
}

/**
 * The part of a client-built `MediaMetadata` the server's probe cannot know: browser analysis
 * (keyframes, codec support) and caller-supplied overrides (exact fps) go to the contentHash
 * keyed overlay, while user tags and attribution are durable on the server.
 */
export function mediaMetadataToUpdate(media: MediaMetadata): MediaMetadataUpdate {
  const update: MediaMetadataUpdate = {
    duration: media.duration,
    width: media.width,
    height: media.height,
    fps: media.fps,
    codec: media.codec,
    bitrate: media.bitrate,
    audioCodec: media.audioCodec,
    audioCodecSupported: media.audioCodecSupported,
    videoCodecSupported: media.videoCodecSupported,
    keyframeTimestamps: media.keyframeTimestamps,
    gopInterval: media.gopInterval,
  }
  if (media.tags.length > 0) update.tags = media.tags
  if (media.attribution) update.attribution = media.attribution
  return update
}

interface PersistGeneratedMediaOptions {
  file: File
  /** Link the imported media to this project; omitted imports only enter the library. */
  projectId?: string
  /** Client-side view of the media; the server record supplies id, hash, size and file name. */
  mediaMetadata: MediaMetadata
  thumbnailBlob?: Blob
  thumbnailWidth?: number
  thumbnailHeight?: number
  signal?: AbortSignal
  onProgress?: (sent: number, total: number) => void
}

/**
 * Upload `file` to the server (which probes, hashes and links it to `projectId`), then store the
 * browser-side extras. The upload is the only durable, irreversible step: a failure after it
 * deletes the server media again so a half-imported asset never lingers in the library.
 */
export async function persistGeneratedMediaAsset({
  file,
  projectId,
  mediaMetadata,
  thumbnailBlob,
  thumbnailWidth,
  thumbnailHeight,
  signal,
  onProgress,
}: PersistGeneratedMediaOptions): Promise<MediaMetadata> {
  const opId = createOperationId()
  const event = logger.startEvent('persistGeneratedMediaAsset', opId)
  event.merge({ projectId })

  const imported = await importMediaSource(file, {
    projectId,
    mimeType: mediaMetadata.mimeType,
    signal,
    onProgress,
  }).catch((error: unknown) => {
    event.failure(error, { rollback: { mediaDeleted: 'none' } })
    throw error
  })
  event.merge({ mediaId: imported.id })

  let thumbnailSaved = false
  try {
    const updates = mediaMetadataToUpdate(mediaMetadata)

    if (thumbnailBlob) {
      const thumbnailId = await saveMediaThumbnail(
        imported.id,
        thumbnailBlob,
        thumbnailWidth,
        thumbnailHeight,
      )
      if (thumbnailId) {
        thumbnailSaved = true
        updates.thumbnailId = thumbnailId
      }
    }

    const persisted = await updateMediaDB(imported.id, updates)
    event.success({ projectId, mediaId: imported.id })
    return persisted
  } catch (error) {
    try {
      await deleteMediaDB(imported.id)
    } catch (cleanupError) {
      logger.warn(`Failed to roll back imported media ${imported.id}:`, cleanupError)
    }

    if (thumbnailSaved) {
      try {
        await deleteThumbnailsByMediaId(imported.id)
      } catch (cleanupError) {
        logger.warn(`Failed to roll back generated thumbnail ${imported.id}:`, cleanupError)
      }
    }

    event.failure(error, {
      rollback: {
        mediaDeleted: 'attempted',
        thumbnailDeleted: thumbnailSaved ? 'attempted' : 'none',
      },
    })
    throw error
  }
}
