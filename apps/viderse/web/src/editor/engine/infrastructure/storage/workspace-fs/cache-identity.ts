/**
 * Media content identity for the disposable OPFS cache.
 *
 * On-disk cache paths are already versioned by the server `contentHash`, so a
 * replaced source can never read stale bytes from disk. In-memory state
 * (object URLs, decoded peaks, per-media maps) is keyed by `mediaId` alone and
 * would survive the replacement, so every cache operation feeds the hash it
 * resolved through here and listeners drop whatever they hold for that media.
 */

import { StorageClientError } from '@editor/storage/errors'
import { resolveCacheIdentity, type CacheIdentity } from '@editor/storage/opfs-cache'

export interface MediaContentChange {
  mediaId: string
  /** Hash this tab last observed for the media. */
  previousHash: string
  contentHash: string
}

type MediaContentChangeListener = (change: MediaContentChange) => void

const hashByMedia = new Map<string, string>()
const listeners = new Set<MediaContentChangeListener>()

/**
 * Record the hash seen for `mediaId`. Returns true (and notifies listeners)
 * when it differs from a previously observed hash; the first observation is
 * never a change.
 */
export function observeMediaContentHash(mediaId: string, contentHash: string): boolean {
  const previousHash = hashByMedia.get(mediaId)
  hashByMedia.set(mediaId, contentHash)
  if (previousHash === undefined || previousHash === contentHash) return false
  for (const listener of [...listeners]) listener({ mediaId, previousHash, contentHash })
  return true
}

/** Subscribe to content replacement of any media. Returns an unsubscribe function. */
export function onMediaContentChange(listener: MediaContentChangeListener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

const inFlight = new Map<string, Promise<CacheIdentity>>()

/**
 * Resolve the authoritative identity for `mediaId` and feed the change tracker.
 * Concurrent lookups for one media share a single request (a burst of bin reads
 * must not become a burst of metadata fetches); nothing is cached beyond the
 * request, so a replaced source is seen by the very next operation.
 */
export function resolveMediaIdentity(mediaId: string): Promise<CacheIdentity> {
  const pending = inFlight.get(mediaId)
  if (pending) return pending
  const lookup = resolveCacheIdentity(mediaId)
    .then((identity) => {
      observeMediaContentHash(mediaId, identity.contentHash)
      return identity
    })
    .finally(() => {
      inFlight.delete(mediaId)
    })
  inFlight.set(mediaId, lookup)
  return lookup
}

/**
 * Like {@link resolveMediaIdentity}, but resolves to null when the server no
 * longer has the media — callers treat that as a cache miss / dropped write.
 * Any other failure (network, server error) still rejects.
 */
export async function tryResolveMediaIdentity(mediaId: string): Promise<CacheIdentity | null> {
  try {
    return await resolveMediaIdentity(mediaId)
  } catch (error) {
    if (error instanceof StorageClientError && error.code === 'not_found') return null
    throw error
  }
}

/** Forget tracked hashes — for tests that reuse the module across cases. */
export function resetMediaIdentityTracking(): void {
  hashByMedia.clear()
  inFlight.clear()
}
