/**
 * Media source bytes, owned by the server.
 *
 * Imports and replacements upload a `File` (a disk-backed Blob, which the browser streams
 * without copying it into JS memory). Reads are normally direct `mediaUrl(id, contentHash)`
 * URLs that `<video>`/mediabunny range-request; `readMediaSource` exists only for the few
 * paths that need an actual Blob (decoding, proxy generation inputs).
 */

import type { MediaMetadata } from '@editor/types/storage'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { StorageClientError } from '@editor/storage/errors'

import { hydrateServerMedia } from './media'
import { removeDerivedMedia } from './media-derived'

export interface MediaUploadOptions {
  signal?: AbortSignal
  /** MIME type to record when the browser's `file.type` is empty or too generic. */
  mimeType?: string
  /** Bytes sent so far, and the total. */
  onProgress?: (sent: number, total: number) => void
}

export interface MediaImportSourceOptions extends MediaUploadOptions {
  /** Link the new media to this project as part of the import. */
  projectId?: string
}

/** Direct, immutable-cacheable URL of a media's current bytes. */
export function mediaSourceUrl(media: Pick<MediaMetadata, 'id' | 'contentHash'>): string {
  if (!media.contentHash) throw new Error(`Media ${media.id} has no content hash`)
  return getEditorStorage().mediaUrl(media.id, media.contentHash)
}

/** Fetch the source bytes as a Blob. Returns null when the media does not exist. */
export async function readMediaSource(mediaId: string): Promise<Blob | null> {
  const storage = getEditorStorage()
  let url: string
  try {
    const record = await storage.getMedia(mediaId)
    url = storage.mediaUrl(record.id, record.contentHash)
  } catch (error) {
    if (error instanceof StorageClientError && error.code === 'not_found') return null
    throw error
  }
  const response = await fetch(url)
  if (response.status === 404) return null
  if (!response.ok) {
    throw new StorageClientError('internal_error', `Cannot read media ${mediaId}`, response.status)
  }
  return response.blob()
}

export async function hasMediaSource(mediaId: string): Promise<boolean> {
  try {
    await getEditorStorage().getMedia(mediaId)
    return true
  } catch (error) {
    if (error instanceof StorageClientError && error.code === 'not_found') return false
    throw error
  }
}

/** Upload a new media. The server probes it and hashes its bytes. */
export async function importMediaSource(
  file: File,
  options: MediaImportSourceOptions = {},
): Promise<MediaMetadata> {
  const record = await getEditorStorage().importMedia(file, {
    fileName: file.name,
    mime: options.mimeType || file.type || undefined,
    projectId: options.projectId,
    signal: options.signal,
    onProgress: options.onProgress,
  })
  return hydrateServerMedia(record)
}

/**
 * Replace a media's bytes. The id is kept, the content hash changes, and every cache entry
 * of the previous bytes is discarded.
 */
export async function replaceMediaSource(
  mediaId: string,
  file: File,
  options: MediaUploadOptions = {},
): Promise<MediaMetadata> {
  const record = await getEditorStorage().replaceMedia(mediaId, file, {
    fileName: file.name,
    mime: options.mimeType || file.type || undefined,
    signal: options.signal,
    onProgress: options.onProgress,
  })
  await removeDerivedMedia(mediaId)
  return hydrateServerMedia(record)
}
