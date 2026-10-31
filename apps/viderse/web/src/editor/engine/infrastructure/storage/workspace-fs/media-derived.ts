/**
 * Browser-derived media metadata, stored as a disposable OPFS cache.
 *
 * The server owns the durable record (probe fields, tags, attribution). Values only the
 * client can compute — keyframe index, WebCodecs support flags, the thumbnail change-marker,
 * and caller-supplied overrides such as an exact interpolated frame rate — are layered on top
 * of it. The overlay lives at `media/<id>/<contentHash>/derived.json` in the versioned cache,
 * so replacing a media's bytes (a new hash) drops it automatically and a cleared cache simply
 * means the next import/analysis pass recomputes it.
 */

import type { MediaMetadata } from '@editor/types/storage'
import { getEditorStorage } from '@editor/storage/storage-backend'
import {
  readCacheBlob,
  removeCacheEntry,
  writeCacheBlob,
  type CacheIdentity,
} from '@editor/storage/opfs-cache'

import { withKeyLock } from './with-key-lock'
import { observeMediaContentHash } from './cache-identity'

export const DERIVED_MEDIA_KEYS = [
  'duration',
  'width',
  'height',
  'fps',
  'codec',
  'bitrate',
  'audioCodec',
  'audioCodecSupported',
  'videoCodecSupported',
  'keyframeTimestamps',
  'gopInterval',
  'thumbnailId',
  'aiCaptions',
] as const satisfies ReadonlyArray<keyof MediaMetadata>

export type DerivedMediaKey = (typeof DERIVED_MEDIA_KEYS)[number]
export type DerivedMediaMetadata = Partial<Pick<MediaMetadata, DerivedMediaKey>>

const DERIVED_FILE = 'derived.json'
const DERIVED_KEY_LOOKUP: Record<string, true> = Object.fromEntries(
  DERIVED_MEDIA_KEYS.map((key) => [key, true] as const),
)

export function isDerivedMediaKey(key: string): key is DerivedMediaKey {
  return DERIVED_KEY_LOOKUP[key] === true
}

async function identityFor(mediaId: string, contentHash: string): Promise<CacheIdentity> {
  // Pinned hashes still count as observations: a caller holding an older hash than the server
  // has now is exactly the stale-identity signal the tracker propagates.
  observeMediaContentHash(mediaId, contentHash)
  return { workspaceId: await getEditorStorage().workspaceId(), mediaId, contentHash }
}

function pickDerived(value: unknown): DerivedMediaMetadata {
  if (typeof value !== 'object' || value === null) return {}
  const picked: Record<string, unknown> = {}
  for (const [key, entry] of Object.entries(value)) {
    if (isDerivedMediaKey(key) && entry !== undefined) picked[key] = entry
  }
  return picked as DerivedMediaMetadata
}

export async function readDerivedMedia(
  mediaId: string,
  contentHash: string,
): Promise<DerivedMediaMetadata> {
  const blob = await readCacheBlob(
    ['media', mediaId, DERIVED_FILE],
    await identityFor(mediaId, contentHash),
  )
  if (!blob) return {}
  try {
    return pickDerived(JSON.parse(await blob.text()))
  } catch {
    // A torn or corrupt cache entry is a miss.
    return {}
  }
}

/** Merge `updates` into the overlay. `undefined` values remove the field. */
export async function writeDerivedMedia(
  mediaId: string,
  contentHash: string,
  updates: DerivedMediaMetadata,
): Promise<DerivedMediaMetadata> {
  return withKeyLock(`media-derived:${mediaId}`, async () => {
    const merged: Record<string, unknown> = { ...(await readDerivedMedia(mediaId, contentHash)) }
    for (const [key, value] of Object.entries(updates)) {
      if (!isDerivedMediaKey(key)) continue
      if (value === undefined) delete merged[key]
      else merged[key] = value
    }
    await writeCacheBlob(
      ['media', mediaId, DERIVED_FILE],
      JSON.stringify(merged),
      await identityFor(mediaId, contentHash),
    )
    return merged as DerivedMediaMetadata
  })
}

/** Drop the overlay of every version of a media (delete, relink). */
export async function removeDerivedMedia(mediaId: string): Promise<void> {
  await removeCacheEntry(['media', mediaId], true)
}
