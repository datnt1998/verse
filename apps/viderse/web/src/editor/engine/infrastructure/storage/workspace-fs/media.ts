/**
 * Media metadata, backed by the server `EditorStorage`.
 *
 * The durable record (probe fields, tags, attribution) lives on the server. Values only the
 * browser can compute are layered on top from the OPFS derived-metadata cache, keyed by
 * `contentHash` (see `media-derived.ts`). Nothing here persists a file handle.
 */

import type { MediaAttribution, MediaMetadata } from '@editor/types/storage'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { StorageClientError } from '@editor/storage/errors'
import { serverMediaToMetadata } from '@editor/storage/media-adapter'
import type { ServerMediaRecord } from '@editor/storage/types'

import {
  isDerivedMediaKey,
  readDerivedMedia,
  removeDerivedMedia,
  writeDerivedMedia,
  type DerivedMediaMetadata,
} from './media-derived'

import { observeMediaContentHash } from './cache-identity'

/**
 * Fields a caller may change on an existing media. Tags and attribution are durable and go
 * to the server; the rest are browser-derived and go to the contentHash-keyed overlay.
 * Source identity (file name, size, hash, mime) only changes by replacing the media bytes.
 */
export type MediaMetadataUpdate = DerivedMediaMetadata & {
  tags?: string[]
  /** `null` clears the attribution. */
  attribution?: MediaAttribution | null
}

export async function hydrateServerMedia(record: ServerMediaRecord): Promise<MediaMetadata> {
  // Every server record the tab sees feeds the change tracker, so in-memory caches keyed by
  // media id (object URLs, decoded peaks) are dropped when a replacement surfaces — even when
  // a sibling tab or a stale in-memory record did the replacing.
  observeMediaContentHash(record.id, record.contentHash)
  const derived = await readDerivedMedia(record.id, record.contentHash)
  return { ...serverMediaToMetadata(record), ...derived }
}

export async function getAllMedia(): Promise<MediaMetadata[]> {
  const records = await getEditorStorage().listMedia()
  return Promise.all(records.map(hydrateServerMedia))
}

export async function getMedia(id: string): Promise<MediaMetadata | undefined> {
  try {
    return await hydrateServerMedia(await getEditorStorage().getMedia(id))
  } catch (error) {
    if (error instanceof StorageClientError && error.code === 'not_found') return undefined
    throw error
  }
}

export async function updateMedia(id: string, updates: MediaMetadataUpdate): Promise<MediaMetadata> {
  const storage = getEditorStorage()
  const derivedUpdates: DerivedMediaMetadata = {}
  for (const [key, value] of Object.entries(updates)) {
    if (key === 'tags' || key === 'attribution') continue
    if (!isDerivedMediaKey(key)) throw new Error(`Cannot update media field "${key}"`)
    Object.assign(derivedUpdates, { [key]: value })
  }

  let record = await storage.getMedia(id)
  if ('tags' in updates || 'attribution' in updates) {
    record = await storage.updateMediaMetadata(id, {
      ...('tags' in updates && { tags: updates.tags }),
      ...('attribution' in updates && { attribution: updates.attribution ?? null }),
    })
  }
  if (Object.keys(derivedUpdates).length > 0) {
    await writeDerivedMedia(id, record.contentHash, derivedUpdates)
  }
  return hydrateServerMedia(record)
}

/** Delete the media on the server, then drop every cached derivative of it. */
export async function deleteMedia(id: string): Promise<void> {
  await getEditorStorage().deleteMedia(id)
  await removeDerivedMedia(id)
}
