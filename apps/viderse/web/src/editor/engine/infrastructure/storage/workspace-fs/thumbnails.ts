/**
 * Media and project thumbnails in the disposable OPFS cache.
 *
 * One thumbnail per media, stored as:
 *   `media/{mediaId}/thumbnail.jpg`
 * which the cache layer versions by the server content hash, so replacing a
 * source never serves its old thumbnail. Project thumbnails are regenerated
 * on every save and are cache-only too.
 *
 * The legacy IDB supported multiple thumbnails per media but in practice
 * getThumbnailByMediaId always returned the first — so we collapse to one
 * per media. If a caller saves a new thumbnail it overwrites the existing.
 *
 * v2 note: the `thumbnail.meta.json` sidecar was dropped. `ThumbnailData.id`
 * is derived from `mediaId` on read; callers that want a change-marker
 * (cache-busting) should keep tracking `media.thumbnailId` on the media
 * metadata record rather than reading it from the blob.
 *
 * Project thumbnails live under `projects/{projectId}/thumbnail.jpg`
 * (see `saveProjectThumbnail` / `loadProjectThumbnail`) — do not pass
 * project ids into the media-scoped helpers below.
 */

import type { ThumbnailData } from '@editor/types/storage'
import { createLogger } from '@editor/shared/logging/logger'

import { getEditorStorage } from '@editor/storage/storage-backend'
import { observeMediaContentHash } from './cache-identity'
import { requireWorkspaceRoot } from './root'
import { readBlob, removeEntry, writeBlob } from './fs-primitives'
import { mediaThumbnailPath, projectThumbnailPath } from './paths'
import { blobToArrayBuffer } from './blob-utils'
import { mapWithConcurrency } from '@editor/shared/utils/async-utils'

const logger = createLogger('WorkspaceFS:Thumbnails')

/* ────────────────────────────── Public API ───────────────────────────── */

export async function saveThumbnail(thumbnail: ThumbnailData): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    const bytes = new Uint8Array(await blobToArrayBuffer(thumbnail.blob))
    await writeBlob(root, mediaThumbnailPath(thumbnail.mediaId), bytes)
  } catch (error) {
    logger.error('saveThumbnail failed', error)
    throw new Error('Failed to save thumbnail', { cause: error })
  }
}

/**
 * Legacy lookup-by-thumbnail-id. In v2 the id is derived from mediaId, so
 * `id` and `mediaId` are interchangeable — callers should migrate to
 * `getThumbnailByMediaId`.
 */
export async function getThumbnail(id: string): Promise<ThumbnailData | undefined> {
  return getThumbnailByMediaId(id)
}

export async function getThumbnailByMediaId(mediaId: string): Promise<ThumbnailData | undefined> {
  const root = requireWorkspaceRoot()
  try {
    const blob = await readBlob(root, mediaThumbnailPath(mediaId))
    if (!blob) return undefined
    return {
      id: mediaId,
      mediaId,
      blob,
      timestamp: 0,
      width: 0,
      height: 0,
    }
  } catch (error) {
    logger.error(`getThumbnailByMediaId(${mediaId}) failed`, error)
    return undefined
  }
}

/**
 * Batch-read thumbnails for many media at once.
 *
 * One `listMedia()` call supplies every content hash, so the reads below pin
 * their cache identity instead of paying a metadata round-trip per id. Reads
 * run with bounded concurrency; missing thumbnails are simply absent from the
 * returned map. This warms callers' caches on project load so cards render
 * without each mounting an independent async fetch.
 */
const THUMBNAIL_READ_CONCURRENCY = 8

export async function getThumbnailsByMediaIds(mediaIds: string[]): Promise<Map<string, Blob>> {
  const result = new Map<string, Blob>()
  if (mediaIds.length === 0) return result

  const root = requireWorkspaceRoot()
  const storage = getEditorStorage()
  let workspaceId: string
  let hashById: Map<string, string>
  try {
    const [id, records] = await Promise.all([storage.workspaceId(), storage.listMedia()])
    workspaceId = id
    hashById = new Map(records.map((record) => [record.id, record.contentHash]))
  } catch (error) {
    logger.warn('getThumbnailsByMediaIds: failed to resolve media identities', error)
    return result
  }

  const reads = await mapWithConcurrency(
    mediaIds,
    THUMBNAIL_READ_CONCURRENCY,
    async (mediaId): Promise<{ mediaId: string; blob: Blob } | null> => {
      const contentHash = hashById.get(mediaId)
      if (!contentHash) return null
      observeMediaContentHash(mediaId, contentHash)
      try {
        const blob = await readBlob(root, mediaThumbnailPath(mediaId), {
          workspaceId,
          mediaId,
          contentHash,
        })
        return blob ? { mediaId, blob } : null
      } catch (error) {
        logger.warn(`getThumbnailsByMediaIds(${mediaId}) failed`, error)
        return null
      }
    },
  )

  for (const read of reads) {
    if (read) result.set(read.mediaId, read.blob)
  }
  return result
}

export async function deleteThumbnailsByMediaId(mediaId: string): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    await removeEntry(root, mediaThumbnailPath(mediaId))
  } catch (error) {
    logger.error(`deleteThumbnailsByMediaId(${mediaId}) failed`, error)
    throw new Error('Failed to delete thumbnails')
  }
}

/* ─────────────────────────── Project thumbnails ───────────────────────── */

export async function saveProjectThumbnail(projectId: string, blob: Blob): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    const bytes = new Uint8Array(await blobToArrayBuffer(blob))
    await writeBlob(root, projectThumbnailPath(projectId), bytes)
  } catch (error) {
    logger.error(`saveProjectThumbnail(${projectId}) failed`, error)
    throw new Error('Failed to save project thumbnail', { cause: error })
  }
}

export async function loadProjectThumbnail(projectId: string): Promise<Blob | undefined> {
  const root = requireWorkspaceRoot()
  try {
    const blob = await readBlob(root, projectThumbnailPath(projectId))
    return blob ?? undefined
  } catch (error) {
    logger.error(`loadProjectThumbnail(${projectId}) failed`, error)
    return undefined
  }
}
