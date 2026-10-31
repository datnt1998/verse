/**
 * Generic OPFS cache store for derived data (proxies, conformed video,
 * waveforms, …).
 *
 * Despite the legacy "mirror" names, there is no second copy anywhere: entries
 * live once, in the versioned OPFS cache (`CACHE_ROOT`), scoped by workspace id
 * and — for `media/<id>/...` paths — by the server content hash. Everything
 * stored here is disposable; a failed write or an evicted entry is simply a
 * miss the caller regenerates from the server's source media.
 *
 * Paths are `string[]` segments, namespace-agnostic: callers pick the
 * convention (`['content', 'proxies', key, 'proxy.mp4']`,
 * `['media', id, 'cache', 'filmstrip', '3.jpg']`, …).
 */

import { createLogger } from '@editor/shared/logging/logger'

import { requireWorkspaceRoot } from './root'
import { exists, readBlob, removeEntry, writeBlob, writeJsonAtomic } from './fs-primitives'

const logger = createLogger('WorkspaceFS:CacheMirror')

/** Store a blob at `segments`. Idempotent: an existing entry is kept. Never throws. */
export async function mirrorBlobToWorkspace(segments: string[], blob: Blob): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    if (await exists(root, segments)) return
    await writeBlob(root, segments, blob)
  } catch (error) {
    logger.warn(`mirrorBlobToWorkspace(${segments.join('/')}) failed`, error)
  }
}

/** Same as {@link mirrorBlobToWorkspace} for ArrayBuffer/Uint8Array inputs. */
export async function mirrorBytesToWorkspace(
  segments: string[],
  bytes: ArrayBuffer | Uint8Array,
): Promise<void> {
  const root = requireWorkspaceRoot()
  try {
    if (await exists(root, segments)) return
    await writeBlob(root, segments, bytes)
  } catch (error) {
    logger.warn(`mirrorBytesToWorkspace(${segments.join('/')}) failed`, error)
  }
}

/** Store a JSON value (replacing any existing entry). Never throws. */
export async function mirrorJsonToWorkspace(segments: string[], data: unknown): Promise<void> {
  try {
    await writeJsonAtomic(requireWorkspaceRoot(), segments, data)
  } catch (error) {
    logger.warn(`mirrorJsonToWorkspace(${segments.join('/')}) failed`, error)
  }
}

/** Read a cached blob. Resolves to null on any miss (absent, evicted, media deleted). */
export async function readWorkspaceBlob(segments: string[]): Promise<Blob | null> {
  try {
    return await readBlob(requireWorkspaceRoot(), segments)
  } catch (error) {
    logger.warn(`readWorkspaceBlob(${segments.join('/')}) failed`, error)
    return null
  }
}

/** Remove a cached file or subtree. No-op when absent. */
export async function removeWorkspaceCacheEntry(
  segments: string[],
  options: { recursive?: boolean } = {},
): Promise<void> {
  try {
    await removeEntry(requireWorkspaceRoot(), segments, options)
  } catch (error) {
    logger.warn(`removeWorkspaceCacheEntry(${segments.join('/')}) failed`, error)
  }
}
